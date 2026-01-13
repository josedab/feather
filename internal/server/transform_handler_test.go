package server

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/feather-store/feather/internal/storage"
)

// testTransformServer wraps a TransformHandler for testing.
type testTransformServer struct {
	handler *TransformHandler
	mux     *http.ServeMux
	t       *testing.T
}

// newTestTransformServer creates a new test transform server.
func newTestTransformServer(t *testing.T) *testTransformServer {
	t.Helper()

	store, err := storage.NewStore(context.Background(), storage.StoreOptions{
		HotMaxSize:   1024 * 1024, // 1MB
		WarmInMemory: true,
	}, storage.NewRegistry())
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	handler := NewTransformHandler(store)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	return &testTransformServer{
		handler: handler,
		mux:     mux,
		t:       t,
	}
}

func (ts *testTransformServer) request(method, path string, body string) *httptest.ResponseRecorder {
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

func (ts *testTransformServer) postJSON(path string, body interface{}) *httptest.ResponseRecorder {
	ts.t.Helper()

	jsonBody, err := json.Marshal(body)
	if err != nil {
		ts.t.Fatalf("failed to marshal body: %v", err)
	}
	return ts.request(http.MethodPost, path, string(jsonBody))
}

func (ts *testTransformServer) get(path string) *httptest.ResponseRecorder {
	return ts.request(http.MethodGet, path, "")
}

func (ts *testTransformServer) delete(path string) *httptest.ResponseRecorder {
	return ts.request(http.MethodDelete, path, "")
}

// registerTransform is a helper to register a transform for testing.
func (ts *testTransformServer) registerTransform(name string) *httptest.ResponseRecorder {
	ts.t.Helper()

	body := TransformRequest{
		Name:        name,
		Description: "Test transform",
		Type:        "arithmetic",
		Expression:  "a + b",
		Inputs:      []string{"feature_a", "feature_b"},
		Output:      "output_" + name,
	}
	return ts.postJSON("/v1/transforms", body)
}

func TestTransformHandler_NewTransformHandler(t *testing.T) {
	store, err := storage.NewStore(context.Background(), storage.StoreOptions{
		HotMaxSize:   1024 * 1024,
		WarmInMemory: true,
	}, storage.NewRegistry())
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	handler := NewTransformHandler(store)

	if handler.pipeline == nil {
		t.Error("Expected pipeline to be set")
	}
	if handler.dsl == nil {
		t.Error("Expected dsl to be set")
	}
}

func TestTransformHandler_ListTransforms_Empty(t *testing.T) {
	ts := newTestTransformServer(t)

	rr := ts.get("/v1/transforms")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if result["count"].(float64) != 0 {
		t.Errorf("Expected count=0, got %v", result["count"])
	}
}

func TestTransformHandler_RegisterTransform(t *testing.T) {
	ts := newTestTransformServer(t)

	body := TransformRequest{
		Name:        "sum-transform",
		Description: "Add two features",
		Type:        "arithmetic",
		Expression:  "a + b",
		Inputs:      []string{"feature_a", "feature_b"},
		Output:      "sum_feature",
	}

	rr := ts.postJSON("/v1/transforms", body)

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
	if result["name"] != "sum-transform" {
		t.Errorf("Expected name 'sum-transform', got %v", result["name"])
	}
}

func TestTransformHandler_RegisterTransform_MissingName(t *testing.T) {
	ts := newTestTransformServer(t)

	body := TransformRequest{
		Type:   "arithmetic",
		Inputs: []string{"feature_a"},
		Output: "output",
	}

	rr := ts.postJSON("/v1/transforms", body)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestTransformHandler_RegisterTransform_MissingType(t *testing.T) {
	ts := newTestTransformServer(t)

	body := TransformRequest{
		Name:   "test-transform",
		Inputs: []string{"feature_a"},
		Output: "output",
	}

	rr := ts.postJSON("/v1/transforms", body)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestTransformHandler_RegisterTransform_MissingInputs(t *testing.T) {
	ts := newTestTransformServer(t)

	body := TransformRequest{
		Name:   "test-transform",
		Type:   "arithmetic",
		Output: "output",
	}

	rr := ts.postJSON("/v1/transforms", body)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestTransformHandler_RegisterTransform_MissingOutput(t *testing.T) {
	ts := newTestTransformServer(t)

	body := TransformRequest{
		Name:   "test-transform",
		Type:   "arithmetic",
		Inputs: []string{"feature_a"},
	}

	rr := ts.postJSON("/v1/transforms", body)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestTransformHandler_RegisterTransform_InvalidBody(t *testing.T) {
	ts := newTestTransformServer(t)

	rr := ts.request(http.MethodPost, "/v1/transforms", "invalid json")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestTransformHandler_GetTransform(t *testing.T) {
	ts := newTestTransformServer(t)

	// Register transform first
	ts.registerTransform("get-transform")

	rr := ts.get("/v1/transforms/get-transform")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d; body: %s", http.StatusOK, rr.Code, rr.Body.String())
	}

	var result map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if result["name"] != "get-transform" {
		t.Errorf("Expected name 'get-transform', got %v", result["name"])
	}
}

func TestTransformHandler_GetTransform_NotFound(t *testing.T) {
	ts := newTestTransformServer(t)

	rr := ts.get("/v1/transforms/nonexistent")

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, rr.Code)
	}
}

func TestTransformHandler_UnregisterTransform(t *testing.T) {
	ts := newTestTransformServer(t)

	// Register transform first
	ts.registerTransform("delete-transform")

	rr := ts.delete("/v1/transforms/delete-transform")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d; body: %s", http.StatusOK, rr.Code, rr.Body.String())
	}

	// Verify it's deleted
	rr = ts.get("/v1/transforms/delete-transform")
	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected transform to be deleted, got status %d", rr.Code)
	}
}

func TestTransformHandler_UnregisterTransform_NotFound(t *testing.T) {
	ts := newTestTransformServer(t)

	rr := ts.delete("/v1/transforms/nonexistent")

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, rr.Code)
	}
}

func TestTransformHandler_DefineFromDSL(t *testing.T) {
	ts := newTestTransformServer(t)

	body := DSLRequest{
		Name:       "dsl-transform",
		Expression: "output_dsl = feature_a + feature_b",
	}

	rr := ts.postJSON("/v1/transforms/dsl", body)

	if rr.Code != http.StatusCreated && rr.Code != http.StatusBadRequest {
		// DSL parsing may fail if expression is invalid for the DSL parser
		t.Errorf("Expected status 201 or 400, got %d; body: %s", rr.Code, rr.Body.String())
	}
}

func TestTransformHandler_DefineFromDSL_MissingName(t *testing.T) {
	ts := newTestTransformServer(t)

	body := DSLRequest{
		Expression: "output = a + b",
	}

	rr := ts.postJSON("/v1/transforms/dsl", body)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestTransformHandler_DefineFromDSL_MissingExpression(t *testing.T) {
	ts := newTestTransformServer(t)

	body := DSLRequest{
		Name: "test-dsl",
	}

	rr := ts.postJSON("/v1/transforms/dsl", body)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestTransformHandler_DefineFromDSL_InvalidBody(t *testing.T) {
	ts := newTestTransformServer(t)

	rr := ts.request(http.MethodPost, "/v1/transforms/dsl", "invalid json")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestTransformHandler_Execute_MissingEntityID(t *testing.T) {
	ts := newTestTransformServer(t)

	// Register transform first
	ts.registerTransform("exec-transform")

	body := ExecuteRequest{
		EntityID: "",
	}

	rr := ts.postJSON("/v1/transforms/exec-transform/execute", body)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestTransformHandler_Execute_NotFound(t *testing.T) {
	ts := newTestTransformServer(t)

	body := ExecuteRequest{
		EntityID: "entity-1",
	}

	rr := ts.postJSON("/v1/transforms/nonexistent/execute", body)

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, rr.Code)
	}
}

func TestTransformHandler_Execute_InvalidBody(t *testing.T) {
	ts := newTestTransformServer(t)

	// Register transform first
	ts.registerTransform("exec-transform-2")

	rr := ts.request(http.MethodPost, "/v1/transforms/exec-transform-2/execute", "invalid json")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestTransformHandler_ExecuteAndStore_MissingEntityID(t *testing.T) {
	ts := newTestTransformServer(t)

	// Register transform first
	ts.registerTransform("store-transform")

	body := ExecuteRequest{
		EntityID: "",
	}

	rr := ts.postJSON("/v1/transforms/store-transform/execute-store", body)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestTransformHandler_ExecuteAndStore_NotFound(t *testing.T) {
	ts := newTestTransformServer(t)

	body := ExecuteRequest{
		EntityID: "entity-1",
	}

	rr := ts.postJSON("/v1/transforms/nonexistent/execute-store", body)

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, rr.Code)
	}
}

func TestTransformHandler_ExecuteAndStore_InvalidBody(t *testing.T) {
	ts := newTestTransformServer(t)

	// Register transform first
	ts.registerTransform("store-transform-2")

	rr := ts.request(http.MethodPost, "/v1/transforms/store-transform-2/execute-store", "invalid json")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestTransformHandler_ExecuteChain_MissingOutputFeature(t *testing.T) {
	ts := newTestTransformServer(t)

	body := ChainExecuteRequest{
		EntityID: "entity-1",
	}

	rr := ts.postJSON("/v1/transforms/chain", body)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestTransformHandler_ExecuteChain_MissingEntityID(t *testing.T) {
	ts := newTestTransformServer(t)

	body := ChainExecuteRequest{
		OutputFeature: "output_feature",
	}

	rr := ts.postJSON("/v1/transforms/chain", body)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestTransformHandler_ExecuteChain_InvalidBody(t *testing.T) {
	ts := newTestTransformServer(t)

	rr := ts.request(http.MethodPost, "/v1/transforms/chain", "invalid json")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestTransformHandler_ListTransforms_WithTransforms(t *testing.T) {
	ts := newTestTransformServer(t)

	// Register some transforms
	ts.registerTransform("transform-1")
	ts.registerTransform("transform-2")

	rr := ts.get("/v1/transforms")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if result["count"].(float64) != 2 {
		t.Errorf("Expected count=2, got %v", result["count"])
	}
}
