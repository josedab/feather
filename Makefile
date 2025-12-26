.PHONY: build test lint run clean generate tidy fmt vet proto

APP_NAME := feather
BUILD_DIR := ./bin
MAIN_PATH := ./cmd/feather

# Build settings
GO := go
GOFLAGS := -v
LDFLAGS := -s -w

build:
	$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(APP_NAME) $(MAIN_PATH)

build-tui:
	$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(APP_NAME)-tui ./cmd/feather-tui

build-mcp:
	$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(APP_NAME)-mcp ./cmd/feather-mcp

build-all: build build-tui build-mcp

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

# Generate protobuf (requires protoc and protoc-gen-go)
proto:
	protoc --go_out=. --go-grpc_out=. api/proto/feather.proto

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
