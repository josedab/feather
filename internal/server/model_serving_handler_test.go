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

// testModelServingServer wraps a ModelServingHandler for testing.
type testModelServingServer struct {
	handler *ModelServingHandler
	mux     *http.ServeMux
	t       *testing.T
}

// newTestModelServingServer creates a new test model serving server.
func newTestModelServingServer(t *testing.T) *testModelServingServer {
	t.Helper()

	// Create a minimal store for testing
	store, err := storage.NewStore(context.Background(), storage.StoreOptions{
		HotMaxSize:   1024 * 1024,
		WarmInMemory: true,
	}, nil)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	handler := NewModelServingHandler(store)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	return &testModelServingServer{
		handler: handler,
		mux:     mux,
		t:       t,
	}
}

func (ts *testModelServingServer) request(method, path string, body string) *httptest.ResponseRecorder {
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

func (ts *testModelServingServer) postJSON(path string, body interface{}) *httptest.ResponseRecorder {
	ts.t.Helper()

	jsonBody, err := json.Marshal(body)
	if err != nil {
		ts.t.Fatalf("failed to marshal body: %v", err)
	}
	return ts.request(http.MethodPost, path, string(jsonBody))
}

func (ts *testModelServingServer) get(path string) *httptest.ResponseRecorder {
	return ts.request(http.MethodGet, path, "")
}

func (ts *testModelServingServer) delete(path string) *httptest.ResponseRecorder {
	return ts.request(http.MethodDelete, path, "")
}

func TestModelServingHandler_NewModelServingHandler(t *testing.T) {
	store, err := storage.NewStore(context.Background(), storage.StoreOptions{
		HotMaxSize:   1024 * 1024,
		WarmInMemory: true,
	}, nil)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	handler := NewModelServingHandler(store)

	if handler == nil {
		t.Error("Expected handler to be non-nil")
	}
}

// Model Registry tests
func TestModelServingHandler_ListModels(t *testing.T) {
	ts := newTestModelServingServer(t)

	rr := ts.get("/v1/models")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}
}

func TestModelServingHandler_RegisterModel_InvalidBody(t *testing.T) {
	ts := newTestModelServingServer(t)

	rr := ts.request(http.MethodPost, "/v1/models", "invalid json")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestModelServingHandler_RegisterModel(t *testing.T) {
	ts := newTestModelServingServer(t)

	body := map[string]interface{}{
		"id":          "model-1",
		"name":        "Test Model",
		"version":     "1.0.0",
		"framework":   "tensorflow",
		"features":    []string{"feature1", "feature2"},
		"description": "Test model description",
	}

	rr := ts.postJSON("/v1/models", body)

	if rr.Code != http.StatusCreated && rr.Code != http.StatusOK {
		t.Errorf("Expected status %d or %d, got %d; body: %s", http.StatusCreated, http.StatusOK, rr.Code, rr.Body.String())
	}
}

func TestModelServingHandler_GetModel_NotFound(t *testing.T) {
	ts := newTestModelServingServer(t)

	rr := ts.get("/v1/models/nonexistent")

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, rr.Code)
	}
}

func TestModelServingHandler_DeleteModel_NotFound(t *testing.T) {
	ts := newTestModelServingServer(t)

	rr := ts.delete("/v1/models/nonexistent")

	if rr.Code != http.StatusNotFound && rr.Code != http.StatusOK {
		t.Errorf("Expected status %d or %d, got %d", http.StatusNotFound, http.StatusOK, rr.Code)
	}
}

// Snapshot tests - snapshots are under /v1/models/{id}/snapshots
func TestModelServingHandler_ListSnapshots_ModelNotFound(t *testing.T) {
	ts := newTestModelServingServer(t)

	rr := ts.get("/v1/models/nonexistent/snapshots")

	// Model not found returns 404
	if rr.Code != http.StatusNotFound && rr.Code != http.StatusOK {
		t.Errorf("Expected status %d or %d, got %d", http.StatusNotFound, http.StatusOK, rr.Code)
	}
}

func TestModelServingHandler_CreateSnapshot_InvalidBody(t *testing.T) {
	ts := newTestModelServingServer(t)

	rr := ts.request(http.MethodPost, "/v1/models/test-model/snapshots", "invalid json")

	if rr.Code != http.StatusBadRequest && rr.Code != http.StatusNotFound {
		t.Errorf("Expected status %d or %d, got %d", http.StatusBadRequest, http.StatusNotFound, rr.Code)
	}
}

// Validation tests - validation is under /v1/models/{id}/validate
func TestModelServingHandler_ValidateFeatures_InvalidBody(t *testing.T) {
	ts := newTestModelServingServer(t)

	rr := ts.request(http.MethodPost, "/v1/models/test-model/validate", "invalid json")

	if rr.Code != http.StatusBadRequest && rr.Code != http.StatusNotFound {
		t.Errorf("Expected status %d or %d, got %d", http.StatusBadRequest, http.StatusNotFound, rr.Code)
	}
}

// Serving tests - serving is under /v1/models/{id}/serve
func TestModelServingHandler_GetServingFeatures_InvalidBody(t *testing.T) {
	ts := newTestModelServingServer(t)

	rr := ts.request(http.MethodPost, "/v1/models/test-model/serve", "invalid json")

	if rr.Code != http.StatusBadRequest && rr.Code != http.StatusNotFound {
		t.Errorf("Expected status %d or %d, got %d", http.StatusBadRequest, http.StatusNotFound, rr.Code)
	}
}

// Stats tests
func TestModelServingHandler_GetStats(t *testing.T) {
	ts := newTestModelServingServer(t)

	rr := ts.get("/v1/models/stats")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}
}

// End-to-end test: Register model, create snapshot
func TestModelServingHandler_EndToEnd(t *testing.T) {
	ts := newTestModelServingServer(t)

	// Register a model
	modelBody := map[string]interface{}{
		"id":        "e2e-model",
		"name":      "E2E Test Model",
		"version":   "1.0.0",
		"framework": "pytorch",
		"features":  []string{"feature1", "feature2"},
	}
	modelRR := ts.postJSON("/v1/models", modelBody)
	if modelRR.Code != http.StatusCreated && modelRR.Code != http.StatusOK {
		t.Fatalf("Failed to register model: %s", modelRR.Body.String())
	}

	// Get the model
	getRR := ts.get("/v1/models/e2e-model")
	if getRR.Code != http.StatusOK {
		t.Errorf("Failed to get model: %d - %s", getRR.Code, getRR.Body.String())
	}

	// Create a snapshot for the model
	snapshotBody := map[string]interface{}{
		"name":     "e2e-snapshot",
		"features": []string{"feature1", "feature2"},
	}
	snapshotRR := ts.postJSON("/v1/models/e2e-model/snapshots", snapshotBody)
	if snapshotRR.Code != http.StatusCreated && snapshotRR.Code != http.StatusOK {
		t.Logf("Snapshot creation returned: %d - %s", snapshotRR.Code, snapshotRR.Body.String())
	}

	// Delete the model
	deleteRR := ts.delete("/v1/models/e2e-model")
	if deleteRR.Code != http.StatusOK && deleteRR.Code != http.StatusNoContent {
		t.Errorf("Failed to delete model: %d - %s", deleteRR.Code, deleteRR.Body.String())
	}
}

// Version tests
func TestModelServingHandler_ListVersions_NotFound(t *testing.T) {
	ts := newTestModelServingServer(t)

	rr := ts.get("/v1/models/nonexistent/versions")

	if rr.Code != http.StatusNotFound && rr.Code != http.StatusOK {
		t.Errorf("Expected status %d or %d, got %d", http.StatusNotFound, http.StatusOK, rr.Code)
	}
}

// Drift alerts tests
func TestModelServingHandler_GetAllAlerts(t *testing.T) {
	ts := newTestModelServingServer(t)

	rr := ts.get("/v1/models/drift/alerts")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}
}
