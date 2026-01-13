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

// testConsistencyServer wraps a ConsistencyHandler for testing.
type testConsistencyServer struct {
	handler *ConsistencyHandler
	mux     *http.ServeMux
	t       *testing.T
}

// newTestConsistencyServer creates a new test consistency server.
func newTestConsistencyServer(t *testing.T) *testConsistencyServer {
	t.Helper()

	store, err := storage.NewStore(context.Background(), storage.StoreOptions{
		HotMaxSize:   1024 * 1024, // 1MB
		WarmInMemory: true,
	}, storage.NewRegistry())
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	handler := NewConsistencyHandler(store)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	return &testConsistencyServer{
		handler: handler,
		mux:     mux,
		t:       t,
	}
}

func (ts *testConsistencyServer) request(method, path string, body string) *httptest.ResponseRecorder {
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

func (ts *testConsistencyServer) postJSON(path string, body interface{}) *httptest.ResponseRecorder {
	ts.t.Helper()

	jsonBody, err := json.Marshal(body)
	if err != nil {
		ts.t.Fatalf("failed to marshal body: %v", err)
	}
	return ts.request(http.MethodPost, path, string(jsonBody))
}

func (ts *testConsistencyServer) get(path string) *httptest.ResponseRecorder {
	return ts.request(http.MethodGet, path, "")
}

func TestConsistencyHandler_NewConsistencyHandler(t *testing.T) {
	store, err := storage.NewStore(context.Background(), storage.StoreOptions{
		HotMaxSize:   1024 * 1024,
		WarmInMemory: true,
	}, storage.NewRegistry())
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	handler := NewConsistencyHandler(store)

	if handler.checker == nil {
		t.Error("Expected checker to be set")
	}
}

func TestConsistencyHandler_GetChecker(t *testing.T) {
	store, err := storage.NewStore(context.Background(), storage.StoreOptions{
		HotMaxSize:   1024 * 1024,
		WarmInMemory: true,
	}, storage.NewRegistry())
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	handler := NewConsistencyHandler(store)

	checker := handler.GetChecker()
	if checker == nil {
		t.Error("Expected GetChecker to return non-nil")
	}
}

func TestConsistencyHandler_AddHTTPSource(t *testing.T) {
	ts := newTestConsistencyServer(t)

	body := HTTPSourceRequest{
		Name:     "offline-store",
		Endpoint: "http://localhost:8082/features",
		Headers:  map[string]string{"Authorization": "Bearer token"},
	}

	rr := ts.postJSON("/v1/consistency/sources/http", body)

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
	if result["name"] != "offline-store" {
		t.Errorf("Expected name 'offline-store', got %v", result["name"])
	}
}

func TestConsistencyHandler_AddHTTPSource_MissingFields(t *testing.T) {
	ts := newTestConsistencyServer(t)

	body := HTTPSourceRequest{
		Name: "test-source",
	}

	rr := ts.postJSON("/v1/consistency/sources/http", body)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestConsistencyHandler_AddHTTPSource_InvalidBody(t *testing.T) {
	ts := newTestConsistencyServer(t)

	rr := ts.request(http.MethodPost, "/v1/consistency/sources/http", "invalid json")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestConsistencyHandler_CheckFeature_MissingFields(t *testing.T) {
	ts := newTestConsistencyServer(t)

	body := CheckFeatureRequest{
		EntityID: "user-1",
	}

	rr := ts.postJSON("/v1/consistency/check", body)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestConsistencyHandler_CheckFeature_InvalidBody(t *testing.T) {
	ts := newTestConsistencyServer(t)

	rr := ts.request(http.MethodPost, "/v1/consistency/check", "invalid json")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestConsistencyHandler_CheckBatch_MissingFields(t *testing.T) {
	ts := newTestConsistencyServer(t)

	body := CheckBatchRequest{
		EntityIDs: []string{"user-1"},
	}

	rr := ts.postJSON("/v1/consistency/check/batch", body)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestConsistencyHandler_CheckBatch_InvalidBody(t *testing.T) {
	ts := newTestConsistencyServer(t)

	rr := ts.request(http.MethodPost, "/v1/consistency/check/batch", "invalid json")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestConsistencyHandler_GetResults(t *testing.T) {
	ts := newTestConsistencyServer(t)

	rr := ts.get("/v1/consistency/results")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	// results key should exist (may be nil for empty data)
	if _, exists := result["results"]; !exists {
		t.Error("Expected results key in response")
	}
}

func TestConsistencyHandler_GetResults_WithFilters(t *testing.T) {
	ts := newTestConsistencyServer(t)

	rr := ts.get("/v1/consistency/results?feature=test&limit=10&since=2024-01-01T00:00:00Z")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}
}

func TestConsistencyHandler_GetInconsistencies(t *testing.T) {
	ts := newTestConsistencyServer(t)

	rr := ts.get("/v1/consistency/inconsistencies")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	// inconsistencies key should exist (may be nil for empty data)
	if _, exists := result["inconsistencies"]; !exists {
		t.Error("Expected inconsistencies key in response")
	}
}

func TestConsistencyHandler_GetInconsistencies_WithFilters(t *testing.T) {
	ts := newTestConsistencyServer(t)

	rr := ts.get("/v1/consistency/inconsistencies?feature=test&limit=10")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}
}

func TestConsistencyHandler_GetReport(t *testing.T) {
	ts := newTestConsistencyServer(t)

	rr := ts.get("/v1/consistency/report")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}
}

func TestConsistencyHandler_GetReport_WithSince(t *testing.T) {
	ts := newTestConsistencyServer(t)

	rr := ts.get("/v1/consistency/report?since=2024-01-01T00:00:00Z")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}
}
