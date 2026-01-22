# Feather Java/Kotlin SDK

Official Java/Kotlin client for [Feather Feature Store](https://github.com/feather-store/feather).

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
GetFeaturesResponse response = client.getFeatures("user:123", Arrays.asList("age", "country"));
System.out.println(response.getFeatures());

// Store features
Map<String, Object> features = new HashMap<>();
features.put("age", 25);
features.put("country", "US");
client.putFeatures("user:123", features, null);
```

## Vector Search

```kotlin
// Create a vector index
client.vectors.createIndex("embeddings", 128, DistanceType.COSINE)

// Upsert vectors
client.vectors.upsert("embeddings", listOf(
    VectorRecord("doc:1", listOf(0.1, 0.2, ...), mapOf("title" to "Document 1")),
    VectorRecord("doc:2", listOf(0.3, 0.4, ...), mapOf("title" to "Document 2"))
))

// Search for similar vectors
val results = client.vectors.search("embeddings", queryVector, 10)
results.forEach { result ->
    println("${result.id}: ${result.score}")
}
```

## Point-in-Time Queries

```kotlin
import java.time.Instant

// Get features as they were at a specific time
val historicalFeatures = client.getFeaturesAsOf(
    "user:123",
    Instant.parse("2024-01-01T00:00:00Z"),
    listOf("age", "plan")
)
```

## Aggregations

```kotlin
// Get real-time aggregations
val result = client.getAggregation(
    entityId = "user:123",
    feature = "purchase_amount",
    function = AggFunction.SUM,
    windowSeconds = 3600 // 1 hour window
)
println("Total: ${result.value}")
```

## Configuration

```kotlin
val client = FeatherClient(ClientConfig(
    // Required: Feather server URL
    baseUrl = "http://localhost:8080",

    // Optional: Request timeout (default: 30 seconds)
    timeout = Duration.ofSeconds(10),

    // Optional: API key for authentication
    apiKey = "your-api-key",

    // Optional: Additional headers
    headers = mapOf("X-Custom-Header" to "value"),

    // Optional: Retry configuration
    maxRetries = 3,
    initialRetryDelay = Duration.ofMillis(100),
    maxRetryDelay = Duration.ofSeconds(5)
))
```

## Error Handling

```kotlin
import io.feather.client.*

try {
    client.getFeatures("unknown-entity")
} catch (e: NotFoundException) {
    println("Entity not found")
} catch (e: ValidationException) {
    println("Invalid request: ${e.message}")
} catch (e: ConnectionException) {
    println("Connection failed: ${e.message}")
} catch (e: FeatherException) {
    println("Error ${e.statusCode}: ${e.message}")
}
```

## Health Checks

```kotlin
// Full health status
val health = client.health()
println("Status: ${health.status}")
health.components.forEach { (name, component) ->
    println("  $name: ${component.status}")
}

// Simple readiness check
val isReady = client.ready()
```

## Resource Management

The client uses OkHttp under the hood. Remember to close the client when done:

```kotlin
client.close()
```

Or use Kotlin's `use` function:

```kotlin
FeatherClient(config).use { client ->
    // Use client
}
```

## Requirements

- Java 17+
- Kotlin 1.9+ (for Kotlin usage)

## License

Apache 2.0 - see [LICENSE](../../LICENSE) for details.
