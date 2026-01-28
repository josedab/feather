package tracing

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"google.golang.org/grpc"
)

// newTestTracer creates a Tracer with an in-memory span exporter for testing.
func newTestTracer(t *testing.T) (*Tracer, *tracetest.InMemoryExporter) {
	t.Helper()

	exporter := tracetest.NewInMemoryExporter()
	provider := sdktrace.NewTracerProvider(
		sdktrace.WithSyncer(exporter),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)

	tr := &Tracer{
		provider: provider,
		tracer:   provider.Tracer("feather-test"),
		config:   Config{Enabled: true, ServiceName: "feather-test"},
	}

	t.Cleanup(func() {
		provider.Shutdown(context.Background())
	})

	return tr, exporter
}

func TestTracer_StartSpan_Enabled(t *testing.T) {
	tr, exporter := newTestTracer(t)
	ctx := context.Background()

	ctx, span := tr.StartSpan(ctx, "test-operation")
	span.End()

	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	if spans[0].Name != "test-operation" {
		t.Errorf("span name = %q, want %q", spans[0].Name, "test-operation")
	}
}

func TestTracer_StartStorageSpan_Enabled(t *testing.T) {
	tr, exporter := newTestTracer(t)
	ctx := context.Background()

	ctx, span := tr.StartStorageSpan(ctx, "get", "hot")
	span.End()

	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	if spans[0].Name != "storage.get" {
		t.Errorf("span name = %q, want %q", spans[0].Name, "storage.get")
	}
}

func TestTracer_StartHotTierSpan_Enabled(t *testing.T) {
	tr, exporter := newTestTracer(t)
	ctx := context.Background()

	_, span := tr.StartHotTierSpan(ctx, "put", "user:123")
	span.End()

	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	if spans[0].Name != "storage.hot.put" {
		t.Errorf("span name = %q, want %q", spans[0].Name, "storage.hot.put")
	}
}

func TestTracer_StartWarmTierSpan_Enabled(t *testing.T) {
	tr, exporter := newTestTracer(t)
	ctx := context.Background()

	_, span := tr.StartWarmTierSpan(ctx, "scan", "entity:456")
	span.End()

	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	if spans[0].Name != "storage.warm.scan" {
		t.Errorf("span name = %q, want %q", spans[0].Name, "storage.warm.scan")
	}
}

func TestTracer_StartAggregationSpan_Enabled(t *testing.T) {
	tr, exporter := newTestTracer(t)
	ctx := context.Background()

	_, span := tr.StartAggregationSpan(ctx, "compute", "tx_count_1h", "sum")
	span.End()

	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	if spans[0].Name != "aggregation.compute" {
		t.Errorf("span name = %q, want %q", spans[0].Name, "aggregation.compute")
	}
}

func TestTracer_StartFederationSpan_Enabled(t *testing.T) {
	tr, exporter := newTestTracer(t)
	ctx := context.Background()

	_, span := tr.StartFederationSpan(ctx, "route", "node-2")
	span.End()

	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	if spans[0].Name != "federation.route" {
		t.Errorf("span name = %q, want %q", spans[0].Name, "federation.route")
	}
}

func TestTracer_HTTPMiddleware_Enabled(t *testing.T) {
	tr, exporter := newTestTracer(t)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	wrapped := tr.HTTPMiddleware(handler)
	req := httptest.NewRequest("GET", "/v1/features?entity=user:1", nil)
	rr := httptest.NewRecorder()

	wrapped.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rr.Code)
	}

	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span from middleware, got %d", len(spans))
	}
}

func TestTracer_HTTPMiddleware_ErrorStatus(t *testing.T) {
	tr, exporter := newTestTracer(t)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	})

	wrapped := tr.HTTPMiddleware(handler)
	req := httptest.NewRequest("POST", "/v1/features", nil)
	rr := httptest.NewRecorder()

	wrapped.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rr.Code)
	}

	spans := exporter.GetSpans()
	if len(spans) == 0 {
		t.Fatal("expected at least 1 span")
	}
}

func TestTracer_GRPCUnaryInterceptor_Enabled(t *testing.T) {
	tr, exporter := newTestTracer(t)
	interceptor := tr.GRPCUnaryInterceptor()

	info := &grpc.UnaryServerInfo{FullMethod: "/feather.v1.FeatureService/GetFeatures"}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return "features", nil
	}

	resp, err := interceptor(context.Background(), nil, info, handler)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp != "features" {
		t.Errorf("expected 'features', got %v", resp)
	}

	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
}

func TestTracer_GRPCUnaryInterceptor_Error(t *testing.T) {
	tr, exporter := newTestTracer(t)
	interceptor := tr.GRPCUnaryInterceptor()

	info := &grpc.UnaryServerInfo{FullMethod: "/feather.v1.FeatureService/GetFeatures"}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return nil, errors.New("not found")
	}

	_, err := interceptor(context.Background(), nil, info, handler)
	if err == nil {
		t.Error("expected error")
	}

	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
}

func TestTracer_GRPCStreamInterceptor_Enabled(t *testing.T) {
	tr, exporter := newTestTracer(t)
	interceptor := tr.GRPCStreamInterceptor()

	ctx := context.Background()
	stream := &mockServerStream{ctx: ctx}
	info := &grpc.StreamServerInfo{FullMethod: "/feather.v1.FeatureService/StreamFeatures"}

	handler := func(srv interface{}, ss grpc.ServerStream) error {
		return nil
	}

	err := interceptor(nil, stream, info, handler)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
}

func TestTracer_GRPCStreamInterceptor_Error(t *testing.T) {
	tr, exporter := newTestTracer(t)
	interceptor := tr.GRPCStreamInterceptor()

	ctx := context.Background()
	stream := &mockServerStream{ctx: ctx}
	info := &grpc.StreamServerInfo{FullMethod: "/feather.v1.FeatureService/StreamFeatures"}

	handler := func(srv interface{}, ss grpc.ServerStream) error {
		return errors.New("stream error")
	}

	err := interceptor(nil, stream, info, handler)
	if err == nil {
		t.Error("expected error")
	}

	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
}

func TestTracer_InjectExtractContext_Enabled(t *testing.T) {
	tr, _ := newTestTracer(t)

	// InjectContext and ExtractContext use the global propagator set by otel.
	// With our test tracer (not created via New), the global propagator may not be set,
	// so we test that inject/extract don't panic and work symmetrically.
	ctx := context.Background()
	ctx, span := tr.StartSpan(ctx, "test")
	defer span.End()

	headers := make(http.Header)
	// Should not panic
	tr.InjectContext(ctx, headers)

	// Extract should return a valid context
	ctx2 := tr.ExtractContext(context.Background(), headers)
	if ctx2 == nil {
		t.Error("expected non-nil context from ExtractContext")
	}
}

func TestTracer_Shutdown(t *testing.T) {
	tr, _ := newTestTracer(t)
	err := tr.Shutdown(context.Background())
	if err != nil {
		t.Errorf("Shutdown() error = %v", err)
	}
}

func TestSetStorageMetrics_WithActiveSpan(t *testing.T) {
	tr, exporter := newTestTracer(t)
	ctx, span := tr.StartSpan(context.Background(), "storage-op")
	SetStorageMetrics(ctx, 10, 2, 1.5)
	span.End()

	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatal("expected 1 span")
	}
}

func TestSetAggregationMetrics_WithActiveSpan(t *testing.T) {
	tr, exporter := newTestTracer(t)
	ctx, span := tr.StartSpan(context.Background(), "agg-op")
	SetAggregationMetrics(ctx, 3600, 1000, 0.5)
	span.End()

	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatal("expected 1 span")
	}
}

func TestSetFederationMetrics_WithActiveSpan(t *testing.T) {
	tr, exporter := newTestTracer(t)
	ctx, span := tr.StartSpan(context.Background(), "fed-op")
	SetFederationMetrics(ctx, 3, 25.0, false)
	span.End()

	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatal("expected 1 span")
	}
}

func TestAddEvent_WithActiveSpan(t *testing.T) {
	tr, exporter := newTestTracer(t)
	ctx, span := tr.StartSpan(context.Background(), "event-op")
	AddEvent(ctx, "cache.hit", attribute.String("key", "user:1"))
	span.End()

	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatal("expected 1 span")
	}
	if len(spans[0].Events) == 0 {
		t.Error("expected at least one event on span")
	}
}

func TestSetError_WithActiveSpan(t *testing.T) {
	tr, exporter := newTestTracer(t)
	ctx, span := tr.StartSpan(context.Background(), "error-op")
	SetError(ctx, errors.New("test error"))
	span.End()

	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatal("expected 1 span")
	}
}

func TestSetAttributes_WithActiveSpan(t *testing.T) {
	tr, exporter := newTestTracer(t)
	ctx, span := tr.StartSpan(context.Background(), "attr-op")
	SetAttributes(ctx, attribute.String("custom.key", "value"))
	span.End()

	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatal("expected 1 span")
	}
}
