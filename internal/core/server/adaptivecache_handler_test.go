package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/feather-store/feather/internal/extensions/adaptivecache"
)

type testAdaptiveCacheServer struct {
	handler   *AdaptiveCacheHandler
	predictor *adaptivecache.Predictor
	mux       *http.ServeMux
	t         *testing.T
}

func newTestAdaptiveCacheServer(t *testing.T) *testAdaptiveCacheServer {
	t.Helper()
	predictor := adaptivecache.NewPredictor(adaptivecache.DefaultPredictorConfig())
	handler := NewAdaptiveCacheHandler(predictor)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	return &testAdaptiveCacheServer{handler: handler, predictor: predictor, mux: mux, t: t}
}

func (ts *testAdaptiveCacheServer) request(method, path string, body string) *httptest.ResponseRecorder {
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

func TestAdaptiveCacheHandler_GetStats(t *testing.T) {
	ts := newTestAdaptiveCacheServer(t)
	rr := ts.request(http.MethodGet, "/v1/cache/adaptive/stats", "")
	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}
	var result map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}
}

func TestAdaptiveCacheHandler_RecordAccess(t *testing.T) {
	ts := newTestAdaptiveCacheServer(t)
	body := `{"key":"user:123"}`
	rr := ts.request(http.MethodPost, "/v1/cache/record", body)
	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d; body: %s", http.StatusOK, rr.Code, rr.Body.String())
	}
	var result map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}
	if result["success"] != true {
		t.Error("Expected success to be true")
	}
}

func TestAdaptiveCacheHandler_RecordAccess_InvalidJSON(t *testing.T) {
	ts := newTestAdaptiveCacheServer(t)
	rr := ts.request(http.MethodPost, "/v1/cache/record", "not json")
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestAdaptiveCacheHandler_GetPredictions(t *testing.T) {
	ts := newTestAdaptiveCacheServer(t)
	rr := ts.request(http.MethodGet, "/v1/cache/predictions?top_k=5", "")
	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}
	var result map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}
}
