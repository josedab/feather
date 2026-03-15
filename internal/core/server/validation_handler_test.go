package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/feather-store/feather/internal/platform/validation"
)

func setupValidationHandler(t *testing.T) *http.ServeMux {
	t.Helper()
	handler := NewValidationHandler(validation.NewValidator(validation.DefaultValidatorConfig()))
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	return mux
}

func TestValidation_GetListRules(t *testing.T) {
	mux := setupValidationHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/validation/rules", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestValidation_PostAddRule(t *testing.T) {
	mux := setupValidationHandler(t)
	body := `{"name":"rule1","feature":"age","compare_method":"absolute","tolerance":0.01,"sample_rate":1.0,"enabled":true}`
	req := httptest.NewRequest(http.MethodPost, "/v1/validation/rules", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK && rr.Code != http.StatusCreated {
		t.Errorf("expected 200 or 201, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestValidation_PostInvalidJSON(t *testing.T) {
	mux := setupValidationHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/v1/validation/rules", io.NopCloser(strings.NewReader("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}
