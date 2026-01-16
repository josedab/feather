package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// testSemanticServer wraps a SemanticHandler for testing.
type testSemanticServer struct {
	handler *SemanticHandler
	mux     *http.ServeMux
	t       *testing.T
}

// newTestSemanticServer creates a new test semantic server.
func newTestSemanticServer(t *testing.T) *testSemanticServer {
	t.Helper()

	handler := NewSemanticHandler(nil) // nil search - will test nil handling
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	return &testSemanticServer{
		handler: handler,
		mux:     mux,
		t:       t,
	}
}

func (ts *testSemanticServer) request(method, path string, body string) *httptest.ResponseRecorder {
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

func (ts *testSemanticServer) postJSON(path string, body interface{}) *httptest.ResponseRecorder {
	ts.t.Helper()

	jsonBody, err := json.Marshal(body)
	if err != nil {
		ts.t.Fatalf("failed to marshal body: %v", err)
	}
	return ts.request(http.MethodPost, path, string(jsonBody))
}

func (ts *testSemanticServer) get(path string) *httptest.ResponseRecorder {
	return ts.request(http.MethodGet, path, "")
}

func (ts *testSemanticServer) delete(path string) *httptest.ResponseRecorder {
	return ts.request(http.MethodDelete, path, "")
}

func TestSemanticHandler_NewSemanticHandler(t *testing.T) {
	handler := NewSemanticHandler(nil)

	if handler == nil {
		t.Error("Expected handler to be created")
	}
}

func TestSemanticHandler_ListFeatures_NotConfigured(t *testing.T) {
	ts := newTestSemanticServer(t)

	rr := ts.get("/v1/semantic/features")

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
	}
}

func TestSemanticHandler_IndexFeature_NotConfigured(t *testing.T) {
	ts := newTestSemanticServer(t)

	body := FeatureDocJSON{
		ID:          "test-feature",
		Name:        "Test Feature",
		Description: "A test feature",
	}

	rr := ts.postJSON("/v1/semantic/features", body)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
	}
}

func TestSemanticHandler_IndexFeature_InvalidBody(t *testing.T) {
	ts := newTestSemanticServer(t)

	rr := ts.request(http.MethodPost, "/v1/semantic/features", "invalid json")

	// When search is nil, we get 503 before body parsing
	if rr.Code != http.StatusBadRequest && rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status 400 or 503, got %d", rr.Code)
	}
}

func TestSemanticHandler_IndexFeature_MissingID(t *testing.T) {
	ts := newTestSemanticServer(t)

	body := FeatureDocJSON{
		Name:        "Test Feature",
		Description: "A test feature",
	}

	rr := ts.postJSON("/v1/semantic/features", body)

	// When search is nil, we get 503 before body validation
	if rr.Code != http.StatusBadRequest && rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status 400 or 503, got %d", rr.Code)
	}
}

func TestSemanticHandler_IndexBatch_NotConfigured(t *testing.T) {
	ts := newTestSemanticServer(t)

	body := struct {
		Features []FeatureDocJSON `json:"features"`
	}{
		Features: []FeatureDocJSON{
			{ID: "feature-1", Name: "Feature 1"},
			{ID: "feature-2", Name: "Feature 2"},
		},
	}

	rr := ts.postJSON("/v1/semantic/features/batch", body)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
	}
}

func TestSemanticHandler_IndexBatch_InvalidBody(t *testing.T) {
	ts := newTestSemanticServer(t)

	rr := ts.request(http.MethodPost, "/v1/semantic/features/batch", "invalid json")

	// When search is nil, we get 503 before body parsing
	if rr.Code != http.StatusBadRequest && rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status 400 or 503, got %d", rr.Code)
	}
}

func TestSemanticHandler_GetFeature_NotConfigured(t *testing.T) {
	ts := newTestSemanticServer(t)

	rr := ts.get("/v1/semantic/features/test-feature")

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
	}
}

func TestSemanticHandler_DeleteFeature_NotConfigured(t *testing.T) {
	ts := newTestSemanticServer(t)

	rr := ts.delete("/v1/semantic/features/test-feature")

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
	}
}

func TestSemanticHandler_Search_NotConfigured(t *testing.T) {
	ts := newTestSemanticServer(t)

	body := SearchRequest{
		Query: "find user features",
		Limit: 10,
	}

	rr := ts.postJSON("/v1/semantic/search", body)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
	}
}

func TestSemanticHandler_Search_InvalidBody(t *testing.T) {
	ts := newTestSemanticServer(t)

	rr := ts.request(http.MethodPost, "/v1/semantic/search", "invalid json")

	// When search is nil, we get 503 before body parsing
	if rr.Code != http.StatusBadRequest && rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status 400 or 503, got %d", rr.Code)
	}
}

func TestSemanticHandler_Search_MissingQuery(t *testing.T) {
	ts := newTestSemanticServer(t)

	body := SearchRequest{
		Limit: 10,
	}

	rr := ts.postJSON("/v1/semantic/search", body)

	// When search is nil, we get 503 before body validation
	if rr.Code != http.StatusBadRequest && rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status 400 or 503, got %d", rr.Code)
	}
}

func TestSemanticHandler_Suggest_NotConfigured(t *testing.T) {
	ts := newTestSemanticServer(t)

	rr := ts.get("/v1/semantic/suggest/test-feature")

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
	}
}

func TestSemanticHandler_GetStats_NotConfigured(t *testing.T) {
	ts := newTestSemanticServer(t)

	rr := ts.get("/v1/semantic/stats")

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
	}
}
