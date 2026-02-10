package metrics

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"google.golang.org/grpc"
)

func TestNewMetrics(t *testing.T) {
	m := NewMetrics("feather_test")
	if m == nil {
		t.Fatal("NewMetrics returned nil")
	}
}

func TestMetrics_RecordHTTPLatency(t *testing.T) {
	m := NewMetrics("feather_test_http")

	// Should not panic
	m.RecordHTTPLatency("GET", "/v1/features", 100*time.Millisecond)
}

func TestMetrics_RecordHTTPRequest(t *testing.T) {
	m := NewMetrics("feather_test_httpreq")

	// Should not panic
	m.RecordHTTPRequest("GET", "/v1/features", http.StatusOK)
	m.RecordHTTPRequest("POST", "/v1/features", http.StatusCreated)
	m.RecordHTTPRequest("GET", "/v1/features", http.StatusNotFound)
}

func TestMetrics_RecordGRPCLatency(t *testing.T) {
	m := NewMetrics("feather_test_grpc")

	// Should not panic
	m.RecordGRPCLatency("/feather.v1.FeatureService/GetFeatures", 50*time.Millisecond)
}

func TestMetrics_RecordGRPCRequest(t *testing.T) {
	m := NewMetrics("feather_test_grpcreq")

	// Should not panic
	m.RecordGRPCRequest("/feather.v1.FeatureService/GetFeatures", "OK")
	m.RecordGRPCRequest("/feather.v1.FeatureService/PutFeatures", "ERROR")
}

func TestMetrics_CacheOperations(t *testing.T) {
	m := NewMetrics("feather_test_cache")

	// Should not panic
	m.RecordCacheHit()
	m.RecordCacheHit()
	m.RecordCacheMiss()
}

func TestMetrics_SetHotTierSize(t *testing.T) {
	m := NewMetrics("feather_test_hot")

	// Should not panic
	m.SetHotTierSize(1024 * 1024)
}

func TestMetrics_SetWarmTierSize(t *testing.T) {
	m := NewMetrics("feather_test_warm")

	// Should not panic
	m.SetWarmTierSize(1024 * 1024 * 1024)
}

func TestMetrics_SetEntityCount(t *testing.T) {
	m := NewMetrics("feather_test_entity")

	// Should not panic
	m.SetEntityCount(10000)
}

func TestMetrics_RecordIngestion(t *testing.T) {
	m := NewMetrics("feather_test_ingest")

	// Should not panic
	m.RecordIngestion("kafka", true)
	m.RecordIngestion("http", false)
}

func TestMetrics_SetIngestionLag(t *testing.T) {
	m := NewMetrics("feather_test_lag")

	// Should not panic
	m.SetIngestionLag(5 * time.Second)
}

func TestMetrics_SetFeatureFreshness(t *testing.T) {
	m := NewMetrics("feather_test_fresh")

	// Should not panic
	m.SetFeatureFreshness("user_features", 30*time.Second)
}

func TestMetrics_RecordFeatureRequest(t *testing.T) {
	m := NewMetrics("feather_test_featreq")

	// Should not panic
	m.RecordFeatureRequest("age")
	m.RecordFeatureRequest("income")
}

func TestMetrics_RecordAggregationCompute(t *testing.T) {
	m := NewMetrics("feather_test_agg")

	// Should not panic
	m.RecordAggregationCompute("purchase_count", 5*time.Millisecond)
}

func TestMetrics_RecordWarmTierOp(t *testing.T) {
	m := NewMetrics("feather_test_warmop")

	// Should not panic
	m.RecordWarmTierOp("get", 10*time.Millisecond)
	m.RecordWarmTierOp("put", 20*time.Millisecond)
}

func TestMetrics_RecordEviction(t *testing.T) {
	m := NewMetrics("feather_test_evict")

	// Should not panic
	m.RecordEviction("hot", "size_limit")
	m.RecordEviction("hot", "ttl")
}

func TestMetrics_RecordShardWait(t *testing.T) {
	m := NewMetrics("feather_test_shard")

	// Should not panic
	m.RecordShardWait("0", 1*time.Microsecond)
}

func TestMetrics_RecordError(t *testing.T) {
	m := NewMetrics("feather_test_err")

	// Should not panic
	m.RecordError("storage", "not_found")
	m.RecordError("ingestion", "decode_error")
}

func TestMetrics_UpdateRuntimeMetrics(t *testing.T) {
	m := NewMetrics("feather_test_runtime")

	// Should not panic
	m.UpdateRuntimeMetrics()
}

func TestMetrics_StartRuntimeMetricsCollector(t *testing.T) {
	m := NewMetrics("feather_test_collector")

	stop := m.StartRuntimeMetricsCollector(100 * time.Millisecond)

	// Let it run for a bit
	time.Sleep(250 * time.Millisecond)

	// Stop should work without panic (only call once)
	stop()
}

func TestMetrics_Handler(t *testing.T) {
	m := NewMetrics("feather_test_handler")

	handler := m.Handler()
	if handler == nil {
		t.Fatal("Handler() returned nil")
	}

	// Test that it responds to requests
	req := httptest.NewRequest("GET", "/metrics", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Handler returned status %d, want 200", rec.Code)
	}
}

func TestMetrics_UnaryServerInterceptor(t *testing.T) {
	m := NewMetrics("feather_test_unary")

	interceptor := m.UnaryServerInterceptor()
	if interceptor == nil {
		t.Fatal("UnaryServerInterceptor() returned nil")
	}

	// Test successful call
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return "ok", nil
	}
	info := &grpc.UnaryServerInfo{FullMethod: "/test.Service/Method"}
	resp, err := interceptor(context.Background(), "request", info, handler)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if resp != "ok" {
		t.Errorf("response = %v, want 'ok'", resp)
	}

	// Test error call
	handlerErr := func(ctx context.Context, req interface{}) (interface{}, error) {
		return nil, fmt.Errorf("test error")
	}
	resp, err = interceptor(context.Background(), "request", info, handlerErr)
	if err == nil {
		t.Error("expected error")
	}
	if resp != nil {
		t.Errorf("response = %v, want nil", resp)
	}
}

func TestMetrics_StreamServerInterceptor(t *testing.T) {
	m := NewMetrics("feather_test_stream")

	interceptor := m.StreamServerInterceptor()
	if interceptor == nil {
		t.Fatal("StreamServerInterceptor() returned nil")
	}

	// Test successful stream
	handler := func(srv interface{}, ss grpc.ServerStream) error {
		return nil
	}
	info := &grpc.StreamServerInfo{FullMethod: "/test.Service/StreamMethod"}
	err := interceptor(nil, nil, info, handler)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Test error stream
	handlerErr := func(srv interface{}, ss grpc.ServerStream) error {
		return fmt.Errorf("stream error")
	}
	err = interceptor(nil, nil, info, handlerErr)
	if err == nil {
		t.Error("expected error")
	}
}
