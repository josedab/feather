---
sidebar_position: 2
title: Observability Guide
description: Set up monitoring, alerting, and tracing for Feather.
---

# Observability Guide

Feather provides comprehensive observability through Prometheus metrics, OpenTelemetry tracing, and structured logging.

## Overview

| Component | Port | Purpose |
|-----------|------|---------|
| Prometheus Metrics | 9090 | Performance monitoring |
| OpenTelemetry | OTLP | Distributed tracing |
| Structured Logging | stdout | Operational debugging |

## Prometheus Metrics

### Enabling Metrics

```yaml title="feather.yaml"
observability:
  metrics:
    enabled: true
    port: 9090
```

### Scraping Metrics

```bash
# View raw metrics
curl http://localhost:9090/metrics
```

### Key Metrics

#### Request Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `feather_http_requests_total` | Counter | Total HTTP requests |
| `feather_http_request_duration_seconds` | Histogram | Request latency |
| `feather_grpc_requests_total` | Counter | Total gRPC requests |
| `feather_grpc_request_duration_seconds` | Histogram | gRPC latency |

#### Storage Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `feather_hot_tier_size_bytes` | Gauge | Hot tier memory usage |
| `feather_hot_tier_entries` | Gauge | Entries in hot tier |
| `feather_cache_hits_total` | Counter | Cache hits |
| `feather_cache_misses_total` | Counter | Cache misses |
| `feather_evictions_total` | Counter | Cache evictions |
| `feather_warm_tier_size_bytes` | Gauge | Warm tier disk usage |

#### Ingestion Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `feather_features_stored_total` | Counter | Features written |
| `feather_ingestion_messages_total` | Counter | Kafka messages processed |
| `feather_ingestion_lag` | Gauge | Kafka consumer lag |

### Prometheus Configuration

```yaml title="prometheus.yml"
scrape_configs:
  - job_name: 'feather'
    static_configs:
      - targets: ['feather:9090']
    scrape_interval: 15s
    metrics_path: /metrics

  # For Kubernetes with ServiceMonitor
  - job_name: 'feather-k8s'
    kubernetes_sd_configs:
      - role: pod
    relabel_configs:
      - source_labels: [__meta_kubernetes_pod_label_app]
        action: keep
        regex: feather
```

### Kubernetes ServiceMonitor

```yaml title="servicemonitor.yaml"
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: feather
  namespace: feather-system
spec:
  selector:
    matchLabels:
      app: feather
  endpoints:
  - port: metrics
    interval: 15s
```

## Grafana Dashboards

### Importing the Dashboard

1. Go to Grafana → Dashboards → Import
2. Upload `feather-dashboard.json` from the repository
3. Select your Prometheus data source

### Dashboard Panels

The default dashboard includes:

**Overview Row:**
- Request Rate
- Error Rate
- P99 Latency
- Cache Hit Rate

**Storage Row:**
- Hot Tier Memory Usage
- Warm Tier Disk Usage
- Entries Count
- Eviction Rate

**Latency Row:**
- HTTP Latency Distribution
- gRPC Latency Distribution
- Hot Tier Read Latency
- Warm Tier Read Latency

### Key Queries

**Request Rate:**
```promql
rate(feather_http_requests_total[5m])
```

**P99 Latency:**
```promql
histogram_quantile(0.99, rate(feather_http_request_duration_seconds_bucket[5m]))
```

**Cache Hit Rate:**
```promql
rate(feather_cache_hits_total[5m]) /
(rate(feather_cache_hits_total[5m]) + rate(feather_cache_misses_total[5m]))
```

**Error Rate:**
```promql
rate(feather_http_requests_total{status=~"5.."}[5m]) /
rate(feather_http_requests_total[5m])
```

## Alerting Rules

### Prometheus AlertManager Rules

```yaml title="alerts.yaml"
groups:
  - name: feather
    rules:
      # High latency alert
      - alert: FeatherHighLatency
        expr: |
          histogram_quantile(0.99, rate(feather_http_request_duration_seconds_bucket[5m])) > 0.01
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "Feather P99 latency exceeds 10ms"
          description: "P99 latency is {{ $value | humanizeDuration }}"

      # Low cache hit rate
      - alert: FeatherLowCacheHitRate
        expr: |
          rate(feather_cache_hits_total[5m]) /
          (rate(feather_cache_hits_total[5m]) + rate(feather_cache_misses_total[5m])) < 0.8
        for: 10m
        labels:
          severity: warning
        annotations:
          summary: "Feather cache hit rate below 80%"
          description: "Cache hit rate is {{ $value | humanizePercentage }}"

      # High memory usage
      - alert: FeatherHighMemoryUsage
        expr: |
          feather_hot_tier_size_bytes / feather_hot_tier_max_bytes > 0.9
        for: 5m
        labels:
          severity: critical
        annotations:
          summary: "Feather hot tier memory usage exceeds 90%"

      # High error rate
      - alert: FeatherHighErrorRate
        expr: |
          rate(feather_http_requests_total{status=~"5.."}[5m]) /
          rate(feather_http_requests_total[5m]) > 0.01
        for: 5m
        labels:
          severity: critical
        annotations:
          summary: "Feather error rate exceeds 1%"

      # Instance down
      - alert: FeatherDown
        expr: up{job="feather"} == 0
        for: 1m
        labels:
          severity: critical
        annotations:
          summary: "Feather instance is down"
```

## OpenTelemetry Tracing

### Enabling Tracing

```yaml title="feather.yaml"
observability:
  tracing:
    enabled: true
    endpoint: "jaeger:4317"  # OTLP gRPC endpoint
    sample_rate: 0.1         # Sample 10% of requests
    service_name: "feather"
```

### Environment Variables

```bash
FEATHER_TRACING_ENABLED=true
FEATHER_TRACING_ENDPOINT=jaeger:4317
FEATHER_TRACING_SAMPLE_RATE=0.1
```

### Span Structure

Feather creates spans for:

```
http.request
├── storage.get
│   ├── hot_tier.get
│   └── warm_tier.get (on cache miss)
└── response.serialize
```

### Jaeger Setup

```yaml title="docker-compose.yaml"
services:
  jaeger:
    image: jaegertracing/all-in-one:latest
    ports:
      - "16686:16686"  # UI
      - "4317:4317"    # OTLP gRPC
    environment:
      - COLLECTOR_OTLP_ENABLED=true
```

### Viewing Traces

1. Open Jaeger UI at `http://localhost:16686`
2. Select "feather" service
3. Search for traces by operation or tag

### Trace Context Propagation

Feather propagates trace context via headers:
- `traceparent` (W3C Trace Context)
- `X-Request-ID` (custom)

## Structured Logging

### Configuration

```yaml title="feather.yaml"
observability:
  logging:
    level: "info"      # debug, info, warn, error
    format: "json"     # json or text
```

### Log Levels

| Level | Use Case |
|-------|----------|
| `debug` | Development, troubleshooting |
| `info` | Normal operations |
| `warn` | Recoverable issues |
| `error` | Failures requiring attention |

### JSON Log Format

```json
{
  "time": "2024-01-15T10:30:00.000Z",
  "level": "INFO",
  "msg": "HTTP request completed",
  "method": "GET",
  "path": "/v1/features",
  "status": 200,
  "duration_ms": 0.85,
  "request_id": "req-a1b2c3d4"
}
```

### Log Aggregation

**Fluentd Configuration:**
```yaml
<source>
  @type tail
  path /var/log/feather/*.log
  tag feather
  <parse>
    @type json
  </parse>
</source>

<match feather>
  @type elasticsearch
  host elasticsearch
  port 9200
  index_name feather-logs
</match>
```

**Kubernetes Logging:**
Logs go to stdout and are collected by your cluster's logging stack (e.g., Loki, ELK, CloudWatch).

## Health Monitoring

### Health Endpoints

| Endpoint | Purpose | Response |
|----------|---------|----------|
| `/live` | Liveness | 200 if running |
| `/ready` | Readiness | 200 if ready |
| `/health` | Detailed | JSON with component status |

### Health Check Script

```bash
#!/bin/bash
# health-check.sh

HEALTH=$(curl -s http://localhost:8080/health)
STATUS=$(echo $HEALTH | jq -r '.status')

if [ "$STATUS" != "healthy" ]; then
  echo "CRITICAL: Feather is unhealthy"
  echo $HEALTH | jq
  exit 2
fi

echo "OK: Feather is healthy"
exit 0
```

## Debugging

### Common Issues

**High Latency:**
```promql
# Check if it's hot tier misses
rate(feather_cache_misses_total[5m]) / rate(feather_cache_hits_total[5m])

# Check warm tier latency
histogram_quantile(0.99, rate(feather_warm_tier_read_duration_seconds_bucket[5m]))
```

**Memory Pressure:**
```promql
# Check eviction rate
rate(feather_evictions_total[5m])

# Check memory trend
deriv(feather_hot_tier_size_bytes[1h])
```

**Error Spikes:**
```promql
# Errors by endpoint
sum by (path) (rate(feather_http_requests_total{status=~"5.."}[5m]))
```

### Debug Endpoints

```bash
# Runtime statistics
curl http://localhost:8080/debug/stats

# Goroutine dump
curl http://localhost:8080/debug/pprof/goroutine?debug=1

# Memory profile
curl http://localhost:8080/debug/pprof/heap > heap.prof
go tool pprof heap.prof
```

## Related Documentation

- [Deployment Guide](./deployment) - Production setup
- [Performance Tuning](./performance) - Optimization
- [Architecture Decision Records](/docs/adr/) - Design decisions
