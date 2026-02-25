package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// --- PII Detection Tests ---

func TestGovernancePII_DetectPII(t *testing.T) {
	ts := newTestGovernanceServerConfigured(t)

	body := PIIDetectionRequest{
		Content: "My email is john@example.com and phone is 555-123-4567",
	}

	rr := ts.postJSON("/v1/governance/pii/detect", body)
	if rr.Code != http.StatusOK {
		t.Errorf("Expected %d, got %d; body: %s", http.StatusOK, rr.Code, rr.Body.String())
	}

	var result map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if _, ok := result["detections"]; !ok {
		t.Error("Expected detections in response")
	}
}

func TestGovernancePII_DetectPII_EmptyContent(t *testing.T) {
	ts := newTestGovernanceServerConfigured(t)

	rr := ts.postJSON("/v1/governance/pii/detect", PIIDetectionRequest{})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestGovernancePII_DetectPII_InvalidBody(t *testing.T) {
	ts := newTestGovernanceServerConfigured(t)

	rr := ts.request(http.MethodPost, "/v1/governance/pii/detect", "invalid json")
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestGovernancePII_DetectPII_WithCategories(t *testing.T) {
	ts := newTestGovernanceServerConfigured(t)

	body := PIIDetectionRequest{
		Content:    "john@example.com 555-123-4567",
		Categories: []string{"email"},
	}

	rr := ts.postJSON("/v1/governance/pii/detect", body)
	if rr.Code != http.StatusOK {
		t.Errorf("Expected %d, got %d; body: %s", http.StatusOK, rr.Code, rr.Body.String())
	}
}

func TestGovernancePII_ScanPII(t *testing.T) {
	ts := newTestGovernanceServerConfigured(t)

	body := PIIScanRequest{
		Contents: []string{"john@example.com", "no pii here", "555-123-4567"},
	}

	rr := ts.postJSON("/v1/governance/pii/scan", body)
	if rr.Code != http.StatusOK {
		t.Errorf("Expected %d, got %d; body: %s", http.StatusOK, rr.Code, rr.Body.String())
	}
}

func TestGovernancePII_ScanPII_EmptyContents(t *testing.T) {
	ts := newTestGovernanceServerConfigured(t)

	rr := ts.postJSON("/v1/governance/pii/scan", PIIScanRequest{})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestGovernancePII_ListPatterns(t *testing.T) {
	ts := newTestGovernanceServerConfigured(t)

	rr := ts.get("/v1/governance/pii/patterns")
	if rr.Code != http.StatusOK {
		t.Errorf("Expected %d, got %d", http.StatusOK, rr.Code)
	}
}

func TestGovernancePII_AddPattern(t *testing.T) {
	ts := newTestGovernanceServerConfigured(t)

	body := PIIPatternRequest{
		Name:        "custom-ssn",
		Category:    "ssn",
		Pattern:     `\d{3}-\d{2}-\d{4}`,
		Sensitivity: "high",
		Description: "Social Security Number",
	}

	rr := ts.postJSON("/v1/governance/pii/patterns", body)
	if rr.Code != http.StatusCreated {
		t.Errorf("Expected %d, got %d; body: %s", http.StatusCreated, rr.Code, rr.Body.String())
	}
}

func TestGovernancePII_AddPattern_MissingFields(t *testing.T) {
	ts := newTestGovernanceServerConfigured(t)

	rr := ts.postJSON("/v1/governance/pii/patterns", PIIPatternRequest{Name: "x"})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestGovernancePII_RemovePattern(t *testing.T) {
	ts := newTestGovernanceServerConfigured(t)

	rr := ts.delete("/v1/governance/pii/patterns/some-pattern")
	if rr.Code != http.StatusNotImplemented {
		t.Errorf("Expected %d, got %d", http.StatusNotImplemented, rr.Code)
	}
}

// --- Data Masking Tests ---

func TestGovernanceMask_MaskData(t *testing.T) {
	ts := newTestGovernanceServerConfigured(t)

	body := MaskRequest{
		Value: "john@example.com",
		Field: "email",
	}

	rr := ts.postJSON("/v1/governance/mask", body)
	if rr.Code != http.StatusOK {
		t.Errorf("Expected %d, got %d; body: %s", http.StatusOK, rr.Code, rr.Body.String())
	}

	var result map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if result["original"] != "john@example.com" {
		t.Errorf("Expected original preserved, got %v", result["original"])
	}
}

func TestGovernanceMask_MaskData_EmptyValue(t *testing.T) {
	ts := newTestGovernanceServerConfigured(t)

	rr := ts.postJSON("/v1/governance/mask", MaskRequest{Field: "email"})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestGovernanceMask_MaskData_WithPrincipal(t *testing.T) {
	ts := newTestGovernanceServerConfigured(t)

	body := MaskRequest{
		Value:     "sensitive-data",
		Field:     "ssn",
		Principal: &PrincipalContext{ID: "user-1", Roles: []string{"admin"}},
	}

	rr := ts.postJSON("/v1/governance/mask", body)
	if rr.Code != http.StatusOK {
		t.Errorf("Expected %d, got %d; body: %s", http.StatusOK, rr.Code, rr.Body.String())
	}
}

func TestGovernanceMask_MaskBatch(t *testing.T) {
	ts := newTestGovernanceServerConfigured(t)

	body := MaskBatchRequest{
		Items: []MaskRequest{
			{Value: "john@example.com", Field: "email"},
			{Value: "555-123-4567", Field: "phone"},
		},
	}

	rr := ts.postJSON("/v1/governance/mask/batch", body)
	if rr.Code != http.StatusOK {
		t.Errorf("Expected %d, got %d; body: %s", http.StatusOK, rr.Code, rr.Body.String())
	}

	var result map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	results, ok := result["results"].([]interface{})
	if !ok {
		t.Fatal("Expected results array")
	}
	if len(results) != 2 {
		t.Errorf("Expected 2 results, got %d", len(results))
	}
}

func TestGovernanceMask_MaskBatch_Empty(t *testing.T) {
	ts := newTestGovernanceServerConfigured(t)

	rr := ts.postJSON("/v1/governance/mask/batch", MaskBatchRequest{})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestGovernanceMask_ListRules(t *testing.T) {
	ts := newTestGovernanceServerConfigured(t)

	rr := ts.get("/v1/governance/mask/rules")
	if rr.Code != http.StatusOK {
		t.Errorf("Expected %d, got %d", http.StatusOK, rr.Code)
	}
}

func TestGovernanceMask_AddRule(t *testing.T) {
	ts := newTestGovernanceServerConfigured(t)

	body := MaskingRuleRequest{
		Field:       "email",
		Type:        "redact",
		Categories:  []string{"email"},
		Sensitivity: "high",
	}

	rr := ts.postJSON("/v1/governance/mask/rules", body)
	if rr.Code != http.StatusCreated {
		t.Errorf("Expected %d, got %d; body: %s", http.StatusCreated, rr.Code, rr.Body.String())
	}
}

func TestGovernanceMask_AddRule_MissingFields(t *testing.T) {
	ts := newTestGovernanceServerConfigured(t)

	rr := ts.postJSON("/v1/governance/mask/rules", MaskingRuleRequest{Field: "x"})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestGovernanceMask_RemoveRule(t *testing.T) {
	ts := newTestGovernanceServerConfigured(t)

	rr := ts.delete("/v1/governance/mask/rules/email")
	if rr.Code != http.StatusOK {
		t.Errorf("Expected %d, got %d", http.StatusOK, rr.Code)
	}
}

// --- Nil component tests ---

func TestGovernancePII_NilDetector(t *testing.T) {
	handler := NewGovernanceHandler(GovernanceHandlerConfig{})
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/v1/governance/pii/detect", strings.NewReader(`{"content":"test"}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected %d, got %d", http.StatusServiceUnavailable, rr.Code)
	}
}

func TestGovernanceMask_NilEngine(t *testing.T) {
	handler := NewGovernanceHandler(GovernanceHandlerConfig{})
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/v1/governance/mask", strings.NewReader(`{"value":"test","field":"x"}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected %d, got %d", http.StatusServiceUnavailable, rr.Code)
	}
}
