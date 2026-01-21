package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/feather-store/feather/internal/platform/replication"
)

func setupReplicationHandler(t *testing.T) *http.ServeMux {
	t.Helper()
	handler := NewReplicationHandler(replication.NewManager(replication.DefaultManagerConfig()))
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	return mux
}

func TestReplication_GetListRegions(t *testing.T) {
	mux := setupReplicationHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/replication/regions", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestReplication_PostAddRegion(t *testing.T) {
	mux := setupReplicationHandler(t)
	body := `{"id":"us-east-1","endpoint":"https://us-east-1.example.com"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/replication/regions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK && rr.Code != http.StatusCreated {
		t.Errorf("expected 200 or 201, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestReplication_PostInvalidJSON(t *testing.T) {
	mux := setupReplicationHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/v1/replication/regions", io.NopCloser(strings.NewReader("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}
