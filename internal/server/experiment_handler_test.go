package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/feather-store/feather/internal/experiment"
)

// testExperimentServer wraps an ExperimentHandler for testing.
type testExperimentServer struct {
	handler *ExperimentHandler
	engine  *experiment.Engine
	mux     *http.ServeMux
	t       *testing.T
}

func newTestExperimentServer(t *testing.T) *testExperimentServer {
	t.Helper()

	engine := experiment.NewEngine()
	handler := NewExperimentHandler(engine)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	return &testExperimentServer{
		handler: handler,
		engine:  engine,
		mux:     mux,
		t:       t,
	}
}

func newTestExperimentServerWithoutEngine(t *testing.T) *testExperimentServer {
	t.Helper()

	handler := NewExperimentHandler(nil)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	return &testExperimentServer{
		handler: handler,
		engine:  nil,
		mux:     mux,
		t:       t,
	}
}

func (ts *testExperimentServer) request(method, path string, body string) *httptest.ResponseRecorder {
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

func (ts *testExperimentServer) postJSON(path string, body interface{}) *httptest.ResponseRecorder {
	ts.t.Helper()

	jsonBody, err := json.Marshal(body)
	if err != nil {
		ts.t.Fatalf("failed to marshal body: %v", err)
	}
	return ts.request(http.MethodPost, path, string(jsonBody))
}

func (ts *testExperimentServer) putJSON(path string, body interface{}) *httptest.ResponseRecorder {
	ts.t.Helper()

	jsonBody, err := json.Marshal(body)
	if err != nil {
		ts.t.Fatalf("failed to marshal body: %v", err)
	}
	return ts.request(http.MethodPut, path, string(jsonBody))
}

func (ts *testExperimentServer) get(path string) *httptest.ResponseRecorder {
	return ts.request(http.MethodGet, path, "")
}

func TestExperimentHandler_NewExperimentHandler(t *testing.T) {
	engine := experiment.NewEngine()
	handler := NewExperimentHandler(engine)

	if handler.engine == nil {
		t.Error("Expected engine to be set")
	}
}

func TestExperimentHandler_ListExperiments_Empty(t *testing.T) {
	ts := newTestExperimentServer(t)

	rr := ts.get("/v1/experiments")

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

func TestExperimentHandler_ListExperiments_NoEngine(t *testing.T) {
	ts := newTestExperimentServerWithoutEngine(t)

	rr := ts.get("/v1/experiments")

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
	}
}

func TestExperimentHandler_CreateExperiment(t *testing.T) {
	ts := newTestExperimentServer(t)

	body := ExperimentJSON{
		ID:   "exp-1",
		Name: "Test Experiment",
		Type: "ab_test",
		Variants: []VariantJSON{
			{ID: "control", Name: "Control", Weight: 0.5, IsControl: true},
			{ID: "treatment", Name: "Treatment", Weight: 0.5},
		},
		Allocation: AllocationConfigJSON{
			Strategy:   "random",
			Percentage: 100,
		},
	}

	rr := ts.postJSON("/v1/experiments", body)

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

func TestExperimentHandler_CreateExperiment_InvalidBody(t *testing.T) {
	ts := newTestExperimentServer(t)

	rr := ts.request(http.MethodPost, "/v1/experiments", "invalid json")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestExperimentHandler_CreateExperiment_NoEngine(t *testing.T) {
	ts := newTestExperimentServerWithoutEngine(t)

	body := ExperimentJSON{
		ID:   "exp-1",
		Name: "Test",
	}

	rr := ts.postJSON("/v1/experiments", body)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
	}
}

func TestExperimentHandler_GetExperiment(t *testing.T) {
	ts := newTestExperimentServer(t)

	// Create experiment first with valid A/B test configuration
	ts.postJSON("/v1/experiments", ExperimentJSON{
		ID:   "get-exp",
		Name: "Get Test",
		Type: "ab_test",
		Variants: []VariantJSON{
			{ID: "control", Name: "Control", Weight: 0.5, IsControl: true},
			{ID: "treatment", Name: "Treatment", Weight: 0.5},
		},
		Allocation: AllocationConfigJSON{Strategy: "random", Percentage: 100},
	})

	rr := ts.get("/v1/experiments/get-exp")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d; body: %s", http.StatusOK, rr.Code, rr.Body.String())
	}
}

func TestExperimentHandler_GetExperiment_NotFound(t *testing.T) {
	ts := newTestExperimentServer(t)

	rr := ts.get("/v1/experiments/nonexistent")

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, rr.Code)
	}
}

func TestExperimentHandler_GetExperiment_NoEngine(t *testing.T) {
	ts := newTestExperimentServerWithoutEngine(t)

	rr := ts.get("/v1/experiments/test")

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
	}
}

func TestExperimentHandler_ListActiveExperiments(t *testing.T) {
	ts := newTestExperimentServer(t)

	rr := ts.get("/v1/experiments/active")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}
}

func TestExperimentHandler_ListActiveExperiments_NoEngine(t *testing.T) {
	ts := newTestExperimentServerWithoutEngine(t)

	rr := ts.get("/v1/experiments/active")

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
	}
}

func TestExperimentHandler_UpdateExperiment(t *testing.T) {
	ts := newTestExperimentServer(t)

	// Create experiment first with valid A/B test configuration
	ts.postJSON("/v1/experiments", ExperimentJSON{
		ID:   "update-exp",
		Name: "Original Name",
		Type: "ab_test",
		Variants: []VariantJSON{
			{ID: "control", Name: "Control", Weight: 0.5, IsControl: true},
			{ID: "treatment", Name: "Treatment", Weight: 0.5},
		},
		Allocation: AllocationConfigJSON{Strategy: "random", Percentage: 100},
	})

	// Update it
	body := ExperimentJSON{
		Name: "Updated Name",
		Type: "ab_test",
		Variants: []VariantJSON{
			{ID: "control", Name: "Control", Weight: 0.5, IsControl: true},
			{ID: "treatment", Name: "Treatment", Weight: 0.5},
		},
		Allocation: AllocationConfigJSON{Strategy: "random", Percentage: 100},
	}

	rr := ts.putJSON("/v1/experiments/update-exp", body)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d; body: %s", http.StatusOK, rr.Code, rr.Body.String())
	}
}

func TestExperimentHandler_UpdateExperiment_InvalidBody(t *testing.T) {
	ts := newTestExperimentServer(t)

	rr := ts.request(http.MethodPut, "/v1/experiments/test", "invalid json")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestExperimentHandler_UpdateExperiment_NoEngine(t *testing.T) {
	ts := newTestExperimentServerWithoutEngine(t)

	rr := ts.putJSON("/v1/experiments/test", ExperimentJSON{})

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
	}
}

func TestExperimentHandler_StartExperiment(t *testing.T) {
	ts := newTestExperimentServer(t)

	// Create experiment first with valid A/B test configuration
	ts.postJSON("/v1/experiments", ExperimentJSON{
		ID:   "start-exp",
		Name: "Start Test",
		Type: "ab_test",
		Variants: []VariantJSON{
			{ID: "control", Name: "Control", Weight: 0.5, IsControl: true},
			{ID: "treatment", Name: "Treatment", Weight: 0.5},
		},
		Allocation: AllocationConfigJSON{Strategy: "random", Percentage: 100},
	})

	rr := ts.postJSON("/v1/experiments/start-exp/start", nil)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d; body: %s", http.StatusOK, rr.Code, rr.Body.String())
	}
}

func TestExperimentHandler_StartExperiment_NotFound(t *testing.T) {
	ts := newTestExperimentServer(t)

	rr := ts.postJSON("/v1/experiments/nonexistent/start", nil)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestExperimentHandler_StartExperiment_NoEngine(t *testing.T) {
	ts := newTestExperimentServerWithoutEngine(t)

	rr := ts.postJSON("/v1/experiments/test/start", nil)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
	}
}

func TestExperimentHandler_PauseExperiment(t *testing.T) {
	ts := newTestExperimentServer(t)

	// Create and start experiment with valid A/B test configuration
	ts.postJSON("/v1/experiments", ExperimentJSON{
		ID:   "pause-exp",
		Name: "Pause Test",
		Type: "ab_test",
		Variants: []VariantJSON{
			{ID: "control", Name: "Control", Weight: 0.5, IsControl: true},
			{ID: "treatment", Name: "Treatment", Weight: 0.5},
		},
		Allocation: AllocationConfigJSON{Strategy: "random", Percentage: 100},
	})
	ts.postJSON("/v1/experiments/pause-exp/start", nil)

	rr := ts.postJSON("/v1/experiments/pause-exp/pause", nil)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d; body: %s", http.StatusOK, rr.Code, rr.Body.String())
	}
}

func TestExperimentHandler_PauseExperiment_NoEngine(t *testing.T) {
	ts := newTestExperimentServerWithoutEngine(t)

	rr := ts.postJSON("/v1/experiments/test/pause", nil)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
	}
}

func TestExperimentHandler_StopExperiment(t *testing.T) {
	ts := newTestExperimentServer(t)

	// Create and start experiment with valid A/B test configuration
	ts.postJSON("/v1/experiments", ExperimentJSON{
		ID:   "stop-exp",
		Name: "Stop Test",
		Type: "ab_test",
		Variants: []VariantJSON{
			{ID: "control", Name: "Control", Weight: 0.5, IsControl: true},
			{ID: "treatment", Name: "Treatment", Weight: 0.5},
		},
		Allocation: AllocationConfigJSON{Strategy: "random", Percentage: 100},
	})
	ts.postJSON("/v1/experiments/stop-exp/start", nil)

	rr := ts.postJSON("/v1/experiments/stop-exp/stop", map[string]bool{"completed": true})

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d; body: %s", http.StatusOK, rr.Code, rr.Body.String())
	}
}

func TestExperimentHandler_StopExperiment_NoEngine(t *testing.T) {
	ts := newTestExperimentServerWithoutEngine(t)

	rr := ts.postJSON("/v1/experiments/test/stop", nil)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
	}
}

func TestExperimentHandler_GetAssignment(t *testing.T) {
	ts := newTestExperimentServer(t)

	// Create and start experiment with valid A/B test configuration
	ts.postJSON("/v1/experiments", ExperimentJSON{
		ID:   "assign-exp",
		Name: "Assignment Test",
		Type: "ab_test",
		Variants: []VariantJSON{
			{ID: "control", Name: "Control", Weight: 0.5, IsControl: true},
			{ID: "treatment", Name: "Treatment", Weight: 0.5},
		},
		Allocation: AllocationConfigJSON{Strategy: "random", Percentage: 100},
	})
	ts.postJSON("/v1/experiments/assign-exp/start", nil)

	body := AssignmentRequest{
		UserID: "user-123",
	}

	rr := ts.postJSON("/v1/experiments/assign-exp/assign", body)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d; body: %s", http.StatusOK, rr.Code, rr.Body.String())
	}
}

func TestExperimentHandler_GetAssignment_MissingUserID(t *testing.T) {
	ts := newTestExperimentServer(t)

	rr := ts.postJSON("/v1/experiments/test/assign", AssignmentRequest{})

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestExperimentHandler_GetAssignment_InvalidBody(t *testing.T) {
	ts := newTestExperimentServer(t)

	rr := ts.request(http.MethodPost, "/v1/experiments/test/assign", "invalid json")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestExperimentHandler_GetAssignment_NoEngine(t *testing.T) {
	ts := newTestExperimentServerWithoutEngine(t)

	rr := ts.postJSON("/v1/experiments/test/assign", AssignmentRequest{UserID: "user"})

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
	}
}

func TestExperimentHandler_TrackExposure(t *testing.T) {
	ts := newTestExperimentServer(t)

	body := ExposureEventJSON{
		ExperimentID: "exp-1",
		VariantID:    "control",
		UserID:       "user-123",
	}

	rr := ts.postJSON("/v1/experiments/exposure", body)

	if rr.Code != http.StatusCreated {
		t.Errorf("Expected status %d, got %d; body: %s", http.StatusCreated, rr.Code, rr.Body.String())
	}
}

func TestExperimentHandler_TrackExposure_MissingFields(t *testing.T) {
	ts := newTestExperimentServer(t)

	body := ExposureEventJSON{
		ExperimentID: "exp-1",
		// Missing UserID
	}

	rr := ts.postJSON("/v1/experiments/exposure", body)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestExperimentHandler_TrackExposure_InvalidBody(t *testing.T) {
	ts := newTestExperimentServer(t)

	rr := ts.request(http.MethodPost, "/v1/experiments/exposure", "invalid json")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestExperimentHandler_TrackExposure_NoEngine(t *testing.T) {
	ts := newTestExperimentServerWithoutEngine(t)

	body := ExposureEventJSON{
		ExperimentID: "exp-1",
		UserID:       "user-123",
	}

	rr := ts.postJSON("/v1/experiments/exposure", body)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
	}
}

func TestExperimentHandler_TrackMetric(t *testing.T) {
	ts := newTestExperimentServer(t)

	body := MetricEventJSON{
		ExperimentID: "exp-1",
		MetricID:     "conversion",
		UserID:       "user-123",
		VariantID:    "control",
		Value:        1.0,
	}

	rr := ts.postJSON("/v1/experiments/metric", body)

	if rr.Code != http.StatusCreated {
		t.Errorf("Expected status %d, got %d; body: %s", http.StatusCreated, rr.Code, rr.Body.String())
	}
}

func TestExperimentHandler_TrackMetric_MissingFields(t *testing.T) {
	ts := newTestExperimentServer(t)

	body := MetricEventJSON{
		ExperimentID: "exp-1",
		// Missing MetricID and UserID
	}

	rr := ts.postJSON("/v1/experiments/metric", body)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestExperimentHandler_TrackMetric_InvalidBody(t *testing.T) {
	ts := newTestExperimentServer(t)

	rr := ts.request(http.MethodPost, "/v1/experiments/metric", "invalid json")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestExperimentHandler_TrackMetric_NoEngine(t *testing.T) {
	ts := newTestExperimentServerWithoutEngine(t)

	body := MetricEventJSON{
		ExperimentID: "exp-1",
		MetricID:     "conversion",
		UserID:       "user-123",
	}

	rr := ts.postJSON("/v1/experiments/metric", body)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
	}
}

func TestExperimentHandler_AnalyzeExperiment(t *testing.T) {
	ts := newTestExperimentServer(t)

	// Create experiment with valid A/B test configuration
	ts.postJSON("/v1/experiments", ExperimentJSON{
		ID:   "analyze-exp",
		Name: "Analyze Test",
		Type: "ab_test",
		Variants: []VariantJSON{
			{ID: "control", Name: "Control", Weight: 0.5, IsControl: true},
			{ID: "treatment", Name: "Treatment", Weight: 0.5},
		},
		Allocation: AllocationConfigJSON{Strategy: "random", Percentage: 100},
	})

	rr := ts.get("/v1/experiments/analyze-exp/results")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d; body: %s", http.StatusOK, rr.Code, rr.Body.String())
	}
}

func TestExperimentHandler_AnalyzeExperiment_NotFound(t *testing.T) {
	ts := newTestExperimentServer(t)

	rr := ts.get("/v1/experiments/nonexistent/results")

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, rr.Code)
	}
}

func TestExperimentHandler_AnalyzeExperiment_NoEngine(t *testing.T) {
	ts := newTestExperimentServerWithoutEngine(t)

	rr := ts.get("/v1/experiments/test/results")

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
	}
}

func TestExperimentHandler_GetExperimentsByFeature(t *testing.T) {
	ts := newTestExperimentServer(t)

	// Create experiment with feature ID (feature_flag type allows single variant)
	ts.postJSON("/v1/experiments", ExperimentJSON{
		ID:        "feature-exp",
		Name:      "Feature Test",
		Type:      "feature_flag",
		FeatureID: "my-feature",
		Variants: []VariantJSON{
			{ID: "enabled", Name: "Enabled", Weight: 1.0},
		},
		Allocation: AllocationConfigJSON{Strategy: "random", Percentage: 100},
	})

	rr := ts.get("/v1/features/my-feature/experiments")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d; body: %s", http.StatusOK, rr.Code, rr.Body.String())
	}
}

func TestExperimentHandler_GetExperimentsByFeature_NoEngine(t *testing.T) {
	ts := newTestExperimentServerWithoutEngine(t)

	rr := ts.get("/v1/features/test/experiments")

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
	}
}

func TestExperimentHandler_GetFeatureValue(t *testing.T) {
	ts := newTestExperimentServer(t)

	body := FeatureValueRequest{
		FeatureID:    "my-feature",
		UserID:       "user-123",
		DefaultValue: false,
	}

	rr := ts.postJSON("/v1/features/value", body)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d; body: %s", http.StatusOK, rr.Code, rr.Body.String())
	}
}

func TestExperimentHandler_GetFeatureValue_MissingFields(t *testing.T) {
	ts := newTestExperimentServer(t)

	body := FeatureValueRequest{
		// Missing FeatureID and UserID
	}

	rr := ts.postJSON("/v1/features/value", body)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestExperimentHandler_GetFeatureValue_InvalidBody(t *testing.T) {
	ts := newTestExperimentServer(t)

	rr := ts.request(http.MethodPost, "/v1/features/value", "invalid json")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestExperimentHandler_GetFeatureValue_NoEngine(t *testing.T) {
	ts := newTestExperimentServerWithoutEngine(t)

	body := FeatureValueRequest{
		FeatureID: "my-feature",
		UserID:    "user-123",
	}

	rr := ts.postJSON("/v1/features/value", body)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
	}
}
