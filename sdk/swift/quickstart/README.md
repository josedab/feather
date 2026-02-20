# Feather Swift Quickstart

Get started with Feather on Apple platforms in 30 seconds.

## Prerequisites

- Swift 5.9+
- Xcode 15+ (or Swift toolchain on Linux)
- Docker (for running Feather server)

## Step 1: Start Feather

```bash
# From source (no Docker needed)
cd /path/to/feather && make run-dev

# Or with Docker
docker run -d --name feather -p 8080:8080 ghcr.io/feather-store/feather:latest
```

## Step 2: Add the SDK

Add to your `Package.swift`:

```swift
dependencies: [
    .package(url: "https://github.com/feather-store/feather.git", from: "0.1.0")
]
```

Or run the quickstart directly:

```bash
cd sdk/swift/quickstart
swift run
```

## Step 3: Run the Quickstart

```bash
swift run FeatherQuickstart
```

## What This Does

1. Connects to Feather with offline-first defaults
2. Stores features locally (queued for sync)
3. Retrieves features from the local cache
4. Demonstrates background sync with the server

## Next Steps

- Check out the [full documentation](https://feather-store.dev/docs)
- Learn about [offline-first architecture](https://feather-store.dev/docs/sdks/swift)
- Explore [conflict resolution strategies](https://feather-store.dev/docs/sdks/swift#conflict-resolution)
