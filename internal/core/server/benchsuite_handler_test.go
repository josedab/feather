package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/feather-store/feather/internal/tools/benchsuite"
)

func setupBenchSuiteHandler(t *testing.T) *http.ServeMux {
	t.Helper()
	suite := benchsuite.NewSuite(benchsuite.DefaultSuiteConfig())
	handler := NewBenchSuiteHandler(suite)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	return mux
}

func TestBenchSuite_GetStats(t *testing.T) {
	mux := setupBenchSuiteHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/benchmarks/stats", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestBenchSuite_PostCreateRun(t *testing.T) {
	mux := setupBenchSuiteHandler(t)
	body := `{"name":"test-run","workload":"read_heavy"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/benchmarks/runs", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK && rr.Code != http.StatusCreated {
		t.Errorf("expected 200 or 201, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestBenchSuite_PostInvalidJSON(t *testing.T) {
	mux := setupBenchSuiteHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/v1/benchmarks/runs", io.NopCloser(strings.NewReader("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}
