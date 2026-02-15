package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/feather-store/feather/internal/tools/compute"
)

func setupComputeHandler(t *testing.T) *http.ServeMux {
	t.Helper()
	engine := compute.NewComputeEngine(compute.DefaultComputeConfig())
	handler := NewComputeHandler(engine)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	return mux
}

func TestCompute_GetStats(t *testing.T) {
	mux := setupComputeHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/compute/stats", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestCompute_PostDefineFeature(t *testing.T) {
	mux := setupComputeHandler(t)
	body := `{"name":"avg_order","expression":"avg(order_total)","inputs":[],"output_type":"float64","mode":"batch"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/compute/definitions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK && rr.Code != http.StatusCreated {
		t.Errorf("expected 200 or 201, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestCompute_PostInvalidJSON(t *testing.T) {
	mux := setupComputeHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/v1/compute/definitions", io.NopCloser(strings.NewReader("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}
