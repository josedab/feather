---
sidebar_position: 1
title: Deployment Guide
description: Deploy Feather in production with Docker, Kubernetes, and bare metal.
---

# Deployment Guide

This guide covers deploying Feather in production environments, from single-node to Kubernetes clusters.

## Deployment Options

| Method | Best For | Complexity |
|--------|----------|------------|
| Binary | Development, small deployments | Low |
| Docker | Single-node production | Low |
| Kubernetes | Scalable production | Medium |
| Helm | Kubernetes with customization | Medium |

## Binary Deployment

### Download and Install

```bash
# Create directory
sudo mkdir -p /opt/feather
cd /opt/feather

# Download binary
curl -sSL https://github.com/feather-store/feather/releases/latest/download/feather-linux-amd64 -o feather
chmod +x feather

# Create data directory
sudo mkdir -p /var/lib/feather/data
sudo chown $USER:$USER /var/lib/feather/data
```

### Configuration File

```yaml title="/opt/feather/feather.yaml"
server:
  http:
    port: 8080
    read_timeout: 30s
    write_timeout: 30s
  grpc:
    port: 50051
    max_concurrent: 1000

storage:
  hot:
    max_memory: "8GB"
    ttl: "2h"
  warm:
    path: "/var/lib/feather/data"
    sync_writes: false

observability:
  metrics:
    enabled: true
    port: 9090
  logging:
    level: "info"
    format: "json"
```

### Systemd Service

```ini title="/etc/systemd/system/feather.service"
[Unit]
Description=Feather Feature Store
After=network.target

[Service]
Type=simple
User=feather
Group=feather
ExecStart=/opt/feather/feather -config /opt/feather/feather.yaml
Restart=always
RestartSec=5
LimitNOFILE=65535

# Environment
Environment=GOGC=200

[Install]
WantedBy=multi-user.target
```

```bash
# Enable and start
sudo systemctl daemon-reload
sudo systemctl enable feather
sudo systemctl start feather

# Check status
sudo systemctl status feather
```

## Docker Deployment

### Basic Docker Run

```bash
docker run -d \
  --name feather \
  -p 8080:8080 \
  -p 50051:50051 \
  -p 9090:9090 \
  -v feather-data:/var/lib/feather/data \
  -v $(pwd)/feather.yaml:/etc/feather/config.yaml \
  -e FEATHER_HOT_MAX_MEMORY=8GB \
  ghcr.io/feather-store/feather:latest \
  -config /etc/feather/config.yaml
```

### Docker Compose

```yaml title="docker-compose.yaml"
version: '3.8'

services:
  feather:
    image: ghcr.io/feather-store/feather:latest
    container_name: feather
    ports:
      - "8080:8080"
      - "50051:50051"
      - "9090:9090"
    volumes:
      - feather-data:/var/lib/feather/data
      - ./feather.yaml:/etc/feather/config.yaml:ro
    environment:
      - FEATHER_HOT_MAX_MEMORY=8GB
      - GOGC=200
    command: ["-config", "/etc/feather/config.yaml"]
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8080/health"]
      interval: 10s
      timeout: 5s
      retries: 3
    deploy:
      resources:
        limits:
          memory: 16G
        reservations:
          memory: 8G

volumes:
  feather-data:
```

```bash
docker-compose up -d
```

## Kubernetes Deployment

### Namespace and ConfigMap

```yaml title="namespace.yaml"
apiVersion: v1
kind: Namespace
metadata:
  name: feather-system
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: feather-config
  namespace: feather-system
data:
  feather.yaml: |
    server:
      http:
        port: 8080
      grpc:
        port: 50051
    storage:
      hot:
        max_memory: "8GB"
      warm:
        path: "/var/lib/feather/data"
    observability:
      metrics:
        enabled: true
        port: 9090
```

### StatefulSet

```yaml title="statefulset.yaml"
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: feather
  namespace: feather-system
spec:
  serviceName: feather
  replicas: 1
  selector:
    matchLabels:
      app: feather
  template:
    metadata:
      labels:
        app: feather
      annotations:
        prometheus.io/scrape: "true"
        prometheus.io/port: "9090"
    spec:
      containers:
      - name: feather
        image: ghcr.io/feather-store/feather:latest
        args: ["-config", "/etc/feather/feather.yaml"]
        ports:
        - name: http
          containerPort: 8080
        - name: grpc
          containerPort: 50051
        - name: metrics
          containerPort: 9090
        env:
        - name: GOGC
          value: "200"
        resources:
          requests:
            memory: "8Gi"
            cpu: "2"
          limits:
            memory: "16Gi"
            cpu: "4"
        volumeMounts:
        - name: config
          mountPath: /etc/feather
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
      volumes:
      - name: config
        configMap:
          name: feather-config
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

### Services

```yaml title="services.yaml"
apiVersion: v1
kind: Service
metadata:
  name: feather
  namespace: feather-system
spec:
  type: ClusterIP
  selector:
    app: feather
  ports:
  - name: http
    port: 8080
    targetPort: 8080
  - name: grpc
    port: 50051
    targetPort: 50051
---
apiVersion: v1
kind: Service
metadata:
  name: feather-metrics
  namespace: feather-system
  labels:
    app: feather
spec:
  type: ClusterIP
  selector:
    app: feather
  ports:
  - name: metrics
    port: 9090
    targetPort: 9090
```

### Ingress (Optional)

```yaml title="ingress.yaml"
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: feather
  namespace: feather-system
  annotations:
    nginx.ingress.kubernetes.io/proxy-body-size: "10m"
spec:
  rules:
  - host: feather.example.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: feather
            port:
              number: 8080
```

### PodDisruptionBudget

```yaml title="pdb.yaml"
apiVersion: policy/v1
kind: PodDisruptionBudget
metadata:
  name: feather
  namespace: feather-system
spec:
  minAvailable: 1
  selector:
    matchLabels:
      app: feather
```

### Apply All Resources

```bash
kubectl apply -f namespace.yaml
kubectl apply -f statefulset.yaml
kubectl apply -f services.yaml
kubectl apply -f pdb.yaml
```

## Helm Deployment

### Using the Helm Chart

```bash
# Add the repository
helm repo add feather https://feather-store.github.io/charts
helm repo update

# Install with defaults
helm install feather feather/feather \
  --namespace feather-system \
  --create-namespace

# Install with custom values
helm install feather feather/feather \
  --namespace feather-system \
  --create-namespace \
  -f values.yaml
```

### Custom Values

```yaml title="values.yaml"
replicaCount: 1

image:
  repository: ghcr.io/feather-store/feather
  tag: latest

resources:
  requests:
    memory: "8Gi"
    cpu: "2"
  limits:
    memory: "16Gi"
    cpu: "4"

storage:
  hot:
    maxMemory: "8GB"
  warm:
    size: 100Gi
    storageClass: fast-ssd

ingress:
  enabled: true
  host: feather.example.com

metrics:
  enabled: true
  serviceMonitor:
    enabled: true
```

## Health Checks

Feather provides three health endpoints:

| Endpoint | Purpose | Use Case |
|----------|---------|----------|
| `/live` | Process is running | Kubernetes liveness probe |
| `/ready` | Ready to serve traffic | Kubernetes readiness probe |
| `/health` | Detailed component status | Debugging, monitoring |

### Health Response

```json
{
  "status": "healthy",
  "components": {
    "hot_tier": {"status": "healthy", "memory_used": "2.1GB"},
    "warm_tier": {"status": "healthy", "disk_used": "15GB"},
    "http_server": {"status": "healthy"},
    "grpc_server": {"status": "healthy"}
  }
}
```

## TLS Configuration

### Enable TLS

```yaml
server:
  http:
    tls:
      enabled: true
      cert_file: "/etc/feather/tls/tls.crt"
      key_file: "/etc/feather/tls/tls.key"
  grpc:
    tls:
      enabled: true
      cert_file: "/etc/feather/tls/tls.crt"
      key_file: "/etc/feather/tls/tls.key"
```

### Kubernetes Secret

```bash
kubectl create secret tls feather-tls \
  --cert=tls.crt \
  --key=tls.key \
  -n feather-system
```

## Resource Sizing

### Memory Sizing Guide

```
Hot tier memory = (entities) × (features per entity) × (bytes per feature)

Example:
1M entities × 10 features × 100 bytes = 1GB
+ 20% overhead = 1.2GB hot tier

Recommendation: Set container limit to 2× hot tier max
```

### CPU Sizing Guide

| QPS | Recommended vCPU |
|-----|------------------|
| < 10K | 2 |
| 10K - 50K | 4 |
| 50K - 100K | 8 |
| > 100K | 16+ |

## Backup and Recovery

### Backup Script

```bash
#!/bin/bash
BACKUP_DIR="/backup/feather/$(date +%Y%m%d-%H%M%S)"
DATA_DIR="/var/lib/feather/data"

# Stop feather or use online backup
systemctl stop feather

# Copy data
mkdir -p $BACKUP_DIR
cp -r $DATA_DIR/* $BACKUP_DIR/

# Restart
systemctl start feather

echo "Backup created: $BACKUP_DIR"
```

### Recovery

```bash
# Stop feather
systemctl stop feather

# Restore from backup
rm -rf /var/lib/feather/data/*
cp -r /backup/feather/20240115-120000/* /var/lib/feather/data/

# Start feather
systemctl start feather
```

## Related Documentation

- [Configuration Reference](/docs/configuration) - All configuration options
- [Observability Guide](./observability) - Monitoring setup
- [Performance Tuning](./performance) - Optimization guide
