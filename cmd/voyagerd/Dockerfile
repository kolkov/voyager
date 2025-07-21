# Builder stage
FROM golang:1.24-alpine AS builder
ARG VERSION
WORKDIR /app

COPY ../../go.mod go.sum ./
RUN go mod download

COPY ../.. .
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-w -s -X main.version=$VERSION" \
    -o voyagerd ./cmd/voyagerd

# Final stage
FROM alpine:3.18
RUN apk add --no-cache ca-certificates tzdata curl dumb-init
WORKDIR /app

COPY --from=builder /app/voyagerd /app/
COPY cmd/voyagerd/docker-entrypoint.sh /app/
COPY cmd/voyagerd/voyagerd.template.yaml /etc/voyager/

RUN chmod +x /app/docker-entrypoint.sh && \
    addgroup -S voyager && adduser -S voyager -G voyager
USER voyager

EXPOSE 50050 2112
HEALTHCHECK --interval=30s --timeout=5s \
  CMD curl -f http://localhost:2112/health || exit 1
ENTRYPOINT ["/usr/bin/dumb-init", "--", "/app/docker-entrypoint.sh"]