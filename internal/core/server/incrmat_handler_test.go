package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/feather-store/feather/internal/extensions/incrmat"
)

type testIncrMatServer struct {
	handler *IncrMatHandler
	engine  *incrmat.Engine
	mux     *http.ServeMux
	t       *testing.T
}

func newTestIncrMatServer(t *testing.T) *testIncrMatServer {
	t.Helper()

	engine := incrmat.NewEngine(incrmat.DefaultEngineConfig())
	handler := NewIncrMatHandler(engine)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	return &testIncrMatServer{
		handler: handler,
		engine:  engine,
		mux:     mux,
		t:       t,
	}
}

func (ts *testIncrMatServer) request(method, path string, body string) *httptest.ResponseRecorder {
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

func (ts *testIncrMatServer) postJSON(path string, body interface{}) *httptest.ResponseRecorder {
	ts.t.Helper()

	jsonBody, err := json.Marshal(body)
	if err != nil {
		ts.t.Fatalf("failed to marshal body: %v", err)
	}
	return ts.request(http.MethodPost, path, string(jsonBody))
}

func (ts *testIncrMatServer) get(path string) *httptest.ResponseRecorder {
	return ts.request(http.MethodGet, path, "")
}

func TestIncrMatHandler_NewHandler(t *testing.T) {
	engine := incrmat.NewEngine(incrmat.DefaultEngineConfig())
	handler := NewIncrMatHandler(engine)

	if handler.engine == nil {
		t.Error("Expected engine to be set")
	}
}

func TestIncrMatHandler_ListNodes_Empty(t *testing.T) {
	ts := newTestIncrMatServer(t)

	rr := ts.get("/v1/materialization/nodes")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if result["nodes"] == nil {
		t.Error("Expected nodes array in response")
	}
}

func TestIncrMatHandler_RegisterNode(t *testing.T) {
	ts := newTestIncrMatServer(t)

	node := incrmat.MaterializationNode{
		ID:           "node-1",
		FeatureGroup: "user_features",
		Expression:   "sum(clicks)",
	}

	rr := ts.postJSON("/v1/materialization/nodes", node)

	if rr.Code != http.StatusCreated {
		t.Errorf("Expected status %d, got %d; body: %s", http.StatusCreated, rr.Code, rr.Body.String())
	}

	var result SuccessResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if !result.Success {
		t.Error("Expected success to be true")
	}
}

func TestIncrMatHandler_RegisterNode_InvalidBody(t *testing.T) {
	ts := newTestIncrMatServer(t)

	rr := ts.request(http.MethodPost, "/v1/materialization/nodes", "invalid json")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestIncrMatHandler_RemoveNode(t *testing.T) {
	ts := newTestIncrMatServer(t)

	node := incrmat.MaterializationNode{ID: "node-1", FeatureGroup: "user_features", Expression: "sum(clicks)"}
	ts.engine.RegisterNode(node)

	rr := ts.request(http.MethodDelete, "/v1/materialization/nodes/node-1", "")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d; body: %s", http.StatusOK, rr.Code, rr.Body.String())
	}

	var result SuccessResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if !result.Success {
		t.Error("Expected success to be true")
	}
}

func TestIncrMatHandler_RemoveNode_NotFound(t *testing.T) {
	ts := newTestIncrMatServer(t)

	rr := ts.request(http.MethodDelete, "/v1/materialization/nodes/nonexistent", "")

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, rr.Code)
	}
}

func TestIncrMatHandler_RecordChange(t *testing.T) {
	ts := newTestIncrMatServer(t)

	event := incrmat.ChangeEvent{
		EntityID:      "user:123",
		FeatureGroup:  "user_features",
		ChangedFields: []string{"click_rate"},
	}

	rr := ts.postJSON("/v1/materialization/changes", event)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d; body: %s", http.StatusOK, rr.Code, rr.Body.String())
	}

	var result SuccessResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if !result.Success {
		t.Error("Expected success to be true")
	}
}

func TestIncrMatHandler_RecordChange_InvalidBody(t *testing.T) {
	ts := newTestIncrMatServer(t)

	rr := ts.request(http.MethodPost, "/v1/materialization/changes", "invalid json")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestIncrMatHandler_GetDirtyNodes(t *testing.T) {
	ts := newTestIncrMatServer(t)

	rr := ts.get("/v1/materialization/dirty")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}
}

func TestIncrMatHandler_Materialize(t *testing.T) {
	ts := newTestIncrMatServer(t)

	rr := ts.postJSON("/v1/materialization/run", nil)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}
}

func TestIncrMatHandler_GetResults(t *testing.T) {
	ts := newTestIncrMatServer(t)

	rr := ts.get("/v1/materialization/results?limit=10")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}
}

func TestIncrMatHandler_GetStats(t *testing.T) {
	ts := newTestIncrMatServer(t)

	rr := ts.get("/v1/materialization/incr/stats")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}
}
