# ADR-0008: Pluggable HTTP Handler Registration

## Status

Accepted

## Context

The HTTP server exposes 50+ endpoints across different feature areas:

- Core: `/v1/features/*`, `/v1/schema/*`
- Vectors: `/v1/vectors/*`
- Drift: `/v1/drift/*`
- ML: `/v1/ml/*`
- Governance: `/v1/governance/*`
- Freshness: `/v1/freshness/*`
- And many more...

Not all deployments need all features:
- A simple feature cache doesn't need vector similarity
- A read-only replica doesn't need ingestion endpoints
- A lightweight deployment doesn't need governance overhead

We needed a way to:
1. **Selectively enable** endpoint groups
2. **Reduce attack surface** by not exposing unused APIs
3. **Minimize memory** by not initializing unused handlers
4. **Keep code organized** with handlers in separate files

## Decision

We implement a **pluggable handler registration system** with feature flags:

### Handler Organization

Each feature area has its own handler file:

```
internal/core/server/
├── http.go              # Core server, routing
├── handlers.go          # Core feature handlers
├── vector_handler.go    # Vector similarity handlers
├── drift_handler.go     # Drift detection handlers
├── ml_handler.go        # ML serving handlers
├── governance_handler.go
├── freshness_handler.go
├── lineage_handler.go
├── composition_handler.go
└── ... (20+ handler files)
```

### Registration Pattern

Each handler group registers itself conditionally:

```go
func (s *HTTPServer) registerRoutes() {
    // Core routes (always enabled)
    s.mux.HandleFunc("GET /v1/features", s.handleGetFeatures)
    s.mux.HandleFunc("POST /v1/features", s.handlePutFeatures)
    s.mux.HandleFunc("GET /health", s.handleHealth)

    // Conditional registration
    if s.config.EnableVectors {
        s.registerVectorRoutes()
    }
    if s.config.EnableDrift {
        s.registerDriftRoutes()
    }
    if s.config.EnableML {
        s.registerMLRoutes()
    }
    // ... etc
}
```

### Configuration

```yaml
server:
  http:
    port: 8080
    features:
      vectors: true
      drift: true
      ml: false           # Disabled
      governance: false   # Disabled
      freshness: true
      lineage: true
```

Or via environment variables:
```bash
FEATHER_HTTP_ENABLE_VECTORS=true
FEATHER_HTTP_ENABLE_DRIFT=true
FEATHER_HTTP_ENABLE_ML=false
```

### Handler Interface Pattern

Each handler file follows the same pattern:

```go
// vector_handler.go
func (s *HTTPServer) registerVectorRoutes() {
    s.mux.HandleFunc("GET /v1/vectors", s.handleListVectorIndexes)
    s.mux.HandleFunc("POST /v1/vectors", s.handleCreateVectorIndex)
    s.mux.HandleFunc("POST /v1/vectors/{index}/search", s.handleVectorSearch)
    // ...
}

func (s *HTTPServer) handleVectorSearch(w http.ResponseWriter, r *http.Request) {
    // Implementation
}
```

## Consequences

### Positive

- **Minimal deployments**: Enable only what you need
- **Reduced attack surface**: Unexposed endpoints can't be exploited
- **Clear organization**: Each feature's handlers in one file
- **Easy testing**: Handler groups tested independently
- **Documentation**: Config flags document available features
- **Startup speed**: Fewer handlers = faster initialization

### Negative

- **Flag proliferation**: 30+ enable flags to manage
- **Dependency tracking**: Must ensure handlers have their dependencies
- **Testing matrix**: Should test various flag combinations
- **Discovery complexity**: Users must know what flags exist

### Neutral

- **No hot reload**: Changing flags requires restart
- **Binary size unchanged**: All code compiled in, just not registered

## Alternatives Considered

| Alternative | Rejected Because |
|-------------|------------------|
| Separate binaries | Operational complexity; many artifacts to manage |
| Plugin system | Go plugins are complex and platform-specific |
| Microservices | Overkill for this use case; adds network hops |
| All-or-nothing | Doesn't serve varied deployment needs |

## Implementation Notes

### HTTPServerConfig

Key file: `internal/core/server/http.go`

```go
type HTTPServerConfig struct {
    // Core settings
    Port         int
    ReadTimeout  time.Duration
    WriteTimeout time.Duration

    // Feature flags
    EnableVectors     bool
    EnableDrift       bool
    EnableML          bool
    EnableGovernance  bool
    EnableFreshness   bool
    EnableLineage     bool
    EnableComposition bool
    EnableExperiment  bool
    EnableBackfill    bool
    EnableMigration   bool
    EnableCost        bool
    EnableQuality     bool
    EnableFederation  bool
    EnableWarehouse   bool
    EnableSpark       bool
    EnableFlink       bool
    // ... etc
}
```

### Dependency Injection

Handlers receive their dependencies at construction:

```go
type HTTPServer struct {
    config     HTTPServerConfig
    store      *storage.Store
    aggregator *aggregation.Engine

    // Optional components (nil if feature disabled)
    vectorStore  *vector.Store
    driftMonitor *drift.Monitor
    mlServing    *ml.ServingEngine
    governance   *governance.Manager
    // ...
}

func NewHTTPServer(cfg HTTPServerConfig, store *storage.Store, opts ...Option) *HTTPServer {
    s := &HTTPServer{config: cfg, store: store}
    for _, opt := range opts {
        opt(s)
    }
    return s
}

// Options for optional dependencies
func WithVectorStore(vs *vector.Store) Option {
    return func(s *HTTPServer) { s.vectorStore = vs }
}
```

### Route Listing

For debugging/documentation, list registered routes:

```go
func (s *HTTPServer) RegisteredRoutes() []string {
    // Return list of all registered paths
}
```

Exposed at `/debug/routes` in development mode.
