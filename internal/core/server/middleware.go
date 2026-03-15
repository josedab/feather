package server

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"runtime/debug"
	"strings"
	"sync/atomic"
	"time"

	"github.com/feather-store/feather/internal/core/domain"
	"github.com/feather-store/feather/internal/core/logging"
)

// requestIDMiddleware adds a unique request ID to each request.
func requestIDMiddleware(next http.Handler) http.Handler {
	var counter atomic.Uint64
	hostname, _ := os.Hostname()
	pid := os.Getpid()

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check for existing request ID header
		requestID := r.Header.Get("X-Request-ID")
		if requestID == "" {
			c := counter.Add(1)
			requestID = fmt.Sprintf("%s-%d-%d-%d", hostname, pid, time.Now().UnixNano(), c)
		}

		// Add request ID to response header
		w.Header().Set("X-Request-ID", requestID)

		// Add request ID to context
		ctx := logging.WithRequestID(r.Context(), requestID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// gzipResponseWriter wraps ResponseWriter to support gzip compression.
type gzipResponseWriter struct {
	http.ResponseWriter
	Writer io.Writer
}

func (w *gzipResponseWriter) Write(b []byte) (int, error) {
	return w.Writer.Write(b)
}

// Flush implements http.Flusher for streaming/SSE compatibility.
func (w *gzipResponseWriter) Flush() {
	if f, ok := w.Writer.(interface{ Flush() error }); ok {
		_ = f.Flush()
	}
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// compressionMiddleware adds gzip compression to responses.
func compressionMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if client accepts gzip
		if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			next.ServeHTTP(w, r)
			return
		}

		// Skip compression for health endpoints (low overhead)
		if strings.HasPrefix(r.URL.Path, "/health") ||
			strings.HasPrefix(r.URL.Path, "/ready") ||
			strings.HasPrefix(r.URL.Path, "/live") {
			next.ServeHTTP(w, r)
			return
		}

		w.Header().Set("Content-Encoding", "gzip")
		ctx := r.Context()
		gz := gzip.NewWriter(w)
		defer func() {
			if err := gz.Close(); err != nil {
				logging.FromContext(ctx, nil).Warn("failed to close gzip writer", "error", err)
			}
		}()

		gzw := &gzipResponseWriter{ResponseWriter: w, Writer: gz}
		next.ServeHTTP(gzw, r)
	})
}

// maxRequestSizeMiddleware limits the size of request bodies to prevent DoS attacks.
func maxRequestSizeMiddleware(maxSize int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Only limit request body for methods that have bodies
			if r.Method == http.MethodPost || r.Method == http.MethodPut || r.Method == http.MethodPatch {
				r.Body = http.MaxBytesReader(w, r.Body, maxSize)
			}
			next.ServeHTTP(w, r)
		})
	}
}

// panicRecoveryMiddleware recovers from panics and returns a 500 error.
// This prevents a single panic from crashing the entire server.
func panicRecoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		defer func() {
			if rec := recover(); rec != nil {
				// Capture stack trace for debugging
				stack := debug.Stack()

				logger := logging.FromContext(ctx, nil)
				logger.Error("panic recovered in HTTP handler",
					"panic", rec,
					"path", r.URL.Path,
					"method", r.Method,
					"stack", string(stack),
				)

				resp := domain.NewErrorResponse(domain.ErrCodeInternal, "internal server error")
				if requestID := logging.GetRequestID(ctx); requestID != "" {
					resp.WithRequestID(requestID)
				} else if headerID := w.Header().Get("X-Request-ID"); headerID != "" {
					resp.WithRequestID(headerID)
				}

				// Return 500 error
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				if err := json.NewEncoder(w).Encode(resp); err != nil {
					logger.Error("failed to encode panic response", "error", err)
				}
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// securityHeadersMiddleware adds security-related HTTP headers to responses.
func securityHeadersMiddleware(tlsEnabled bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Prevent MIME type sniffing
			w.Header().Set("X-Content-Type-Options", "nosniff")

			// Prevent clickjacking
			w.Header().Set("X-Frame-Options", "DENY")

			// Enable XSS filter (legacy browsers)
			w.Header().Set("X-XSS-Protection", "1; mode=block")

			// Referrer policy - only send origin for cross-origin requests
			w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")

			// Content Security Policy - restrict resource loading
			w.Header().Set("Content-Security-Policy", "default-src 'self'; frame-ancestors 'none'")

			// Permissions Policy - disable unnecessary browser features
			w.Header().Set("Permissions-Policy", "geolocation=(), microphone=(), camera=()")

			// HTTP Strict Transport Security (only when TLS is enabled)
			if tlsEnabled {
				// max-age=31536000 is 1 year; includeSubDomains for all subdomains
				w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
			}

			next.ServeHTTP(w, r)
		})
	}
}

// CORSConfig configures CORS behavior.
type CORSConfig struct {
	AllowedOrigins   []string
	AllowedMethods   []string
	AllowedHeaders   []string
	ExposedHeaders   []string
	AllowCredentials bool
	MaxAge           int // Preflight cache duration in seconds
}

// DefaultCORSConfig returns a restrictive CORS configuration.
// Configure AllowedOrigins explicitly for production use.
func DefaultCORSConfig() *CORSConfig {
	return &CORSConfig{
		AllowedOrigins:   []string{},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "HEAD", "PATCH"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-Request-ID"},
		ExposedHeaders:   []string{"X-Request-ID", "X-Total-Count"},
		AllowCredentials: false,
		MaxAge:           86400, // 24 hours
	}
}

// corsMiddleware adds CORS headers to responses.
func corsMiddleware(config *CORSConfig) func(http.Handler) http.Handler {
	if config == nil {
		config = DefaultCORSConfig()
	}

	allowedOrigins := make(map[string]bool)
	allowAll := false
	for _, origin := range config.AllowedOrigins {
		if origin == "*" {
			slog.Warn("CORS: wildcard '*' origin is not recommended for production — configure explicit AllowedOrigins instead")
			allowAll = true
			break
		}
		allowedOrigins[origin] = true
	}

	// When no explicit origins are configured and wildcard is not set,
	// no cross-origin requests will be allowed (secure default).
	if !allowAll && len(allowedOrigins) == 0 {
		slog.Info("CORS: no AllowedOrigins configured — cross-origin requests will be blocked")
	}

	// Prevent wildcard origin with credentials - this combination is a browser
	// security violation and would be rejected by browsers anyway.
	if allowAll && config.AllowCredentials {
		slog.Warn("CORS: disabling AllowCredentials because AllowedOrigins contains wildcard '*' — this combination is a browser security violation")
		config.AllowCredentials = false
	}

	methodsHeader := strings.Join(config.AllowedMethods, ", ")
	headersHeader := strings.Join(config.AllowedHeaders, ", ")
	exposedHeader := strings.Join(config.ExposedHeaders, ", ")
	maxAgeHeader := fmt.Sprintf("%d", config.MaxAge)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			// Check if origin is allowed
			if origin != "" {
				if allowAll {
					w.Header().Set("Access-Control-Allow-Origin", "*")
				} else if allowedOrigins[origin] {
					w.Header().Set("Access-Control-Allow-Origin", origin)
					w.Header().Set("Vary", "Origin")
				}

				if config.AllowCredentials {
					w.Header().Set("Access-Control-Allow-Credentials", "true")
				}

				if exposedHeader != "" {
					w.Header().Set("Access-Control-Expose-Headers", exposedHeader)
				}
			}

			// Handle preflight requests
			if r.Method == "OPTIONS" {
				w.Header().Set("Access-Control-Allow-Methods", methodsHeader)
				w.Header().Set("Access-Control-Allow-Headers", headersHeader)
				w.Header().Set("Access-Control-Max-Age", maxAgeHeader)
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
