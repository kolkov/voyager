package main

import (
	"context"
	"log"
	"os"
	"time"

	orderv1 "github.com/kolkov/voyager/proto/order/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	// Get server address from env or use default
	target := "localhost:55416"
	if envTarget := os.Getenv("SERVER_ADDRESS"); envTarget != "" {
		target = envTarget
	}

	log.Printf("Connecting to server at: %s", target)

	// Connect with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(ctx, target,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	client := orderv1.NewOrderServiceClient(conn)

	// Send request
	resp, err := client.CreateOrder(context.Background(), &orderv1.CreateOrderRequest{
		UserId: "test-user",
		Items: []*orderv1.OrderItem{
			{ProductId: "prod1", Quantity: 2, Price: 10.5},
		},
		TotalAmount: 21.0,
	})

	if err != nil {
		log.Fatalf("CreateOrder failed: %v", err)
	}

	log.Printf("Order created successfully: %+v", resp)
}
