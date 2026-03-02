package server

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/feather-store/feather/internal/platform/controlplane"
)

func setupControlPlaneHandler(t *testing.T) *http.ServeMux {
	t.Helper()
	manager := controlplane.NewManager(context.Background(), controlplane.DefaultManagerConfig())
	handler := NewControlPlaneHandler(manager)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	return mux
}

func TestControlPlane_GetFleetStatus(t *testing.T) {
	mux := setupControlPlaneHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/controlplane/fleet/status", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestControlPlane_PostAddRegion(t *testing.T) {
	mux := setupControlPlaneHandler(t)
	body := `{"name":"us-east-1","provider":"aws","primary":true}`
	req := httptest.NewRequest(http.MethodPost, "/v1/controlplane/regions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK && rr.Code != http.StatusCreated {
		t.Errorf("expected 200 or 201, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestControlPlane_PostInvalidJSON(t *testing.T) {
	mux := setupControlPlaneHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/v1/controlplane/regions", io.NopCloser(strings.NewReader("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}
