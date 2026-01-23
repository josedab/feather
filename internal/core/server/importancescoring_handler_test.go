package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/feather-store/feather/internal/extensions/importancescoring"
)

type testImportanceScoringServer struct {
	handler *ImportanceScoringHandler
	scorer  *importancescoring.Scorer
	mux     *http.ServeMux
	t       *testing.T
}

func newTestImportanceScoringServer(t *testing.T) *testImportanceScoringServer {
	t.Helper()

	scorer := importancescoring.NewScorer(importancescoring.DefaultScorerConfig())
	handler := NewImportanceScoringHandler(scorer)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	return &testImportanceScoringServer{
		handler: handler,
		scorer:  scorer,
		mux:     mux,
		t:       t,
	}
}

func (ts *testImportanceScoringServer) request(method, path string, body string) *httptest.ResponseRecorder {
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

func (ts *testImportanceScoringServer) postJSON(path string, body interface{}) *httptest.ResponseRecorder {
	ts.t.Helper()

	jsonBody, err := json.Marshal(body)
	if err != nil {
		ts.t.Fatalf("failed to marshal body: %v", err)
	}
	return ts.request(http.MethodPost, path, string(jsonBody))
}

func (ts *testImportanceScoringServer) get(path string) *httptest.ResponseRecorder {
	return ts.request(http.MethodGet, path, "")
}

func TestImportanceScoringHandler_NewHandler(t *testing.T) {
	scorer := importancescoring.NewScorer(importancescoring.DefaultScorerConfig())
	handler := NewImportanceScoringHandler(scorer)

	if handler.scorer == nil {
		t.Error("Expected scorer to be set")
	}
}

func TestImportanceScoringHandler_RecordAccess(t *testing.T) {
	ts := newTestImportanceScoringServer(t)

	body := map[string]interface{}{
		"name":  "click_rate",
		"value": 1.0,
	}

	rr := ts.postJSON("/v1/importance/record", body)

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

func TestImportanceScoringHandler_RecordAccess_MissingName(t *testing.T) {
	ts := newTestImportanceScoringServer(t)

	body := map[string]interface{}{
		"value": 1.0,
	}

	rr := ts.postJSON("/v1/importance/record", body)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestImportanceScoringHandler_RecordAccess_InvalidBody(t *testing.T) {
	ts := newTestImportanceScoringServer(t)

	rr := ts.request(http.MethodPost, "/v1/importance/record", "invalid json")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestImportanceScoringHandler_ScoreAll(t *testing.T) {
	ts := newTestImportanceScoringServer(t)

	rr := ts.postJSON("/v1/importance/score", nil)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}
}

func TestImportanceScoringHandler_GetScore_NotFound(t *testing.T) {
	ts := newTestImportanceScoringServer(t)

	rr := ts.get("/v1/importance/scores/nonexistent")

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, rr.Code)
	}
}

func TestImportanceScoringHandler_GetTopK(t *testing.T) {
	ts := newTestImportanceScoringServer(t)

	rr := ts.get("/v1/importance/top?k=5")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}
}

func TestImportanceScoringHandler_GetBottomK(t *testing.T) {
	ts := newTestImportanceScoringServer(t)

	rr := ts.get("/v1/importance/bottom?k=5")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}
}

func TestImportanceScoringHandler_GetDeprecationCandidates(t *testing.T) {
	ts := newTestImportanceScoringServer(t)

	rr := ts.get("/v1/importance/deprecation")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}
}

func TestImportanceScoringHandler_GetStats(t *testing.T) {
	ts := newTestImportanceScoringServer(t)

	rr := ts.get("/v1/importance/stats")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}
}
