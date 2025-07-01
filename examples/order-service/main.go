package main

import (
	"context"
	"log"
	"net"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/kolkov/voyager/client"
	orderv1 "github.com/kolkov/voyager/proto/order/v1"
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
	err = voyager.Register("order-service", getLocalIP(), port, map[string]string{
		"environment": "production",
		"version":     "1.0.0",
	})
	if err != nil {
		log.Fatalf("Failed to register service: %v", err)
	}

	// Create gRPC server
	server := grpc.NewServer()
	orderServer := &orderServer{
		voyager: voyager,
	}
	orderv1.RegisterOrderServiceServer(server, orderServer)

	reflection.Register(server)

	// Graceful shutdown
	go handleShutdown(voyager, server)

	log.Printf("Order service started on port %d", port)
	if err := server.Serve(listener); err != nil {
		log.Fatalf("gRPC server failed: %v", err)
	}
}

// orderServer implements order service
type orderServer struct {
	orderv1.UnimplementedOrderServiceServer
	voyager *client.Client
}

// CreateOrder handles order creation
func (s *orderServer) CreateOrder(ctx context.Context, req *orderv1.CreateOrderRequest) (*orderv1.CreateOrderResponse, error) {
	log.Printf("Creating order for user: %s, items: %d, total: %.2f",
		req.UserId, len(req.Items), req.TotalAmount)

	// Generate order ID
	orderID := "ord_" + req.UserId + "-" + strconv.FormatInt(time.Now().UnixNano(), 10)

	// Discover payment service
	paymentConn, err := s.voyager.Discover(ctx, "payment-service")
	if err != nil {
		log.Printf("Failed to discover payment service: %v", err)
		return nil, status.Errorf(codes.Unavailable, "payment service unavailable")
	}
	defer paymentConn.Close()

	// Create payment client
	paymentClient := paymentv1.NewPaymentServiceClient(paymentConn)

	// Process payment
	paymentReq := &paymentv1.ProcessPaymentRequest{
		OrderId:  orderID,
		Amount:   req.TotalAmount,
		Currency: "USD",
	}

	const maxRetries = 3
	var paymentResp *paymentv1.ProcessPaymentResponse

	for attempt := 1; attempt <= maxRetries; attempt++ {
		paymentCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		paymentResp, err = paymentClient.ProcessPayment(paymentCtx, paymentReq)
		cancel()

		if err == nil && paymentResp.Success {
			break
		}

		if err != nil {
			log.Printf("Payment attempt %d/%d failed: %v", attempt, maxRetries, err)
		} else {
			log.Printf("Payment attempt %d/%d failed: %s", attempt, maxRetries, paymentResp.ErrorMessage)
		}

		if attempt < maxRetries {
			backoff := time.Duration(attempt*attempt) * 500 * time.Millisecond
			time.Sleep(backoff)
		}
	}

	if err != nil {
		log.Printf("All payment attempts failed: %v", err)
		return nil, status.Errorf(codes.Internal, "payment processing failed: %v", err)
	}

	if !paymentResp.Success {
		log.Printf("Payment failed: %s", paymentResp.ErrorMessage)
		return nil, status.Errorf(codes.FailedPrecondition, "payment failed: %s", paymentResp.ErrorMessage)
	}

	log.Printf("Payment successful for order %s, transaction ID: %s", orderID, paymentResp.TransactionId)

	return &orderv1.CreateOrderResponse{
		OrderId:       orderID,
		Status:        "completed",
		TransactionId: paymentResp.TransactionId,
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
