package main

import (
	"context"
	"log"
	"math/rand"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/kolkov/voyager/client"
	paymentv1 "github.com/kolkov/voyager/proto/payment/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
)

func main() {
	// Get discovery server address
	voyagerAddr := os.Getenv("VOYAGER_ADDR")
	if voyagerAddr == "" {
		voyagerAddr = "localhost:50050"
		log.Printf("Using default VOYAGER_ADDR: %s", voyagerAddr)
	} else {
		log.Printf("Using VOYAGER_ADDR from env: %s", voyagerAddr)
	}

	// Create Voyager client
	voyager, err := client.New(voyagerAddr,
		client.WithInsecure(),
		client.WithRetryPolicy(5, 2*time.Second),
	)
	if err != nil {
		log.Fatalf("Failed to create Voyager client: %v", err)
	}
	defer voyager.Close()

	// Get dynamic port
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port

	// Register service
	err = voyager.Register("payment-service", getLocalIP(), port, map[string]string{
		"environment": "production",
		"version":     "1.0.0",
	})
	if err != nil {
		log.Fatalf("Failed to register service: %v", err)
	}

	// Create gRPC server
	server := grpc.NewServer()
	paymentServer := &paymentServer{}
	paymentv1.RegisterPaymentServiceServer(server, paymentServer)

	reflection.Register(server)

	// Graceful shutdown
	go handleShutdown(voyager, server)

	log.Printf("Payment service started on port %d", port)
	if err := server.Serve(listener); err != nil {
		log.Fatalf("gRPC server failed: %v", err)
	}
}

// paymentServer implements payment service
type paymentServer struct {
	paymentv1.UnimplementedPaymentServiceServer
}

// ProcessPayment handles payment processing
func (s *paymentServer) ProcessPayment(ctx context.Context, req *paymentv1.ProcessPaymentRequest) (*paymentv1.ProcessPaymentResponse, error) {
	log.Printf("Processing payment for order: %s, amount: %.2f, currency: %s",
		req.OrderId, req.Amount, req.Currency)

	// Validate amount
	if req.Amount <= 0 {
		return nil, status.Error(codes.InvalidArgument, "invalid amount")
	}

	// Simulate processing
	time.Sleep(500 * time.Millisecond)

	// Simulate random failure (10% chance)
	if rand.Intn(10) == 0 {
		errorMsg := "Insufficient funds"
		log.Printf("Payment failed: %s", errorMsg)
		return &paymentv1.ProcessPaymentResponse{
			Success:      false,
			ErrorMessage: errorMsg,
		}, nil
	}

	transactionID := "tx_" + req.OrderId
	log.Printf("Payment successful, transaction ID: %s", transactionID)
	return &paymentv1.ProcessPaymentResponse{
		Success:       true,
		TransactionId: transactionID,
	}, nil
}

// handleShutdown gracefully shuts down the server
func handleShutdown(voyager *client.Client, server *grpc.Server) {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	log.Println("Shutting down gracefully...")

	// Deregister service
	if err := voyager.Deregister(); err != nil {
		log.Printf("Deregistration error: %v", err)
	}

	// Stop gRPC server
	server.GracefulStop()
	log.Println("Server stopped")
	os.Exit(0)
}

// getLocalIP gets the local IP address
func getLocalIP() string {
	if ip := os.Getenv("SERVICE_IP"); ip != "" {
		return ip
	}
	return "localhost"
}

func init() {
	rand.Seed(time.Now().UnixNano())
}
