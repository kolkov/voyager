# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Copy go.mod and go.sum to download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy the entire project
COPY . .

# Build the payment-service
RUN cd examples/payment-service && \
    go build -o /payment-service

# Final stage
FROM alpine:latest
RUN apk --no-cache add ca-certificates
COPY --from=builder /payment-service /payment-service

EXPOSE 8080
ENTRYPOINT ["/payment-service"]