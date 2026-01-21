package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/feather-store/feather/internal/extensions/activeactive"
)

func setupActiveActiveHandler(t *testing.T) *http.ServeMux {
	t.Helper()
	replicator := activeactive.NewReplicator(activeactive.DefaultReplicatorConfig())
	handler := NewActiveActiveHandler(replicator)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	return mux
}

func TestActiveActive_GetStats(t *testing.T) {
	mux := setupActiveActiveHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/activeactive/stats", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestActiveActive_PostAddPeer(t *testing.T) {
	mux := setupActiveActiveHandler(t)
	body := `{"id":"peer1","address":"localhost:9000"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/activeactive/peers", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK && rr.Code != http.StatusCreated {
		t.Errorf("expected 200 or 201, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestActiveActive_PostInvalidJSON(t *testing.T) {
	mux := setupActiveActiveHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/v1/activeactive/peers", io.NopCloser(strings.NewReader("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}
