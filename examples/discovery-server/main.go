package main

import (
	"context"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/kolkov/voyager/server"
	clientv3 "go.etcd.io/etcd/client/v3"
)

func main() {
	var etcdEndpoints []string
	if envEndpoints := os.Getenv("ETCD_ENDPOINTS"); envEndpoints != "" {
		etcdEndpoints = []string{envEndpoints}
	} else {
		etcdEndpoints = []string{"http://localhost:2379"}
	}

	// Check ETCD availability
	etcdAvailable := isEtcdAvailable(etcdEndpoints)
	if etcdAvailable {
		log.Printf("ETCD available at: %v", etcdEndpoints)
	} else {
		log.Printf("WARNING: ETCD unavailable at: %v. Using in-memory mode", etcdEndpoints)
		etcdEndpoints = nil
	}

	cfg := server.Config{
		ETCDEndpoints: etcdEndpoints,
		CacheTTL:      30 * time.Second,
		AuthToken:     os.Getenv("AUTH_TOKEN"),
	}

	srv, err := server.NewServer(cfg)
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}

	// Create gRPC server with auth interceptor
	grpcSrv := srv.GRPCServer()

	lis, err := net.Listen("tcp", ":50050")
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	log.Printf("Discovery server running on %s", lis.Addr().String())
	if len(etcdEndpoints) > 0 {
		log.Printf("Using ETCD: %v", etcdEndpoints)
	} else {
		log.Println("Mode: in-memory")
	}

	// Start metrics updater
	go srv.UpdateMetricsTicker(10 * time.Second)

	// Start HTTP server for metrics
	go func() {
		http.Handle("/metrics", server.MetricsHandler())
		log.Println("Metrics server running on :2112")
		if err := http.ListenAndServe(":2112", nil); err != nil {
			log.Printf("Metrics server error: %v", err)
		}
	}()

	// Start service logger
	go func() {
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			srv.LogCurrentServices()
		}
	}()

	if err := grpcSrv.Serve(lis); err != nil {
		log.Fatalf("gRPC server failed: %v", err)
	}
}

func isEtcdAvailable(endpoints []string) bool {
	if len(endpoints) == 0 {
		return false
	}

	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: 2 * time.Second,
	})
	if err != nil {
		return false
	}
	defer cli.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err = cli.Status(ctx, endpoints[0])
	return err == nil
}
