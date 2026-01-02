# ADR-0001: Tiered Storage Architecture (Hot/Warm)

## Status

Accepted

## Context

Feather is a feature store designed to serve machine learning features with sub-millisecond latency while maintaining data durability. Feature stores face a fundamental tension between:

1. **Speed**: ML inference requires features in <1ms to avoid becoming a bottleneck
2. **Durability**: Features must survive process restarts and be recoverable
3. **Historical Access**: Point-in-time queries need access to past feature versions
4. **Cost**: Keeping all data in memory is expensive at scale

A single storage tier cannot satisfy all these requirements. Pure in-memory storage is fast but volatile. Pure disk-based storage is durable but too slow for real-time serving. We needed an architecture that optimizes for the common case (recent features, hot entities) while still supporting the full range of access patterns.

## Decision

We implement a **two-tier storage architecture**:

### Hot Tier (In-Memory)
- LRU-based cache storing the most recently accessed features
- Target latency: <1ms P99
- Configurable memory limit (default 4GB)
- Automatic eviction when memory pressure increases

### Warm Tier (Persistent)
- BadgerDB-backed storage for all features with historical versions
- Target latency: ~10ms P99
- Supports point-in-time queries via versioned keys
- Compression enabled for storage efficiency

### Coordination
The `Store` type coordinates both tiers with the following semantics:

**Read Path** (cache-through):
```
Request → Hot Tier (hit?) → return
                 ↓ (miss)
         Warm Tier → populate Hot → return
```

**Write Path** (write-through):
```
Request → Hot Tier (sync) → return to caller
              ↓ (async)
          Warm Tier
```

This design ensures:
- Reads always check the hot tier first
- Cache misses transparently fall through to warm tier
- Writes are immediately visible in hot tier
- Warm tier writes happen asynchronously to avoid blocking

## Consequences

### Positive

- **Sub-millisecond serving**: Hot entities served entirely from memory
- **Durability**: All data persisted to disk, survives restarts
- **Historical queries**: Warm tier maintains versioned history
- **Memory efficiency**: Only hot data consumes RAM; cold data stays on disk
- **Graceful degradation**: If hot tier is full, system continues working (slower)
- **Simple mental model**: Developers think in terms of "cache + database"

### Negative

- **Complexity**: Two storage systems to configure, monitor, and debug
- **Consistency window**: Brief period where hot and warm may diverge (async writes)
- **Cold start latency**: After restart, hot tier is empty; first requests hit warm tier
- **Memory sizing**: Requires capacity planning to size hot tier appropriately

### Neutral

- **Write amplification**: Each write goes to two places (acceptable for feature store workload)
- **Cache invalidation**: Not needed since we control all writes (no external mutations)

## Implementation Notes

Key files:
- `internal/storage/store.go` - Unified Store interface coordinating both tiers
- `internal/storage/hot.go` - In-memory LRU implementation
- `internal/storage/warm.go` - BadgerDB wrapper with versioning

Configuration:
```yaml
storage:
  hot:
    max_memory: 4GB
    default_ttl: 24h
  warm:
    path: /var/lib/feather/data
    sync_interval: 1s
    compression: true
```
