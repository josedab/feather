# ADR-0018: Prometheus Pull-Based Metrics

## Status

Accepted

## Context

Feather requires comprehensive metrics for:
1. **Operational visibility**: Is the system healthy?
2. **Performance monitoring**: Are latency targets being met?
3. **Capacity planning**: When will we need more resources?
4. **Alerting**: Notify on-call when things go wrong
5. **Debugging**: What happened during an incident?

We evaluated metrics approaches:

| Approach | Pros | Cons |
|----------|------|------|
| **Prometheus (pull)** | Simple, battle-tested, rich ecosystem | Requires scrape infrastructure |
| **StatsD (push)** | Fire-and-forget, low overhead | UDP packet loss, aggregation complexity |
| **OpenTelemetry Metrics** | Unified with tracing | Less mature than Prometheus |
| **Cloud-specific** | Managed infrastructure | Vendor lock-in |

## Decision

We adopt **Prometheus pull-based metrics** with the official Go client library.

### Metrics Endpoint

```
GET /metrics (port 9090)

# HELP feather_http_requests_total Total HTTP requests
# TYPE feather_http_requests_total counter
feather_http_requests_total{method="GET",path="/v1/features",status="200"} 1234567

# HELP feather_http_request_duration_seconds HTTP request latency
# TYPE feather_http_request_duration_seconds histogram
feather_http_request_duration_seconds_bucket{method="GET",path="/v1/features",le="0.001"} 1200000
feather_http_request_duration_seconds_bucket{method="GET",path="/v1/features",le="0.005"} 1230000
...
```

### Metric Categories

**Request Metrics**:
| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `http_requests_total` | Counter | method, path, status | Request count |
| `http_request_duration_seconds` | Histogram | method, path | Latency distribution |
| `grpc_requests_total` | Counter | method, status | gRPC request count |
| `grpc_request_duration_seconds` | Histogram | method | gRPC latency |

**Storage Metrics**:
| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `hot_tier_bytes` | Gauge | - | Memory used by hot tier |
| `hot_tier_entries` | Gauge | - | Items in hot tier |
| `hot_tier_hits_total` | Counter | - | Cache hits |
| `hot_tier_misses_total` | Counter | - | Cache misses |
| `hot_tier_evictions_total` | Counter | - | Evicted entries |
| `warm_tier_bytes` | Gauge | - | Disk used by warm tier |
| `warm_tier_reads_total` | Counter | - | Warm tier reads |
| `warm_tier_writes_total` | Counter | - | Warm tier writes |

**Ingestion Metrics**:
| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `kafka_messages_received_total` | Counter | topic | Messages received |
| `kafka_messages_processed_total` | Counter | topic | Messages processed |
| `kafka_consumer_lag` | Gauge | topic, partition | Consumer lag |
| `http_ingest_requests_total` | Counter | status | HTTP ingestion requests |

**Aggregation Metrics**:
| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `aggregation_compute_duration_seconds` | Histogram | function | Computation time |
| `aggregation_windows_active` | Gauge | - | Active window managers |

**Runtime Metrics**:
| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `go_goroutines` | Gauge | - | Active goroutines |
| `go_memstats_alloc_bytes` | Gauge | - | Allocated memory |
| `go_gc_duration_seconds` | Summary | - | GC pause duration |

### Histogram Buckets

Latency histograms use buckets tuned for feature store workloads:

```go
// Hot tier: sub-millisecond target
hotTierBuckets := []float64{
    0.0001, 0.0005, 0.001, 0.002, 0.005,
    0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0,
}

// HTTP/gRPC: millisecond-range
requestBuckets := []float64{
    0.001, 0.005, 0.01, 0.025, 0.05,
    0.1, 0.25, 0.5, 1.0, 2.5, 5.0, 10.0,
}

// Aggregation: microsecond-range
aggregationBuckets := []float64{
    0.00001, 0.00005, 0.0001, 0.0005, 0.001,
    0.005, 0.01, 0.05, 0.1,
}
```

### Label Cardinality

We carefully limit label cardinality to prevent metrics explosion:

**Allowed**:
- `method`: GET, POST, PUT, DELETE (4 values)
- `status`: 200, 400, 404, 500, etc. (10-20 values)
- `path`: Parameterized routes only (10-20 values)

**Forbidden**:
- Entity IDs (unbounded)
- Feature names (potentially thousands)
- User IDs (unbounded)
- Timestamps (unbounded)

```go
// GOOD: Parameterized path
path := "/v1/features"  // Not "/v1/features/user:12345"

// BAD: High cardinality
featureName := "click_count"  // Don't use as label
```

## Consequences

### Positive

- **Industry standard**: Grafana, AlertManager, widespread tooling
- **Pull model simplicity**: Feather doesn't need to know about monitoring infrastructure
- **Efficient**: Scraping is lightweight; no UDP packets to lose
- **Rich ecosystem**: Thousands of pre-built dashboards and alerts
- **Histogram support**: Native percentile computation
- **Service discovery**: Works with Kubernetes, Consul, etc.

### Negative

- **Scrape infrastructure required**: Need Prometheus server
- **Pull latency**: Metrics up to scrape_interval old
- **Memory overhead**: Histogram buckets consume memory
- **Cardinality risk**: Bad labels can explode memory

### Neutral

- **Not push-based**: Different model than StatsD/Graphite
- **Separate port**: Metrics on 9090, separate from API on 8080

## Implementation Notes

### Metrics Registration

```go
// internal/metrics/metrics.go

package metrics

import (
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promauto"
)

var (
    HTTPRequestsTotal = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Namespace: "feather",
            Name:      "http_requests_total",
            Help:      "Total HTTP requests",
        },
        []string{"method", "path", "status"},
    )

    HTTPRequestDuration = promauto.NewHistogramVec(
        prometheus.HistogramOpts{
            Namespace: "feather",
            Name:      "http_request_duration_seconds",
            Help:      "HTTP request latency",
            Buckets:   requestBuckets,
        },
        []string{"method", "path"},
    )

    HotTierBytes = promauto.NewGauge(
        prometheus.GaugeOpts{
            Namespace: "feather",
            Name:      "hot_tier_bytes",
            Help:      "Memory used by hot tier",
        },
    )

    HotTierHits = promauto.NewCounter(
        prometheus.CounterOpts{
            Namespace: "feather",
            Name:      "hot_tier_hits_total",
            Help:      "Hot tier cache hits",
        },
    )

    HotTierMisses = promauto.NewCounter(
        prometheus.CounterOpts{
            Namespace: "feather",
            Name:      "hot_tier_misses_total",
            Help:      "Hot tier cache misses",
        },
    )
)
```

### Middleware Integration

```go
// HTTP middleware
func MetricsMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        start := time.Now()

        // Wrap response writer to capture status
        wrapped := &statusWriter{ResponseWriter: w, status: 200}

        next.ServeHTTP(wrapped, r)

        duration := time.Since(start).Seconds()
        path := normalizePath(r.URL.Path)  // Parameterize paths

        metrics.HTTPRequestsTotal.WithLabelValues(
            r.Method, path, strconv.Itoa(wrapped.status),
        ).Inc()

        metrics.HTTPRequestDuration.WithLabelValues(
            r.Method, path,
        ).Observe(duration)
    })
}
```

### Metrics Server

```go
// Separate server for metrics (port 9090)
func startMetricsServer(port int) *http.Server {
    mux := http.NewServeMux()
    mux.Handle("/metrics", promhttp.Handler())

    server := &http.Server{
        Addr:    fmt.Sprintf(":%d", port),
        Handler: mux,
    }

    go server.ListenAndServe()
    return server
}
```

### Prometheus Configuration

```yaml
# prometheus.yml
scrape_configs:
  - job_name: 'feather'
    static_configs:
      - targets: ['feather:9090']
    scrape_interval: 15s
    metrics_path: /metrics
```

### Key Alerts

```yaml
# alerts.yml
groups:
  - name: feather
    rules:
      - alert: HighLatency
        expr: histogram_quantile(0.99, rate(feather_http_request_duration_seconds_bucket[5m])) > 0.01
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "P99 latency exceeds 10ms"

      - alert: HighCacheMissRate
        expr: rate(feather_hot_tier_misses_total[5m]) / (rate(feather_hot_tier_hits_total[5m]) + rate(feather_hot_tier_misses_total[5m])) > 0.2
        for: 10m
        labels:
          severity: warning
        annotations:
          summary: "Cache miss rate exceeds 20%"

      - alert: HighMemoryUsage
        expr: feather_hot_tier_bytes / feather_hot_tier_max_bytes > 0.9
        for: 5m
        labels:
          severity: critical
        annotations:
          summary: "Hot tier memory usage exceeds 90%"
```

### Grafana Dashboard

Key panels:
1. **Request Rate**: `rate(feather_http_requests_total[5m])`
2. **P99 Latency**: `histogram_quantile(0.99, rate(feather_http_request_duration_seconds_bucket[5m]))`
3. **Cache Hit Rate**: `rate(feather_hot_tier_hits_total[5m]) / (rate(feather_hot_tier_hits_total[5m]) + rate(feather_hot_tier_misses_total[5m]))`
4. **Memory Usage**: `feather_hot_tier_bytes`
5. **Error Rate**: `rate(feather_http_requests_total{status=~"5.."}[5m])`
