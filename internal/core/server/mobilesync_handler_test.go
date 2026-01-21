package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/feather-store/feather/internal/extensions/mobilesync"
)

func setupMobileSyncHandler(t *testing.T) (*MobileSyncHandler, *http.ServeMux) {
	t.Helper()
	manager := mobilesync.NewSyncManager(mobilesync.DefaultSyncConfig())
	handler := NewMobileSyncHandler(manager)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	return handler, mux
}

func TestMobileSyncHandler_ListDevices(t *testing.T) {
	_, mux := setupMobileSyncHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/mobile/devices", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected %d, got %d", http.StatusOK, rr.Code)
	}
	var result map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if result["success"] != true {
		t.Error("expected success=true")
	}
}

func TestMobileSyncHandler_RegisterDevice(t *testing.T) {
	_, mux := setupMobileSyncHandler(t)
	body := `{"id":"dev1","name":"iPhone","platform":"ios","features":["clicks"]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/mobile/devices", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Errorf("expected %d, got %d; body: %s", http.StatusCreated, rr.Code, rr.Body.String())
	}
}

func TestMobileSyncHandler_RegisterDevice_InvalidJSON(t *testing.T) {
	_, mux := setupMobileSyncHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/v1/mobile/devices", strings.NewReader("invalid"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected %d, got %d", http.StatusBadRequest, rr.Code)
	}
}
