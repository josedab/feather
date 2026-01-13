// Package tracing provides OpenTelemetry tracing for Feather.
package tracing

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Config holds tracing configuration.
type Config struct {
	Enabled     bool
	Endpoint    string
	ServiceName string
	SampleRate  float64
	Insecure    bool
}

// Tracer wraps OpenTelemetry tracing functionality.
type Tracer struct {
	provider *sdktrace.TracerProvider
	tracer   trace.Tracer
	config   Config
}

// New creates a new tracer instance.
func New(ctx context.Context, config Config) (*Tracer, error) {
	if !config.Enabled {
		return &Tracer{config: config}, nil
	}

	// Create OTLP exporter
	opts := []otlptracegrpc.Option{
		otlptracegrpc.WithEndpoint(config.Endpoint),
	}

	if config.Insecure {
		opts = append(opts, otlptracegrpc.WithDialOption(grpc.WithTransportCredentials(insecure.NewCredentials())))
		opts = append(opts, otlptracegrpc.WithInsecure())
	}

	client := otlptracegrpc.NewClient(opts...)
	exporter, err := otlptrace.New(ctx, client)
	if err != nil {
		// Shutdown the client to avoid resource leak
		_ = client.Stop(ctx)
		return nil, fmt.Errorf("creating OTLP exporter: %w", err)
	}

	// Create resource
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String(config.ServiceName),
			semconv.ServiceVersionKey.String("1.0.0"),
		),
		resource.WithHost(),
		resource.WithProcess(),
	)
	if err != nil {
		// Shutdown the exporter to avoid resource leak
		_ = exporter.Shutdown(ctx)
		return nil, fmt.Errorf("creating resource: %w", err)
	}

	// Create sampler
	var sampler sdktrace.Sampler
	if config.SampleRate >= 1.0 {
		sampler = sdktrace.AlwaysSample()
	} else if config.SampleRate <= 0 {
		sampler = sdktrace.NeverSample()
	} else {
		sampler = sdktrace.TraceIDRatioBased(config.SampleRate)
	}

	// Create provider
	provider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sampler),
	)

	// Set global provider
	otel.SetTracerProvider(provider)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return &Tracer{
		provider: provider,
		tracer:   provider.Tracer(config.ServiceName),
		config:   config,
	}, nil
}

// Shutdown gracefully shuts down the tracer.
func (t *Tracer) Shutdown(ctx context.Context) error {
	if t.provider == nil {
		return nil
	}
	return t.provider.Shutdown(ctx)
}

// StartSpan starts a new span.
func (t *Tracer) StartSpan(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	if t.tracer == nil {
		return ctx, trace.SpanFromContext(ctx)
	}
	return t.tracer.Start(ctx, name, opts...)
}

// HTTPMiddleware returns an HTTP middleware that traces requests.
func (t *Tracer) HTTPMiddleware(next http.Handler) http.Handler {
	if !t.config.Enabled {
		return next
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract context from incoming request
		ctx := otel.GetTextMapPropagator().Extract(r.Context(), propagation.HeaderCarrier(r.Header))

		// Start span
		ctx, span := t.StartSpan(ctx, r.URL.Path,
			trace.WithSpanKind(trace.SpanKindServer),
			trace.WithAttributes(
				semconv.HTTPMethodKey.String(r.Method),
				semconv.HTTPURLKey.String(r.URL.String()),
				semconv.HTTPSchemeKey.String(r.URL.Scheme),
				semconv.HTTPRouteKey.String(r.URL.Path),
				semconv.UserAgentOriginalKey.String(r.UserAgent()),
			),
		)
		defer span.End()

		// Wrap response writer to capture status
		rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		// Serve request
		next.ServeHTTP(rw, r.WithContext(ctx))

		// Record status
		span.SetAttributes(semconv.HTTPStatusCodeKey.Int(rw.statusCode))
		if rw.statusCode >= 400 {
			span.SetAttributes(attribute.Bool("error", true))
		}
	})
}

// responseWriter wraps http.ResponseWriter to capture the status code.
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// GRPCUnaryInterceptor returns a gRPC unary interceptor for tracing.
func (t *Tracer) GRPCUnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		if !t.config.Enabled {
			return handler(ctx, req)
		}

		ctx, span := t.StartSpan(ctx, info.FullMethod,
			trace.WithSpanKind(trace.SpanKindServer),
			trace.WithAttributes(
				semconv.RPCSystemKey.String("grpc"),
				semconv.RPCServiceKey.String(info.FullMethod),
			),
		)
		defer span.End()

		start := time.Now()
		resp, err := handler(ctx, req)
		duration := time.Since(start)

		span.SetAttributes(attribute.Int64("rpc.duration_ms", duration.Milliseconds()))

		if err != nil {
			span.SetAttributes(attribute.Bool("error", true))
			span.RecordError(err)
		}

		return resp, err
	}
}

// GRPCStreamInterceptor returns a gRPC stream interceptor for tracing.
func (t *Tracer) GRPCStreamInterceptor() grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		if !t.config.Enabled {
			return handler(srv, ss)
		}

		ctx, span := t.StartSpan(ss.Context(), info.FullMethod,
			trace.WithSpanKind(trace.SpanKindServer),
			trace.WithAttributes(
				semconv.RPCSystemKey.String("grpc"),
				semconv.RPCServiceKey.String(info.FullMethod),
				attribute.Bool("rpc.is_streaming", true),
			),
		)
		defer span.End()

		// Wrap the stream to use the traced context
		wrappedStream := &tracedServerStream{
			ServerStream: ss,
			ctx:          ctx,
		}

		err := handler(srv, wrappedStream)
		if err != nil {
			span.SetAttributes(attribute.Bool("error", true))
			span.RecordError(err)
		}

		return err
	}
}

// tracedServerStream wraps a gRPC ServerStream with a traced context.
type tracedServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (s *tracedServerStream) Context() context.Context {
	return s.ctx
}

// SpanFromContext returns the current span from context.
func SpanFromContext(ctx context.Context) trace.Span {
	return trace.SpanFromContext(ctx)
}

// AddEvent adds an event to the current span.
func AddEvent(ctx context.Context, name string, attrs ...attribute.KeyValue) {
	span := trace.SpanFromContext(ctx)
	span.AddEvent(name, trace.WithAttributes(attrs...))
}

// SetError marks the current span as having an error.
func SetError(ctx context.Context, err error) {
	span := trace.SpanFromContext(ctx)
	span.RecordError(err)
	span.SetAttributes(attribute.Bool("error", true))
}

// SetAttributes adds attributes to the current span.
func SetAttributes(ctx context.Context, attrs ...attribute.KeyValue) {
	span := trace.SpanFromContext(ctx)
	span.SetAttributes(attrs...)
}

// Storage operation span helpers

// StartStorageSpan starts a span for a storage operation.
func (t *Tracer) StartStorageSpan(ctx context.Context, operation, tier string) (context.Context, trace.Span) {
	if t.tracer == nil {
		return ctx, trace.SpanFromContext(ctx)
	}
	return t.tracer.Start(ctx, "storage."+operation,
		trace.WithAttributes(
			attribute.String("storage.tier", tier),
			attribute.String("storage.operation", operation),
		),
	)
}

// StartHotTierSpan starts a span for a hot tier operation.
func (t *Tracer) StartHotTierSpan(ctx context.Context, operation string, entityKey string) (context.Context, trace.Span) {
	if t.tracer == nil {
		return ctx, trace.SpanFromContext(ctx)
	}
	return t.tracer.Start(ctx, "storage.hot."+operation,
		trace.WithAttributes(
			attribute.String("storage.tier", "hot"),
			attribute.String("storage.operation", operation),
			attribute.String("entity.key", entityKey),
		),
	)
}

// StartWarmTierSpan starts a span for a warm tier operation.
func (t *Tracer) StartWarmTierSpan(ctx context.Context, operation string, entityKey string) (context.Context, trace.Span) {
	if t.tracer == nil {
		return ctx, trace.SpanFromContext(ctx)
	}
	return t.tracer.Start(ctx, "storage.warm."+operation,
		trace.WithAttributes(
			attribute.String("storage.tier", "warm"),
			attribute.String("storage.operation", operation),
			attribute.String("entity.key", entityKey),
		),
	)
}

// Aggregation operation span helpers

// StartAggregationSpan starts a span for an aggregation operation.
func (t *Tracer) StartAggregationSpan(ctx context.Context, operation, feature, aggFunction string) (context.Context, trace.Span) {
	if t.tracer == nil {
		return ctx, trace.SpanFromContext(ctx)
	}
	return t.tracer.Start(ctx, "aggregation."+operation,
		trace.WithAttributes(
			attribute.String("aggregation.feature", feature),
			attribute.String("aggregation.function", aggFunction),
			attribute.String("aggregation.operation", operation),
		),
	)
}

// Federation span helpers

// StartFederationSpan starts a span for a federation operation.
func (t *Tracer) StartFederationSpan(ctx context.Context, operation, targetNode string) (context.Context, trace.Span) {
	if t.tracer == nil {
		return ctx, trace.SpanFromContext(ctx)
	}
	return t.tracer.Start(ctx, "federation."+operation,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("federation.operation", operation),
			attribute.String("federation.target_node", targetNode),
		),
	)
}

// InjectContext injects trace context into HTTP headers for propagation.
func (t *Tracer) InjectContext(ctx context.Context, headers http.Header) {
	if !t.config.Enabled {
		return
	}
	otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(headers))
}

// ExtractContext extracts trace context from HTTP headers.
func (t *Tracer) ExtractContext(ctx context.Context, headers http.Header) context.Context {
	if !t.config.Enabled {
		return ctx
	}
	return otel.GetTextMapPropagator().Extract(ctx, propagation.HeaderCarrier(headers))
}

// SetStorageMetrics adds storage-related metrics to the current span.
func SetStorageMetrics(ctx context.Context, hitCount, missCount int, latencyMs float64) {
	span := trace.SpanFromContext(ctx)
	span.SetAttributes(
		attribute.Int("storage.cache_hits", hitCount),
		attribute.Int("storage.cache_misses", missCount),
		attribute.Float64("storage.latency_ms", latencyMs),
	)
}

// SetAggregationMetrics adds aggregation-related metrics to the current span.
func SetAggregationMetrics(ctx context.Context, windowSize int, dataPoints int, computeTimeMs float64) {
	span := trace.SpanFromContext(ctx)
	span.SetAttributes(
		attribute.Int("aggregation.window_size", windowSize),
		attribute.Int("aggregation.data_points", dataPoints),
		attribute.Float64("aggregation.compute_time_ms", computeTimeMs),
	)
}

// SetFederationMetrics adds federation-related metrics to the current span.
func SetFederationMetrics(ctx context.Context, nodeCount int, responseTimeMs float64, failed bool) {
	span := trace.SpanFromContext(ctx)
	span.SetAttributes(
		attribute.Int("federation.node_count", nodeCount),
		attribute.Float64("federation.response_time_ms", responseTimeMs),
		attribute.Bool("federation.failed", failed),
	)
}
