package server

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/feather-store/feather/internal/extensions/prefetch"
)

// PrefetchHandler handles prefetch API requests.
type PrefetchHandler struct {
	controller *prefetch.Controller
}

// NewPrefetchHandler creates a new prefetch handler.
func NewPrefetchHandler(controller *prefetch.Controller) *PrefetchHandler {
	return &PrefetchHandler{controller: controller}
}

// RegisterRoutes registers prefetch API routes.
func (h *PrefetchHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/prefetch/record", h.handleRecordAccess)
	mux.HandleFunc("GET /v1/prefetch/predict/{entity}", h.handlePredict)
	mux.HandleFunc("GET /v1/prefetch/plan/{entity}", h.handleGetPlan)
	mux.HandleFunc("GET /v1/prefetch/stats", h.handleGetStats)
}

// handleRecordAccess handles POST /v1/prefetch/record
func (h *PrefetchHandler) handleRecordAccess(w http.ResponseWriter, r *http.Request) {
	var req struct {
		EntityKey string   `json:"entity_key"`
		Features  []string `json:"features"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.EntityKey == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "entity_key required")
		return
	}

	h.controller.RecordAccess(req.EntityKey, req.Features)
	h.writeJSON(r.Context(), w, http.StatusOK, SuccessResponse{Success: true, Message: "access recorded"})
}

// handlePredict handles GET /v1/prefetch/predict/{entity}
func (h *PrefetchHandler) handlePredict(w http.ResponseWriter, r *http.Request) {
	entity := r.PathValue("entity")
	if entity == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "entity required")
		return
	}

	candidates := h.controller.Predict(entity)
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"entity":     entity,
		"candidates": candidates,
	})
}

// handleGetPlan handles GET /v1/prefetch/plan/{entity}
func (h *PrefetchHandler) handleGetPlan(w http.ResponseWriter, r *http.Request) {
	entity := r.PathValue("entity")
	if entity == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "entity required")
		return
	}

	plan := h.controller.GetPrefetchPlan(entity)
	h.writeJSON(r.Context(), w, http.StatusOK, plan)
}

// handleGetStats handles GET /v1/prefetch/stats
func (h *PrefetchHandler) handleGetStats(w http.ResponseWriter, r *http.Request) {
	stats := h.controller.Stats()
	h.writeJSON(r.Context(), w, http.StatusOK, stats)
}

func (h *PrefetchHandler) writeJSON(ctx context.Context, w http.ResponseWriter, status int, data interface{}) {
	writeJSONResponse(ctx, w, status, data)
}

func (h *PrefetchHandler) writeError(ctx context.Context, w http.ResponseWriter, status int, message string) {
	writeJSONError(ctx, w, status, message)
}
