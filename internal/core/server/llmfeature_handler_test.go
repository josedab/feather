package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/feather-store/feather/internal/extensions/llmfeature"
)

func setupLLMFeatureHandler(t *testing.T) (*LLMFeatureHandler, *http.ServeMux) {
	t.Helper()
	store := llmfeature.NewStore(llmfeature.DefaultStoreConfig())
	handler := NewLLMFeatureHandler(store)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	return handler, mux
}

func TestLLMFeatureHandler_ListTemplates(t *testing.T) {
	_, mux := setupLLMFeatureHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/llm/templates", nil)
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

func TestLLMFeatureHandler_CreateTemplate(t *testing.T) {
	_, mux := setupLLMFeatureHandler(t)
	body := `{"id":"t1","name":"test","template":"hello {{.name}}","model":"gpt-4"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/llm/templates", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Errorf("expected %d, got %d; body: %s", http.StatusCreated, rr.Code, rr.Body.String())
	}
}

func TestLLMFeatureHandler_CreateTemplate_InvalidJSON(t *testing.T) {
	_, mux := setupLLMFeatureHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/v1/llm/templates", strings.NewReader("invalid"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected %d, got %d", http.StatusBadRequest, rr.Code)
	}
}
