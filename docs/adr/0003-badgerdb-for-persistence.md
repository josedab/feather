# ADR-0003: BadgerDB for Embedded Persistence

## Status

Accepted

## Context

The warm tier (ADR-0001) requires persistent storage with:

1. **Embedded operation**: No external database process to manage
2. **Key-value semantics**: Feature storage maps naturally to KV
3. **Historical versions**: Support point-in-time queries
4. **Good write performance**: Handle ingestion throughput
5. **Compression**: Reduce disk footprint
6. **Pure Go**: Simplify deployment (no CGo dependencies)

We evaluated several embedded databases for Go applications.

## Decision

We chose **BadgerDB v4** as our persistent storage engine.

### Key Characteristics

- **LSM-tree architecture**: Optimized for write-heavy workloads
- **Pure Go implementation**: No CGo, simple cross-compilation
- **Built-in compression**: Snappy/ZSTD support
- **Transactions**: ACID guarantees with MVCC
- **TTL support**: Native expiration without manual cleanup

### Key Schema Design

Features are stored with composite keys enabling versioned access:

```
Key format: {entity_type}:{entity_id}:{feature_name}:{version_timestamp}
Example:    user:12345:click_count:1699876543000000000
```

- **Prefix scans**: Efficiently retrieve all features for an entity
- **Version ordering**: Timestamps enable point-in-time queries
- **TTL**: Set per-key expiration for automatic cleanup

### Configuration

```go
opts := badger.DefaultOptions(path).
    WithCompression(options.ZSTD).
    WithBlockCacheSize(256 << 20).     // 256MB block cache
    WithIndexCacheSize(128 << 20).     // 128MB index cache
    WithValueLogFileSize(256 << 20).   // 256MB value log files
    WithNumVersionsToKeep(100).        // Historical versions
    WithSyncWrites(false)              // Async for performance
```

## Consequences

### Positive

- **No ops burden**: Embedded database; no separate process to manage
- **Simple deployment**: Single binary includes storage
- **Pure Go**: Cross-compiles easily; no CGo headaches
- **Write performance**: LSM-tree handles high ingestion rates
- **Compression**: 3-5x storage reduction with ZSTD
- **Battle-tested**: Powers Dgraph; production-proven

### Negative

- **Memory usage**: LSM-tree requires memory for write buffer and caches
- **Compaction spikes**: Background compaction can cause latency variance
- **Value log GC**: Requires periodic garbage collection of old values
- **Learning curve**: Different mental model than B-tree databases

### Neutral

- **Not distributed**: Single-node only; clustering handled at application layer
- **No SQL**: Pure KV interface; complex queries done in application code

## Alternatives Considered

| Alternative | Rejected Because |
|-------------|------------------|
| **BoltDB** | B-tree is slower for writes; project in maintenance mode |
| **RocksDB** | CGo dependency complicates builds and deployment |
| **SQLite** | Relational model adds overhead; CGo required |
| **LevelDB** | CGo wrapper; fewer features than BadgerDB |
| **Pebble** | Less mature; CockroachDB-specific optimizations |

## Implementation Notes

Key file: `internal/core/storage/warm.go`

```go
type WarmTier struct {
    db     *badger.DB
    opts   WarmOptions
    pool   *sync.Pool  // Buffer reuse
}

func (w *WarmTier) Get(entityID string, features []string) (map[string]domain.FeatureValue, error) {
    return w.db.View(func(txn *badger.Txn) error {
        // Prefix scan for entity's features
        prefix := []byte(entityID + ":")
        opts := badger.DefaultIteratorOptions
        opts.Prefix = prefix
        // ...
    })
}

func (w *WarmTier) GetAsOf(entityID string, features []string, asOf time.Time) (map[string]domain.FeatureValue, error) {
    // Query with version timestamp filter
    // Return latest version <= asOf
}
```

### Operational Considerations

- **Compaction**: Run `db.RunValueLogGC(0.5)` periodically
- **Backup**: Use `db.Backup()` for point-in-time snapshots
- **Monitoring**: Expose BadgerDB metrics via Prometheus
