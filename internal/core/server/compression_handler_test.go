package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/feather-store/feather/internal/extensions/compression"
)

type testCompressionServer struct {
	handler  *CompressionHandler
	selector *compression.Selector
	mux      *http.ServeMux
	t        *testing.T
}

func newTestCompressionServer(t *testing.T) *testCompressionServer {
	t.Helper()
	selector := compression.NewSelector(compression.DefaultConfig())
	handler := NewCompressionHandler(selector)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	return &testCompressionServer{handler: handler, selector: selector, mux: mux, t: t}
}

func (ts *testCompressionServer) request(method, path string, body string) *httptest.ResponseRecorder {
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

func TestCompressionHandler_GetStats(t *testing.T) {
	ts := newTestCompressionServer(t)
	rr := ts.request(http.MethodGet, "/v1/compression/stats", "")
	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}
	var result map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}
}

func TestCompressionHandler_Analyze(t *testing.T) {
	ts := newTestCompressionServer(t)
	body := `{"data":"aGVsbG8=","data_type":"string"}`
	rr := ts.request(http.MethodPost, "/v1/compression/analyze", body)
	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d; body: %s", http.StatusOK, rr.Code, rr.Body.String())
	}
}

func TestCompressionHandler_Analyze_InvalidJSON(t *testing.T) {
	ts := newTestCompressionServer(t)
	rr := ts.request(http.MethodPost, "/v1/compression/analyze", "not json")
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestCompressionHandler_SelectStrategy(t *testing.T) {
	ts := newTestCompressionServer(t)
	body := `{"size":1024,"cardinality":100}`
	rr := ts.request(http.MethodPost, "/v1/compression/select", body)
	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d; body: %s", http.StatusOK, rr.Code, rr.Body.String())
	}
	var result map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}
	if _, ok := result["strategy"]; !ok {
		t.Error("Expected strategy key in response")
	}
}

func TestCompressionHandler_ShouldReEncode(t *testing.T) {
	ts := newTestCompressionServer(t)
	rr := ts.request(http.MethodGet, "/v1/compression/reencode/click_count", "")
	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}
	var result map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}
	if _, ok := result["feature"]; !ok {
		t.Error("Expected feature key in response")
	}
}

func TestCompressionHandler_Compress_InvalidJSON(t *testing.T) {
	ts := newTestCompressionServer(t)
	rr := ts.request(http.MethodPost, "/v1/compression/compress", "not json")
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}
