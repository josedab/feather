---
sidebar_position: 7
title: Configuration Reference
description: Complete configuration reference for Feather feature store.
---

# Configuration Reference

Feather can be configured via YAML file or environment variables. This reference covers all available options.

## Configuration Methods

### YAML File

```bash
./feather -config /path/to/feather.yaml
```

### Environment Variables

All configuration options can be set via environment variables with the `FEATHER_` prefix:

```bash
FEATHER_HTTP_PORT=8080 \
FEATHER_HOT_MAX_MEMORY=8GB \
./feather
```

### Precedence

1. Environment variables (highest priority)
2. Configuration file
3. Default values

## Complete Configuration

```yaml title="feather.yaml"
# =============================================================================
# Server Configuration
# =============================================================================
serving:
  http:
    port: 8080                    # HTTP server port
    host: "0.0.0.0"              # Bind address
    read_timeout: 30s            # Request read timeout
    write_timeout: 30s           # Response write timeout
    idle_timeout: 120s           # Keep-alive timeout
    max_header_bytes: 1048576    # Max header size (1MB)
    compression: true            # Enable gzip compression

    tls:
      enabled: false
      cert_file: ""              # Path to TLS certificate
      key_file: ""               # Path to TLS private key

  grpc:
    port: 50051                  # gRPC server port
    host: "0.0.0.0"             # Bind address
    max_recv_msg_size: 16777216  # Max receive message (16MB)
    max_send_msg_size: 16777216  # Max send message (16MB)
    max_concurrent: 1000         # Max concurrent streams

    tls:
      enabled: false
      cert_file: ""
      key_file: ""

# =============================================================================
# Storage Configuration
# =============================================================================
storage:
  # Hot tier (in-memory cache)
  hot:
    max_memory: "4GB"            # Maximum memory usage
    ttl: "2h"                    # Default TTL for cached entries
    shards: 256                  # Number of cache shards
    eviction_policy: "lru"       # Eviction policy: lru, lfu

  # Warm tier (persistent storage)
  warm:
    path: "/var/lib/feather/data"  # Data directory
    sync_writes: false           # Sync writes to disk (slower but safer)

    # BadgerDB options
    value_log_file_size: 268435456  # Value log file size (256MB)
    block_cache_size: 536870912     # Block cache size (512MB)
    index_cache_size: 268435456     # Index cache size (256MB)

    # Compaction
    compaction:
      level_size_multiplier: 10
      max_levels: 7

    # History retention
    history:
      max_versions: 100          # Max versions per feature
      max_age: "30d"             # Or versions younger than this
      gc_interval: "1h"          # Cleanup check frequency

# =============================================================================
# Schema Configuration
# =============================================================================
schema:
  validation:
    enabled: true                # Enable schema validation
    strict: false                # Reject unknown features

  groups:
    - name: user_engagement
      entity_type: user
      ttl: "24h"
      description: "User interaction metrics"
      features:
        - name: click_count
          data_type: int64
          description: "Total clicks"
        - name: purchase_total
          data_type: float64
          description: "Lifetime purchase amount"
        - name: is_premium
          data_type: bool
          description: "Premium subscription status"

    - name: product_catalog
      entity_type: product
      ttl: "1h"
      features:
        - name: price
          data_type: float64
        - name: category
          data_type: string
        - name: embedding
          data_type: vector
          dimensions: 384

# =============================================================================
# Vector Search Configuration
# =============================================================================
vectors:
  indexes:
    - name: product_embeddings
      dimensions: 384
      metric: cosine              # cosine, euclidean, dot_product
      hnsw:
        m: 16                     # Connections per node
        ef_construction: 200      # Build-time search width
        ef_search: 100            # Query-time search width

# =============================================================================
# Ingestion Configuration
# =============================================================================
ingestion:
  http:
    enabled: true
    port: 8081                   # Separate ingestion port
    rate_limit: 10000            # Requests per second

  kafka:
    enabled: false
    brokers:
      - "kafka:9092"
    topic: "feather-features"
    group_id: "feather-consumer"
    batch_size: 1000
    batch_timeout: "100ms"

    # Circuit breaker
    circuit_breaker:
      enabled: true
      failure_threshold: 5
      recovery_timeout: "30s"

    # TLS
    tls:
      enabled: false
      ca_cert: ""
      client_cert: ""
      client_key: ""

    # SASL authentication
    sasl:
      enabled: false
      mechanism: "PLAIN"         # PLAIN, SCRAM-SHA-256, SCRAM-SHA-512
      username: ""
      password: ""

# =============================================================================
# Observability Configuration
# =============================================================================
observability:
  metrics:
    enabled: true
    port: 9090                   # Prometheus metrics port
    path: "/metrics"

  tracing:
    enabled: false
    endpoint: "jaeger:4317"      # OTLP gRPC endpoint
    sample_rate: 0.1             # Sample 10% of requests
    service_name: "feather"

  logging:
    level: "info"                # debug, info, warn, error
    format: "json"               # json, text
    output: "stdout"             # stdout, stderr, file
    file_path: ""                # Log file path (if output=file)

# =============================================================================
# Drift Detection Configuration
# =============================================================================
drift:
  enabled: false

  defaults:
    window_size: 1000            # Samples for reference distribution
    detection_method: "ks"       # ks, psi, js
    threshold: 0.05              # Detection threshold
    check_interval: "5m"         # Check frequency

  features:
    - name: purchase_total
      threshold: 0.01            # Override for critical features

  notifications:
    webhook:
      url: ""
      headers: {}
    slack:
      webhook_url: ""
      channel: ""

# =============================================================================
# Freshness Configuration
# =============================================================================
freshness:
  enabled: false
  default_max_age: "1h"          # Default freshness threshold

  features:
    - name: user:last_activity
      max_age: "5m"
    - name: user:click_count
      max_age: "15m"
    - name: user:daily_spend
      max_age: "24h"

  notifications:
    webhook:
      url: ""

# =============================================================================
# Export Configuration
# =============================================================================
export:
  parallelism: 8                 # Export worker count
  batch_size: 10000              # Rows per batch
  buffer_size: "256MB"           # Memory buffer

  parquet:
    compression: "snappy"        # snappy, zstd, gzip, none
    row_group_size: 100000

  # Cloud storage credentials
  s3:
    region: ""
    access_key_id: ""
    secret_access_key: ""

  gcs:
    credentials_file: ""

  # Scheduled exports
  schedules:
    - name: daily_snapshot
      cron: "0 2 * * *"
      format: parquet
      entities: ["user:*"]
      features: ["click_count", "purchase_total"]
      output_path: "s3://bucket/snapshots/{{.Date}}/features.parquet"
      retention: "30d"

# =============================================================================
# Security Configuration
# =============================================================================
security:
  auth:
    enabled: false
    type: "api_key"              # api_key, jwt, oauth2

    api_key:
      header: "X-API-Key"
      keys:
        - "key1"
        - "key2"

    jwt:
      issuer: ""
      audience: ""
      jwks_url: ""

  rate_limiting:
    enabled: true
    requests_per_minute: 10000
    burst: 1000

  cors:
    enabled: false
    allowed_origins:
      - "*"
    allowed_methods:
      - "GET"
      - "POST"
      - "PUT"
      - "DELETE"
    allowed_headers:
      - "Content-Type"
      - "Authorization"

# =============================================================================
# Runtime Configuration
# =============================================================================
runtime:
  gomaxprocs: 0                  # 0 = use all CPUs
  gc_percent: 100                # GOGC value
  graceful_shutdown_timeout: "30s"
```

## Environment Variable Reference

| Variable | Description | Default |
|----------|-------------|---------|
| `FEATHER_HTTP_PORT` | HTTP server port | 8080 |
| `FEATHER_GRPC_PORT` | gRPC server port | 50051 |
| `FEATHER_HOT_MAX_MEMORY` | Hot tier memory limit | 4GB |
| `FEATHER_HOT_TTL` | Hot tier entry TTL | 2h |
| `FEATHER_WARM_PATH` | Warm tier data path | /var/lib/feather/data |
| `FEATHER_KAFKA_ENABLED` | Enable Kafka ingestion | false |
| `FEATHER_KAFKA_BROKERS` | Kafka broker addresses | |
| `FEATHER_METRICS_ENABLED` | Enable Prometheus metrics | true |
| `FEATHER_METRICS_PORT` | Metrics port | 9090 |
| `FEATHER_TRACING_ENABLED` | Enable OpenTelemetry tracing | false |
| `FEATHER_TRACING_ENDPOINT` | OTLP endpoint | |
| `FEATHER_LOG_LEVEL` | Log level | info |
| `FEATHER_LOG_FORMAT` | Log format (json/text) | json |

## Memory Sizing Guide

### Hot Tier Sizing

```
Required memory = (active entities) × (features per entity) × (bytes per feature) + 20% overhead

Example:
1M entities × 10 features × 100 bytes = 1GB base
+ 20% overhead = 1.2GB
Set max_memory = 1.5GB (with headroom)
```

### Warm Tier Sizing

BadgerDB cache sizing:

```yaml
storage:
  warm:
    # For read-heavy workloads
    block_cache_size: "1GB"      # Increase for better read performance
    index_cache_size: "512MB"    # Increase for large datasets
```

## Performance Tuning

### High Throughput

```yaml
serving:
  grpc:
    max_concurrent: 5000

storage:
  hot:
    shards: 512                  # More shards for less contention

runtime:
  gc_percent: 200                # Less frequent GC
```

### Low Latency

```yaml
storage:
  hot:
    max_memory: "16GB"           # Larger cache
    ttl: "4h"                    # Longer TTL

  warm:
    block_cache_size: "2GB"      # Larger block cache
    sync_writes: false           # Async writes
```

### Memory Constrained

```yaml
storage:
  hot:
    max_memory: "512MB"
    ttl: "30m"
    eviction_policy: "lru"

  warm:
    block_cache_size: "128MB"
    index_cache_size: "64MB"
```

## Validation

Validate your configuration:

```bash
./feather -config feather.yaml -validate
```

## Related Documentation

- [Deployment Guide](./guides/deployment) - Production setup
- [Performance Tuning](./guides/performance) - Optimization
- [Observability Guide](./guides/observability) - Monitoring setup
