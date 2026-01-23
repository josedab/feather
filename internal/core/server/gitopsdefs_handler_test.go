package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/feather-store/feather/internal/extensions/gitopsdefs"
)

type testGitOpsDefsServer struct {
	handler    *GitOpsDefsHandler
	reconciler *gitopsdefs.Reconciler
	mux        *http.ServeMux
	t          *testing.T
}

func newTestGitOpsDefsServer(t *testing.T) *testGitOpsDefsServer {
	t.Helper()
	reconciler := gitopsdefs.NewReconciler(gitopsdefs.DefaultReconcilerConfig())
	handler := NewGitOpsDefsHandler(reconciler)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	return &testGitOpsDefsServer{handler: handler, reconciler: reconciler, mux: mux, t: t}
}

func (ts *testGitOpsDefsServer) request(method, path string, body string) *httptest.ResponseRecorder {
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

func TestGitOpsDefsHandler_GetStats(t *testing.T) {
	ts := newTestGitOpsDefsServer(t)
	rr := ts.request(http.MethodGet, "/v1/gitops/stats", "")
	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}
	var result map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}
}

func TestGitOpsDefsHandler_LoadDefinition(t *testing.T) {
	ts := newTestGitOpsDefsServer(t)
	body := `{"name":"click_rate","version":"v1","type":"float64","entity":"user","description":"Click-through rate"}`
	rr := ts.request(http.MethodPost, "/v1/gitops/definitions", body)
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
}

func TestGitOpsDefsHandler_LoadDefinition_InvalidJSON(t *testing.T) {
	ts := newTestGitOpsDefsServer(t)
	rr := ts.request(http.MethodPost, "/v1/gitops/definitions", "not json")
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestGitOpsDefsHandler_GetDefinition_NotFound(t *testing.T) {
	ts := newTestGitOpsDefsServer(t)
	rr := ts.request(http.MethodGet, "/v1/gitops/definitions/nonexistent", "")
	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, rr.Code)
	}
}
