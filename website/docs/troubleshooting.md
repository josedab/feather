---
sidebar_position: 10
title: Troubleshooting
description: Common issues and solutions for Feather feature store.
---

# Troubleshooting

This guide covers common issues you might encounter when running Feather and how to resolve them.

## Quick Diagnostics

### Health Check

```bash
# Check if Feather is running
curl http://localhost:8080/health

# Expected response
{
  "status": "healthy",
  "components": {
    "hot_tier": {"status": "healthy"},
    "warm_tier": {"status": "healthy"},
    "http_server": {"status": "healthy"},
    "grpc_server": {"status": "healthy"}
  }
}
```

### Check Logs

```bash
# View recent logs
docker logs feather --tail 100

# Or for systemd
journalctl -u feather -n 100
```

### Check Metrics

```bash
# Get key metrics
curl -s http://localhost:9090/metrics | grep -E "feather_(http|hot_tier|warm_tier)"
```

---

## Startup Issues

### Port Already in Use

**Symptom:**
```
Error: listen tcp :8080: bind: address already in use
```

**Solution:**
```bash
# Find what's using the port
lsof -i :8080

# Kill the process or change Feather's port
FEATHER_HTTP_PORT=8081 ./feather
```

### Permission Denied on Data Directory

**Symptom:**
```
Error: open /var/lib/feather/data: permission denied
```

**Solution:**
```bash
# Fix permissions
sudo chown -R $(whoami):$(whoami) /var/lib/feather

# Or use a different directory
FEATHER_WARM_PATH=/tmp/feather-data ./feather
```

### Out of Memory on Startup

**Symptom:**
```
fatal error: runtime: out of memory
```

**Solution:**
```yaml
# Reduce hot tier memory
storage:
  hot:
    max_memory: "512MB"  # Lower from default 4GB
```

---

## Performance Issues

### High Latency

**Symptom:** P99 latency exceeds 10ms

**Diagnosis:**
```promql
# Check cache hit rate
rate(feather_cache_hits_total[5m]) /
(rate(feather_cache_hits_total[5m]) + rate(feather_cache_misses_total[5m]))

# Check warm tier latency
histogram_quantile(0.99, rate(feather_warm_tier_read_duration_seconds_bucket[5m]))
```

**Solutions:**

1. **Increase hot tier size:**
   ```yaml
   storage:
     hot:
       max_memory: "8GB"
       ttl: "4h"
   ```

2. **Check for GC pauses:**
   ```bash
   GOGC=200 ./feather  # Reduce GC frequency
   ```

3. **Profile the application:**
   ```bash
   curl http://localhost:8080/debug/pprof/profile?seconds=30 > profile.out
   go tool pprof profile.out
   ```

### Low Throughput

**Symptom:** Can't achieve expected QPS

**Diagnosis:**
```bash
# Check CPU usage
top -p $(pgrep feather)

# Check for lock contention
curl http://localhost:8080/debug/pprof/mutex?debug=1
```

**Solutions:**

1. **Increase connection pool:**
   ```go
   client, _ := feather.NewClient("localhost:8080",
       feather.WithConnectionPool(100),
   )
   ```

2. **Use batch APIs:**
   ```python
   # Instead of individual calls
   for entity in entities:
       client.get_features(entity, features)

   # Use batch
   client.get_features_batch(entities, features)
   ```

3. **Use gRPC for high-throughput:**
   ```go
   client, _ := feather.NewGRPCClient("localhost:50051")
   ```

### High Memory Usage

**Symptom:** Memory usage exceeds configured limit

**Diagnosis:**
```bash
# Check memory breakdown
curl http://localhost:8080/debug/stats

# Profile memory
curl http://localhost:8080/debug/pprof/heap > heap.out
go tool pprof heap.out
```

**Solutions:**

1. **Reduce TTL:**
   ```yaml
   storage:
     hot:
       ttl: "30m"  # Evict sooner
   ```

2. **Check for memory leaks:**
   ```bash
   # Compare heap profiles over time
   go tool pprof -diff_base=heap1.out heap2.out
   ```

---

## Data Issues

### Feature Not Found

**Symptom:**
```json
{"error": {"code": "NOT_FOUND", "message": "Entity not found: user:123"}}
```

**Diagnosis:**
```bash
# Verify the entity exists
curl "http://localhost:8080/v1/features?entity=user:123"

# Check warm tier directly
curl http://localhost:8080/debug/stats | jq '.warm_tier.entries'
```

**Solutions:**

1. **Check entity key format:**
   ```bash
   # Entity keys are case-sensitive
   user:123  # correct
   User:123  # different entity
   ```

2. **Check if data was ingested:**
   ```promql
   rate(feather_features_stored_total[5m])
   ```

3. **Check TTL settings:**
   ```yaml
   storage:
     hot:
       ttl: "2h"  # Data may have expired
   ```

### Stale Data

**Symptom:** Features not updating

**Diagnosis:**
```bash
# Check ingestion status
curl http://localhost:8080/v1/ingestion/status

# Check Kafka lag (if using Kafka)
curl http://localhost:8080/v1/ingestion/kafka/lag
```

**Solutions:**

1. **Check ingestion pipeline:**
   ```bash
   # Verify messages are being consumed
   kafka-consumer-groups.sh --bootstrap-server kafka:9092 \
     --group feather-consumer --describe
   ```

2. **Check circuit breaker:**
   ```promql
   feather_circuit_breaker_state{name="kafka"}
   # 0=closed (healthy), 1=open (failing)
   ```

3. **Verify timestamps:**
   ```python
   # Store with explicit timestamp
   client.put_features("user:123", features, timestamp=datetime.now())
   ```

### Data Corruption

**Symptom:** Unexpected values or errors reading data

**Solutions:**

1. **Restart Feather to rebuild hot tier:**
   ```bash
   systemctl restart feather
   ```

2. **Check BadgerDB integrity:**
   ```bash
   # Stop Feather first
   badger backup --dir /var/lib/feather/data --backup-file backup.bak
   badger restore --dir /var/lib/feather/data-new --backup-file backup.bak
   ```

3. **Restore from backup:**
   ```bash
   systemctl stop feather
   rm -rf /var/lib/feather/data/*
   cp -r /backup/feather/latest/* /var/lib/feather/data/
   systemctl start feather
   ```

---

## Connectivity Issues

### Connection Refused

**Symptom:**
```
Error: connection refused
```

**Solutions:**

1. **Check if Feather is running:**
   ```bash
   systemctl status feather
   # or
   docker ps | grep feather
   ```

2. **Check bind address:**
   ```yaml
   server:
     http:
       host: "0.0.0.0"  # Listen on all interfaces
   ```

3. **Check firewall:**
   ```bash
   # Allow traffic
   sudo ufw allow 8080/tcp
   ```

### Connection Timeout

**Symptom:**
```
Error: context deadline exceeded
```

**Solutions:**

1. **Increase client timeout:**
   ```python
   client = FeatherClient("localhost:8080", timeout=30.0)
   ```

2. **Check network latency:**
   ```bash
   ping feather-server
   ```

3. **Check server load:**
   ```promql
   rate(feather_http_requests_total[5m])
   ```

### TLS Errors

**Symptom:**
```
Error: tls: bad certificate
```

**Solutions:**

1. **Verify certificate paths:**
   ```yaml
   server:
     http:
       tls:
         cert_file: "/path/to/cert.pem"
         key_file: "/path/to/key.pem"
   ```

2. **Check certificate validity:**
   ```bash
   openssl x509 -in cert.pem -noout -dates
   ```

3. **For development, disable verification:**
   ```python
   client = FeatherClient("localhost:8080", ssl_verify=False)
   ```

---

## Kafka Issues

### Consumer Not Receiving Messages

**Diagnosis:**
```bash
# Check consumer group
kafka-consumer-groups.sh --bootstrap-server kafka:9092 \
  --group feather-consumer --describe
```

**Solutions:**

1. **Check topic exists:**
   ```bash
   kafka-topics.sh --bootstrap-server kafka:9092 --list
   ```

2. **Check consumer group configuration:**
   ```yaml
   ingestion:
     kafka:
       group_id: "feather-consumer"
       auto_offset_reset: "earliest"  # or "latest"
   ```

3. **Reset consumer offset:**
   ```bash
   kafka-consumer-groups.sh --bootstrap-server kafka:9092 \
     --group feather-consumer --reset-offsets --to-earliest --execute \
     --topic feather-features
   ```

### High Consumer Lag

**Diagnosis:**
```promql
feather_ingestion_lag
```

**Solutions:**

1. **Increase parallelism:**
   ```yaml
   ingestion:
     kafka:
       parallelism: 10
   ```

2. **Increase batch size:**
   ```yaml
   ingestion:
     kafka:
       batch_size: 2000
   ```

### Circuit Breaker Open

**Symptom:**
```
Kafka circuit breaker open
```

**Solutions:**

1. **Check Kafka connectivity:**
   ```bash
   kafka-broker-api-versions.sh --bootstrap-server kafka:9092
   ```

2. **Wait for recovery (automatic):**
   ```yaml
   ingestion:
     kafka:
       circuit_breaker:
         recovery_timeout: "30s"
   ```

3. **Force reset:**
   ```bash
   curl -X POST http://localhost:8080/v1/ingestion/kafka/reset
   ```

---

## Kubernetes Issues

### Pod CrashLoopBackOff

**Diagnosis:**
```bash
kubectl describe pod feather-0 -n feather-system
kubectl logs feather-0 -n feather-system --previous
```

**Common causes:**
- Insufficient memory (increase limits)
- Permission issues on PVC
- Config map errors

### Readiness Probe Failing

**Diagnosis:**
```bash
kubectl describe pod feather-0 -n feather-system | grep -A5 Conditions
```

**Solutions:**

1. **Increase probe timeout:**
   ```yaml
   readinessProbe:
     httpGet:
       path: /ready
       port: 8080
     initialDelaySeconds: 30
     timeoutSeconds: 10
   ```

2. **Check service connectivity:**
   ```bash
   kubectl exec -it feather-0 -- curl localhost:8080/ready
   ```

### PVC Issues

**Symptom:** Pod stuck in Pending

**Diagnosis:**
```bash
kubectl describe pvc data-feather-0 -n feather-system
```

**Solutions:**

1. **Check storage class exists:**
   ```bash
   kubectl get storageclass
   ```

2. **Verify capacity:**
   ```bash
   kubectl describe nodes | grep -A5 "Allocated resources"
   ```

---

## Getting Help

### Collect Diagnostic Information

```bash
# Create diagnostic bundle
mkdir feather-diagnostics
cd feather-diagnostics

# Collect info
curl http://localhost:8080/health > health.json
curl http://localhost:8080/debug/stats > stats.json
curl http://localhost:9090/metrics > metrics.txt
docker logs feather > logs.txt 2>&1

# Create archive
tar -czvf diagnostics.tar.gz *
```

### Community Support

- **GitHub Issues**: [github.com/feather-store/feather/issues](https://github.com/feather-store/feather/issues)
- **Discussions**: [github.com/feather-store/feather/discussions](https://github.com/feather-store/feather/discussions)

### Reporting Bugs

When reporting issues, include:
1. Feather version (`./feather -version`)
2. Configuration (redact sensitive values)
3. Error messages and logs
4. Steps to reproduce

## Related Documentation

- [Observability Guide](./guides/observability) - Monitoring setup
- [Performance Tuning](./guides/performance) - Optimization
- [Configuration](./configuration) - All config options
