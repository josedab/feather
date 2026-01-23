---
title: "ADR-0016: Object Pooling"
sidebar_label: "0016: Object Pooling"
---

# ADR-0016: Object Pooling for High-Throughput Paths

## Status

Accepted

## Context

Feather's hot tier targets sub-millisecond P99 latency. At high throughput (100K+ requests/second), Go's garbage collector becomes a significant factor:

1. **Allocation pressure**: Each request allocates slices, maps, and buffers
2. **GC pauses**: While typically under 1ms, GC can cause latency spikes
3. **Memory churn**: Short-lived allocations increase GC frequency
4. **CPU overhead**: Allocation and GC consume CPU cycles

Profiling revealed hotspots:
```
flat  flat%   cum%   function
12.5%  12.5%  15.2%  runtime.mallocgc
 8.3%   8.3%  23.5%  runtime.gcDrain
 6.2%   6.2%  29.7%  encoding/json.Marshal
```

We needed to reduce allocation pressure without sacrificing code clarity.

## Decision

We use **`sync.Pool`** for object reuse in high-throughput code paths.

### Pooled Objects

| Pool | Object Type | Usage |
|------|-------------|-------|
| `featureNameSlicePool` | `[]string` | Feature name lists for batch ops |
| `entityKeySlicePool` | `[]string` | Entity key lists for iterations |
| `keyBytesPool` | `[]byte` | Key buffers for warm tier |
| `jsonBufferPool` | `*bytes.Buffer` | JSON encoding buffers |
| `featureMapPool` | `map[string]FeatureValue` | Result maps |

### Pool Implementation Pattern

```go
var featureNameSlicePool = sync.Pool{
    New: func() interface{} {
        // Pre-allocate reasonable capacity
        s := make([]string, 0, 32)
        return &s
    },
}

// Acquire from pool
func acquireFeatureNames() *[]string {
    return featureNameSlicePool.Get().(*[]string)
}

// Return to pool (reset state)
func releaseFeatureNames(s *[]string) {
    *s = (*s)[:0]  // Reset length, keep capacity
    featureNameSlicePool.Put(s)
}
```

### Safety Rules

1. **Always defer release**: Prevents leaks on error paths
2. **Reset before release**: Clear sensitive data, reset length
3. **Don't pool return values**: Only pool internal working memory
4. **Size appropriately**: Pre-allocate common capacity to avoid regrowth

## Consequences

### Positive

- **Reduced allocations**: 60-80% fewer allocations in hot paths
- **Lower GC pressure**: Less frequent GC cycles
- **Consistent latency**: Fewer GC-induced spikes
- **Reused capacity**: Slices grow once, stay grown
- **CPU efficiency**: Less time in `runtime.mallocgc`

### Negative

- **Code complexity**: Must remember to acquire/release
- **Potential leaks**: Forgetting to release wastes memory
- **Subtle bugs**: Using object after release causes data races
- **Pool contention**: Under extreme load, pool access can serialize
- **Memory retention**: Pools hold memory even when idle

### Neutral

- **Not a silver bullet**: Helps hot paths; cold paths don't benefit
- **Profiling required**: Must measure to identify what's worth pooling

## Benchmark Results

Before pooling:
```
BenchmarkGetBatch/1000_entities-8    5000    312456 ns/op    89432 B/op    1847 allocs/op
```

After pooling:
```
BenchmarkGetBatch/1000_entities-8    8000    187234 ns/op    24128 B/op     412 allocs/op
```

**Results**:
- 40% latency reduction
- 73% fewer allocations
- 73% less memory allocated per operation

## Common Pitfalls

```go
// WRONG: Using pooled object after release
names := AcquireFeatureNames()
ReleaseFeatureNames(names)
doSomething(*names)  // BUG: names may be reused

// CORRECT: Release after all usage
names := AcquireFeatureNames()
defer ReleaseFeatureNames(names)
doSomething(*names)  // Safe: released after function returns

// WRONG: Returning pooled object to caller
func getNames() *[]string {
    names := AcquireFeatureNames()
    *names = append(*names, "a", "b", "c")
    return names  // BUG: caller might not release
}

// CORRECT: Copy to non-pooled slice for return
func getNames() []string {
    names := AcquireFeatureNames()
    defer ReleaseFeatureNames(names)
    *names = append(*names, "a", "b", "c")
    result := make([]string, len(*names))
    copy(result, *names)
    return result
}
```
