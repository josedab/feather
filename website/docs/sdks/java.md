---
sidebar_position: 4
title: Java/Kotlin SDK
description: Official Java and Kotlin client for Feather feature store.
---

# Java/Kotlin SDK

Official Java and Kotlin client for Feather feature store.

## Installation

### Maven

```xml
<dependency>
    <groupId>io.feather</groupId>
    <artifactId>feather-client</artifactId>
    <version>0.1.0</version>
</dependency>
```

### Gradle (Kotlin DSL)

```kotlin
implementation("io.feather:feather-client:0.1.0")
```

### Gradle (Groovy)

```groovy
implementation 'io.feather:feather-client:0.1.0'
```

## Quick Start

### Kotlin

```kotlin
import io.feather.client.*

// Create a client
val client = FeatherClient(ClientConfig(
    baseUrl = "http://localhost:8080"
))

// Get features for an entity
val features = client.getFeatures("user:123", listOf("age", "country"))
println(features.features)

// Store features
client.putFeatures("user:123", mapOf(
    "age" to 25,
    "country" to "US"
))

// Batch get
val batch = client.batchGet(listOf("user:1", "user:2", "user:3"))
```

### Java

```java
import io.feather.client.*;
import java.util.*;
import java.time.Duration;

// Create a client
ClientConfig config = new ClientConfig(
    "http://localhost:8080",  // baseUrl
    Duration.ofSeconds(30),   // timeout
    null,                     // apiKey
    Collections.emptyMap(),   // headers
    3,                        // maxRetries
    Duration.ofMillis(100),   // initialRetryDelay
    Duration.ofSeconds(5)     // maxRetryDelay
);
FeatherClient client = new FeatherClient(config);

// Get features
GetFeaturesResponse response = client.getFeatures(
    "user:123",
    Arrays.asList("age", "country")
);
System.out.println(response.getFeatures());

// Store features
Map<String, Object> features = new HashMap<>();
features.put("age", 25);
features.put("country", "US");
client.putFeatures("user:123", features, null);
```

## Feature Operations

### Get Features

Retrieve features for a single entity:

```kotlin
// Get specific features
val response = client.getFeatures("user:123", listOf("clicks", "purchases"))

// Access feature values
val clicks = response.features["clicks"]
println("Clicks: ${clicks?.value}, Updated: ${clicks?.timestamp}")

// Get all features for an entity
val allFeatures = client.getFeatures("user:123")
```

### Store Features

Store features with optional timestamp:

```kotlin
import java.time.Instant

// Store with current timestamp
client.putFeatures("user:123", mapOf(
    "clicks" to 42,
    "purchases" to 150.0,
    "is_premium" to true
))

// Store with explicit timestamp
client.putFeatures(
    "user:123",
    mapOf("clicks" to 42),
    Instant.now().minusSeconds(3600)  // 1 hour ago
)
```

### Batch Operations

Retrieve features for multiple entities efficiently:

```kotlin
val response = client.batchGet(
    entities = listOf("user:1", "user:2", "user:3"),
    features = listOf("clicks", "purchases")
)

for (entity in response.results) {
    println("${entity.entityId}: ${entity.features}")
}
```

## Point-in-Time Queries

Retrieve features as they existed at a specific timestamp:

```kotlin
import java.time.Instant

// Get historical feature values
val historicalFeatures = client.getFeaturesAsOf(
    "user:123",
    Instant.parse("2024-01-01T00:00:00Z"),
    listOf("age", "plan")
)

println("Age at that time: ${historicalFeatures.features["age"]?.value}")
```

This is essential for generating training data without data leakage.

## Aggregations

Get real-time sliding window aggregations:

```kotlin
val result = client.getAggregation(
    entityId = "user:123",
    feature = "purchase_amount",
    function = AggFunction.SUM,
    windowSeconds = 3600  // 1 hour window
)
println("Total purchases in last hour: ${result.value}")
```

Available aggregation functions:

| Function | Description |
|----------|-------------|
| `AggFunction.COUNT` | Number of values |
| `AggFunction.SUM` | Sum of values |
| `AggFunction.AVG` | Average of values |
| `AggFunction.MIN` | Minimum value |
| `AggFunction.MAX` | Maximum value |

## Vector Search

Feather includes built-in vector similarity search:

### Create an Index

```kotlin
// Create a vector index
client.vectors.createIndex(
    name = "embeddings",
    dimension = 128,
    distanceType = DistanceType.COSINE
)
```

### Upsert Vectors

```kotlin
client.vectors.upsert("embeddings", listOf(
    VectorRecord(
        id = "doc:1",
        values = listOf(0.1f, 0.2f, /* ... 128 dimensions */),
        metadata = mapOf("title" to "Document 1", "category" to "tech")
    ),
    VectorRecord(
        id = "doc:2",
        values = listOf(0.3f, 0.4f, /* ... */),
        metadata = mapOf("title" to "Document 2", "category" to "science")
    )
))
```

### Search for Similar Vectors

```kotlin
val results = client.vectors.search(
    indexName = "embeddings",
    queryVector = listOf(0.15f, 0.18f, /* ... */),
    topK = 10,
    includeMetadata = true
)

for (result in results) {
    println("${result.id}: score=${result.score}")
    result.metadata?.let { meta ->
        println("  Title: ${meta["title"]}")
    }
}
```

### Manage Indexes

```kotlin
// List all indexes
val indexes = client.vectors.listIndexes()

// Get index info
val info = client.vectors.getIndex("embeddings")
println("Dimension: ${info.dimension}, Count: ${info.count}")

// Delete a vector
client.vectors.deleteVector("embeddings", "doc:1")

// Delete an index
client.vectors.deleteIndex("embeddings")
```

## Schema Operations

Work with feature groups:

```kotlin
// List all feature groups
val groups = client.listFeatureGroups()
for (group in groups) {
    println("Group: ${group.name}")
    for (feature in group.features) {
        println("  - ${feature.name}: ${feature.dataType}")
    }
}

// Get a specific feature group
val userFeatures = client.getFeatureGroup("user_features")
```

## Configuration

Full configuration options:

```kotlin
val client = FeatherClient(ClientConfig(
    // Required: Feather server URL
    baseUrl = "http://localhost:8080",

    // Optional: Request timeout (default: 30 seconds)
    timeout = Duration.ofSeconds(10),

    // Optional: API key for authentication
    apiKey = "your-api-key",

    // Optional: Additional headers
    headers = mapOf(
        "X-Custom-Header" to "value",
        "X-Request-Source" to "my-service"
    ),

    // Optional: Retry configuration
    maxRetries = 3,
    initialRetryDelay = Duration.ofMillis(100),
    maxRetryDelay = Duration.ofSeconds(5)
))
```

## Error Handling

The SDK provides typed exceptions:

```kotlin
import io.feather.client.*

try {
    client.getFeatures("unknown-entity")
} catch (e: NotFoundException) {
    println("Entity not found: ${e.message}")
} catch (e: ValidationException) {
    println("Invalid request: ${e.message}")
} catch (e: AuthenticationException) {
    println("Authentication failed: ${e.message}")
} catch (e: RateLimitException) {
    println("Rate limited, retry after: ${e.retryAfter}")
} catch (e: ConnectionException) {
    println("Connection failed: ${e.message}")
} catch (e: FeatherException) {
    println("Error ${e.statusCode}: ${e.message}")
}
```

### Java Error Handling

```java
try {
    client.getFeatures("unknown-entity", null);
} catch (NotFoundException e) {
    System.out.println("Not found: " + e.getMessage());
} catch (FeatherException e) {
    System.out.println("Error: " + e.getStatusCode() + " - " + e.getMessage());
}
```

## Health Checks

Monitor Feather server health:

```kotlin
// Full health status
val health = client.health()
println("Status: ${health.status}")
health.components.forEach { (name, component) ->
    println("  $name: ${component.status}")
}

// Simple readiness check (returns boolean)
val isReady = client.ready()
if (!isReady) {
    println("Server not ready")
}

// Liveness check
val isLive = client.live()
```

## Resource Management

The client uses OkHttp internally and should be closed when done:

```kotlin
// Manual close
val client = FeatherClient(config)
try {
    // Use client
} finally {
    client.close()
}

// Or use Kotlin's use function
FeatherClient(config).use { client ->
    val features = client.getFeatures("user:123")
}
```

### Java Resource Management

```java
// Try-with-resources
try (FeatherClient client = new FeatherClient(config)) {
    // Use client
}

// Or manual close
FeatherClient client = new FeatherClient(config);
try {
    // Use client
} finally {
    client.close();
}
```

## Async Operations (Kotlin Coroutines)

The SDK provides suspend functions for Kotlin coroutines:

```kotlin
import kotlinx.coroutines.*

suspend fun processUser(client: FeatherClient, userId: String) {
    val features = client.getFeaturesAsync("user:$userId")
    // Process features
}

// Run multiple requests concurrently
runBlocking {
    val results = listOf("user:1", "user:2", "user:3").map { entity ->
        async {
            client.getFeaturesAsync(entity)
        }
    }.awaitAll()
}
```

## Requirements

- **Java**: 17 or higher
- **Kotlin**: 1.9 or higher (for Kotlin usage)
- **Dependencies**: OkHttp 4.x, Moshi (included transitively)

## Thread Safety

`FeatherClient` is thread-safe and can be shared across threads. It's recommended to create a single instance and reuse it:

```kotlin
// Application-level singleton
object Feather {
    val client: FeatherClient by lazy {
        FeatherClient(ClientConfig(baseUrl = "http://localhost:8080"))
    }
}

// Usage anywhere in your application
val features = Feather.client.getFeatures("user:123")
```

## Complete Example

```kotlin
import io.feather.client.*
import java.time.Duration
import java.time.Instant

fun main() {
    // Create client
    val client = FeatherClient(ClientConfig(
        baseUrl = "http://localhost:8080",
        timeout = Duration.ofSeconds(10)
    ))

    client.use {
        // Check health
        val health = it.health()
        println("Server status: ${health.status}")

        // Store features
        it.putFeatures("user:123", mapOf(
            "name" to "Alice",
            "age" to 30,
            "premium" to true,
            "balance" to 150.50
        ))

        // Get features
        val features = it.getFeatures("user:123", listOf("name", "age"))
        println("Name: ${features.features["name"]?.value}")
        println("Age: ${features.features["age"]?.value}")

        // Point-in-time query
        val historical = it.getFeaturesAsOf(
            "user:123",
            Instant.now().minusSeconds(3600),
            listOf("balance")
        )
        println("Balance 1 hour ago: ${historical.features["balance"]?.value}")

        // Batch get
        val batch = it.batchGet(
            listOf("user:123", "user:456"),
            listOf("name", "premium")
        )
        batch.results.forEach { entity ->
            println("${entity.entityId}: ${entity.features}")
        }
    }
}
```

## Related Documentation

- [API Reference](/docs/api-reference) - Complete HTTP/gRPC API documentation
- [Python SDK](/docs/sdks/python) - Python client documentation
- [Go SDK](/docs/sdks/go) - Go client documentation
- [Configuration](/docs/configuration) - Server configuration options
