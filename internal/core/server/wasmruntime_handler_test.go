package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/feather-store/feather/internal/extensions/wasmruntime"
)

func newTestWasmRuntimeHandler(t *testing.T) *http.ServeMux {
	t.Helper()
	manager := wasmruntime.NewEdgeManager(wasmruntime.DefaultEdgeManagerConfig())
	handler := NewWasmRuntimeHandler(manager)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	return mux
}

func TestWasmRuntimeHandler_RegisterModule(t *testing.T) {
	mux := newTestWasmRuntimeHandler(t)

	body := `{"id":"m1","name":"Age Bucket","wasm_bytes":1024,"inputs":["age"],"outputs":["age_bucket"]}`
	req := httptest.NewRequest("POST", "/v1/edge/modules", strings.NewReader(body))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Errorf("POST module = %d, want %d; body: %s", rr.Code, http.StatusCreated, rr.Body.String())
	}
}

func TestWasmRuntimeHandler_ListModules(t *testing.T) {
	mux := newTestWasmRuntimeHandler(t)

	req := httptest.NewRequest("GET", "/v1/edge/modules", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("GET modules = %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestWasmRuntimeHandler_RegisterDevice(t *testing.T) {
	mux := newTestWasmRuntimeHandler(t)

	body := `{"id":"d1","name":"Edge Device 1","region":"us-west"}`
	req := httptest.NewRequest("POST", "/v1/edge/devices", strings.NewReader(body))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Errorf("POST device = %d, want %d; body: %s", rr.Code, http.StatusCreated, rr.Body.String())
	}
}

func TestWasmRuntimeHandler_DeviceNotFound(t *testing.T) {
	mux := newTestWasmRuntimeHandler(t)

	req := httptest.NewRequest("GET", "/v1/edge/devices/nonexistent", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Errorf("GET nonexistent = %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestWasmRuntimeHandler_Stats(t *testing.T) {
	mux := newTestWasmRuntimeHandler(t)

	req := httptest.NewRequest("GET", "/v1/edge/stats", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("GET stats = %d, want %d", rr.Code, http.StatusOK)
	}
}
