package server

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/feather-store/feather/internal/extensions/featuredashboard"
)

// FeatureDashboardHandler handles observability dashboard API requests.
type FeatureDashboardHandler struct {
	dashboard *featuredashboard.Dashboard
}

// NewFeatureDashboardHandler creates a new feature dashboard handler.
func NewFeatureDashboardHandler(d *featuredashboard.Dashboard) *FeatureDashboardHandler {
	return &FeatureDashboardHandler{dashboard: d}
}

// RegisterRoutes registers dashboard API routes.
func (h *FeatureDashboardHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/dashboard/snapshot", h.handleGetSnapshot)
	mux.HandleFunc("POST /v1/dashboard/snapshot", h.handleTakeSnapshot)
	mux.HandleFunc("GET /v1/dashboard/features/{name}", h.handleGetFeatureHealth)
	mux.HandleFunc("GET /v1/dashboard/history", h.handleGetHistory)
}

func (h *FeatureDashboardHandler) handleGetSnapshot(w http.ResponseWriter, r *http.Request) {
	snapshot := h.dashboard.GetSnapshot()
	h.writeJSON(r.Context(), w, http.StatusOK, snapshot)
}

func (h *FeatureDashboardHandler) handleTakeSnapshot(w http.ResponseWriter, r *http.Request) {
	snapshot := h.dashboard.TakeSnapshot()
	h.writeJSON(r.Context(), w, http.StatusOK, snapshot)
}

func (h *FeatureDashboardHandler) handleGetFeatureHealth(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	health, err := h.dashboard.GetFeatureHealth(name)
	if err != nil {
		if errors.Is(err, featuredashboard.ErrFeatureNotTracked) {
			h.writeError(r.Context(), w, http.StatusNotFound, "feature not tracked")
			return
		}
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusOK, health)
}

func (h *FeatureDashboardHandler) handleGetHistory(w http.ResponseWriter, r *http.Request) {
	limit := 10
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	history := h.dashboard.GetHistory(limit)
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"snapshots": history,
		"count":     len(history),
	})
}

func (h *FeatureDashboardHandler) writeJSON(ctx context.Context, w http.ResponseWriter, status int, data interface{}) {
	writeJSONResponse(ctx, w, status, data)
}

func (h *FeatureDashboardHandler) writeError(ctx context.Context, w http.ResponseWriter, status int, message string) {
	writeJSONError(ctx, w, status, message)
}
