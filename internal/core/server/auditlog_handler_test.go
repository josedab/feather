package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/feather-store/feather/internal/extensions/auditlog"
)

type testAuditLogServer struct {
	handler *AuditLogHandler
	logger  *auditlog.Logger
	mux     *http.ServeMux
	t       *testing.T
}

func newTestAuditLogServer(t *testing.T) *testAuditLogServer {
	t.Helper()

	logger := auditlog.NewLogger(auditlog.DefaultLoggerConfig())
	handler := NewAuditLogHandler(logger)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	return &testAuditLogServer{
		handler: handler,
		logger:  logger,
		mux:     mux,
		t:       t,
	}
}

func (ts *testAuditLogServer) request(method, path string, body string) *httptest.ResponseRecorder {
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

func (ts *testAuditLogServer) postJSON(path string, body interface{}) *httptest.ResponseRecorder {
	ts.t.Helper()

	jsonBody, err := json.Marshal(body)
	if err != nil {
		ts.t.Fatalf("failed to marshal body: %v", err)
	}
	return ts.request(http.MethodPost, path, string(jsonBody))
}

func (ts *testAuditLogServer) get(path string) *httptest.ResponseRecorder {
	return ts.request(http.MethodGet, path, "")
}

func TestAuditLogHandler_NewHandler(t *testing.T) {
	logger := auditlog.NewLogger(auditlog.DefaultLoggerConfig())
	handler := NewAuditLogHandler(logger)

	if handler.logger == nil {
		t.Error("Expected logger to be set")
	}
}

func TestAuditLogHandler_LogEntry(t *testing.T) {
	ts := newTestAuditLogServer(t)

	entry := auditlog.AuditEntry{
		ID:       "entry-1",
		Action:   auditlog.ActionRead,
		Actor:    "user:admin",
		Resource: "feature_group:clicks",
		Success:  true,
	}

	rr := ts.postJSON("/v1/audit/log", entry)

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

func TestAuditLogHandler_LogEntry_InvalidBody(t *testing.T) {
	ts := newTestAuditLogServer(t)

	rr := ts.request(http.MethodPost, "/v1/audit/log", "invalid json")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestAuditLogHandler_QueryLogs(t *testing.T) {
	ts := newTestAuditLogServer(t)

	rr := ts.get("/v1/audit/logs")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if _, ok := result["entries"]; !ok {
		t.Error("Expected entries key in response")
	}
}

func TestAuditLogHandler_QueryLogs_WithFilters(t *testing.T) {
	ts := newTestAuditLogServer(t)

	rr := ts.get("/v1/audit/logs?action=read&actor=admin&since=2024-01-01T00:00:00Z&limit=10")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}
}

func TestAuditLogHandler_GetEntry_NotFound(t *testing.T) {
	ts := newTestAuditLogServer(t)

	rr := ts.get("/v1/audit/logs/nonexistent")

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, rr.Code)
	}
}

func TestAuditLogHandler_GetEntry(t *testing.T) {
	ts := newTestAuditLogServer(t)

	entry := auditlog.AuditEntry{
		ID:       "entry-1",
		Action:   auditlog.ActionRead,
		Actor:    "user:admin",
		Resource: "feature_group:clicks",
		Success:  true,
	}
	ts.logger.Log(entry)

	rr := ts.get("/v1/audit/logs/entry-1")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}
}

func TestAuditLogHandler_Export(t *testing.T) {
	ts := newTestAuditLogServer(t)

	body := map[string]interface{}{
		"filter": map[string]interface{}{},
		"format": "json",
	}

	rr := ts.postJSON("/v1/audit/export", body)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d; body: %s", http.StatusOK, rr.Code, rr.Body.String())
	}
}

func TestAuditLogHandler_Export_InvalidBody(t *testing.T) {
	ts := newTestAuditLogServer(t)

	rr := ts.request(http.MethodPost, "/v1/audit/export", "invalid json")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestAuditLogHandler_Purge(t *testing.T) {
	ts := newTestAuditLogServer(t)

	body := map[string]interface{}{
		"before": "2024-01-01T00:00:00Z",
	}

	rr := ts.postJSON("/v1/audit/purge", body)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d; body: %s", http.StatusOK, rr.Code, rr.Body.String())
	}

	var result SuccessResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if !result.Success {
		t.Error("Expected success to be true")
	}
}

func TestAuditLogHandler_Purge_InvalidTimestamp(t *testing.T) {
	ts := newTestAuditLogServer(t)

	body := map[string]interface{}{
		"before": "not-a-timestamp",
	}

	rr := ts.postJSON("/v1/audit/purge", body)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestAuditLogHandler_Purge_InvalidBody(t *testing.T) {
	ts := newTestAuditLogServer(t)

	rr := ts.request(http.MethodPost, "/v1/audit/purge", "invalid json")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestAuditLogHandler_GetStats(t *testing.T) {
	ts := newTestAuditLogServer(t)

	rr := ts.get("/v1/audit/stats")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}
}
