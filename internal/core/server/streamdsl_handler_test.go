package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/feather-store/feather/internal/extensions/streamdsl"
)

func setupStreamDSLHandler(t *testing.T) *http.ServeMux {
	t.Helper()
	handler := NewStreamDSLHandler(streamdsl.NewPipelineManager(streamdsl.DefaultCompilerConfig()))
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	return mux
}

func TestStreamDSL_GetListPipelines(t *testing.T) {
	mux := setupStreamDSLHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/streamdsl/pipelines", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestStreamDSL_PostCompile(t *testing.T) {
	mux := setupStreamDSLHandler(t)
	body := `{"name":"pipeline1","source":"events","stages":[{"type":"filter","config":{"field":"type","value":"click"}}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/streamdsl/compile", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK && rr.Code != http.StatusCreated && rr.Code != http.StatusBadRequest {
		t.Errorf("expected 200, 201, or 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestStreamDSL_PostInvalidJSON(t *testing.T) {
	mux := setupStreamDSLHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/v1/streamdsl/compile", io.NopCloser(strings.NewReader("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}
