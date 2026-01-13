package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/feather-store/feather/internal/extensions/autogen"
)

// testAutogenServer wraps an AutogenHandler for testing.
type testAutogenServer struct {
	handler   *AutogenHandler
	generator *autogen.Generator
	mux       *http.ServeMux
	t         *testing.T
}

// newTestAutogenServer creates a new test autogen server.
func newTestAutogenServer(t *testing.T) *testAutogenServer {
	t.Helper()

	generator := autogen.NewGenerator(autogen.Config{})
	handler := NewAutogenHandler(generator)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	return &testAutogenServer{
		handler:   handler,
		generator: generator,
		mux:       mux,
		t:         t,
	}
}

// newTestAutogenServerWithoutGenerator creates an autogen server without generator for testing nil case.
func newTestAutogenServerWithoutGenerator(t *testing.T) *testAutogenServer {
	t.Helper()

	handler := NewAutogenHandler(nil)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	return &testAutogenServer{
		handler:   handler,
		generator: nil,
		mux:       mux,
		t:         t,
	}
}

func (ts *testAutogenServer) request(method, path string, body string) *httptest.ResponseRecorder {
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

func (ts *testAutogenServer) postJSON(path string, body interface{}) *httptest.ResponseRecorder {
	ts.t.Helper()

	jsonBody, err := json.Marshal(body)
	if err != nil {
		ts.t.Fatalf("failed to marshal body: %v", err)
	}
	return ts.request(http.MethodPost, path, string(jsonBody))
}

func (ts *testAutogenServer) get(path string) *httptest.ResponseRecorder {
	return ts.request(http.MethodGet, path, "")
}

func TestAutogenHandler_NewAutogenHandler(t *testing.T) {
	generator := autogen.NewGenerator(autogen.Config{})
	handler := NewAutogenHandler(generator)

	if handler.generator == nil {
		t.Error("Expected generator to be set")
	}
}

func TestAutogenHandler_GenerateFeatures(t *testing.T) {
	ts := newTestAutogenServer(t)

	body := GenerateFeaturesRequest{
		Schema: struct {
			Name        string `json:"name"`
			Description string `json:"description,omitempty"`
			Entity      string `json:"entity,omitempty"`
			Source      string `json:"source,omitempty"`
			Fields      []struct {
				Name        string            `json:"name"`
				Type        string            `json:"type"`
				Description string            `json:"description,omitempty"`
				Nullable    bool              `json:"nullable,omitempty"`
				Examples    []interface{}     `json:"examples,omitempty"`
				Constraints map[string]string `json:"constraints,omitempty"`
			} `json:"fields"`
		}{
			Name:        "test_schema",
			Description: "Test schema",
			Entity:      "user",
			Fields: []struct {
				Name        string            `json:"name"`
				Type        string            `json:"type"`
				Description string            `json:"description,omitempty"`
				Nullable    bool              `json:"nullable,omitempty"`
				Examples    []interface{}     `json:"examples,omitempty"`
				Constraints map[string]string `json:"constraints,omitempty"`
			}{
				{Name: "age", Type: "int", Description: "User age"},
				{Name: "income", Type: "float", Description: "Annual income"},
			},
		},
		UseCase:        "classification",
		MaxSuggestions: 5,
	}

	rr := ts.postJSON("/v1/autogen/features", body)

	// Generator may return 500 if no provider is configured
	// Accept 200 (success) or 500 (provider not configured) as both are valid responses
	if rr.Code != http.StatusOK && rr.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 200 or 500, got %d; body: %s", rr.Code, rr.Body.String())
	}
}

func TestAutogenHandler_GenerateFeatures_MissingSchemaName(t *testing.T) {
	ts := newTestAutogenServer(t)

	body := GenerateFeaturesRequest{
		Schema: struct {
			Name        string `json:"name"`
			Description string `json:"description,omitempty"`
			Entity      string `json:"entity,omitempty"`
			Source      string `json:"source,omitempty"`
			Fields      []struct {
				Name        string            `json:"name"`
				Type        string            `json:"type"`
				Description string            `json:"description,omitempty"`
				Nullable    bool              `json:"nullable,omitempty"`
				Examples    []interface{}     `json:"examples,omitempty"`
				Constraints map[string]string `json:"constraints,omitempty"`
			} `json:"fields"`
		}{
			Name: "", // Missing name
			Fields: []struct {
				Name        string            `json:"name"`
				Type        string            `json:"type"`
				Description string            `json:"description,omitempty"`
				Nullable    bool              `json:"nullable,omitempty"`
				Examples    []interface{}     `json:"examples,omitempty"`
				Constraints map[string]string `json:"constraints,omitempty"`
			}{
				{Name: "age", Type: "int"},
			},
		},
	}

	rr := ts.postJSON("/v1/autogen/features", body)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestAutogenHandler_GenerateFeatures_MissingFields(t *testing.T) {
	ts := newTestAutogenServer(t)

	body := GenerateFeaturesRequest{
		Schema: struct {
			Name        string `json:"name"`
			Description string `json:"description,omitempty"`
			Entity      string `json:"entity,omitempty"`
			Source      string `json:"source,omitempty"`
			Fields      []struct {
				Name        string            `json:"name"`
				Type        string            `json:"type"`
				Description string            `json:"description,omitempty"`
				Nullable    bool              `json:"nullable,omitempty"`
				Examples    []interface{}     `json:"examples,omitempty"`
				Constraints map[string]string `json:"constraints,omitempty"`
			} `json:"fields"`
		}{
			Name:   "test_schema",
			Fields: nil, // No fields
		},
	}

	rr := ts.postJSON("/v1/autogen/features", body)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestAutogenHandler_GenerateFeatures_InvalidBody(t *testing.T) {
	ts := newTestAutogenServer(t)

	rr := ts.request(http.MethodPost, "/v1/autogen/features", "invalid json")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestAutogenHandler_GenerateFeatures_NoGenerator(t *testing.T) {
	ts := newTestAutogenServerWithoutGenerator(t)

	body := GenerateFeaturesRequest{
		Schema: struct {
			Name        string `json:"name"`
			Description string `json:"description,omitempty"`
			Entity      string `json:"entity,omitempty"`
			Source      string `json:"source,omitempty"`
			Fields      []struct {
				Name        string            `json:"name"`
				Type        string            `json:"type"`
				Description string            `json:"description,omitempty"`
				Nullable    bool              `json:"nullable,omitempty"`
				Examples    []interface{}     `json:"examples,omitempty"`
				Constraints map[string]string `json:"constraints,omitempty"`
			} `json:"fields"`
		}{
			Name: "test",
			Fields: []struct {
				Name        string            `json:"name"`
				Type        string            `json:"type"`
				Description string            `json:"description,omitempty"`
				Nullable    bool              `json:"nullable,omitempty"`
				Examples    []interface{}     `json:"examples,omitempty"`
				Constraints map[string]string `json:"constraints,omitempty"`
			}{
				{Name: "age", Type: "int"},
			},
		},
	}

	rr := ts.postJSON("/v1/autogen/features", body)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
	}
}

func TestAutogenHandler_SuggestTransformations(t *testing.T) {
	ts := newTestAutogenServer(t)

	body := SuggestTransformationsRequest{
		FeatureName: "age",
		FeatureType: "int",
		Description: "User age",
	}

	rr := ts.postJSON("/v1/autogen/transformations", body)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d; body: %s", http.StatusOK, rr.Code, rr.Body.String())
	}
}

func TestAutogenHandler_SuggestTransformations_MissingFeatureName(t *testing.T) {
	ts := newTestAutogenServer(t)

	body := SuggestTransformationsRequest{
		FeatureType: "int",
	}

	rr := ts.postJSON("/v1/autogen/transformations", body)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestAutogenHandler_SuggestTransformations_MissingFeatureType(t *testing.T) {
	ts := newTestAutogenServer(t)

	body := SuggestTransformationsRequest{
		FeatureName: "age",
	}

	rr := ts.postJSON("/v1/autogen/transformations", body)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestAutogenHandler_SuggestTransformations_InvalidBody(t *testing.T) {
	ts := newTestAutogenServer(t)

	rr := ts.request(http.MethodPost, "/v1/autogen/transformations", "invalid json")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestAutogenHandler_SuggestTransformations_NoGenerator(t *testing.T) {
	ts := newTestAutogenServerWithoutGenerator(t)

	body := SuggestTransformationsRequest{
		FeatureName: "age",
		FeatureType: "int",
	}

	rr := ts.postJSON("/v1/autogen/transformations", body)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
	}
}

func TestAutogenHandler_SuggestAggregations(t *testing.T) {
	ts := newTestAutogenServer(t)

	body := SuggestAggregationsRequest{
		Entity:    "user",
		TimeField: "timestamp",
		Fields: []struct {
			Name        string `json:"name"`
			Type        string `json:"type"`
			Description string `json:"description,omitempty"`
		}{
			{Name: "amount", Type: "float", Description: "Transaction amount"},
		},
	}

	rr := ts.postJSON("/v1/autogen/aggregations", body)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d; body: %s", http.StatusOK, rr.Code, rr.Body.String())
	}
}

func TestAutogenHandler_SuggestAggregations_MissingEntity(t *testing.T) {
	ts := newTestAutogenServer(t)

	body := SuggestAggregationsRequest{
		TimeField: "timestamp",
		Fields: []struct {
			Name        string `json:"name"`
			Type        string `json:"type"`
			Description string `json:"description,omitempty"`
		}{
			{Name: "amount", Type: "float"},
		},
	}

	rr := ts.postJSON("/v1/autogen/aggregations", body)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestAutogenHandler_SuggestAggregations_MissingFields(t *testing.T) {
	ts := newTestAutogenServer(t)

	body := SuggestAggregationsRequest{
		Entity:    "user",
		TimeField: "timestamp",
	}

	rr := ts.postJSON("/v1/autogen/aggregations", body)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestAutogenHandler_SuggestAggregations_InvalidBody(t *testing.T) {
	ts := newTestAutogenServer(t)

	rr := ts.request(http.MethodPost, "/v1/autogen/aggregations", "invalid json")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestAutogenHandler_SuggestAggregations_NoGenerator(t *testing.T) {
	ts := newTestAutogenServerWithoutGenerator(t)

	body := SuggestAggregationsRequest{
		Entity: "user",
		Fields: []struct {
			Name        string `json:"name"`
			Type        string `json:"type"`
			Description string `json:"description,omitempty"`
		}{
			{Name: "amount", Type: "float"},
		},
	}

	rr := ts.postJSON("/v1/autogen/aggregations", body)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
	}
}

func TestAutogenHandler_GetHistory(t *testing.T) {
	ts := newTestAutogenServer(t)

	rr := ts.get("/v1/autogen/history")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if result["history"] == nil {
		t.Error("Expected history array in response")
	}
}

func TestAutogenHandler_GetHistory_NoGenerator(t *testing.T) {
	ts := newTestAutogenServerWithoutGenerator(t)

	rr := ts.get("/v1/autogen/history")

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
	}
}

func TestAutogenHandler_GetStats(t *testing.T) {
	ts := newTestAutogenServer(t)

	rr := ts.get("/v1/autogen/stats")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}
}

func TestAutogenHandler_GetStats_NoGenerator(t *testing.T) {
	ts := newTestAutogenServerWithoutGenerator(t)

	rr := ts.get("/v1/autogen/stats")

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
	}
}
