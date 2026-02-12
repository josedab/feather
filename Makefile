.PHONY: build test test-quick test-short test-core lint run run-config run-dev run-cli run-tui clean generate tidy fmt fmt-check vet validate-config proto help install-tools setup quickstart quickstart-docker quickstart-local demo doctor smoke-test dev-start dev-stop stop-dev check-quick explore examples docs api-routes list-extensions

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

## setup: One-command contributor setup (doctor, tools, hooks, build, test)
setup: doctor install-tools
	@git config core.hooksPath .githooks
	@echo "Git hooks configured (.githooks/pre-commit)."
	@$(MAKE) build
	@$(MAKE) test-core
	@echo ""
	@echo "✅ Setup complete! You're ready to contribute."
	@echo "   Run 'make run-dev' to start the server."
	@echo "   Run 'make check-quick' before committing."

## build: Build the feather server binary (CGO disabled; use build-cgo for Kafka)
build:
	CGO_ENABLED=0 $(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(APP_NAME) $(MAIN_PATH)

## build-cgo: Build with CGO enabled (required for Kafka/librdkafka support)
build-cgo:
	CGO_ENABLED=1 $(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(APP_NAME) $(MAIN_PATH)

## build-tui: Build the terminal UI binary
build-tui:
	CGO_ENABLED=0 $(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(APP_NAME)-tui ./cmd/feather-tui

## build-mcp: Build the MCP server binary
build-mcp:
	CGO_ENABLED=0 $(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(APP_NAME)-mcp ./cmd/feather-mcp

## build-cli: Build the CLI client binary
build-cli:
	CGO_ENABLED=0 $(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS) -X github.com/feather-store/feather/cmd/feather-cli/cmd.Version=$(VERSION) -X github.com/feather-store/feather/cmd/feather-cli/cmd.GitCommit=$(GIT_COMMIT) -X github.com/feather-store/feather/cmd/feather-cli/cmd.BuildDate=$(BUILD_DATE)" -o $(BUILD_DIR)/$(APP_NAME)-cli ./cmd/feather-cli

## build-all: Build all binaries (server, tui, mcp, cli)
build-all: build build-tui build-mcp build-cli

build-race:
	$(GO) build $(GOFLAGS) -race -o $(BUILD_DIR)/$(APP_NAME) $(MAIN_PATH)

## test: Run all tests with race detector and coverage
test:
	$(GO) test -v -race -count=1 -cover -timeout 300s ./...

## test-quick: Run short tests without race detector (fast feedback)
test-quick:
	@START=$$(date +%s); \
	$(GO) test -short -count=1 -timeout 120s ./... 2>&1 | grep -E '(^ok|FAIL|---)'; \
	EXIT=$$?; \
	ELAPSED=$$(( $$(date +%s) - $$START )); \
	if [ $$EXIT -eq 0 ]; then echo ""; echo "✅ All tests passed ($$ELAPSED""s)"; \
	else echo ""; echo "❌ Some tests failed ($$ELAPSED""s)"; exit 1; fi

## test-core: Run core package tests only (~10s) with coverage summary
test-core:
	@START=$$(date +%s); \
	$(GO) test -short -count=1 -cover -timeout 60s ./internal/core/... 2>&1 | grep -E '(^ok|FAIL|coverage|---)'; \
	EXIT=$$?; \
	ELAPSED=$$(( $$(date +%s) - $$START )); \
	if [ $$EXIT -eq 0 ]; then echo ""; echo "✅ Core tests passed ($$ELAPSED""s)"; \
	else echo ""; echo "❌ Some core tests failed ($$ELAPSED""s)"; exit 1; fi

## test-short: Run short tests with verbose output
test-short:
	$(GO) test -v -short -count=1 -timeout 120s ./...

## test-coverage: Run tests and generate HTML coverage report
test-coverage:
	$(GO) test -v -race -count=1 -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out -o coverage.html

## test-integration: Run integration tests (requires Docker for some tests)
test-integration:
	$(GO) test -v -tags=integration -count=1 ./test/...

## lint: Run golangci-lint (auto-installs if missing)
lint:
	@command -v golangci-lint >/dev/null 2>&1 || { echo "Installing golangci-lint..."; go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.62.2; }
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

## run-cli: Run the CLI client
run-cli:
	$(GO) run ./cmd/feather-cli

## run-tui: Run the terminal UI
run-tui:
	$(GO) run ./cmd/feather-tui

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

## validate-config: Validate a config file without starting the server
validate-config: build
	@./bin/feather -config $(CONFIG) -validate
CONFIG ?= configs/feather.yaml

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

## docs: Start the documentation site locally (requires Node.js)
docs:
	cd website && npm install --silent && npm start
## dev: Format, vet, test, and build
dev: fmt vet test build

## quickstart: Build, start, seed, and verify (auto-detects Docker vs source)
quickstart:
	@if command -v docker >/dev/null 2>&1 && timeout 5 docker info >/dev/null 2>&1; then \
		echo "Docker detected — using container quickstart."; \
		./scripts/quickstart.sh; \
	elif command -v go >/dev/null 2>&1; then \
		echo "Go detected — building from source."; \
		./scripts/quickstart-local.sh; \
	else \
		echo "Neither Docker nor Go found." >&2; \
		echo "Install one of:" >&2; \
		echo "  • Go 1.24+: https://golang.org/dl/" >&2; \
		echo "  • Docker:   https://docs.docker.com/get-docker/" >&2; \
		exit 1; \
	fi

## quickstart-docker: Run the prebuilt Docker quickstart
quickstart-docker:
	./scripts/quickstart.sh

## quickstart-local: Build, start, seed, and verify — one command from source
quickstart-local:
	./scripts/quickstart-local.sh

## demo: Seed demo data against a running server
demo:
	./scripts/seed-demo.sh

## doctor: Validate local prerequisites
doctor:
	./scripts/doctor.sh

## smoke-test: Run end-to-end smoke tests against a running server
smoke-test:
	./scripts/smoke-test.sh

## dev-start: Start server in background (use dev-stop to stop)
dev-start: build
	@if [ -f .feather.pid ] && kill -0 $$(cat .feather.pid) 2>/dev/null; then \
		echo "Feather is already running (PID $$(cat .feather.pid))"; \
		exit 0; \
	fi
	@./bin/feather -config configs/feather-dev.yaml > /dev/null 2>&1 & echo $$! > .feather.pid
	@printf "Waiting for server..."
	@for i in $$(seq 1 30); do \
		curl -sf http://localhost:8080/health >/dev/null 2>&1 && break; \
		sleep 0.5; printf "."; \
	done
	@echo ""
	@if curl -sf http://localhost:8080/health >/dev/null 2>&1; then \
		echo "✅ Feather running in background (PID $$(cat .feather.pid))"; \
		echo "   Stop: make dev-stop"; \
	else \
		echo "❌ Server failed to start. Check logs."; \
		rm -f .feather.pid; \
		exit 1; \
	fi

## dev-stop: Stop a background dev server started by dev-start
dev-stop:
	@if [ -f .feather.pid ]; then \
		kill $$(cat .feather.pid) 2>/dev/null && echo "Feather stopped." || echo "Process not running."; \
		rm -f .feather.pid; \
	else \
		echo "No .feather.pid file found. Is Feather running?"; \
	fi

## stop-dev: Stop a running dev server started by quickstart-local
stop-dev:
	@if [ -f .feather.pid ]; then \
		kill $$(cat .feather.pid) 2>/dev/null && echo "Feather stopped." || echo "Process not running."; \
		rm -f .feather.pid; \
	else \
		echo "No .feather.pid file found. Is Feather running?"; \
	fi

## explore: Walk through all API operations interactively (requires running server)
explore:
	./scripts/explore.sh

## examples: Run all examples (requires running server: make run-dev)
examples:
	@echo "Running Python examples..."
	python3 examples/ml-pipeline.py
	python3 examples/fraud-detection.py
	@echo ""
	@echo "Running Go example..."
	cd examples/go-basic && go run main.go
	@echo ""
	@echo "✅ All examples completed."

# Developer Experience
## api-routes: List all registered API handlers with maturity levels
api-routes:
	@echo "Registered API handlers (from internal/core/server/features.go):"
	@echo ""
	@echo "STABLE (production-ready):"
	@grep 'registerHandler(' internal/core/server/features.go | grep 'MaturityStable' | sed 's/.*registerHandler("\([^"]*\)".*/  \1/' | sort
	@echo ""
	@echo "BETA (functional, API may change):"
	@grep 'registerHandler(' internal/core/server/features.go | grep 'MaturityBeta' | sed 's/.*registerHandler("\([^"]*\)".*/  \1/' | sort
	@echo ""
	@echo "EXPERIMENTAL (may be incomplete):"
	@grep 'registerHandler(' internal/core/server/features.go | grep 'MaturityExperimental' | sed 's/.*registerHandler("\([^"]*\)".*/  \1/' | sort
	@echo ""
	@echo "OpenAPI spec: api/openapi/feather.yaml"
	@echo "Handler registry: internal/core/server/features.go"

## list-extensions: Show enabled features vs available features
list-extensions:
	@echo "ENABLED features (in cmd/feather/main.go EnabledFeatures map):"
	@grep -E '"[a-z_]+":[[:space:]]*true' cmd/feather/main.go | sed 's/.*"\([a-z_]*\)".*/  ✅ \1/' | sort
	@echo ""
	@echo "CONDITIONALLY ENABLED:"
	@grep -E '"[a-z_]+":[[:space:]]*cfg\.' cmd/feather/main.go | sed 's/.*"\([a-z_]*\)".*/  ⚙️  \1/' | sort
	@echo ""
	@TOTAL=$$(grep -c 'registerHandler(' internal/core/server/features.go); \
	ENABLED=$$(grep -cE '"[a-z_]+":[[:space:]]*true' cmd/feather/main.go); \
	echo "Total available: $$TOTAL | Enabled by default: $$ENABLED"

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

## check-quick: Fast pre-commit checks (format, vet, lint, core tests — ~20s)
check-quick: fmt-check vet lint test-core
