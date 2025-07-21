package server

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"runtime"
	"strconv"
	"testing"
	"time"

	"github.com/phayes/freeport"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/server/v3/embed"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	voyagerv1 "github.com/kolkov/voyager/gen/proto/voyager/v1"
)

// startEmbeddedETCD starts an embedded ETCD server for tests
func startEmbeddedETCD(t *testing.T) (string, func()) {
	// Skip on Windows due to instability
	if runtime.GOOS == "windows" {
		t.Skip("Skipping ETCD test on Windows due to instability")
	}

	clientPort, err := freeport.GetFreePort()
	require.NoError(t, err, "Failed to get free port")
	peerPort, err := freeport.GetFreePort()
	require.NoError(t, err, "Failed to get free port")

	// Use 127.0.0.1 instead of localhost to avoid DNS issues
	clientURL := url.URL{Scheme: "http", Host: "127.0.0.1:" + strconv.Itoa(clientPort)}
	peerURL := url.URL{Scheme: "http", Host: "127.0.0.1:" + strconv.Itoa(peerPort)}

	// Create unique temp directory
	dir, err := os.MkdirTemp("", "etcd-test")
	require.NoError(t, err, "Failed to create temp dir")

	cfg := embed.NewConfig()
	cfg.Name = "test-node"
	cfg.Dir = dir
	cfg.ListenClientUrls = []url.URL{clientURL}
	cfg.AdvertiseClientUrls = []url.URL{clientURL}
	cfg.ListenPeerUrls = []url.URL{peerURL}
	cfg.AdvertisePeerUrls = []url.URL{peerURL}
	cfg.InitialCluster = fmt.Sprintf("%s=%s", cfg.Name, peerURL.String())
	cfg.ClusterState = embed.ClusterStateFlagNew
	cfg.Logger = "zap"
	cfg.LogLevel = "error"
	cfg.LogOutputs = []string{"stderr"}

	etcd, err := embed.StartEtcd(cfg)
	require.NoError(t, err, "Failed to start embedded ETCD")

	select {
	case <-etcd.Server.ReadyNotify():
		t.Logf("Embedded ETCD server ready at: %s", clientURL.String())

		// Add retry logic for client connection
		var cli *clientv3.Client
		for i := 0; i < 5; i++ {
			cli, err = clientv3.New(clientv3.Config{
				Endpoints:   []string{clientURL.String()},
				DialTimeout: 5 * time.Second,
			})
			if err == nil {
				if closeErr := cli.Close(); closeErr != nil {
					t.Logf("failed to close etcd client: %v", closeErr)
				}
				break
			}
			time.Sleep(500 * time.Millisecond)
		}
		require.NoError(t, err, "Failed to connect to embedded ETCD")

		return clientURL.String(), func() {
			etcd.Close()
			if removeErr := os.RemoveAll(dir); removeErr != nil {
				t.Logf("failed to remove temp dir: %v", removeErr)
			}
		}
	case <-time.After(30 * time.Second):
		etcd.Close()
		if err := os.RemoveAll(dir); err != nil {
			t.Logf("failed to remove temp dir: %v", err)
		}
		t.Fatal("Timed out waiting for ETCD to start")
		return "", nil
	}
}

// TestNewServer tests server creation
func TestNewServer(t *testing.T) {
	t.Run("InMemory mode", func(t *testing.T) {
		srv, err := NewServer(Config{
			CacheTTL: time.Minute,
		})
		require.NoError(t, err)
		defer srv.Close()
		assert.True(t, srv.inMemory)
	})

	t.Run("ETCD mode", func(t *testing.T) {
		endpoint, cleanup := startEmbeddedETCD(t)
		defer cleanup()
		time.Sleep(500 * time.Millisecond) // Give server time to stabilize

		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		srv, err := NewServer(Config{
			ETCDEndpoints: []string{endpoint},
			CacheTTL:      30 * time.Second,
		})
		require.NoError(t, err, "Failed to create server")
		defer srv.Close()

		assert.False(t, srv.inMemory)
		assert.NotNil(t, srv.etcdClient)

		// Test registration
		reg := &voyagerv1.Registration{
			ServiceName: "test-service",
			InstanceId:  "instance-1",
			Address:     "127.0.0.1", // Use IP instead of localhost
			Port:        8080,
		}

		resp, err := srv.Register(ctx, reg)
		require.NoError(t, err, "Registration failed")
		assert.True(t, resp.Success)

		// Test discovery
		list, err := srv.Discover(ctx, &voyagerv1.ServiceQuery{
			ServiceName: "test-service",
		})
		require.NoError(t, err, "Discovery failed")
		require.Len(t, list.Instances, 1)
		assert.Equal(t, reg, list.Instances[0])

		// Test health check
		healthResp, err := srv.HealthCheck(ctx, &voyagerv1.HealthRequest{
			ServiceName: "test-service",
			InstanceId:  "instance-1",
		})
		require.NoError(t, err, "Health check failed")
		assert.Equal(t, voyagerv1.HealthResponse_HEALTHY, healthResp.Status)

		// Test deregistration
		deregResp, err := srv.Deregister(ctx, &voyagerv1.InstanceID{
			ServiceName: "test-service",
			InstanceId:  "instance-1",
		})
		require.NoError(t, err, "Deregistration failed")
		assert.True(t, deregResp.Success)

		// Verify removal - should return empty list
		list, err = srv.Discover(ctx, &voyagerv1.ServiceQuery{
			ServiceName: "test-service",
		})
		require.NoError(t, err)
		assert.Len(t, list.Instances, 0)
	})

	t.Run("ETCD fallback to in-memory", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("Skipping on Windows due to instability")
		}

		srv, err := NewServer(Config{
			ETCDEndpoints: []string{"http://invalid-host:2379"},
			CacheTTL:      time.Minute,
		})
		require.NoError(t, err)
		defer srv.Close()
		assert.True(t, srv.inMemory)
	})
}

// TestAuthInterceptor tests authentication middleware
func TestAuthInterceptor(t *testing.T) {
	srv := &Server{authToken: "test-token"}

	t.Run("Valid token", func(t *testing.T) {
		ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "test-token"))
		_, err := srv.AuthInterceptor(ctx, nil, nil, func(_ context.Context, req interface{}) (interface{}, error) {
			return nil, nil
		})
		assert.NoError(t, err)
	})

	t.Run("Invalid token", func(t *testing.T) {
		ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "wrong-token"))
		_, err := srv.AuthInterceptor(ctx, nil, nil, nil)
		assert.Equal(t, codes.PermissionDenied, status.Code(err))
	})

	t.Run("Missing metadata", func(t *testing.T) {
		_, err := srv.AuthInterceptor(context.Background(), nil, nil, nil)
		assert.Equal(t, codes.Unauthenticated, status.Code(err))
	})
}

// TestRegisterAndDiscover tests service registration and discovery
func TestRegisterAndDiscover(t *testing.T) {
	srv := createInMemoryServer(t)
	defer srv.Close()

	reg := &voyagerv1.Registration{
		ServiceName: "test-service",
		InstanceId:  "instance-1",
		Address:     "127.0.0.1", // Use IP instead of localhost
		Port:        8080,
	}

	// Register service
	resp, err := srv.Register(context.Background(), reg)
	require.NoError(t, err)
	assert.True(t, resp.Success)

	// Discover service
	list, err := srv.Discover(context.Background(), &voyagerv1.ServiceQuery{
		ServiceName: "test-service",
	})
	require.NoError(t, err)
	require.Len(t, list.Instances, 1)
	assert.Equal(t, reg, list.Instances[0])

	// Discover non-existing service should return empty list
	list, err = srv.Discover(context.Background(), &voyagerv1.ServiceQuery{
		ServiceName: "missing-service",
	})
	require.NoError(t, err)
	assert.Len(t, list.Instances, 0)
}

// TestHealthCheck tests health status reporting
func TestHealthCheck(t *testing.T) {
	srv := createInMemoryServer(t)
	defer srv.Close()

	reg := registerTestService(t, srv)

	t.Run("Healthy instance", func(t *testing.T) {
		resp, err := srv.HealthCheck(context.Background(), &voyagerv1.HealthRequest{
			ServiceName: reg.ServiceName,
			InstanceId:  reg.InstanceId,
		})
		require.NoError(t, err)
		assert.Equal(t, voyagerv1.HealthResponse_HEALTHY, resp.Status)
	})

	t.Run("Unhealthy instance", func(t *testing.T) {
		resp, err := srv.HealthCheck(context.Background(), &voyagerv1.HealthRequest{
			ServiceName: "missing",
			InstanceId:  "instance",
		})
		require.NoError(t, err)
		assert.Equal(t, voyagerv1.HealthResponse_UNHEALTHY, resp.Status)
	})
}

// TestDeregister tests service deregistration
func TestDeregister(t *testing.T) {
	srv := createInMemoryServer(t)
	defer srv.Close()

	reg := registerTestService(t, srv)

	resp, err := srv.Deregister(context.Background(), &voyagerv1.InstanceID{
		ServiceName: reg.ServiceName,
		InstanceId:  reg.InstanceId,
	})
	require.NoError(t, err)
	assert.True(t, resp.Success)

	// Verify removal - should return empty list
	list, err := srv.Discover(context.Background(), &voyagerv1.ServiceQuery{
		ServiceName: reg.ServiceName,
	})
	require.NoError(t, err)
	assert.Len(t, list.Instances, 0)
}

// TestJanitorCleanup tests expired instance cleanup
func TestJanitorCleanup(t *testing.T) {
	srv := createInMemoryServer(t)
	defer srv.Close()

	reg := registerTestService(t, srv)

	srv.mu.Lock()
	srv.cacheTTL = 100 * time.Millisecond
	srv.mu.Unlock()

	time.Sleep(150 * time.Millisecond)

	srv.cleanupExpiredInstances()

	srv.mu.RLock()
	defer srv.mu.RUnlock()
	_, exists := srv.inMemoryInstances[reg.ServiceName]
	assert.False(t, exists)
}

// TestEtcdAdapter tests ETCD adapter operations
func TestEtcdAdapter(t *testing.T) {
	endpoint, cleanup := startEmbeddedETCD(t)
	defer cleanup()
	time.Sleep(500 * time.Millisecond) // Stabilize connection

	adapter, err := NewEtcdAdapter([]string{endpoint})
	require.NoError(t, err)
	defer func() {
		if err2 := adapter.Close(); err2 != nil {
			t.Logf("failed to close etcd adapter: %v", err2)
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second) // Increased timeout
	defer cancel()

	key := "/test/key"
	value := map[string]string{"test": "value"}

	// Test Put
	err = adapter.Put(ctx, key, value, 5*time.Second)
	require.NoError(t, err)

	// Test GetPrefix
	data, err := adapter.GetPrefix(ctx, "/test/")
	require.NoError(t, err)
	require.Len(t, data, 1)

	// Test Delete
	err = adapter.Delete(ctx, key)
	require.NoError(t, err)

	// Verify deletion
	data, err = adapter.GetPrefix(ctx, "/test/")
	require.NoError(t, err)
	assert.Empty(t, data)
}

// TestMetrics tests metrics collection
func TestMetrics(t *testing.T) {
	srv := createInMemoryServer(t)
	defer srv.Close()

	serviceInstancesGauge.Reset()

	reg := registerTestService(t, srv)
	srv.UpdateServiceMetrics()

	metrics, err := prometheus.DefaultGatherer.Gather()
	require.NoError(t, err)

	var found bool
	for _, mf := range metrics {
		if mf.GetName() == "voyager_service_instances" {
			for _, metric := range mf.GetMetric() {
				for _, label := range metric.GetLabel() {
					if label.GetName() == "service" && label.GetValue() == reg.ServiceName {
						assert.Equal(t, 1.0, metric.GetGauge().GetValue())
						found = true
					}
				}
			}
		}
	}
	assert.True(t, found, "Metric not found")
}

// Helper functions

// createInMemoryServer creates in-memory server for tests
func createInMemoryServer(t *testing.T) *Server {
	srv, err := NewServer(Config{
		CacheTTL: time.Minute,
	})
	require.NoError(t, err)
	return srv
}

// registerTestService registers test service
func registerTestService(t *testing.T, srv *Server) *voyagerv1.Registration {
	reg := &voyagerv1.Registration{
		ServiceName: "test-service",
		InstanceId:  "instance-1",
		Address:     "127.0.0.1", // Use IP instead of localhost
		Port:        8080,
	}
	_, err := srv.Register(context.Background(), reg)
	require.NoError(t, err)
	return reg
}
