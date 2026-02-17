package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/feather-store/feather/internal/extensions/embeddingmgmt"
)

func newTestEmbeddingMgmtHandler(t *testing.T) (*http.ServeMux, *embeddingmgmt.Manager) {
	t.Helper()
	mgr := embeddingmgmt.NewManager(embeddingmgmt.DefaultManagerConfig())
	handler := NewEmbeddingMgmtHandler(mgr)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	return mux, mgr
}

func TestEmbeddingMgmtHandler_ListModels(t *testing.T) {
	mux, _ := newTestEmbeddingMgmtHandler(t)

	req := httptest.NewRequest("GET", "/v1/embeddings/models", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("GET /v1/embeddings/models = %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestEmbeddingMgmtHandler_RegisterModel(t *testing.T) {
	mux, _ := newTestEmbeddingMgmtHandler(t)

	body := `{"id":"text-embed","name":"Text Embedder","dimensions":128,"provider":"local"}`
	req := httptest.NewRequest("POST", "/v1/embeddings/models", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated && rr.Code != http.StatusOK {
		t.Errorf("POST /v1/embeddings/models = %d, want 200 or 201; body: %s", rr.Code, rr.Body.String())
	}
}

func TestEmbeddingMgmtHandler_InvalidJSON(t *testing.T) {
	mux, _ := newTestEmbeddingMgmtHandler(t)

	req := httptest.NewRequest("POST", "/v1/embeddings/models", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("POST with bad JSON = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestEmbeddingMgmtHandler_GetCollectionNotFound(t *testing.T) {
	mux, _ := newTestEmbeddingMgmtHandler(t)

	req := httptest.NewRequest("GET", "/v1/embeddings/collections/nonexistent", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Errorf("GET /v1/embeddings/collections/nonexistent = %d, want %d", rr.Code, http.StatusNotFound)
	}
}
