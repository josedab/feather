---
title: "ADR-0002: Sharded In-Memory Cache"
sidebar_label: "0002: Sharded Cache"
---

# ADR-0002: Sharded In-Memory Cache Design

## Status

Accepted

## Context

The hot tier (ADR-0001) requires an in-memory cache that can handle:

1. **High concurrency**: Thousands of concurrent readers and writers
2. **Low latency**: Sub-millisecond access even under contention
3. **Memory efficiency**: Minimal overhead per cached item
4. **Predictable eviction**: LRU semantics to keep hot data in cache

A naive single-lock cache would serialize all operations, creating a bottleneck. Go's `sync.Map` is optimized for read-heavy workloads with few writes, but feature stores have significant write traffic from ingestion. We needed a design that scales with CPU cores.

## Decision

We implement a **256-shard LRU cache** with the following characteristics:

### Sharding Strategy
- **256 shards**: Chosen as a power of 2 for fast modulo (bitwise AND)
- **FNV-1a hashing**: Fast, well-distributed hash function for entity keys
- **Shard selection**: `shard = fnv1a(entityKey) & 0xFF`

### Per-Shard Structure
Each shard contains:
- `sync.RWMutex` for shard-level locking
- LRU doubly-linked list for eviction ordering
- Hash map for O(1) key lookup
- Per-entity locks for fine-grained write coordination

### Locking Hierarchy
```
Shard Lock (RWMutex)
    └── Entity Lock (Mutex, optional)
```

- **Reads**: Acquire shard read lock, lookup, release
- **Writes**: Acquire shard write lock, acquire entity lock, mutate, release both
- **Eviction**: Acquire shard write lock, evict LRU entries, release

### Memory Management
- **Object pooling**: Reuse allocated objects to reduce GC pressure
- **Approximate size tracking**: Atomic counters track memory usage without locking
- **Lazy eviction**: Triggered when size exceeds threshold, not on every write

### Atomic Metrics
All statistics use `atomic.Int64` for lock-free updates:
- Cache hits/misses
- Eviction count
- Current size (approximate)

## Consequences

### Positive

- **Linear scalability**: Throughput scales with shard count (up to 256 concurrent operations)
- **Low contention**: 256 shards means less than 1% collision probability for random keys
- **Predictable latency**: No single lock bottleneck; P99 stays low under load
- **Memory-aware**: Eviction prevents OOM while keeping hot data
- **Lock-free reads**: RWMutex allows concurrent readers within a shard

### Negative

- **Memory overhead**: 256 shards × (mutex + map + list) adds ~50KB baseline
- **Complexity**: More moving parts than a simple cache
- **Approximate sizing**: Memory tracking is not exact (acceptable tradeoff)
- **Shard imbalance**: Poor hash distribution could create hot shards (FNV-1a mitigates)

### Neutral

- **256 is not configurable**: Hardcoded for simplicity; could be made configurable if needed
- **No inter-shard LRU**: Each shard has independent LRU; global LRU would require coordination

## Alternatives Considered

| Alternative | Rejected Because |
|-------------|------------------|
| `sync.Map` | Poor write performance; no LRU eviction |
| Single-lock cache | Serializes all operations; doesn't scale |
| Channel-based cache | Message passing overhead; complex implementation |
| Third-party cache (BigCache, FreeCache) | External dependency; less control over behavior |

## Implementation Notes

Key file: `internal/storage/hot.go`

```go
const numShards = 256

type HotTier struct {
    shards [numShards]*shard
    // ...
}

type shard struct {
    mu      sync.RWMutex
    items   map[string]*entry
    lruHead *entry
    lruTail *entry
}

func (h *HotTier) shardFor(key string) *shard {
    hash := fnv1a(key)
    return h.shards[hash&0xFF]
}
```
