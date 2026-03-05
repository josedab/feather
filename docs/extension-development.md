# Extension Development Guide

This guide explains how to create new extensions for Feather using the pluggable handler system.

## Overview

Feather uses a handler registry pattern to support optional feature modules. Each extension implements the `FeatureHandler` interface and registers a factory function during `init()`. Extensions are enabled at startup via the `EnabledFeatures` configuration map.

For architectural context, see [ADR-0008: Pluggable HTTP Handlers](./adr/0008-pluggable-http-handlers.md).

## The FeatureHandler Interface

Every extension must implement this interface from `internal/core/server/features.go`:

```go
type FeatureHandler interface {
    RegisterRoutes(mux *http.ServeMux)
}
```

`RegisterRoutes` is called once during server startup. Use it to register your HTTP routes on the shared `ServeMux`.

## Step-by-Step: Creating an Extension

### 1. Create the Package

Create a new package under the appropriate directory:

| Directory | Purpose |
|-----------|---------|
| `internal/extensions/` | Feature modules (drift, marketplace, search, etc.) |
| `internal/integrations/` | External system connectors (dbt, Spark, Flink, etc.) |
| `internal/platform/` | Cross-cutting infrastructure (auth, cost, governance) |
| `internal/tools/` | Developer and operational utilities |

```bash
mkdir -p internal/extensions/myfeature
```

### 2. Implement the Core Logic

Create the domain logic in your package. Keep HTTP concerns out of this layer.

```go
// internal/extensions/myfeature/myfeature.go
package myfeature

type Service struct {
    // ...
}

func NewService(cfg Config) *Service {
    return &Service{/* ... */}
}

func (s *Service) DoSomething(input string) (string, error) {
    // Business logic here
    return "result", nil
}
```

### 3. Create the HTTP Handler

Create a handler struct in `internal/core/server/` that implements `FeatureHandler`:

```go
// internal/core/server/handler_myfeature.go
package server

import (
    "encoding/json"
    "net/http"

    "github.com/feather-store/feather/internal/extensions/myfeature"
)

type MyFeatureHandler struct {
    service     *myfeature.Service
    requireAuth func(http.Handler) http.Handler
}

func NewMyFeatureHandler(svc *myfeature.Service) *MyFeatureHandler {
    return &MyFeatureHandler{service: svc}
}

func (h *MyFeatureHandler) RegisterRoutes(mux *http.ServeMux) {
    mux.HandleFunc("GET /v1/myfeature/status", h.handleStatus)
    mux.HandleFunc("POST /v1/myfeature/action", h.handleAction)
}

func (h *MyFeatureHandler) handleStatus(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]any{
        "success": true,
        "data":    map[string]string{"status": "ok"},
    })
}

func (h *MyFeatureHandler) handleAction(w http.ResponseWriter, r *http.Request) {
    // Parse request, call service, write response
}
```

### 4. Register the Handler

Add a `registerHandler` call in the appropriate `features_*.go` file:

| File | Category |
|------|----------|
| `features_core.go` | Stable, production-ready handlers |
| `features_platform.go` | Beta platform and extension handlers |
| `features_experimental.go` | Experimental handlers |

```go
// In the init() function of the appropriate file
registerHandler("myfeature", MaturityBeta, func(deps *handlerDeps) FeatureHandler {
    return NewMyFeatureHandler(myfeature.NewService(myfeature.DefaultConfig()))
})
```

The `deps` parameter provides access to shared dependencies:

| Field | Type | Description |
|-------|------|-------------|
| `Ctx` | `context.Context` | Server lifecycle context |
| `Store` | `*storage.Store` | Feature store (hot + warm tiers) |
| `Aggregation` | `*aggregation.Engine` | Aggregation engine |
| `Schema` | `*storage.Registry` | Schema registry |
| `Metrics` | `*metrics.Metrics` | Prometheus metrics |
| `Config` | `HTTPServerConfig` | Server configuration |
| `AuthMiddleware` | `func(http.Handler) http.Handler` | Auth middleware wrapper |

Return `nil` from the factory if the handler cannot be created (e.g., missing dependencies).

### 5. Enable the Handler

Add your handler to the `EnabledFeatures` map in `cmd/feather/main.go`:

```go
EnabledFeatures: map[string]bool{
    // ... existing handlers ...
    "myfeature": true,
},
```

### 6. Update Documentation

1. Add an entry to `docs/package-guide.md` with the appropriate maturity level
2. Add a row to the handler reference table in `docs/api/extensions.md`
3. If the API is non-trivial, add detailed endpoint documentation

## Maturity Levels

Choose the appropriate maturity level when registering:

| Level | Constant | Meaning |
|-------|----------|---------|
| **Stable** | `MaturityStable` | Production-ready, well-tested, breaking changes follow semver |
| **Beta** | `MaturityBeta` | Functional and tested, API may change between minor releases |
| **Experimental** | `MaturityExperimental` | Working implementation, may be incomplete or change significantly |

## Conventions

- **Route prefix**: Use `/v1/<feature>/` as the route prefix
- **JSON envelope**: Return `{"success": true, "data": ...}` for successful responses
- **Error format**: Return `{"success": false, "error": "message"}` with appropriate HTTP status
- **Auth**: Store `requireAuth` middleware and apply it in `RegisterRoutes` if your endpoints need authentication
- **Naming**: Handler files follow the pattern `handler_<feature>.go` in the server package
- **Testing**: Add `*_test.go` files alongside your implementation

## Verifying Your Extension

```bash
# Check that your handler is registered
make api-routes

# Run tests
make test-quick

# Build and run
make run-dev
```

## Examples

Look at existing handlers for reference:

- **Simple handler**: `internal/core/server/handler_impact.go` (minimal, no dependencies)
- **Handler with auth**: `internal/core/server/handler_groups.go` (uses `requireAuth` middleware)
- **Handler with service**: `internal/core/server/handler_drift.go` (wraps a service from `internal/extensions/drift`)
- **Full extension**: `internal/extensions/semantic/` (complete package with search, embeddings, and discovery)
