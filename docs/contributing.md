# Contributing to Feather

Thank you for your interest in contributing to Feather! This guide will help you get started with development and explain our coding standards and processes.

## Table of Contents

- [Getting Started](#getting-started)
- [Development Setup](#development-setup)
- [Project Structure](#project-structure)
- [Coding Standards](#coding-standards)
- [Testing](#testing)
- [Pull Request Process](#pull-request-process)
- [Code Review Guidelines](#code-review-guidelines)
- [Release Process](#release-process)

---

## Getting Started

### Prerequisites

- **Go 1.22+**: [Download Go](https://golang.org/dl/)
- **Make**: Build automation
- **Docker**: For integration tests (optional)
- **golangci-lint**: For code linting

```bash
# Install golangci-lint
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Verify installation
golangci-lint --version
```

### Fork and Clone

```bash
# Fork the repository on GitHub, then:
git clone https://github.com/YOUR_USERNAME/feather.git
cd feather

# Add upstream remote
git remote add upstream https://github.com/feather-store/feather.git

# Verify remotes
git remote -v
```

---

## Development Setup

### Building

```bash
# Build the binary
make build

# The binary is created at ./bin/feather
./bin/feather --help
```

### Running Locally

```bash
# Run with default configuration
make run

# Run with a config file
make run-config

# Or directly
./bin/feather -config configs/feather.yaml
```

### Running Tests

```bash
# Run all tests with race detector
make test

# Run short tests only (skip integration tests)
make test-short

# Run with coverage report
make test-coverage

# Run benchmarks
make bench
```

### Linting

```bash
# Run all linters
make lint

# Format code
make fmt

# Run all checks (fmt, vet, lint, test)
make check
```

---

## Project Structure

```
feather/
├── cmd/feather/          # Application entrypoint
│   └── main.go           # Server initialization, graceful shutdown
│
├── internal/             # Private application packages
│   ├── aggregation/      # Real-time aggregation engine
│   │   ├── engine.go     # Window management and computation
│   │   └── ring_buffer.go
│   │
│   ├── config/           # Configuration loading and validation
│   │   └── config.go     # YAML and environment variable parsing
│   │
│   ├── domain/           # Core domain types (no external dependencies)
│   │   ├── types.go      # FeatureValue, FeatureGroup, etc.
│   │   └── errors.go     # Sentinel errors and error codes
│   │
│   ├── ingestion/        # Data ingestion pipelines
│   │   ├── kafka.go      # Kafka consumer with circuit breaker
│   │   ├── http.go       # HTTP push endpoint
│   │   └── batch.go      # CSV/JSON file import
│   │
│   ├── server/           # HTTP and gRPC servers
│   │   ├── http.go       # REST API handlers
│   │   ├── grpc.go       # gRPC service implementation
│   │   └── health.go     # Health check endpoints
│   │
│   ├── storage/          # Tiered storage engine
│   │   ├── store.go      # Unified store interface
│   │   ├── hot.go        # In-memory LRU cache (256 shards)
│   │   ├── warm.go       # BadgerDB persistent storage
│   │   └── registry.go   # Schema registry
│   │
│   ├── metrics/          # Prometheus metrics
│   ├── tracing/          # OpenTelemetry integration
│   ├── logging/          # Structured logging (slog)
│   └── vector/           # Vector similarity search (HNSW)
│
├── api/                  # Protocol buffer definitions
│   └── proto/v1/         # gRPC service definitions
│
├── sdk/                  # Client SDKs
│   ├── go/               # Go client library
│   └── python/           # Python client library
│
├── test/                 # Integration and benchmark tests
├── configs/              # Example configuration files
├── deploy/               # Kubernetes manifests, Helm charts
└── docs/                 # Documentation
```

---

## Coding Standards

### Go Idioms

Follow the [Effective Go](https://golang.org/doc/effective_go) guidelines and these project-specific conventions:

#### Context Propagation

Always accept `context.Context` as the first parameter for functions that may block:

```go
// Good
func (s *Store) Get(ctx context.Context, entityKey string) (*FeatureValue, error)

// Bad
func (s *Store) Get(entityKey string) (*FeatureValue, error)
```

#### Error Handling

Return errors, don't panic. Wrap errors with context:

```go
// Good
if err != nil {
    return fmt.Errorf("getting feature %s: %w", name, err)
}

// Bad
if err != nil {
    panic(err)
}
```

Use sentinel errors for expected conditions:

```go
// Define in domain/errors.go
var ErrEntityNotFound = errors.New("entity not found")

// Check with errors.Is
if errors.Is(err, domain.ErrEntityNotFound) {
    // Handle not found
}
```

#### Interface Design

Define interfaces where they're used, not where implemented:

```go
// Good - interface defined in the consumer package
package server

type FeatureStore interface {
    Get(ctx context.Context, key string) (*domain.FeatureValue, error)
}

// Bad - interface defined with implementation
package storage

type StoreInterface interface { ... }
type Store struct { ... }
```

### Naming Conventions

| Type | Convention | Example |
|------|------------|---------|
| Package | lowercase, singular | `storage`, `domain` |
| Exported types | PascalCase | `FeatureValue`, `HotTier` |
| Unexported types | camelCase | `entityData`, `shard` |
| Constants | PascalCase or camelCase | `MaxShards`, `defaultTimeout` |
| Avoid stutter | Don't repeat package name | `storage.Store` not `storage.StorageStore` |

### Concurrency Patterns

#### Lock Ordering

Always acquire locks in consistent order to prevent deadlocks:

```go
// Document lock hierarchy in package comments
// Lock order: store.mu → shard.mu → entity.mu
```

#### Atomic Operations

Use atomic operations for metrics and counters:

```go
// Good - lock-free counter
atomic.AddInt64(&h.metrics.Hits, 1)

// Bad - mutex for simple counter
h.mu.Lock()
h.metrics.Hits++
h.mu.Unlock()
```

### Documentation

Write GoDoc comments for all exported types and functions:

```go
// Store provides unified access to Feather's tiered storage.
//
// Store coordinates reads and writes between the hot (memory) and
// warm (disk) tiers, implementing read-through caching and async
// background writes for optimal latency.
//
// The store is safe for concurrent use from multiple goroutines.
type Store struct {
    // ...
}

// Get retrieves features for an entity.
//
// It first checks the hot tier, falling back to warm tier on cache miss.
// Returns domain.ErrEntityNotFound if the entity doesn't exist in either tier.
func (s *Store) Get(ctx context.Context, entityKey string, features []string) (map[string]*domain.FeatureValue, error) {
    // ...
}
```

---

## Testing

### Test File Organization

- Place tests next to the code: `foo.go` → `foo_test.go`
- Integration tests go in the `test/` directory
- Benchmark tests use the `_test.go` suffix with `Benchmark` prefix

### Writing Tests

Use table-driven tests for multiple cases:

```go
func TestStore_Get(t *testing.T) {
    tests := []struct {
        name      string
        entityKey string
        features  []string
        setup     func(*Store)
        want      map[string]*domain.FeatureValue
        wantErr   error
    }{
        {
            name:      "returns features from hot tier",
            entityKey: "user:123",
            features:  []string{"click_count"},
            setup: func(s *Store) {
                s.Put(context.Background(), "user:123", map[string]*domain.FeatureValue{
                    "click_count": {Value: 42, Timestamp: time.Now().UnixNano()},
                })
            },
            want: map[string]*domain.FeatureValue{
                "click_count": {Value: 42},
            },
        },
        {
            name:      "returns error for missing entity",
            entityKey: "user:999",
            features:  []string{"click_count"},
            wantErr:   domain.ErrEntityNotFound,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            store := newTestStore(t)
            if tt.setup != nil {
                tt.setup(store)
            }

            got, err := store.Get(context.Background(), tt.entityKey, tt.features)

            if !errors.Is(err, tt.wantErr) {
                t.Errorf("Get() error = %v, wantErr %v", err, tt.wantErr)
                return
            }
            // Compare got and tt.want...
        })
    }
}
```

### Running Specific Tests

```bash
# Run tests matching a pattern
go test -v -run TestStore ./internal/storage/...

# Run tests in a specific package
go test -v ./internal/aggregation/...

# Run with race detector (always use in CI)
go test -race ./...
```

### Benchmarks

Write benchmarks for performance-critical code:

```go
func BenchmarkHotTier_Get(b *testing.B) {
    h := NewHotTier(1024 * 1024 * 1024) // 1GB

    // Setup test data
    for i := 0; i < 10000; i++ {
        h.Put(fmt.Sprintf("entity:%d", i), map[string]*domain.FeatureValue{
            "feature": {Value: i, Timestamp: time.Now().UnixNano()},
        })
    }

    b.ResetTimer()
    b.RunParallel(func(pb *testing.PB) {
        i := 0
        for pb.Next() {
            h.Get(fmt.Sprintf("entity:%d", i%10000), []string{"feature"})
            i++
        }
    })
}
```

Run benchmarks:

```bash
# Run all benchmarks
make bench

# Run specific benchmark
go test -bench=BenchmarkHotTier -benchmem ./internal/storage/...

# Compare benchmarks
go install golang.org/x/perf/cmd/benchstat@latest
go test -bench=. -count=10 ./... > old.txt
# Make changes
go test -bench=. -count=10 ./... > new.txt
benchstat old.txt new.txt
```

---

## Pull Request Process

### 1. Create a Branch

```bash
# Sync with upstream
git fetch upstream
git checkout main
git merge upstream/main

# Create feature branch
git checkout -b feature/my-feature
# Or for bug fixes
git checkout -b fix/issue-123
```

### 2. Make Changes

- Write code following our [coding standards](#coding-standards)
- Add tests for new functionality
- Update documentation if needed
- Keep commits atomic and well-described

### 3. Commit Messages

Use conventional commit format:

```
type(scope): short description

Longer description if needed.

Fixes #123
```

**Types:**
- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation only
- `style`: Formatting, no code change
- `refactor`: Code change that neither fixes nor adds
- `perf`: Performance improvement
- `test`: Adding tests
- `chore`: Build process, dependencies

**Examples:**
```
feat(storage): add point-in-time query support

Implements GetAsOf() method for historical feature retrieval.
Uses reverse iterator on BadgerDB historical keys.

Closes #45
```

```
fix(aggregation): handle empty window in avg computation

Returns 0 instead of NaN when window has no data.

Fixes #89
```

### 4. Run Checks

```bash
# Run all checks before pushing
make check

# This runs: fmt, vet, lint, test
```

### 5. Push and Create PR

```bash
git push origin feature/my-feature
```

Then create a pull request on GitHub with:

- Clear title describing the change
- Description of what and why
- Link to any related issues
- Screenshots for UI changes (if applicable)

### 6. Address Review Feedback

- Respond to all comments
- Push additional commits to address feedback
- Request re-review when ready

---

## Code Review Guidelines

### For Authors

- Keep PRs focused and reasonably sized (<500 lines ideal)
- Self-review before requesting review
- Respond to feedback constructively
- Don't take feedback personally

### For Reviewers

- Review promptly (within 1 business day)
- Be constructive and specific
- Approve when satisfied, don't nitpick endlessly
- Use suggestions for minor changes

**Review Checklist:**

- [ ] Code follows project conventions
- [ ] Tests are included and pass
- [ ] Documentation is updated
- [ ] No security vulnerabilities introduced
- [ ] Performance impact is acceptable
- [ ] Error handling is appropriate

---

## Release Process

### Versioning

We follow [Semantic Versioning](https://semver.org/):

- **MAJOR**: Breaking API changes
- **MINOR**: New features, backward compatible
- **PATCH**: Bug fixes, backward compatible

### Creating a Release

1. Update CHANGELOG.md
2. Update version in code if needed
3. Create and push a tag:

```bash
git tag -a v1.2.3 -m "Release v1.2.3"
git push origin v1.2.3
```

4. GitHub Actions will build and publish releases

---

## Getting Help

- **Questions**: Open a [Discussion](https://github.com/feather-store/feather/discussions)
- **Bugs**: File an [Issue](https://github.com/feather-store/feather/issues)
- **Security**: Email security@feather.io (do not file public issues)

---

## License

By contributing to Feather, you agree that your contributions will be licensed under the MIT License.

---

Thank you for contributing to Feather!
