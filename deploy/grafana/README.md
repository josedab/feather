# Feather Grafana Dashboards

Pre-built Grafana dashboard templates for monitoring Feather.

## Dashboards

| Dashboard | File | Description |
|-----------|------|-------------|
| Feather Overview | `feather-overview.json` | Request rate, latency, cache hits, errors, memory |
| Feather Features | `feather-features.json` | Feature freshness, drift scores, distributions |
| Feather Operations | `feather-operations.json` | Warm tier ops, goroutines, GC, ingestion rate |

## Import Instructions

### Grafana UI

1. Open Grafana → **Dashboards** → **Import**
2. Click **Upload JSON file** and select the desired `.json` file
3. Select your Prometheus datasource when prompted
4. Click **Import**

### Grafana API

```bash
curl -X POST http://localhost:3000/api/dashboards/db \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $GRAFANA_API_KEY" \
  -d @feather-overview.json
```

### Provisioning

Copy the JSON files to your Grafana provisioning directory:

```bash
cp deploy/grafana/*.json /etc/grafana/provisioning/dashboards/
```

## Prerequisites

- Grafana 9.0+
- Prometheus datasource configured and scraping Feather metrics (port 9090)
- Feather running with Prometheus metrics enabled
