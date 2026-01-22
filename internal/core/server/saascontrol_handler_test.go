package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/feather-store/feather/internal/extensions/saascontrol"
)

func newTestSaaSControlHandler(t *testing.T) (*http.ServeMux, *saascontrol.ControlPlane) {
	t.Helper()
	cp := saascontrol.NewControlPlane(saascontrol.DefaultControlPlaneConfig())
	handler := NewSaaSControlHandler(cp)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	return mux, cp
}

func TestSaaSControlHandler_ListPlans(t *testing.T) {
	mux, _ := newTestSaaSControlHandler(t)

	req := httptest.NewRequest("GET", "/v1/saas/plans", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("GET /v1/saas/plans = %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestSaaSControlHandler_CreateTenant(t *testing.T) {
	mux, _ := newTestSaaSControlHandler(t)

	body := `{"id":"tenant-1","name":"Acme Corp","email":"admin@acme.com","plan_id":"starter"}`
	req := httptest.NewRequest("POST", "/v1/saas/tenants", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated && rr.Code != http.StatusOK {
		t.Errorf("POST /v1/saas/tenants = %d, want 200 or 201; body: %s", rr.Code, rr.Body.String())
	}
}

func TestSaaSControlHandler_InvalidJSON(t *testing.T) {
	mux, _ := newTestSaaSControlHandler(t)

	req := httptest.NewRequest("POST", "/v1/saas/tenants", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("POST with bad JSON = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestSaaSControlHandler_GetTenantNotFound(t *testing.T) {
	mux, _ := newTestSaaSControlHandler(t)

	req := httptest.NewRequest("GET", "/v1/saas/tenants/nonexistent", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Errorf("GET /v1/saas/tenants/nonexistent = %d, want %d", rr.Code, http.StatusNotFound)
	}
}
