package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/feather-store/feather/internal/platform/registry"
)

func setupModelRegistryHandler(t *testing.T) *http.ServeMux {
	t.Helper()
	reg := registry.NewModelRegistry()
	handler := NewModelRegistryHandler(reg)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	return mux
}

func TestModelRegistry_ListModels(t *testing.T) {
	mux := setupModelRegistryHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/registry/models", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestModelRegistry_RegisterModel(t *testing.T) {
	mux := setupModelRegistryHandler(t)
	body := `{"model_id":"model-1","model_name":"fraud-detector","features":["txn_amount","user_age"]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/registry/models", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestModelRegistry_RegisterModelInvalidJSON(t *testing.T) {
	mux := setupModelRegistryHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/v1/registry/models", io.NopCloser(strings.NewReader("invalid")))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestModelRegistry_GetModelNotFound(t *testing.T) {
	mux := setupModelRegistryHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/registry/models/nonexistent", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestModelRegistry_RemoveModelNotFound(t *testing.T) {
	mux := setupModelRegistryHandler(t)
	req := httptest.NewRequest(http.MethodDelete, "/v1/registry/models/nonexistent", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestModelRegistry_FeatureUsage(t *testing.T) {
	mux := setupModelRegistryHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/registry/features/txn_amount/usage", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestModelRegistry_BlastRadius(t *testing.T) {
	mux := setupModelRegistryHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/registry/features/txn_amount/blast-radius", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestModelRegistry_ListDeprecations(t *testing.T) {
	mux := setupModelRegistryHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/registry/deprecations", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestModelRegistry_DeprecateFeatureInvalidJSON(t *testing.T) {
	mux := setupModelRegistryHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/v1/registry/deprecations", strings.NewReader("bad"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestModelRegistry_AckDeprecationInvalidJSON(t *testing.T) {
	mux := setupModelRegistryHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/v1/registry/deprecations/feat1/ack", strings.NewReader("bad"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestModelRegistry_Stats(t *testing.T) {
	mux := setupModelRegistryHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/registry/stats", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}
