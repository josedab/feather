package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/feather-store/feather/internal/aggregation"
	"github.com/feather-store/feather/internal/domain"
	"github.com/feather-store/feather/internal/storage"
)

// testServer wraps an HTTPServer for testing.
type testServer struct {
	*HTTPServer
	handler http.Handler // Full handler chain with middleware
	t       *testing.T
}

// newTestServer creates a new test server with in-memory storage.
func newTestServer(t *testing.T) *testServer {
	t.Helper()

	schema := storage.NewRegistry()
	store, err := storage.NewStore(storage.StoreOptions{
		HotMaxSize:   1 << 20, // 1MB
		WarmInMemory: true,
	}, schema)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	agg := aggregation.NewEngine()
	healthChecker := NewHealthChecker(store, agg, schema)

	srv := NewHTTPServer(store, agg, schema, nil, HTTPServerConfig{
		Port:          0,
		ReadTimeout:   5 * time.Second,
		WriteTimeout:  10 * time.Second,
		HealthChecker: healthChecker,
	})

	t.Cleanup(func() {
		store.Close()
	})

	return &testServer{
		HTTPServer: srv,
		handler:    srv.server.Handler, // Use full handler with middleware
		t:          t,
	}
}

// request makes a test HTTP request and returns the response.
func (ts *testServer) request(method, path string, body string) *httptest.ResponseRecorder {
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
	ts.handler.ServeHTTP(rr, req)
	return rr
}

// get makes a GET request.
func (ts *testServer) get(path string) *httptest.ResponseRecorder {
	return ts.request(http.MethodGet, path, "")
}

// post makes a POST request with JSON body.
func (ts *testServer) post(path string, body interface{}) *httptest.ResponseRecorder {
	ts.t.Helper()

	jsonBody, err := json.Marshal(body)
	if err != nil {
		ts.t.Fatalf("failed to marshal body: %v", err)
	}
	return ts.request(http.MethodPost, path, string(jsonBody))
}

// assertStatus checks the response status code.
func assertStatus(t *testing.T, rr *httptest.ResponseRecorder, want int) {
	t.Helper()
	if rr.Code != want {
		t.Errorf("status = %d, want %d; body: %s", rr.Code, want, rr.Body.String())
	}
}

// assertJSON checks that the response is valid JSON and optionally checks fields.
func assertJSON(t *testing.T, rr *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()

	var result map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("response is not valid JSON: %v; body: %s", err, rr.Body.String())
	}
	return result
}

// assertContentType checks the Content-Type header.
func assertContentType(t *testing.T, rr *httptest.ResponseRecorder, want string) {
	t.Helper()
	got := rr.Header().Get("Content-Type")
	if !strings.HasPrefix(got, want) {
		t.Errorf("Content-Type = %q, want prefix %q", got, want)
	}
}

// seedFeatures adds test features to the store.
func (ts *testServer) seedFeatures(entityID string, features map[string]interface{}) {
	ts.t.Helper()

	featureValues := make(map[string]*domain.FeatureValue)
	for name, value := range features {
		featureValues[name] = &domain.FeatureValue{
			Value:     value,
			Timestamp: time.Now().UnixNano(),
			Version:   1,
		}
	}

	if err := ts.store.Put(entityID, featureValues); err != nil {
		ts.t.Fatalf("failed to seed features: %v", err)
	}
}

// seedFeatureGroup registers a feature group in the schema.
func (ts *testServer) seedFeatureGroup(name string, entityType string, features []string) {
	ts.t.Helper()

	group := &domain.FeatureGroup{
		Name:       name,
		EntityType: entityType,
		Features:   make([]domain.FeatureSpec, len(features)),
	}

	for i, f := range features {
		group.Features[i] = domain.FeatureSpec{
			Name:     f,
			DataType: domain.DataTypeFloat64,
		}
	}

	if err := ts.schema.RegisterGroup(group); err != nil {
		ts.t.Fatalf("failed to register group: %v", err)
	}
}
