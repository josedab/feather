package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/feather-store/feather/internal/tools/dashboard"
)

func setupExplorerHandler(t *testing.T) *http.ServeMux {
	t.Helper()
	explorer := dashboard.NewExplorer(dashboard.DefaultExplorerConfig())
	handler := NewExplorerHandler(explorer)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	return mux
}

func TestExplorer_GetStats(t *testing.T) {
	mux := setupExplorerHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/explorer/stats", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestExplorer_PostRecordInsight(t *testing.T) {
	mux := setupExplorerHandler(t)
	body := `{"feature_id":"feat1","entity_count":100}`
	req := httptest.NewRequest(http.MethodPost, "/v1/explorer/insights", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK && rr.Code != http.StatusCreated {
		t.Errorf("expected 200 or 201, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestExplorer_PostInvalidJSON(t *testing.T) {
	mux := setupExplorerHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/v1/explorer/insights", io.NopCloser(strings.NewReader("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}
