package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/kolkov/voyager/server"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc"
)

const (
	version = "1.0.0-beta" // Version will be set during build
)

var (
	cfgFile string
	rootCmd = &cobra.Command{
		Use:     "voyagerd",
		Version: version,
		Short:   "Voyager Service Discovery Server",
		Long: `High-performance service discovery server for microservices architecture.
Supports ETCD backend for persistence and clustering.`,
		Run: runServer,
	}
)

func init() {
	cobra.OnInitialize(initConfig)

	// Command line flags
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default ./voyagerd.yaml)")
	rootCmd.PersistentFlags().StringSlice("etcd-endpoints", []string{"http://localhost:2379"}, "ETCD endpoints")
	rootCmd.PersistentFlags().Duration("cache-ttl", 30*time.Second, "Cache TTL duration")
	rootCmd.PersistentFlags().String("auth-token", "", "Authentication token")
	rootCmd.PersistentFlags().String("grpc-addr", ":50050", "gRPC server address")
	rootCmd.PersistentFlags().String("metrics-addr", ":2112", "Metrics HTTP server address")
	rootCmd.PersistentFlags().Duration("log-interval", 15*time.Second, "Service logging interval")
	rootCmd.PersistentFlags().Bool("debug", false, "Enable debug logging")
	rootCmd.PersistentFlags().String("log-format", "text", "Log format (text/json)")

	// Bind flags to Viper variables
	viper.BindPFlag("etcd_endpoints", rootCmd.PersistentFlags().Lookup("etcd-endpoints"))
	viper.BindPFlag("cache_ttl", rootCmd.PersistentFlags().Lookup("cache-ttl"))
	viper.BindPFlag("auth_token", rootCmd.PersistentFlags().Lookup("auth-token"))
	viper.BindPFlag("grpc_addr", rootCmd.PersistentFlags().Lookup("grpc-addr"))
	viper.BindPFlag("metrics_addr", rootCmd.PersistentFlags().Lookup("metrics-addr"))
	viper.BindPFlag("log_interval", rootCmd.PersistentFlags().Lookup("log-interval"))
	viper.BindPFlag("debug", rootCmd.PersistentFlags().Lookup("debug"))
	viper.BindPFlag("log_format", rootCmd.PersistentFlags().Lookup("log-format"))

	// Automatic environment variable reading
	viper.AutomaticEnv()
	viper.SetEnvPrefix("voyager")
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		// Search config in standard locations
		viper.AddConfigPath(".")
		viper.AddConfigPath("/etc/voyager/")
		viper.AddConfigPath("$HOME/.voyager")
		viper.SetConfigName("voyagerd")
	}

	// Read config if found
	if err := viper.ReadInConfig(); err == nil {
		log.Printf("Using config file: %s", viper.ConfigFileUsed())
	}
}

func runServer(cmd *cobra.Command, args []string) {
	// Configure logging
	setupLogger()

	log.Println("Starting Voyager Discovery Server")
	log.Printf("Version: %s", version)

	cfg := server.Config{
		ETCDEndpoints: viper.GetStringSlice("etcd_endpoints"),
		CacheTTL:      viper.GetDuration("cache_ttl"),
		AuthToken:     viper.GetString("auth_token"),
	}

	log.Printf("Configuration:")
	log.Printf("  ETCD Endpoints: %v", cfg.ETCDEndpoints)
	log.Printf("  Cache TTL: %v", cfg.CacheTTL)
	log.Printf("  Auth Token: %t", cfg.AuthToken != "") // Don't log actual token
	log.Printf("  gRPC Address: %s", viper.GetString("grpc_addr"))
	log.Printf("  Metrics Address: %s", viper.GetString("metrics_addr"))

	// Check ETCD availability
	etcdAvailable := isEtcdAvailable(cfg.ETCDEndpoints)
	if etcdAvailable {
		log.Printf("ETCD cluster available")
	} else if len(cfg.ETCDEndpoints) > 0 {
		log.Printf("WARNING: ETCD unavailable, switching to in-memory mode")
		cfg.ETCDEndpoints = nil
	}

	srv, err := server.NewServer(cfg)
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}
	defer srv.Close()

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start gRPC server
	grpcSrv := runGRPCServer(srv)

	// Start HTTP metrics server
	metricsSrv := runMetricsServer()

	// Start periodic service logging
	go runServiceLogger(srv)

	// Wait for shutdown signals
	waitForShutdown(ctx, grpcSrv, metricsSrv)
	log.Println("Server shutdown complete")
}

func setupLogger() {
	// In a real application, integrate with zap or logrus
	if viper.GetBool("debug") {
		log.Println("Debug logging enabled")
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

func runGRPCServer(srv *server.Server) *grpc.Server {
	grpcSrv := srv.GRPCServer()
	grpcLis, err := net.Listen("tcp", viper.GetString("grpc_addr"))
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	go func() {
		log.Printf("gRPC server running on %s", viper.GetString("grpc_addr"))
		if err := grpcSrv.Serve(grpcLis); err != nil && err != grpc.ErrServerStopped {
			log.Fatalf("gRPC server failed: %v", err)
		}
	}()

	return grpcSrv
}

func runMetricsServer() *http.Server {
	metricsMux := http.NewServeMux()
	metricsMux.Handle("/metrics", server.MetricsHandler())
	metricsMux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})
	metricsMux.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	metricsSrv := &http.Server{
		Addr:    viper.GetString("metrics_addr"),
		Handler: metricsMux,
	}

	go func() {
		log.Printf("Metrics server running on %s", viper.GetString("metrics_addr"))
		if err := metricsSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("Metrics server error: %v", err)
		}
	}()

	return metricsSrv
}

func runServiceLogger(srv *server.Server) {
	ticker := time.NewTicker(viper.GetDuration("log_interval"))
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			srv.LogCurrentServices()
		}
	}
}

func waitForShutdown(ctx context.Context, grpcSrv *grpc.Server, metricsSrv *http.Server) {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	<-quit
	log.Println("Shutting down server...")

	// Graceful shutdown gRPC
	go func() {
		grpcSrv.GracefulStop()
	}()

	// Graceful shutdown HTTP
	ctxTimeout, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := metricsSrv.Shutdown(ctxTimeout); err != nil {
		log.Printf("Metrics server shutdown error: %v", err)
	}
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
