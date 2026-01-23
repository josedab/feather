package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/feather-store/feather/internal/extensions/offlinestore"
)

type testOfflineStoreServer struct {
	handler *OfflineStoreHandler
	store   *offlinestore.Store
	mux     *http.ServeMux
	t       *testing.T
}

func newTestOfflineStoreServer(t *testing.T) *testOfflineStoreServer {
	t.Helper()
	store := offlinestore.NewStore(offlinestore.DefaultStoreConfig())
	handler := NewOfflineStoreHandler(store)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	return &testOfflineStoreServer{handler: handler, store: store, mux: mux, t: t}
}

func (ts *testOfflineStoreServer) request(method, path string, body string) *httptest.ResponseRecorder {
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

func TestOfflineStoreHandler_GetStats(t *testing.T) {
	ts := newTestOfflineStoreServer(t)
	rr := ts.request(http.MethodGet, "/v1/offline/stats", "")
	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}
	var result map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}
}

func TestOfflineStoreHandler_CreateDataset(t *testing.T) {
	ts := newTestOfflineStoreServer(t)
	body := `{"name":"training_data","entity_key":"user_id","features":["click_rate","view_count"]}`
	rr := ts.request(http.MethodPost, "/v1/offline/datasets", body)
	if rr.Code != http.StatusCreated {
		t.Errorf("Expected status %d, got %d; body: %s", http.StatusCreated, rr.Code, rr.Body.String())
	}
	var result map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}
}

func TestOfflineStoreHandler_CreateDataset_InvalidJSON(t *testing.T) {
	ts := newTestOfflineStoreServer(t)
	rr := ts.request(http.MethodPost, "/v1/offline/datasets", "not json")
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestOfflineStoreHandler_GetDataset_NotFound(t *testing.T) {
	ts := newTestOfflineStoreServer(t)
	rr := ts.request(http.MethodGet, "/v1/offline/datasets/nonexistent", "")
	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, rr.Code)
	}
}
