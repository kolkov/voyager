package client

import (
	"context"
	"errors"
	"log"
	"net"
	"testing"
	"time"

	voyagerv1 "github.com/kolkov/voyager/gen/proto/voyager/v1"
	"github.com/patrickmn/go-cache"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

// MockDiscoveryClient simulates the behavior of the discovery service
type MockDiscoveryClient struct {
	mock.Mock
}

func (m *MockDiscoveryClient) Register(
	ctx context.Context,
	req *voyagerv1.Registration,
	opts ...grpc.CallOption,
) (*voyagerv1.Response, error) {
	args := m.Called(ctx, req)
	return args.Get(0).(*voyagerv1.Response), args.Error(1)
}

func (m *MockDiscoveryClient) Discover(
	ctx context.Context,
	req *voyagerv1.ServiceQuery,
	opts ...grpc.CallOption,
) (*voyagerv1.ServiceList, error) {
	args := m.Called(ctx, req)
	return args.Get(0).(*voyagerv1.ServiceList), args.Error(1)
}

func (m *MockDiscoveryClient) HealthCheck(
	ctx context.Context,
	req *voyagerv1.HealthRequest,
	opts ...grpc.CallOption,
) (*voyagerv1.HealthResponse, error) {
	args := m.Called(ctx, req)
	return args.Get(0).(*voyagerv1.HealthResponse), args.Error(1)
}

func (m *MockDiscoveryClient) Deregister(
	ctx context.Context,
	req *voyagerv1.InstanceID,
	opts ...grpc.CallOption,
) (*voyagerv1.Response, error) {
	args := m.Called(ctx, req)
	return args.Get(0).(*voyagerv1.Response), args.Error(1)
}

// MockConnectionPool simulates connection pool behavior
type MockConnectionPool struct {
	mock.Mock
}

func (m *MockConnectionPool) Get(ctx context.Context, address string) (*grpc.ClientConn, error) {
	args := m.Called(ctx, address)
	conn := args.Get(0)
	if conn == nil {
		return nil, args.Error(1)
	}
	return conn.(*grpc.ClientConn), args.Error(1)
}

func (m *MockConnectionPool) Release(address string) {
	m.Called(address)
}

func (m *MockConnectionPool) Close() {
	m.Called()
}

func (m *MockConnectionPool) ConnectionCount(address string) int64 {
	args := m.Called(address)
	return args.Get(0).(int64)
}

// TestClient_Register tests service registration functionality
func TestClient_Register(t *testing.T) {
	t.Run("Successful registration", func(t *testing.T) {
		mockClient := new(MockDiscoveryClient)
		cli := &Client{
			discoverySvc: mockClient,
			options: &Options{
				TTL: 30 * time.Second,
			},
			cache: cache.New(30*time.Second, 10*time.Minute),
		}

		mockClient.On("Register", mock.Anything, mock.MatchedBy(func(req *voyagerv1.Registration) bool {
			return req.ServiceName == "test-service" &&
				req.Address == "localhost" &&
				req.Port == 8080
		})).Return(&voyagerv1.Response{Success: true}, nil)

		err := cli.Register("test-service", "localhost", 8080, nil)
		assert.NoError(t, err)
		mockClient.AssertExpectations(t)
	})

	t.Run("Registration failure", func(t *testing.T) {
		mockClient := new(MockDiscoveryClient)
		cli := &Client{
			discoverySvc: mockClient,
			options:      &Options{},
			cache:        cache.New(30*time.Second, 10*time.Minute),
		}

		mockClient.On("Register", mock.Anything, mock.Anything).Return(
			&voyagerv1.Response{Success: false, Error: "registration failed"},
			nil,
		)

		err := cli.Register("test-service", "localhost", 8080, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "registration failed")
	})
}

// TestClient_Discover tests service discovery functionality
func TestClient_Discover(t *testing.T) {
	t.Run("Successful discovery", func(t *testing.T) {
		mockClient := new(MockDiscoveryClient)
		mockPool := new(MockConnectionPool)

		cli := &Client{
			discoverySvc: mockClient,
			options: &Options{
				TTL: 30 * time.Second,
			},
			connectionPool: mockPool,
			balancer:       newRoundRobinBalancer(),
			cache:          cache.New(30*time.Second, 10*time.Minute),
		}

		instances := []*voyagerv1.Registration{
			{ServiceName: "test-service", Address: "localhost", Port: 8080},
		}

		mockClient.On("Discover", mock.Anything, mock.Anything).Return(
			&voyagerv1.ServiceList{Instances: instances},
			nil,
		)

		conn, err := grpc.NewClient(
			"passthrough:///localhost:0",
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		)
		assert.NoError(t, err)
		defer func() {
			if closeErr := conn.Close(); closeErr != nil {
				t.Logf("failed to close connection: %v", closeErr)
			}
		}()

		mockPool.On("Get", mock.Anything, "localhost:8080").Return(conn, nil)

		result, err := cli.Discover(context.Background(), "test-service")
		assert.NoError(t, err)
		assert.NotNil(t, result)
		mockClient.AssertExpectations(t)
		mockPool.AssertExpectations(t)
	})

	t.Run("No instances available", func(t *testing.T) {
		mockClient := new(MockDiscoveryClient)
		cli := &Client{
			discoverySvc: mockClient,
			options: &Options{
				TTL: 30 * time.Second,
			},
			connectionPool: NewConnectionPool(&Options{ConnectionTimeout: 100 * time.Millisecond}),
			balancer:       newRoundRobinBalancer(),
			cache:          cache.New(30*time.Second, 10*time.Minute),
		}

		mockClient.On("Discover", mock.Anything, mock.Anything).Return(
			&voyagerv1.ServiceList{Instances: []*voyagerv1.Registration{}},
			nil,
		)

		conn, err := cli.Discover(context.Background(), "test-service")
		assert.Error(t, err)
		assert.Nil(t, conn)
		assert.Contains(t, err.Error(), "no instances available")
	})

	t.Run("Discovery error", func(t *testing.T) {
		mockClient := new(MockDiscoveryClient)
		cli := &Client{
			discoverySvc: mockClient,
			options: &Options{
				TTL: 30 * time.Second,
			},
			connectionPool: NewConnectionPool(&Options{ConnectionTimeout: 100 * time.Millisecond}),
			balancer:       newRoundRobinBalancer(),
			cache:          cache.New(30*time.Second, 10*time.Minute),
		}

		var nilList *voyagerv1.ServiceList
		mockClient.On("Discover", mock.Anything, mock.Anything).Return(
			nilList,
			errors.New("discovery error"),
		)

		conn, err := cli.Discover(context.Background(), "test-service")
		assert.Error(t, err)
		assert.Nil(t, conn)
		assert.Contains(t, err.Error(), "discovery error")
	})
}

// TestClient_HealthCheck tests health check functionality
func TestClient_HealthCheck(t *testing.T) {
	t.Run("Successful health check", func(t *testing.T) {
		mockClient := new(MockDiscoveryClient)
		cli := &Client{
			discoverySvc: mockClient,
			options: &Options{
				HealthCheckInterval: 100 * time.Millisecond,
			},
			serviceName: "test-service",
			instanceID:  "test-instance",
			cache:       cache.New(30*time.Second, 10*time.Minute),
		}

		healthReq := &voyagerv1.HealthRequest{
			ServiceName: "test-service",
			InstanceId:  "test-instance",
		}

		mockClient.On("HealthCheck", mock.Anything, healthReq).Return(
			&voyagerv1.HealthResponse{Status: voyagerv1.HealthResponse_HEALTHY},
			nil,
		)

		cli.startHealthChecks()
		time.Sleep(150 * time.Millisecond)
		cli.stopHealthChecks()

		mockClient.AssertExpectations(t)
	})

	t.Run("Health check failure with re-registration", func(t *testing.T) {
		mockClient := new(MockDiscoveryClient)
		cli := &Client{
			discoverySvc: mockClient,
			options: &Options{
				HealthCheckInterval: 100 * time.Millisecond,
			},
			serviceName: "test-service",
			instanceID:  "test-instance",
			address:     "localhost",
			port:        8080,
			cache:       cache.New(30*time.Second, 10*time.Minute),
		}

		healthReq := &voyagerv1.HealthRequest{
			ServiceName: "test-service",
			InstanceId:  "test-instance",
		}

		// First health check fails
		mockClient.On("HealthCheck", mock.Anything, healthReq).Return(
			(*voyagerv1.HealthResponse)(nil),
			status.Error(codes.Unavailable, "service unavailable"),
		).Once()

		// Re-registration succeeds
		mockClient.On("Register", mock.Anything, mock.MatchedBy(func(req *voyagerv1.Registration) bool {
			return req.ServiceName == "test-service" &&
				req.InstanceId == "test-instance" &&
				req.Address == "localhost" &&
				req.Port == 8080
		})).Return(
			&voyagerv1.Response{Success: true},
			nil,
		)

		// Subsequent health checks succeed
		mockClient.On("HealthCheck", mock.Anything, healthReq).Return(
			&voyagerv1.HealthResponse{Status: voyagerv1.HealthResponse_HEALTHY},
			nil,
		)

		cli.startHealthChecks()
		time.Sleep(250 * time.Millisecond)
		cli.stopHealthChecks()

		mockClient.AssertExpectations(t)
	})
}

// TestClient_Deregister tests service deregistration functionality
func TestClient_Deregister(t *testing.T) {
	t.Run("Successful deregistration", func(t *testing.T) {
		mockClient := new(MockDiscoveryClient)
		cli := &Client{
			discoverySvc: mockClient,
			serviceName:  "test-service",
			instanceID:   "test-instance",
			cache:        cache.New(30*time.Second, 10*time.Minute),
		}

		deregReq := &voyagerv1.InstanceID{
			ServiceName: "test-service",
			InstanceId:  "test-instance",
		}

		mockClient.On("Deregister", mock.Anything, deregReq).Return(&voyagerv1.Response{Success: true}, nil)

		err := cli.Deregister()
		assert.NoError(t, err)
		mockClient.AssertExpectations(t)
	})

	t.Run("Deregistration error", func(t *testing.T) {
		mockClient := new(MockDiscoveryClient)
		cli := &Client{
			discoverySvc: mockClient,
			serviceName:  "test-service",
			instanceID:   "test-instance",
			cache:        cache.New(30*time.Second, 10*time.Minute),
		}

		deregReq := &voyagerv1.InstanceID{
			ServiceName: "test-service",
			InstanceId:  "test-instance",
		}

		var nilResponse *voyagerv1.Response
		mockClient.On("Deregister", mock.Anything, deregReq).Return(
			nilResponse,
			errors.New("deregistration failed"),
		)

		err := cli.Deregister()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "deregistration failed")
	})
}

// TestClient_Close tests resource cleanup functionality
func TestClient_Close(t *testing.T) {
	t.Run("Close with active health checks", func(t *testing.T) {
		mockClient := new(MockDiscoveryClient)
		cli := &Client{
			discoverySvc:   mockClient,
			serviceName:    "test-service",
			instanceID:     "test-instance",
			connectionPool: NewConnectionPool(&Options{ConnectionTimeout: 100 * time.Millisecond}),
			cache:          cache.New(30*time.Second, 10*time.Minute),
			options:        &Options{HealthCheckInterval: 100 * time.Millisecond},
		}

		cli.startHealthChecks()
		time.Sleep(10 * time.Millisecond)

		err := cli.Close()
		assert.NoError(t, err)
	})
}

// TestClient_ConnectionPool tests connection pooling functionality
func TestClient_ConnectionPool(t *testing.T) {
	t.Run("Connection reuse and reference counting", func(t *testing.T) {
		const bufSize = 1024 * 1024
		lis := bufconn.Listen(bufSize)
		defer func() {
			if err := lis.Close(); err != nil {
				t.Logf("failed to close listener: %v", err)
			}
		}()

		srv := grpc.NewServer()
		go func() {
			if err := srv.Serve(lis); err != nil {
				log.Printf("Test server error: %v", err)
			}
		}()
		defer srv.Stop()

		pool := NewConnectionPool(&Options{
			ConnectionTimeout: 100 * time.Millisecond,
			Insecure:          true,
			DialFunc: func(ctx context.Context, address string) (net.Conn, error) {
				return lis.Dial()
			},
		})

		address := "bufnet"

		// First connection
		conn1, err := pool.Get(context.Background(), address)
		assert.NoError(t, err)
		assert.NotNil(t, conn1)

		// Second connection to same address
		conn2, err := pool.Get(context.Background(), address)
		assert.NoError(t, err)
		assert.NotNil(t, conn2)

		// Verify two active connections
		count := pool.ConnectionCount(address)
		assert.Equal(t, int64(2), count)

		// Release connections
		pool.Release(address)
		pool.Release(address)

		// Allow time for reference count updates
		time.Sleep(10 * time.Millisecond)
		count = pool.ConnectionCount(address)
		assert.Equal(t, int64(0), count)
	})
}

// TestClient_LoadBalancing tests load balancing strategies
func TestClient_LoadBalancing(t *testing.T) {
	t.Run("Round-robin strategy", func(t *testing.T) {
		mockClient := new(MockDiscoveryClient)
		mockPool := new(MockConnectionPool)

		cli := &Client{
			discoverySvc: mockClient,
			options: &Options{
				TTL: 30 * time.Second,
			},
			connectionPool: mockPool,
			balancer:       newRoundRobinBalancer(),
			cache:          cache.New(30*time.Second, 10*time.Minute),
		}

		instances := []*voyagerv1.Registration{
			{InstanceId: "instance-1", Address: "host1", Port: 8080},
			{InstanceId: "instance-2", Address: "host2", Port: 8080},
		}

		mockClient.On("Discover", mock.Anything, mock.Anything).Return(
			&voyagerv1.ServiceList{Instances: instances},
			nil,
		).Times(3)

		conn1, _ := grpc.NewClient(
			"passthrough:///localhost:0",
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		)
		defer func() {
			if err := conn1.Close(); err != nil {
				t.Logf("failed to close conn1: %v", err)
			}
		}()

		conn2, _ := grpc.NewClient(
			"passthrough:///localhost:0",
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		)
		defer func() {
			if err := conn2.Close(); err != nil {
				t.Logf("failed to close conn2: %v", err)
			}
		}()

		// Expect round-robin order: host1 → host2 → host1
		mockPool.On("Get", mock.Anything, "host1:8080").Return(conn1, nil).Once()
		mockPool.On("Get", mock.Anything, "host2:8080").Return(conn2, nil).Once()
		mockPool.On("Get", mock.Anything, "host1:8080").Return(conn1, nil).Once()

		// Clear cache before each discovery
		cli.cache.Delete("test-service")
		result1, err := cli.Discover(context.Background(), "test-service")
		assert.NoError(t, err)
		assert.Equal(t, conn1, result1)

		cli.cache.Delete("test-service")
		result2, err := cli.Discover(context.Background(), "test-service")
		assert.NoError(t, err)
		assert.Equal(t, conn2, result2)

		cli.cache.Delete("test-service")
		result3, err := cli.Discover(context.Background(), "test-service")
		assert.NoError(t, err)
		assert.Equal(t, conn1, result3)

		mockClient.AssertExpectations(t)
		mockPool.AssertExpectations(t)
	})
}

// TestClient_Reregister tests service re-registration after health check failures
func TestClient_Reregister(t *testing.T) {
	t.Run("Re-register after health check failure", func(t *testing.T) {
		mockClient := new(MockDiscoveryClient)
		cli := &Client{
			discoverySvc: mockClient,
			options: &Options{
				HealthCheckInterval: 100 * time.Millisecond,
			},
			serviceName: "test-service",
			instanceID:  "test-instance",
			address:     "localhost",
			port:        8080,
			cache:       cache.New(30*time.Second, 10*time.Minute),
		}

		// First health check fails
		mockClient.On("HealthCheck", mock.Anything, mock.Anything).Return(
			(*voyagerv1.HealthResponse)(nil),
			status.Error(codes.Unavailable, "server unavailable"),
		).Once()

		// Re-registration succeeds
		mockClient.On("Register", mock.Anything, mock.MatchedBy(func(req *voyagerv1.Registration) bool {
			return req.ServiceName == "test-service" &&
				req.InstanceId == "test-instance" &&
				req.Address == "localhost" &&
				req.Port == 8080
		})).Return(&voyagerv1.Response{Success: true}, nil)

		// Subsequent health checks succeed
		mockClient.On("HealthCheck", mock.Anything, mock.Anything).Return(
			&voyagerv1.HealthResponse{Status: voyagerv1.HealthResponse_HEALTHY},
			nil,
		)

		cli.startHealthChecks()
		time.Sleep(250 * time.Millisecond)
		cli.stopHealthChecks()

		mockClient.AssertExpectations(t)
	})
}
