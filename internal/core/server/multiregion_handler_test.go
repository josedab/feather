package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/feather-store/feather/internal/platform/multiregion"
)

func setupMultiRegionHandler(t *testing.T) (*MultiRegionHandler, *http.ServeMux) {
	t.Helper()
	federation := multiregion.NewFederation(multiregion.DefaultFederationConfig())
	handler := NewMultiRegionHandler(federation)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	return handler, mux
}

func TestMultiRegionHandler_ListRegions(t *testing.T) {
	_, mux := setupMultiRegionHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/federation/regions", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected %d, got %d", http.StatusOK, rr.Code)
	}
	var result map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
}

func TestMultiRegionHandler_AddRegion(t *testing.T) {
	_, mux := setupMultiRegionHandler(t)
	body := `{"name":"us-east-1","endpoint":"http://localhost:8080","cloud":"aws"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/federation/regions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Errorf("expected %d, got %d; body: %s", http.StatusCreated, rr.Code, rr.Body.String())
	}
}

func TestMultiRegionHandler_AddRegion_InvalidJSON(t *testing.T) {
	_, mux := setupMultiRegionHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/v1/federation/regions", strings.NewReader("invalid"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected %d, got %d", http.StatusBadRequest, rr.Code)
	}
}
