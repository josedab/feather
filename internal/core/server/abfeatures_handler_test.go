package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/feather-store/feather/internal/extensions/abfeatures"
)

type testABFeaturesServer struct {
	handler *ABFeaturesHandler
	manager *abfeatures.Manager
	mux     *http.ServeMux
	t       *testing.T
}

func newTestABFeaturesServer(t *testing.T) *testABFeaturesServer {
	t.Helper()
	manager := abfeatures.NewManager(abfeatures.DefaultExperimentConfig())
	handler := NewABFeaturesHandler(manager)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	return &testABFeaturesServer{handler: handler, manager: manager, mux: mux, t: t}
}

func (ts *testABFeaturesServer) request(method, path string, body string) *httptest.ResponseRecorder {
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

func TestABFeaturesHandler_GetStats(t *testing.T) {
	ts := newTestABFeaturesServer(t)
	rr := ts.request(http.MethodGet, "/v1/experiments/stats", "")
	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}
	var result map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}
}

func TestABFeaturesHandler_CreateExperiment(t *testing.T) {
	ts := newTestABFeaturesServer(t)
	body := `{"id":"exp-1","name":"button_color","variants":[{"id":"control","name":"blue","weight":50},{"id":"treatment","name":"green","weight":50}]}`
	rr := ts.request(http.MethodPost, "/v1/experiments", body)
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

func TestABFeaturesHandler_CreateExperiment_InvalidJSON(t *testing.T) {
	ts := newTestABFeaturesServer(t)
	rr := ts.request(http.MethodPost, "/v1/experiments", "not json")
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestABFeaturesHandler_GetExperiment_NotFound(t *testing.T) {
	ts := newTestABFeaturesServer(t)
	rr := ts.request(http.MethodGet, "/v1/experiments/nonexistent", "")
	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, rr.Code)
	}
}

func TestABFeaturesHandler_ListExperiments(t *testing.T) {
	ts := newTestABFeaturesServer(t)

	// Create an experiment first
	ts.request(http.MethodPost, "/v1/experiments",
		`{"id":"exp-1","name":"test","variants":[{"id":"control","name":"a","weight":50},{"id":"treat","name":"b","weight":50}]}`)

	rr := ts.request(http.MethodGet, "/v1/experiments", "")
	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	var result map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	experiments, ok := result["experiments"].([]interface{})
	if !ok {
		t.Fatal("expected experiments array")
	}
	if len(experiments) == 0 {
		t.Error("expected at least one experiment")
	}
}

func TestABFeaturesHandler_GetExperiment(t *testing.T) {
	ts := newTestABFeaturesServer(t)

	// Create experiment
	ts.request(http.MethodPost, "/v1/experiments",
		`{"id":"exp-1","name":"test","variants":[{"id":"control","name":"a","weight":50},{"id":"treat","name":"b","weight":50}]}`)

	rr := ts.request(http.MethodGet, "/v1/experiments/exp-1", "")
	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want %d; body: %s", rr.Code, http.StatusOK, rr.Body.String())
	}
}

func TestABFeaturesHandler_StartExperiment(t *testing.T) {
	ts := newTestABFeaturesServer(t)

	ts.request(http.MethodPost, "/v1/experiments",
		`{"id":"exp-1","name":"test","variants":[{"id":"control","name":"a","weight":50},{"id":"treat","name":"b","weight":50}]}`)

	rr := ts.request(http.MethodPost, "/v1/experiments/exp-1/start", "")
	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want %d; body: %s", rr.Code, http.StatusOK, rr.Body.String())
	}
}

func TestABFeaturesHandler_StopExperiment(t *testing.T) {
	ts := newTestABFeaturesServer(t)

	ts.request(http.MethodPost, "/v1/experiments",
		`{"id":"exp-1","name":"test","variants":[{"id":"control","name":"a","weight":50},{"id":"treat","name":"b","weight":50}]}`)
	ts.request(http.MethodPost, "/v1/experiments/exp-1/start", "")

	rr := ts.request(http.MethodPost, "/v1/experiments/exp-1/stop", "")
	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want %d; body: %s", rr.Code, http.StatusOK, rr.Body.String())
	}
}

func TestABFeaturesHandler_ResolveVariant(t *testing.T) {
	ts := newTestABFeaturesServer(t)

	ts.request(http.MethodPost, "/v1/experiments",
		`{"id":"exp-1","name":"test","variants":[{"id":"control","name":"a","weight":50},{"id":"treat","name":"b","weight":50}]}`)
	ts.request(http.MethodPost, "/v1/experiments/exp-1/start", "")

	rr := ts.request(http.MethodGet, "/v1/experiments/exp-1/resolve?entity_id=user:123", "")
	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want %d; body: %s", rr.Code, http.StatusOK, rr.Body.String())
	}
	var result map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if result["variant"] == nil {
		t.Error("expected variant in response")
	}
}

func TestABFeaturesHandler_ResolveVariant_MissingEntityID(t *testing.T) {
	ts := newTestABFeaturesServer(t)
	rr := ts.request(http.MethodGet, "/v1/experiments/exp-1/resolve", "")
	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestABFeaturesHandler_RecordMetric(t *testing.T) {
	ts := newTestABFeaturesServer(t)

	ts.request(http.MethodPost, "/v1/experiments",
		`{"id":"exp-1","name":"test","variants":[{"id":"control","name":"a","weight":50},{"id":"treat","name":"b","weight":50}]}`)

	rr := ts.request(http.MethodPost, "/v1/experiments/exp-1/metric",
		`{"variant_id":"control","latency_ms":15.5}`)
	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want %d; body: %s", rr.Code, http.StatusOK, rr.Body.String())
	}
}

func TestABFeaturesHandler_RecordMetric_InvalidJSON(t *testing.T) {
	ts := newTestABFeaturesServer(t)
	rr := ts.request(http.MethodPost, "/v1/experiments/exp-1/metric", "not json")
	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestABFeaturesHandler_RecordScore(t *testing.T) {
	ts := newTestABFeaturesServer(t)

	ts.request(http.MethodPost, "/v1/experiments",
		`{"id":"exp-1","name":"test","variants":[{"id":"control","name":"a","weight":50},{"id":"treat","name":"b","weight":50}]}`)

	rr := ts.request(http.MethodPost, "/v1/experiments/exp-1/score",
		`{"variant_id":"control","score":0.85}`)
	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want %d; body: %s", rr.Code, http.StatusOK, rr.Body.String())
	}
}

func TestABFeaturesHandler_RecordScore_InvalidJSON(t *testing.T) {
	ts := newTestABFeaturesServer(t)
	rr := ts.request(http.MethodPost, "/v1/experiments/exp-1/score", "bad")
	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestABFeaturesHandler_NilManager(t *testing.T) {
	handler := NewABFeaturesHandler(nil)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	endpoints := []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodGet, "/v1/experiments", ""},
		{http.MethodPost, "/v1/experiments", `{}`},
		{http.MethodGet, "/v1/experiments/exp-1", ""},
		{http.MethodPost, "/v1/experiments/exp-1/start", ""},
		{http.MethodPost, "/v1/experiments/exp-1/stop", ""},
		{http.MethodGet, "/v1/experiments/exp-1/resolve?entity_id=x", ""},
		{http.MethodPost, "/v1/experiments/exp-1/metric", `{"variant_id":"c","latency_ms":1}`},
		{http.MethodPost, "/v1/experiments/exp-1/score", `{"variant_id":"c","score":1}`},
		{http.MethodGet, "/v1/experiments/exp-1/significance", ""},
		{http.MethodGet, "/v1/experiments/stats", ""},
	}

	for _, ep := range endpoints {
		t.Run(ep.method+" "+ep.path, func(t *testing.T) {
			var bodyReader io.Reader
			if ep.body != "" {
				bodyReader = strings.NewReader(ep.body)
			}
			req := httptest.NewRequest(ep.method, ep.path, bodyReader)
			if ep.body != "" {
				req.Header.Set("Content-Type", "application/json")
			}
			rr := httptest.NewRecorder()
			mux.ServeHTTP(rr, req)
			if rr.Code != http.StatusServiceUnavailable {
				t.Errorf("status = %d, want %d", rr.Code, http.StatusServiceUnavailable)
			}
		})
	}
}

func TestErrFromString(t *testing.T) {
	if errFromString("") != nil {
		t.Error("empty string should return nil")
	}
	err := errFromString("test error")
	if err == nil || err.Error() != "test error" {
		t.Errorf("expected error with message 'test error', got %v", err)
	}
}
