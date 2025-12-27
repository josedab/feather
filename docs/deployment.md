# Feather Deployment Guide

This guide covers deploying Feather in various environments from development to production.

## Table of Contents

1. [Prerequisites](#prerequisites)
2. [Local Development](#local-development)
3. [Docker Deployment](#docker-deployment)
4. [Kubernetes Deployment](#kubernetes-deployment)
5. [Helm Chart Installation](#helm-chart-installation)
6. [Production Configuration](#production-configuration)
7. [High Availability Setup](#high-availability-setup)
8. [Monitoring Setup](#monitoring-setup)
9. [Troubleshooting](#troubleshooting)

## Prerequisites

### System Requirements

| Component | Minimum | Recommended |
|-----------|---------|-------------|
| CPU | 2 cores | 4+ cores |
| Memory | 4GB | 16GB+ |
| Disk | 10GB SSD | 100GB+ SSD |
| Go | 1.22+ | Latest |

### Network Ports

| Port | Protocol | Purpose |
|------|----------|---------|
| 8080 | HTTP | REST API |
| 8081 | HTTP | Ingestion API |
| 50051 | gRPC | gRPC API |
| 9090 | HTTP | Prometheus metrics |

## Local Development

### Building from Source

```bash
# Clone the repository
git clone https://github.com/feather-store/feather.git
cd feather

# Build the binary
make build

# Run tests
make test

# Run with default configuration
./bin/feather

# Run with custom config
./bin/feather -config configs/feather.yaml
```

### Quick Start

```bash
# Start Feather with minimal configuration
FEATHER_HTTP_PORT=8080 \
FEATHER_GRPC_PORT=50051 \
FEATHER_WARM_PATH=/tmp/feather/data \
./bin/feather
```

### Verify Installation

```bash
# Check health
curl http://localhost:8080/health

# Store a feature
curl -X POST http://localhost:8080/v1/features \
  -H "Content-Type: application/json" \
  -d '{
    "entity_id": "user:123",
    "features": {
      "age": 25,
      "name": "Alice"
    }
  }'

# Retrieve features
curl "http://localhost:8080/v1/features?entity=user:123&feature=age&feature=name"
```

## Docker Deployment

### Building the Image

```bash
# Build Docker image
make docker-build

# Or manually
docker build -t feather:latest .
```

### Running with Docker

```bash
# Basic run
docker run -d \
  --name feather \
  -p 8080:8080 \
  -p 50051:50051 \
  -p 9090:9090 \
  -v feather-data:/var/lib/feather/data \
  feather:latest

# With custom configuration
docker run -d \
  --name feather \
  -p 8080:8080 \
  -p 50051:50051 \
  -e FEATHER_HTTP_PORT=8080 \
  -e FEATHER_GRPC_PORT=50051 \
  -e FEATHER_HOT_MAX_MEMORY=2GB \
  -e FEATHER_WARM_PATH=/data \
  -v feather-data:/data \
  feather:latest
```

### Docker Compose

```yaml
# docker-compose.yml
version: '3.8'

services:
  feather:
    image: feather:latest
    ports:
      - "8080:8080"
      - "50051:50051"
      - "9090:9090"
    environment:
      - FEATHER_HTTP_PORT=8080
      - FEATHER_GRPC_PORT=50051
      - FEATHER_HOT_MAX_MEMORY=4GB
      - FEATHER_WARM_PATH=/data
      - FEATHER_KAFKA_ENABLED=true
      - FEATHER_KAFKA_BROKERS=kafka:9092
    volumes:
      - feather-data:/data
    depends_on:
      - kafka
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8080/health"]
      interval: 10s
      timeout: 5s
      retries: 3

  kafka:
    image: confluentinc/cp-kafka:latest
    environment:
      KAFKA_ZOOKEEPER_CONNECT: zookeeper:2181
      KAFKA_ADVERTISED_LISTENERS: PLAINTEXT://kafka:9092
      KAFKA_OFFSETS_TOPIC_REPLICATION_FACTOR: 1
    depends_on:
      - zookeeper

  zookeeper:
    image: confluentinc/cp-zookeeper:latest
    environment:
      ZOOKEEPER_CLIENT_PORT: 2181

  prometheus:
    image: prom/prometheus:latest
    ports:
      - "9091:9090"
    volumes:
      - ./prometheus.yml:/etc/prometheus/prometheus.yml

volumes:
  feather-data:
```

## Kubernetes Deployment

### Using Raw Manifests

```bash
# Apply the namespace and CRDs
kubectl apply -f deploy/kubernetes/namespace.yaml
kubectl apply -f deploy/kubernetes/crds.yaml

# Apply configuration
kubectl apply -f deploy/kubernetes/configmap.yaml
kubectl apply -f deploy/kubernetes/secret.yaml

# Apply RBAC
kubectl apply -f deploy/kubernetes/serviceaccount.yaml
kubectl apply -f deploy/kubernetes/rbac.yaml

# Deploy the application
kubectl apply -f deploy/kubernetes/service.yaml
kubectl apply -f deploy/kubernetes/statefulset.yaml
kubectl apply -f deploy/kubernetes/pdb.yaml

# Or use kustomize
kubectl apply -k deploy/kubernetes/
```

### StatefulSet Configuration

```yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: feather
  namespace: feather-system
spec:
  serviceName: feather-headless
  replicas: 3
  selector:
    matchLabels:
      app: feather
  template:
    metadata:
      labels:
        app: feather
    spec:
      containers:
      - name: feather
        image: feather:latest
        ports:
        - containerPort: 8080
          name: http
        - containerPort: 50051
          name: grpc
        - containerPort: 9090
          name: metrics
        env:
        - name: POD_NAME
          valueFrom:
            fieldRef:
              fieldPath: metadata.name
        - name: FEATHER_NODE_ID
          valueFrom:
            fieldRef:
              fieldPath: metadata.name
        resources:
          requests:
            cpu: "500m"
            memory: "2Gi"
          limits:
            cpu: "2"
            memory: "8Gi"
        volumeMounts:
        - name: data
          mountPath: /var/lib/feather/data
        livenessProbe:
          httpGet:
            path: /live
            port: 8080
          initialDelaySeconds: 10
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /ready
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 5
  volumeClaimTemplates:
  - metadata:
      name: data
    spec:
      accessModes: ["ReadWriteOnce"]
      storageClassName: fast-ssd
      resources:
        requests:
          storage: 100Gi
```

## Helm Chart Installation

### Add the Helm Repository

```bash
# If using a Helm repository (future)
helm repo add feather https://charts.feather.io
helm repo update
```

### Install from Local Chart

```bash
# Install with default values
helm install feather ./deploy/helm/feather \
  --namespace feather-system \
  --create-namespace

# Install with custom values
helm install feather ./deploy/helm/feather \
  --namespace feather-system \
  --create-namespace \
  -f custom-values.yaml

# Or with inline values
helm install feather ./deploy/helm/feather \
  --namespace feather-system \
  --create-namespace \
  --set replicaCount=3 \
  --set storage.hot.maxMemory="8GB" \
  --set resources.requests.memory="4Gi" \
  --set resources.limits.memory="16Gi"
```

### Custom Values Example

```yaml
# custom-values.yaml
replicaCount: 3

image:
  repository: feather
  tag: "1.0.0"
  pullPolicy: IfNotPresent

config:
  hot:
    maxMemory: "8GB"
    ttl: "2h"
    numShards: 512
  warm:
    path: "/var/lib/feather/data"
    syncWrites: true

storage:
  enabled: true
  size: 200Gi
  storageClass: fast-ssd

resources:
  requests:
    cpu: "1"
    memory: "4Gi"
  limits:
    cpu: "4"
    memory: "16Gi"

autoscaling:
  enabled: true
  minReplicas: 3
  maxReplicas: 10
  targetCPUUtilization: 70
  targetMemoryUtilization: 80

podDisruptionBudget:
  enabled: true
  minAvailable: 2

monitoring:
  serviceMonitor:
    enabled: true
    interval: 15s

ingress:
  enabled: true
  className: nginx
  hosts:
    - host: feather.example.com
      paths:
        - path: /
          pathType: Prefix
  tls:
    - secretName: feather-tls
      hosts:
        - feather.example.com
```

### Upgrade

```bash
helm upgrade feather ./deploy/helm/feather \
  --namespace feather-system \
  -f custom-values.yaml
```

### Uninstall

```bash
helm uninstall feather --namespace feather-system
```

## Production Configuration

### Memory Tuning

```yaml
config:
  hot:
    # Set to 50-70% of available memory
    maxMemory: "12GB"
    # Longer TTL for stable workloads
    ttl: "4h"
    # More shards for high concurrency
    numShards: 1024
```

### Storage Tuning

```yaml
config:
  warm:
    # Use fast SSD storage
    path: "/var/lib/feather/data"
    # Enable sync writes for durability
    syncWrites: true
```

### Network Configuration

```yaml
server:
  http:
    port: 8080
    readTimeout: "30s"
    writeTimeout: "30s"
    maxHeaderBytes: 1048576
  grpc:
    port: 50051
    maxRecvMsgSize: 16777216
    maxSendMsgSize: 16777216
```

### TLS Configuration

```yaml
tls:
  enabled: true
  certFile: "/etc/feather/tls/tls.crt"
  keyFile: "/etc/feather/tls/tls.key"
  minVersion: "1.2"
```

### Security Hardening

```yaml
# Pod Security Context
securityContext:
  runAsNonRoot: true
  runAsUser: 1000
  runAsGroup: 1000
  fsGroup: 1000

# Container Security Context
containerSecurityContext:
  allowPrivilegeEscalation: false
  readOnlyRootFilesystem: true
  capabilities:
    drop:
      - ALL

# Network Policy
networkPolicy:
  enabled: true
  ingress:
    - from:
        - namespaceSelector:
            matchLabels:
              name: allowed-namespace
```

## High Availability Setup

### Multi-Replica Deployment

```yaml
replicaCount: 3

podAntiAffinity:
  preferredDuringSchedulingIgnoredDuringExecution:
    - weight: 100
      podAffinityTerm:
        labelSelector:
          matchLabels:
            app: feather
        topologyKey: kubernetes.io/hostname

topologySpreadConstraints:
  - maxSkew: 1
    topologyKey: topology.kubernetes.io/zone
    whenUnsatisfiable: ScheduleAnyway
    labelSelector:
      matchLabels:
        app: feather
```

### Pod Disruption Budget

```yaml
podDisruptionBudget:
  enabled: true
  minAvailable: 2
  # Or use maxUnavailable
  # maxUnavailable: 1
```

### Horizontal Pod Autoscaler

```yaml
autoscaling:
  enabled: true
  minReplicas: 3
  maxReplicas: 10
  metrics:
    - type: Resource
      resource:
        name: cpu
        target:
          type: Utilization
          averageUtilization: 70
    - type: Resource
      resource:
        name: memory
        target:
          type: Utilization
          averageUtilization: 80
```

### Federation Setup

For multi-region deployments:

```yaml
# US-East cluster values
federation:
  enabled: true
  nodeId: "us-east-1"
  nodeName: "US East Primary"
  region: "us-east"
  peers:
    - address: "feather.us-west.example.com:8080"
    - address: "feather.eu.example.com:8080"
```

## Monitoring Setup

### Prometheus Configuration

```yaml
# prometheus.yml
scrape_configs:
  - job_name: 'feather'
    kubernetes_sd_configs:
      - role: pod
        namespaces:
          names:
            - feather-system
    relabel_configs:
      - source_labels: [__meta_kubernetes_pod_label_app]
        regex: feather
        action: keep
      - source_labels: [__meta_kubernetes_pod_container_port_name]
        regex: metrics
        action: keep
```

### Grafana Dashboard

Import the provided Grafana dashboard from `deploy/grafana/feather-dashboard.json`.

Key panels:
- Request rate and latency
- Hot tier hit rate
- Storage utilization
- Error rates

### Alerting Rules

```yaml
groups:
  - name: feather
    rules:
      - alert: FeatherHighLatency
        expr: histogram_quantile(0.99, rate(feather_request_duration_seconds_bucket[5m])) > 0.1
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: High request latency

      - alert: FeatherLowCacheHitRate
        expr: feather_hot_tier_hit_ratio < 0.8
        for: 10m
        labels:
          severity: warning
        annotations:
          summary: Low cache hit rate

      - alert: FeatherHighMemoryUsage
        expr: feather_hot_tier_size_bytes / feather_hot_tier_max_bytes > 0.9
        for: 5m
        labels:
          severity: critical
        annotations:
          summary: Hot tier memory near capacity
```

## Troubleshooting

### Common Issues

#### Pod Not Starting

```bash
# Check pod status
kubectl describe pod feather-0 -n feather-system

# Check logs
kubectl logs feather-0 -n feather-system

# Common causes:
# - Insufficient resources
# - PVC not bound
# - Image pull errors
```

#### High Latency

```bash
# Check hot tier hit rate
curl http://localhost:9090/metrics | grep feather_hot_tier

# Possible solutions:
# - Increase hot tier memory
# - Adjust TTL settings
# - Scale horizontally
```

#### Out of Memory

```bash
# Check memory usage
kubectl top pod feather-0 -n feather-system

# Solutions:
# - Reduce hot tier max memory
# - Increase pod memory limits
# - Enable eviction policies
```

#### Disk Full

```bash
# Check PVC usage
kubectl exec feather-0 -n feather-system -- df -h /var/lib/feather/data

# Solutions:
# - Expand PVC (if supported)
# - Enable warm tier compaction
# - Reduce retention period
```

### Health Check Endpoints

```bash
# Liveness probe
curl http://localhost:8080/live

# Readiness probe
curl http://localhost:8080/ready

# Deep health check
curl http://localhost:8080/health
```

### Debugging Commands

```bash
# Get all resources
kubectl get all -n feather-system

# Get pod logs
kubectl logs -f feather-0 -n feather-system

# Execute into pod
kubectl exec -it feather-0 -n feather-system -- /bin/sh

# Port forward for local access
kubectl port-forward svc/feather 8080:8080 -n feather-system

# Check events
kubectl get events -n feather-system --sort-by='.lastTimestamp'
```

### Performance Profiling

```bash
# Enable pprof (if built with debug flags)
curl http://localhost:6060/debug/pprof/heap > heap.out
go tool pprof heap.out

# CPU profile
curl http://localhost:6060/debug/pprof/profile?seconds=30 > cpu.out
go tool pprof cpu.out
```
