package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/feather-store/feather/internal/tools/mcp"
)

func TestMCPServerHandler_Info(t *testing.T) {
	info := mcp.GetServerInfo()
	handler := NewMCPServerHandler(info, mcp.BuiltinResources(), mcp.BuiltinPrompts())

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	req := httptest.NewRequest("GET", "/v1/mcp/info", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestMCPServerHandler_Capabilities(t *testing.T) {
	info := mcp.GetServerInfo()
	handler := NewMCPServerHandler(info, mcp.BuiltinResources(), mcp.BuiltinPrompts())

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	req := httptest.NewRequest("GET", "/v1/mcp/capabilities", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}
