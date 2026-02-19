package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/feather-store/feather/internal/extensions/semantic"
)

func setupSemanticCatalogHandler(t *testing.T) *http.ServeMux {
	t.Helper()
	catalog := semantic.NewCatalog(semantic.DefaultCatalogConfig())
	handler := NewSemanticCatalogHandler(catalog)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	return mux
}

func TestSemanticCatalog_SearchMissingQuery(t *testing.T) {
	mux := setupSemanticCatalogHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/catalog/search", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestSemanticCatalog_SearchWithQuery(t *testing.T) {
	mux := setupSemanticCatalogHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/catalog/search?q=user&limit=5", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestSemanticCatalog_Index(t *testing.T) {
	mux := setupSemanticCatalogHandler(t)
	body := `{"name":"user_clicks","description":"Total clicks","data_type":"INT64","entity_type":"user"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/catalog/index", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestSemanticCatalog_IndexInvalidJSON(t *testing.T) {
	mux := setupSemanticCatalogHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/v1/catalog/index", io.NopCloser(strings.NewReader("bad")))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestSemanticCatalog_List(t *testing.T) {
	mux := setupSemanticCatalogHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/catalog/entries", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestSemanticCatalog_GetNotFound(t *testing.T) {
	mux := setupSemanticCatalogHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/catalog/entries/nonexistent", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestSemanticCatalog_DetectDuplicates(t *testing.T) {
	mux := setupSemanticCatalogHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/catalog/duplicates", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestSemanticCatalog_Stats(t *testing.T) {
	mux := setupSemanticCatalogHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/catalog/stats", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}
