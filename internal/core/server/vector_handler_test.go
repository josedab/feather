package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/feather-store/feather/internal/core/vector"
)

// testVectorServer wraps a VectorHandler for testing.
type testVectorServer struct {
	handler *VectorHandler
	mux     *http.ServeMux
	t       *testing.T
}

// newTestVectorServer creates a new test vector server.
func newTestVectorServer(t *testing.T) *testVectorServer {
	t.Helper()

	store := vector.NewStore(vector.StoreConfig{})
	handler := NewVectorHandler(store, nil)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	return &testVectorServer{
		handler: handler,
		mux:     mux,
		t:       t,
	}
}

func (ts *testVectorServer) request(method, path string, body string) *httptest.ResponseRecorder {
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

func (ts *testVectorServer) postJSON(path string, body interface{}) *httptest.ResponseRecorder {
	ts.t.Helper()

	jsonBody, err := json.Marshal(body)
	if err != nil {
		ts.t.Fatalf("failed to marshal body: %v", err)
	}
	return ts.request(http.MethodPost, path, string(jsonBody))
}

func (ts *testVectorServer) get(path string) *httptest.ResponseRecorder {
	return ts.request(http.MethodGet, path, "")
}

func (ts *testVectorServer) delete(path string) *httptest.ResponseRecorder {
	return ts.request(http.MethodDelete, path, "")
}

func TestVectorHandler_NewVectorHandler(t *testing.T) {
	store := vector.NewStore(vector.StoreConfig{})
	handler := NewVectorHandler(store, nil)

	if handler.store == nil {
		t.Error("Expected store to be set")
	}
}

func TestVectorHandler_ListIndexes_Empty(t *testing.T) {
	ts := newTestVectorServer(t)

	rr := ts.get("/v1/vectors")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	indexes, ok := result["indexes"].([]interface{})
	if !ok {
		t.Fatal("Expected indexes array in response")
	}
	if len(indexes) != 0 {
		t.Errorf("Expected empty indexes, got %d", len(indexes))
	}
}

func TestVectorHandler_CreateIndex(t *testing.T) {
	ts := newTestVectorServer(t)

	body := CreateIndexRequest{
		Name:      "test-index",
		Dimension: 128,
	}

	rr := ts.postJSON("/v1/vectors", body)

	if rr.Code != http.StatusCreated {
		t.Errorf("Expected status %d, got %d; body: %s", http.StatusCreated, rr.Code, rr.Body.String())
	}

	var result IndexInfo
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if result.Name != "test-index" {
		t.Errorf("Expected name 'test-index', got %s", result.Name)
	}
	if result.Dimension != 128 {
		t.Errorf("Expected dimension 128, got %d", result.Dimension)
	}
}

func TestVectorHandler_CreateIndex_MissingName(t *testing.T) {
	ts := newTestVectorServer(t)

	body := CreateIndexRequest{
		Dimension: 128,
	}

	rr := ts.postJSON("/v1/vectors", body)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestVectorHandler_CreateIndex_InvalidDimension(t *testing.T) {
	ts := newTestVectorServer(t)

	body := CreateIndexRequest{
		Name:      "test-index",
		Dimension: 0,
	}

	rr := ts.postJSON("/v1/vectors", body)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestVectorHandler_CreateIndex_DimensionTooLarge(t *testing.T) {
	ts := newTestVectorServer(t)

	body := CreateIndexRequest{
		Name:      "test-index",
		Dimension: 5000,
	}

	rr := ts.postJSON("/v1/vectors", body)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d: %s", http.StatusBadRequest, rr.Code, rr.Body.String())
	}
}

func TestVectorHandler_CreateIndex_Duplicate(t *testing.T) {
	ts := newTestVectorServer(t)

	body := CreateIndexRequest{
		Name:      "dup-index",
		Dimension: 128,
	}

	// Create first time
	rr := ts.postJSON("/v1/vectors", body)
	if rr.Code != http.StatusCreated {
		t.Fatalf("First create failed: %s", rr.Body.String())
	}

	// Create again - should fail
	rr = ts.postJSON("/v1/vectors", body)
	if rr.Code != http.StatusConflict {
		t.Errorf("Expected status %d, got %d", http.StatusConflict, rr.Code)
	}
}

func TestVectorHandler_CreateIndex_InvalidBody(t *testing.T) {
	ts := newTestVectorServer(t)

	rr := ts.request(http.MethodPost, "/v1/vectors", "invalid json")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestVectorHandler_GetIndex(t *testing.T) {
	ts := newTestVectorServer(t)

	// Create an index first
	ts.postJSON("/v1/vectors", CreateIndexRequest{
		Name:      "get-test",
		Dimension: 64,
	})

	rr := ts.get("/v1/vectors/get-test")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d; body: %s", http.StatusOK, rr.Code, rr.Body.String())
	}

	var result IndexInfo
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if result.Name != "get-test" {
		t.Errorf("Expected name 'get-test', got %s", result.Name)
	}
}

func TestVectorHandler_GetIndex_NotFound(t *testing.T) {
	ts := newTestVectorServer(t)

	rr := ts.get("/v1/vectors/nonexistent")

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, rr.Code)
	}
}

func TestVectorHandler_DeleteIndex(t *testing.T) {
	ts := newTestVectorServer(t)

	// Create an index first
	ts.postJSON("/v1/vectors", CreateIndexRequest{
		Name:      "delete-test",
		Dimension: 64,
	})

	rr := ts.delete("/v1/vectors/delete-test")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d; body: %s", http.StatusOK, rr.Code, rr.Body.String())
	}

	// Verify it's deleted
	rr = ts.get("/v1/vectors/delete-test")
	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected index to be deleted, got status %d", rr.Code)
	}
}

func TestVectorHandler_DeleteIndex_NotFound(t *testing.T) {
	ts := newTestVectorServer(t)

	rr := ts.delete("/v1/vectors/nonexistent")

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, rr.Code)
	}
}

func TestVectorHandler_Upsert(t *testing.T) {
	ts := newTestVectorServer(t)

	// Create an index first
	ts.postJSON("/v1/vectors", CreateIndexRequest{
		Name:      "upsert-test",
		Dimension: 4,
	})

	body := UpsertRequest{
		Vectors: []VectorInput{
			{
				ID:     "vec1",
				Vector: []float32{1.0, 2.0, 3.0, 4.0},
				Metadata: map[string]interface{}{
					"label": "test",
				},
			},
			{
				ID:     "vec2",
				Vector: []float32{5.0, 6.0, 7.0, 8.0},
			},
		},
	}

	rr := ts.postJSON("/v1/vectors/upsert-test/upsert", body)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d; body: %s", http.StatusOK, rr.Code, rr.Body.String())
	}

	var result map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if result["upserted"].(float64) != 2 {
		t.Errorf("Expected upserted=2, got %v", result["upserted"])
	}
}

func TestVectorHandler_Upsert_IndexNotFound(t *testing.T) {
	ts := newTestVectorServer(t)

	body := UpsertRequest{
		Vectors: []VectorInput{
			{
				ID:     "vec1",
				Vector: []float32{1.0, 2.0, 3.0, 4.0},
			},
		},
	}

	rr := ts.postJSON("/v1/vectors/nonexistent/upsert", body)

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, rr.Code)
	}
}

func TestVectorHandler_Upsert_EmptyVectors(t *testing.T) {
	ts := newTestVectorServer(t)

	// Create an index first
	ts.postJSON("/v1/vectors", CreateIndexRequest{
		Name:      "empty-upsert-test",
		Dimension: 4,
	})

	body := UpsertRequest{
		Vectors: []VectorInput{},
	}

	rr := ts.postJSON("/v1/vectors/empty-upsert-test/upsert", body)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestVectorHandler_Upsert_InvalidBody(t *testing.T) {
	ts := newTestVectorServer(t)

	// Create an index first
	ts.postJSON("/v1/vectors", CreateIndexRequest{
		Name:      "invalid-upsert-test",
		Dimension: 4,
	})

	rr := ts.request(http.MethodPost, "/v1/vectors/invalid-upsert-test/upsert", "invalid json")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestVectorHandler_Search(t *testing.T) {
	ts := newTestVectorServer(t)

	// Create an index and add vectors
	ts.postJSON("/v1/vectors", CreateIndexRequest{
		Name:      "search-test",
		Dimension: 4,
	})

	ts.postJSON("/v1/vectors/search-test/upsert", UpsertRequest{
		Vectors: []VectorInput{
			{ID: "vec1", Vector: []float32{1.0, 0.0, 0.0, 0.0}},
			{ID: "vec2", Vector: []float32{0.0, 1.0, 0.0, 0.0}},
			{ID: "vec3", Vector: []float32{0.0, 0.0, 1.0, 0.0}},
		},
	})

	body := VectorSearchRequest{
		Vector: []float32{1.0, 0.0, 0.0, 0.0},
		TopK:   2,
	}

	rr := ts.postJSON("/v1/vectors/search-test/search", body)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d; body: %s", http.StatusOK, rr.Code, rr.Body.String())
	}

	var result map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	results, ok := result["results"].([]interface{})
	if !ok {
		t.Fatal("Expected results array in response")
	}

	if len(results) == 0 {
		t.Error("Expected at least one result")
	}
}

func TestVectorHandler_Search_IndexNotFound(t *testing.T) {
	ts := newTestVectorServer(t)

	body := VectorSearchRequest{
		Vector: []float32{1.0, 0.0, 0.0, 0.0},
		TopK:   2,
	}

	rr := ts.postJSON("/v1/vectors/nonexistent/search", body)

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, rr.Code)
	}
}

func TestVectorHandler_Search_EmptyVector(t *testing.T) {
	ts := newTestVectorServer(t)

	// Create an index
	ts.postJSON("/v1/vectors", CreateIndexRequest{
		Name:      "empty-search-test",
		Dimension: 4,
	})

	body := VectorSearchRequest{
		Vector: []float32{},
		TopK:   2,
	}

	rr := ts.postJSON("/v1/vectors/empty-search-test/search", body)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestVectorHandler_Search_InvalidBody(t *testing.T) {
	ts := newTestVectorServer(t)

	// Create an index
	ts.postJSON("/v1/vectors", CreateIndexRequest{
		Name:      "invalid-search-test",
		Dimension: 4,
	})

	rr := ts.request(http.MethodPost, "/v1/vectors/invalid-search-test/search", "invalid json")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestVectorHandler_Search_DefaultTopK(t *testing.T) {
	ts := newTestVectorServer(t)

	// Create an index
	ts.postJSON("/v1/vectors", CreateIndexRequest{
		Name:      "default-topk-test",
		Dimension: 4,
	})

	// TopK = 0 should default to 10
	body := VectorSearchRequest{
		Vector: []float32{1.0, 0.0, 0.0, 0.0},
		TopK:   0,
	}

	rr := ts.postJSON("/v1/vectors/default-topk-test/search", body)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}
}

func TestVectorHandler_GetVector(t *testing.T) {
	ts := newTestVectorServer(t)

	// Create an index and add a vector
	ts.postJSON("/v1/vectors", CreateIndexRequest{
		Name:      "get-vec-test",
		Dimension: 4,
	})

	ts.postJSON("/v1/vectors/get-vec-test/upsert", UpsertRequest{
		Vectors: []VectorInput{
			{
				ID:       "vec1",
				Vector:   []float32{1.0, 2.0, 3.0, 4.0},
				Metadata: map[string]interface{}{"label": "test"},
			},
		},
	})

	rr := ts.get("/v1/vectors/get-vec-test/vec1")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d; body: %s", http.StatusOK, rr.Code, rr.Body.String())
	}

	var result map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if result["id"] != "vec1" {
		t.Errorf("Expected id 'vec1', got %v", result["id"])
	}
}

func TestVectorHandler_GetVector_NotFound(t *testing.T) {
	ts := newTestVectorServer(t)

	// Create an index
	ts.postJSON("/v1/vectors", CreateIndexRequest{
		Name:      "get-vec-notfound-test",
		Dimension: 4,
	})

	rr := ts.get("/v1/vectors/get-vec-notfound-test/nonexistent")

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, rr.Code)
	}
}

func TestVectorHandler_GetVector_IndexNotFound(t *testing.T) {
	ts := newTestVectorServer(t)

	rr := ts.get("/v1/vectors/nonexistent/vec1")

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, rr.Code)
	}
}

func TestVectorHandler_GetVector_IncludeVector(t *testing.T) {
	ts := newTestVectorServer(t)

	// Create an index and add a vector
	ts.postJSON("/v1/vectors", CreateIndexRequest{
		Name:      "include-vec-test",
		Dimension: 4,
	})

	ts.postJSON("/v1/vectors/include-vec-test/upsert", UpsertRequest{
		Vectors: []VectorInput{
			{ID: "vec1", Vector: []float32{1.0, 2.0, 3.0, 4.0}},
		},
	})

	// Without include_vector
	rr := ts.get("/v1/vectors/include-vec-test/vec1")
	var result map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &result)
	if result["vector"] != nil {
		t.Error("Expected vector to be nil without include_vector")
	}

	// With include_vector=true
	rr = ts.get("/v1/vectors/include-vec-test/vec1?include_vector=true")
	json.Unmarshal(rr.Body.Bytes(), &result)
	if result["vector"] == nil {
		t.Error("Expected vector to be present with include_vector=true")
	}
}

func TestVectorHandler_DeleteVector(t *testing.T) {
	ts := newTestVectorServer(t)

	// Create an index and add a vector
	ts.postJSON("/v1/vectors", CreateIndexRequest{
		Name:      "delete-vec-test",
		Dimension: 4,
	})

	ts.postJSON("/v1/vectors/delete-vec-test/upsert", UpsertRequest{
		Vectors: []VectorInput{
			{ID: "vec1", Vector: []float32{1.0, 2.0, 3.0, 4.0}},
		},
	})

	rr := ts.delete("/v1/vectors/delete-vec-test/vec1")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d; body: %s", http.StatusOK, rr.Code, rr.Body.String())
	}

	// Verify it's deleted
	rr = ts.get("/v1/vectors/delete-vec-test/vec1")
	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected vector to be deleted, got status %d", rr.Code)
	}
}

func TestVectorHandler_DeleteVector_IndexNotFound(t *testing.T) {
	ts := newTestVectorServer(t)

	rr := ts.delete("/v1/vectors/nonexistent/vec1")

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, rr.Code)
	}
}

func TestVectorHandler_ListIndexes_WithIndexes(t *testing.T) {
	ts := newTestVectorServer(t)

	// Create some indexes
	ts.postJSON("/v1/vectors", CreateIndexRequest{Name: "idx1", Dimension: 64})
	ts.postJSON("/v1/vectors", CreateIndexRequest{Name: "idx2", Dimension: 128})

	rr := ts.get("/v1/vectors")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	indexes, ok := result["indexes"].([]interface{})
	if !ok {
		t.Fatal("Expected indexes array in response")
	}
	if len(indexes) != 2 {
		t.Errorf("Expected 2 indexes, got %d", len(indexes))
	}
}
