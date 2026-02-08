package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// testImpactServer wraps an ImpactHandler for testing.
type testImpactServer struct {
	handler *ImpactHandler
	mux     *http.ServeMux
	t       *testing.T
}

// newTestImpactServer creates a new test impact server.
func newTestImpactServer(t *testing.T) *testImpactServer {
	t.Helper()

	handler := NewImpactHandler()
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	return &testImpactServer{
		handler: handler,
		mux:     mux,
		t:       t,
	}
}

func (ts *testImpactServer) request(method, path string, body string) *httptest.ResponseRecorder {
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

func (ts *testImpactServer) postJSON(path string, body interface{}) *httptest.ResponseRecorder {
	ts.t.Helper()

	jsonBody, err := json.Marshal(body)
	if err != nil {
		ts.t.Fatalf("failed to marshal body: %v", err)
	}
	return ts.request(http.MethodPost, path, string(jsonBody))
}

func (ts *testImpactServer) get(path string) *httptest.ResponseRecorder {
	return ts.request(http.MethodGet, path, "")
}

func TestImpactHandler_NewImpactHandler(t *testing.T) {
	handler := NewImpactHandler()

	if handler.tracker == nil {
		t.Error("Expected tracker to be set")
	}
}

func TestImpactHandler_GetTracker(t *testing.T) {
	handler := NewImpactHandler()

	tracker := handler.GetTracker()
	if tracker == nil {
		t.Error("Expected GetTracker to return non-nil")
	}
}

func TestImpactHandler_RecordAccess(t *testing.T) {
	ts := newTestImpactServer(t)

	body := ImpactAccessRequest{
		Feature:   "test-feature",
		LatencyMs: 10.5,
		IsError:   false,
		IsNull:    false,
	}

	rr := ts.postJSON("/v1/impact/access", body)

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

func TestImpactHandler_RecordAccess_MissingFeature(t *testing.T) {
	ts := newTestImpactServer(t)

	body := ImpactAccessRequest{
		LatencyMs: 10.5,
	}

	rr := ts.postJSON("/v1/impact/access", body)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestImpactHandler_RecordAccess_InvalidBody(t *testing.T) {
	ts := newTestImpactServer(t)

	rr := ts.request(http.MethodPost, "/v1/impact/access", "invalid json")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestImpactHandler_GetFeatures(t *testing.T) {
	ts := newTestImpactServer(t)

	rr := ts.get("/v1/impact/features")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if result["features"] == nil {
		t.Error("Expected features in response")
	}
}

func TestImpactHandler_GetFeatureUsage(t *testing.T) {
	ts := newTestImpactServer(t)

	// Record access first
	ts.postJSON("/v1/impact/access", ImpactAccessRequest{
		Feature:   "usage-feature",
		LatencyMs: 5.0,
	})

	rr := ts.get("/v1/impact/features/usage-feature")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d; body: %s", http.StatusOK, rr.Code, rr.Body.String())
	}
}

func TestImpactHandler_GetFeatureUsage_NotFound(t *testing.T) {
	ts := newTestImpactServer(t)

	rr := ts.get("/v1/impact/features/nonexistent")

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, rr.Code)
	}
}

func TestImpactHandler_SetDependencies(t *testing.T) {
	ts := newTestImpactServer(t)

	body := SetDependenciesRequest{
		Dependencies: []string{"dep-1", "dep-2"},
	}

	rr := ts.postJSON("/v1/impact/features/test-feature/dependencies", body)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d; body: %s", http.StatusOK, rr.Code, rr.Body.String())
	}
}

func TestImpactHandler_SetDependencies_InvalidBody(t *testing.T) {
	ts := newTestImpactServer(t)

	rr := ts.request(http.MethodPost, "/v1/impact/features/test/dependencies", "invalid json")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestImpactHandler_RegisterModel(t *testing.T) {
	ts := newTestImpactServer(t)

	body := RegisterModelRequest{
		ModelID:      "model-1",
		ModelVersion: "v1.0",
		Features:     []string{"feature-1", "feature-2"},
		Environment:  "production",
	}

	rr := ts.postJSON("/v1/impact/models", body)

	if rr.Code != http.StatusCreated {
		t.Errorf("Expected status %d, got %d; body: %s", http.StatusCreated, rr.Code, rr.Body.String())
	}
}

func TestImpactHandler_RegisterModel_MissingFields(t *testing.T) {
	ts := newTestImpactServer(t)

	body := RegisterModelRequest{
		ModelID: "model-1",
	}

	rr := ts.postJSON("/v1/impact/models", body)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestImpactHandler_RegisterModel_InvalidBody(t *testing.T) {
	ts := newTestImpactServer(t)

	rr := ts.request(http.MethodPost, "/v1/impact/models", "invalid json")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestImpactHandler_RecordInference(t *testing.T) {
	ts := newTestImpactServer(t)

	// Register model first
	ts.postJSON("/v1/impact/models", RegisterModelRequest{
		ModelID:  "inference-model",
		Features: []string{"feature-1"},
	})

	body := RecordInferenceRequest{
		LatencyMs: 50.0,
		IsError:   false,
	}

	rr := ts.postJSON("/v1/impact/models/inference-model/inference", body)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d; body: %s", http.StatusOK, rr.Code, rr.Body.String())
	}
}

func TestImpactHandler_RecordInference_InvalidBody(t *testing.T) {
	ts := newTestImpactServer(t)

	rr := ts.request(http.MethodPost, "/v1/impact/models/test/inference", "invalid json")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestImpactHandler_GetModels(t *testing.T) {
	ts := newTestImpactServer(t)

	rr := ts.get("/v1/impact/models")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}
}

func TestImpactHandler_GetModelUsage(t *testing.T) {
	ts := newTestImpactServer(t)

	// Register model first
	ts.postJSON("/v1/impact/models", RegisterModelRequest{
		ModelID:  "get-model",
		Features: []string{"feature-1"},
	})

	rr := ts.get("/v1/impact/models/get-model")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d; body: %s", http.StatusOK, rr.Code, rr.Body.String())
	}
}

func TestImpactHandler_GetModelUsage_NotFound(t *testing.T) {
	ts := newTestImpactServer(t)

	rr := ts.get("/v1/impact/models/nonexistent")

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, rr.Code)
	}
}

func TestImpactHandler_GetImpactScore(t *testing.T) {
	ts := newTestImpactServer(t)

	// Record access first
	ts.postJSON("/v1/impact/access", ImpactAccessRequest{
		Feature:   "score-feature",
		LatencyMs: 5.0,
	})

	rr := ts.get("/v1/impact/scores/score-feature")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d; body: %s", http.StatusOK, rr.Code, rr.Body.String())
	}
}

func TestImpactHandler_GetImpactScore_NotFound(t *testing.T) {
	ts := newTestImpactServer(t)

	rr := ts.get("/v1/impact/scores/nonexistent")

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, rr.Code)
	}
}

func TestImpactHandler_GetTopFeatures(t *testing.T) {
	ts := newTestImpactServer(t)

	rr := ts.get("/v1/impact/scores")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}
}

func TestImpactHandler_GetTopFeatures_WithN(t *testing.T) {
	ts := newTestImpactServer(t)

	rr := ts.get("/v1/impact/scores?n=5")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}
}

func TestImpactHandler_GetLowImpact(t *testing.T) {
	ts := newTestImpactServer(t)

	rr := ts.get("/v1/impact/low-impact")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}
}

func TestImpactHandler_GetLowImpact_WithThreshold(t *testing.T) {
	ts := newTestImpactServer(t)

	rr := ts.get("/v1/impact/low-impact?threshold=10")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}
}

func TestImpactHandler_GetUnused(t *testing.T) {
	ts := newTestImpactServer(t)

	rr := ts.get("/v1/impact/unused")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}
}

func TestImpactHandler_GetUnused_WithDays(t *testing.T) {
	ts := newTestImpactServer(t)

	rr := ts.get("/v1/impact/unused?days=7")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}
}

func TestImpactHandler_GetLineage(t *testing.T) {
	ts := newTestImpactServer(t)

	// Record access first
	ts.postJSON("/v1/impact/access", ImpactAccessRequest{
		Feature:   "lineage-feature",
		LatencyMs: 5.0,
	})

	rr := ts.get("/v1/impact/lineage/lineage-feature")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d; body: %s", http.StatusOK, rr.Code, rr.Body.String())
	}
}

func TestImpactHandler_GetLineage_NotFound(t *testing.T) {
	ts := newTestImpactServer(t)

	rr := ts.get("/v1/impact/lineage/nonexistent")

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, rr.Code)
	}
}

func TestImpactHandler_GetGraph(t *testing.T) {
	ts := newTestImpactServer(t)

	rr := ts.get("/v1/impact/graph")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}
}

func TestImpactHandler_RequestDeprecation(t *testing.T) {
	ts := newTestImpactServer(t)

	// Record access first
	ts.postJSON("/v1/impact/access", ImpactAccessRequest{
		Feature:   "deprecate-feature",
		LatencyMs: 5.0,
	})

	body := DeprecationRequestBody{
		Feature:     "deprecate-feature",
		Reason:      "No longer needed",
		RequestedBy: "alice",
	}

	rr := ts.postJSON("/v1/impact/deprecations", body)

	if rr.Code != http.StatusCreated {
		t.Errorf("Expected status %d, got %d; body: %s", http.StatusCreated, rr.Code, rr.Body.String())
	}
}

func TestImpactHandler_RequestDeprecation_MissingFields(t *testing.T) {
	ts := newTestImpactServer(t)

	body := DeprecationRequestBody{
		Feature: "test-feature",
	}

	rr := ts.postJSON("/v1/impact/deprecations", body)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestImpactHandler_RequestDeprecation_InvalidBody(t *testing.T) {
	ts := newTestImpactServer(t)

	rr := ts.request(http.MethodPost, "/v1/impact/deprecations", "invalid json")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestImpactHandler_GetDeprecations(t *testing.T) {
	ts := newTestImpactServer(t)

	rr := ts.get("/v1/impact/deprecations")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}
}

func TestImpactHandler_GetDeprecation_NotFound(t *testing.T) {
	ts := newTestImpactServer(t)

	rr := ts.get("/v1/impact/deprecations/nonexistent")

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, rr.Code)
	}
}

func TestImpactHandler_ApproveDeprecation_NotFound(t *testing.T) {
	ts := newTestImpactServer(t)

	rr := ts.postJSON("/v1/impact/deprecations/nonexistent/approve", struct{}{})

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestImpactHandler_GetReport(t *testing.T) {
	ts := newTestImpactServer(t)

	rr := ts.get("/v1/impact/report")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}
}
