package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/feather-store/feather/internal/platform/plugin"
)

func setupPluginHandler(t *testing.T) *http.ServeMux {
	t.Helper()
	handler := NewPluginHandler(plugin.NewRegistry(plugin.DefaultRegistryConfig()))
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	return mux
}

func TestPlugin_GetListPlugins(t *testing.T) {
	mux := setupPluginHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/plugins", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestPlugin_PostRegisterPlugin(t *testing.T) {
	mux := setupPluginHandler(t)
	body := `{"id":"plugin1","name":"test-plugin","version":"1.0.0"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/plugins", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK && rr.Code != http.StatusCreated {
		t.Errorf("expected 200 or 201, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestPlugin_PostInvalidJSON(t *testing.T) {
	mux := setupPluginHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/v1/plugins", io.NopCloser(strings.NewReader("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}
