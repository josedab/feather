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
  <a href="https://github.com/feather-store/feather/actions/workflows/ci.yml"><img src="https://github.com/feather-store/feather/actions/workflows/ci.yml/badge.svg" alt="CI"></a>
  <img src="https://img.shields.io/badge/go-%3E%3D1.24-blue.svg" alt="Go Version">
  <img src="https://img.shields.io/badge/license-Apache%202.0-green.svg" alt="License">
  <img src="https://img.shields.io/codecov/c/github/feather-store/feather.svg" alt="Coverage">
  <img src="https://img.shields.io/badge/PRs-welcome-brightgreen.svg" alt="PRs Welcome">
</p>

---

Feather is a production-ready feature store designed for **sub-millisecond P99 latency** at scale. It enables ML teams to serve features in real-time through a tiered storage architecture, real-time aggregations, and multiple serving APIs.

## Try It in 30 Seconds

[![Open in GitHub Codespaces](https://github.com/codespaces/badge.svg)](https://codespaces.new/feather-store/feather?quickstart=1)

```bash
git clone https://github.com/feather-store/feather.git && cd feather
make quickstart        # builds, starts, seeds demo data, and verifies
```

That's it — `make quickstart` auto-detects Docker or Go and does the right thing.
It prints copy-paste curl commands when ready. [More options ↓](#quick-start)

Once running, walk through every feature interactively:

```bash
make explore           # guided tour: features, batch, point-in-time, vectors
```

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

### From source (recommended)

Requires only Go 1.24+ and Make.

```bash
git clone https://github.com/feather-store/feather.git
cd feather

# One command: build, start, seed demo data, verify
make quickstart

# Or for interactive development (foreground, text logs)
make run-dev
```

### Go install (no clone needed)

```bash
go install github.com/feather-store/feather/cmd/feather@latest
feather   # starts with built-in dev config, demo schema loaded
```

### Docker

```bash
# Pull and run the prebuilt image
make quickstart-docker

# Or explicitly
docker run -d \
  --name feather \
  -p 8080:8080 \
  -p 50051:50051 \
  -p 9090:9090 \
  ghcr.io/feather-store/feather:latest

# Minimal compose (uses configs/feather-dev.yaml)
docker compose -f docker-compose.dev.yml up

# Full stack (Kafka + Prometheus + Grafana)
docker compose -f docker-compose.yml up --build
```

### Verify Installation

```bash
# Health check
curl http://localhost:8080/health

# Expected output:
# {"status":"healthy","components":{"hot_tier":"healthy","warm_tier":"healthy"}}
```

### Seed Demo Data

```bash
# Requires the quickstart schema (configs/feather-dev.yaml)
make demo
```

## API Examples

> These examples assume Feather is running (`make quickstart` or `make run-dev`).
>
> **Full API spec:** [`api/openapi/feather.yaml`](./api/openapi/feather.yaml) — import into Postman, Bruno, or any OpenAPI-compatible tool.

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

> Install from this repository. Start Feather first with `make run-dev`.

**End-to-end example:** `python examples/ml-pipeline.py`

| Language | Install | Quickstart |
|----------|---------|------------|
| **Go** | `import "github.com/feather-store/feather/sdk/go/feather"` | [Quickstart](./sdk/go/feather/quickstart/README.md) |
| **Python** | `pip install -e sdk/python/` | [Quickstart](./sdk/python/quickstart/README.md) |
| **TypeScript** | `cd sdk/typescript && npm install` | [Quickstart](./sdk/typescript/quickstart/README.md) |
| **Java** | Maven/Gradle from `sdk/java/` | [Quickstart](./sdk/java/quickstart/README.md) |
| **Rust** | Cargo from `sdk/rust/` | [Quickstart](./sdk/rust/quickstart/README.md) |
| **Swift** | SPM from `sdk/swift/` | [Source](./sdk/swift/) |
| **Kotlin** | Gradle from `sdk/kotlin/` | [Source](./sdk/kotlin/) |

<details>
<summary>Quick code examples</summary>

**Go:**
```go
client, _ := feather.NewClient("localhost:8080")
features, _ := client.GetFeatures(ctx, "user:123", []string{"click_count"})
```

**Python:**
```python
from feather_client import FeatherClient
client = FeatherClient("localhost:8080")
features = client.get_features("user:123", ["click_count"])
```
</details>

## Configuration

Feather can be configured via **YAML file** or **environment variables**.

| Config File | Use Case |
|-------------|----------|
| `configs/feather-dev.yaml` | Local development, zero external dependencies — **start here** |
| `configs/feather-local.yaml` | Local development with disk persistence |
| `configs/feather.yaml` | Production reference with all features |

### Key Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `FEATHER_HTTP_PORT` | `8080` | HTTP API port |
| `FEATHER_GRPC_PORT` | `50051` | gRPC API port |
| `FEATHER_HOT_MAX_MEMORY` | `4GB` | Maximum hot tier memory |
| `FEATHER_WARM_PATH` | `/var/lib/feather/data` | Warm tier storage path |
| `FEATHER_KAFKA_ENABLED` | `false` | Enable Kafka ingestion |
| `FEATHER_TRACING_ENABLED` | `false` | Enable OpenTelemetry tracing |

See the [full configuration reference](./configs/feather.yaml) for all options.

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

For detailed architecture documentation with Mermaid diagrams, data flow, and concurrency model, see **[Architecture Overview](./docs/architecture.md)**.

## Deployment

Feather supports Docker, Kubernetes (Helm), and raw binary deployment.

```bash
# Kubernetes with Helm
helm install feather ./deploy/helm/feather \
  --namespace feather-system --create-namespace

# Docker
docker compose -f docker-compose.dev.yml up
```

| Health Endpoint | Purpose |
|----------------|---------|
| `GET /live` | K8s liveness probe |
| `GET /ready` | K8s readiness probe |
| `GET /health` | Deep health check |

See [Deployment Guide](./docs/deployment.md) for full instructions.

## Documentation

| Document | Description |
|----------|-------------|
| [Architecture Overview](./docs/architecture.md) | System design, data flow, components |
| [Package Guide](./docs/package-guide.md) | Maturity matrix for all internal packages |
| [API Reference](./docs/api-reference.md) | Complete HTTP and gRPC API documentation |
| [Client SDK Guide](./docs/sdk-guide.md) | Go, Python, Rust, TypeScript, and Java SDKs |
| [Deployment Guide](./docs/deployment.md) | Docker, Kubernetes, Helm installation |
| [Kubernetes Operator](./docs/operator.md) | Custom resources and operator deployment |
| [Real-Time Streaming](./docs/streaming.md) | Streaming pipelines, windowing, CEP |
| [Cloud Storage Backends](./docs/cloud-storage.md) | DynamoDB, S3, GCS, Bigtable integration |
| [LLM Embeddings](./docs/llm-embeddings.md) | OpenAI, Ollama, HuggingFace embedding generation |
| [Dashboard & Web UI](./docs/dashboard.md) | Web interface for monitoring and exploration |
| [Observability Guide](./docs/observability.md) | Prometheus metrics, Grafana dashboards, alerting |
| [Feature Freshness](./docs/freshness.md) | Adaptive TTL, SLA management, alerting, remediation |
| [AI-Powered Discovery](./docs/discovery.md) | Semantic search, NL queries, recommendations |
| [Offline Sync](./docs/offline-sync.md) | Apache Spark and Flink integration for batch export |
| [Performance Guide](./docs/performance.md) | Optimization tips and benchmarking |
| [Contributing Guide](./docs/contributing.md) | Development setup, coding standards |

Run `make docs` to start the full documentation site locally (requires Node.js). The source is in `website/`.

## Development

**Prerequisites:** Go 1.24+, Make. Docker optional. Default build uses `CGO_ENABLED=0` — no C compiler needed.

```bash
make setup          # One-command contributor setup (first time)
make doctor         # Check prerequisites
make build          # Build binary (~5s cached)
make run-dev        # Run with minimal dev config
make test-core      # Core tests (~10s, fast feedback)
make test-quick     # All tests, short mode (~60s)
make check-quick    # Pre-commit checks: fmt + vet + lint + tests (~20s)
make api-routes     # List all API handlers with maturity levels
make list-extensions # Show enabled vs available features
make help           # All available targets
```

### CLI & TUI

```bash
make build-cli && ./bin/feather-cli --help
make build-tui && ./bin/feather-tui
```

### Project Structure

```
feather/
├── cmd/                  # Server, CLI, TUI, MCP binaries
├── internal/
│   ├── core/             # Essential packages (stable) — storage, server, ingestion, etc.
│   ├── extensions/       # Optional feature modules — 38 packages (see docs/package-guide.md)
│   ├── integrations/     # External connectors — dbt, Spark, Flink, MLflow, etc.
│   ├── platform/         # Infrastructure — auth, cluster, governance, operator, etc.
│   └── tools/            # Developer tools — benchmark, dashboard, playground
├── sdk/                  # Client SDKs: Go, Python, TypeScript, Java, Rust, Swift, Kotlin
├── api/                  # Protocol buffer and OpenAPI definitions
├── configs/              # Example configurations
├── deploy/               # Kubernetes manifests, Helm charts
├── docs/                 # Documentation (see docs/package-guide.md for full package matrix)
└── test/                 # Integration and benchmark tests
```

Run `make api-routes` to see all registered handlers with maturity levels.

## Roadmap

See [GitHub Issues](https://github.com/feather-store/feather/issues) for current priorities. Next up:

- [ ] BigQuery integration for data warehouse sync

## Troubleshooting

| Symptom | Cause | Fix |
|---------|-------|-----|
| `bind: address already in use` | Port 8080, 50051, or 9090 is taken | Run `make doctor` to check ports, or `lsof -i :8080` |
| `golangci-lint not found` | Dev tools not installed | `make install-tools` |
| Build fails with C compiler / `librdkafka` errors | CGO enabled but no C toolchain | Default build uses `CGO_ENABLED=0`. Use `make build-cgo` only if you need Kafka |
| `make test` takes 5+ minutes | Full suite includes race detector | Use `make test-core` (~10s) for fast feedback |
| `feature group not found` on API calls | Server started without a schema config | Run `make run-dev` (loads dev config with demo schema) |
| `make quickstart` hangs | Docker installed but daemon not running | Start Docker Desktop, or run `make quickstart-local` to skip Docker |
| Connection refused during smoke tests | Server not running | Start with `make run-dev` first, then `make smoke-test` |

Run `make doctor` to check your environment for common issues.

## Contributing

We welcome contributions! Please see our [Contributing Guide](./docs/contributing.md) for details on:

- Setting up your development environment
- Code style and conventions
- Submitting pull requests
- Running tests

## License

This project is licensed under the Apache License 2.0 - see the [LICENSE](LICENSE) file for details.

---

<p align="center">
  <sub>Built with performance in mind for ML teams worldwide.</sub>
</p>
