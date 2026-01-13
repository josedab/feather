package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/feather-store/feather/internal/sla"
)

func TestSLAHandler_ListSLAs_Empty(t *testing.T) {
	manager := sla.NewManager(nil, sla.DefaultManagerConfig())
	handler := NewSLAHandler(manager)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	req := httptest.NewRequest("GET", "/v1/sla", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)

	slas := resp["slas"].([]interface{})
	if len(slas) != 0 {
		t.Errorf("expected 0 SLAs, got %d", len(slas))
	}
}

func TestSLAHandler_CreateSLA(t *testing.T) {
	manager := sla.NewManager(nil, sla.DefaultManagerConfig())
	handler := NewSLAHandler(manager)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	body := `{
		"name": "latency-p99",
		"description": "P99 latency must be under 100ms",
		"type": "latency",
		"target": 100,
		"priority": "high",
		"window": "5m",
		"alert_threshold": 0.9,
		"enabled": true
	}`
	req := httptest.NewRequest("POST", "/v1/sla", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status %d, got %d: %s", http.StatusCreated, w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)

	if resp["success"] != true {
		t.Error("expected success=true")
	}

	slaObj := resp["sla"].(map[string]interface{})
	if slaObj["name"] != "latency-p99" {
		t.Errorf("expected name 'latency-p99', got %v", slaObj["name"])
	}
	if slaObj["type"] != "latency" {
		t.Errorf("expected type 'latency', got %v", slaObj["type"])
	}
}

func TestSLAHandler_CreateSLA_MissingFields(t *testing.T) {
	manager := sla.NewManager(nil, sla.DefaultManagerConfig())
	handler := NewSLAHandler(manager)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	tests := []struct {
		name string
		body string
	}{
		{"missing name", `{"type":"latency","target":100,"window":"5m"}`},
		{"missing type", `{"name":"test","target":100,"window":"5m"}`},
		{"missing window", `{"name":"test","type":"latency","target":100}`},
		{"zero target", `{"name":"test","type":"latency","target":0,"window":"5m"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/v1/sla", bytes.NewBufferString(tt.body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, req)

			if w.Code != http.StatusBadRequest {
				t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
			}
		})
	}
}

func TestSLAHandler_GetSLA(t *testing.T) {
	manager := sla.NewManager(nil, sla.DefaultManagerConfig())
	manager.RegisterSLA(&sla.Spec{
		Name:    "test-sla",
		Type:    sla.TypeLatency,
		Target:  100,
		Window:  5 * time.Minute,
		Enabled: true,
	})
	handler := NewSLAHandler(manager)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	req := httptest.NewRequest("GET", "/v1/sla/test-sla", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var slaObj SLASpecJSON
	json.NewDecoder(w.Body).Decode(&slaObj)

	if slaObj.Name != "test-sla" {
		t.Errorf("expected name 'test-sla', got %s", slaObj.Name)
	}
}

func TestSLAHandler_GetSLA_NotFound(t *testing.T) {
	manager := sla.NewManager(nil, sla.DefaultManagerConfig())
	handler := NewSLAHandler(manager)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	req := httptest.NewRequest("GET", "/v1/sla/nonexistent", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, w.Code)
	}
}

func TestSLAHandler_DeleteSLA(t *testing.T) {
	manager := sla.NewManager(nil, sla.DefaultManagerConfig())
	manager.RegisterSLA(&sla.Spec{
		Name:    "test-sla",
		Type:    sla.TypeLatency,
		Target:  100,
		Window:  5 * time.Minute,
		Enabled: true,
	})
	handler := NewSLAHandler(manager)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	req := httptest.NewRequest("DELETE", "/v1/sla/test-sla", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	// Verify SLA is deleted
	_, err := manager.GetSLA("test-sla")
	if err == nil {
		t.Error("expected SLA to be deleted")
	}
}

func TestSLAHandler_DeleteSLA_NotFound(t *testing.T) {
	manager := sla.NewManager(nil, sla.DefaultManagerConfig())
	handler := NewSLAHandler(manager)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	req := httptest.NewRequest("DELETE", "/v1/sla/nonexistent", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, w.Code)
	}
}

func TestSLAHandler_GetStatus(t *testing.T) {
	manager := sla.NewManager(nil, sla.DefaultManagerConfig())
	manager.RegisterSLA(&sla.Spec{
		Name:    "test-sla",
		Type:    sla.TypeLatency,
		Target:  100,
		Window:  5 * time.Minute,
		Enabled: true,
	})
	handler := NewSLAHandler(manager)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	req := httptest.NewRequest("GET", "/v1/sla/test-sla/status", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var status SLAStatusJSON
	json.NewDecoder(w.Body).Decode(&status)

	if status.Name != "test-sla" {
		t.Errorf("expected name 'test-sla', got %s", status.Name)
	}
	if status.CompliancePercentage != 100 {
		t.Errorf("expected compliance 100, got %f", status.CompliancePercentage)
	}
}

func TestSLAHandler_GetAllStatuses(t *testing.T) {
	manager := sla.NewManager(nil, sla.DefaultManagerConfig())
	manager.RegisterSLA(&sla.Spec{
		Name:    "sla-1",
		Type:    sla.TypeLatency,
		Target:  100,
		Window:  5 * time.Minute,
		Enabled: true,
	})
	manager.RegisterSLA(&sla.Spec{
		Name:    "sla-2",
		Type:    sla.TypeAvailability,
		Target:  99.9,
		Window:  1 * time.Hour,
		Enabled: true,
	})
	handler := NewSLAHandler(manager)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	req := httptest.NewRequest("GET", "/v1/sla/status", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)

	statuses := resp["statuses"].([]interface{})
	if len(statuses) != 2 {
		t.Errorf("expected 2 statuses, got %d", len(statuses))
	}
}

func TestSLAHandler_GetBreaches(t *testing.T) {
	manager := sla.NewManager(nil, sla.DefaultManagerConfig())
	handler := NewSLAHandler(manager)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	req := httptest.NewRequest("GET", "/v1/sla/breaches", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)

	breaches := resp["breaches"].([]interface{})
	if len(breaches) != 0 {
		t.Errorf("expected 0 breaches, got %d", len(breaches))
	}
}

func TestSLAHandler_GetBreaches_WithSince(t *testing.T) {
	manager := sla.NewManager(nil, sla.DefaultManagerConfig())
	handler := NewSLAHandler(manager)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	// Use UTC to avoid timezone issues in URL encoding
	since := time.Now().UTC().Add(-1 * time.Hour).Format(time.RFC3339)
	req := httptest.NewRequest("GET", "/v1/sla/breaches", nil)
	q := req.URL.Query()
	q.Set("since", since)
	req.URL.RawQuery = q.Encode()
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}
}

func TestSLAHandler_GetBreaches_InvalidSince(t *testing.T) {
	manager := sla.NewManager(nil, sla.DefaultManagerConfig())
	handler := NewSLAHandler(manager)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	req := httptest.NewRequest("GET", "/v1/sla/breaches?since=invalid", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestSLAHandler_GetSummary(t *testing.T) {
	manager := sla.NewManager(nil, sla.DefaultManagerConfig())
	manager.RegisterSLA(&sla.Spec{
		Name:    "sla-1",
		Type:    sla.TypeLatency,
		Target:  100,
		Window:  5 * time.Minute,
		Enabled: true,
	})
	handler := NewSLAHandler(manager)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	req := httptest.NewRequest("GET", "/v1/sla/summary", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)

	if resp["totalSLAs"].(float64) != 1 {
		t.Errorf("expected totalSLAs 1, got %v", resp["totalSLAs"])
	}
}

func TestSLAHandler_EvaluateNow(t *testing.T) {
	manager := sla.NewManager(nil, sla.DefaultManagerConfig())
	handler := NewSLAHandler(manager)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	req := httptest.NewRequest("POST", "/v1/sla/evaluate", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)

	if resp["success"] != true {
		t.Error("expected success=true")
	}
}

func TestSLAHandler_NilManager(t *testing.T) {
	handler := NewSLAHandler(nil)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	endpoints := []struct {
		method string
		path   string
	}{
		{"GET", "/v1/sla"},
		{"GET", "/v1/sla/test"},
		{"POST", "/v1/sla"},
		{"DELETE", "/v1/sla/test"},
		{"GET", "/v1/sla/test/status"},
		{"GET", "/v1/sla/status"},
		{"GET", "/v1/sla/breaches"},
		{"GET", "/v1/sla/summary"},
		{"POST", "/v1/sla/evaluate"},
	}

	for _, ep := range endpoints {
		t.Run(ep.method+" "+ep.path, func(t *testing.T) {
			var req *http.Request
			if ep.method == "POST" && ep.path == "/v1/sla" {
				req = httptest.NewRequest(ep.method, ep.path, bytes.NewBufferString(`{}`))
			} else {
				req = httptest.NewRequest(ep.method, ep.path, nil)
			}
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, req)

			if w.Code != http.StatusServiceUnavailable {
				t.Errorf("expected status %d, got %d", http.StatusServiceUnavailable, w.Code)
			}
		})
	}
}

func TestSLAHandler_SLATypes(t *testing.T) {
	manager := sla.NewManager(nil, sla.DefaultManagerConfig())
	handler := NewSLAHandler(manager)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	slaTypes := []struct {
		slaType string
		target  float64
	}{
		{"latency", 100},
		{"freshness", 60},
		{"availability", 99.9},
		{"throughput", 1000},
	}

	for _, tt := range slaTypes {
		t.Run(tt.slaType, func(t *testing.T) {
			body := map[string]interface{}{
				"name":    "sla-" + tt.slaType,
				"type":    tt.slaType,
				"target":  tt.target,
				"window":  "5m",
				"enabled": true,
			}
			bodyBytes, _ := json.Marshal(body)
			req := httptest.NewRequest("POST", "/v1/sla", bytes.NewBuffer(bodyBytes))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, req)

			if w.Code != http.StatusCreated {
				t.Errorf("expected status %d for type %s, got %d: %s", http.StatusCreated, tt.slaType, w.Code, w.Body.String())
			}
		})
	}
}

func TestSLAHandler_PriorityParsing(t *testing.T) {
	tests := []struct {
		input    string
		expected sla.Priority
	}{
		{"low", sla.PriorityLow},
		{"medium", sla.PriorityMedium},
		{"high", sla.PriorityHigh},
		{"critical", sla.PriorityCritical},
		{"unknown", sla.PriorityMedium}, // Default
		{"", sla.PriorityMedium},        // Default
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parsePriority(tt.input)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestSLAHandler_ListSLAs_WithSLAs(t *testing.T) {
	manager := sla.NewManager(nil, sla.DefaultManagerConfig())
	manager.RegisterSLA(&sla.Spec{
		Name:    "sla-1",
		Type:    sla.TypeLatency,
		Target:  100,
		Window:  5 * time.Minute,
		Enabled: true,
	})
	manager.RegisterSLA(&sla.Spec{
		Name:    "sla-2",
		Type:    sla.TypeAvailability,
		Target:  99.9,
		Window:  1 * time.Hour,
		Enabled: true,
	})
	handler := NewSLAHandler(manager)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	req := httptest.NewRequest("GET", "/v1/sla", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)

	slas := resp["slas"].([]interface{})
	if len(slas) != 2 {
		t.Errorf("expected 2 SLAs, got %d", len(slas))
	}
	if resp["count"].(float64) != 2 {
		t.Errorf("expected count 2, got %v", resp["count"])
	}
}
