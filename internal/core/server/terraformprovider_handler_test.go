package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/feather-store/feather/internal/extensions/terraformprovider"
)

type testTerraformProviderServer struct {
	handler  *TerraformProviderHandler
	provider *terraformprovider.Provider
	mux      *http.ServeMux
	t        *testing.T
}

func newTestTerraformProviderServer(t *testing.T) *testTerraformProviderServer {
	t.Helper()

	provider := terraformprovider.NewProvider(terraformprovider.DefaultProviderConfig())
	handler := NewTerraformProviderHandler(provider)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	return &testTerraformProviderServer{
		handler:  handler,
		provider: provider,
		mux:      mux,
		t:        t,
	}
}

func (ts *testTerraformProviderServer) request(method, path string, body string) *httptest.ResponseRecorder {
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

func (ts *testTerraformProviderServer) postJSON(path string, body interface{}) *httptest.ResponseRecorder {
	ts.t.Helper()

	jsonBody, err := json.Marshal(body)
	if err != nil {
		ts.t.Fatalf("failed to marshal body: %v", err)
	}
	return ts.request(http.MethodPost, path, string(jsonBody))
}

func (ts *testTerraformProviderServer) get(path string) *httptest.ResponseRecorder {
	return ts.request(http.MethodGet, path, "")
}

func TestTerraformProviderHandler_NewHandler(t *testing.T) {
	provider := terraformprovider.NewProvider(terraformprovider.DefaultProviderConfig())
	handler := NewTerraformProviderHandler(provider)

	if handler.provider == nil {
		t.Error("Expected provider to be set")
	}
}

func TestTerraformProviderHandler_ListResources_Empty(t *testing.T) {
	ts := newTestTerraformProviderServer(t)

	rr := ts.get("/v1/terraform/resources")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if _, ok := result["resources"]; !ok {
		t.Error("Expected resources key in response")
	}
}

func TestTerraformProviderHandler_CreateResource(t *testing.T) {
	ts := newTestTerraformProviderServer(t)

	body := map[string]interface{}{
		"type":       string(terraformprovider.ResourceFeatureGroup),
		"id":         "res-1",
		"attributes": map[string]interface{}{"name": "test"},
	}

	rr := ts.postJSON("/v1/terraform/resources", body)

	if rr.Code != http.StatusCreated {
		t.Errorf("Expected status %d, got %d; body: %s", http.StatusCreated, rr.Code, rr.Body.String())
	}
}

func TestTerraformProviderHandler_CreateResource_Duplicate(t *testing.T) {
	ts := newTestTerraformProviderServer(t)

	ts.provider.CreateResource(terraformprovider.ResourceFeatureGroup, "res-1", map[string]interface{}{"name": "test"})

	body := map[string]interface{}{
		"type":       string(terraformprovider.ResourceFeatureGroup),
		"id":         "res-1",
		"attributes": map[string]interface{}{"name": "test"},
	}

	rr := ts.postJSON("/v1/terraform/resources", body)

	if rr.Code != http.StatusConflict {
		t.Errorf("Expected status %d, got %d", http.StatusConflict, rr.Code)
	}
}

func TestTerraformProviderHandler_CreateResource_InvalidBody(t *testing.T) {
	ts := newTestTerraformProviderServer(t)

	rr := ts.request(http.MethodPost, "/v1/terraform/resources", "invalid json")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestTerraformProviderHandler_ReadResource(t *testing.T) {
	ts := newTestTerraformProviderServer(t)

	ts.provider.CreateResource(terraformprovider.ResourceFeatureGroup, "res-1", map[string]interface{}{"name": "test"})

	rr := ts.get("/v1/terraform/resources/res-1")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}
}

func TestTerraformProviderHandler_ReadResource_NotFound(t *testing.T) {
	ts := newTestTerraformProviderServer(t)

	rr := ts.get("/v1/terraform/resources/nonexistent")

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, rr.Code)
	}
}

func TestTerraformProviderHandler_UpdateResource(t *testing.T) {
	ts := newTestTerraformProviderServer(t)

	ts.provider.CreateResource(terraformprovider.ResourceFeatureGroup, "res-1", map[string]interface{}{"name": "test"})

	body := map[string]interface{}{
		"attributes": map[string]interface{}{"name": "updated"},
	}
	b, _ := json.Marshal(body)
	rr := ts.request(http.MethodPut, "/v1/terraform/resources/res-1", string(b))

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d; body: %s", http.StatusOK, rr.Code, rr.Body.String())
	}
}

func TestTerraformProviderHandler_UpdateResource_NotFound(t *testing.T) {
	ts := newTestTerraformProviderServer(t)

	body := map[string]interface{}{
		"attributes": map[string]interface{}{"name": "updated"},
	}
	b, _ := json.Marshal(body)
	rr := ts.request(http.MethodPut, "/v1/terraform/resources/nonexistent", string(b))

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, rr.Code)
	}
}

func TestTerraformProviderHandler_DeleteResource(t *testing.T) {
	ts := newTestTerraformProviderServer(t)

	ts.provider.CreateResource(terraformprovider.ResourceFeatureGroup, "res-1", map[string]interface{}{"name": "test"})

	rr := ts.request(http.MethodDelete, "/v1/terraform/resources/res-1", "")

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

func TestTerraformProviderHandler_DeleteResource_NotFound(t *testing.T) {
	ts := newTestTerraformProviderServer(t)

	rr := ts.request(http.MethodDelete, "/v1/terraform/resources/nonexistent", "")

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, rr.Code)
	}
}

func TestTerraformProviderHandler_Plan(t *testing.T) {
	ts := newTestTerraformProviderServer(t)

	body := map[string]interface{}{
		"desired": []map[string]interface{}{
			{
				"id":         "res-1",
				"type":       string(terraformprovider.ResourceFeatureGroup),
				"attributes": map[string]interface{}{"name": "test"},
			},
		},
	}

	rr := ts.postJSON("/v1/terraform/plan", body)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d; body: %s", http.StatusOK, rr.Code, rr.Body.String())
	}
}

func TestTerraformProviderHandler_Plan_InvalidBody(t *testing.T) {
	ts := newTestTerraformProviderServer(t)

	rr := ts.request(http.MethodPost, "/v1/terraform/plan", "invalid json")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestTerraformProviderHandler_Apply(t *testing.T) {
	ts := newTestTerraformProviderServer(t)

	body := map[string]interface{}{
		"plan": []map[string]interface{}{},
	}

	rr := ts.postJSON("/v1/terraform/apply", body)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}
}

func TestTerraformProviderHandler_Apply_InvalidBody(t *testing.T) {
	ts := newTestTerraformProviderServer(t)

	rr := ts.request(http.MethodPost, "/v1/terraform/apply", "invalid json")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestTerraformProviderHandler_Import(t *testing.T) {
	ts := newTestTerraformProviderServer(t)

	body := map[string]interface{}{
		"type": string(terraformprovider.ResourceFeatureGroup),
		"id":   "res-import-1",
	}

	rr := ts.postJSON("/v1/terraform/import", body)

	if rr.Code != http.StatusCreated {
		t.Errorf("Expected status %d, got %d; body: %s", http.StatusCreated, rr.Code, rr.Body.String())
	}
}

func TestTerraformProviderHandler_Import_InvalidBody(t *testing.T) {
	ts := newTestTerraformProviderServer(t)

	rr := ts.request(http.MethodPost, "/v1/terraform/import", "invalid json")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestTerraformProviderHandler_GetStats(t *testing.T) {
	ts := newTestTerraformProviderServer(t)

	rr := ts.get("/v1/terraform/stats")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}
}
