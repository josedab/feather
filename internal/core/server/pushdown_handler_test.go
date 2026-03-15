package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/feather-store/feather/internal/platform/pushdown"
)

func setupPushdownHandler(t *testing.T) *http.ServeMux {
	t.Helper()
	handler := NewPushdownHandler(pushdown.NewEvaluator())
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	return mux
}

func TestPushdown_GetListDerived(t *testing.T) {
	mux := setupPushdownHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/pushdown/derived", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestPushdown_PostRegisterDerived(t *testing.T) {
	mux := setupPushdownHandler(t)
	body := `{"name":"derived1","expression":"a + b","inputs":["a","b"]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/pushdown/derived", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK && rr.Code != http.StatusCreated {
		t.Errorf("expected 200 or 201, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestPushdown_PostInvalidJSON(t *testing.T) {
	mux := setupPushdownHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/v1/pushdown/derived", io.NopCloser(strings.NewReader("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}
