package integration_test

import (
	"context"
	"fmt"
	"github.com/phayes/freeport"
	"go.etcd.io/etcd/server/v3/embed"
	"log"
	"net"
	"net/url"
	"os"
	"runtime"
	"strconv"
	"testing"
	"time"

	voyagerv1 "github.com/kolkov/voyager/proto/voyager/v1"
	"github.com/kolkov/voyager/server"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/test/bufconn"
)

const (
	bufSize   = 1024 * 1024
	timeout   = 2 * time.Second
	testToken = "test-auth-token"
)

// setupTestEnvironment prepares an isolated testing environment
func setupTestEnvironment(t *testing.T) (voyagerv1.DiscoveryClient, func()) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping integration test on Windows")
	}

	// Create in-memory etcd
	endpoint, cleanupETCD := setupEmbeddedETCD(t)

	// Create discovery server
	srv, err := server.NewServer(server.Config{
		ETCDEndpoints: []string{endpoint},
		CacheTTL:      time.Minute,
		AuthToken:     testToken,
	})
	require.NoError(t, err)

	// Create in-memory gRPC server
	lis := bufconn.Listen(bufSize)
	grpcSrv := grpc.NewServer(
		grpc.UnaryInterceptor(srv.AuthInterceptor),
	)
	voyagerv1.RegisterDiscoveryServer(grpcSrv, srv)

	// Start server in background
	go func() {
		if err := grpcSrv.Serve(lis); err != nil {
			log.Printf("gRPC server exited: %v", err)
		}
	}()

	// Create authenticated client
	conn, err := grpc.DialContext(context.Background(), "bufnet",
		grpc.WithContextDialer(func(ctx context.Context, s string) (net.Conn, error) {
			return lis.Dial()
		}),
		grpc.WithPerRPCCredentials(&authCreds{token: testToken}),
		grpc.WithInsecure(),
	)
	require.NoError(t, err)

	client := voyagerv1.NewDiscoveryClient(conn)

	return client, func() {
		conn.Close()
		grpcSrv.Stop()
		cleanupETCD()
		srv.Close()
	}
}

// authCreds implements PerRPCCredentials
type authCreds struct {
	token string
}

func (c *authCreds) GetRequestMetadata(ctx context.Context, uri ...string) (map[string]string, error) {
	return map[string]string{"authorization": c.token}, nil
}

func (c *authCreds) RequireTransportSecurity() bool {
	return false
}

// TestServiceRegistration verifies registration flow
func TestServiceRegistration(t *testing.T) {
	client, cleanup := setupTestEnvironment(t)
	defer cleanup()

	reg := &voyagerv1.Registration{
		ServiceName: "test-service",
		InstanceId:  "instance-1",
		Address:     "localhost",
		Port:        8080,
		Metadata:    map[string]string{"env": "test"},
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	resp, err := client.Register(ctx, reg)
	require.NoError(t, err)
	require.True(t, resp.Success)

	discoverResp, err := client.Discover(ctx, &voyagerv1.ServiceQuery{
		ServiceName: "test-service",
	})
	require.NoError(t, err)
	require.Len(t, discoverResp.Instances, 1)
	require.Equal(t, reg.ServiceName, discoverResp.Instances[0].ServiceName)
	require.Equal(t, reg.Address, discoverResp.Instances[0].Address)
	require.Equal(t, reg.Port, discoverResp.Instances[0].Port)
}

// TestHealthCheck verifies health mechanism
func TestHealthCheck(t *testing.T) {
	client, cleanup := setupTestEnvironment(t)
	defer cleanup()

	reg := &voyagerv1.Registration{
		ServiceName: "health-service",
		InstanceId:  "health-instance-1",
		Address:     "localhost",
		Port:        9090,
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	_, err := client.Register(ctx, reg)
	require.NoError(t, err)

	healthResp, err := client.HealthCheck(ctx, &voyagerv1.HealthRequest{
		ServiceName: "health-service",
		InstanceId:  "health-instance-1",
	})
	require.NoError(t, err)
	require.Equal(t, voyagerv1.HealthResponse_HEALTHY, healthResp.Status)
}

// TestDeregistration verifies removal process
func TestDeregistration(t *testing.T) {
	client, cleanup := setupTestEnvironment(t)
	defer cleanup()

	reg := &voyagerv1.Registration{
		ServiceName: "temp-service",
		InstanceId:  "temp-instance",
		Address:     "localhost",
		Port:        7070,
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	_, err := client.Register(ctx, reg)
	require.NoError(t, err)

	deregResp, err := client.Deregister(ctx, &voyagerv1.InstanceID{
		ServiceName: "temp-service",
		InstanceId:  "temp-instance",
	})
	require.NoError(t, err)
	require.True(t, deregResp.Success)

	discoverResp, err := client.Discover(ctx, &voyagerv1.ServiceQuery{
		ServiceName: "temp-service",
	})
	require.NoError(t, err)
	require.Len(t, discoverResp.Instances, 0)
}

// setupEmbeddedETCD creates embedded ETCD server
func setupEmbeddedETCD(t *testing.T) (string, func()) {
	clientPort, err := freeport.GetFreePort()
	require.NoError(t, err, "Failed to get free port")
	peerPort, err := freeport.GetFreePort()
	require.NoError(t, err, "Failed to get free port")

	clientURL := url.URL{Scheme: "http", Host: "localhost:" + strconv.Itoa(clientPort)}
	peerURL := url.URL{Scheme: "http", Host: "localhost:" + strconv.Itoa(peerPort)}

	dir, err := os.MkdirTemp("", "etcd-test")
	require.NoError(t, err, "Failed to create temp dir")

	cfg := embed.NewConfig()
	cfg.Dir = dir
	cfg.ListenClientUrls = []url.URL{clientURL}
	cfg.AdvertiseClientUrls = []url.URL{clientURL}
	cfg.ListenPeerUrls = []url.URL{peerURL}
	cfg.AdvertisePeerUrls = []url.URL{peerURL}
	cfg.LogLevel = "error"
	cfg.Name = "test-node"
	cfg.InitialCluster = fmt.Sprintf("%s=%s", cfg.Name, peerURL.String())
	cfg.ClusterState = embed.ClusterStateFlagNew

	etcd, err := embed.StartEtcd(cfg)
	require.NoError(t, err, "Failed to start embedded ETCD")

	select {
	case <-etcd.Server.ReadyNotify():
		time.Sleep(1 * time.Second)
		t.Logf("Embedded ETCD server ready at: %s", clientURL.String())
		return clientURL.String(), func() {
			etcd.Close()
			os.RemoveAll(dir)
		}
	case <-time.After(15 * time.Second):
		etcd.Close()
		os.RemoveAll(dir)
		t.Fatal("Timed out waiting for ETCD to start")
		return "", func() {}
	}
}
