package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/feather-store/feather/internal/extensions/featuredashboard"
)

func newTestFeatureDashboardHandler(t *testing.T) (*http.ServeMux, *featuredashboard.Dashboard) {
	t.Helper()
	d := featuredashboard.NewDashboard(featuredashboard.DefaultDashboardConfig())
	handler := NewFeatureDashboardHandler(d)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	return mux, d
}

func TestFeatureDashboardHandler_GetSnapshot(t *testing.T) {
	mux, _ := newTestFeatureDashboardHandler(t)

	req := httptest.NewRequest("GET", "/v1/dashboard/snapshot", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("GET /v1/dashboard/snapshot = %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestFeatureDashboardHandler_TakeSnapshot(t *testing.T) {
	mux, _ := newTestFeatureDashboardHandler(t)

	req := httptest.NewRequest("POST", "/v1/dashboard/snapshot", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("POST /v1/dashboard/snapshot = %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestFeatureDashboardHandler_GetHistory(t *testing.T) {
	mux, _ := newTestFeatureDashboardHandler(t)

	req := httptest.NewRequest("GET", "/v1/dashboard/history", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("GET /v1/dashboard/history = %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestFeatureDashboardHandler_FeatureNotFound(t *testing.T) {
	mux, _ := newTestFeatureDashboardHandler(t)

	req := httptest.NewRequest("GET", "/v1/dashboard/features/nonexistent", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Errorf("GET /v1/dashboard/features/nonexistent = %d, want %d", rr.Code, http.StatusNotFound)
	}
}
