# Feather Documentation

Welcome to the Feather Feature Store documentation. This guide covers everything from getting started to advanced production deployments.

## Table of Contents

- **Getting Started**
  - [Quick Start](../README.md#quick-start)
  - [API Examples](../README.md#api-examples)
  - [Configuration](../README.md#configuration)
  - [Troubleshooting](../README.md#troubleshooting)

- **Core Documentation**
  - [Architecture Overview](./architecture.md) — System design, data flow, concurrency model
  - [API Reference](./api-reference.md) — Complete HTTP REST, gRPC, and extension APIs
  - [Client SDK Guide](./sdk-guide.md) — Go, Python, Rust, TypeScript, Java, Swift, Kotlin SDKs
  - [Package Guide](./package-guide.md) — Maturity matrix for all internal packages

- **Features & Capabilities**
  - [Real-Time Streaming](./streaming.md) — Streaming pipelines, windowed aggregations, CEP
  - [LLM Embeddings](./llm-embeddings.md) — OpenAI, Ollama, HuggingFace embedding generation
  - [Feature Freshness](./freshness.md) — Adaptive TTL, SLA management, alerting
  - [AI-Powered Discovery](./discovery.md) — Semantic search, NL queries, recommendations
  - [Feature Flags](./feature-flags.md) — Feature flag management
  - [Drift Detection](./api-reference.md#drift-detection) — KL divergence, JS divergence, PSI
  - [Vector Search](./api-reference.md#vector-search) — HNSW similarity search for embeddings
  - [Offline Sync](./offline-sync.md) — Apache Spark and Flink integration
  - [Extension Maturity](./EXTENSION_MATURITY.md) — Maturity levels for extension modules

- **Operations & Deployment**
  - [Deployment Guide](./deployment.md) — Docker, Kubernetes, Helm instructions
  - [Kubernetes Operator](./operator.md) — Custom resources (FeatureStore, FeatureGroup, FeatureView)
  - [Cloud Storage Backends](./cloud-storage.md) — DynamoDB, S3, GCS, Bigtable
  - [Dashboard & Web UI](./dashboard.md) — Monitoring, exploration, alert management
  - [Observability Guide](./observability.md) — Prometheus metrics, Grafana dashboards, alerting
  - [Performance Guide](./performance.md) — Benchmarking, optimization, capacity planning

- **Development**
  - [Contributing Guide](./contributing.md) — Setup, coding standards, PR workflow
  - [ADRs](./adr/README.md) — Architecture Decision Records

- **Additional Resources**
  - [Tutorials](./tutorials/) — Step-by-step guides
  - [Benchmarks](./benchmarks/) — Performance benchmark results
  - [Blog](./blog/) — Engineering blog posts
  - [Talks](./talks/) — Conference talks and presentations
  - [Community](./community/) — Community resources

## Quick Links

| Getting Started | Core Concepts | Operations |
|-----------------|---------------|------------|
| [Quick Start](../README.md#quick-start) | [Architecture](./architecture.md) | [Deployment](./deployment.md) |
| [API Examples](../README.md#api-examples) | [API Reference](./api-reference.md) | [Observability](./observability.md) |
| [Configuration](../README.md#configuration) | [SDK Guide](./sdk-guide.md) | [Performance](./performance.md) |

## Architecture Decision Records

The [ADR directory](./adr/) contains records of significant architectural decisions:

| ADR | Decision |
|-----|----------|
| [ADR-0001](./adr/0001-tiered-storage-architecture.md) | Tiered storage architecture (hot/warm) |
| [ADR-0002](./adr/0002-sharded-in-memory-cache.md) | 256-shard in-memory cache design |
| [ADR-0003](./adr/0003-badgerdb-for-persistence.md) | BadgerDB selection for warm tier |
| [ADR-0004](./adr/0004-dual-protocol-api.md) | HTTP REST + gRPC dual protocol |
| [ADR-0005](./adr/0005-dual-path-ingestion.md) | Kafka + HTTP push ingestion |
| [ADR-0006](./adr/0006-sliding-window-aggregation.md) | Sliding window aggregation design |
| [ADR-0007](./adr/0007-modular-package-architecture.md) | Modular package structure |
| [ADR-0008](./adr/0008-pluggable-http-handlers.md) | Pluggable HTTP handler system |
| [ADR-0009](./adr/0009-structured-logging-slog.md) | slog for structured logging |
| [ADR-0010](./adr/0010-opentelemetry-tracing.md) | OpenTelemetry integration |
| [ADR-0011](./adr/0011-multi-language-sdk-generation.md) | Multi-language SDK strategy |
| [ADR-0012](./adr/0012-graceful-shutdown.md) | Graceful shutdown protocol |
| [ADR-0013](./adr/0013-go-implementation-language.md) | Go as implementation language |
| [ADR-0014](./adr/0014-point-in-time-versioned-keys.md) | Point-in-time query key design |
| [ADR-0015](./adr/0015-hnsw-vector-similarity.md) | HNSW for vector similarity |
| [ADR-0016](./adr/0016-object-pooling.md) | Object pooling for performance |
| [ADR-0017](./adr/0017-single-binary-deployment.md) | Single binary deployment |
| [ADR-0018](./adr/0018-prometheus-pull-metrics.md) | Prometheus pull-based metrics |

## Getting Help

- **Issues**: [GitHub Issues](https://github.com/feather-store/feather/issues)
- **Discussions**: [GitHub Discussions](https://github.com/feather-store/feather/discussions)
- **Contributing**: See the [Contributing Guide](./contributing.md)

## Document Conventions

Throughout the documentation:

- `code blocks` indicate commands, code, or configuration
- **Bold** highlights important terms or concepts
- > Blockquotes provide tips or important notes
- Tables summarize options, parameters, or comparisons
- Mermaid diagrams illustrate architecture and data flow
