package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHTTPServer_GetFeatures(t *testing.T) {
	ts := newTestServer(t)

	// Seed test data
	ts.seedFeatures("user:123", map[string]interface{}{
		"purchase_count": float64(42),
		"total_spent":    float64(1234.56),
	})

	tests := []struct {
		name       string
		path       string
		wantStatus int
		wantEntity string
	}{
		{
			name:       "get single feature",
			path:       "/v1/features?entity=user:123&feature=purchase_count",
			wantStatus: http.StatusOK,
			wantEntity: "user:123",
		},
		{
			name:       "get multiple features",
			path:       "/v1/features?entity=user:123&feature=purchase_count&feature=total_spent",
			wantStatus: http.StatusOK,
			wantEntity: "user:123",
		},
		{
			name:       "get nonexistent entity",
			path:       "/v1/features?entity=user:999&feature=purchase_count",
			wantStatus: http.StatusOK, // Returns empty result, not 404
		},
		{
			name:       "missing entity parameter",
			path:       "/v1/features?feature=purchase_count",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "missing feature parameter",
			path:       "/v1/features?entity=user:123",
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rr := ts.get(tt.path)
			assertStatus(t, rr, tt.wantStatus)

			if tt.wantStatus == http.StatusOK {
				assertContentType(t, rr, "application/json")
				result := assertJSON(t, rr)

				// Unwrap APIResponse
				data, ok := result["data"].(map[string]interface{})
				if !ok {
					t.Fatalf("expected data field in APIResponse")
				}

				if tt.wantEntity != "" {
					entities, ok := data["entities"].(map[string]interface{})
					if !ok {
						t.Fatalf("expected entities map in response data")
					}
					if _, ok := entities[tt.wantEntity]; !ok {
						t.Errorf("expected entity %q in response", tt.wantEntity)
					}
				}
			}
		})
	}
}

func TestHTTPServer_PutFeatures(t *testing.T) {
	ts := newTestServer(t)

	tests := []struct {
		name       string
		body       interface{}
		wantStatus int
	}{
		{
			name: "put single feature",
			body: map[string]interface{}{
				"entity_key": "user:456",
				"features": map[string]interface{}{
					"score": 0.95,
				},
			},
			wantStatus: http.StatusCreated,
		},
		{
			name: "put multiple features",
			body: map[string]interface{}{
				"entity_key": "user:789",
				"features": map[string]interface{}{
					"score":     0.85,
					"rank":      float64(10),
					"is_active": true,
				},
			},
			wantStatus: http.StatusCreated,
		},
		{
			name: "put with timestamp",
			body: map[string]interface{}{
				"entity_key": "user:111",
				"features": map[string]interface{}{
					"count": float64(5),
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
			name: "empty features treated as valid",
			body: map[string]interface{}{
				"entity_key": "user:222",
			},
			wantStatus: http.StatusCreated, // Empty features map is valid
		},
		{
			name:       "invalid json",
			body:       "not json",
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rr := ts.post("/v1/features", tt.body)
			assertStatus(t, rr, tt.wantStatus)

			if tt.wantStatus == http.StatusCreated {
				assertContentType(t, rr, "application/json")
				result := assertJSON(t, rr)
				// Check APIResponse success field
				if success, ok := result["success"].(bool); !ok || !success {
					t.Errorf("expected success=true in APIResponse")
				}
				// Check wrapped data
				data, ok := result["data"].(map[string]interface{})
				if !ok {
					t.Fatalf("expected data field in APIResponse")
				}
				if success, ok := data["success"].(bool); !ok || !success {
					t.Errorf("expected success=true in response data")
				}
			}
		})
	}
}

func TestHTTPServer_GetFeaturesBatch(t *testing.T) {
	ts := newTestServer(t)

	// Seed test data
	ts.seedFeatures("user:1", map[string]interface{}{"score": float64(100)})
	ts.seedFeatures("user:2", map[string]interface{}{"score": float64(200)})
	ts.seedFeatures("user:3", map[string]interface{}{"score": float64(300)})

	tests := []struct {
		name       string
		body       interface{}
		wantStatus int
		wantCount  int
	}{
		{
			name: "batch get multiple entities",
			body: map[string]interface{}{
				"entities": []string{"user:1", "user:2", "user:3"},
				"features": []string{"score"},
			},
			wantStatus: http.StatusOK,
			wantCount:  3,
		},
		{
			name: "batch get with nonexistent entities",
			body: map[string]interface{}{
				"entities": []string{"user:1", "user:999"},
				"features": []string{"score"},
			},
			wantStatus: http.StatusOK,
			wantCount:  2, // Returns entries for all requested entities (some may be empty)
		},
		{
			name: "missing entities",
			body: map[string]interface{}{
				"features": []string{"score"},
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "missing features",
			body: map[string]interface{}{
				"entities": []string{"user:1"},
			},
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rr := ts.post("/v1/features/batch", tt.body)
			assertStatus(t, rr, tt.wantStatus)

			if tt.wantStatus == http.StatusOK {
				assertContentType(t, rr, "application/json")
				result := assertJSON(t, rr)

				// Unwrap APIResponse
				data, ok := result["data"].(map[string]interface{})
				if !ok {
					t.Fatalf("expected data field in APIResponse")
				}

				entities, ok := data["entities"].(map[string]interface{})
				if !ok {
					t.Fatalf("expected entities map in response data")
				}
				if len(entities) != tt.wantCount {
					t.Errorf("got %d entities, want %d", len(entities), tt.wantCount)
				}
			}
		})
	}
}

func TestHTTPServer_GetFeaturesAsOf(t *testing.T) {
	ts := newTestServer(t)

	// Seed test data
	ts.seedFeatures("user:123", map[string]interface{}{
		"purchase_count": float64(42),
	})

	tests := []struct {
		name       string
		path       string
		wantStatus int
	}{
		{
			name:       "get features as of recent time",
			path:       "/v1/features/history?entity=user:123&feature=purchase_count&as_of=2030-01-01T00:00:00Z",
			wantStatus: http.StatusOK,
		},
		{
			name:       "get features as of past time",
			path:       "/v1/features/history?entity=user:123&feature=purchase_count&as_of=2020-01-01T00:00:00Z",
			wantStatus: http.StatusOK,
		},
		{
			name:       "missing as_of parameter",
			path:       "/v1/features/history?entity=user:123&feature=purchase_count",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "invalid as_of format",
			path:       "/v1/features/history?entity=user:123&feature=purchase_count&as_of=invalid",
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rr := ts.get(tt.path)
			assertStatus(t, rr, tt.wantStatus)

			if tt.wantStatus == http.StatusOK {
				assertContentType(t, rr, "application/json")
				assertJSON(t, rr)
			}
		})
	}
}

func TestHTTPServer_SchemaEndpoints(t *testing.T) {
	ts := newTestServer(t)

	// Seed a feature group
	ts.seedFeatureGroup("user_features", "user", []string{"score", "rank"})

	t.Run("list groups", func(t *testing.T) {
		rr := ts.get("/v1/schema/groups")
		assertStatus(t, rr, http.StatusOK)
		assertContentType(t, rr, "application/json")
	})

	t.Run("get group", func(t *testing.T) {
		rr := ts.get("/v1/schema/groups/user_features")
		assertStatus(t, rr, http.StatusOK)
		assertContentType(t, rr, "application/json")
		result := assertJSON(t, rr)

		// Unwrap APIResponse
		data, ok := result["data"].(map[string]interface{})
		if !ok {
			t.Fatalf("expected data field in APIResponse")
		}

		if name, ok := data["name"].(string); !ok || name != "user_features" {
			t.Errorf("expected group name 'user_features', got %v", data["name"])
		}
	})

	t.Run("get nonexistent group", func(t *testing.T) {
		rr := ts.get("/v1/schema/groups/nonexistent")
		assertStatus(t, rr, http.StatusNotFound)
	})

	t.Run("create group", func(t *testing.T) {
		body := map[string]interface{}{
			"name":        "product_features",
			"entity_type": "product",
			"features": []map[string]interface{}{
				{"name": "price", "data_type": "float64"},
				{"name": "inventory", "data_type": "int64"},
			},
		}
		rr := ts.post("/v1/schema/groups", body)
		assertStatus(t, rr, http.StatusCreated)
	})

	t.Run("create duplicate group", func(t *testing.T) {
		body := map[string]interface{}{
			"name":        "user_features", // Already exists
			"entity_type": "user",
			"features": []map[string]interface{}{
				{"name": "some_feature", "data_type": "string"},
			},
		}
		rr := ts.post("/v1/schema/groups", body)
		assertStatus(t, rr, http.StatusConflict)
	})
}

func TestHTTPServer_SecurityHeaders(t *testing.T) {
	ts := newTestServer(t)

	rr := ts.get("/health")

	// Check security headers are set
	headers := map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":        "DENY",
		"X-Xss-Protection":       "1; mode=block",
		"Referrer-Policy":        "strict-origin-when-cross-origin",
	}

	for header, want := range headers {
		got := rr.Header().Get(header)
		if got != want {
			t.Errorf("%s = %q, want %q", header, got, want)
		}
	}
}

func TestHTTPServer_RequestID(t *testing.T) {
	ts := newTestServer(t)

	rr := ts.get("/health")

	requestID := rr.Header().Get("X-Request-ID")
	if requestID == "" {
		t.Error("expected X-Request-ID header to be set")
	}
}

func TestHTTPServer_Compression(t *testing.T) {
	ts := newTestServer(t)

	// Seed enough data to trigger compression
	ts.seedFeatures("user:123", map[string]interface{}{
		"feature1": "this is a long string value that should be compressed",
		"feature2": "another long string value for testing compression",
		"feature3": "and yet another string to ensure we have enough data",
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/features?entity=user:123&feature=feature1&feature=feature2&feature=feature3", nil)
	req.Header.Set("Accept-Encoding", "gzip")

	rr := httptest.NewRecorder()
	ts.handler.ServeHTTP(rr, req)

	// Note: Compression may only kick in for responses above a certain size
	// This test verifies the middleware doesn't break the response
	assertStatus(t, rr, http.StatusOK)
}

func TestPanicRecoveryMiddleware(t *testing.T) {
	// Create a handler that panics
	panicHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	})

	// Wrap with panic recovery middleware
	handler := panicRecoveryMiddleware(panicHandler)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rr := httptest.NewRecorder()

	// This should not panic
	handler.ServeHTTP(rr, req)

	// Should return 500
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
	}

	// Should have JSON content type
	if ct := rr.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want %q", ct, "application/json")
	}

	// Should contain error message
	if body := rr.Body.String(); body == "" {
		t.Error("expected non-empty response body")
	}
}

func TestErrorCodeToStatus(t *testing.T) {
	tests := []struct {
		code       string
		wantStatus int
	}{
		{"BAD_REQUEST", http.StatusBadRequest},
		{"VALIDATION_FAILED", http.StatusBadRequest},
		{"UNAUTHORIZED", http.StatusUnauthorized},
		{"FORBIDDEN", http.StatusForbidden},
		{"NOT_FOUND", http.StatusNotFound},
		{"CONFLICT", http.StatusConflict},
		{"RATE_LIMITED", http.StatusTooManyRequests},
		{"REQUEST_TOO_LARGE", http.StatusRequestEntityTooLarge},
		{"SERVICE_UNAVAILABLE", http.StatusServiceUnavailable},
		{"STORAGE_FULL", http.StatusInsufficientStorage},
		{"TIMEOUT", http.StatusGatewayTimeout},
		{"INTERNAL_ERROR", http.StatusInternalServerError},
		{"UNKNOWN_CODE", http.StatusInternalServerError},
		{"", http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			got := errorCodeToStatus(tt.code)
			if got != tt.wantStatus {
				t.Errorf("errorCodeToStatus(%q) = %d, want %d", tt.code, got, tt.wantStatus)
			}
		})
	}
}
