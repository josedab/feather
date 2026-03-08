package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/feather-store/feather/internal/extensions/garbagecollect"
)

func TestGarbageCollectHandler_CreateAndList(t *testing.T) {
	collector := garbagecollect.NewCollector()
	handler := NewGarbageCollectHandler(collector)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	// Create policy
	body, _ := json.Marshal(garbagecollect.GCPolicy{
		Name:            "test",
		MaxIdleDuration: 3600000000000,
		DryRun:          true,
	})
	req := httptest.NewRequest("POST", "/v1/gc/policies", bytes.NewReader(body))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	// List policies
	req = httptest.NewRequest("GET", "/v1/gc/policies", nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["total"].(float64) != 1 {
		t.Fatalf("expected 1 policy, got %v", resp["total"])
	}
}
