# VoyagerSD - Service Discovery for Go Microservices
# Cross-platform Makefile for PowerShell 7.5+ and bash

GO := go
GOOS ?= $(shell $(GO) env GOOS)
GOARCH ?= $(shell $(GO) env GOARCH)
BIN_DIR := bin
PROTO_DIR := proto
PROTO_OUT := proto
DOCKER_TAG ?= latest
VERSION ?= $(shell git describe --tags --always)

# Manually list proto files for cross-platform compatibility
PROTO_FILES := \
    proto/voyager/v1/voyager.proto \
    proto/order/v1/order.proto \
    proto/payment/v1/payment.proto

# Cross-platform commands
ifeq ($(OS),Windows_NT)
    RM_CMD := del /f /q /s
    RMDIR_CMD := rmdir /s /q
    MKDIR_CMD := mkdir
    NULL_DEVICE := NUL
    SHELL := cmd
    .SHELLFLAGS := /c
    EXT := .exe
else
    RM_CMD := rm -f
    RMDIR_CMD := rm -rf
    MKDIR_CMD := mkdir -p
    NULL_DEVICE := /dev/null
    EXT :=
endif

SERVICES := order-service payment-service discovery-server

.PHONY: all generate test test-all test-integration build docker clean run-examples lint cover check-generated verify-generated

all: lint test-all build

## generate: Generate code from .proto files
generate:
	@echo "Generating gRPC code..."
	@$(MKDIR_CMD) "$(PROTO_OUT)"
	@for proto in $(PROTO_FILES); do \
		protoc --go_out=. --go_opt=module=github.com/kolkov/voyager \
			--go-grpc_out=. --go-grpc_opt=module=github.com/kolkov/voyager \
			-I"$(PROTO_DIR)" "$$proto"; \
	done
	@echo "Code generation complete"

## check-generated: Verify generated code is up-to-date
check-generated:
	@echo "Checking generated code is up-to-date..."
	@$(MAKE) generate > $(NULL_DEVICE) 2>&1
	@git diff --exit-code || (echo "Error: Generated files are out of date. Run 'make generate' and commit changes."; exit 1)
	@echo "Generated code is up-to-date"

## verify-generated: Check if files were generated
verify-generated:
	@echo "Verifying generated files..."
	@for file in \
		proto/voyager/v1/voyager.pb.go \
		proto/order/v1/order_grpc.pb.go \
		proto/payment/v1/payment.pb.go; \
	do \
		if [ ! -f "$$file" ]; then \
			echo "Error: Generated file $$file is missing"; \
			exit 1; \
		fi; \
	done
	@echo "All generated files present"

## test: Run unit tests with coverage
test: check-generated
	@echo "Running unit tests..."
	@$(GO) test -v -coverprofile=coverage.out -covermode=atomic ./...

## test-integration: Run integration tests
test-integration: check-generated
	@echo "Running integration tests..."
	@$(GO) test -v -tags=integration ./test/integration

## test-all: Run all tests (unit + integration)
test-all: test test-integration

## lint: Run linters (golangci-lint required)
lint: check-generated
	@echo "Running linters..."
	@golangci-lint run

## cover: Open coverage report
cover:
	@$(GO) tool cover -html=coverage.out

## build: Build binaries for current OS
build: build-voyagerd build-examples

## build-voyagerd: Build the voyagerd server
build-voyagerd: generate
	@echo "Building voyagerd for $(GOOS)/$(GOARCH)..."
	@$(MKDIR_CMD) "$(BIN_DIR)"
	@$(GO) build -ldflags="-X main.version=$(VERSION)" -o "$(BIN_DIR)/voyagerd$(EXT)" ./cmd/voyagerd
	@echo "voyagerd build complete"

## build-examples: Build example services
build-examples: generate
	@$(MKDIR_CMD) "$(BIN_DIR)"
	@for service in $(SERVICES); do \
		echo "Building $$service for $(GOOS)/$(GOARCH)..."; \
		$(GO) build -o "$(BIN_DIR)/$$service$(EXT)" "./examples/$$service"; \
	done
	@echo "Example services build complete"

## build-windows: Build binaries for Windows
build-windows:
	@$(MAKE) build GOOS=windows GOARCH=amd64

## build-linux: Build binaries for Linux
build-linux:
	@$(MAKE) build GOOS=linux GOARCH=amd64

## docker: Build Docker images
docker: docker-voyagerd docker-examples

## docker-voyagerd: Build Docker image for voyagerd
docker-voyagerd: generate
	@echo "Building Docker image for voyagerd..."
	@docker build \
		--build-arg VERSION=$(VERSION) \
		-t voyagerd:$(VERSION) \
		-f cmd/voyagerd/Dockerfile \
		.

## docker-examples: Build Docker images for example services
.PHONY: docker-examples
docker-examples:
	@echo "Building Docker images for example services..."
	@for service in $(SERVICES); do \
		echo "Building $$service..."; \
		docker build -f "examples/$$service/Dockerfile" \
			-t "voyager-example-$$service:$(DOCKER_TAG)" .; \
	done

## clean: Remove generated files and binaries
clean:
	@echo "Cleaning up..."
ifeq ($(OS),Windows_NT)
	@if exist "$(BIN_DIR)" $(RMDIR_CMD) "$(BIN_DIR)"
	@$(RM_CMD) coverage.out coverage.txt coverage-unit.txt coverage-integration.txt .goreleaser.yaml
	@for /r . %%f in (*.pb.go) do @del /q "%%f" >nul 2>&1
else
	@$(RMDIR_CMD) "$(BIN_DIR)"
	@$(RM_CMD) coverage.out coverage.txt coverage-unit.txt coverage-integration.txt .goreleaser.yaml
	@find . -name '*.pb.go' -delete
endif
	@echo "Clean complete"

## run-full: Run voyagerd and example services together
run-full: build
	@echo "Starting Voyager Discovery Server..."
	@"$(BIN_DIR)/voyagerd$(EXT)" &
	@timeout 2 >$(NULL_DEVICE) || sleep 2
	@echo "Starting example services..."
	@VOYAGER_ADDR=localhost:50050 "$(BIN_DIR)/order-service$(EXT)" &
	@VOYAGER_ADDR=localhost:50050 "$(BIN_DIR)/payment-service$(EXT)" &
	@echo "All services running. Press Ctrl+C to stop."

## run-examples-windows: Run examples in PowerShell (Windows only)
run-examples-windows: build-examples
	@echo "Starting example services in separate PowerShell windows..."
	@start pwsh -NoExit -Command "$$env:VOYAGER_ADDR='localhost:50050'; .\bin\order-service.exe"
	@start pwsh -NoExit -Command "$$env:VOYAGER_ADDR='localhost:50050'; .\bin\payment-service.exe"
	@echo "Services started in separate windows"

## deploy-kubernetes: Deploy to Kubernetes cluster
deploy-kubernetes:
	@echo "Deploying Voyager to Kubernetes..."
	@kubectl apply -f deployments/kubernetes/
	@echo "Deployment complete"

## run-docker-stack: Run full docker stack with examples
run-docker-stack:
	@echo "Starting Voyager Docker stack..."
	@docker compose -f examples/docker-compose.yaml up -d
	@echo "Stack started. Use 'docker compose -f examples/docker-compose.yaml down' to stop."

## help: Show available commands
help:
	@echo "VoyagerSD - Service Discovery System"
	@echo "Available commands:"
	@echo
	@sed -n 's/^## //p' $(MAKEFILE_LIST) | column -t -s ':'