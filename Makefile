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
SERVICES := voyagerd order-service payment-service discovery-server

# Common commands
MKDIR := mkdir -p
RM := rm -rf
CP := cp

.PHONY: all tidy generate test build docker clean lint cover install-tools help version release release-test release-prepare release-publish release-abort goreleaser goreleaser-release release-guide

all: lint test build

## version: Show version information
version:
	@echo "Version:    $(VERSION)"
	@echo "Commit:     $(COMMIT)"
	@echo "Build Date: $(DATE)"
	@echo "Platform:   $(GOOS)/$(GOARCH)"

## install-tools: Install required development tools
install-tools:
	@echo "🛠️ Installing development tools..."
	@$(GO) install github.com/bufbuild/buf/cmd/buf@v1.55.1
	@$(GO) install google.golang.org/protobuf/cmd/protoc-gen-go@v1.28.1
	@$(GO) install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.2.0
	@$(GO) install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest
	@$(GO) install github.com/goreleaser/goreleaser@latest
	@echo "✅ Tools installed successfully"

## tidy: Tidying Go modules
tidy: generate
	@echo "🧹 Tidying Go modules..."
	@$(GO) mod tidy
	@echo "✅ Go modules tidied"

## generate: Generate code from .proto files
generate: install-tools
	@echo "🔧 Generating gRPC code..."
	@buf generate --template buf.gen.yaml
	@echo "✅ Code generation complete"

## test: Run unit tests
test:
	@echo "🧪 Running unit tests..."
	@$(GO) test -v -coverprofile=coverage.out ./...
	@echo "✅ Unit tests completed"

## clean: Remove build artifacts
clean:
	@echo "🧼 Cleaning up..."
	@$(RM) $(BIN_DIR)
	@$(RM) coverage.out
	@$(RM) dist
	@echo "✅ Cleanup complete"

## lint: Run code linters
lint: generate
	@echo "🔍 Running linters..."
	@golangci-lint run --fix
	@echo "✅ Linting complete"

## cover: Open coverage report
cover:
	@$(GO) tool cover -html=coverage.out

## build: Build all binaries
build:
	@echo "🔨 Building binaries for $(GOOS)/$(GOARCH)..."
	@$(MKDIR) $(BIN_DIR)
	@for service in $(SERVICES); do \
		echo "Building $$service..."; \
		if [ -d "./cmd/$$service" ]; then \
			$(GO) build $(LDFLAGS) -o $(BIN_DIR)/$$service$(EXT) ./cmd/$$service; \
		else \
			$(GO) build $(LDFLAGS) -o $(BIN_DIR)/$$service$(EXT) ./examples/$$service; \
		fi; \
	done
	@echo "✅ Binaries built in $(BIN_DIR)/"

## docker: Build all Docker images
docker:
	@echo "🐳 Building Docker images..."
	@docker build -t voyagerd:$(VERSION) -f cmd/voyagerd/Dockerfile .
	@for service in $(SERVICES); do \
		if [ -f "examples/$$service/Dockerfile" ]; then \
			echo "Building $$service Docker image..."; \
			docker build -f examples/$$service/Dockerfile -t voyager-example-$$service:$(DOCKER_TAG) .; \
		fi; \
	done
	@echo "✅ Docker images built"

## run: Run all services locally
run: build
	@echo "🚀 Starting services..."
	@$(BIN_DIR)/voyagerd$(EXT) &
	@sleep 2
	@VOYAGER_ADDR=localhost:50050 $(BIN_DIR)/order-service$(EXT) &
	@VOYAGER_ADDR=localhost:50050 $(BIN_DIR)/payment-service$(EXT) &
	@echo "✅ Services running. Press Ctrl+C to stop."

## release: Prepare release artifacts
release: clean lint test build
	@echo "📦 Preparing release artifacts..."
	@$(MKDIR) release
	@$(CP) $(BIN_DIR)/* release/
	@$(CP) LICENSE release/
	@$(CP) README.md release/
	@$(CP) CHANGELOG.md release/
	@$(CP) -R deployments release/
	@tar -czf voyager-$(VERSION)-$(GOOS)-$(GOARCH).tar.gz -C release .
	@$(RM) release
	@echo "✅ Release package created: voyager-$(VERSION)-$(GOOS)-$(GOARCH).tar.gz"

## release-test: Run all tests required for release
release-test: lint test
	@echo "✅ Release validation passed"

## release-prepare: Prepare release branch
release-prepare:
ifndef VERSION
	$(error VERSION is not set. Usage: make release-prepare VERSION=vX.Y.Z)
endif
	@echo "🚧 Preparing release $(VERSION)"
	@git checkout develop
	@git pull
	@git checkout -b release/$(VERSION)
	@echo "✅ Release branch created: release/$(VERSION)"

## release-publish: Publish release to main
release-publish:
ifndef VERSION
	$(error VERSION is not set. Usage: make release-publish VERSION=vX.Y.Z)
endif
	@echo "🚀 Publishing release $(VERSION)"
	@git checkout main
	@git merge --no-ff release/$(VERSION)
	@git tag -a $(VERSION) -m "Release $(VERSION)"
	@git push origin main
	@git push origin --tags
	@git checkout develop
	@git merge --no-ff release/$(VERSION)
	@git push origin develop
	@git branch -d release/$(VERSION)
	@echo "✅ Release $(VERSION) published successfully"

## release-abort: Abort current release
release-abort:
ifndef VERSION
	$(error VERSION is not set. Usage: make release-abort VERSION=vX.Y.Z)
endif
	@echo "🛑 Aborting release $(VERSION)"
	@git checkout develop
	@git branch -D release/$(VERSION)
	@echo "✅ Release aborted"

## goreleaser: Build release with GoReleaser (dry-run)
goreleaser:
	@echo "🔧 Building release snapshot with GoReleaser..."
	@goreleaser build --snapshot --clean
	@echo "✅ Snapshot build complete"

## goreleaser-release: Create full release with GoReleaser (requires GITHUB_TOKEN)
goreleaser-release:
	@echo "🚀 Creating full release with GoReleaser..."
	@goreleaser release --clean
	@echo "✅ Release published"

## release-guide: Open release process documentation
release-guide:
	@echo "📖 Opening release guide..."
	@$(OPEN_CMD) RELEASE_GUIDE.ru.md 2>/dev/null || echo "Open the file manually: RELEASE_GUIDE.ru.md"

## help: Show available commands
help:
	@echo "Voyager Service Discovery System - Makefile"
	@echo "=========================================="
	@echo "Available commands:"
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-20s %s\n", $$1, $$2}' $(MAKEFILE_LIST) | sort
	@echo ""
	@echo "Examples:"
	@echo "  make build             # Build all binaries"
	@echo "  make release-test      # Run all release validation checks"
	@echo "  make release-prepare VERSION=v1.2.0"
	@echo "  make release-guide     # Open release process documentation"