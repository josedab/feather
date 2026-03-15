package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/feather-store/feather/internal/extensions/marketplace"
)

func setupBillingHandler(t *testing.T) *http.ServeMux {
	t.Helper()
	billing := marketplace.NewBillingEngine(marketplace.DefaultBillingConfig())
	handler := NewBillingHandler(billing)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	return mux
}

func TestBilling_GetStats(t *testing.T) {
	mux := setupBillingHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/billing/stats", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestBilling_PostCreatePlan(t *testing.T) {
	mux := setupBillingHandler(t)
	body := `{"id":"plan1","name":"basic","price_per_unit":999}`
	req := httptest.NewRequest(http.MethodPost, "/v1/billing/plans", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK && rr.Code != http.StatusCreated {
		t.Errorf("expected 200 or 201, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestBilling_PostInvalidJSON(t *testing.T) {
	mux := setupBillingHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/v1/billing/plans", io.NopCloser(strings.NewReader("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}
