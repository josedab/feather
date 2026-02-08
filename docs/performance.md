# Performance Tuning Guide

This guide provides comprehensive recommendations for optimizing Feather's performance across different deployment scenarios and workload characteristics.

## Table of Contents

- [Performance Targets](#performance-targets)
- [Quick Start: Common Configurations](#quick-start-common-configurations)
- [Storage Optimization](#storage-optimization)
  - [Hot Tier Tuning](#hot-tier-tuning)
  - [Warm Tier Tuning](#warm-tier-tuning)
  - [Memory Management](#memory-management)
- [Server Configuration](#server-configuration)
  - [HTTP Server](#http-server)
  - [gRPC Server](#grpc-server)
  - [Connection Handling](#connection-handling)
- [Query Optimization](#query-optimization)
  - [Batch Operations](#batch-operations)
  - [Feature Selection](#feature-selection)
  - [Caching Strategies](#caching-strategies)
- [Ingestion Optimization](#ingestion-optimization)
  - [Kafka Configuration](#kafka-configuration)
  - [HTTP Ingestion](#http-ingestion)
  - [Batch Ingestion](#batch-ingestion)
- [Vector Search Optimization](#vector-search-optimization)
- [Client-Side Optimization](#client-side-optimization)
- [Monitoring and Profiling](#monitoring-and-profiling)
- [Benchmarking](#benchmarking)
- [Troubleshooting Performance](#troubleshooting-performance)

---

## Performance Targets

Feather is designed to meet the following performance targets:

| Metric | Target | Tier |
|--------|--------|------|
| Read latency P50 | < 100 µs | Hot |
| Read latency P99 | < 1 ms | Hot |
| Read latency P99 | < 10 ms | Warm |
| Write latency P99 | < 5 ms | Hot + Warm |
| Throughput | > 100,000 req/s | Single node |
| Concurrent connections | > 10,000 | Default config |

### Understanding Latency Tiers

```
┌─────────────────────────────────────────────────────────────┐
│                     Request Flow                             │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  Client  ──→  Network  ──→  HTTP/gRPC  ──→  Hot Tier        │
│  (SDK)        (RTT)          Server         (Memory)         │
│                                │                             │
│  ~0.1ms      ~0.1-5ms        ~0.05ms        ~0.1ms          │
│                                │                             │
│                                └──→  Warm Tier (if miss)    │
│                                      (BadgerDB)              │
│                                      ~1-10ms                 │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

---

## Quick Start: Common Configurations

### High-Throughput Read-Heavy Workload

```yaml
storage:
  hot:
    max_memory: 16GB        # Maximize cache hit rate
    eviction_policy: lru

serving:
  grpc:
    max_concurrent: 5000
  http:
    read_timeout: 5s
    write_timeout: 5s
```

```bash
# Environment variables
export FEATHER_HOT_MAX_MEMORY=16GB
export FEATHER_GRPC_MAX_CONCURRENT=5000
```

### Low-Latency Real-Time Serving

```yaml
storage:
  hot:
    max_memory: 8GB
  warm:
    sync_interval: 100ms    # Fast durability

serving:
  http:
    read_timeout: 1s        # Tight timeouts
    write_timeout: 1s
```

### High-Volume Write Ingestion

```yaml
storage:
  hot:
    max_memory: 4GB
  warm:
    sync_interval: 5s       # Batch writes for throughput

ingestion:
  kafka:
    enabled: true
    # Use multiple partitions for parallelism
```

---

## Storage Optimization

### Hot Tier Tuning

The hot tier is an in-memory cache with 256 shards using FNV-1a hashing for distribution.

#### Memory Sizing

Calculate your hot tier memory based on working set size:

```
Required Memory = (Number of Entities × Average Features per Entity × 100 bytes)
```

**Guidelines:**

| Working Set Size | Recommended Memory |
|------------------|-------------------|
| < 1M entities | 2-4 GB |
| 1-10M entities | 8-16 GB |
| 10-100M entities | 32-64 GB |
| > 100M entities | 128+ GB or sharding |

```yaml
storage:
  hot:
    max_memory: 8GB    # Supports ~80M feature values
```

#### Eviction Policy

Currently only LRU (Least Recently Used) is supported:

```yaml
storage:
  hot:
    eviction_policy: lru
```

**LRU Behavior:**
- Eviction triggers when `curSize > maxSize`
- Eviction happens asynchronously via a buffered channel (1000 capacity)
- Each feature value adds ~100 bytes to size tracking

#### Shard Distribution

The hot tier uses 256 shards with FNV-1a hashing:

```go
shardIndex = fnvHash(entityKey) % 256
```

**Best Practices for Entity Keys:**
- Use consistent prefixes: `user:123`, `product:456`
- Avoid sequential IDs if possible (use UUIDs or hashes)
- Entity keys are hashed, so distribution is generally good

#### Lock Contention

The hot tier uses a two-level locking hierarchy:

1. **Shard-level RWMutex**: Protects entity lookup
2. **Entity-level RWMutex**: Protects feature access

This allows concurrent reads across entities, even within the same shard.

### Warm Tier Tuning

The warm tier uses BadgerDB for persistent storage.

#### Sync Interval

```yaml
storage:
  warm:
    sync_interval: 1s    # Default: 1 second
```

| Sync Interval | Trade-off |
|---------------|-----------|
| 100ms | Lower durability latency, higher I/O |
| 1s | Balanced (default) |
| 5s | Higher throughput, risk of data loss |
| 10s+ | Maximum throughput, higher risk |

#### Disk I/O Optimization

**Recommended filesystem settings:**

```bash
# For ext4/xfs on Linux
echo 'deadline' > /sys/block/sda/queue/scheduler

# Mount options for SSD
mount -o noatime,nodiratime,discard /dev/sda1 /var/lib/feather
```

**Storage recommendations:**
- NVMe SSD for production
- Separate disk for data directory
- Minimum 100GB free space recommended

#### Historical Data Retention

```yaml
storage:
  historical:
    enabled: true
    retention: 720h    # 30 days
```

Disable if not using point-in-time queries:

```yaml
storage:
  historical:
    enabled: false
```

### Memory Management

#### GC Tuning

Feather uses object pools to reduce GC pressure:

```go
// Feature name slice pool (capacity: 32)
featureNameSlicePool

// Entity key slice pool (capacity: 64)
entityKeySlicePool
```

**Recommended GOGC settings:**

```bash
# For low-latency workloads (more memory, less GC pauses)
export GOGC=200

# For memory-constrained environments
export GOGC=50
```

**GOMEMLIMIT (Go 1.19+):**

```bash
# Set soft memory limit to 80% of container limit
export GOMEMLIMIT=6400MiB  # For 8GB container
```

#### Arena Allocator

The hot tier uses an arena allocator for reducing allocation overhead:

```go
arena := NewArena(1024 * 1024)  // 1MB chunks
```

This is pre-configured and doesn't require tuning.

---

## Server Configuration

### HTTP Server

```yaml
serving:
  http:
    port: 8080
    read_timeout: 10s
    write_timeout: 10s
```

#### Timeout Tuning

| Use Case | Read Timeout | Write Timeout |
|----------|--------------|---------------|
| Real-time serving | 1-5s | 1-5s |
| Batch operations | 30-60s | 30-60s |
| Large vector searches | 10-30s | 10-30s |

```bash
export FEATHER_HTTP_READ_TIMEOUT=5s
export FEATHER_HTTP_WRITE_TIMEOUT=5s
```

#### Compression

HTTP responses are automatically compressed with gzip. For bandwidth-constrained environments, this provides 60-80% reduction for JSON responses.

### gRPC Server

```yaml
serving:
  grpc:
    port: 50051
    max_concurrent: 1000
```

#### Concurrent Streams

```yaml
serving:
  grpc:
    max_concurrent: 5000    # For high-throughput scenarios
```

**Sizing guidelines:**

| Concurrent Clients | max_concurrent |
|-------------------|----------------|
| < 100 | 500-1000 |
| 100-1000 | 1000-5000 |
| > 1000 | 5000-10000 |

### Connection Handling

#### TCP Keepalive

For long-lived connections, enable TCP keepalive:

```bash
# Linux kernel parameters
sysctl -w net.ipv4.tcp_keepalive_time=60
sysctl -w net.ipv4.tcp_keepalive_intvl=10
sysctl -w net.ipv4.tcp_keepalive_probes=6
```

#### File Descriptor Limits

```bash
# Increase file descriptor limits for high concurrency
ulimit -n 100000

# Persistent configuration (/etc/security/limits.conf)
feather soft nofile 100000
feather hard nofile 100000
```

---

## Query Optimization

### Batch Operations

Always prefer batch operations over individual requests:

```python
# Good: Single batch request
features = client.get_features_batch(
    entities=["user:1", "user:2", "user:3", ...],  # Up to 1000 entities
    features=["score", "tier"]
)

# Bad: N individual requests
for user_id in user_ids:
    features = client.get_features(f"user:{user_id}", ["score", "tier"])
```

**Batch size recommendations:**

| Scenario | Batch Size |
|----------|------------|
| Few features, many entities | 500-1000 |
| Many features, few entities | 100-500 |
| Large feature values (vectors) | 50-100 |

### Feature Selection

Only request features you need:

```python
# Good: Specific features
features = client.get_features("user:123", ["score", "tier"])

# Avoid: All features (if you don't need them all)
features = client.get_all_features("user:123")
```

### Caching Strategies

#### Client-Side Caching

For features that don't change frequently:

```python
from functools import lru_cache

@lru_cache(maxsize=10000)
def get_user_tier(user_id: str) -> str:
    features = client.get_features(f"user:{user_id}", ["tier"])
    return features["tier"].value
```

#### TTL-Based Caching

```python
from cachetools import TTLCache

cache = TTLCache(maxsize=10000, ttl=60)  # 60 second TTL

def get_features_cached(entity: str, features: list[str]):
    key = (entity, tuple(features))
    if key not in cache:
        cache[key] = client.get_features(entity, features)
    return cache[key]
```

---

## Ingestion Optimization

### Kafka Configuration

```yaml
ingestion:
  kafka:
    enabled: true
    brokers:
      - kafka-1:9092
      - kafka-2:9092
      - kafka-3:9092
    topic: feature-updates
    consumer_group: feather
```

#### Partition Strategy

For optimal ingestion throughput:

1. **Partition by entity key**: Ensures ordering per entity
2. **Partition count**: Match to number of Feather instances

```
Partitions = Number of Feather Instances × 2-4
```

#### Consumer Tuning

```properties
# Kafka consumer settings
fetch.min.bytes=1048576
fetch.max.wait.ms=500
max.poll.records=500
```

#### Circuit Breaker

Feather includes a circuit breaker for Kafka resilience:

- **Threshold**: 5 consecutive failures
- **Recovery**: Half-open after 30 seconds
- **Behavior**: Logs errors, doesn't crash

### HTTP Ingestion

```yaml
ingestion:
  http:
    enabled: true
    port: 8081
```

#### Rate Limiting

HTTP ingestion includes token bucket rate limiting per client IP.

**For high-volume ingestion, use bulk endpoint:**

```bash
# Single update
POST /ingest
{"entity_key": "user:123", "features": {"score": 0.95}}

# Bulk updates (preferred)
POST /ingest/bulk
[
  {"entity_key": "user:1", "features": {"score": 0.95}},
  {"entity_key": "user:2", "features": {"score": 0.87}},
  ...
]
```

### Batch Ingestion

For DataFrame operations, tune the batch size:

```python
from feather_client.dataframe import DataFrameClient

df_client = DataFrameClient("http://localhost:8080")

# Tune batch size based on feature size
df_client.put_features_df(
    df,
    entity_column="user_id",
    batch_size=5000  # Larger for small features
)
```

| Feature Size | Recommended Batch Size |
|--------------|----------------------|
| Small (< 100 bytes) | 5000-10000 |
| Medium (100-1000 bytes) | 1000-5000 |
| Large (> 1KB, vectors) | 100-500 |

---

## Vector Search Optimization

### Index Configuration

```python
client.vectors.create_index(
    name="embeddings",
    dimension=384,
    distance_type="cosine"  # cosine, euclidean, or dot_product
)
```

#### Distance Type Selection

| Distance Type | Use Case | Performance |
|--------------|----------|-------------|
| `cosine` | Text embeddings, normalized vectors | Fast |
| `euclidean` | Image embeddings, spatial data | Medium |
| `dot_product` | Pre-normalized vectors, recommendations | Fastest |

### Search Parameters

```python
results = client.vectors.search(
    index="embeddings",
    vector=query_vector,
    top_k=10,
    ef=100,  # Search expansion factor
    include_metadata=True,
    include_vectors=False  # Don't return vectors unless needed
)
```

#### EF (Expansion Factor) Tuning

| Accuracy Need | EF Value | Latency Impact |
|---------------|----------|----------------|
| Low (draft) | 50 | Fastest |
| Medium | 100 | Balanced |
| High | 200-400 | Slower |
| Maximum | 500+ | Slowest |

**Rule of thumb:** `ef >= top_k` and typically `ef = top_k * 10` for good recall.

### Batch Vector Operations

```python
# Batch upsert for better throughput
batch_size = 100
for i in range(0, len(vectors), batch_size):
    batch = vectors[i:i+batch_size]
    client.vectors.upsert("embeddings", batch)
```

---

## Client-Side Optimization

### Connection Pooling

#### Python Async Client

```python
from feather_client import AsyncFeatherClient

client = AsyncFeatherClient(
    "http://localhost:8080",
    max_connections=100,           # Total connections
    max_keepalive_connections=50,  # Idle connections
    keepalive_expiry=30.0,         # Connection lifetime
)
```

#### Go Client

```go
client, err := feather.NewClient(
    feather.WithAddress("localhost:8080"),
    feather.WithPoolSize(100),
    feather.WithMaxIdleConns(50),
)
```

### Retry Configuration

```python
client = AsyncFeatherClient(
    "http://localhost:8080",
    max_retries=3,
    retry_delay=0.1,  # Exponential backoff
)
```

### Parallel Requests

```python
import asyncio
from feather_client import AsyncFeatherClient

async def fetch_all(user_ids: list[str]):
    async with AsyncFeatherClient("http://localhost:8080") as client:
        # Limit concurrency with semaphore
        semaphore = asyncio.Semaphore(100)

        async def fetch_one(uid: str):
            async with semaphore:
                return await client.get_features(f"user:{uid}", ["score"])

        results = await asyncio.gather(*[
            fetch_one(uid) for uid in user_ids
        ])
        return results
```

---

## Monitoring and Profiling

### Prometheus Metrics

```yaml
metrics:
  prometheus:
    enabled: true
    port: 9090
```

**Key metrics to monitor:**

| Metric | Description | Alert Threshold |
|--------|-------------|-----------------|
| `feather_http_request_duration_seconds` | Request latency | P99 > 10ms |
| `feather_hot_tier_hits_total` | Cache hits | Hit rate < 90% |
| `feather_hot_tier_misses_total` | Cache misses | - |
| `feather_hot_tier_evictions_total` | Evictions | Sudden spikes |
| `feather_hot_tier_size_bytes` | Memory usage | > 90% of max |
| `feather_warm_tier_read_duration_seconds` | Disk read latency | P99 > 50ms |

### Grafana Dashboard

Import the Feather dashboard from `deploy/grafana/feather-dashboard.json` or create queries:

```promql
# Cache hit rate
sum(rate(feather_hot_tier_hits_total[5m])) /
(sum(rate(feather_hot_tier_hits_total[5m])) +
 sum(rate(feather_hot_tier_misses_total[5m])))

# Request latency P99
histogram_quantile(0.99, sum(rate(feather_http_request_duration_seconds_bucket[5m])) by (le))

# Throughput
sum(rate(feather_http_requests_total[5m]))
```

### Tracing

```yaml
tracing:
  enabled: true
  endpoint: jaeger:4317
  service_name: feather
  sample_rate: 0.1    # 10% sampling for production
```

For debugging specific issues, temporarily increase sample rate:

```yaml
tracing:
  sample_rate: 1.0    # 100% for debugging
```

### Go pprof

Enable pprof endpoints for profiling:

```bash
# CPU profile
curl -o cpu.prof http://localhost:8080/debug/pprof/profile?seconds=30
go tool pprof cpu.prof

# Heap profile
curl -o heap.prof http://localhost:8080/debug/pprof/heap
go tool pprof heap.prof

# Goroutine dump
curl http://localhost:8080/debug/pprof/goroutine?debug=2
```

---

## Benchmarking

### Running Built-in Benchmarks

```bash
# Run all benchmarks
make bench

# Run specific benchmark
go test -bench=BenchmarkStore_Get -benchmem ./internal/storage/...

# Run with CPU profiling
go test -bench=BenchmarkStore_Get -cpuprofile=cpu.prof ./internal/storage/...
```

### Benchmark Results Interpretation

```
BenchmarkStore_Get-8    5000000    234 ns/op    48 B/op    2 allocs/op
                  │          │          │         │              │
                  │          │          │         │              └─ Allocations per op
                  │          │          │         └─ Bytes allocated per op
                  │          │          └─ Nanoseconds per operation
                  │          └─ Number of iterations
                  └─ Number of CPU cores
```

### Latency Testing

```bash
# Run latency tests
go test -v -run TestLatencyP99 ./test/...

# Expected output:
# Latency Distribution (n=10000):
#   P50: 45.2µs
#   P90: 89.1µs
#   P95: 112.3µs
#   P99: 287.4µs
#   Max: 1.2ms
```

### Load Testing with wrk

```bash
# Install wrk
brew install wrk  # macOS
apt install wrk   # Ubuntu

# GET benchmark
wrk -t12 -c400 -d30s "http://localhost:8080/v1/features?entity=user:1&feature=score"

# POST benchmark (with lua script)
wrk -t12 -c400 -d30s -s post.lua http://localhost:8080/v1/features
```

**post.lua:**
```lua
wrk.method = "POST"
wrk.headers["Content-Type"] = "application/json"
wrk.body = '{"entity_key":"user:1","features":{"score":0.95}}'
```

### Comparing Configurations

```bash
# Baseline
go test -bench=. -count=10 ./... > baseline.txt

# After changes
go test -bench=. -count=10 ./... > optimized.txt

# Compare
benchstat baseline.txt optimized.txt
```

---

## Troubleshooting Performance

### High Latency

**Symptom:** P99 latency exceeds targets

**Diagnosis:**
```bash
# Check cache hit rate
curl http://localhost:9090/metrics | grep hot_tier

# Check disk I/O
iostat -x 1

# Check goroutine count
curl http://localhost:8080/debug/pprof/goroutine?debug=1 | head
```

**Solutions:**

1. **Low cache hit rate (< 90%)**
   - Increase `hot.max_memory`
   - Review access patterns for better entity key design

2. **High disk latency**
   - Use NVMe SSD
   - Increase `warm.sync_interval`
   - Check disk space and I/O wait

3. **Too many goroutines**
   - Reduce `grpc.max_concurrent`
   - Add client-side rate limiting

### High Memory Usage

**Symptom:** Memory approaching limits or OOM

**Diagnosis:**
```bash
# Check current memory
curl http://localhost:9090/metrics | grep hot_tier_size

# Heap profile
curl http://localhost:8080/debug/pprof/heap > heap.prof
go tool pprof heap.prof
```

**Solutions:**

1. Reduce `hot.max_memory`
2. Enable aggressive TTL on feature groups
3. Review feature value sizes (vectors, large strings)

### Low Throughput

**Symptom:** Requests per second below expectations

**Diagnosis:**
```bash
# CPU profile
curl -o cpu.prof http://localhost:8080/debug/pprof/profile?seconds=30
go tool pprof cpu.prof

# Check CPU usage
top -p $(pgrep feather)
```

**Solutions:**

1. **CPU-bound**
   - Scale horizontally (more instances)
   - Review aggregation computations
   - Disable tracing/logging if not needed

2. **I/O-bound**
   - Use faster storage
   - Increase batch sizes
   - Add more Kafka partitions

### Connection Errors

**Symptom:** Connection refused or timeout errors

**Diagnosis:**
```bash
# Check open connections
ss -tuln | grep 8080

# Check file descriptors
ls /proc/$(pgrep feather)/fd | wc -l
```

**Solutions:**

1. Increase file descriptor limits
2. Reduce client connection pool sizes
3. Enable connection keepalive
4. Check network firewall rules

---

## Configuration Reference

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `FEATHER_HOT_MAX_MEMORY` | 4GB | Hot tier memory limit |
| `FEATHER_HOT_EVICTION` | lru | Eviction policy |
| `FEATHER_WARM_PATH` | /var/lib/feather/data | BadgerDB path |
| `FEATHER_WARM_SYNC_INTERVAL` | 1s | Disk sync interval |
| `FEATHER_HTTP_PORT` | 8080 | HTTP API port |
| `FEATHER_GRPC_PORT` | 50051 | gRPC API port |
| `FEATHER_GRPC_MAX_CONCURRENT` | 1000 | Max concurrent streams |
| `FEATHER_HTTP_READ_TIMEOUT` | 10s | HTTP read timeout |
| `FEATHER_HTTP_WRITE_TIMEOUT` | 10s | HTTP write timeout |
| `FEATHER_PROMETHEUS_PORT` | 9090 | Metrics port |
| `FEATHER_LOG_LEVEL` | info | Logging level |
| `FEATHER_TRACING_ENABLED` | false | Enable tracing |
| `FEATHER_TRACING_SAMPLE_RATE` | 0.1 | Trace sample rate |

### Complete Example Configuration

```yaml
# Production configuration for high-throughput serving

schema:
  groups:
    - name: user_features
      entity_type: user
      ttl: 24h
      features:
        - name: score
          data_type: float64
        - name: embedding
          data_type: vector
          dimensions: [384]

storage:
  hot:
    max_memory: 16GB
    eviction_policy: lru
  warm:
    path: /var/lib/feather/data
    sync_interval: 1s
  historical:
    enabled: true
    retention: 720h

ingestion:
  kafka:
    enabled: true
    brokers:
      - kafka-1:9092
      - kafka-2:9092
    topic: feature-updates
    consumer_group: feather
  http:
    enabled: true
    port: 8081

serving:
  grpc:
    port: 50051
    max_concurrent: 5000
  http:
    port: 8080
    read_timeout: 5s
    write_timeout: 5s

metrics:
  prometheus:
    enabled: true
    port: 9090

logging:
  level: info
  format: json

tracing:
  enabled: true
  endpoint: jaeger:4317
  service_name: feather
  sample_rate: 0.1
```

---

## Binary Size

The default Feather binary is ~40MB because it statically links all extension,
platform, and integration packages. This is intentional — a single binary
simplifies deployment (see [ADR-0017](./adr/0017-single-binary-deployment.md)).

To inspect what contributes to binary size:

```bash
go build -o bin/feather ./cmd/feather
go tool nm -size bin/feather | sort -k2 -rn | head -20
```

Stripping debug symbols (the default in `make build`) reduces size by ~30%.
The `-ldflags "-s -w"` flags are already set in the Makefile.

## Next Steps

- Review the [Architecture Overview](./architecture.md) for system design details
- See [Deployment Guide](./deployment.md) for production setup
- Check [API Reference](./api-reference.md) for endpoint documentation
