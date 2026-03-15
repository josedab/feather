package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/feather-store/feather/internal/extensions/qualitygates"
)

type testQualityGatesServer struct {
	handler   *QualityGatesHandler
	validator *qualitygates.Validator
	mux       *http.ServeMux
	t         *testing.T
}

func newTestQualityGatesServer(t *testing.T) *testQualityGatesServer {
	t.Helper()
	validator := qualitygates.NewValidator(qualitygates.DefaultConfig())
	handler := NewQualityGatesHandler(validator)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	return &testQualityGatesServer{handler: handler, validator: validator, mux: mux, t: t}
}

func (ts *testQualityGatesServer) request(method, path string, body string) *httptest.ResponseRecorder {
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

func TestQualityGatesHandler_GetStats(t *testing.T) {
	ts := newTestQualityGatesServer(t)
	rr := ts.request(http.MethodGet, "/v1/quality/gates/stats", "")
	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}
	var result map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}
}

func TestQualityGatesHandler_ValidateSchema(t *testing.T) {
	ts := newTestQualityGatesServer(t)
	body := `{"name":"test_schema","features":[{"name":"clicks","data_type":"int64"}]}`
	rr := ts.request(http.MethodPost, "/v1/quality/validate/schema", body)
	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d; body: %s", http.StatusOK, rr.Code, rr.Body.String())
	}
}

func TestQualityGatesHandler_ValidateSchema_InvalidJSON(t *testing.T) {
	ts := newTestQualityGatesServer(t)
	rr := ts.request(http.MethodPost, "/v1/quality/validate/schema", "not json")
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestQualityGatesHandler_AssertQuality(t *testing.T) {
	ts := newTestQualityGatesServer(t)
	body := `{"feature":"clicks","samples":[1.0,2.0,3.0,4.0,5.0]}`
	rr := ts.request(http.MethodPost, "/v1/quality/validate/data", body)
	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d; body: %s", http.StatusOK, rr.Code, rr.Body.String())
	}
}

func TestQualityGatesHandler_AssertQuality_MissingFeature(t *testing.T) {
	ts := newTestQualityGatesServer(t)
	body := `{"feature":"","samples":[1.0]}`
	rr := ts.request(http.MethodPost, "/v1/quality/validate/data", body)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestQualityGatesHandler_GenerateReport_InvalidJSON(t *testing.T) {
	ts := newTestQualityGatesServer(t)
	rr := ts.request(http.MethodPost, "/v1/quality/report", "not json")
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}
