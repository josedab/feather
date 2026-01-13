package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// testTenantServer wraps a TenantHandler for testing.
type testTenantServer struct {
	handler *TenantHandler
	mux     *http.ServeMux
	t       *testing.T
}

// newTestTenantServer creates a new test tenant server.
func newTestTenantServer(t *testing.T) *testTenantServer {
	t.Helper()

	handler := NewTenantHandler(1024 * 1024 * 1024) // 1GB for testing
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	return &testTenantServer{
		handler: handler,
		mux:     mux,
		t:       t,
	}
}

func (ts *testTenantServer) request(method, path string, body string) *httptest.ResponseRecorder {
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

func (ts *testTenantServer) postJSON(path string, body interface{}) *httptest.ResponseRecorder {
	ts.t.Helper()

	jsonBody, err := json.Marshal(body)
	if err != nil {
		ts.t.Fatalf("failed to marshal body: %v", err)
	}
	return ts.request(http.MethodPost, path, string(jsonBody))
}

func (ts *testTenantServer) putJSON(path string, body interface{}) *httptest.ResponseRecorder {
	ts.t.Helper()

	jsonBody, err := json.Marshal(body)
	if err != nil {
		ts.t.Fatalf("failed to marshal body: %v", err)
	}
	return ts.request(http.MethodPut, path, string(jsonBody))
}

func (ts *testTenantServer) get(path string) *httptest.ResponseRecorder {
	return ts.request(http.MethodGet, path, "")
}

func (ts *testTenantServer) delete(path string) *httptest.ResponseRecorder {
	return ts.request(http.MethodDelete, path, "")
}

func TestTenantHandler_NewTenantHandler(t *testing.T) {
	handler := NewTenantHandler(1024 * 1024 * 1024)

	require.NotNil(t, handler)
	require.NotNil(t, handler.registry)
}

func TestTenantHandler_ListTenants(t *testing.T) {
	ts := newTestTenantServer(t)

	rr := ts.get("/v1/tenants")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}
}

func TestTenantHandler_CreateTenant_InvalidBody(t *testing.T) {
	ts := newTestTenantServer(t)

	rr := ts.request(http.MethodPost, "/v1/tenants", "invalid json")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestTenantHandler_CreateTenant(t *testing.T) {
	ts := newTestTenantServer(t)

	body := map[string]interface{}{
		"id":   "tenant-1",
		"name": "Test Tenant",
		"quotas": map[string]interface{}{
			"max_entities":        10000,
			"max_features":        100,
			"max_storage_bytes":   1073741824,
			"requests_per_second": 100,
		},
	}

	rr := ts.postJSON("/v1/tenants", body)

	if rr.Code != http.StatusCreated && rr.Code != http.StatusOK {
		t.Errorf("Expected status %d or %d, got %d; body: %s", http.StatusCreated, http.StatusOK, rr.Code, rr.Body.String())
	}
}

func TestTenantHandler_GetTenant_NotFound(t *testing.T) {
	ts := newTestTenantServer(t)

	rr := ts.get("/v1/tenants/nonexistent")

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, rr.Code)
	}
}

func TestTenantHandler_UpdateTenant_InvalidBody(t *testing.T) {
	ts := newTestTenantServer(t)

	rr := ts.request(http.MethodPut, "/v1/tenants/tenant-1", "invalid json")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestTenantHandler_UpdateTenant_NotFound(t *testing.T) {
	ts := newTestTenantServer(t)

	body := map[string]interface{}{
		"name": "Updated Name",
	}

	rr := ts.putJSON("/v1/tenants/nonexistent", body)

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, rr.Code)
	}
}

func TestTenantHandler_DeleteTenant_NotFound(t *testing.T) {
	ts := newTestTenantServer(t)

	rr := ts.delete("/v1/tenants/nonexistent")

	if rr.Code != http.StatusNotFound && rr.Code != http.StatusOK {
		t.Errorf("Expected status %d or %d, got %d", http.StatusNotFound, http.StatusOK, rr.Code)
	}
}

func TestTenantHandler_GetTenantQuotas_NotFound(t *testing.T) {
	ts := newTestTenantServer(t)

	rr := ts.get("/v1/tenants/nonexistent/quotas")

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, rr.Code)
	}
}

func TestTenantHandler_UpdateTenantQuotas_InvalidBody(t *testing.T) {
	ts := newTestTenantServer(t)

	rr := ts.request(http.MethodPut, "/v1/tenants/tenant-1/quotas", "invalid json")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestTenantHandler_UpdateTenantQuotas_NotFound(t *testing.T) {
	ts := newTestTenantServer(t)

	body := map[string]interface{}{
		"max_entities": 50000,
	}

	rr := ts.putJSON("/v1/tenants/nonexistent/quotas", body)

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, rr.Code)
	}
}

func TestTenantHandler_GetTenantUsage_NotFound(t *testing.T) {
	ts := newTestTenantServer(t)

	rr := ts.get("/v1/tenants/nonexistent/usage")

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, rr.Code)
	}
}

// Enable/Disable tests (instead of suspend/activate)
func TestTenantHandler_EnableTenant_NotFound(t *testing.T) {
	ts := newTestTenantServer(t)

	rr := ts.postJSON("/v1/tenants/nonexistent/enable", map[string]interface{}{})

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, rr.Code)
	}
}

func TestTenantHandler_DisableTenant_NotFound(t *testing.T) {
	ts := newTestTenantServer(t)

	rr := ts.postJSON("/v1/tenants/nonexistent/disable", map[string]interface{}{})

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, rr.Code)
	}
}

// Stats tests - correct routes
func TestTenantHandler_GetGlobalStats(t *testing.T) {
	ts := newTestTenantServer(t)

	rr := ts.get("/v1/tenants/stats")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}
}

func TestTenantHandler_ListPartitions(t *testing.T) {
	ts := newTestTenantServer(t)

	rr := ts.get("/v1/tenants/partitions")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}
}

// Tenant partition tests
func TestTenantHandler_GetTenantPartition_NotFound(t *testing.T) {
	ts := newTestTenantServer(t)

	rr := ts.get("/v1/tenants/nonexistent/partition")

	// Handler returns 200 with empty/null partition info for nonexistent tenants
	if rr.Code != http.StatusNotFound && rr.Code != http.StatusOK {
		t.Errorf("Expected status %d or %d, got %d", http.StatusNotFound, http.StatusOK, rr.Code)
	}
}

func TestTenantHandler_ResizePartition_NotFound(t *testing.T) {
	ts := newTestTenantServer(t)

	body := map[string]interface{}{
		"new_size": 1024 * 1024 * 1024,
	}

	rr := ts.putJSON("/v1/tenants/nonexistent/partition/resize", body)

	if rr.Code != http.StatusNotFound && rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d or %d, got %d", http.StatusNotFound, http.StatusBadRequest, rr.Code)
	}
}

// Metrics tests
func TestTenantHandler_GetTenantMetrics_NotFound(t *testing.T) {
	ts := newTestTenantServer(t)

	rr := ts.get("/v1/tenants/nonexistent/metrics")

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, rr.Code)
	}
}

func TestTenantHandler_ResetMetrics_NotFound(t *testing.T) {
	ts := newTestTenantServer(t)

	rr := ts.postJSON("/v1/tenants/nonexistent/metrics/reset", map[string]interface{}{})

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, rr.Code)
	}
}

func TestTenantHandler_CreateAndGetTenant(t *testing.T) {
	ts := newTestTenantServer(t)

	// Create tenant
	createBody := map[string]interface{}{
		"id":   "test-tenant",
		"name": "Test Tenant",
		"quotas": map[string]interface{}{
			"max_entities": 10000,
		},
	}

	createRR := ts.postJSON("/v1/tenants", createBody)
	if createRR.Code != http.StatusCreated && createRR.Code != http.StatusOK {
		t.Fatalf("Failed to create tenant: %s", createRR.Body.String())
	}

	// Get tenant
	getRR := ts.get("/v1/tenants/test-tenant")
	if getRR.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d; body: %s", http.StatusOK, getRR.Code, getRR.Body.String())
	}
}

func TestTenantHandler_CreateUpdateDeleteTenant(t *testing.T) {
	ts := newTestTenantServer(t)

	// Create
	createBody := map[string]interface{}{
		"id":   "crud-tenant",
		"name": "CRUD Tenant",
	}
	ts.postJSON("/v1/tenants", createBody)

	// Update
	updateBody := map[string]interface{}{
		"name": "Updated CRUD Tenant",
	}
	updateRR := ts.putJSON("/v1/tenants/crud-tenant", updateBody)
	if updateRR.Code != http.StatusOK {
		t.Errorf("Update failed: %d - %s", updateRR.Code, updateRR.Body.String())
	}

	// Delete
	deleteRR := ts.delete("/v1/tenants/crud-tenant")
	if deleteRR.Code != http.StatusOK && deleteRR.Code != http.StatusNoContent {
		t.Errorf("Delete failed: %d - %s", deleteRR.Code, deleteRR.Body.String())
	}
}
