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
