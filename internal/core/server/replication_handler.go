package server

import (
	"encoding/json"
	"net/http"

	"github.com/feather-store/feather/internal/platform/replication"
)

// ReplicationHandler provides HTTP endpoints for multi-region replication.
type ReplicationHandler struct {
	manager *replication.Manager
}

// NewReplicationHandler creates a new replication handler.
func NewReplicationHandler(manager *replication.Manager) *ReplicationHandler {
	return &ReplicationHandler{manager: manager}
}

// RegisterRoutes registers replication API routes.
func (h *ReplicationHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/replication/regions", h.handleListRegions)
	mux.HandleFunc("POST /v1/replication/regions", h.handleAddRegion)
	mux.HandleFunc("GET /v1/replication/regions/{id}", h.handleGetRegion)
	mux.HandleFunc("DELETE /v1/replication/regions/{id}", h.handleRemoveRegion)
	mux.HandleFunc("POST /v1/replication/regions/{id}/drain", h.handleDrainRegion)
	mux.HandleFunc("POST /v1/replication/regions/{id}/activate", h.handleActivateRegion)
	mux.HandleFunc("GET /v1/replication/stats", h.handleStats)
	mux.HandleFunc("GET /v1/replication/pending", h.handlePendingEvents)
}

func (h *ReplicationHandler) handleListRegions(w http.ResponseWriter, r *http.Request) {
	if h.manager == nil {
		writeJSONError(r.Context(), w, http.StatusServiceUnavailable, "replication not configured")
		return
	}
	regions := h.manager.ListRegions()
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success": true, "regions": regions, "count": len(regions),
	})
}

func (h *ReplicationHandler) handleAddRegion(w http.ResponseWriter, r *http.Request) {
	if h.manager == nil {
		writeJSONError(r.Context(), w, http.StatusServiceUnavailable, "replication not configured")
		return
	}
	var region replication.Region
	if err := json.NewDecoder(r.Body).Decode(&region); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.manager.AddRegion(&region); err != nil {
		writeJSONError(r.Context(), w, http.StatusConflict, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusCreated, map[string]interface{}{"success": true, "region": region})
}

func (h *ReplicationHandler) handleGetRegion(w http.ResponseWriter, r *http.Request) {
	if h.manager == nil {
		writeJSONError(r.Context(), w, http.StatusServiceUnavailable, "replication not configured")
		return
	}
	region, err := h.manager.GetRegion(r.PathValue("id"))
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{"success": true, "region": region})
}

func (h *ReplicationHandler) handleRemoveRegion(w http.ResponseWriter, r *http.Request) {
	if h.manager == nil {
		writeJSONError(r.Context(), w, http.StatusServiceUnavailable, "replication not configured")
		return
	}
	if err := h.manager.RemoveRegion(r.PathValue("id")); err != nil {
		writeJSONError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{"success": true, "message": "region removed"})
}

func (h *ReplicationHandler) handleDrainRegion(w http.ResponseWriter, r *http.Request) {
	if h.manager == nil {
		writeJSONError(r.Context(), w, http.StatusServiceUnavailable, "replication not configured")
		return
	}
	if err := h.manager.DrainRegion(r.PathValue("id")); err != nil {
		writeJSONError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{"success": true, "message": "region draining"})
}

func (h *ReplicationHandler) handleActivateRegion(w http.ResponseWriter, r *http.Request) {
	if h.manager == nil {
		writeJSONError(r.Context(), w, http.StatusServiceUnavailable, "replication not configured")
		return
	}
	if err := h.manager.ActivateRegion(r.PathValue("id")); err != nil {
		writeJSONError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{"success": true, "message": "region activated"})
}

func (h *ReplicationHandler) handleStats(w http.ResponseWriter, r *http.Request) {
	if h.manager == nil {
		writeJSONError(r.Context(), w, http.StatusServiceUnavailable, "replication not configured")
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success": true, "stats": h.manager.Stats(),
	})
}

func (h *ReplicationHandler) handlePendingEvents(w http.ResponseWriter, r *http.Request) {
	if h.manager == nil {
		writeJSONError(r.Context(), w, http.StatusServiceUnavailable, "replication not configured")
		return
	}
	events := h.manager.GetPendingEvents()
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success": true, "events": events, "count": len(events),
	})
}
