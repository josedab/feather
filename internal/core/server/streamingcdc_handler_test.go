package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/feather-store/feather/internal/extensions/streamingcdc"
)

func TestStreamingCDCHandler_CreateAndList(t *testing.T) {
	mgr := streamingcdc.NewManager(streamingcdc.DefaultManagerConfig())
	handler := NewStreamingCDCHandler(mgr)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	// Create pipeline
	body, _ := json.Marshal(streamingcdc.PipelineConfig{
		ID:                 "p1",
		Name:               "Test",
		SourceID:           "src-1",
		TargetFeatureGroup: "features",
	})
	req := httptest.NewRequest("POST", "/v1/streaming/pipelines", bytes.NewReader(body))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	// List pipelines
	req = httptest.NewRequest("GET", "/v1/streaming/pipelines", nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["total"].(float64) != 1 {
		t.Fatalf("expected 1 pipeline, got %v", resp["total"])
	}
}
