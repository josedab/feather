package server

import (
	"context"
	"net/http"
	"testing"
)

func TestHTTPServer_HealthEndpoints(t *testing.T) {
	ts := newTestServer(t)

	tests := []struct {
		name       string
		path       string
		wantStatus int
	}{
		{
			name:       "health endpoint",
			path:       "/health",
			wantStatus: http.StatusOK,
		},
		{
			name:       "ready endpoint",
			path:       "/ready",
			wantStatus: http.StatusOK,
		},
		{
			name:       "live endpoint",
			path:       "/live",
			wantStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rr := ts.get(tt.path)
			assertStatus(t, rr, tt.wantStatus)
		})
	}
}

func TestHTTPServer_HealthResponse(t *testing.T) {
	ts := newTestServer(t)

	rr := ts.get("/health")
	assertStatus(t, rr, http.StatusOK)
	assertContentType(t, rr, "application/json")

	result := assertJSON(t, rr)

	// Check expected fields
	if _, ok := result["status"]; !ok {
		t.Error("expected 'status' field in health response")
	}

	if _, ok := result["components"]; !ok {
		t.Error("expected 'components' field in health response")
	}
}

func TestHTTPServer_ReadyResponse(t *testing.T) {
	ts := newTestServer(t)

	rr := ts.get("/ready")
	assertStatus(t, rr, http.StatusOK)
	assertContentType(t, rr, "application/json")

	result := assertJSON(t, rr)

	if status, ok := result["status"].(string); !ok || status != "ready" {
		t.Errorf("expected status='ready', got %v", result["status"])
	}
}

func TestHTTPServer_LiveResponse(t *testing.T) {
	ts := newTestServer(t)

	rr := ts.get("/live")
	assertStatus(t, rr, http.StatusOK)
	assertContentType(t, rr, "application/json")

	result := assertJSON(t, rr)

	if status, ok := result["status"].(string); !ok || status != "alive" {
		t.Errorf("expected status='alive', got %v", result["status"])
	}
}

func TestHealthChecker_Check(t *testing.T) {
	ts := newTestServer(t)

	status := ts.healthChecker.Check(context.Background())

	if status.Status != HealthStatusHealthy && status.Status != HealthStatusDegraded {
		t.Errorf("unexpected health status: %s", status.Status)
	}

	if len(status.Components) == 0 {
		t.Error("expected at least one component in health check")
	}
}

func TestHealthChecker_IsReady(t *testing.T) {
	ts := newTestServer(t)

	// Fresh server should be ready
	if !ts.healthChecker.IsReady() {
		t.Error("expected health checker to report ready")
	}
}

func TestHealthChecker_LivenessCheck(t *testing.T) {
	ts := newTestServer(t)

	// Fresh server should be alive
	if !ts.healthChecker.LivenessCheck() {
		t.Error("expected health checker to report alive")
	}
}
