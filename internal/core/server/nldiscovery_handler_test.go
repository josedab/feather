package server

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/feather-store/feather/internal/extensions/semantic"
)

func setupNLDiscoveryHandler(t *testing.T) *http.ServeMux {
	t.Helper()
	search := semantic.NewSearch(nil, slog.Default())
	engine := semantic.NewNLDiscoveryEngine(search)
	handler := NewNLDiscoveryHandler(engine)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	return mux
}

func TestNLDiscovery_Query(t *testing.T) {
	mux := setupNLDiscoveryHandler(t)
	body := `{"query":"find user features"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/discover/query", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestNLDiscovery_QueryEmpty(t *testing.T) {
	mux := setupNLDiscoveryHandler(t)
	body := `{"query":""}`
	req := httptest.NewRequest(http.MethodPost, "/v1/discover/query", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestNLDiscovery_QueryInvalidJSON(t *testing.T) {
	mux := setupNLDiscoveryHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/v1/discover/query", io.NopCloser(strings.NewReader("invalid")))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestNLDiscovery_Chat(t *testing.T) {
	mux := setupNLDiscoveryHandler(t)
	body := `{"conversation_id":"conv1","message":"what features exist?"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/discover/chat", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestNLDiscovery_ChatEmptyMessage(t *testing.T) {
	mux := setupNLDiscoveryHandler(t)
	body := `{"conversation_id":"conv1","message":""}`
	req := httptest.NewRequest(http.MethodPost, "/v1/discover/chat", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestNLDiscovery_ChatInvalidJSON(t *testing.T) {
	mux := setupNLDiscoveryHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/v1/discover/chat", strings.NewReader("bad"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestNLDiscovery_GetConversationNotFound(t *testing.T) {
	mux := setupNLDiscoveryHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/discover/conversations/nonexistent", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestNLDiscovery_History(t *testing.T) {
	mux := setupNLDiscoveryHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/discover/history?limit=5", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestNLDiscovery_RegisterFeature(t *testing.T) {
	mux := setupNLDiscoveryHandler(t)
	body := `{"name":"user_age","description":"User age in years","entity_type":"user","tags":["demographics"]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/discover/catalog", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestNLDiscovery_RegisterFeatureMissingName(t *testing.T) {
	mux := setupNLDiscoveryHandler(t)
	body := `{"description":"some desc","entity_type":"user"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/discover/catalog", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestNLDiscovery_RegisterFeatureInvalidJSON(t *testing.T) {
	mux := setupNLDiscoveryHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/v1/discover/catalog", strings.NewReader("bad"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}
