---
sidebar_position: 1
title: Architecture Decision Records
description: Documented architectural decisions for the Feather feature store.
---

# Architecture Decision Records

Architecture Decision Records (ADRs) capture the key architectural decisions made during the development of Feather. They provide context for why certain approaches were chosen over alternatives.

## What are ADRs?

ADRs document significant architectural decisions along with their context and consequences. They help:

- **New team members** understand why the system is built the way it is
- **Future maintainers** avoid revisiting settled decisions
- **Stakeholders** understand trade-offs that were considered

## ADR Index

All ADRs are available in the [GitHub repository](https://github.com/feather-store/feather/tree/main/docs/adr). Key ADRs are documented below in this section.

### Core Architecture

| ADR | Title | Status |
|-----|-------|--------|
| [ADR-0001](./tiered-storage) | Tiered Storage Architecture | Accepted |
| ADR-0002 | BadgerDB for Warm Tier | Accepted |
| ADR-0003 | gRPC + HTTP Dual API | Accepted |
| ADR-0004 | Kafka + HTTP Dual Ingestion | Accepted |
| [ADR-0013](./go-implementation-language) | Go as Implementation Language | Accepted |
| [ADR-0014](./point-in-time-versioned-keys) | Versioned Keys for Point-in-Time | Accepted |
| ADR-0017 | Single Binary Deployment | Accepted |

### Performance & Reliability

| ADR | Title | Status |
|-----|-------|--------|
| ADR-0005 | Circuit Breaker for Kafka | Accepted |
| ADR-0006 | Sharded LRU Cache | Accepted |
| ADR-0016 | Object Pooling with sync.Pool | Accepted |

### Data Model

| ADR | Title | Status |
|-----|-------|--------|
| ADR-0007 | Entity-Feature Key Schema | Accepted |
| ADR-0008 | Ring Buffers for Aggregation | Accepted |
| ADR-0015 | HNSW for Vector Similarity | Accepted |

### Export & Integration

| ADR | Title | Status |
|-----|-------|--------|
| ADR-0009 | Parquet as Export Format | Accepted |
| ADR-0010 | YAML Configuration Format | Accepted |

### Observability

| ADR | Title | Status |
|-----|-------|--------|
| ADR-0011 | slog for Structured Logging | Accepted |
| ADR-0012 | OpenTelemetry for Tracing | Accepted |
| ADR-0018 | Prometheus Pull-Based Metrics | Accepted |

## Recommended Reading Order

If you're new to Feather, we recommend reading the ADRs in this order to understand the system's evolution:

1. **ADR-0013**: Go Implementation Language - Understand the foundational technology choice
2. **ADR-0001**: Tiered Storage Architecture - The core architectural pattern
3. **ADR-0002**: BadgerDB for Warm Tier - Why we chose BadgerDB
4. **ADR-0006**: Sharded LRU Cache - Hot tier implementation
5. **ADR-0007**: Entity-Feature Key Schema - Data model design
6. **ADR-0003**: gRPC + HTTP Dual API - API design decisions
7. **ADR-0014**: Point-in-Time Versioned Keys - Historical query support
8. **ADR-0015**: HNSW Vector Similarity - Vector search capabilities
9. **ADR-0018**: Prometheus Metrics - Observability approach
10. **ADR-0017**: Single Binary Deployment - Deployment philosophy

## ADR Format

Each ADR follows this format:

```markdown
# ADR-NNNN: Title

## Status
[Proposed | Accepted | Deprecated | Superseded]

## Context
What is the issue that we're seeing that motivates this decision?

## Decision
What is the change that we're proposing and/or doing?

## Consequences
What becomes easier or more difficult because of this change?
```

## Creating New ADRs

When making significant architectural decisions:

1. Create a new file: `docs/adr/NNNN-title.md`
2. Use the next available number
3. Follow the format above
4. Submit for review before implementation

## Status Definitions

| Status | Description |
|--------|-------------|
| **Proposed** | Under discussion, not yet accepted |
| **Accepted** | Approved and being implemented |
| **Deprecated** | No longer recommended, but not replaced |
| **Superseded** | Replaced by a newer ADR |

## Related Resources

- [Architecture Overview](/docs/concepts/architecture) - High-level system design
- [Michael Nygard's ADR format](https://cognitect.com/blog/2011/11/15/documenting-architecture-decisions) - Original ADR proposal
- [ADR GitHub Organization](https://adr.github.io/) - ADR tools and templates
