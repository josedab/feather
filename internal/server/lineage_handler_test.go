package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/feather-store/feather/internal/lineage"
)

// testLineageServer wraps a LineageHandler for testing.
type testLineageServer struct {
	handler *LineageHandler
	tracker *lineage.Tracker
	mux     *http.ServeMux
	t       *testing.T
}

// newTestLineageServer creates a new test lineage server.
func newTestLineageServer(t *testing.T) *testLineageServer {
	t.Helper()

	tracker := lineage.NewTracker()
	handler := NewLineageHandler(tracker)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	return &testLineageServer{
		handler: handler,
		tracker: tracker,
		mux:     mux,
		t:       t,
	}
}

// newTestLineageServerWithoutTracker creates a lineage server without tracker for testing nil case.
func newTestLineageServerWithoutTracker(t *testing.T) *testLineageServer {
	t.Helper()

	handler := NewLineageHandler(nil)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	return &testLineageServer{
		handler: handler,
		tracker: nil,
		mux:     mux,
		t:       t,
	}
}

func (ts *testLineageServer) request(method, path string, body string) *httptest.ResponseRecorder {
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

func (ts *testLineageServer) postJSON(path string, body interface{}) *httptest.ResponseRecorder {
	ts.t.Helper()

	jsonBody, err := json.Marshal(body)
	if err != nil {
		ts.t.Fatalf("failed to marshal body: %v", err)
	}
	return ts.request(http.MethodPost, path, string(jsonBody))
}

func (ts *testLineageServer) get(path string) *httptest.ResponseRecorder {
	return ts.request(http.MethodGet, path, "")
}

// registerFeature is a helper to register a feature for testing.
func (ts *testLineageServer) registerFeature(id string) *httptest.ResponseRecorder {
	ts.t.Helper()

	body := RegisterLineageRequest{
		FeatureID:   id,
		Name:        "Test Feature",
		Description: "Test description",
		Tags:        []string{"test"},
		CreatedBy:   "test-user",
	}

	return ts.postJSON("/v1/lineage/features", body)
}

func TestLineageHandler_NewLineageHandler(t *testing.T) {
	tracker := lineage.NewTracker()
	handler := NewLineageHandler(tracker)

	if handler.tracker == nil {
		t.Error("Expected tracker to be set")
	}
}

func TestLineageHandler_ListFeatures_Empty(t *testing.T) {
	ts := newTestLineageServer(t)

	rr := ts.get("/v1/lineage/features")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if result["count"].(float64) != 0 {
		t.Errorf("Expected count=0, got %v", result["count"])
	}
}

func TestLineageHandler_ListFeatures_NoTracker(t *testing.T) {
	ts := newTestLineageServerWithoutTracker(t)

	rr := ts.get("/v1/lineage/features")

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
	}
}

func TestLineageHandler_RegisterFeature(t *testing.T) {
	ts := newTestLineageServer(t)

	body := RegisterLineageRequest{
		FeatureID:   "feature-1",
		Name:        "Test Feature",
		Description: "Test description",
		Tags:        []string{"test", "example"},
		CreatedBy:   "test-user",
	}

	rr := ts.postJSON("/v1/lineage/features", body)

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
	if result["feature_id"] != "feature-1" {
		t.Errorf("Expected feature_id 'feature-1', got %v", result["feature_id"])
	}
}

func TestLineageHandler_RegisterFeature_MissingID(t *testing.T) {
	ts := newTestLineageServer(t)

	body := RegisterLineageRequest{
		Name: "Test Feature",
	}

	rr := ts.postJSON("/v1/lineage/features", body)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestLineageHandler_RegisterFeature_InvalidBody(t *testing.T) {
	ts := newTestLineageServer(t)

	rr := ts.request(http.MethodPost, "/v1/lineage/features", "invalid json")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestLineageHandler_RegisterFeature_NoTracker(t *testing.T) {
	ts := newTestLineageServerWithoutTracker(t)

	body := RegisterLineageRequest{
		FeatureID: "test",
		Name:      "Test",
	}

	rr := ts.postJSON("/v1/lineage/features", body)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
	}
}

func TestLineageHandler_GetFeature(t *testing.T) {
	ts := newTestLineageServer(t)

	// Register feature first
	ts.registerFeature("get-feature")

	rr := ts.get("/v1/lineage/features/get-feature")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d; body: %s", http.StatusOK, rr.Code, rr.Body.String())
	}
}

func TestLineageHandler_GetFeature_NotFound(t *testing.T) {
	ts := newTestLineageServer(t)

	rr := ts.get("/v1/lineage/features/nonexistent")

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, rr.Code)
	}
}

func TestLineageHandler_GetFeature_NoTracker(t *testing.T) {
	ts := newTestLineageServerWithoutTracker(t)

	rr := ts.get("/v1/lineage/features/test")

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
	}
}

func TestLineageHandler_ListSources(t *testing.T) {
	ts := newTestLineageServer(t)

	rr := ts.get("/v1/lineage/sources")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}
}

func TestLineageHandler_ListSources_NoTracker(t *testing.T) {
	ts := newTestLineageServerWithoutTracker(t)

	rr := ts.get("/v1/lineage/sources")

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
	}
}

func TestLineageHandler_RegisterSource(t *testing.T) {
	ts := newTestLineageServer(t)

	body := RegisterSourceRequest{
		ID:          "source-1",
		Name:        "Test Source",
		Type:        "database",
		Connection:  "postgres://localhost:5432/db",
		Owner:       "test-owner",
		Description: "Test source",
	}

	rr := ts.postJSON("/v1/lineage/sources", body)

	if rr.Code != http.StatusCreated {
		t.Errorf("Expected status %d, got %d; body: %s", http.StatusCreated, rr.Code, rr.Body.String())
	}
}

func TestLineageHandler_RegisterSource_MissingID(t *testing.T) {
	ts := newTestLineageServer(t)

	body := RegisterSourceRequest{
		Name: "Test Source",
		Type: "database",
	}

	rr := ts.postJSON("/v1/lineage/sources", body)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestLineageHandler_RegisterSource_InvalidBody(t *testing.T) {
	ts := newTestLineageServer(t)

	rr := ts.request(http.MethodPost, "/v1/lineage/sources", "invalid json")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestLineageHandler_RegisterSource_NoTracker(t *testing.T) {
	ts := newTestLineageServerWithoutTracker(t)

	body := RegisterSourceRequest{
		ID:   "test",
		Name: "Test",
	}

	rr := ts.postJSON("/v1/lineage/sources", body)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
	}
}

func TestLineageHandler_ListConsumers(t *testing.T) {
	ts := newTestLineageServer(t)

	rr := ts.get("/v1/lineage/consumers")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}
}

func TestLineageHandler_ListConsumers_NoTracker(t *testing.T) {
	ts := newTestLineageServerWithoutTracker(t)

	rr := ts.get("/v1/lineage/consumers")

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
	}
}

func TestLineageHandler_RegisterConsumer(t *testing.T) {
	ts := newTestLineageServer(t)

	body := RegisterConsumerRequest{
		ID:          "consumer-1",
		Name:        "Test Consumer",
		Type:        "model",
		Owner:       "test-owner",
		Description: "Test consumer",
		Endpoint:    "http://localhost:8080/predict",
	}

	rr := ts.postJSON("/v1/lineage/consumers", body)

	if rr.Code != http.StatusCreated {
		t.Errorf("Expected status %d, got %d; body: %s", http.StatusCreated, rr.Code, rr.Body.String())
	}
}

func TestLineageHandler_RegisterConsumer_MissingID(t *testing.T) {
	ts := newTestLineageServer(t)

	body := RegisterConsumerRequest{
		Name: "Test Consumer",
		Type: "model",
	}

	rr := ts.postJSON("/v1/lineage/consumers", body)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestLineageHandler_RegisterConsumer_InvalidBody(t *testing.T) {
	ts := newTestLineageServer(t)

	rr := ts.request(http.MethodPost, "/v1/lineage/consumers", "invalid json")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestLineageHandler_RegisterConsumer_NoTracker(t *testing.T) {
	ts := newTestLineageServerWithoutTracker(t)

	body := RegisterConsumerRequest{
		ID:   "test",
		Name: "Test",
	}

	rr := ts.postJSON("/v1/lineage/consumers", body)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
	}
}

func TestLineageHandler_LinkSource(t *testing.T) {
	ts := newTestLineageServer(t)

	// Register feature and source first
	ts.registerFeature("link-feature")
	ts.postJSON("/v1/lineage/sources", RegisterSourceRequest{
		ID:   "link-source",
		Name: "Link Source",
		Type: "database",
	})

	body := LinkSourceRequest{
		SourceID:  "link-source",
		FeatureID: "link-feature",
		Fields:    []string{"field1", "field2"},
	}

	rr := ts.postJSON("/v1/lineage/link/source", body)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d; body: %s", http.StatusOK, rr.Code, rr.Body.String())
	}
}

func TestLineageHandler_LinkSource_InvalidBody(t *testing.T) {
	ts := newTestLineageServer(t)

	rr := ts.request(http.MethodPost, "/v1/lineage/link/source", "invalid json")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestLineageHandler_LinkSource_NoTracker(t *testing.T) {
	ts := newTestLineageServerWithoutTracker(t)

	body := LinkSourceRequest{
		SourceID:  "src",
		FeatureID: "feat",
	}

	rr := ts.postJSON("/v1/lineage/link/source", body)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
	}
}

func TestLineageHandler_LinkConsumer(t *testing.T) {
	ts := newTestLineageServer(t)

	// Register feature and consumer first
	ts.registerFeature("link-consumer-feature")
	ts.postJSON("/v1/lineage/consumers", RegisterConsumerRequest{
		ID:   "link-consumer",
		Name: "Link Consumer",
		Type: "model",
	})

	body := LinkConsumerRequest{
		FeatureID:  "link-consumer-feature",
		ConsumerID: "link-consumer",
		Purpose:    "inference",
	}

	rr := ts.postJSON("/v1/lineage/link/consumer", body)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d; body: %s", http.StatusOK, rr.Code, rr.Body.String())
	}
}

func TestLineageHandler_LinkConsumer_InvalidBody(t *testing.T) {
	ts := newTestLineageServer(t)

	rr := ts.request(http.MethodPost, "/v1/lineage/link/consumer", "invalid json")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestLineageHandler_LinkConsumer_NoTracker(t *testing.T) {
	ts := newTestLineageServerWithoutTracker(t)

	body := LinkConsumerRequest{
		FeatureID:  "feat",
		ConsumerID: "consumer",
	}

	rr := ts.postJSON("/v1/lineage/link/consumer", body)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
	}
}

func TestLineageHandler_ImpactAnalysis(t *testing.T) {
	ts := newTestLineageServer(t)

	// Register feature first
	ts.registerFeature("impact-feature")

	rr := ts.get("/v1/lineage/impact/impact-feature")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d; body: %s", http.StatusOK, rr.Code, rr.Body.String())
	}
}

func TestLineageHandler_ImpactAnalysis_NotFound(t *testing.T) {
	ts := newTestLineageServer(t)

	rr := ts.get("/v1/lineage/impact/nonexistent")

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, rr.Code)
	}
}

func TestLineageHandler_ImpactAnalysis_NoTracker(t *testing.T) {
	ts := newTestLineageServerWithoutTracker(t)

	rr := ts.get("/v1/lineage/impact/test")

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
	}
}

func TestLineageHandler_GetGraph(t *testing.T) {
	ts := newTestLineageServer(t)

	rr := ts.get("/v1/lineage/graph")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}

	contentType := rr.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Expected Content-Type application/json, got %s", contentType)
	}
}

func TestLineageHandler_GetGraph_NoTracker(t *testing.T) {
	ts := newTestLineageServerWithoutTracker(t)

	rr := ts.get("/v1/lineage/graph")

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
	}
}

func TestLineageHandler_GetGraphDOT(t *testing.T) {
	ts := newTestLineageServer(t)

	rr := ts.get("/v1/lineage/graph/dot")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}

	contentType := rr.Header().Get("Content-Type")
	if contentType != "text/plain" {
		t.Errorf("Expected Content-Type text/plain, got %s", contentType)
	}
}

func TestLineageHandler_GetGraphDOT_NoTracker(t *testing.T) {
	ts := newTestLineageServerWithoutTracker(t)

	rr := ts.get("/v1/lineage/graph/dot")

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
	}
}

func TestLineageHandler_GetGraphMermaid(t *testing.T) {
	ts := newTestLineageServer(t)

	rr := ts.get("/v1/lineage/graph/mermaid")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}

	contentType := rr.Header().Get("Content-Type")
	if contentType != "text/plain" {
		t.Errorf("Expected Content-Type text/plain, got %s", contentType)
	}
}

func TestLineageHandler_GetGraphMermaid_NoTracker(t *testing.T) {
	ts := newTestLineageServerWithoutTracker(t)

	rr := ts.get("/v1/lineage/graph/mermaid")

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
	}
}

func TestLineageHandler_GetPIIFeatures(t *testing.T) {
	ts := newTestLineageServer(t)

	rr := ts.get("/v1/lineage/pii")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}
}

func TestLineageHandler_GetPIIFeatures_WithLevel(t *testing.T) {
	ts := newTestLineageServer(t)

	rr := ts.get("/v1/lineage/pii?min_level=medium")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}
}

func TestLineageHandler_GetPIIFeatures_NoTracker(t *testing.T) {
	ts := newTestLineageServerWithoutTracker(t)

	rr := ts.get("/v1/lineage/pii")

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
	}
}

func TestLineageHandler_SetPIIMetadata(t *testing.T) {
	ts := newTestLineageServer(t)

	// Register feature first
	ts.registerFeature("pii-feature")

	body := SetPIIMetadataRequest{
		PIILevel:        "high",
		PIITypes:        []string{"email", "name"},
		LegalBasis:      "consent",
		RetentionPolicy: "30 days",
		Encrypted:       true,
	}

	rr := ts.postJSON("/v1/lineage/pii/pii-feature", body)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d; body: %s", http.StatusOK, rr.Code, rr.Body.String())
	}
}

func TestLineageHandler_SetPIIMetadata_NotFound(t *testing.T) {
	ts := newTestLineageServer(t)

	body := SetPIIMetadataRequest{
		PIILevel: "low",
	}

	rr := ts.postJSON("/v1/lineage/pii/nonexistent", body)

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, rr.Code)
	}
}

func TestLineageHandler_SetPIIMetadata_InvalidBody(t *testing.T) {
	ts := newTestLineageServer(t)

	rr := ts.request(http.MethodPost, "/v1/lineage/pii/test", "invalid json")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestLineageHandler_SetPIIMetadata_NoTracker(t *testing.T) {
	ts := newTestLineageServerWithoutTracker(t)

	body := SetPIIMetadataRequest{
		PIILevel: "low",
	}

	rr := ts.postJSON("/v1/lineage/pii/test", body)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
	}
}

func TestLineageHandler_GetAuditLog(t *testing.T) {
	ts := newTestLineageServer(t)

	rr := ts.get("/v1/lineage/audit")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}
}

func TestLineageHandler_GetAuditLog_WithSince(t *testing.T) {
	ts := newTestLineageServer(t)

	rr := ts.get("/v1/lineage/audit?since=2024-01-01T00:00:00Z")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}
}

func TestLineageHandler_GetAuditLog_NoTracker(t *testing.T) {
	ts := newTestLineageServerWithoutTracker(t)

	rr := ts.get("/v1/lineage/audit")

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
	}
}
