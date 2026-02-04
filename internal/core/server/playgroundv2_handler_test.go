package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/feather-store/feather/internal/extensions/playgroundv2"
)

type testPlaygroundV2Server struct {
	handler *PlaygroundV2Handler
	env     *playgroundv2.Environment
	mux     *http.ServeMux
	t       *testing.T
}

func newTestPlaygroundV2Server(t *testing.T) *testPlaygroundV2Server {
	t.Helper()
	env := playgroundv2.NewEnvironment(playgroundv2.DefaultConfig(), nil)
	handler := NewPlaygroundV2Handler(env)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	return &testPlaygroundV2Server{handler: handler, env: env, mux: mux, t: t}
}

func (ts *testPlaygroundV2Server) request(method, path string, body string) *httptest.ResponseRecorder {
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

func TestPlaygroundV2Handler_GetStats(t *testing.T) {
	ts := newTestPlaygroundV2Server(t)
	rr := ts.request(http.MethodGet, "/v1/playground/v2/stats", "")
	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}
	var result map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}
}

func TestPlaygroundV2Handler_BrowseSchemas(t *testing.T) {
	ts := newTestPlaygroundV2Server(t)
	rr := ts.request(http.MethodGet, "/v1/playground/schemas", "")
	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}
	var result map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}
	if _, ok := result["schemas"]; !ok {
		t.Error("Expected schemas key in response")
	}
}

func TestPlaygroundV2Handler_ExecuteQuery(t *testing.T) {
	ts := newTestPlaygroundV2Server(t)
	body := `{"text":"SELECT * FROM features","entity_filters":["user:123"]}`
	rr := ts.request(http.MethodPost, "/v1/playground/query", body)
	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d; body: %s", http.StatusOK, rr.Code, rr.Body.String())
	}
}

func TestPlaygroundV2Handler_ExecuteQuery_InvalidJSON(t *testing.T) {
	ts := newTestPlaygroundV2Server(t)
	rr := ts.request(http.MethodPost, "/v1/playground/query", "not json")
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestPlaygroundV2Handler_StartSimulation(t *testing.T) {
	ts := newTestPlaygroundV2Server(t)
	body := `{"features":["clicks","views"],"duration_seconds":60,"update_frequency_ms":100}`
	rr := ts.request(http.MethodPost, "/v1/playground/simulate", body)
	if rr.Code != http.StatusCreated {
		t.Errorf("Expected status %d, got %d; body: %s", http.StatusCreated, rr.Code, rr.Body.String())
	}
}

func TestPlaygroundV2Handler_StartSimulation_InvalidJSON(t *testing.T) {
	ts := newTestPlaygroundV2Server(t)
	rr := ts.request(http.MethodPost, "/v1/playground/simulate", "not json")
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestPlaygroundV2Handler_StopSimulation_NotFound(t *testing.T) {
	ts := newTestPlaygroundV2Server(t)
	req := httptest.NewRequest(http.MethodDelete, "/v1/playground/simulate/nonexistent", nil)
	rr := httptest.NewRecorder()
	ts.mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, rr.Code)
	}
}

func TestPlaygroundV2Handler_PreviewRegistration_InvalidJSON(t *testing.T) {
	ts := newTestPlaygroundV2Server(t)
	rr := ts.request(http.MethodPost, "/v1/playground/deploy/preview", "not json")
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}
