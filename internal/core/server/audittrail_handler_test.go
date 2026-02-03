package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/feather-store/feather/internal/extensions/audittrail"
)

type testAuditTrailServer struct {
	handler *AuditTrailHandler
	trail   *audittrail.Trail
	mux     *http.ServeMux
	t       *testing.T
}

func newTestAuditTrailServer(t *testing.T) *testAuditTrailServer {
	t.Helper()
	trail := audittrail.New(audittrail.DefaultConfig())
	handler := NewAuditTrailHandler(trail)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	return &testAuditTrailServer{handler: handler, trail: trail, mux: mux, t: t}
}

func (ts *testAuditTrailServer) request(method, path string, body string) *httptest.ResponseRecorder {
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

func TestAuditTrailHandler_GetStats(t *testing.T) {
	ts := newTestAuditTrailServer(t)
	rr := ts.request(http.MethodGet, "/v1/audit/trail/stats", "")
	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}
	var result map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}
}

func TestAuditTrailHandler_Record(t *testing.T) {
	ts := newTestAuditTrailServer(t)
	body := `{"entity":"user:123","feature":"clicks","action":"write","actor":"admin"}`
	rr := ts.request(http.MethodPost, "/v1/audit/trail/record", body)
	if rr.Code != http.StatusCreated {
		t.Errorf("Expected status %d, got %d; body: %s", http.StatusCreated, rr.Code, rr.Body.String())
	}
	var result SuccessResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}
	if !result.Success {
		t.Error("Expected success to be true")
	}
}

func TestAuditTrailHandler_Record_InvalidJSON(t *testing.T) {
	ts := newTestAuditTrailServer(t)
	rr := ts.request(http.MethodPost, "/v1/audit/trail/record", "not json")
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestAuditTrailHandler_QueryByEntity(t *testing.T) {
	ts := newTestAuditTrailServer(t)
	rr := ts.request(http.MethodGet, "/v1/audit/trail/entity/user123", "")
	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}
	var result map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}
	if _, ok := result["entity"]; !ok {
		t.Error("Expected entity key in response")
	}
}

func TestAuditTrailHandler_QueryByFeature(t *testing.T) {
	ts := newTestAuditTrailServer(t)
	rr := ts.request(http.MethodGet, "/v1/audit/trail/feature/clicks", "")
	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}
	var result map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}
	if _, ok := result["feature"]; !ok {
		t.Error("Expected feature key in response")
	}
}

func TestAuditTrailHandler_VerifyChain(t *testing.T) {
	ts := newTestAuditTrailServer(t)
	rr := ts.request(http.MethodGet, "/v1/audit/trail/verify", "")
	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}
	var result map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}
	if _, ok := result["valid"]; !ok {
		t.Error("Expected valid key in response")
	}
}

func TestAuditTrailHandler_ComplianceReport_InvalidJSON(t *testing.T) {
	ts := newTestAuditTrailServer(t)
	rr := ts.request(http.MethodPost, "/v1/audit/trail/compliance", "not json")
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}
