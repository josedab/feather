# Cloud Storage Backends

> Extend Feather's warm tier with cloud-native storage solutions.

## Table of Contents

- [Overview](#overview)
- [Supported Backends](#supported-backends)
  - [Amazon DynamoDB](#amazon-dynamodb)
  - [Amazon S3](#amazon-s3)
  - [Google Cloud Storage](#google-cloud-storage)
  - [Google Cloud Bigtable](#google-cloud-bigtable)
- [Configuration](#configuration)
- [Data Model](#data-model)
- [Performance Tuning](#performance-tuning)
- [Migration](#migration)
- [Cost Optimization](#cost-optimization)

---

## Overview

Cloud storage backends extend Feather's tiered storage architecture with managed cloud services. They provide:

- **Scalability**: Virtually unlimited storage capacity
- **Durability**: Multi-region replication
- **Managed Operations**: No infrastructure maintenance
- **Cost Efficiency**: Pay-per-use pricing models

### Storage Hierarchy

```
┌─────────────────────────────────────────────────────────────────┐
│                    Storage Hierarchy                             │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│   Hot Tier (Memory)         <1ms latency                        │
│   ├── LRU Cache (256 shards)                                    │
│   └── Recent/frequent features                                  │
│                                                                  │
│   Warm Tier (Local Disk)    1-10ms latency                      │
│   ├── BadgerDB                                                  │
│   └── Historical versions                                        │
│                                                                  │
│   Cold Tier (Cloud)         10-100ms latency                    │
│   ├── DynamoDB/Bigtable (structured)                            │
│   └── S3/GCS (archive)                                          │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

### Backend Interface

All cloud backends implement the `StorageBackend` interface:

```go
type StorageBackend interface {
    // Get retrieves features for an entity
    Get(ctx context.Context, entityKey string, features []string) (map[string]*FeatureValue, error)

    // Put stores features for an entity
    Put(ctx context.Context, entityKey string, features map[string]*FeatureValue) error

    // Delete removes an entity and all its features
    Delete(ctx context.Context, entityKey string) error

    // GetAsOf retrieves historical features at a specific timestamp
    GetAsOf(ctx context.Context, entityKey string, features []string, asOf time.Time) (map[string]*FeatureValue, error)

    // BatchGet retrieves features for multiple entities
    BatchGet(ctx context.Context, entities []string, features []string) (map[string]map[string]*FeatureValue, error)

    // Close releases resources
    Close() error
}
```

---

## Supported Backends

### Amazon DynamoDB

Fully managed NoSQL database with single-digit millisecond latency.

#### Configuration

```yaml
storage:
  cloud:
    provider: dynamodb
    config:
      region: "us-east-1"
      table_name: "feather-features"

      # Capacity settings
      billing_mode: "PAY_PER_REQUEST"  # or "PROVISIONED"
      read_capacity: 1000              # Only for PROVISIONED
      write_capacity: 500              # Only for PROVISIONED

      # Optional DAX caching
      dax:
        enabled: true
        endpoint: "dax://feather-dax.abc123.dax-clusters.us-east-1.amazonaws.com"
        ttl: "5m"

      # Connection settings
      max_retries: 3
      timeout: "5s"
```

#### Table Schema

```
Primary Key: pk (String) = entity_key
Sort Key:    sk (String) = feature_name

Attributes:
  - value (Binary/String/Number) - Feature value
  - ts (Number) - Timestamp (Unix nanoseconds)
  - version (Number) - Version number
  - ttl (Number) - TTL for auto-expiration
```

#### Historical Data (GSI)

```
GSI: history-index
  Partition Key: pk (String) = entity_key
  Sort Key: sk_ts (String) = feature_name#timestamp

Enables point-in-time queries with efficient range scans.
```

#### IAM Permissions

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "dynamodb:GetItem",
        "dynamodb:PutItem",
        "dynamodb:DeleteItem",
        "dynamodb:Query",
        "dynamodb:BatchGetItem",
        "dynamodb:BatchWriteItem"
      ],
      "Resource": [
        "arn:aws:dynamodb:*:*:table/feather-features",
        "arn:aws:dynamodb:*:*:table/feather-features/index/*"
      ]
    }
  ]
}
```

#### DAX Caching

Amazon DynamoDB Accelerator (DAX) provides microsecond latency:

```yaml
dax:
  enabled: true
  endpoint: "dax://feather-dax.abc123.dax-clusters.us-east-1.amazonaws.com"
  ttl: "5m"
  connection_pool_size: 10
```

**Benefits:**
- 10x latency improvement for reads
- Automatic cache invalidation
- No code changes required

---

### Amazon S3

Object storage for archival and cold data.

#### Configuration

```yaml
storage:
  cloud:
    provider: s3
    config:
      region: "us-east-1"
      bucket: "feather-archive"
      prefix: "features/"

      # Storage class
      storage_class: "STANDARD_IA"  # or "GLACIER", "DEEP_ARCHIVE"

      # Lifecycle policies
      lifecycle:
        transition_days: 30          # Move to IA after 30 days
        glacier_days: 90             # Move to Glacier after 90 days
        expiration_days: 365         # Delete after 1 year

      # Performance
      multipart_threshold: "100MB"
      multipart_chunk_size: "10MB"
      max_concurrent_uploads: 10
```

#### Object Key Format

```
s3://feather-archive/features/{entity_type}/{entity_id}/{feature_name}/{timestamp}.json

Example:
s3://feather-archive/features/user/12345/purchase_history/1705315800000000000.json
```

#### Parquet Export

For analytics workloads, export to Parquet format:

```yaml
export:
  format: parquet
  compression: "snappy"
  partition_by: ["entity_type", "date"]
  path: "s3://feather-archive/exports/"
```

#### IAM Permissions

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "s3:GetObject",
        "s3:PutObject",
        "s3:DeleteObject",
        "s3:ListBucket"
      ],
      "Resource": [
        "arn:aws:s3:::feather-archive",
        "arn:aws:s3:::feather-archive/*"
      ]
    }
  ]
}
```

---

### Google Cloud Storage

Object storage with multi-regional replication.

#### Configuration

```yaml
storage:
  cloud:
    provider: gcs
    config:
      project_id: "my-project"
      bucket: "feather-features"
      prefix: "features/"

      # Location
      location: "US"  # or specific region like "us-central1"

      # Storage class
      storage_class: "STANDARD"  # or "NEARLINE", "COLDLINE", "ARCHIVE"

      # Lifecycle
      lifecycle:
        age_days: 30
        action: "SetStorageClass"
        target_class: "NEARLINE"

      # Authentication
      credentials_file: "/path/to/service-account.json"
      # Or use environment: GOOGLE_APPLICATION_CREDENTIALS
```

#### Object Versioning

Enable for point-in-time recovery:

```yaml
versioning:
  enabled: true
  max_versions: 10
```

#### IAM Permissions

```json
{
  "bindings": [
    {
      "role": "roles/storage.objectUser",
      "members": ["serviceAccount:feather@my-project.iam.gserviceaccount.com"]
    }
  ]
}
```

---

### Google Cloud Bigtable

Wide-column NoSQL database for high-throughput workloads.

#### Configuration

```yaml
storage:
  cloud:
    provider: bigtable
    config:
      project_id: "my-project"
      instance_id: "feather-instance"
      table_id: "features"

      # Column families
      column_families:
        - name: "f"          # Features
          max_versions: 10
          gc_policy:
            max_age: "720h"  # 30 days

        - name: "h"          # Historical
          max_versions: 100
          gc_policy:
            max_age: "8760h" # 1 year

      # Connection pool
      pool_size: 10
      timeout: "5s"

      # App profile (optional)
      app_profile_id: "feather-serving"
```

#### Row Key Design

```
Row Key: {entity_type}#{entity_id}

Column Family: f (features)
  Columns: {feature_name}
  Cell: value, timestamp

Column Family: h (history)
  Columns: {feature_name}#{reverse_timestamp}
  Cell: value
```

**Reverse Timestamp Pattern:**
```go
// Convert timestamp for newest-first ordering
reverseTS := math.MaxInt64 - timestamp
```

#### Cluster Configuration

```yaml
clusters:
  - id: "feather-cluster-1"
    zone: "us-central1-a"
    nodes: 3
    storage_type: "SSD"

  - id: "feather-cluster-2"
    zone: "us-central1-b"
    nodes: 3
    storage_type: "SSD"

replication:
  type: "MULTI_CLUSTER_ROUTING"
```

---

## Configuration

### Multi-Backend Configuration

Use multiple backends for different data tiers:

```yaml
storage:
  hot:
    max_memory: "8GB"
    ttl: "1h"

  warm:
    path: "/var/lib/feather/data"
    sync_interval: "5s"

  cloud:
    # Primary cloud backend
    primary:
      provider: dynamodb
      config:
        table_name: "feather-features"
        region: "us-east-1"

    # Archive backend
    archive:
      provider: s3
      config:
        bucket: "feather-archive"
        storage_class: "GLACIER"

  # Tiering policy
  tiering:
    warm_to_cloud_age: "24h"      # Move to cloud after 24h
    cloud_to_archive_age: "720h"  # Archive after 30 days
```

### Environment Variables

| Variable | Description |
|----------|-------------|
| `FEATHER_CLOUD_PROVIDER` | Cloud backend type |
| `AWS_REGION` | AWS region |
| `AWS_ACCESS_KEY_ID` | AWS access key |
| `AWS_SECRET_ACCESS_KEY` | AWS secret key |
| `GOOGLE_APPLICATION_CREDENTIALS` | GCP credentials path |
| `GOOGLE_CLOUD_PROJECT` | GCP project ID |

---

## Data Model

### Feature Value Storage

```json
{
  "entity_key": "user:12345",
  "feature_name": "purchase_total",
  "value": 1250.75,
  "value_type": "float64",
  "timestamp": 1705315800000000000,
  "version": 42,
  "metadata": {
    "source": "kafka",
    "partition": 3,
    "offset": 1234567
  }
}
```

### Vector Feature Storage

```json
{
  "entity_key": "product:abc123",
  "feature_name": "embedding",
  "value_type": "vector",
  "dimensions": [384],
  "value": "base64_encoded_float32_array",
  "timestamp": 1705315800000000000
}
```

### Historical Data

```json
{
  "entity_key": "user:12345",
  "feature_name": "purchase_total",
  "history": [
    {"value": 1250.75, "timestamp": 1705315800000000000},
    {"value": 1100.50, "timestamp": 1705229400000000000},
    {"value": 950.25, "timestamp": 1705143000000000000}
  ]
}
```

---

## Performance Tuning

### DynamoDB

| Setting | Recommendation |
|---------|----------------|
| Billing Mode | Use PAY_PER_REQUEST for variable workloads |
| DAX | Enable for read-heavy workloads |
| Batch Size | Max 25 items per BatchWrite |
| Connection Pool | 10-50 connections per instance |

### S3

| Setting | Recommendation |
|---------|----------------|
| Multipart Threshold | 100MB for large objects |
| Request Timeout | 30s for uploads, 10s for downloads |
| Transfer Acceleration | Enable for cross-region access |

### Bigtable

| Setting | Recommendation |
|---------|----------------|
| Row Key Design | Avoid hotspots with good key distribution |
| Column Family | Limit to 2-3 families |
| Cell Size | Keep under 10MB |
| Cluster Nodes | Start with 3, scale based on load |

### Latency Comparison

| Backend | Read P50 | Read P99 | Write P50 | Write P99 |
|---------|----------|----------|-----------|-----------|
| DynamoDB | 5ms | 20ms | 8ms | 30ms |
| DynamoDB + DAX | 0.5ms | 2ms | 8ms | 30ms |
| S3 | 50ms | 200ms | 100ms | 500ms |
| GCS | 40ms | 150ms | 80ms | 400ms |
| Bigtable | 3ms | 15ms | 5ms | 25ms |

---

## Migration

### From BadgerDB to DynamoDB

```go
// Migration script
func MigrateToDynamoDB(ctx context.Context, warm *storage.WarmTier, dynamo *cloud.DynamoDBBackend) error {
    // Iterate all keys in BadgerDB
    err := warm.ForEach(func(entityKey string, features map[string]*FeatureValue) error {
        // Write to DynamoDB
        return dynamo.Put(ctx, entityKey, features)
    })
    return err
}
```

### Using the CLI

```bash
# Export from local storage
feather export --source warm --format jsonl --output features.jsonl

# Import to cloud
feather import --dest dynamodb --input features.jsonl --table feather-features
```

### Zero-Downtime Migration

1. **Dual-Write Phase**: Write to both old and new backends
2. **Backfill Phase**: Copy historical data
3. **Verification Phase**: Compare data consistency
4. **Cutover Phase**: Switch reads to new backend
5. **Cleanup Phase**: Remove old backend

```yaml
migration:
  mode: "dual_write"
  source:
    provider: badgerdb
    config:
      path: "/var/lib/feather/data"
  destination:
    provider: dynamodb
    config:
      table_name: "feather-features"
  verify:
    sample_rate: 0.01
    on_mismatch: "log"  # or "fail"
```

---

## Cost Optimization

### DynamoDB

| Strategy | Savings |
|----------|---------|
| On-Demand → Provisioned | 30-70% for stable workloads |
| Reserved Capacity | 50-70% for long-term |
| TTL Auto-Delete | Reduce storage costs |
| Compression | 50-80% storage reduction |

### S3

| Strategy | Cost |
|----------|------|
| Standard | $0.023/GB/month |
| Standard-IA | $0.0125/GB/month |
| Glacier | $0.004/GB/month |
| Deep Archive | $0.00099/GB/month |

### Cost Monitoring

```yaml
observability:
  cost_tracking:
    enabled: true
    metrics:
      - dynamodb_consumed_read_capacity
      - dynamodb_consumed_write_capacity
      - s3_storage_bytes
      - s3_request_count
    alerts:
      daily_cost_threshold: 100  # USD
```

---

## Further Reading

- [Architecture Overview](./architecture.md) - Storage tier design
- [Performance Guide](./performance.md) - Optimization strategies
- [Deployment Guide](./deployment.md) - Cloud deployment
