package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/feather-store/feather/internal/extensions/notebooksdk"
)

type testNotebookSDKServer struct {
	handler *NotebookSDKHandler
	service *notebooksdk.Service
	mux     *http.ServeMux
	t       *testing.T
}

func newTestNotebookSDKServer(t *testing.T) *testNotebookSDKServer {
	t.Helper()
	service := notebooksdk.NewService(notebooksdk.DefaultConfig())
	handler := NewNotebookSDKHandler(service)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	return &testNotebookSDKServer{handler: handler, service: service, mux: mux, t: t}
}

func (ts *testNotebookSDKServer) request(method, path string, body string) *httptest.ResponseRecorder {
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

func TestNotebookSDKHandler_GetStats(t *testing.T) {
	ts := newTestNotebookSDKServer(t)
	rr := ts.request(http.MethodGet, "/v1/notebook/stats", "")
	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}
	var result map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}
}

func TestNotebookSDKHandler_CreateSession(t *testing.T) {
	ts := newTestNotebookSDKServer(t)
	body := `{"notebook":"test-session"}`
	rr := ts.request(http.MethodPost, "/v1/notebook/sessions", body)
	if rr.Code != http.StatusCreated {
		t.Errorf("Expected status %d, got %d; body: %s", http.StatusCreated, rr.Code, rr.Body.String())
	}
}

func TestNotebookSDKHandler_CreateSession_InvalidJSON(t *testing.T) {
	ts := newTestNotebookSDKServer(t)
	rr := ts.request(http.MethodPost, "/v1/notebook/sessions", "not json")
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestNotebookSDKHandler_ListSessions(t *testing.T) {
	ts := newTestNotebookSDKServer(t)
	rr := ts.request(http.MethodGet, "/v1/notebook/sessions", "")
	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}
	var result map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}
	if _, ok := result["sessions"]; !ok {
		t.Error("Expected sessions key in response")
	}
}

func TestNotebookSDKHandler_GetSession_NotFound(t *testing.T) {
	ts := newTestNotebookSDKServer(t)
	rr := ts.request(http.MethodGet, "/v1/notebook/sessions/nonexistent", "")
	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, rr.Code)
	}
}

func TestNotebookSDKHandler_Execute_MissingFields(t *testing.T) {
	ts := newTestNotebookSDKServer(t)
	body := `{"session_id":"","command":""}`
	rr := ts.request(http.MethodPost, "/v1/notebook/execute", body)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestNotebookSDKHandler_Visualize_InvalidJSON(t *testing.T) {
	ts := newTestNotebookSDKServer(t)
	rr := ts.request(http.MethodPost, "/v1/notebook/visualize", "not json")
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}
