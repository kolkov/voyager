# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Copy go.mod and go.sum to download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy the entire project
COPY . .

# Build the order-service
RUN cd examples/order-service && \
    go build -o /order-service

# Final stage
FROM alpine:latest
RUN apk --no-cache add ca-certificates
COPY --from=builder /order-service /order-service

EXPOSE 8080
ENTRYPOINT ["/order-service"]