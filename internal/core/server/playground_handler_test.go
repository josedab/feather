package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/feather-store/feather/internal/tools/playground"
)

func setupPlaygroundHandler(t *testing.T) *http.ServeMux {
	t.Helper()
	handler := NewPlaygroundHandler(playground.NewService(nil))
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	return mux
}

func TestPlayground_GetListQueries(t *testing.T) {
	mux := setupPlaygroundHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/playground/queries", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestPlayground_PostSaveQuery(t *testing.T) {
	mux := setupPlaygroundHandler(t)
	body := `{"name":"test-query","features":["age","score"]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/playground/queries", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK && rr.Code != http.StatusCreated {
		t.Errorf("expected 200 or 201, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestPlayground_PostInvalidJSON(t *testing.T) {
	mux := setupPlaygroundHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/v1/playground/queries", io.NopCloser(strings.NewReader("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}
