---
sidebar_position: 1
title: Go SDK
description: Official Go client for Feather feature store.
---

# Go SDK

The official Go client provides a type-safe, high-performance interface to Feather.

## Installation

```bash
go get github.com/feather-store/feather/sdk/go/feather
```

**Requirements:** Go 1.24+

## Quick Start

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/feather-store/feather/sdk/go/feather"
)

func main() {
    // Create client
    client, err := feather.NewClient("localhost:8080")
    if err != nil {
        log.Fatal(err)
    }
    defer client.Close()

    ctx := context.Background()

    // Store features
    err = client.PutFeatures(ctx, "user:123", map[string]interface{}{
        "click_count":    42,
        "purchase_total": 299.99,
    })
    if err != nil {
        log.Fatal(err)
    }

    // Retrieve features
    features, err := client.GetFeatures(ctx, "user:123",
        []string{"click_count", "purchase_total"})
    if err != nil {
        log.Fatal(err)
    }

    fmt.Printf("Features: %+v\n", features)
}
```

## Client Configuration

### Basic Configuration

```go
client, err := feather.NewClient("localhost:8080")
```

### Advanced Configuration

```go
client, err := feather.NewClient("localhost:8080",
    feather.WithTimeout(5*time.Second),
    feather.WithRetries(3),
    feather.WithConnectionPool(10),
    feather.WithCompression(true),
)
```

### Configuration Options

| Option | Description | Default |
|--------|-------------|---------|
| `WithTimeout(d)` | Request timeout | 30s |
| `WithRetries(n)` | Max retry attempts | 3 |
| `WithConnectionPool(n)` | Connection pool size | 10 |
| `WithCompression(b)` | Enable gzip compression | false |
| `WithTLS(config)` | TLS configuration | nil |
| `WithAuth(token)` | Bearer token auth | "" |

### TLS Configuration

```go
tlsConfig := &tls.Config{
    RootCAs: certPool,
    // Or for mTLS:
    Certificates: []tls.Certificate{clientCert},
}

client, err := feather.NewClient("localhost:8080",
    feather.WithTLS(tlsConfig),
)
```

### gRPC Client

For higher throughput, use the gRPC client:

```go
client, err := feather.NewGRPCClient("localhost:50051",
    feather.WithGRPCTimeout(5*time.Second),
    feather.WithGRPCCompression("gzip"),
)
```

## Storing Features

### Single Entity

```go
err := client.PutFeatures(ctx, "user:123", map[string]interface{}{
    "click_count":    42,
    "purchase_total": 299.99,
    "is_premium":     true,
    "last_activity":  time.Now(),
})
```

### Batch Write

```go
batch := []feather.FeatureUpdate{
    {
        Entity: "user:123",
        Features: map[string]interface{}{
            "click_count": 42,
        },
    },
    {
        Entity: "user:456",
        Features: map[string]interface{}{
            "click_count": 15,
        },
    },
}

err := client.PutFeaturesBatch(ctx, batch)
```

### With Timestamp

```go
// Store with explicit timestamp (for backfill)
err := client.PutFeaturesWithTimestamp(ctx, "user:123",
    map[string]interface{}{
        "click_count": 42,
    },
    time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
)
```

## Retrieving Features

### Single Entity

```go
features, err := client.GetFeatures(ctx, "user:123",
    []string{"click_count", "purchase_total"})

if err != nil {
    if feather.IsNotFound(err) {
        // Entity doesn't exist
    }
    return err
}

clickCount := features["click_count"].(int64)
purchaseTotal := features["purchase_total"].(float64)
```

### All Features for Entity

```go
features, err := client.GetAllFeatures(ctx, "user:123")
```

### Batch Read

```go
entities := []string{"user:123", "user:456", "user:789"}
featureNames := []string{"click_count", "purchase_total"}

results, err := client.GetFeaturesBatch(ctx, entities, featureNames)

for entity, features := range results {
    fmt.Printf("%s: %+v\n", entity, features)
}
```

### With Metadata

```go
features, metadata, err := client.GetFeaturesWithMetadata(ctx, "user:123",
    []string{"click_count"})

for name, meta := range metadata {
    fmt.Printf("%s: updated %v (%d ms ago)\n",
        name, meta.Timestamp, meta.AgeMs)
}
```

## Point-in-Time Queries

### Single Query

```go
asOf := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)

features, err := client.GetFeaturesAsOf(ctx, "user:123",
    []string{"click_count", "purchase_total"},
    asOf)
```

### Batch Point-in-Time

```go
queries := []feather.PITQuery{
    {Entity: "user:123", AsOf: time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)},
    {Entity: "user:456", AsOf: time.Date(2024, 1, 16, 0, 0, 0, 0, time.UTC)},
    {Entity: "user:789", AsOf: time.Date(2024, 1, 17, 0, 0, 0, 0, time.UTC)},
}

results, err := client.GetFeaturesAsOfBatch(ctx, queries,
    []string{"click_count", "purchase_total"})

for _, result := range results {
    fmt.Printf("%s @ %s: %+v\n", result.Entity, result.AsOf, result.Features)
}
```

## Vector Operations

### Create Index

```go
err := client.Vectors.CreateIndex(ctx, feather.VectorIndexConfig{
    Name:       "product_embeddings",
    Dimensions: 384,
    Metric:     "cosine",
    HNSW: &feather.HNSWConfig{
        M:              16,
        EfConstruction: 200,
    },
})
```

### Upsert Vectors

```go
vectors := []feather.Vector{
    {
        ID:     "product:123",
        Values: []float32{0.1, 0.2, 0.3, /* ... */},
        Metadata: map[string]interface{}{
            "category": "electronics",
            "price":    299.99,
        },
    },
    {
        ID:     "product:456",
        Values: []float32{0.4, 0.5, 0.6, /* ... */},
        Metadata: map[string]interface{}{
            "category": "clothing",
            "price":    49.99,
        },
    },
}

err := client.Vectors.Upsert(ctx, "product_embeddings", vectors)
```

### Search Vectors

```go
queryVector := []float32{0.1, 0.2, 0.3, /* ... */}

results, err := client.Vectors.Search(ctx, "product_embeddings",
    feather.SearchRequest{
        Vector: queryVector,
        TopK:   10,
        Filter: map[string]interface{}{
            "category": map[string]string{"$eq": "electronics"},
            "price":    map[string]float64{"$lt": 500},
        },
    })

for _, r := range results {
    fmt.Printf("%s: score=%.3f, metadata=%v\n", r.ID, r.Score, r.Metadata)
}
```

## Schema Management

### List Feature Groups

```go
groups, err := client.Schema.ListGroups(ctx)

for _, group := range groups {
    fmt.Printf("Group: %s (entity: %s)\n", group.Name, group.EntityType)
    for _, feature := range group.Features {
        fmt.Printf("  - %s: %s\n", feature.Name, feature.DataType)
    }
}
```

### Create Feature Group

```go
err := client.Schema.CreateGroup(ctx, feather.FeatureGroup{
    Name:       "user_engagement",
    EntityType: "user",
    TTL:        24 * time.Hour,
    Features: []feather.FeatureDefinition{
        {Name: "click_count", DataType: "int64"},
        {Name: "purchase_total", DataType: "float64"},
        {Name: "is_premium", DataType: "bool"},
    },
})
```

## Drift Detection

```go
// Register feature for monitoring
err := client.Drift.Register(ctx, feather.DriftConfig{
    Feature:         "user:purchase_total",
    WindowSize:      1000,
    DetectionMethod: "ks",
    Threshold:       0.05,
})

// Check status
status, err := client.Drift.Status(ctx)
for _, f := range status.Features {
    if f.Status == "drifted" {
        log.Printf("DRIFT: %s (%.3f > %.3f)", f.Feature, f.MetricValue, f.Threshold)
    }
}

// Get alerts
alerts, err := client.Drift.Alerts(ctx, time.Now().Add(-24*time.Hour))

// Reset reference after expected change
err = client.Drift.Reset(ctx, "user:purchase_total")
```

## Freshness Monitoring

```go
// Get freshness status
status, err := client.Freshness.Status(ctx)

for _, f := range status.Features {
    fmt.Printf("%s: avg age %.1fs\n", f.Feature, float64(f.Freshness.AvgAgeMs)/1000)

    if f.SLA.Status == "breached" {
        fmt.Printf("  SLA BREACHED: %d entities stale\n", f.SLA.EntitiesStale)
    }
}

// Get stale entities
stale, err := client.Freshness.GetStaleEntities(ctx, "user:click_count", 100)
```

## Export

```go
// Export to file
result, err := client.Export(ctx, feather.ExportRequest{
    Format:     "parquet",
    Entities:   []string{"user:*"},
    Features:   []string{"click_count", "purchase_total"},
    OutputPath: "/data/export.parquet",
})

fmt.Printf("Exported %d rows\n", result.RowsExported)

// Point-in-time export
queries := []feather.PITQuery{
    {Entity: "user:123", AsOf: time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)},
    {Entity: "user:456", AsOf: time.Date(2024, 1, 16, 0, 0, 0, 0, time.UTC)},
}

result, err = client.ExportPIT(ctx, feather.PITExportRequest{
    Format:     "parquet",
    Queries:    queries,
    Features:   []string{"click_count", "purchase_total"},
    OutputPath: "s3://bucket/training.parquet",
})
```

## Error Handling

### Error Types

```go
features, err := client.GetFeatures(ctx, "user:123", []string{"click_count"})
if err != nil {
    switch {
    case feather.IsNotFound(err):
        // Entity or feature not found
        log.Printf("Entity not found")

    case feather.IsTimeout(err):
        // Request timed out
        log.Printf("Request timed out, will retry")

    case feather.IsUnavailable(err):
        // Server unavailable
        log.Printf("Server unavailable")

    case feather.IsInvalidArgument(err):
        // Invalid request parameters
        log.Printf("Invalid request: %v", err)

    default:
        // Other error
        log.Printf("Error: %v", err)
    }
}
```

### Retries

The client automatically retries transient errors:

```go
client, err := feather.NewClient("localhost:8080",
    feather.WithRetries(3),
    feather.WithRetryBackoff(100*time.Millisecond),
)
```

## Context and Cancellation

```go
// With timeout
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

features, err := client.GetFeatures(ctx, "user:123", []string{"click_count"})

// With cancellation
ctx, cancel := context.WithCancel(context.Background())

go func() {
    // Cancel if needed
    cancel()
}()

features, err := client.GetFeatures(ctx, "user:123", []string{"click_count"})
if ctx.Err() == context.Canceled {
    log.Println("Request was canceled")
}
```

## Performance Tips

### Connection Pooling

```go
// Increase pool size for high-throughput applications
client, _ := feather.NewClient("localhost:8080",
    feather.WithConnectionPool(50),
)
```

### Batch Operations

```go
// Good: batch requests
results, err := client.GetFeaturesBatch(ctx, entities, features)

// Avoid: individual requests in a loop
for _, entity := range entities {
    features, err := client.GetFeatures(ctx, entity, features) // Slow!
}
```

### gRPC for High Throughput

```go
// Use gRPC for lower latency and higher throughput
client, _ := feather.NewGRPCClient("localhost:50051")
```

## Health Checks

```go
// Simple health check
healthy, err := client.HealthCheck(ctx)

// Detailed health status
status, err := client.Health(ctx)
fmt.Printf("Status: %s\n", status.Status)
for component, health := range status.Components {
    fmt.Printf("  %s: %s\n", component, health.Status)
}
```

## Complete Example

```go
package main

import (
    "context"
    "fmt"
    "log"
    "time"

    "github.com/feather-store/feather/sdk/go/feather"
)

func main() {
    // Create client with configuration
    client, err := feather.NewClient("localhost:8080",
        feather.WithTimeout(5*time.Second),
        feather.WithRetries(3),
    )
    if err != nil {
        log.Fatal(err)
    }
    defer client.Close()

    ctx := context.Background()

    // Store features
    err = client.PutFeatures(ctx, "user:123", map[string]interface{}{
        "click_count":    42,
        "purchase_total": 299.99,
        "is_premium":     true,
    })
    if err != nil {
        log.Fatal(err)
    }

    // Retrieve features
    features, err := client.GetFeatures(ctx, "user:123",
        []string{"click_count", "purchase_total", "is_premium"})
    if err != nil {
        log.Fatal(err)
    }

    fmt.Printf("click_count: %v\n", features["click_count"])
    fmt.Printf("purchase_total: %v\n", features["purchase_total"])
    fmt.Printf("is_premium: %v\n", features["is_premium"])

    // Point-in-time query
    asOf := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
    historical, err := client.GetFeaturesAsOf(ctx, "user:123",
        []string{"click_count"}, asOf)
    if err != nil {
        log.Fatal(err)
    }

    fmt.Printf("Historical click_count (as of %s): %v\n", asOf, historical["click_count"])
}
```

## Related Documentation

- [REST API](./rest-api) - HTTP API reference
- [Python SDK](./python) - Python client
- [Architecture](/docs/concepts/architecture) - System design
