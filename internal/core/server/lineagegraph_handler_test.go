package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/feather-store/feather/internal/extensions/lineagegraph"
)

type testLineageGraphServer struct {
	handler *LineageGraphHandler
	graph   *lineagegraph.Graph
	mux     *http.ServeMux
	t       *testing.T
}

func newTestLineageGraphServer(t *testing.T) *testLineageGraphServer {
	t.Helper()
	graph := lineagegraph.NewGraph(lineagegraph.DefaultGraphConfig())
	handler := NewLineageGraphHandler(graph)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	return &testLineageGraphServer{handler: handler, graph: graph, mux: mux, t: t}
}

func (ts *testLineageGraphServer) request(method, path string, body string) *httptest.ResponseRecorder {
	ts.t.Helper()
	var bodyReader io.Reader
	if body != "" {
		bodyReader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, bodyReader)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	rr := httptest.NewRecorder()
	ts.mux.ServeHTTP(rr, req)
	return rr
}

func TestLineageGraphHandler_GetStats(t *testing.T) {
	ts := newTestLineageGraphServer(t)
	rr := ts.request(http.MethodGet, "/v1/lineage/stats", "")
	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}
	var result map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}
}

func TestLineageGraphHandler_AddNode(t *testing.T) {
	ts := newTestLineageGraphServer(t)
	body := `{"id":"feat_1","type":"feature","name":"click_rate"}`
	rr := ts.request(http.MethodPost, "/v1/lineage/nodes", body)
	if rr.Code != http.StatusCreated {
		t.Errorf("Expected status %d, got %d; body: %s", http.StatusCreated, rr.Code, rr.Body.String())
	}
	var result map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}
	if result["success"] != true {
		t.Error("Expected success to be true")
	}
}

func TestLineageGraphHandler_AddNode_InvalidJSON(t *testing.T) {
	ts := newTestLineageGraphServer(t)
	rr := ts.request(http.MethodPost, "/v1/lineage/nodes", "not json")
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestLineageGraphHandler_GetNode_NotFound(t *testing.T) {
	ts := newTestLineageGraphServer(t)
	rr := ts.request(http.MethodGet, "/v1/lineage/nodes/nonexistent", "")
	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, rr.Code)
	}
}
