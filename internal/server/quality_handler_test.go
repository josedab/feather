package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/feather-store/feather/internal/quality"
)

// testQualityServer wraps a QualityHandler for testing.
type testQualityServer struct {
	handler   *QualityHandler
	validator *quality.Validator
	mux       *http.ServeMux
	t         *testing.T
}

// newTestQualityServer creates a new test quality server.
func newTestQualityServer(t *testing.T) *testQualityServer {
	t.Helper()

	validator := quality.NewValidator()
	handler := NewQualityHandler(validator)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	return &testQualityServer{
		handler:   handler,
		validator: validator,
		mux:       mux,
		t:         t,
	}
}

// newTestQualityServerWithoutValidator creates a quality server without validator for testing nil case.
func newTestQualityServerWithoutValidator(t *testing.T) *testQualityServer {
	t.Helper()

	handler := NewQualityHandler(nil)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	return &testQualityServer{
		handler:   handler,
		validator: nil,
		mux:       mux,
		t:         t,
	}
}

func (ts *testQualityServer) request(method, path string, body string) *httptest.ResponseRecorder {
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

func (ts *testQualityServer) postJSON(path string, body interface{}) *httptest.ResponseRecorder {
	ts.t.Helper()

	jsonBody, err := json.Marshal(body)
	if err != nil {
		ts.t.Fatalf("failed to marshal body: %v", err)
	}
	return ts.request(http.MethodPost, path, string(jsonBody))
}

func (ts *testQualityServer) get(path string) *httptest.ResponseRecorder {
	return ts.request(http.MethodGet, path, "")
}

func (ts *testQualityServer) delete(path string) *httptest.ResponseRecorder {
	return ts.request(http.MethodDelete, path, "")
}

// addRule is a helper to add a validation rule for testing.
func (ts *testQualityServer) addRule(id, featureID string) *httptest.ResponseRecorder {
	ts.t.Helper()

	body := ValidationRuleJSON{
		ID:          id,
		Name:        "Test Rule",
		Description: "Test validation rule",
		Type:        "not_null",
		FeatureID:   featureID,
		Severity:    "warning",
		Enabled:     true,
	}

	return ts.postJSON("/v1/quality/rules", body)
}

func TestQualityHandler_NewQualityHandler(t *testing.T) {
	validator := quality.NewValidator()
	handler := NewQualityHandler(validator)

	if handler.validator == nil {
		t.Error("Expected validator to be set")
	}
}

func TestQualityHandler_ListRules_Empty(t *testing.T) {
	ts := newTestQualityServer(t)

	rr := ts.get("/v1/quality/rules")

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

func TestQualityHandler_ListRules_NoValidator(t *testing.T) {
	ts := newTestQualityServerWithoutValidator(t)

	rr := ts.get("/v1/quality/rules")

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
	}
}

func TestQualityHandler_AddRule(t *testing.T) {
	ts := newTestQualityServer(t)

	body := ValidationRuleJSON{
		ID:          "rule-1",
		Name:        "Not Null Rule",
		Description: "Ensures value is not null",
		Type:        "not_null",
		FeatureID:   "feature-1",
		Severity:    "error",
		Enabled:     true,
	}

	rr := ts.postJSON("/v1/quality/rules", body)

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
	if result["rule_id"] != "rule-1" {
		t.Errorf("Expected rule_id 'rule-1', got %v", result["rule_id"])
	}
}

func TestQualityHandler_AddRule_InvalidBody(t *testing.T) {
	ts := newTestQualityServer(t)

	rr := ts.request(http.MethodPost, "/v1/quality/rules", "invalid json")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestQualityHandler_AddRule_NoValidator(t *testing.T) {
	ts := newTestQualityServerWithoutValidator(t)

	body := ValidationRuleJSON{
		ID:        "test",
		Name:      "Test",
		Type:      "not_null",
		FeatureID: "feature-1",
	}

	rr := ts.postJSON("/v1/quality/rules", body)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
	}
}

func TestQualityHandler_GetRule(t *testing.T) {
	ts := newTestQualityServer(t)

	// Add rule first
	ts.addRule("get-rule", "feature-1")

	rr := ts.get("/v1/quality/rules/get-rule")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d; body: %s", http.StatusOK, rr.Code, rr.Body.String())
	}
}

func TestQualityHandler_GetRule_NotFound(t *testing.T) {
	ts := newTestQualityServer(t)

	rr := ts.get("/v1/quality/rules/nonexistent")

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, rr.Code)
	}
}

func TestQualityHandler_GetRule_NoValidator(t *testing.T) {
	ts := newTestQualityServerWithoutValidator(t)

	rr := ts.get("/v1/quality/rules/test")

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
	}
}

func TestQualityHandler_RemoveRule(t *testing.T) {
	ts := newTestQualityServer(t)

	// Add rule first
	ts.addRule("remove-rule", "feature-1")

	rr := ts.delete("/v1/quality/rules/remove-rule")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d; body: %s", http.StatusOK, rr.Code, rr.Body.String())
	}
}

func TestQualityHandler_RemoveRule_NotFound(t *testing.T) {
	ts := newTestQualityServer(t)

	rr := ts.delete("/v1/quality/rules/nonexistent")

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, rr.Code)
	}
}

func TestQualityHandler_RemoveRule_NoValidator(t *testing.T) {
	ts := newTestQualityServerWithoutValidator(t)

	rr := ts.delete("/v1/quality/rules/test")

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
	}
}

func TestQualityHandler_GetRulesForFeature(t *testing.T) {
	ts := newTestQualityServer(t)

	// Add rules for feature
	ts.addRule("feature-rule-1", "test-feature")
	ts.addRule("feature-rule-2", "test-feature")

	rr := ts.get("/v1/quality/rules/feature/test-feature")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d; body: %s", http.StatusOK, rr.Code, rr.Body.String())
	}

	var result map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if result["feature_id"] != "test-feature" {
		t.Errorf("Expected feature_id 'test-feature', got %v", result["feature_id"])
	}
}

func TestQualityHandler_GetRulesForFeature_NoValidator(t *testing.T) {
	ts := newTestQualityServerWithoutValidator(t)

	rr := ts.get("/v1/quality/rules/feature/test")

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
	}
}

func TestQualityHandler_ValidateValue(t *testing.T) {
	ts := newTestQualityServer(t)

	body := ValidateValueRequest{
		FeatureID: "feature-1",
		Value:     "test value",
	}

	rr := ts.postJSON("/v1/quality/validate", body)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d; body: %s", http.StatusOK, rr.Code, rr.Body.String())
	}
}

func TestQualityHandler_ValidateValue_MissingFeatureID(t *testing.T) {
	ts := newTestQualityServer(t)

	body := ValidateValueRequest{
		Value: "test value",
	}

	rr := ts.postJSON("/v1/quality/validate", body)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestQualityHandler_ValidateValue_InvalidBody(t *testing.T) {
	ts := newTestQualityServer(t)

	rr := ts.request(http.MethodPost, "/v1/quality/validate", "invalid json")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestQualityHandler_ValidateValue_NoValidator(t *testing.T) {
	ts := newTestQualityServerWithoutValidator(t)

	body := ValidateValueRequest{
		FeatureID: "feature-1",
		Value:     "test",
	}

	rr := ts.postJSON("/v1/quality/validate", body)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
	}
}

func TestQualityHandler_ValidateBatch(t *testing.T) {
	ts := newTestQualityServer(t)

	body := ValidateBatchRequest{
		FeatureID: "feature-1",
		Values:    []interface{}{"value1", "value2", "value3"},
	}

	rr := ts.postJSON("/v1/quality/validate/batch", body)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d; body: %s", http.StatusOK, rr.Code, rr.Body.String())
	}
}

func TestQualityHandler_ValidateBatch_MissingFeatureID(t *testing.T) {
	ts := newTestQualityServer(t)

	body := ValidateBatchRequest{
		Values: []interface{}{"value1"},
	}

	rr := ts.postJSON("/v1/quality/validate/batch", body)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestQualityHandler_ValidateBatch_MissingValues(t *testing.T) {
	ts := newTestQualityServer(t)

	body := ValidateBatchRequest{
		FeatureID: "feature-1",
	}

	rr := ts.postJSON("/v1/quality/validate/batch", body)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestQualityHandler_ValidateBatch_InvalidBody(t *testing.T) {
	ts := newTestQualityServer(t)

	rr := ts.request(http.MethodPost, "/v1/quality/validate/batch", "invalid json")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestQualityHandler_ValidateBatch_NoValidator(t *testing.T) {
	ts := newTestQualityServerWithoutValidator(t)

	body := ValidateBatchRequest{
		FeatureID: "feature-1",
		Values:    []interface{}{"value1"},
	}

	rr := ts.postJSON("/v1/quality/validate/batch", body)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
	}
}

func TestQualityHandler_GetQualityScore(t *testing.T) {
	ts := newTestQualityServer(t)

	rr := ts.get("/v1/quality/score/feature-1")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d; body: %s", http.StatusOK, rr.Code, rr.Body.String())
	}
}

func TestQualityHandler_GetQualityScore_NoValidator(t *testing.T) {
	ts := newTestQualityServerWithoutValidator(t)

	rr := ts.get("/v1/quality/score/feature-1")

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
	}
}

func TestQualityHandler_GetHistory(t *testing.T) {
	ts := newTestQualityServer(t)

	rr := ts.get("/v1/quality/history")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if result["history"] == nil {
		t.Error("Expected history array in response")
	}
}

func TestQualityHandler_GetHistory_WithLimit(t *testing.T) {
	ts := newTestQualityServer(t)

	rr := ts.get("/v1/quality/history?limit=10")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}
}

func TestQualityHandler_GetHistory_NoValidator(t *testing.T) {
	ts := newTestQualityServerWithoutValidator(t)

	rr := ts.get("/v1/quality/history")

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
	}
}

func TestQualityHandler_GetStats(t *testing.T) {
	ts := newTestQualityServer(t)

	rr := ts.get("/v1/quality/stats")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}
}

func TestQualityHandler_GetStats_NoValidator(t *testing.T) {
	ts := newTestQualityServerWithoutValidator(t)

	rr := ts.get("/v1/quality/stats")

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
	}
}
