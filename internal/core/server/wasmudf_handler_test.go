package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/feather-store/feather/internal/extensions/wasmudf"
)

type testWasmUDFServer struct {
	handler *WasmUDFHandler
	runtime *wasmudf.Runtime
	mux     *http.ServeMux
	t       *testing.T
}

func newTestWasmUDFServer(t *testing.T) *testWasmUDFServer {
	t.Helper()
	runtime := wasmudf.NewRuntime(wasmudf.DefaultRuntimeConfig())
	handler := NewWasmUDFHandler(runtime)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	return &testWasmUDFServer{handler: handler, runtime: runtime, mux: mux, t: t}
}

func (ts *testWasmUDFServer) request(method, path string, body string) *httptest.ResponseRecorder {
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

func TestWasmUDFHandler_GetStats(t *testing.T) {
	ts := newTestWasmUDFServer(t)
	rr := ts.request(http.MethodGet, "/v1/wasm/stats", "")
	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}
	var result map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}
}

func TestWasmUDFHandler_RegisterModule(t *testing.T) {
	ts := newTestWasmUDFServer(t)
	body := `{"id":"normalize-v1","name":"normalize","language":"rust","source":"base64wasmdata","function":"transform"}`
	rr := ts.request(http.MethodPost, "/v1/wasm/modules", body)
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

func TestWasmUDFHandler_RegisterModule_InvalidJSON(t *testing.T) {
	ts := newTestWasmUDFServer(t)
	rr := ts.request(http.MethodPost, "/v1/wasm/modules", "not json")
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestWasmUDFHandler_GetModule_NotFound(t *testing.T) {
	ts := newTestWasmUDFServer(t)
	rr := ts.request(http.MethodGet, "/v1/wasm/modules/nonexistent", "")
	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, rr.Code)
	}
}
