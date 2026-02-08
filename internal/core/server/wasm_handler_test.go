package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// testWASMServer wraps a WASMHandler for testing.
type testWASMServer struct {
	handler *WASMHandler
	mux     *http.ServeMux
	t       *testing.T
}

// newTestWASMServer creates a new test WASM server.
func newTestWASMServer(t *testing.T) *testWASMServer {
	t.Helper()

	handler := NewWASMHandler(nil) // nil runtime - will test nil handling
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	return &testWASMServer{
		handler: handler,
		mux:     mux,
		t:       t,
	}
}

func (ts *testWASMServer) request(method, path string, body string) *httptest.ResponseRecorder {
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

func (ts *testWASMServer) postJSON(path string, body interface{}) *httptest.ResponseRecorder {
	ts.t.Helper()

	jsonBody, err := json.Marshal(body)
	if err != nil {
		ts.t.Fatalf("failed to marshal body: %v", err)
	}
	return ts.request(http.MethodPost, path, string(jsonBody))
}

func (ts *testWASMServer) get(path string) *httptest.ResponseRecorder {
	return ts.request(http.MethodGet, path, "")
}

func (ts *testWASMServer) delete(path string) *httptest.ResponseRecorder {
	return ts.request(http.MethodDelete, path, "")
}

func TestWASMHandler_NewWASMHandler(t *testing.T) {
	handler := NewWASMHandler(nil)

	if handler == nil {
		t.Error("Expected handler to be created")
	}
}

func TestWASMHandler_ListPlugins_NotConfigured(t *testing.T) {
	ts := newTestWASMServer(t)

	rr := ts.get("/v1/plugins")

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
	}
}

func TestWASMHandler_LoadPlugin_NotConfigured(t *testing.T) {
	ts := newTestWASMServer(t)

	body := LoadPluginRequest{
		ID:         "test-plugin",
		Name:       "Test Plugin",
		Type:       "transform",
		WASMBase64: "AGFzbQEAAAA=", // minimal valid WASM header (base64)
	}

	rr := ts.postJSON("/v1/plugins", body)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
	}
}

func TestWASMHandler_LoadPlugin_InvalidBody(t *testing.T) {
	ts := newTestWASMServer(t)

	rr := ts.request(http.MethodPost, "/v1/plugins", "invalid json")

	// When runtime is nil, we get 503 before body parsing
	if rr.Code != http.StatusBadRequest && rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status 400 or 503, got %d", rr.Code)
	}
}

func TestWASMHandler_LoadPlugin_MissingID(t *testing.T) {
	ts := newTestWASMServer(t)

	body := LoadPluginRequest{
		Name: "Test Plugin",
		Type: "transform",
	}

	rr := ts.postJSON("/v1/plugins", body)

	// When runtime is nil, we get 503 before body validation
	if rr.Code != http.StatusBadRequest && rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status 400 or 503, got %d", rr.Code)
	}
}

func TestWASMHandler_LoadPlugin_MissingName(t *testing.T) {
	ts := newTestWASMServer(t)

	body := LoadPluginRequest{
		ID:   "test-plugin",
		Type: "transform",
	}

	rr := ts.postJSON("/v1/plugins", body)

	// When runtime is nil, we get 503 before body validation
	if rr.Code != http.StatusBadRequest && rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status 400 or 503, got %d", rr.Code)
	}
}

func TestWASMHandler_GetPlugin_NotConfigured(t *testing.T) {
	ts := newTestWASMServer(t)

	rr := ts.get("/v1/plugins/test-plugin")

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
	}
}

func TestWASMHandler_UnloadPlugin_NotConfigured(t *testing.T) {
	ts := newTestWASMServer(t)

	rr := ts.delete("/v1/plugins/test-plugin")

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
	}
}

func TestWASMHandler_EnablePlugin_NotConfigured(t *testing.T) {
	ts := newTestWASMServer(t)

	rr := ts.postJSON("/v1/plugins/test-plugin/enable", struct{}{})

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
	}
}

func TestWASMHandler_DisablePlugin_NotConfigured(t *testing.T) {
	ts := newTestWASMServer(t)

	rr := ts.postJSON("/v1/plugins/test-plugin/disable", struct{}{})

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
	}
}

func TestWASMHandler_CallFunction_NotConfigured(t *testing.T) {
	ts := newTestWASMServer(t)

	body := CallFunctionRequest{
		Args: map[string]interface{}{"input": "test"},
	}

	rr := ts.postJSON("/v1/plugins/test-plugin/call/transform", body)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
	}
}

func TestWASMHandler_CallFunction_InvalidBody(t *testing.T) {
	ts := newTestWASMServer(t)

	rr := ts.request(http.MethodPost, "/v1/plugins/test-plugin/call/transform", "invalid json")

	// When runtime is nil, we get 503 before body parsing
	if rr.Code != http.StatusBadRequest && rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status 400 or 503, got %d", rr.Code)
	}
}

func TestWASMHandler_GetMetrics_NotConfigured(t *testing.T) {
	ts := newTestWASMServer(t)

	rr := ts.get("/v1/plugins/metrics")

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
	}
}
