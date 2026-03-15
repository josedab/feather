package server

import (
	"net/http"

	"github.com/feather-store/feather/internal/extensions/sharding"
)

// ShardingHandler provides HTTP endpoints for the sharding router.
type ShardingHandler struct {
	router *sharding.Router
}

// NewShardingHandler creates a new sharding handler.
func NewShardingHandler(router *sharding.Router) *ShardingHandler {
	return &ShardingHandler{router: router}
}

// RegisterRoutes registers sharding API routes.
func (h *ShardingHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/sharding/stats", h.handleGetStats)
	mux.HandleFunc("GET /v1/sharding/partition", h.handleGetPartition)
	mux.HandleFunc("GET /v1/sharding/owners", h.handleGetOwners)
	mux.HandleFunc("POST /v1/sharding/recompute", h.handleRecompute)
}

func (h *ShardingHandler) handleGetStats(w http.ResponseWriter, r *http.Request) {
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success": true,
		"stats":   h.router.Stats(),
	})
}

func (h *ShardingHandler) handleGetPartition(w http.ResponseWriter, r *http.Request) {
	key := r.URL.Query().Get("key")
	if key == "" {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "key parameter is required")
		return
	}
	partition := h.router.GetPartitionForKey(key)
	owners := h.router.GetOwnersForKey(key)
	isLocal := h.router.IsLocalKey(key)

	ownerIDs := make([]string, len(owners))
	for i, o := range owners {
		ownerIDs[i] = o.ID
	}

	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success":   true,
		"key":       key,
		"partition": partition,
		"owners":    ownerIDs,
		"is_local":  isLocal,
	})
}

func (h *ShardingHandler) handleGetOwners(w http.ResponseWriter, r *http.Request) {
	key := r.URL.Query().Get("key")
	if key == "" {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "key parameter is required")
		return
	}
	owners := h.router.GetOwnersForKey(key)
	ownerIDs := make([]string, len(owners))
	for i, o := range owners {
		ownerIDs[i] = o.ID
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success": true,
		"owners":  ownerIDs,
	})
}

func (h *ShardingHandler) handleRecompute(w http.ResponseWriter, r *http.Request) {
	h.router.Recompute()
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "partition map recomputed",
	})
}


