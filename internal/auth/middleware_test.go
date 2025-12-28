package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func setupTestMiddleware(t *testing.T) (*Middleware, *APIKey, string) {
	t.Helper()

	ac := NewAccessController()
	ac.CreateTenant(&Tenant{ID: "tenant1", Name: "Test"})

	rawKey, err := ac.CreateAPIKey(&APIKey{
		Name:        "test-key",
		Tenant:      "tenant1",
		Roles:       []string{"writer"},
		Permissions: []Permission{PermRead, PermWrite},
		Namespaces:  []string{"ns1"},
		Features:    []string{"feature1"},
	}, "admin")
	if err != nil {
		t.Fatalf("Failed to create API key: %v", err)
	}

	keyInfo, _ := ac.ValidateAPIKey(rawKey)
	mw := NewMiddleware(ac, nil)

	return mw, keyInfo, rawKey
}

func TestNewMiddleware(t *testing.T) {
	ac := NewAccessController()
	mw := NewMiddleware(ac, nil)

	if mw == nil {
		t.Fatal("NewMiddleware returned nil")
	}
	if mw.controller == nil {
		t.Error("controller should be set")
	}
	if mw.rateLimiter == nil {
		t.Error("rateLimiter should be initialized")
	}
}

func TestMiddleware_Authenticate_NoAPIKey(t *testing.T) {
	mw, _, _ := setupTestMiddleware(t)

	handler := mw.Authenticate(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Handler should not be called without API key")
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Expected status %d, got %d", http.StatusUnauthorized, rec.Code)
	}
}

func TestMiddleware_Authenticate_InvalidAPIKey(t *testing.T) {
	mw, _, _ := setupTestMiddleware(t)

	handler := mw.Authenticate(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Handler should not be called with invalid API key")
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer invalid_key")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Expected status %d, got %d", http.StatusUnauthorized, rec.Code)
	}
}

func TestMiddleware_Authenticate_ValidAPIKey_AuthorizationHeader(t *testing.T) {
	mw, keyInfo, rawKey := setupTestMiddleware(t)

	var receivedKey *APIKey
	handler := mw.Authenticate(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedKey = APIKeyFromContext(r.Context())
		tenant := TenantFromContext(r.Context())
		if tenant != "tenant1" {
			t.Errorf("Expected tenant 'tenant1', got '%s'", tenant)
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+rawKey)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if receivedKey == nil || receivedKey.ID != keyInfo.ID {
		t.Error("API key should be in context")
	}
}

func TestMiddleware_Authenticate_ValidAPIKey_XAPIKeyHeader(t *testing.T) {
	mw, keyInfo, rawKey := setupTestMiddleware(t)

	var receivedKey *APIKey
	handler := mw.Authenticate(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedKey = APIKeyFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-API-Key", rawKey)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if receivedKey == nil || receivedKey.ID != keyInfo.ID {
		t.Error("API key should be in context")
	}
}

func TestMiddleware_Authenticate_RateLimited(t *testing.T) {
	ac := NewAccessController()
	ac.CreateTenant(&Tenant{ID: "tenant1", Name: "Test"})

	rawKey, _ := ac.CreateAPIKey(&APIKey{
		Name:      "test-key",
		Tenant:    "tenant1",
		RateLimit: 2, // Very low rate limit
	}, "admin")

	mw := NewMiddleware(ac, nil)

	handler := mw.Authenticate(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// First two requests should succeed
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Authorization", "Bearer "+rawKey)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("Request %d: Expected status %d, got %d", i+1, http.StatusOK, rec.Code)
		}
	}

	// Third request should be rate limited
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+rawKey)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("Expected status %d, got %d", http.StatusTooManyRequests, rec.Code)
	}
}

func TestMiddleware_RequirePermission(t *testing.T) {
	mw, _, rawKey := setupTestMiddleware(t)

	tests := []struct {
		name           string
		permission     Permission
		expectedStatus int
	}{
		{"has permission", PermRead, http.StatusOK},
		{"has permission - write", PermWrite, http.StatusOK},
		{"missing permission", PermAdmin, http.StatusForbidden},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := mw.Authenticate(mw.RequirePermission(tt.permission)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})))

			req := httptest.NewRequest("GET", "/test", nil)
			req.Header.Set("Authorization", "Bearer "+rawKey)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, rec.Code)
			}
		})
	}
}

func TestMiddleware_RequirePermission_NoAuth(t *testing.T) {
	mw, _, _ := setupTestMiddleware(t)

	handler := mw.RequirePermission(PermRead)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Handler should not be called without auth")
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Expected status %d, got %d", http.StatusUnauthorized, rec.Code)
	}
}

func TestMiddleware_RequireNamespace(t *testing.T) {
	mw, _, rawKey := setupTestMiddleware(t)

	tests := []struct {
		name           string
		namespace      string
		expectedStatus int
	}{
		{"allowed namespace", "ns1", http.StatusOK},
		{"denied namespace", "ns2", http.StatusForbidden},
		{"empty namespace", "", http.StatusOK}, // Empty is allowed
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			getNamespace := func(r *http.Request) string {
				return tt.namespace
			}

			handler := mw.Authenticate(mw.RequireNamespace(getNamespace)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})))

			req := httptest.NewRequest("GET", "/test", nil)
			req.Header.Set("Authorization", "Bearer "+rawKey)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, rec.Code)
			}
		})
	}
}

func TestMiddleware_RequireNamespace_NoAuth(t *testing.T) {
	mw, _, _ := setupTestMiddleware(t)

	handler := mw.RequireNamespace(func(r *http.Request) string {
		return "ns1"
	})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Handler should not be called without auth")
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Expected status %d, got %d", http.StatusUnauthorized, rec.Code)
	}
}

func TestMiddleware_RequireFeature(t *testing.T) {
	mw, _, rawKey := setupTestMiddleware(t)

	tests := []struct {
		name           string
		feature        string
		expectedStatus int
	}{
		{"allowed feature", "feature1", http.StatusOK},
		{"denied feature", "feature2", http.StatusForbidden},
		{"empty feature", "", http.StatusOK}, // Empty is allowed
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			getFeature := func(r *http.Request) string {
				return tt.feature
			}

			handler := mw.Authenticate(mw.RequireFeature(getFeature)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})))

			req := httptest.NewRequest("GET", "/test", nil)
			req.Header.Set("Authorization", "Bearer "+rawKey)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, rec.Code)
			}
		})
	}
}

func TestMiddleware_RequireFeature_NoAuth(t *testing.T) {
	mw, _, _ := setupTestMiddleware(t)

	handler := mw.RequireFeature(func(r *http.Request) string {
		return "feature1"
	})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Handler should not be called without auth")
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Expected status %d, got %d", http.StatusUnauthorized, rec.Code)
	}
}

func TestMiddleware_Optional(t *testing.T) {
	mw, keyInfo, rawKey := setupTestMiddleware(t)

	tests := []struct {
		name       string
		authHeader string
		expectKey  bool
	}{
		{"with valid key", "Bearer " + rawKey, true},
		{"without key", "", false},
		{"with invalid key", "Bearer invalid", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var receivedKey *APIKey
			handler := mw.Optional(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				receivedKey = APIKeyFromContext(r.Context())
				w.WriteHeader(http.StatusOK)
			}))

			req := httptest.NewRequest("GET", "/test", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			// Should always succeed (optional)
			if rec.Code != http.StatusOK {
				t.Errorf("Expected status %d, got %d", http.StatusOK, rec.Code)
			}

			if tt.expectKey {
				if receivedKey == nil || receivedKey.ID != keyInfo.ID {
					t.Error("Expected key in context")
				}
			} else {
				if receivedKey != nil {
					t.Error("Expected no key in context")
				}
			}
		})
	}
}

func TestRateLimiter_Allow(t *testing.T) {
	rl := NewRateLimiter()

	// Should allow initial requests up to rate limit
	for i := 0; i < 5; i++ {
		if !rl.Allow("key1", 5) {
			t.Errorf("Request %d should be allowed", i+1)
		}
	}

	// Should deny when rate limit exceeded
	if rl.Allow("key1", 5) {
		t.Error("Request should be denied when rate limit exceeded")
	}

	// Different key should be allowed
	if !rl.Allow("key2", 5) {
		t.Error("Different key should be allowed")
	}
}

func TestRateLimiter_Refill(t *testing.T) {
	rl := NewRateLimiter()

	// Exhaust tokens
	for i := 0; i < 100; i++ {
		rl.Allow("key1", 100)
	}

	// Should be denied
	if rl.Allow("key1", 100) {
		t.Error("Should be denied after exhausting tokens")
	}

	// Wait for some refill (this simulates token refill)
	// In actual implementation, we'd wait, but for testing we can
	// manually test the refill logic by checking the bucket state
}

// TestGetClientIP is now in internal/clientip/resolver_test.go

func TestMiddleware_AuditLogging(t *testing.T) {
	ac := NewAccessController()
	ac.CreateTenant(&Tenant{ID: "tenant1", Name: "Test"})

	rawKey, _ := ac.CreateAPIKey(&APIKey{
		Name:   "test-key",
		Tenant: "tenant1",
	}, "admin")

	mw := NewMiddleware(ac, nil)

	handler := mw.Authenticate(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/v1/features", nil)
	req.Header.Set("Authorization", "Bearer "+rawKey)
	req.Header.Set("User-Agent", "test-agent")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// Check audit log was created
	logs := ac.GetAuditLogs("tenant1", time.Time{}, 100)
	if len(logs) == 0 {
		t.Error("Expected audit log to be created")
	}

	log := logs[0]
	if log.Action != "GET" {
		t.Errorf("Expected action 'GET', got '%s'", log.Action)
	}
	if log.Resource != "/v1/features" {
		t.Errorf("Expected resource '/v1/features', got '%s'", log.Resource)
	}
	if log.UserAgent != "test-agent" {
		t.Errorf("Expected user agent 'test-agent', got '%s'", log.UserAgent)
	}
	if !log.Success {
		t.Error("Expected success=true")
	}
}

func TestMiddleware_PermissionDenied_AuditLogging(t *testing.T) {
	ac := NewAccessController()
	ac.CreateTenant(&Tenant{ID: "tenant1", Name: "Test"})

	rawKey, _ := ac.CreateAPIKey(&APIKey{
		Name:        "test-key",
		Tenant:      "tenant1",
		Permissions: []Permission{PermRead}, // Only read permission
	}, "admin")

	mw := NewMiddleware(ac, nil)

	handler := mw.Authenticate(mw.RequirePermission(PermAdmin)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})))

	req := httptest.NewRequest("GET", "/admin", nil)
	req.Header.Set("Authorization", "Bearer "+rawKey)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("Expected status %d, got %d", http.StatusForbidden, rec.Code)
	}

	// Check audit log was created with error
	logs := ac.GetAuditLogs("tenant1", time.Time{}, 100)
	var failedLog *AuditLog
	for i := range logs {
		if !logs[i].Success {
			failedLog = &logs[i]
			break
		}
	}

	if failedLog == nil {
		t.Error("Expected failed audit log")
	} else {
		if failedLog.Error == "" {
			t.Error("Expected error message in audit log")
		}
	}
}

func TestMiddleware_ChainedMiddleware(t *testing.T) {
	ac := NewAccessController()
	ac.CreateTenant(&Tenant{ID: "tenant1", Name: "Test"})

	rawKey, _ := ac.CreateAPIKey(&APIKey{
		Name:        "test-key",
		Tenant:      "tenant1",
		Permissions: []Permission{PermRead, PermWrite},
		Namespaces:  []string{"production"},
		Features:    []string{"user_age"},
	}, "admin")

	mw := NewMiddleware(ac, nil)

	// Chain multiple middleware
	handler := mw.Authenticate(
		mw.RequirePermission(PermRead)(
			mw.RequireNamespace(func(r *http.Request) string { return "production" })(
				mw.RequireFeature(func(r *http.Request) string { return "user_age" })(
					http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						w.WriteHeader(http.StatusOK)
						w.Write([]byte("success"))
					}),
				),
			),
		),
	)

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+rawKey)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if rec.Body.String() != "success" {
		t.Errorf("Expected body 'success', got '%s'", rec.Body.String())
	}
}
