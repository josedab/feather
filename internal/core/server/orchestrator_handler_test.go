package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func setupOrchestratorHandler(t *testing.T) (*OrchestratorHandler, *http.ServeMux) {
	t.Helper()
	handler := NewOrchestratorHandler(nil)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	return handler, mux
}

func TestOrchestratorHandler_ListDAGs(t *testing.T) {
	_, mux := setupOrchestratorHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/orchestrator/dags", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected %d, got %d", http.StatusOK, rr.Code)
	}
	var result map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
}

func TestOrchestratorHandler_CreateDAG(t *testing.T) {
	_, mux := setupOrchestratorHandler(t)
	body := `{"name":"test-dag","nodes":[{"id":"n1","name":"step1","type":"transform"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/orchestrator/dags", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Errorf("expected %d, got %d; body: %s", http.StatusCreated, rr.Code, rr.Body.String())
	}
}

func TestOrchestratorHandler_CreateDAG_InvalidJSON(t *testing.T) {
	_, mux := setupOrchestratorHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/v1/orchestrator/dags", strings.NewReader("invalid"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected %d, got %d", http.StatusBadRequest, rr.Code)
	}
}
