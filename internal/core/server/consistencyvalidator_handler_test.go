package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/feather-store/feather/internal/extensions/consistencyvalidator"
)

func newTestConsistencyValidatorHandler(t *testing.T) (*http.ServeMux, *consistencyvalidator.Validator) {
	t.Helper()
	v := consistencyvalidator.NewValidator(consistencyvalidator.DefaultValidatorConfig())
	handler := NewConsistencyValidatorHandler(v)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	return mux, v
}

func TestConsistencyValidatorHandler_ListFeatures(t *testing.T) {
	mux, _ := newTestConsistencyValidatorHandler(t)

	req := httptest.NewRequest("GET", "/v1/consistency/features", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("GET /v1/consistency/features = %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestConsistencyValidatorHandler_Register(t *testing.T) {
	mux, _ := newTestConsistencyValidatorHandler(t)

	body := `{"name":"click_rate"}`
	req := httptest.NewRequest("POST", "/v1/consistency/register", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated && rr.Code != http.StatusOK {
		t.Errorf("POST /v1/consistency/register = %d, want 200 or 201; body: %s", rr.Code, rr.Body.String())
	}
}

func TestConsistencyValidatorHandler_InvalidJSON(t *testing.T) {
	mux, _ := newTestConsistencyValidatorHandler(t)

	req := httptest.NewRequest("POST", "/v1/consistency/register", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("POST with bad JSON = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestConsistencyValidatorHandler_CheckFeatureNotFound(t *testing.T) {
	mux, _ := newTestConsistencyValidatorHandler(t)

	req := httptest.NewRequest("GET", "/v1/consistency/check/nonexistent", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Errorf("GET /v1/consistency/check/nonexistent = %d, want %d", rr.Code, http.StatusNotFound)
	}
}
