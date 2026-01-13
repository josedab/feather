package server

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// testStreamingServer wraps a StreamingHandler for testing.
type testStreamingServer struct {
	handler *StreamingHandler
	mux     *http.ServeMux
	t       *testing.T
}

// newTestStreamingServer creates a new test streaming server.
func newTestStreamingServer(t *testing.T) *testStreamingServer {
	t.Helper()

	handler := NewStreamingHandler(context.Background()) // Creates internal hub
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	return &testStreamingServer{
		handler: handler,
		mux:     mux,
		t:       t,
	}
}

func (ts *testStreamingServer) request(method, path string, body string) *httptest.ResponseRecorder {
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

func (ts *testStreamingServer) postJSON(path string, body interface{}) *httptest.ResponseRecorder {
	ts.t.Helper()

	jsonBody, err := json.Marshal(body)
	if err != nil {
		ts.t.Fatalf("failed to marshal body: %v", err)
	}
	return ts.request(http.MethodPost, path, string(jsonBody))
}

func (ts *testStreamingServer) get(path string) *httptest.ResponseRecorder {
	return ts.request(http.MethodGet, path, "")
}

func (ts *testStreamingServer) delete(path string) *httptest.ResponseRecorder {
	return ts.request(http.MethodDelete, path, "")
}

func TestStreamingHandler_NewStreamingHandler(t *testing.T) {
	handler := NewStreamingHandler(context.Background())

	if handler == nil {
		t.Error("Expected handler to be created")
	}
}

func TestStreamingHandler_GetHub(t *testing.T) {
	handler := NewStreamingHandler(context.Background())

	hub := handler.GetHub()
	if hub == nil {
		t.Error("Expected GetHub to return non-nil hub")
	}
}

func TestStreamingHandler_Subscribe(t *testing.T) {
	ts := newTestStreamingServer(t)

	body := StreamSubscribeRequest{
		ClientID:   "test-client",
		Features:   []string{"feature-1", "feature-2"},
		EventTypes: []string{"update", "delete"},
	}

	rr := ts.postJSON("/v1/stream/subscribe", body)

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

	if result["subscription_id"] == nil {
		t.Error("Expected subscription_id in response")
	}
}

func TestStreamingHandler_Subscribe_InvalidBody(t *testing.T) {
	ts := newTestStreamingServer(t)

	rr := ts.request(http.MethodPost, "/v1/stream/subscribe", "invalid json")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestStreamingHandler_Unsubscribe(t *testing.T) {
	ts := newTestStreamingServer(t)

	// First create a subscription
	body := StreamSubscribeRequest{
		ClientID: "test-client",
		Features: []string{"feature-1"},
	}
	subRR := ts.postJSON("/v1/stream/subscribe", body)
	var subResult map[string]interface{}
	json.Unmarshal(subRR.Body.Bytes(), &subResult)
	subID := subResult["subscription_id"].(string)

	// Now unsubscribe
	rr := ts.delete("/v1/stream/subscribe/" + subID)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d; body: %s", http.StatusOK, rr.Code, rr.Body.String())
	}
}

func TestStreamingHandler_Unsubscribe_MissingID(t *testing.T) {
	ts := newTestStreamingServer(t)

	// Empty ID should return 404 (no matching route) or bad request
	rr := ts.delete("/v1/stream/subscribe/")

	// Route won't match with empty ID, expect 404
	if rr.Code != http.StatusNotFound && rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status 404 or 400, got %d", rr.Code)
	}
}

func TestStreamingHandler_ListSubscriptions(t *testing.T) {
	ts := newTestStreamingServer(t)

	rr := ts.get("/v1/stream/subscriptions")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if result["subscriptions"] == nil {
		t.Error("Expected subscriptions in response")
	}
}

func TestStreamingHandler_Publish(t *testing.T) {
	ts := newTestStreamingServer(t)

	body := StreamPublishRequest{
		Type:     "update",
		EntityID: "user-123",
		Feature:  "test-feature",
		Value:    1.5,
		Metadata: map[string]interface{}{"source": "test"},
	}

	rr := ts.postJSON("/v1/stream/publish", body)

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

func TestStreamingHandler_Publish_InvalidBody(t *testing.T) {
	ts := newTestStreamingServer(t)

	rr := ts.request(http.MethodPost, "/v1/stream/publish", "invalid json")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestStreamingHandler_Publish_MissingType(t *testing.T) {
	ts := newTestStreamingServer(t)

	body := StreamPublishRequest{
		EntityID: "user-123",
		Feature:  "test-feature",
	}

	rr := ts.postJSON("/v1/stream/publish", body)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestStreamingHandler_GetStats(t *testing.T) {
	ts := newTestStreamingServer(t)

	rr := ts.get("/v1/stream/stats")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}
}
