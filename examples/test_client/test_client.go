// Package main implements a Voyager client tester for Order Service
package main

import (
	"context"
	"flag"
	"log"
	"time"

	"github.com/kolkov/voyager/client"
	orderv1 "github.com/kolkov/voyager/gen/proto/order/v1"
	paymentv1 "github.com/kolkov/voyager/gen/proto/payment/v1"
)

func main() {
	target := flag.String("target", "localhost:50050", "Discovery service address")
	userID := flag.String("user", "test-user", "User ID for order")
	flag.Parse()

	log.Printf("Connecting to discovery service at: %s", *target)

	// Create Voyager client
	voyager, err := client.New(*target,
		client.WithInsecure(),
		client.WithConnectionTimeout(5*time.Second),
		client.WithRetryPolicy(3, 2*time.Second))
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer func() {
		if closeErr := voyager.Close(); closeErr != nil {
			log.Printf("failed to close voyager client: %v", closeErr)
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Phase 1: Create order through Order Service
	log.Printf("Discovering Order Service")
	orderConn, err := voyager.Discover(ctx, "order-service")
	if err != nil {
		log.Fatalf("❌ Order Service discovery failed: %v", err)
	}
	defer func() {
		if connErr := orderConn.Close(); connErr != nil {
			log.Printf("failed to close order connection: %v", err)
		}
	}()
	log.Printf("✅ Connected to Order Service at: %s", orderConn.Target())

	// Create sample order request
	req := &orderv1.CreateOrderRequest{
		UserId: *userID,
		Items: []*orderv1.OrderItem{
			{
				ProductId: "prod-123",
				Quantity:  2,
				Price:     19.99,
			},
			{
				ProductId: "prod-456",
				Quantity:  1,
				Price:     49.99,
			},
		},
		TotalAmount: 89.97, // 19.99*2 + 49.99
	}

	// Call Order Service to create order
	orderClient := orderv1.NewOrderServiceClient(orderConn)
	log.Printf("📦 Creating order for user: %s", *userID)
	orderResp, err := orderClient.CreateOrder(ctx, req)
	if err != nil {
		log.Fatalf("❌ CreateOrder failed: %v", err)
	}

	log.Printf("✅ Order created successfully!")
	log.Printf("   Order ID: %s", orderResp.OrderId)
	log.Printf("   Status: %s", orderResp.Status)
	log.Printf("   Transaction ID: %s", orderResp.TransactionId)

	// Phase 2: Check payment status through Payment Service
	log.Printf("\nDiscovering Payment Service")
	paymentConn, err := voyager.Discover(ctx, "payment-service")
	if err != nil {
		log.Fatalf("❌ Payment Service discovery failed: %v", err)
	}
	defer func() {
		if closeErr := paymentConn.Close(); closeErr != nil {
			log.Printf("failed to close payment connection: %v", closeErr)
		}
	}()
	log.Printf("✅ Connected to Payment Service at: %s", paymentConn.Target())

	// Check payment status
	paymentClient := paymentv1.NewPaymentServiceClient(paymentConn)
	paymentReq := &paymentv1.PaymentStatusRequest{
		TransactionId: orderResp.TransactionId,
	}

	log.Printf("💳 Checking payment status for transaction: %s", orderResp.TransactionId)
	paymentStatus, err := paymentClient.GetPaymentStatus(ctx, paymentReq)
	if err != nil {
		log.Fatalf("❌ Payment status check failed: %v", err)
	}

	log.Printf("✅ Payment status:")
	log.Printf("   Success: %t", paymentStatus.Success)
	log.Printf("   Amount: %.2f %s", paymentStatus.Amount, paymentStatus.Currency)
	log.Printf("   Timestamp: %s", paymentStatus.Timestamp.AsTime().Format(time.RFC3339))
}
