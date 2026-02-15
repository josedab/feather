package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func setupPipelineHandler(t *testing.T) (*PipelineHandler, *http.ServeMux) {
	t.Helper()
	handler := NewPipelineHandler()
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	return handler, mux
}

func TestPipelineHandler_ListPipelines(t *testing.T) {
	_, mux := setupPipelineHandler(t)
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

func TestPipelineHandler_CreatePipeline(t *testing.T) {
	_, mux := setupPipelineHandler(t)
	body := `{"name":"test-pipeline","description":"a test pipeline"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/pipelines", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Errorf("expected %d, got %d; body: %s", http.StatusCreated, rr.Code, rr.Body.String())
	}
}

func TestPipelineHandler_CreatePipeline_InvalidJSON(t *testing.T) {
	_, mux := setupPipelineHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/v1/pipelines", strings.NewReader("invalid"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected %d, got %d", http.StatusBadRequest, rr.Code)
	}
}
