package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/feather-store/feather/internal/extensions/computegraph"
)

func setupComputeGraphHandler(t *testing.T) *http.ServeMux {
	t.Helper()
	engine := computegraph.NewEngine(computegraph.DefaultEngineConfig())
	handler := NewComputeGraphHandler(engine)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	return mux
}

func TestComputeGraph_GetStats(t *testing.T) {
	mux := setupComputeGraphHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/graph/stats", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestComputeGraph_PostAddNode(t *testing.T) {
	mux := setupComputeGraphHandler(t)
	body := `{"name":"node1","expression":"sum(clicks)"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/graph/nodes", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK && rr.Code != http.StatusCreated {
		t.Errorf("expected 200 or 201, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestComputeGraph_PostInvalidJSON(t *testing.T) {
	mux := setupComputeGraphHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/v1/graph/nodes", io.NopCloser(strings.NewReader("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}
