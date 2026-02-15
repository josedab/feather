package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/feather-store/feather/internal/platform/autoscaler"
)

func setupAutoscalerHandler(t *testing.T) *http.ServeMux {
	t.Helper()
	scaler := autoscaler.NewAutoscaler(autoscaler.DefaultConfig())
	handler := NewAutoscalerHandler(scaler)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	return mux
}

func TestAutoscaler_GetStats(t *testing.T) {
	mux := setupAutoscalerHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/autoscaler/stats", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestAutoscaler_PostRecordMetric(t *testing.T) {
	mux := setupAutoscalerHandler(t)
	body := `{"metric":"cpu","value":75.5}`
	req := httptest.NewRequest(http.MethodPost, "/v1/autoscaler/metrics", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK && rr.Code != http.StatusCreated {
		t.Errorf("expected 200 or 201, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestAutoscaler_PostInvalidJSON(t *testing.T) {
	mux := setupAutoscalerHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/v1/autoscaler/metrics", io.NopCloser(strings.NewReader("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}
