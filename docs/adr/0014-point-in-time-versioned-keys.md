# ADR-0014: Point-in-Time Queries via Versioned Keys

## Status

Accepted

## Context

Machine learning training requires **point-in-time correctness**: when training a model for a prediction made at time T, features must reflect only data available at time T. Using future data causes **data leakage**, resulting in overfitted models that fail in production.

Example: Predicting user churn at 2024-01-15 10:00 AM requires:
- Features computed from data available before that timestamp
- NOT features updated after the prediction time

Feature stores must support:
1. **Point-in-time queries**: "What were the features at timestamp T?"
2. **Historical feature retrieval**: For training data generation
3. **Efficient current-value access**: For real-time serving (most common case)

We needed a storage schema that supports both historical and current access patterns efficiently.

## Decision

We store feature values with **versioned keys using inverted timestamps**, enabling efficient point-in-time queries via prefix iteration.

### Key Schema

```
Current value:    c:{entity}:{feature}
Historical value: h:{entity}:{feature}:{inverted_timestamp}

Inverted timestamp = MaxInt64 - UnixNano(timestamp)
```

**Example**:
```
c:user:123:click_count                          → current value
h:user:123:click_count:9223370337854775807      → value at 2024-01-15T10:00:00Z
h:user:123:click_count:9223370337914775807      → value at 2024-01-14T10:00:00Z
h:user:123:click_count:9223370337974775807      → value at 2024-01-13T10:00:00Z
```

### Why Inverted Timestamps?

BadgerDB iterates keys in lexicographic (ascending) order. With inverted timestamps:
- **Most recent first**: Iterator returns newest values first
- **Efficient point-in-time**: Stop iteration at first key ≤ query timestamp
- **No reverse iteration**: Forward-only iteration is faster in LSM trees

```
Query: "click_count as of 2024-01-14T15:00:00Z"

Inverted query timestamp: 9223370337884775807

Scan: h:user:123:click_count:*
      ↓
      9223370337854775807 (2024-01-15) > query → skip
      9223370337884775807 (2024-01-14 15:00) ≤ query → FOUND
```

### Write Path

```go
func (w *WarmTier) Put(entityID, feature string, value FeatureValue) error {
    return w.db.Update(func(txn *badger.Txn) error {
        // 1. Write current value (fast path for serving)
        currentKey := fmt.Sprintf("c:%s:%s", entityID, feature)
        txn.Set([]byte(currentKey), encode(value))

        // 2. Write historical version (for point-in-time)
        invertedTS := math.MaxInt64 - value.Timestamp
        histKey := fmt.Sprintf("h:%s:%s:%019d", entityID, feature, invertedTS)
        txn.Set([]byte(histKey), encode(value))

        return nil
    })
}
```

### Read Paths

**Current value (real-time serving)**:
```go
func (w *WarmTier) Get(entityID, feature string) (FeatureValue, error) {
    key := fmt.Sprintf("c:%s:%s", entityID, feature)
    // Single key lookup - O(1)
}
```

**Point-in-time (training data)**:
```go
func (w *WarmTier) GetAsOf(entityID, feature string, asOf time.Time) (FeatureValue, error) {
    prefix := fmt.Sprintf("h:%s:%s:", entityID, feature)
    targetInverted := math.MaxInt64 - asOf.UnixNano()

    // Iterate until we find first key with inverted_ts >= targetInverted
    // (meaning actual timestamp <= asOf)
    opts := badger.DefaultIteratorOptions
    opts.Prefix = []byte(prefix)

    it := txn.NewIterator(opts)
    defer it.Close()

    for it.Rewind(); it.Valid(); it.Next() {
        key := it.Item().Key()
        keyInverted := parseInvertedTimestamp(key)

        if keyInverted >= targetInverted {
            // Found: this version existed at query time
            return decode(it.Item().Value())
        }
    }
    return FeatureValue{}, ErrNotFound
}
```

## Consequences

### Positive

- **Efficient point-in-time**: O(versions scanned) with early termination
- **No separate history table**: Single storage system for current and historical
- **Prefix locality**: All versions of a feature stored together (cache-friendly)
- **Simple cleanup**: TTL or prefix deletion removes old versions
- **Natural ordering**: Most recent versions accessed first (matches typical query pattern)

### Negative

- **Write amplification**: Each update writes 2 keys (current + historical)
- **Storage overhead**: Historical versions accumulate (mitigated by TTL)
- **Timestamp parsing**: Key parsing adds small overhead
- **19-digit formatting**: Large numbers require fixed-width formatting for sort order

### Neutral

- **Version retention policy**: Configurable (keep N versions, or versions within TTL)
- **No cross-entity queries**: Point-in-time is per-entity; batch queries iterate

## Alternatives Considered

| Alternative | Rejected Because |
|-------------|------------------|
| **Separate history table** | Complicates queries; need to check two places |
| **Regular (non-inverted) timestamps** | Requires reverse iteration (slower in LSM) |
| **Version numbers instead of timestamps** | Loses temporal semantics; can't query "as of time T" |
| **Append-only log** | Requires scanning entire log for point-in-time |
| **Event sourcing** | Overkill; we don't need full event replay |

## Implementation Notes

### Key Files

- `internal/core/storage/warm.go` - Versioned key implementation
- `internal/core/storage/keys.go` - Key formatting utilities

### Key Format Utilities

```go
const (
    prefixCurrent  = "c:"
    prefixHistory  = "h:"
    maxTimestamp   = int64(math.MaxInt64)
)

func currentKey(entityID, feature string) []byte {
    return []byte(fmt.Sprintf("%s%s:%s", prefixCurrent, entityID, feature))
}

func historyKey(entityID, feature string, timestamp int64) []byte {
    inverted := maxTimestamp - timestamp
    return []byte(fmt.Sprintf("%s%s:%s:%019d", prefixHistory, entityID, feature, inverted))
}

func parseHistoryKey(key []byte) (entityID, feature string, timestamp int64, err error) {
    // Parse h:{entity}:{feature}:{inverted_ts}
    parts := bytes.SplitN(key[2:], []byte(":"), 3)
    if len(parts) != 3 {
        return "", "", 0, ErrInvalidKey
    }
    inverted, _ := strconv.ParseInt(string(parts[2]), 10, 64)
    return string(parts[0]), string(parts[1]), maxTimestamp - inverted, nil
}
```

### Version Retention

```yaml
storage:
  warm:
    history:
      max_versions: 100       # Keep last 100 versions per feature
      max_age: 30d            # Or versions younger than 30 days
      gc_interval: 1h         # Check for expired versions hourly
```

### Training Data Export

Point-in-time queries enable training data generation:

```go
// Generate training dataset with labels and point-in-time features
for _, example := range trainingExamples {
    // Get features as they existed when label was recorded
    features, _ := store.GetAsOf(example.EntityID, featureNames, example.LabelTime)

    writer.Write(TrainingRow{
        EntityID:  example.EntityID,
        Features:  features,
        Label:     example.Label,
        LabelTime: example.LabelTime,
    })
}
```
