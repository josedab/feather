package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/feather-store/feather/internal/platform/governance"
)

// newTestGovernanceServerConfigured creates a server with real governance components.
func newTestGovernanceServerConfigured(t *testing.T) *testGovernanceServer {
	t.Helper()

	pii, err := governance.NewPIIDetector(governance.PIIConfig{Enabled: true})
	if err != nil {
		t.Fatalf("failed to create PII detector: %v", err)
	}

	masking := governance.NewDataMasker(governance.MaskingConfig{
		Enabled:            true,
		DefaultMaskingType: "redact",
		DefaultRedactValue: "[REDACTED]",
	}, pii)

	acl := governance.NewColumnACLController(governance.ACLConfig{
		Enabled:       true,
		DefaultEffect: "deny",
	}, nil)

	residency := governance.NewResidencyController(governance.ResidencyConfig{
		Enabled:       true,
		CurrentRegion: "us-east-1",
	}, nil)

	handler := NewGovernanceHandler(GovernanceHandlerConfig{
		PII:       pii,
		Masking:   masking,
		ACL:       acl,
		Residency: residency,
	})
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	return &testGovernanceServer{handler: handler, mux: mux, t: t}
}

// --- ACL Tests with configured components ---

func TestGovernanceACL_CreateACL(t *testing.T) {
	ts := newTestGovernanceServerConfigured(t)

	body := ACLRequest{
		ID:          "acl-1",
		Resource:    "feature_x",
		Principal:   "user-1",
		Permissions: []string{"read"},
		Effect:      "allow",
	}

	rr := ts.postJSON("/v1/governance/acl", body)
	if rr.Code != http.StatusCreated {
		t.Errorf("Expected %d, got %d; body: %s", http.StatusCreated, rr.Code, rr.Body.String())
	}
}

func TestGovernanceACL_CreateACL_InvalidBody(t *testing.T) {
	ts := newTestGovernanceServerConfigured(t)

	rr := ts.request(http.MethodPost, "/v1/governance/acl", "invalid json")
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestGovernanceACL_CreateACL_MissingFields(t *testing.T) {
	ts := newTestGovernanceServerConfigured(t)

	rr := ts.postJSON("/v1/governance/acl", ACLRequest{ID: "acl-x"})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestGovernanceACL_GetACL(t *testing.T) {
	ts := newTestGovernanceServerConfigured(t)

	ts.postJSON("/v1/governance/acl", ACLRequest{
		ID: "acl-get", Resource: "feat", Principal: "u1", Permissions: []string{"read"}, Effect: "allow",
	})

	rr := ts.get("/v1/governance/acl/acl-get")
	if rr.Code != http.StatusOK {
		t.Errorf("Expected %d, got %d; body: %s", http.StatusOK, rr.Code, rr.Body.String())
	}
}

func TestGovernanceACL_GetACL_NotFound(t *testing.T) {
	ts := newTestGovernanceServerConfigured(t)

	rr := ts.get("/v1/governance/acl/nonexistent")
	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected %d, got %d", http.StatusNotFound, rr.Code)
	}
}

func TestGovernanceACL_ListACLs(t *testing.T) {
	ts := newTestGovernanceServerConfigured(t)

	ts.postJSON("/v1/governance/acl", ACLRequest{
		ID: "acl-list-1", Resource: "feat", Principal: "u1", Permissions: []string{"read"}, Effect: "allow",
	})

	rr := ts.get("/v1/governance/acl")
	if rr.Code != http.StatusOK {
		t.Errorf("Expected %d, got %d", http.StatusOK, rr.Code)
	}
}

func TestGovernanceACL_UpdateACL(t *testing.T) {
	ts := newTestGovernanceServerConfigured(t)

	ts.postJSON("/v1/governance/acl", ACLRequest{
		ID: "acl-update", Resource: "feat", Principal: "u1", Permissions: []string{"read"}, Effect: "allow",
	})

	rr := ts.request(http.MethodPut, "/v1/governance/acl/acl-update",
		`{"resource":"feat","principal":"u2","permissions":["write"],"effect":"deny"}`)
	if rr.Code != http.StatusOK {
		t.Errorf("Expected %d, got %d; body: %s", http.StatusOK, rr.Code, rr.Body.String())
	}
}

func TestGovernanceACL_UpdateACL_NotFound(t *testing.T) {
	ts := newTestGovernanceServerConfigured(t)

	rr := ts.request(http.MethodPut, "/v1/governance/acl/nonexistent",
		`{"resource":"feat","principal":"u1","permissions":["read"],"effect":"allow"}`)
	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected %d, got %d", http.StatusNotFound, rr.Code)
	}
}

func TestGovernanceACL_UpdateACL_InvalidBody(t *testing.T) {
	ts := newTestGovernanceServerConfigured(t)

	rr := ts.request(http.MethodPut, "/v1/governance/acl/some-id", "invalid json")
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestGovernanceACL_DeleteACL(t *testing.T) {
	ts := newTestGovernanceServerConfigured(t)

	ts.postJSON("/v1/governance/acl", ACLRequest{
		ID: "acl-delete", Resource: "feat", Principal: "u1", Permissions: []string{"read"}, Effect: "allow",
	})

	rr := ts.delete("/v1/governance/acl/acl-delete")
	if rr.Code != http.StatusOK {
		t.Errorf("Expected %d, got %d; body: %s", http.StatusOK, rr.Code, rr.Body.String())
	}
}

func TestGovernanceACL_DeleteACL_NotFound(t *testing.T) {
	ts := newTestGovernanceServerConfigured(t)

	rr := ts.delete("/v1/governance/acl/nonexistent")
	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected %d, got %d", http.StatusNotFound, rr.Code)
	}
}

func TestGovernanceACL_CheckAccess(t *testing.T) {
	ts := newTestGovernanceServerConfigured(t)

	ts.postJSON("/v1/governance/acl", ACLRequest{
		ID: "acl-check", Resource: "feature_a", Principal: "user-1",
		Permissions: []string{"read"}, Effect: "allow",
	})

	body := AccessCheckRequest{
		Resource:   "feature_a",
		Permission: "read",
		Principal:  &PrincipalContext{ID: "user-1"},
	}

	rr := ts.postJSON("/v1/governance/acl/check", body)
	if rr.Code != http.StatusOK {
		t.Errorf("Expected %d, got %d; body: %s", http.StatusOK, rr.Code, rr.Body.String())
	}
}

func TestGovernanceACL_CheckAccess_InvalidBody(t *testing.T) {
	ts := newTestGovernanceServerConfigured(t)

	rr := ts.request(http.MethodPost, "/v1/governance/acl/check", "invalid json")
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestGovernanceACL_CheckAccess_MissingFields(t *testing.T) {
	ts := newTestGovernanceServerConfigured(t)

	rr := ts.postJSON("/v1/governance/acl/check", AccessCheckRequest{Resource: "x"})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

// --- Residency Tests ---

func TestGovernanceResidency_AddPolicy(t *testing.T) {
	ts := newTestGovernanceServerConfigured(t)

	body := ResidencyPolicyRequest{
		ID:      "policy-1",
		Regions: []string{"us-east-1", "eu-west-1"},
	}

	rr := ts.postJSON("/v1/governance/residency/policies", body)
	if rr.Code != http.StatusCreated {
		t.Errorf("Expected %d, got %d; body: %s", http.StatusCreated, rr.Code, rr.Body.String())
	}
}

func TestGovernanceResidency_AddPolicy_MissingFields(t *testing.T) {
	ts := newTestGovernanceServerConfigured(t)

	rr := ts.postJSON("/v1/governance/residency/policies", ResidencyPolicyRequest{ID: "x"})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestGovernanceResidency_Validate(t *testing.T) {
	ts := newTestGovernanceServerConfigured(t)

	body := ResidencyValidationRequest{
		SourceRegion: "us-east-1",
		TargetRegion: "eu-west-1",
		Operation:    "read",
	}

	rr := ts.postJSON("/v1/governance/residency/validate", body)
	if rr.Code != http.StatusOK {
		t.Errorf("Expected %d, got %d; body: %s", http.StatusOK, rr.Code, rr.Body.String())
	}
}

func TestGovernanceResidency_Validate_MissingFields(t *testing.T) {
	ts := newTestGovernanceServerConfigured(t)

	rr := ts.postJSON("/v1/governance/residency/validate", ResidencyValidationRequest{SourceRegion: "x"})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

// --- Nil component tests ---

func TestGovernanceACL_NilController(t *testing.T) {
	handler := NewGovernanceHandler(GovernanceHandlerConfig{})
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/v1/governance/acl", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected %d, got %d", http.StatusServiceUnavailable, rr.Code)
	}
}
