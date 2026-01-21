package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/feather-store/feather/internal/extensions/mesh"
)

func setupMeshHandler(t *testing.T) (*MeshHandler, *http.ServeMux) {
	t.Helper()
	manager := mesh.NewMeshManager(mesh.DefaultMeshConfig())
	handler := NewMeshHandler(manager)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	return handler, mux
}

func TestMeshHandler_ListNodes(t *testing.T) {
	_, mux := setupMeshHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/mesh/nodes", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected %d, got %d", http.StatusOK, rr.Code)
	}
	var result map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
}

func TestMeshHandler_RegisterNode(t *testing.T) {
	_, mux := setupMeshHandler(t)
	body := `{"id":"node1","name":"test-node","endpoint":"http://localhost:8080","region":"us-east-1"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/mesh/nodes", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Errorf("expected %d, got %d; body: %s", http.StatusCreated, rr.Code, rr.Body.String())
	}
}

func TestMeshHandler_RegisterNode_InvalidJSON(t *testing.T) {
	_, mux := setupMeshHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/v1/mesh/nodes", strings.NewReader("invalid"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected %d, got %d", http.StatusBadRequest, rr.Code)
	}
}
