# ADR-0007: Modular Package Architecture

## Status

Accepted

## Context

Feather started as a simple feature store but evolved to include:

- Core serving (storage, caching, aggregation)
- Multiple ingestion paths (Kafka, HTTP)
- Multiple serving protocols (gRPC, HTTP, GraphQL)
- Observability (metrics, tracing, logging)
- Advanced features (vectors, drift detection, ML serving)
- Enterprise features (multi-tenancy, governance, RBAC)
- Integrations (Spark, Flink, warehouses)

Without careful organization, this would become an unmaintainable monolith. We needed an architecture that:

1. **Enables independent development**: Teams can work on features without conflicts
2. **Supports selective deployment**: Not everyone needs every feature
3. **Maintains clear boundaries**: Prevents spaghetti dependencies
4. **Scales with complexity**: Can add features without refactoring

## Decision

We organize the codebase into **50+ focused internal packages**, each owning a single capability:

### Package Categories

```
internal/
├── Core (always required)
│   ├── domain/       # Domain types, errors, interfaces
│   ├── storage/      # Hot/warm tier storage
│   ├── config/       # Configuration loading
│   └── server/       # HTTP/gRPC servers
│
├── Ingestion
│   ├── ingestion/    # Kafka, HTTP ingestion
│   └── streaming/    # Stream processing
│
├── Computation
│   ├── aggregation/  # Real-time aggregations
│   ├── transform/    # Feature transformations
│   └── composition/  # Feature composition DAG
│
├── Observability
│   ├── metrics/      # Prometheus metrics
│   ├── tracing/      # OpenTelemetry tracing
│   └── logging/      # Structured logging
│
├── Advanced Features
│   ├── vector/       # Similarity search
│   ├── drift/        # Drift detection
│   ├── ml/           # Model serving
│   ├── semantic/     # NL feature discovery
│   └── freshness/    # SLA monitoring
│
├── Enterprise
│   ├── auth/         # Authentication/authorization
│   ├── tenant/       # Multi-tenancy
│   ├── governance/   # Data governance
│   └── lineage/      # Data lineage
│
├── Integrations
│   ├── spark/        # Spark connector
│   ├── flink/        # Flink connector
│   └── warehouse/    # Data warehouse sync
│
└── Operations
    ├── cluster/      # Distributed coordination
    ├── migration/    # Schema migrations
    └── gitops/       # GitOps workflows
```

### Package Design Principles

1. **Single responsibility**: Each package does one thing well
2. **Explicit dependencies**: Import only what you need
3. **No circular imports**: Dependency graph is a DAG
4. **Domain at the center**: All packages depend on `domain/`, nothing depends on them
5. **Interface boundaries**: Packages expose interfaces, not implementations

### Dependency Flow

```
                    ┌─────────┐
                    │ domain  │ ← Core types, errors
                    └────┬────┘
                         │
        ┌────────────────┼────────────────┐
        ▼                ▼                ▼
   ┌─────────┐     ┌─────────┐     ┌─────────┐
   │ storage │     │ config  │     │ logging │
   └────┬────┘     └────┬────┘     └────┬────┘
        │               │               │
        └───────────────┼───────────────┘
                        ▼
                   ┌─────────┐
                   │ server  │ ← Composes everything
                   └─────────┘
```

## Consequences

### Positive

- **Clear ownership**: Each package has obvious purpose and scope
- **Independent testing**: Packages tested in isolation
- **Selective builds**: Can exclude unused packages
- **Parallel development**: Teams work without stepping on each other
- **Easy navigation**: New developers find code quickly
- **Refactoring safety**: Changes contained within package boundaries

### Negative

- **Package proliferation**: 50+ packages can feel overwhelming
- **Import management**: Must be vigilant about dependency direction
- **Boilerplate**: Each package needs its own types, errors, tests
- **Coordination cost**: Cross-package changes require more planning

### Neutral

- **No microservices**: Still a monolith, just well-organized
- **Single binary**: All packages compile into one executable

## Implementation Notes

### Enforcing Boundaries

We use Go's `internal/` convention:
- All application code lives under `internal/`
- External consumers cannot import internal packages
- Forces clean API surface in `api/` and `sdk/`

### Avoiding Circular Imports

Pattern: Use interfaces in `domain/` that implementations satisfy:

```go
// domain/interfaces.go
type FeatureStore interface {
    Get(entityID string, features []string) (map[string]FeatureValue, error)
    Put(entityID string, features map[string]FeatureValue) error
}

// storage/store.go
type Store struct { /* ... */ }
func (s *Store) Get(...) { /* ... */ }  // Implements domain.FeatureStore
func (s *Store) Put(...) { /* ... */ }

// server/http.go
type HTTPServer struct {
    store domain.FeatureStore  // Depends on interface, not implementation
}
```

### Feature Flags

The server uses configuration flags to enable/disable features:

```go
type HTTPServerConfig struct {
    EnableVectors     bool
    EnableDrift       bool
    EnableML          bool
    EnableGovernance  bool
    // ... 30+ flags
}
```

This allows deploying a minimal Feather with only core features, or a full-featured enterprise deployment.
