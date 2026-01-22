# Feather Rust Client

A Rust client library for [Feather Feature Store](https://github.com/feather-store/feather).

## Installation

Add to your `Cargo.toml`:

```toml
[dependencies]
feather-client = "0.1.0"
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

## Features

### Feature Operations

```rust
use feather_client::{FeatherClient, ClientConfig, AggFunction};
use chrono::Utc;

// Get features for an entity
let response = client.get_features("user:123", Some(&["age", "country"])).await?;
println!("Entity: {}", response.entity_id);
for (name, value) in &response.features {
    println!("  {}: {}", name, value);
}

// Batch get features for multiple entities
let batch = client.batch_get(&["user:123", "user:456"], Some(&["age"])).await?;
for entity in batch.results {
    println!("{}: {:?}", entity.entity_id, entity.features);
}

// Point-in-time feature retrieval
let as_of = Utc::now() - chrono::Duration::hours(1);
let historical = client.get_features_as_of("user:123", as_of, None).await?;

// Aggregations
let agg = client.get_aggregation(
    "user:123",
    "purchase_amount",
    AggFunction::Sum,
    3600, // 1 hour window
).await?;
println!("Sum of purchases: {}", agg.value);
```

### Vector Operations

```rust
use feather_client::{FeatherClient, VectorRecord, DistanceType};
use std::collections::HashMap;

let vectors = client.vectors();

// Create an index
let index = vectors.create_index(
    "embeddings",
    384,
    Some(DistanceType::Cosine),
).await?;
println!("Created index: {} with {} dimensions", index.name, index.dimension);

// Upsert vectors
let records = vec![
    VectorRecord {
        id: "doc1".to_string(),
        vector: vec![0.1, 0.2, 0.3], // 384 dimensions in practice
        metadata: Some(HashMap::from([
            ("title".to_string(), serde_json::json!("Hello World")),
        ])),
    },
];
let count = vectors.upsert("embeddings", records).await?;
println!("Upserted {} vectors", count);

// Search for similar vectors
let results = vectors.search(
    "embeddings",
    vec![0.1, 0.2, 0.3], // query vector
    Some(10),            // top_k
    None,                // filter
    Some(true),          // include_metadata
    Some(false),         // include_vector
).await?;

for result in results {
    println!("ID: {}, Score: {:.4}", result.id, result.score);
    if let Some(meta) = result.metadata {
        println!("  Metadata: {:?}", meta);
    }
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

### Schema Operations

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
    ]),

    // Retry configuration
    max_retries: 3,
    initial_retry_delay: Duration::from_millis(100),
    max_retry_delay: Duration::from_secs(5),
};

let client = FeatherClient::new(config)?;
```

## Error Handling

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

// Check error types
if let Err(e) = result {
    if e.is_not_found() {
        // Handle not found
    }
    if e.is_retryable() {
        // Could retry this error
    }
}
```

## Data Types

The client supports all Feather data types:

| Rust Type | Feather Type |
|-----------|--------------|
| `String` | `string` |
| `i64` | `int64` |
| `f64` | `float64` |
| `bool` | `bool` |
| `Vec<u8>` | `bytes` |
| `DateTime<Utc>` | `timestamp` |
| `Vec<String>` | `string_list` |
| `Vec<i64>` | `int64_list` |
| `Vec<f64>` | `float64_list` |
| `HashMap<String, Value>` | `map` |

Feature values use `serde_json::Value` for flexibility:

```rust
use serde_json::json;

let mut features = HashMap::new();
features.insert("name".to_string(), json!("Alice"));
features.insert("age".to_string(), json!(30));
features.insert("score".to_string(), json!(0.95));
features.insert("active".to_string(), json!(true));
features.insert("tags".to_string(), json!(["vip", "premium"]));
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

Or with a custom runtime:

```rust
let rt = tokio::runtime::Runtime::new().unwrap();
rt.block_on(async {
    let client = FeatherClient::new(ClientConfig::default()).unwrap();
    // Use client...
});
```

## Thread Safety

`FeatherClient` is `Send + Sync` and can be safely shared across threads using `Arc`:

```rust
use std::sync::Arc;

let client = Arc::new(FeatherClient::new(ClientConfig::default())?);

let client_clone = Arc::clone(&client);
tokio::spawn(async move {
    let features = client_clone.get_features("user:123", None).await;
});
```

## License

Apache 2.0 - See [LICENSE](../../LICENSE) for details.
