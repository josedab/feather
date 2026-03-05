# ADR-0009: Structured Logging with slog

## Status

Accepted

## Context

Production systems require structured logging for:

1. **Observability**: Correlate logs with traces and metrics
2. **Searchability**: Filter logs by entity, request ID, error type
3. **Aggregation**: Count errors, measure latencies from log data
4. **Debugging**: Trace request flow across components

Traditional `fmt.Printf` or `log.Println` produce unstructured text that's difficult to parse and query. We needed structured logging that:

- Outputs machine-readable format (JSON for production)
- Supports human-readable format (text for development)
- Has minimal performance overhead
- Propagates context (request IDs, entity keys)
- Integrates with Go's ecosystem

## Decision

We adopt **Go's stdlib `log/slog`** (introduced in Go 1.21) as our logging framework.

### Key Characteristics

- **Standard library**: No external dependency
- **Structured by design**: Key-value pairs, not format strings
- **Handler abstraction**: Swap JSON/text without code changes
- **Context integration**: `slog.InfoContext(ctx, ...)` propagates values
- **Level filtering**: Debug, Info, Warn, Error
- **Performance**: Zero-allocation for disabled levels

### Logger Configuration

```go
func NewLogger(cfg LogConfig) *slog.Logger {
    var handler slog.Handler

    opts := &slog.HandlerOptions{
        Level: parseLevel(cfg.Level),
        AddSource: cfg.Level == "debug",  // Include source location in debug
    }

    if cfg.Format == "json" {
        handler = slog.NewJSONHandler(os.Stdout, opts)
    } else {
        handler = slog.NewTextHandler(os.Stdout, opts)
    }

    return slog.New(handler)
}
```

### Usage Pattern

```go
// Basic logging
logger.Info("feature retrieved",
    "entity_id", entityID,
    "feature_count", len(features),
    "latency_ms", latency.Milliseconds(),
)

// With context (includes request_id if set)
logger.InfoContext(ctx, "processing request",
    "method", r.Method,
    "path", r.URL.Path,
)

// Errors with stack context
logger.Error("storage failure",
    "error", err,
    "entity_id", entityID,
    "operation", "get",
)
```

### Output Formats

**JSON (production)**:
```json
{"time":"2024-01-15T10:30:00Z","level":"INFO","msg":"feature retrieved","entity_id":"user:123","feature_count":5,"latency_ms":2}
```

**Text (development)**:
```
2024/01/15 10:30:00 INFO feature retrieved entity_id=user:123 feature_count=5 latency_ms=2
```

## Consequences

### Positive

- **Zero dependencies**: Part of Go stdlib; no vendor lock-in
- **Future-proof**: Official Go solution, will be maintained
- **Performance**: Designed for high-throughput systems
- **Consistent API**: Same interface across all packages
- **Context propagation**: Request IDs flow through automatically
- **Level filtering**: Disable debug logs in production with zero cost

### Negative

- **Go 1.21+ required**: Not available in older Go versions
- **Learning curve**: Different mental model than printf-style logging
- **Verbosity**: More characters to type than `log.Println`
- **Migration effort**: Existing code needed updates

### Neutral

- **No automatic caller info**: Must opt-in via `AddSource`
- **No log rotation**: Use external tools (logrotate, container runtime)

## Alternatives Considered

| Alternative | Rejected Because |
|-------------|------------------|
| `log` (stdlib) | No structure; hard to parse |
| `logrus` | External dependency; maintenance concerns |
| `zap` | External dependency; more complex API |
| `zerolog` | External dependency; less ecosystem integration |

**Note**: We chose slog specifically because it's now the standard, ensuring long-term support and ecosystem compatibility.

## Implementation Notes

### Logger Package

Key file: `internal/core/logging/logger.go`

```go
package logging

import (
    "context"
    "log/slog"
    "os"
)

type Config struct {
    Level  string `yaml:"level"`   // debug, info, warn, error
    Format string `yaml:"format"`  // json, text
}

func New(cfg Config) *slog.Logger {
    level := slog.LevelInfo
    switch cfg.Level {
    case "debug":
        level = slog.LevelDebug
    case "warn":
        level = slog.LevelWarn
    case "error":
        level = slog.LevelError
    }

    opts := &slog.HandlerOptions{
        Level:     level,
        AddSource: level == slog.LevelDebug,
    }

    var handler slog.Handler
    if cfg.Format == "json" {
        handler = slog.NewJSONHandler(os.Stdout, opts)
    } else {
        handler = slog.NewTextHandler(os.Stdout, opts)
    }

    return slog.New(handler)
}
```

### Context Keys

```go
// Add request ID to context
type ctxKey string
const RequestIDKey ctxKey = "request_id"

func WithRequestID(ctx context.Context, id string) context.Context {
    return context.WithValue(ctx, RequestIDKey, id)
}

// Middleware extracts and logs request ID
func RequestIDMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        id := r.Header.Get("X-Request-ID")
        if id == "" {
            id = generateID()
        }
        ctx := WithRequestID(r.Context(), id)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}
```

### Configuration

```yaml
logging:
  level: info      # debug, info, warn, error
  format: json     # json, text
```

Environment override:
```bash
FEATHER_LOG_LEVEL=debug
FEATHER_LOG_FORMAT=text
```
