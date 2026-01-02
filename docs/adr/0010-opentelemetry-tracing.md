# ADR-0010: OpenTelemetry for Distributed Tracing

## Status

Accepted

## Context

Feather operates in distributed environments where a single request may:

1. Enter via HTTP or gRPC
2. Check the hot tier cache
3. Fall through to warm tier (BadgerDB)
4. Trigger aggregation computation
5. Return through middleware layers

Without distributed tracing, debugging latency issues or failures requires correlating logs manually—a time-consuming and error-prone process.

We needed a tracing solution that:
- Captures request flow across components
- Integrates with existing observability backends (Jaeger, Zipkin, Datadog, etc.)
- Has low overhead in production
- Supports sampling for high-throughput systems
- Doesn't lock us into a specific vendor

## Decision

We adopt **OpenTelemetry (OTEL)** as our distributed tracing framework.

### Key Characteristics

- **Vendor-neutral**: CNCF project with broad industry support
- **OTLP protocol**: Standard export format accepted by most backends
- **Context propagation**: Automatic trace context across HTTP/gRPC
- **Sampling**: Configurable to balance detail vs. overhead
- **Baggage**: Propagate custom attributes across service boundaries

### Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                        Feather                               │
│  ┌─────────┐    ┌─────────┐    ┌─────────┐    ┌─────────┐  │
│  │  HTTP   │───▶│ Storage │───▶│  Agg    │───▶│Response │  │
│  │ Handler │    │  Layer  │    │ Engine  │    │         │  │
│  └────┬────┘    └────┬────┘    └────┬────┘    └────┬────┘  │
│       │              │              │              │        │
│       ▼              ▼              ▼              ▼        │
│  ┌──────────────────────────────────────────────────────┐  │
│  │              OpenTelemetry SDK                        │  │
│  │  • Span creation  • Context propagation  • Sampling  │  │
│  └────────────────────────┬─────────────────────────────┘  │
│                           │                                 │
└───────────────────────────┼─────────────────────────────────┘
                            │ OTLP (gRPC or HTTP)
                            ▼
                    ┌───────────────┐
                    │  Collector /  │
                    │    Backend    │
                    │ (Jaeger, etc) │
                    └───────────────┘
```

### Span Structure

```
Trace: GetFeatures
├── Span: HTTP Handler (root)
│   ├── Attribute: http.method = GET
│   ├── Attribute: http.url = /v1/features
│   └── Child Spans:
│       ├── Span: HotTier.Get
│       │   ├── Attribute: cache.hit = false
│       │   └── Duration: 0.1ms
│       ├── Span: WarmTier.Get
│       │   ├── Attribute: entity_id = user:123
│       │   └── Duration: 5ms
│       └── Span: Aggregation.Compute
│           ├── Attribute: function = count
│           └── Duration: 0.5ms
```

### Configuration

```yaml
tracing:
  enabled: true
  endpoint: "localhost:4317"      # OTLP gRPC endpoint
  sample_rate: 0.1                # 10% of requests
  service_name: "feather"
  insecure: false                 # TLS for production
```

## Consequences

### Positive

- **Vendor flexibility**: Switch backends without code changes
- **Industry standard**: Broad tooling and community support
- **Low overhead**: Sampling keeps production impact minimal
- **Automatic propagation**: W3C Trace Context headers handled
- **Rich context**: Attributes capture relevant debugging info
- **Future-proof**: OTEL is the emerging standard, replacing OpenTracing/OpenCensus

### Negative

- **Complexity**: OTEL SDK has many configuration options
- **Dependency size**: OTEL packages add to binary size
- **Collector requirement**: Often need OTEL Collector as intermediary
- **Learning curve**: Span/trace concepts need team understanding

### Neutral

- **Optional feature**: Can disable in simple deployments
- **Fail-soft**: Tracing failures don't affect request handling

## Alternatives Considered

| Alternative | Rejected Because |
|-------------|------------------|
| Jaeger client | Vendor-specific; OTEL is the standard now |
| Zipkin client | Vendor-specific; OTEL is the standard now |
| OpenTracing | Deprecated in favor of OpenTelemetry |
| OpenCensus | Merged into OpenTelemetry |
| No tracing | Debugging distributed issues too difficult |

## Implementation Notes

### Tracer Initialization

Key file: `internal/tracing/tracing.go`

```go
package tracing

import (
    "context"

    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
    "go.opentelemetry.io/otel/sdk/resource"
    sdktrace "go.opentelemetry.io/otel/sdk/trace"
    semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
)

type Config struct {
    Enabled     bool    `yaml:"enabled"`
    Endpoint    string  `yaml:"endpoint"`
    SampleRate  float64 `yaml:"sample_rate"`
    ServiceName string  `yaml:"service_name"`
    Insecure    bool    `yaml:"insecure"`
}

func Init(ctx context.Context, cfg Config) (func(), error) {
    if !cfg.Enabled {
        return func() {}, nil
    }

    opts := []otlptracegrpc.Option{
        otlptracegrpc.WithEndpoint(cfg.Endpoint),
    }
    if cfg.Insecure {
        opts = append(opts, otlptracegrpc.WithInsecure())
    }

    exporter, err := otlptracegrpc.New(ctx, opts...)
    if err != nil {
        return nil, err
    }

    res := resource.NewWithAttributes(
        semconv.SchemaURL,
        semconv.ServiceName(cfg.ServiceName),
    )

    sampler := sdktrace.ParentBased(
        sdktrace.TraceIDRatioBased(cfg.SampleRate),
    )

    tp := sdktrace.NewTracerProvider(
        sdktrace.WithBatcher(exporter),
        sdktrace.WithResource(res),
        sdktrace.WithSampler(sampler),
    )

    otel.SetTracerProvider(tp)

    return func() { tp.Shutdown(context.Background()) }, nil
}
```

### Instrumenting Handlers

```go
func (s *HTTPServer) handleGetFeatures(w http.ResponseWriter, r *http.Request) {
    ctx, span := otel.Tracer("feather").Start(r.Context(), "GetFeatures")
    defer span.End()

    span.SetAttributes(
        attribute.String("entity_id", r.URL.Query().Get("entity")),
    )

    features, err := s.store.Get(ctx, entityID, featureNames)
    if err != nil {
        span.RecordError(err)
        span.SetStatus(codes.Error, err.Error())
        // ...
    }
}
```

### Storage Layer Spans

```go
func (s *Store) Get(ctx context.Context, entityID string, features []string) (map[string]FeatureValue, error) {
    ctx, span := otel.Tracer("feather").Start(ctx, "Store.Get")
    defer span.End()

    // Try hot tier
    ctx, hotSpan := otel.Tracer("feather").Start(ctx, "HotTier.Get")
    result, found := s.hot.Get(entityID, features)
    hotSpan.SetAttributes(attribute.Bool("cache.hit", found))
    hotSpan.End()

    if found {
        return result, nil
    }

    // Fall through to warm tier
    ctx, warmSpan := otel.Tracer("feather").Start(ctx, "WarmTier.Get")
    defer warmSpan.End()
    return s.warm.Get(entityID, features)
}
```
