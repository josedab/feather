package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/feather-store/feather/internal/extensions/modelserving"
)

func setupModelGatewayHandler(t *testing.T) *http.ServeMux {
	t.Helper()
	registry := modelserving.NewRegistry(modelserving.DefaultRegistryConfig())
	gateway := modelserving.NewGateway(registry)
	handler := NewModelGatewayHandler(gateway)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	return mux
}

func TestModelGateway_PredictMissingModelID(t *testing.T) {
	mux := setupModelGatewayHandler(t)
	body := `{"features":{"age":25}}`
	req := httptest.NewRequest(http.MethodPost, "/v1/predict", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestModelGateway_PredictInvalidJSON(t *testing.T) {
	mux := setupModelGatewayHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/v1/predict", io.NopCloser(strings.NewReader("bad")))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestModelGateway_ListAdapters(t *testing.T) {
	mux := setupModelGatewayHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/predict/adapters", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestModelGateway_SetABConfig(t *testing.T) {
	mux := setupModelGatewayHandler(t)
	body := `{"model_id":"model-1","config":{"model_a":"v1","model_b":"v2","traffic_pct_b":50,"enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/v1/predict/ab-config", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestModelGateway_SetABConfigMissingModelID(t *testing.T) {
	mux := setupModelGatewayHandler(t)
	body := `{"config":{"model_a":"v1","model_b":"v2"}}`
	req := httptest.NewRequest(http.MethodPost, "/v1/predict/ab-config", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestModelGateway_SetABConfigInvalidJSON(t *testing.T) {
	mux := setupModelGatewayHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/v1/predict/ab-config", strings.NewReader("bad"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestModelGateway_GetABConfigNotFound(t *testing.T) {
	mux := setupModelGatewayHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/predict/ab-config/nonexistent", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestModelGateway_Stats(t *testing.T) {
	mux := setupModelGatewayHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/predict/stats", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}
