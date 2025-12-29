package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/feather-store/feather/internal/clientip"
	"github.com/feather-store/feather/internal/logging"
)

// Middleware provides authentication and authorization middleware.
type Middleware struct {
	controller  *AccessController
	rateLimiter *RateLimiter
	ipResolver  *clientip.Resolver
}

// NewMiddleware creates new auth middleware.
// The ipResolver parameter controls how client IPs are extracted from requests.
// If nil, a resolver that trusts no proxies is used (safest default).
func NewMiddleware(controller *AccessController, ipResolver *clientip.Resolver) *Middleware {
	if ipResolver == nil {
		// Safe default: trust no proxies
		ipResolver, _ = clientip.NewResolver(nil)
	}
	return &Middleware{
		controller:  controller,
		rateLimiter: NewRateLimiter(),
		ipResolver:  ipResolver,
	}
}

// Stop stops the middleware's background goroutines.
// This should be called during shutdown to prevent goroutine leaks.
func (m *Middleware) Stop() {
	if m.rateLimiter != nil {
		m.rateLimiter.Stop()
	}
}

// Authenticate validates API key and adds auth info to request context.
func (m *Middleware) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract API key from header
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			// Also check X-API-Key header
			authHeader = r.Header.Get("X-API-Key")
		}

		if authHeader == "" {
			writeAuthError(w, http.StatusUnauthorized, "API key required")
			return
		}

		// Remove "Bearer " prefix if present
		apiKey := strings.TrimPrefix(authHeader, "Bearer ")

		// Validate API key
		key, err := m.controller.ValidateAPIKey(apiKey)
		if err != nil {
			writeAuthError(w, http.StatusUnauthorized, err.Error())
			return
		}

		// Check rate limit
		if key.RateLimit > 0 {
			if !m.rateLimiter.Allow(key.ID, key.RateLimit) {
				writeAuthError(w, http.StatusTooManyRequests, "rate limit exceeded")
				return
			}
		}

		// Add auth info to context
		ctx := WithAPIKey(r.Context(), key)
		ctx = WithTenant(ctx, key.Tenant)

		// Log audit
		m.controller.LogAudit(AuditLog{
			Tenant:    key.Tenant,
			APIKeyID:  key.ID,
			Action:    r.Method,
			Resource:  r.URL.Path,
			IP:        m.ipResolver.GetClientIP(r),
			UserAgent: r.UserAgent(),
			Success:   true,
		})

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequirePermission creates middleware that requires a specific permission.
func (m *Middleware) RequirePermission(perm Permission) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := APIKeyFromContext(r.Context())
			if key == nil {
				writeAuthError(w, http.StatusUnauthorized, "authentication required")
				return
			}

			if !m.controller.HasPermission(key, perm) {
				m.controller.LogAudit(AuditLog{
					Tenant:   key.Tenant,
					APIKeyID: key.ID,
					Action:   r.Method,
					Resource: r.URL.Path,
					IP:       m.ipResolver.GetClientIP(r),
					Success:  false,
					Error:    "permission denied: " + string(perm),
				})
				writeAuthError(w, http.StatusForbidden, "permission denied")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RequireNamespace creates middleware that validates namespace access.
func (m *Middleware) RequireNamespace(getNamespace func(*http.Request) string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := APIKeyFromContext(r.Context())
			if key == nil {
				writeAuthError(w, http.StatusUnauthorized, "authentication required")
				return
			}

			namespace := getNamespace(r)
			if namespace != "" && !m.controller.CanAccessNamespace(key, namespace) {
				writeAuthError(w, http.StatusForbidden, "access to namespace denied")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RequireFeature creates middleware that validates feature access.
func (m *Middleware) RequireFeature(getFeature func(*http.Request) string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := APIKeyFromContext(r.Context())
			if key == nil {
				writeAuthError(w, http.StatusUnauthorized, "authentication required")
				return
			}

			feature := getFeature(r)
			if feature != "" && !m.controller.CanAccessFeature(key, feature) {
				writeAuthError(w, http.StatusForbidden, "access to feature denied")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// Optional allows requests without API key but enriches context if present.
func (m *Middleware) Optional(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			authHeader = r.Header.Get("X-API-Key")
		}

		if authHeader != "" {
			apiKey := strings.TrimPrefix(authHeader, "Bearer ")
			if key, err := m.controller.ValidateAPIKey(apiKey); err == nil {
				ctx := WithAPIKey(r.Context(), key)
				ctx = WithTenant(ctx, key.Tenant)
				r = r.WithContext(ctx)
			}
		}

		next.ServeHTTP(w, r)
	})
}

func writeAuthError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(map[string]string{"error": message}); err != nil {
		logging.FromContext(context.Background(), nil).Error("failed to encode auth error response", "error", err)
	}
}

// RateLimiter provides token bucket rate limiting.
type RateLimiter struct {
	buckets map[string]*tokenBucket
	mu      sync.Mutex
	stopCh  chan struct{}
}

type tokenBucket struct {
	tokens    float64
	lastTime  time.Time
	rateLimit int // tokens per minute
}

// NewRateLimiter creates a new rate limiter.
func NewRateLimiter() *RateLimiter {
	rl := &RateLimiter{
		buckets: make(map[string]*tokenBucket),
		stopCh:  make(chan struct{}),
	}

	// Start cleanup goroutine
	go rl.cleanup()

	return rl
}

// Stop stops the rate limiter's cleanup goroutine.
// This should be called during shutdown to prevent goroutine leaks.
func (rl *RateLimiter) Stop() {
	close(rl.stopCh)
}

// Allow checks if a request is allowed under rate limiting.
func (rl *RateLimiter) Allow(key string, rateLimit int) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	bucket, ok := rl.buckets[key]

	if !ok {
		bucket = &tokenBucket{
			tokens:    float64(rateLimit),
			lastTime:  now,
			rateLimit: rateLimit,
		}
		rl.buckets[key] = bucket
	}

	// Refill tokens based on time elapsed
	elapsed := now.Sub(bucket.lastTime).Minutes()
	bucket.tokens += elapsed * float64(bucket.rateLimit)
	if bucket.tokens > float64(bucket.rateLimit) {
		bucket.tokens = float64(bucket.rateLimit)
	}
	bucket.lastTime = now

	// Check if we have a token
	if bucket.tokens >= 1 {
		bucket.tokens--
		return true
	}

	return false
}

func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-rl.stopCh:
			return
		case <-ticker.C:
			rl.mu.Lock()
			now := time.Now()
			for key, bucket := range rl.buckets {
				if now.Sub(bucket.lastTime) > 10*time.Minute {
					delete(rl.buckets, key)
				}
			}
			rl.mu.Unlock()
		}
	}
}
