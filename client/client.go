package client

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/patrickmn/go-cache"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"

	voyagerv1 "github.com/kolkov/voyager/proto/voyager/v1"
)

// Client manages service registration, discovery, and connection pooling
type Client struct {
	discoveryAddr     string
	discoverySvc      voyagerv1.DiscoveryClient
	conn              *grpc.ClientConn
	cache             *cache.Cache
	connectionPool    ConnectionPooler
	options           *Options
	instanceID        string
	serviceName       string
	healthMutex       sync.Mutex
	healthCheckCtx    context.Context
	healthCheckCancel context.CancelFunc
	mu                sync.RWMutex
	balancer          LoadBalancer
	address           string
	port              int
}

// New creates a new Voyager client with configured options
func New(discoveryAddr string, opts ...Option) (*Client, error) {
	options := defaultOptions()
	for _, opt := range opts {
		opt(options)
	}

	if discoveryAddr == "" {
		return nil, errors.New("discovery address cannot be empty")
	}

	log.Printf("Creating Voyager client for discovery service at: %s", discoveryAddr)

	conn, svc, err := connectWithRetry(discoveryAddr, options)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to discovery service: %w", err)
	}

	log.Printf("Successfully connected to discovery service")

	pool := NewConnectionPool(options)

	var balancer LoadBalancer
	switch options.BalancerStrategy {
	case Random:
		balancer = newRandomBalancer()
	case LeastConnections:
		balancer = newLeastConnectionsBalancer(pool)
	default:
		balancer = newRoundRobinBalancer()
	}

	return &Client{
		discoveryAddr:  discoveryAddr,
		discoverySvc:   svc,
		conn:           conn,
		cache:          cache.New(options.TTL, 10*time.Minute),
		connectionPool: pool,
		options:        options,
		balancer:       balancer,
	}, nil
}

// Register registers the service instance with the discovery service
func (c *Client) Register(serviceName, address string, port int, metadata map[string]string) error {
	if serviceName == "" || address == "" || port == 0 {
		return errors.New("invalid registration parameters")
	}

	c.serviceName = serviceName
	c.address = address
	c.port = port

	if c.instanceID == "" {
		hostname, _ := os.Hostname()
		c.instanceID = fmt.Sprintf("%s-%d", hostname, time.Now().UnixNano())
	}

	reg := &voyagerv1.Registration{
		ServiceName: serviceName,
		InstanceId:  c.instanceID,
		Address:     address,
		Port:        int32(port),
		Metadata:    metadata,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	resp, err := c.discoverySvc.Register(ctx, reg)
	if err != nil {
		return fmt.Errorf("registration failed: %w", err)
	}

	if !resp.Success {
		return errors.New("registration failed: " + resp.Error)
	}

	c.startHealthChecks()
	return nil
}

// Discover returns a connection to a service instance using load balancing
func (c *Client) Discover(ctx context.Context, serviceName string) (*grpc.ClientConn, error) {
	instances, err := c.getServiceInstances(ctx, serviceName)
	if err != nil {
		return nil, err
	}

	if len(instances) == 0 {
		return nil, fmt.Errorf("no instances available for service: %s", serviceName)
	}

	selected := c.balancer.Select(serviceName, instances)
	if selected == nil {
		return nil, errors.New("no instance selected")
	}

	address := net.JoinHostPort(selected.Address, strconv.Itoa(int(selected.Port)))
	return c.connectionPool.Get(ctx, address)
}

// Deregister removes the service instance from the discovery service
func (c *Client) Deregister() error {
	if c.serviceName == "" || c.instanceID == "" {
		return errors.New("service not registered")
	}

	c.stopHealthChecks()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	resp, err := c.discoverySvc.Deregister(ctx, &voyagerv1.InstanceID{
		ServiceName: c.serviceName,
		InstanceId:  c.instanceID,
	})
	if err != nil {
		return err
	}

	if !resp.Success {
		return errors.New("deregistration failed: " + resp.Error)
	}

	return nil
}

// Close cleans up resources and stops background processes
func (c *Client) Close() error {
	c.stopHealthChecks()

	if c.connectionPool != nil {
		c.connectionPool.Close()
	}

	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// getServiceInstances retrieves service instances from cache or discovery service
func (c *Client) getServiceInstances(ctx context.Context, serviceName string) ([]*voyagerv1.Registration, error) {
	if cached, found := c.cache.Get(serviceName); found {
		return cached.([]*voyagerv1.Registration), nil
	}

	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	resp, err := c.discoverySvc.Discover(ctx, &voyagerv1.ServiceQuery{
		ServiceName: serviceName,
		HealthyOnly: true,
	})
	if err != nil {
		return nil, err
	}

	c.cache.Set(serviceName, resp.Instances, c.options.TTL)
	return resp.Instances, nil
}

// connectWithRetry establishes connection with retry logic
func connectWithRetry(addr string, opts *Options) (*grpc.ClientConn, voyagerv1.DiscoveryClient, error) {
	for i := 0; i < opts.MaxRetries; i++ {
		creds, credErr := getTransportCredentials(opts)
		if credErr != nil {
			return nil, nil, credErr
		}

		dialOpts := []grpc.DialOption{
			grpc.WithTransportCredentials(creds),
			grpc.WithBlock(),
		}

		if opts.DialFunc != nil {
			dialOpts = append(dialOpts, grpc.WithContextDialer(opts.DialFunc))
		}

		ctx, cancel := context.WithTimeout(context.Background(), opts.ConnectionTimeout)
		conn, err := grpc.DialContext(ctx, addr, dialOpts...)
		cancel()

		if err == nil {
			return conn, voyagerv1.NewDiscoveryClient(conn), nil
		}

		log.Printf("Connection attempt %d/%d failed: %v", i+1, opts.MaxRetries, err)
		time.Sleep(opts.RetryDelay)
	}

	return nil, nil, fmt.Errorf("failed after %d attempts", opts.MaxRetries)
}

// getTransportCredentials returns appropriate transport credentials
func getTransportCredentials(opts *Options) (credentials.TransportCredentials, error) {
	if opts.Insecure {
		return insecure.NewCredentials(), nil
	}

	if opts.TLSConfig != nil {
		return credentials.NewTLS(opts.TLSConfig), nil
	}

	return credentials.NewClientTLSFromCert(nil, ""), nil
}
