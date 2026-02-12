# Contributing to Feather

Thank you for your interest in contributing to Feather! We welcome contributions from the community.

## Quick Start

```bash
# Clone the repository
git clone https://github.com/feather-store/feather.git
cd feather

# One-command setup: checks prerequisites, installs tools, configures
# git hooks, builds, and runs core tests
make setup

# Or step by step:
# make doctor          # Check prerequisites
# make install-tools   # Install golangci-lint, goimports
# make build           # Build the binary
# make test-core       # Core tests only (~10s)

# Recommended test workflow (fastest → most thorough):
make test-core     # Core packages only (~10s, start here)
make test-quick    # All packages, short mode (~60s)
make check-quick   # Fast pre-commit: fmt + vet + lint + core tests (~20s)
make check         # Full suite: fmt + vet + lint + all tests with race detector

# See all available targets
make help
```

## How to Contribute

### Reporting Bugs

- Search [existing issues](https://github.com/feather-store/feather/issues) first
- Use the bug report template when creating a new issue
- Include reproduction steps, expected behavior, and environment details

### Suggesting Features

- Open a [feature request issue](https://github.com/feather-store/feather/issues/new)
- Describe the use case and proposed solution
- Be open to discussion about alternative approaches

### Submitting Code

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/your-feature`)
3. Make your changes following our coding conventions
4. Write or update tests as needed
5. Run `make check-quick` before committing (or `make check` for the full suite)
6. Commit using [conventional commits](https://www.conventionalcommits.org/) (e.g., `feat:`, `fix:`, `docs:`)
7. Push and open a pull request

## Development Guidelines

### How to Add a New API Endpoint

Feather uses a pluggable handler registry (see [ADR-0008](./docs/adr/0008-pluggable-http-handlers.md)).
Follow these four steps:

**Step 1 — Create your package** under the appropriate directory:

| Directory | Purpose |
|-----------|---------|
| `internal/extensions/` | Optional feature modules |
| `internal/integrations/` | External system connectors |
| `internal/platform/` | Cross-cutting infrastructure |
| `internal/tools/` | Developer and operational utilities |

```go
// internal/extensions/myfeature/engine.go
package myfeature

type EngineConfig struct { /* ... */ }
func DefaultEngineConfig() EngineConfig { /* ... */ }
type Engine struct { /* ... */ }
func NewEngine(cfg EngineConfig) *Engine { /* ... */ }
```

**Step 2 — Create a handler** in `internal/core/server/`:

```go
// internal/core/server/myfeature_handler.go
package server

type MyFeatureHandler struct {
    engine *myfeature.Engine
}

func NewMyFeatureHandler(engine *myfeature.Engine) *MyFeatureHandler {
    return &MyFeatureHandler{engine: engine}
}

func (h *MyFeatureHandler) RegisterRoutes(mux *http.ServeMux) {
    mux.HandleFunc("GET /v1/myfeature/items", h.handleList)
    mux.HandleFunc("POST /v1/myfeature/items", h.handleCreate)
}

func (h *MyFeatureHandler) handleList(w http.ResponseWriter, r *http.Request) {
    items := h.engine.List()
    writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
        "items": items,
        "count": len(items),
    })
}
```

**Step 3 — Register** in `internal/core/server/features.go` `init()`:

```go
registerHandler("myfeature", MaturityBeta, func(deps *handlerDeps) FeatureHandler {
    return NewMyFeatureHandler(myfeature.NewEngine(myfeature.DefaultEngineConfig()))
})
```

**Step 4 — Enable** in `cmd/feather/main.go` `EnabledFeatures` map:

```go
"myfeature": true,
```

Then update `docs/package-guide.md` with your new package. Run `make api-routes` to verify
your handler appears, and `make check-quick` before committing.

For detailed development guidelines, including:

- Project structure and architecture
- Coding conventions and Go idioms
- Error handling patterns
- Testing best practices
- Commit message format
- Pull request process

Please see our **[full contributing guide](./docs/contributing.md)** or the [website documentation](website/docs/contributing.md).

To preview documentation changes locally, run `make docs` (requires Node.js).

## Code of Conduct

We are committed to providing a welcoming and inclusive environment for all contributors. Please read and follow our [Code of Conduct](CODE_OF_CONDUCT.md).

## License

By contributing to Feather, you agree that your contributions will be licensed under the [Apache 2.0 License](LICENSE).

## Getting Help

- **GitHub Issues**: Bug reports and feature requests
- **GitHub Discussions**: Questions and ideas
- **Pull Request Comments**: Code review discussions

Thank you for helping make Feather better!
