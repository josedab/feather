package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/feather-store/feather/internal/platform/contract"
)

func setupContractHandler(t *testing.T) *http.ServeMux {
	t.Helper()
	manager := contract.NewManager(contract.DefaultManagerConfig(), nil)
	handler := NewContractHandler(manager)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	return mux
}

func TestContract_GetList(t *testing.T) {
	mux := setupContractHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/contracts", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestContract_PostCreate(t *testing.T) {
	mux := setupContractHandler(t)
	body := `{"name":"test-contract","feature_group":"user_features","rules":[{"type":"freshness"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/contracts", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK && rr.Code != http.StatusCreated {
		t.Errorf("expected 200 or 201, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestContract_PostInvalidJSON(t *testing.T) {
	mux := setupContractHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/v1/contracts", io.NopCloser(strings.NewReader("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}
