package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/feather-store/feather/internal/platform/finops"
)

func setupFinOpsHandler(t *testing.T) (*FinOpsHandler, *http.ServeMux) {
	t.Helper()
	manager := finops.NewManager(finops.DefaultManagerConfig())
	handler := NewFinOpsHandler(manager)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	return handler, mux
}

func TestFinOpsHandler_ListTeams(t *testing.T) {
	_, mux := setupFinOpsHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/finops/teams", nil)
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

func TestFinOpsHandler_RegisterTeam(t *testing.T) {
	_, mux := setupFinOpsHandler(t)
	body := `{"id":"team1","name":"ML Team","budget":1000}`
	req := httptest.NewRequest(http.MethodPost, "/v1/finops/teams", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Errorf("expected %d, got %d; body: %s", http.StatusCreated, rr.Code, rr.Body.String())
	}
}

func TestFinOpsHandler_RegisterTeam_InvalidJSON(t *testing.T) {
	_, mux := setupFinOpsHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/v1/finops/teams", strings.NewReader("invalid"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected %d, got %d", http.StatusBadRequest, rr.Code)
	}
}
