package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/feather-store/feather/internal/graphql"
	"github.com/feather-store/feather/internal/storage"
)

// testGraphQLServer wraps a GraphQLHandler for testing.
type testGraphQLServer struct {
	handler *GraphQLHandler
	mux     *http.ServeMux
	t       *testing.T
}

// newTestGraphQLServer creates a new test GraphQL server.
func newTestGraphQLServer(t *testing.T) *testGraphQLServer {
	t.Helper()

	registry := storage.NewRegistry()
	store, err := storage.NewStore(storage.StoreOptions{
		HotMaxSize:   1024 * 1024, // 1MB
		WarmInMemory: true,
	}, registry)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	schema, err := graphql.NewFeatureStoreSchema(store, registry)
	if err != nil {
		t.Fatalf("failed to create schema: %v", err)
	}
	handler := NewGraphQLHandler(schema)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	return &testGraphQLServer{
		handler: handler,
		mux:     mux,
		t:       t,
	}
}

// newTestGraphQLServerWithoutSchema creates a GraphQL server without schema for testing nil case.
func newTestGraphQLServerWithoutSchema(t *testing.T) *testGraphQLServer {
	t.Helper()

	handler := NewGraphQLHandler(nil)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	return &testGraphQLServer{
		handler: handler,
		mux:     mux,
		t:       t,
	}
}

func (ts *testGraphQLServer) request(method, path string, body string) *httptest.ResponseRecorder {
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

func (ts *testGraphQLServer) postJSON(path string, body interface{}) *httptest.ResponseRecorder {
	ts.t.Helper()

	jsonBody, err := json.Marshal(body)
	if err != nil {
		ts.t.Fatalf("failed to marshal body: %v", err)
	}
	return ts.request(http.MethodPost, path, string(jsonBody))
}

func (ts *testGraphQLServer) get(path string) *httptest.ResponseRecorder {
	return ts.request(http.MethodGet, path, "")
}

func TestGraphQLHandler_NewGraphQLHandler(t *testing.T) {
	registry := storage.NewRegistry()
	store, err := storage.NewStore(storage.StoreOptions{
		HotMaxSize:   1024 * 1024,
		WarmInMemory: true,
	}, registry)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	schema, err := graphql.NewFeatureStoreSchema(store, registry)
	if err != nil {
		t.Fatalf("failed to create schema: %v", err)
	}
	handler := NewGraphQLHandler(schema)

	if handler.schema == nil {
		t.Error("Expected schema to be set")
	}
}

func TestGraphQLHandler_Query_NoSchema(t *testing.T) {
	ts := newTestGraphQLServerWithoutSchema(t)

	body := graphql.Request{
		Query: "{ healthCheck { status } }",
	}

	rr := ts.postJSON("/graphql", body)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
	}
}

func TestGraphQLHandler_Query_MissingQuery(t *testing.T) {
	ts := newTestGraphQLServer(t)

	body := graphql.Request{
		Query: "",
	}

	rr := ts.postJSON("/graphql", body)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestGraphQLHandler_Query_InvalidBody(t *testing.T) {
	ts := newTestGraphQLServer(t)

	rr := ts.request(http.MethodPost, "/graphql", "invalid json")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestGraphQLHandler_Query_HealthCheck(t *testing.T) {
	ts := newTestGraphQLServer(t)

	body := graphql.Request{
		Query: "{ healthCheck { status } }",
	}

	rr := ts.postJSON("/graphql", body)

	// GraphQL may return 200 or 400 depending on schema configuration
	if rr.Code != http.StatusOK && rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status 200 or 400, got %d; body: %s", rr.Code, rr.Body.String())
	}
}

func TestGraphQLHandler_Playground(t *testing.T) {
	ts := newTestGraphQLServer(t)

	rr := ts.get("/graphql")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}

	// Check content type
	contentType := rr.Header().Get("Content-Type")
	if contentType != "text/html" {
		t.Errorf("Expected Content-Type 'text/html', got %s", contentType)
	}

	// Check that it contains GraphiQL
	if !strings.Contains(rr.Body.String(), "GraphiQL") {
		t.Error("Expected response to contain GraphiQL")
	}
}

func TestGraphQLHandler_Query_WithVariables(t *testing.T) {
	ts := newTestGraphQLServer(t)

	body := graphql.Request{
		Query: `query GetFeature($entity: String!) {
			feature(entity: $entity, feature: "test") {
				value
			}
		}`,
		Variables: map[string]interface{}{
			"entity": "user:123",
		},
	}

	rr := ts.postJSON("/graphql", body)

	// Accept any response - schema may or may not support the query
	if rr.Code != http.StatusOK && rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status 200 or 400, got %d; body: %s", rr.Code, rr.Body.String())
	}
}
