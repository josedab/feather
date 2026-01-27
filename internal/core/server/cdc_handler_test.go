package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/feather-store/feather/internal/extensions/incrmat"
)

func newTestCDCHandler(t *testing.T) *http.ServeMux {
	t.Helper()
	engine := incrmat.NewEngine(incrmat.DefaultEngineConfig())
	handler := NewCDCHandler(engine)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	return mux
}

func TestCDCHandler_RegisterSource(t *testing.T) {
	mux := newTestCDCHandler(t)

	body := `{"id":"pg1","name":"Users DB","type":"postgresql","feature_group":"user_features","enabled":true}`
	req := httptest.NewRequest("POST", "/v1/cdc/sources", strings.NewReader(body))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Errorf("POST source = %d, want %d; body: %s", rr.Code, http.StatusCreated, rr.Body.String())
	}
}

func TestCDCHandler_ListSources(t *testing.T) {
	mux := newTestCDCHandler(t)

	req := httptest.NewRequest("GET", "/v1/cdc/sources", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("GET sources = %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestCDCHandler_ProcessEvent(t *testing.T) {
	mux := newTestCDCHandler(t)

	// Register source first
	body := `{"id":"s1","name":"S1","type":"generic","feature_group":"fg1","enabled":true}`
	req := httptest.NewRequest("POST", "/v1/cdc/sources", strings.NewReader(body))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	// Process event
	body = `{"source_id":"s1","operation":"UPDATE","entity_id":"e1","after":{"age":30}}`
	req = httptest.NewRequest("POST", "/v1/cdc/events", strings.NewReader(body))
	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("POST event = %d, want %d; body: %s", rr.Code, http.StatusOK, rr.Body.String())
	}
}

func TestCDCHandler_Stats(t *testing.T) {
	mux := newTestCDCHandler(t)

	req := httptest.NewRequest("GET", "/v1/cdc/stats", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("GET stats = %d, want %d", rr.Code, http.StatusOK)
	}
}
