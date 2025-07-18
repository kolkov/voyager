// Package main implements a sample payment service
package main

import (
	"context"
	"fmt"
	"google.golang.org/protobuf/types/known/timestamppb"
	"log"
	"math/rand"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/kolkov/voyager/client"
	paymentv1 "github.com/kolkov/voyager/gen/proto/payment/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

type paymentServer struct {
	paymentv1.UnimplementedPaymentServiceServer
	voyager *client.Client
}

func (s *paymentServer) ProcessPayment(_ context.Context, req *paymentv1.ProcessPaymentRequest) (*paymentv1.ProcessPaymentResponse, error) {
	// Simulate payment processing
	amount := rand.Intn(100) + 1
	log.Printf("Processing payment of $%d for order: %s", amount, req.OrderId)

	return &paymentv1.ProcessPaymentResponse{
		Success:       true,
		TransactionId: fmt.Sprintf("PAY-%d", time.Now().UnixNano()),
	}, nil
}

func main() {
	discoveryAddr := "localhost:50050"
	if addr := os.Getenv("DISCOVERY_ADDR"); addr != "" {
		discoveryAddr = addr
	}

	// Create Voyager client
	voyager, err := client.New(discoveryAddr, client.WithInsecure())
	if err != nil {
		log.Fatalf("Failed to create Voyager client: %v", err)
	}
	defer func() {
		if voyErr := voyager.Close(); voyErr != nil {
			log.Printf("failed to close voyager client: %v", voyErr)
		}
	}()

	// Get dynamic port
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port

	// Register service
	metadata := map[string]string{
		"environment": "production",
		"version":     "1.0.0",
	}
	err = voyager.Register("payment-service", "localhost", port, metadata)
	if err != nil {
		log.Fatalf("Registration failed: %v", err)
	}
	log.Printf("Service registered at localhost:%d", port)

	// Create gRPC server
	server := grpc.NewServer()
	reflection.Register(server)
	paymentv1.RegisterPaymentServiceServer(server, &paymentServer{voyager: voyager})

	// Start server
	go func() {
		log.Printf("Payment service starting on port %d", port)
		if err := server.Serve(listener); err != nil {
			log.Fatalf("Failed to serve: %v", err)
		}
	}()

	// Handle shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	log.Println("Shutting down payment service...")
	server.GracefulStop()
	log.Println("Payment service stopped")
}

func (s *paymentServer) GetPaymentStatus(ctx context.Context, req *paymentv1.PaymentStatusRequest) (*paymentv1.PaymentStatusResponse, error) {
	log.Printf("Fetching payment status for: %s", req.TransactionId)

	// В реальной системе здесь был бы запрос в БД
	return &paymentv1.PaymentStatusResponse{
		Success:       true,
		Amount:        89.97,
		Currency:      "USD",
		TransactionId: req.TransactionId,
		Status:        "completed",
		Timestamp:     timestamppb.Now(),
	}, nil
}
