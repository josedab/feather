package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/feather-store/feather/internal/platform/auth"
)

// testAuthServer wraps an AuthHandler for testing.
type testAuthServer struct {
	*AuthHandler
	mux    *http.ServeMux
	t      *testing.T
	apiKey string // raw API key for authenticated requests
}

// newTestAuthServer creates a new test auth server with a pre-configured
// API key so tests can authenticate against the protected endpoints.
func newTestAuthServer(t *testing.T) *testAuthServer {
	t.Helper()

	handler := NewAuthHandler()
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	// Bootstrap an admin API key for test authentication.
	controller := handler.GetController()
	_ = controller.CreateTenant(&auth.Tenant{
		ID:      "test-tenant",
		Name:    "Test Tenant",
		Enabled: true,
	})
	rawKey, err := controller.CreateAPIKey(&auth.APIKey{
		Name:        "test-admin",
		Tenant:      "test-tenant",
		Permissions: []auth.Permission{auth.PermAdmin},
	}, "test-setup")
	if err != nil {
		t.Fatalf("failed to create test API key: %v", err)
	}

	return &testAuthServer{
		AuthHandler: handler,
		mux:         mux,
		t:           t,
		apiKey:      rawKey,
	}
}

// request makes a test HTTP request and returns the response.
func (ts *testAuthServer) request(method, path string, body string) *httptest.ResponseRecorder {
	ts.t.Helper()

	var bodyReader io.Reader
	if body != "" {
		bodyReader = strings.NewReader(body)
	}

	req := httptest.NewRequest(method, path, bodyReader)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Authorization", "Bearer "+ts.apiKey)

	rr := httptest.NewRecorder()
	ts.mux.ServeHTTP(rr, req)
	return rr
}

// post makes a POST request with JSON body.
func (ts *testAuthServer) postJSON(path string, body interface{}) *httptest.ResponseRecorder {
	ts.t.Helper()

	jsonBody, err := json.Marshal(body)
	if err != nil {
		ts.t.Fatalf("failed to marshal body: %v", err)
	}
	return ts.request(http.MethodPost, path, string(jsonBody))
}

// get makes a GET request.
func (ts *testAuthServer) get(path string) *httptest.ResponseRecorder {
	return ts.request(http.MethodGet, path, "")
}

// delete makes a DELETE request.
func (ts *testAuthServer) delete(path string) *httptest.ResponseRecorder {
	return ts.request(http.MethodDelete, path, "")
}

// createTenant creates a tenant for testing and returns whether it succeeded.
func (ts *testAuthServer) createTenant(id, name string) bool {
	ts.t.Helper()
	rr := ts.postJSON("/v1/auth/tenants", map[string]interface{}{
		"id":   id,
		"name": name,
	})
	return rr.Code == http.StatusCreated || rr.Code == http.StatusConflict
}

func TestAuthHandler_NewAuthHandler(t *testing.T) {
	handler := NewAuthHandler()

	if handler.controller == nil {
		t.Error("Expected controller to be initialized")
	}
	if handler.middleware == nil {
		t.Error("Expected middleware to be initialized")
	}
}

func TestAuthHandler_GetController(t *testing.T) {
	handler := NewAuthHandler()

	if handler.GetController() == nil {
		t.Error("Expected GetController to return non-nil controller")
	}
	if handler.GetController() != handler.controller {
		t.Error("Expected GetController to return same instance")
	}
}

func TestAuthHandler_GetMiddleware(t *testing.T) {
	handler := NewAuthHandler()

	if handler.GetMiddleware() == nil {
		t.Error("Expected GetMiddleware to return non-nil middleware")
	}
	if handler.GetMiddleware() != handler.middleware {
		t.Error("Expected GetMiddleware to return same instance")
	}
}

func TestAuthHandler_CreateAPIKey(t *testing.T) {
	ts := newTestAuthServer(t)

	// Create tenant first
	ts.createTenant("test-tenant", "Test Tenant")

	body := CreateAPIKeyRequest{
		Name:        "test-key",
		Tenant:      "test-tenant",
		Roles:       []string{"reader"},
		Permissions: []string{"read"},
	}

	rr := ts.postJSON("/v1/auth/keys", body)

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

	if result["key"] == nil || result["key"] == "" {
		t.Error("Expected key to be returned")
	}

	if result["id"] == nil || result["id"] == "" {
		t.Error("Expected id to be returned")
	}
}

func TestAuthHandler_CreateAPIKey_MissingName(t *testing.T) {
	ts := newTestAuthServer(t)

	body := CreateAPIKeyRequest{
		Tenant: "test-tenant",
	}

	rr := ts.postJSON("/v1/auth/keys", body)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestAuthHandler_CreateAPIKey_MissingTenant(t *testing.T) {
	ts := newTestAuthServer(t)

	body := CreateAPIKeyRequest{
		Name: "test-key",
	}

	rr := ts.postJSON("/v1/auth/keys", body)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestAuthHandler_CreateAPIKey_InvalidBody(t *testing.T) {
	ts := newTestAuthServer(t)

	rr := ts.request(http.MethodPost, "/v1/auth/keys", "invalid json")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestAuthHandler_CreateAPIKey_WithExpiration(t *testing.T) {
	ts := newTestAuthServer(t)

	// Create tenant first
	ts.createTenant("test-tenant", "Test Tenant")

	body := CreateAPIKeyRequest{
		Name:      "expiring-key",
		Tenant:    "test-tenant",
		ExpiresIn: "30d",
	}

	rr := ts.postJSON("/v1/auth/keys", body)

	if rr.Code != http.StatusCreated {
		t.Errorf("Expected status %d, got %d; body: %s", http.StatusCreated, rr.Code, rr.Body.String())
	}
}

func TestAuthHandler_CreateAPIKey_InvalidExpiration(t *testing.T) {
	ts := newTestAuthServer(t)

	// Create tenant first
	ts.createTenant("test-tenant", "Test Tenant")

	body := CreateAPIKeyRequest{
		Name:      "bad-expiring-key",
		Tenant:    "test-tenant",
		ExpiresIn: "invalid",
	}

	rr := ts.postJSON("/v1/auth/keys", body)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestAuthHandler_ListAPIKeys(t *testing.T) {
	ts := newTestAuthServer(t)

	// Create tenant first
	ts.createTenant("list-tenant", "List Tenant")

	// Create a key first
	body := CreateAPIKeyRequest{
		Name:   "list-test-key",
		Tenant: "list-tenant",
	}
	ts.postJSON("/v1/auth/keys", body)

	// List all keys
	rr := ts.get("/v1/auth/keys")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	keys, ok := result["keys"].([]interface{})
	if !ok || keys == nil {
		t.Fatal("Expected keys array in response")
	}
	if len(keys) == 0 {
		t.Error("Expected at least one key")
	}
}

func TestAuthHandler_ListAPIKeys_ByTenant(t *testing.T) {
	ts := newTestAuthServer(t)

	// Create tenants first
	ts.createTenant("tenant-a", "Tenant A")
	ts.createTenant("tenant-b", "Tenant B")

	// Create keys for different tenants
	ts.postJSON("/v1/auth/keys", CreateAPIKeyRequest{Name: "key1", Tenant: "tenant-a"})
	ts.postJSON("/v1/auth/keys", CreateAPIKeyRequest{Name: "key2", Tenant: "tenant-b"})

	// List keys for tenant-a only
	rr := ts.get("/v1/auth/keys?tenant=tenant-a")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	keys, ok := result["keys"].([]interface{})
	if !ok || keys == nil {
		t.Fatal("Expected keys array in response")
	}
	for _, k := range keys {
		key := k.(map[string]interface{})
		if key["tenant"] != "tenant-a" {
			t.Errorf("Expected all keys to be for tenant-a, got %v", key["tenant"])
		}
	}
}

func TestAuthHandler_GetAPIKey(t *testing.T) {
	ts := newTestAuthServer(t)

	// Create tenant first
	ts.createTenant("get-tenant", "Get Tenant")

	// Create a key
	body := CreateAPIKeyRequest{
		Name:   "get-test-key",
		Tenant: "get-tenant",
	}
	createRr := ts.postJSON("/v1/auth/keys", body)

	if createRr.Code != http.StatusCreated {
		t.Fatalf("Failed to create API key: %s", createRr.Body.String())
	}

	var createResult map[string]interface{}
	json.Unmarshal(createRr.Body.Bytes(), &createResult)
	keyID := createResult["id"].(string)

	// Get the key
	rr := ts.get("/v1/auth/keys/" + keyID)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if result["name"] != "get-test-key" {
		t.Errorf("Expected name 'get-test-key', got %v", result["name"])
	}
}

func TestAuthHandler_GetAPIKey_NotFound(t *testing.T) {
	ts := newTestAuthServer(t)

	rr := ts.get("/v1/auth/keys/nonexistent-id")

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, rr.Code)
	}
}

func TestAuthHandler_DeleteAPIKey(t *testing.T) {
	ts := newTestAuthServer(t)

	// Create tenant first
	ts.createTenant("delete-tenant", "Delete Tenant")

	// Create a key
	body := CreateAPIKeyRequest{
		Name:   "delete-test-key",
		Tenant: "delete-tenant",
	}
	createRr := ts.postJSON("/v1/auth/keys", body)

	if createRr.Code != http.StatusCreated {
		t.Fatalf("Failed to create API key: %s", createRr.Body.String())
	}

	var createResult map[string]interface{}
	json.Unmarshal(createRr.Body.Bytes(), &createResult)
	keyID := createResult["id"].(string)

	// Delete the key
	rr := ts.delete("/v1/auth/keys/" + keyID)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d; body: %s", http.StatusOK, rr.Code, rr.Body.String())
	}

	// Verify it's deleted
	getRr := ts.get("/v1/auth/keys/" + keyID)
	if getRr.Code != http.StatusNotFound {
		t.Errorf("Expected key to be deleted, but got status %d", getRr.Code)
	}
}

func TestAuthHandler_DeleteAPIKey_NotFound(t *testing.T) {
	ts := newTestAuthServer(t)

	rr := ts.delete("/v1/auth/keys/nonexistent-id")

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, rr.Code)
	}
}

func TestAuthHandler_RevokeAPIKey(t *testing.T) {
	ts := newTestAuthServer(t)

	// Create tenant first
	ts.createTenant("revoke-tenant", "Revoke Tenant")

	// Create a key
	body := CreateAPIKeyRequest{
		Name:   "revoke-test-key",
		Tenant: "revoke-tenant",
	}
	createRr := ts.postJSON("/v1/auth/keys", body)

	if createRr.Code != http.StatusCreated {
		t.Fatalf("Failed to create API key: %s", createRr.Body.String())
	}

	var createResult map[string]interface{}
	json.Unmarshal(createRr.Body.Bytes(), &createResult)
	keyID := createResult["id"].(string)

	// Revoke the key
	rr := ts.postJSON("/v1/auth/keys/"+keyID+"/revoke", nil)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d; body: %s", http.StatusOK, rr.Code, rr.Body.String())
	}

	var result map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if result["status"] != "revoked" {
		t.Errorf("Expected status 'revoked', got %v", result["status"])
	}
}

func TestAuthHandler_RevokeAPIKey_NotFound(t *testing.T) {
	ts := newTestAuthServer(t)

	rr := ts.postJSON("/v1/auth/keys/nonexistent-id/revoke", nil)

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, rr.Code)
	}
}

func TestAuthHandler_ValidateAPIKey(t *testing.T) {
	ts := newTestAuthServer(t)

	// Create tenant first
	ts.createTenant("validate-tenant", "Validate Tenant")

	// Create a key
	body := CreateAPIKeyRequest{
		Name:   "validate-test-key",
		Tenant: "validate-tenant",
	}
	createRr := ts.postJSON("/v1/auth/keys", body)

	if createRr.Code != http.StatusCreated {
		t.Fatalf("Failed to create API key: %s", createRr.Body.String())
	}

	var createResult map[string]interface{}
	json.Unmarshal(createRr.Body.Bytes(), &createResult)
	rawKey := createResult["key"].(string)

	// Validate the key
	validateBody := ValidateKeyRequest{
		Key: rawKey,
	}
	rr := ts.postJSON("/v1/auth/validate", validateBody)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d; body: %s", http.StatusOK, rr.Code, rr.Body.String())
	}

	var result map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if result["valid"] != true {
		t.Error("Expected key to be valid")
	}
}

func TestAuthHandler_ValidateAPIKey_Invalid(t *testing.T) {
	ts := newTestAuthServer(t)

	validateBody := ValidateKeyRequest{
		Key: "invalid-key",
	}
	rr := ts.postJSON("/v1/auth/validate", validateBody)

	// API returns 401 for invalid keys
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Expected status %d, got %d", http.StatusUnauthorized, rr.Code)
	}
}

func TestAuthHandler_ValidateAPIKey_InvalidBody(t *testing.T) {
	ts := newTestAuthServer(t)

	rr := ts.request(http.MethodPost, "/v1/auth/validate", "invalid json")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestAuthHandler_CreateTenant(t *testing.T) {
	ts := newTestAuthServer(t)

	body := map[string]interface{}{
		"id":          "new-tenant",
		"name":        "New Tenant",
		"description": "A test tenant",
	}

	rr := ts.postJSON("/v1/auth/tenants", body)

	if rr.Code != http.StatusCreated {
		t.Errorf("Expected status %d, got %d; body: %s", http.StatusCreated, rr.Code, rr.Body.String())
	}
}

func TestAuthHandler_ListTenants(t *testing.T) {
	ts := newTestAuthServer(t)

	// Create a tenant
	ts.postJSON("/v1/auth/tenants", map[string]interface{}{
		"id":   "list-tenant",
		"name": "List Tenant",
	})

	rr := ts.get("/v1/auth/tenants")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if result["tenants"] == nil {
		t.Error("Expected tenants in response")
	}
}

func TestAuthHandler_GetTenant(t *testing.T) {
	ts := newTestAuthServer(t)

	// Create a tenant
	ts.postJSON("/v1/auth/tenants", map[string]interface{}{
		"id":   "get-tenant",
		"name": "Get Tenant",
	})

	rr := ts.get("/v1/auth/tenants/get-tenant")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d; body: %s", http.StatusOK, rr.Code, rr.Body.String())
	}
}

func TestAuthHandler_GetTenant_NotFound(t *testing.T) {
	ts := newTestAuthServer(t)

	rr := ts.get("/v1/auth/tenants/nonexistent")

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, rr.Code)
	}
}

func TestAuthHandler_ListRoles(t *testing.T) {
	ts := newTestAuthServer(t)

	rr := ts.get("/v1/auth/roles")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	// Should have default roles
	roles, ok := result["roles"].([]interface{})
	if !ok || roles == nil {
		t.Fatal("Expected roles array in response")
	}
	if len(roles) == 0 {
		t.Error("Expected at least default roles")
	}
}

func TestAuthHandler_GetAuditLogs(t *testing.T) {
	ts := newTestAuthServer(t)

	rr := ts.get("/v1/auth/audit")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	// Response contains "logs" array (may be empty)
	if _, ok := result["logs"]; !ok {
		t.Error("Expected logs in response")
	}
}

func TestParseDuration(t *testing.T) {
	tests := []struct {
		input    string
		expected string // Expected duration as string for comparison
		wantErr  bool
	}{
		{"30d", "720h0m0s", false}, // 30 days = 720 hours
		{"7d", "168h0m0s", false},  // 7 days = 168 hours
		{"1w", "168h0m0s", false},  // 1 week = 168 hours
		{"1y", "8760h0m0s", false}, // 1 year = 8760 hours
		{"2h", "2h0m0s", false},    // Standard Go duration
		{"30m", "30m0s", false},    // Standard Go duration
		{"", "0s", false},          // Empty string
		{"invalid", "", true},      // Invalid format
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := parseDuration(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseDuration(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if !tt.wantErr && got.String() != tt.expected {
				t.Errorf("parseDuration(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}
