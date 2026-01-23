---
sidebar_position: 14
title: "ADR-0013: Go as Implementation Language"
description: Choice of Go for Feather's primary implementation language.
---

# ADR-0013: Go as Primary Implementation Language

## Status

Accepted

## Context

When starting Feather, we needed to choose a primary implementation language for a feature store with these requirements:

1. **Sub-millisecond latency**: Feature serving must not become an ML inference bottleneck
2. **High concurrency**: Handle thousands of simultaneous feature requests
3. **Operational simplicity**: Easy to deploy, run, and debug in production
4. **Memory control**: Predictable memory usage for cache and storage tiers
5. **Cross-platform**: Run on Linux, macOS, and in containers

We evaluated several languages commonly used for infrastructure software:

| Language | Strengths | Concerns |
|----------|-----------|----------|
| **Go** | Fast compilation, great concurrency, single binary | GC pauses, no generics (at time of decision) |
| **Rust** | Zero-cost abstractions, no GC | Steep learning curve, slower iteration |
| **Java/Kotlin** | Mature ecosystem, JVM optimizations | JVM tuning complexity, memory overhead |
| **C++** | Maximum performance | Memory safety, build complexity |
| **Python** | ML ecosystem integration | Too slow for serving path |

## Decision

We chose **Go** as the primary implementation language for Feather.

### Key Factors

**1. Concurrency Model**
Go's goroutines and channels provide lightweight concurrency primitives that map naturally to our architecture:
- Each gRPC/HTTP request handled in its own goroutine
- Channel-based coordination for background tasks (eviction, sync)
- `sync.RWMutex` and atomics for fine-grained locking

**2. Single Binary Deployment**
Go produces statically-linked binaries with no runtime dependencies:
```bash
# Build
go build -o feather ./cmd/feather

# Deploy (that's it)
./feather -config config.yaml
```
No JVM installation, no Python environment, no shared libraries to manage.

**3. Predictable Performance**
While Go has garbage collection, its GC is:
- Concurrent and incremental (sub-millisecond pauses)
- Tunable via `GOGC` environment variable
- Predictable enough for P99 latency targets

Combined with object pooling (ADR-0016), we achieve consistent sub-millisecond hot tier access.

**4. Standard Library Quality**
Go's standard library provides production-ready implementations for:
- HTTP server (`net/http`)
- JSON encoding (`encoding/json`)
- Context propagation (`context`)
- Structured logging (`log/slog` in Go 1.21+)
- Cryptography (`crypto/tls`)

Fewer external dependencies means fewer security vulnerabilities and upgrade headaches.

**5. Ecosystem Fit**
Key dependencies are first-class Go citizens:
- BadgerDB (pure Go embedded database)
- Prometheus client (native instrumentation)
- gRPC (official Google-maintained bindings)
- OpenTelemetry (actively maintained Go SDK)

### Code Style Implications

Go's idioms shape our codebase:

```go
// Explicit error handling (no exceptions)
value, err := store.Get(ctx, entityID, features)
if err != nil {
    return fmt.Errorf("fetching features: %w", err)
}

// Interface-based abstractions (defined where used)
type FeatureStore interface {
    Get(ctx context.Context, entityID string, features []string) (map[string]FeatureValue, error)
    Put(ctx context.Context, entityID string, features map[string]FeatureValue) error
}

// Context for cancellation and deadlines
ctx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
defer cancel()
```

## Consequences

### Positive

- **Fast iteration**: Compile times measured in seconds, not minutes
- **Easy onboarding**: Go's simplicity means new developers contribute quickly
- **Operational simplicity**: Single binary with no runtime dependencies
- **Strong tooling**: Built-in formatting (`gofmt`), testing, profiling, race detection
- **Memory safety**: No buffer overflows, null pointer dereferences caught at runtime
- **Cross-compilation**: Build Linux binaries on macOS with `GOOS=linux go build`

### Negative

- **GC pauses**: While rare, GC can cause latency spikes (mitigated by object pooling)
- **Verbosity**: Explicit error handling adds boilerplate
- **Limited generics**: Go 1.18+ added generics, but ecosystem adoption is gradual
- **Binary size**: ~15-20MB for Feather binary (acceptable for server software)

### Neutral

- **No inheritance**: Composition over inheritance is enforced; different from OOP languages
- **Package structure**: `internal/` convention for private packages is Go-specific
- **Dependency management**: Go modules work well but differ from npm/pip/cargo

## Alternatives Reconsidered

### Rust
Rust would provide:
- Guaranteed memory safety without GC
- Potentially lower tail latencies
- More powerful type system

We chose Go because:
- Faster development velocity (critical for early-stage project)
- Larger pool of engineers familiar with Go
- BadgerDB (our storage engine) is pure Go; Rust would require bindings

### Java/Kotlin with GraalVM
JVM languages could provide:
- Mature ecosystem for distributed systems
- GraalVM native-image for AOT compilation

We chose Go because:
- Native-image compilation is complex and has limitations
- JVM tuning expertise is a barrier to adoption
- Memory overhead for JVM compared to Go runtime

## Implementation Notes

### Build Configuration

Key file: `Makefile`

```makefile
BUILD_FLAGS := -ldflags "-s -w -X main.version=$(VERSION)"

build:
    go build $(BUILD_FLAGS) -o bin/feather ./cmd/feather

test:
    go test -race -cover ./...
```

### Performance Tuning

Runtime configuration for production:
```bash
# Reduce GC frequency (default 100)
GOGC=200

# Limit OS threads (usually leave at default)
GOMAXPROCS=0

# Enable mutex profiling
GODEBUG=schedtrace=1000
```

### Required Go Version

Feather requires Go 1.21+ for:
- `log/slog` structured logging
- `maps` and `slices` packages
- Improved generics support
