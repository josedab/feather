package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/feather-store/feather/internal/extensions/materialization"
)

func setupMaterializationHandler(t *testing.T) (*MaterializationHandler, *http.ServeMux) {
	t.Helper()
	engine := materialization.NewEngine(materialization.DefaultEngineConfig())
	handler := NewMaterializationHandler(engine)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	return handler, mux
}

func TestMaterializationHandler_ListPipelines(t *testing.T) {
	_, mux := setupMaterializationHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/pipelines", nil)
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

func TestMaterializationHandler_CreatePipeline(t *testing.T) {
	_, mux := setupMaterializationHandler(t)
	body := `{"name":"test-pipeline","steps":[{"name":"step1","query":"SELECT * FROM users"}],"trigger":"manual"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/pipelines", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Errorf("expected %d, got %d; body: %s", http.StatusCreated, rr.Code, rr.Body.String())
	}
}

func TestMaterializationHandler_CreatePipeline_InvalidJSON(t *testing.T) {
	_, mux := setupMaterializationHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/v1/pipelines", strings.NewReader("invalid"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected %d, got %d", http.StatusBadRequest, rr.Code)
	}
}
