package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/feather-store/feather/internal/extensions/webhooks"
)

type testWebhooksServer struct {
	handler    *WebhooksHandler
	dispatcher *webhooks.Dispatcher
	mux        *http.ServeMux
	t          *testing.T
}

func newTestWebhooksServer(t *testing.T) *testWebhooksServer {
	t.Helper()

	dispatcher := webhooks.NewDispatcher(webhooks.DefaultDispatcherConfig())
	handler := NewWebhooksHandler(dispatcher)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	return &testWebhooksServer{
		handler:    handler,
		dispatcher: dispatcher,
		mux:        mux,
		t:          t,
	}
}

func (ts *testWebhooksServer) request(method, path string, body string) *httptest.ResponseRecorder {
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

func (ts *testWebhooksServer) postJSON(path string, body interface{}) *httptest.ResponseRecorder {
	ts.t.Helper()

	jsonBody, err := json.Marshal(body)
	if err != nil {
		ts.t.Fatalf("failed to marshal body: %v", err)
	}
	return ts.request(http.MethodPost, path, string(jsonBody))
}

func (ts *testWebhooksServer) get(path string) *httptest.ResponseRecorder {
	return ts.request(http.MethodGet, path, "")
}

func TestWebhooksHandler_NewHandler(t *testing.T) {
	dispatcher := webhooks.NewDispatcher(webhooks.DefaultDispatcherConfig())
	handler := NewWebhooksHandler(dispatcher)

	if handler.dispatcher == nil {
		t.Error("Expected dispatcher to be set")
	}
}

func TestWebhooksHandler_ListWebhooks_Empty(t *testing.T) {
	ts := newTestWebhooksServer(t)

	rr := ts.get("/v1/webhooks")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if result["webhooks"] == nil {
		t.Error("Expected webhooks array in response")
	}
}

func TestWebhooksHandler_RegisterWebhook(t *testing.T) {
	ts := newTestWebhooksServer(t)

	wh := webhooks.WebhookConfig{
		ID:     "wh-1",
		Name:   "test-webhook",
		URL:    "http://example.com/hook",
		Events: []webhooks.EventType{"feature.created"},
		Active: true,
	}

	rr := ts.postJSON("/v1/webhooks", wh)

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

func TestWebhooksHandler_RegisterWebhook_Duplicate(t *testing.T) {
	ts := newTestWebhooksServer(t)

	wh := webhooks.WebhookConfig{
		ID:     "wh-1",
		Name:   "test-webhook",
		URL:    "http://example.com/hook",
		Events: []webhooks.EventType{"feature.created"},
		Active: true,
	}

	ts.dispatcher.RegisterWebhook(wh)

	rr := ts.postJSON("/v1/webhooks", wh)

	if rr.Code != http.StatusConflict {
		t.Errorf("Expected status %d, got %d", http.StatusConflict, rr.Code)
	}
}

func TestWebhooksHandler_RegisterWebhook_InvalidBody(t *testing.T) {
	ts := newTestWebhooksServer(t)

	rr := ts.request(http.MethodPost, "/v1/webhooks", "invalid json")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestWebhooksHandler_GetWebhook(t *testing.T) {
	ts := newTestWebhooksServer(t)

	wh := webhooks.WebhookConfig{ID: "wh-1", Name: "test", URL: "http://example.com/hook", Active: true}
	ts.dispatcher.RegisterWebhook(wh)

	rr := ts.get("/v1/webhooks/wh-1")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}
}

func TestWebhooksHandler_GetWebhook_NotFound(t *testing.T) {
	ts := newTestWebhooksServer(t)

	rr := ts.get("/v1/webhooks/nonexistent")

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, rr.Code)
	}
}

func TestWebhooksHandler_UpdateWebhook(t *testing.T) {
	ts := newTestWebhooksServer(t)

	wh := webhooks.WebhookConfig{ID: "wh-1", Name: "test", URL: "http://example.com/hook", Active: true}
	ts.dispatcher.RegisterWebhook(wh)

	updated := webhooks.WebhookConfig{ID: "wh-1", Name: "updated", URL: "http://example.com/hook2", Active: true}
	body, _ := json.Marshal(updated)
	rr := ts.request(http.MethodPut, "/v1/webhooks/wh-1", string(body))

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

func TestWebhooksHandler_UpdateWebhook_NotFound(t *testing.T) {
	ts := newTestWebhooksServer(t)

	updated := webhooks.WebhookConfig{ID: "nonexistent", Name: "updated", URL: "http://example.com/hook2"}
	body, _ := json.Marshal(updated)
	rr := ts.request(http.MethodPut, "/v1/webhooks/nonexistent", string(body))

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, rr.Code)
	}
}

func TestWebhooksHandler_DeleteWebhook(t *testing.T) {
	ts := newTestWebhooksServer(t)

	wh := webhooks.WebhookConfig{ID: "wh-1", Name: "test", URL: "http://example.com/hook", Active: true}
	ts.dispatcher.RegisterWebhook(wh)

	rr := ts.request(http.MethodDelete, "/v1/webhooks/wh-1", "")

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

func TestWebhooksHandler_DeleteWebhook_NotFound(t *testing.T) {
	ts := newTestWebhooksServer(t)

	rr := ts.request(http.MethodDelete, "/v1/webhooks/nonexistent", "")

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, rr.Code)
	}
}

func TestWebhooksHandler_Dispatch(t *testing.T) {
	ts := newTestWebhooksServer(t)

	event := webhooks.Event{
		ID:   "evt-1",
		Type: "feature.created",
	}

	rr := ts.postJSON("/v1/webhooks/dispatch", event)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}
}

func TestWebhooksHandler_Dispatch_InvalidBody(t *testing.T) {
	ts := newTestWebhooksServer(t)

	rr := ts.request(http.MethodPost, "/v1/webhooks/dispatch", "invalid json")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestWebhooksHandler_GetDeadLetter(t *testing.T) {
	ts := newTestWebhooksServer(t)

	rr := ts.get("/v1/webhooks/dead-letter?limit=10")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}
}

func TestWebhooksHandler_RetryDeadLetter(t *testing.T) {
	ts := newTestWebhooksServer(t)

	rr := ts.postJSON("/v1/webhooks/dead-letter/retry", nil)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}
}

func TestWebhooksHandler_GetStats(t *testing.T) {
	ts := newTestWebhooksServer(t)

	rr := ts.get("/v1/webhooks/stats")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}
}
