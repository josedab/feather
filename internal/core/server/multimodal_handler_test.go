package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/feather-store/feather/internal/extensions/multimodal"
)

func setupMultiModalHandler(t *testing.T) (*MultiModalHandler, *http.ServeMux) {
	t.Helper()
	store := multimodal.NewMultiModalStore(multimodal.DefaultStoreConfig())
	embeddings := multimodal.NewEmbeddingIndex(multimodal.DefaultEmbeddingConfig())
	handler := NewMultiModalHandler(store, embeddings)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	return handler, mux
}

func TestMultiModalHandler_ListBlobs(t *testing.T) {
	_, mux := setupMultiModalHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/multimodal/blobs", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected %d, got %d", http.StatusOK, rr.Code)
	}
	var result map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if result["success"] != true {
		t.Error("expected success=true")
	}
}

func TestMultiModalHandler_StoreBlob(t *testing.T) {
	_, mux := setupMultiModalHandler(t)
	body := `{"modality":"text","content_type":"text/plain","data":"aGVsbG8=","tags":{"env":"test"}}`
	req := httptest.NewRequest(http.MethodPost, "/v1/multimodal/blobs", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Errorf("expected %d, got %d; body: %s", http.StatusCreated, rr.Code, rr.Body.String())
	}
}

func TestMultiModalHandler_StoreBlob_InvalidJSON(t *testing.T) {
	_, mux := setupMultiModalHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/v1/multimodal/blobs", strings.NewReader("invalid"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected %d, got %d", http.StatusBadRequest, rr.Code)
	}
}
