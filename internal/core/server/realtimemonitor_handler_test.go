package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/feather-store/feather/internal/platform/realtimemonitor"
)

func newTestRealtimeMonitorHandler(t *testing.T) *http.ServeMux {
	t.Helper()
	dashboard := realtimemonitor.NewDashboard(realtimemonitor.DefaultDashboardConfig())
	handler := NewRealtimeMonitorHandler(dashboard)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	return mux
}

func TestRealtimeMonitorHandler_Dashboard(t *testing.T) {
	mux := newTestRealtimeMonitorHandler(t)

	req := httptest.NewRequest("GET", "/v1/monitor/dashboard", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("GET dashboard = %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestRealtimeMonitorHandler_RecordFreshness(t *testing.T) {
	mux := newTestRealtimeMonitorHandler(t)

	body := `{"feature":"user_age","group":"user_features"}`
	req := httptest.NewRequest("POST", "/v1/monitor/freshness", strings.NewReader(body))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("POST freshness = %d, want %d; body: %s", rr.Code, http.StatusOK, rr.Body.String())
	}
}

func TestRealtimeMonitorHandler_FireAlert(t *testing.T) {
	mux := newTestRealtimeMonitorHandler(t)

	body := `{"name":"high_latency","severity":"warning","message":"p99 > 100ms","source":"/v1/features"}`
	req := httptest.NewRequest("POST", "/v1/monitor/alerts", strings.NewReader(body))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Errorf("POST alert = %d, want %d; body: %s", rr.Code, http.StatusCreated, rr.Body.String())
	}
}

func TestRealtimeMonitorHandler_ListAlerts(t *testing.T) {
	mux := newTestRealtimeMonitorHandler(t)

	req := httptest.NewRequest("GET", "/v1/monitor/alerts", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("GET alerts = %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestRealtimeMonitorHandler_RecordLatency(t *testing.T) {
	mux := newTestRealtimeMonitorHandler(t)

	body := `{"endpoint":"/v1/features","latency_ms":5.2,"is_error":false}`
	req := httptest.NewRequest("POST", "/v1/monitor/latency", strings.NewReader(body))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("POST latency = %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestRealtimeMonitorHandler_PipelineHealth(t *testing.T) {
	mux := newTestRealtimeMonitorHandler(t)

	body := `{"pipeline_id":"p1","status":"healthy","events_per_sec":1000}`
	req := httptest.NewRequest("POST", "/v1/monitor/pipelines", strings.NewReader(body))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("POST pipeline health = %d, want %d", rr.Code, http.StatusOK)
	}
}
