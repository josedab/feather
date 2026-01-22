package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/feather-store/feather/internal/extensions/feastcompat"
)

func newTestFeastCompatHandler(t *testing.T) (*http.ServeMux, *feastcompat.Adapter) {
	t.Helper()
	adapter := feastcompat.NewAdapter(feastcompat.DefaultAdapterConfig())
	handler := NewFeastCompatHandler(adapter)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	return mux, adapter
}

func TestFeastCompatHandler_ListMappings(t *testing.T) {
	mux, _ := newTestFeastCompatHandler(t)

	req := httptest.NewRequest("GET", "/v1/feast/mappings", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("GET /v1/feast/mappings = %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestFeastCompatHandler_RegisterMapping(t *testing.T) {
	mux, _ := newTestFeastCompatHandler(t)

	body := `{"feast_view":"user_features","feather_group":"users","feature_mapping":{"age":"user_age"}}`
	req := httptest.NewRequest("POST", "/v1/feast/mappings", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated && rr.Code != http.StatusOK {
		t.Errorf("POST /v1/feast/mappings = %d, want 200 or 201; body: %s", rr.Code, rr.Body.String())
	}
}

func TestFeastCompatHandler_InvalidJSON(t *testing.T) {
	mux, _ := newTestFeastCompatHandler(t)

	req := httptest.NewRequest("POST", "/v1/feast/mappings", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("POST with bad JSON = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestFeastCompatHandler_GetMappingNotFound(t *testing.T) {
	mux, _ := newTestFeastCompatHandler(t)

	req := httptest.NewRequest("GET", "/v1/feast/mappings/nonexistent", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Errorf("GET /v1/feast/mappings/nonexistent = %d, want %d", rr.Code, http.StatusNotFound)
	}
}
