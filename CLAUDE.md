# CLAUDE.md - Feather Feature Store

This file provides context for Claude Code when working with the Feather codebase.

## Project Overview

Feather is a high-performance, real-time feature store written in Go. It provides sub-millisecond feature retrieval through a tiered storage architecture (hot/warm), real-time aggregations, and multiple serving APIs (gRPC/HTTP).

## Build & Test Commands

```bash
# Build the binary
make build

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

# Run all checks (fmt, vet, lint, test)
make check

# Build and run
make run

# Run with config file
make run-config

# Run benchmarks
make bench

# Docker build and run
make docker-build
make docker-run
```

## Project Structure

```
feather/
├── cmd/feather/          # Application entrypoint
│   └── main.go           # Server initialization, graceful shutdown
├── internal/
│   ├── aggregation/      # Real-time aggregation engine (count, sum, avg, min, max)
│   ├── config/           # YAML/env configuration loading and validation
│   ├── domain/           # Domain types (FeatureValue, FeatureGroup, errors)
│   ├── export/           # Training data export (CSV, JSON, JSONL, Parquet)
│   ├── ingestion/        # Data ingestion (Kafka consumer, HTTP push)
│   ├── logging/          # Structured logging with slog
│   ├── metrics/          # Prometheus metrics
│   ├── server/           # HTTP and gRPC servers, health checks
│   ├── storage/          # Tiered storage (hot=memory, warm=BadgerDB)
│   └── tracing/          # OpenTelemetry tracing
├── test/                 # Integration and benchmark tests
├── configs/              # Example configuration files
├── api/                  # API definitions (proto files)
└── Dockerfile            # Multi-stage Docker build
```

## Key Packages

### `internal/storage`
- **Store**: Unified interface to hot and warm tiers
- **HotTier**: LRU-based in-memory cache with TTL support
- **WarmTier**: BadgerDB-backed persistent storage with historical versions
- **Registry**: Schema registry for feature groups and validation

### `internal/server`
- **HTTPServer**: REST API at `/v1/features`, `/v1/schema/groups`, health endpoints
- **GRPCServer**: gRPC serving with streaming support
- **HealthChecker**: Deep health checks for Kubernetes probes

### `internal/ingestion`
- **KafkaConsumer**: Kafka consumer with circuit breaker pattern
- **HTTPIngestion**: HTTP push endpoint with rate limiting

### `internal/aggregation`
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

Key environment variables:
- `FEATHER_HTTP_PORT` (default: 8080)
- `FEATHER_GRPC_PORT` (default: 50051)
- `FEATHER_HOT_MAX_MEMORY` (default: 4GB)
- `FEATHER_WARM_PATH` (default: /var/lib/feather/data)
- `FEATHER_KAFKA_ENABLED` (default: false)
- `FEATHER_TRACING_ENABLED` (default: false)

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

## Common Tasks

### Adding a New Feature Type
1. Add to `domain.DataType` enum in `internal/domain/types.go`
2. Update `storage.Registry.Validate()` for type validation
3. Add serialization in `storage.WarmTier` if needed

### Adding a New API Endpoint
1. Add handler method to `HTTPServer` in `internal/server/http.go`
2. Register route in `registerRoutes()`
3. Add metrics recording in the handler

### Adding a New Aggregation Function
1. Add to `domain.AggFunction` in `internal/domain/types.go`
2. Implement in `aggregation.Engine.Compute()`

## Dependencies

Key external dependencies:
- `github.com/dgraph-io/badger/v4` - Warm tier storage
- `github.com/confluentinc/confluent-kafka-go/v2` - Kafka consumer
- `github.com/prometheus/client_golang` - Metrics
- `google.golang.org/grpc` - gRPC server
- `go.opentelemetry.io/otel` - Distributed tracing
- `github.com/xitongsys/parquet-go` - Parquet export
