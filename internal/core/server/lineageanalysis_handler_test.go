package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/feather-store/feather/internal/extensions/lineageanalysis"
)

func newTestLineageAnalysisHandler(t *testing.T) *http.ServeMux {
	t.Helper()
	tracker := lineageanalysis.NewTracker(lineageanalysis.DefaultTrackerConfig())
	handler := NewLineageAnalysisHandler(tracker)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	return mux
}

func TestLineageAnalysisHandler_AddNode(t *testing.T) {
	mux := newTestLineageAnalysisHandler(t)

	body := `{"id":"users_db","name":"Users Database","type":"source"}`
	req := httptest.NewRequest("POST", "/v1/lineage/nodes", strings.NewReader(body))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Errorf("POST node = %d, want %d; body: %s", rr.Code, http.StatusCreated, rr.Body.String())
	}
}

func TestLineageAnalysisHandler_ListNodes(t *testing.T) {
	mux := newTestLineageAnalysisHandler(t)

	req := httptest.NewRequest("GET", "/v1/lineage/nodes", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("GET nodes = %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestLineageAnalysisHandler_Impact(t *testing.T) {
	mux := newTestLineageAnalysisHandler(t)

	// Add nodes and edges
	for _, body := range []string{
		`{"id":"db","name":"DB","type":"source"}`,
		`{"id":"feat","name":"Feature","type":"feature"}`,
	} {
		req := httptest.NewRequest("POST", "/v1/lineage/nodes", strings.NewReader(body))
		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, req)
	}

	body := `{"from_id":"db","to_id":"feat"}`
	req := httptest.NewRequest("POST", "/v1/lineage/edges", strings.NewReader(body))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Errorf("POST edge = %d, want %d; body: %s", rr.Code, http.StatusCreated, rr.Body.String())
	}

	// Analyze impact
	req = httptest.NewRequest("GET", "/v1/lineage/nodes/db/impact", nil)
	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("GET impact = %d, want %d; body: %s", rr.Code, http.StatusOK, rr.Body.String())
	}
}

func TestLineageAnalysisHandler_NodeNotFound(t *testing.T) {
	mux := newTestLineageAnalysisHandler(t)

	req := httptest.NewRequest("GET", "/v1/lineage/nodes/nonexistent", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Errorf("GET nonexistent = %d, want %d", rr.Code, http.StatusNotFound)
	}
}
