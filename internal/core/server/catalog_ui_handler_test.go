package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/feather-store/feather/internal/tools/catalog"
)

func setupCatalogUIHandler(t *testing.T) *http.ServeMux {
	t.Helper()
	service := catalog.NewService(catalog.DefaultConfig())
	handler := NewCatalogUIHandler(service)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	return mux
}

func TestCatalogUI_GetStats(t *testing.T) {
	mux := setupCatalogUIHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/catalog/stats", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestCatalogUI_PostRegisterFeature(t *testing.T) {
	mux := setupCatalogUIHandler(t)
	body := `{"name":"click_count","description":"Number of clicks","data_type":"int64","entity":"user","owner":"ml-team","tags":["clicks"]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/catalog/features", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK && rr.Code != http.StatusCreated {
		t.Errorf("expected 200 or 201, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestCatalogUI_PostInvalidJSON(t *testing.T) {
	mux := setupCatalogUIHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/v1/catalog/features", io.NopCloser(strings.NewReader("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}
