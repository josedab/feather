# Feather Feature Store Architecture

## Overview

Feather is a high-performance, real-time feature store designed for machine learning applications. It provides sub-millisecond feature retrieval through a tiered storage architecture, real-time aggregations, and multiple serving APIs.

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              Client Applications                              │
│         (ML Models, Training Pipelines, Feature Engineering Jobs)            │
└────────────────────────────────┬────────────────────────────────────────────┘
                                 │
                                 ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                              API Gateway                                      │
│                    ┌──────────────┬──────────────┐                           │
│                    │   HTTP/REST  │    gRPC      │                           │
│                    │   :8080      │   :50051     │                           │
│                    └──────────────┴──────────────┘                           │
└────────────────────────────────┬────────────────────────────────────────────┘
                                 │
┌────────────────────────────────┼────────────────────────────────────────────┐
│                                ▼                                              │
│  ┌──────────────────────────────────────────────────────────────────────┐   │
│  │                         Server Layer                                  │   │
│  │  ┌──────────────┐ ┌──────────────┐ ┌──────────────┐ ┌─────────────┐  │   │
│  │  │ Auth Handler │ │Feature Handler│ │Schema Handler│ │Vector Handler│  │   │
│  │  └──────────────┘ └──────────────┘ └──────────────┘ └─────────────┘  │   │
│  │  ┌──────────────┐ ┌──────────────┐ ┌──────────────┐ ┌─────────────┐  │   │
│  │  │Drift Handler │ │Export Handler │ │Federation    │ │Health Check │  │   │
│  │  └──────────────┘ └──────────────┘ └──────────────┘ └─────────────┘  │   │
│  └──────────────────────────────────────────────────────────────────────┘   │
│                                                                              │
│  ┌──────────────────────────────────────────────────────────────────────┐   │
│  │                         Core Services                                 │   │
│  │  ┌──────────────┐ ┌──────────────┐ ┌──────────────┐ ┌─────────────┐  │   │
│  │  │  Transform   │ │ Aggregation  │ │   Backfill   │ │  Streaming  │  │   │
│  │  │   Pipeline   │ │    Engine    │ │   Manager    │ │   Engine    │  │   │
│  │  └──────────────┘ └──────────────┘ └──────────────┘ └─────────────┘  │   │
│  │  ┌──────────────┐ ┌──────────────┐ ┌──────────────┐ ┌─────────────┐  │   │
│  │  │    SLA       │ │  Registry    │ │ Consistency  │ │   Vector    │  │   │
│  │  │   Manager    │ │   Catalog    │ │   Checker    │ │    Index    │  │   │
│  │  └──────────────┘ └──────────────┘ └──────────────┘ └─────────────┘  │   │
│  └──────────────────────────────────────────────────────────────────────┘   │
│                                                                              │
│  ┌──────────────────────────────────────────────────────────────────────┐   │
│  │                         Storage Layer                                 │   │
│  │  ┌──────────────────────────┐ ┌─────────────────────────────────────┐│   │
│  │  │       Hot Tier           │ │           Warm Tier                 ││   │
│  │  │   (In-Memory LRU)        │ │         (BadgerDB)                  ││   │
│  │  │   - Sub-ms latency       │ │   - Persistent storage              ││   │
│  │  │   - TTL support          │ │   - Historical versions             ││   │
│  │  │   - Sharded access       │ │   - Point-in-time queries           ││   │
│  │  └──────────────────────────┘ └─────────────────────────────────────┘│   │
│  └──────────────────────────────────────────────────────────────────────┘   │
│                                                                              │
└──────────────────────────────────────────────────────────────────────────────┘
                                 │
                                 ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                            External Systems                                   │
│  ┌──────────────┐ ┌──────────────┐ ┌──────────────┐ ┌─────────────────────┐  │
│  │    Kafka     │ │  Prometheus  │ │    Jaeger    │ │  Federation Nodes   │  │
│  │  (Ingestion) │ │  (Metrics)   │ │  (Tracing)   │ │  (Distributed)      │  │
│  └──────────────┘ └──────────────┘ └──────────────┘ └─────────────────────┘  │
└─────────────────────────────────────────────────────────────────────────────┘
```

## Core Components

### 1. Storage Layer

#### Tiered Architecture

The storage layer implements a two-tier architecture for optimal performance:

**Hot Tier (`internal/storage/hot.go`)**
- In-memory LRU cache with configurable size limits
- Sharded design for concurrent access (default: 256 shards)
- TTL support for automatic expiration
- Sub-millisecond read latency (~50-100μs)

```go
type HotTier struct {
    shards    []*shard
    numShards int
    maxMemory int64
    ttl       time.Duration
}
```

**Warm Tier (`internal/storage/warm.go`)**
- BadgerDB-backed persistent storage
- Historical version support for point-in-time queries
- Automatic compaction and garbage collection
- Efficient range queries for time-series data

**Unified Store (`internal/storage/store.go`)**
- Coordinates read/write operations between tiers
- Implements read-through caching
- Manages data consistency between tiers

### 2. API Layer

#### HTTP REST API (`internal/server/http.go`)

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/v1/features` | GET | Get features for entity |
| `/v1/features` | POST | Store features |
| `/v1/features/batch` | POST | Batch get features |
| `/v1/features/history` | GET | Point-in-time retrieval |
| `/v1/schema/groups` | GET/POST | Manage feature groups |
| `/v1/vectors/{index}/search` | POST | Vector similarity search |
| `/v1/drift/status` | GET | Drift monitoring status |
| `/health`, `/ready`, `/live` | GET | Health probes |

#### gRPC API (`internal/server/grpc.go`)

- High-performance binary protocol
- Streaming support for bulk operations
- Service reflection for debugging

### 3. Ingestion Layer

#### Kafka Consumer (`internal/ingestion/kafka.go`)
- Consumer group support for horizontal scaling
- Circuit breaker pattern for resilience
- Configurable batch processing

#### HTTP Ingestion (`internal/ingestion/http.go`)
- Rate limiting per client IP
- Bulk ingestion endpoint
- Schema validation

### 4. Feature Registry

#### Schema Registry (`internal/storage/registry.go`)
- Feature group definitions
- Data type validation
- Schema versioning

#### Feature Catalog (`internal/registry/catalog.go`)
- Feature discovery and metadata
- Version comparison and evolution tracking
- Breaking change detection

### 5. Real-Time Aggregations

#### Aggregation Engine (`internal/aggregation/engine.go`)
- Sliding window aggregations
- Supported functions: count, sum, avg, min, max
- Configurable window sizes and slide intervals

### 6. Feature Transformations

#### Transform Pipeline (`internal/transform/`)
- **ArithmeticExecutor**: Mathematical operations
- **AggregationExecutor**: Statistical aggregations
- **WindowExecutor**: Time-windowed computations
- **ConditionalExecutor**: Conditional logic
- **StringExecutor**: String manipulations
- **TimestampExecutor**: Date/time operations
- **LookupExecutor**: Cross-entity lookups

### 7. Vector Similarity Search

#### HNSW Index (`internal/vector/hnsw.go`)
- Hierarchical Navigable Small World graphs
- Configurable M (connections) and efConstruction
- Distance metrics: cosine, euclidean, dot product

### 8. Observability

#### Metrics (`internal/metrics/metrics.go`)
- Prometheus-compatible metrics
- Request latency histograms
- Storage tier utilization
- Cache hit rates

#### Tracing (`internal/tracing/tracing.go`)
- OpenTelemetry integration
- OTLP export support
- Request correlation

#### Logging (`internal/logging/logger.go`)
- Structured logging with slog
- JSON and text formats
- Log levels and sampling

### 9. Federation

#### Multi-Node Federation (`internal/federation/`)
- Node discovery and health checking
- Feature sharing across nodes
- Replication policies
- Access control

### 10. SLA Management

#### SLA Framework (`internal/sla/`)
- SLA types: latency, freshness, availability, throughput
- Real-time monitoring and alerting
- Error budget tracking

## Data Flow

### Write Path

```
1. Client Request → HTTP/gRPC Server
2. Authentication & Authorization
3. Schema Validation (Registry)
4. Write to Hot Tier (in-memory)
5. Async write to Warm Tier (BadgerDB)
6. Trigger on-write transforms (if configured)
7. Emit metrics and traces
```

### Read Path

```
1. Client Request → HTTP/gRPC Server
2. Authentication & Authorization
3. Check Hot Tier (LRU cache)
   └── Cache Hit → Return immediately
   └── Cache Miss → Continue to step 4
4. Read from Warm Tier (BadgerDB)
5. Populate Hot Tier (read-through cache)
6. Execute on-read transforms (if configured)
7. Return to client
```

### Streaming Path

```
1. Kafka Message → Kafka Consumer
2. Deserialization & Validation
3. Circuit Breaker Check
4. Batch Processing
5. Write to Store
6. Update Aggregations
7. Emit events
```

## Deployment Architecture

### Single Node

```
┌─────────────────────────────────────┐
│           Feather Node              │
│  ┌─────────────────────────────────┐│
│  │          Application            ││
│  │   HTTP :8080 | gRPC :50051      ││
│  └─────────────────────────────────┘│
│  ┌─────────────────────────────────┐│
│  │         Storage                 ││
│  │   Hot Tier | Warm Tier          ││
│  └─────────────────────────────────┘│
└─────────────────────────────────────┘
```

### Kubernetes Deployment

```
┌──────────────────────────────────────────────────────────────────┐
│                     Kubernetes Cluster                            │
│  ┌─────────────────────────────────────────────────────────────┐ │
│  │                    feather-system namespace                  │ │
│  │                                                              │ │
│  │  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐            │ │
│  │  │  Feather-0  │ │  Feather-1  │ │  Feather-2  │            │ │
│  │  │ (StatefulSet)│ │             │ │             │            │ │
│  │  └─────────────┘ └─────────────┘ └─────────────┘            │ │
│  │         │               │               │                    │ │
│  │         └───────────────┼───────────────┘                    │ │
│  │                         ▼                                    │ │
│  │              ┌─────────────────────┐                         │ │
│  │              │   Headless Service  │                         │ │
│  │              │   (feather-headless)│                         │ │
│  │              └─────────────────────┘                         │ │
│  │                         │                                    │ │
│  │                         ▼                                    │ │
│  │              ┌─────────────────────┐                         │ │
│  │              │     ClusterIP       │                         │ │
│  │              │   Service (feather) │                         │ │
│  │              └─────────────────────┘                         │ │
│  │                                                              │ │
│  │  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐            │ │
│  │  │     PDB     │ │     HPA     │ │ServiceMonitor│            │ │
│  │  │ (min: 2)    │ │ (2-10 pods) │ │ (Prometheus) │            │ │
│  │  └─────────────┘ └─────────────┘ └─────────────┘            │ │
│  └─────────────────────────────────────────────────────────────┘ │
└──────────────────────────────────────────────────────────────────┘
```

### Federated Deployment

```
┌─────────────────────────────────────────────────────────────────────┐
│                     Multi-Region Federation                          │
│                                                                      │
│  ┌──────────────────┐      ┌──────────────────┐                     │
│  │   US-East Node   │◄────►│   US-West Node   │                     │
│  │  (Primary)       │      │  (Replica)       │                     │
│  └────────┬─────────┘      └────────┬─────────┘                     │
│           │                         │                                │
│           └───────────┬─────────────┘                                │
│                       │                                              │
│                       ▼                                              │
│              ┌──────────────────┐                                    │
│              │    EU Node       │                                    │
│              │  (Read Replica)  │                                    │
│              └──────────────────┘                                    │
│                                                                      │
│  Features:                                                           │
│  - Async replication                                                 │
│  - Conflict resolution                                               │
│  - Regional failover                                                 │
│  - Access control                                                    │
└─────────────────────────────────────────────────────────────────────┘
```

## Configuration

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `FEATHER_HTTP_PORT` | 8080 | HTTP API port |
| `FEATHER_GRPC_PORT` | 50051 | gRPC API port |
| `FEATHER_HOT_MAX_MEMORY` | 4GB | Hot tier memory limit |
| `FEATHER_HOT_TTL` | 1h | Hot tier TTL |
| `FEATHER_WARM_PATH` | /var/lib/feather/data | Warm tier data path |
| `FEATHER_KAFKA_ENABLED` | false | Enable Kafka ingestion |
| `FEATHER_TRACING_ENABLED` | false | Enable OpenTelemetry |

### Configuration File

```yaml
server:
  http:
    port: 8080
    read_timeout: 30s
    write_timeout: 30s
  grpc:
    port: 50051

storage:
  hot:
    max_memory: "4GB"
    ttl: "1h"
    num_shards: 256
  warm:
    path: "/var/lib/feather/data"
    sync_writes: false

ingestion:
  kafka:
    enabled: false
    brokers: ["localhost:9092"]
    topic: "features"
    group_id: "feather"

observability:
  metrics:
    enabled: true
    port: 9090
  tracing:
    enabled: false
    endpoint: "localhost:4317"
  logging:
    level: "info"
    format: "json"
```

## Performance Characteristics

### Latency

| Operation | P50 | P99 | P999 |
|-----------|-----|-----|------|
| Hot tier read | 50μs | 100μs | 200μs |
| Warm tier read | 500μs | 2ms | 5ms |
| Write (async) | 100μs | 300μs | 1ms |
| Vector search (1M vectors) | 5ms | 20ms | 50ms |

### Throughput

| Operation | Single Node | Clustered (3 nodes) |
|-----------|-------------|---------------------|
| Reads | 100K+ ops/sec | 300K+ ops/sec |
| Writes | 50K+ ops/sec | 150K+ ops/sec |
| Batch reads | 10K batches/sec | 30K batches/sec |

### Memory Usage

- Hot tier: Configurable, default 4GB
- Per-feature overhead: ~100 bytes + value size
- Warm tier: Disk-based, limited only by storage

## Security

### Authentication
- API key authentication
- JWT token support
- mTLS for gRPC

### Authorization
- Role-based access control
- Feature-level permissions
- Team/organization scopes

### Encryption
- TLS 1.3 for transport
- At-rest encryption (BadgerDB)

## Monitoring

### Key Metrics

- `feather_requests_total` - Total requests by endpoint
- `feather_request_duration_seconds` - Request latency histogram
- `feather_hot_tier_size_bytes` - Hot tier memory usage
- `feather_hot_tier_hit_ratio` - Cache hit rate
- `feather_warm_tier_size_bytes` - Warm tier disk usage
- `feather_features_total` - Total features stored

### Health Endpoints

- `/health` - Deep health check
- `/ready` - Readiness probe
- `/live` - Liveness probe

## Extending Feather

### Custom Executors

```go
type CustomExecutor struct{}

func (e *CustomExecutor) Execute(ctx context.Context, t *Transform, inputs map[string]interface{}) (interface{}, error) {
    // Custom transformation logic
    return result, nil
}

func (e *CustomExecutor) Validate(t *Transform) error {
    // Validation logic
    return nil
}
```

### Custom Ingestion Sources

Implement the `Ingester` interface to add custom data sources.

### Custom Metrics

Add custom metrics using the Prometheus client library.
