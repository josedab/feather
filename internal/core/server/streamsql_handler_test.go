package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/feather-store/feather/internal/integrations/streamsql"
)

func setupStreamSQLHandler(t *testing.T) *http.ServeMux {
	t.Helper()
	handler := NewStreamSQLHandler(streamsql.NewEngine(streamsql.DefaultEngineConfig()))
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	return mux
}

func TestStreamSQL_GetListStreams(t *testing.T) {
	mux := setupStreamSQLHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/sql/streams", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestStreamSQL_PostCreateStream(t *testing.T) {
	mux := setupStreamSQLHandler(t)
	body := `{"name":"events","schema":{"user_id":"string","amount":"float"}}`
	req := httptest.NewRequest(http.MethodPost, "/v1/sql/streams", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK && rr.Code != http.StatusCreated {
		t.Errorf("expected 200 or 201, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestStreamSQL_PostInvalidJSON(t *testing.T) {
	mux := setupStreamSQLHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/v1/sql/streams", io.NopCloser(strings.NewReader("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}
