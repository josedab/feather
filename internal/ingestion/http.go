package ingestion

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/feather-store/feather/internal/aggregation"
	"github.com/feather-store/feather/internal/clientip"
	"github.com/feather-store/feather/internal/domain"
	"github.com/feather-store/feather/internal/logging"
	"github.com/feather-store/feather/internal/storage"
)

// HTTPIngestion handles HTTP-based feature ingestion.
type HTTPIngestion struct {
	store       *storage.Store
	agg         *aggregation.Engine
	schema      *storage.Registry
	metrics     *HTTPIngestionMetrics
	rateLimiter *rateLimiter
	ipResolver  *clientip.Resolver
	config      HTTPIngestionConfig
}

// HTTPIngestionConfig configures the HTTP ingestion handler.
type HTTPIngestionConfig struct {
	// RateLimitEnabled enables rate limiting.
	RateLimitEnabled bool
	// RateLimitPerSecond is the max requests per second per client.
	RateLimitPerSecond int
	// RateLimitBurst is the burst allowance.
	RateLimitBurst int
	// ValidateSchema enables strict schema validation (reject invalid features).
	ValidateSchema bool
	// TrustedProxies is a list of CIDR ranges for trusted proxy servers.
	// When a request comes from a trusted proxy, X-Forwarded-For header is used
	// to determine the real client IP. If empty, proxy headers are not trusted.
	TrustedProxies []string
	// MaxRequestSize is the maximum allowed request body size in bytes.
	// If 0, defaults to DefaultMaxRequestSize (1MB).
	MaxRequestSize int64
}

// DefaultMaxRequestSize is the default maximum request body size (1MB).
const DefaultMaxRequestSize = 1 << 20 // 1MB

// HTTPIngestionMetrics tracks HTTP ingestion performance.
type HTTPIngestionMetrics struct {
	RequestsReceived int64
	RequestsSuccess  int64
	RequestsError    int64
	FeaturesIngested int64
}

// NewHTTPIngestion creates a new HTTP ingestion handler.
func NewHTTPIngestion(
	store *storage.Store,
	agg *aggregation.Engine,
	schema *storage.Registry,
) *HTTPIngestion {
	return NewHTTPIngestionWithConfig(store, agg, schema, HTTPIngestionConfig{
		ValidateSchema: true, // Enable by default
	})
}

// NewHTTPIngestionWithConfig creates a new HTTP ingestion handler with config.
func NewHTTPIngestionWithConfig(
	store *storage.Store,
	agg *aggregation.Engine,
	schema *storage.Registry,
	config HTTPIngestionConfig,
) *HTTPIngestion {
	// Create IP resolver for secure client IP extraction
	ipResolver, err := clientip.NewResolver(config.TrustedProxies)
	if err != nil {
		// Fall back to safe default (trust no proxies)
		ipResolver, _ = clientip.NewResolver(nil)
	}

	h := &HTTPIngestion{
		store:      store,
		agg:        agg,
		schema:     schema,
		metrics:    &HTTPIngestionMetrics{},
		ipResolver: ipResolver,
		config:     config,
	}

	if config.RateLimitEnabled {
		rps := config.RateLimitPerSecond
		if rps == 0 {
			rps = 100 // default
		}
		burst := config.RateLimitBurst
		if burst == 0 {
			burst = 200 // default
		}
		h.rateLimiter = newRateLimiter(rps, burst)
	}

	return h
}

// maxRequestSize returns the configured max request size or the default.
func (h *HTTPIngestion) maxRequestSize() int64 {
	if h.config.MaxRequestSize > 0 {
		return h.config.MaxRequestSize
	}
	return DefaultMaxRequestSize
}

// HandlePush handles POST /ingest for single feature updates.
func (h *HTTPIngestion) HandlePush(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&h.metrics.RequestsReceived, 1)

	// Limit request body size to prevent DoS
	r.Body = http.MaxBytesReader(w, r.Body, h.maxRequestSize())

	// Check rate limit
	if h.rateLimiter != nil {
		clientIP := h.ipResolver.GetClientIP(r)
		if !h.rateLimiter.allow(clientIP) {
			atomic.AddInt64(&h.metrics.RequestsError, 1)
			w.Header().Set("Retry-After", "1")
			writeError(w, http.StatusTooManyRequests, "rate limit exceeded")
			return
		}
	}

	var update domain.FeatureUpdate
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		atomic.AddInt64(&h.metrics.RequestsError, 1)
		// Check if the error is due to body size limit
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			writeError(w, http.StatusRequestEntityTooLarge, "request body too large")
			return
		}
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if update.EntityKey == "" {
		atomic.AddInt64(&h.metrics.RequestsError, 1)
		writeError(w, http.StatusBadRequest, "entity_key required")
		return
	}

	if err := h.ingestUpdate(&update); err != nil {
		atomic.AddInt64(&h.metrics.RequestsError, 1)
		if _, ok := err.(*ValidationError); ok {
			writeError(w, http.StatusBadRequest, err.Error())
		} else {
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	atomic.AddInt64(&h.metrics.RequestsSuccess, 1)
	atomic.AddInt64(&h.metrics.FeaturesIngested, int64(len(update.Features)))

	writeJSON(w, http.StatusCreated, map[string]bool{"success": true})
}

// HandleBulkPush handles POST /ingest/bulk for bulk feature updates.
func (h *HTTPIngestion) HandleBulkPush(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&h.metrics.RequestsReceived, 1)

	// Limit request body size to prevent DoS (use 10x default for bulk)
	r.Body = http.MaxBytesReader(w, r.Body, h.maxRequestSize()*10)

	var updates []domain.FeatureUpdate
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		atomic.AddInt64(&h.metrics.RequestsError, 1)
		// Check if the error is due to body size limit
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			writeError(w, http.StatusRequestEntityTooLarge, "request body too large")
			return
		}
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	successCount := 0
	errorCount := 0

	for _, update := range updates {
		if update.EntityKey == "" {
			errorCount++
			continue
		}

		if err := h.ingestUpdate(&update); err != nil {
			errorCount++
			continue
		}

		successCount++
		atomic.AddInt64(&h.metrics.FeaturesIngested, int64(len(update.Features)))
	}

	atomic.AddInt64(&h.metrics.RequestsSuccess, 1)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": successCount,
		"errors":  errorCount,
		"total":   len(updates),
	})
}

func (h *HTTPIngestion) ingestUpdate(update *domain.FeatureUpdate) error {
	if update.Timestamp == 0 {
		update.Timestamp = time.Now().UnixNano()
	}

	// Validate features if schema is available and validation is enabled
	if h.schema != nil && h.config.ValidateSchema {
		var validationErrors []string
		for name, val := range update.Features {
			if err := h.schema.Validate(name, val); err != nil {
				validationErrors = append(validationErrors, fmt.Sprintf("%s: %v", name, err))
			}
		}
		if len(validationErrors) > 0 {
			return &ValidationError{Errors: validationErrors}
		}
	}

	// Store features
	features := make(map[string]*domain.FeatureValue)
	for name, val := range update.Features {
		features[name] = &domain.FeatureValue{
			Value:     val,
			Timestamp: update.Timestamp,
			Version:   update.Version,
		}
	}

	if err := h.store.Put(update.EntityKey, features); err != nil {
		return err
	}

	// Update aggregations
	for name, val := range update.Features {
		if h.agg.GetSpec(name) != nil {
			if floatVal, ok := toFloat64(val); ok {
				h.agg.Update(update.EntityKey, name, floatVal, time.Unix(0, update.Timestamp))
			}
		}
	}

	return nil
}

// ValidationError represents schema validation errors.
type ValidationError struct {
	Errors []string
}

func (e *ValidationError) Error() string {
	if len(e.Errors) == 1 {
		return fmt.Sprintf("validation error: %s", e.Errors[0])
	}
	return fmt.Sprintf("validation errors: %v", e.Errors)
}

// Metrics returns current metrics.
func (h *HTTPIngestion) Metrics() HTTPIngestionMetrics {
	return HTTPIngestionMetrics{
		RequestsReceived: atomic.LoadInt64(&h.metrics.RequestsReceived),
		RequestsSuccess:  atomic.LoadInt64(&h.metrics.RequestsSuccess),
		RequestsError:    atomic.LoadInt64(&h.metrics.RequestsError),
		FeaturesIngested: atomic.LoadInt64(&h.metrics.FeaturesIngested),
	}
}

// RegisterRoutes registers ingestion routes on the given mux.
func (h *HTTPIngestion) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /ingest", h.HandlePush)
	mux.HandleFunc("POST /ingest/bulk", h.HandleBulkPush)
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		logging.FromContext(context.Background(), nil).Error("failed to encode JSON response", "error", err)
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

// toFloat64 converts a value to float64 if possible.
func toFloat64(v interface{}) (float64, bool) {
	switch val := v.(type) {
	case float64:
		return val, true
	case float32:
		return float64(val), true
	case int:
		return float64(val), true
	case int64:
		return float64(val), true
	case int32:
		return float64(val), true
	default:
		return 0, false
	}
}

// rateLimiter implements a simple token bucket rate limiter per client.
type rateLimiter struct {
	mu       sync.Mutex
	clients  map[string]*tokenBucket
	rps      int
	burst    int
	cleanupT time.Time
}

type tokenBucket struct {
	tokens     float64
	lastRefill time.Time
}

func newRateLimiter(rps, burst int) *rateLimiter {
	return &rateLimiter{
		clients:  make(map[string]*tokenBucket),
		rps:      rps,
		burst:    burst,
		cleanupT: time.Now(),
	}
}

func (rl *rateLimiter) allow(clientID string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()

	// Periodic cleanup of stale clients
	if now.Sub(rl.cleanupT) > time.Minute {
		for id, bucket := range rl.clients {
			if now.Sub(bucket.lastRefill) > 5*time.Minute {
				delete(rl.clients, id)
			}
		}
		rl.cleanupT = now
	}

	bucket, ok := rl.clients[clientID]
	if !ok {
		bucket = &tokenBucket{
			tokens:     float64(rl.burst),
			lastRefill: now,
		}
		rl.clients[clientID] = bucket
	}

	// Refill tokens based on time elapsed
	elapsed := now.Sub(bucket.lastRefill).Seconds()
	bucket.tokens += elapsed * float64(rl.rps)
	if bucket.tokens > float64(rl.burst) {
		bucket.tokens = float64(rl.burst)
	}
	bucket.lastRefill = now

	// Check if we have tokens available
	if bucket.tokens < 1 {
		return false
	}

	bucket.tokens--
	return true
}
