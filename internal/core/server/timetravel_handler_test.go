package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/feather-store/feather/internal/extensions/timetravel"
)

func setupTimeTravelHandler(t *testing.T) *http.ServeMux {
	t.Helper()
	handler := NewTimeTravelHandler(timetravel.NewDebugger(timetravel.DefaultDebuggerConfig()))
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	return mux
}

func TestTimeTravel_GetListSessions(t *testing.T) {
	mux := setupTimeTravelHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/timetravel/sessions", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestTimeTravel_PostCreateSession(t *testing.T) {
	mux := setupTimeTravelHandler(t)
	body := `{"id":"session1","entity_key":"user:123","features":["age","score"]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/timetravel/sessions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK && rr.Code != http.StatusCreated {
		t.Errorf("expected 200 or 201, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestTimeTravel_PostInvalidJSON(t *testing.T) {
	mux := setupTimeTravelHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/v1/timetravel/sessions", io.NopCloser(strings.NewReader("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}
