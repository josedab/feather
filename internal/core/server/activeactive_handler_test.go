package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/feather-store/feather/internal/extensions/activeactive"
)

func setupActiveActiveHandler(t *testing.T) *http.ServeMux {
	t.Helper()
	replicator := activeactive.NewReplicator(activeactive.DefaultReplicatorConfig())
	handler := NewActiveActiveHandler(replicator)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	return mux
}

func TestActiveActive_GetStats(t *testing.T) {
	mux := setupActiveActiveHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/activeactive/stats", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestActiveActive_PostAddPeer(t *testing.T) {
	mux := setupActiveActiveHandler(t)
	body := `{"id":"peer1","address":"localhost:9000"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/activeactive/peers", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK && rr.Code != http.StatusCreated {
		t.Errorf("expected 200 or 201, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestActiveActive_PostInvalidJSON(t *testing.T) {
	mux := setupActiveActiveHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/v1/activeactive/peers", io.NopCloser(strings.NewReader("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func activeActiveRequest(t *testing.T, mux *http.ServeMux, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	var bodyReader io.Reader
	if body != "" {
		bodyReader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, bodyReader)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	return rr
}

func TestActiveActive_ListPeers(t *testing.T) {
	mux := setupActiveActiveHandler(t)
	rr := activeActiveRequest(t, mux, http.MethodGet, "/v1/activeactive/peers", "")
	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestActiveActive_GetPeer_NotFound(t *testing.T) {
	mux := setupActiveActiveHandler(t)
	rr := activeActiveRequest(t, mux, http.MethodGet, "/v1/activeactive/peers/nonexistent", "")
	if rr.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestActiveActive_GetPeer(t *testing.T) {
	mux := setupActiveActiveHandler(t)
	// Add peer first
	activeActiveRequest(t, mux, http.MethodPost, "/v1/activeactive/peers",
		`{"id":"peer-1","address":"10.0.0.1:5000","region":"us-east-1"}`)

	rr := activeActiveRequest(t, mux, http.MethodGet, "/v1/activeactive/peers/peer-1", "")
	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want %d; body: %s", rr.Code, http.StatusOK, rr.Body.String())
	}
}

func TestActiveActive_RemovePeer(t *testing.T) {
	mux := setupActiveActiveHandler(t)
	activeActiveRequest(t, mux, http.MethodPost, "/v1/activeactive/peers",
		`{"id":"peer-1","address":"10.0.0.1:5000"}`)

	rr := activeActiveRequest(t, mux, http.MethodDelete, "/v1/activeactive/peers/peer-1", "")
	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestActiveActive_RemovePeer_NotFound(t *testing.T) {
	mux := setupActiveActiveHandler(t)
	rr := activeActiveRequest(t, mux, http.MethodDelete, "/v1/activeactive/peers/nonexistent", "")
	if rr.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestActiveActive_Replicate(t *testing.T) {
	mux := setupActiveActiveHandler(t)
	activeActiveRequest(t, mux, http.MethodPost, "/v1/activeactive/peers",
		`{"id":"peer-1","address":"10.0.0.1:5000"}`)

	rr := activeActiveRequest(t, mux, http.MethodPost, "/v1/activeactive/replicate",
		`{"type":"put","target_peer":"peer-1","payload":{"key":"value"}}`)
	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want %d; body: %s", rr.Code, http.StatusOK, rr.Body.String())
	}
}

func TestActiveActive_Replicate_InvalidJSON(t *testing.T) {
	mux := setupActiveActiveHandler(t)
	rr := activeActiveRequest(t, mux, http.MethodPost, "/v1/activeactive/replicate", "bad")
	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestActiveActive_Replicate_UnknownPeer(t *testing.T) {
	mux := setupActiveActiveHandler(t)
	rr := activeActiveRequest(t, mux, http.MethodPost, "/v1/activeactive/replicate",
		`{"type":"put","target_peer":"unknown","payload":{}}`)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestActiveActive_Receive(t *testing.T) {
	mux := setupActiveActiveHandler(t)
	rr := activeActiveRequest(t, mux, http.MethodPost, "/v1/activeactive/receive",
		`{"type":"put","source_peer":"remote-1","payload":{"key":"value"}}`)
	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want %d; body: %s", rr.Code, http.StatusOK, rr.Body.String())
	}
}

func TestActiveActive_Receive_InvalidJSON(t *testing.T) {
	mux := setupActiveActiveHandler(t)
	rr := activeActiveRequest(t, mux, http.MethodPost, "/v1/activeactive/receive", "bad")
	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestActiveActive_Receive_MissingSource(t *testing.T) {
	mux := setupActiveActiveHandler(t)
	rr := activeActiveRequest(t, mux, http.MethodPost, "/v1/activeactive/receive",
		`{"type":"put","source_peer":"","payload":{}}`)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestActiveActive_AntiEntropy(t *testing.T) {
	mux := setupActiveActiveHandler(t)
	activeActiveRequest(t, mux, http.MethodPost, "/v1/activeactive/peers",
		`{"id":"peer-1","address":"10.0.0.1:5000"}`)

	rr := activeActiveRequest(t, mux, http.MethodPost, "/v1/activeactive/anti-entropy/peer-1", "")
	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want %d; body: %s", rr.Code, http.StatusOK, rr.Body.String())
	}
}

func TestActiveActive_AntiEntropy_NotFound(t *testing.T) {
	mux := setupActiveActiveHandler(t)
	rr := activeActiveRequest(t, mux, http.MethodPost, "/v1/activeactive/anti-entropy/nonexistent", "")
	if rr.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestActiveActive_GossipState(t *testing.T) {
	mux := setupActiveActiveHandler(t)
	rr := activeActiveRequest(t, mux, http.MethodGet, "/v1/activeactive/gossip", "")
	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestActiveActive_AddPeer_EmptyID(t *testing.T) {
	mux := setupActiveActiveHandler(t)
	rr := activeActiveRequest(t, mux, http.MethodPost, "/v1/activeactive/peers",
		`{"id":"","address":"10.0.0.1:5000"}`)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestActiveActive_AddPeer_Duplicate(t *testing.T) {
	mux := setupActiveActiveHandler(t)
	activeActiveRequest(t, mux, http.MethodPost, "/v1/activeactive/peers",
		`{"id":"peer-1","address":"10.0.0.1:5000"}`)

	rr := activeActiveRequest(t, mux, http.MethodPost, "/v1/activeactive/peers",
		`{"id":"peer-1","address":"10.0.0.2:5000"}`)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}
