package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/feather-store/feather/internal/extensions/composition"
)

// testCompositionServer wraps a CompositionHandler for testing.
type testCompositionServer struct {
	handler *CompositionHandler
	mux     *http.ServeMux
	t       *testing.T
}

// newTestCompositionServer creates a new test composition server.
func newTestCompositionServer(t *testing.T) *testCompositionServer {
	t.Helper()

	engine := composition.NewEngine(composition.EngineConfig{
		ExecutorConfig: composition.DefaultExecutorConfig(),
	})

	handler := NewCompositionHandler(engine)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	return &testCompositionServer{
		handler: handler,
		mux:     mux,
		t:       t,
	}
}

// newTestCompositionServerNoEngine creates a test server with nil engine.
func newTestCompositionServerNoEngine(t *testing.T) *testCompositionServer {
	t.Helper()

	handler := NewCompositionHandler(nil)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	return &testCompositionServer{
		handler: handler,
		mux:     mux,
		t:       t,
	}
}

func (ts *testCompositionServer) request(method, path string, body string) *httptest.ResponseRecorder {
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

func (ts *testCompositionServer) postJSON(path string, body interface{}) *httptest.ResponseRecorder {
	ts.t.Helper()

	jsonBody, err := json.Marshal(body)
	if err != nil {
		ts.t.Fatalf("failed to marshal body: %v", err)
	}
	return ts.request(http.MethodPost, path, string(jsonBody))
}

func (ts *testCompositionServer) get(path string) *httptest.ResponseRecorder {
	return ts.request(http.MethodGet, path, "")
}

func (ts *testCompositionServer) delete(path string) *httptest.ResponseRecorder {
	return ts.request(http.MethodDelete, path, "")
}

func TestCompositionHandler_NewHandler(t *testing.T) {
	engine := composition.NewEngine(composition.EngineConfig{
		ExecutorConfig: composition.DefaultExecutorConfig(),
	})
	handler := NewCompositionHandler(engine)

	if handler == nil {
		t.Error("Expected handler to be non-nil")
	}
}

func TestCompositionHandler_ListDAGs_Empty(t *testing.T) {
	ts := newTestCompositionServer(t)

	rr := ts.get("/v1/composition/dags")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if result["count"].(float64) != 0 {
		t.Errorf("Expected 0 DAGs, got %v", result["count"])
	}
}

func TestCompositionHandler_ListDAGs_NoEngine(t *testing.T) {
	ts := newTestCompositionServerNoEngine(t)

	rr := ts.get("/v1/composition/dags")

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
	}
}

func TestCompositionHandler_CreateDAG_InvalidBody(t *testing.T) {
	ts := newTestCompositionServer(t)

	rr := ts.request(http.MethodPost, "/v1/composition/dags", "invalid json")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestCompositionHandler_CreateDAG_MissingID(t *testing.T) {
	ts := newTestCompositionServer(t)

	body := map[string]interface{}{
		"name": "Test DAG",
	}

	rr := ts.postJSON("/v1/composition/dags", body)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestCompositionHandler_CreateDAG_NoEngine(t *testing.T) {
	ts := newTestCompositionServerNoEngine(t)

	body := map[string]interface{}{
		"id":   "test-dag",
		"name": "Test DAG",
	}

	rr := ts.postJSON("/v1/composition/dags", body)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
	}
}

func TestCompositionHandler_CreateDAG(t *testing.T) {
	ts := newTestCompositionServer(t)

	body := map[string]interface{}{
		"id":          "test-dag",
		"name":        "Test DAG",
		"description": "A test DAG",
		"nodes": []map[string]interface{}{
			{
				"id":         "source",
				"name":       "Source Node",
				"type":       "source",
				"expression": "feature_name",
			},
		},
		"outputs": []string{"source"},
	}

	rr := ts.postJSON("/v1/composition/dags", body)

	if rr.Code != http.StatusCreated {
		t.Errorf("Expected status %d, got %d; body: %s", http.StatusCreated, rr.Code, rr.Body.String())
	}
}

func TestCompositionHandler_CreateDAG_WithTransform(t *testing.T) {
	ts := newTestCompositionServer(t)

	body := map[string]interface{}{
		"id":   "transform-dag",
		"name": "Transform DAG",
		"nodes": []map[string]interface{}{
			{
				"id":         "src",
				"name":       "Source",
				"type":       "source",
				"expression": "feature1",
			},
			{
				"id":         "transform",
				"name":       "Transform",
				"type":       "transform",
				"inputs":     []string{"src"},
				"expression": "sum",
			},
		},
		"outputs": []string{"transform"},
	}

	rr := ts.postJSON("/v1/composition/dags", body)

	if rr.Code != http.StatusCreated {
		t.Errorf("Expected status %d, got %d; body: %s", http.StatusCreated, rr.Code, rr.Body.String())
	}
}

func TestCompositionHandler_CreateDAG_InvalidNode(t *testing.T) {
	ts := newTestCompositionServer(t)

	body := map[string]interface{}{
		"id":   "invalid-dag",
		"name": "Invalid DAG",
		"nodes": []map[string]interface{}{
			{
				"id":     "transform",
				"name":   "Transform",
				"type":   "transform",
				"inputs": []string{"nonexistent"},
			},
		},
		"outputs": []string{"transform"},
	}

	rr := ts.postJSON("/v1/composition/dags", body)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestCompositionHandler_GetDAG_NotFound(t *testing.T) {
	ts := newTestCompositionServer(t)

	rr := ts.get("/v1/composition/dags/nonexistent")

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, rr.Code)
	}
}

func TestCompositionHandler_GetDAG_NoEngine(t *testing.T) {
	ts := newTestCompositionServerNoEngine(t)

	rr := ts.get("/v1/composition/dags/test")

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
	}
}

func TestCompositionHandler_DeleteDAG_NotFound(t *testing.T) {
	ts := newTestCompositionServer(t)

	rr := ts.delete("/v1/composition/dags/nonexistent")

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, rr.Code)
	}
}

func TestCompositionHandler_DeleteDAG_NoEngine(t *testing.T) {
	ts := newTestCompositionServerNoEngine(t)

	rr := ts.delete("/v1/composition/dags/test")

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
	}
}

func TestCompositionHandler_Compose_NotFound(t *testing.T) {
	ts := newTestCompositionServer(t)

	body := map[string]interface{}{
		"entity_id": "entity-1",
	}

	rr := ts.postJSON("/v1/composition/dags/nonexistent/compose", body)

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, rr.Code)
	}
}

func TestCompositionHandler_Compose_InvalidBody(t *testing.T) {
	ts := newTestCompositionServer(t)

	rr := ts.request(http.MethodPost, "/v1/composition/dags/test/compose", "invalid json")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestCompositionHandler_Compose_MissingEntityID(t *testing.T) {
	ts := newTestCompositionServer(t)

	body := map[string]interface{}{}

	rr := ts.postJSON("/v1/composition/dags/test/compose", body)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestCompositionHandler_Compose_NoEngine(t *testing.T) {
	ts := newTestCompositionServerNoEngine(t)

	body := map[string]interface{}{
		"entity_id": "entity-1",
	}

	rr := ts.postJSON("/v1/composition/dags/test/compose", body)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
	}
}

func TestCompositionHandler_ComposeBatch_NotFound(t *testing.T) {
	ts := newTestCompositionServer(t)

	body := map[string]interface{}{
		"entity_ids": []string{"entity-1", "entity-2"},
	}

	rr := ts.postJSON("/v1/composition/dags/nonexistent/compose/batch", body)

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, rr.Code)
	}
}

func TestCompositionHandler_ComposeBatch_InvalidBody(t *testing.T) {
	ts := newTestCompositionServer(t)

	rr := ts.request(http.MethodPost, "/v1/composition/dags/test/compose/batch", "invalid json")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestCompositionHandler_ComposeBatch_MissingEntityIDs(t *testing.T) {
	ts := newTestCompositionServer(t)

	body := map[string]interface{}{
		"entity_ids": []string{},
	}

	rr := ts.postJSON("/v1/composition/dags/test/compose/batch", body)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestCompositionHandler_ComposeBatch_NoEngine(t *testing.T) {
	ts := newTestCompositionServerNoEngine(t)

	body := map[string]interface{}{
		"entity_ids": []string{"entity-1"},
	}

	rr := ts.postJSON("/v1/composition/dags/test/compose/batch", body)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
	}
}

func TestCompositionHandler_Stats(t *testing.T) {
	ts := newTestCompositionServer(t)

	rr := ts.get("/v1/composition/stats")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}
}

func TestCompositionHandler_Stats_NoEngine(t *testing.T) {
	ts := newTestCompositionServerNoEngine(t)

	rr := ts.get("/v1/composition/stats")

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
	}
}

func TestCompositionHandler_ClearCache(t *testing.T) {
	ts := newTestCompositionServer(t)

	rr := ts.postJSON("/v1/composition/cache/clear", map[string]interface{}{})

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}
}

func TestCompositionHandler_ClearCache_NoEngine(t *testing.T) {
	ts := newTestCompositionServerNoEngine(t)

	rr := ts.postJSON("/v1/composition/cache/clear", map[string]interface{}{})

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
	}
}

func TestCompositionHandler_CreateAndGetDAG(t *testing.T) {
	ts := newTestCompositionServer(t)

	// Create DAG
	createBody := map[string]interface{}{
		"id":          "crud-dag",
		"name":        "CRUD DAG",
		"description": "Test description",
		"nodes": []map[string]interface{}{
			{
				"id":            "source",
				"name":          "Source Node",
				"type":          "source",
				"expression":    "feature_name",
				"cache_enabled": true,
				"cache_ttl":     "5m",
				"timeout":       "30s",
			},
		},
		"outputs": []string{"source"},
	}

	createRR := ts.postJSON("/v1/composition/dags", createBody)
	if createRR.Code != http.StatusCreated {
		t.Fatalf("Failed to create DAG: %s", createRR.Body.String())
	}

	// Get DAG
	getRR := ts.get("/v1/composition/dags/crud-dag")
	if getRR.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d; body: %s", http.StatusOK, getRR.Code, getRR.Body.String())
	}

	var result map[string]interface{}
	if err := json.Unmarshal(getRR.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if result["id"] != "crud-dag" {
		t.Errorf("Expected id 'crud-dag', got '%v'", result["id"])
	}
	if result["description"] != "Test description" {
		t.Errorf("Expected description 'Test description', got '%v'", result["description"])
	}
}

func TestCompositionHandler_CreateAndDeleteDAG(t *testing.T) {
	ts := newTestCompositionServer(t)

	// Create DAG
	createBody := map[string]interface{}{
		"id":   "delete-dag",
		"name": "Delete DAG",
		"nodes": []map[string]interface{}{
			{
				"id":   "source",
				"name": "Source",
				"type": "source",
			},
		},
		"outputs": []string{"source"},
	}

	ts.postJSON("/v1/composition/dags", createBody)

	// Delete DAG
	deleteRR := ts.delete("/v1/composition/dags/delete-dag")
	if deleteRR.Code != http.StatusOK {
		t.Errorf("Delete failed: %d - %s", deleteRR.Code, deleteRR.Body.String())
	}

	// Verify deleted
	getRR := ts.get("/v1/composition/dags/delete-dag")
	if getRR.Code != http.StatusNotFound {
		t.Errorf("Expected DAG to be deleted, got status %d", getRR.Code)
	}
}

func TestCompositionHandler_ListDAGs_WithDAGs(t *testing.T) {
	ts := newTestCompositionServer(t)

	// Create multiple DAGs
	for i := 0; i < 3; i++ {
		body := map[string]interface{}{
			"id":   string(rune('a' + i)),
			"name": "DAG " + string(rune('a'+i)),
			"nodes": []map[string]interface{}{
				{
					"id":   "source",
					"name": "Source",
					"type": "source",
				},
			},
			"outputs": []string{"source"},
		}
		ts.postJSON("/v1/composition/dags", body)
	}

	// List DAGs
	rr := ts.get("/v1/composition/dags")
	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if result["count"].(float64) != 3 {
		t.Errorf("Expected 3 DAGs, got %v", result["count"])
	}
}

func TestParseNodeType(t *testing.T) {
	tests := []struct {
		input    string
		expected composition.NodeType
	}{
		{"source", composition.NodeTypeSource},
		{"SOURCE", composition.NodeTypeSource},
		{"transform", composition.NodeTypeTransform},
		{"aggregate", composition.NodeTypeAggregate},
		{"join", composition.NodeTypeJoin},
		{"filter", composition.NodeTypeFilter},
		{"custom", composition.NodeTypeCustom},
		{"unknown", composition.NodeTypeCustom},
	}

	for _, tt := range tests {
		result := parseNodeType(tt.input)
		if result != tt.expected {
			t.Errorf("parseNodeType(%s): expected %s, got %s", tt.input, tt.expected, result)
		}
	}
}
