// Package testutil provides shared test helpers for Feather test suites.
//
// This package consolidates common patterns found across handler tests,
// integration tests, and storage tests to reduce duplication.
//
// Example usage:
//
//	func TestMyHandler(t *testing.T) {
//	    store := testutil.NewTestStore(t)
//	    ts := testutil.NewHandlerTestServer(t, mux)
//	    rr := ts.Get("/v1/features")
//	    testutil.AssertStatus(t, rr, http.StatusOK)
//	    result := testutil.AssertJSON(t, rr)
//	}
package testutil

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/feather-store/feather/internal/core/domain"
	"github.com/feather-store/feather/internal/core/storage"
)

// NewTestStore creates an in-memory Store suitable for unit tests.
// The store is automatically closed when the test finishes.
func NewTestStore(t *testing.T) *storage.Store {
	t.Helper()
	store, err := storage.NewStore(context.Background(), storage.StoreOptions{
		HotMaxSize:   1 << 20, // 1MB
		WarmInMemory: true,
	}, storage.NewRegistry())
	if err != nil {
		t.Fatalf("testutil: failed to create store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

// NewTestStoreWithRegistry creates an in-memory Store with a custom registry.
func NewTestStoreWithRegistry(t *testing.T, registry *storage.Registry) *storage.Store {
	t.Helper()
	store, err := storage.NewStore(context.Background(), storage.StoreOptions{
		HotMaxSize:   1 << 20,
		WarmInMemory: true,
	}, registry)
	if err != nil {
		t.Fatalf("testutil: failed to create store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

// HandlerTestServer provides convenience methods for testing HTTP handlers.
type HandlerTestServer struct {
	Handler http.Handler
	T       *testing.T
}

// NewHandlerTestServer wraps an http.Handler for convenient test requests.
func NewHandlerTestServer(t *testing.T, handler http.Handler) *HandlerTestServer {
	t.Helper()
	return &HandlerTestServer{Handler: handler, T: t}
}

// Request makes a test HTTP request and returns the response recorder.
func (ts *HandlerTestServer) Request(method, path string, body string) *httptest.ResponseRecorder {
	ts.T.Helper()

	var bodyReader io.Reader
	if body != "" {
		bodyReader = strings.NewReader(body)
	}

	req := httptest.NewRequest(method, path, bodyReader)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}

	rr := httptest.NewRecorder()
	ts.Handler.ServeHTTP(rr, req)
	return rr
}

// Get makes a GET request.
func (ts *HandlerTestServer) Get(path string) *httptest.ResponseRecorder {
	return ts.Request(http.MethodGet, path, "")
}

// PostJSON makes a POST request with a JSON-encoded body.
func (ts *HandlerTestServer) PostJSON(path string, body interface{}) *httptest.ResponseRecorder {
	ts.T.Helper()
	jsonBody, err := json.Marshal(body)
	if err != nil {
		ts.T.Fatalf("testutil: failed to marshal body: %v", err)
	}
	return ts.Request(http.MethodPost, path, string(jsonBody))
}

// Delete makes a DELETE request.
func (ts *HandlerTestServer) Delete(path string) *httptest.ResponseRecorder {
	return ts.Request(http.MethodDelete, path, "")
}

// AssertStatus checks the response status code.
func AssertStatus(t *testing.T, rr *httptest.ResponseRecorder, want int) {
	t.Helper()
	if rr.Code != want {
		t.Errorf("status = %d, want %d; body: %s", rr.Code, want, rr.Body.String())
	}
}

// AssertJSON checks that the response is valid JSON and returns the parsed map.
func AssertJSON(t *testing.T, rr *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	var result map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("response is not valid JSON: %v; body: %s", err, rr.Body.String())
	}
	return result
}

// AssertContentType checks the Content-Type header.
func AssertContentType(t *testing.T, rr *httptest.ResponseRecorder, want string) {
	t.Helper()
	got := rr.Header().Get("Content-Type")
	if !strings.HasPrefix(got, want) {
		t.Errorf("Content-Type = %q, want prefix %q", got, want)
	}
}

// SeedFeatures stores features directly into the store for test setup.
func SeedFeatures(t *testing.T, store *storage.Store, entityID string, features map[string]interface{}) {
	t.Helper()
	featureValues := make(map[string]*domain.FeatureValue)
	for name, value := range features {
		featureValues[name] = &domain.FeatureValue{
			Value:   value,
			Version: 1,
		}
	}
	if err := store.Put(entityID, featureValues); err != nil {
		t.Fatalf("testutil: failed to seed features: %v", err)
	}
}
