# Offline Synchronization Guide

> Integrating Feather with Apache Spark, Apache Flink, and batch processing pipelines.

## Table of Contents

- [Overview](#overview)
- [Architecture](#architecture)
- [Apache Spark Integration](#apache-spark-integration)
  - [Spark Connector Configuration](#spark-connector-configuration)
  - [Exporting Features to Spark](#exporting-features-to-spark)
  - [Importing Features from Spark](#importing-features-from-spark)
  - [PySpark Examples](#pyspark-examples)
- [Apache Flink Integration](#apache-flink-integration)
  - [Flink Connector Configuration](#flink-connector-configuration)
  - [Flink Sink (Writing to Feather)](#flink-sink-writing-to-feather)
  - [Flink Source (Reading from Feather)](#flink-source-reading-from-feather)
  - [Checkpointing](#checkpointing)
- [Offline Sync Engine](#offline-sync-engine)
  - [Creating Sync Jobs](#creating-sync-jobs)
  - [Job Scheduling](#job-scheduling)
  - [Sync Strategies](#sync-strategies)
  - [Job Dependencies](#job-dependencies)
- [Best Practices](#best-practices)
- [Troubleshooting](#troubleshooting)

---

## Overview

Feather provides native integration with Apache Spark and Apache Flink for bidirectional synchronization between batch/streaming processing and the online feature store.

| Integration | Use Case | Direction |
|-------------|----------|-----------|
| **Spark Connector** | Batch feature computation | Bidirectional |
| **Flink Connector** | Real-time streaming pipelines | Bidirectional |
| **Offline Sync Engine** | Scheduled materialization | Offline → Online |

```
┌─────────────────────────────────────────────────────────────────────────┐
│                     BATCH/STREAMING PIPELINES                            │
├──────────────────┬──────────────────┬────────────────────────────────────┤
│   Apache Spark   │   Apache Flink   │   Custom Batch Jobs                │
│   (Batch)        │   (Streaming)    │   (Airflow, etc.)                  │
└────────┬─────────┴────────┬─────────┴──────────┬─────────────────────────┘
         │                  │                    │
         ▼                  ▼                    ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                      FEATHER INTEGRATION LAYER                           │
├──────────────────┬──────────────────┬────────────────────────────────────┤
│  Spark Connector │  Flink Connector │  Offline Sync Engine               │
│  • Export        │  • Sink          │  • Scheduled Jobs                  │
│  • Import        │  • Source        │  • Versioning                      │
│  • Schema Gen    │  • Checkpointing │  • Dependencies                    │
└────────┬─────────┴────────┬─────────┴──────────┬─────────────────────────┘
         │                  │                    │
         └──────────────────┴────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                      FEATHER ONLINE STORE                                │
│                 Hot Tier (Memory) ↔ Warm Tier (Disk)                    │
└─────────────────────────────────────────────────────────────────────────┘
```

---

## Architecture

### Data Flow Patterns

**Pattern 1: Batch Feature Computation (Spark)**

```
┌─────────────────┐     ┌─────────────────┐     ┌─────────────────┐
│  Data Lake      │────►│  Spark Job      │────►│  Feather        │
│  (S3/HDFS)      │     │  (Daily)        │     │  (Online)       │
└─────────────────┘     └─────────────────┘     └─────────────────┘
```

**Pattern 2: Real-time Feature Update (Flink)**

```
┌─────────────────┐     ┌─────────────────┐     ┌─────────────────┐
│  Kafka          │────►│  Flink Job      │────►│  Feather        │
│  (Events)       │     │  (Streaming)    │     │  (Online)       │
└─────────────────┘     └─────────────────┘     └─────────────────┘
```

**Pattern 3: Scheduled Materialization**

```
┌─────────────────┐     ┌─────────────────┐     ┌─────────────────┐
│  Parquet Files  │────►│  Sync Engine    │────►│  Feather        │
│  (Feature Store)│     │  (Scheduled)    │     │  (Online)       │
└─────────────────┘     └─────────────────┘     └─────────────────┘
```

---

## Apache Spark Integration

The Spark connector enables bidirectional data flow between Feather and Apache Spark.

### Spark Connector Configuration

```go
import "github.com/feather-store/feather/internal/spark"

config := spark.Config{
    SparkMaster:             "local[*]",  // or "yarn", "k8s://..."
    AppName:                 "feather-spark-connector",
    TempPath:                "/tmp/feather-spark",
    BatchSize:               10000,
    Parallelism:             4,
    CompressionCodec:        "snappy",    // snappy, gzip, lz4, zstd
    RowGroupSize:            128 * 1024 * 1024,  // 128MB
    EnableArrowOptimization: true,
    EnableVectorizedReader:  true,
    MaxRetries:              3,
    RetryBackoff:            time.Second,
}

connector, err := spark.NewConnector(config, store, schema, logger)
if err != nil {
    log.Fatal(err)
}
defer connector.Close()
```

### Exporting Features to Spark

Export features from Feather to Spark-compatible formats:

```go
// Export to Parquet
result, err := connector.ExportToParquet(ctx, &spark.ExportRequest{
    OutputPath:      "/data/exports/user_features",
    Features:        []string{"click_count", "purchase_total", "last_active"},
    Entities:        nil,  // nil = all entities
    StartTime:       &startTime,  // optional time filter
    EndTime:         &endTime,
    PartitionBy:     []string{"date"},
    WriteMode:       spark.WriteModeOverwrite,
    IncludeMetadata: true,
})

fmt.Printf("Exported %d rows, %d entities\n",
    result.RowsExported, result.EntitiesExported)
```

**Supported Export Formats:**

| Format | Extension | Use Case |
|--------|-----------|----------|
| `FormatParquet` | `.parquet.json` | Columnar storage for Spark |
| `FormatJSON` | `.json` | Human-readable, flexible |
| `FormatCSV` | `.csv` | Simple, universal |
| `FormatArrow` | `.arrow` | High-performance interchange |

### Importing Features from Spark

Import features computed by Spark back into Feather:

```go
result, err := connector.Import(ctx, &spark.ImportRequest{
    InputPath:       "/data/spark/computed_features",
    Format:          spark.FormatJSON,
    EntityColumn:    "user_id",
    TimestampColumn: "event_time",
    FeatureColumns: map[string]string{
        "total_clicks":     "click_count",
        "lifetime_value":   "purchase_total",
        "last_seen":        "last_active",
    },
    WriteMode:      spark.WriteModeMerge,
    ValidateSchema: true,
    BatchSize:      5000,
})

fmt.Printf("Imported %d rows, %d features updated\n",
    result.RowsImported, result.FeaturesUpdated)
```

**Write Modes:**

| Mode | Behavior |
|------|----------|
| `WriteModeOverwrite` | Replace all existing features |
| `WriteModeAppend` | Add new features, don't update existing |
| `WriteModeMerge` | Merge with existing (keep newer timestamp) |
| `WriteModeIgnore` | Skip if entity exists |

### PySpark Examples

**Generate PySpark Schema:**

```go
schema := connector.GenerateSparkSchema([]string{"click_count", "purchase_total"})
fmt.Println(schema)
```

Output:
```python
from pyspark.sql.types import StructType, StructField, StringType, LongType, DoubleType, TimestampType

schema = StructType([
    StructField("entity_key", StringType(), False),
    StructField("timestamp", TimestampType(), False),
    StructField("click_count", LongType(), True),
    StructField("purchase_total", DoubleType(), True)
])
```

**Reading Exported Features in PySpark:**

```python
from pyspark.sql import SparkSession

spark = SparkSession.builder \
    .appName("FeatherFeaturePipeline") \
    .getOrCreate()

# Read exported features
df = spark.read.json("/data/exports/user_features/features.parquet.json")

# Compute new features
aggregated = df.groupBy("entity_key") \
    .agg(
        F.sum("click_count").alias("total_clicks"),
        F.avg("purchase_total").alias("avg_purchase")
    )

# Write back for import
aggregated.write.mode("overwrite").json("/data/spark/computed_features")
```

**Complete Feature Pipeline:**

```python
from pyspark.sql import SparkSession
from pyspark.sql import functions as F
from pyspark.sql.window import Window

spark = SparkSession.builder \
    .appName("FeatherDailyFeatures") \
    .config("spark.sql.shuffle.partitions", "200") \
    .getOrCreate()

# Read raw events
events = spark.read.parquet("s3://data-lake/events/")

# Compute features
user_features = events \
    .filter(F.col("event_date") == F.current_date()) \
    .groupBy("user_id") \
    .agg(
        F.count("*").alias("daily_events"),
        F.sum(F.when(F.col("event_type") == "click", 1).otherwise(0)).alias("click_count"),
        F.sum(F.when(F.col("event_type") == "purchase", F.col("amount")).otherwise(0)).alias("purchase_total"),
        F.max("event_time").alias("last_active")
    ) \
    .withColumn("entity_key", F.concat(F.lit("user:"), F.col("user_id"))) \
    .withColumn("timestamp", F.current_timestamp())

# Write for Feather import
user_features.select(
    "entity_key", "timestamp", "daily_events", "click_count", "purchase_total", "last_active"
).write.mode("overwrite").json("/data/feather-import/daily_features/")
```

---

## Apache Flink Integration

The Flink connector enables real-time streaming feature updates with exactly-once semantics.

### Flink Connector Configuration

```go
import "github.com/feather-store/feather/internal/flink"

config := flink.Config{
    JobManagerAddress:  "localhost:8081",
    TaskSlots:          4,
    Parallelism:        4,
    BufferSize:         10000,
    FlushInterval:      100 * time.Millisecond,
    DeliveryGuarantee:  flink.GuaranteeAtLeastOnce,
    CheckpointInterval: 10 * time.Second,
    CheckpointTimeout:  60 * time.Second,
    CheckpointMode:     flink.CheckpointModeAligned,
    MaxRetries:         3,
    RetryBackoff:       100 * time.Millisecond,
    EnableMetrics:      true,
    MetricsPort:        9250,
    WatermarkStrategy:  "bounded_out_of_orderness",
    MaxOutOfOrderness:  5 * time.Second,
    IdleTimeout:        time.Minute,
}

connector, err := flink.NewConnector(config, store, schema, logger)
if err != nil {
    log.Fatal(err)
}

// Start the connector
if err := connector.Start(ctx); err != nil {
    log.Fatal(err)
}
defer connector.Close()
```

**Delivery Guarantees:**

| Guarantee | Description |
|-----------|-------------|
| `GuaranteeAtMostOnce` | May lose records on failure |
| `GuaranteeAtLeastOnce` | May duplicate records on failure (default) |
| `GuaranteeExactlyOnce` | No duplicates, no loss (requires checkpointing) |

### Flink Sink (Writing to Feather)

Create a sink to write streaming data to Feather:

```go
// Create sink configuration
sinkConfig := flink.SinkConfig{
    EntityColumn:    "user_id",
    FeatureColumns: map[string]string{
        "click_count":    "click_count",
        "purchase_total": "purchase_total",
    },
    TimestampColumn: "event_time",
    BatchSize:       1000,
    MaxWaitTime:     100 * time.Millisecond,
    ValidateSchema:  true,
}

sink, err := flink.NewSink(connector, sinkConfig)
if err != nil {
    log.Fatal(err)
}

// Write a record
err = sink.Write(ctx, map[string]interface{}{
    "user_id":        "user:123",
    "click_count":    42,
    "purchase_total": 150.99,
    "event_time":     time.Now(),
})
```

**Process Stream Records:**

```go
// Process individual records
record := flink.StreamRecord{
    EntityKey: "user:123",
    Features: map[string]interface{}{
        "click_count":    42,
        "purchase_total": 150.99,
    },
    Timestamp: time.Now(),
    Watermark: &watermark,
}

err := connector.ProcessRecord(ctx, record)

// Process batch
records := []flink.StreamRecord{record1, record2, record3}
err = connector.ProcessBatch(ctx, records)
```

### Flink Source (Reading from Feather)

Create a source to read features into Flink:

```go
sourceConfig := flink.SourceConfig{
    Features:         []string{"click_count", "purchase_total"},
    Entities:         []string{"user:123", "user:456"},  // or nil for all
    PollInterval:     time.Second,
    IncludeTimestamp: true,
    StartFromLatest:  true,
}

source, err := flink.NewSource(connector, sourceConfig)
if err != nil {
    log.Fatal(err)
}

// Read into channel
output := make(chan flink.StreamRecord, 1000)
go source.Read(ctx, output)

for record := range output {
    fmt.Printf("Entity: %s, Features: %v\n", record.EntityKey, record.Features)
}
```

### Checkpointing

Enable exactly-once semantics with checkpointing:

```go
// Trigger a checkpoint
checkpoint, err := connector.TriggerCheckpoint(ctx)
if err != nil {
    log.Error("checkpoint failed", "error", err)
}
fmt.Printf("Checkpoint %d completed in %v\n", checkpoint.ID, checkpoint.Duration)

// Restore from checkpoint
err = connector.RestoreFromCheckpoint(checkpoint)

// Get last checkpoint
lastCheckpoint := connector.GetLastCheckpoint()
```

**PyFlink Example:**

```python
from pyflink.datastream import StreamExecutionEnvironment
from pyflink.common import WatermarkStrategy
from pyflink.datastream.connectors import FlinkKafkaConsumer

def main():
    env = StreamExecutionEnvironment.get_execution_environment()

    # Enable checkpointing for exactly-once
    env.enable_checkpointing(10000)  # 10 seconds
    env.get_checkpoint_config().set_checkpoint_timeout(60000)

    # Configure parallelism
    env.set_parallelism(4)

    # Create Kafka source
    kafka_source = FlinkKafkaConsumer(
        topics='user-events',
        deserialization_schema=JsonRowDeserializationSchema.builder().build(),
        properties={
            'bootstrap.servers': 'kafka:9092',
            'group.id': 'feather-flink'
        }
    )

    # Create stream with watermarks
    stream = env.add_source(kafka_source) \
        .assign_timestamps_and_watermarks(
            WatermarkStrategy
                .for_bounded_out_of_orderness(Duration.of_seconds(5))
                .with_timestamp_assigner(lambda event, _: event['timestamp'])
        )

    # Process and compute features
    processed = stream \
        .key_by(lambda x: x['user_id']) \
        .window(TumblingEventTimeWindows.of(Time.minutes(1))) \
        .aggregate(FeatureAggregator())

    # Write to Feather via HTTP sink
    processed.add_sink(FeatherHttpSink(feather_config))

    env.execute("Feather Feature Pipeline")

if __name__ == "__main__":
    main()
```

---

## Offline Sync Engine

The offline sync engine coordinates scheduled materialization of features from batch storage to the online store.

### Creating Sync Jobs

```go
import "github.com/feather-store/feather/internal/offlinesync"

// Create engine
config := offlinesync.Config{
    WorkDir:           "/var/lib/feather/sync",
    MaxConcurrentJobs: 4,
    DefaultBatchSize:  10000,
    DefaultTimeout:    30 * time.Minute,
    RetryAttempts:     3,
    RetryBackoff:      time.Second,
    EnableVersioning:  true,
    VersionRetention:  7 * 24 * time.Hour,
}

engine, err := offlinesync.NewEngine(config, store, schema, logger)
if err != nil {
    log.Fatal(err)
}
defer engine.Close()

// Start engine
engine.Start(ctx)
```

**Create a Job:**

```go
job, err := engine.CreateJob(&offlinesync.JobSpec{
    ID:          "daily_user_features",
    Name:        "Daily User Features Sync",
    Description: "Sync daily computed user features",
    Source:      "/data/features/daily/user_features.json",
    SourceType:  offlinesync.SourceTypeJSON,
    EntityColumn:    "user_id",
    TimestampColumn: "computed_at",
    FeatureColumns: map[string]string{
        "total_clicks":   "click_count",
        "total_spend":    "purchase_total",
        "last_activity":  "last_active",
    },
    Strategy:       offlinesync.SyncStrategyMerge,
    Schedule:       "0 2 * * *",  // 2 AM daily
    Priority:       10,
    Timeout:        30 * time.Minute,
    BatchSize:      10000,
    ValidateSchema: true,
    Tags:           []string{"daily", "user"},
    Owner:          "ml-team@example.com",
})
```

### Job Scheduling

Jobs can be scheduled using cron expressions:

```go
// Daily at 2 AM
Schedule: "0 2 * * *"

// Every hour
Schedule: "0 * * * *"

// Every 15 minutes
Schedule: "*/15 * * * *"

// Weekdays at 9 AM
Schedule: "0 9 * * 1-5"
```

**Manual Execution:**

```go
// Run job immediately
execution, err := engine.RunJob(ctx, "daily_user_features")
if err != nil {
    log.Error("job failed", "error", err)
}

fmt.Printf("Synced %d records in %v\n",
    execution.RecordsSync, execution.Duration)
```

### Sync Strategies

| Strategy | Behavior |
|----------|----------|
| `SyncStrategyReplace` | Delete all existing, then insert new |
| `SyncStrategyMerge` | Update existing, insert new (default) |
| `SyncStrategyAppend` | Insert only, never update |
| `SyncStrategyIncremental` | Sync only changes since last run |

### Job Dependencies

Define job dependencies for ordered execution:

```go
// User features depend on event aggregation
userFeaturesJob, _ := engine.CreateJob(&offlinesync.JobSpec{
    ID:           "user_features",
    Dependencies: []string{"event_aggregation"},
    // ...
})

// Recommendations depend on user features
recommendationsJob, _ := engine.CreateJob(&offlinesync.JobSpec{
    ID:           "recommendations",
    Dependencies: []string{"user_features", "product_features"},
    // ...
})
```

**Job Management:**

```go
// List all jobs
jobs := engine.ListJobs(nil)

// List running jobs
running := offlinesync.JobStatusRunning
jobs = engine.ListJobs(&running)

// Get specific job
job, err := engine.GetJob("daily_user_features")

// Cancel running job
err = engine.CancelJob("daily_user_features")

// Delete job
err = engine.DeleteJob("daily_user_features")
```

---

## Best Practices

### 1. Feature Versioning

Enable versioning to track feature changes over time:

```go
config := offlinesync.Config{
    EnableVersioning: true,
    VersionRetention: 7 * 24 * time.Hour,  // Keep 7 days
}
```

### 2. Batch Size Tuning

Optimize batch size based on your workload:

| Scenario | Recommended Batch Size |
|----------|----------------------|
| Small features (<100 bytes) | 10,000 - 50,000 |
| Large features (>1KB) | 1,000 - 5,000 |
| High memory pressure | 1,000 - 2,000 |
| Low latency requirement | 100 - 500 |

### 3. Checkpoint Intervals

For Flink streaming, balance latency vs overhead:

| Interval | Use Case |
|----------|----------|
| 1-5 seconds | Low-latency applications |
| 10-30 seconds | Balanced (default) |
| 1-5 minutes | High-throughput, latency-tolerant |

### 4. Schema Validation

Always validate schemas in production:

```go
jobSpec.ValidateSchema = true
```

### 5. Monitoring

Monitor sync operations:

```go
// Spark connector metrics
metrics := sparkConnector.Metrics()
fmt.Printf("Exported: %d rows, Imported: %d rows\n",
    metrics.RowsExported, metrics.RowsImported)

// Flink connector metrics
metrics := flinkConnector.Metrics()
fmt.Printf("Processed: %d, Failed: %d, Latency: %v\n",
    metrics.RecordsProcessed, metrics.RecordsFailed, metrics.ProcessingLatency)

// Sync engine metrics
metrics := engine.Metrics()
fmt.Printf("Jobs: %d completed, %d failed\n",
    metrics.JobsCompleted, metrics.JobsFailed)
```

---

## Troubleshooting

### Spark Export Fails

**Symptom:** Export completes but no data

```go
// Check if entities exist
entities := []string{"user:123", "user:456"}
for _, e := range entities {
    features, err := store.Get(e, nil)
    fmt.Printf("Entity %s: %v features\n", e, len(features))
}
```

**Solution:** Verify entities exist and features are populated

### Flink Backpressure

**Symptom:** `ErrBackpressure` errors

```go
metrics := connector.Metrics()
if metrics.BackpressureEvents > 0 {
    // Increase buffer or reduce parallelism
    config.BufferSize = 50000
}
```

**Solutions:**
1. Increase buffer size
2. Reduce source rate
3. Scale up Flink parallelism

### Sync Job Stuck

**Symptom:** Job stays in "running" state

```go
job, _ := engine.GetJob("stuck_job")
fmt.Printf("Progress: %.1f%%, Records: %d/%d\n",
    job.Progress.Percentage,
    job.Progress.ProcessedRecords,
    job.Progress.TotalRecords)

// Cancel if necessary
engine.CancelJob("stuck_job")
```

**Solutions:**
1. Check source file accessibility
2. Verify store connectivity
3. Increase timeout
4. Check for schema validation errors

### Version Conflicts

**Symptom:** `ErrVersionConflict`

```go
// Use merge strategy to handle conflicts
jobSpec.Strategy = offlinesync.SyncStrategyMerge
```

---

## Further Reading

- [Architecture Overview](./architecture.md) - System design
- [API Reference](./api-reference.md) - Complete API docs
- [Observability Guide](./observability.md) - Monitoring setup
- [Feature Freshness](./freshness.md) - SLA configuration
