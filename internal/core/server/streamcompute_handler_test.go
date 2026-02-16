package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/feather-store/feather/internal/extensions/streamcompute"
)

func newTestStreamComputeHandler(t *testing.T) (*http.ServeMux, *streamcompute.Engine) {
	t.Helper()
	engine := streamcompute.NewEngine(streamcompute.DefaultEngineConfig())
	handler := NewStreamComputeHandler(engine)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	return mux, engine
}

func TestStreamComputeHandler_ListPipelines(t *testing.T) {
	mux, _ := newTestStreamComputeHandler(t)

	req := httptest.NewRequest("GET", "/v1/stream/pipelines", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("GET /v1/stream/pipelines = %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestStreamComputeHandler_Stats(t *testing.T) {
	mux, _ := newTestStreamComputeHandler(t)

	req := httptest.NewRequest("GET", "/v1/stream/stats", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("GET /v1/stream/stats = %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestStreamComputeHandler_CreatePipeline(t *testing.T) {
	mux, _ := newTestStreamComputeHandler(t)

	body := `{"id":"test-pipe","window_type":"tumbling","window_size":"1m","aggregation":"sum"}`
	req := httptest.NewRequest("POST", "/v1/stream/pipelines", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated && rr.Code != http.StatusOK {
		t.Errorf("POST /v1/stream/pipelines = %d, want 200 or 201; body: %s", rr.Code, rr.Body.String())
	}
}

func TestStreamComputeHandler_InvalidJSON(t *testing.T) {
	mux, _ := newTestStreamComputeHandler(t)

	req := httptest.NewRequest("POST", "/v1/stream/pipelines", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("POST with bad JSON = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestStreamComputeHandler_GetPipelineNotFound(t *testing.T) {
	mux, _ := newTestStreamComputeHandler(t)

	req := httptest.NewRequest("GET", "/v1/stream/pipelines/nonexistent", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Errorf("GET /v1/stream/pipelines/nonexistent = %d, want %d", rr.Code, http.StatusNotFound)
	}
}
