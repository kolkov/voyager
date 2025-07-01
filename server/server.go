package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	voyagerv1 "github.com/kolkov/voyager/proto/voyager/v1"
)

// Config defines server configuration options
type Config struct {
	ETCDEndpoints []string
	CacheTTL      time.Duration
	AuthToken     string // Optional authentication token
}

// instanceInfo tracks registration and last seen time for in-memory mode
type instanceInfo struct {
	registration *voyagerv1.Registration
	lastSeen     time.Time
}

// Server implements voyagerv1.DiscoveryServer
type Server struct {
	voyagerv1.UnimplementedDiscoveryServer
	etcdClient        *clientv3.Client
	services          map[string]map[string]*voyagerv1.Registration
	inMemoryInstances map[string]map[string]*instanceInfo
	mu                sync.RWMutex
	cacheTTL          time.Duration
	inMemory          bool
	janitorOnce       sync.Once
	authToken         string
	ctx               context.Context    // Context for lifecycle management
	cancel            context.CancelFunc // Cancel function to stop background tasks
}

// NewServer creates a new VoyagerSD server instance
func NewServer(cfg Config) (*Server, error) {
	// Create context for lifecycle management
	ctx, cancel := context.WithCancel(context.Background())

	srv := &Server{
		services:          make(map[string]map[string]*voyagerv1.Registration),
		inMemoryInstances: make(map[string]map[string]*instanceInfo),
		cacheTTL:          cfg.CacheTTL,
		inMemory:          len(cfg.ETCDEndpoints) == 0,
		authToken:         cfg.AuthToken,
		ctx:               ctx,
		cancel:            cancel,
	}

	if !srv.inMemory {
		cli, err := clientv3.New(clientv3.Config{
			Endpoints:   cfg.ETCDEndpoints,
			DialTimeout: 2 * time.Second, // Shorter timeout
		})
		if err != nil {
			log.Printf("WARNING: Failed to connect to ETCD: %v. Switching to in-memory mode", err)
			srv.inMemory = true
		} else {
			srv.etcdClient = cli
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			if err := srv.loadInitialData(ctx); err != nil {
				log.Printf("Warning: failed to load initial data: %v", err)
				// Explicit fallback if initial load fails
				srv.inMemory = true
				cli.Close()
			} else {
				go srv.startCacheRefresher()
			}
		}
	}

	if srv.inMemory {
		log.Println("WARNING: Running in in-memory mode without persistence")
		srv.startJanitor()
	}

	return srv, nil
}

// Close releases server resources and stops background tasks
func (s *Server) Close() {
	// Cancel context to stop all background goroutines
	s.cancel()

	if s.etcdClient != nil {
		s.etcdClient.Close()
	}
}

// GRPCServer returns a pre-configured gRPC server
func (s *Server) GRPCServer(opts ...grpc.ServerOption) *grpc.Server {
	serverOpts := []grpc.ServerOption{}
	if s.authToken != "" {
		serverOpts = append(serverOpts, grpc.UnaryInterceptor(s.AuthInterceptor))
	}
	serverOpts = append(serverOpts, opts...)

	srv := grpc.NewServer(serverOpts...)
	voyagerv1.RegisterDiscoveryServer(srv, s)
	return srv
}

// AuthInterceptor provides authentication for gRPC methods
func (s *Server) AuthInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	if s.authToken != "" {
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, status.Error(codes.Unauthenticated, "missing metadata")
		}

		tokens := md.Get("authorization")
		if len(tokens) == 0 || tokens[0] != s.authToken {
			return nil, status.Error(codes.PermissionDenied, "invalid auth token")
		}
	}
	return handler(ctx, req)
}

// Register handles service registration
func (s *Server) Register(ctx context.Context, req *voyagerv1.Registration) (*voyagerv1.Response, error) {
	log.Printf("Registering service: %s, instance: %s, address: %s:%d",
		req.ServiceName, req.InstanceId, req.Address, req.Port)

	IncRegistrationCounter(req.ServiceName)

	if req.ServiceName == "" || req.InstanceId == "" || req.Address == "" || req.Port == 0 {
		return nil, status.Error(codes.InvalidArgument, "invalid registration data")
	}

	// For in-memory mode
	if s.inMemory {
		s.mu.Lock()
		defer s.mu.Unlock()

		if _, exists := s.inMemoryInstances[req.ServiceName]; !exists {
			s.inMemoryInstances[req.ServiceName] = make(map[string]*instanceInfo)
		}

		s.inMemoryInstances[req.ServiceName][req.InstanceId] = &instanceInfo{
			registration: req,
			lastSeen:     time.Now(),
		}
		return &voyagerv1.Response{Success: true}, nil
	}

	// ETCD mode
	key := fmt.Sprintf("/services/%s/%s", req.ServiceName, req.InstanceId)
	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to marshal registration")
	}

	leaseResp, err := s.etcdClient.Grant(ctx, int64(s.cacheTTL.Seconds()))
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to create lease")
	}

	_, err = s.etcdClient.Put(ctx, key, string(jsonData), clientv3.WithLease(leaseResp.ID))
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to store registration")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.services[req.ServiceName]; !exists {
		s.services[req.ServiceName] = make(map[string]*voyagerv1.Registration)
	}

	s.services[req.ServiceName][req.InstanceId] = req

	return &voyagerv1.Response{Success: true}, nil
}

// Discover returns service instances
func (s *Server) Discover(ctx context.Context, req *voyagerv1.ServiceQuery) (*voyagerv1.ServiceList, error) {
	log.Printf("Discover request for service: %s", req.ServiceName)

	discoveryStatus := "success"
	defer func() {
		IncDiscoveryCounter(req.ServiceName, discoveryStatus)
	}()

	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.inMemory {
		list := &voyagerv1.ServiceList{}
		if instances, exists := s.inMemoryInstances[req.ServiceName]; exists {
			for _, info := range instances {
				list.Instances = append(list.Instances, info.registration)
			}
		} else {
			discoveryStatus = "not_found"
		}
		return list, nil
	}

	// ETCD implementation
	list := &voyagerv1.ServiceList{}
	if instances, exists := s.services[req.ServiceName]; exists {
		for _, inst := range instances {
			list.Instances = append(list.Instances, inst)
		}
	} else {
		discoveryStatus = "not_found"
	}
	return list, nil
}

// HealthCheck handles health status reporting
func (s *Server) HealthCheck(ctx context.Context, req *voyagerv1.HealthRequest) (*voyagerv1.HealthResponse, error) {
	log.Printf("Health check received for service %s instance %s",
		req.ServiceName, req.InstanceId)

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.inMemory {
		if service, exists := s.inMemoryInstances[req.ServiceName]; exists {
			if info, exists := service[req.InstanceId]; exists {
				info.lastSeen = time.Now()
				return &voyagerv1.HealthResponse{
					Status: voyagerv1.HealthResponse_HEALTHY,
				}, nil
			}
		}
		return &voyagerv1.HealthResponse{
			Status: voyagerv1.HealthResponse_UNHEALTHY,
		}, nil
	}

	// For ETCD, refresh TTL by re-storing the existing value
	if service, exists := s.services[req.ServiceName]; exists {
		if reg, exists := service[req.InstanceId]; exists {
			key := fmt.Sprintf("/services/%s/%s", req.ServiceName, req.InstanceId)

			// Marshal the existing registration instead of using empty string
			jsonData, err := json.Marshal(reg)
			if err != nil {
				log.Printf("Failed to marshal registration: %v", err)
				return &voyagerv1.HealthResponse{
					Status: voyagerv1.HealthResponse_UNHEALTHY,
				}, nil
			}

			leaseResp, err := s.etcdClient.Grant(ctx, int64(s.cacheTTL.Seconds()))
			if err != nil {
				log.Printf("Failed to create lease: %v", err)
				return &voyagerv1.HealthResponse{
					Status: voyagerv1.HealthResponse_UNHEALTHY,
				}, nil
			}

			_, err = s.etcdClient.Put(ctx, key, string(jsonData), clientv3.WithLease(leaseResp.ID))
			if err != nil {
				log.Printf("Failed to refresh TTL: %v", err)
			}

			return &voyagerv1.HealthResponse{
				Status: voyagerv1.HealthResponse_HEALTHY,
			}, nil
		}
	}

	return &voyagerv1.HealthResponse{
		Status: voyagerv1.HealthResponse_UNHEALTHY,
	}, nil
}

// Deregister removes a service instance
func (s *Server) Deregister(ctx context.Context, req *voyagerv1.InstanceID) (*voyagerv1.Response, error) {
	if s.inMemory {
		s.mu.Lock()
		defer s.mu.Unlock()

		if service, exists := s.inMemoryInstances[req.ServiceName]; exists {
			delete(service, req.InstanceId)
			if len(service) == 0 {
				delete(s.inMemoryInstances, req.ServiceName)
			}
		}

		return &voyagerv1.Response{Success: true}, nil
	}

	// ETCD mode
	key := fmt.Sprintf("/services/%s/%s", req.ServiceName, req.InstanceId)

	_, err := s.etcdClient.Delete(ctx, key)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to deregister")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if service, exists := s.services[req.ServiceName]; exists {
		delete(service, req.InstanceId)
		if len(service) == 0 {
			delete(s.services, req.ServiceName)
		}
	}

	return &voyagerv1.Response{Success: true}, nil
}

// LogCurrentServices logs current service state
func (s *Server) LogCurrentServices() {
	s.mu.RLock()
	defer s.mu.RUnlock()

	log.Println("=== Current registered services ===")
	if s.inMemory {
		for service, instances := range s.inMemoryInstances {
			log.Printf("  %s: %d instances", service, len(instances))
			for id, info := range instances {
				log.Printf("    - ID: %s, Address: %s:%d, LastSeen: %s",
					id, info.registration.Address, info.registration.Port,
					info.lastSeen.Format(time.RFC3339))
			}
		}
	} else {
		for service, instances := range s.services {
			log.Printf("  %s: %d instances", service, len(instances))
			for id, reg := range instances {
				log.Printf("    - ID: %s, Address: %s:%d",
					id, reg.Address, reg.Port)
			}
		}
	}
	log.Println("==================================")
}

// UpdateMetricsTicker periodically updates metrics
func (s *Server) UpdateMetricsTicker(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for range ticker.C {
		s.UpdateServiceMetrics()
	}
}

// startJanitor starts background cleanup of expired instances
func (s *Server) startJanitor() {
	s.janitorOnce.Do(func() {
		go func() {
			for {
				s.mu.RLock()
				ttl := s.cacheTTL
				s.mu.RUnlock()

				select {
				case <-time.After(ttl / 2):
					s.cleanupExpiredInstances()
				case <-s.ctx.Done():
					log.Println("Stopping janitor, server shutting down")
					return
				}
			}
		}()
	})
}

// cleanupExpiredInstances removes instances that haven't been seen within TTL
func (s *Server) cleanupExpiredInstances() {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	for serviceName, instances := range s.inMemoryInstances {
		for instanceID, info := range instances {
			if now.Sub(info.lastSeen) > s.cacheTTL {
				delete(instances, instanceID)
				log.Printf("Removed expired instance: %s/%s", serviceName, instanceID)
			}
		}
		if len(instances) == 0 {
			delete(s.inMemoryInstances, serviceName)
		}
	}
}

// loadInitialData loads existing registrations from ETCD
func (s *Server) loadInitialData(ctx context.Context) error {
	if s.inMemory {
		return nil
	}

	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	resp, err := s.etcdClient.Get(ctx, "/services/", clientv3.WithPrefix())
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for _, kv := range resp.Kvs {
		var reg voyagerv1.Registration
		if err := json.Unmarshal(kv.Value, &reg); err != nil {
			log.Printf("Failed to unmarshal registration: %v", err)
			continue
		}

		if _, exists := s.services[reg.ServiceName]; !exists {
			s.services[reg.ServiceName] = make(map[string]*voyagerv1.Registration)
		}

		s.services[reg.ServiceName][reg.InstanceId] = &reg
	}

	return nil
}
