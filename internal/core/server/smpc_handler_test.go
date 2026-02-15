package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/feather-store/feather/internal/platform/federation"
)

func setupSMPCHandler(t *testing.T) *http.ServeMux {
	t.Helper()
	handler := NewSMPCHandler(federation.NewSMPCEngine(federation.DefaultSMPCConfig()))
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	return mux
}

func TestSMPC_GetListParties(t *testing.T) {
	mux := setupSMPCHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/smpc/parties", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestSMPC_PostRegisterParty(t *testing.T) {
	mux := setupSMPCHandler(t)
	body := `{"id":"party1","name":"Alice","endpoint":"https://alice.example.com"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/smpc/parties", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK && rr.Code != http.StatusCreated {
		t.Errorf("expected 200 or 201, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestSMPC_PostInvalidJSON(t *testing.T) {
	mux := setupSMPCHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/v1/smpc/parties", io.NopCloser(strings.NewReader("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}
