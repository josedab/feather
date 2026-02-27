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

// MarketplaceHandler provides HTTP endpoints for the feature marketplace.
type MarketplaceHandler struct{}

// NewMarketplaceHandler creates a new marketplace handler.
func NewMarketplaceHandler() *MarketplaceHandler {
	return &MarketplaceHandler{}
}

// RegisterRoutes registers marketplace API routes.
func (h *MarketplaceHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/marketplace/features", h.handleListFeatures)
	mux.HandleFunc("POST /v1/marketplace/features", h.handlePublishFeature)
	mux.HandleFunc("GET /v1/marketplace/features/{id}", h.handleGetFeature)
	mux.HandleFunc("POST /v1/marketplace/features/{id}/deprecate", h.handleDeprecateFeature)
	mux.HandleFunc("POST /v1/marketplace/features/{id}/subscribe", h.handleSubscribe)
	mux.HandleFunc("DELETE /v1/marketplace/features/{id}/subscribe", h.handleUnsubscribe)
	mux.HandleFunc("GET /v1/marketplace/features/{id}/subscribers", h.handleGetSubscribers)
	mux.HandleFunc("GET /v1/marketplace/search", h.handleSearch)
	mux.HandleFunc("GET /v1/marketplace/stats", h.handleMarketplaceStats)
}

func (h *MarketplaceHandler) handleListFeatures(w http.ResponseWriter, r *http.Request) {
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success":  true,
		"features": []interface{}{},
	})
}

func (h *MarketplaceHandler) handlePublishFeature(w http.ResponseWriter, r *http.Request) {
	var req map[string]interface{}
	if err := strictDecode(r.Body, &req); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusCreated, map[string]interface{}{
		"success": true,
		"feature": req,
	})
}

func (h *MarketplaceHandler) handleGetFeature(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success": true,
		"feature": map[string]string{"id": id},
	})
}

func (h *MarketplaceHandler) handleDeprecateFeature(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "feature " + id + " deprecated",
	})
}

func (h *MarketplaceHandler) handleSubscribe(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	writeJSONResponse(r.Context(), w, http.StatusCreated, map[string]interface{}{
		"success":    true,
		"feature_id": id,
		"subscribed": true,
	})
}

func (h *MarketplaceHandler) handleUnsubscribe(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success":      true,
		"feature_id":   id,
		"unsubscribed": true,
	})
}

func (h *MarketplaceHandler) handleGetSubscribers(w http.ResponseWriter, r *http.Request) {
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success":     true,
		"subscribers": []interface{}{},
	})
}

func (h *MarketplaceHandler) handleSearch(w http.ResponseWriter, r *http.Request) {
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success": true,
		"results": []interface{}{},
	})
}

func (h *MarketplaceHandler) handleMarketplaceStats(w http.ResponseWriter, r *http.Request) {
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success": true,
		"stats":   map[string]int{"total_features": 0, "total_subscriptions": 0},
	})
}
