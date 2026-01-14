# Changelog

All notable changes to Feather will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- **Developer Experience**
  - `make help` target listing all available Make commands
  - `make install-tools` for one-command dev tool setup (golangci-lint, goimports)
  - `make test-quick` for fast feedback during development (~30s)
  - `make run-dev` with minimal in-memory config (no external dependencies)
  - `make fmt-check` for non-destructive format validation in `make check`
  - `configs/feather-dev.yaml` — zero-dependency config for local development
  - `docs/package-guide.md` — maturity matrix for all 82 internal packages
  - Build tags on integration tests (`//go:build integration`)
  - Documentation for CLI tools (feather-cli, feather-tui, feather-mcp)
  - Codecov badge in README

### Fixed

- Go version badge in README now shows `>=1.24` (was `>=1.22`)
- Go version in `website/docs/contributing.md` now shows `1.24` (was `1.21`)
- `docs/contributing.md` license reference corrected to Apache 2.0 (was MIT)
- `configs/feather.yaml` now ships with `kafka.enabled: false` for out-of-box use
- `make check` no longer silently modifies source files (uses format check instead)
- `.gitignore` now includes all binary names (feather-cli, feather-mcp)

## [1.0.0] - 2024-01-15

### Added

- **Core Feature Store**
  - Tiered storage architecture with hot (in-memory LRU) and warm (BadgerDB) tiers
  - Sub-millisecond feature retrieval (<1ms P99 for hot tier)
  - Point-in-time queries for historical feature values
  - Real-time aggregations (count, sum, avg, min, max) with sliding windows

- **APIs**
  - HTTP REST API for feature serving and management
  - gRPC API with streaming support
  - HTTP ingestion API with rate limiting
  - Vector similarity search API (HNSW index)

- **Ingestion**
  - Kafka consumer with circuit breaker pattern
  - HTTP push endpoint with batch support
  - CSV and JSONL batch import

- **Security**
  - API key authentication with SHA256 hashing
  - Role-based access control (RBAC)
  - Per-client rate limiting
  - TLS support for all servers
  - Security headers (CSP, HSTS, X-Frame-Options)

- **Observability**
  - Prometheus metrics endpoint
  - OpenTelemetry distributed tracing
  - Structured logging with slog
  - Health check endpoints for Kubernetes

- **Operations**
  - Graceful shutdown with configurable timeout
  - Docker image with non-root user
  - Kubernetes manifests and Helm chart
  - Configuration via YAML and environment variables

- **Extended Features**
  - Drift detection and monitoring
  - Feature lineage tracking
  - Semantic search for features
  - Federation for distributed deployments
  - WASM runtime for custom transformations
  - GraphQL API
  - Feature catalog and registry

### Security

- Request body size limits to prevent DoS
- IP spoofing protection with trusted proxy validation
- Panic recovery middleware
- Path traversal protection for vector indexes
