---
title: "ADR-0017: Single Binary Deployment"
sidebar_label: "0017: Single Binary"
---

# ADR-0017: Single-Binary Self-Hosted Deployment Model

## Status

Accepted

## Context

Feature stores in the market typically require complex deployment topologies:

**Feast (open source)**:
- Separate online/offline stores
- Redis or DynamoDB for online
- Spark or BigQuery for offline
- Registry service
- Multiple configuration files

**Tecton (SaaS)**:
- Cloud-managed service
- Requires AWS/GCP account integration
- Data residency concerns

**Sagemaker Feature Store**:
- AWS-only
- Requires S3, Glue, Athena
- Vendor lock-in

For teams wanting:
- **Self-hosted**: Data stays in their infrastructure
- **Simple operations**: Minimal moving parts
- **Quick evaluation**: Get started in minutes, not days
- **Predictable costs**: No per-request cloud pricing

...existing solutions are over-complicated.

## Decision

We design Feather as a **single-binary, self-hosted feature store** that runs with minimal external dependencies.

### Deployment Model

```
┌─────────────────────────────────────────────┐
│              Feather Binary                 │
│  ┌────────────────────────────────────────┐ │
│  │  HTTP Server (:8080)                   │ │
│  │  gRPC Server (:50051)                  │ │
│  │  Metrics Server (:9090)                │ │
│  │  Ingestion Server (:8081)              │ │
│  └────────────────────────────────────────┘ │
│  ┌────────────────────────────────────────┐ │
│  │  Hot Tier (in-memory)                  │ │
│  │  Warm Tier (embedded BadgerDB)         │ │
│  │  Aggregation Engine                    │ │
│  │  Vector Index                          │ │
│  └────────────────────────────────────────┘ │
└─────────────────────────────────────────────┘
          │
          ▼ (optional)
    ┌───────────┐
    │   Kafka   │  ← Only external dependency
    └───────────┘     (optional for streaming)
```

### Core Principles

**1. Embedded Everything**
- Storage: BadgerDB embedded (no external database)
- Cache: In-process memory (no external Redis)
- Search: HNSW in-process (no external vector DB)

**2. Single Process**
- All functionality in one binary
- No microservice coordination
- Simplified debugging and profiling

**3. File-Based Configuration**
```yaml
# config.yaml - everything in one file
server:
  http_port: 8080
  grpc_port: 50051
storage:
  hot:
    max_memory: 4GB
  warm:
    path: /var/lib/feather/data
```

**4. Optional Dependencies**
- Kafka: Disabled by default, enable for streaming
- Tracing: Disabled by default, enable for observability
- TLS: Disabled by default, enable for production

### Minimal Deployment

```bash
# Download
curl -L https://github.com/feather/releases/latest/feather -o feather
chmod +x feather

# Run (uses embedded defaults)
./feather

# Or with config
./feather -config config.yaml
```

## Consequences

### Positive

- **Fast time-to-value**: Running in under 5 minutes
- **Low operational burden**: One process to monitor and debug
- **Predictable resource usage**: No distributed system surprises
- **Easy local development**: Same binary runs on laptop
- **No vendor lock-in**: Runs anywhere with Linux/macOS
- **Simple backups**: Copy data directory
- **Debuggable**: Single process with standard Go tooling

### Negative

- **Vertical scaling only**: Single node limits throughput
- **No high availability**: Single point of failure
- **Memory-bound**: Hot tier limited to single machine RAM
- **Storage-bound**: Warm tier limited to single machine disk
- **Not cloud-native**: No auto-scaling, managed services

### Neutral

- **Horizontal scaling later**: Clustering can be added (ADR pending)
- **Backup responsibility**: User manages data backups
- **Capacity planning**: User must size machine appropriately

## Scaling Considerations

### When Single-Binary is Sufficient

| Metric | Comfortable Range |
|--------|-------------------|
| Entities | < 10M |
| Features/entity | < 100 |
| QPS | < 50K |
| Hot tier size | < 32GB |
| Vectors | < 5M |

### When to Consider Alternatives

- QPS > 50K sustained
- Hot tier > 32GB
- Strict HA requirements (99.99%+)
- Multi-region deployment needed
- Real-time cross-entity queries

### Binary Size

| Component | Contribution |
|-----------|--------------|
| Go runtime | ~5MB |
| BadgerDB | ~3MB |
| gRPC/Protobuf | ~4MB |
| Prometheus client | ~1MB |
| Application code | ~2MB |
| **Total** | **~15MB** |
