---
sidebar_position: 11
title: Contributing
description: How to contribute to the Feather feature store project.
---

# Contributing to Feather

Thank you for your interest in contributing to Feather! This guide will help you get started.

## Ways to Contribute

### Code Contributions

- **Bug fixes**: Fix issues from the [issue tracker](https://github.com/feather-store/feather/issues)
- **Features**: Implement features from the [roadmap](https://github.com/feather-store/feather/projects)
- **Performance**: Optimize hot paths and reduce resource usage
- **Tests**: Improve test coverage and add edge cases

### Non-Code Contributions

- **Documentation**: Improve guides, fix typos, add examples
- **Bug reports**: Report issues with clear reproduction steps
- **Feature requests**: Suggest improvements with use cases
- **Community support**: Help others in discussions

## Development Setup

### Prerequisites

- Go 1.21 or later
- Make
- Docker (for integration tests)

### Clone and Build

```bash
# Clone the repository
git clone https://github.com/feather-store/feather.git
cd feather

# Install dependencies
go mod download

# Build
make build

# Run tests
make test

# Run linter
make lint
```

### Project Structure

```
feather/
├── cmd/feather/          # Application entrypoint
├── internal/             # Private packages
│   ├── aggregation/      # Real-time aggregations
│   ├── config/           # Configuration loading
│   ├── domain/           # Domain types and errors
│   ├── export/           # Data export
│   ├── ingestion/        # Kafka and HTTP ingestion
│   ├── logging/          # Structured logging
│   ├── metrics/          # Prometheus metrics
│   ├── server/           # HTTP and gRPC servers
│   ├── storage/          # Tiered storage
│   └── tracing/          # OpenTelemetry
├── sdk/                  # Client SDKs
│   ├── go/               # Go SDK
│   └── python/           # Python SDK
├── test/                 # Integration tests
├── configs/              # Example configurations
├── api/                  # Proto definitions
└── docs/                 # Documentation
```

### Running Locally

```bash
# Run with default settings
make run

# Run with custom config
./bin/feather -config configs/dev.yaml

# Run with debug logging
FEATHER_LOG_LEVEL=debug make run
```

### Running Tests

```bash
# All tests
make test

# Short tests only
make test-short

# With race detector
make test-race

# With coverage
make test-coverage

# Integration tests (requires Docker)
make test-integration

# Benchmarks
make bench
```

## Development Workflow

### 1. Find an Issue

- Browse [open issues](https://github.com/feather-store/feather/issues)
- Look for `good first issue` labels for newcomers
- Comment on the issue to claim it

### 2. Create a Branch

```bash
# Create feature branch from main
git checkout main
git pull origin main
git checkout -b feature/your-feature-name

# Or for bug fixes
git checkout -b fix/issue-description
```

### 3. Make Changes

Follow the coding conventions below and ensure:
- All tests pass
- Linter passes
- New code has tests
- Documentation is updated

### 4. Commit Changes

```bash
# Stage changes
git add .

# Commit with descriptive message
git commit -m "feat: add vector similarity search

- Implement HNSW algorithm for ANN
- Add REST endpoints for vector operations
- Include benchmarks showing sub-ms latency

Closes #123"
```

### 5. Submit Pull Request

```bash
# Push to your fork
git push origin feature/your-feature-name
```

Then open a PR on GitHub with:
- Clear title describing the change
- Description of what and why
- Link to related issue
- Screenshots/benchmarks if applicable

## Coding Conventions

### Go Style

Follow standard Go conventions and the project patterns:

```go
// Good: use context for cancellation
func (s *Store) Get(ctx context.Context, key string) ([]byte, error) {
    select {
    case <-ctx.Done():
        return nil, ctx.Err()
    default:
    }
    // ...
}

// Good: wrap errors with context
if err != nil {
    return fmt.Errorf("reading from warm tier: %w", err)
}

// Good: define interfaces where used
type Reader interface {
    Read(ctx context.Context, key string) ([]byte, error)
}
```

### Error Handling

```go
// Use domain errors
if !found {
    return nil, domain.ErrNotFound
}

// Check errors with type assertions
if domain.IsNotFound(err) {
    // Handle not found
}

// Wrap with context
return fmt.Errorf("getting feature %s: %w", name, err)
```

### Testing

```go
// Table-driven tests
func TestStore_Get(t *testing.T) {
    tests := []struct {
        name    string
        key     string
        want    []byte
        wantErr error
    }{
        {
            name: "existing key",
            key:  "user:123",
            want: []byte("value"),
        },
        {
            name:    "missing key",
            key:     "user:999",
            wantErr: domain.ErrNotFound,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := store.Get(ctx, tt.key)
            if !errors.Is(err, tt.wantErr) {
                t.Errorf("Get() error = %v, want %v", err, tt.wantErr)
            }
            if !bytes.Equal(got, tt.want) {
                t.Errorf("Get() = %v, want %v", got, tt.want)
            }
        })
    }
}
```

### Documentation

```go
// Package storage implements tiered storage for features.
//
// The storage system uses a two-tier architecture:
//   - Hot tier: In-memory LRU cache for low-latency serving
//   - Warm tier: BadgerDB for persistence and historical data
package storage

// Store provides unified access to feature storage.
// It automatically handles promotion from warm to hot tier
// on cache misses.
type Store struct {
    // ...
}
```

## Commit Messages

Follow [Conventional Commits](https://www.conventionalcommits.org/):

```
<type>(<scope>): <description>

[optional body]

[optional footer]
```

### Types

| Type | Description |
|------|-------------|
| `feat` | New feature |
| `fix` | Bug fix |
| `docs` | Documentation |
| `style` | Formatting (no code change) |
| `refactor` | Code restructuring |
| `perf` | Performance improvement |
| `test` | Adding tests |
| `chore` | Build, CI, dependencies |

### Examples

```
feat(storage): add TTL support for hot tier entries

fix(server): handle nil pointer in batch endpoint

docs(readme): add installation instructions

perf(cache): reduce lock contention with sharding

test(storage): add integration tests for warm tier
```

## Pull Request Guidelines

### PR Checklist

- [ ] Tests pass (`make test`)
- [ ] Linter passes (`make lint`)
- [ ] New code has tests
- [ ] Documentation updated
- [ ] Commit messages follow conventions
- [ ] PR description explains changes

### PR Size

- Keep PRs focused and reasonably sized
- Large changes should be split into smaller PRs
- Each PR should be independently reviewable

### Review Process

1. Automated checks run on PR
2. Maintainer reviews code
3. Address feedback with new commits
4. Once approved, maintainer merges

## Architecture Decision Records

For significant changes, create an ADR:

```bash
# Create new ADR
cp docs/adr/template.md docs/adr/NNNN-your-decision.md
```

ADR template:

```markdown
# ADR-NNNN: Title

## Status
Proposed

## Context
What is the issue we're addressing?

## Decision
What is our approach?

## Consequences
What are the trade-offs?
```

## Getting Help

### Communication Channels

- **GitHub Issues**: Bug reports and feature requests
- **GitHub Discussions**: Questions and ideas
- **Pull Request Comments**: Code review discussions

### Mentorship

New contributors can request mentorship on issues labeled `good first issue`. Maintainers will provide guidance through the contribution process.

## Recognition

Contributors are recognized in:
- Release notes
- Contributors list in README
- Annual contributor highlights

## Code of Conduct

All contributors must follow our [Code of Conduct](https://github.com/feather-store/feather/blob/main/CODE_OF_CONDUCT.md). We are committed to providing a welcoming and inclusive environment.

## License

By contributing, you agree that your contributions will be licensed under the Apache 2.0 License.

## Related Documentation

- [Architecture](/docs/concepts/architecture) - System design
- [ADR Index](/docs/adr) - Architectural decisions
- [API Reference](./api-reference) - API documentation
