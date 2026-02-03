package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/feather-store/feather/internal/extensions/diffprivacy"
)

type testDiffPrivacyServer struct {
	handler *DiffPrivacyHandler
	engine  *diffprivacy.Engine
	mux     *http.ServeMux
	t       *testing.T
}

func newTestDiffPrivacyServer(t *testing.T) *testDiffPrivacyServer {
	t.Helper()
	engine := diffprivacy.NewEngine(diffprivacy.DefaultConfig())
	handler := NewDiffPrivacyHandler(engine)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	return &testDiffPrivacyServer{handler: handler, engine: engine, mux: mux, t: t}
}

func (ts *testDiffPrivacyServer) request(method, path string, body string) *httptest.ResponseRecorder {
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

func TestDiffPrivacyHandler_GetStats(t *testing.T) {
	ts := newTestDiffPrivacyServer(t)
	rr := ts.request(http.MethodGet, "/v1/privacy/stats", "")
	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}
	var result map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}
}

func TestDiffPrivacyHandler_Register(t *testing.T) {
	ts := newTestDiffPrivacyServer(t)
	body := `{"name":"click_count","config":{"epsilon":1.0,"delta":0.00001,"sensitivity":1.0,"mechanism":"laplace"}}`
	rr := ts.request(http.MethodPost, "/v1/privacy/register", body)
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

func TestDiffPrivacyHandler_Register_InvalidJSON(t *testing.T) {
	ts := newTestDiffPrivacyServer(t)
	rr := ts.request(http.MethodPost, "/v1/privacy/register", "not json")
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestDiffPrivacyHandler_Register_MissingName(t *testing.T) {
	ts := newTestDiffPrivacyServer(t)
	body := `{"name":"","config":{}}`
	rr := ts.request(http.MethodPost, "/v1/privacy/register", body)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestDiffPrivacyHandler_AddNoise_MissingFeature(t *testing.T) {
	ts := newTestDiffPrivacyServer(t)
	body := `{"feature":"","value":42.0}`
	rr := ts.request(http.MethodPost, "/v1/privacy/noise", body)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestDiffPrivacyHandler_Aggregate_InvalidJSON(t *testing.T) {
	ts := newTestDiffPrivacyServer(t)
	rr := ts.request(http.MethodPost, "/v1/privacy/aggregate", "not json")
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}
