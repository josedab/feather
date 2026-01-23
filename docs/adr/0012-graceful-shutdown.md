# ADR-0012: Graceful Shutdown with Coordinated Lifecycle

## Status

Accepted

## Context

Feather runs multiple concurrent components:

- HTTP server (port 8080)
- gRPC server (port 50051)
- HTTP ingestion server (port 8081)
- Metrics server (port 9090)
- Kafka consumer (background)
- Background workers (aggregation, sync, cleanup)

In production environments (especially Kubernetes), processes receive termination signals (SIGTERM) and must:

1. **Stop accepting new requests** immediately
2. **Complete in-flight requests** within a deadline
3. **Flush buffers** (metrics, logs, traces)
4. **Close connections** cleanly (database, Kafka)
5. **Exit with appropriate status code**

A naive `os.Exit(0)` would:
- Drop in-flight requests (client errors)
- Lose unflushed data (metrics gaps)
- Leave connections hanging (resource leaks)
- Cause service mesh issues (stale endpoints)

## Decision

We implement **coordinated graceful shutdown** with a server manager pattern.

### Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                      main.go                                 │
│  ┌─────────────────────────────────────────────────────┐   │
│  │                  ServerManager                       │   │
│  │  • Tracks all running servers                       │   │
│  │  • Coordinates startup order                        │   │
│  │  • Orchestrates parallel shutdown                   │   │
│  └─────────────────────────┬───────────────────────────┘   │
│                            │                                │
│            ┌───────────────┼───────────────┐               │
│            │               │               │               │
│            ▼               ▼               ▼               │
│       ┌─────────┐    ┌─────────┐    ┌─────────┐          │
│       │  HTTP   │    │  gRPC   │    │ Metrics │          │
│       │ Server  │    │ Server  │    │ Server  │          │
│       └─────────┘    └─────────┘    └─────────┘          │
│            │               │               │               │
│            └───────────────┼───────────────┘               │
│                            │                                │
│                    Signal Handler                           │
│                    (SIGTERM, SIGINT)                       │
└─────────────────────────────────────────────────────────────┘
```

### Shutdown Sequence

```
1. Receive SIGTERM/SIGINT
   │
2. Cancel root context
   │
3. ServerManager.Shutdown() called
   │
4. For each server (parallel):
   │  ├── Stop accepting new connections
   │  ├── Wait for in-flight requests (with timeout)
   │  └── Close server
   │
5. Flush telemetry (traces, metrics)
   │
6. Close storage (BadgerDB sync)
   │
7. Close Kafka consumer
   │
8. Exit(0) or Exit(1) based on errors
```

### Timeout Configuration

```yaml
server:
  shutdown_timeout: 30s  # Max time to wait for graceful shutdown
```

If components don't shut down within timeout, they're forcefully terminated.

### Kubernetes Integration

```yaml
# Kubernetes deployment
spec:
  terminationGracePeriodSeconds: 35  # > shutdown_timeout
  containers:
    - name: feather
      lifecycle:
        preStop:
          exec:
            command: ["sleep", "5"]  # Allow LB to drain
```

The preStop hook gives load balancers time to stop routing before shutdown begins.

## Consequences

### Positive

- **Zero dropped requests**: In-flight requests complete
- **Clean metrics**: Prometheus scrape sees final values
- **No resource leaks**: All connections properly closed
- **Kubernetes-native**: Works with rolling updates
- **Observable**: Shutdown progress logged
- **Configurable**: Timeout adjustable per deployment

### Negative

- **Complexity**: More code than simple exit
- **Slow shutdown**: Must wait for timeout in worst case
- **Coordination bugs**: Deadlocks possible if not careful
- **Testing difficulty**: Shutdown paths hard to test

### Neutral

- **30s default**: Matches Kubernetes default; tunable
- **Parallel shutdown**: Faster but requires careful synchronization

## Alternatives Considered

| Alternative | Rejected Because |
|-------------|------------------|
| Immediate exit | Drops requests; loses data |
| Sequential shutdown | Too slow; components independent |
| No timeout | Could hang forever on stuck component |
| External orchestration | Adds operational complexity |

## Implementation Notes

### Signal Handling

Key file: `cmd/feather/main.go`

```go
func main() {
    // Create root context with cancellation
    ctx, cancel := context.WithCancel(context.Background())

    // Handle shutdown signals
    sigCh := make(chan os.Signal, 1)
    signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)

    go func() {
        sig := <-sigCh
        logger.Info("received signal, initiating shutdown", "signal", sig)
        cancel()
    }()

    // Start servers...
    serverManager := NewServerManager(logger)
    serverManager.Add(httpServer)
    serverManager.Add(grpcServer)
    serverManager.Add(metricsServer)

    // Wait for shutdown signal
    <-ctx.Done()

    // Graceful shutdown with timeout
    shutdownCtx, shutdownCancel := context.WithTimeout(
        context.Background(),
        cfg.Server.ShutdownTimeout,
    )
    defer shutdownCancel()

    if err := serverManager.Shutdown(shutdownCtx); err != nil {
        logger.Error("shutdown error", "error", err)
        os.Exit(1)
    }

    logger.Info("shutdown complete")
}
```

### Server Manager

```go
type ServerManager struct {
    servers []Server
    logger  *slog.Logger
    mu      sync.Mutex
}

type Server interface {
    Shutdown(ctx context.Context) error
}

func (sm *ServerManager) Add(s Server) {
    sm.mu.Lock()
    defer sm.mu.Unlock()
    sm.servers = append(sm.servers, s)
}

func (sm *ServerManager) Shutdown(ctx context.Context) error {
    sm.mu.Lock()
    servers := sm.servers
    sm.mu.Unlock()

    var wg sync.WaitGroup
    errCh := make(chan error, len(servers))

    for _, s := range servers {
        wg.Add(1)
        go func(server Server) {
            defer wg.Done()
            if err := server.Shutdown(ctx); err != nil {
                sm.logger.Error("server shutdown failed", "error", err)
                errCh <- err
            }
        }(s)
    }

    wg.Wait()
    close(errCh)

    // Collect errors
    var errs []error
    for err := range errCh {
        errs = append(errs, err)
    }

    if len(errs) > 0 {
        return fmt.Errorf("shutdown errors: %v", errs)
    }
    return nil
}
```

### HTTP Server Shutdown

```go
func (s *HTTPServer) Shutdown(ctx context.Context) error {
    s.logger.Info("shutting down HTTP server")

    // Stop accepting new connections, wait for existing
    if err := s.server.Shutdown(ctx); err != nil {
        return fmt.Errorf("http shutdown: %w", err)
    }

    s.logger.Info("HTTP server stopped")
    return nil
}
```

### Health Check During Shutdown

Once shutdown begins, health endpoints return unhealthy:

```go
func (s *HTTPServer) handleReady(w http.ResponseWriter, r *http.Request) {
    if s.shuttingDown.Load() {
        w.WriteHeader(http.StatusServiceUnavailable)
        w.Write([]byte("shutting down"))
        return
    }
    w.WriteHeader(http.StatusOK)
    w.Write([]byte("ready"))
}
```

This allows load balancers to stop routing traffic before the server fully stops.
