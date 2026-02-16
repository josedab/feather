package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/feather-store/feather/internal/extensions/featherqlv2"
)

func newTestFeatherQLv2Handler(t *testing.T) (*http.ServeMux, *featherqlv2.Engine) {
	t.Helper()
	engine := featherqlv2.NewEngine(featherqlv2.DefaultEngineConfig())
	handler := NewFeatherQLv2Handler(engine)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	return mux, engine
}

func TestFeatherQLv2Handler_ListPipelines(t *testing.T) {
	mux, _ := newTestFeatherQLv2Handler(t)

	req := httptest.NewRequest("GET", "/v1/featherql/v2/pipelines", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("GET /v1/featherql/v2/pipelines = %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestFeatherQLv2Handler_Compile(t *testing.T) {
	mux, _ := newTestFeatherQLv2Handler(t)

	body := `{"id":"test-pipe","query":"SELECT feature FROM users"}`
	req := httptest.NewRequest("POST", "/v1/featherql/v2/compile", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated && rr.Code != http.StatusOK {
		t.Errorf("POST /v1/featherql/v2/compile = %d, want 200 or 201; body: %s", rr.Code, rr.Body.String())
	}
}

func TestFeatherQLv2Handler_InvalidJSON(t *testing.T) {
	mux, _ := newTestFeatherQLv2Handler(t)

	req := httptest.NewRequest("POST", "/v1/featherql/v2/parse", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("POST with bad JSON = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestFeatherQLv2Handler_GetPipelineNotFound(t *testing.T) {
	mux, _ := newTestFeatherQLv2Handler(t)

	req := httptest.NewRequest("GET", "/v1/featherql/v2/pipelines/nonexistent", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Errorf("GET /v1/featherql/v2/pipelines/nonexistent = %d, want %d", rr.Code, http.StatusNotFound)
	}
}
