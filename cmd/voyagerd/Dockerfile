# Build stage
FROM golang:1.24-alpine AS builder
ARG VERSION="dev"
WORKDIR /app

# Cache dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy required components
COPY cmd/ cmd/
COPY server/ server/
COPY client/ client/
COPY internal/ internal/
COPY proto/ proto/

# Build binary
RUN cd cmd/voyagerd && \
    CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-w -s -X main.version=$VERSION" \
    -o /voyagerd

# Final stage
FROM alpine:3.18
RUN apk --no-cache add ca-certificates tzdata curl dumb-init envsubst
WORKDIR /app

# Copy binary and config template
COPY --from=builder /voyagerd /app/voyagerd
COPY cmd/voyagerd/voyagerd.template.yaml /etc/voyager/

# Entrypoint script
COPY cmd/voyagerd/docker-entrypoint.sh /app/entrypoint.sh
RUN chmod +x /app/entrypoint.sh

# Setup non-root user
RUN addgroup -S voyager && adduser -S voyager -G voyager
USER voyager

# Expose ports
EXPOSE 50050 2112

# Healthcheck
HEALTHCHECK --interval=30s --timeout=5s \
  CMD curl -f http://localhost:2112/health || exit 1

ENTRYPOINT ["/app/entrypoint.sh"]