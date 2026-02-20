# Feather Client SDKs

Official client SDKs for the [Feather Feature Store](https://github.com/feather-store/feather).

All SDKs connect to a running Feather server. Start one with `make run-dev` from the repository root.

## Available SDKs

| SDK | Language | Install | Status | README |
|-----|----------|---------|--------|--------|
| [Go](./go/feather/) | Go 1.21+ | `go get github.com/feather-store/feather/sdk/go/feather` | ✅ Stable | [README](./go/feather/README.md) |
| [Python](./python/) | Python 3.9+ | `pip install -e sdk/python/` | ✅ Stable | [README](./python/README.md) |
| [TypeScript](./typescript/) | Node.js 18+ | `cd sdk/typescript && npm install` | ✅ Stable | [README](./typescript/README.md) |
| [Java](./java/) | Java 11+ | Maven/Gradle from `sdk/java/` | ✅ Stable | [README](./java/README.md) |
| [Rust](./rust/) | Rust 1.75+ | `cargo add` from `sdk/rust/` | ✅ Stable | [README](./rust/README.md) |
| [Kotlin](./kotlin/) | Kotlin 1.9+ | Gradle from `sdk/kotlin/` | Beta | [README](./kotlin/README.md) |
| [Swift](./swift/) | Swift 5.9+ | SPM from `sdk/swift/` | Beta | [README](./swift/README.md) |

## Quick Start

```bash
# Start the Feather server
make run-dev

# Then use any SDK — example with Python:
pip install -e sdk/python/
python examples/ml-pipeline.py
```

## Common Features

All SDKs support:

- **Feature CRUD** — get, put, delete features by entity key
- **Batch operations** — retrieve features for multiple entities in one call
- **Point-in-time queries** — historical feature values for training data
- **Vector search** — similarity search with HNSW indexes
- **Schema management** — create and list feature groups
- **Error handling** — typed errors for not-found, rate-limited, and server errors

## Documentation

- [SDK Guide](../docs/sdk-guide.md) — detailed usage across all SDKs
- [API Reference](../docs/api-reference.md) — HTTP and gRPC API specification
- [Examples](../examples/) — runnable examples in multiple languages
