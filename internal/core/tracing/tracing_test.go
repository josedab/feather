package tracing

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

type tracingTestKey struct{}

func TestNew_Disabled(t *testing.T) {
	ctx := context.Background()
	tracer, err := New(ctx, Config{
		Enabled: false,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if tracer.provider != nil {
		t.Error("expected nil provider when disabled")
	}

	// Should not panic when using disabled tracer
	ctx2, span := tracer.StartSpan(ctx, "test")
	if ctx2 == nil {
		t.Error("expected non-nil context")
	}
	if span == nil {
		t.Error("expected non-nil span (noop)")
	}
}

func TestTracer_StartSpan_Disabled(t *testing.T) {
	tracer := &Tracer{config: Config{Enabled: false}}
	ctx := context.Background()

	ctx2, span := tracer.StartSpan(ctx, "test-span")
	if ctx2 != ctx {
		t.Error("expected same context when disabled")
	}
	// Span should be the noop span from context
	_ = span
}

func TestTracer_StartStorageSpan_Disabled(t *testing.T) {
	tracer := &Tracer{config: Config{Enabled: false}}
	ctx := context.Background()

	ctx2, span := tracer.StartStorageSpan(ctx, "get", "hot")
	if ctx2 != ctx {
		t.Error("expected same context when disabled")
	}
	_ = span
}

func TestTracer_StartHotTierSpan_Disabled(t *testing.T) {
	tracer := &Tracer{config: Config{Enabled: false}}
	ctx := context.Background()

	ctx2, span := tracer.StartHotTierSpan(ctx, "get", "entity:123")
	if ctx2 != ctx {
		t.Error("expected same context when disabled")
	}
	_ = span
}

func TestTracer_StartWarmTierSpan_Disabled(t *testing.T) {
	tracer := &Tracer{config: Config{Enabled: false}}
	ctx := context.Background()

	ctx2, span := tracer.StartWarmTierSpan(ctx, "get", "entity:456")
	if ctx2 != ctx {
		t.Error("expected same context when disabled")
	}
	_ = span
}

func TestTracer_StartAggregationSpan_Disabled(t *testing.T) {
	tracer := &Tracer{config: Config{Enabled: false}}
	ctx := context.Background()

	ctx2, span := tracer.StartAggregationSpan(ctx, "compute", "feature_1h", "sum")
	if ctx2 != ctx {
		t.Error("expected same context when disabled")
	}
	_ = span
}

func TestTracer_StartFederationSpan_Disabled(t *testing.T) {
	tracer := &Tracer{config: Config{Enabled: false}}
	ctx := context.Background()

	ctx2, span := tracer.StartFederationSpan(ctx, "route", "node-2")
	if ctx2 != ctx {
		t.Error("expected same context when disabled")
	}
	_ = span
}

func TestTracer_HTTPMiddleware_Disabled(t *testing.T) {
	tracer := &Tracer{config: Config{Enabled: false}}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrapped := tracer.HTTPMiddleware(handler)

	req := httptest.NewRequest("GET", "/test", nil)
	rr := httptest.NewRecorder()

	wrapped.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}
}

func TestTracer_InjectExtractContext_Disabled(t *testing.T) {
	tracer := &Tracer{config: Config{Enabled: false}}
	ctx := context.Background()

	headers := make(http.Header)
	tracer.InjectContext(ctx, headers)

	// Should not add any headers when disabled
	if len(headers) > 0 {
		t.Errorf("expected no headers when disabled, got %v", headers)
	}

	ctx2 := tracer.ExtractContext(ctx, headers)
	if ctx2 != ctx {
		t.Error("expected same context when disabled")
	}
}

func TestTracer_Shutdown_NilProvider(t *testing.T) {
	tracer := &Tracer{config: Config{Enabled: false}}
	err := tracer.Shutdown(context.Background())
	if err != nil {
		t.Errorf("Shutdown() error = %v", err)
	}
}

func TestResponseWriter(t *testing.T) {
	rw := httptest.NewRecorder()
	wrapped := &responseWriter{ResponseWriter: rw, statusCode: http.StatusOK}

	wrapped.WriteHeader(http.StatusCreated)

	if wrapped.statusCode != http.StatusCreated {
		t.Errorf("expected status 201, got %d", wrapped.statusCode)
	}

	if rw.Code != http.StatusCreated {
		t.Errorf("expected underlying recorder status 201, got %d", rw.Code)
	}
}

func TestSpanFromContext(t *testing.T) {
	ctx := context.Background()
	span := SpanFromContext(ctx)
	if span == nil {
		t.Error("expected non-nil span (noop)")
	}
}

func TestAddEvent(t *testing.T) {
	ctx := context.Background()
	// Should not panic on noop span
	AddEvent(ctx, "test-event", attribute.String("key", "value"))
}

func TestSetError(t *testing.T) {
	ctx := context.Background()
	// Should not panic on noop span
	SetError(ctx, http.ErrServerClosed)
}

func TestSetAttributes(t *testing.T) {
	ctx := context.Background()
	// Should not panic on noop span
	SetAttributes(ctx, attribute.String("key", "value"))
}

func TestSetStorageMetrics(t *testing.T) {
	ctx := context.Background()
	// Should not panic on noop span
	SetStorageMetrics(ctx, 10, 2, 1.5)
}

func TestSetAggregationMetrics(t *testing.T) {
	ctx := context.Background()
	// Should not panic on noop span
	SetAggregationMetrics(ctx, 3600, 1000, 0.5)
}

func TestSetFederationMetrics(t *testing.T) {
	ctx := context.Background()
	// Should not panic on noop span
	SetFederationMetrics(ctx, 3, 25.0, false)
}

func TestGRPCUnaryInterceptor_Disabled(t *testing.T) {
	tracer := &Tracer{config: Config{Enabled: false}}
	interceptor := tracer.GRPCUnaryInterceptor()

	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return "response", nil
	}

	resp, err := interceptor(context.Background(), nil, nil, handler)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if resp != "response" {
		t.Errorf("expected 'response', got %v", resp)
	}
}

func TestGRPCStreamInterceptor_Disabled(t *testing.T) {
	tracer := &Tracer{config: Config{Enabled: false}}
	interceptor := tracer.GRPCStreamInterceptor()

	handler := func(srv interface{}, ss grpc.ServerStream) error {
		return nil
	}

	err := interceptor(nil, nil, nil, handler)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// mockServerStream implements grpc.ServerStream for testing
type mockServerStream struct {
	ctx context.Context
}

func (m *mockServerStream) SetHeader(metadata.MD) error  { return nil }
func (m *mockServerStream) SendHeader(metadata.MD) error { return nil }
func (m *mockServerStream) SetTrailer(metadata.MD)       {}
func (m *mockServerStream) Context() context.Context     { return m.ctx }
func (m *mockServerStream) SendMsg(interface{}) error    { return nil }
func (m *mockServerStream) RecvMsg(interface{}) error    { return nil }

func TestTracedServerStream_Context(t *testing.T) {
	ctx := context.WithValue(context.Background(), tracingTestKey{}, "value")
	stream := &tracedServerStream{
		ServerStream: &mockServerStream{ctx: context.Background()},
		ctx:          ctx,
	}

	if stream.Context() != ctx {
		t.Error("expected traced context")
	}
}

// --- New() enabled path tests ---

func TestNew_Enabled_InvalidEndpoint(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Use an unreachable endpoint; New should still succeed
	// because the OTLP exporter creates a client but doesn't connect synchronously
	tracer, err := New(ctx, Config{
		Enabled:     true,
		Endpoint:    "localhost:0",
		ServiceName: "test-service",
		SampleRate:  1.0,
		Insecure:    true,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer tracer.Shutdown(ctx)

	if tracer.provider == nil {
		t.Error("expected non-nil provider when enabled")
	}
	if tracer.tracer == nil {
		t.Error("expected non-nil tracer when enabled")
	}
}

func TestNew_Enabled_AlwaysSample(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	tracer, err := New(ctx, Config{
		Enabled:     true,
		Endpoint:    "localhost:0",
		ServiceName: "always-sample",
		SampleRate:  1.0,
		Insecure:    true,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer tracer.Shutdown(ctx)

	// Should be able to create spans
	_, span := tracer.StartSpan(ctx, "test-span")
	if span == nil {
		t.Error("expected non-nil span")
	}
	span.End()
}

func TestNew_Enabled_NeverSample(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	tracer, err := New(ctx, Config{
		Enabled:     true,
		Endpoint:    "localhost:0",
		ServiceName: "never-sample",
		SampleRate:  0,
		Insecure:    true,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer tracer.Shutdown(ctx)

	if tracer.provider == nil {
		t.Error("expected non-nil provider")
	}
}

func TestNew_Enabled_TraceIDRatio(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	tracer, err := New(ctx, Config{
		Enabled:     true,
		Endpoint:    "localhost:0",
		ServiceName: "ratio-sample",
		SampleRate:  0.5,
		Insecure:    true,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer tracer.Shutdown(ctx)

	if tracer.provider == nil {
		t.Error("expected non-nil provider for ratio sampler")
	}
}

func TestShutdown_AfterEnabled(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	tracer, err := New(ctx, Config{
		Enabled:     true,
		Endpoint:    "localhost:0",
		ServiceName: "shutdown-test",
		SampleRate:  1.0,
		Insecure:    true,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	err = tracer.Shutdown(ctx)
	if err != nil {
		t.Errorf("Shutdown() error = %v", err)
	}
}

func TestNew_Enabled_ServiceNamePropagation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	serviceName := "feather-test-service"
	tracer, err := New(ctx, Config{
		Enabled:     true,
		Endpoint:    "localhost:0",
		ServiceName: serviceName,
		SampleRate:  1.0,
		Insecure:    true,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer tracer.Shutdown(ctx)

	if tracer.config.ServiceName != serviceName {
		t.Errorf("expected service name %s, got %s", serviceName, tracer.config.ServiceName)
	}
}

func TestNew_Enabled_SpanOperations(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	tracer, err := New(ctx, Config{
		Enabled:     true,
		Endpoint:    "localhost:0",
		ServiceName: "span-ops",
		SampleRate:  1.0,
		Insecure:    true,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer tracer.Shutdown(ctx)

	// Storage span
	ctx2, span := tracer.StartStorageSpan(ctx, "get", "hot")
	if ctx2 == nil || span == nil {
		t.Error("expected non-nil context and span")
	}
	span.End()

	// Hot tier span
	ctx3, span2 := tracer.StartHotTierSpan(ctx, "get", "user:1")
	if ctx3 == nil || span2 == nil {
		t.Error("expected non-nil context and span")
	}
	span2.End()

	// Warm tier span
	ctx4, span3 := tracer.StartWarmTierSpan(ctx, "put", "user:1")
	if ctx4 == nil || span3 == nil {
		t.Error("expected non-nil context and span")
	}
	span3.End()

	// Aggregation span
	ctx5, span4 := tracer.StartAggregationSpan(ctx, "compute", "clicks_1h", "sum")
	if ctx5 == nil || span4 == nil {
		t.Error("expected non-nil context and span")
	}
	span4.End()

	// Federation span
	ctx6, span5 := tracer.StartFederationSpan(ctx, "route", "node-2")
	if ctx6 == nil || span5 == nil {
		t.Error("expected non-nil context and span")
	}
	span5.End()
}

func TestNew_Enabled_HTTPMiddleware(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	tracer, err := New(ctx, Config{
		Enabled:     true,
		Endpoint:    "localhost:0",
		ServiceName: "http-middleware",
		SampleRate:  1.0,
		Insecure:    true,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer tracer.Shutdown(ctx)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrapped := tracer.HTTPMiddleware(handler)
	req := httptest.NewRequest("GET", "/test", nil)
	rr := httptest.NewRecorder()
	wrapped.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestNew_Enabled_InjectExtractContext(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	tracer, err := New(ctx, Config{
		Enabled:     true,
		Endpoint:    "localhost:0",
		ServiceName: "inject-extract",
		SampleRate:  1.0,
		Insecure:    true,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer tracer.Shutdown(ctx)

	headers := make(http.Header)
	tracer.InjectContext(ctx, headers)

	ctx2 := tracer.ExtractContext(ctx, headers)
	if ctx2 == nil {
		t.Error("expected non-nil context from extract")
	}
}
