package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/feather-store/feather/internal/extensions/schemaevolution"
)

func newTestSchemaEvolutionHandler(t *testing.T) (*http.ServeMux, *schemaevolution.Manager) {
	t.Helper()
	mgr := schemaevolution.NewManager(schemaevolution.DefaultManagerConfig())
	handler := NewSchemaEvolutionHandler(mgr)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	return mux, mgr
}

func TestSchemaEvolutionHandler_ListSchemas(t *testing.T) {
	mux, _ := newTestSchemaEvolutionHandler(t)

	req := httptest.NewRequest("GET", "/v1/schema/evolution/groups", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("GET /v1/schema/evolution/groups = %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestSchemaEvolutionHandler_RegisterSchema(t *testing.T) {
	mux, _ := newTestSchemaEvolutionHandler(t)

	body := `{"group":"user_features","fields":{"age":"int64","name":"string"}}`
	req := httptest.NewRequest("POST", "/v1/schema/evolution/groups", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated && rr.Code != http.StatusOK {
		t.Errorf("POST /v1/schema/evolution/groups = %d, want 200 or 201; body: %s", rr.Code, rr.Body.String())
	}
}

func TestSchemaEvolutionHandler_InvalidJSON(t *testing.T) {
	mux, _ := newTestSchemaEvolutionHandler(t)

	req := httptest.NewRequest("POST", "/v1/schema/evolution/groups", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("POST with bad JSON = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestSchemaEvolutionHandler_GetSchemaNotFound(t *testing.T) {
	mux, _ := newTestSchemaEvolutionHandler(t)

	req := httptest.NewRequest("GET", "/v1/schema/evolution/groups/nonexistent", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Errorf("GET /v1/schema/evolution/groups/nonexistent = %d, want %d", rr.Code, http.StatusNotFound)
	}
}
