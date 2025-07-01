FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o /voyager-discovery ./examples/discovery-server

FROM alpine:latest
RUN apk --no-cache add ca-certificates
COPY --from=builder /voyager-discovery /voyager-discovery

EXPOSE 50050
ENTRYPOINT ["/voyager-discovery"]