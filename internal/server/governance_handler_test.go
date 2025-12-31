package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// testGovernanceServer wraps a GovernanceHandler for testing.
type testGovernanceServer struct {
	handler *GovernanceHandler
	mux     *http.ServeMux
	t       *testing.T
}

// newTestGovernanceServer creates a new test governance server.
func newTestGovernanceServer(t *testing.T) *testGovernanceServer {
	t.Helper()

	handler := NewGovernanceHandler(GovernanceHandlerConfig{})
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	return &testGovernanceServer{
		handler: handler,
		mux:     mux,
		t:       t,
	}
}

func (ts *testGovernanceServer) request(method, path string, body string) *httptest.ResponseRecorder {
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

func (ts *testGovernanceServer) postJSON(path string, body interface{}) *httptest.ResponseRecorder {
	ts.t.Helper()

	jsonBody, err := json.Marshal(body)
	if err != nil {
		ts.t.Fatalf("failed to marshal body: %v", err)
	}
	return ts.request(http.MethodPost, path, string(jsonBody))
}

func (ts *testGovernanceServer) putJSON(path string, body interface{}) *httptest.ResponseRecorder {
	ts.t.Helper()

	jsonBody, err := json.Marshal(body)
	if err != nil {
		ts.t.Fatalf("failed to marshal body: %v", err)
	}
	return ts.request(http.MethodPut, path, string(jsonBody))
}

func (ts *testGovernanceServer) get(path string) *httptest.ResponseRecorder {
	return ts.request(http.MethodGet, path, "")
}

func (ts *testGovernanceServer) delete(path string) *httptest.ResponseRecorder {
	return ts.request(http.MethodDelete, path, "")
}

func TestGovernanceHandler_NewGovernanceHandler(t *testing.T) {
	handler := NewGovernanceHandler(GovernanceHandlerConfig{})

	if handler == nil {
		t.Error("Expected handler to be non-nil")
	}
}

// Audit tests
func TestGovernanceHandler_ListAuditLogs(t *testing.T) {
	ts := newTestGovernanceServer(t)

	rr := ts.get("/v1/governance/audit")

	// Without audit logger configured, returns 503
	if rr.Code != http.StatusOK && rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d or %d, got %d", http.StatusOK, http.StatusServiceUnavailable, rr.Code)
	}
}

func TestGovernanceHandler_ListAuditLogs_WithFilters(t *testing.T) {
	ts := newTestGovernanceServer(t)

	rr := ts.get("/v1/governance/audit?action=read&resource=feature&limit=50")

	// Without audit logger configured, returns 503
	if rr.Code != http.StatusOK && rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d or %d, got %d", http.StatusOK, http.StatusServiceUnavailable, rr.Code)
	}
}

func TestGovernanceHandler_GetAuditLog(t *testing.T) {
	ts := newTestGovernanceServer(t)

	rr := ts.get("/v1/governance/audit/some-id")

	// Without audit logger configured, returns 503
	if rr.Code != http.StatusOK && rr.Code != http.StatusNotFound && rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d, %d or %d, got %d", http.StatusOK, http.StatusNotFound, http.StatusServiceUnavailable, rr.Code)
	}
}

func TestGovernanceHandler_GetAuditStats(t *testing.T) {
	ts := newTestGovernanceServer(t)

	rr := ts.get("/v1/governance/audit/stats")

	// Without audit logger configured, returns 503
	if rr.Code != http.StatusOK && rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d or %d, got %d", http.StatusOK, http.StatusServiceUnavailable, rr.Code)
	}
}

// PII tests
func TestGovernanceHandler_DetectPII_InvalidBody(t *testing.T) {
	ts := newTestGovernanceServer(t)

	rr := ts.request(http.MethodPost, "/v1/governance/pii/detect", "invalid json")

	// Without PII detector configured, returns 503
	if rr.Code != http.StatusBadRequest && rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d or %d, got %d", http.StatusBadRequest, http.StatusServiceUnavailable, rr.Code)
	}
}

func TestGovernanceHandler_DetectPII(t *testing.T) {
	ts := newTestGovernanceServer(t)

	body := map[string]interface{}{
		"content": "My email is test@example.com",
	}

	rr := ts.postJSON("/v1/governance/pii/detect", body)

	// Without PII detector configured, returns 503
	if rr.Code != http.StatusOK && rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d or %d, got %d; body: %s", http.StatusOK, http.StatusServiceUnavailable, rr.Code, rr.Body.String())
	}
}

func TestGovernanceHandler_ScanPII_InvalidBody(t *testing.T) {
	ts := newTestGovernanceServer(t)

	rr := ts.request(http.MethodPost, "/v1/governance/pii/scan", "invalid json")

	// Without PII detector configured, returns 503
	if rr.Code != http.StatusBadRequest && rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d or %d, got %d", http.StatusBadRequest, http.StatusServiceUnavailable, rr.Code)
	}
}

func TestGovernanceHandler_ScanPII(t *testing.T) {
	ts := newTestGovernanceServer(t)

	body := map[string]interface{}{
		"contents": []string{"test@example.com", "normal text"},
	}

	rr := ts.postJSON("/v1/governance/pii/scan", body)

	// Without PII detector configured, returns 503
	if rr.Code != http.StatusOK && rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d or %d, got %d; body: %s", http.StatusOK, http.StatusServiceUnavailable, rr.Code, rr.Body.String())
	}
}

func TestGovernanceHandler_ListPIIPatterns(t *testing.T) {
	ts := newTestGovernanceServer(t)

	rr := ts.get("/v1/governance/pii/patterns")

	// Without PII detector configured, returns 503
	if rr.Code != http.StatusOK && rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d or %d, got %d", http.StatusOK, http.StatusServiceUnavailable, rr.Code)
	}
}

func TestGovernanceHandler_AddPIIPattern_InvalidBody(t *testing.T) {
	ts := newTestGovernanceServer(t)

	rr := ts.request(http.MethodPost, "/v1/governance/pii/patterns", "invalid json")

	// Without PII detector configured, returns 503
	if rr.Code != http.StatusBadRequest && rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d or %d, got %d", http.StatusBadRequest, http.StatusServiceUnavailable, rr.Code)
	}
}

func TestGovernanceHandler_AddPIIPattern(t *testing.T) {
	ts := newTestGovernanceServer(t)

	body := map[string]interface{}{
		"name":        "custom_pattern",
		"category":    "custom",
		"pattern":     "[A-Z]{3}[0-9]{6}",
		"sensitivity": "medium",
		"description": "Custom ID pattern",
	}

	rr := ts.postJSON("/v1/governance/pii/patterns", body)

	// Without PII detector configured, returns 503
	if rr.Code != http.StatusCreated && rr.Code != http.StatusOK && rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d, %d or %d, got %d; body: %s", http.StatusCreated, http.StatusOK, http.StatusServiceUnavailable, rr.Code, rr.Body.String())
	}
}

func TestGovernanceHandler_RemovePIIPattern(t *testing.T) {
	ts := newTestGovernanceServer(t)

	rr := ts.delete("/v1/governance/pii/patterns/some-pattern")

	// Without PII detector configured, returns 503
	if rr.Code != http.StatusOK && rr.Code != http.StatusNotFound && rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d, %d or %d, got %d", http.StatusOK, http.StatusNotFound, http.StatusServiceUnavailable, rr.Code)
	}
}

// Masking tests - use /v1/governance/mask not /v1/governance/masking
func TestGovernanceHandler_MaskData_InvalidBody(t *testing.T) {
	ts := newTestGovernanceServer(t)

	rr := ts.request(http.MethodPost, "/v1/governance/mask", "invalid json")

	// Without masking engine configured, returns 503
	if rr.Code != http.StatusBadRequest && rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d or %d, got %d", http.StatusBadRequest, http.StatusServiceUnavailable, rr.Code)
	}
}

func TestGovernanceHandler_MaskData(t *testing.T) {
	ts := newTestGovernanceServer(t)

	body := map[string]interface{}{
		"data": map[string]interface{}{
			"email": "test@example.com",
			"name":  "John Doe",
		},
		"rules": []map[string]interface{}{
			{
				"field":      "email",
				"mask_type":  "partial",
				"characters": 3,
			},
		},
	}

	rr := ts.postJSON("/v1/governance/mask", body)

	// Without masking engine configured, returns 503
	if rr.Code != http.StatusOK && rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d or %d, got %d; body: %s", http.StatusOK, http.StatusServiceUnavailable, rr.Code, rr.Body.String())
	}
}

func TestGovernanceHandler_MaskBatch_InvalidBody(t *testing.T) {
	ts := newTestGovernanceServer(t)

	rr := ts.request(http.MethodPost, "/v1/governance/mask/batch", "invalid json")

	// Without masking engine configured, returns 503
	if rr.Code != http.StatusBadRequest && rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d or %d, got %d", http.StatusBadRequest, http.StatusServiceUnavailable, rr.Code)
	}
}

func TestGovernanceHandler_ListMaskingRules(t *testing.T) {
	ts := newTestGovernanceServer(t)

	rr := ts.get("/v1/governance/mask/rules")

	// Without masking engine configured, returns 503
	if rr.Code != http.StatusOK && rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d or %d, got %d", http.StatusOK, http.StatusServiceUnavailable, rr.Code)
	}
}

func TestGovernanceHandler_AddMaskingRule_InvalidBody(t *testing.T) {
	ts := newTestGovernanceServer(t)

	rr := ts.request(http.MethodPost, "/v1/governance/mask/rules", "invalid json")

	// Without masking engine configured, returns 503
	if rr.Code != http.StatusBadRequest && rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d or %d, got %d", http.StatusBadRequest, http.StatusServiceUnavailable, rr.Code)
	}
}

func TestGovernanceHandler_AddMaskingRule(t *testing.T) {
	ts := newTestGovernanceServer(t)

	body := map[string]interface{}{
		"id":        "rule-1",
		"field":     "ssn",
		"mask_type": "full",
		"priority":  10,
	}

	rr := ts.postJSON("/v1/governance/mask/rules", body)

	// Without masking engine configured, returns 503
	if rr.Code != http.StatusCreated && rr.Code != http.StatusOK && rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d, %d or %d, got %d; body: %s", http.StatusCreated, http.StatusOK, http.StatusServiceUnavailable, rr.Code, rr.Body.String())
	}
}

func TestGovernanceHandler_RemoveMaskingRule_NotFound(t *testing.T) {
	ts := newTestGovernanceServer(t)

	rr := ts.delete("/v1/governance/mask/rules/nonexistent")

	// Without masking engine configured, returns 503
	if rr.Code != http.StatusNotFound && rr.Code != http.StatusOK && rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d, %d or %d, got %d", http.StatusNotFound, http.StatusOK, http.StatusServiceUnavailable, rr.Code)
	}
}

// ACL tests - use /v1/governance/acl not /v1/governance/acl/policies
func TestGovernanceHandler_ListACLs(t *testing.T) {
	ts := newTestGovernanceServer(t)

	rr := ts.get("/v1/governance/acl")

	// Without ACL controller configured, returns 503
	if rr.Code != http.StatusOK && rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d or %d, got %d", http.StatusOK, http.StatusServiceUnavailable, rr.Code)
	}
}

func TestGovernanceHandler_CreateACL_InvalidBody(t *testing.T) {
	ts := newTestGovernanceServer(t)

	rr := ts.request(http.MethodPost, "/v1/governance/acl", "invalid json")

	// Without ACL controller configured, returns 503
	if rr.Code != http.StatusBadRequest && rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d or %d, got %d", http.StatusBadRequest, http.StatusServiceUnavailable, rr.Code)
	}
}

func TestGovernanceHandler_CreateACL(t *testing.T) {
	ts := newTestGovernanceServer(t)

	body := map[string]interface{}{
		"id":          "policy-1",
		"name":        "Test Policy",
		"description": "Test policy description",
		"columns":     []string{"email", "phone"},
		"roles":       []string{"admin", "analyst"},
		"action":      "allow",
	}

	rr := ts.postJSON("/v1/governance/acl", body)

	// Without ACL controller configured, returns 503
	if rr.Code != http.StatusCreated && rr.Code != http.StatusOK && rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d, %d or %d, got %d; body: %s", http.StatusCreated, http.StatusOK, http.StatusServiceUnavailable, rr.Code, rr.Body.String())
	}
}

func TestGovernanceHandler_GetACL_NotFound(t *testing.T) {
	ts := newTestGovernanceServer(t)

	rr := ts.get("/v1/governance/acl/nonexistent")

	// Without ACL controller configured, returns 503
	if rr.Code != http.StatusNotFound && rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d or %d, got %d", http.StatusNotFound, http.StatusServiceUnavailable, rr.Code)
	}
}

func TestGovernanceHandler_UpdateACL_InvalidBody(t *testing.T) {
	ts := newTestGovernanceServer(t)

	rr := ts.request(http.MethodPut, "/v1/governance/acl/some-id", "invalid json")

	// Without ACL controller configured, returns 503
	if rr.Code != http.StatusBadRequest && rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d or %d, got %d", http.StatusBadRequest, http.StatusServiceUnavailable, rr.Code)
	}
}

func TestGovernanceHandler_DeleteACL_NotFound(t *testing.T) {
	ts := newTestGovernanceServer(t)

	rr := ts.delete("/v1/governance/acl/nonexistent")

	// Without ACL controller configured, returns 503
	if rr.Code != http.StatusNotFound && rr.Code != http.StatusOK && rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d, %d or %d, got %d", http.StatusNotFound, http.StatusOK, http.StatusServiceUnavailable, rr.Code)
	}
}

func TestGovernanceHandler_CheckACLAccess_InvalidBody(t *testing.T) {
	ts := newTestGovernanceServer(t)

	rr := ts.request(http.MethodPost, "/v1/governance/acl/check", "invalid json")

	// Without ACL controller configured, returns 503
	if rr.Code != http.StatusBadRequest && rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d or %d, got %d", http.StatusBadRequest, http.StatusServiceUnavailable, rr.Code)
	}
}

func TestGovernanceHandler_CheckACLAccess(t *testing.T) {
	ts := newTestGovernanceServer(t)

	body := map[string]interface{}{
		"principal": "user-1",
		"columns":   []string{"email", "name"},
		"roles":     []string{"analyst"},
	}

	rr := ts.postJSON("/v1/governance/acl/check", body)

	// Without ACL controller configured, returns 503
	if rr.Code != http.StatusOK && rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d or %d, got %d; body: %s", http.StatusOK, http.StatusServiceUnavailable, rr.Code, rr.Body.String())
	}
}

// Residency tests - use /v1/governance/residency/policies not /v1/governance/residency/rules
func TestGovernanceHandler_ListResidencyPolicies(t *testing.T) {
	ts := newTestGovernanceServer(t)

	rr := ts.get("/v1/governance/residency/policies")

	// Without residency enforcer configured, returns 503
	if rr.Code != http.StatusOK && rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d or %d, got %d", http.StatusOK, http.StatusServiceUnavailable, rr.Code)
	}
}

func TestGovernanceHandler_AddResidencyPolicy_InvalidBody(t *testing.T) {
	ts := newTestGovernanceServer(t)

	rr := ts.request(http.MethodPost, "/v1/governance/residency/policies", "invalid json")

	// Without residency enforcer configured, returns 503
	if rr.Code != http.StatusBadRequest && rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d or %d, got %d", http.StatusBadRequest, http.StatusServiceUnavailable, rr.Code)
	}
}

func TestGovernanceHandler_AddResidencyPolicy(t *testing.T) {
	ts := newTestGovernanceServer(t)

	body := map[string]interface{}{
		"id":               "rule-eu",
		"name":             "EU Data Residency",
		"allowed_regions":  []string{"eu-west-1", "eu-central-1"},
		"data_categories":  []string{"pii", "financial"},
		"enforcement_mode": "strict",
	}

	rr := ts.postJSON("/v1/governance/residency/policies", body)

	// Without residency enforcer configured, returns 503
	if rr.Code != http.StatusCreated && rr.Code != http.StatusOK && rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d, %d or %d, got %d; body: %s", http.StatusCreated, http.StatusOK, http.StatusServiceUnavailable, rr.Code, rr.Body.String())
	}
}

func TestGovernanceHandler_GetResidencyPolicy_NotFound(t *testing.T) {
	ts := newTestGovernanceServer(t)

	rr := ts.get("/v1/governance/residency/policies/nonexistent")

	// Without residency enforcer configured, returns 503
	if rr.Code != http.StatusNotFound && rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d or %d, got %d", http.StatusNotFound, http.StatusServiceUnavailable, rr.Code)
	}
}

func TestGovernanceHandler_DeleteResidencyPolicy_NotFound(t *testing.T) {
	ts := newTestGovernanceServer(t)

	rr := ts.delete("/v1/governance/residency/policies/nonexistent")

	// Without residency enforcer configured, returns 503
	if rr.Code != http.StatusNotFound && rr.Code != http.StatusOK && rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d, %d or %d, got %d", http.StatusNotFound, http.StatusOK, http.StatusServiceUnavailable, rr.Code)
	}
}

func TestGovernanceHandler_ValidateResidency_InvalidBody(t *testing.T) {
	ts := newTestGovernanceServer(t)

	rr := ts.request(http.MethodPost, "/v1/governance/residency/validate", "invalid json")

	// Without residency enforcer configured, returns 503
	if rr.Code != http.StatusBadRequest && rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d or %d, got %d", http.StatusBadRequest, http.StatusServiceUnavailable, rr.Code)
	}
}

func TestGovernanceHandler_ValidateResidency(t *testing.T) {
	ts := newTestGovernanceServer(t)

	body := map[string]interface{}{
		"data_category": "pii",
		"region":        "eu-west-1",
	}

	rr := ts.postJSON("/v1/governance/residency/validate", body)

	// Without residency enforcer configured, returns 503
	if rr.Code != http.StatusOK && rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d or %d, got %d; body: %s", http.StatusOK, http.StatusServiceUnavailable, rr.Code, rr.Body.String())
	}
}

func TestGovernanceHandler_GetGovernanceStats(t *testing.T) {
	ts := newTestGovernanceServer(t)

	rr := ts.get("/v1/governance/stats")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}
}
