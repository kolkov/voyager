# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Copy go.mod and go.sum to download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy the entire project
COPY . .

# Build the discovery server
RUN go build -o /voyager-discovery ./examples/discovery-server

# Final stage
FROM alpine:3.18
RUN apk --no-cache add ca-certificates
COPY --from=builder /voyager-discovery /voyager-discovery

# gRPC port
EXPOSE 50050
# Metrics port
EXPOSE 2112

CMD ["/voyager-discovery"]