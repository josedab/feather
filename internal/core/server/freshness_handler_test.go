package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/feather-store/feather/internal/extensions/freshness"
)

func createTestFreshnessHandler() *FreshnessHandler {
	config := freshness.DefaultManagerConfig()
	config.Monitor.CleanupInterval = 1 * time.Hour
	manager := freshness.NewManager(config)
	return NewFreshnessHandler(manager)
}

func TestNewFreshnessHandler(t *testing.T) {
	handler := createTestFreshnessHandler()
	if handler == nil {
		t.Fatal("Expected handler to be non-nil")
	}
}

func TestFreshnessHandler_RegisterRoutes(t *testing.T) {
	handler := createTestFreshnessHandler()
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
}

func TestFreshnessHandler_GetAllMetrics(t *testing.T) {
	handler := createTestFreshnessHandler()
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	req := httptest.NewRequest("GET", "/v1/freshness/metrics", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestFreshnessHandler_GetFeatureMetrics_NotFound(t *testing.T) {
	handler := createTestFreshnessHandler()
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	req := httptest.NewRequest("GET", "/v1/freshness/metrics/nonexistent", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestFreshnessHandler_GetTTL(t *testing.T) {
	handler := createTestFreshnessHandler()
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	req := httptest.NewRequest("GET", "/v1/freshness/ttl/test_feature", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestFreshnessHandler_GetAllPredictions(t *testing.T) {
	handler := createTestFreshnessHandler()
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	req := httptest.NewRequest("GET", "/v1/freshness/predictions", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestFreshnessHandler_GetPrediction(t *testing.T) {
	handler := createTestFreshnessHandler()
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	req := httptest.NewRequest("GET", "/v1/freshness/predictions/test_feature", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestFreshnessHandler_ListPolicies(t *testing.T) {
	handler := createTestFreshnessHandler()
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	req := httptest.NewRequest("GET", "/v1/freshness/policies", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestFreshnessHandler_CreatePolicy(t *testing.T) {
	handler := createTestFreshnessHandler()
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	policy := PolicyRequest{
		ID:             "test-policy",
		Name:           "Test Policy",
		Type:           freshness.PolicyTypeFixed,
		FeaturePattern: "*",
		Priority:       10,
		Enabled:        true,
		Config: freshness.PolicyConfig{
			FixedTTL: 5 * time.Minute,
		},
	}
	body, _ := json.Marshal(policy)

	req := httptest.NewRequest("POST", "/v1/freshness/policies", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("Expected status 201, got %d: %s", w.Code, w.Body.String())
	}
}

func TestFreshnessHandler_CreatePolicy_InvalidJSON(t *testing.T) {
	handler := createTestFreshnessHandler()
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	req := httptest.NewRequest("POST", "/v1/freshness/policies", bytes.NewReader([]byte("invalid")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestFreshnessHandler_CreatePolicy_Duplicate(t *testing.T) {
	handler := createTestFreshnessHandler()
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	policy := PolicyRequest{
		ID:             "dup-policy",
		Name:           "Duplicate Policy",
		Type:           freshness.PolicyTypeFixed,
		FeaturePattern: "*",
		Priority:       10,
		Enabled:        true,
		Config: freshness.PolicyConfig{
			FixedTTL: 5 * time.Minute,
		},
	}
	body, _ := json.Marshal(policy)

	// First create
	req := httptest.NewRequest("POST", "/v1/freshness/policies", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	// Second create (duplicate)
	req = httptest.NewRequest("POST", "/v1/freshness/policies", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("Expected status 409, got %d", w.Code)
	}
}

func TestFreshnessHandler_GetPolicy(t *testing.T) {
	handler := createTestFreshnessHandler()
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	// Create policy first
	policy := PolicyRequest{
		ID:             "get-policy",
		Name:           "Get Policy",
		Type:           freshness.PolicyTypeFixed,
		FeaturePattern: "*",
		Priority:       10,
		Enabled:        true,
		Config: freshness.PolicyConfig{
			FixedTTL: 5 * time.Minute,
		},
	}
	body, _ := json.Marshal(policy)
	req := httptest.NewRequest("POST", "/v1/freshness/policies", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	// Get policy
	req = httptest.NewRequest("GET", "/v1/freshness/policies/get-policy", nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestFreshnessHandler_GetPolicy_NotFound(t *testing.T) {
	handler := createTestFreshnessHandler()
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	req := httptest.NewRequest("GET", "/v1/freshness/policies/nonexistent", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestFreshnessHandler_UpdatePolicy(t *testing.T) {
	handler := createTestFreshnessHandler()
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	// Create policy first
	policy := PolicyRequest{
		ID:             "update-policy",
		Name:           "Update Policy",
		Type:           freshness.PolicyTypeFixed,
		FeaturePattern: "*",
		Priority:       10,
		Enabled:        true,
		Config: freshness.PolicyConfig{
			FixedTTL: 5 * time.Minute,
		},
	}
	body, _ := json.Marshal(policy)
	req := httptest.NewRequest("POST", "/v1/freshness/policies", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	// Update policy
	policy.Name = "Updated Policy"
	policy.Config.FixedTTL = 10 * time.Minute
	body, _ = json.Marshal(policy)
	req = httptest.NewRequest("PUT", "/v1/freshness/policies/update-policy", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestFreshnessHandler_UpdatePolicy_NotFound(t *testing.T) {
	handler := createTestFreshnessHandler()
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	policy := PolicyRequest{
		ID:             "nonexistent",
		Name:           "Policy",
		Type:           freshness.PolicyTypeFixed,
		FeaturePattern: "*",
		Priority:       10,
		Enabled:        true,
		Config: freshness.PolicyConfig{
			FixedTTL: 5 * time.Minute,
		},
	}
	body, _ := json.Marshal(policy)

	req := httptest.NewRequest("PUT", "/v1/freshness/policies/nonexistent", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestFreshnessHandler_DeletePolicy(t *testing.T) {
	handler := createTestFreshnessHandler()
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	// Create policy first
	policy := PolicyRequest{
		ID:             "delete-policy",
		Name:           "Delete Policy",
		Type:           freshness.PolicyTypeFixed,
		FeaturePattern: "*",
		Priority:       10,
		Enabled:        true,
		Config: freshness.PolicyConfig{
			FixedTTL: 5 * time.Minute,
		},
	}
	body, _ := json.Marshal(policy)
	req := httptest.NewRequest("POST", "/v1/freshness/policies", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	// Delete policy
	req = httptest.NewRequest("DELETE", "/v1/freshness/policies/delete-policy", nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("Expected status 204, got %d", w.Code)
	}
}

func TestFreshnessHandler_DeletePolicy_NotFound(t *testing.T) {
	handler := createTestFreshnessHandler()
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	req := httptest.NewRequest("DELETE", "/v1/freshness/policies/nonexistent", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestFreshnessHandler_RecordAccess(t *testing.T) {
	handler := createTestFreshnessHandler()
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	record := AccessRecordRequest{
		Feature:  "test_feature",
		Latency:  10 * time.Millisecond,
		CacheHit: true,
	}
	body, _ := json.Marshal(record)

	req := httptest.NewRequest("POST", "/v1/freshness/access", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Errorf("Expected status 202, got %d", w.Code)
	}
}

func TestFreshnessHandler_RecordAccess_MissingFeature(t *testing.T) {
	handler := createTestFreshnessHandler()
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	record := AccessRecordRequest{
		Feature:  "",
		Latency:  10 * time.Millisecond,
		CacheHit: true,
	}
	body, _ := json.Marshal(record)

	req := httptest.NewRequest("POST", "/v1/freshness/access", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestFreshnessHandler_RecordChange(t *testing.T) {
	handler := createTestFreshnessHandler()
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	record := ChangeRecordRequest{
		Feature:  "test_feature",
		OldValue: 100.0,
		NewValue: 110.0,
	}
	body, _ := json.Marshal(record)

	req := httptest.NewRequest("POST", "/v1/freshness/change", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Errorf("Expected status 202, got %d", w.Code)
	}
}

func TestFreshnessHandler_RecordChange_MissingFeature(t *testing.T) {
	handler := createTestFreshnessHandler()
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	record := ChangeRecordRequest{
		Feature:  "",
		OldValue: 100.0,
		NewValue: 110.0,
	}
	body, _ := json.Marshal(record)

	req := httptest.NewRequest("POST", "/v1/freshness/change", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestFreshnessHandler_RecordDrift(t *testing.T) {
	handler := createTestFreshnessHandler()
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	record := DriftRecordRequest{
		Feature:    "test_feature",
		DriftScore: 0.75,
	}
	body, _ := json.Marshal(record)

	req := httptest.NewRequest("POST", "/v1/freshness/drift", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Errorf("Expected status 202, got %d", w.Code)
	}
}

func TestFreshnessHandler_RecordStale(t *testing.T) {
	handler := createTestFreshnessHandler()
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	// Record access first
	access := AccessRecordRequest{
		Feature:  "test_feature",
		Latency:  10 * time.Millisecond,
		CacheHit: true,
	}
	accessBody, _ := json.Marshal(access)
	req := httptest.NewRequest("POST", "/v1/freshness/access", bytes.NewReader(accessBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	// Record stale
	record := StaleRecordRequest{
		Feature: "test_feature",
	}
	body, _ := json.Marshal(record)

	req = httptest.NewRequest("POST", "/v1/freshness/stale", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Errorf("Expected status 202, got %d", w.Code)
	}
}

func TestFreshnessHandler_GetStats(t *testing.T) {
	handler := createTestFreshnessHandler()
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	req := httptest.NewRequest("GET", "/v1/freshness/stats", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestFreshnessHandler_EvaluateAll(t *testing.T) {
	handler := createTestFreshnessHandler()
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	req := httptest.NewRequest("GET", "/v1/freshness/evaluate", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestFreshnessHandler_Integration(t *testing.T) {
	handler := createTestFreshnessHandler()
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	// Record some accesses
	for i := 0; i < 10; i++ {
		record := AccessRecordRequest{
			Feature:  "integration_feature",
			Latency:  time.Duration(i+1) * time.Millisecond,
			CacheHit: i%2 == 0,
		}
		body, _ := json.Marshal(record)
		req := httptest.NewRequest("POST", "/v1/freshness/access", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)
	}

	// Record some changes
	for i := 0; i < 5; i++ {
		record := ChangeRecordRequest{
			Feature:  "integration_feature",
			OldValue: float64(i * 10),
			NewValue: float64((i + 1) * 10),
		}
		body, _ := json.Marshal(record)
		req := httptest.NewRequest("POST", "/v1/freshness/change", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)
	}

	// Get metrics
	req := httptest.NewRequest("GET", "/v1/freshness/metrics/integration_feature", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200 for metrics, got %d", w.Code)
	}

	// Get TTL recommendation
	req = httptest.NewRequest("GET", "/v1/freshness/ttl/integration_feature", nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200 for TTL, got %d", w.Code)
	}

	// Create a policy
	policy := PolicyRequest{
		ID:             "integration-policy",
		Name:           "Integration Policy",
		Type:           freshness.PolicyTypeFixed,
		FeaturePattern: "integration_*",
		Priority:       10,
		Enabled:        true,
		Config: freshness.PolicyConfig{
			FixedTTL: 3 * time.Minute,
		},
	}
	body, _ := json.Marshal(policy)
	req = httptest.NewRequest("POST", "/v1/freshness/policies", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("Expected status 201 for policy creation, got %d", w.Code)
	}

	// Get TTL again (should be affected by policy)
	req = httptest.NewRequest("GET", "/v1/freshness/ttl/integration_feature", nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200 for TTL with policy, got %d", w.Code)
	}
}
