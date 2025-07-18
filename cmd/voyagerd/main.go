// Package main implements Voyager discovery server
package main

import (
	"context"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/kolkov/voyager/server"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

var rootCmd = &cobra.Command{
	Use:     "voyagerd",
	Short:   "Voyager Service Discovery Server",
	Run:     runServer,
	Version: version,
}

func init() {
	flags := rootCmd.Flags()
	flags.StringSlice("etcd-endpoints", []string{"http://localhost:2379"}, "ETCD endpoints")
	flags.Duration("cache-ttl", 30*time.Second, "Cache TTL duration")
	flags.String("auth-token", "", "Authentication token")
	flags.String("grpc-addr", ":50050", "gRPC server address")
	flags.String("metrics-addr", ":2112", "Metrics HTTP address")
	flags.Duration("log-interval", 15*time.Second, "Service logging interval")
	flags.String("log-format", "text", "Log format (text/json)")
	flags.Bool("debug", false, "Enable debug logging")

	if err := viper.BindPFlags(flags); err != nil {
		log.Fatalf("failed to bind flags: %v", err)
	}
	viper.AutomaticEnv()
	viper.SetEnvPrefix("voyager")
}

func runServer(_ *cobra.Command, _ []string) {
	log.Printf("Starting Voyager Discovery Server %s (commit: %s, built: %s)",
		version, commit, date)

	cfg := server.Config{
		ETCDEndpoints: viper.GetStringSlice("etcd_endpoints"),
		CacheTTL:      viper.GetDuration("cache_ttl"),
		AuthToken:     viper.GetString("auth_token"),
	}

	srv, err := server.NewServer(cfg)
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}
	defer srv.Close()

	// Start gRPC server
	grpcSrv := srv.GRPCServer()
	grpcListener, err := net.Listen("tcp", viper.GetString("grpc_addr"))
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	go func() {
		log.Printf("gRPC server starting on %s", viper.GetString("grpc_addr"))
		if err := grpcSrv.Serve(grpcListener); err != nil {
			log.Fatalf("gRPC server failed: %v", err)
		}
	}()

	// Start metrics server
	metricsMux := http.NewServeMux()
	metricsMux.Handle("/metrics", promhttp.Handler())
	metricsMux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	metricsMux.HandleFunc("/ready", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	metricsSrv := &http.Server{
		Addr:    viper.GetString("metrics_addr"),
		Handler: metricsMux,
	}

	go func() {
		log.Printf("Metrics server starting on %s", viper.GetString("metrics_addr"))
		if err := metricsSrv.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatalf("Metrics server failed: %v", err)
		}
	}()

	// Start periodic service logging
	logTicker := time.NewTicker(viper.GetDuration("log_interval"))
	defer logTicker.Stop()

	go func() {
		for range logTicker.C {
			srv.LogCurrentServices()
		}
	}()

	// Start metrics updater
	go srv.UpdateMetricsTicker(30 * time.Second)

	// Handle graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	<-sigCh
	log.Println("Shutting down servers...")

	// Create shutdown context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Shutdown metrics server
	if err := metricsSrv.Shutdown(ctx); err != nil {
		log.Printf("Metrics server shutdown error: %v", err)
	}

	// Stop gRPC server gracefully
	stopped := make(chan struct{})
	go func() {
		grpcSrv.GracefulStop()
		close(stopped)
	}()

	// Wait for graceful stop or timeout
	select {
	case <-stopped:
		log.Println("gRPC server stopped gracefully")
	case <-ctx.Done():
		log.Println("gRPC server forced to stop")
		grpcSrv.Stop()
	}

	log.Println("Voyager discovery server stopped")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
