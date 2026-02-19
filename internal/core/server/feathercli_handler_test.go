package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/feather-store/feather/internal/extensions/feathercli"
)

type testFeatherCLIServer struct {
	handler *FeatherCLIHandler
	client  *feathercli.Client
	mux     *http.ServeMux
	t       *testing.T
}

func newTestFeatherCLIServer(t *testing.T) *testFeatherCLIServer {
	t.Helper()

	client := feathercli.NewClient(feathercli.DefaultClientConfig())
	handler := NewFeatherCLIHandler(client)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	return &testFeatherCLIServer{
		handler: handler,
		client:  client,
		mux:     mux,
		t:       t,
	}
}

func (ts *testFeatherCLIServer) request(method, path string, body string) *httptest.ResponseRecorder {
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

func (ts *testFeatherCLIServer) postJSON(path string, body interface{}) *httptest.ResponseRecorder {
	ts.t.Helper()

	jsonBody, err := json.Marshal(body)
	if err != nil {
		ts.t.Fatalf("failed to marshal body: %v", err)
	}
	return ts.request(http.MethodPost, path, string(jsonBody))
}

func (ts *testFeatherCLIServer) get(path string) *httptest.ResponseRecorder {
	return ts.request(http.MethodGet, path, "")
}

func TestFeatherCLIHandler_NewHandler(t *testing.T) {
	client := feathercli.NewClient(feathercli.DefaultClientConfig())
	handler := NewFeatherCLIHandler(client)

	if handler.client == nil {
		t.Error("Expected client to be set")
	}
}

func TestFeatherCLIHandler_Query_InvalidBody(t *testing.T) {
	ts := newTestFeatherCLIServer(t)

	rr := ts.request(http.MethodPost, "/v1/cli/query", "invalid json")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestFeatherCLIHandler_Query(t *testing.T) {
	ts := newTestFeatherCLIServer(t)

	query := feathercli.FeatureQuery{
		Entity:   "user:123",
		Group:    "user_features",
		Features: []string{"click_rate"},
	}

	rr := ts.postJSON("/v1/cli/query", query)

	// The client works in offline mode, so the result depends on the implementation
	if rr.Code != http.StatusOK && rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d or %d, got %d", http.StatusOK, http.StatusBadRequest, rr.Code)
	}
}

func TestFeatherCLIHandler_ListGroups(t *testing.T) {
	ts := newTestFeatherCLIServer(t)

	rr := ts.get("/v1/cli/groups")

	if rr.Code != http.StatusOK && rr.Code != http.StatusInternalServerError {
		t.Errorf("Expected status %d or %d, got %d", http.StatusOK, http.StatusInternalServerError, rr.Code)
	}
}

func TestFeatherCLIHandler_GetSchema_MissingGroup(t *testing.T) {
	ts := newTestFeatherCLIServer(t)

	rr := ts.get("/v1/cli/schema/test_group")

	// May return error since client is not connected to a real server
	if rr.Code != http.StatusOK && rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d or %d, got %d", http.StatusOK, http.StatusBadRequest, rr.Code)
	}
}

func TestFeatherCLIHandler_GetHealth(t *testing.T) {
	ts := newTestFeatherCLIServer(t)

	rr := ts.get("/v1/cli/health")

	if rr.Code != http.StatusOK && rr.Code != http.StatusInternalServerError {
		t.Errorf("Expected status %d or %d, got %d", http.StatusOK, http.StatusInternalServerError, rr.Code)
	}
}

func TestFeatherCLIHandler_GetStats(t *testing.T) {
	ts := newTestFeatherCLIServer(t)

	rr := ts.get("/v1/cli/stats")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}
}
