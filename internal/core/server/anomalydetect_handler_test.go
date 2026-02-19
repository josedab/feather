package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/feather-store/feather/internal/extensions/anomalydetect"
)

type testAnomalyDetectServer struct {
	handler  *AnomalyDetectHandler
	detector *anomalydetect.Detector
	mux      *http.ServeMux
	t        *testing.T
}

func newTestAnomalyDetectServer(t *testing.T) *testAnomalyDetectServer {
	t.Helper()

	detector := anomalydetect.NewDetector(anomalydetect.DefaultDetectorConfig())
	handler := NewAnomalyDetectHandler(detector)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	return &testAnomalyDetectServer{
		handler:  handler,
		detector: detector,
		mux:      mux,
		t:        t,
	}
}

func (ts *testAnomalyDetectServer) request(method, path string, body string) *httptest.ResponseRecorder {
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

func (ts *testAnomalyDetectServer) postJSON(path string, body interface{}) *httptest.ResponseRecorder {
	ts.t.Helper()

	jsonBody, err := json.Marshal(body)
	if err != nil {
		ts.t.Fatalf("failed to marshal body: %v", err)
	}
	return ts.request(http.MethodPost, path, string(jsonBody))
}

func (ts *testAnomalyDetectServer) get(path string) *httptest.ResponseRecorder {
	return ts.request(http.MethodGet, path, "")
}

func TestAnomalyDetectHandler_NewHandler(t *testing.T) {
	detector := anomalydetect.NewDetector(anomalydetect.DefaultDetectorConfig())
	handler := NewAnomalyDetectHandler(detector)

	if handler.detector == nil {
		t.Error("Expected detector to be set")
	}
}

func TestAnomalyDetectHandler_RegisterFeature(t *testing.T) {
	ts := newTestAnomalyDetectServer(t)

	body := map[string]interface{}{
		"name": "click_rate",
	}

	rr := ts.postJSON("/v1/anomaly/register", body)

	if rr.Code != http.StatusCreated {
		t.Errorf("Expected status %d, got %d; body: %s", http.StatusCreated, rr.Code, rr.Body.String())
	}

	var result SuccessResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if !result.Success {
		t.Error("Expected success to be true")
	}
}

func TestAnomalyDetectHandler_RegisterFeature_MissingName(t *testing.T) {
	ts := newTestAnomalyDetectServer(t)

	body := map[string]interface{}{}

	rr := ts.postJSON("/v1/anomaly/register", body)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestAnomalyDetectHandler_RegisterFeature_InvalidBody(t *testing.T) {
	ts := newTestAnomalyDetectServer(t)

	rr := ts.request(http.MethodPost, "/v1/anomaly/register", "invalid json")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestAnomalyDetectHandler_Check(t *testing.T) {
	ts := newTestAnomalyDetectServer(t)

	ts.detector.RegisterFeature("click_rate")

	body := map[string]interface{}{
		"feature": "click_rate",
		"value":   1.5,
	}

	rr := ts.postJSON("/v1/anomaly/check", body)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d; body: %s", http.StatusOK, rr.Code, rr.Body.String())
	}
}

func TestAnomalyDetectHandler_Check_MissingFeature(t *testing.T) {
	ts := newTestAnomalyDetectServer(t)

	body := map[string]interface{}{
		"value": 1.5,
	}

	rr := ts.postJSON("/v1/anomaly/check", body)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestAnomalyDetectHandler_GetAlerts(t *testing.T) {
	ts := newTestAnomalyDetectServer(t)

	rr := ts.get("/v1/anomaly/alerts")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if _, ok := result["alerts"]; !ok {
		t.Error("Expected alerts key in response")
	}
}

func TestAnomalyDetectHandler_GetAlerts_WithSince(t *testing.T) {
	ts := newTestAnomalyDetectServer(t)

	rr := ts.get("/v1/anomaly/alerts?since=2024-01-01T00:00:00Z&limit=10")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}
}

func TestAnomalyDetectHandler_IsQuarantined(t *testing.T) {
	ts := newTestAnomalyDetectServer(t)

	ts.detector.RegisterFeature("click_rate")

	rr := ts.get("/v1/anomaly/quarantine/click_rate")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}
}

func TestAnomalyDetectHandler_ClearQuarantine_NotMonitored(t *testing.T) {
	ts := newTestAnomalyDetectServer(t)

	rr := ts.postJSON("/v1/anomaly/quarantine/nonexistent/clear", nil)

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, rr.Code)
	}
}

func TestAnomalyDetectHandler_GetFeatureStats_NotMonitored(t *testing.T) {
	ts := newTestAnomalyDetectServer(t)

	rr := ts.get("/v1/anomaly/features/nonexistent/stats")

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, rr.Code)
	}
}

func TestAnomalyDetectHandler_GetStats(t *testing.T) {
	ts := newTestAnomalyDetectServer(t)

	rr := ts.get("/v1/anomaly/stats")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}
}
