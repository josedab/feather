package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/feather-store/feather/internal/extensions/ftl"
)

func setupFTLHandler(t *testing.T) (*FTLHandler, *http.ServeMux) {
	t.Helper()
	compiler := ftl.NewCompiler(ftl.DefaultCompilerConfig())
	handler := NewFTLHandler(compiler)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	return handler, mux
}

func TestFTLHandler_Stats(t *testing.T) {
	_, mux := setupFTLHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/ftl/stats", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected %d, got %d", http.StatusOK, rr.Code)
	}
}

func TestFTLHandler_Tokenize(t *testing.T) {
	_, mux := setupFTLHandler(t)
	body := `{"query":"SELECT a FROM b"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/ftl/tokenize", strings.NewReader(body))
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

func TestFTLHandler_Tokenize_InvalidJSON(t *testing.T) {
	_, mux := setupFTLHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/v1/ftl/tokenize", strings.NewReader("invalid"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected %d, got %d", http.StatusBadRequest, rr.Code)
	}
}
