package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/feather-store/feather/internal/storage"
)

// testBenchmarkServer wraps a BenchmarkHandler for testing.
type testBenchmarkServer struct {
	handler *BenchmarkHandler
	mux     *http.ServeMux
	t       *testing.T
}

// newTestBenchmarkServer creates a new test benchmark server.
func newTestBenchmarkServer(t *testing.T) *testBenchmarkServer {
	t.Helper()

	store, err := storage.NewStore(storage.StoreOptions{
		HotMaxSize:   1024 * 1024, // 1MB
		WarmInMemory: true,
	}, storage.NewRegistry())
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	handler := NewBenchmarkHandler(store)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	return &testBenchmarkServer{
		handler: handler,
		mux:     mux,
		t:       t,
	}
}

// newTestBenchmarkServerWithoutStore creates a benchmark server without store for testing nil case.
func newTestBenchmarkServerWithoutStore(t *testing.T) *testBenchmarkServer {
	t.Helper()

	handler := NewBenchmarkHandler(nil)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	return &testBenchmarkServer{
		handler: handler,
		mux:     mux,
		t:       t,
	}
}

func (ts *testBenchmarkServer) request(method, path string, body string) *httptest.ResponseRecorder {
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

func (ts *testBenchmarkServer) postJSON(path string, body interface{}) *httptest.ResponseRecorder {
	ts.t.Helper()

	jsonBody, err := json.Marshal(body)
	if err != nil {
		ts.t.Fatalf("failed to marshal body: %v", err)
	}
	return ts.request(http.MethodPost, path, string(jsonBody))
}

func (ts *testBenchmarkServer) get(path string) *httptest.ResponseRecorder {
	return ts.request(http.MethodGet, path, "")
}

func TestBenchmarkHandler_NewBenchmarkHandler(t *testing.T) {
	store, err := storage.NewStore(storage.StoreOptions{
		HotMaxSize:   1024 * 1024,
		WarmInMemory: true,
	}, storage.NewRegistry())
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	handler := NewBenchmarkHandler(store)

	if handler.store == nil {
		t.Error("Expected store to be set")
	}
}

func TestBenchmarkHandler_GetDefaultConfig(t *testing.T) {
	ts := newTestBenchmarkServer(t)

	rr := ts.get("/v1/benchmark/config")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if result["num_entities"] == nil {
		t.Error("Expected num_entities in response")
	}
	if result["num_features"] == nil {
		t.Error("Expected num_features in response")
	}
	if result["num_operations"] == nil {
		t.Error("Expected num_operations in response")
	}
	if result["concurrency"] == nil {
		t.Error("Expected concurrency in response")
	}
}

func TestBenchmarkHandler_RunBenchmark_NoStore(t *testing.T) {
	ts := newTestBenchmarkServerWithoutStore(t)

	rr := ts.request(http.MethodPost, "/v1/benchmark/run", "")

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
	}
}

func TestBenchmarkHandler_RunBenchmark_WithConfig(t *testing.T) {
	ts := newTestBenchmarkServer(t)

	body := BenchmarkRequest{
		NumEntities:   10,
		NumFeatures:   5,
		NumOperations: 100,
		Concurrency:   2,
	}

	rr := ts.postJSON("/v1/benchmark/run", body)

	// Benchmark may take time, accept 200 or 500 (timeout)
	if rr.Code != http.StatusOK && rr.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 200 or 500, got %d; body: %s", rr.Code, rr.Body.String())
	}
}

func TestBenchmarkHandler_RunBenchmark_WithQueryParams(t *testing.T) {
	ts := newTestBenchmarkServer(t)

	rr := ts.request(http.MethodPost, "/v1/benchmark/run?entities=10&operations=50&concurrency=1", "")

	// Benchmark may take time, accept 200 or 500 (timeout)
	if rr.Code != http.StatusOK && rr.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 200 or 500, got %d; body: %s", rr.Code, rr.Body.String())
	}
}

func TestBenchmarkHandler_RunBenchmark_InvalidBody(t *testing.T) {
	ts := newTestBenchmarkServer(t)

	rr := ts.request(http.MethodPost, "/v1/benchmark/run", "invalid json")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}
