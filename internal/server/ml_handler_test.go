package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/feather-store/feather/internal/storage"
)

// testMLServer wraps an MLHandler for testing.
type testMLServer struct {
	handler *MLHandler
	mux     *http.ServeMux
	t       *testing.T
}

// newTestMLServer creates a new test ML server.
func newTestMLServer(t *testing.T) *testMLServer {
	t.Helper()

	store, err := storage.NewStore(storage.StoreOptions{
		HotMaxSize:   1024 * 1024, // 1MB
		WarmInMemory: true,
	}, storage.NewRegistry())
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	handler := NewMLHandler(store)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	return &testMLServer{
		handler: handler,
		mux:     mux,
		t:       t,
	}
}

func (ts *testMLServer) request(method, path string, body string) *httptest.ResponseRecorder {
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

func (ts *testMLServer) postJSON(path string, body interface{}) *httptest.ResponseRecorder {
	ts.t.Helper()

	jsonBody, err := json.Marshal(body)
	if err != nil {
		ts.t.Fatalf("failed to marshal body: %v", err)
	}
	return ts.request(http.MethodPost, path, string(jsonBody))
}

func (ts *testMLServer) get(path string) *httptest.ResponseRecorder {
	return ts.request(http.MethodGet, path, "")
}

func (ts *testMLServer) delete(path string) *httptest.ResponseRecorder {
	return ts.request(http.MethodDelete, path, "")
}

// registerConnector is a helper to register a connector for testing.
func (ts *testMLServer) registerConnector(name, connType string) *httptest.ResponseRecorder {
	ts.t.Helper()

	body := ConnectorRequest{
		Name:     name,
		Type:     connType,
		Endpoint: "http://localhost:8501",
	}
	return ts.postJSON("/v1/ml/connectors", body)
}

func TestMLHandler_NewMLHandler(t *testing.T) {
	store, err := storage.NewStore(storage.StoreOptions{
		HotMaxSize:   1024 * 1024,
		WarmInMemory: true,
	}, storage.NewRegistry())
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	handler := NewMLHandler(store)

	if handler.registry == nil {
		t.Error("Expected registry to be set")
	}
	if handler.store == nil {
		t.Error("Expected store to be set")
	}
}

func TestMLHandler_ListConnectors_Empty(t *testing.T) {
	ts := newTestMLServer(t)

	rr := ts.get("/v1/ml/connectors")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if result["count"].(float64) != 0 {
		t.Errorf("Expected count=0, got %v", result["count"])
	}
}

func TestMLHandler_RegisterConnector_TensorFlow(t *testing.T) {
	ts := newTestMLServer(t)

	body := ConnectorRequest{
		Name:     "tf-connector",
		Type:     "tensorflow",
		Endpoint: "http://localhost:8501",
	}

	rr := ts.postJSON("/v1/ml/connectors", body)

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
	if result["name"] != "tf-connector" {
		t.Errorf("Expected name 'tf-connector', got %v", result["name"])
	}
	if result["type"] != "tensorflow" {
		t.Errorf("Expected type 'tensorflow', got %v", result["type"])
	}
}

func TestMLHandler_RegisterConnector_MLflow(t *testing.T) {
	ts := newTestMLServer(t)

	body := ConnectorRequest{
		Name:        "mlflow-connector",
		Type:        "mlflow",
		Endpoint:    "http://localhost:5000",
		TrackingURI: "http://localhost:5000",
	}

	rr := ts.postJSON("/v1/ml/connectors", body)

	if rr.Code != http.StatusCreated {
		t.Errorf("Expected status %d, got %d; body: %s", http.StatusCreated, rr.Code, rr.Body.String())
	}
}

func TestMLHandler_RegisterConnector_SageMaker(t *testing.T) {
	ts := newTestMLServer(t)

	body := ConnectorRequest{
		Name:         "sagemaker-connector",
		Type:         "sagemaker",
		Endpoint:     "https://runtime.sagemaker.us-west-2.amazonaws.com",
		Region:       "us-west-2",
		EndpointName: "my-endpoint",
	}

	rr := ts.postJSON("/v1/ml/connectors", body)

	if rr.Code != http.StatusCreated {
		t.Errorf("Expected status %d, got %d; body: %s", http.StatusCreated, rr.Code, rr.Body.String())
	}
}

func TestMLHandler_RegisterConnector_MissingName(t *testing.T) {
	ts := newTestMLServer(t)

	body := ConnectorRequest{
		Type:     "tensorflow",
		Endpoint: "http://localhost:8501",
	}

	rr := ts.postJSON("/v1/ml/connectors", body)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestMLHandler_RegisterConnector_MissingType(t *testing.T) {
	ts := newTestMLServer(t)

	body := ConnectorRequest{
		Name:     "test-connector",
		Endpoint: "http://localhost:8501",
	}

	rr := ts.postJSON("/v1/ml/connectors", body)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestMLHandler_RegisterConnector_UnsupportedType(t *testing.T) {
	ts := newTestMLServer(t)

	body := ConnectorRequest{
		Name:     "test-connector",
		Type:     "unknown",
		Endpoint: "http://localhost:8501",
	}

	rr := ts.postJSON("/v1/ml/connectors", body)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestMLHandler_RegisterConnector_Duplicate(t *testing.T) {
	ts := newTestMLServer(t)

	body := ConnectorRequest{
		Name:     "dup-connector",
		Type:     "tensorflow",
		Endpoint: "http://localhost:8501",
	}

	// Register first time
	ts.postJSON("/v1/ml/connectors", body)

	// Try to register again
	rr := ts.postJSON("/v1/ml/connectors", body)

	if rr.Code != http.StatusConflict {
		t.Errorf("Expected status %d, got %d", http.StatusConflict, rr.Code)
	}
}

func TestMLHandler_RegisterConnector_InvalidBody(t *testing.T) {
	ts := newTestMLServer(t)

	rr := ts.request(http.MethodPost, "/v1/ml/connectors", "invalid json")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestMLHandler_GetConnector(t *testing.T) {
	ts := newTestMLServer(t)

	// Register connector first
	ts.registerConnector("get-connector", "tensorflow")

	rr := ts.get("/v1/ml/connectors/get-connector")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d; body: %s", http.StatusOK, rr.Code, rr.Body.String())
	}

	var result map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if result["name"] != "get-connector" {
		t.Errorf("Expected name 'get-connector', got %v", result["name"])
	}
	if result["type"] != "tensorflow" {
		t.Errorf("Expected type 'tensorflow', got %v", result["type"])
	}
}

func TestMLHandler_GetConnector_NotFound(t *testing.T) {
	ts := newTestMLServer(t)

	rr := ts.get("/v1/ml/connectors/nonexistent")

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, rr.Code)
	}
}

func TestMLHandler_UnregisterConnector(t *testing.T) {
	ts := newTestMLServer(t)

	// Register connector first
	ts.registerConnector("delete-connector", "tensorflow")

	rr := ts.delete("/v1/ml/connectors/delete-connector")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d; body: %s", http.StatusOK, rr.Code, rr.Body.String())
	}

	// Verify it's deleted
	rr = ts.get("/v1/ml/connectors/delete-connector")
	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected connector to be deleted, got status %d", rr.Code)
	}
}

func TestMLHandler_UnregisterConnector_NotFound(t *testing.T) {
	ts := newTestMLServer(t)

	rr := ts.delete("/v1/ml/connectors/nonexistent")

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, rr.Code)
	}
}

func TestMLHandler_Connect(t *testing.T) {
	ts := newTestMLServer(t)

	// Register connector first
	ts.registerConnector("connect-test", "tensorflow")

	rr := ts.postJSON("/v1/ml/connectors/connect-test/connect", struct{}{})

	// Connect may fail if endpoint is not available, accept 200 or 500
	if rr.Code != http.StatusOK && rr.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 200 or 500, got %d; body: %s", rr.Code, rr.Body.String())
	}
}

func TestMLHandler_Connect_NotFound(t *testing.T) {
	ts := newTestMLServer(t)

	rr := ts.postJSON("/v1/ml/connectors/nonexistent/connect", struct{}{})

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, rr.Code)
	}
}

func TestMLHandler_Disconnect(t *testing.T) {
	ts := newTestMLServer(t)

	// Register connector first
	ts.registerConnector("disconnect-test", "tensorflow")

	rr := ts.postJSON("/v1/ml/connectors/disconnect-test/disconnect", struct{}{})

	if rr.Code != http.StatusOK && rr.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 200 or 500, got %d; body: %s", rr.Code, rr.Body.String())
	}
}

func TestMLHandler_Disconnect_NotFound(t *testing.T) {
	ts := newTestMLServer(t)

	rr := ts.postJSON("/v1/ml/connectors/nonexistent/disconnect", struct{}{})

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, rr.Code)
	}
}

func TestMLHandler_Predict_MissingConnector(t *testing.T) {
	ts := newTestMLServer(t)

	body := PredictAPIRequest{
		ModelName: "my-model",
		Features:  map[string]interface{}{"feature1": 1.0},
	}

	rr := ts.postJSON("/v1/ml/predict", body)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestMLHandler_Predict_MissingModelName(t *testing.T) {
	ts := newTestMLServer(t)

	body := PredictAPIRequest{
		Connector: "test-connector",
		Features:  map[string]interface{}{"feature1": 1.0},
	}

	rr := ts.postJSON("/v1/ml/predict", body)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestMLHandler_Predict_ConnectorNotFound(t *testing.T) {
	ts := newTestMLServer(t)

	body := PredictAPIRequest{
		Connector: "nonexistent",
		ModelName: "my-model",
		Features:  map[string]interface{}{"feature1": 1.0},
	}

	rr := ts.postJSON("/v1/ml/predict", body)

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, rr.Code)
	}
}

func TestMLHandler_Predict_InvalidBody(t *testing.T) {
	ts := newTestMLServer(t)

	rr := ts.request(http.MethodPost, "/v1/ml/predict", "invalid json")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestMLHandler_BatchPredict_MissingConnector(t *testing.T) {
	ts := newTestMLServer(t)

	body := BatchPredictAPIRequest{
		ModelName: "my-model",
		Features:  []map[string]interface{}{{"feature1": 1.0}},
	}

	rr := ts.postJSON("/v1/ml/predict/batch", body)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestMLHandler_BatchPredict_MissingModelName(t *testing.T) {
	ts := newTestMLServer(t)

	body := BatchPredictAPIRequest{
		Connector: "test-connector",
		Features:  []map[string]interface{}{{"feature1": 1.0}},
	}

	rr := ts.postJSON("/v1/ml/predict/batch", body)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestMLHandler_BatchPredict_ConnectorNotFound(t *testing.T) {
	ts := newTestMLServer(t)

	body := BatchPredictAPIRequest{
		Connector: "nonexistent",
		ModelName: "my-model",
		Features:  []map[string]interface{}{{"feature1": 1.0}},
	}

	rr := ts.postJSON("/v1/ml/predict/batch", body)

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, rr.Code)
	}
}

func TestMLHandler_BatchPredict_InvalidBody(t *testing.T) {
	ts := newTestMLServer(t)

	rr := ts.request(http.MethodPost, "/v1/ml/predict/batch", "invalid json")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestMLHandler_ListConnectors_WithConnectors(t *testing.T) {
	ts := newTestMLServer(t)

	// Register some connectors
	ts.registerConnector("connector-1", "tensorflow")
	ts.registerConnector("connector-2", "mlflow")

	rr := ts.get("/v1/ml/connectors")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if result["count"].(float64) != 2 {
		t.Errorf("Expected count=2, got %v", result["count"])
	}
}
