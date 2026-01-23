---
sidebar_position: 3
title: Performance Tuning
description: Optimize Feather for maximum throughput and minimum latency.
---

# Performance Tuning

This guide helps you optimize Feather for your specific workload, covering memory sizing, cache tuning, and operational best practices.

## Understanding Your Workload

Before tuning, characterize your workload:

| Metric | How to Measure | Impact |
|--------|----------------|--------|
| **Entity count** | Unique entities in hot tier | Memory sizing |
| **Features per entity** | Avg features requested | Latency |
| **Request rate** | QPS at peak | CPU sizing |
| **Read/write ratio** | Reads vs writes | Cache effectiveness |
| **Access pattern** | Skewed vs uniform | Cache hit rate |

### Workload Profiles

| Profile | Characteristics | Optimization Focus |
|---------|-----------------|-------------------|
| **Real-time serving** | High QPS, low latency | Hot tier size, sharding |
| **Training data** | Batch, point-in-time | Warm tier, export |
| **Mixed** | Both patterns | Balance both tiers |

## Memory Tuning

### Hot Tier Sizing

**Formula:**
```
Required memory = (active entities) × (features per entity) × (bytes per feature)
                + 20% overhead
```

**Example calculation:**
```
1M active entities
× 10 features per entity
× 100 bytes per feature
= 1 GB base

+ 20% overhead = 1.2 GB recommended hot tier

Set max_memory = 1.5 GB (with headroom)
```

### Configuration

```yaml
storage:
  hot:
    max_memory: "1536MB"  # 1.5 GB
    ttl: "2h"             # Evict unused after 2 hours
```

### Monitoring Memory

```promql
# Current usage percentage
feather_hot_tier_size_bytes / feather_hot_tier_max_bytes

# Eviction rate (high = undersized)
rate(feather_evictions_total[5m])

# Memory growth rate
deriv(feather_hot_tier_size_bytes[1h])
```

**Target: Keep usage below 80% to avoid frequent evictions.**

## Cache Optimization

### Maximizing Cache Hits

1. **Size appropriately**: Fit your working set in memory
2. **Set sensible TTL**: Match your access patterns
3. **Warm the cache**: Pre-load frequently accessed entities

### Cache Warming

```python
# Pre-load hot entities at startup
hot_entities = get_frequently_accessed_entities()

for entity in hot_entities:
    client.get_features(entity, feature_list)
```

### Measuring Cache Effectiveness

```promql
# Hit rate (target: > 90%)
rate(feather_cache_hits_total[5m]) /
(rate(feather_cache_hits_total[5m]) + rate(feather_cache_misses_total[5m]))
```

**If hit rate is low:**
- Increase `max_memory`
- Increase `ttl`
- Check for uniform access patterns (hard to cache)

## Latency Optimization

### Request Latency Breakdown

| Component | Typical | Optimized |
|-----------|---------|-----------|
| Network | 0.1ms | 0.1ms |
| HTTP parsing | 0.05ms | 0.05ms |
| Hot tier lookup | 0.1ms | 0.05ms |
| Response serialization | 0.1ms | 0.05ms |
| **Total** | **0.35ms** | **0.25ms** |

### Reducing Latency

**1. Batch requests:**
```bash
# Instead of 100 individual requests
for entity in entities:
    client.get_features(entity, features)

# Use batch API
client.get_features_batch(entities, features)
```

**2. Request only needed features:**
```bash
# Bad: request all features
curl "http://localhost:8080/v1/features?entity=user:123"

# Good: request specific features
curl "http://localhost:8080/v1/features?entity=user:123&feature=click_count&feature=purchase_total"
```

**3. Use gRPC for high-throughput:**
```go
// gRPC has lower overhead than HTTP for high-frequency calls
client, _ := feather.NewGRPCClient("localhost:50051")
```

### P99 Latency Issues

If P99 is significantly higher than P50:

1. **Check for GC pauses:**
   ```bash
   # Increase GOGC to reduce frequency
   GOGC=200 ./feather
   ```

2. **Check warm tier fallback:**
   ```promql
   # High miss rate causes P99 spikes
   rate(feather_cache_misses_total[5m])
   ```

3. **Check for lock contention:**
   ```bash
   # Profile with pprof
   go tool pprof http://localhost:8080/debug/pprof/mutex
   ```

## Throughput Optimization

### Scaling Reads

**Single node capacity:**
- Hot tier: 1M+ reads/sec
- Warm tier: 100K+ reads/sec

**To increase throughput:**

1. **Vertical scaling**: More CPU cores
2. **Horizontal scaling**: Multiple read replicas (roadmap)

### Scaling Writes

**Single node capacity:**
- 50K+ writes/sec

**Optimization:**

1. **Batch writes:**
   ```bash
   curl -X POST http://localhost:8080/v1/features/batch \
     -d '{"entities": [...], "features": [...]}'
   ```

2. **Use Kafka for high-volume:**
   ```yaml
   ingestion:
     kafka:
       enabled: true
       batch_size: 1000
   ```

## Go Runtime Tuning

### GOGC (Garbage Collection)

```bash
# Default: 100 (GC when heap doubles)
# Higher = less frequent GC, more memory, lower latency
GOGC=200 ./feather
```

| GOGC | Memory | GC Frequency | Latency |
|------|--------|--------------|---------|
| 50 | Lower | More frequent | Higher P99 |
| 100 | Default | Default | Default |
| 200 | Higher | Less frequent | Lower P99 |

### GOMAXPROCS

```bash
# Usually leave at default (all CPUs)
# Reduce if sharing machine with other services
GOMAXPROCS=8 ./feather
```

## BadgerDB Tuning

### Write Performance

```yaml
storage:
  warm:
    sync_writes: false     # Async for speed (default)
    value_log_file_size: 256MB
```

### Read Performance

```yaml
storage:
  warm:
    block_cache_size: 512MB   # Increase for read-heavy
    index_cache_size: 256MB
```

### Compaction

BadgerDB compacts automatically, but you can tune:

```yaml
storage:
  warm:
    compaction:
      level_size_multiplier: 10
      max_levels: 7
```

## Network Optimization

### Keep-Alive Connections

Enable connection pooling in clients:

```python
# Python
client = FeatherClient("localhost:8080", pool_size=10)
```

```go
// Go
transport := &http.Transport{
    MaxIdleConns:        100,
    MaxIdleConnsPerHost: 100,
    IdleConnTimeout:     90 * time.Second,
}
```

### Compression

For large responses, enable compression:

```yaml
server:
  http:
    compression: true
```

## Monitoring Performance

### Key Metrics Dashboard

```promql
# Request rate
sum(rate(feather_http_requests_total[5m]))

# P50/P99 latency
histogram_quantile(0.5, rate(feather_http_request_duration_seconds_bucket[5m]))
histogram_quantile(0.99, rate(feather_http_request_duration_seconds_bucket[5m]))

# Cache efficiency
rate(feather_cache_hits_total[5m]) / rate(feather_cache_hits_total[5m]) + rate(feather_cache_misses_total[5m])

# Memory pressure
feather_hot_tier_size_bytes / feather_hot_tier_max_bytes
```

### Performance Alerts

```yaml
# Alert on latency degradation
- alert: FeatherLatencyDegraded
  expr: |
    histogram_quantile(0.99, rate(feather_http_request_duration_seconds_bucket[5m])) > 0.005
  for: 5m

# Alert on throughput drop
- alert: FeatherThroughputDrop
  expr: |
    rate(feather_http_requests_total[5m]) < 1000
  for: 5m
```

## Benchmarking

### Built-in Benchmark

```bash
# Run benchmark suite
make bench

# Custom parameters
go test -bench=. -benchtime=30s ./test/benchmark/...
```

### Load Testing

```bash
# Using hey
hey -n 100000 -c 100 \
  "http://localhost:8080/v1/features?entity=user:123&feature=click_count"

# Using wrk
wrk -t12 -c400 -d30s \
  "http://localhost:8080/v1/features?entity=user:123&feature=click_count"
```

### Expected Results

On c5.4xlarge (16 vCPU, 32GB):

| Operation | P50 | P99 | Throughput |
|-----------|-----|-----|------------|
| Hot read | 50μs | 100μs | 1M/sec |
| Warm read | 500μs | 2ms | 100K/sec |
| Write | 100μs | 300μs | 50K/sec |

## Troubleshooting Performance

### High Latency

1. Check cache hit rate
2. Check GC pauses (`GODEBUG=gctrace=1`)
3. Profile with pprof
4. Check disk I/O (warm tier)

### Low Throughput

1. Check CPU utilization
2. Check for lock contention
3. Verify batch usage
4. Check network saturation

### Memory Issues

1. Check hot tier sizing
2. Monitor eviction rate
3. Profile memory allocation
4. Check for memory leaks

## Related Documentation

- [Architecture Overview](/docs/concepts/architecture)
- [Tiered Storage](/docs/concepts/tiered-storage)
- [Observability Guide](./observability)
