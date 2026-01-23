# ADR-0005: Dual-Path Ingestion (Kafka + HTTP)

## Status

Accepted

## Context

Feature data enters Feather from multiple sources:

1. **Streaming pipelines**: Continuous updates from Spark/Flink jobs
2. **Batch processes**: Scheduled feature computation jobs
3. **Real-time services**: Direct writes from application code
4. **External systems**: Webhooks, data sync processes

Different sources have different requirements:
- **Streaming**: Needs durability, exactly-once semantics, replay capability
- **Real-time**: Needs lowest latency, fire-and-forget acceptable
- **Batch**: Needs high throughput, backpressure handling

A single ingestion path cannot optimize for all these patterns.

## Decision

We implement **two independent ingestion paths**:

### Kafka Consumer (Port: N/A, internal)

```yaml
ingestion:
  kafka:
    enabled: true
    brokers: ["kafka:9092"]
    topic: feature-updates
    group_id: feather-consumer
    auto_offset_reset: earliest
```

**Characteristics**:
- Consumes from Kafka topic with consumer group semantics
- At-least-once delivery with idempotent writes
- Automatic offset management
- Supports message replay for recovery

**Message Format**:
```json
{
  "entity_id": "user:12345",
  "features": {
    "click_count": 42,
    "last_active": "2024-01-15T10:30:00Z"
  },
  "timestamp": 1705315800000,
  "version": 1
}
```

### HTTP Ingestion (Port 8081)

```
POST /ingest        - Single feature update
POST /ingest/bulk   - Batch updates (up to 1000)
```

**Characteristics**:
- Synchronous response confirms write to hot tier
- Rate limiting per client IP (token bucket)
- Request validation before processing
- Lower latency than Kafka path

### Circuit Breaker Pattern

The Kafka consumer implements a circuit breaker for resilience:

```
States: Closed → Open → Half-Open → Closed
```

- **Closed**: Normal operation, failures counted
- **Open**: After N failures, stop consuming, wait timeout
- **Half-Open**: Allow single request to test recovery
- **Back to Closed**: On success, resume normal operation

This prevents cascade failures when downstream systems are unhealthy.

## Consequences

### Positive

- **Flexibility**: Choose ingestion path based on source requirements
- **Durability**: Kafka provides replay and exactly-once semantics
- **Low latency**: HTTP path bypasses message queue for real-time
- **Resilience**: Circuit breaker prevents cascade failures
- **Throughput**: Kafka handles high-volume streaming efficiently
- **Decoupling**: Producers don't need direct Feather access

### Negative

- **Complexity**: Two ingestion systems to operate and monitor
- **Consistency**: Different paths may have different latency characteristics
- **Kafka dependency**: Requires Kafka cluster for streaming path
- **Port proliferation**: HTTP ingestion on separate port from serving

### Neutral

- **No Pulsar/Kinesis**: Could add other message queues if needed
- **Separate ports**: 8080 for serving, 8081 for ingestion (security boundary)

## Alternatives Considered

| Alternative | Rejected Because |
|-------------|------------------|
| Kafka only | Too high latency for real-time use cases |
| HTTP only | No durability, replay, or backpressure |
| gRPC streaming | More complex client implementation |
| Redis Streams | Additional infrastructure; Kafka already common |

## Implementation Notes

### Kafka Consumer

Key file: `internal/ingestion/kafka.go`

```go
type KafkaConsumer struct {
    consumer *kafka.Consumer
    store    *storage.Store
    breaker  *CircuitBreaker
    decoder  FeatureDecoder
}

func (k *KafkaConsumer) Start(ctx context.Context) error {
    for {
        select {
        case <-ctx.Done():
            return nil
        default:
            if k.breaker.Allow() {
                msg, err := k.consumer.ReadMessage(timeout)
                if err != nil {
                    k.breaker.RecordFailure()
                    continue
                }
                k.processMessage(msg)
                k.breaker.RecordSuccess()
            } else {
                time.Sleep(k.breaker.BackoffDuration())
            }
        }
    }
}
```

### HTTP Ingestion

Key file: `internal/ingestion/http.go`

```go
type HTTPIngestion struct {
    store   *storage.Store
    limiter *RateLimiter
}

func (h *HTTPIngestion) handleIngest(w http.ResponseWriter, r *http.Request) {
    // Rate limit check
    if !h.limiter.Allow(clientIP(r)) {
        http.Error(w, "rate limited", http.StatusTooManyRequests)
        return
    }

    var update FeatureUpdate
    json.NewDecoder(r.Body).Decode(&update)

    // Write to store (sync to hot, async to warm)
    err := h.store.Put(update.EntityID, update.Features)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    w.WriteHeader(http.StatusAccepted)
}
```

### Rate Limiter

Token bucket algorithm per client IP:
- **Bucket size**: 100 tokens
- **Refill rate**: 10 tokens/second
- **Configurable**: Via environment or config file
