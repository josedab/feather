# Architecture Decision Records

This directory contains Architecture Decision Records (ADRs) for the Feather Feature Store. ADRs document significant architectural decisions, their context, and consequences.

## Index

| ADR | Title | Status |
|-----|-------|--------|
| [ADR-0001](0001-tiered-storage-architecture.md) | Tiered Storage Architecture (Hot/Warm) | Accepted |
| [ADR-0002](0002-sharded-in-memory-cache.md) | Sharded In-Memory Cache Design | Accepted |
| [ADR-0003](0003-badgerdb-for-persistence.md) | BadgerDB for Embedded Persistence | Accepted |
| [ADR-0004](0004-dual-protocol-api.md) | Dual Protocol API (gRPC + HTTP REST) | Accepted |
| [ADR-0005](0005-dual-path-ingestion.md) | Dual-Path Ingestion (Kafka + HTTP) | Accepted |
| [ADR-0006](0006-sliding-window-aggregation.md) | Sliding Window Aggregation with Ring Buffers | Accepted |
| [ADR-0007](0007-modular-package-architecture.md) | Modular Package Architecture | Accepted |
| [ADR-0008](0008-pluggable-http-handlers.md) | Pluggable HTTP Handler Registration | Accepted |
| [ADR-0009](0009-structured-logging-slog.md) | Structured Logging with slog | Accepted |
| [ADR-0010](0010-opentelemetry-tracing.md) | OpenTelemetry for Distributed Tracing | Accepted |
| [ADR-0011](0011-multi-language-sdk-generation.md) | Multi-Language SDK Generation from Protobuf | Accepted |
| [ADR-0012](0012-graceful-shutdown.md) | Graceful Shutdown with Coordinated Lifecycle | Accepted |
| [ADR-0013](0013-go-implementation-language.md) | Go as Primary Implementation Language | Accepted |
| [ADR-0014](0014-point-in-time-versioned-keys.md) | Point-in-Time Queries via Versioned Keys | Accepted |
| [ADR-0015](0015-hnsw-vector-similarity.md) | HNSW Algorithm for Vector Similarity Search | Accepted |
| [ADR-0016](0016-object-pooling.md) | Object Pooling for High-Throughput Paths | Accepted |
| [ADR-0017](0017-single-binary-deployment.md) | Single-Binary Self-Hosted Deployment Model | Accepted |
| [ADR-0018](0018-prometheus-pull-metrics.md) | Prometheus Pull-Based Metrics | Accepted |

## Reading Order

For new team members, we recommend reading the ADRs in this order to understand how the system evolved:

### Foundation (Start Here)
1. **ADR-0013**: Go Language - Why we chose Go as implementation language
2. **ADR-0017**: Single-Binary Deployment - Our self-hosted deployment philosophy
3. **ADR-0001**: Tiered Storage - The core architectural pattern
4. **ADR-0002**: Sharded Cache - How we achieve sub-millisecond latency
5. **ADR-0003**: BadgerDB - Why we chose this for persistence

### Data Flow
6. **ADR-0005**: Dual-Path Ingestion - How data enters the system
7. **ADR-0004**: Dual Protocol API - How data is served
8. **ADR-0014**: Point-in-Time Queries - How historical data access works
9. **ADR-0006**: Sliding Window Aggregation - Real-time computations

### ML Capabilities
10. **ADR-0015**: HNSW Vector Search - Embedding similarity search

### Performance
11. **ADR-0016**: Object Pooling - High-throughput optimizations

### Code Organization
12. **ADR-0007**: Modular Architecture - How the codebase is structured
13. **ADR-0008**: Pluggable Handlers - How features are selectively enabled

### Operations & Observability
14. **ADR-0009**: Structured Logging - Observability foundation
15. **ADR-0010**: OpenTelemetry - Distributed tracing
16. **ADR-0018**: Prometheus Metrics - Pull-based instrumentation
17. **ADR-0012**: Graceful Shutdown - Production reliability

### Ecosystem
18. **ADR-0011**: SDK Generation - Multi-language client support

## ADR Format

Each ADR follows this structure:

- **Title**: Short descriptive name
- **Status**: Accepted, Superseded, or Deprecated
- **Context**: What prompted this decision?
- **Decision**: What was decided?
- **Consequences**: Tradeoffs, what this enabled/prevented

## Contributing

When making significant architectural changes:

1. Create a new ADR file: `NNNN-short-title.md`
2. Follow the format above
3. Update this index
4. Get team review before implementing

ADRs are immutable once accepted. If a decision is reversed, create a new ADR that supersedes the old one.
