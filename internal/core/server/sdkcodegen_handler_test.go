package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/feather-store/feather/internal/extensions/sdkcodegen"
)

func newTestSDKCodegenHandler(t *testing.T) (*http.ServeMux, *sdkcodegen.Generator) {
	t.Helper()
	gen := sdkcodegen.NewGenerator(sdkcodegen.DefaultGeneratorConfig())
	handler := NewSDKCodegenHandler(gen)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	return mux, gen
}

func TestSDKCodegenHandler_ListSchemas(t *testing.T) {
	mux, _ := newTestSDKCodegenHandler(t)

	req := httptest.NewRequest("GET", "/v1/codegen/schemas", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("GET /v1/codegen/schemas = %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestSDKCodegenHandler_RegisterSchema(t *testing.T) {
	mux, _ := newTestSDKCodegenHandler(t)

	body := `{"name":"user_features","entity_type":"user","version":"1.0","fields":[{"name":"age","type":"int64"}]}`
	req := httptest.NewRequest("POST", "/v1/codegen/schemas", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated && rr.Code != http.StatusOK {
		t.Errorf("POST /v1/codegen/schemas = %d, want 200 or 201; body: %s", rr.Code, rr.Body.String())
	}
}

func TestSDKCodegenHandler_InvalidJSON(t *testing.T) {
	mux, _ := newTestSDKCodegenHandler(t)

	req := httptest.NewRequest("POST", "/v1/codegen/schemas", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("POST with bad JSON = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestSDKCodegenHandler_History(t *testing.T) {
	mux, _ := newTestSDKCodegenHandler(t)

	req := httptest.NewRequest("GET", "/v1/codegen/history", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("GET /v1/codegen/history = %d, want %d", rr.Code, http.StatusOK)
	}
}
