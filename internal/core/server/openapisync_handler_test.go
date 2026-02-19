package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/feather-store/feather/internal/extensions/openapisync"
)

type testOpenAPISyncServer struct {
	handler   *OpenAPISyncHandler
	generator *openapisync.Generator
	mux       *http.ServeMux
	t         *testing.T
}

func newTestOpenAPISyncServer(t *testing.T) *testOpenAPISyncServer {
	t.Helper()

	generator := openapisync.NewGenerator(openapisync.DefaultGeneratorConfig())
	handler := NewOpenAPISyncHandler(generator)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	return &testOpenAPISyncServer{
		handler:   handler,
		generator: generator,
		mux:       mux,
		t:         t,
	}
}

func (ts *testOpenAPISyncServer) request(method, path string, body string) *httptest.ResponseRecorder {
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

func (ts *testOpenAPISyncServer) postJSON(path string, body interface{}) *httptest.ResponseRecorder {
	ts.t.Helper()

	jsonBody, err := json.Marshal(body)
	if err != nil {
		ts.t.Fatalf("failed to marshal body: %v", err)
	}
	return ts.request(http.MethodPost, path, string(jsonBody))
}

func (ts *testOpenAPISyncServer) get(path string) *httptest.ResponseRecorder {
	return ts.request(http.MethodGet, path, "")
}

func TestOpenAPISyncHandler_NewHandler(t *testing.T) {
	generator := openapisync.NewGenerator(openapisync.DefaultGeneratorConfig())
	handler := NewOpenAPISyncHandler(generator)

	if handler.generator == nil {
		t.Error("Expected generator to be set")
	}
}

func TestOpenAPISyncHandler_GetSpec(t *testing.T) {
	ts := newTestOpenAPISyncServer(t)

	rr := ts.get("/v1/openapi/spec")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}
}

func TestOpenAPISyncHandler_ListRoutes(t *testing.T) {
	ts := newTestOpenAPISyncServer(t)

	rr := ts.get("/v1/openapi/routes")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if result["routes"] == nil {
		t.Error("Expected routes array in response")
	}
}

func TestOpenAPISyncHandler_AddRoute(t *testing.T) {
	ts := newTestOpenAPISyncServer(t)

	route := openapisync.RouteInfo{
		Method:  "GET",
		Path:    "/v1/test",
		Summary: "Test endpoint",
	}

	rr := ts.postJSON("/v1/openapi/routes", route)

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

func TestOpenAPISyncHandler_AddRoute_InvalidBody(t *testing.T) {
	ts := newTestOpenAPISyncServer(t)

	rr := ts.request(http.MethodPost, "/v1/openapi/routes", "invalid json")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestOpenAPISyncHandler_GetStats(t *testing.T) {
	ts := newTestOpenAPISyncServer(t)

	rr := ts.get("/v1/openapi/stats")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}
}
