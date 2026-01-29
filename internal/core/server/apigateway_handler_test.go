package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/feather-store/feather/internal/extensions/apigateway"
)

type testAPIGatewayServer struct {
	handler *APIGatewayHandler
	gateway *apigateway.Gateway
	mux     *http.ServeMux
	t       *testing.T
}

func newTestAPIGatewayServer(t *testing.T) *testAPIGatewayServer {
	t.Helper()

	gateway := apigateway.NewGateway(apigateway.DefaultGatewayConfig())
	handler := NewAPIGatewayHandler(gateway)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	return &testAPIGatewayServer{
		handler: handler,
		gateway: gateway,
		mux:     mux,
		t:       t,
	}
}

func (ts *testAPIGatewayServer) request(method, path string, body string) *httptest.ResponseRecorder {
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

func (ts *testAPIGatewayServer) get(path string) *httptest.ResponseRecorder {
	return ts.request(http.MethodGet, path, "")
}

func TestAPIGatewayHandler_NewHandler(t *testing.T) {
	gateway := apigateway.NewGateway(apigateway.DefaultGatewayConfig())
	handler := NewAPIGatewayHandler(gateway)

	if handler.gateway == nil {
		t.Error("Expected gateway to be set")
	}
}

func TestAPIGatewayHandler_ListBackends_Empty(t *testing.T) {
	ts := newTestAPIGatewayServer(t)

	rr := ts.get("/v1/gateway/backends")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if result["backends"] == nil {
		t.Error("Expected backends array in response")
	}
}

func TestAPIGatewayHandler_AddBackend(t *testing.T) {
	ts := newTestAPIGatewayServer(t)

	backend := apigateway.Backend{
		ID:     "backend-1",
		URL:    "http://localhost:9090",
		Weight: 1,
	}

	body, _ := json.Marshal(backend)
	rr := ts.request(http.MethodPost, "/v1/gateway/backends", string(body))

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

func TestAPIGatewayHandler_AddBackend_InvalidBody(t *testing.T) {
	ts := newTestAPIGatewayServer(t)

	rr := ts.request(http.MethodPost, "/v1/gateway/backends", "invalid json")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestAPIGatewayHandler_RemoveBackend(t *testing.T) {
	ts := newTestAPIGatewayServer(t)

	backend := apigateway.Backend{ID: "backend-1", URL: "http://localhost:9090", Weight: 1}
	ts.gateway.AddBackend(backend)

	rr := ts.request(http.MethodDelete, "/v1/gateway/backends/backend-1", "")

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

func TestAPIGatewayHandler_RemoveBackend_NotFound(t *testing.T) {
	ts := newTestAPIGatewayServer(t)

	rr := ts.request(http.MethodDelete, "/v1/gateway/backends/nonexistent", "")

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, rr.Code)
	}
}

func TestAPIGatewayHandler_Route_InvalidBody(t *testing.T) {
	ts := newTestAPIGatewayServer(t)

	rr := ts.request(http.MethodPost, "/v1/gateway/route", "invalid json")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestAPIGatewayHandler_GetStats(t *testing.T) {
	ts := newTestAPIGatewayServer(t)

	rr := ts.get("/v1/gateway/stats")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}
}

func TestAPIGatewayHandler_GetBackendStats(t *testing.T) {
	ts := newTestAPIGatewayServer(t)

	rr := ts.get("/v1/gateway/backends/stats")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}
}
