package server

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/feather-store/feather/internal/extensions/adaptivecache"
)

// AdaptiveCacheHandler handles adaptive cache API requests.
type AdaptiveCacheHandler struct {
	predictor *adaptivecache.Predictor
}

// NewAdaptiveCacheHandler creates a new adaptive cache handler.
func NewAdaptiveCacheHandler(predictor *adaptivecache.Predictor) *AdaptiveCacheHandler {
	return &AdaptiveCacheHandler{predictor: predictor}
}

// RegisterRoutes registers adaptive cache API routes.
func (h *AdaptiveCacheHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/cache/record", h.handleRecordAccess)
	mux.HandleFunc("GET /v1/cache/predictions", h.handleGetPredictions)
	mux.HandleFunc("GET /v1/cache/promote/{key}", h.handleShouldPromote)
	mux.HandleFunc("POST /v1/cache/hit/{key}", h.handleRecordHit)
	mux.HandleFunc("POST /v1/cache/miss/{key}", h.handleRecordMiss)
	mux.HandleFunc("GET /v1/cache/adaptive/stats", h.handleGetStats)
}

// handleRecordAccess handles POST /v1/cache/record
func (h *AdaptiveCacheHandler) handleRecordAccess(w http.ResponseWriter, r *http.Request) {
	if h.predictor == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "adaptive cache not configured")
		return
	}

	var req struct {
		Key string `json:"key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Key == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "key is required")
		return
	}

	h.predictor.RecordAccess(req.Key)

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success": true,
		"key":     req.Key,
	})
}

// handleGetPredictions handles GET /v1/cache/predictions
func (h *AdaptiveCacheHandler) handleGetPredictions(w http.ResponseWriter, r *http.Request) {
	if h.predictor == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "adaptive cache not configured")
		return
	}

	topK := 10
	if v := r.URL.Query().Get("top_k"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			topK = parsed
		}
	}

	predictions := h.predictor.GetPredictions(topK)

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"predictions": predictions,
		"top_k":       topK,
	})
}

// handleShouldPromote handles GET /v1/cache/promote/{key}
func (h *AdaptiveCacheHandler) handleShouldPromote(w http.ResponseWriter, r *http.Request) {
	if h.predictor == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "adaptive cache not configured")
		return
	}

	key := r.PathValue("key")
	if key == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "key is required")
		return
	}

	shouldPromote := h.predictor.ShouldPromote(key)

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"key":            key,
		"should_promote": shouldPromote,
	})
}

// handleRecordHit handles POST /v1/cache/hit/{key}
func (h *AdaptiveCacheHandler) handleRecordHit(w http.ResponseWriter, r *http.Request) {
	if h.predictor == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "adaptive cache not configured")
		return
	}

	key := r.PathValue("key")
	if key == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "key is required")
		return
	}

	h.predictor.RecordHit(key)

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success": true,
		"key":     key,
		"event":   "hit",
	})
}

// handleRecordMiss handles POST /v1/cache/miss/{key}
func (h *AdaptiveCacheHandler) handleRecordMiss(w http.ResponseWriter, r *http.Request) {
	if h.predictor == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "adaptive cache not configured")
		return
	}

	key := r.PathValue("key")
	if key == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "key is required")
		return
	}

	h.predictor.RecordMiss(key)

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success": true,
		"key":     key,
		"event":   "miss",
	})
}

// handleGetStats handles GET /v1/cache/adaptive/stats
func (h *AdaptiveCacheHandler) handleGetStats(w http.ResponseWriter, r *http.Request) {
	if h.predictor == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "adaptive cache not configured")
		return
	}

	stats := h.predictor.Stats()

	h.writeJSON(r.Context(), w, http.StatusOK, stats)
}

func (h *AdaptiveCacheHandler) writeJSON(ctx context.Context, w http.ResponseWriter, status int, data interface{}) {
	writeJSONResponse(ctx, w, status, data)
}

func (h *AdaptiveCacheHandler) writeError(ctx context.Context, w http.ResponseWriter, status int, message string) {
	writeJSONError(ctx, w, status, message)
}
