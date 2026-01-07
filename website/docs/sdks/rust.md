---
sidebar_position: 5
title: Rust SDK
description: Official Rust client for Feather feature store.
---

# Rust SDK

Official Rust client for Feather feature store. Async-first design with full type safety.

## Installation

Add to your `Cargo.toml`:

```toml
[dependencies]
feather-client = "0.1.0"
tokio = { version = "1", features = ["full"] }
```

## Quick Start

```rust
use feather_client::{FeatherClient, ClientConfig};
use std::collections::HashMap;

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    // Create client
    let client = FeatherClient::new(ClientConfig {
        base_url: "http://localhost:8080".to_string(),
        ..Default::default()
    })?;

    // Check health
    let health = client.health().await?;
    println!("Server status: {}", health.status);

    // Get features
    let features = client.get_features("user:123", Some(&["age", "country"])).await?;
    println!("Features: {:?}", features);

    // Store features
    let mut feature_map = HashMap::new();
    feature_map.insert("age".to_string(), serde_json::json!(25));
    feature_map.insert("country".to_string(), serde_json::json!("US"));
    client.put_features("user:123", feature_map, None).await?;

    Ok(())
}
```

## Feature Operations

### Get Features

```rust
use feather_client::{FeatherClient, ClientConfig};

// Get specific features
let response = client.get_features("user:123", Some(&["age", "country"])).await?;
println!("Entity: {}", response.entity_id);
for (name, value) in &response.features {
    println!("  {}: {:?}", name, value.value);
}

// Get all features for an entity (pass None)
let all_features = client.get_features("user:123", None).await?;
```

### Store Features

```rust
use std::collections::HashMap;
use serde_json::json;
use chrono::Utc;

// Store with current timestamp
let mut features = HashMap::new();
features.insert("clicks".to_string(), json!(42));
features.insert("purchases".to_string(), json!(150.0));
features.insert("is_premium".to_string(), json!(true));
client.put_features("user:123", features, None).await?;

// Store with explicit timestamp
let timestamp = Utc::now() - chrono::Duration::hours(1);
client.put_features("user:123", features, Some(timestamp)).await?;
```

### Batch Operations

Retrieve features for multiple entities efficiently:

```rust
let batch = client.batch_get(
    &["user:123", "user:456", "user:789"],
    Some(&["clicks", "purchases"])
).await?;

for entity in batch.results {
    println!("{}: {:?}", entity.entity_id, entity.features);
}
```

## Point-in-Time Queries

Retrieve features as they existed at a specific timestamp:

```rust
use chrono::Utc;

// Get features from 1 hour ago
let as_of = Utc::now() - chrono::Duration::hours(1);
let historical = client.get_features_as_of(
    "user:123",
    as_of,
    Some(&["balance", "plan"])
).await?;

println!("Balance 1 hour ago: {:?}", historical.features.get("balance"));
```

This is essential for generating training data without data leakage.

## Aggregations

Get real-time sliding window aggregations:

```rust
use feather_client::AggFunction;

let agg = client.get_aggregation(
    "user:123",
    "purchase_amount",
    AggFunction::Sum,
    3600, // 1 hour window in seconds
).await?;
println!("Sum of purchases: {}", agg.value);
```

Available aggregation functions:

| Function | Description |
|----------|-------------|
| `AggFunction::Count` | Number of values |
| `AggFunction::Sum` | Sum of values |
| `AggFunction::Avg` | Average of values |
| `AggFunction::Min` | Minimum value |
| `AggFunction::Max` | Maximum value |

## Vector Search

Feather includes built-in vector similarity search:

### Create an Index

```rust
use feather_client::DistanceType;

let vectors = client.vectors();

// Create a vector index
let index = vectors.create_index(
    "embeddings",
    384,  // dimension
    Some(DistanceType::Cosine),
).await?;
println!("Created index: {} with {} dimensions", index.name, index.dimension);
```

### Upsert Vectors

```rust
use feather_client::VectorRecord;
use std::collections::HashMap;
use serde_json::json;

let records = vec![
    VectorRecord {
        id: "doc1".to_string(),
        vector: vec![0.1, 0.2, 0.3, /* ... 384 dimensions */],
        metadata: Some(HashMap::from([
            ("title".to_string(), json!("Hello World")),
            ("category".to_string(), json!("tech")),
        ])),
    },
    VectorRecord {
        id: "doc2".to_string(),
        vector: vec![0.4, 0.5, 0.6, /* ... */],
        metadata: Some(HashMap::from([
            ("title".to_string(), json!("Rust Programming")),
        ])),
    },
];

let count = vectors.upsert("embeddings", records).await?;
println!("Upserted {} vectors", count);
```

### Search for Similar Vectors

```rust
let results = vectors.search(
    "embeddings",
    vec![0.1, 0.2, 0.3, /* query vector */],
    Some(10),           // top_k
    None,               // filter
    Some(true),         // include_metadata
    Some(false),        // include_vector
).await?;

for result in results {
    println!("ID: {}, Score: {:.4}", result.id, result.score);
    if let Some(meta) = result.metadata {
        println!("  Metadata: {:?}", meta);
    }
}
```

### Manage Indexes

```rust
// List all indexes
let indexes = vectors.list_indexes().await?;
for index in indexes {
    println!("{}: {} dimensions, {} vectors", index.name, index.dimension, index.count);
}

// Get a specific vector
if let Some(record) = vectors.get("embeddings", "doc1").await? {
    println!("Found vector: {}", record.id);
}

// Delete a vector
vectors.delete("embeddings", "doc1").await?;

// Delete an index
vectors.delete_index("embeddings").await?;
```

## Schema Operations

Work with feature groups:

```rust
// List feature groups
let groups = client.list_feature_groups().await?;
for group in groups {
    println!("Group: {} (entity: {})", group.name, group.entity_type);
    for feature in &group.features {
        println!("  - {}: {:?}", feature.name, feature.data_type);
    }
}

// Get a specific feature group
if let Some(group) = client.get_feature_group("user_features").await? {
    println!("Found group: {}", group.name);
}
```

## Configuration

Full configuration options:

```rust
use feather_client::ClientConfig;
use std::time::Duration;
use std::collections::HashMap;

let config = ClientConfig {
    // Base URL of the Feather server
    base_url: "http://localhost:8080".to_string(),

    // Request timeout (default: 30s)
    timeout: Duration::from_secs(30),

    // API key for authentication
    api_key: Some("your-api-key".to_string()),

    // Additional headers
    headers: HashMap::from([
        ("X-Custom-Header".to_string(), "value".to_string()),
        ("X-Request-Source".to_string(), "my-service".to_string()),
    ]),

    // Retry configuration
    max_retries: 3,
    initial_retry_delay: Duration::from_millis(100),
    max_retry_delay: Duration::from_secs(5),
};

let client = FeatherClient::new(config)?;
```

### Default Configuration

```rust
// Use all defaults
let client = FeatherClient::new(ClientConfig {
    base_url: "http://localhost:8080".to_string(),
    ..Default::default()
})?;
```

## Error Handling

The SDK provides typed errors:

```rust
use feather_client::{FeatherClient, FeatherError};

match client.get_features("user:123", None).await {
    Ok(response) => {
        println!("Features: {:?}", response.features);
    }
    Err(FeatherError::NotFound(msg)) => {
        println!("Entity not found: {}", msg);
    }
    Err(FeatherError::Validation(msg)) => {
        println!("Invalid request: {}", msg);
    }
    Err(FeatherError::Authentication(msg)) => {
        println!("Auth failed: {}", msg);
    }
    Err(FeatherError::RateLimit(msg)) => {
        println!("Rate limited: {}", msg);
    }
    Err(FeatherError::Timeout) => {
        println!("Request timed out");
    }
    Err(FeatherError::Connection(msg)) => {
        println!("Connection error: {}", msg);
    }
    Err(e) => {
        println!("Other error: {}", e);
    }
}
```

### Error Inspection

```rust
if let Err(e) = result {
    // Check error type
    if e.is_not_found() {
        // Handle not found
    }

    // Check if error is retryable
    if e.is_retryable() {
        // Could retry this request
    }

    // Get HTTP status code (if applicable)
    if let Some(status) = e.status_code() {
        println!("HTTP status: {}", status);
    }
}
```

## Data Types

The client supports all Feather data types:

| Rust Type | Feather Type | Example |
|-----------|--------------|---------|
| `String` | `string` | `json!("hello")` |
| `i64` | `int64` | `json!(42)` |
| `f64` | `float64` | `json!(3.14)` |
| `bool` | `bool` | `json!(true)` |
| `Vec<u8>` | `bytes` | Base64 encoded |
| `DateTime<Utc>` | `timestamp` | RFC3339 string |
| `Vec<String>` | `string_list` | `json!(["a", "b"])` |
| `Vec<i64>` | `int64_list` | `json!([1, 2, 3])` |
| `Vec<f64>` | `float64_list` | `json!([1.0, 2.0])` |
| `HashMap<String, Value>` | `map` | `json!({"key": "value"})` |

Feature values use `serde_json::Value` for flexibility:

```rust
use serde_json::json;

let mut features = HashMap::new();
features.insert("name".to_string(), json!("Alice"));
features.insert("age".to_string(), json!(30));
features.insert("score".to_string(), json!(0.95));
features.insert("active".to_string(), json!(true));
features.insert("tags".to_string(), json!(["vip", "premium"]));
features.insert("metadata".to_string(), json!({
    "source": "api",
    "version": 2
}));
```

## Health Checks

Monitor Feather server health:

```rust
// Full health status
let health = client.health().await?;
println!("Status: {}", health.status);
for (name, component) in &health.components {
    println!("  {}: {}", name, component.status);
}

// Simple readiness check
let is_ready = client.ready().await?;
if !is_ready {
    eprintln!("Server not ready!");
}

// Liveness check
let is_live = client.live().await?;
```

## Async Runtime

This client is async and requires a Tokio runtime:

```rust
#[tokio::main]
async fn main() {
    let client = FeatherClient::new(ClientConfig::default()).unwrap();
    // Use client...
}
```

With a custom runtime:

```rust
fn main() {
    let rt = tokio::runtime::Runtime::new().unwrap();
    rt.block_on(async {
        let client = FeatherClient::new(ClientConfig::default()).unwrap();
        let health = client.health().await.unwrap();
        println!("Status: {}", health.status);
    });
}
```

### Concurrent Requests

```rust
use futures::future::join_all;

let entities = vec!["user:1", "user:2", "user:3"];
let futures: Vec<_> = entities.iter()
    .map(|entity| client.get_features(entity, None))
    .collect();

let results = join_all(futures).await;
for result in results {
    match result {
        Ok(response) => println!("{}: {:?}", response.entity_id, response.features),
        Err(e) => eprintln!("Error: {}", e),
    }
}
```

## Thread Safety

`FeatherClient` is `Send + Sync` and can be safely shared across threads using `Arc`:

```rust
use std::sync::Arc;
use tokio::task;

let client = Arc::new(FeatherClient::new(ClientConfig::default())?);

// Spawn multiple tasks sharing the client
let handles: Vec<_> = (0..10).map(|i| {
    let client = Arc::clone(&client);
    task::spawn(async move {
        let entity = format!("user:{}", i);
        client.get_features(&entity, None).await
    })
}).collect();

// Wait for all tasks
for handle in handles {
    let result = handle.await?;
    println!("{:?}", result);
}
```

## Complete Example

```rust
use feather_client::{FeatherClient, ClientConfig, AggFunction, DistanceType, VectorRecord};
use std::collections::HashMap;
use std::time::Duration;
use serde_json::json;
use chrono::Utc;

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    // Create client with custom config
    let client = FeatherClient::new(ClientConfig {
        base_url: "http://localhost:8080".to_string(),
        timeout: Duration::from_secs(10),
        ..Default::default()
    })?;

    // Check server health
    let health = client.health().await?;
    println!("Server status: {}", health.status);

    // Store features
    let mut features = HashMap::new();
    features.insert("name".to_string(), json!("Alice"));
    features.insert("age".to_string(), json!(30));
    features.insert("premium".to_string(), json!(true));
    features.insert("balance".to_string(), json!(150.50));
    client.put_features("user:123", features, None).await?;
    println!("Features stored");

    // Get features
    let response = client.get_features("user:123", Some(&["name", "age"])).await?;
    println!("Name: {:?}", response.features.get("name"));
    println!("Age: {:?}", response.features.get("age"));

    // Point-in-time query
    let as_of = Utc::now() - chrono::Duration::hours(1);
    let historical = client.get_features_as_of("user:123", as_of, Some(&["balance"])).await?;
    println!("Balance 1 hour ago: {:?}", historical.features.get("balance"));

    // Batch get
    let batch = client.batch_get(
        &["user:123", "user:456"],
        Some(&["name", "premium"])
    ).await?;
    for entity in batch.results {
        println!("{}: {:?}", entity.entity_id, entity.features);
    }

    // Aggregation
    let agg = client.get_aggregation("user:123", "purchases", AggFunction::Sum, 3600).await?;
    println!("Total purchases (1h): {}", agg.value);

    // Vector search
    let vectors = client.vectors();

    // Create index
    vectors.create_index("embeddings", 128, Some(DistanceType::Cosine)).await?;

    // Upsert vectors
    let records = vec![
        VectorRecord {
            id: "doc1".to_string(),
            vector: vec![0.1; 128],
            metadata: Some(HashMap::from([
                ("title".to_string(), json!("Document 1")),
            ])),
        },
    ];
    vectors.upsert("embeddings", records).await?;

    // Search
    let results = vectors.search("embeddings", vec![0.1; 128], Some(5), None, Some(true), None).await?;
    for result in results {
        println!("Found: {} (score: {:.4})", result.id, result.score);
    }

    println!("Done!");
    Ok(())
}
```

## Related Documentation

- [API Reference](/docs/api-reference) - Complete HTTP/gRPC API documentation
- [Python SDK](/docs/sdks/python) - Python client documentation
- [Go SDK](/docs/sdks/go) - Go client documentation
- [Java/Kotlin SDK](/docs/sdks/java) - Java/Kotlin client documentation
- [Configuration](/docs/configuration) - Server configuration options
