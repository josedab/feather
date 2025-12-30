package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/feather-store/feather/internal/drift"
)

// testDriftServer wraps a DriftHandler for testing.
type testDriftServer struct {
	handler  *DriftHandler
	detector *drift.Detector
	mux      *http.ServeMux
	t        *testing.T
}

// newTestDriftServer creates a new test drift server.
func newTestDriftServer(t *testing.T) *testDriftServer {
	t.Helper()

	detector := drift.NewDetector(drift.Config{})
	handler := NewDriftHandler(detector)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	return &testDriftServer{
		handler:  handler,
		detector: detector,
		mux:      mux,
		t:        t,
	}
}

// newTestDriftServerWithoutDetector creates a drift server without detector for testing nil case.
func newTestDriftServerWithoutDetector(t *testing.T) *testDriftServer {
	t.Helper()

	handler := NewDriftHandler(nil)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	return &testDriftServer{
		handler:  handler,
		detector: nil,
		mux:      mux,
		t:        t,
	}
}

func (ts *testDriftServer) request(method, path string, body string) *httptest.ResponseRecorder {
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

func (ts *testDriftServer) postJSON(path string, body interface{}) *httptest.ResponseRecorder {
	ts.t.Helper()

	jsonBody, err := json.Marshal(body)
	if err != nil {
		ts.t.Fatalf("failed to marshal body: %v", err)
	}
	return ts.request(http.MethodPost, path, string(jsonBody))
}

func (ts *testDriftServer) get(path string) *httptest.ResponseRecorder {
	return ts.request(http.MethodGet, path, "")
}

func TestDriftHandler_NewDriftHandler(t *testing.T) {
	detector := drift.NewDetector(drift.Config{})
	handler := NewDriftHandler(detector)

	if handler.detector == nil {
		t.Error("Expected detector to be set")
	}
}

func TestDriftHandler_GetStatus_Empty(t *testing.T) {
	ts := newTestDriftServer(t)

	rr := ts.get("/v1/drift/status")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}

	var result DriftStatusResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if result.Monitors == nil {
		t.Error("Expected monitors array in response")
	}
}

func TestDriftHandler_GetStatus_WithMonitors(t *testing.T) {
	ts := newTestDriftServer(t)

	// Register a feature
	ts.detector.RegisterFeature("click_rate", drift.TypeNumeric)

	rr := ts.get("/v1/drift/status")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}

	var result DriftStatusResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if len(result.Monitors) != 1 {
		t.Errorf("Expected 1 monitor, got %d", len(result.Monitors))
	}

	if result.Monitors[0].Feature != "click_rate" {
		t.Errorf("Expected feature 'click_rate', got %s", result.Monitors[0].Feature)
	}
}

func TestDriftHandler_GetStatus_NoDetector(t *testing.T) {
	ts := newTestDriftServerWithoutDetector(t)

	rr := ts.get("/v1/drift/status")

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
	}
}

func TestDriftHandler_GetAlerts(t *testing.T) {
	ts := newTestDriftServer(t)

	rr := ts.get("/v1/drift/alerts")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if result["alerts"] == nil {
		t.Error("Expected alerts array in response")
	}
}

func TestDriftHandler_GetAlerts_WithSince(t *testing.T) {
	ts := newTestDriftServer(t)

	rr := ts.get("/v1/drift/alerts?since=2024-01-01T00:00:00Z")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}
}

func TestDriftHandler_GetAlerts_NoDetector(t *testing.T) {
	ts := newTestDriftServerWithoutDetector(t)

	rr := ts.get("/v1/drift/alerts")

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
	}
}

func TestDriftHandler_RegisterFeature(t *testing.T) {
	ts := newTestDriftServer(t)

	body := RegisterFeatureRequest{
		Name: "conversion_rate",
		Type: "numeric",
	}

	rr := ts.postJSON("/v1/drift/register", body)

	if rr.Code != http.StatusCreated {
		t.Errorf("Expected status %d, got %d; body: %s", http.StatusCreated, rr.Code, rr.Body.String())
	}

	var result map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if result["success"] != true {
		t.Error("Expected success to be true")
	}
	if result["feature"] != "conversion_rate" {
		t.Errorf("Expected feature 'conversion_rate', got %v", result["feature"])
	}
}

func TestDriftHandler_RegisterFeature_Categorical(t *testing.T) {
	ts := newTestDriftServer(t)

	body := RegisterFeatureRequest{
		Name: "category",
		Type: "categorical",
	}

	rr := ts.postJSON("/v1/drift/register", body)

	if rr.Code != http.StatusCreated {
		t.Errorf("Expected status %d, got %d", http.StatusCreated, rr.Code)
	}
}

func TestDriftHandler_RegisterFeature_MissingName(t *testing.T) {
	ts := newTestDriftServer(t)

	body := RegisterFeatureRequest{
		Type: "numeric",
	}

	rr := ts.postJSON("/v1/drift/register", body)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestDriftHandler_RegisterFeature_InvalidBody(t *testing.T) {
	ts := newTestDriftServer(t)

	rr := ts.request(http.MethodPost, "/v1/drift/register", "invalid json")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestDriftHandler_RegisterFeature_NoDetector(t *testing.T) {
	ts := newTestDriftServerWithoutDetector(t)

	body := RegisterFeatureRequest{
		Name: "test",
		Type: "numeric",
	}

	rr := ts.postJSON("/v1/drift/register", body)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
	}
}

func TestDriftHandler_ResetReference(t *testing.T) {
	ts := newTestDriftServer(t)

	// Register a feature first
	ts.detector.RegisterFeature("test_feature", drift.TypeNumeric)

	rr := ts.postJSON("/v1/drift/reset/test_feature", nil)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d; body: %s", http.StatusOK, rr.Code, rr.Body.String())
	}

	var result map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if result["success"] != true {
		t.Error("Expected success to be true")
	}
}

func TestDriftHandler_ResetReference_NotFound(t *testing.T) {
	ts := newTestDriftServer(t)

	rr := ts.postJSON("/v1/drift/reset/nonexistent", nil)

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, rr.Code)
	}
}

func TestDriftHandler_ResetReference_NoDetector(t *testing.T) {
	ts := newTestDriftServerWithoutDetector(t)

	rr := ts.postJSON("/v1/drift/reset/test", nil)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
	}
}
