# Feather Feature Store Architecture

> A comprehensive guide to Feather's internal architecture for developers and operators.

## Table of Contents

- [System Overview](#system-overview)
- [High-Level Architecture](#high-level-architecture)
- [Core Components](#core-components)
  - [Storage Layer](#storage-layer)
  - [Serving Layer](#serving-layer)
  - [Ingestion Layer](#ingestion-layer)
  - [Aggregation Engine](#aggregation-engine)
- [Data Flow](#data-flow)
- [Concurrency Model](#concurrency-model)
- [Observability](#observability)
- [Extension Points](#extension-points)
- [Performance Characteristics](#performance-characteristics)
- [Deployment Patterns](#deployment-patterns)

---

## System Overview

Feather is a high-performance, real-time feature store designed for machine learning applications. It provides **sub-millisecond P99 latency** for feature retrieval through a sophisticated multi-tier architecture optimized for serving ML features at scale.

### Key Design Principles

| Principle | Implementation |
|-----------|----------------|
| **Low Latency** | Two-tier storage with in-memory hot cache (256 shards) |
| **High Throughput** | Lock-free atomic counters, sharded access patterns |
| **Durability** | BadgerDB-backed warm tier with historical versioning |
| **Observability** | Prometheus metrics, OpenTelemetry tracing, structured logging |
| **Resilience** | Circuit breakers, graceful degradation, health probes |

---

## High-Level Architecture

```mermaid
flowchart TB
    subgraph Clients["Client Applications"]
        ML["ML Models"]
        Training["Training Pipelines"]
        FE["Feature Engineering"]
    end

    subgraph Serving["Serving Layer"]
        HTTP["HTTP REST API<br/>:8080"]
        GRPC["gRPC Server<br/>:50051"]
        Metrics["Prometheus Metrics<br/>:9090"]
    end

    subgraph Core["Core Processing"]
        Registry["Schema Registry"]
        Agg["Aggregation Engine"]
        Transform["Transform Pipeline"]
    end

    subgraph Storage["Storage Layer"]
        Hot["Hot Tier<br/>(In-Memory LRU)"]
        Warm["Warm Tier<br/>(BadgerDB)"]
    end

    subgraph Ingestion["Ingestion Layer"]
        Kafka["Kafka Consumer"]
        HTTPIngest["HTTP Ingestion<br/>:8081"]
        Batch["Batch Importer"]
    end

    subgraph External["External Systems"]
        KafkaBroker["Kafka Brokers"]
        Prometheus["Prometheus"]
        Jaeger["Jaeger/OTLP"]
    end

    Clients --> Serving
    Serving --> Core
    Core --> Storage
    Ingestion --> Core
    KafkaBroker --> Kafka
    Metrics --> Prometheus
    Serving --> Jaeger

    style Hot fill:#4CAF50,color:#fff
    style Warm fill:#2196F3,color:#fff
    style HTTP fill:#FF9800,color:#fff
    style GRPC fill:#FF9800,color:#fff
```

---

## Core Components

### Storage Layer

The storage layer implements a **two-tier hybrid architecture** optimized for both latency and durability.

```mermaid
flowchart LR
    subgraph HotTier["Hot Tier (Memory)"]
        direction TB
        S1["Shard 0"]
        S2["Shard 1"]
        S3["..."]
        S4["Shard 255"]
    end

    subgraph WarmTier["Warm Tier (BadgerDB)"]
        direction TB
        Current["Current Values<br/>f:entity:feature"]
        History["Historical Values<br/>h:entity:feature:ts"]
    end

    Request["GET Request"] --> HotTier
    HotTier -->|Cache Hit| Response["Response<br/>&lt;1ms"]
    HotTier -->|Cache Miss| WarmTier
    WarmTier --> Response2["Response<br/>1-10ms"]

    style HotTier fill:#4CAF50,color:#fff
    style WarmTier fill:#2196F3,color:#fff
```

#### Hot Tier Architecture

The hot tier provides sub-millisecond access through an in-memory sharded LRU cache.

```mermaid
flowchart TB
    subgraph HotTierDetail["Hot Tier Implementation"]
        Hash["FNV-1a Hash<br/>entity_key → shard_id"]

        subgraph Shards["256 Shards"]
            subgraph Shard0["Shard 0"]
                RW0["RWMutex"]
                Map0["map[entity]→entityData"]
            end
            subgraph Shard1["Shard 1"]
                RW1["RWMutex"]
                Map1["map[entity]→entityData"]
            end
            subgraph ShardN["Shard N..."]
                RWN["RWMutex"]
                MapN["map[entity]→entityData"]
            end
        end

        subgraph EntityData["Entity Data Structure"]
            Features["map[feature]→FeatureValue"]
            Mutex["RWMutex (per entity)"]
        end
    end

    Request["Request for entity:123"] --> Hash
    Hash -->|"hash % 256 = 42"| Shard1
    Shard1 --> EntityData

    style Hash fill:#9C27B0,color:#fff
    style Shards fill:#E3F2FD
```

**Key Implementation Details:**

| Component | Implementation | Purpose |
|-----------|---------------|---------|
| **Sharding** | 256 shards with FNV-1a hash | Reduces lock contention |
| **Locking** | RWMutex per shard + per entity | Fine-grained concurrency |
| **Metrics** | Atomic counters (lock-free) | Zero-cost observability |
| **Memory Tracking** | Approximate size (~100 bytes/value) | Memory limit enforcement |
| **TTL Eviction** | Background goroutine (1-minute interval) | Automatic cleanup |

#### Warm Tier Architecture

The warm tier provides persistent storage with historical versioning for point-in-time queries.

```mermaid
flowchart TB
    subgraph WarmTierDetail["Warm Tier (BadgerDB)"]
        subgraph Keys["Key Structure"]
            CurrentKey["Current: f:entity:feature"]
            HistoryKey["History: h:entity:feature:timestamp"]
        end

        subgraph Operations["Operations"]
            Get["Get() → Transaction Read"]
            Put["Put() → Atomic Write (Current + History)"]
            GetAsOf["GetAsOf() → Reverse Iterator"]
        end

        subgraph Optimizations["Performance Optimizations"]
            BufferPool["Buffer Pooling"]
            AsyncWrite["Async Background Writes"]
            Compaction["Automatic Compaction"]
        end
    end

    Write["Write Request"] --> Put
    Read["Read Request"] --> Get
    PointInTime["Point-in-Time Query"] --> GetAsOf

    style Keys fill:#BBDEFB
    style Operations fill:#C8E6C9
```

**Key Format Examples:**
```
Current value:   f:user:123:click_count → {"value": 42, "ts": 1705320600}
Historical:      h:user:123:click_count:1705320600000000000 → {"value": 40}
Historical:      h:user:123:click_count:1705320500000000000 → {"value": 38}
```

---

### Serving Layer

The serving layer exposes features through multiple protocols optimized for different use cases.

```mermaid
flowchart TB
    subgraph ServingLayer["Serving Layer"]
        subgraph HTTP["HTTP Server (:8080)"]
            direction TB
            MW1["RequestID Middleware"]
            MW2["Compression Middleware"]
            MW3["Security Headers"]
            MW4["CORS Middleware"]

            subgraph Handlers["Route Handlers"]
                FH["Feature Handler<br/>/v1/features"]
                SH["Schema Handler<br/>/v1/schema"]
                VH["Vector Handler<br/>/v1/vectors"]
                DH["Drift Handler<br/>/v1/drift"]
                HH["Health Handler<br/>/health, /ready, /live"]
            end
        end

        subgraph GRPC["gRPC Server (:50051)"]
            direction TB
            Interceptor["Metrics Interceptor"]
            FeatureSvc["FeatureService"]
            StreamSvc["Streaming Support"]
        end
    end

    Client["Client Request"] --> HTTP
    Client --> GRPC
    MW1 --> MW2 --> MW3 --> MW4 --> Handlers
    Interceptor --> FeatureSvc

    style HTTP fill:#FF9800,color:#fff
    style GRPC fill:#FF5722,color:#fff
```

#### HTTP API Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/v1/features` | GET | Retrieve features for an entity |
| `/v1/features` | POST | Store features |
| `/v1/features/batch` | POST | Batch retrieval for multiple entities |
| `/v1/features/history` | GET | Point-in-time feature retrieval |
| `/v1/schema/groups` | GET/POST | Manage feature group schemas |
| `/v1/vectors/{index}/search` | POST | Vector similarity search |
| `/v1/drift/status` | GET | Data drift monitoring status |
| `/health` | GET | Deep component health check |
| `/ready` | GET | Kubernetes readiness probe |
| `/live` | GET | Kubernetes liveness probe |

#### gRPC Service Definition

```protobuf
service FeatureService {
  rpc GetFeatures(GetFeaturesRequest) returns (GetFeaturesResponse);
  rpc GetFeaturesAsOf(GetFeaturesAsOfRequest) returns (GetFeaturesResponse);
  rpc PutFeatures(PutFeaturesRequest) returns (PutFeaturesResponse);
  rpc StreamFeatures(stream FeatureUpdate) returns (stream FeatureResponse);
}
```

---

### Ingestion Layer

The ingestion layer supports multiple data sources with built-in resilience patterns.

```mermaid
flowchart LR
    subgraph Sources["Data Sources"]
        KafkaTopic["Kafka Topics"]
        HTTPPush["HTTP Push"]
        BatchFiles["Batch Files<br/>(CSV/JSON/Parquet)"]
    end

    subgraph Ingestion["Ingestion Layer"]
        subgraph KafkaConsumer["Kafka Consumer"]
            CB["Circuit Breaker"]
            CG["Consumer Group"]
            Decoder["JSON Decoder"]
        end

        subgraph HTTPIngestion["HTTP Ingestion (:8081)"]
            RL["Rate Limiter"]
            Validator["Schema Validator"]
        end

        subgraph BatchImport["Batch Importer"]
            Parser["File Parser"]
            Backfill["Backfill Support"]
        end
    end

    subgraph Processing["Processing"]
        Store["Storage Engine"]
        Agg["Aggregation Engine"]
    end

    KafkaTopic --> KafkaConsumer
    HTTPPush --> HTTPIngestion
    BatchFiles --> BatchImport

    KafkaConsumer --> Store
    HTTPIngestion --> Store
    BatchImport --> Store
    Store --> Agg

    style CB fill:#F44336,color:#fff
    style RL fill:#FF9800,color:#fff
```

#### Circuit Breaker Pattern

```mermaid
stateDiagram-v2
    [*] --> Closed
    Closed --> Open: Failures ≥ Threshold (5)
    Open --> HalfOpen: Timeout Elapsed (30s)
    HalfOpen --> Closed: Success
    HalfOpen --> Open: Failure

    Closed: Normal operation
    Closed: Requests pass through
    Open: Fast fail
    Open: Reject all requests
    HalfOpen: Testing recovery
    HalfOpen: Allow single request
```

---

### Aggregation Engine

The aggregation engine computes real-time sliding window aggregations incrementally.

```mermaid
flowchart TB
    subgraph AggEngine["Aggregation Engine"]
        subgraph Windows["Window Management"]
            WM1["WindowManager<br/>(user:123, click_count)"]
            WM2["WindowManager<br/>(user:456, purchase_total)"]
        end

        subgraph RingBuffer["Ring Buffer (Buckets)"]
            B1["Bucket 0<br/>12:00-12:01"]
            B2["Bucket 1<br/>12:01-12:02"]
            B3["Bucket 2<br/>12:02-12:03"]
            BN["Bucket N<br/>..."]
        end

        subgraph Functions["Aggregation Functions"]
            Count["Count"]
            Sum["Sum"]
            Avg["Avg"]
            Min["Min"]
            Max["Max"]
        end
    end

    Update["Feature Update"] --> Windows
    Windows --> RingBuffer
    RingBuffer --> Functions
    Functions --> Result["Aggregate Result"]

    style RingBuffer fill:#E1BEE7
```

#### Bucket Time Granularity

| Window Size | Bucket Size | Max Buckets |
|-------------|-------------|-------------|
| ≤ 1 hour | 1 minute | 60 |
| ≤ 24 hours | 1 hour | 24 |
| > 24 hours | 1 day | 365 |

#### Bucket Data Structure

```go
type AggregationBucket struct {
    StartTime int64   // Unix nanoseconds (bucket boundary)
    Count     int64   // Number of values in bucket
    Sum       float64 // Sum of values
    Min       float64 // Minimum value
    Max       float64 // Maximum value
    LastValue float64 // Most recent value
}
```

---

## Data Flow

### Write Path (Feature Ingestion)

```mermaid
sequenceDiagram
    participant Client
    participant Server as HTTP/gRPC Server
    participant Registry as Schema Registry
    participant Hot as Hot Tier
    participant Warm as Warm Tier
    participant Agg as Aggregation Engine

    Client->>Server: POST /v1/features
    Server->>Registry: Validate schema
    Registry-->>Server: OK
    Server->>Hot: Put() [sync]
    Hot-->>Server: OK
    Server-->>Client: 200 OK

    Note over Server,Warm: Async background write
    Server-)Warm: Put() [async goroutine]
    Warm--)Server: Complete

    Note over Server,Agg: If aggregation configured
    Server-)Agg: Update window
    Agg--)Server: Complete
```

**Write Path Latency Breakdown:**

| Step | Latency | Mode |
|------|---------|------|
| Schema validation | ~0.01ms | Sync |
| Hot tier write | ~0.1ms | Sync |
| Response to client | ~0.15ms total | — |
| Warm tier write | ~2-5ms | Async (background) |
| Aggregation update | ~0.5ms | Async (background) |

### Read Path (Feature Serving)

```mermaid
sequenceDiagram
    participant Client
    participant Server as HTTP/gRPC Server
    participant Hot as Hot Tier
    participant Warm as Warm Tier
    participant Metrics

    Client->>Server: GET /v1/features?entity=user:123
    Server->>Hot: Get(entity, features)

    alt Cache Hit
        Hot-->>Server: Features found
        Server->>Metrics: Record hit
    else Cache Miss
        Hot-->>Server: Partial/no results
        Server->>Warm: Get(entity, missing)
        Warm-->>Server: Features from disk
        Server->>Metrics: Record miss
    end

    Server-->>Client: 200 OK (features)
```

**Read Path Latency Breakdown:**

| Scenario | P50 | P99 | P999 |
|----------|-----|-----|------|
| Hot tier hit | 0.05ms | 0.1ms | 0.2ms |
| Warm tier lookup | 0.5ms | 2ms | 5ms |
| Mixed (with miss) | 0.3ms | 1ms | 3ms |

### Point-in-Time Query Flow

```mermaid
sequenceDiagram
    participant Client
    participant Server as HTTP Server
    participant Warm as Warm Tier

    Client->>Server: GET /v1/features/history?as_of=2024-01-15T12:00:00Z
    Server->>Warm: GetAsOf(entity, timestamp)

    Note over Warm: Reverse iterator on<br/>historical keys
    Warm->>Warm: Seek to h:entity:feature:timestamp
    Warm->>Warm: Find latest entry ≤ as_of

    Warm-->>Server: Historical values
    Server-->>Client: 200 OK (point-in-time state)
```

---

## Concurrency Model

### Lock Hierarchy

```mermaid
flowchart TB
    subgraph Locks["Lock Hierarchy (Acquisition Order)"]
        L1["Package-level: store.mu, registry.mu<br/>(Schema modifications)"]
        L2["Shard-level: shard.mu<br/>(Per 256 shards)"]
        L3["Entity-level: entityData.mu<br/>(Per entity within shard)"]
    end

    L1 --> L2 --> L3

    subgraph LockFree["Lock-Free Operations"]
        Atomic["Atomic Counters<br/>(Metrics: hits, misses, etc.)"]
        CAS["Compare-And-Swap<br/>(State transitions)"]
    end

    style L1 fill:#FFCDD2
    style L2 fill:#FFE0B2
    style L3 fill:#C8E6C9
    style LockFree fill:#E1BEE7
```

### Read-Write Patterns

```go
// Read path (multiple goroutines can race safely)
shard.mu.RLock()
entity, ok := shard.data[entityKey]
shard.mu.RUnlock()
if ok {
    entity.mu.RLock()
    result := entity.features[featureName]
    entity.mu.RUnlock()
}

// Write path (serialized per entity)
shard.mu.Lock()
entity := shard.data[entityKey]
shard.mu.Unlock()
entity.mu.Lock()
entity.features[name] = value
entity.mu.Unlock()
```

---

## Observability

### Metrics Architecture

```mermaid
flowchart LR
    subgraph Feather["Feather Application"]
        HTTP["HTTP Handler"]
        GRPC["gRPC Handler"]
        Storage["Storage Layer"]
        Ingestion["Ingestion Layer"]
    end

    subgraph Metrics["Prometheus Metrics (:9090)"]
        Counters["Counters<br/>requests_total, errors_total"]
        Histograms["Histograms<br/>request_duration_seconds"]
        Gauges["Gauges<br/>hot_tier_size_bytes"]
    end

    subgraph External["External Systems"]
        Prometheus["Prometheus Server"]
        Grafana["Grafana Dashboards"]
    end

    Feather --> Metrics
    Prometheus -->|scrape| Metrics
    Prometheus --> Grafana

    style Metrics fill:#E65100,color:#fff
```

### Key Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `feather_http_requests_total` | Counter | Total HTTP requests by method, path, status |
| `feather_http_request_duration_seconds` | Histogram | Request latency distribution |
| `feather_cache_hits_total` | Counter | Hot tier cache hits |
| `feather_cache_misses_total` | Counter | Hot tier cache misses |
| `feather_hot_tier_size_bytes` | Gauge | Current hot tier memory usage |
| `feather_features_stored_total` | Counter | Total features written |
| `feather_ingestion_messages_total` | Counter | Kafka messages processed |

### Tracing Integration

```mermaid
flowchart LR
    subgraph Request["Request Flow"]
        R1["HTTP Request"]
        R2["Storage.Get()"]
        R3["HotTier.Get()"]
        R4["WarmTier.Get()"]
    end

    subgraph Spans["OpenTelemetry Spans"]
        S1["http.request<br/>method, path, status"]
        S2["storage.get<br/>entity, tier"]
        S3["hot_tier.get<br/>hit/miss"]
        S4["warm_tier.get<br/>duration"]
    end

    subgraph Export["Export"]
        OTLP["OTLP Exporter"]
        Jaeger["Jaeger"]
    end

    R1 --> S1
    R2 --> S2
    R3 --> S3
    R4 --> S4
    Spans --> OTLP --> Jaeger

    style Spans fill:#26A69A,color:#fff
```

---

## Extension Points

### Vector Similarity Search

```mermaid
flowchart TB
    subgraph VectorSearch["Vector Search System"]
        subgraph Index["HNSW Index"]
            Layers["Hierarchical Layers"]
            Nodes["Vector Nodes"]
            Edges["Proximity Edges"]
        end

        subgraph Distance["Distance Metrics"]
            Cosine["Cosine Similarity"]
            Euclidean["Euclidean (L2)"]
            Manhattan["Manhattan (L1)"]
        end

        subgraph API["Vector API"]
            Create["POST /v1/vectors"]
            Upsert["POST /v1/vectors/{index}/upsert"]
            Search["POST /v1/vectors/{index}/search"]
        end
    end

    Query["Search Query<br/>(vector + k)"] --> Index
    Index --> Distance
    Distance --> Results["Top-K Results"]

    style Index fill:#7E57C2,color:#fff
```

### Data Drift Detection

```mermaid
flowchart LR
    subgraph DriftDetection["Drift Detection"]
        Monitor["Statistical Monitor"]
        Reference["Reference Distribution"]
        Current["Current Distribution"]
        Compare["Hypothesis Testing"]
    end

    subgraph Alerts["Alert System"]
        Threshold["Threshold Check"]
        Alert["Drift Alert"]
    end

    FeatureUpdates["Feature Updates"] --> Monitor
    Monitor --> Reference
    Monitor --> Current
    Current --> Compare
    Compare -->|"Drift Detected"| Threshold
    Threshold --> Alert

    style Compare fill:#F44336,color:#fff
```

---

## Performance Characteristics

### Latency Profile

```mermaid
xychart-beta
    title "Operation Latency (P99)"
    x-axis ["Hot Read", "Warm Read", "Batch 100", "Point-in-Time", "Vector Search"]
    y-axis "Latency (ms)" 0 --> 25
    bar [0.8, 8, 4, 12, 20]
```

### Throughput Capacity

| Operation | Single Node | 3-Node Cluster |
|-----------|-------------|----------------|
| Reads (hot) | 1M ops/sec | 3M ops/sec |
| Reads (warm) | 100K ops/sec | 300K ops/sec |
| Writes | 50K ops/sec | 150K ops/sec |
| Batch reads | 10K batches/sec | 30K batches/sec |

### Memory Efficiency

- **Per feature overhead**: ~100 bytes + value size
- **Entity index overhead**: ~50 bytes per entity
- **Shard overhead**: ~1KB per shard (256 total)
- **Recommended ratio**: 1B features in 64GB memory

---

## Deployment Patterns

### Single Node Deployment

```mermaid
flowchart TB
    subgraph Node["Single Feather Node"]
        App["Application"]
        Hot["Hot Tier<br/>(Memory)"]
        Warm["Warm Tier<br/>(Disk)"]

        subgraph Ports["Exposed Ports"]
            P8080["HTTP :8080"]
            P50051["gRPC :50051"]
            P9090["Metrics :9090"]
        end
    end

    Client["Clients"] --> Ports
    App --> Hot
    App --> Warm

    style Node fill:#E3F2FD
```

### Kubernetes Deployment

```mermaid
flowchart TB
    subgraph K8s["Kubernetes Cluster"]
        subgraph NS["feather-system namespace"]
            subgraph StatefulSet["StatefulSet (replicas: 3)"]
                Pod0["feather-0"]
                Pod1["feather-1"]
                Pod2["feather-2"]
            end

            Headless["Headless Service<br/>(Pod Discovery)"]
            ClusterIP["ClusterIP Service<br/>(Load Balancing)"]

            subgraph Support["Supporting Resources"]
                HPA["HPA<br/>(2-10 pods)"]
                PDB["PDB<br/>(minAvailable: 2)"]
                SM["ServiceMonitor"]
            end
        end
    end

    Ingress["Ingress"] --> ClusterIP
    ClusterIP --> StatefulSet
    SM --> Prometheus["Prometheus"]

    style StatefulSet fill:#1976D2,color:#fff
```

### Federated Multi-Region

```mermaid
flowchart TB
    subgraph USEast["US-East Region"]
        Primary["Primary Node"]
    end

    subgraph USWest["US-West Region"]
        Replica1["Read Replica"]
    end

    subgraph EU["EU Region"]
        Replica2["Read Replica"]
    end

    Primary <-->|"Async Replication"| Replica1
    Primary <-->|"Async Replication"| Replica2
    Replica1 <-->|"Peer Discovery"| Replica2

    style Primary fill:#4CAF50,color:#fff
    style Replica1 fill:#2196F3,color:#fff
    style Replica2 fill:#2196F3,color:#fff
```

---

## Key Interfaces

### Core Abstractions

```go
// SchemaRegistry validates and manages feature schemas
type SchemaRegistry interface {
    GetGroup(name string) (*domain.FeatureGroup, error)
    GetFeatureSpec(featureName string) (*domain.FeatureSpec, error)
    ListGroups() []*domain.FeatureGroup
}

// StorageTier represents a storage tier (hot or warm)
type StorageTier interface {
    Get(entityKey string, features []string) (map[string]*domain.FeatureValue, error)
    Put(entityKey string, features map[string]*domain.FeatureValue) error
    Delete(entityKey string) error
}

// HealthChecker provides component health status
type HealthChecker interface {
    Check() *HealthCheckResult
    IsReady() bool
    IsHealthy() bool
}
```

---

## Further Reading

- [API Reference](./api-reference.md) - Detailed API documentation
- [Deployment Guide](./deployment.md) - Production deployment instructions
- [Contributing Guide](./contributing.md) - Development guidelines
- [Performance Tuning](./performance.md) - Optimization strategies
