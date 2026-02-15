package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/feather-store/feather/internal/integrations/dbt"
)

func setupDBTHandler(t *testing.T) *http.ServeMux {
	t.Helper()
	handler := NewDBTHandler(&dbt.SyncOptions{DefaultEntityType: "user"})
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	return mux
}

func TestDBT_GetStatus(t *testing.T) {
	mux := setupDBTHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/dbt/status", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestDBT_PostValidate(t *testing.T) {
	mux := setupDBTHandler(t)
	manifest := `{"metadata":{"dbt_schema_version":"https://schemas.getdbt.com/dbt/manifest/v1.json"},"nodes":{}}`
	body := `{"manifest":` + manifest + `}`
	req := httptest.NewRequest(http.MethodPost, "/v1/dbt/validate", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK && rr.Code != http.StatusCreated {
		t.Errorf("expected 200 or 201, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestDBT_PostInvalidJSON(t *testing.T) {
	mux := setupDBTHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/v1/dbt/validate", io.NopCloser(strings.NewReader("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}
