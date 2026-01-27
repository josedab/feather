package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/feather-store/feather/internal/platform/multiregion"
)

func newTestRegionFederationHandler(t *testing.T) *http.ServeMux {
	t.Helper()
	fed := multiregion.NewFederation(multiregion.DefaultFederationConfig())
	handler := NewRegionFederationHandler(fed)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	return mux
}

func TestRegionFederationHandler_Replicate(t *testing.T) {
	mux := newTestRegionFederationHandler(t)

	body := `{"from_region":"eu-west-1","entity":"e1","feature":"age","version":1}`
	req := httptest.NewRequest("POST", "/v1/federation/replicate", strings.NewReader(body))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("POST replicate = %d, want %d; body: %s", rr.Code, http.StatusOK, rr.Body.String())
	}
}

func TestRegionFederationHandler_Apply(t *testing.T) {
	mux := newTestRegionFederationHandler(t)

	body := `{"id":"r1","from_region":"eu-west-1","entity":"e1","version":1}`
	req := httptest.NewRequest("POST", "/v1/federation/apply", strings.NewReader(body))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("POST apply = %d, want %d; body: %s", rr.Code, http.StatusOK, rr.Body.String())
	}
}

func TestRegionFederationHandler_Conflicts(t *testing.T) {
	mux := newTestRegionFederationHandler(t)

	req := httptest.NewRequest("GET", "/v1/federation/conflicts", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("GET conflicts = %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestRegionFederationHandler_Stats(t *testing.T) {
	mux := newTestRegionFederationHandler(t)

	req := httptest.NewRequest("GET", "/v1/federation/replication/stats", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("GET stats = %d, want %d", rr.Code, http.StatusOK)
	}
}
