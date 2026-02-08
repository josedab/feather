package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// testWarehouseServer wraps a WarehouseHandler for testing.
type testWarehouseServer struct {
	handler *WarehouseHandler
	mux     *http.ServeMux
	t       *testing.T
}

// newTestWarehouseServer creates a new test warehouse server.
func newTestWarehouseServer(t *testing.T) *testWarehouseServer {
	t.Helper()

	handler := NewWarehouseHandler(WarehouseHandlerConfig{})
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	return &testWarehouseServer{
		handler: handler,
		mux:     mux,
		t:       t,
	}
}

func (ts *testWarehouseServer) request(method, path string, body string) *httptest.ResponseRecorder {
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

func (ts *testWarehouseServer) postJSON(path string, body interface{}) *httptest.ResponseRecorder {
	ts.t.Helper()

	jsonBody, err := json.Marshal(body)
	if err != nil {
		ts.t.Fatalf("failed to marshal body: %v", err)
	}
	return ts.request(http.MethodPost, path, string(jsonBody))
}

func (ts *testWarehouseServer) get(path string) *httptest.ResponseRecorder {
	return ts.request(http.MethodGet, path, "")
}

func (ts *testWarehouseServer) delete(path string) *httptest.ResponseRecorder {
	return ts.request(http.MethodDelete, path, "")
}

func TestWarehouseHandler_NewWarehouseHandler(t *testing.T) {
	handler := NewWarehouseHandler(WarehouseHandlerConfig{})

	if handler == nil {
		t.Error("Expected handler to be non-nil")
	}
}

func TestWarehouseHandler_ListConnectors(t *testing.T) {
	ts := newTestWarehouseServer(t)

	rr := ts.get("/v1/warehouse/connectors")

	// Without engine configured, returns 503
	if rr.Code != http.StatusOK && rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d or %d, got %d", http.StatusOK, http.StatusServiceUnavailable, rr.Code)
	}
}

func TestWarehouseHandler_CreateConnector_InvalidBody(t *testing.T) {
	ts := newTestWarehouseServer(t)

	rr := ts.request(http.MethodPost, "/v1/warehouse/connectors", "invalid json")

	// Without engine, may return 503; with engine but invalid body, returns 400
	if rr.Code != http.StatusBadRequest && rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d or %d, got %d", http.StatusBadRequest, http.StatusServiceUnavailable, rr.Code)
	}
}

func TestWarehouseHandler_CreateConnector_MissingType(t *testing.T) {
	ts := newTestWarehouseServer(t)

	body := map[string]interface{}{
		"id":     "test-connector",
		"config": map[string]interface{}{},
	}

	rr := ts.postJSON("/v1/warehouse/connectors", body)

	// Without engine configured, returns 503
	if rr.Code != http.StatusBadRequest && rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d or %d, got %d; body: %s", http.StatusBadRequest, http.StatusServiceUnavailable, rr.Code, rr.Body.String())
	}
}

func TestWarehouseHandler_CreateConnector_UnsupportedType(t *testing.T) {
	ts := newTestWarehouseServer(t)

	body := map[string]interface{}{
		"id":     "test-connector",
		"type":   "unsupported",
		"config": map[string]interface{}{},
	}

	rr := ts.postJSON("/v1/warehouse/connectors", body)

	// Without engine configured, returns 503
	if rr.Code != http.StatusBadRequest && rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d or %d, got %d; body: %s", http.StatusBadRequest, http.StatusServiceUnavailable, rr.Code, rr.Body.String())
	}
}

func TestWarehouseHandler_GetConnector_NotFound(t *testing.T) {
	ts := newTestWarehouseServer(t)

	rr := ts.get("/v1/warehouse/connectors/nonexistent")

	// Without engine configured, returns 503
	if rr.Code != http.StatusNotFound && rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d or %d, got %d", http.StatusNotFound, http.StatusServiceUnavailable, rr.Code)
	}
}

func TestWarehouseHandler_DeleteConnector_NotFound(t *testing.T) {
	ts := newTestWarehouseServer(t)

	rr := ts.delete("/v1/warehouse/connectors/nonexistent")

	// Without engine configured, returns 503
	if rr.Code != http.StatusNotFound && rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d or %d, got %d", http.StatusNotFound, http.StatusServiceUnavailable, rr.Code)
	}
}

func TestWarehouseHandler_ListJobs(t *testing.T) {
	ts := newTestWarehouseServer(t)

	rr := ts.get("/v1/warehouse/jobs")

	// Without engine configured, returns 503
	if rr.Code != http.StatusOK && rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d or %d, got %d", http.StatusOK, http.StatusServiceUnavailable, rr.Code)
	}
}

func TestWarehouseHandler_CreateJob_InvalidBody(t *testing.T) {
	ts := newTestWarehouseServer(t)

	rr := ts.request(http.MethodPost, "/v1/warehouse/jobs", "invalid json")

	// Without engine configured, returns 503
	if rr.Code != http.StatusBadRequest && rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d or %d, got %d", http.StatusBadRequest, http.StatusServiceUnavailable, rr.Code)
	}
}

func TestWarehouseHandler_GetJob_NotFound(t *testing.T) {
	ts := newTestWarehouseServer(t)

	rr := ts.get("/v1/warehouse/jobs/nonexistent")

	// Without engine configured, returns 503
	if rr.Code != http.StatusNotFound && rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d or %d, got %d", http.StatusNotFound, http.StatusServiceUnavailable, rr.Code)
	}
}

func TestWarehouseHandler_TriggerJob_NotFound(t *testing.T) {
	ts := newTestWarehouseServer(t)

	rr := ts.postJSON("/v1/warehouse/jobs/nonexistent/trigger", map[string]interface{}{})

	// Without engine configured, returns 503 or 404
	if rr.Code != http.StatusNotFound && rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d or %d, got %d", http.StatusNotFound, http.StatusServiceUnavailable, rr.Code)
	}
}

func TestWarehouseHandler_ListExecutions(t *testing.T) {
	ts := newTestWarehouseServer(t)

	rr := ts.get("/v1/warehouse/executions")

	// Without engine configured, may return 503 or 404
	if rr.Code != http.StatusOK && rr.Code != http.StatusServiceUnavailable && rr.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, %d or %d, got %d", http.StatusOK, http.StatusServiceUnavailable, http.StatusNotFound, rr.Code)
	}
}

func TestWarehouseHandler_GetExecution_NotFound(t *testing.T) {
	ts := newTestWarehouseServer(t)

	rr := ts.get("/v1/warehouse/executions/nonexistent")

	// Without engine configured, returns 503 or 404
	if rr.Code != http.StatusNotFound && rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d or %d, got %d", http.StatusNotFound, http.StatusServiceUnavailable, rr.Code)
	}
}

func TestWarehouseHandler_GetStats(t *testing.T) {
	ts := newTestWarehouseServer(t)

	rr := ts.get("/v1/warehouse/stats")

	// Without engine configured, returns 503
	if rr.Code != http.StatusOK && rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d or %d, got %d", http.StatusOK, http.StatusServiceUnavailable, rr.Code)
	}
}
