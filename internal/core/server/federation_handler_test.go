package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/feather-store/feather/internal/platform/federation"
)

// testFederationServer wraps a FederationHandler for testing.
type testFederationServer struct {
	handler    *FederationHandler
	federation *federation.Federation
	mux        *http.ServeMux
	t          *testing.T
}

func newTestFederationServer(t *testing.T) *testFederationServer {
	t.Helper()

	config := federation.Config{
		NodeID:      "local-node",
		NodeName:    "Local Test Node",
		NodeAddress: "http://localhost:8080",
		Region:      "us-east-1",
	}
	fed := federation.NewFederation(config)
	handler := NewFederationHandler(fed)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	return &testFederationServer{
		handler:    handler,
		federation: fed,
		mux:        mux,
		t:          t,
	}
}

func newTestFederationServerWithoutFederation(t *testing.T) *testFederationServer {
	t.Helper()

	handler := NewFederationHandler(nil)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	return &testFederationServer{
		handler:    handler,
		federation: nil,
		mux:        mux,
		t:          t,
	}
}

func (ts *testFederationServer) request(method, path string, body string) *httptest.ResponseRecorder {
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

func (ts *testFederationServer) postJSON(path string, body interface{}) *httptest.ResponseRecorder {
	ts.t.Helper()

	jsonBody, err := json.Marshal(body)
	if err != nil {
		ts.t.Fatalf("failed to marshal body: %v", err)
	}
	return ts.request(http.MethodPost, path, string(jsonBody))
}

func (ts *testFederationServer) putJSON(path string, body interface{}) *httptest.ResponseRecorder {
	ts.t.Helper()

	jsonBody, err := json.Marshal(body)
	if err != nil {
		ts.t.Fatalf("failed to marshal body: %v", err)
	}
	return ts.request(http.MethodPut, path, string(jsonBody))
}

func (ts *testFederationServer) get(path string) *httptest.ResponseRecorder {
	return ts.request(http.MethodGet, path, "")
}

func (ts *testFederationServer) delete(path string) *httptest.ResponseRecorder {
	return ts.request(http.MethodDelete, path, "")
}

func TestFederationHandler_NewFederationHandler(t *testing.T) {
	config := federation.Config{NodeID: "test-node"}
	fed := federation.NewFederation(config)
	handler := NewFederationHandler(fed)

	if handler.federation == nil {
		t.Error("Expected federation to be set")
	}
}

func TestFederationHandler_ListNodes_Empty(t *testing.T) {
	ts := newTestFederationServer(t)

	rr := ts.get("/v1/federation/nodes")

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

func TestFederationHandler_ListNodes_NoFederation(t *testing.T) {
	ts := newTestFederationServerWithoutFederation(t)

	rr := ts.get("/v1/federation/nodes")

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
	}
}

func TestFederationHandler_JoinNode(t *testing.T) {
	ts := newTestFederationServer(t)

	body := JoinNodeRequest{
		ID:      "new-node",
		Name:    "New Test Node",
		Address: "http://newnode:8080",
		Role:    "peer",
		Region:  "us-west-2",
	}

	rr := ts.postJSON("/v1/federation/nodes", body)

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

func TestFederationHandler_JoinNode_MissingID(t *testing.T) {
	ts := newTestFederationServer(t)

	body := JoinNodeRequest{
		Address: "http://newnode:8080",
	}

	rr := ts.postJSON("/v1/federation/nodes", body)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestFederationHandler_JoinNode_MissingAddress(t *testing.T) {
	ts := newTestFederationServer(t)

	body := JoinNodeRequest{
		ID: "new-node",
	}

	rr := ts.postJSON("/v1/federation/nodes", body)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestFederationHandler_JoinNode_InvalidBody(t *testing.T) {
	ts := newTestFederationServer(t)

	rr := ts.request(http.MethodPost, "/v1/federation/nodes", "invalid json")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestFederationHandler_JoinNode_NoFederation(t *testing.T) {
	ts := newTestFederationServerWithoutFederation(t)

	body := JoinNodeRequest{
		ID:      "new-node",
		Address: "http://newnode:8080",
	}

	rr := ts.postJSON("/v1/federation/nodes", body)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
	}
}

func TestFederationHandler_GetNode(t *testing.T) {
	ts := newTestFederationServer(t)

	// Join a node first
	ts.postJSON("/v1/federation/nodes", JoinNodeRequest{
		ID:      "get-node",
		Name:    "Get Test Node",
		Address: "http://getnode:8080",
	})

	rr := ts.get("/v1/federation/nodes/get-node")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d; body: %s", http.StatusOK, rr.Code, rr.Body.String())
	}
}

func TestFederationHandler_GetNode_NotFound(t *testing.T) {
	ts := newTestFederationServer(t)

	rr := ts.get("/v1/federation/nodes/nonexistent")

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, rr.Code)
	}
}

func TestFederationHandler_GetNode_NoFederation(t *testing.T) {
	ts := newTestFederationServerWithoutFederation(t)

	rr := ts.get("/v1/federation/nodes/test")

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
	}
}

func TestFederationHandler_LeaveNode(t *testing.T) {
	ts := newTestFederationServer(t)

	// Join a node first
	ts.postJSON("/v1/federation/nodes", JoinNodeRequest{
		ID:      "leave-node",
		Address: "http://leavenode:8080",
	})

	rr := ts.delete("/v1/federation/nodes/leave-node")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d; body: %s", http.StatusOK, rr.Code, rr.Body.String())
	}
}

func TestFederationHandler_LeaveNode_NotFound(t *testing.T) {
	ts := newTestFederationServer(t)

	rr := ts.delete("/v1/federation/nodes/nonexistent")

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, rr.Code)
	}
}

func TestFederationHandler_LeaveNode_NoFederation(t *testing.T) {
	ts := newTestFederationServerWithoutFederation(t)

	rr := ts.delete("/v1/federation/nodes/test")

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
	}
}

func TestFederationHandler_ListCatalog(t *testing.T) {
	ts := newTestFederationServer(t)

	rr := ts.get("/v1/federation/catalog")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if result["catalog"] == nil {
		t.Error("Expected catalog array in response")
	}
}

func TestFederationHandler_ListCatalog_NoFederation(t *testing.T) {
	ts := newTestFederationServerWithoutFederation(t)

	rr := ts.get("/v1/federation/catalog")

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
	}
}

func TestFederationHandler_ShareFeature(t *testing.T) {
	ts := newTestFederationServer(t)

	body := ShareFeatureRequest{
		ID:         "shared-feature",
		Name:       "Shared Feature",
		Visibility: "federation",
	}

	rr := ts.postJSON("/v1/federation/features", body)

	if rr.Code != http.StatusCreated {
		t.Errorf("Expected status %d, got %d; body: %s", http.StatusCreated, rr.Code, rr.Body.String())
	}
}

func TestFederationHandler_ShareFeature_MissingID(t *testing.T) {
	ts := newTestFederationServer(t)

	body := ShareFeatureRequest{
		Name: "No ID Feature",
	}

	rr := ts.postJSON("/v1/federation/features", body)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestFederationHandler_ShareFeature_InvalidBody(t *testing.T) {
	ts := newTestFederationServer(t)

	rr := ts.request(http.MethodPost, "/v1/federation/features", "invalid json")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestFederationHandler_ShareFeature_NoFederation(t *testing.T) {
	ts := newTestFederationServerWithoutFederation(t)

	body := ShareFeatureRequest{
		ID:   "test",
		Name: "Test",
	}

	rr := ts.postJSON("/v1/federation/features", body)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
	}
}

func TestFederationHandler_GetFeature(t *testing.T) {
	ts := newTestFederationServer(t)

	// Share a feature first
	ts.postJSON("/v1/federation/features", ShareFeatureRequest{
		ID:         "get-feature",
		Name:       "Get Feature",
		Visibility: "federation",
	})

	rr := ts.get("/v1/federation/features/get-feature")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d; body: %s", http.StatusOK, rr.Code, rr.Body.String())
	}
}

func TestFederationHandler_GetFeature_NotFound(t *testing.T) {
	ts := newTestFederationServer(t)

	rr := ts.get("/v1/federation/features/nonexistent")

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, rr.Code)
	}
}

func TestFederationHandler_GetFeature_NoFederation(t *testing.T) {
	ts := newTestFederationServerWithoutFederation(t)

	rr := ts.get("/v1/federation/features/test")

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
	}
}

func TestFederationHandler_UpdateFeature(t *testing.T) {
	ts := newTestFederationServer(t)

	// Share a feature first
	ts.postJSON("/v1/federation/features", ShareFeatureRequest{
		ID:         "update-feature",
		Name:       "Update Feature",
		Visibility: "federation",
	})

	body := ShareFeatureRequest{
		Name:        "Updated Feature Name",
		Description: "Updated description",
		Visibility:  "public",
	}

	rr := ts.putJSON("/v1/federation/features/update-feature", body)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d; body: %s", http.StatusOK, rr.Code, rr.Body.String())
	}
}

func TestFederationHandler_UpdateFeature_NotFound(t *testing.T) {
	ts := newTestFederationServer(t)

	body := ShareFeatureRequest{
		Name: "Updated Feature",
	}

	rr := ts.putJSON("/v1/federation/features/nonexistent", body)

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, rr.Code)
	}
}

func TestFederationHandler_UpdateFeature_InvalidBody(t *testing.T) {
	ts := newTestFederationServer(t)

	rr := ts.request(http.MethodPut, "/v1/federation/features/test", "invalid json")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestFederationHandler_UpdateFeature_NoFederation(t *testing.T) {
	ts := newTestFederationServerWithoutFederation(t)

	body := ShareFeatureRequest{
		Name: "Updated Feature",
	}

	rr := ts.putJSON("/v1/federation/features/test", body)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
	}
}

func TestFederationHandler_DeleteFeature(t *testing.T) {
	ts := newTestFederationServer(t)

	// Share a feature first
	ts.postJSON("/v1/federation/features", ShareFeatureRequest{
		ID:         "delete-feature",
		Name:       "Delete Feature",
		Visibility: "federation",
	})

	rr := ts.delete("/v1/federation/features/delete-feature")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d; body: %s", http.StatusOK, rr.Code, rr.Body.String())
	}
}

func TestFederationHandler_DeleteFeature_NotFound(t *testing.T) {
	ts := newTestFederationServer(t)

	rr := ts.delete("/v1/federation/features/nonexistent")

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, rr.Code)
	}
}

func TestFederationHandler_DeleteFeature_NoFederation(t *testing.T) {
	ts := newTestFederationServerWithoutFederation(t)

	rr := ts.delete("/v1/federation/features/test")

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
	}
}

func TestFederationHandler_SearchFeatures(t *testing.T) {
	ts := newTestFederationServer(t)

	body := SearchFeaturesRequest{
		Name:  "test",
		Limit: 10,
	}

	rr := ts.postJSON("/v1/federation/search", body)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d; body: %s", http.StatusOK, rr.Code, rr.Body.String())
	}

	var result map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if result["results"] == nil {
		t.Error("Expected results array in response")
	}
}

func TestFederationHandler_SearchFeatures_InvalidBody(t *testing.T) {
	ts := newTestFederationServer(t)

	rr := ts.request(http.MethodPost, "/v1/federation/search", "invalid json")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestFederationHandler_SearchFeatures_NoFederation(t *testing.T) {
	ts := newTestFederationServerWithoutFederation(t)

	body := SearchFeaturesRequest{
		Name: "test",
	}

	rr := ts.postJSON("/v1/federation/search", body)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
	}
}

func TestFederationHandler_ReplicateFeature_MissingTargets(t *testing.T) {
	ts := newTestFederationServer(t)

	body := ReplicateRequest{
		TargetNodes: []string{},
	}

	rr := ts.postJSON("/v1/federation/features/test/replicate", body)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestFederationHandler_ReplicateFeature_InvalidBody(t *testing.T) {
	ts := newTestFederationServer(t)

	rr := ts.request(http.MethodPost, "/v1/federation/features/test/replicate", "invalid json")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestFederationHandler_ReplicateFeature_NoFederation(t *testing.T) {
	ts := newTestFederationServerWithoutFederation(t)

	body := ReplicateRequest{
		TargetNodes: []string{"node1"},
	}

	rr := ts.postJSON("/v1/federation/features/test/replicate", body)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
	}
}

func TestFederationHandler_SetReplicationPolicy_InvalidBody(t *testing.T) {
	ts := newTestFederationServer(t)

	rr := ts.request(http.MethodPut, "/v1/federation/features/test/policy", "invalid json")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestFederationHandler_SetReplicationPolicy_NoFederation(t *testing.T) {
	ts := newTestFederationServerWithoutFederation(t)

	body := ReplicationPolicyRequest{
		Mode:        "sync",
		MinReplicas: 1,
	}

	rr := ts.putJSON("/v1/federation/features/test/policy", body)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
	}
}

func TestFederationHandler_GetStats(t *testing.T) {
	ts := newTestFederationServer(t)

	rr := ts.get("/v1/federation/stats")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}
}

func TestFederationHandler_GetStats_NoFederation(t *testing.T) {
	ts := newTestFederationServerWithoutFederation(t)

	rr := ts.get("/v1/federation/stats")

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
	}
}

func TestFederationHandler_GetLocalNode(t *testing.T) {
	ts := newTestFederationServer(t)

	rr := ts.get("/v1/federation/local")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if result["id"] != "local-node" {
		t.Errorf("Expected id 'local-node', got %v", result["id"])
	}
}

func TestFederationHandler_GetLocalNode_NoFederation(t *testing.T) {
	ts := newTestFederationServerWithoutFederation(t)

	rr := ts.get("/v1/federation/local")

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
	}
}

func TestFederationHandler_JoinNode_WithPermissions(t *testing.T) {
	ts := newTestFederationServer(t)

	body := JoinNodeRequest{
		ID:      "perm-node",
		Name:    "Permission Node",
		Address: "http://permnode:8080",
		Permissions: &NodePermissionsJSON{
			CanRead:      true,
			CanWrite:     false,
			CanReplicate: true,
			AllowedTeams: []string{"team-a"},
		},
	}

	rr := ts.postJSON("/v1/federation/nodes", body)

	if rr.Code != http.StatusCreated {
		t.Errorf("Expected status %d, got %d; body: %s", http.StatusCreated, rr.Code, rr.Body.String())
	}
}

func TestFederationHandler_ShareFeature_WithAccessControl(t *testing.T) {
	ts := newTestFederationServer(t)

	body := ShareFeatureRequest{
		ID:         "acl-feature",
		Name:       "ACL Feature",
		Visibility: "team",
		AccessControl: &AccessControlJSON{
			AllowedTeams: []string{"team-a", "team-b"},
			RequireAuth:  true,
		},
	}

	rr := ts.postJSON("/v1/federation/features", body)

	if rr.Code != http.StatusCreated {
		t.Errorf("Expected status %d, got %d; body: %s", http.StatusCreated, rr.Code, rr.Body.String())
	}
}
