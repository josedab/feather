.PHONY: build test lint run clean generate tidy fmt vet proto

APP_NAME := feather
BUILD_DIR := ./bin
MAIN_PATH := ./cmd/feather

# Build settings
GO := go
GOFLAGS := -v
LDFLAGS := -s -w
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
GIT_COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

build:
	$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(APP_NAME) $(MAIN_PATH)

build-tui:
	$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(APP_NAME)-tui ./cmd/feather-tui

build-mcp:
	$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(APP_NAME)-mcp ./cmd/feather-mcp

build-cli:
	$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS) -X github.com/feather-store/feather/cmd/feather-cli/cmd.Version=$(VERSION) -X github.com/feather-store/feather/cmd/feather-cli/cmd.GitCommit=$(GIT_COMMIT) -X github.com/feather-store/feather/cmd/feather-cli/cmd.BuildDate=$(BUILD_DATE)" -o $(BUILD_DIR)/$(APP_NAME)-cli ./cmd/feather-cli

build-all: build build-tui build-mcp build-cli

build-race:
	$(GO) build $(GOFLAGS) -race -o $(BUILD_DIR)/$(APP_NAME) $(MAIN_PATH)

test:
	$(GO) test -v -race -cover ./...

test-short:
	$(GO) test -v -short ./...

test-coverage:
	$(GO) test -v -race -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out -o coverage.html

lint:
	golangci-lint run ./...

run:
	$(GO) run $(MAIN_PATH)

run-config:
	$(GO) run $(MAIN_PATH) -config configs/feather.yaml

clean:
	rm -rf $(BUILD_DIR)
	rm -f coverage.out coverage.html

tidy:
	$(GO) mod tidy

fmt:
	$(GO) fmt ./...
	goimports -w .

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
dev: fmt vet test build

# Docker
docker-build:
	docker build -t $(APP_NAME):latest .

docker-run:
	docker run -p 8080:8080 -p 50051:50051 -p 9090:9090 $(APP_NAME):latest

# Benchmarks
bench:
	$(GO) test -bench=. -benchmem ./...

# All checks
check: fmt vet lint test
