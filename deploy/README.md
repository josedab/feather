# Feather Deployment Guide

This directory contains deployment configurations and tooling for running Feather in various environments.

## Deployment Targets

| Directory | Target | Description |
|-----------|--------|-------------|
| [`fly/`](fly/) | Fly.io | Hosted demo deployment with auto-scaling and sample data |
| [`grafana/`](grafana/) | Grafana | Pre-built dashboard templates for monitoring Feather |
| [`helm/`](helm/) | Helm / Kubernetes | Helm chart for production Kubernetes deployments |
| [`kubernetes/`](kubernetes/) | Kubernetes (raw) | Raw Kubernetes manifests (namespace, StatefulSet, RBAC, NetworkPolicy, etc.) |
| [`observability/`](observability/) | Prometheus + Grafana | Alerting rules and full observability stack dashboards |

## Quick Start

### Local Development (Docker Compose)

The fastest way to run Feather with all dependencies:

```bash
# From the repository root
docker-compose up -d
```

This starts Feather, Kafka, Zookeeper, Prometheus, and Grafana. See [`docker-compose.yml`](../docker-compose.yml) for details.

### Kubernetes (Helm)

```bash
cd deploy/helm/feather
helm install feather . -n feather --create-namespace
```

See [`helm/feather/values.yaml`](helm/feather/values.yaml) for configuration options.

### Kubernetes (Raw Manifests)

```bash
kubectl apply -k deploy/kubernetes/
```

Manifests include namespace, StatefulSet, Service, ConfigMap, RBAC, NetworkPolicy, PodDisruptionBudget, and Ingress.

### Fly.io (Demo)

```bash
cd deploy/fly
fly launch --no-deploy
fly deploy
```

See [`fly/README.md`](fly/README.md) for full instructions.

## Monitoring

### Grafana Dashboards

Import the pre-built dashboards from [`grafana/`](grafana/) into your Grafana instance:

- **Feather Overview** — request rate, latency, cache hits, errors
- **Feather Features** — feature freshness, drift scores
- **Feather Operations** — warm tier ops, goroutines, GC, ingestion rate

### Prometheus Alerts

Apply alerting rules from [`observability/prometheus/`](observability/prometheus/):

```bash
kubectl apply -f deploy/observability/prometheus/feather-alerts.yaml
```

## Prerequisites

| Target | Requirements |
|--------|-------------|
| Docker Compose | Docker, Docker Compose |
| Kubernetes | kubectl, cluster access |
| Helm | Helm 3+, kubectl, cluster access |
| Fly.io | flyctl, Fly.io account |
| Observability | Prometheus, Grafana |

## Further Reading

- [Configuration Reference](../docs/) — YAML and environment variable options
- [Contributing Guide](../docs/contributing.md) — development setup and testing
