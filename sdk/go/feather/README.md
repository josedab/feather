# Feather Go SDK

A type-safe, idiomatic Go client for the Feather Feature Store.

## Installation

```bash
go get github.com/feather-store/feather/sdk/go/feather
```

## Quick Start

```go
package main

import (
    "context"
    "log"
    "time"

    "github.com/feather-store/feather/sdk/go/feather"
)

func main() {
    // Create client with default configuration
    client := feather.NewClient("http://localhost:8080", "your-api-key", nil)

    ctx := context.Background()

    // Get features for an entity
    resp, err := client.Features.Get(ctx, "user:123", []string{"purchase_count", "avg_order_value"})
    if err != nil {
        log.Fatal(err)
    }

    for name, val := range resp.Features {
        log.Printf("Feature %s: %v", name, val.Value)
    }

    // Store features
    err = client.Features.Put(ctx, &feather.PutRequest{
        EntityID: "user:123",
        Features: map[string]interface{}{
            "purchase_count":  42,
            "avg_order_value": 55.99,
        },
    })
    if err != nil {
        log.Fatal(err)
    }
}
```

## Configuration

### Client Configuration

```go
config := &feather.ClientConfig{
    Timeout:         30 * time.Second,    // Request timeout
    MaxRetries:      3,                   // Number of retry attempts
    RetryBackoff:    100 * time.Millisecond, // Base delay for exponential backoff
    MaxRetryBackoff: 10 * time.Second,    // Maximum delay between retries
    RetryJitter:     0.2,                 // Jitter factor (0.0-1.0) for randomization
    MaxIdleConns:    100,                 // Max idle HTTP connections
    IdleConnTimeout: 90 * time.Second,    // Idle connection timeout
}

client := feather.NewClient("http://localhost:8080", "your-api-key", config)
```

## Features

### Feature Operations

```go
// Get features for a single entity
resp, err := client.Features.Get(ctx, "user:123", []string{"feature1", "feature2"})

// Get features for multiple entities (batch)
results, err := client.Features.GetBatch(ctx,
    []string{"user:1", "user:2", "user:3"},
    []string{"feature1", "feature2"},
)

// Store features
err = client.Features.Put(ctx, &feather.PutRequest{
    EntityID:  "user:123",
    Features:  map[string]interface{}{"score": 0.95},
    Timestamp: &timestamp, // Optional: set custom timestamp
})

// Point-in-time retrieval
asOf := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
resp, err := client.Features.GetAsOf(ctx, "user:123", []string{"feature1"}, asOf)
```

### Catalog Operations

```go
// Register a feature definition
err := client.Catalog.Register(ctx, &feather.FeatureDefinition{
    Name:        "purchase_count",
    Description: "Total number of purchases",
    DataType:    "int64",
    EntityType:  "user",
    Owner:       "ml-team",
    Tags:        []string{"commerce", "behavior"},
})

// Get feature definition
def, err := client.Catalog.Get(ctx, "purchase_count")

// List features with filter
features, err := client.Catalog.List(ctx, map[string]string{
    "entity_type": "user",
    "owner":       "ml-team",
})

// Search features
results, err := client.Catalog.Search(ctx, "purchase", 10)
```

### Vector Search

```go
// Create a vector index
err := client.Vectors.CreateIndex(ctx, &feather.VectorIndex{
    Name:       "embeddings",
    Dimensions: 384,
    Metric:     "cosine",
})

// Upsert vectors
vectors := map[string][]float64{
    "doc:1": {0.1, 0.2, 0.3, ...},
    "doc:2": {0.4, 0.5, 0.6, ...},
}
metadata := map[string]map[string]interface{}{
    "doc:1": {"title": "Hello World"},
    "doc:2": {"title": "Goodbye World"},
}
err = client.Vectors.Upsert(ctx, "embeddings", vectors, metadata)

// Search for similar vectors
query := []float64{0.1, 0.2, 0.3, ...}
results, err := client.Vectors.Search(ctx, "embeddings", query, 10)
for _, r := range results {
    fmt.Printf("ID: %s, Score: %f\n", r.ID, r.Score)
}
```

### Transform Operations

```go
// Create a transform
err := client.Transform.Create(ctx, &feather.Transform{
    Name:   "normalize_score",
    Type:   "expression",
    Inputs: []string{"raw_score"},
    Output: "normalized_score",
    Expression: "(raw_score - min_score) / (max_score - min_score)",
})

// Execute a transform
result, err := client.Transform.Execute(ctx, "normalize_score", map[string]interface{}{
    "raw_score": 75.5,
    "min_score": 0.0,
    "max_score": 100.0,
})
```

## Advanced Usage

### Connection Pooling

For high-throughput scenarios, use a connection pool:

```go
pool := feather.NewConnectionPool(
    "http://localhost:8080",
    "your-api-key",
    10,   // Pool size
    nil,  // Use default config
)
defer pool.Close()

// Get a client from the pool (round-robin)
client := pool.Get()
resp, err := client.Features.Get(ctx, "user:123", []string{"feature1"})
```

### Batch Client

For efficient bulk writes:

```go
batch := feather.NewBatchClient(client, 100, time.Second)
defer batch.Close()

// Queue writes - they'll be batched automatically
for _, userID := range userIDs {
    err := batch.Put(ctx, userID, map[string]interface{}{
        "last_seen": time.Now().Unix(),
    })
    if err != nil {
        log.Printf("Error: %v", err)
    }
}
```

### Async Client

For non-blocking operations:

```go
async := feather.NewAsyncClient(client)

// Async get
resultCh := async.GetAsync(ctx, "user:123", []string{"feature1"})
// ... do other work ...
result := <-resultCh
if result.Err != nil {
    log.Fatal(result.Err)
}
log.Printf("Got: %v", result.Value)

// Parallel gets
requests := []feather.GetRequest{
    {EntityID: "user:1", Features: []string{"feature1"}},
    {EntityID: "user:2", Features: []string{"feature1"}},
    {EntityID: "user:3", Features: []string{"feature1"}},
}
results := async.ParallelGet(ctx, requests)
```

### Cached Client

For read-heavy workloads:

```go
cached := feather.NewCachedClient(client, &feather.CacheConfig{
    MaxSize: 10000,
    TTL:     5 * time.Minute,
    Enabled: true,
})

// Cached reads
resp, err := cached.Get(ctx, "user:123", []string{"feature1"})

// Invalidate cache entry after write
cached.Invalidate("user:123")
```

### Retry with Custom Config

```go
config := &feather.RetryConfig{
    MaxRetries:     5,
    InitialBackoff: 50 * time.Millisecond,
    MaxBackoff:     5 * time.Second,
    Multiplier:     2.0,
}

result, err := feather.WithRetry(ctx, config, func() (*feather.GetResponse, error) {
    return client.Features.Get(ctx, "user:123", []string{"feature1"})
})
```

## Error Handling

```go
resp, err := client.Features.Get(ctx, "user:123", []string{"feature1"})
if err != nil {
    if apiErr, ok := err.(*feather.APIError); ok {
        switch apiErr.StatusCode {
        case 404:
            log.Printf("Entity not found")
        case 429:
            log.Printf("Rate limited, retry after backoff")
        case 500:
            log.Printf("Server error: %s", apiErr.Message)
        }
    } else {
        log.Printf("Network error: %v", err)
    }
}
```

## Performance Tips

1. **Use Connection Pooling**: For high-throughput workloads, create a pool of clients.

2. **Use Batch Operations**: Prefer `GetBatch` over multiple `Get` calls.

3. **Enable Caching**: For read-heavy workloads with acceptable staleness.

4. **Configure Retries**: Tune retry settings based on your latency requirements.

5. **Use Batch Client for Writes**: Queue writes for automatic batching.

## License

Apache 2.0
