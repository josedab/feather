package ingestion

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/feather-store/feather/internal/aggregation"
	"github.com/feather-store/feather/internal/domain"
	"github.com/feather-store/feather/internal/storage"
)

// newTestHTTPIngestion creates an HTTPIngestion handler for testing.
func newTestHTTPIngestion(t *testing.T, config HTTPIngestionConfig) *HTTPIngestion {
	t.Helper()

	schema := storage.NewRegistry()
	store, err := storage.NewStore(context.Background(), storage.StoreOptions{
		HotMaxSize:   1 << 20, // 1MB
		WarmInMemory: true,
	}, schema)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	agg := aggregation.NewEngine()

	t.Cleanup(func() {
		store.Close()
	})

	return NewHTTPIngestionWithConfig(store, agg, schema, config)
}

func TestHTTPIngestion_HandlePush(t *testing.T) {
	h := newTestHTTPIngestion(t, HTTPIngestionConfig{
		ValidateSchema: false, // Disable schema validation for basic tests
	})

	tests := []struct {
		name       string
		body       interface{}
		wantStatus int
	}{
		{
			name: "valid single feature",
			body: map[string]interface{}{
				"entity_key": "user:123",
				"features": map[string]interface{}{
					"score": 0.95,
				},
			},
			wantStatus: http.StatusCreated,
		},
		{
			name: "valid multiple features",
			body: map[string]interface{}{
				"entity_key": "user:456",
				"features": map[string]interface{}{
					"score":      0.85,
					"rank":       float64(10),
					"is_premium": true,
				},
			},
			wantStatus: http.StatusCreated,
		},
		{
			name: "with timestamp",
			body: map[string]interface{}{
				"entity_key": "user:789",
				"features": map[string]interface{}{
					"score": 0.75,
				},
				"timestamp": 1704067200000000000,
			},
			wantStatus: http.StatusCreated,
		},
		{
			name: "missing entity_key",
			body: map[string]interface{}{
				"features": map[string]interface{}{
					"score": 0.5,
				},
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "empty body",
			body:       map[string]interface{}{},
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jsonBody, err := json.Marshal(tt.body)
			if err != nil {
				t.Fatalf("failed to marshal body: %v", err)
			}

			req := httptest.NewRequest(http.MethodPost, "/ingest", bytes.NewReader(jsonBody))
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()

			h.HandlePush(rr, req)

			if rr.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d; body: %s", rr.Code, tt.wantStatus, rr.Body.String())
			}
		})
	}
}

func TestHTTPIngestion_HandlePush_InvalidJSON(t *testing.T) {
	h := newTestHTTPIngestion(t, HTTPIngestionConfig{})

	req := httptest.NewRequest(http.MethodPost, "/ingest", bytes.NewReader([]byte("not json")))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.HandlePush(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestHTTPIngestion_HandleBulkPush(t *testing.T) {
	h := newTestHTTPIngestion(t, HTTPIngestionConfig{
		ValidateSchema: false,
	})

	tests := []struct {
		name          string
		body          interface{}
		wantStatus    int
		wantSuccesses int
	}{
		{
			name: "multiple updates",
			body: []map[string]interface{}{
				{
					"entity_key": "user:1",
					"features":   map[string]interface{}{"score": 0.9},
				},
				{
					"entity_key": "user:2",
					"features":   map[string]interface{}{"score": 0.8},
				},
				{
					"entity_key": "user:3",
					"features":   map[string]interface{}{"score": 0.7},
				},
			},
			wantStatus:    http.StatusOK,
			wantSuccesses: 3,
		},
		{
			name: "with errors",
			body: []map[string]interface{}{
				{
					"entity_key": "user:4",
					"features":   map[string]interface{}{"score": 0.9},
				},
				{
					// Missing entity_key
					"features": map[string]interface{}{"score": 0.8},
				},
			},
			wantStatus:    http.StatusOK,
			wantSuccesses: 1,
		},
		{
			name:          "empty array",
			body:          []map[string]interface{}{},
			wantStatus:    http.StatusOK,
			wantSuccesses: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jsonBody, err := json.Marshal(tt.body)
			if err != nil {
				t.Fatalf("failed to marshal body: %v", err)
			}

			req := httptest.NewRequest(http.MethodPost, "/ingest/bulk", bytes.NewReader(jsonBody))
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()

			h.HandleBulkPush(rr, req)

			if rr.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d; body: %s", rr.Code, tt.wantStatus, rr.Body.String())
			}

			if tt.wantStatus == http.StatusOK {
				var resp map[string]interface{}
				if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
					t.Fatalf("failed to parse response: %v", err)
				}

				if success, ok := resp["success"].(float64); ok {
					if int(success) != tt.wantSuccesses {
						t.Errorf("success = %d, want %d", int(success), tt.wantSuccesses)
					}
				}
			}
		})
	}
}

func TestHTTPIngestion_RateLimiting(t *testing.T) {
	h := newTestHTTPIngestion(t, HTTPIngestionConfig{
		RateLimitEnabled:   true,
		RateLimitPerSecond: 2,
		RateLimitBurst:     2,
		ValidateSchema:     false,
	})

	body := map[string]interface{}{
		"entity_key": "user:123",
		"features":   map[string]interface{}{"score": 0.5},
	}
	jsonBody, _ := json.Marshal(body)

	// First few requests should succeed (using burst)
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodPost, "/ingest", bytes.NewReader(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		req.RemoteAddr = "192.168.1.1:12345"
		rr := httptest.NewRecorder()

		h.HandlePush(rr, req)

		if rr.Code != http.StatusCreated {
			t.Errorf("request %d: status = %d, want %d", i+1, rr.Code, http.StatusCreated)
		}
	}

	// Third request should be rate limited
	req := httptest.NewRequest(http.MethodPost, "/ingest", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "192.168.1.1:12345"
	rr := httptest.NewRecorder()

	h.HandlePush(rr, req)

	if rr.Code != http.StatusTooManyRequests {
		t.Errorf("rate limited request: status = %d, want %d", rr.Code, http.StatusTooManyRequests)
	}

	// Check Retry-After header
	if rr.Header().Get("Retry-After") == "" {
		t.Error("expected Retry-After header to be set")
	}
}

// TestHTTPIngestion_ClientIPExtraction is now in internal/clientip/resolver_test.go

func TestHTTPIngestion_Metrics(t *testing.T) {
	h := newTestHTTPIngestion(t, HTTPIngestionConfig{
		ValidateSchema: false,
	})

	// Make some requests
	body := map[string]interface{}{
		"entity_key": "user:123",
		"features":   map[string]interface{}{"score": 0.5},
	}
	jsonBody, _ := json.Marshal(body)

	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodPost, "/ingest", bytes.NewReader(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		h.HandlePush(rr, req)
	}

	// Make an error request
	req := httptest.NewRequest(http.MethodPost, "/ingest", bytes.NewReader([]byte("{}")))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.HandlePush(rr, req)

	metrics := h.Metrics()

	if metrics.RequestsReceived != 4 {
		t.Errorf("RequestsReceived = %d, want 4", metrics.RequestsReceived)
	}
	if metrics.RequestsSuccess != 3 {
		t.Errorf("RequestsSuccess = %d, want 3", metrics.RequestsSuccess)
	}
	if metrics.RequestsError != 1 {
		t.Errorf("RequestsError = %d, want 1", metrics.RequestsError)
	}
	if metrics.FeaturesIngested != 3 {
		t.Errorf("FeaturesIngested = %d, want 3", metrics.FeaturesIngested)
	}
}

func TestValidationError(t *testing.T) {
	tests := []struct {
		name   string
		errors []string
		want   string
	}{
		{
			name:   "single error",
			errors: []string{"field: invalid type"},
			want:   "validation error: field: invalid type",
		},
		{
			name:   "multiple errors",
			errors: []string{"field1: error1", "field2: error2"},
			want:   "validation errors: [field1: error1 field2: error2]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := &ValidationError{Errors: tt.errors}
			if err.Error() != tt.want {
				t.Errorf("Error() = %q, want %q", err.Error(), tt.want)
			}
		})
	}
}

func TestToFloat64(t *testing.T) {
	tests := []struct {
		name  string
		input interface{}
		want  float64
		ok    bool
	}{
		{"float64", float64(3.14), 3.14, true},
		{"float32", float32(2.5), 2.5, true},
		{"int", int(42), 42.0, true},
		{"int64", int64(100), 100.0, true},
		{"int32", int32(50), 50.0, true},
		{"string", "not a number", 0, false},
		{"nil", nil, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := domain.ToFloat64(tt.input)
			if ok != tt.ok {
				t.Errorf("toFloat64() ok = %v, want %v", ok, tt.ok)
			}
			if ok && got != tt.want {
				t.Errorf("toFloat64() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRateLimiter(t *testing.T) {
	rl := newRateLimiter(10, 5) // 10 rps, burst of 5

	clientID := "client1"

	// Should allow burst
	for i := 0; i < 5; i++ {
		if !rl.allow(clientID) {
			t.Errorf("request %d should be allowed (within burst)", i+1)
		}
	}

	// Next request should be denied (burst exhausted)
	if rl.allow(clientID) {
		t.Error("request after burst should be denied")
	}

	// Different client should have its own bucket
	if !rl.allow("client2") {
		t.Error("different client should be allowed")
	}
}

func TestHTTPIngestion_RequestSizeLimit(t *testing.T) {
	// Create handler with small max request size
	h := newTestHTTPIngestion(t, HTTPIngestionConfig{
		ValidateSchema: false,
		MaxRequestSize: 100, // 100 bytes
	})

	// Create valid JSON body that exceeds the limit
	// This is a valid JSON structure with a large value to exceed 100 bytes
	largeFeatures := make(map[string]interface{})
	for i := 0; i < 20; i++ {
		largeFeatures[fmt.Sprintf("feature_%d", i)] = 1.234567890
	}
	body := map[string]interface{}{
		"entity_key": "user:123",
		"features":   largeFeatures,
	}
	jsonBody, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("failed to marshal body: %v", err)
	}

	// Verify the body is larger than the limit
	if len(jsonBody) <= 100 {
		t.Fatalf("test body should be > 100 bytes, got %d", len(jsonBody))
	}

	req := httptest.NewRequest(http.MethodPost, "/ingest", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.HandlePush(rr, req)

	if rr.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("status = %d, want %d; body: %s", rr.Code, http.StatusRequestEntityTooLarge, rr.Body.String())
	}
}

func TestHTTPIngestion_BulkRequestSizeLimit(t *testing.T) {
	// Create handler with small max request size
	h := newTestHTTPIngestion(t, HTTPIngestionConfig{
		ValidateSchema: false,
		MaxRequestSize: 100, // 100 bytes, bulk gets 10x = 1000 bytes
	})

	// Create valid JSON array that exceeds the bulk limit (1000 bytes)
	var updates []map[string]interface{}
	for i := 0; i < 50; i++ {
		updates = append(updates, map[string]interface{}{
			"entity_key": fmt.Sprintf("user:%d", i),
			"features":   map[string]interface{}{"score": 0.123456789},
		})
	}
	jsonBody, err := json.Marshal(updates)
	if err != nil {
		t.Fatalf("failed to marshal body: %v", err)
	}

	// Verify the body is larger than the bulk limit (1000 bytes)
	if len(jsonBody) <= 1000 {
		t.Fatalf("test body should be > 1000 bytes, got %d", len(jsonBody))
	}

	req := httptest.NewRequest(http.MethodPost, "/ingest/bulk", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.HandleBulkPush(rr, req)

	if rr.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("status = %d, want %d; body: %s", rr.Code, http.StatusRequestEntityTooLarge, rr.Body.String())
	}
}
