package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/feather-store/feather/internal/extensions/prefetch"
)

type testPrefetchServer struct {
	handler    *PrefetchHandler
	controller *prefetch.Controller
	mux        *http.ServeMux
	t          *testing.T
}

func newTestPrefetchServer(t *testing.T) *testPrefetchServer {
	t.Helper()
	controller := prefetch.NewController(prefetch.DefaultConfig())
	handler := NewPrefetchHandler(controller)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	return &testPrefetchServer{handler: handler, controller: controller, mux: mux, t: t}
}

func (ts *testPrefetchServer) request(method, path string, body string) *httptest.ResponseRecorder {
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

func TestPrefetchHandler_GetStats(t *testing.T) {
	ts := newTestPrefetchServer(t)
	rr := ts.request(http.MethodGet, "/v1/prefetch/stats", "")
	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}
	var result map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}
}

func TestPrefetchHandler_RecordAccess(t *testing.T) {
	ts := newTestPrefetchServer(t)
	body := `{"entity_key":"user:123","features":["clicks","views"]}`
	rr := ts.request(http.MethodPost, "/v1/prefetch/record", body)
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

func TestPrefetchHandler_RecordAccess_InvalidJSON(t *testing.T) {
	ts := newTestPrefetchServer(t)
	rr := ts.request(http.MethodPost, "/v1/prefetch/record", "not json")
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestPrefetchHandler_RecordAccess_MissingEntity(t *testing.T) {
	ts := newTestPrefetchServer(t)
	body := `{"entity_key":"","features":["clicks"]}`
	rr := ts.request(http.MethodPost, "/v1/prefetch/record", body)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestPrefetchHandler_Predict(t *testing.T) {
	ts := newTestPrefetchServer(t)
	rr := ts.request(http.MethodGet, "/v1/prefetch/predict/user123", "")
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

func TestPrefetchHandler_GetPlan(t *testing.T) {
	ts := newTestPrefetchServer(t)
	rr := ts.request(http.MethodGet, "/v1/prefetch/plan/user123", "")
	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}
}
