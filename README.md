# VoyagerSD - Service Discovery for Go Microservices

[![Go Reference](https://pkg.go.dev/badge/github.com/kolkov/voyager.svg)](https://pkg.go.dev/github.com/kolkov/voyager)
[![CI Status](https://github.com/kolkov/voyager/actions/workflows/test.yml/badge.svg)](https://github.com/kolkov/voyager/actions)
[![Coverage Status](https://coveralls.io/repos/github/kolkov/voyager/badge.svg)](https://coveralls.io/github/kolkov/voyager)
[![GitHub release](https://img.shields.io/github/release/kolkov/voyager.svg)](https://github.com/kolkov/voyager/releases)
[![Beta Release](https://img.shields.io/badge/release-v1.0.0--beta.3-blue)](https://github.com/kolkov/voyager/releases/tag/v1.0.0-beta.3)
[![Go Report Card](https://goreportcard.com/badge/github.com/kolkov/voyager)](https://goreportcard.com/report/github.com/kolkov/voyager)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](https://github.com/kolkov/voyager/blob/main/LICENSE)
[![Multi-Platform](https://img.shields.io/badge/platform-windows%20|%20linux%20|%20macos-lightgrey)](https://github.com/kolkov/voyager)
[![Multi-Arch](https://img.shields.io/badge/arch-amd64%20|%20arm64%20|%20armv7-blue)](https://github.com/kolkov/voyager)

> **Latest Beta: VoyagerSD 1.0.0-beta.3** - Our most advanced pre-release version with full CI/CD automation, multi-arch support, and enhanced security. We're actively refining before the stable release.

VoyagerSD is a production-ready service discovery solution for Go microservices with:

- Dynamic service registration and health checking
- Intelligent client-side load balancing
- ETCD backend with in-memory development mode
- Connection pooling and automatic failover
- Comprehensive metrics and tracing
- Kubernetes-native design
- Multi-platform/arch support (Windows, Linux, macOS, amd64, arm64)
- Automated release management with GoReleaser

## 🚀 Key Features

- **Automatic Service Registration**: Services self-register on startup with metadata
- **Health Monitoring**: Active health checks with TTL support
- **Smart Caching**: Local cache with automatic refresh for fast lookups
- **Connection Management**: Efficient gRPC connection reuse with pooling
- **Multiple Strategies**: RoundRobin, Random, LeastConnections load balancing
- **ETCD Integration**: Persistent, distributed storage for service data
- **Kubernetes Optimized**: Designed for container orchestration environments
- **Security**: TLS encryption and token-based authentication
- **Observability**: Prometheus metrics and OpenTelemetry tracing
- **Production CLI**: Easy deployment with `voyagerd` command
- **Release Automation**: Full CI/CD pipeline with GitHub Actions

## 📦 Installation (Latest Beta)

```bash
# Install beta version of discovery server
go install github.com/kolkov/voyager/cmd/voyagerd@beta

# Or via Docker (multi-arch image)
docker pull ghcr.io/kolkov/voyagerd:beta
```

## ⚡ Quick Start

### 1. Start Discovery Server

```bash
voyagerd \
  --etcd-endpoints=http://localhost:2379 \
  --auth-token=secure-token \
  --log-format=json \
  --metrics-addr=:2112
```

### 2. Register a Service

```go
package main

import (
	"context"
	"log"
	"net"

	"github.com/kolkov/voyager/client"
	"google.golang.org/grpc"
)

func main() {
	// Create Voyager client
	voyager, err := client.New("localhost:50050",
		client.WithAuthToken("secure-token"),
		client.WithBalancerStrategy(client.LeastConnections))
	if err != nil {
		log.Fatal(err)
	}
	defer voyager.Close()

	// Start gRPC server on dynamic port
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		log.Fatal(err)
	}
	
	// Register service with metadata
	err = voyager.Register("order-service", "localhost", port, map[string]string{
		"environment": "production",
		"version":     "1.2.0",
		"region":      "us-west",
	})
	if err != nil {
		log.Fatal(err)
	}

	// Start your gRPC server
	server := grpc.NewServer()
	// ... register your service handlers
	log.Printf("Service started on port %d", port)
	server.Serve(listener)
}
```

### 3. Discover and Connect to Services

```go
func callPaymentService(ctx context.Context, voyager *client.Client) error {
    conn, err := voyager.Discover(ctx, "payment-service",
        client.WithTimeout(3*time.Second),
        client.WithRetryPolicy(3, 1*time.Second))
    if err != nil {
        return err
    }
    defer conn.Close()

    client := paymentv1.NewPaymentServiceClient(conn)
    resp, err := client.ProcessPayment(ctx, &paymentv1.PaymentRequest{
        Amount: 99.99,
        Currency: "USD",
    })
    // Handle response
}
```

## 🐳 Deployment (Production-Ready)

### Docker Compose

```yaml
version: '3.8'
services:
  voyagerd:
    image: ghcr.io/kolkov/voyagerd:beta
    ports:
      - "50050:50050"
      - "2112:2112"
    environment:
      VOYAGER_ETCD_ENDPOINTS: "http://etcd:2379"
      VOYAGER_AUTH_TOKEN: "your-secure-token"
      VOYAGER_LOG_FORMAT: "json"
    depends_on:
      - etcd

  etcd:
    image: quay.io/coreos/etcd:v3.5.0
    command: etcd -advertise-client-urls http://etcd:2379 -listen-client-urls http://0.0.0.0:2379
    ports:
      - "2379:2379"
```

### Kubernetes (HA Deployment)

```yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: voyager-discovery
spec:
  replicas: 3
  serviceName: voyager-discovery
  selector:
    matchLabels:
      app: voyager-discovery
  template:
    metadata:
      labels:
        app: voyager-discovery
    spec:
      containers:
        - name: discovery
          image: ghcr.io/kolkov/voyagerd:beta
          ports:
            - containerPort: 50050
              name: grpc
            - containerPort: 2112
              name: metrics
          env:
            - name: VOYAGER_ETCD_ENDPOINTS
              value: "http://etcd-cluster:2379"
            - name: VOYAGER_AUTH_TOKEN
              valueFrom:
                secretKeyRef:
                  name: voyager-secrets
                  key: auth-token
          readinessProbe:
            httpGet:
              path: /ready
              port: metrics
            initialDelaySeconds: 5
            periodSeconds: 10
          livenessProbe:
            httpGet:
              path: /health
              port: metrics
            initialDelaySeconds: 15
            periodSeconds: 20
---
apiVersion: v1
kind: Service
metadata:
  name: voyager-discovery
spec:
  selector:
    app: voyager-discovery
  ports:
    - name: grpc
      port: 50050
      targetPort: grpc
    - name: metrics
      port: 2112
      targetPort: metrics
  type: LoadBalancer
```

## 🔧 Development Workflow

### Getting Started
```bash
# Set up development environment
./dev-setup.sh

# Build all binaries
make build

# Run tests
make test

# Start local cluster
docker-compose up -d
```

### Release Management
```bash
# Prepare release branch
make release-prepare VERSION=v1.0.0-beta.4

# Run release validation
make release-test

# Publish release (CI automated)
make release-publish VERSION=v1.0.0-beta.4
```

## 📊 Monitoring & Metrics

VoyagerSD exposes Prometheus metrics on `:2112/metrics`:

```bash
# Sample metrics output
voyager_registrations_total 248
voyager_discoveries_total{service="payment-service"} 142
voyager_service_instances{service="order-service"} 5
voyager_grpc_request_duration_seconds_bucket{method="Discover",le="0.1"} 128
```

### Key Metrics
| Metric | Type | Description |
|--------|------|-------------|
| `voyager_registrations_total` | Counter | Total service registrations |
| `voyager_discoveries_total` | Counter | Service discovery requests |
| `voyager_service_instances` | Gauge | Registered instances per service |
| `voyager_cache_size` | Gauge | Service cache entries |
| `voyager_grpc_request_duration_seconds` | Histogram | gRPC method latency |
| `voyager_etcd_operations_total` | Counter | ETCD backend operations |
| `voyager_connection_pool_size` | Gauge | Active connections in pool |

### Health Endpoints
- `GET /health` - Liveness probe (200 when running)
- `GET /ready` - Readiness probe (200 when serving requests)

## 🛠️ Makefile Reference

```bash
make install-tools      # Install dev tools (buf, golangci-lint, etc.)
make generate           # Generate protobuf code
make build              # Build all binaries
make test               # Run unit tests
make test-integration   # Run integration tests (non-Windows)
make lint               # Run linters and code checks
make docker             # Build Docker images
make run                # Start service locally
make release-test       # Validate release readiness
make release-guide      # Open release documentation
```

## 🔒 Security Best Practices

1. **Always use TLS** for production communications
2. **Rotate authentication tokens** quarterly
3. **Limit network exposure** with firewalls
4. **Run as non-root** in containers
5. **Enable audit logging** for sensitive operations

```go
// Secure client with TLS
voyager, err := client.New("discovery:50050",
    client.WithTLSConfig(&tls.Config{
        MinVersion: tls.VersionTLS12,
        CipherSuites: []uint16{
            tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
            tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
        },
    }),
    client.WithAuthToken("rotated-quarterly-token"))
```

## ❓ Getting Help

- [File a GitHub Issue](https://github.com/kolkov/voyager/issues)
- [Join Discord Community](https://discord.gg/voyager-sd)
- [Read Release Guide](RELEASE_GUIDE.md)
- [View Russian Documentation](RELEASE_GUIDE.ru.md)

## 🤝 Contributing to Beta

We welcome contributions during our beta phase! Please follow:

1. Fork repository and create feature branch
2. Implement changes with tests
3. Update documentation
4. Submit PR against `develop` branch

Before contributing:
- Read [CONTRIBUTING.md](CONTRIBUTING.md)
- Follow semantic commit messages
- Maintain 85%+ test coverage
- Verify with `make release-test`

## 📜 License

Apache 2.0 - See [LICENSE](LICENSE) for details

---

VoyagerSD Beta is developed and maintained by the Kolkov team with ❤️  
Help us improve by reporting issues and suggesting features!
