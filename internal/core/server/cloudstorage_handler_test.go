package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/feather-store/feather/internal/extensions/cloudstorage"
)

type testCloudStorageServer struct {
	handler *CloudStorageHandler
	store   *cloudstorage.ObjectStore
	mux     *http.ServeMux
	t       *testing.T
}

func newTestCloudStorageServer(t *testing.T) *testCloudStorageServer {
	t.Helper()

	store := cloudstorage.NewObjectStore(cloudstorage.DefaultStoreConfig())
	handler := NewCloudStorageHandler(store)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	return &testCloudStorageServer{
		handler: handler,
		store:   store,
		mux:     mux,
		t:       t,
	}
}

func (ts *testCloudStorageServer) request(method, path string, body string) *httptest.ResponseRecorder {
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

func (ts *testCloudStorageServer) postJSON(path string, body interface{}) *httptest.ResponseRecorder {
	ts.t.Helper()

	jsonBody, err := json.Marshal(body)
	if err != nil {
		ts.t.Fatalf("failed to marshal body: %v", err)
	}
	return ts.request(http.MethodPost, path, string(jsonBody))
}

func (ts *testCloudStorageServer) get(path string) *httptest.ResponseRecorder {
	return ts.request(http.MethodGet, path, "")
}

func TestCloudStorageHandler_NewHandler(t *testing.T) {
	store := cloudstorage.NewObjectStore(cloudstorage.DefaultStoreConfig())
	handler := NewCloudStorageHandler(store)

	if handler.store == nil {
		t.Error("Expected store to be set")
	}
}

func TestCloudStorageHandler_PutObject(t *testing.T) {
	ts := newTestCloudStorageServer(t)

	req := httptest.NewRequest(http.MethodPut, "/v1/storage/objects/test-key", strings.NewReader("hello world"))
	req.Header.Set("Content-Type", "text/plain")

	rr := httptest.NewRecorder()
	ts.mux.ServeHTTP(rr, req)

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

func TestCloudStorageHandler_GetObject(t *testing.T) {
	ts := newTestCloudStorageServer(t)

	ts.store.Put("test-key", []byte("hello world"), "text/plain", nil)

	rr := ts.get("/v1/storage/objects/test-key")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}

	if rr.Body.String() != "hello world" {
		t.Errorf("Expected body 'hello world', got '%s'", rr.Body.String())
	}
}

func TestCloudStorageHandler_GetObject_NotFound(t *testing.T) {
	ts := newTestCloudStorageServer(t)

	rr := ts.get("/v1/storage/objects/nonexistent")

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, rr.Code)
	}
}

func TestCloudStorageHandler_DeleteObject(t *testing.T) {
	ts := newTestCloudStorageServer(t)

	ts.store.Put("test-key", []byte("hello world"), "text/plain", nil)

	rr := ts.request(http.MethodDelete, "/v1/storage/objects/test-key", "")

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

func TestCloudStorageHandler_DeleteObject_NotFound(t *testing.T) {
	ts := newTestCloudStorageServer(t)

	rr := ts.request(http.MethodDelete, "/v1/storage/objects/nonexistent", "")

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, rr.Code)
	}
}

func TestCloudStorageHandler_ListObjects(t *testing.T) {
	ts := newTestCloudStorageServer(t)

	rr := ts.get("/v1/storage/objects?prefix=test&limit=10")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if _, ok := result["objects"]; !ok {
		t.Error("Expected objects key in response")
	}
}

func TestCloudStorageHandler_HeadObject_Exists(t *testing.T) {
	ts := newTestCloudStorageServer(t)

	ts.store.Put("test-key", []byte("hello"), "text/plain", nil)

	req := httptest.NewRequest(http.MethodHead, "/v1/storage/objects/test-key", nil)
	rr := httptest.NewRecorder()
	ts.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}
}

func TestCloudStorageHandler_HeadObject_NotFound(t *testing.T) {
	ts := newTestCloudStorageServer(t)

	req := httptest.NewRequest(http.MethodHead, "/v1/storage/objects/nonexistent", nil)
	rr := httptest.NewRecorder()
	ts.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, rr.Code)
	}
}

func TestCloudStorageHandler_CopyObject(t *testing.T) {
	ts := newTestCloudStorageServer(t)

	ts.store.Put("src-key", []byte("hello"), "text/plain", nil)

	body := map[string]interface{}{
		"src": "src-key",
		"dst": "dst-key",
	}

	rr := ts.postJSON("/v1/storage/objects/copy", body)

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

func TestCloudStorageHandler_CopyObject_SrcNotFound(t *testing.T) {
	ts := newTestCloudStorageServer(t)

	body := map[string]interface{}{
		"src": "nonexistent",
		"dst": "dst-key",
	}

	rr := ts.postJSON("/v1/storage/objects/copy", body)

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, rr.Code)
	}
}

func TestCloudStorageHandler_CopyObject_MissingKeys(t *testing.T) {
	ts := newTestCloudStorageServer(t)

	body := map[string]interface{}{}

	rr := ts.postJSON("/v1/storage/objects/copy", body)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestCloudStorageHandler_CopyObject_InvalidBody(t *testing.T) {
	ts := newTestCloudStorageServer(t)

	rr := ts.request(http.MethodPost, "/v1/storage/objects/copy", "invalid json")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestCloudStorageHandler_GetBucketStats(t *testing.T) {
	ts := newTestCloudStorageServer(t)

	rr := ts.get("/v1/storage/bucket/stats")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}
}

func TestCloudStorageHandler_GetStats(t *testing.T) {
	ts := newTestCloudStorageServer(t)

	rr := ts.get("/v1/storage/stats")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}
}
