package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/feather-store/feather/internal/platform/cloud"
)

func setupCloudHandler(t *testing.T) *http.ServeMux {
	t.Helper()
	cp := cloud.NewControlPlane(cloud.DefaultConfig())
	handler := NewCloudHandler(cp)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	return mux
}

func TestCloud_GetStats(t *testing.T) {
	mux := setupCloudHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/cloud/stats", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestCloud_PostProvision(t *testing.T) {
	mux := setupCloudHandler(t)
	body := `{"name":"my-instance","tenant_id":"tenant1","tier":"starter","region":"us-east-1"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/cloud/instances", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK && rr.Code != http.StatusCreated {
		t.Errorf("expected 200 or 201, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestCloud_PostInvalidJSON(t *testing.T) {
	mux := setupCloudHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/v1/cloud/instances", io.NopCloser(strings.NewReader("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}
