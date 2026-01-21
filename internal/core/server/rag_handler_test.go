package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/feather-store/feather/internal/extensions/rag"
)

func setupRAGHandler(t *testing.T) *http.ServeMux {
	t.Helper()
	handler := NewRAGHandler(rag.NewPipeline(rag.DefaultPipelineConfig()))
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	return mux
}

func TestRAG_GetListDocuments(t *testing.T) {
	mux := setupRAGHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/rag/documents", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestRAG_PostIngestDocument(t *testing.T) {
	mux := setupRAGHandler(t)
	body := `{"id":"doc1","content":"test document content","metadata":{}}`
	req := httptest.NewRequest(http.MethodPost, "/v1/rag/documents", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK && rr.Code != http.StatusCreated {
		t.Errorf("expected 200 or 201, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestRAG_PostInvalidJSON(t *testing.T) {
	mux := setupRAGHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/v1/rag/documents", io.NopCloser(strings.NewReader("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}
