package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/feather-store/feather/internal/platform/federation"
)

func setupCrossOrgHandler(t *testing.T) *http.ServeMux {
	t.Helper()
	fed := federation.NewCrossOrgFederation(federation.DefaultCrossOrgConfig())
	handler := NewCrossOrgFederationHandler(fed)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	return mux
}

func TestCrossOrg_ListOrgs(t *testing.T) {
	mux := setupCrossOrgHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/federation/orgs", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestCrossOrg_RegisterOrg(t *testing.T) {
	mux := setupCrossOrgHandler(t)
	body := `{"id":"org1","name":"Test Org","trust_level":"full"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/federation/orgs", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestCrossOrg_RegisterOrgInvalidJSON(t *testing.T) {
	mux := setupCrossOrgHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/v1/federation/orgs", strings.NewReader("invalid"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestCrossOrg_ListAgreements(t *testing.T) {
	mux := setupCrossOrgHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/federation/agreements", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestCrossOrg_CreateAgreementInvalidJSON(t *testing.T) {
	mux := setupCrossOrgHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/v1/federation/agreements", strings.NewReader("bad"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestCrossOrg_ProcessRequestInvalidJSON(t *testing.T) {
	mux := setupCrossOrgHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/v1/federation/request", io.NopCloser(strings.NewReader("invalid")))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestCrossOrg_GetPrivacyBudgetNotFound(t *testing.T) {
	mux := setupCrossOrgHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/federation/privacy/nonexistent", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestCrossOrg_GetAuditLog(t *testing.T) {
	mux := setupCrossOrgHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/federation/audit?limit=10", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestCrossOrg_Stats(t *testing.T) {
	mux := setupCrossOrgHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/federation/crossorg/stats", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}
