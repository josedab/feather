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

// testCacheServer wraps a CacheHandler for testing.
type testCacheServer struct {
	handler *CacheHandler
	mux     *http.ServeMux
	t       *testing.T
}

// newTestCacheServer creates a new test cache server.
func newTestCacheServer(t *testing.T) *testCacheServer {
	t.Helper()

	// Create a minimal store for testing
	store, err := storage.NewStore(context.Background(), storage.StoreOptions{
		HotMaxSize:   1024 * 1024, // 1MB
		WarmInMemory: true,
	}, storage.NewRegistry())
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	handler := NewCacheHandler(store)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	return &testCacheServer{
		handler: handler,
		mux:     mux,
		t:       t,
	}
}

func (ts *testCacheServer) request(method, path string, body string) *httptest.ResponseRecorder {
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

func (ts *testCacheServer) postJSON(path string, body interface{}) *httptest.ResponseRecorder {
	ts.t.Helper()

	jsonBody, err := json.Marshal(body)
	if err != nil {
		ts.t.Fatalf("failed to marshal body: %v", err)
	}
	return ts.request(http.MethodPost, path, string(jsonBody))
}

func (ts *testCacheServer) get(path string) *httptest.ResponseRecorder {
	return ts.request(http.MethodGet, path, "")
}

func TestCacheHandler_NewCacheHandler(t *testing.T) {
	store, err := storage.NewStore(context.Background(), storage.StoreOptions{
		HotMaxSize:   1024 * 1024,
		WarmInMemory: true,
	}, storage.NewRegistry())
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	handler := NewCacheHandler(store)

	if handler.predictive == nil {
		t.Error("Expected predictive cache to be set")
	}
	if handler.coAccess == nil {
		t.Error("Expected coAccess tracker to be set")
	}
}

func TestCacheHandler_RecordAccess(t *testing.T) {
	ts := newTestCacheServer(t)

	body := RecordAccessRequest{
		EntityID: "user-1",
		Features: []string{"feature1", "feature2", "feature3"},
	}

	rr := ts.postJSON("/v1/cache/access", body)

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
	if result["recorded"].(float64) != 3 {
		t.Errorf("Expected recorded=3, got %v", result["recorded"])
	}
}

func TestCacheHandler_RecordAccess_MissingEntityID(t *testing.T) {
	ts := newTestCacheServer(t)

	body := RecordAccessRequest{
		Features: []string{"feature1"},
	}

	rr := ts.postJSON("/v1/cache/access", body)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestCacheHandler_RecordAccess_MissingFeatures(t *testing.T) {
	ts := newTestCacheServer(t)

	body := RecordAccessRequest{
		EntityID: "user-1",
	}

	rr := ts.postJSON("/v1/cache/access", body)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestCacheHandler_RecordAccess_InvalidBody(t *testing.T) {
	ts := newTestCacheServer(t)

	rr := ts.request(http.MethodPost, "/v1/cache/access", "invalid json")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestCacheHandler_GetPatterns(t *testing.T) {
	ts := newTestCacheServer(t)

	rr := ts.get("/v1/cache/patterns")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if result["patterns"] == nil {
		t.Error("Expected patterns array in response")
	}
}

func TestCacheHandler_GetPatterns_WithLimit(t *testing.T) {
	ts := newTestCacheServer(t)

	rr := ts.get("/v1/cache/patterns?limit=10")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}
}

func TestCacheHandler_GetPattern(t *testing.T) {
	ts := newTestCacheServer(t)

	// Record some access first
	ts.postJSON("/v1/cache/access", RecordAccessRequest{
		EntityID: "pattern-entity",
		Features: []string{"pattern-feature"},
	})

	rr := ts.get("/v1/cache/patterns/pattern-entity/pattern-feature")

	// Pattern may not exist yet (needs more access), so accept 200 or 404
	if rr.Code != http.StatusOK && rr.Code != http.StatusNotFound {
		t.Errorf("Expected status 200 or 404, got %d", rr.Code)
	}
}

func TestCacheHandler_GetPattern_NotFound(t *testing.T) {
	ts := newTestCacheServer(t)

	rr := ts.get("/v1/cache/patterns/nonexistent/feature")

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, rr.Code)
	}
}

func TestCacheHandler_GetPredictions(t *testing.T) {
	ts := newTestCacheServer(t)

	rr := ts.get("/v1/cache/predictions")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	// predictions key should exist (may be nil for empty data)
	if _, exists := result["predictions"]; !exists {
		t.Error("Expected predictions key in response")
	}
}

func TestCacheHandler_GetPredictions_WithWindow(t *testing.T) {
	ts := newTestCacheServer(t)

	rr := ts.get("/v1/cache/predictions?window=10m")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}
}

func TestCacheHandler_GetRelated(t *testing.T) {
	ts := newTestCacheServer(t)

	rr := ts.get("/v1/cache/related/feature1")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if result["feature"] != "feature1" {
		t.Errorf("Expected feature 'feature1', got %v", result["feature"])
	}
}

func TestCacheHandler_GetRelated_WithLimit(t *testing.T) {
	ts := newTestCacheServer(t)

	rr := ts.get("/v1/cache/related/feature1?limit=5")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}
}

func TestCacheHandler_GetStats(t *testing.T) {
	ts := newTestCacheServer(t)

	rr := ts.get("/v1/cache/stats")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}
}

func TestCacheHandler_UpdateConfig(t *testing.T) {
	ts := newTestCacheServer(t)

	body := CacheConfigRequest{
		WarmingWindow:   "5m",
		WarmingInterval: "1m",
		MaxWarmItems:    100,
		MinScore:        0.5,
	}

	rr := ts.postJSON("/v1/cache/config", body)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d; body: %s", http.StatusOK, rr.Code, rr.Body.String())
	}
}

func TestCacheHandler_UpdateConfig_InvalidBody(t *testing.T) {
	ts := newTestCacheServer(t)

	rr := ts.request(http.MethodPost, "/v1/cache/config", "invalid json")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestCacheHandler_GetPredictiveCache(t *testing.T) {
	store, err := storage.NewStore(context.Background(), storage.StoreOptions{
		HotMaxSize:   1024 * 1024,
		WarmInMemory: true,
	}, storage.NewRegistry())
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	handler := NewCacheHandler(store)

	pc := handler.GetPredictiveCache()
	if pc == nil {
		t.Error("Expected GetPredictiveCache to return non-nil")
	}
}
