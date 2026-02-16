# Feather Kotlin/Android SDK

Official Kotlin/Android client for [Feather Feature Store](https://github.com/feather-store/feather) with offline-first architecture.

## Installation

### Gradle (Kotlin DSL)

```kotlin
implementation("com.feather:feather-sdk:0.1.0")
```

### Gradle (Groovy)

```groovy
implementation 'com.feather:feather-sdk:0.1.0'
```

## Requirements

- Kotlin 1.9+
- Java 17+
- Android API 26+ (for Android projects)

## Quick Start

```kotlin
import com.feather.sdk.*

// Create a client with offline-first defaults
val client = FeatherClient(FeatherConfig(
    baseURL = "http://localhost:8080",
    deviceID = "device-001"
))

// Store a feature locally (queued for sync)
client.put("user_score", "user:123", 0.95)

// Retrieve from local cache (instant, works offline)
val feature = client.get("user_score", "user:123")
println("Score: ${feature?.value}")

// Start background sync with the server
client.startSync()

// Trigger an immediate sync
val result = client.sync()
println("Received: ${result.updatesReceived}, Sent: ${result.updatesSent}")

// Stop background sync
client.stopSync()
```

## Features

### Offline-First Architecture

Features are served from a local in-memory cache. Writes are queued locally and pushed to the server on the next sync cycle.

```kotlin
// Works without network connectivity
val cached = client.get("click_count", "user:456")

// Writes are queued and synced later
client.put("click_count", "user:456", 42)
```

### Background Sync

Automatic delta synchronization with configurable intervals:

```kotlin
val client = FeatherClient(FeatherConfig(
    baseURL = "https://feather.example.com",
    deviceID = "android-001",
    syncIntervalSeconds = 60  // sync every minute
))

client.startSync()
```

### Conflict Resolution

Three strategies for handling conflicts between local and server state:

```kotlin
val config = FeatherConfig(
    baseURL = "http://localhost:8080",
    deviceID = "device-001",
    conflictStrategy = ConflictStrategy.SERVER_WINS  // default
    // Options: SERVER_WINS, CLIENT_WINS, LAST_WRITE_WINS
)
```

### Cache Management

```kotlin
// Configure cache limits
val config = FeatherConfig(
    baseURL = "http://localhost:8080",
    deviceID = "device-001",
    offlineStorageLimit = 10_000  // max cached features
)

// Get cache statistics
val stats = client.cacheStats()
println("Cached: ${stats["size"]}, Pending: ${stats["pending_sync"]}")
```

## API Reference

### `FeatherClient`

| Method | Description |
|--------|-------------|
| `get(featureID, entityKey)` | Retrieve a cached feature (offline-first) |
| `put(featureID, entityKey, value)` | Store a feature locally, queue for sync |
| `sync()` | Perform an immediate sync with the server |
| `startSync()` | Start periodic background sync |
| `stopSync()` | Stop background sync |
| `cacheStats()` | Return cache size and pending sync count |

### `FeatherConfig`

| Property | Type | Default | Description |
|----------|------|---------|-------------|
| `baseURL` | `String` | required | Feather server URL |
| `deviceID` | `String` | required | Unique device identifier |
| `syncIntervalSeconds` | `Long` | `300` | Background sync interval |
| `offlineStorageLimit` | `Int` | `10,000` | Max cached features |
| `conflictStrategy` | `ConflictStrategy` | `SERVER_WINS` | Conflict resolution mode |
| `compressionEnabled` | `Boolean` | `true` | Enable response compression |

## Building from Source

```bash
cd sdk/kotlin
./gradlew build
./gradlew test
```

## License

Apache 2.0 — see [LICENSE](../../LICENSE).
