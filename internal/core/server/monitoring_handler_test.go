package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/feather-store/feather/internal/platform/monitoring"
)

func setupMonitoringHandler(t *testing.T) (*MonitoringHandler, *http.ServeMux) {
	t.Helper()
	manager := monitoring.NewManager(monitoring.DefaultManagerConfig())
	handler := NewMonitoringHandler(manager)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	return handler, mux
}

func TestMonitoringHandler_ListMonitors(t *testing.T) {
	_, mux := setupMonitoringHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/monitoring/monitors", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected %d, got %d", http.StatusOK, rr.Code)
	}
	var result map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if result["success"] != true {
		t.Error("expected success=true")
	}
}

func TestMonitoringHandler_RegisterMonitor(t *testing.T) {
	_, mux := setupMonitoringHandler(t)
	body := `{"id":"mon1","feature_name":"clicks","type":"numeric"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/monitoring/monitors", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Errorf("expected %d, got %d; body: %s", http.StatusCreated, rr.Code, rr.Body.String())
	}
}

func TestMonitoringHandler_RegisterMonitor_InvalidJSON(t *testing.T) {
	_, mux := setupMonitoringHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/v1/monitoring/monitors", strings.NewReader("invalid"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected %d, got %d", http.StatusBadRequest, rr.Code)
	}
}
