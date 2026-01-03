# Feather Observability

This directory contains observability resources for monitoring Feather in production.

## Contents

```
observability/
├── grafana/                    # Grafana dashboards
│   ├── feather-overview.json   # Service health, requests, errors
│   ├── feather-storage.json    # Cache performance, storage tiers
│   ├── feather-ingestion.json  # Ingestion rates, freshness
│   └── feather-system.json     # Go runtime, system resources
└── prometheus/
    └── feather-alerts.yaml     # PrometheusRule alerting rules
```

## Grafana Dashboards

### Importing Dashboards

1. **Via Grafana UI:**
   - Navigate to Dashboards → Import
   - Upload the JSON file or paste its contents
   - Select your Prometheus datasource
   - Click Import

2. **Via ConfigMap (Kubernetes):**
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

3. **Via Grafana Provisioning:**
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

| Dashboard | Description | Key Metrics |
|-----------|-------------|-------------|
| **Overview** | Service health at a glance | Request rate, error rate, latency, cache hit rate |
| **Storage** | Cache and persistence | Hit/miss rate, tier sizes, evictions, shard contention |
| **Ingestion** | Data flow monitoring | Messages/sec, lag, freshness, aggregation latency |
| **System** | Runtime resources | Goroutines, memory, GC, file descriptors |

### Variables

All dashboards use a `datasource` variable for Prometheus selection. This allows switching between different Prometheus instances if you have multiple environments.

## Prometheus Alerting Rules

### Deploying Alerts

**For Prometheus Operator (Kubernetes):**
```bash
kubectl apply -f prometheus/feather-alerts.yaml
```

**For vanilla Prometheus:**

Add to your `prometheus.yml`:
```yaml
rule_files:
  - /etc/prometheus/rules/feather-alerts.yaml
```

### Alert Categories

| Category | Alerts | Description |
|----------|--------|-------------|
| **Service** | `FeatherDown`, `FeatherHighRestartRate` | Instance availability |
| **HTTP** | `FeatherHighHTTPErrorRate`, `FeatherHighHTTPLatency` | API health |
| **gRPC** | `FeatherHighGRPCErrorRate`, `FeatherHighGRPCLatency` | gRPC API health |
| **Storage** | `FeatherLowCacheHitRate`, `FeatherHighEvictionRate` | Cache performance |
| **Ingestion** | `FeatherHighIngestionLag`, `FeatherIngestionDropping` | Data pipeline |
| **Freshness** | `FeatherStaleFeatures`, `FeatherFeatureFreshnessCritical` | Feature staleness |
| **Resources** | `FeatherHighMemoryUsage`, `FeatherHighGoroutineCount` | Resource usage |
| **SLO** | `FeatherAvailabilitySLOBreach`, `FeatherLatencySLOBreach` | SLO violations |

### Severity Levels

- **critical**: Requires immediate attention (pages on-call)
- **warning**: Should be investigated soon (creates ticket)

### Tuning Thresholds

Default thresholds are conservative. Adjust based on your SLOs:

```yaml
# Example: Increase error rate threshold
- alert: FeatherHighHTTPErrorRate
  expr: |
    (
      sum(rate(feather_http_requests_total{status=~"5.."}[5m]))
      /
      sum(rate(feather_http_requests_total[5m]))
    ) > 0.10  # Changed from 0.05 to 0.10
```

## SLO Definitions

The alerting rules include SLO-based alerts:

| SLO | Target | Alert |
|-----|--------|-------|
| Availability | 99.9% | `FeatherAvailabilitySLOBreach` |
| Latency | 99% < 10ms | `FeatherLatencySLOBreach` |
| Freshness | <1% stale | `FeatherFreshnessSLOBreach` |

## Integration with Docker Compose

The `docker-compose.yaml` in the repository root includes Prometheus and Grafana. To use these dashboards:

1. Start the stack:
   ```bash
   docker-compose up -d
   ```

2. Access Grafana at http://localhost:3000 (admin/admin)

3. Import dashboards from this directory

## Metrics Reference

Key metrics exposed by Feather:

| Metric | Type | Description |
|--------|------|-------------|
| `feather_http_requests_total` | Counter | HTTP requests by method, path, status |
| `feather_http_request_duration_seconds` | Histogram | HTTP request latency |
| `feather_grpc_requests_total` | Counter | gRPC requests by method, status |
| `feather_grpc_request_duration_seconds` | Histogram | gRPC request latency |
| `feather_cache_hits_total` | Counter | Cache hit count |
| `feather_cache_misses_total` | Counter | Cache miss count |
| `feather_hot_tier_size_bytes` | Gauge | Hot tier memory usage |
| `feather_warm_tier_size_bytes` | Gauge | Warm tier disk usage |
| `feather_entity_count` | Gauge | Total entities stored |
| `feather_messages_received_total` | Counter | Ingestion messages received |
| `feather_messages_processed_total` | Counter | Ingestion messages processed |
| `feather_ingestion_lag` | Gauge | Messages behind head |
| `feather_feature_freshness_seconds` | Gauge | Feature age by feature name |
| `feather_errors_total` | Counter | Errors by type |
| `feather_goroutines` | Gauge | Active goroutines |
| `feather_memory_alloc_bytes` | Gauge | Allocated memory |

## Troubleshooting

### No Data in Dashboards

1. Verify Prometheus is scraping Feather:
   ```bash
   curl http://localhost:9090/api/v1/targets | jq '.data.activeTargets[] | select(.labels.job=="feather")'
   ```

2. Check Feather metrics endpoint:
   ```bash
   curl http://localhost:9090/metrics | grep feather_
   ```

3. Verify datasource configuration in Grafana

### Alerts Not Firing

1. Check Prometheus rules are loaded:
   ```bash
   curl http://localhost:9090/api/v1/rules | jq '.data.groups[] | select(.name | startswith("feather"))'
   ```

2. Verify alert expressions manually in Prometheus UI

3. Check Alertmanager configuration for routing
