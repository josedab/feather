package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/feather-store/feather/internal/integrations/flinkpipeline"
)

func newTestFlinkPipelineHandler(t *testing.T) *http.ServeMux {
	t.Helper()
	manager := flinkpipeline.NewManager(flinkpipeline.DefaultManagerConfig())
	handler := NewFlinkPipelineHandler(manager)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	return mux
}

func TestFlinkPipelineHandler_ListEmpty(t *testing.T) {
	mux := newTestFlinkPipelineHandler(t)

	req := httptest.NewRequest("GET", "/v1/pipelines/streaming", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("GET /v1/pipelines/streaming = %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestFlinkPipelineHandler_CreateAndGet(t *testing.T) {
	mux := newTestFlinkPipelineHandler(t)

	body := `{"id":"p1","name":"Test Pipeline","runtime":"kafka_streams","source":{"type":"kafka","topic":"events"},"sink":{"type":"feather","feature_group":"users"}}`
	req := httptest.NewRequest("POST", "/v1/pipelines/streaming", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Errorf("POST = %d, want %d; body: %s", rr.Code, http.StatusCreated, rr.Body.String())
	}

	req = httptest.NewRequest("GET", "/v1/pipelines/streaming/p1", nil)
	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("GET /v1/pipelines/streaming/p1 = %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestFlinkPipelineHandler_InvalidJSON(t *testing.T) {
	mux := newTestFlinkPipelineHandler(t)

	req := httptest.NewRequest("POST", "/v1/pipelines/streaming", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("POST with bad JSON = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestFlinkPipelineHandler_NotFound(t *testing.T) {
	mux := newTestFlinkPipelineHandler(t)

	req := httptest.NewRequest("GET", "/v1/pipelines/streaming/nonexistent", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Errorf("GET nonexistent = %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestFlinkPipelineHandler_StartStop(t *testing.T) {
	mux := newTestFlinkPipelineHandler(t)

	// Create
	body := `{"id":"p1","name":"Test","source":{"type":"kafka"},"sink":{"type":"feather"}}`
	req := httptest.NewRequest("POST", "/v1/pipelines/streaming", strings.NewReader(body))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	// Start
	req = httptest.NewRequest("POST", "/v1/pipelines/streaming/p1/start", nil)
	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("POST start = %d, want %d", rr.Code, http.StatusOK)
	}

	// Stop
	req = httptest.NewRequest("POST", "/v1/pipelines/streaming/p1/stop", nil)
	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("POST stop = %d, want %d", rr.Code, http.StatusOK)
	}
}
