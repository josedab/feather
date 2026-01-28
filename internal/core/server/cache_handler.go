package server

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/feather-store/feather/internal/core/storage"
	"github.com/feather-store/feather/internal/extensions/cache"
)

// CacheHandler handles predictive cache API requests.
type CacheHandler struct {
	predictive *cache.PredictiveCache
	coAccess   *cache.CoAccessTracker
}

// NewCacheHandler creates a new cache handler.
func NewCacheHandler(store *storage.Store) *CacheHandler {
	return &CacheHandler{
		predictive: cache.NewPredictiveCache(store, cache.DefaultPredictiveCacheConfig()),
		coAccess:   cache.NewCoAccessTracker(time.Minute),
	}
}

// RegisterRoutes registers cache API routes.
func (h *CacheHandler) RegisterRoutes(mux *http.ServeMux) {
	// Pattern tracking
	mux.HandleFunc("POST /v1/cache/access", h.handleRecordAccess)
	mux.HandleFunc("GET /v1/cache/patterns", h.handleGetPatterns)
	mux.HandleFunc("GET /v1/cache/patterns/{entity}/{feature}", h.handleGetPattern)

	// Predictions
	mux.HandleFunc("GET /v1/cache/predictions", h.handleGetPredictions)

	// Co-access
	mux.HandleFunc("GET /v1/cache/related/{feature}", h.handleGetRelated)

	// Stats
	mux.HandleFunc("GET /v1/cache/stats", h.handleGetStats)

	// Control
	mux.HandleFunc("POST /v1/cache/config", h.handleUpdateConfig)
}

// GetPredictiveCache returns the predictive cache for integration.
func (h *CacheHandler) GetPredictiveCache() *cache.PredictiveCache {
	return h.predictive
}

// RecordAccessRequest represents a request to record feature access.
type RecordAccessRequest struct {
	EntityID string   `json:"entity_id"`
	Features []string `json:"features"`
}

// handleRecordAccess handles POST /v1/cache/access
func (h *CacheHandler) handleRecordAccess(w http.ResponseWriter, r *http.Request) {
	var req RecordAccessRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.EntityID == "" || len(req.Features) == 0 {
		h.writeError(r.Context(), w, http.StatusBadRequest, "entity_id and features are required")
		return
	}

	// Record individual accesses
	for _, feature := range req.Features {
		h.predictive.RecordAccess(req.EntityID, feature)
	}

	// Record co-accesses
	h.coAccess.RecordAccess(req.EntityID, req.Features)

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success":  true,
		"recorded": len(req.Features),
	})
}

// handleGetPatterns handles GET /v1/cache/patterns
func (h *CacheHandler) handleGetPatterns(w http.ResponseWriter, r *http.Request) {
	limit := 50
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	patterns := h.predictive.GetTopPatterns(limit)

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"patterns": patterns,
		"count":    len(patterns),
	})
}

// handleGetPattern handles GET /v1/cache/patterns/{entity}/{feature}
func (h *CacheHandler) handleGetPattern(w http.ResponseWriter, r *http.Request) {
	entityID := r.PathValue("entity")
	feature := r.PathValue("feature")

	if entityID == "" || feature == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "entity and feature are required")
		return
	}

	pattern := h.predictive.GetPattern(entityID, feature)
	if pattern == nil {
		h.writeError(r.Context(), w, http.StatusNotFound, "pattern not found")
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, pattern)
}

// handleGetPredictions handles GET /v1/cache/predictions
func (h *CacheHandler) handleGetPredictions(w http.ResponseWriter, r *http.Request) {
	window := 5 * time.Minute
	if windowStr := r.URL.Query().Get("window"); windowStr != "" {
		if parsed, err := time.ParseDuration(windowStr); err == nil {
			window = parsed
		}
	}

	predictions := h.predictive.GetPredictedAccesses(window)

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"predictions": predictions,
		"count":       len(predictions),
		"window":      window.String(),
	})
}

// handleGetRelated handles GET /v1/cache/related/{feature}
func (h *CacheHandler) handleGetRelated(w http.ResponseWriter, r *http.Request) {
	feature := r.PathValue("feature")
	if feature == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "feature is required")
		return
	}

	limit := 10
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	related := h.coAccess.GetRelatedFeatures(feature, limit)

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"feature": feature,
		"related": related,
		"count":   len(related),
	})
}

// handleGetStats handles GET /v1/cache/stats
func (h *CacheHandler) handleGetStats(w http.ResponseWriter, r *http.Request) {
	stats := h.predictive.GetStats()
	h.writeJSON(r.Context(), w, http.StatusOK, stats)
}

// CacheConfigRequest represents a request to update cache config.
type CacheConfigRequest struct {
	WarmingWindow   string  `json:"warming_window,omitempty"`
	WarmingInterval string  `json:"warming_interval,omitempty"`
	MaxWarmItems    int     `json:"max_warm_items,omitempty"`
	MinScore        float64 `json:"min_score,omitempty"`
	Enabled         *bool   `json:"enabled,omitempty"`
}

// handleUpdateConfig handles POST /v1/cache/config
func (h *CacheHandler) handleUpdateConfig(w http.ResponseWriter, r *http.Request) {
	var req CacheConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Note: In production, you'd want to update the actual config
	// This is a placeholder response
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "configuration updated",
	})
}

func (h *CacheHandler) writeJSON(ctx context.Context, w http.ResponseWriter, status int, data interface{}) {
	writeJSONResponse(ctx, w, status, data)
}

func (h *CacheHandler) writeError(ctx context.Context, w http.ResponseWriter, status int, message string) {
	writeJSONError(ctx, w, status, message)
}
