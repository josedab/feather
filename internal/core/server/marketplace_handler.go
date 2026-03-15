package server

import (
	"net/http"
	"strings"

	"github.com/feather-store/feather/internal/extensions/marketplace"
)

// MarketplaceHandler provides HTTP endpoints for the feature marketplace.
type MarketplaceHandler struct {
	catalog *marketplace.Catalog
}

// NewMarketplaceHandler creates a new marketplace handler.
func NewMarketplaceHandler(catalog *marketplace.Catalog) *MarketplaceHandler {
	return &MarketplaceHandler{catalog: catalog}
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
	features := h.catalog.List()

	// Apply limit/offset pagination
	limit, offset := parsePagination(r, 100, 1000)
	total := len(features)
	if offset > total {
		offset = total
	}
	end := offset + limit
	if end > total {
		end = total
	}
	page := features[offset:end]

	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success":  true,
		"features": page,
		"total":    total,
		"limit":    limit,
		"offset":   offset,
	})
}

func (h *MarketplaceHandler) handlePublishFeature(w http.ResponseWriter, r *http.Request) {
	var feat marketplace.PublishedFeature
	if err := strictDecode(r.Body, &feat); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	if err := h.catalog.Publish(&feat); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSONResponse(r.Context(), w, http.StatusCreated, map[string]interface{}{
		"success": true,
		"feature": feat,
	})
}

func (h *MarketplaceHandler) handleGetFeature(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	feat, err := h.catalog.Get(id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeJSONError(r.Context(), w, http.StatusNotFound, err.Error())
			return
		}
		writeJSONError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success": true,
		"feature": feat,
	})
}

func (h *MarketplaceHandler) handleDeprecateFeature(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.catalog.Deprecate(id); err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeJSONError(r.Context(), w, http.StatusNotFound, err.Error())
			return
		}
		writeJSONError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "feature " + id + " deprecated",
	})
}

func (h *MarketplaceHandler) handleSubscribe(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req struct {
		SubscriberID string `json:"subscriber_id"`
		Team         string `json:"team"`
	}
	if err := strictDecode(r.Body, &req); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	if req.SubscriberID == "" {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "subscriber_id is required")
		return
	}

	sub, err := h.catalog.Subscribe(id, req.SubscriberID, req.Team)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeJSONError(r.Context(), w, http.StatusNotFound, err.Error())
			return
		}
		if strings.Contains(err.Error(), "not published") {
			writeJSONError(r.Context(), w, http.StatusConflict, err.Error())
			return
		}
		writeJSONError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSONResponse(r.Context(), w, http.StatusCreated, map[string]interface{}{
		"success":      true,
		"subscription": sub,
	})
}

func (h *MarketplaceHandler) handleUnsubscribe(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req struct {
		SubscriberID string `json:"subscriber_id"`
	}
	if err := strictDecode(r.Body, &req); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	if req.SubscriberID == "" {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "subscriber_id is required")
		return
	}

	if err := h.catalog.Unsubscribe(id, req.SubscriberID); err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeJSONError(r.Context(), w, http.StatusNotFound, err.Error())
			return
		}
		writeJSONError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success":      true,
		"feature_id":   id,
		"unsubscribed": true,
	})
}

func (h *MarketplaceHandler) handleGetSubscribers(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	subscribers := h.catalog.GetSubscribers(id)

	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success":     true,
		"subscribers": subscribers,
		"total":       len(subscribers),
	})
}

func (h *MarketplaceHandler) handleSearch(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	limit, offset := parsePagination(r, 100, 1000)

	filter := marketplace.SearchFilter{
		Query:      query.Get("q"),
		Owner:      query.Get("owner"),
		Team:       query.Get("team"),
		EntityType: query.Get("entity_type"),
		Limit:      limit,
		Offset:     offset,
	}

	if tags := query.Get("tags"); tags != "" {
		filter.Tags = strings.Split(tags, ",")
	}
	if status := query.Get("status"); status != "" {
		filter.Status = marketplace.FeatureStatus(status)
	}
	if q := query.Get("quality"); q != "" {
		filter.Quality = marketplace.QualityTier(q)
	}

	results := h.catalog.Search(filter)

	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success": true,
		"results": results,
		"total":   len(results),
	})
}

func (h *MarketplaceHandler) handleMarketplaceStats(w http.ResponseWriter, r *http.Request) {
	stats := h.catalog.Stats()
	stats["success"] = true

	writeJSONResponse(r.Context(), w, http.StatusOK, stats)
}
