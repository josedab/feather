package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/feather-store/feather/internal/platform/cloudcontrol"
)

func newTestCloudControlHandler(t *testing.T) *http.ServeMux {
	t.Helper()
	cp := cloudcontrol.NewControlPlane(cloudcontrol.DefaultControlPlaneConfig())
	handler := NewCloudControlHandler(cp)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	return mux
}

func TestCloudControlHandler_CreateTenant(t *testing.T) {
	mux := newTestCloudControlHandler(t)

	body := `{"id":"t1","name":"Acme Corp"}`
	req := httptest.NewRequest("POST", "/v1/cloud/tenants", strings.NewReader(body))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Errorf("POST tenant = %d, want %d; body: %s", rr.Code, http.StatusCreated, rr.Body.String())
	}
}

func TestCloudControlHandler_ListTenants(t *testing.T) {
	mux := newTestCloudControlHandler(t)

	req := httptest.NewRequest("GET", "/v1/cloud/tenants", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("GET tenants = %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestCloudControlHandler_ProvisionInstance(t *testing.T) {
	mux := newTestCloudControlHandler(t)

	// Create tenant first
	body := `{"id":"t1","name":"Acme"}`
	req := httptest.NewRequest("POST", "/v1/cloud/tenants", strings.NewReader(body))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	// Provision instance
	body = `{"id":"i1","name":"prod-1","tenant_id":"t1","region":"us-west-2"}`
	req = httptest.NewRequest("POST", "/v1/cloud/instances", strings.NewReader(body))
	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Errorf("POST instance = %d, want %d; body: %s", rr.Code, http.StatusCreated, rr.Body.String())
	}
}

func TestCloudControlHandler_InstanceNotFound(t *testing.T) {
	mux := newTestCloudControlHandler(t)

	req := httptest.NewRequest("GET", "/v1/cloud/instances/nonexistent", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Errorf("GET nonexistent = %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestCloudControlHandler_Stats(t *testing.T) {
	mux := newTestCloudControlHandler(t)

	req := httptest.NewRequest("GET", "/v1/cloud/control/stats", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("GET stats = %d, want %d", rr.Code, http.StatusOK)
	}
}
