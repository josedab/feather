<p align="center">
  <img src="docs/assets/feather-logo.svg" alt="Feather Logo" width="120" height="120">
</p>

<h1 align="center">Feather</h1>

<p align="center">
  <strong>High-Performance Real-Time Feature Store for Machine Learning</strong>
</p>

<p align="center">
  <a href="#key-features">Features</a> •
  <a href="#quick-start">Quick Start</a> •
  <a href="#api-examples">API</a> •
  <a href="#documentation">Docs</a> •
  <a href="#benchmarks">Benchmarks</a>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/go-%3E%3D1.22-blue.svg" alt="Go Version">
  <img src="https://img.shields.io/badge/license-MIT-green.svg" alt="License">
  <img src="https://img.shields.io/badge/PRs-welcome-brightgreen.svg" alt="PRs Welcome">
</p>

---

Feather is a production-ready feature store designed for **sub-millisecond P99 latency** at scale. It enables ML teams to serve features in real-time through a tiered storage architecture, real-time aggregations, and multiple serving APIs.

```
┌─────────────────────────────────────────────────────────────────────────┐
│                             FEATHER                                      │
├─────────────────────────────────────────────────────────────────────────┤
│  Ingestion          │  Processing           │  Serving                  │
│  ─────────          │  ──────────           │  ───────                  │
│  • Kafka Streaming  │  • Schema Registry    │  • HTTP REST API          │
│  • HTTP Push        │  • Real-time Aggs     │  • gRPC (streaming)       │
│  • Batch Import     │  • Transformations    │  • Point-in-time queries  │
├─────────────────────────────────────────────────────────────────────────┤
│  Storage                                                                 │
│  ───────                                                                 │
│  Hot Tier (Memory, <1ms)  ←→  Warm Tier (BadgerDB, <10ms)              │
└─────────────────────────────────────────────────────────────────────────┘
```

## Key Features

| Feature | Description |
|---------|-------------|
| **Sub-millisecond Latency** | P99 < 1ms for hot tier reads through 256-shard in-memory cache |
| **Tiered Storage** | Automatic hot (memory) / warm (disk) tier management with LRU eviction |
| **Real-time Aggregations** | Sliding window aggregations (count, sum, avg, min, max) computed incrementally |
| **Point-in-Time Queries** | Historical feature retrieval for consistent training data generation |
| **Multiple APIs** | HTTP REST and gRPC with streaming support |
| **Vector Search** | HNSW-based similarity search for embeddings (cosine, euclidean, manhattan) |
| **Schema Registry** | Feature group definitions with type validation and versioning |
| **Drift Detection** | Statistical drift monitoring with KL divergence, JS divergence, and PSI metrics |
| **Feature Freshness SLAs** | Adaptive TTL management, ML-driven predictions, and SLA enforcement with alerting |
| **AI-Powered Discovery** | Semantic search, natural language queries, and personalized feature recommendations |
| **Offline Sync** | Apache Spark and Flink connectors for batch training data export |
| **Production Ready** | Prometheus metrics, OpenTelemetry tracing, structured logging, health probes |

## Quick Start

### Installation

```bash
# Clone the repository
git clone https://github.com/your-org/feather.git
cd feather

# Build the binary
make build

# Run with defaults
./bin/feather

# Or with a config file
./bin/feather -config configs/feather.yaml
```

### Docker

```bash
# Build and run with Docker
make docker-build
docker run -d \
  --name feather \
  -p 8080:8080 \
  -p 50051:50051 \
  -p 9090:9090 \
  -v feather-data:/var/lib/feather/data \
  feather:latest
```

### Verify Installation

```bash
# Health check
curl http://localhost:8080/health

# Expected output:
# {"status":"healthy","components":{"hot_tier":"healthy","warm_tier":"healthy"}}
```

## API Examples

### Store Features

```bash
curl -X POST http://localhost:8080/v1/features \
  -H "Content-Type: application/json" \
  -d '{
    "entity_key": "user:123",
    "features": {
      "click_count": 15,
      "purchase_total": 245.50,
      "last_activity": "2024-01-15T10:30:00Z"
    }
  }'
```

### Retrieve Features

```bash
curl "http://localhost:8080/v1/features?entity=user:123&feature=click_count&feature=purchase_total"
```

**Response:**
```json
{
  "success": true,
  "data": {
    "entities": {
      "user:123": {
        "features": {
          "click_count": {"value": 15, "timestamp": 1705315800000000000},
          "purchase_total": {"value": 245.50, "timestamp": 1705315800000000000}
        }
      }
    }
  },
  "request_id": "req-a1b2c3d4"
}
```

### Batch Retrieval

```bash
curl -X POST http://localhost:8080/v1/features/batch \
  -H "Content-Type: application/json" \
  -d '{
    "entities": ["user:123", "user:456", "user:789"],
    "features": ["click_count", "purchase_total"]
  }'
```

### Point-in-Time Query

Retrieve feature values as they existed at a specific moment in time — essential for generating consistent training data without data leakage.

```bash
curl "http://localhost:8080/v1/features/history?entity=user:123&feature=click_count&as_of=2024-01-15T00:00:00Z"
```

### Real-Time Aggregations

Define sliding window aggregations in your feature schema:

```yaml
# configs/feather.yaml
schema:
  groups:
    - name: user_engagement
      entity_type: user
      features:
        - name: clicks_last_hour
          data_type: int64
          aggregation:
            function: count
            window: 1h
            slide_by: 1m
        - name: spend_last_24h
          data_type: float64
          aggregation:
            function: sum
            window: 24h
            slide_by: 1h
```

### Vector Similarity Search

```bash
# Create an index
curl -X POST http://localhost:8080/v1/vectors \
  -H "Content-Type: application/json" \
  -d '{
    "name": "product_embeddings",
    "dimension": 384,
    "distance_type": "cosine"
  }'

# Upsert vectors
curl -X POST http://localhost:8080/v1/vectors/product_embeddings/upsert \
  -H "Content-Type: application/json" \
  -d '{
    "vectors": [
      {"id": "prod_001", "values": [0.1, 0.2, ...], "metadata": {"category": "electronics"}},
      {"id": "prod_002", "values": [0.3, 0.1, ...], "metadata": {"category": "books"}}
    ]
  }'

# Search for similar vectors
curl -X POST http://localhost:8080/v1/vectors/product_embeddings/search \
  -H "Content-Type: application/json" \
  -d '{
    "vector": [0.15, 0.18, ...],
    "top_k": 10
  }'
```

## Client SDKs

### Go Client

```go
import "github.com/your-org/feather/sdk/go/feather"

client, err := feather.NewClient("localhost:8080")
if err != nil {
    log.Fatal(err)
}

// Get features
features, err := client.GetFeatures(ctx, "user:123", []string{"click_count", "purchase_total"})

// Store features
err = client.PutFeatures(ctx, "user:123", map[string]interface{}{
    "click_count": 15,
    "purchase_total": 245.50,
})
```

### Python Client

```python
from feather import FeatherClient

client = FeatherClient("localhost:8080")

# Get features
features = client.get_features("user:123", ["click_count", "purchase_total"])

# Store features
client.put_features("user:123", {
    "click_count": 15,
    "purchase_total": 245.50
})

# Point-in-time retrieval
historical = client.get_features_as_of(
    "user:123",
    ["click_count"],
    as_of="2024-01-15T00:00:00Z"
)
```

## Configuration

Feather can be configured via **YAML file** or **environment variables**.

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `FEATHER_HTTP_PORT` | `8080` | HTTP API port |
| `FEATHER_GRPC_PORT` | `50051` | gRPC API port |
| `FEATHER_PROMETHEUS_PORT` | `9090` | Prometheus metrics port |
| `FEATHER_HOT_MAX_MEMORY` | `4GB` | Maximum hot tier memory |
| `FEATHER_HOT_TTL` | `1h` | Hot tier entry TTL |
| `FEATHER_WARM_PATH` | `/var/lib/feather/data` | Warm tier storage path |
| `FEATHER_KAFKA_ENABLED` | `false` | Enable Kafka ingestion |
| `FEATHER_KAFKA_BROKERS` | `localhost:9092` | Kafka broker addresses |
| `FEATHER_TRACING_ENABLED` | `false` | Enable OpenTelemetry tracing |

### Configuration File

```yaml
# configs/feather.yaml
server:
  http:
    port: 8080
    read_timeout: 30s
    write_timeout: 30s
  grpc:
    port: 50051
    max_concurrent: 1000

storage:
  hot:
    max_memory: "8GB"
    ttl: "2h"
    num_shards: 256
  warm:
    path: "/var/lib/feather/data"
    sync_writes: false

ingestion:
  kafka:
    enabled: true
    brokers: ["kafka1:9092", "kafka2:9092"]
    topic: "feature-updates"
    consumer_group: "feather"
  http:
    enabled: true
    port: 8081
    rate_limit: 10000  # requests per second

observability:
  metrics:
    enabled: true
    port: 9090
  tracing:
    enabled: true
    endpoint: "jaeger:4317"
    sample_rate: 0.1
  logging:
    level: "info"
    format: "json"

schema:
  groups:
    - name: user_features
      entity_type: user
      ttl: 30d
      features:
        - name: click_count
          data_type: int64
        - name: purchase_total
          data_type: float64
        - name: last_activity
          data_type: timestamp
```

## Benchmarks

Performance measured on a single node (AWS c5.4xlarge: 16 vCPU, 32GB RAM):

| Operation | P50 | P99 | P999 | Throughput |
|-----------|-----|-----|------|------------|
| Hot tier read | 50μs | 100μs | 200μs | 1M+ ops/sec |
| Warm tier read | 500μs | 2ms | 5ms | 100K+ ops/sec |
| Write (async) | 100μs | 300μs | 1ms | 50K+ ops/sec |
| Batch read (100 entities) | 1ms | 4ms | 8ms | 10K batches/sec |
| Vector search (1M vectors) | 5ms | 20ms | 50ms | 5K queries/sec |

### Memory Efficiency

- **1 billion features** in **64GB** memory
- ~100 bytes overhead per feature value
- ~50 bytes overhead per entity

### Run Your Own Benchmarks

```bash
# Run the benchmark suite
make bench

# Run with custom parameters
go test -bench=. -benchtime=10s ./test/benchmark/...
```

## Architecture

```
┌──────────────────────────────────────────────────────────────────────┐
│                         Client Applications                           │
│              (ML Models, Training Pipelines, Dashboards)              │
└────────────────────────────────┬─────────────────────────────────────┘
                                 │
                    ┌────────────┴────────────┐
                    ▼                         ▼
            ┌──────────────┐          ┌──────────────┐
            │  HTTP :8080  │          │ gRPC :50051  │
            └──────┬───────┘          └──────┬───────┘
                   │                         │
                   └───────────┬─────────────┘
                               ▼
                    ┌─────────────────────┐
                    │   Schema Registry   │
                    │   Aggregation Eng   │
                    │   Rate Limiting     │
                    └──────────┬──────────┘
                               │
              ┌────────────────┼────────────────┐
              ▼                                 ▼
    ┌─────────────────┐              ┌─────────────────┐
    │    Hot Tier     │              │   Warm Tier     │
    │  (Memory LRU)   │◄────────────►│   (BadgerDB)    │
    │   256 shards    │   overflow   │  + historical   │
    │    <1ms P99     │              │    <10ms P99    │
    └─────────────────┘              └─────────────────┘
              ▲
              │
    ┌─────────┴─────────────────────────┐
    │          Ingestion Layer          │
    │  ┌───────────┬──────────────────┐ │
    │  │   Kafka   │  HTTP Ingestion  │ │
    │  │ Consumer  │    (push API)    │ │
    │  └───────────┴──────────────────┘ │
    └───────────────────────────────────┘
```

For detailed architecture documentation with Mermaid diagrams, see [Architecture Overview](./docs/architecture.md).

## Deployment

### Kubernetes

```bash
# Using Helm
helm install feather ./deploy/helm/feather \
  --namespace feather-system \
  --create-namespace \
  --set replicaCount=3 \
  --set storage.hot.maxMemory="8GB"

# Using raw manifests
kubectl apply -k deploy/kubernetes/
```

### Health Probes

| Endpoint | Purpose | Use Case |
|----------|---------|----------|
| `GET /live` | Liveness probe | K8s restart trigger |
| `GET /ready` | Readiness probe | K8s traffic routing |
| `GET /health` | Deep health check | Debugging, monitoring |

For complete deployment instructions, see [Deployment Guide](./docs/deployment.md).

## Documentation

| Document | Description |
|----------|-------------|
| [Architecture Overview](./docs/architecture.md) | System design, data flow, components |
| [API Reference](./docs/api-reference.md) | Complete HTTP and gRPC API documentation |
| [Deployment Guide](./docs/deployment.md) | Docker, Kubernetes, Helm installation |
| [Observability Guide](./docs/observability.md) | Prometheus metrics, Grafana dashboards, alerting |
| [Feature Freshness](./docs/freshness.md) | Adaptive TTL, SLA management, alerting, remediation |
| [AI-Powered Discovery](./docs/discovery.md) | Semantic search, NL queries, recommendations |
| [Offline Sync](./docs/offline-sync.md) | Apache Spark and Flink integration for batch export |
| [Performance Guide](./docs/performance.md) | Optimization tips and benchmarking |
| [Contributing Guide](./docs/contributing.md) | Development setup, coding standards |

## Development

### Prerequisites

- Go 1.22+
- Make
- Docker (optional)

### Commands

```bash
# Build
make build

# Run tests with race detector
make test

# Run short tests only
make test-short

# Run with coverage report
make test-coverage

# Lint code
make lint

# Format code
make fmt

# Run all checks (fmt, vet, lint, test)
make check

# Run benchmarks
make bench

# Build Docker image
make docker-build
```

### Project Structure

```
feather/
├── cmd/feather/          # Application entrypoint
├── internal/
│   ├── aggregation/      # Real-time aggregation engine
│   ├── config/           # Configuration loading
│   ├── domain/           # Core domain types
│   ├── drift/            # Drift detection and monitoring
│   ├── freshness/        # Feature freshness SLAs and TTL management
│   ├── ingestion/        # Kafka and HTTP ingestion
│   ├── offline/          # Spark and Flink connectors
│   ├── semantic/         # AI-powered discovery and search
│   ├── server/           # HTTP and gRPC servers
│   ├── storage/          # Hot/warm tiered storage
│   ├── metrics/          # Prometheus metrics
│   ├── tracing/          # OpenTelemetry tracing
│   └── vector/           # Vector similarity search
├── sdk/
│   ├── go/               # Go client SDK
│   ├── python/           # Python client SDK
│   ├── java/             # Java/Kotlin client SDK
│   ├── rust/             # Rust client SDK
│   └── typescript/       # TypeScript client SDK
├── api/                  # Protocol buffer and OpenAPI definitions
├── configs/              # Example configurations
├── deploy/               # Kubernetes manifests, Helm charts, observability
├── docs/                 # Documentation
└── test/                 # Integration and benchmark tests
```

## Roadmap

- [x] Offline feature store integration (Apache Spark, Flink)
- [x] Drift detection and monitoring
- [x] Feature freshness SLAs with auto-remediation
- [x] AI-powered feature discovery and recommendations
- [ ] Feature lineage tracking
- [ ] A/B testing support for feature experimentation
- [ ] Multi-tenant isolation
- [ ] Distributed mode with automatic sharding
- [ ] BigQuery integration for data warehouse sync

## Contributing

We welcome contributions! Please see our [Contributing Guide](./docs/contributing.md) for details on:

- Setting up your development environment
- Code style and conventions
- Submitting pull requests
- Running tests

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

---

<p align="center">
  <sub>Built with performance in mind for ML teams worldwide.</sub>
</p>
