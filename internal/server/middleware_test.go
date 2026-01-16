package server

import (
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRequestIDMiddleware(t *testing.T) {
	handler := requestIDMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	t.Run("generates request ID when not provided", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		requestID := rr.Header().Get("X-Request-ID")
		if requestID == "" {
			t.Error("Expected X-Request-ID header to be set")
		}
	})

	t.Run("preserves existing request ID", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("X-Request-ID", "existing-id-123")
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		requestID := rr.Header().Get("X-Request-ID")
		if requestID != "existing-id-123" {
			t.Errorf("Expected X-Request-ID 'existing-id-123', got '%s'", requestID)
		}
	})
}

func TestCompressionMiddleware(t *testing.T) {
	responseBody := "Hello, World! This is a test response."
	handler := compressionMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(responseBody))
	}))

	t.Run("compresses when Accept-Encoding includes gzip", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("Accept-Encoding", "gzip, deflate")
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if rr.Header().Get("Content-Encoding") != "gzip" {
			t.Error("Expected Content-Encoding to be gzip")
		}

		// Verify we can decompress the response
		gzReader, err := gzip.NewReader(rr.Body)
		if err != nil {
			t.Fatalf("Failed to create gzip reader: %v", err)
		}
		defer gzReader.Close()

		decompressed, err := io.ReadAll(gzReader)
		if err != nil {
			t.Fatalf("Failed to decompress: %v", err)
		}

		if string(decompressed) != responseBody {
			t.Errorf("Expected '%s', got '%s'", responseBody, string(decompressed))
		}
	})

	t.Run("does not compress without Accept-Encoding gzip", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if rr.Header().Get("Content-Encoding") == "gzip" {
			t.Error("Expected no gzip compression")
		}

		if rr.Body.String() != responseBody {
			t.Errorf("Expected '%s', got '%s'", responseBody, rr.Body.String())
		}
	})

	t.Run("skips compression for health endpoints", func(t *testing.T) {
		endpoints := []string{"/health", "/ready", "/live"}

		for _, endpoint := range endpoints {
			req := httptest.NewRequest(http.MethodGet, endpoint, nil)
			req.Header.Set("Accept-Encoding", "gzip")
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			if rr.Header().Get("Content-Encoding") == "gzip" {
				t.Errorf("Expected no gzip compression for %s", endpoint)
			}
		}
	})
}

func TestMaxRequestSizeMiddleware(t *testing.T) {
	maxSize := int64(100)
	handler := maxRequestSizeMiddleware(maxSize)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusRequestEntityTooLarge)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write(body)
	}))

	t.Run("allows requests within size limit", func(t *testing.T) {
		body := strings.Repeat("a", 50)
		req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(body))
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rr.Code)
		}
	})

	t.Run("rejects requests exceeding size limit", func(t *testing.T) {
		body := strings.Repeat("a", 200)
		req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(body))
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusRequestEntityTooLarge {
			t.Errorf("Expected status 413, got %d", rr.Code)
		}
	})

	t.Run("does not limit GET requests", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rr.Code)
		}
	})
}

func TestPanicRecoveryMiddleware_Recovery(t *testing.T) {
	t.Run("recovers from panic", func(t *testing.T) {
		handler := panicRecoveryMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			panic("test panic")
		}))

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rr := httptest.NewRecorder()

		// Should not panic
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Errorf("Expected status 500, got %d", rr.Code)
		}
	})

	t.Run("passes through normal requests", func(t *testing.T) {
		handler := panicRecoveryMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rr.Code)
		}
	})
}

func TestSecurityHeadersMiddleware(t *testing.T) {
	expectedHeaders := map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":        "DENY",
		"X-XSS-Protection":       "1; mode=block",
		"Referrer-Policy":        "strict-origin-when-cross-origin",
		"Content-Security-Policy": "default-src 'self'; frame-ancestors 'none'",
		"Permissions-Policy":     "geolocation=(), microphone=(), camera=()",
	}

	t.Run("sets security headers without TLS", func(t *testing.T) {
		handler := securityHeadersMiddleware(false)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		for header, expected := range expectedHeaders {
			if rr.Header().Get(header) != expected {
				t.Errorf("Expected %s '%s', got '%s'", header, expected, rr.Header().Get(header))
			}
		}

		if rr.Header().Get("Strict-Transport-Security") != "" {
			t.Error("Expected no HSTS header without TLS")
		}
	})

	t.Run("sets HSTS header with TLS", func(t *testing.T) {
		handler := securityHeadersMiddleware(true)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		hsts := rr.Header().Get("Strict-Transport-Security")
		if hsts != "max-age=31536000; includeSubDomains" {
			t.Errorf("Expected HSTS header, got '%s'", hsts)
		}
	})
}

func TestCORSMiddleware(t *testing.T) {
	t.Run("allows configured origin", func(t *testing.T) {
		config := &CORSConfig{
			AllowedOrigins: []string{"https://example.com"},
			AllowedMethods: []string{"GET", "POST"},
			AllowedHeaders: []string{"Content-Type"},
		}
		handler := corsMiddleware(config)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("Origin", "https://example.com")
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if rr.Header().Get("Access-Control-Allow-Origin") != "https://example.com" {
			t.Errorf("Expected origin header, got '%s'", rr.Header().Get("Access-Control-Allow-Origin"))
		}
	})

	t.Run("rejects non-configured origin", func(t *testing.T) {
		config := &CORSConfig{
			AllowedOrigins: []string{"https://example.com"},
		}
		handler := corsMiddleware(config)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("Origin", "https://evil.com")
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if rr.Header().Get("Access-Control-Allow-Origin") != "" {
			t.Error("Expected no origin header for non-configured origin")
		}
	})

	t.Run("handles preflight requests", func(t *testing.T) {
		config := &CORSConfig{
			AllowedOrigins: []string{"https://example.com"},
			AllowedMethods: []string{"GET", "POST"},
			AllowedHeaders: []string{"Content-Type", "Authorization"},
			MaxAge:         3600,
		}
		handler := corsMiddleware(config)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest(http.MethodOptions, "/test", nil)
		req.Header.Set("Origin", "https://example.com")
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusNoContent {
			t.Errorf("Expected status 204, got %d", rr.Code)
		}
		if rr.Header().Get("Access-Control-Allow-Methods") == "" {
			t.Error("Expected Access-Control-Allow-Methods header")
		}
		if rr.Header().Get("Access-Control-Max-Age") != "3600" {
			t.Errorf("Expected max-age 3600, got '%s'", rr.Header().Get("Access-Control-Max-Age"))
		}
	})

	t.Run("wildcard origin", func(t *testing.T) {
		config := &CORSConfig{
			AllowedOrigins: []string{"*"},
		}
		handler := corsMiddleware(config)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("Origin", "https://any-domain.com")
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if rr.Header().Get("Access-Control-Allow-Origin") != "*" {
			t.Errorf("Expected wildcard origin, got '%s'", rr.Header().Get("Access-Control-Allow-Origin"))
		}
	})

	t.Run("disables credentials with wildcard origin", func(t *testing.T) {
		config := &CORSConfig{
			AllowedOrigins:   []string{"*"},
			AllowCredentials: true,
		}
		handler := corsMiddleware(config)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("Origin", "https://example.com")
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		// Credentials should be disabled when wildcard is used
		if rr.Header().Get("Access-Control-Allow-Credentials") == "true" {
			t.Error("Credentials should be disabled with wildcard origin")
		}
	})

	t.Run("uses default config when nil", func(t *testing.T) {
		handler := corsMiddleware(nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest(http.MethodOptions, "/test", nil)
		req.Header.Set("Origin", "https://example.com")
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		// Should handle OPTIONS without panic
		if rr.Code != http.StatusNoContent {
			t.Errorf("Expected status 204, got %d", rr.Code)
		}
	})
}

func TestDefaultCORSConfig(t *testing.T) {
	config := DefaultCORSConfig()

	if len(config.AllowedOrigins) != 0 {
		t.Errorf("Expected empty AllowedOrigins, got %v", config.AllowedOrigins)
	}

	if config.AllowCredentials {
		t.Error("Expected AllowCredentials to be false by default")
	}

	if config.MaxAge != 86400 {
		t.Errorf("Expected MaxAge 86400, got %d", config.MaxAge)
	}
}
