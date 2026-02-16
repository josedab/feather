# Feather Swift/iOS SDK

Official Swift client for [Feather Feature Store](https://github.com/feather-store/feather) with offline-first architecture for Apple platforms.

## Installation

### Swift Package Manager

Add to your `Package.swift`:

```swift
dependencies: [
    .package(url: "https://github.com/feather-store/feather.git", from: "0.1.0")
]
```

Or in Xcode: **File → Add Package Dependencies** and enter the repository URL.

## Requirements

- Swift 5.9+
- iOS 15+ / macOS 12+ / watchOS 8+ / tvOS 15+

## Quick Start

```swift
import FeatherSDK

// Create a client with offline-first defaults
let config = FeatherConfig(baseURL: "http://localhost:8080", deviceID: "iphone-001")
let client = FeatherClient(config: config)

// Store a feature locally (queued for sync)
client.put(featureID: "user_score", entityKey: "user:123", value: AnyCodable(0.95))

// Retrieve from local cache (instant, works offline)
if let feature = client.get(featureID: "user_score", entityKey: "user:123") {
    print("Score: \(feature.value)")
}

// Start background sync with the server
client.startSync()

// Trigger an immediate sync
let result = client.sync()
print("Received: \(result.updatesReceived), Sent: \(result.updatesSent)")

// Stop background sync
client.stopSync()
```

## Features

### Offline-First Architecture

Features are served from an in-memory cache and synchronized with the server via background delta sync. Reads are instant and work without network connectivity.

```swift
// Always available, even offline
let cached = client.get(featureID: "click_count", entityKey: "user:456")

// Writes are queued and synced on next cycle
client.put(featureID: "click_count", entityKey: "user:456", value: AnyCodable(42))
```

### Background Sync

Automatic delta synchronization using GCD timers:

```swift
var config = FeatherConfig(baseURL: "https://feather.example.com", deviceID: "ios-001")
config.syncIntervalSeconds = 60  // sync every minute

let client = FeatherClient(config: config)
client.startSync()
```

### Conflict Resolution

Three strategies for handling conflicts between local and server state:

```swift
var config = FeatherConfig(baseURL: "http://localhost:8080", deviceID: "device-001")
config.conflictStrategy = .serverWins  // default
// Options: .serverWins, .clientWins, .lastWriteWins
```

### Cache Management

```swift
// Configure cache limits
var config = FeatherConfig(baseURL: "http://localhost:8080", deviceID: "device-001")
config.offlineStorageLimit = 10_000  // max cached features

// Get cache statistics
let stats = client.cacheStats()
print("Cached: \(stats["size"] ?? 0), Pending: \(stats["pending_sync"] ?? 0)")
```

## API Reference

### `FeatherClient`

| Method | Description |
|--------|-------------|
| `get(featureID:entityKey:)` | Retrieve a cached feature (offline-first) |
| `put(featureID:entityKey:value:)` | Store a feature locally, queue for sync |
| `sync()` | Perform an immediate sync with the server |
| `startSync()` | Start periodic background sync |
| `stopSync()` | Stop background sync |
| `cacheStats()` | Return cache size and pending sync count |

### `FeatherConfig`

| Property | Type | Default | Description |
|----------|------|---------|-------------|
| `baseURL` | `String` | required | Feather server URL |
| `deviceID` | `String` | required | Unique device identifier |
| `syncIntervalSeconds` | `TimeInterval` | `300` | Background sync interval |
| `offlineStorageLimit` | `Int` | `10,000` | Max cached features |
| `conflictStrategy` | `ConflictStrategy` | `.serverWins` | Conflict resolution mode |
| `compressionEnabled` | `Bool` | `true` | Enable response compression |

## Building from Source

```bash
cd sdk/swift
swift build
swift test
```

## License

Apache 2.0 — see [LICENSE](../../LICENSE).
