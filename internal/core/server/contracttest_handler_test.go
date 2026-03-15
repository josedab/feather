package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/feather-store/feather/internal/extensions/contracttest"
)

type testContractTestServer struct {
	handler *ContractTestHandler
	runner  *contracttest.Runner
	mux     *http.ServeMux
	t       *testing.T
}

func newTestContractTestServer(t *testing.T) *testContractTestServer {
	t.Helper()
	runner := contracttest.NewRunner(contracttest.DefaultRunnerConfig())
	handler := NewContractTestHandler(runner)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	return &testContractTestServer{handler: handler, runner: runner, mux: mux, t: t}
}

func (ts *testContractTestServer) request(method, path string, body string) *httptest.ResponseRecorder {
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

func TestContractTestHandler_GetStats(t *testing.T) {
	ts := newTestContractTestServer(t)
	rr := ts.request(http.MethodGet, "/v1/contracts/stats", "")
	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}
	var result map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}
}

func TestContractTestHandler_RegisterContract(t *testing.T) {
	ts := newTestContractTestServer(t)
	body := `{"id":"schema-v1","name":"click_rate_schema","feature_group":"click_rate","type":"schema","rules":{"click_rate":"float64"}}`
	rr := ts.request(http.MethodPost, "/v1/contracts", body)
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
}

func TestContractTestHandler_RegisterContract_InvalidJSON(t *testing.T) {
	ts := newTestContractTestServer(t)
	rr := ts.request(http.MethodPost, "/v1/contracts", "not json")
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestContractTestHandler_GetContract_NotFound(t *testing.T) {
	ts := newTestContractTestServer(t)
	rr := ts.request(http.MethodGet, "/v1/contracts/nonexistent", "")
	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, rr.Code)
	}
}
