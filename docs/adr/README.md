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

## Reading Order

For new team members, we recommend reading the ADRs in this order to understand how the system evolved:

### Foundation (Start Here)
1. **ADR-0001**: Tiered Storage - The core architectural pattern
2. **ADR-0002**: Sharded Cache - How we achieve sub-millisecond latency
3. **ADR-0003**: BadgerDB - Why we chose this for persistence

### Data Flow
4. **ADR-0005**: Dual-Path Ingestion - How data enters the system
5. **ADR-0004**: Dual Protocol API - How data is served
6. **ADR-0006**: Sliding Window Aggregation - Real-time computations

### Code Organization
7. **ADR-0007**: Modular Architecture - How the codebase is structured
8. **ADR-0008**: Pluggable Handlers - How features are selectively enabled

### Operations
9. **ADR-0009**: Structured Logging - Observability foundation
10. **ADR-0010**: OpenTelemetry - Distributed tracing
11. **ADR-0012**: Graceful Shutdown - Production reliability

### Ecosystem
12. **ADR-0011**: SDK Generation - Multi-language client support

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
