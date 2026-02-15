package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/feather-store/feather/internal/extensions/llmgateway"
)

func setupLLMGatewayHandler(t *testing.T) (*LLMGatewayHandler, *http.ServeMux) {
	t.Helper()
	gw := llmgateway.NewGateway(llmgateway.DefaultGatewayConfig())
	handler := NewLLMGatewayHandler(gw)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	return handler, mux
}

func TestLLMGatewayHandler_GetStats(t *testing.T) {
	_, mux := setupLLMGatewayHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/llm/gateway/stats", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected %d, got %d", http.StatusOK, rr.Code)
	}
}

func TestLLMGatewayHandler_Lookup(t *testing.T) {
	_, mux := setupLLMGatewayHandler(t)
	body := `{"prompt":"hello","model":"gpt-4"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/llm/gateway/lookup", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected %d, got %d; body: %s", http.StatusOK, rr.Code, rr.Body.String())
	}
	var result map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
}

func TestLLMGatewayHandler_Lookup_InvalidJSON(t *testing.T) {
	_, mux := setupLLMGatewayHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/v1/llm/gateway/lookup", strings.NewReader("invalid"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected %d, got %d", http.StatusBadRequest, rr.Code)
	}
}
