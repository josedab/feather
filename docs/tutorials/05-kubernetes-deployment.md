# Deploying Feather on Kubernetes

Deploy Feather to a Kubernetes cluster with persistent storage, health checks, Prometheus monitoring, and horizontal autoscaling.

**Time:** ~20 minutes

## What You'll Learn

- How to build and push the Feather Docker image
- How to create Kubernetes resources (Namespace, ConfigMap, Deployment, Service)
- How to configure liveness and readiness probes
- How to set up Prometheus monitoring and Grafana dashboards
- How to scale horizontally with a Horizontal Pod Autoscaler

## Prerequisites

- Docker installed and running
- kubectl configured with access to a Kubernetes cluster
- A container registry you can push to (e.g., Docker Hub, ECR, GCR)
- Helm (optional, for Prometheus/Grafana)

---

## Step 1: Build the Docker Image

Build the multi-stage Docker image:

```bash
$ make docker-build
```

This produces `feather:latest`. Tag and push it to your registry:

```bash
$ docker tag feather:latest your-registry.com/feather:v1.0.0
$ docker push your-registry.com/feather:v1.0.0
```

Verify the image runs locally:

```bash
$ docker run --rm -p 8080:8080 feather:latest &
$ curl -s http://localhost:8080/health | jq .status
```

```output
"healthy"
```

---

## Step 2: Create the Kubernetes Namespace

Create a dedicated namespace:

```yaml
# namespace.yaml
apiVersion: v1
kind: Namespace
metadata:
  name: feather-system
  labels:
    app.kubernetes.io/name: feather
    app.kubernetes.io/part-of: ml-platform
```

```bash
$ kubectl apply -f namespace.yaml
```

```output
namespace/feather-system created
```

---

## Step 3: Create the ConfigMap

Store the Feather configuration as a ConfigMap:

```yaml
# configmap.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: feather-config
  namespace: feather-system
data:
  feather.yaml: |
    storage:
      hot:
        max_memory: 4GB
      warm:
        path: /var/lib/feather/data
        retention: 720h  # 30 days

    serving:
      http:
        port: 8080
      grpc:
        port: 50051

    ingestion:
      http:
        enabled: true
        port: 8081

    metrics:
      prometheus:
        enabled: true
        port: 9090

    schema:
      groups:
        - name: user_features
          entity_type: user
          ttl: 24h
          features:
            - name: click_count
              data_type: int64
            - name: purchase_total
              data_type: float64
            - name: last_activity
              data_type: timestamp
```

```bash
$ kubectl apply -f configmap.yaml
```

```output
configmap/feather-config created
```

---

## Step 4: Deploy Feather

Create a Deployment with health probes, resource limits, and persistent storage:

```yaml
# deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: feather
  namespace: feather-system
  labels:
    app.kubernetes.io/name: feather
    app.kubernetes.io/component: feature-store
spec:
  replicas: 3
  selector:
    matchLabels:
      app.kubernetes.io/name: feather
  template:
    metadata:
      labels:
        app.kubernetes.io/name: feather
      annotations:
        prometheus.io/scrape: "true"
        prometheus.io/port: "9090"
        prometheus.io/path: "/metrics"
    spec:
      securityContext:
        runAsNonRoot: true
        runAsUser: 1000
        fsGroup: 1000
      containers:
        - name: feather
          image: your-registry.com/feather:v1.0.0
          args:
            - "-config"
            - "/etc/feather/feather.yaml"
          ports:
            - name: http
              containerPort: 8080
              protocol: TCP
            - name: grpc
              containerPort: 50051
              protocol: TCP
            - name: ingest
              containerPort: 8081
              protocol: TCP
            - name: metrics
              containerPort: 9090
              protocol: TCP
          # Readiness probe — pod receives traffic only when ready
          readinessProbe:
            httpGet:
              path: /ready
              port: http
            initialDelaySeconds: 5
            periodSeconds: 5
            failureThreshold: 3
          # Liveness probe — pod is restarted if unhealthy
          livenessProbe:
            httpGet:
              path: /live
              port: http
            initialDelaySeconds: 10
            periodSeconds: 10
            failureThreshold: 5
          # Startup probe — gives slow-starting pods time to initialize
          startupProbe:
            httpGet:
              path: /health
              port: http
            initialDelaySeconds: 5
            periodSeconds: 5
            failureThreshold: 12
          resources:
            requests:
              cpu: 500m
              memory: 1Gi
            limits:
              cpu: "2"
              memory: 4Gi
          volumeMounts:
            - name: config
              mountPath: /etc/feather
              readOnly: true
            - name: data
              mountPath: /var/lib/feather/data
          securityContext:
            readOnlyRootFilesystem: true
            allowPrivilegeEscalation: false
            capabilities:
              drop:
                - ALL
      volumes:
        - name: config
          configMap:
            name: feather-config
        - name: data
          emptyDir:
            sizeLimit: 10Gi
      affinity:
        podAntiAffinity:
          preferredDuringSchedulingIgnoredDuringExecution:
            - weight: 100
              podAffinityTerm:
                labelSelector:
                  matchLabels:
                    app.kubernetes.io/name: feather
                topologyKey: kubernetes.io/hostname
```

> **Note:** This example uses `emptyDir` for simplicity. For production, replace with a `PersistentVolumeClaim` (see the tip at the end of this section).

```bash
$ kubectl apply -f deployment.yaml
```

```output
deployment.apps/feather created
```

---

## Step 5: Create the Service

Expose Feather within the cluster:

```yaml
# service.yaml
apiVersion: v1
kind: Service
metadata:
  name: feather
  namespace: feather-system
  labels:
    app.kubernetes.io/name: feather
spec:
  type: ClusterIP
  ports:
    - name: http
      port: 8080
      targetPort: http
      protocol: TCP
    - name: grpc
      port: 50051
      targetPort: grpc
      protocol: TCP
    - name: ingest
      port: 8081
      targetPort: ingest
      protocol: TCP
    - name: metrics
      port: 9090
      targetPort: metrics
      protocol: TCP
  selector:
    app.kubernetes.io/name: feather
```

```bash
$ kubectl apply -f service.yaml
```

```output
service/feather created
```

---

## Step 6: Verify the Deployment

Check pod status:

```bash
$ kubectl -n feather-system get pods
```

```output
NAME                       READY   STATUS    RESTARTS   AGE
feather-6d4f8b7c9d-abc12   1/1     Running   0          30s
feather-6d4f8b7c9d-def34   1/1     Running   0          30s
feather-6d4f8b7c9d-ghi56   1/1     Running   0          30s
```

Port-forward and test:

```bash
$ kubectl -n feather-system port-forward svc/feather 8080:8080 &
$ curl -s http://localhost:8080/health | jq .
```

Expected output:

```json
{
  "status": "healthy",
  "components": {
    "hot_storage": "healthy",
    "warm_storage": "healthy",
    "aggregation_engine": "healthy"
  },
  "uptime": "45s"
}
```

Store and retrieve a test feature:

```bash
$ curl -s -X POST http://localhost:8080/v1/features \
  -H "Content-Type: application/json" \
  -d '{
    "entity_key": "user:k8s-test",
    "features": {"click_count": 99}
  }' | jq .status
```

```output
"ok"
```

```bash
$ curl -s "http://localhost:8080/v1/features?entity=user:k8s-test" | jq .features.click_count.value
```

```output
99
```

---

## Step 7: Set Up Prometheus Monitoring

If you have the Prometheus Operator installed, create a ServiceMonitor:

```yaml
# servicemonitor.yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: feather
  namespace: feather-system
  labels:
    app.kubernetes.io/name: feather
spec:
  selector:
    matchLabels:
      app.kubernetes.io/name: feather
  endpoints:
    - port: metrics
      interval: 15s
      path: /metrics
```

```bash
$ kubectl apply -f servicemonitor.yaml
```

If using plain Prometheus, the pod annotations (`prometheus.io/scrape: "true"`) handle auto-discovery.

### Key Metrics to Monitor

| Metric | Description |
|--------|-------------|
| `feather_feature_get_duration_seconds` | Feature retrieval latency |
| `feather_feature_put_duration_seconds` | Feature storage latency |
| `feather_hot_tier_hit_total` | Hot tier cache hits |
| `feather_hot_tier_miss_total` | Hot tier cache misses |
| `feather_hot_tier_size_bytes` | Hot tier memory usage |
| `feather_warm_tier_size_bytes` | Warm tier disk usage |
| `feather_http_requests_total` | Total HTTP requests by status |
| `feather_grpc_requests_total` | Total gRPC requests by method |

### Import a Grafana Dashboard

If you have Grafana running, import the built-in dashboard:

```bash
# Port-forward to Grafana
$ kubectl -n monitoring port-forward svc/grafana 3000:3000 &

# Import the dashboard via API
$ curl -s -X POST http://admin:admin@localhost:3000/api/dashboards/db \
  -H "Content-Type: application/json" \
  -d '{
    "dashboard": {
      "title": "Feather Feature Store",
      "panels": [
        {
          "title": "Feature Retrieval Latency (p99)",
          "type": "timeseries",
          "targets": [{"expr": "histogram_quantile(0.99, rate(feather_feature_get_duration_seconds_bucket[5m]))"}]
        },
        {
          "title": "Hot Tier Hit Rate",
          "type": "gauge",
          "targets": [{"expr": "rate(feather_hot_tier_hit_total[5m]) / (rate(feather_hot_tier_hit_total[5m]) + rate(feather_hot_tier_miss_total[5m]))"}]
        },
        {
          "title": "Requests per Second",
          "type": "timeseries",
          "targets": [{"expr": "sum(rate(feather_http_requests_total[1m])) by (status)"}]
        },
        {
          "title": "Memory Usage",
          "type": "timeseries",
          "targets": [{"expr": "feather_hot_tier_size_bytes"}]
        }
      ]
    },
    "overwrite": true
  }' | jq .status
```

```output
"success"
```

---

## Step 8: Scale Horizontally with HPA

Create a Horizontal Pod Autoscaler that scales based on CPU usage:

```yaml
# hpa.yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: feather
  namespace: feather-system
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: feather
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
  behavior:
    scaleUp:
      stabilizationWindowSeconds: 60
      policies:
        - type: Pods
          value: 2
          periodSeconds: 60
    scaleDown:
      stabilizationWindowSeconds: 300
      policies:
        - type: Pods
          value: 1
          periodSeconds: 120
```

```bash
$ kubectl apply -f hpa.yaml
```

```output
horizontalpodautoscaler.autoscaling/feather created
```

Verify the HPA is active:

```bash
$ kubectl -n feather-system get hpa
```

```output
NAME      REFERENCE            TARGETS           MINPODS   MAXPODS   REPLICAS   AGE
feather   Deployment/feather   12%/70%, 35%/80%   3         10        3          30s
```

---

## Production Tips

### Use PersistentVolumeClaims for Warm Tier

For production, replace `emptyDir` with a PVC to survive pod restarts:

```yaml
# Add to deployment.yaml volumes section:
volumes:
  - name: data
    persistentVolumeClaim:
      claimName: feather-data

---
# pvc.yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: feather-data
  namespace: feather-system
spec:
  accessModes:
    - ReadWriteOnce
  storageClassName: gp3  # adjust for your cloud provider
  resources:
    requests:
      storage: 50Gi
```

> **Tip:** If you need ReadWriteMany access for multiple replicas sharing the same warm tier, consider using a StatefulSet instead. See `deploy/kubernetes/statefulset.yaml` in the Feather repository for a production-ready example.

### Pod Disruption Budget

Ensure availability during cluster maintenance:

```yaml
# pdb.yaml
apiVersion: policy/v1
kind: PodDisruptionBudget
metadata:
  name: feather
  namespace: feather-system
spec:
  minAvailable: 2
  selector:
    matchLabels:
      app.kubernetes.io/name: feather
```

```bash
$ kubectl apply -f pdb.yaml
```

### Network Policy

Restrict traffic to only what's needed:

```yaml
# networkpolicy.yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: feather
  namespace: feather-system
spec:
  podSelector:
    matchLabels:
      app.kubernetes.io/name: feather
  policyTypes:
    - Ingress
  ingress:
    - ports:
        - port: 8080  # HTTP API
        - port: 50051  # gRPC
        - port: 9090  # Prometheus metrics
```

---

## Summary

You now have Feather running on Kubernetes with:

| Component | Status |
|-----------|--------|
| Multi-replica Deployment | ✅ 3 pods with anti-affinity |
| Health Probes | ✅ Liveness, readiness, startup |
| Prometheus Monitoring | ✅ Metrics on port 9090 |
| Grafana Dashboard | ✅ Latency, hit rate, RPS |
| Autoscaling | ✅ HPA with CPU/memory targets |
| Security | ✅ Non-root, read-only FS, dropped capabilities |

---

## What's Next?

- **[Getting Started](01-getting-started.md)** — Learn the basics first
- **[Real-Time Fraud Detection](02-fraud-detection.md)** — Build a feature pipeline on this infrastructure
- Review `deploy/kubernetes/` in the repository for additional manifests (Ingress, RBAC, StatefulSet)
