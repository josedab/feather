package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/feather-store/feather/internal/extensions/federateddiscovery"
)

type testFederatedDiscoveryServer struct {
	handler *FederatedDiscoveryHandler
	catalog *federateddiscovery.Catalog
	mux     *http.ServeMux
	t       *testing.T
}

func newTestFederatedDiscoveryServer(t *testing.T) *testFederatedDiscoveryServer {
	t.Helper()
	catalog := federateddiscovery.NewCatalog(federateddiscovery.DefaultCatalogConfig())
	handler := NewFederatedDiscoveryHandler(catalog)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	return &testFederatedDiscoveryServer{handler: handler, catalog: catalog, mux: mux, t: t}
}

func (ts *testFederatedDiscoveryServer) request(method, path string, body string) *httptest.ResponseRecorder {
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

func TestFederatedDiscoveryHandler_GetStats(t *testing.T) {
	ts := newTestFederatedDiscoveryServer(t)
	rr := ts.request(http.MethodGet, "/v1/federation/stats", "")
	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}
	var result map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}
}

func TestFederatedDiscoveryHandler_Publish(t *testing.T) {
	ts := newTestFederatedDiscoveryServer(t)
	body := `{"id":"feat-click","name":"click_rate","owner":"team-ml","description":"User click-through rate","tags":["ml","engagement"]}`
	rr := ts.request(http.MethodPost, "/v1/federation/catalog", body)
	if rr.Code != http.StatusCreated {
		t.Errorf("Expected status %d, got %d; body: %s", http.StatusCreated, rr.Code, rr.Body.String())
	}
	var result map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}
	if result["success"] != true {
		t.Error("Expected success to be true")
	}
}

func TestFederatedDiscoveryHandler_Publish_InvalidJSON(t *testing.T) {
	ts := newTestFederatedDiscoveryServer(t)
	rr := ts.request(http.MethodPost, "/v1/federation/catalog", "not json")
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestFederatedDiscoveryHandler_Get_NotFound(t *testing.T) {
	ts := newTestFederatedDiscoveryServer(t)
	rr := ts.request(http.MethodGet, "/v1/federation/catalog/nonexistent", "")
	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, rr.Code)
	}
}
