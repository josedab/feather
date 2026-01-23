# ADR-0006: Sliding Window Aggregation with Ring Buffers

## Status

Accepted

## Context

Feature stores need to compute real-time aggregations over time windows:

- "Count of clicks in the last hour"
- "Average transaction amount in the last 24 hours"
- "Maximum session duration in the last 7 days"

These aggregations must be:
1. **Fast**: Computed on-demand without scanning all raw data
2. **Memory-efficient**: Can't store every raw event
3. **Accurate**: Results must be correct within acceptable bounds
4. **Concurrent**: Multiple readers and writers simultaneously

Naive approaches fail at scale:
- **Store all events**: Memory explodes with high-cardinality entities
- **Recompute on read**: Latency grows with window size
- **Periodic batch**: Stale results, not real-time

## Decision

We implement **sliding window aggregations using pre-bucketed ring buffers**:

### Core Data Structure

```
Window: 1 hour, Buckets: 60 (1-minute granularity)

Ring Buffer (circular):
┌────┬────┬────┬────┬────┬────┬────┬────┐
│ B0 │ B1 │ B2 │...│B57 │B58 │B59 │ B0 │ ← wraps
└────┴────┴────┴────┴────┴────┴────┴────┘
  ↑                              ↑
  oldest                         newest (head)
```

Each bucket stores pre-aggregated values:
```go
type Bucket struct {
    Count     int64
    Sum       float64
    Min       float64
    Max       float64
    Timestamp int64  // Bucket start time
}
```

### Aggregation Flow

**On Write**:
1. Determine target bucket from event timestamp
2. Update bucket's pre-aggregated values (atomic)
3. O(1) operation regardless of data volume

**On Read**:
1. Scan buckets within query window
2. Combine bucket aggregates
3. O(buckets) operation, typically O(60) for hourly windows

### Supported Aggregations

| Function | Computation | Combinable? |
|----------|-------------|-------------|
| `count`  | Sum of counts | Yes |
| `sum`    | Sum of sums | Yes |
| `avg`    | Sum / Count | Yes |
| `min`    | Min of mins | Yes |
| `max`    | Max of maxes | Yes |
| `last`   | Most recent value | Special |

### Window Manager

Each entity-feature pair has a window manager:
```
Entity "user:123"
  └── Feature "click_count"
        └── WindowManager
              ├── 1h window → RingBuffer(60 buckets)
              └── 24h window → RingBuffer(288 buckets)
```

## Consequences

### Positive

- **O(buckets) read**: Constant time regardless of event volume
- **O(1) write**: Single bucket update per event
- **Memory bounded**: Fixed memory per window, not per event
- **Composable**: Can compute aggregates of aggregates
- **Concurrent-safe**: Atomic bucket updates, lock-free reads

### Negative

- **Bucket granularity**: Results are approximate within bucket boundaries
- **Memory per feature**: Each window requires dedicated buffer
- **No percentiles**: Cannot compute P50/P99 without storing all values
- **Clock sensitivity**: Bucket assignment depends on accurate timestamps

### Neutral

- **Trade-off**: Memory vs. accuracy (more buckets = more memory, better accuracy)
- **Sliding vs. tumbling**: We chose sliding windows; tumbling would be simpler

## Alternatives Considered

| Alternative | Rejected Because |
|-------------|------------------|
| Store all events | Memory grows unbounded |
| Redis Sorted Sets | External dependency; network latency |
| HyperLogLog | Only for cardinality, not general aggregations |
| T-Digest | Complex; overkill for supported functions |

## Implementation Notes

### Ring Buffer

Key file: `internal/aggregation/ring_buffer.go`

```go
type RingBuffer struct {
    buckets    []Bucket
    head       int
    bucketSize time.Duration
    mu         sync.RWMutex
}

func (rb *RingBuffer) Add(value float64, timestamp time.Time) {
    rb.mu.Lock()
    defer rb.mu.Unlock()

    bucketIdx := rb.bucketIndex(timestamp)
    rb.advanceHead(timestamp)

    rb.buckets[bucketIdx].Count++
    rb.buckets[bucketIdx].Sum += value
    rb.buckets[bucketIdx].Min = min(rb.buckets[bucketIdx].Min, value)
    rb.buckets[bucketIdx].Max = max(rb.buckets[bucketIdx].Max, value)
}

func (rb *RingBuffer) Aggregate(fn AggFunction, window time.Duration) float64 {
    rb.mu.RLock()
    defer rb.mu.RUnlock()

    // Scan relevant buckets and combine
    var count int64
    var sum float64
    // ...
}
```

### Aggregation Engine

Key file: `internal/aggregation/engine.go`

```go
type Engine struct {
    managers map[string]map[string]*WindowManager  // entity → feature → manager
    mu       sync.RWMutex
}

func (e *Engine) Record(entityID, feature string, value float64, ts time.Time) {
    manager := e.getOrCreateManager(entityID, feature)
    manager.Record(value, ts)
}

func (e *Engine) Compute(entityID, feature string, fn AggFunction, window time.Duration) (float64, error) {
    manager := e.getManager(entityID, feature)
    if manager == nil {
        return 0, ErrNotFound
    }
    return manager.Compute(fn, window), nil
}
```

### Configuration

```yaml
aggregation:
  default_buckets: 60
  windows:
    - duration: 1h
      slide_by: 1m
    - duration: 24h
      slide_by: 5m
    - duration: 7d
      slide_by: 1h
```
