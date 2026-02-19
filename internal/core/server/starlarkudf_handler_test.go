package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/feather-store/feather/internal/extensions/starlarkudf"
)

func setupStarlarkUDFHandler(t *testing.T) *http.ServeMux {
	t.Helper()
	registry := starlarkudf.NewRegistry(starlarkudf.DefaultRegistryConfig())
	handler := NewStarlarkUDFHandler(registry)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	return mux
}

func TestStarlarkUDF_List(t *testing.T) {
	mux := setupStarlarkUDFHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/udfs", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestStarlarkUDF_Register(t *testing.T) {
	mux := setupStarlarkUDFHandler(t)
	body := `{"name":"double_it","expression":"def transform(x): return x * 2","type":"transform"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/udfs", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestStarlarkUDF_RegisterInvalidJSON(t *testing.T) {
	mux := setupStarlarkUDFHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/v1/udfs", io.NopCloser(strings.NewReader("bad")))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestStarlarkUDF_GetNotFound(t *testing.T) {
	mux := setupStarlarkUDFHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/udfs/nonexistent", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestStarlarkUDF_RemoveNotFound(t *testing.T) {
	mux := setupStarlarkUDFHandler(t)
	req := httptest.NewRequest(http.MethodDelete, "/v1/udfs/nonexistent", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestStarlarkUDF_ExecuteNotFound(t *testing.T) {
	mux := setupStarlarkUDFHandler(t)
	body := `{"inputs":{"x":5}}`
	req := httptest.NewRequest(http.MethodPost, "/v1/udfs/nonexistent/execute", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestStarlarkUDF_ExecuteInvalidJSON(t *testing.T) {
	mux := setupStarlarkUDFHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/v1/udfs/test/execute", strings.NewReader("bad"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestStarlarkUDF_Stats(t *testing.T) {
	mux := setupStarlarkUDFHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/udfs/stats", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}
