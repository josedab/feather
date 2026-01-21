package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/feather-store/feather/internal/platform/parity"
)

func setupParityHandler(t *testing.T) (*ParityHandler, *http.ServeMux) {
	t.Helper()
	checker := parity.NewChecker(parity.DefaultConfig())
	handler := NewParityHandler(checker)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	return handler, mux
}

func TestParityHandler_GetAllStatuses(t *testing.T) {
	_, mux := setupParityHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/parity/status", nil)
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

func TestParityHandler_RecordPair(t *testing.T) {
	_, mux := setupParityHandler(t)
	body := `{"feature":"clicks","entity_key":"user1","online_value":10,"offline_value":10}`
	req := httptest.NewRequest(http.MethodPost, "/v1/parity/record", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected %d, got %d; body: %s", http.StatusOK, rr.Code, rr.Body.String())
	}
}

func TestParityHandler_RecordPair_InvalidJSON(t *testing.T) {
	_, mux := setupParityHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/v1/parity/record", strings.NewReader("invalid"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected %d, got %d", http.StatusBadRequest, rr.Code)
	}
}
