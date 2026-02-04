package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/feather-store/feather/internal/extensions/fedlearning"
)

type testFedLearningServer struct {
	handler *FedLearningHandler
	adapter *fedlearning.Adapter
	mux     *http.ServeMux
	t       *testing.T
}

func newTestFedLearningServer(t *testing.T) *testFedLearningServer {
	t.Helper()
	adapter := fedlearning.NewAdapter(fedlearning.DefaultConfig())
	handler := NewFedLearningHandler(adapter)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	return &testFedLearningServer{handler: handler, adapter: adapter, mux: mux, t: t}
}

func (ts *testFedLearningServer) request(method, path string, body string) *httptest.ResponseRecorder {
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

func TestFedLearningHandler_GetStats(t *testing.T) {
	ts := newTestFedLearningServer(t)
	rr := ts.request(http.MethodGet, "/v1/fedlearn/stats", "")
	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}
	var result map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}
}

func TestFedLearningHandler_RegisterOrg(t *testing.T) {
	ts := newTestFedLearningServer(t)
	body := `{"id":"org-1","config":{"name":"Test Org"}}`
	rr := ts.request(http.MethodPost, "/v1/fedlearn/orgs", body)
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

func TestFedLearningHandler_RegisterOrg_InvalidJSON(t *testing.T) {
	ts := newTestFedLearningServer(t)
	rr := ts.request(http.MethodPost, "/v1/fedlearn/orgs", "not json")
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestFedLearningHandler_RegisterOrg_MissingID(t *testing.T) {
	ts := newTestFedLearningServer(t)
	body := `{"id":"","config":{}}`
	rr := ts.request(http.MethodPost, "/v1/fedlearn/orgs", body)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestFedLearningHandler_ListOrgs(t *testing.T) {
	ts := newTestFedLearningServer(t)
	rr := ts.request(http.MethodGet, "/v1/fedlearn/orgs", "")
	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}
	var result map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}
	if _, ok := result["orgs"]; !ok {
		t.Error("Expected orgs key in response")
	}
}

func TestFedLearningHandler_SetPolicy_InvalidJSON(t *testing.T) {
	ts := newTestFedLearningServer(t)
	rr := ts.request(http.MethodPost, "/v1/fedlearn/policy", "not json")
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestFedLearningHandler_SubmitGradient_MissingFields(t *testing.T) {
	ts := newTestFedLearningServer(t)
	body := `{"org_id":"","feature":"","gradient":[1.0]}`
	rr := ts.request(http.MethodPost, "/v1/fedlearn/gradient", body)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestFedLearningHandler_DeregisterOrg_NotFound(t *testing.T) {
	ts := newTestFedLearningServer(t)
	req := httptest.NewRequest(http.MethodDelete, "/v1/fedlearn/orgs/nonexistent", nil)
	rr := httptest.NewRecorder()
	ts.mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, rr.Code)
	}
}
