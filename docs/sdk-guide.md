# Client SDK Guide

> Official client libraries for Feather in Go, Python, Rust, TypeScript, and Java.

## Table of Contents

- [Overview](#overview)
- [Go SDK](#go-sdk)
- [Python SDK](#python-sdk)
- [Rust SDK](#rust-sdk)
- [TypeScript SDK](#typescript-sdk)
- [Java SDK](#java-sdk)
- [Common Patterns](#common-patterns)
- [Error Handling](#error-handling)
- [Best Practices](#best-practices)

---

## Overview

Feather provides official client SDKs for seamless integration with your applications. All SDKs provide:

- **Type Safety**: Strongly-typed feature values
- **Batching**: Automatic request batching for efficiency
- **Retries**: Exponential backoff with jitter
- **Connection Pooling**: Efficient connection management
- **Async Support**: Non-blocking operations where applicable

### Quick Comparison

| SDK | Install | Async | Streaming | gRPC |
|-----|---------|-------|-----------|------|
| Go | `go get` | Context-based | Yes | Yes |
| Python | `pip install` | asyncio | Yes | Yes |
| Rust | `cargo add` | tokio | Yes | Yes |
| TypeScript | `npm install` | Promise | Yes | No |
| Java | Maven/Gradle | CompletableFuture | Yes | Yes |

---

## Go SDK

### Installation

```bash
go get github.com/feather-store/feather/sdk/go/feather
```

### Basic Usage

```go
package main

import (
    "context"
    "log"
    "time"

    "github.com/feather-store/feather/sdk/go/feather"
)

func main() {
    // Create client
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
    err = client.Features.Put(ctx, "user:123", map[string]interface{}{
        "click_count":    42,
        "purchase_total": 1250.75,
        "is_premium":     true,
    })
    if err != nil {
        log.Fatal(err)
    }

    // Get features
    features, err := client.Features.Get(ctx, "user:123",
        []string{"click_count", "purchase_total"})
    if err != nil {
        log.Fatal(err)
    }

    log.Printf("Click count: %d", features["click_count"].Int64())
    log.Printf("Purchase total: %.2f", features["purchase_total"].Float64())
}
```

### Client Configuration

```go
client, err := feather.NewClient("localhost:8080",
    // Timeouts
    feather.WithTimeout(10*time.Second),
    feather.WithConnectTimeout(5*time.Second),

    // Retries with exponential backoff
    feather.WithRetries(3),
    feather.WithRetryBackoff(100*time.Millisecond),
    feather.WithMaxRetryBackoff(5*time.Second),
    feather.WithRetryJitter(0.2),  // ±20%

    // Connection pooling
    feather.WithMaxIdleConns(100),
    feather.WithIdleConnTimeout(90*time.Second),

    // Authentication
    feather.WithAPIKey("your-api-key"),

    // TLS
    feather.WithTLS(&tls.Config{
        InsecureSkipVerify: false,
    }),
)
```

### Batch Operations

```go
// Batch get - retrieves features for multiple entities
entities := []string{"user:1", "user:2", "user:3"}
features := []string{"click_count", "purchase_total"}

results, err := client.Features.BatchGet(ctx, entities, features)
if err != nil {
    log.Fatal(err)
}

for entity, feats := range results {
    log.Printf("%s: clicks=%d", entity, feats["click_count"].Int64())
}
```

### Batch Client (Auto-batching)

```go
// Create batch client for high-throughput scenarios
batchClient := feather.NewBatchClient(client,
    feather.WithBatchSize(100),
    feather.WithFlushInterval(100*time.Millisecond),
)

// Add features - automatically batched
for i := 0; i < 10000; i++ {
    batchClient.Put(ctx, fmt.Sprintf("user:%d", i), map[string]interface{}{
        "score": rand.Float64() * 100,
    })
}

// Ensure all writes complete
batchClient.Flush(ctx)
```

### Point-in-Time Queries

```go
// Get features as of a specific time
asOf := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)

historical, err := client.Features.GetAsOf(ctx, "user:123",
    []string{"click_count", "purchase_total"},
    asOf,
)
if err != nil {
    log.Fatal(err)
}
```

### Vector Operations

```go
// Create vector index
err = client.Vectors.CreateIndex(ctx, "products", feather.IndexConfig{
    Dimension:    384,
    DistanceType: feather.DistanceCosine,
})

// Upsert vectors
vectors := []feather.Vector{
    {ID: "prod_1", Values: embedding1, Metadata: map[string]interface{}{"category": "electronics"}},
    {ID: "prod_2", Values: embedding2, Metadata: map[string]interface{}{"category": "books"}},
}
err = client.Vectors.Upsert(ctx, "products", vectors)

// Search
results, err := client.Vectors.Search(ctx, "products", queryVector, feather.SearchOptions{
    TopK: 10,
    Filter: map[string]interface{}{"category": "electronics"},
})
```

### gRPC Client

```go
// Use gRPC for lower latency
grpcClient, err := feather.NewGRPCClient("localhost:50051",
    feather.WithGRPCInsecure(),
    feather.WithGRPCMaxMsgSize(4*1024*1024),
)
defer grpcClient.Close()

// Stream features
stream, err := grpcClient.StreamFeatures(ctx)
for {
    features, err := stream.Recv()
    if err == io.EOF {
        break
    }
    process(features)
}
```

---

## Python SDK

### Installation

```bash
pip install feather-sdk
```

### Basic Usage

```python
from feather import FeatherClient
from datetime import datetime, timedelta

# Create client
client = FeatherClient(
    host="localhost:8080",
    timeout=5.0,
    max_retries=3,
)

# Store features
client.put_features("user:123", {
    "click_count": 42,
    "purchase_total": 1250.75,
    "is_premium": True,
})

# Get features
features = client.get_features("user:123", ["click_count", "purchase_total"])
print(f"Click count: {features['click_count'].value}")
print(f"Purchase total: {features['purchase_total'].value}")
```

### Async Client

```python
import asyncio
from feather import AsyncFeatherClient

async def main():
    async with AsyncFeatherClient("localhost:8080") as client:
        # Concurrent operations
        tasks = [
            client.get_features(f"user:{i}", ["click_count"])
            for i in range(100)
        ]
        results = await asyncio.gather(*tasks)

asyncio.run(main())
```

### Pandas Integration

```python
import pandas as pd
from feather import FeatherClient

client = FeatherClient("localhost:8080")

# Batch get as DataFrame
entities = [f"user:{i}" for i in range(1000)]
features = ["click_count", "purchase_total", "last_activity"]

df = client.batch_get_as_dataframe(entities, features)
print(df.head())
#     entity_key  click_count  purchase_total      last_activity
# 0    user:0           42         1250.75  2024-01-15 10:30:00
# 1    user:1           15          89.99  2024-01-14 22:15:00
# ...
```

### Training Data Export

```python
from feather import FeatherClient
from datetime import datetime, timedelta

client = FeatherClient("localhost:8080")

# Export point-in-time features for training
timestamps = pd.date_range(
    start="2024-01-01",
    end="2024-01-31",
    freq="1H"
)

training_data = client.export_training_data(
    entities=training_entities,
    features=["click_count", "purchase_total", "engagement_score"],
    timestamps=timestamps,
    format="parquet",
    output_path="training_data.parquet"
)
```

### Vector Operations

```python
import numpy as np
from feather import FeatherClient

client = FeatherClient("localhost:8080")

# Create index
client.vectors.create_index(
    name="product_embeddings",
    dimension=384,
    distance_type="cosine"
)

# Upsert vectors
vectors = [
    {"id": "prod_1", "values": embedding1.tolist(), "metadata": {"category": "electronics"}},
    {"id": "prod_2", "values": embedding2.tolist(), "metadata": {"category": "books"}},
]
client.vectors.upsert("product_embeddings", vectors)

# Search
query = model.encode("wireless headphones")
results = client.vectors.search(
    "product_embeddings",
    vector=query.tolist(),
    top_k=10,
    filter={"category": "electronics"}
)

for result in results:
    print(f"{result.id}: {result.score:.3f}")
```

---

## Rust SDK

### Installation

```toml
# Cargo.toml
[dependencies]
feather-sdk = "1.0"
tokio = { version = "1", features = ["full"] }
```

### Basic Usage

```rust
use feather_sdk::{Client, FeatureValue};
use std::collections::HashMap;

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    // Create client
    let client = Client::builder()
        .address("localhost:8080")
        .timeout(std::time::Duration::from_secs(5))
        .build()?;

    // Store features
    let mut features = HashMap::new();
    features.insert("click_count", FeatureValue::Int64(42));
    features.insert("purchase_total", FeatureValue::Float64(1250.75));

    client.put_features("user:123", features).await?;

    // Get features
    let result = client
        .get_features("user:123", &["click_count", "purchase_total"])
        .await?;

    if let Some(clicks) = result.get("click_count") {
        println!("Click count: {}", clicks.as_i64().unwrap());
    }

    Ok(())
}
```

### Async Streams

```rust
use futures::StreamExt;

// Stream feature updates
let mut stream = client.stream_features("user:*").await?;

while let Some(update) = stream.next().await {
    match update {
        Ok(features) => {
            println!("Entity: {}, Features: {:?}", features.entity_key, features.values);
        }
        Err(e) => eprintln!("Error: {}", e),
    }
}
```

### Connection Pool

```rust
use feather_sdk::{Client, PoolConfig};

let client = Client::builder()
    .address("localhost:8080")
    .pool_config(PoolConfig {
        max_connections: 100,
        min_connections: 10,
        acquire_timeout: Duration::from_secs(5),
        idle_timeout: Duration::from_secs(60),
    })
    .build()?;
```

---

## TypeScript SDK

### Installation

```bash
npm install @feather/sdk
# or
yarn add @feather/sdk
```

### Basic Usage

```typescript
import { FeatherClient } from '@feather/sdk';

// Create client
const client = new FeatherClient({
  host: 'localhost:8080',
  timeout: 5000,
  maxRetries: 3,
});

async function main() {
  // Store features
  await client.putFeatures('user:123', {
    click_count: 42,
    purchase_total: 1250.75,
    is_premium: true,
  });

  // Get features
  const features = await client.getFeatures('user:123', [
    'click_count',
    'purchase_total',
  ]);

  console.log(`Click count: ${features.click_count.value}`);
  console.log(`Purchase total: ${features.purchase_total.value}`);
}

main();
```

### Type Safety

```typescript
import { FeatherClient, FeatureSchema } from '@feather/sdk';

// Define feature schema
interface UserFeatures extends FeatureSchema {
  click_count: number;
  purchase_total: number;
  is_premium: boolean;
  last_activity: Date;
}

const client = new FeatherClient({ host: 'localhost:8080' });

// Type-safe operations
const features = await client.getFeatures<UserFeatures>('user:123', [
  'click_count',
  'purchase_total',
]);

// TypeScript knows these types
const clicks: number = features.click_count.value;
const total: number = features.purchase_total.value;
```

### React Integration

```typescript
import { useFeatures, FeatherProvider } from '@feather/sdk/react';

// Provider at app root
function App() {
  return (
    <FeatherProvider host="localhost:8080">
      <UserProfile userId="123" />
    </FeatherProvider>
  );
}

// Hook for feature retrieval
function UserProfile({ userId }: { userId: string }) {
  const { data, loading, error } = useFeatures(
    `user:${userId}`,
    ['click_count', 'purchase_total']
  );

  if (loading) return <div>Loading...</div>;
  if (error) return <div>Error: {error.message}</div>;

  return (
    <div>
      <p>Clicks: {data.click_count.value}</p>
      <p>Purchases: ${data.purchase_total.value}</p>
    </div>
  );
}
```

---

## Java SDK

### Installation

**Maven:**
```xml
<dependency>
  <groupId>io.feather</groupId>
  <artifactId>feather-sdk</artifactId>
  <version>1.0.0</version>
</dependency>
```

**Gradle:**
```groovy
implementation 'io.feather:feather-sdk:1.0.0'
```

### Basic Usage

```java
import io.feather.sdk.FeatherClient;
import io.feather.sdk.FeatureValue;

public class Example {
    public static void main(String[] args) {
        // Create client
        FeatherClient client = FeatherClient.builder()
            .host("localhost:8080")
            .timeout(Duration.ofSeconds(5))
            .maxRetries(3)
            .build();

        // Store features
        Map<String, Object> features = Map.of(
            "click_count", 42,
            "purchase_total", 1250.75,
            "is_premium", true
        );
        client.putFeatures("user:123", features);

        // Get features
        Map<String, FeatureValue> result = client.getFeatures(
            "user:123",
            List.of("click_count", "purchase_total")
        );

        System.out.println("Click count: " + result.get("click_count").asLong());
        System.out.println("Purchase total: " + result.get("purchase_total").asDouble());

        client.close();
    }
}
```

### Async Operations

```java
import java.util.concurrent.CompletableFuture;

// Async get
CompletableFuture<Map<String, FeatureValue>> future = client.getFeaturesAsync(
    "user:123",
    List.of("click_count", "purchase_total")
);

future.thenAccept(features -> {
    System.out.println("Received: " + features);
}).exceptionally(e -> {
    System.err.println("Error: " + e.getMessage());
    return null;
});
```

### Spring Integration

```java
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import io.feather.sdk.FeatherClient;

@Configuration
public class FeatherConfig {

    @Bean
    public FeatherClient featherClient() {
        return FeatherClient.builder()
            .host("${feather.host:localhost:8080}")
            .timeout(Duration.ofSeconds(5))
            .build();
    }
}

// Usage in service
@Service
public class UserService {

    private final FeatherClient featherClient;

    public UserService(FeatherClient featherClient) {
        this.featherClient = featherClient;
    }

    public UserFeatures getUserFeatures(String userId) {
        Map<String, FeatureValue> features = featherClient.getFeatures(
            "user:" + userId,
            List.of("click_count", "purchase_total", "engagement_score")
        );
        return new UserFeatures(features);
    }
}
```

---

## Common Patterns

### Feature Caching

```python
from functools import lru_cache
from feather import FeatherClient

client = FeatherClient("localhost:8080")

@lru_cache(maxsize=1000)
def get_user_features(user_id: str, features: tuple) -> dict:
    return client.get_features(f"user:{user_id}", list(features))

# Usage - cached for repeated calls
features = get_user_features("123", ("click_count", "purchase_total"))
```

### Graceful Degradation

```go
func GetFeaturesWithFallback(ctx context.Context, entityKey string, features []string) (map[string]*FeatureValue, error) {
    // Try feature store
    result, err := client.Features.Get(ctx, entityKey, features)
    if err == nil {
        return result, nil
    }

    // Log warning
    log.Printf("Feature store error: %v, using defaults", err)

    // Return defaults
    defaults := make(map[string]*FeatureValue)
    for _, f := range features {
        defaults[f] = &FeatureValue{Value: defaultValues[f]}
    }
    return defaults, nil
}
```

### Circuit Breaker

```python
from feather import FeatherClient
from pybreaker import CircuitBreaker

breaker = CircuitBreaker(
    fail_max=5,
    reset_timeout=30,
    exclude=[ValueError]
)

@breaker
def get_features_safe(entity_key: str, features: list) -> dict:
    return client.get_features(entity_key, features)

# Usage
try:
    features = get_features_safe("user:123", ["click_count"])
except CircuitBreakerError:
    features = default_features
```

---

## Error Handling

### Error Types

| Error | Description | Recovery |
|-------|-------------|----------|
| `ConnectionError` | Cannot connect to server | Retry with backoff |
| `TimeoutError` | Request timed out | Retry or use defaults |
| `NotFoundError` | Entity/feature not found | Use defaults |
| `ValidationError` | Invalid request | Fix request params |
| `RateLimitError` | Too many requests | Back off and retry |
| `ServerError` | Internal server error | Retry with backoff |

### Go Error Handling

```go
import "github.com/feather-store/feather/sdk/go/feather/errors"

features, err := client.Features.Get(ctx, "user:123", []string{"score"})
if err != nil {
    switch {
    case errors.IsNotFound(err):
        // Entity doesn't exist
        return defaultFeatures, nil
    case errors.IsTimeout(err):
        // Request timed out
        return nil, fmt.Errorf("feature store timeout: %w", err)
    case errors.IsRateLimit(err):
        // Rate limited
        time.Sleep(err.(errors.RateLimitError).RetryAfter)
        return client.Features.Get(ctx, "user:123", []string{"score"})
    default:
        return nil, err
    }
}
```

### Python Error Handling

```python
from feather.exceptions import (
    NotFoundError,
    TimeoutError,
    RateLimitError,
    FeatherError
)

try:
    features = client.get_features("user:123", ["score"])
except NotFoundError:
    features = default_features
except TimeoutError:
    logger.warning("Feature store timeout, using cache")
    features = cache.get("user:123")
except RateLimitError as e:
    time.sleep(e.retry_after)
    features = client.get_features("user:123", ["score"])
except FeatherError as e:
    logger.error(f"Feature store error: {e}")
    raise
```

---

## Best Practices

### 1. Use Batch Operations

```python
# Bad: Individual requests
for user_id in user_ids:
    features = client.get_features(f"user:{user_id}", ["score"])

# Good: Batch request
entities = [f"user:{uid}" for uid in user_ids]
all_features = client.batch_get(entities, ["score"])
```

### 2. Request Only Needed Features

```python
# Bad: Get all features
features = client.get_features("user:123")

# Good: Specify needed features
features = client.get_features("user:123", ["score", "segment"])
```

### 3. Handle Missing Features

```python
def get_score(user_id: str) -> float:
    features = client.get_features(f"user:{user_id}", ["score"])
    if "score" not in features or features["score"].value is None:
        return DEFAULT_SCORE
    return features["score"].value
```

### 4. Use Connection Pooling

```go
// Configure connection pool appropriately for your workload
client, _ := feather.NewClient("localhost:8080",
    feather.WithMaxIdleConns(100),      // Match expected concurrency
    feather.WithIdleConnTimeout(90*time.Second),
)
```

### 5. Set Appropriate Timeouts

```python
# Set timeouts based on your SLA requirements
client = FeatherClient(
    host="localhost:8080",
    timeout=1.0,           # 1 second for online serving
    connect_timeout=0.5,   # 500ms to establish connection
)
```

---

## Further Reading

- [API Reference](./api-reference.md) - Complete API documentation
- [Architecture Overview](./architecture.md) - System design
- [Performance Guide](./performance.md) - Optimization tips
