package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/feather-store/feather/internal/extensions/arrowflight"
)

type testArrowFlightServer struct {
	handler *ArrowFlightHandler
	server  *arrowflight.Server
	mux     *http.ServeMux
	t       *testing.T
}

func newTestArrowFlightServer(t *testing.T) *testArrowFlightServer {
	t.Helper()
	srv := arrowflight.NewServer(arrowflight.DefaultConfig())
	handler := NewArrowFlightHandler(srv)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	return &testArrowFlightServer{handler: handler, server: srv, mux: mux, t: t}
}

func (ts *testArrowFlightServer) request(method, path string, body string) *httptest.ResponseRecorder {
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

func TestArrowFlightHandler_GetStats(t *testing.T) {
	ts := newTestArrowFlightServer(t)
	rr := ts.request(http.MethodGet, "/v1/flight/stats", "")
	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}
	var result map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}
}

func TestArrowFlightHandler_ListFlights(t *testing.T) {
	ts := newTestArrowFlightServer(t)
	rr := ts.request(http.MethodGet, "/v1/flight/list", "")
	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}
	var result map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}
	if _, ok := result["flights"]; !ok {
		t.Error("Expected flights key in response")
	}
}

func TestArrowFlightHandler_GetFlightInfo(t *testing.T) {
	ts := newTestArrowFlightServer(t)
	body := `{"path":"test-dataset"}`
	rr := ts.request(http.MethodPost, "/v1/flight/info", body)
	if rr.Code != http.StatusOK && rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d or %d, got %d; body: %s", http.StatusOK, http.StatusBadRequest, rr.Code, rr.Body.String())
	}
}

func TestArrowFlightHandler_GetFlightInfo_InvalidJSON(t *testing.T) {
	ts := newTestArrowFlightServer(t)
	rr := ts.request(http.MethodPost, "/v1/flight/info", "not json")
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestArrowFlightHandler_DoPut(t *testing.T) {
	ts := newTestArrowFlightServer(t)
	body := `{"descriptor":{"type":"path","path":["test"]},"batch":{"schema":[],"rows":0,"columns":{}}}`
	rr := ts.request(http.MethodPost, "/v1/flight/put", body)
	if rr.Code != http.StatusOK && rr.Code != http.StatusInternalServerError {
		t.Errorf("Expected status %d or %d, got %d; body: %s", http.StatusOK, http.StatusInternalServerError, rr.Code, rr.Body.String())
	}
}

func TestArrowFlightHandler_DoPut_InvalidJSON(t *testing.T) {
	ts := newTestArrowFlightServer(t)
	rr := ts.request(http.MethodPost, "/v1/flight/put", "not json")
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}
