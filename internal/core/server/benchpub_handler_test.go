package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/feather-store/feather/internal/extensions/benchpub"
)

type testBenchPubServer struct {
	handler *BenchPubHandler
	suite   *benchpub.Suite
	mux     *http.ServeMux
	t       *testing.T
}

func newTestBenchPubServer(t *testing.T) *testBenchPubServer {
	t.Helper()
	suite := benchpub.NewSuite(benchpub.DefaultSuiteConfig())
	handler := NewBenchPubHandler(suite)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	return &testBenchPubServer{handler: handler, suite: suite, mux: mux, t: t}
}

func (ts *testBenchPubServer) request(method, path string, body string) *httptest.ResponseRecorder {
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

func TestBenchPubHandler_GetStats(t *testing.T) {
	ts := newTestBenchPubServer(t)
	rr := ts.request(http.MethodGet, "/v1/benchmarks/stats", "")
	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}
	var result map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}
}

func TestBenchPubHandler_RunBenchmark(t *testing.T) {
	ts := newTestBenchPubServer(t)
	body := `{"name":"latency_test","concurrency":4}`
	rr := ts.request(http.MethodPost, "/v1/benchmarks/run", body)
	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d; body: %s", http.StatusOK, rr.Code, rr.Body.String())
	}
	var result map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}
}

func TestBenchPubHandler_RunBenchmark_InvalidJSON(t *testing.T) {
	ts := newTestBenchPubServer(t)
	rr := ts.request(http.MethodPost, "/v1/benchmarks/run", "not json")
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestBenchPubHandler_GetResult_NotFound(t *testing.T) {
	ts := newTestBenchPubServer(t)
	rr := ts.request(http.MethodGet, "/v1/benchmarks/results/nonexistent", "")
	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, rr.Code)
	}
}
