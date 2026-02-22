# CLAUDE.md - Feather Feature Store

This file provides context for Claude Code when working with the Feather codebase.

## Project Overview

Feather is a high-performance, real-time feature store written in Go. It provides sub-millisecond feature retrieval through a tiered storage architecture (hot/warm), real-time aggregations, and multiple serving APIs (gRPC/HTTP).

## Build & Test Commands

```bash
# Install development tools (golangci-lint, goimports)
make install-tools

# Build the binary
make build

# Run quick tests (fast feedback, ~30s)
make test-quick

# Run tests with race detector
make test

# Run short tests only
make test-short

# Run with coverage
make test-coverage

# Lint code
make lint

# Format code
make fmt

# Check formatting without modifying files
make fmt-check

# Run all checks (fmt-check, vet, lint, test)
make check

# Build and run
make run

# Run with minimal dev config (no external deps)
make run-dev

# Run with config file
make run-config

# Run benchmarks
make bench

# Docker build and run
make docker-build
make docker-run

# See all targets
make help

# List all API handlers with maturity levels
make api-routes

# Show enabled vs available features
make list-extensions
```

## Project Structure

```
feather/
├── cmd/feather/          # Application entrypoint
│   └── main.go           # Server initialization, graceful shutdown
├── internal/
│   ├── core/             # Essential packages
│   │   ├── aggregation/  # Real-time aggregation engine (count, sum, avg, min, max)
│   │   ├── config/       # YAML/env configuration loading and validation
│   │   ├── domain/       # Domain types (FeatureValue, FeatureGroup, errors)
│   │   ├── export/       # Training data export (CSV, JSON, JSONL, Parquet)
│   │   ├── ingestion/    # Data ingestion (Kafka consumer, HTTP push)
│   │   ├── logging/      # Structured logging with slog
│   │   ├── metrics/      # Prometheus metrics
│   │   ├── server/       # HTTP and gRPC servers, health checks
│   │   ├── storage/      # Tiered storage (hot=memory, warm=BadgerDB)
│   │   ├── tracing/      # OpenTelemetry tracing
│   │   └── vector/       # Vector similarity search (HNSW)
│   ├── extensions/       # Optional feature modules
│   │   ├── sharding/     # Distributed sharding with consistent hashing
│   │   ├── marketplace/  # Feature marketplace (publish, discover, subscribe)
│   │   ├── featherql/    # SQL-like DSL for declarative feature pipelines
│   │   ├── llmcache/     # Semantic LLM prompt/response caching
│   │   ├── autofe/       # Automated feature engineering
│   │   ├── georouting/   # Multi-cloud geo-routing with data residency
│   │   ├── abrollout/    # Feature versioning with A/B canary rollouts
│   │   └── edgeruntime/  # Lightweight edge runtime with offline-first sync
│   ├── integrations/     # External system connectors (dbt, Spark, Flink, etc.)
│   ├── platform/         # Cross-cutting concerns (auth, cluster, governance, etc.)
│   └── tools/            # Developer tools (benchmark, dashboard, playground)
├── test/                 # Integration and benchmark tests
├── configs/              # Example configuration files
├── api/                  # API definitions (proto files)
└── Dockerfile            # Multi-stage Docker build
```

## Key Packages

### `internal/core/storage`
- **Store**: Unified interface to hot and warm tiers
- **HotTier**: LRU-based in-memory cache with TTL support
- **WarmTier**: BadgerDB-backed persistent storage with historical versions
- **Registry**: Schema registry for feature groups and validation

### `internal/core/server`
- **HTTPServer**: REST API at `/v1/features`, `/v1/schema/groups`, health endpoints
- **GRPCServer**: gRPC serving with streaming support
- **HealthChecker**: Deep health checks for Kubernetes probes
- **features.go**: Handler factory registry with maturity levels (stable/beta/experimental)
  - Use `registerHandler(name, MaturityLevel, factory)` to add new handlers
  - Run `make api-routes` to see all handlers with their maturity levels

### `internal/core/ingestion`
- **KafkaConsumer**: Kafka consumer with circuit breaker pattern
- **HTTPIngestion**: HTTP push endpoint with rate limiting

### `internal/core/aggregation`
- **Engine**: Sliding window aggregations (count, sum, avg, min, max)
- Window-based computation with configurable slide intervals

## Coding Conventions

### Go Idioms
- Use `context.Context` for cancellation and request-scoped values
- Return errors, don't panic (except for programmer errors)
- Use `fmt.Errorf("doing X: %w", err)` for error wrapping
- Interfaces are defined where they're used, not where implemented

### Naming
- Use `internal/` for private packages
- Package names are singular and lowercase
- Avoid stutter: `storage.Store` not `storage.StorageStore`

### Error Handling
- Use `domain.ErrNotFound` for not-found errors
- Check with `domain.IsNotFound(err)` for type checking
- Wrap errors with context about the operation

### Testing
- Tests are colocated with code (`foo_test.go` next to `foo.go`)
- Integration tests are in `test/` directory
- Use table-driven tests for multiple cases
- Run with `-race` flag for race detection

## Configuration

The application can be configured via:
1. **YAML file**: `./feather -config configs/feather.yaml`
2. **Environment variables**: `FEATHER_*` prefix

Key environment variables (6 of 51 — see `docs/configuration.md` for the full reference):
- `FEATHER_HTTP_PORT` (default: 8080)
- `FEATHER_GRPC_PORT` (default: 50051)
- `FEATHER_HOT_MAX_MEMORY` (default: 4GB)
- `FEATHER_WARM_PATH` (default: /var/lib/feather/data)
- `FEATHER_KAFKA_ENABLED` (default: false)
- `FEATHER_TRACING_ENABLED` (default: false)

All 51 environment variables are defined in `internal/core/config/config.go` (`LoadFromEnv` function)
and documented in `docs/configuration.md`. Additional extension env vars are read via `os.Getenv`
in their respective packages:
- `FEATHER_MESH_ADVERTISE_ADDR` — mesh cluster membership
- `FEATHER_STARLARK_SIDECAR_ADDR` — Starlark UDF sidecar
- `FEATHER_PYTHON_WORKER_ENDPOINT` — Python worker process
- `FEATHER_FLINK_JOBMANAGER_ADDR` — Apache Flink integration
- `FEATHER_E2E_URL` — end-to-end test target URL

## API Endpoints

### HTTP REST API (port 8080)
- `GET /v1/features?entity=X&feature=Y` - Get features
- `POST /v1/features` - Store features
- `POST /v1/features/batch` - Batch get
- `GET /v1/features/history?entity=X&as_of=T` - Point-in-time
- `GET /v1/schema/groups` - List feature groups
- `GET /health` - Deep health check
- `GET /ready` - Readiness probe
- `GET /live` - Liveness probe

### Vector Similarity Search API (port 8080)
- `GET /v1/vectors` - List all vector indexes
- `POST /v1/vectors` - Create a new vector index
- `GET /v1/vectors/{index}` - Get index info
- `DELETE /v1/vectors/{index}` - Delete an index
- `POST /v1/vectors/{index}/upsert` - Upsert vectors
- `POST /v1/vectors/{index}/search` - Search for similar vectors
- `GET /v1/vectors/{index}/{id}` - Get a vector by ID
- `DELETE /v1/vectors/{index}/{id}` - Delete a vector

### HTTP Ingestion API (port 8081)
- `POST /ingest` - Single feature update
- `POST /ingest/bulk` - Bulk updates

### gRPC API (port 50051)
- `GetFeatures` - Retrieve features
- `PutFeatures` - Store features
- `GetFeaturesAsOf` - Point-in-time retrieval

### Drift Detection API (port 8080)
- `GET /v1/drift/status` - Get drift monitoring status for all features
- `GET /v1/drift/alerts` - Get drift alerts (query param: since=RFC3339)
- `POST /v1/drift/register` - Register a feature for drift monitoring
- `POST /v1/drift/reset/{feature}` - Reset reference distribution

## Production Features

- **Structured Logging**: slog with JSON/text formats
- **Graceful Shutdown**: 30s timeout for all servers
- **Health Checks**: Component-level health for K8s
- **Rate Limiting**: Token bucket per-client IP
- **Circuit Breaker**: Kafka resilience with half-open recovery
- **HTTP Compression**: gzip for API responses
- **Request Tracing**: X-Request-ID propagation
- **OpenTelemetry**: OTLP export for distributed tracing
- **Prometheus Metrics**: Full observability (port 9090)

### Next-Gen Feature APIs (port 8080)

#### Sharding & Replication
- `GET /v1/sharding/stats` - Get shard routing statistics
- `GET /v1/sharding/partition?key=X` - Get partition for a key
- `GET /v1/sharding/owners?key=X` - Get replica owners for a key
- `POST /v1/sharding/recompute` - Recompute partition map

#### Feature Marketplace
- `GET /v1/marketplace/features` - List published features
- `POST /v1/marketplace/features` - Publish a feature
- `GET /v1/marketplace/features/{id}` - Get feature details
- `POST /v1/marketplace/features/{id}/subscribe` - Subscribe to a feature
- `POST /v1/marketplace/features/{id}/deprecate` - Deprecate a feature
- `GET /v1/marketplace/search` - Search marketplace
- `GET /v1/marketplace/stats` - Marketplace statistics

#### Cloud Service
- `GET /v1/cloud/instances` - List managed instances
- `POST /v1/cloud/instances` - Provision a new instance
- `POST /v1/cloud/instances/{id}/scale` - Scale an instance
- `DELETE /v1/cloud/instances/{id}` - Terminate an instance

#### FeatherQL (Declarative Pipelines)
- `POST /v1/featherql/parse` - Parse a FeatherQL query
- `POST /v1/featherql/compile` - Compile a FeatherQL pipeline
- `POST /v1/featherql/execute` - Execute a FeatherQL query
- `GET /v1/featherql/pipelines` - List compiled pipelines

#### LLM Cache
- `POST /v1/llm/cache/lookup` - Lookup cached LLM response
- `POST /v1/llm/cache/store` - Store LLM response
- `GET /v1/llm/cache/stats` - Cache hit/miss statistics
- `GET /v1/llm/cache/costs` - Cost savings by provider

#### AutoFE (Automated Feature Engineering)
- `POST /v1/autofe/generate` - Generate candidate features
- `GET /v1/autofe/candidates/top` - Get top candidates by score

#### Geo-Routing
- `GET /v1/georouting/regions` - List registered regions
- `POST /v1/georouting/regions` - Add a cloud region
- `GET /v1/georouting/route?entity=X` - Route request to best region

#### A/B Rollout
- `GET /v1/rollouts` - List rollouts
- `POST /v1/rollouts` - Start a canary rollout
- `POST /v1/rollouts/{id}/advance` - Advance to next traffic step
- `POST /v1/rollouts/{id}/rollback` - Rollback to base version
- `GET /v1/rollouts/{id}/quality` - Evaluate quality gates
- `GET /v1/rollouts/resolve?feature=X&entity=Y` - Resolve version for entity

#### Edge Runtime
- `GET /v1/edge/devices` - List edge devices
- `POST /v1/edge/devices/{id}/sync` - Trigger sync for device
- `GET /v1/edge/devices/{id}/stats` - Get device statistics

## Common Tasks

### Adding a New API Endpoint
1. Create package in `internal/extensions/`, `internal/integrations/`, or `internal/platform/`
2. Create handler in `internal/core/server/` implementing `FeatureHandler` interface
3. Register with `registerHandler("name", MaturityLevel, factory)` in `features.go` `init()`
4. Enable in `cmd/feather/main.go` `EnabledFeatures` map
5. Add to `docs/package-guide.md`

### Adding a New Feature Type
1. Add to `domain.DataType` enum in `internal/core/domain/types.go`
2. Update `storage.Registry.Validate()` for type validation
3. Add serialization in `storage.WarmTier` if needed

### Adding a New API Endpoint
1. Add handler method to `HTTPServer` in `internal/core/server/http.go`
2. Register route in `registerRoutes()`
3. Add metrics recording in the handler

### Adding a New Aggregation Function
1. Add to `domain.AggFunction` in `internal/core/domain/types.go`
2. Implement in `aggregation.Engine.Compute()`

## Dependencies

Key external dependencies:
- `github.com/dgraph-io/badger/v4` - Warm tier storage
- `github.com/confluentinc/confluent-kafka-go/v2` - Kafka consumer
- `github.com/prometheus/client_golang` - Metrics
- `google.golang.org/grpc` - gRPC server
- `go.opentelemetry.io/otel` - Distributed tracing
- `github.com/xitongsys/parquet-go` - Parquet export
