package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/feather-store/feather/internal/extensions/versioning"
)

func setupVersioningHandler(t *testing.T) *http.ServeMux {
	t.Helper()
	handler := NewVersioningHandler(versioning.NewVersionStore())
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	return mux
}

func TestVersioning_GetListBranches(t *testing.T) {
	mux := setupVersioningHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/versioning/branches", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestVersioning_PostCreateBranch(t *testing.T) {
	mux := setupVersioningHandler(t)
	body := `{"name":"feature-branch","base_branch":"main"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/versioning/branches", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK && rr.Code != http.StatusCreated {
		t.Errorf("expected 200 or 201, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestVersioning_PostInvalidJSON(t *testing.T) {
	mux := setupVersioningHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/v1/versioning/branches", io.NopCloser(strings.NewReader("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}
