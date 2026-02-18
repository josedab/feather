package server

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/feather-store/feather/internal/extensions/federateddiscovery"
)

// FederatedDiscoveryHandler handles federated discovery API requests.
type FederatedDiscoveryHandler struct {
	catalog *federateddiscovery.Catalog
}

// NewFederatedDiscoveryHandler creates a new federated discovery handler.
func NewFederatedDiscoveryHandler(catalog *federateddiscovery.Catalog) *FederatedDiscoveryHandler {
	return &FederatedDiscoveryHandler{catalog: catalog}
}

// RegisterRoutes registers federated discovery API routes.
func (h *FederatedDiscoveryHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/federation/catalog", h.handleListAll)
	mux.HandleFunc("POST /v1/federation/catalog", h.handlePublish)
	mux.HandleFunc("GET /v1/federation/catalog/{id}", h.handleGet)
	mux.HandleFunc("DELETE /v1/federation/catalog/{id}", h.handleUnpublish)
	mux.HandleFunc("POST /v1/federation/search", h.handleSearch)
	mux.HandleFunc("POST /v1/federation/subscribe", h.handleSubscribe)
	mux.HandleFunc("DELETE /v1/federation/subscribe", h.handleUnsubscribe)
	mux.HandleFunc("GET /v1/federation/catalog/{id}/subscribers", h.handleGetSubscribers)
	mux.HandleFunc("GET /v1/federation/subscriptions/{subscriber}", h.handleGetSubscriptions)
	mux.HandleFunc("GET /v1/federation/stats", h.handleGetStats)
}

// handleListAll handles GET /v1/federation/catalog
func (h *FederatedDiscoveryHandler) handleListAll(w http.ResponseWriter, r *http.Request) {
	if h.catalog == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "federated catalog not configured")
		return
	}

	entries := h.catalog.ListAll()

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"entries": entries,
	})
}

// handlePublish handles POST /v1/federation/catalog
func (h *FederatedDiscoveryHandler) handlePublish(w http.ResponseWriter, r *http.Request) {
	if h.catalog == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "federated catalog not configured")
		return
	}

	var entry federateddiscovery.CatalogEntry
	if err := json.NewDecoder(r.Body).Decode(&entry); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.catalog.Publish(entry); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusCreated, map[string]interface{}{
		"success": true,
		"id":      entry.ID,
	})
}

// handleGet handles GET /v1/federation/catalog/{id}
func (h *FederatedDiscoveryHandler) handleGet(w http.ResponseWriter, r *http.Request) {
	if h.catalog == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "federated catalog not configured")
		return
	}

	id := r.PathValue("id")
	if id == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "entry id is required")
		return
	}

	entry, err := h.catalog.Get(id)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, entry)
}

// handleUnpublish handles DELETE /v1/federation/catalog/{id}
func (h *FederatedDiscoveryHandler) handleUnpublish(w http.ResponseWriter, r *http.Request) {
	if h.catalog == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "federated catalog not configured")
		return
	}

	id := r.PathValue("id")
	if id == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "entry id is required")
		return
	}

	if err := h.catalog.Unpublish(id); err != nil {
		h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, SuccessResponse{Success: true, Message: "entry unpublished"})
}

// handleSearch handles POST /v1/federation/search
func (h *FederatedDiscoveryHandler) handleSearch(w http.ResponseWriter, r *http.Request) {
	if h.catalog == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "federated catalog not configured")
		return
	}

	var query federateddiscovery.SearchQuery
	if err := json.NewDecoder(r.Body).Decode(&query); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	results := h.catalog.Search(query)

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"results": results,
	})
}

// handleSubscribe handles POST /v1/federation/subscribe
func (h *FederatedDiscoveryHandler) handleSubscribe(w http.ResponseWriter, r *http.Request) {
	if h.catalog == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "federated catalog not configured")
		return
	}

	var req struct {
		EntryID    string `json:"entry_id"`
		Subscriber string `json:"subscriber"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	subscription, err := h.catalog.Subscribe(req.EntryID, req.Subscriber)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusCreated, subscription)
}

// handleUnsubscribe handles DELETE /v1/federation/subscribe
func (h *FederatedDiscoveryHandler) handleUnsubscribe(w http.ResponseWriter, r *http.Request) {
	if h.catalog == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "federated catalog not configured")
		return
	}

	var req struct {
		EntryID    string `json:"entry_id"`
		Subscriber string `json:"subscriber"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.catalog.Unsubscribe(req.EntryID, req.Subscriber); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, SuccessResponse{Success: true, Message: "unsubscribed"})
}

// handleGetSubscribers handles GET /v1/federation/catalog/{id}/subscribers
func (h *FederatedDiscoveryHandler) handleGetSubscribers(w http.ResponseWriter, r *http.Request) {
	if h.catalog == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "federated catalog not configured")
		return
	}

	id := r.PathValue("id")
	if id == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "entry id is required")
		return
	}

	subscribers := h.catalog.GetSubscribers(id)

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"subscribers": subscribers,
	})
}

// handleGetSubscriptions handles GET /v1/federation/subscriptions/{subscriber}
func (h *FederatedDiscoveryHandler) handleGetSubscriptions(w http.ResponseWriter, r *http.Request) {
	if h.catalog == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "federated catalog not configured")
		return
	}

	subscriber := r.PathValue("subscriber")
	if subscriber == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "subscriber is required")
		return
	}

	entries := h.catalog.GetSubscriptions(subscriber)

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"entries": entries,
	})
}

// handleGetStats handles GET /v1/federation/stats
func (h *FederatedDiscoveryHandler) handleGetStats(w http.ResponseWriter, r *http.Request) {
	if h.catalog == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "federated catalog not configured")
		return
	}

	stats := h.catalog.Stats()

	h.writeJSON(r.Context(), w, http.StatusOK, stats)
}

func (h *FederatedDiscoveryHandler) writeJSON(ctx context.Context, w http.ResponseWriter, status int, data interface{}) {
	writeJSONResponse(ctx, w, status, data)
}

func (h *FederatedDiscoveryHandler) writeError(ctx context.Context, w http.ResponseWriter, status int, message string) {
	writeJSONError(ctx, w, status, message)
}
