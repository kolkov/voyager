# VoyagerSD - Service Discovery for Go Microservices

[![Go Reference](https://pkg.go.dev/badge/github.com/kolkov/voyager.svg)](https://pkg.go.dev/github.com/kolkov/voyager)
[![CI Status](https://github.com/kolkov/voyager/actions/workflows/test.yml/badge.svg)](https://github.com/kolkov/voyager/actions)
[![Coverage Status](https://coveralls.io/repos/github/kolkov/voyager/badge.svg)](https://coveralls.io/github/kolkov/voyager)
[![GitHub release](https://img.shields.io/github/release/kolkov/voyager.svg)](https://github.com/kolkov/voyager/releases)
[![Beta Release](https://img.shields.io/badge/release-beta-blue)](https://github.com/kolkov/voyager/releases/tag/v1.0.0-beta)
[![Go Report Card](https://goreportcard.com/badge/github.com/kolkov/voyager)](https://goreportcard.com/report/github.com/kolkov/voyager)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](https://github.com/kolkov/voyager/blob/main/LICENSE)
[![Multi-Platform](https://img.shields.io/badge/platform-windows%20|%20linux%20|%20macos-lightgrey)](https://github.com/kolkov/voyager)

> **Beta Notice**: VoyagerSD 1.0.0-beta is our current pre-release version. We're actively testing and refining before the stable release.

VoyagerSD is a service discovery solution for Go microservices with:

- Dynamic service registration
- Health checking
- Load balancing (RoundRobin, Random, LeastConnections)
- ETCD backend
- Connection pooling
- Metrics and tracing
- Kubernetes-ready design
- Multi-platform support (Windows, Linux, macOS)
- Automated release management

## 🚀 Features

- **Automatic Service Registration**: Services register on startup
- **Health Checks**: Periodic health verification
- **Intelligent Caching**: Local cache for fast lookups
- **Connection Pooling**: Reuse gRPC connections efficiently
- **Multiple Strategies**: RoundRobin, Random, LeastConnections
- **ETCD Integration**: Persistent storage for service data
- **Kubernetes Ready**: Designed for container environments
- **TLS Support**: Secure communication between services
- **Authentication**: Token-based authentication
- **Metrics**: Prometheus metrics integration
- **Production-Ready CLI**: Easy deployment with `voyagerd` command

## 📦 Installation (Beta Version)

```bash
# Install beta version of discovery server
go install github.com/kolkov/voyager/cmd/voyagerd@beta

# Or via Docker (beta image)
docker pull ghcr.io/kolkov/voyagerd:beta
```

## ⚡ Quick Start

### 1. Start Discovery Server

```bash
voyagerd --etcd-endpoints=http://localhost:2379 --auth-token=secure-token
```

### 2. Register a Service

```go
package main

import (
	"log"
	"net"

	"github.com/kolkov/voyager/client"
	"google.golang.org/grpc"
)

func main() {
	// Create Voyager client
	voyager, err := client.New("localhost:50050",
		client.WithAuthToken("secure-token"))
	if err != nil {
		log.Fatal(err)
	}
	defer voyager.Close()

	// Get dynamic port
	listener, err := net.Listen("tcp", ":0")
	port := listener.Addr().(*net.TCPAddr).Port

	// Register service
	err = voyager.Register("order-service", "localhost", port, map[string]string{
		"environment": "production",
		"version":     "1.0.0",
	})
	if err != nil {
		log.Fatal(err)
	}

	// Start your gRPC server
	server := grpc.NewServer()
	// ... register your service
	server.Serve(listener)
}
```

### 3. Discover and Connect to Services

```go
func callPaymentService(ctx context.Context, voyager *client.Client) error {
    conn, err := voyager.Discover(ctx, "payment-service")
    if err != nil {
        return err
    }
    defer conn.Close()

    client := paymentv1.NewPaymentServiceClient(conn)
    _, err = client.ProcessPayment(ctx, &paymentv1.PaymentRequest{...})
    return err
}
```

## 🐳 Deployment (Beta)

### Docker

```bash
docker run -d \
  -p 50050:50050 \
  -p 2112:2112 \
  -e VOYAGER_ETCD_ENDPOINTS=http://etcd1:2379 \
  -e VOYAGER_AUTH_TOKEN=your-secure-token \
  ghcr.io/kolkov/voyagerd:beta
```

### Kubernetes (Beta)

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: voyager-discovery
spec:
  replicas: 3
  template:
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
              value: "etcd-cluster:2379"
            - name: VOYAGER_AUTH_TOKEN
              valueFrom:
                secretKeyRef:
                  name: voyager-secrets
                  key: auth-token
```

## 🔧 Development Workflow

### Before You Start
1. Read the [Release Guide](RELEASE_GUIDE.md)
2. Set up development environment:

```bash
# Install tools
make install-tools

# Generate code
make generate
```

### Common Tasks

```bash
# Build binaries
make build

# Run tests
make test

# Lint code
make lint

# Run full service stack
make run

# Prepare release branch
make release-prepare VERSION=v1.0.0-beta.X
```

## ⚙️ Configuration Options

### Server Configuration (voyagerd)

| Option | Description | Default | Environment Variable |
|--------|-------------|---------|----------------------|
| `--etcd-endpoints` | ETCD endpoints | `http://localhost:2379` | `VOYAGER_ETCD_ENDPOINTS` |
| `--cache-ttl` | Cache TTL | `30s` | `VOYAGER_CACHE_TTL` |
| `--auth-token` | Authentication token | "" | `VOYAGER_AUTH_TOKEN` |
| `--grpc-addr` | gRPC server address | `:50050` | `VOYAGER_GRPC_ADDR` |
| `--metrics-addr` | Metrics HTTP address | `:2112` | `VOYAGER_METRICS_ADDR` |
| `--log-interval` | Service logging interval | `15s` | `VOYAGER_LOG_INTERVAL` |
| `--log-format` | Log format (text/json) | `text` | `VOYAGER_LOG_FORMAT` |
| `--debug` | Enable debug logging | `false` | `VOYAGER_DEBUG` |

### Client Options

| Option | Description | Default |
|--------|-------------|---------|
| `WithTTL` | Cache TTL | 30s |
| `WithInsecure` | Disable TLS | false |
| `WithTLS` | Configure TLS | - |
| `WithBalancerStrategy` | Load balancing strategy | RoundRobin |
| `WithConnectionTimeout` | Connection timeout | 5s |
| `WithRetryPolicy` | Retry policy (maxRetries, delay) | 5, 2s |
| `WithAuthToken` | Authentication token | "" |

## 📊 Monitoring

VoyagerSD exposes Prometheus metrics:

```bash
curl http://localhost:2112/metrics
```

### Key Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `voyager_registrations_total` | Counter | Total service registrations |
| `voyager_discoveries_total` | Counter | Total service discoveries |
| `voyager_service_instances` | Gauge | Number of service instances |
| `voyager_cache_refreshes_total` | Counter | Total cache refresh operations |
| `voyager_cache_refresh_errors_total` | Counter | Total cache refresh errors |
| `voyager_grpc_requests_total` | Counter | Total gRPC requests |
| `voyager_grpc_response_time_seconds` | Histogram | gRPC response time |

### Health Endpoints
- `http://localhost:2112/health` - Liveness probe
- `http://localhost:2112/ready` - Readiness probe

## 🛠️ Makefile Reference

```bash
make install-tools      # Install development dependencies
make generate          # Generate protobuf code
make build             # Build all binaries
make test              # Run unit tests
make lint              # Run linters
make docker            # Build Docker images
make run               # Run services locally
make release-test      # Run release validation checks
```

## 🔒 Security Best Practices

1. **Always use TLS** for gRPC communications
2. **Rotate authentication tokens** regularly
3. **Use network policies** to restrict access
4. **Run as non-root user** in containers
5. **Limit resource usage** with Kubernetes resource quotas

```go
// Secure client configuration
voyager, err := client.New("discovery:50050",
    client.WithTLSConfig(&tls.Config{
        // Your TLS configuration
    }),
    client.WithAuthToken("secure-token"))
```

## ❓ Getting Help

For usage questions, bug reports, or feature requests:

- [File a GitHub issue](https://github.com/kolkov/voyager/issues)
- Join our [Discord community](https://discord.gg/voyager-sd)

## 🤝 Contributing to Beta

We especially welcome contributions during our beta phase! Please follow these steps:

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/your-feature`)
3. Commit your changes (`git commit -am 'Add some feature'`)
4. Push to the branch (`git push origin feature/your-feature`)
5. Submit a pull request against the `beta` branch

Before submitting, please ensure:
- All tests pass
- Code is properly formatted
- New features include tests
- Documentation is updated

## 📜 License

Apache 2.0 - See [LICENSE](LICENSE) for details

---

VoyagerSD Beta is developed and maintained by the Kolkov team.  
Help us improve by reporting issues and suggesting features!
