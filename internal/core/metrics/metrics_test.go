package metrics

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	io_prometheus_client "github.com/prometheus/client_model/go"
	"google.golang.org/grpc"
)

// newTestMetrics creates Metrics with an isolated registry for testing.
func newTestMetrics(t *testing.T) (*Metrics, *prometheus.Registry) {
	t.Helper()
	reg := prometheus.NewRegistry()
	m := NewMetricsWithRegistry("test", reg)
	return m, reg
}

// getMetricValue gathers all metrics from the registry and returns the value
// of the first metric matching the given name substring.
func getMetricValue(t *testing.T, reg *prometheus.Registry, nameSubstr string) *io_prometheus_client.MetricFamily {
	t.Helper()
	families, err := reg.Gather()
	if err != nil {
		t.Fatalf("failed to gather metrics: %v", err)
	}
	for _, f := range families {
		if strings.Contains(f.GetName(), nameSubstr) {
			return f
		}
	}
	return nil
}

func TestNewMetrics(t *testing.T) {
	m := NewMetrics("feather_test_new")
	if m == nil {
		t.Fatal("NewMetrics returned nil")
	}
}

func TestNewMetricsWithRegistry(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := NewMetricsWithRegistry("custom", reg)
	if m == nil {
		t.Fatal("NewMetricsWithRegistry returned nil")
	}
}

func TestMetrics_RecordHTTPRequest_Values(t *testing.T) {
	m, reg := newTestMetrics(t)

	m.RecordHTTPRequest("GET", "/v1/features", http.StatusOK)
	m.RecordHTTPRequest("GET", "/v1/features", http.StatusOK)
	m.RecordHTTPRequest("POST", "/v1/features", http.StatusCreated)

	fam := getMetricValue(t, reg, "http_requests_total")
	if fam == nil {
		t.Fatal("http_requests_total metric not found")
	}

	// Should have recorded 3 total across all label combinations
	var total float64
	for _, m := range fam.GetMetric() {
		total += m.GetCounter().GetValue()
	}
	if total != 3 {
		t.Errorf("total HTTP requests = %v, want 3", total)
	}
}

func TestMetrics_RecordHTTPLatency_Values(t *testing.T) {
	m, reg := newTestMetrics(t)

	m.RecordHTTPLatency("GET", "/v1/features", 100*time.Millisecond)
	m.RecordHTTPLatency("GET", "/v1/features", 200*time.Millisecond)

	fam := getMetricValue(t, reg, "http_request_duration_seconds")
	if fam == nil {
		t.Fatal("http_request_duration_seconds metric not found")
	}

	metric := fam.GetMetric()[0]
	if metric.GetHistogram().GetSampleCount() != 2 {
		t.Errorf("sample count = %d, want 2", metric.GetHistogram().GetSampleCount())
	}
	sum := metric.GetHistogram().GetSampleSum()
	if sum < 0.29 || sum > 0.31 {
		t.Errorf("sample sum = %v, want ~0.3", sum)
	}
}

func TestMetrics_RecordGRPCRequest_Values(t *testing.T) {
	m, reg := newTestMetrics(t)

	m.RecordGRPCRequest("/feather.v1/GetFeatures", "OK")
	m.RecordGRPCRequest("/feather.v1/GetFeatures", "OK")
	m.RecordGRPCRequest("/feather.v1/PutFeatures", "ERROR")

	fam := getMetricValue(t, reg, "grpc_requests_total")
	if fam == nil {
		t.Fatal("grpc_requests_total metric not found")
	}

	var total float64
	for _, m := range fam.GetMetric() {
		total += m.GetCounter().GetValue()
	}
	if total != 3 {
		t.Errorf("total gRPC requests = %v, want 3", total)
	}
}

func TestMetrics_CacheHitMiss_Values(t *testing.T) {
	m, reg := newTestMetrics(t)

	m.RecordCacheHit()
	m.RecordCacheHit()
	m.RecordCacheHit()
	m.RecordCacheMiss()

	hitFam := getMetricValue(t, reg, "cache_hits_total")
	if hitFam == nil {
		t.Fatal("cache_hits_total metric not found")
	}
	hits := hitFam.GetMetric()[0].GetCounter().GetValue()
	if hits != 3 {
		t.Errorf("cache hits = %v, want 3", hits)
	}

	missFam := getMetricValue(t, reg, "cache_misses_total")
	if missFam == nil {
		t.Fatal("cache_misses_total metric not found")
	}
	misses := missFam.GetMetric()[0].GetCounter().GetValue()
	if misses != 1 {
		t.Errorf("cache misses = %v, want 1", misses)
	}
}

func TestMetrics_SetGauges_Values(t *testing.T) {
	m, reg := newTestMetrics(t)

	m.SetHotTierSize(1024 * 1024)
	m.SetWarmTierSize(1024 * 1024 * 1024)
	m.SetEntityCount(5000)

	hotFam := getMetricValue(t, reg, "hot_tier_bytes")
	if hotFam == nil {
		t.Fatal("hot_tier_bytes metric not found")
	}
	if v := hotFam.GetMetric()[0].GetGauge().GetValue(); v != float64(1024*1024) {
		t.Errorf("hot tier size = %v, want %v", v, 1024*1024)
	}

	warmFam := getMetricValue(t, reg, "warm_tier_bytes")
	if warmFam == nil {
		t.Fatal("warm_tier_bytes metric not found")
	}
	if v := warmFam.GetMetric()[0].GetGauge().GetValue(); v != float64(1024*1024*1024) {
		t.Errorf("warm tier size = %v, want %v", v, 1024*1024*1024)
	}

	entityFam := getMetricValue(t, reg, "entity_count")
	if entityFam == nil {
		t.Fatal("entity_count metric not found")
	}
	if v := entityFam.GetMetric()[0].GetGauge().GetValue(); v != 5000 {
		t.Errorf("entity count = %v, want 5000", v)
	}
}

func TestMetrics_RecordIngestion_Values(t *testing.T) {
	m, reg := newTestMetrics(t)

	m.RecordIngestion("kafka", true)
	m.RecordIngestion("kafka", true)
	m.RecordIngestion("http", false)

	recvFam := getMetricValue(t, reg, "ingestion_messages_received_total")
	if recvFam == nil {
		t.Fatal("ingestion_messages_received_total not found")
	}
	var totalRecv float64
	for _, metric := range recvFam.GetMetric() {
		totalRecv += metric.GetCounter().GetValue()
	}
	if totalRecv != 3 {
		t.Errorf("total received = %v, want 3", totalRecv)
	}

	procFam := getMetricValue(t, reg, "ingestion_messages_processed_total")
	if procFam == nil {
		t.Fatal("ingestion_messages_processed_total not found")
	}
	// Check we have both success and error status labels
	var successCount, errorCount float64
	for _, metric := range procFam.GetMetric() {
		for _, lp := range metric.GetLabel() {
			if lp.GetName() == "status" && lp.GetValue() == "success" {
				successCount += metric.GetCounter().GetValue()
			}
			if lp.GetName() == "status" && lp.GetValue() == "error" {
				errorCount += metric.GetCounter().GetValue()
			}
		}
	}
	if successCount != 2 {
		t.Errorf("success count = %v, want 2", successCount)
	}
	if errorCount != 1 {
		t.Errorf("error count = %v, want 1", errorCount)
	}
}

func TestMetrics_SetIngestionLag_Values(t *testing.T) {
	m, reg := newTestMetrics(t)

	m.SetIngestionLag(5 * time.Second)

	fam := getMetricValue(t, reg, "ingestion_lag_seconds")
	if fam == nil {
		t.Fatal("ingestion_lag_seconds not found")
	}
	if v := fam.GetMetric()[0].GetGauge().GetValue(); v != 5.0 {
		t.Errorf("ingestion lag = %v, want 5.0", v)
	}
}

func TestMetrics_RecordFeatureRequest_Values(t *testing.T) {
	m, reg := newTestMetrics(t)

	m.RecordFeatureRequest("age")
	m.RecordFeatureRequest("age")
	m.RecordFeatureRequest("income")

	fam := getMetricValue(t, reg, "feature_requests_total")
	if fam == nil {
		t.Fatal("feature_requests_total not found")
	}
	var total float64
	for _, metric := range fam.GetMetric() {
		total += metric.GetCounter().GetValue()
	}
	if total != 3 {
		t.Errorf("total feature requests = %v, want 3", total)
	}
}

func TestMetrics_RecordAggregationCompute_Values(t *testing.T) {
	m, reg := newTestMetrics(t)

	m.RecordAggregationCompute("purchase_count", 5*time.Millisecond)
	m.RecordAggregationCompute("purchase_count", 3*time.Millisecond)

	fam := getMetricValue(t, reg, "aggregation_compute_seconds")
	if fam == nil {
		t.Fatal("aggregation_compute_seconds not found")
	}
	hist := fam.GetMetric()[0].GetHistogram()
	if hist.GetSampleCount() != 2 {
		t.Errorf("sample count = %d, want 2", hist.GetSampleCount())
	}
	sum := hist.GetSampleSum()
	if sum < 0.007 || sum > 0.009 {
		t.Errorf("sample sum = %v, want ~0.008", sum)
	}
}

func TestMetrics_RecordWarmTierOp_Values(t *testing.T) {
	m, reg := newTestMetrics(t)

	m.RecordWarmTierOp("get", 10*time.Millisecond)
	m.RecordWarmTierOp("put", 20*time.Millisecond)

	fam := getMetricValue(t, reg, "warm_tier_operation_seconds")
	if fam == nil {
		t.Fatal("warm_tier_operation_seconds not found")
	}
	var totalSamples uint64
	for _, metric := range fam.GetMetric() {
		totalSamples += metric.GetHistogram().GetSampleCount()
	}
	if totalSamples != 2 {
		t.Errorf("total samples = %d, want 2", totalSamples)
	}
}

func TestMetrics_RecordEviction_Values(t *testing.T) {
	m, reg := newTestMetrics(t)

	m.RecordEviction("hot", "size_limit")
	m.RecordEviction("hot", "size_limit")
	m.RecordEviction("hot", "ttl")

	fam := getMetricValue(t, reg, "evictions_total")
	if fam == nil {
		t.Fatal("evictions_total not found")
	}
	var total float64
	for _, metric := range fam.GetMetric() {
		total += metric.GetCounter().GetValue()
	}
	if total != 3 {
		t.Errorf("total evictions = %v, want 3", total)
	}
}

func TestMetrics_RecordError_Values(t *testing.T) {
	m, reg := newTestMetrics(t)

	m.RecordError("storage", "not_found")
	m.RecordError("storage", "not_found")
	m.RecordError("ingestion", "decode_error")

	fam := getMetricValue(t, reg, "errors_total")
	if fam == nil {
		t.Fatal("errors_total not found")
	}
	var total float64
	for _, metric := range fam.GetMetric() {
		total += metric.GetCounter().GetValue()
	}
	if total != 3 {
		t.Errorf("total errors = %v, want 3", total)
	}
}

func TestMetrics_UpdateRuntimeMetrics_Values(t *testing.T) {
	m, reg := newTestMetrics(t)

	m.UpdateRuntimeMetrics()

	goroutineFam := getMetricValue(t, reg, "goroutines")
	if goroutineFam == nil {
		t.Fatal("goroutines metric not found")
	}
	v := goroutineFam.GetMetric()[0].GetGauge().GetValue()
	if v < 1 {
		t.Errorf("goroutine count = %v, want >= 1", v)
	}

	memFam := getMetricValue(t, reg, "memory_alloc_bytes")
	if memFam == nil {
		t.Fatal("memory_alloc_bytes not found")
	}
	mem := memFam.GetMetric()[0].GetGauge().GetValue()
	if mem <= 0 {
		t.Errorf("memory alloc = %v, want > 0", mem)
	}
}

func TestMetrics_StartRuntimeMetricsCollector(t *testing.T) {
	m, reg := newTestMetrics(t)

	stop := m.StartRuntimeMetricsCollector(50 * time.Millisecond)
	time.Sleep(150 * time.Millisecond)
	stop()

	// After collector ran, runtime metrics should be populated
	fam := getMetricValue(t, reg, "goroutines")
	if fam == nil {
		t.Fatal("goroutines metric not populated by collector")
	}
	if fam.GetMetric()[0].GetGauge().GetValue() < 1 {
		t.Error("expected goroutine count >= 1 after collector run")
	}
}

func TestMetrics_Handler(t *testing.T) {
	m, _ := newTestMetrics(t)

	handler := m.Handler()
	if handler == nil {
		t.Fatal("Handler() returned nil")
	}

	req := httptest.NewRequest("GET", "/metrics", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Handler returned status %d, want 200", rec.Code)
	}
}

func TestMetrics_UnaryServerInterceptor_Values(t *testing.T) {
	m, reg := newTestMetrics(t)
	interceptor := m.UnaryServerInterceptor()

	// Successful call
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

	// Error call
	handlerErr := func(ctx context.Context, req interface{}) (interface{}, error) {
		return nil, fmt.Errorf("test error")
	}
	_, _ = interceptor(context.Background(), "request", info, handlerErr)

	// Verify metrics recorded
	reqFam := getMetricValue(t, reg, "grpc_requests_total")
	if reqFam == nil {
		t.Fatal("grpc_requests_total not found after interceptor")
	}
	var okCount, errCount float64
	for _, metric := range reqFam.GetMetric() {
		for _, lp := range metric.GetLabel() {
			if lp.GetName() == "status" && lp.GetValue() == "OK" {
				okCount += metric.GetCounter().GetValue()
			}
			if lp.GetName() == "status" && lp.GetValue() == "ERROR" {
				errCount += metric.GetCounter().GetValue()
			}
		}
	}
	if okCount != 1 {
		t.Errorf("OK count = %v, want 1", okCount)
	}
	if errCount != 1 {
		t.Errorf("ERROR count = %v, want 1", errCount)
	}

	durFam := getMetricValue(t, reg, "grpc_request_duration_seconds")
	if durFam == nil {
		t.Fatal("grpc_request_duration_seconds not found")
	}
	var totalSamples uint64
	for _, metric := range durFam.GetMetric() {
		totalSamples += metric.GetHistogram().GetSampleCount()
	}
	if totalSamples != 2 {
		t.Errorf("duration samples = %d, want 2", totalSamples)
	}
}

func TestMetrics_StreamServerInterceptor_Values(t *testing.T) {
	m, reg := newTestMetrics(t)
	interceptor := m.StreamServerInterceptor()

	handler := func(srv interface{}, ss grpc.ServerStream) error {
		return nil
	}
	info := &grpc.StreamServerInfo{FullMethod: "/test.Service/StreamMethod"}
	err := interceptor(nil, nil, info, handler)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	handlerErr := func(srv interface{}, ss grpc.ServerStream) error {
		return fmt.Errorf("stream error")
	}
	_ = interceptor(nil, nil, info, handlerErr)

	reqFam := getMetricValue(t, reg, "grpc_requests_total")
	if reqFam == nil {
		t.Fatal("grpc_requests_total not found after stream interceptor")
	}
	var total float64
	for _, metric := range reqFam.GetMetric() {
		total += metric.GetCounter().GetValue()
	}
	if total != 2 {
		t.Errorf("total gRPC requests = %v, want 2", total)
	}
}

func TestMetrics_SetFeatureFreshness_Values(t *testing.T) {
	m, reg := newTestMetrics(t)

	m.SetFeatureFreshness("user_features", 30*time.Second)

	fam := getMetricValue(t, reg, "feature_freshness_seconds")
	if fam == nil {
		t.Fatal("feature_freshness_seconds not found")
	}
	if v := fam.GetMetric()[0].GetGauge().GetValue(); v != 30.0 {
		t.Errorf("freshness = %v, want 30.0", v)
	}
}

func TestMetrics_RecordShardWait_Values(t *testing.T) {
	m, reg := newTestMetrics(t)

	m.RecordShardWait("0", 100*time.Microsecond)

	fam := getMetricValue(t, reg, "shard_wait_seconds")
	if fam == nil {
		t.Fatal("shard_wait_seconds not found")
	}
	if fam.GetMetric()[0].GetHistogram().GetSampleCount() != 1 {
		t.Error("expected 1 sample")
	}
}
