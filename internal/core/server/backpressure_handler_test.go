package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/feather-store/feather/internal/extensions/backpressure"
)

type testBackpressureServer struct {
	handler *BackpressureHandler
	monitor *backpressure.Monitor
	mux     *http.ServeMux
	t       *testing.T
}

func newTestBackpressureServer(t *testing.T) *testBackpressureServer {
	t.Helper()
	monitor := backpressure.NewMonitor(backpressure.DefaultMonitorConfig())
	handler := NewBackpressureHandler(monitor)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	return &testBackpressureServer{handler: handler, monitor: monitor, mux: mux, t: t}
}

func (ts *testBackpressureServer) request(method, path string, body string) *httptest.ResponseRecorder {
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

func TestBackpressureHandler_GetStats(t *testing.T) {
	ts := newTestBackpressureServer(t)
	rr := ts.request(http.MethodGet, "/v1/backpressure/stats", "")
	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}
	var result map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}
}

func TestBackpressureHandler_RecordQueueDepth(t *testing.T) {
	ts := newTestBackpressureServer(t)
	body := `{"depth":42.5}`
	rr := ts.request(http.MethodPost, "/v1/backpressure/queue", body)
	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d; body: %s", http.StatusOK, rr.Code, rr.Body.String())
	}
	var result map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}
	if result["success"] != true {
		t.Error("Expected success to be true")
	}
}

func TestBackpressureHandler_RecordQueueDepth_InvalidJSON(t *testing.T) {
	ts := newTestBackpressureServer(t)
	rr := ts.request(http.MethodPost, "/v1/backpressure/queue", "not json")
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestBackpressureHandler_GetLevel(t *testing.T) {
	ts := newTestBackpressureServer(t)
	rr := ts.request(http.MethodGet, "/v1/backpressure/level", "")
	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}
	var result map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}
}
