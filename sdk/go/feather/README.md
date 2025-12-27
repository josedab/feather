# Feather Go SDK

<p align="center">
  <strong>Type-safe, idiomatic Go client for the Feather Feature Store</strong>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/go-%3E%3D1.21-blue.svg" alt="Go Version">
  <img src="https://img.shields.io/badge/license-Apache%202.0-green.svg" alt="License">
</p>

---

The Feather Go SDK provides a high-performance, fully-typed client for interacting with the Feather Feature Store. It supports all Feather features including real-time feature serving, batch operations, point-in-time queries, and vector similarity search.

## Table of Contents

- [Installation](#installation)
- [Quick Start](#quick-start)
- [Configuration](#configuration)
- [Feature Operations](#feature-operations)
- [Batch Operations](#batch-operations)
- [Point-in-Time Queries](#point-in-time-queries)
- [Vector Search](#vector-search)
- [Schema Management](#schema-management)
- [Advanced Usage](#advanced-usage)
- [Error Handling](#error-handling)
- [Performance Optimization](#performance-optimization)
- [Testing](#testing)
- [Troubleshooting](#troubleshooting)

---

## Installation

```bash
go get github.com/feather-store/feather/sdk/go/feather
```

### Requirements

- Go 1.21 or later
- Feather server running (default: `http://localhost:8080`)

---

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
    client := feather.NewClient("http://localhost:8080", "your-api-key", nil)
    ctx := context.Background()

    // Store features
    err := client.Features.Put(ctx, &feather.PutRequest{
        EntityID: "user:123",
        Features: map[string]interface{}{
            "purchase_count":  42,
            "avg_order_value": 55.99,
            "is_premium":      true,
        },
    })
    if err != nil {
        log.Fatal(err)
    }

    // Retrieve features
    resp, err := client.Features.Get(ctx, "user:123", []string{"purchase_count", "avg_order_value"})
    if err != nil {
        log.Fatal(err)
    }

    for name, val := range resp.Features {
        fmt.Printf("%s: %v\n", name, val.Value)
    }
}
```

---

## Configuration

### Client Options

```go
config := &feather.ClientConfig{
    // Timeout for individual requests
    Timeout: 30 * time.Second,

    // Retry configuration
    MaxRetries:      3,                        // Number of retry attempts
    RetryBackoff:    100 * time.Millisecond,   // Initial backoff duration
    MaxRetryBackoff: 10 * time.Second,         // Maximum backoff duration
    RetryJitter:     0.2,                      // Jitter factor (0.0-1.0)

    // Connection pool settings
    MaxIdleConns:    100,                      // Max idle connections per host
    IdleConnTimeout: 90 * time.Second,         // Idle connection timeout

    // TLS configuration (optional)
    TLSConfig: &tls.Config{
        InsecureSkipVerify: false,
    },
}

client := feather.NewClient("http://localhost:8080", "your-api-key", config)
```

### Environment Variables

The SDK respects these environment variables:

| Variable | Description | Default |
|----------|-------------|---------|
| `FEATHER_URL` | Server URL | `http://localhost:8080` |
| `FEATHER_API_KEY` | API key for authentication | — |
| `FEATHER_TIMEOUT` | Request timeout | `30s` |

```go
// Create client from environment
client := feather.NewClientFromEnv()
```

---

## Feature Operations

### Get Features

Retrieve features for a single entity:

```go
resp, err := client.Features.Get(ctx, "user:123", []string{"purchase_count", "avg_order_value"})
if err != nil {
    log.Fatal(err)
}

// Access feature values
for name, feature := range resp.Features {
    fmt.Printf("Feature: %s\n", name)
    fmt.Printf("  Value: %v\n", feature.Value)
    fmt.Printf("  Timestamp: %d\n", feature.Timestamp)
    fmt.Printf("  Version: %d\n", feature.Version)
}
```

### Get All Features

Retrieve all features for an entity (when you don't know the feature names):

```go
resp, err := client.Features.GetAll(ctx, "user:123")
if err != nil {
    log.Fatal(err)
}

for name, feature := range resp.Features {
    fmt.Printf("%s = %v\n", name, feature.Value)
}
```

### Store Features

Store or update features for an entity:

```go
err := client.Features.Put(ctx, &feather.PutRequest{
    EntityID: "user:123",
    Features: map[string]interface{}{
        "purchase_count":  42,
        "avg_order_value": 55.99,
        "last_activity":   time.Now(),
        "preferences":     []string{"electronics", "books"},
    },
    Timestamp: nil,  // Use server time (default)
    Version:   nil,  // Auto-increment (default)
})
if err != nil {
    log.Fatal(err)
}
```

### Delete Features

Delete specific features for an entity:

```go
err := client.Features.Delete(ctx, "user:123", []string{"old_feature"})
```

---

## Batch Operations

### Batch Get

Retrieve features for multiple entities efficiently:

```go
results, err := client.Features.GetBatch(ctx,
    []string{"user:1", "user:2", "user:3", "user:4", "user:5"},
    []string{"purchase_count", "avg_order_value"},
)
if err != nil {
    log.Fatal(err)
}

for entityID, features := range results {
    fmt.Printf("Entity: %s\n", entityID)
    for name, val := range features {
        fmt.Printf("  %s: %v\n", name, val.Value)
    }
}
```

### Batch Put

Store features for multiple entities in one request:

```go
updates := []*feather.PutRequest{
    {EntityID: "user:1", Features: map[string]interface{}{"score": 0.95}},
    {EntityID: "user:2", Features: map[string]interface{}{"score": 0.87}},
    {EntityID: "user:3", Features: map[string]interface{}{"score": 0.92}},
}

results, err := client.Features.PutBatch(ctx, updates)
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Successfully updated %d entities\n", results.SuccessCount)
if len(results.Errors) > 0 {
    for entityID, err := range results.Errors {
        log.Printf("Failed to update %s: %v", entityID, err)
    }
}
```

---

## Point-in-Time Queries

Retrieve feature values as they existed at a specific timestamp—essential for generating training data without data leakage:

```go
// Get features as of a specific time
asOf := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
resp, err := client.Features.GetAsOf(ctx, "user:123", []string{"purchase_count"}, asOf)
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Purchase count at %s: %v\n", asOf, resp.Features["purchase_count"].Value)
```

### Batch Point-in-Time

For training data generation, retrieve historical features for many entities:

```go
// Multiple entities at the same timestamp
queries := []feather.AsOfQuery{
    {EntityID: "user:1", AsOf: labelTime1},
    {EntityID: "user:2", AsOf: labelTime2},
    {EntityID: "user:3", AsOf: labelTime3},
}

results, err := client.Features.GetAsOfBatch(ctx, queries, []string{"purchase_count", "avg_order_value"})
```

---

## Vector Search

### Create Index

```go
err := client.Vectors.CreateIndex(ctx, &feather.VectorIndex{
    Name:       "product_embeddings",
    Dimensions: 384,
    Metric:     feather.MetricCosine,  // or MetricEuclidean, MetricManhattan
    Config: &feather.HNSWConfig{
        M:              16,   // Max connections per node
        EfConstruction: 200,  // Construction-time search depth
    },
})
if err != nil {
    log.Fatal(err)
}
```

### Upsert Vectors

```go
vectors := []feather.Vector{
    {
        ID:     "prod:001",
        Values: embedding1,  // []float32 with 384 dimensions
        Metadata: map[string]interface{}{
            "name":     "Wireless Headphones",
            "category": "electronics",
            "price":    99.99,
        },
    },
    {
        ID:     "prod:002",
        Values: embedding2,
        Metadata: map[string]interface{}{
            "name":     "Bluetooth Speaker",
            "category": "electronics",
            "price":    49.99,
        },
    },
}

err := client.Vectors.Upsert(ctx, "product_embeddings", vectors)
if err != nil {
    log.Fatal(err)
}
```

### Search Similar Vectors

```go
results, err := client.Vectors.Search(ctx, &feather.SearchRequest{
    Index:           "product_embeddings",
    Vector:          queryEmbedding,
    TopK:            10,
    IncludeMetadata: true,
    IncludeVectors:  false,
    Filter: map[string]interface{}{
        "category": "electronics",
    },
})
if err != nil {
    log.Fatal(err)
}

for _, result := range results {
    fmt.Printf("ID: %s, Score: %.4f\n", result.ID, result.Score)
    if result.Metadata != nil {
        fmt.Printf("  Name: %s\n", result.Metadata["name"])
    }
}
```

### Delete Vectors

```go
// Delete specific vectors
err := client.Vectors.Delete(ctx, "product_embeddings", []string{"prod:001", "prod:002"})

// Delete entire index
err := client.Vectors.DeleteIndex(ctx, "product_embeddings")
```

---

## Schema Management

### Feature Groups

```go
// List all feature groups
groups, err := client.Schema.ListGroups(ctx)
for _, group := range groups {
    fmt.Printf("Group: %s (entity: %s)\n", group.Name, group.EntityType)
}

// Get a specific group
group, err := client.Schema.GetGroup(ctx, "user_features")

// Create a new group
err := client.Schema.CreateGroup(ctx, &feather.FeatureGroup{
    Name:        "user_engagement",
    EntityType:  "user",
    Description: "User engagement metrics",
    TTL:         30 * 24 * time.Hour,  // 30 days
    Features: []feather.FeatureSpec{
        {
            Name:     "click_count",
            DataType: feather.DataTypeInt64,
            Aggregation: &feather.AggregationSpec{
                Function: feather.AggCount,
                Window:   time.Hour,
            },
        },
        {
            Name:     "session_duration_avg",
            DataType: feather.DataTypeFloat64,
            Validation: &feather.ValidationSpec{
                Min: ptr(0.0),
                Max: ptr(86400.0),  // Max 24 hours
            },
        },
    },
})
```

### Catalog Operations

```go
// Register a feature definition
err := client.Catalog.Register(ctx, &feather.FeatureDefinition{
    Name:        "purchase_count",
    Description: "Total number of purchases by the user",
    DataType:    "int64",
    EntityType:  "user",
    Owner:       "ml-team",
    Tags:        []string{"commerce", "behavior"},
})

// Search features
results, err := client.Catalog.Search(ctx, "purchase", 10)
for _, feature := range results {
    fmt.Printf("%s: %s\n", feature.Name, feature.Description)
}
```

---

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

// Get a client from the pool (round-robin distribution)
client := pool.Get()
resp, err := client.Features.Get(ctx, "user:123", []string{"feature1"})
```

### Batch Client

Automatically batch writes for efficiency:

```go
batch := feather.NewBatchClient(client, &feather.BatchConfig{
    MaxSize:      100,            // Max items per batch
    FlushTimeout: time.Second,    // Max time before flush
    OnError: func(err error) {
        log.Printf("Batch error: %v", err)
    },
})
defer batch.Close()

// Queue writes - they're batched automatically
for _, userID := range userIDs {
    err := batch.Put(ctx, userID, map[string]interface{}{
        "last_seen": time.Now().Unix(),
    })
    if err != nil {
        log.Printf("Queue error: %v", err)
    }
}

// Force flush if needed
batch.Flush()
```

### Async Client

For non-blocking operations:

```go
async := feather.NewAsyncClient(client)

// Single async get
future := async.GetAsync(ctx, "user:123", []string{"feature1"})
// ... do other work ...
result, err := future.Wait()

// Parallel gets with concurrency limit
requests := []feather.GetRequest{
    {EntityID: "user:1", Features: []string{"feature1"}},
    {EntityID: "user:2", Features: []string{"feature1"}},
    {EntityID: "user:3", Features: []string{"feature1"}},
}
results := async.ParallelGet(ctx, requests, 10)  // Max 10 concurrent
```

### Cached Client

For read-heavy workloads with acceptable staleness:

```go
cached := feather.NewCachedClient(client, &feather.CacheConfig{
    MaxSize:    10000,           // Max cached entities
    TTL:        5 * time.Minute, // Cache TTL
    Enabled:    true,
    OnEvict: func(key string) {
        log.Printf("Evicted: %s", key)
    },
})

// Reads are cached
resp1, _ := cached.Get(ctx, "user:123", []string{"feature1"})
resp2, _ := cached.Get(ctx, "user:123", []string{"feature1"})  // Cache hit

// Invalidate after write
cached.Put(ctx, &feather.PutRequest{EntityID: "user:123", Features: updates})
cached.Invalidate("user:123")

// Get cache stats
stats := cached.Stats()
fmt.Printf("Hit rate: %.2f%%\n", stats.HitRate()*100)
```

### Custom HTTP Client

Use your own HTTP client for custom requirements:

```go
httpClient := &http.Client{
    Transport: &http.Transport{
        MaxIdleConns:        200,
        MaxIdleConnsPerHost: 100,
        IdleConnTimeout:     90 * time.Second,
        TLSClientConfig: &tls.Config{
            MinVersion: tls.VersionTLS12,
        },
    },
    Timeout: 30 * time.Second,
}

client := feather.NewClientWithHTTP("http://localhost:8080", "api-key", httpClient)
```

### Interceptors/Middleware

Add custom logic to all requests:

```go
client := feather.NewClient(url, apiKey, config)

// Add logging interceptor
client.Use(func(next feather.Handler) feather.Handler {
    return func(ctx context.Context, req *feather.Request) (*feather.Response, error) {
        start := time.Now()
        resp, err := next(ctx, req)
        log.Printf("%s %s took %v", req.Method, req.Path, time.Since(start))
        return resp, err
    }
})

// Add metrics interceptor
client.Use(feather.MetricsInterceptor(metricsRegistry))
```

---

## Error Handling

### Error Types

```go
resp, err := client.Features.Get(ctx, "user:123", []string{"feature1"})
if err != nil {
    switch e := err.(type) {
    case *feather.NotFoundError:
        log.Printf("Entity not found: %s", e.EntityID)

    case *feather.ValidationError:
        log.Printf("Validation failed: %s", e.Message)
        for field, msg := range e.FieldErrors {
            log.Printf("  %s: %s", field, msg)
        }

    case *feather.RateLimitError:
        log.Printf("Rate limited. Retry after: %v", e.RetryAfter)
        time.Sleep(e.RetryAfter)
        // Retry...

    case *feather.ServerError:
        log.Printf("Server error (%d): %s", e.StatusCode, e.Message)

    case *feather.NetworkError:
        log.Printf("Network error: %v", e.Cause)
        if e.IsTimeout() {
            log.Println("Request timed out")
        }

    default:
        log.Printf("Unknown error: %v", err)
    }
}
```

### Retry with Custom Logic

```go
config := &feather.RetryConfig{
    MaxRetries:     5,
    InitialBackoff: 50 * time.Millisecond,
    MaxBackoff:     5 * time.Second,
    Multiplier:     2.0,
    RetryIf: func(err error) bool {
        // Only retry on transient errors
        var serverErr *feather.ServerError
        if errors.As(err, &serverErr) {
            return serverErr.StatusCode >= 500
        }
        var netErr *feather.NetworkError
        return errors.As(err, &netErr)
    },
}

result, err := feather.WithRetry(ctx, config, func() (*feather.GetResponse, error) {
    return client.Features.Get(ctx, "user:123", []string{"feature1"})
})
```

---

## Performance Optimization

### Best Practices

1. **Reuse the client**: Create one client and reuse it across your application.

2. **Use batch operations**: Prefer `GetBatch` and `PutBatch` over multiple individual calls.

3. **Enable connection pooling**: For high-throughput, use `NewConnectionPool`.

4. **Use caching wisely**: Enable caching for read-heavy workloads with acceptable staleness.

5. **Set appropriate timeouts**: Configure timeouts based on your latency requirements.

### Benchmark Example

```go
func BenchmarkFeatureGet(b *testing.B) {
    client := feather.NewClient("http://localhost:8080", "", nil)
    ctx := context.Background()

    b.ResetTimer()
    b.RunParallel(func(pb *testing.PB) {
        i := 0
        for pb.Next() {
            client.Features.Get(ctx, fmt.Sprintf("user:%d", i%10000), []string{"feature1"})
            i++
        }
    })
}
```

---

## Testing

### Mock Client

Use the mock client for unit testing:

```go
import "github.com/feather-store/feather/sdk/go/feather/mock"

func TestMyFunction(t *testing.T) {
    mockClient := mock.NewMockClient()

    // Setup expectations
    mockClient.Features.On("Get", mock.Anything, "user:123", []string{"score"}).
        Return(&feather.GetResponse{
            Features: map[string]*feather.Feature{
                "score": {Value: 0.95},
            },
        }, nil)

    // Use the mock
    result := myFunction(mockClient)

    // Verify
    mockClient.Features.AssertExpectations(t)
}
```

### Integration Tests

```go
func TestIntegration(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping integration test")
    }

    client := feather.NewClient(os.Getenv("FEATHER_URL"), os.Getenv("FEATHER_API_KEY"), nil)
    ctx := context.Background()

    // Test put and get
    entityID := fmt.Sprintf("test:%d", time.Now().UnixNano())
    err := client.Features.Put(ctx, &feather.PutRequest{
        EntityID: entityID,
        Features: map[string]interface{}{"test_feature": 42},
    })
    require.NoError(t, err)

    resp, err := client.Features.Get(ctx, entityID, []string{"test_feature"})
    require.NoError(t, err)
    assert.Equal(t, 42, resp.Features["test_feature"].Value)
}
```

---

## Troubleshooting

### Common Issues

#### Connection Refused

```
Error: dial tcp 127.0.0.1:8080: connect: connection refused
```

**Solution**: Ensure Feather server is running and accessible at the configured URL.

#### Timeout Errors

```
Error: context deadline exceeded
```

**Solution**: Increase timeout in client config or check network latency.

```go
config := &feather.ClientConfig{
    Timeout: 60 * time.Second,
}
```

#### Rate Limiting

```
Error: rate limited, retry after 1s
```

**Solution**: Implement backoff or use the built-in retry mechanism:

```go
config := &feather.ClientConfig{
    MaxRetries:   5,
    RetryBackoff: 100 * time.Millisecond,
}
```

### Debug Logging

Enable debug logging for troubleshooting:

```go
feather.SetLogLevel(feather.LogDebug)

// Or use a custom logger
feather.SetLogger(myLogger)
```

### Health Check

Verify server connectivity:

```go
health, err := client.Health(ctx)
if err != nil {
    log.Fatal("Server unreachable:", err)
}

if health.Status != "healthy" {
    log.Printf("Server degraded: %v", health.Components)
}
```

---

## API Reference

For complete API documentation, see the [Go package documentation](https://pkg.go.dev/github.com/feather-store/feather/sdk/go/feather).

---

## License

Apache 2.0 - See [LICENSE](LICENSE) for details.
