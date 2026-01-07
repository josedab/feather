---
sidebar_position: 12
title: Changelog
description: Release history and changelog for Feather feature store.
---

# Changelog

All notable changes to Feather are documented here. This project follows [Semantic Versioning](https://semver.org/).

---

## [Unreleased]

### Added
- AI-powered feature discovery with semantic search
- Natural language queries for feature exploration
- Personalized feature recommendations

### Changed
- Improved hot tier memory efficiency by 15%

### Fixed
- Race condition in aggregation window cleanup

---

## [1.2.0] - 2024-12-15

### Added
- **Feature Freshness SLAs**: Define and enforce freshness requirements for features
  - Adaptive TTL management based on access patterns
  - ML-driven freshness predictions
  - Automatic alerting when SLAs are breached
  - Auto-remediation workflows

- **Drift Detection**: Statistical monitoring for feature drift
  - KL divergence, JS divergence, and PSI metrics
  - Configurable alert thresholds
  - Reference distribution management
  - `/v1/drift/status` and `/v1/drift/alerts` endpoints

- **Offline Sync Connectors**:
  - Apache Spark connector for batch export
  - Apache Flink connector for streaming export
  - Support for Parquet, CSV, and JSON formats

### Changed
- Upgraded BadgerDB to v4.2.0 for improved performance
- Reduced warm tier compaction memory usage by 30%
- Improved error messages for validation failures

### Fixed
- Memory leak in aggregation engine during window slides
- Incorrect P99 calculation in metrics
- gRPC stream cancellation not releasing resources

---

## [1.1.0] - 2024-10-01

### Added
- **Vector Similarity Search**: Built-in HNSW index for embeddings
  - Support for cosine, Euclidean, and dot product distances
  - Up to 4096 dimensions
  - Sub-5ms search on 1M vectors
  - Metadata filtering

- **Java/Kotlin SDK**: Official client library
  - Coroutine support for Kotlin
  - Reactive streams integration
  - Connection pooling

- **Rust SDK**: Official client library
  - Async/await with Tokio
  - Zero-copy deserialization where possible
  - Full type safety

- **TypeScript SDK**: Official client library
  - Works in Node.js and browsers
  - Full TypeScript definitions
  - Promise-based API

### Changed
- HTTP API now returns structured errors with error codes
- Improved batch endpoint to support up to 1000 entities per request
- Schema registry validates feature names against reserved words

### Fixed
- Hot tier LRU eviction not respecting memory limit under high load
- Point-in-time queries returning incorrect version for edge cases
- Kafka consumer not reconnecting after broker restart

### Security
- Fixed potential timing attack in API key validation
- Added rate limiting to authentication endpoints

---

## [1.0.0] - 2024-07-15

Initial stable release.

### Features
- **Tiered Storage Architecture**
  - Hot tier: 256-shard in-memory LRU cache
  - Warm tier: BadgerDB persistent storage
  - Automatic promotion/demotion between tiers

- **Multiple APIs**
  - HTTP REST API on port 8080
  - gRPC API on port 50051
  - Streaming support for large result sets

- **Real-Time Aggregations**
  - Sliding window aggregations: count, sum, avg, min, max
  - Configurable window sizes and slide intervals
  - Incremental computation

- **Point-in-Time Queries**
  - Historical feature retrieval
  - Versioned storage in warm tier
  - Essential for training data generation

- **Schema Registry**
  - Feature group definitions
  - Type validation
  - TTL per feature group

- **Data Ingestion**
  - Kafka consumer with circuit breaker
  - HTTP push endpoint with rate limiting
  - Batch import from CSV/Parquet

- **Observability**
  - Prometheus metrics on port 9090
  - OpenTelemetry tracing (OTLP export)
  - Structured logging with slog
  - Health probes: `/health`, `/ready`, `/live`

- **Production Features**
  - Graceful shutdown (30s timeout)
  - Request ID propagation
  - gzip compression for HTTP responses
  - TLS support

- **Client SDKs**
  - Go SDK
  - Python SDK

### Performance
- P99 latency: under 1ms for hot tier reads
- Throughput: 1M+ ops/sec on single node
- Memory efficiency: ~100 bytes per feature

---

## [0.9.0] - 2024-05-01

Beta release for early adopters.

### Added
- Core feature storage and retrieval
- Basic HTTP API
- In-memory cache with TTL
- BadgerDB persistence
- Prometheus metrics

### Known Issues
- No point-in-time query support
- Limited to single-threaded ingestion
- No streaming API

---

## Migration Guides

### Upgrading to 1.2.0

No breaking changes. New features are opt-in.

To enable drift detection:
```yaml
drift:
  enabled: true
  default_threshold: 0.1
```

### Upgrading to 1.1.0

**Breaking change**: Error response format changed.

Before:
```json
{"error": "not found"}
```

After:
```json
{"error": {"code": "NOT_FOUND", "message": "Entity not found: user:123"}}
```

Update client code to handle the new format, or upgrade to the latest SDK version which handles this automatically.

### Upgrading to 1.0.0

From 0.9.0, the data directory format changed. Run the migration tool:

```bash
./feather migrate --from 0.9 --data-dir /var/lib/feather/data
```

---

## Release Schedule

Feather follows a time-based release schedule:

- **Patch releases** (1.x.y): As needed for bug fixes
- **Minor releases** (1.x.0): Every 2-3 months with new features
- **Major releases** (x.0.0): When breaking changes are necessary

---

## Links

- [GitHub Releases](https://github.com/feather-store/feather/releases)
- [Deployment Guide](/docs/guides/deployment)
- [Breaking Changes Policy](https://github.com/feather-store/feather/blob/main/CHANGELOG.md)
