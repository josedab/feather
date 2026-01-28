package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// testDashboardServer wraps a DashboardHandler for testing.
type testDashboardServer struct {
	handler *DashboardHandler
	mux     *http.ServeMux
	t       *testing.T
}

// newTestDashboardServer creates a new test dashboard server.
func newTestDashboardServer(t *testing.T) *testDashboardServer {
	t.Helper()

	handler := NewDashboardHandler(nil, nil)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	return &testDashboardServer{
		handler: handler,
		mux:     mux,
		t:       t,
	}
}

func (ts *testDashboardServer) get(path string) *httptest.ResponseRecorder {
	ts.t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rr := httptest.NewRecorder()
	ts.mux.ServeHTTP(rr, req)
	return rr
}

func TestDashboardOverview(t *testing.T) {
	ts := newTestDashboardServer(t)

	// Record some operations
	ts.handler.overview.RecordReadLatency(500)
	ts.handler.overview.RecordWriteLatency(1000)

	rr := ts.get("/v1/dashboard/overview")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d; body: %s", http.StatusOK, rr.Code, rr.Body.String())
	}

	var result overviewResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v; body: %s", err, rr.Body.String())
	}

	if result.UptimeSeconds <= 0 {
		t.Error("Expected positive uptime")
	}
	if result.Uptime == "" {
		t.Error("Expected non-empty uptime string")
	}
}

func TestDashboardHealth(t *testing.T) {
	ts := newTestDashboardServer(t)

	rr := ts.get("/v1/dashboard/health")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d; body: %s", http.StatusOK, rr.Code, rr.Body.String())
	}

	var result healthResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if len(result.Components) == 0 {
		t.Error("Expected at least one component")
	}

	// With nil store, hot_tier and warm_tier should be unavailable
	foundHot := false
	for _, c := range result.Components {
		if c.Name == "hot_tier" {
			foundHot = true
			if c.Status != "unavailable" {
				t.Errorf("Expected hot_tier status 'unavailable', got %q", c.Status)
			}
		}
	}
	if !foundHot {
		t.Error("Expected hot_tier component in health check")
	}

	if result.Status != "degraded" {
		t.Errorf("Expected overall status 'degraded' (nil store), got %q", result.Status)
	}
}

func TestDashboardFeatureStats(t *testing.T) {
	ts := newTestDashboardServer(t)

	// Seed some feature stats
	ts.handler.overview.featureStats.mu.Lock()
	ts.handler.overview.featureStats.TotalFeatures = 42
	ts.handler.overview.featureStats.ByType["float64"] = 20
	ts.handler.overview.featureStats.ByType["int64"] = 22
	ts.handler.overview.featureStats.ByGroup["user_features"] = 30
	ts.handler.overview.featureStats.ByGroup["item_features"] = 12
	ts.handler.overview.featureStats.StaleCount = 3
	ts.handler.overview.featureStats.mu.Unlock()

	rr := ts.get("/v1/dashboard/features/stats")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}

	var result featureStatsResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if result.TotalFeatures != 42 {
		t.Errorf("Expected 42 total features, got %d", result.TotalFeatures)
	}
	if result.ByType["float64"] != 20 {
		t.Errorf("Expected 20 float64 features, got %d", result.ByType["float64"])
	}
	if result.StaleCount != 3 {
		t.Errorf("Expected 3 stale features, got %d", result.StaleCount)
	}
}

func TestDashboardLatency(t *testing.T) {
	ts := newTestDashboardServer(t)

	// Record various latencies
	for i := 0; i < 100; i++ {
		ts.handler.overview.RecordReadLatency(50)    // <100us bucket
		ts.handler.overview.RecordWriteLatency(2000) // <5000us bucket
	}
	// Record a few high latencies
	for i := 0; i < 5; i++ {
		ts.handler.overview.RecordReadLatency(60000) // <100000us bucket
	}

	rr := ts.get("/v1/dashboard/latency")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}

	var result latencyResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if result.Reads.Count != 105 {
		t.Errorf("Expected read count 105, got %d", result.Reads.Count)
	}
	if result.Writes.Count != 100 {
		t.Errorf("Expected write count 100, got %d", result.Writes.Count)
	}
	if result.Reads.P50 <= 0 {
		t.Error("Expected positive P50 for reads")
	}
}

func TestDashboardThroughput(t *testing.T) {
	ts := newTestDashboardServer(t)

	// Record some throughput
	for i := 0; i < 10; i++ {
		ts.handler.overview.RecordRead()
	}
	for i := 0; i < 5; i++ {
		ts.handler.overview.RecordWrite()
	}

	// Take a snapshot
	ts.handler.overview.SnapshotThroughput()

	// Record more
	for i := 0; i < 3; i++ {
		ts.handler.overview.RecordRead()
	}

	rr := ts.get("/v1/dashboard/throughput")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	intervals, ok := result["intervals"].([]interface{})
	if !ok {
		t.Fatal("Expected intervals array in response")
	}
	if len(intervals) != 1 {
		t.Errorf("Expected 1 interval, got %d", len(intervals))
	}

	currentReads := result["current_reads"].(float64)
	if currentReads != 3 {
		t.Errorf("Expected 3 current reads, got %v", currentReads)
	}
}

func TestDashboardStorage(t *testing.T) {
	ts := newTestDashboardServer(t)

	rr := ts.get("/v1/dashboard/storage")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}

	var result storageResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	// With nil store, sizes should be 0
	if result.HotTier.SizeBytes != 0 {
		t.Errorf("Expected 0 hot tier size, got %d", result.HotTier.SizeBytes)
	}
}

func TestDashboardAlerts(t *testing.T) {
	ts := newTestDashboardServer(t)

	// Add some alerts
	now := time.Now()
	ts.handler.overview.AddAlert(&DashboardAlert{
		ID:        "alert-1",
		Severity:  "warning",
		Title:     "High latency detected",
		Message:   "P99 latency exceeded 10ms",
		Source:    "latency_monitor",
		Timestamp: now.Add(-5 * time.Minute),
	})
	ts.handler.overview.AddAlert(&DashboardAlert{
		ID:        "alert-2",
		Severity:  "critical",
		Title:     "Drift detected",
		Message:   "Feature click_rate has drifted",
		Source:    "drift_detector",
		Timestamp: now,
	})

	rr := ts.get("/v1/dashboard/alerts/recent")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	alerts, ok := result["alerts"].([]interface{})
	if !ok {
		t.Fatal("Expected alerts array in response")
	}
	if len(alerts) != 2 {
		t.Errorf("Expected 2 alerts, got %d", len(alerts))
	}

	// Most recent should be first
	first := alerts[0].(map[string]interface{})
	if first["id"] != "alert-2" {
		t.Errorf("Expected most recent alert first, got id=%v", first["id"])
	}
}

func TestDashboardAlerts_WithLimit(t *testing.T) {
	ts := newTestDashboardServer(t)

	for i := 0; i < 10; i++ {
		ts.handler.overview.AddAlert(&DashboardAlert{
			ID:        "alert-" + time.Now().Format("150405.000"),
			Severity:  "info",
			Title:     "Test alert",
			Source:    "test",
			Timestamp: time.Now(),
		})
	}

	rr := ts.get("/v1/dashboard/alerts/recent?limit=3")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	alerts := result["alerts"].([]interface{})
	if len(alerts) != 3 {
		t.Errorf("Expected 3 alerts with limit, got %d", len(alerts))
	}
}

func TestDashboardTimeline(t *testing.T) {
	ts := newTestDashboardServer(t)

	// Add some events
	ts.handler.overview.AddTimelineEvent(&TimelineEvent{
		ID:        "evt-1",
		Type:      "feature_update",
		Message:   "Feature user_clicks updated",
		Timestamp: time.Now().Add(-10 * time.Minute),
	})
	ts.handler.overview.AddTimelineEvent(&TimelineEvent{
		ID:        "evt-2",
		Type:      "schema_change",
		Message:   "New feature group registered",
		Timestamp: time.Now(),
	})

	rr := ts.get("/v1/dashboard/timeline")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	events, ok := result["events"].([]interface{})
	if !ok {
		t.Fatal("Expected events array in response")
	}
	if len(events) != 2 {
		t.Errorf("Expected 2 events, got %d", len(events))
	}

	// Most recent should be first
	first := events[0].(map[string]interface{})
	if first["id"] != "evt-2" {
		t.Errorf("Expected most recent event first, got id=%v", first["id"])
	}
}

func TestDashboardTimeline_WithLimit(t *testing.T) {
	ts := newTestDashboardServer(t)

	for i := 0; i < 10; i++ {
		ts.handler.overview.AddTimelineEvent(&TimelineEvent{
			ID:        "evt-" + time.Now().Format("150405.000"),
			Type:      "test",
			Message:   "Test event",
			Timestamp: time.Now(),
		})
	}

	rr := ts.get("/v1/dashboard/timeline?limit=5")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	events := result["events"].([]interface{})
	if len(events) != 5 {
		t.Errorf("Expected 5 events with limit, got %d", len(events))
	}
}

func TestDashboardDriftSummary(t *testing.T) {
	ts := newTestDashboardServer(t)

	rr := ts.get("/v1/dashboard/drift/summary")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}

	var result driftSummaryResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if result.MonitoredFeatures != 0 {
		t.Errorf("Expected 0 monitored features, got %d", result.MonitoredFeatures)
	}
}

func TestDashboardFreshnessSummary(t *testing.T) {
	ts := newTestDashboardServer(t)

	// Set some feature stats
	ts.handler.overview.featureStats.mu.Lock()
	ts.handler.overview.featureStats.TotalFeatures = 10
	ts.handler.overview.featureStats.StaleCount = 2
	ts.handler.overview.featureStats.mu.Unlock()

	rr := ts.get("/v1/dashboard/freshness/summary")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}

	var result freshnessSummaryResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if result.TotalMonitored != 10 {
		t.Errorf("Expected 10 monitored, got %d", result.TotalMonitored)
	}
	if result.Fresh != 8 {
		t.Errorf("Expected 8 fresh, got %d", result.Fresh)
	}
	if result.Stale != 2 {
		t.Errorf("Expected 2 stale, got %d", result.Stale)
	}
	if result.SLACompliance != 0.8 {
		t.Errorf("Expected 0.8 SLA compliance, got %f", result.SLACompliance)
	}
}

func TestLatencyHistogram_Record(t *testing.T) {
	h := NewLatencyHistogram()

	h.Record(50)
	h.Record(200)
	h.Record(5000)

	count, sum := h.Stats()
	if count != 3 {
		t.Errorf("Expected count 3, got %d", count)
	}
	if sum != 5250 {
		t.Errorf("Expected sum 5250, got %d", sum)
	}
}

func TestLatencyHistogram_Percentile(t *testing.T) {
	h := NewLatencyHistogram()

	// All values in the first bucket (<100us)
	for i := 0; i < 100; i++ {
		h.Record(50)
	}

	p50 := h.Percentile(50)
	if p50 != 100 {
		t.Errorf("Expected P50=100 (bucket bound), got %d", p50)
	}

	p99 := h.Percentile(99)
	if p99 != 100 {
		t.Errorf("Expected P99=100 (bucket bound), got %d", p99)
	}
}

func TestLatencyHistogram_PercentileEmpty(t *testing.T) {
	h := NewLatencyHistogram()

	p50 := h.Percentile(50)
	if p50 != 0 {
		t.Errorf("Expected P50=0 for empty histogram, got %d", p50)
	}
}

func TestDashboardTopFeatures(t *testing.T) {
	ts := newTestDashboardServer(t)

	ts.handler.overview.featureStats.mu.Lock()
	ts.handler.overview.featureStats.ByGroup["clicks"] = 100
	ts.handler.overview.featureStats.ByGroup["views"] = 200
	ts.handler.overview.featureStats.ByGroup["purchases"] = 50
	ts.handler.overview.featureStats.mu.Unlock()

	rr := ts.get("/v1/dashboard/features/top?limit=2")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	features, ok := result["top_features"].([]interface{})
	if !ok {
		t.Fatal("Expected top_features array")
	}
	if len(features) != 2 {
		t.Errorf("Expected 2 top features, got %d", len(features))
	}

	// First should be most accessed
	first := features[0].(map[string]interface{})
	if first["name"] != "views" {
		t.Errorf("Expected top feature 'views', got %v", first["name"])
	}
}

func TestNewDashboardHandler(t *testing.T) {
	handler := NewDashboardHandler(nil, nil)

	if handler.overview == nil {
		t.Error("Expected overview to be initialized")
	}
	if handler.overview.latencyTracker == nil {
		t.Error("Expected latency tracker to be initialized")
	}
	if handler.overview.throughput == nil {
		t.Error("Expected throughput tracker to be initialized")
	}
}
