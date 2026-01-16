package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/feather-store/feather/internal/groups"
)

// testGroupsServer wraps a GroupsHandler for testing.
type testGroupsServer struct {
	handler *GroupsHandler
	mux     *http.ServeMux
	t       *testing.T
}

// newTestGroupsServer creates a new test groups server.
func newTestGroupsServer(t *testing.T) *testGroupsServer {
	t.Helper()

	handler := NewGroupsHandler()
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	return &testGroupsServer{
		handler: handler,
		mux:     mux,
		t:       t,
	}
}

func (ts *testGroupsServer) request(method, path string, body string) *httptest.ResponseRecorder {
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

func (ts *testGroupsServer) postJSON(path string, body interface{}) *httptest.ResponseRecorder {
	ts.t.Helper()

	jsonBody, err := json.Marshal(body)
	if err != nil {
		ts.t.Fatalf("failed to marshal body: %v", err)
	}
	return ts.request(http.MethodPost, path, string(jsonBody))
}

func (ts *testGroupsServer) putJSON(path string, body interface{}) *httptest.ResponseRecorder {
	ts.t.Helper()

	jsonBody, err := json.Marshal(body)
	if err != nil {
		ts.t.Fatalf("failed to marshal body: %v", err)
	}
	return ts.request(http.MethodPut, path, string(jsonBody))
}

func (ts *testGroupsServer) get(path string) *httptest.ResponseRecorder {
	return ts.request(http.MethodGet, path, "")
}

func (ts *testGroupsServer) delete(path string) *httptest.ResponseRecorder {
	return ts.request(http.MethodDelete, path, "")
}

// createGroup is a helper to create a feature group for testing.
func (ts *testGroupsServer) createGroup(id string) *httptest.ResponseRecorder {
	ts.t.Helper()

	body := CreateGroupRequest{
		ID:          id,
		Name:        "Test Group",
		Description: "Test description",
		EntityType:  "user",
		Features: []groups.GroupFeature{
			{Name: "feature1", DataType: "float", Description: "Test feature 1"},
			{Name: "feature2", DataType: "string", Description: "Test feature 2"},
		},
		Tags:  []string{"test", "example"},
		Owner: "test-owner",
		Team:  "test-team",
	}

	return ts.postJSON("/v1/groups", body)
}

func TestGroupsHandler_NewGroupsHandler(t *testing.T) {
	handler := NewGroupsHandler()

	if handler.manager == nil {
		t.Error("Expected manager to be set")
	}
}

func TestGroupsHandler_ListGroups_Empty(t *testing.T) {
	ts := newTestGroupsServer(t)

	rr := ts.get("/v1/groups")

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

func TestGroupsHandler_CreateGroup(t *testing.T) {
	ts := newTestGroupsServer(t)

	body := CreateGroupRequest{
		ID:          "group-1",
		Name:        "Test Group",
		Description: "Test description",
		EntityType:  "user",
		Features: []groups.GroupFeature{
			{Name: "feature1", DataType: "float"},
		},
		Tags:  []string{"test"},
		Owner: "test-owner",
		Team:  "test-team",
	}

	rr := ts.postJSON("/v1/groups", body)

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

func TestGroupsHandler_CreateGroup_InvalidBody(t *testing.T) {
	ts := newTestGroupsServer(t)

	rr := ts.request(http.MethodPost, "/v1/groups", "invalid json")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestGroupsHandler_GetGroup(t *testing.T) {
	ts := newTestGroupsServer(t)

	// Create group first
	ts.createGroup("get-group")

	rr := ts.get("/v1/groups/get-group")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d; body: %s", http.StatusOK, rr.Code, rr.Body.String())
	}
}

func TestGroupsHandler_GetGroup_NotFound(t *testing.T) {
	ts := newTestGroupsServer(t)

	rr := ts.get("/v1/groups/nonexistent")

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, rr.Code)
	}
}

func TestGroupsHandler_UpdateGroup(t *testing.T) {
	ts := newTestGroupsServer(t)

	// Create group first
	ts.createGroup("update-group")

	body := CreateGroupRequest{
		Name:        "Updated Name",
		Description: "Updated description",
		EntityType:  "user",
		Features: []groups.GroupFeature{
			{Name: "feature1", DataType: "float"},
			{Name: "feature3", DataType: "int"},
		},
	}

	rr := ts.putJSON("/v1/groups/update-group", body)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d; body: %s", http.StatusOK, rr.Code, rr.Body.String())
	}
}

func TestGroupsHandler_UpdateGroup_InvalidBody(t *testing.T) {
	ts := newTestGroupsServer(t)

	ts.createGroup("update-invalid")

	rr := ts.request(http.MethodPut, "/v1/groups/update-invalid", "invalid json")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestGroupsHandler_DeleteGroup(t *testing.T) {
	ts := newTestGroupsServer(t)

	// Create group first
	ts.createGroup("delete-group")

	rr := ts.delete("/v1/groups/delete-group")

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

func TestGroupsHandler_DeleteGroup_NotFound(t *testing.T) {
	ts := newTestGroupsServer(t)

	rr := ts.delete("/v1/groups/nonexistent")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestGroupsHandler_SetStatus(t *testing.T) {
	ts := newTestGroupsServer(t)

	// Create group first
	ts.createGroup("status-group")

	body := GroupSetStatusRequest{
		Status: groups.GroupStatusActive,
	}

	rr := ts.putJSON("/v1/groups/status-group/status", body)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d; body: %s", http.StatusOK, rr.Code, rr.Body.String())
	}
}

func TestGroupsHandler_SetStatus_InvalidBody(t *testing.T) {
	ts := newTestGroupsServer(t)

	ts.createGroup("status-invalid")

	rr := ts.request(http.MethodPut, "/v1/groups/status-invalid/status", "invalid json")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestGroupsHandler_AddFeature(t *testing.T) {
	ts := newTestGroupsServer(t)

	// Create group first
	ts.createGroup("feature-group")

	feature := groups.GroupFeature{
		Name:        "new_feature",
		DataType:        "float",
		Description: "New feature",
	}

	rr := ts.postJSON("/v1/groups/feature-group/features", feature)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d; body: %s", http.StatusOK, rr.Code, rr.Body.String())
	}
}

func TestGroupsHandler_AddFeature_InvalidBody(t *testing.T) {
	ts := newTestGroupsServer(t)

	ts.createGroup("add-feature-invalid")

	rr := ts.request(http.MethodPost, "/v1/groups/add-feature-invalid/features", "invalid json")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestGroupsHandler_RemoveFeature(t *testing.T) {
	ts := newTestGroupsServer(t)

	// Create group first
	ts.createGroup("remove-feature-group")

	rr := ts.delete("/v1/groups/remove-feature-group/features/feature1")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d; body: %s", http.StatusOK, rr.Code, rr.Body.String())
	}
}

func TestGroupsHandler_GetFeatures(t *testing.T) {
	ts := newTestGroupsServer(t)

	// Create group first
	ts.createGroup("get-features-group")

	rr := ts.get("/v1/groups/get-features-group/features")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d; body: %s", http.StatusOK, rr.Code, rr.Body.String())
	}

	var result map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if result["group_id"] != "get-features-group" {
		t.Errorf("Expected group_id 'get-features-group', got %v", result["group_id"])
	}
}

func TestGroupsHandler_GetFeatures_NotFound(t *testing.T) {
	ts := newTestGroupsServer(t)

	rr := ts.get("/v1/groups/nonexistent/features")

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, rr.Code)
	}
}

func TestGroupsHandler_GetVersion_InvalidVersion(t *testing.T) {
	ts := newTestGroupsServer(t)

	ts.createGroup("version-group")

	rr := ts.get("/v1/groups/version-group/versions/invalid")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestGroupsHandler_GetVersion_NotFound(t *testing.T) {
	ts := newTestGroupsServer(t)

	ts.createGroup("version-group-2")

	rr := ts.get("/v1/groups/version-group-2/versions/999")

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, rr.Code)
	}
}

func TestGroupsHandler_CreateView(t *testing.T) {
	ts := newTestGroupsServer(t)

	// Create group first
	ts.createGroup("view-group")

	body := CreateViewRequest{
		ID:          "view-1",
		Name:        "Test View",
		GroupID:     "view-group",
		Features:    []string{"feature1"},
		Description: "Test view description",
	}

	rr := ts.postJSON("/v1/feature-views", body)

	if rr.Code != http.StatusCreated {
		t.Errorf("Expected status %d, got %d; body: %s", http.StatusCreated, rr.Code, rr.Body.String())
	}
}

func TestGroupsHandler_CreateView_InvalidBody(t *testing.T) {
	ts := newTestGroupsServer(t)

	rr := ts.request(http.MethodPost, "/v1/feature-views", "invalid json")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestGroupsHandler_ListViews(t *testing.T) {
	ts := newTestGroupsServer(t)

	rr := ts.get("/v1/feature-views")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}
}

func TestGroupsHandler_GetView(t *testing.T) {
	ts := newTestGroupsServer(t)

	// Create group and view first
	ts.createGroup("get-view-group")
	ts.postJSON("/v1/feature-views", CreateViewRequest{
		ID:       "get-view",
		Name:     "Get View",
		GroupID:  "get-view-group",
		Features: []string{"feature1"},
	})

	rr := ts.get("/v1/feature-views/get-view")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d; body: %s", http.StatusOK, rr.Code, rr.Body.String())
	}
}

func TestGroupsHandler_GetView_NotFound(t *testing.T) {
	ts := newTestGroupsServer(t)

	rr := ts.get("/v1/feature-views/nonexistent")

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, rr.Code)
	}
}

func TestGroupsHandler_DeleteView(t *testing.T) {
	ts := newTestGroupsServer(t)

	// Create group and view first
	ts.createGroup("delete-view-group")
	ts.postJSON("/v1/feature-views", CreateViewRequest{
		ID:       "delete-view",
		Name:     "Delete View",
		GroupID:  "delete-view-group",
		Features: []string{"feature1"},
	})

	rr := ts.delete("/v1/feature-views/delete-view")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d; body: %s", http.StatusOK, rr.Code, rr.Body.String())
	}
}

func TestGroupsHandler_DeleteView_NotFound(t *testing.T) {
	ts := newTestGroupsServer(t)

	rr := ts.delete("/v1/feature-views/nonexistent")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestGroupsHandler_GetByEntity(t *testing.T) {
	ts := newTestGroupsServer(t)

	// Create group first
	ts.createGroup("entity-group")

	rr := ts.get("/v1/entities/user/groups")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d; body: %s", http.StatusOK, rr.Code, rr.Body.String())
	}

	var result map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if result["entity_type"] != "user" {
		t.Errorf("Expected entity_type 'user', got %v", result["entity_type"])
	}
}

func TestGroupsHandler_GetByTag(t *testing.T) {
	ts := newTestGroupsServer(t)

	// Create group first
	ts.createGroup("tag-group")

	rr := ts.get("/v1/tags/test/groups")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d; body: %s", http.StatusOK, rr.Code, rr.Body.String())
	}

	var result map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if result["tag"] != "test" {
		t.Errorf("Expected tag 'test', got %v", result["tag"])
	}
}

func TestGroupsHandler_GetStats(t *testing.T) {
	ts := newTestGroupsServer(t)

	rr := ts.get("/v1/group-stats")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}
}

func TestGroupsHandler_ListGroups_WithFilter(t *testing.T) {
	ts := newTestGroupsServer(t)

	// Create group first
	ts.createGroup("filter-group")

	rr := ts.get("/v1/groups?entity_type=user&tag=test")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}
}

func TestGroupsHandler_GetManager(t *testing.T) {
	handler := NewGroupsHandler()
	manager := handler.GetManager()

	if manager == nil {
		t.Error("Expected GetManager to return non-nil manager")
	}
}
