---
sidebar_position: 11
title: FAQ
description: Frequently asked questions about Feather feature store.
---

# Frequently Asked Questions

Common questions about Feather and feature stores.

## General

### What is Feather?

Feather is a high-performance, real-time feature store for machine learning applications. It stores, serves, and manages ML features with sub-millisecond latency. Unlike traditional feature stores that require Redis, PostgreSQL, or cloud services, Feather is a single binary with no external dependencies.

### What is a feature store?

A feature store is infrastructure that manages ML features (the input variables for ML models). It solves several problems:

- **Training-serving skew**: Ensures features computed during training match those used at inference
- **Feature reuse**: Share features across models and teams instead of rebuilding them
- **Point-in-time correctness**: Generate training data without data leakage
- **Low-latency serving**: Serve features in real-time for online inference

### How is Feather different from other feature stores?

| Aspect | Feather | Others (Feast, Tecton, etc.) |
|--------|---------|------------------------------|
| **Deployment** | Single binary | Redis + database + cloud services |
| **Latency** | Sub-millisecond P99 | 5-50ms P99 |
| **Setup time** | 5 minutes | Hours to days |
| **Dependencies** | None | Multiple external services |
| **Cost** | Free, self-hosted | Free to expensive |

See our [detailed comparison](/docs/comparison) for more information.

### Is Feather production-ready?

Yes. Feather includes:

- Prometheus metrics and Grafana dashboards
- OpenTelemetry distributed tracing
- Kubernetes health probes (`/health`, `/ready`, `/live`)
- Graceful shutdown with configurable timeout
- Rate limiting and circuit breakers
- Structured JSON logging

### What license is Feather under?

Feather is licensed under the MIT License, which allows commercial use, modification, and distribution.

---

## Performance

### What latency can I expect?

On a single c5.4xlarge instance (16 vCPU, 32GB RAM):

| Operation | P50 | P99 | P999 |
|-----------|-----|-----|------|
| Hot tier read | 50us | 100us | 200us |
| Warm tier read | 500us | 2ms | 5ms |
| Write (async) | 100us | 300us | 1ms |
| Vector search (1M) | 5ms | 20ms | 50ms |

### How many features can Feather store?

With the default hot tier configuration (4GB), Feather can store approximately:

- ~40 million features with average 100-byte values
- ~400 million features with 10-byte values (integers, small strings)

The warm tier (BadgerDB) can store billions of features on disk.

### How does Feather achieve sub-millisecond latency?

1. **256-shard in-memory cache**: Fine-grained locking minimizes contention
2. **No network hops**: Embedded storage, no Redis/database calls
3. **Efficient serialization**: Minimal overhead for encoding/decoding
4. **Go runtime**: Low GC pause times, efficient concurrency

### Does Feather support horizontal scaling?

Currently, Feather runs as a single node. For horizontal scaling:

- Run multiple Feather instances with different entity key ranges
- Use a load balancer with consistent hashing
- Application-level sharding by entity type

Distributed mode with automatic sharding is on the roadmap.

---

## Features

### Does Feather support point-in-time queries?

Yes. Feather stores historical versions of features in the warm tier. You can query features as they existed at any timestamp:

```bash
curl "http://localhost:8080/v1/features/history?entity=user:123&feature=clicks&as_of=2024-01-15T00:00:00Z"
```

This is essential for generating training data without data leakage.

### Does Feather support real-time aggregations?

Yes. Feather computes sliding window aggregations incrementally:

- **Functions**: count, sum, avg, min, max
- **Windows**: Any duration (1 minute to 30 days)
- **Slide intervals**: Configurable granularity

```yaml
features:
  - name: clicks_last_hour
    aggregation:
      function: count
      window: 1h
      slide_by: 1m
```

### Does Feather support vector similarity search?

Yes. Feather includes a built-in HNSW index for approximate nearest neighbor search:

- **Distance metrics**: Cosine, Euclidean, Dot Product
- **Dimensions**: Up to 4096
- **Performance**: Under 5ms for 1M vectors at 95%+ recall

No separate vector database needed.

### Can I use Feather for batch training data export?

Yes. Feather integrates with Apache Spark and Apache Flink for batch export:

```python
# Spark
df = spark.read.format("feather") \
    .option("entities", "user:*") \
    .option("features", "clicks,purchases") \
    .load("http://localhost:8080")
```

See the [Offline Sync Guide](/docs/guides/offline-sync) for details.

### Does Feather detect feature drift?

Yes. Feather monitors statistical drift using:

- KL Divergence
- JS Divergence
- PSI (Population Stability Index)

Alerts trigger when drift exceeds configurable thresholds.

---

## Data

### What data types does Feather support?

| Type | Description | Example |
|------|-------------|---------|
| `int64` | 64-bit integer | `42` |
| `float64` | 64-bit float | `3.14` |
| `string` | UTF-8 string | `"hello"` |
| `bool` | Boolean | `true` |
| `bytes` | Binary data | Base64 encoded |
| `timestamp` | RFC3339 timestamp | `"2024-01-15T10:30:00Z"` |
| `int64_list` | List of integers | `[1, 2, 3]` |
| `float64_list` | List of floats | `[1.0, 2.0]` |
| `string_list` | List of strings | `["a", "b"]` |
| `map` | Key-value map | `{"key": "value"}` |

### How long is data retained?

Data retention is configurable:

- **Hot tier TTL**: Default 1 hour, configurable up to 7 days
- **Warm tier**: Indefinite retention until manual deletion

```yaml
storage:
  hot:
    ttl: "4h"  # Keep in memory for 4 hours
  warm:
    retention: "90d"  # Keep on disk for 90 days
```

### Can I import data from CSV/Parquet?

Yes. Use the batch import API:

```bash
# CSV
curl -X POST http://localhost:8080/v1/import \
  -F "file=@features.csv" \
  -F "format=csv"

# Parquet
curl -X POST http://localhost:8080/v1/import \
  -F "file=@features.parquet" \
  -F "format=parquet"
```

### How do I back up Feather data?

The warm tier uses BadgerDB, which supports online backups:

```bash
# Create backup
curl -X POST http://localhost:8080/v1/admin/backup \
  -d '{"path": "/backups/feather-2024-01-15.bak"}'

# Or use BadgerDB directly
badger backup --dir /var/lib/feather/data --backup-file backup.bak
```

---

## Deployment

### What are the system requirements?

**Minimum**:
- 2 CPU cores
- 4GB RAM
- 10GB disk

**Recommended for production**:
- 8+ CPU cores
- 32GB+ RAM
- SSD storage

### Can I run Feather on Kubernetes?

Yes. Feather includes:

- Helm chart in `deploy/helm/feather/`
- Kubernetes manifests in `deploy/kubernetes/`
- StatefulSet for stable network identity
- PersistentVolumeClaim for data durability

```bash
helm install feather ./deploy/helm/feather \
  --namespace feather-system \
  --create-namespace
```

### Does Feather support TLS?

Yes. Configure TLS in the config file:

```yaml
server:
  http:
    tls:
      enabled: true
      cert_file: "/path/to/cert.pem"
      key_file: "/path/to/key.pem"
  grpc:
    tls:
      enabled: true
      cert_file: "/path/to/cert.pem"
      key_file: "/path/to/key.pem"
```

### Does Feather support authentication?

Yes. Feather supports API key authentication:

```yaml
auth:
  enabled: true
  api_keys:
    - key: "sk-prod-xxxxx"
      name: "production"
      permissions: ["read", "write"]
    - key: "sk-readonly-xxxxx"
      name: "readonly"
      permissions: ["read"]
```

---

## Integration

### What SDKs are available?

Official SDKs:

| Language | Package | Status |
|----------|---------|--------|
| **Go** | `github.com/feather-store/feather/sdk/go/feather` | Stable |
| **Python** | `pip install feather-client` | Stable |
| **Java/Kotlin** | `io.feather:feather-client` | Stable |
| **Rust** | `feather-client` (crates.io) | Stable |
| **TypeScript** | `@feather/client` (npm) | Stable |

### Can I use Feather with gRPC?

Yes. Feather exposes a gRPC API on port 50051 (configurable):

```go
import "github.com/feather-store/feather/api/proto"

conn, _ := grpc.Dial("localhost:50051", grpc.WithInsecure())
client := proto.NewFeatureServiceClient(conn)

resp, _ := client.GetFeatures(ctx, &proto.GetFeaturesRequest{
    EntityKey: "user:123",
    Features:  []string{"clicks", "purchases"},
})
```

### Does Feather integrate with Kafka?

Yes. Feather consumes feature updates from Kafka:

```yaml
ingestion:
  kafka:
    enabled: true
    brokers: ["kafka:9092"]
    topic: "feature-updates"
    consumer_group: "feather"
```

Message format:
```json
{
  "entity_key": "user:123",
  "features": {"clicks": 42},
  "timestamp": "2024-01-15T10:30:00Z"
}
```

### Can I push features via HTTP?

Yes. The HTTP ingestion endpoint accepts real-time updates:

```bash
curl -X POST http://localhost:8081/ingest \
  -H "Content-Type: application/json" \
  -d '{"entity_key": "user:123", "features": {"clicks": 42}}'
```

For high-throughput, use the bulk endpoint:

```bash
curl -X POST http://localhost:8081/ingest/bulk \
  -H "Content-Type: application/json" \
  -d '{"updates": [...]}'
```

---

## Troubleshooting

### Why am I getting "entity not found" errors?

Common causes:

1. **Entity key format**: Keys are case-sensitive (`user:123` ≠ `User:123`)
2. **TTL expiration**: Hot tier data may have expired
3. **Not yet ingested**: Check if the write succeeded

Debug:
```bash
curl http://localhost:8080/debug/stats | jq '.hot_tier.entries'
```

### Why is latency higher than expected?

Check these in order:

1. **Cache hit rate**: Low hit rate means warm tier reads
   ```promql
   rate(feather_cache_hits_total[5m]) / rate(feather_cache_requests_total[5m])
   ```

2. **Hot tier size**: May need more memory
   ```yaml
   storage:
     hot:
       max_memory: "8GB"
   ```

3. **GC pauses**: Check Go runtime stats
   ```bash
   curl http://localhost:8080/debug/vars | jq '.memstats'
   ```

See the [Troubleshooting Guide](/docs/troubleshooting) for more details.

### Where can I get help?

- **GitHub Issues**: [Report bugs](https://github.com/feather-store/feather/issues)
- **GitHub Discussions**: [Ask questions](https://github.com/feather-store/feather/discussions)
- **Documentation**: You're reading it!

---

## Contributing

### How can I contribute?

We welcome contributions! See the [Contributing Guide](/docs/contributing) for:

- Development setup
- Code style guidelines
- Pull request process
- Testing requirements

### How do I report a bug?

Open an issue on [GitHub](https://github.com/feather-store/feather/issues) with:

1. Feather version (`./feather -version`)
2. Configuration (redact secrets)
3. Steps to reproduce
4. Expected vs actual behavior
5. Relevant logs

### How do I request a feature?

Open a [GitHub Discussion](https://github.com/feather-store/feather/discussions) in the "Ideas" category. Describe:

1. The use case
2. Why existing features don't solve it
3. Proposed solution (if any)
