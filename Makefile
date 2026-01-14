.PHONY: build test test-quick test-short lint run clean generate tidy fmt fmt-check vet proto help install-tools

APP_NAME := feather
BUILD_DIR := ./bin
MAIN_PATH := ./cmd/feather

# Build settings
GO := go
GOFLAGS := -v
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
GIT_COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS := -s -w -X main.version=$(VERSION) -X main.gitCommit=$(GIT_COMMIT) -X main.buildDate=$(BUILD_DATE)

## help: Show this help message
help:
	@echo "Usage: make [target]"
	@echo ""
	@sed -n 's/^## //p' $(MAKEFILE_LIST) | column -t -s ':' | sed 's/^/  /'

## install-tools: Install development tools (golangci-lint, goimports)
install-tools:
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.62.2
	go install golang.org/x/tools/cmd/goimports@latest
	@echo "Development tools installed successfully."

## build: Build the feather server binary
build:
	$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(APP_NAME) $(MAIN_PATH)

## build-tui: Build the terminal UI binary
build-tui:
	$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(APP_NAME)-tui ./cmd/feather-tui

## build-mcp: Build the MCP server binary
build-mcp:
	$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(APP_NAME)-mcp ./cmd/feather-mcp

## build-cli: Build the CLI client binary
build-cli:
	$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS) -X github.com/feather-store/feather/cmd/feather-cli/cmd.Version=$(VERSION) -X github.com/feather-store/feather/cmd/feather-cli/cmd.GitCommit=$(GIT_COMMIT) -X github.com/feather-store/feather/cmd/feather-cli/cmd.BuildDate=$(BUILD_DATE)" -o $(BUILD_DIR)/$(APP_NAME)-cli ./cmd/feather-cli

## build-all: Build all binaries (server, tui, mcp, cli)
build-all: build build-tui build-mcp build-cli

build-race:
	$(GO) build $(GOFLAGS) -race -o $(BUILD_DIR)/$(APP_NAME) $(MAIN_PATH)

## test: Run all tests with race detector and coverage
test:
	$(GO) test -v -race -count=1 -cover ./...

## test-quick: Run short tests without race detector (fast feedback)
test-quick:
	$(GO) test -short -count=1 ./...

## test-short: Run short tests with verbose output
test-short:
	$(GO) test -v -short -count=1 ./...

## test-coverage: Run tests and generate HTML coverage report
test-coverage:
	$(GO) test -v -race -count=1 -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out -o coverage.html

## test-integration: Run integration tests (requires Docker for some tests)
test-integration:
	$(GO) test -v -tags=integration -count=1 ./test/...

## lint: Run golangci-lint
lint:
	golangci-lint run ./...

## run: Run the server with default configuration
run:
	$(GO) run $(MAIN_PATH)

## run-config: Run the server with the example config file
run-config:
	$(GO) run $(MAIN_PATH) -config configs/feather.yaml

## run-dev: Run the server with the minimal dev config
run-dev:
	$(GO) run $(MAIN_PATH) -config configs/feather-dev.yaml

## clean: Remove build artifacts and coverage files
clean:
	rm -rf $(BUILD_DIR)
	rm -f coverage.out coverage.html

## tidy: Tidy go module dependencies
tidy:
	$(GO) mod tidy

## fmt: Format Go source files
fmt:
	$(GO) fmt ./...
	goimports -w .

## fmt-check: Check formatting without modifying files
fmt-check:
	@test -z "$$(gofmt -l .)" || (echo "Files need formatting:"; gofmt -l .; exit 1)
	@test -z "$$(goimports -l .)" || (echo "Files need import ordering:"; goimports -l .; exit 1)

## vet: Run go vet
vet:
	$(GO) vet ./...

# Generate protobuf (requires protoc, protoc-gen-go, and protoc-gen-go-grpc)
# Install plugins: go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
#                  go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
proto:
	protoc --go_out=. --go_opt=paths=source_relative \
		--go_opt=Mapi/proto/feather.proto=github.com/feather-store/feather/api/proto \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		--go-grpc_opt=Mapi/proto/feather.proto=github.com/feather-store/feather/api/proto \
		api/proto/feather.proto

# Development helpers
## dev: Format, vet, test, and build
dev: fmt vet test build

# Docker
## docker-build: Build the Docker image
docker-build:
	docker build -t $(APP_NAME):latest .

## docker-run: Run the Docker image with default ports
docker-run:
	docker run -p 8080:8080 -p 50051:50051 -p 9090:9090 $(APP_NAME):latest

# Benchmarks
## bench: Run all benchmarks
bench:
	$(GO) test -bench=. -benchmem ./...

## check: Run all checks (format check, vet, lint, test)
check: fmt-check vet lint test
