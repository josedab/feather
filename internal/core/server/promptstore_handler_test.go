package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/feather-store/feather/internal/extensions/promptstore"
)

func newTestPromptStoreHandler(t *testing.T) (*http.ServeMux, *promptstore.Store) {
	t.Helper()
	store := promptstore.NewStore(promptstore.DefaultStoreConfig())
	handler := NewPromptStoreHandler(store)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	return mux, store
}

func TestPromptStoreHandler_ListPrompts(t *testing.T) {
	mux, _ := newTestPromptStoreHandler(t)

	req := httptest.NewRequest("GET", "/v1/prompts", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("GET /v1/prompts = %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestPromptStoreHandler_CreatePrompt(t *testing.T) {
	mux, _ := newTestPromptStoreHandler(t)

	body := `{"id":"greet","name":"greeting","template":"Hello {{.name}}","variables":["name"]}`
	req := httptest.NewRequest("POST", "/v1/prompts", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated && rr.Code != http.StatusOK {
		t.Errorf("POST /v1/prompts = %d, want 200 or 201; body: %s", rr.Code, rr.Body.String())
	}
}

func TestPromptStoreHandler_InvalidJSON(t *testing.T) {
	mux, _ := newTestPromptStoreHandler(t)

	req := httptest.NewRequest("POST", "/v1/prompts", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("POST with bad JSON = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestPromptStoreHandler_GetNotFound(t *testing.T) {
	mux, _ := newTestPromptStoreHandler(t)

	req := httptest.NewRequest("GET", "/v1/prompts/nonexistent", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Errorf("GET /v1/prompts/nonexistent = %d, want %d", rr.Code, http.StatusNotFound)
	}
}
