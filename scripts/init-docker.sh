#!/bin/bash
set -e

# Create docker network if not exists
docker network create voyager-net || true

# Start ETCD
docker run -d --name etcd --network voyager-net quay.io/coreos/etcd:v3.5.0 \
  etcd -advertise-client-urls http://etcd:2379 -listen-client-urls http://0.0.0.0:2379

# Build images
echo "Building Docker images..."
docker build -f cmd/voyagerd/Dockerfile -t voyagerd:latest .
docker build -f examples/order-service/Dockerfile -t voyager-example-order-service:latest .
docker build -f examples/payment-service/Dockerfile -t voyager-example-payment-service:latest .

# Start services
echo "Starting services..."
docker-compose -f examples/docker-compose.yaml up -d

echo "Voyager stack deployed successfully!"
echo "Discovery server: localhost:50050"
echo "Order service: localhost:8080"
echo "Payment service: localhost:8081"