# Changelog

All notable changes to Feather will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

<!-- Link format: [#N](https://github.com/feather-store/feather/pull/N) for PRs,
     [#N](https://github.com/feather-store/feather/issues/N) for issues -->

## [Unreleased]

### Added

- **Developer Experience**
  - `make help` target listing all available Make commands ([#52](https://github.com/feather-store/feather/pull/52))
  - `make install-tools` for one-command dev tool setup (golangci-lint, goimports) ([#53](https://github.com/feather-store/feather/pull/53))
  - `make test-quick` for fast feedback during development (~30s) ([#54](https://github.com/feather-store/feather/pull/54))
  - `make run-dev` with minimal in-memory config (no external dependencies) ([#55](https://github.com/feather-store/feather/pull/55))
  - `make fmt-check` for non-destructive format validation in `make check` ([#56](https://github.com/feather-store/feather/pull/56))
  - `configs/feather-dev.yaml` — zero-dependency config for local development ([#55](https://github.com/feather-store/feather/pull/55))
  - `docs/package-guide.md` — maturity matrix for all 82 internal packages ([#57](https://github.com/feather-store/feather/pull/57))
  - Build tags on integration tests (`//go:build integration`) ([#58](https://github.com/feather-store/feather/pull/58))
  - Documentation for CLI tools (feather-cli, feather-tui, feather-mcp) ([#59](https://github.com/feather-store/feather/pull/59))
  - Codecov badge in README ([#60](https://github.com/feather-store/feather/pull/60))

### Fixed

- Go version badge in README now shows `>=1.24` (was `>=1.22`) ([#45](https://github.com/feather-store/feather/pull/45))
- Go version in `website/docs/contributing.md` now shows `1.24` (was `1.21`) ([#45](https://github.com/feather-store/feather/pull/45))
- `docs/contributing.md` license reference corrected to Apache 2.0 (was MIT) ([#46](https://github.com/feather-store/feather/pull/46))
- `configs/feather.yaml` now ships with `kafka.enabled: false` for out-of-box use ([#47](https://github.com/feather-store/feather/pull/47))
- `make check` no longer silently modifies source files (uses format check instead) ([#48](https://github.com/feather-store/feather/pull/48))
- `.gitignore` now includes all binary names (feather-cli, feather-mcp) ([#49](https://github.com/feather-store/feather/pull/49))

## [1.0.0] - 2024-01-15

### Added

- **Core Feature Store** ([#1](https://github.com/feather-store/feather/pull/1))
  - Tiered storage architecture with hot (in-memory LRU) and warm (BadgerDB) tiers
  - Sub-millisecond feature retrieval (<1ms P99 for hot tier)
  - Point-in-time queries for historical feature values
  - Real-time aggregations (count, sum, avg, min, max) with sliding windows

- **APIs** ([#2](https://github.com/feather-store/feather/pull/2), [#5](https://github.com/feather-store/feather/pull/5))
  - HTTP REST API for feature serving and management
  - gRPC API with streaming support
  - HTTP ingestion API with rate limiting
  - Vector similarity search API (HNSW index)

- **Ingestion** ([#3](https://github.com/feather-store/feather/pull/3))
  - Kafka consumer with circuit breaker pattern
  - HTTP push endpoint with batch support
  - CSV and JSONL batch import

- **Security** ([#10](https://github.com/feather-store/feather/pull/10), [#12](https://github.com/feather-store/feather/pull/12))
  - API key authentication with SHA256 hashing
  - Role-based access control (RBAC)
  - Per-client rate limiting
  - TLS support for all servers
  - Security headers (CSP, HSTS, X-Frame-Options)

- **Observability** ([#15](https://github.com/feather-store/feather/pull/15))
  - Prometheus metrics endpoint
  - OpenTelemetry distributed tracing
  - Structured logging with slog
  - Health check endpoints for Kubernetes

- **Operations** ([#20](https://github.com/feather-store/feather/pull/20))
  - Graceful shutdown with configurable timeout
  - Docker image with non-root user
  - Kubernetes manifests and Helm chart
  - Configuration via YAML and environment variables

- **Extended Features** ([#25](https://github.com/feather-store/feather/pull/25), [#30](https://github.com/feather-store/feather/pull/30), [#35](https://github.com/feather-store/feather/pull/35))
  - Drift detection and monitoring
  - Feature lineage tracking
  - Semantic search for features
  - Federation for distributed deployments
  - WASM runtime for custom transformations
  - GraphQL API
  - Feature catalog and registry

### Security

- Request body size limits to prevent DoS ([#10](https://github.com/feather-store/feather/pull/10))
- IP spoofing protection with trusted proxy validation ([#12](https://github.com/feather-store/feather/pull/12))
- Panic recovery middleware ([#10](https://github.com/feather-store/feather/pull/10))
- Path traversal protection for vector indexes ([#40](https://github.com/feather-store/feather/pull/40))
