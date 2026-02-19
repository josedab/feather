package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/feather-store/feather/internal/extensions/lifecycle"
)

func setupLifecycleHandler(t *testing.T) *http.ServeMux {
	t.Helper()
	manager := lifecycle.NewManager(lifecycle.DefaultManagerConfig())
	handler := NewLifecycleManagerHandler(manager)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	return mux
}

func TestLifecycle_ListFeatures(t *testing.T) {
	mux := setupLifecycleHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/lifecycle/features", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestLifecycle_GetFeatureNotFound(t *testing.T) {
	mux := setupLifecycleHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/lifecycle/features/nonexistent", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestLifecycle_TrackFeature(t *testing.T) {
	mux := setupLifecycleHandler(t)
	body := `{"name":"user_age"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/lifecycle/features/track", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestLifecycle_TrackFeatureMissingName(t *testing.T) {
	mux := setupLifecycleHandler(t)
	body := `{"name":""}`
	req := httptest.NewRequest(http.MethodPost, "/v1/lifecycle/features/track", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestLifecycle_TrackFeatureInvalidJSON(t *testing.T) {
	mux := setupLifecycleHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/v1/lifecycle/features/track", io.NopCloser(strings.NewReader("bad")))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestLifecycle_RecordAccess(t *testing.T) {
	mux := setupLifecycleHandler(t)
	body := `{"feature":"user_age","consumer":"model-1"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/lifecycle/features/access", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestLifecycle_RecordAccessInvalidJSON(t *testing.T) {
	mux := setupLifecycleHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/v1/lifecycle/features/access", strings.NewReader("bad"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestLifecycle_UpdateMetricsInvalidJSON(t *testing.T) {
	mux := setupLifecycleHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/v1/lifecycle/features/metrics", strings.NewReader("bad"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestLifecycle_ListRules(t *testing.T) {
	mux := setupLifecycleHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/lifecycle/rules", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestLifecycle_AddRuleInvalidJSON(t *testing.T) {
	mux := setupLifecycleHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/v1/lifecycle/rules", strings.NewReader("bad"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestLifecycle_RemoveRuleNotFound(t *testing.T) {
	mux := setupLifecycleHandler(t)
	req := httptest.NewRequest(http.MethodDelete, "/v1/lifecycle/rules/nonexistent", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestLifecycle_Evaluate(t *testing.T) {
	mux := setupLifecycleHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/v1/lifecycle/evaluate", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestLifecycle_GetEvents(t *testing.T) {
	mux := setupLifecycleHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/lifecycle/events?limit=10", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestLifecycle_CostReport(t *testing.T) {
	mux := setupLifecycleHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/lifecycle/cost-report?top=5", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestLifecycle_Stats(t *testing.T) {
	mux := setupLifecycleHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/lifecycle/stats", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}
