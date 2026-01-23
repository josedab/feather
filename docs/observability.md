# Feather Observability Guide

> A comprehensive guide to monitoring and observing Feather in production.

## Table of Contents

- [Overview](#overview)
- [Metrics](#metrics)
  - [Core Metrics](#core-metrics)
  - [HTTP Metrics](#http-metrics)
  - [gRPC Metrics](#grpc-metrics)
  - [Storage Metrics](#storage-metrics)
  - [Ingestion Metrics](#ingestion-metrics)
- [Grafana Dashboards](#grafana-dashboards)
  - [Installing Dashboards](#installing-dashboards)
  - [Dashboard Overview](#dashboard-overview)
- [Alerting](#alerting)
  - [Alert Categories](#alert-categories)
  - [Deploying Alerts](#deploying-alerts)
  - [Tuning Thresholds](#tuning-thresholds)
- [Tracing](#tracing)
  - [OpenTelemetry Setup](#opentelemetry-setup)
  - [Trace Sampling](#trace-sampling)
- [Logging](#logging)
  - [Log Levels](#log-levels)
  - [Structured Logging](#structured-logging)
- [Health Checks](#health-checks)
- [SLO Definitions](#slo-definitions)
- [Troubleshooting](#troubleshooting)

---

## Overview

Feather provides comprehensive observability through three pillars:

| Pillar | Implementation | Default Port |
|--------|----------------|--------------|
| **Metrics** | Prometheus exposition format | 9090 |
| **Tracing** | OpenTelemetry (OTLP) | Configurable |
| **Logging** | Structured JSON/text via slog | stdout |

```
┌─────────────────────────────────────────────────────────────────────┐
│                         FEATHER                                      │
├─────────────────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐                  │
│  │   Metrics   │  │   Tracing   │  │   Logging   │                  │
│  │ (Prometheus)│  │(OpenTelemetry)│ │   (slog)   │                  │
│  └──────┬──────┘  └──────┬──────┘  └──────┬──────┘                  │
└─────────┼────────────────┼────────────────┼─────────────────────────┘
          │                │                │
          ▼                ▼                ▼
    ┌──────────┐    ┌──────────┐    ┌──────────┐
    │Prometheus│    │  Jaeger  │    │   Loki   │
    │          │    │  Tempo   │    │   ELK    │
    └────┬─────┘    └──────────┘    └──────────┘
         │
         ▼
    ┌──────────┐
    │ Grafana  │
    └──────────┘
```

---

## Metrics

Feather exposes metrics in Prometheus format at `http://localhost:9090/metrics`.

### Core Metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `feather_features_stored_total` | Counter | - | Total features written |
| `feather_features_retrieved_total` | Counter | - | Total features retrieved |
| `feather_entity_count` | Gauge | - | Total entities in store |
| `feather_errors_total` | Counter | `type` | Errors by category |

### HTTP Metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `feather_http_requests_total` | Counter | `method`, `path`, `status` | HTTP request count |
| `feather_http_request_duration_seconds` | Histogram | `method`, `path` | Request latency |
| `feather_http_request_size_bytes` | Histogram | `method`, `path` | Request body size |
| `feather_http_response_size_bytes` | Histogram | `method`, `path` | Response body size |

**Example Queries:**

```promql
# Request rate by endpoint
rate(feather_http_requests_total[5m])

# P99 latency
histogram_quantile(0.99, sum(rate(feather_http_request_duration_seconds_bucket[5m])) by (le))

# Error rate
sum(rate(feather_http_requests_total{status=~"5.."}[5m])) / sum(rate(feather_http_requests_total[5m]))
```

### gRPC Metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `feather_grpc_requests_total` | Counter | `method`, `status` | gRPC request count |
| `feather_grpc_request_duration_seconds` | Histogram | `method` | gRPC request latency |
| `feather_grpc_stream_messages_total` | Counter | `direction` | Stream message count |

### Storage Metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `feather_cache_hits_total` | Counter | - | Hot tier cache hits |
| `feather_cache_misses_total` | Counter | - | Hot tier cache misses |
| `feather_hot_tier_size_bytes` | Gauge | - | Hot tier memory usage |
| `feather_warm_tier_size_bytes` | Gauge | - | Warm tier disk usage |
| `feather_eviction_total` | Counter | `tier` | Cache evictions |
| `feather_shard_wait_time_seconds` | Histogram | - | Shard lock wait time |
| `feather_warm_tier_operation_duration_seconds` | Histogram | `operation` | Disk operation latency |

**Example Queries:**

```promql
# Cache hit rate
rate(feather_cache_hits_total[5m]) / (rate(feather_cache_hits_total[5m]) + rate(feather_cache_misses_total[5m]))

# Hot tier memory usage percentage
feather_hot_tier_size_bytes / feather_hot_tier_max_bytes
```

### Ingestion Metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `feather_messages_received_total` | Counter | `source` | Ingested messages count |
| `feather_messages_processed_total` | Counter | `source` | Processed messages count |
| `feather_ingestion_lag` | Gauge | `partition` | Kafka consumer lag |
| `feather_feature_freshness_seconds` | Gauge | `feature` | Feature age |
| `feather_aggregation_compute_duration_seconds` | Histogram | `function` | Aggregation latency |

---

## Grafana Dashboards

Pre-built Grafana dashboards are available in `deploy/observability/grafana/`.

### Installing Dashboards

**Option 1: Grafana UI Import**

1. Navigate to Dashboards → Import
2. Upload the JSON file or paste contents
3. Select your Prometheus datasource
4. Click Import

**Option 2: Kubernetes ConfigMap**

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: feather-dashboards
  labels:
    grafana_dashboard: "1"
data:
  feather-overview.json: |
    <contents of feather-overview.json>
```

**Option 3: Grafana Provisioning**

```yaml
# /etc/grafana/provisioning/dashboards/feather.yaml
apiVersion: 1
providers:
  - name: 'Feather'
    folder: 'Feather'
    type: file
    options:
      path: /var/lib/grafana/dashboards/feather
```

### Dashboard Overview

#### 1. Service Overview (`feather-overview.json`)

High-level service health at a glance.

| Panel | Description |
|-------|-------------|
| Service Status | Up/down indicator |
| Request Rate | HTTP + gRPC requests per second |
| Error Rate | 5xx errors as percentage |
| P50/P90/P99 Latency | Request latency percentiles |
| Cache Hit Rate | Hot tier cache efficiency |
| Active Connections | Current HTTP/gRPC connections |

#### 2. Storage Performance (`feather-storage.json`)

Cache and persistence layer monitoring.

| Panel | Description |
|-------|-------------|
| Cache Hit Rate Gauge | Current hit rate percentage |
| Hot Tier Size | Memory usage over time |
| Warm Tier Size | Disk usage over time |
| Eviction Rate | Cache evictions per second |
| Shard Contention | Lock wait time P99 |
| Read/Write Distribution | Operations by tier |

#### 3. Ingestion Monitoring (`feather-ingestion.json`)

Data pipeline health.

| Panel | Description |
|-------|-------------|
| Messages/Second | Inbound message rate |
| Ingestion Lag | Kafka consumer lag |
| Feature Freshness | Age of features by name |
| Processing Latency | End-to-end processing time |
| Drop Rate | Percentage of dropped messages |
| Aggregation Latency | Real-time aggregation compute time |

#### 4. System Resources (`feather-system.json`)

Go runtime and system metrics.

| Panel | Description |
|-------|-------------|
| Goroutines | Active goroutine count |
| Memory Allocated | Go heap allocation |
| Heap Objects | Number of heap objects |
| GC Pause | Garbage collection pause time |
| CPU Usage | Process CPU utilization |
| File Descriptors | Open FD count vs limit |

---

## Alerting

Prometheus alerting rules are provided in `deploy/observability/prometheus/feather-alerts.yaml`.

### Alert Categories

| Category | Alerts | Severity |
|----------|--------|----------|
| **Service** | `FeatherDown`, `FeatherHighRestartRate` | Critical/Warning |
| **HTTP** | `FeatherHighHTTPErrorRate`, `FeatherHighHTTPLatency` | Critical/Warning |
| **gRPC** | `FeatherHighGRPCErrorRate`, `FeatherHighGRPCLatency` | Critical/Warning |
| **Storage** | `FeatherLowCacheHitRate`, `FeatherHighEvictionRate` | Warning |
| **Ingestion** | `FeatherHighIngestionLag`, `FeatherIngestionDropping` | Critical/Warning |
| **Freshness** | `FeatherStaleFeatures`, `FeatherFeatureFreshnessCritical` | Warning/Critical |
| **Resources** | `FeatherHighMemoryUsage`, `FeatherHighGoroutineCount` | Warning |
| **SLO** | `FeatherAvailabilitySLOBreach`, `FeatherLatencySLOBreach` | Critical/Warning |

### Deploying Alerts

**Prometheus Operator (Kubernetes):**

```bash
kubectl apply -f deploy/observability/prometheus/feather-alerts.yaml
```

**Vanilla Prometheus:**

Add to your `prometheus.yml`:

```yaml
rule_files:
  - /etc/prometheus/rules/feather-alerts.yaml
```

### Tuning Thresholds

Default thresholds are conservative. Adjust based on your requirements:

```yaml
# Example: Relax error rate threshold from 5% to 10%
- alert: FeatherHighHTTPErrorRate
  expr: |
    (
      sum(rate(feather_http_requests_total{status=~"5.."}[5m]))
      /
      sum(rate(feather_http_requests_total[5m]))
    ) > 0.10  # Changed from 0.05
```

**Recommended Tuning:**

| Alert | Default | When to Increase | When to Decrease |
|-------|---------|------------------|------------------|
| HTTPErrorRate | 5% | Noisy during deployments | Production SLO stricter |
| HTTPLatency P99 | 500ms | Batch-heavy workloads | Real-time ML serving |
| CacheHitRate | 70% | Cold start period | Stable workload |
| IngestionLag | 1000 msgs | High-throughput pipelines | Low-latency requirements |
| FeatureFreshness | 1 hour | Batch feature updates | Real-time features |

---

## Tracing

### OpenTelemetry Setup

Enable tracing in your configuration:

```yaml
observability:
  tracing:
    enabled: true
    endpoint: "jaeger:4317"   # OTLP gRPC endpoint
    sample_rate: 0.1          # Sample 10% of requests
    service_name: "feather"
    environment: "production"
```

Or via environment variables:

```bash
export FEATHER_TRACING_ENABLED=true
export FEATHER_TRACING_ENDPOINT=jaeger:4317
export FEATHER_TRACING_SAMPLE_RATE=0.1
```

### Trace Sampling

| Sample Rate | Use Case | Overhead |
|-------------|----------|----------|
| 1.0 (100%) | Development, debugging | High |
| 0.1 (10%) | Production default | Low |
| 0.01 (1%) | High-traffic production | Minimal |

### Trace Attributes

Feather adds the following attributes to spans:

| Attribute | Description |
|-----------|-------------|
| `feather.entity_id` | Entity being queried |
| `feather.feature_count` | Number of features in request |
| `feather.cache_hit` | Whether request hit hot tier |
| `feather.tier` | Storage tier used (`hot`, `warm`) |

---

## Logging

### Log Levels

| Level | Description | When to Use |
|-------|-------------|-------------|
| `debug` | Verbose debugging info | Development only |
| `info` | Normal operation events | Production default |
| `warn` | Potentially problematic situations | |
| `error` | Errors requiring attention | |

Configure via:

```yaml
observability:
  logging:
    level: "info"
    format: "json"  # or "text"
```

### Structured Logging

All logs include structured fields:

```json
{
  "time": "2024-01-15T10:30:00Z",
  "level": "INFO",
  "msg": "Feature retrieved",
  "request_id": "req-abc123",
  "entity": "user:123",
  "features": ["click_count", "purchase_total"],
  "latency_ms": 0.5,
  "cache_hit": true
}
```

**Key Log Fields:**

| Field | Description |
|-------|-------------|
| `request_id` | Unique request identifier (X-Request-ID header) |
| `entity` | Entity key being accessed |
| `features` | Feature names in request |
| `latency_ms` | Operation duration |
| `cache_hit` | Hot tier hit status |
| `error` | Error details (if present) |

---

## Health Checks

### Endpoints

| Endpoint | Purpose | K8s Probe |
|----------|---------|-----------|
| `GET /live` | Liveness | `livenessProbe` |
| `GET /ready` | Readiness | `readinessProbe` |
| `GET /health` | Deep check | Debugging |

### Deep Health Check Response

```json
{
  "status": "healthy",
  "components": {
    "hot_tier": {
      "status": "healthy",
      "latency_ms": 0.1,
      "metrics": {
        "hit_rate": 0.867,
        "size_bytes": 2147483648
      }
    },
    "warm_tier": {
      "status": "healthy",
      "latency_ms": 2.5
    },
    "schema_registry": {
      "status": "healthy",
      "metrics": {
        "registered_groups": 5
      }
    },
    "aggregation_engine": {
      "status": "healthy",
      "metrics": {
        "active_windows": 342
      }
    }
  },
  "version": "1.0.0",
  "uptime_seconds": 86400
}
```

### Kubernetes Configuration

```yaml
livenessProbe:
  httpGet:
    path: /live
    port: 8080
  initialDelaySeconds: 10
  periodSeconds: 10
  failureThreshold: 3

readinessProbe:
  httpGet:
    path: /ready
    port: 8080
  initialDelaySeconds: 5
  periodSeconds: 5
  failureThreshold: 3
```

---

## SLO Definitions

Feather includes built-in SLO monitoring with corresponding alerts.

| SLO | Target | Metric | Alert |
|-----|--------|--------|-------|
| **Availability** | 99.9% | `feather_http_requests_total` | `FeatherAvailabilitySLOBreach` |
| **Latency** | 99% < 10ms | `feather_http_request_duration_seconds` | `FeatherLatencySLOBreach` |
| **Freshness** | <1% stale features | `feather_feature_freshness_seconds` | `FeatherFreshnessSLOBreach` |

### Custom SLO Dashboards

Create custom SLO burn-rate dashboards using these queries:

```promql
# Error budget remaining (30-day window)
1 - (
  sum(increase(feather_http_requests_total{status=~"5.."}[30d]))
  /
  sum(increase(feather_http_requests_total[30d]))
) / 0.001  # 99.9% SLO

# Latency SLO compliance
sum(rate(feather_http_request_duration_seconds_bucket{le="0.01"}[1h]))
/
sum(rate(feather_http_request_duration_seconds_count[1h]))
```

---

## Troubleshooting

### No Metrics Data

1. **Verify Prometheus scraping:**
   ```bash
   curl http://localhost:9090/api/v1/targets | \
     jq '.data.activeTargets[] | select(.labels.job=="feather")'
   ```

2. **Check metrics endpoint:**
   ```bash
   curl http://localhost:9090/metrics | grep feather_
   ```

3. **Verify service discovery:**
   ```bash
   kubectl get servicemonitor -n feather-system
   ```

### High Cache Miss Rate

1. **Check hot tier size:**
   ```bash
   curl http://localhost:8080/health | jq '.components.hot_tier'
   ```

2. **Analyze access patterns:**
   ```promql
   topk(10, sum by (feature) (rate(feather_cache_misses_total[5m])))
   ```

3. **Consider increasing hot tier memory:**
   ```yaml
   storage:
     hot:
       max_memory: "16GB"  # Increase from default 4GB
   ```

### High Latency

1. **Identify slow operations:**
   ```promql
   histogram_quantile(0.99, sum by (path, le) (rate(feather_http_request_duration_seconds_bucket[5m])))
   ```

2. **Check warm tier performance:**
   ```promql
   histogram_quantile(0.99, sum(rate(feather_warm_tier_operation_duration_seconds_bucket[5m])) by (le))
   ```

3. **Review GC impact:**
   ```promql
   rate(feather_gc_pause_total_seconds[5m])
   ```

### Alerts Not Firing

1. **Verify rules are loaded:**
   ```bash
   curl http://localhost:9090/api/v1/rules | \
     jq '.data.groups[] | select(.name | startswith("feather"))'
   ```

2. **Test alert expression manually** in Prometheus UI

3. **Check Alertmanager routing:**
   ```bash
   amtool config routes
   ```

---

## Further Reading

- [Architecture Overview](./architecture.md) - System design
- [API Reference](./api-reference.md) - Complete API docs
- [Deployment Guide](./deployment.md) - Production setup
- [Performance Tuning](./performance.md) - Optimization tips
