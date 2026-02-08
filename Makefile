.PHONY: build test test-quick test-short test-core test-one test-pkg lint lint-fix run run-config run-dev run-cli run-tui clean clean-all generate tidy fmt fmt-check vet validate-config proto help install-tools setup quickstart quickstart-docker quickstart-local demo doctor smoke-test dev-start dev-stop stop-dev check-quick explore examples docs api-routes list-extensions watch verify test-watch hook-check profile-cpu profile-mem deps-check test-changed lint-config changelog

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

### Setup

## help: Show this help message
help:
	@echo "Usage: make [target]"
	@echo ""
	@sed -n -e 's/^### \(.*\)/\n\1:/p' -e 's/^## //p' $(MAKEFILE_LIST) | column -t -s ':' | sed 's/^/  /'

## install-tools: Install development tools (golangci-lint, goimports)
install-tools:
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.62.2
	go install golang.org/x/tools/cmd/goimports@latest
	@echo "Development tools installed successfully."

## setup: One-command contributor setup (doctor, tools, hooks, build, test)
setup: doctor install-tools
	@git config core.hooksPath .githooks
	@echo "Git hooks configured (.githooks/pre-commit)."
	@if [ ! -f .env ] && [ -f .env.example ]; then \
		cp .env.example .env; \
		echo "Copied .env.example → .env (edit as needed; YAML config is preferred)."; \
	fi
	@$(MAKE) build
	@$(MAKE) test-core
	@echo ""
	@echo "✅ Setup complete! You're ready to contribute."
	@echo "   Run 'make run-dev' to start the server."
	@echo "   Run 'make check-quick' before committing."

### Building

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

### Testing

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
	@echo "Coverage report: coverage.html"
	@if command -v open >/dev/null 2>&1; then open coverage.html; \
	elif command -v xdg-open >/dev/null 2>&1; then xdg-open coverage.html; \
	else echo "Open coverage.html in your browser to view the report."; fi

## test-integration: Run integration tests (requires Docker for some tests)
test-integration:
	$(GO) test -v -tags=integration -count=1 ./test/...

## test-one: Run a single test (usage: make test-one RUN=TestFoo [PKG=./internal/core/storage/...])
test-one:
	$(GO) test -v -count=1 -run $(RUN) -timeout 120s $(PKG)
RUN ?= .
PKG ?= ./...

## test-pkg: Run all tests in a package with verbose output and coverage (usage: make test-pkg PKG=./internal/core/storage/...)
test-pkg:
	$(GO) test -v -count=1 -cover -timeout 120s $(TEST_PKG)
TEST_PKG ?= ./internal/core/...

## test-watch: Re-run tests on file changes (TDD workflow; requires fswatch or entr)
test-watch:
	@if command -v fswatch >/dev/null 2>&1; then \
		echo "Watching for changes (fswatch)... Ctrl-C to stop."; \
		fswatch -o --include '\.go$$' --exclude '.*' . | xargs -n1 -I{} $(MAKE) test-quick; \
	elif command -v entr >/dev/null 2>&1; then \
		echo "Watching for changes (entr)... Ctrl-C to stop."; \
		find . -name '*.go' | entr -c $(MAKE) test-quick; \
	else \
		echo "❌ Neither fswatch nor entr found. Install one:"; \
		echo "   macOS:  brew install fswatch"; \
		echo "   Linux:  apt install entr  (or)  brew install entr"; \
		exit 1; \
	fi

## test-changed: Run tests only for packages with uncommitted changes
test-changed:
	@PKGS=$$(git diff --name-only HEAD -- '*.go' | xargs -I{} dirname {} | sort -u | sed 's|^|./|' | grep -v '^\./$$' || true); \
	if [ -z "$$PKGS" ]; then \
		echo "No changed Go packages detected."; \
	else \
		echo "Testing changed packages:"; \
		echo "$$PKGS" | sed 's/^/  /'; \
		echo ""; \
		$(GO) test -v -count=1 -timeout 120s $$PKGS; \
	fi

### Code Quality

## lint: Run golangci-lint (auto-installs if missing)
lint:
	@command -v $(shell go env GOPATH)/bin/golangci-lint >/dev/null 2>&1 || { echo "Installing golangci-lint..."; go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.62.2; }
	$(shell go env GOPATH)/bin/golangci-lint run ./...

## lint-fix: Run golangci-lint with auto-fix
lint-fix:
	@command -v $(shell go env GOPATH)/bin/golangci-lint >/dev/null 2>&1 || { echo "Installing golangci-lint..."; go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.62.2; }
	$(shell go env GOPATH)/bin/golangci-lint run --fix ./...

### Development

## run: Run the server with default configuration
run:
	$(GO) run $(MAIN_PATH)

## run-config: Run the server with the example config file
run-config:
	$(GO) run $(MAIN_PATH) -config configs/feather.yaml

## run-dev: Run the server with the minimal dev config (then: make smoke-test to verify)
run-dev: validate-dev-config
	$(GO) run $(MAIN_PATH) -config configs/feather-dev.yaml

# Quick config validation for dev workflow (doesn't require a full build)
validate-dev-config:
	@$(GO) run $(MAIN_PATH) -config configs/feather-dev.yaml -validate 2>&1 || \
		{ echo "❌ Config validation failed. Fix configs/feather-dev.yaml and retry."; exit 1; }

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

## clean-all: Full reset — remove build artifacts, data, logs, and caches
clean-all: clean
	rm -rf data/
	rm -rf tmp/
	rm -f .feather.pid
	rm -f .feather-quickstart.log
	rm -rf website/node_modules website/build website/.docusaurus

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

## lint-config: Validate all YAML config files against the config struct
lint-config: build
	@./scripts/lint-config.sh ./bin/feather configs

## generate: Run all code generation (protobuf, mocks, etc.)
generate: proto
	@echo "✅ All code generation complete."

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
	@command -v node >/dev/null 2>&1 || { echo "❌ Node.js not found. Install it from https://nodejs.org/"; exit 1; }
	@command -v npm >/dev/null 2>&1 || { echo "❌ npm not found. Install Node.js from https://nodejs.org/"; exit 1; }
	cd website && npm install --silent && npm start
## watch: Auto-rebuild and restart on code changes (requires air)
watch:
	@command -v air >/dev/null 2>&1 || { echo "Installing air..."; go install github.com/air-verse/air@latest; }
	air

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

## verify: Alias for smoke-test — verify a running server works end-to-end
verify: smoke-test

## dev-start: Start server in background (use dev-stop to stop)
dev-start: build
	@if [ -f .feather.pid ]; then \
		if kill -0 $$(cat .feather.pid) 2>/dev/null; then \
			echo "Feather is already running (PID $$(cat .feather.pid))"; \
			exit 0; \
		else \
			echo "Removing stale PID file (process $$(cat .feather.pid) no longer running)"; \
			rm -f .feather.pid; \
		fi; \
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
		echo "   Verify: make smoke-test"; \
		echo "   Stop:   make dev-stop"; \
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
	@echo "Registered API handlers (from internal/core/server/features_*.go):"
	@echo ""
	@echo "STABLE (production-ready):"
	@grep 'registerHandler(' internal/core/server/features_*.go | grep 'MaturityStable' | sed 's/.*registerHandler("\([^"]*\)".*/  \1/' | sort
	@echo ""
	@echo "BETA (functional, API may change):"
	@grep 'registerHandler(' internal/core/server/features_*.go | grep 'MaturityBeta' | sed 's/.*registerHandler("\([^"]*\)".*/  \1/' | sort
	@echo ""
	@echo "EXPERIMENTAL (may be incomplete):"
	@grep 'registerHandler(' internal/core/server/features_*.go | grep 'MaturityExperimental' | sed 's/.*registerHandler("\([^"]*\)".*/  \1/' | sort
	@echo ""
	@echo "OpenAPI spec: api/openapi/feather.yaml"
	@echo "Handler registry: internal/core/server/features_*.go"

## list-extensions: Show enabled features vs available features
list-extensions:
	@echo "ENABLED features (in cmd/feather/main.go EnabledFeatures map):"
	@grep -E '"[a-z_]+":[[:space:]]*true' cmd/feather/main.go | sed 's/.*"\([a-z_]*\)".*/  ✅ \1/' | sort
	@echo ""
	@echo "CONDITIONALLY ENABLED:"
	@grep -E '"[a-z_]+":[[:space:]]*cfg\.' cmd/feather/main.go | sed 's/.*"\([a-z_]*\)".*/  ⚙️  \1/' | sort
	@echo ""
	@TOTAL=$$(grep -c 'registerHandler(' internal/core/server/features_*.go); \
	ENABLED=$$(grep -cE '"[a-z_]+":[[:space:]]*true' cmd/feather/main.go); \
	echo "Total available: $$TOTAL | Enabled by default: $$ENABLED"

### Docker

## docker-build: Build the Docker image
docker-build:
	docker build -t $(APP_NAME):latest .

## docker-run: Run the Docker image with default ports
docker-run:
	docker run -p 8080:8080 -p 50051:50051 -p 9090:9090 $(APP_NAME):latest

### Utilities

## bench: Run all benchmarks
bench:
	$(GO) test -bench=. -benchmem ./...

## check: Run all checks (format check, vet, lint, test)
check: fmt-check vet lint test

## check-quick: Fast pre-commit checks (format, vet, lint, core tests — ~20s)
check-quick: hook-check fmt-check vet lint test-core

# Warn if git hooks are not configured (non-blocking)
hook-check:
	@if [ -z "$$(git config core.hooksPath)" ]; then \
		echo "⚠️  Git hooks not configured. Run 'make setup' to enable pre-commit hooks."; \
	fi

### Profiling

PPROF_HOST ?= localhost:8080

## profile-cpu: Capture a 30s CPU profile from the running server
profile-cpu:
	@echo "Collecting 30s CPU profile from $(PPROF_HOST)..."
	$(GO) tool pprof -http=:6060 http://$(PPROF_HOST)/debug/pprof/profile?seconds=30

## profile-mem: Capture a heap memory profile from the running server
profile-mem:
	@echo "Collecting heap profile from $(PPROF_HOST)..."
	$(GO) tool pprof -http=:6060 http://$(PPROF_HOST)/debug/pprof/heap

### Dependencies

## deps-check: Check for outdated or modified Go module dependencies
deps-check:
	@echo "Checking go.mod tidiness..."
	@$(GO) mod tidy -diff 2>&1 || { echo "❌ go.mod is not tidy. Run 'go mod tidy'."; exit 1; }
	@echo "✅ go.mod is tidy."
	@echo ""
	@echo "Checking for available dependency updates..."
	@$(GO) list -m -u -mod=readonly all 2>/dev/null | grep '\[' || echo "All dependencies are up to date."
	@echo ""
	@echo "Checking for known vulnerabilities (requires govulncheck)..."
	@if command -v govulncheck >/dev/null 2>&1; then \
		govulncheck ./...; \
	else \
		echo "⚠️  govulncheck not installed. Install with: go install golang.org/x/vuln/cmd/govulncheck@latest"; \
	fi

### Release

## changelog: Preview upcoming release notes from conventional commits
changelog:
	@LAST_TAG=$$(git describe --tags --abbrev=0 2>/dev/null || echo ""); \
	if [ -z "$$LAST_TAG" ]; then \
		echo "Changelog (all commits):"; \
		RANGE="HEAD"; \
	else \
		echo "Changelog since $$LAST_TAG:"; \
		RANGE="$$LAST_TAG..HEAD"; \
	fi; \
	echo ""; \
	FEATS=$$(git log $$RANGE --pretty=format:'- %s (%h)' --grep='^feat' 2>/dev/null); \
	FIXES=$$(git log $$RANGE --pretty=format:'- %s (%h)' --grep='^fix' 2>/dev/null); \
	DOCS=$$(git log $$RANGE --pretty=format:'- %s (%h)' --grep='^docs' 2>/dev/null); \
	CHORES=$$(git log $$RANGE --pretty=format:'- %s (%h)' --grep='^chore\|^refactor\|^build\|^ci\|^perf\|^test\|^style' 2>/dev/null); \
	if [ -n "$$FEATS" ]; then echo "### 🚀 Features"; echo "$$FEATS"; echo ""; fi; \
	if [ -n "$$FIXES" ]; then echo "### 🐛 Bug Fixes"; echo "$$FIXES"; echo ""; fi; \
	if [ -n "$$DOCS" ]; then echo "### 📚 Documentation"; echo "$$DOCS"; echo ""; fi; \
	if [ -n "$$CHORES" ]; then echo "### 🔧 Maintenance"; echo "$$CHORES"; echo ""; fi; \
	TOTAL=$$(git log $$RANGE --oneline 2>/dev/null | wc -l | tr -d ' '); \
	echo "---"; \
	echo "Total commits: $$TOTAL"
