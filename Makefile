# Unified Makefile for Voyager Service Discovery
GO := go
BIN_DIR := bin
VERSION := $(shell git describe --tags --always 2>/dev/null | sed 's/-beta-/-beta./' | sed 's/-g[0-9a-f]\+$$//' || echo "v0.0.0-dev")
COMMIT := $(shell git rev-parse HEAD 2>/dev/null || echo "none")
DATE := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
DOCKER_TAG ?= latest
GOOS ?= $(shell $(GO) env GOOS)
GOARCH ?= $(shell $(GO) env GOARCH)

# Set .exe extension for Windows
ifeq ($(OS),Windows_NT)
    EXT := .exe
    OPEN_CMD := start
else
    EXT :=
    UNAME_S := $(shell uname -s)
    ifeq ($(UNAME_S),Linux)
        OPEN_CMD := xdg-open
    else
        OPEN_CMD := open
    endif
endif

# LDFLAGS for version injection
LDFLAGS := -ldflags "-s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)"

# Services to build
SERVICES := voyagerd

# Common commands
MKDIR := mkdir -p
RM := rm -rf
CP := cp

.PHONY: all tidy generate test test-unit test-integration build docker clean lint cover install-tools help version release release-test release-prepare release-publish release-abort goreleaser goreleaser-release release-guide

all: lint test build

## version: Show version information
version:
	@echo "Version:    $(VERSION)"
	@echo "Commit:     $(COMMIT)"
	@echo "Build Date: $(DATE)"
	@echo "Platform:   $(GOOS)/$(GOARCH)"

## install-tools: Install required development tools
install-tools:
	@echo "[TOOLS] Installing development tools..."
	@$(GO) install github.com/bufbuild/buf/cmd/buf@v1.55.1
	@$(GO) install google.golang.org/protobuf/cmd/protoc-gen-go@v1.28.1
	@$(GO) install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.2.0
	@$(GO) install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest
	@$(GO) install github.com/goreleaser/goreleaser@latest
	@echo "OK Tools installed successfully"

## tidy: Tidying Go modules
tidy: generate
	@echo "[TIDY] Tidying Go modules..."
	@$(GO) mod tidy
	@echo "OK Go modules tidied"

## generate: Generate code from .proto files
generate: install-tools
	@echo "[GEN] Generating gRPC code..."
	@buf generate --template buf.gen.yaml
	@echo "OK Code generation complete"

## test: Run all tests (unit + integration)
test: test-unit test-integration

## test-unit: Run unit tests only
test-unit:
	@echo "[TEST] Running unit tests..."
	@$(GO) test -v -coverprofile=unit-coverage.out ./client ./server
	@echo "OK Unit tests completed"

## test-integration: Run integration tests only
test-integration:
	@echo "[TEST] Running integration tests..."
	@if [ "$(GOOS)" != "windows" ]; then \
		$(GO) test -v -coverprofile=integration-coverage.out -tags=integration ./test/integration; \
	else \
		echo "Skipping integration tests on Windows"; \
	fi
	@echo "OK Integration tests completed"

## clean: Remove build artifacts
clean:
	@echo "[CLEAN] Cleaning up..."
	@$(RM) $(BIN_DIR)
	@$(RM) *.out
	@$(RM) dist
	@echo "OK Cleanup complete"

## lint: Run code linters
lint: generate
	@echo "[LINT] Running linters..."
	@golangci-lint run --fix
	@echo "OK Linting complete"

## cover: Open coverage report
cover:
	@$(GO) tool cover -html=coverage.out

## build: Build all binaries
build:
	@echo "[BUILD] Building binaries for $(GOOS)/$(GOARCH)..."
	@$(MKDIR) $(BIN_DIR)
	@for service in $(SERVICES); do \
		echo "Building $$service..."; \
		$(GO) build $(LDFLAGS) -o $(BIN_DIR)/$$service$(EXT) ./cmd/$$service; \
	done
	@echo "OK Binaries built in $(BIN_DIR)/"

## docker: Build Docker image
docker:
	@echo "[DOCKER] Building Docker image..."
	@docker build -t voyagerd:$(VERSION) -f cmd/voyagerd/Dockerfile .
	@echo "OK Docker image built"

## run: Run service locally
run: build
	@echo "[RUN] Starting voyagerd..."
	@$(BIN_DIR)/voyagerd$(EXT) \
		--etcd-endpoints=http://localhost:2379 \
		--auth-token=secure-token \
		--log-format=text \
		--debug

## release-test: Run all tests required for release
release-test: lint test-unit
	@echo "OK Release validation passed"

## release-prepare: Prepare release branch
release-prepare:
ifndef VERSION
	$(error VERSION is not set. Usage: make release-prepare VERSION=vX.Y.Z)
endif
	@echo "[RELEASE] Preparing release $(VERSION)"
	@git checkout -b release/$(VERSION)
	@echo "OK Release branch created: release/$(VERSION)"

## goreleaser: Build release with GoReleaser (dry-run)
goreleaser:
	@echo "[GORELEASER] Building release snapshot..."
	@goreleaser build --snapshot --clean
	@echo "OK Snapshot build complete"

## goreleaser-release: Create full release with GoReleaser (requires GITHUB_TOKEN)
goreleaser-release:
	@echo "[GORELEASER] Creating full release..."
	@goreleaser release --clean
	@echo "OK Release published"

## release-guide: Open release process documentation
release-guide:
	@echo "[DOCS] Opening release guide..."
	@$(OPEN_CMD) RELEASE_GUIDE.md 2>/dev/null || echo "Open manually: RELEASE_GUIDE.md"

## dev-setup: Set up development environment
dev-setup:
	@echo "[SETUP] Setting up development environment..."
	@chmod +x dev-setup.sh
	@./dev-setup.sh

## help: Show available commands
help:
	@echo "Voyager Service Discovery System - Makefile"
	@echo "=========================================="
	@echo "Available commands:"
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-20s %s\n", $$1, $$2}' $(MAKEFILE_LIST) | sort
	@echo ""
	@echo "Examples:"
	@echo "  make build             # Build main binary"
	@echo "  make test-unit         # Run unit tests only"
	@echo "  make test-integration  # Run integration tests (non-Windows)"
	@echo "  make run               # Start service locally"