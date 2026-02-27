package server

import (
	"context"
	"net/http"
	"strconv"

	"github.com/feather-store/feather/internal/platform/multiregion"
)

// RegionFederationHandler handles multi-region federation API requests.
type RegionFederationHandler struct {
	federation  *multiregion.Federation
	replication *multiregion.ReplicationManager
}

// NewRegionFederationHandler creates a new multi-region federation handler.
func NewRegionFederationHandler(fed *multiregion.Federation) *RegionFederationHandler {
	return &RegionFederationHandler{
		federation:  fed,
		replication: multiregion.NewReplicationManager(multiregion.DefaultFederationConfig()),
	}
}

// RegisterRoutes registers multi-region federation API routes.
func (h *RegionFederationHandler) RegisterRoutes(mux *http.ServeMux) {
	// Replication
	mux.HandleFunc("POST /v1/federation/replicate", h.handleEnqueueReplication)
	mux.HandleFunc("POST /v1/federation/apply", h.handleApplyReplication)
	mux.HandleFunc("POST /v1/federation/drain", h.handleDrainPending)
	mux.HandleFunc("GET /v1/federation/conflicts", h.handleGetConflicts)
	mux.HandleFunc("GET /v1/federation/replication/stats", h.handleReplicationStats)
}

func (h *RegionFederationHandler) handleEnqueueReplication(w http.ResponseWriter, r *http.Request) {
	var event multiregion.ReplicationEvent
	if err := strictDecode(r.Body, &event); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.replication.EnqueueReplication(event); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusOK, SuccessResponse{Success: true, Message: "event enqueued"})
}

func (h *RegionFederationHandler) handleApplyReplication(w http.ResponseWriter, r *http.Request) {
	var event multiregion.ReplicationEvent
	if err := strictDecode(r.Body, &event); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	conflict, err := h.replication.ApplyReplication(event)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"applied":  true,
		"conflict": conflict,
	})
}

func (h *RegionFederationHandler) handleDrainPending(w http.ResponseWriter, r *http.Request) {
	limitStr := r.URL.Query().Get("limit")
	limit := 100
	if limitStr != "" {
		if v, err := strconv.Atoi(limitStr); err == nil {
			limit = v
		}
	}

	events := h.replication.DrainPending(limit)
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"events": events,
		"count":  len(events),
	})
}

func (h *RegionFederationHandler) handleGetConflicts(w http.ResponseWriter, r *http.Request) {
	conflicts := h.replication.GetConflicts(100)
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"conflicts": conflicts,
		"count":     len(conflicts),
	})
}

func (h *RegionFederationHandler) handleReplicationStats(w http.ResponseWriter, r *http.Request) {
	stats := h.replication.Stats()
	h.writeJSON(r.Context(), w, http.StatusOK, stats)
}

func (h *RegionFederationHandler) writeJSON(ctx context.Context, w http.ResponseWriter, status int, data interface{}) {
	writeJSONResponse(ctx, w, status, data)
}

func (h *RegionFederationHandler) writeError(ctx context.Context, w http.ResponseWriter, status int, message string) {
	writeJSONError(ctx, w, status, message)
}
