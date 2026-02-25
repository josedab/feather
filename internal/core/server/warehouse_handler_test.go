package server

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/feather-store/feather/internal/core/storage"
	"github.com/feather-store/feather/internal/integrations/warehouse"
)

// testWarehouseServer wraps a WarehouseHandler for testing.
type testWarehouseServer struct {
	handler *WarehouseHandler
	mux     *http.ServeMux
	t       *testing.T
}

// newTestWarehouseServer creates a new test warehouse server.
func newTestWarehouseServer(t *testing.T) *testWarehouseServer {
	t.Helper()

	handler := NewWarehouseHandler(WarehouseHandlerConfig{})
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	return &testWarehouseServer{
		handler: handler,
		mux:     mux,
		t:       t,
	}
}

func (ts *testWarehouseServer) request(method, path string, body string) *httptest.ResponseRecorder {
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

func (ts *testWarehouseServer) postJSON(path string, body interface{}) *httptest.ResponseRecorder {
	ts.t.Helper()

	jsonBody, err := json.Marshal(body)
	if err != nil {
		ts.t.Fatalf("failed to marshal body: %v", err)
	}
	return ts.request(http.MethodPost, path, string(jsonBody))
}

func (ts *testWarehouseServer) get(path string) *httptest.ResponseRecorder {
	return ts.request(http.MethodGet, path, "")
}

func (ts *testWarehouseServer) delete(path string) *httptest.ResponseRecorder {
	return ts.request(http.MethodDelete, path, "")
}

func TestWarehouseHandler_NewWarehouseHandler(t *testing.T) {
	handler := NewWarehouseHandler(WarehouseHandlerConfig{})

	if handler == nil {
		t.Error("Expected handler to be non-nil")
	}
}

func TestWarehouseHandler_ListConnectors(t *testing.T) {
	ts := newTestWarehouseServer(t)

	rr := ts.get("/v1/warehouse/connectors")

	// Without engine configured, returns 503
	if rr.Code != http.StatusOK && rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d or %d, got %d", http.StatusOK, http.StatusServiceUnavailable, rr.Code)
	}
}

func TestWarehouseHandler_CreateConnector_InvalidBody(t *testing.T) {
	ts := newTestWarehouseServer(t)

	rr := ts.request(http.MethodPost, "/v1/warehouse/connectors", "invalid json")

	// Without engine, may return 503; with engine but invalid body, returns 400
	if rr.Code != http.StatusBadRequest && rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d or %d, got %d", http.StatusBadRequest, http.StatusServiceUnavailable, rr.Code)
	}
}

func TestWarehouseHandler_CreateConnector_MissingType(t *testing.T) {
	ts := newTestWarehouseServer(t)

	body := map[string]interface{}{
		"id":     "test-connector",
		"config": map[string]interface{}{},
	}

	rr := ts.postJSON("/v1/warehouse/connectors", body)

	// Without engine configured, returns 503
	if rr.Code != http.StatusBadRequest && rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d or %d, got %d; body: %s", http.StatusBadRequest, http.StatusServiceUnavailable, rr.Code, rr.Body.String())
	}
}

func TestWarehouseHandler_CreateConnector_UnsupportedType(t *testing.T) {
	ts := newTestWarehouseServer(t)

	body := map[string]interface{}{
		"id":     "test-connector",
		"type":   "unsupported",
		"config": map[string]interface{}{},
	}

	rr := ts.postJSON("/v1/warehouse/connectors", body)

	// Without engine configured, returns 503
	if rr.Code != http.StatusBadRequest && rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d or %d, got %d; body: %s", http.StatusBadRequest, http.StatusServiceUnavailable, rr.Code, rr.Body.String())
	}
}

func TestWarehouseHandler_GetConnector_NotFound(t *testing.T) {
	ts := newTestWarehouseServer(t)

	rr := ts.get("/v1/warehouse/connectors/nonexistent")

	// Without engine configured, returns 503
	if rr.Code != http.StatusNotFound && rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d or %d, got %d", http.StatusNotFound, http.StatusServiceUnavailable, rr.Code)
	}
}

func TestWarehouseHandler_DeleteConnector_NotFound(t *testing.T) {
	ts := newTestWarehouseServer(t)

	rr := ts.delete("/v1/warehouse/connectors/nonexistent")

	// Without engine configured, returns 503
	if rr.Code != http.StatusNotFound && rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d or %d, got %d", http.StatusNotFound, http.StatusServiceUnavailable, rr.Code)
	}
}

func TestWarehouseHandler_ListJobs(t *testing.T) {
	ts := newTestWarehouseServer(t)

	rr := ts.get("/v1/warehouse/jobs")

	// Without engine configured, returns 503
	if rr.Code != http.StatusOK && rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d or %d, got %d", http.StatusOK, http.StatusServiceUnavailable, rr.Code)
	}
}

func TestWarehouseHandler_CreateJob_InvalidBody(t *testing.T) {
	ts := newTestWarehouseServer(t)

	rr := ts.request(http.MethodPost, "/v1/warehouse/jobs", "invalid json")

	// Without engine configured, returns 503
	if rr.Code != http.StatusBadRequest && rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d or %d, got %d", http.StatusBadRequest, http.StatusServiceUnavailable, rr.Code)
	}
}

func TestWarehouseHandler_GetJob_NotFound(t *testing.T) {
	ts := newTestWarehouseServer(t)

	rr := ts.get("/v1/warehouse/jobs/nonexistent")

	// Without engine configured, returns 503
	if rr.Code != http.StatusNotFound && rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d or %d, got %d", http.StatusNotFound, http.StatusServiceUnavailable, rr.Code)
	}
}

func TestWarehouseHandler_TriggerJob_NotFound(t *testing.T) {
	ts := newTestWarehouseServer(t)

	rr := ts.postJSON("/v1/warehouse/jobs/nonexistent/trigger", map[string]interface{}{})

	// Without engine configured, returns 503 or 404
	if rr.Code != http.StatusNotFound && rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d or %d, got %d", http.StatusNotFound, http.StatusServiceUnavailable, rr.Code)
	}
}

func TestWarehouseHandler_ListExecutions(t *testing.T) {
	ts := newTestWarehouseServer(t)

	rr := ts.get("/v1/warehouse/executions")

	// Without engine configured, may return 503 or 404
	if rr.Code != http.StatusOK && rr.Code != http.StatusServiceUnavailable && rr.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, %d or %d, got %d", http.StatusOK, http.StatusServiceUnavailable, http.StatusNotFound, rr.Code)
	}
}

func TestWarehouseHandler_GetExecution_NotFound(t *testing.T) {
	ts := newTestWarehouseServer(t)

	rr := ts.get("/v1/warehouse/executions/nonexistent")

	// Without engine configured, returns 503 or 404
	if rr.Code != http.StatusNotFound && rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d or %d, got %d", http.StatusNotFound, http.StatusServiceUnavailable, rr.Code)
	}
}

func TestWarehouseHandler_GetStats(t *testing.T) {
	ts := newTestWarehouseServer(t)

	rr := ts.get("/v1/warehouse/stats")

	// Without engine configured, returns 503
	if rr.Code != http.StatusOK && rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d or %d, got %d", http.StatusOK, http.StatusServiceUnavailable, rr.Code)
	}
}

// --- Tests with configured engine ---

func newConfiguredWarehouseServer(t *testing.T) *testWarehouseServer {
	t.Helper()

	schema := storage.NewRegistry()
	store, err := storage.NewStore(context.Background(), storage.StoreOptions{
		HotMaxSize:   1 << 20,
		WarmInMemory: true,
	}, schema)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	engine := warehouse.NewSyncEngine(warehouse.SyncConfig{
		BatchSize:      100,
		MaxConcurrency: 2,
	}, store, schema, nil)

	handler := NewWarehouseHandler(WarehouseHandlerConfig{
		Engine: engine,
		Store:  store,
		Schema: schema,
	})
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	return &testWarehouseServer{handler: handler, mux: mux, t: t}
}

func TestWarehouseHandler_RegisterConnector_Snowflake(t *testing.T) {
	ts := newConfiguredWarehouseServer(t)

	body := map[string]interface{}{
		"id":   "sf-1",
		"type": "snowflake",
		"config": map[string]interface{}{
			"account":   "test.snowflakecomputing.com",
			"user":      "feather_user",
			"password":  "test_pass",
			"database":  "FEATHER_DB",
			"schema":    "PUBLIC",
			"warehouse": "COMPUTE_WH",
			"role":      "FEATHER_ROLE",
		},
	}

	rr := ts.postJSON("/v1/warehouse/connectors", body)
	if rr.Code != http.StatusCreated && rr.Code != http.StatusOK {
		t.Errorf("Expected 201 or 200, got %d; body: %s", rr.Code, rr.Body.String())
	}
}

func TestWarehouseHandler_RegisterConnector_BigQuery(t *testing.T) {
	ts := newConfiguredWarehouseServer(t)

	body := map[string]interface{}{
		"id":   "bq-1",
		"type": "bigquery",
		"config": map[string]interface{}{
			"project_id":      "my-project",
			"dataset":         "feather_dataset",
			"credentials_json": `{"type":"service_account"}`,
		},
	}

	rr := ts.postJSON("/v1/warehouse/connectors", body)
	if rr.Code != http.StatusCreated && rr.Code != http.StatusOK {
		t.Errorf("Expected 201 or 200, got %d; body: %s", rr.Code, rr.Body.String())
	}
}

func TestWarehouseHandler_RegisterConnector_Unsupported(t *testing.T) {
	ts := newConfiguredWarehouseServer(t)

	body := map[string]interface{}{
		"id":     "bad-1",
		"type":   "unsupported_db",
		"config": map[string]interface{}{},
	}

	rr := ts.postJSON("/v1/warehouse/connectors", body)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %d; body: %s", rr.Code, rr.Body.String())
	}
}

func TestWarehouseHandler_GetConnector_WithEngine(t *testing.T) {
	ts := newConfiguredWarehouseServer(t)

	rr := ts.get("/v1/warehouse/connectors/nonexistent")
	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected 404, got %d", rr.Code)
	}
}

func TestWarehouseHandler_ListConnectors_WithEngine(t *testing.T) {
	ts := newConfiguredWarehouseServer(t)

	rr := ts.get("/v1/warehouse/connectors")
	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", rr.Code)
	}
}

func TestWarehouseHandler_RemoveConnector_WithEngine(t *testing.T) {
	ts := newConfiguredWarehouseServer(t)

	rr := ts.delete("/v1/warehouse/connectors/nonexistent")
	if rr.Code != http.StatusNotFound && rr.Code != http.StatusOK {
		t.Errorf("Expected 404 or 200, got %d", rr.Code)
	}
}

func TestWarehouseHandler_CreateJob_InvalidBody_WithEngine(t *testing.T) {
	ts := newConfiguredWarehouseServer(t)

	rr := ts.request(http.MethodPost, "/v1/warehouse/jobs", "not json")
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %d", rr.Code)
	}
}

func TestWarehouseHandler_CreateJob_WithEngine(t *testing.T) {
	ts := newConfiguredWarehouseServer(t)

	body := map[string]interface{}{
		"name":         "daily-sync",
		"connector_id": "sf-1",
		"query":        "SELECT * FROM features",
		"schedule":     "0 */6 * * *",
	}

	rr := ts.postJSON("/v1/warehouse/jobs", body)
	// May fail because connector doesn't exist yet
	if rr.Code == http.StatusCreated || rr.Code == http.StatusOK || rr.Code == http.StatusBadRequest || rr.Code == http.StatusNotFound {
		// All acceptable
	} else {
		t.Errorf("Unexpected status %d; body: %s", rr.Code, rr.Body.String())
	}
}

func TestWarehouseHandler_ListJobs_WithEngine(t *testing.T) {
	ts := newConfiguredWarehouseServer(t)

	rr := ts.get("/v1/warehouse/jobs")
	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", rr.Code)
	}
}

func TestWarehouseHandler_GetJob_WithEngine(t *testing.T) {
	ts := newConfiguredWarehouseServer(t)

	rr := ts.get("/v1/warehouse/jobs/nonexistent")
	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected 404, got %d", rr.Code)
	}
}

func TestWarehouseHandler_GetStats_WithEngine(t *testing.T) {
	ts := newConfiguredWarehouseServer(t)

	rr := ts.get("/v1/warehouse/stats")
	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", rr.Code)
	}
}
