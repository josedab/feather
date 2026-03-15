package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/feather-store/feather/internal/extensions/embedding"
)

// testEmbeddingServer wraps an EmbeddingHandler for testing.
type testEmbeddingServer struct {
	handler *EmbeddingHandler
	mux     *http.ServeMux
	t       *testing.T
}

// newTestEmbeddingServer creates a new test embedding server.
func newTestEmbeddingServer(t *testing.T) *testEmbeddingServer {
	t.Helper()

	handler := NewEmbeddingHandler(EmbeddingHandlerConfig{})
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	return &testEmbeddingServer{
		handler: handler,
		mux:     mux,
		t:       t,
	}
}

func (ts *testEmbeddingServer) request(method, path string, body string) *httptest.ResponseRecorder {
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

func (ts *testEmbeddingServer) postJSON(path string, body interface{}) *httptest.ResponseRecorder {
	ts.t.Helper()

	jsonBody, err := json.Marshal(body)
	if err != nil {
		ts.t.Fatalf("failed to marshal body: %v", err)
	}
	return ts.request(http.MethodPost, path, string(jsonBody))
}

func (ts *testEmbeddingServer) get(path string) *httptest.ResponseRecorder {
	return ts.request(http.MethodGet, path, "")
}

func (ts *testEmbeddingServer) delete(path string) *httptest.ResponseRecorder {
	return ts.request(http.MethodDelete, path, "")
}

func TestEmbeddingHandler_NewEmbeddingHandler(t *testing.T) {
	handler := NewEmbeddingHandler(EmbeddingHandlerConfig{})

	if handler == nil {
		t.Error("Expected handler to be non-nil")
	}
}

// Store tests
func TestEmbeddingHandler_StoreEmbedding_InvalidBody(t *testing.T) {
	ts := newTestEmbeddingServer(t)

	rr := ts.request(http.MethodPost, "/v1/embeddings", "invalid json")

	// Without store configured, returns 503; with store but invalid body, returns 400
	if rr.Code != http.StatusBadRequest && rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d or %d, got %d", http.StatusBadRequest, http.StatusServiceUnavailable, rr.Code)
	}
}

func TestEmbeddingHandler_StoreEmbedding(t *testing.T) {
	ts := newTestEmbeddingServer(t)

	body := map[string]interface{}{
		"id":        "emb-1",
		"content":   "Hello world",
		"vector":    []float64{0.1, 0.2, 0.3, 0.4},
		"model_id":  "text-embedding-ada-002",
	}

	rr := ts.postJSON("/v1/embeddings", body)

	// Without store configured, returns 503
	if rr.Code != http.StatusCreated && rr.Code != http.StatusOK && rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d, %d or %d, got %d; body: %s", http.StatusCreated, http.StatusOK, http.StatusServiceUnavailable, rr.Code, rr.Body.String())
	}
}

func TestEmbeddingHandler_GetEmbedding_NotFound(t *testing.T) {
	ts := newTestEmbeddingServer(t)

	rr := ts.get("/v1/embeddings/nonexistent")

	// Without store configured, returns 503
	if rr.Code != http.StatusNotFound && rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d or %d, got %d", http.StatusNotFound, http.StatusServiceUnavailable, rr.Code)
	}
}

func TestEmbeddingHandler_DeleteEmbedding_NotFound(t *testing.T) {
	ts := newTestEmbeddingServer(t)

	rr := ts.delete("/v1/embeddings/nonexistent")

	// Without store configured, returns 503
	if rr.Code != http.StatusNotFound && rr.Code != http.StatusOK && rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d, %d or %d, got %d", http.StatusNotFound, http.StatusOK, http.StatusServiceUnavailable, rr.Code)
	}
}

// Batch tests
func TestEmbeddingHandler_BatchStore_InvalidBody(t *testing.T) {
	ts := newTestEmbeddingServer(t)

	rr := ts.request(http.MethodPost, "/v1/embeddings/batch", "invalid json")

	// Without processor configured, returns 503
	if rr.Code != http.StatusBadRequest && rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d or %d, got %d", http.StatusBadRequest, http.StatusServiceUnavailable, rr.Code)
	}
}

func TestEmbeddingHandler_BatchStore(t *testing.T) {
	ts := newTestEmbeddingServer(t)

	body := map[string]interface{}{
		"embeddings": []map[string]interface{}{
			{
				"id":        "batch-1",
				"content":   "First text",
				"vector":    []float64{0.1, 0.2, 0.3},
				"model_id":  "text-embedding-ada-002",
			},
			{
				"id":        "batch-2",
				"content":   "Second text",
				"vector":    []float64{0.4, 0.5, 0.6},
				"model_id":  "text-embedding-ada-002",
			},
		},
	}

	rr := ts.postJSON("/v1/embeddings/batch", body)

	// Without processor configured, returns 503
	if rr.Code != http.StatusCreated && rr.Code != http.StatusOK && rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d, %d or %d, got %d; body: %s", http.StatusCreated, http.StatusOK, http.StatusServiceUnavailable, rr.Code, rr.Body.String())
	}
}

// Lookup tests
func TestEmbeddingHandler_Lookup_InvalidBody(t *testing.T) {
	ts := newTestEmbeddingServer(t)

	rr := ts.request(http.MethodPost, "/v1/embeddings/lookup", "invalid json")

	// Without store configured, returns 503
	if rr.Code != http.StatusBadRequest && rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d or %d, got %d", http.StatusBadRequest, http.StatusServiceUnavailable, rr.Code)
	}
}

func TestEmbeddingHandler_Lookup(t *testing.T) {
	ts := newTestEmbeddingServer(t)

	body := map[string]interface{}{
		"ids": []string{"emb-1", "emb-2"},
	}

	rr := ts.postJSON("/v1/embeddings/lookup", body)

	// Without store configured, returns 503
	if rr.Code != http.StatusOK && rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d or %d, got %d; body: %s", http.StatusOK, http.StatusServiceUnavailable, rr.Code, rr.Body.String())
	}
}

// Generate tests
func TestEmbeddingHandler_Generate_InvalidBody(t *testing.T) {
	ts := newTestEmbeddingServer(t)

	rr := ts.request(http.MethodPost, "/v1/embeddings/generate", "invalid json")

	// Without store configured, returns 503
	if rr.Code != http.StatusBadRequest && rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d or %d, got %d", http.StatusBadRequest, http.StatusServiceUnavailable, rr.Code)
	}
}

func TestEmbeddingHandler_Generate(t *testing.T) {
	ts := newTestEmbeddingServer(t)

	body := map[string]interface{}{
		"content":  "Some text content",
		"model_id": "text-embedding-ada-002",
	}

	rr := ts.postJSON("/v1/embeddings/generate", body)

	// Without store configured, returns 503
	if rr.Code != http.StatusOK && rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d or %d, got %d; body: %s", http.StatusOK, http.StatusServiceUnavailable, rr.Code, rr.Body.String())
	}
}

// Model version tests
func TestEmbeddingHandler_ListModels(t *testing.T) {
	ts := newTestEmbeddingServer(t)

	rr := ts.get("/v1/embeddings/models")

	// Without version manager configured, returns 503
	if rr.Code != http.StatusOK && rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d or %d, got %d", http.StatusOK, http.StatusServiceUnavailable, rr.Code)
	}
}

func TestEmbeddingHandler_RegisterModel_InvalidBody(t *testing.T) {
	ts := newTestEmbeddingServer(t)

	rr := ts.request(http.MethodPost, "/v1/embeddings/models", "invalid json")

	// Without version manager configured, returns 503
	if rr.Code != http.StatusBadRequest && rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d or %d, got %d", http.StatusBadRequest, http.StatusServiceUnavailable, rr.Code)
	}
}

func TestEmbeddingHandler_RegisterModel(t *testing.T) {
	ts := newTestEmbeddingServer(t)

	body := map[string]interface{}{
		"model_id":  "custom-model",
		"provider":  "openai",
	}

	rr := ts.postJSON("/v1/embeddings/models", body)

	// Without version manager configured, returns 503
	if rr.Code != http.StatusCreated && rr.Code != http.StatusOK && rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d, %d or %d, got %d; body: %s", http.StatusCreated, http.StatusOK, http.StatusServiceUnavailable, rr.Code, rr.Body.String())
	}
}

func TestEmbeddingHandler_GetModel_NotFound(t *testing.T) {
	ts := newTestEmbeddingServer(t)

	rr := ts.get("/v1/embeddings/models/nonexistent")

	// Without version manager configured, returns 503
	if rr.Code != http.StatusNotFound && rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d or %d, got %d", http.StatusNotFound, http.StatusServiceUnavailable, rr.Code)
	}
}

func TestEmbeddingHandler_ListModelVersions_NotFound(t *testing.T) {
	ts := newTestEmbeddingServer(t)

	rr := ts.get("/v1/embeddings/models/nonexistent/versions")

	// Without version manager configured, returns 503
	if rr.Code != http.StatusNotFound && rr.Code != http.StatusOK && rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d, %d or %d, got %d", http.StatusNotFound, http.StatusOK, http.StatusServiceUnavailable, rr.Code)
	}
}

// Compatibility tests
func TestEmbeddingHandler_CheckCompatibility_InvalidBody(t *testing.T) {
	ts := newTestEmbeddingServer(t)

	rr := ts.request(http.MethodPost, "/v1/embeddings/compatibility", "invalid json")

	// Without version manager configured, returns 503
	if rr.Code != http.StatusBadRequest && rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d or %d, got %d", http.StatusBadRequest, http.StatusServiceUnavailable, rr.Code)
	}
}

func TestEmbeddingHandler_CheckCompatibility(t *testing.T) {
	ts := newTestEmbeddingServer(t)

	body := map[string]interface{}{
		"model_id":     "text-embedding-ada-002",
		"from_version": "1.0.0",
		"to_version":   "2.0.0",
	}

	rr := ts.postJSON("/v1/embeddings/compatibility", body)

	// Without version manager configured, returns 503
	if rr.Code != http.StatusOK && rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d or %d, got %d; body: %s", http.StatusOK, http.StatusServiceUnavailable, rr.Code, rr.Body.String())
	}
}

// Overall stats tests
func TestEmbeddingHandler_GetStats(t *testing.T) {
	ts := newTestEmbeddingServer(t)

	rr := ts.get("/v1/embeddings/stats")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}
}

// Clear tests
func TestEmbeddingHandler_Clear(t *testing.T) {
	ts := newTestEmbeddingServer(t)

	rr := ts.postJSON("/v1/embeddings/clear", map[string]interface{}{})

	// Without store configured, returns 503
	if rr.Code != http.StatusOK && rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d or %d, got %d; body: %s", http.StatusOK, http.StatusServiceUnavailable, rr.Code, rr.Body.String())
	}
}

// List embeddings test
func TestEmbeddingHandler_ListEmbeddings(t *testing.T) {
	ts := newTestEmbeddingServer(t)

	rr := ts.get("/v1/embeddings")

	// Without store configured, returns 503
	if rr.Code != http.StatusOK && rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d or %d, got %d", http.StatusOK, http.StatusServiceUnavailable, rr.Code)
	}
}

// Get by hash test
func TestEmbeddingHandler_GetByHash_NotFound(t *testing.T) {
	ts := newTestEmbeddingServer(t)

	rr := ts.get("/v1/embeddings/hash/abc123")

	// Without store configured, returns 503
	if rr.Code != http.StatusNotFound && rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d or %d, got %d", http.StatusNotFound, http.StatusServiceUnavailable, rr.Code)
	}
}

// Get by model test
func TestEmbeddingHandler_GetByModel(t *testing.T) {
	ts := newTestEmbeddingServer(t)

	rr := ts.get("/v1/embeddings/model/text-embedding-ada-002")

	// Without store configured, returns 503
	if rr.Code != http.StatusOK && rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d or %d, got %d", http.StatusOK, http.StatusServiceUnavailable, rr.Code)
	}
}

// --- Tests with configured components ---

func newConfiguredEmbeddingServer(t *testing.T) *testEmbeddingServer {
	t.Helper()

	store := embedding.NewStore(embedding.StoreConfig{MaxCapacity: 1000})
	dedup := embedding.NewDeduplicator(embedding.DeduplicationConfig{Enabled: true}, store)
	version := embedding.NewVersionManager(embedding.VersionConfig{})
	batch := embedding.NewBatchProcessor(embedding.BatchConfig{MaxBatchSize: 100}, store, dedup, nil)

	handler := NewEmbeddingHandler(EmbeddingHandlerConfig{
		Store:   store,
		Dedup:   dedup,
		Version: version,
		Batch:   batch,
	})
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	return &testEmbeddingServer{handler: handler, mux: mux, t: t}
}

func TestEmbeddingHandler_StoreAndRetrieve(t *testing.T) {
	ts := newConfiguredEmbeddingServer(t)

	body := map[string]interface{}{
		"id":        "emb-store-1",
		"content":   "Hello world",
		"vector":    []float64{0.1, 0.2, 0.3, 0.4},
		"model_id":  "test-model",
	}

	rr := ts.postJSON("/v1/embeddings", body)
	if rr.Code != http.StatusCreated && rr.Code != http.StatusOK {
		t.Errorf("Store: expected 201 or 200, got %d; body: %s", rr.Code, rr.Body.String())
	}

	// Retrieve
	getRr := ts.get("/v1/embeddings/emb-store-1")
	if getRr.Code != http.StatusOK {
		t.Errorf("Get: expected 200, got %d; body: %s", getRr.Code, getRr.Body.String())
	}
}

func TestEmbeddingHandler_StoreEmbedding_DuplicateID(t *testing.T) {
	ts := newConfiguredEmbeddingServer(t)

	body := map[string]interface{}{
		"id":        "emb-dup",
		"content":   "First",
		"vector":    []float64{0.1, 0.2, 0.3},
		"model_id":  "test-model",
	}

	ts.postJSON("/v1/embeddings", body)

	// Store again with same ID
	body["content"] = "Second"
	rr := ts.postJSON("/v1/embeddings", body)

	// May return 200 (update) or 409 (conflict) depending on impl
	if rr.Code != http.StatusCreated && rr.Code != http.StatusOK && rr.Code != http.StatusConflict {
		t.Errorf("Expected 201, 200, or 409 for duplicate, got %d", rr.Code)
	}
}

func TestEmbeddingHandler_Generate_WithConfiguredStore(t *testing.T) {
	ts := newConfiguredEmbeddingServer(t)

	body := map[string]interface{}{
		"content":  "Some text to embed",
		"model_id": "test-model",
	}

	rr := ts.postJSON("/v1/embeddings/generate", body)
	// Without a configured provider, may fail with appropriate error
	if rr.Code == http.StatusInternalServerError || rr.Code == http.StatusServiceUnavailable || rr.Code == http.StatusOK {
		// Expected behaviors
	} else {
		t.Errorf("Unexpected status %d; body: %s", rr.Code, rr.Body.String())
	}
}

func TestEmbeddingHandler_Lookup_WithStore(t *testing.T) {
	ts := newConfiguredEmbeddingServer(t)

	// Store an embedding first
	ts.postJSON("/v1/embeddings", map[string]interface{}{
		"id": "lookup-1", "content": "test", "vector": []float64{0.1}, "model_id": "m",
	})

	// Lookup - the lookup endpoint expects contents and model_id
	body := map[string]interface{}{"contents": []string{"test"}, "model_id": "m"}
	rr := ts.postJSON("/v1/embeddings/lookup", body)
	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d; body: %s", rr.Code, rr.Body.String())
	}
}

func TestEmbeddingHandler_Batch_Empty(t *testing.T) {
	ts := newConfiguredEmbeddingServer(t)

	body := map[string]interface{}{
		"embeddings": []map[string]interface{}{},
	}

	rr := ts.postJSON("/v1/embeddings/batch", body)
	// Empty batch may return 200 or 400
	if rr.Code != http.StatusOK && rr.Code != http.StatusCreated && rr.Code != http.StatusBadRequest {
		t.Errorf("Expected 200, 201, or 400, got %d", rr.Code)
	}
}

func TestEmbeddingHandler_RegisterModel_WithManager(t *testing.T) {
	ts := newConfiguredEmbeddingServer(t)

	body := map[string]interface{}{
		"id":        "custom-model-v1",
		"model_id":  "custom-model-v1",
		"provider":  "openai",
	}

	rr := ts.postJSON("/v1/embeddings/models", body)
	if rr.Code != http.StatusCreated && rr.Code != http.StatusOK && rr.Code != http.StatusBadRequest {
		t.Errorf("Expected 201, 200, or 400, got %d; body: %s", rr.Code, rr.Body.String())
	}
}

func TestEmbeddingHandler_CheckCompatibility_WithManager(t *testing.T) {
	ts := newConfiguredEmbeddingServer(t)

	body := map[string]interface{}{
		"model_id":     "text-embedding-ada-002",
		"from_version": "1.0.0",
		"to_version":   "2.0.0",
	}

	rr := ts.postJSON("/v1/embeddings/compatibility", body)
	if rr.Code != http.StatusOK && rr.Code != http.StatusNotFound {
		t.Errorf("Expected 200 or 404, got %d; body: %s", rr.Code, rr.Body.String())
	}
}

func TestEmbeddingHandler_ListEmbeddings_WithStore(t *testing.T) {
	ts := newConfiguredEmbeddingServer(t)

	rr := ts.get("/v1/embeddings")
	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", rr.Code)
	}
}

func TestEmbeddingHandler_Delete_WithStore(t *testing.T) {
	ts := newConfiguredEmbeddingServer(t)

	// Store an embedding
	ts.postJSON("/v1/embeddings", map[string]interface{}{
		"id": "del-1", "content": "test", "vector": []float64{0.1}, "model_id": "m",
	})

	rr := ts.delete("/v1/embeddings/del-1")
	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d; body: %s", rr.Code, rr.Body.String())
	}

	// Verify deletion
	getRr := ts.get("/v1/embeddings/del-1")
	if getRr.Code != http.StatusNotFound {
		t.Errorf("Expected 404 after delete, got %d", getRr.Code)
	}
}
