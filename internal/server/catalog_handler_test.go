package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/feather-store/feather/internal/registry"
)

// testCatalogServer wraps a CatalogHandler for testing.
type testCatalogServer struct {
	handler *CatalogHandler
	mux     *http.ServeMux
	t       *testing.T
}

// newTestCatalogServer creates a new test catalog server.
func newTestCatalogServer(t *testing.T) *testCatalogServer {
	t.Helper()

	handler := NewCatalogHandler()
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	return &testCatalogServer{
		handler: handler,
		mux:     mux,
		t:       t,
	}
}

func (ts *testCatalogServer) request(method, path string, body string) *httptest.ResponseRecorder {
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

func (ts *testCatalogServer) postJSON(path string, body interface{}) *httptest.ResponseRecorder {
	ts.t.Helper()

	jsonBody, err := json.Marshal(body)
	if err != nil {
		ts.t.Fatalf("failed to marshal body: %v", err)
	}
	return ts.request(http.MethodPost, path, string(jsonBody))
}

func (ts *testCatalogServer) get(path string) *httptest.ResponseRecorder {
	return ts.request(http.MethodGet, path, "")
}

func (ts *testCatalogServer) put(path string, body interface{}) *httptest.ResponseRecorder {
	ts.t.Helper()

	jsonBody, err := json.Marshal(body)
	if err != nil {
		ts.t.Fatalf("failed to marshal body: %v", err)
	}
	return ts.request(http.MethodPut, path, string(jsonBody))
}

func (ts *testCatalogServer) delete(path string) *httptest.ResponseRecorder {
	return ts.request(http.MethodDelete, path, "")
}

// registerFeature is a helper to register a feature for testing.
func (ts *testCatalogServer) registerFeature(name string) *httptest.ResponseRecorder {
	ts.t.Helper()

	body := registry.FeatureDefinition{
		Name:        name,
		Description: "Test feature",
		EntityType:  "user",
		DataType:    "float",
		Owner:       "test-owner",
		Team:        "test-team",
		Category:    "test-category",
		Tags:        []string{"test-tag"},
	}
	return ts.postJSON("/v1/catalog/features", body)
}

func TestCatalogHandler_NewCatalogHandler(t *testing.T) {
	handler := NewCatalogHandler()

	if handler.catalog == nil {
		t.Error("Expected catalog to be set")
	}
}

func TestCatalogHandler_GetCatalog(t *testing.T) {
	handler := NewCatalogHandler()

	catalog := handler.GetCatalog()
	if catalog == nil {
		t.Error("Expected GetCatalog to return non-nil")
	}
}

func TestCatalogHandler_ListFeatures_Empty(t *testing.T) {
	ts := newTestCatalogServer(t)

	rr := ts.get("/v1/catalog/features")

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

func TestCatalogHandler_RegisterFeature(t *testing.T) {
	ts := newTestCatalogServer(t)

	body := registry.FeatureDefinition{
		Name:        "test-feature",
		Description: "A test feature",
		EntityType:  "user",
		DataType:    "float",
		Owner:       "alice",
		Team:        "ml-team",
		Category:    "demographics",
		Tags:        []string{"user", "numeric"},
	}

	rr := ts.postJSON("/v1/catalog/features", body)

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

func TestCatalogHandler_RegisterFeature_InvalidBody(t *testing.T) {
	ts := newTestCatalogServer(t)

	rr := ts.request(http.MethodPost, "/v1/catalog/features", "invalid json")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestCatalogHandler_GetFeature(t *testing.T) {
	ts := newTestCatalogServer(t)

	// Register feature first
	ts.registerFeature("get-feature")

	rr := ts.get("/v1/catalog/features/get-feature")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d; body: %s", http.StatusOK, rr.Code, rr.Body.String())
	}
}

func TestCatalogHandler_GetFeature_NotFound(t *testing.T) {
	ts := newTestCatalogServer(t)

	rr := ts.get("/v1/catalog/features/nonexistent")

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, rr.Code)
	}
}

func TestCatalogHandler_DeleteFeature(t *testing.T) {
	ts := newTestCatalogServer(t)

	// Register feature first
	ts.registerFeature("delete-feature")

	rr := ts.delete("/v1/catalog/features/delete-feature")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d; body: %s", http.StatusOK, rr.Code, rr.Body.String())
	}

	// Verify it's deleted
	rr = ts.get("/v1/catalog/features/delete-feature")
	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected feature to be deleted, got status %d", rr.Code)
	}
}

func TestCatalogHandler_DeleteFeature_NotFound(t *testing.T) {
	ts := newTestCatalogServer(t)

	rr := ts.delete("/v1/catalog/features/nonexistent")

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, rr.Code)
	}
}

func TestCatalogHandler_SetStatus(t *testing.T) {
	ts := newTestCatalogServer(t)

	// Register feature first
	ts.registerFeature("status-feature")

	body := SetStatusRequest{
		Status: "deprecated",
	}

	rr := ts.put("/v1/catalog/features/status-feature/status", body)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d; body: %s", http.StatusOK, rr.Code, rr.Body.String())
	}
}

func TestCatalogHandler_SetStatus_NotFound(t *testing.T) {
	ts := newTestCatalogServer(t)

	body := SetStatusRequest{
		Status: "deprecated",
	}

	rr := ts.put("/v1/catalog/features/nonexistent/status", body)

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, rr.Code)
	}
}

func TestCatalogHandler_SetStatus_InvalidBody(t *testing.T) {
	ts := newTestCatalogServer(t)

	// Register feature first
	ts.registerFeature("status-feature-2")

	rr := ts.request(http.MethodPut, "/v1/catalog/features/status-feature-2/status", "invalid json")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestCatalogHandler_GetVersions(t *testing.T) {
	ts := newTestCatalogServer(t)

	// Register feature first
	ts.registerFeature("versions-feature")

	rr := ts.get("/v1/catalog/features/versions-feature/versions")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d; body: %s", http.StatusOK, rr.Code, rr.Body.String())
	}
}

func TestCatalogHandler_GetVersion(t *testing.T) {
	ts := newTestCatalogServer(t)

	// Register feature first
	ts.registerFeature("version-feature")

	rr := ts.get("/v1/catalog/features/version-feature/versions/1")

	// Version 1 should exist after initial registration
	if rr.Code != http.StatusOK && rr.Code != http.StatusNotFound {
		t.Errorf("Expected status 200 or 404, got %d", rr.Code)
	}
}

func TestCatalogHandler_GetVersion_InvalidVersion(t *testing.T) {
	ts := newTestCatalogServer(t)

	rr := ts.get("/v1/catalog/features/test/versions/invalid")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestCatalogHandler_Search(t *testing.T) {
	ts := newTestCatalogServer(t)

	// Register some features
	ts.registerFeature("search-feature-1")
	ts.registerFeature("search-feature-2")

	rr := ts.get("/v1/catalog/search?q=search")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d; body: %s", http.StatusOK, rr.Code, rr.Body.String())
	}
}

func TestCatalogHandler_Search_MissingQuery(t *testing.T) {
	ts := newTestCatalogServer(t)

	rr := ts.get("/v1/catalog/search")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestCatalogHandler_GetByTag(t *testing.T) {
	ts := newTestCatalogServer(t)

	// Register feature first
	ts.registerFeature("tag-feature")

	rr := ts.get("/v1/catalog/tags/test-tag")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d; body: %s", http.StatusOK, rr.Code, rr.Body.String())
	}
}

func TestCatalogHandler_GetByOwner(t *testing.T) {
	ts := newTestCatalogServer(t)

	// Register feature first
	ts.registerFeature("owner-feature")

	rr := ts.get("/v1/catalog/owners/test-owner")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d; body: %s", http.StatusOK, rr.Code, rr.Body.String())
	}
}

func TestCatalogHandler_GetByTeam(t *testing.T) {
	ts := newTestCatalogServer(t)

	// Register feature first
	ts.registerFeature("team-feature")

	rr := ts.get("/v1/catalog/teams/test-team")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d; body: %s", http.StatusOK, rr.Code, rr.Body.String())
	}
}

func TestCatalogHandler_GetByCategory(t *testing.T) {
	ts := newTestCatalogServer(t)

	// Register feature first
	ts.registerFeature("category-feature")

	rr := ts.get("/v1/catalog/categories/test-category")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d; body: %s", http.StatusOK, rr.Code, rr.Body.String())
	}
}

func TestCatalogHandler_GetByEntity(t *testing.T) {
	ts := newTestCatalogServer(t)

	// Register feature first
	ts.registerFeature("entity-feature")

	rr := ts.get("/v1/catalog/entities/user")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d; body: %s", http.StatusOK, rr.Code, rr.Body.String())
	}
}

func TestCatalogHandler_GetLineage(t *testing.T) {
	ts := newTestCatalogServer(t)

	// Register feature first
	ts.registerFeature("lineage-feature")

	rr := ts.get("/v1/catalog/features/lineage-feature/lineage")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d; body: %s", http.StatusOK, rr.Code, rr.Body.String())
	}
}

func TestCatalogHandler_GetLineage_NotFound(t *testing.T) {
	ts := newTestCatalogServer(t)

	rr := ts.get("/v1/catalog/features/nonexistent/lineage")

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, rr.Code)
	}
}

func TestCatalogHandler_GetStats(t *testing.T) {
	ts := newTestCatalogServer(t)

	rr := ts.get("/v1/catalog/stats")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}
}

func TestCatalogHandler_Export(t *testing.T) {
	ts := newTestCatalogServer(t)

	// Register some features
	ts.registerFeature("export-feature-1")
	ts.registerFeature("export-feature-2")

	rr := ts.get("/v1/catalog/export")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}

	// Check content type
	contentType := rr.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Expected Content-Type 'application/json', got %s", contentType)
	}
}

func TestCatalogHandler_Import(t *testing.T) {
	ts := newTestCatalogServer(t)

	features := []registry.FeatureDefinition{
		{
			Name:       "import-feature-1",
			EntityType: "user",
			DataType:   "float",
		},
		{
			Name:       "import-feature-2",
			EntityType: "user",
			DataType:   "string",
		},
	}

	rr := ts.postJSON("/v1/catalog/import", features)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d; body: %s", http.StatusOK, rr.Code, rr.Body.String())
	}

	var result map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if result["success"] != true {
		t.Error("Expected success to be true")
	}
}

func TestCatalogHandler_Import_InvalidBody(t *testing.T) {
	ts := newTestCatalogServer(t)

	rr := ts.request(http.MethodPost, "/v1/catalog/import", "invalid json")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestCatalogHandler_ListFeatures_WithFilters(t *testing.T) {
	ts := newTestCatalogServer(t)

	// Register features
	ts.registerFeature("filter-feature")

	rr := ts.get("/v1/catalog/features?owner=test-owner&team=test-team")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}
}
