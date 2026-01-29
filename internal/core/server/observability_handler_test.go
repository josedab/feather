package server

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/feather-store/feather/internal/core/storage"
)

// testObservabilityServer wraps an ObservabilityHandler for testing.
type testObservabilityServer struct {
	handler *ObservabilityHandler
	mux     *http.ServeMux
	t       *testing.T
}

// newTestObservabilityServer creates a new test observability server.
func newTestObservabilityServer(t *testing.T) *testObservabilityServer {
	t.Helper()

	store, err := storage.NewStore(context.Background(), storage.StoreOptions{
		HotMaxSize:   1024 * 1024, // 1MB
		WarmInMemory: true,
	}, storage.NewRegistry())
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	handler := NewObservabilityHandler(store)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	return &testObservabilityServer{
		handler: handler,
		mux:     mux,
		t:       t,
	}
}

func (ts *testObservabilityServer) request(method, path string, body string) *httptest.ResponseRecorder {
	ts.t.Helper()

	var bodyReader io.Reader
	if body != "" {
		bodyReader = strings.NewReader(body)
	}

	req := httptest.NewRequest(method, path, bodyReader)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}

	rr := httptest.NewRecorder()
	ts.mux.ServeHTTP(rr, req)
	return rr
}

func (ts *testObservabilityServer) postJSON(path string, body interface{}) *httptest.ResponseRecorder {
	ts.t.Helper()

	jsonBody, err := json.Marshal(body)
	if err != nil {
		ts.t.Fatalf("failed to marshal body: %v", err)
	}
	return ts.request(http.MethodPost, path, string(jsonBody))
}

func (ts *testObservabilityServer) get(path string) *httptest.ResponseRecorder {
	return ts.request(http.MethodGet, path, "")
}

func TestObservabilityHandler_NewObservabilityHandler(t *testing.T) {
	store, err := storage.NewStore(context.Background(), storage.StoreOptions{
		HotMaxSize:   1024 * 1024,
		WarmInMemory: true,
	}, storage.NewRegistry())
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	handler := NewObservabilityHandler(store)

	if handler.stack == nil {
		t.Error("Expected stack to be set")
	}
	if handler.store == nil {
		t.Error("Expected store to be set")
	}
}

func TestObservabilityHandler_GetStack(t *testing.T) {
	store, err := storage.NewStore(context.Background(), storage.StoreOptions{
		HotMaxSize:   1024 * 1024,
		WarmInMemory: true,
	}, storage.NewRegistry())
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	handler := NewObservabilityHandler(store)

	stack := handler.GetStack()
	if stack == nil {
		t.Error("Expected GetStack to return non-nil")
	}
}

func TestObservabilityHandler_GetMetrics(t *testing.T) {
	ts := newTestObservabilityServer(t)

	rr := ts.get("/v1/observability/metrics")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if result["metrics"] == nil {
		t.Error("Expected metrics in response")
	}
}

func TestObservabilityHandler_GetFeatureMetrics_NotFound(t *testing.T) {
	ts := newTestObservabilityServer(t)

	rr := ts.get("/v1/observability/metrics/nonexistent")

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, rr.Code)
	}
}

func TestObservabilityHandler_GetTopFeatures(t *testing.T) {
	ts := newTestObservabilityServer(t)

	rr := ts.get("/v1/observability/metrics/top")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}
}

func TestObservabilityHandler_GetTopFeatures_WithN(t *testing.T) {
	ts := newTestObservabilityServer(t)

	rr := ts.get("/v1/observability/metrics/top?n=5")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}
}

func TestObservabilityHandler_CheckFreshness(t *testing.T) {
	ts := newTestObservabilityServer(t)

	rr := ts.get("/v1/observability/freshness")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}
}

func TestObservabilityHandler_SetFreshnessThreshold(t *testing.T) {
	ts := newTestObservabilityServer(t)

	body := FreshnessThresholdRequest{
		Feature:   "test-feature",
		Threshold: "1h",
	}

	rr := ts.postJSON("/v1/observability/freshness/threshold", body)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d; body: %s", http.StatusOK, rr.Code, rr.Body.String())
	}
}

func TestObservabilityHandler_SetFreshnessThreshold_MissingFeature(t *testing.T) {
	ts := newTestObservabilityServer(t)

	body := FreshnessThresholdRequest{
		Threshold: "1h",
	}

	rr := ts.postJSON("/v1/observability/freshness/threshold", body)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestObservabilityHandler_SetFreshnessThreshold_InvalidThreshold(t *testing.T) {
	ts := newTestObservabilityServer(t)

	body := FreshnessThresholdRequest{
		Feature:   "test-feature",
		Threshold: "invalid",
	}

	rr := ts.postJSON("/v1/observability/freshness/threshold", body)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestObservabilityHandler_SetFreshnessThreshold_InvalidBody(t *testing.T) {
	ts := newTestObservabilityServer(t)

	rr := ts.request(http.MethodPost, "/v1/observability/freshness/threshold", "invalid json")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestObservabilityHandler_GetUsagePattern_NotFound(t *testing.T) {
	ts := newTestObservabilityServer(t)

	rr := ts.get("/v1/observability/usage/nonexistent")

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, rr.Code)
	}
}

func TestObservabilityHandler_GetQualityScore_NotFound(t *testing.T) {
	ts := newTestObservabilityServer(t)

	rr := ts.get("/v1/observability/quality/nonexistent")

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, rr.Code)
	}
}

func TestObservabilityHandler_AddQualityRule(t *testing.T) {
	ts := newTestObservabilityServer(t)

	body := QualityRuleRequest{
		Name:     "test-rule",
		Feature:  "test-feature",
		RuleType: "not_null",
		Severity: "warning",
	}

	rr := ts.postJSON("/v1/observability/quality/rules", body)

	if rr.Code != http.StatusCreated {
		t.Errorf("Expected status %d, got %d; body: %s", http.StatusCreated, rr.Code, rr.Body.String())
	}
}

func TestObservabilityHandler_AddQualityRule_MissingFields(t *testing.T) {
	ts := newTestObservabilityServer(t)

	body := QualityRuleRequest{
		Name: "test-rule",
	}

	rr := ts.postJSON("/v1/observability/quality/rules", body)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestObservabilityHandler_AddQualityRule_InvalidBody(t *testing.T) {
	ts := newTestObservabilityServer(t)

	rr := ts.request(http.MethodPost, "/v1/observability/quality/rules", "invalid json")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestObservabilityHandler_GetViolations(t *testing.T) {
	ts := newTestObservabilityServer(t)

	rr := ts.get("/v1/observability/quality/violations")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}
}

func TestObservabilityHandler_GetViolations_WithFilters(t *testing.T) {
	ts := newTestObservabilityServer(t)

	rr := ts.get("/v1/observability/quality/violations?feature=test&since=2024-01-01T00:00:00Z")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}
}

func TestObservabilityHandler_GetAlerts(t *testing.T) {
	ts := newTestObservabilityServer(t)

	rr := ts.get("/v1/observability/alerts")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}
}

func TestObservabilityHandler_GetAlerts_WithFilters(t *testing.T) {
	ts := newTestObservabilityServer(t)

	rr := ts.get("/v1/observability/alerts?type=error&feature=test&since=2024-01-01T00:00:00Z")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}
}

func TestObservabilityHandler_AddAlertRule(t *testing.T) {
	ts := newTestObservabilityServer(t)

	body := AlertRuleRequest{
		Name:      "test-alert",
		Type:      "latency",
		Feature:   "test-feature",
		Condition: "gt",
		Threshold: 100,
		Severity:  "warning",
	}

	rr := ts.postJSON("/v1/observability/alerts/rules", body)

	if rr.Code != http.StatusCreated {
		t.Errorf("Expected status %d, got %d; body: %s", http.StatusCreated, rr.Code, rr.Body.String())
	}
}

func TestObservabilityHandler_AddAlertRule_MissingFields(t *testing.T) {
	ts := newTestObservabilityServer(t)

	body := AlertRuleRequest{
		Name: "test-alert",
	}

	rr := ts.postJSON("/v1/observability/alerts/rules", body)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestObservabilityHandler_AddAlertRule_InvalidDuration(t *testing.T) {
	ts := newTestObservabilityServer(t)

	body := AlertRuleRequest{
		Name:      "test-alert",
		Type:      "latency",
		Feature:   "test-feature",
		Condition: "gt",
		Threshold: 100,
		Duration:  "invalid",
	}

	rr := ts.postJSON("/v1/observability/alerts/rules", body)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestObservabilityHandler_AddAlertRule_InvalidCooldown(t *testing.T) {
	ts := newTestObservabilityServer(t)

	body := AlertRuleRequest{
		Name:      "test-alert",
		Type:      "latency",
		Feature:   "test-feature",
		Condition: "gt",
		Threshold: 100,
		Cooldown:  "invalid",
	}

	rr := ts.postJSON("/v1/observability/alerts/rules", body)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestObservabilityHandler_AddAlertRule_InvalidBody(t *testing.T) {
	ts := newTestObservabilityServer(t)

	rr := ts.request(http.MethodPost, "/v1/observability/alerts/rules", "invalid json")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestObservabilityHandler_AckAlert_NotFound(t *testing.T) {
	ts := newTestObservabilityServer(t)

	rr := ts.postJSON("/v1/observability/alerts/nonexistent/ack", struct{}{})

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, rr.Code)
	}
}

func TestObservabilityHandler_ResolveAlert_NotFound(t *testing.T) {
	ts := newTestObservabilityServer(t)

	rr := ts.postJSON("/v1/observability/alerts/nonexistent/resolve", struct{}{})

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, rr.Code)
	}
}
