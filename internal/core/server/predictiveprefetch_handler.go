package server

import (
	"net/http"
	"time"

	"github.com/feather-store/feather/internal/extensions/prefetch"
)

// PredictivePrefetchHandler handles predictive feature prefetching API requests.
type PredictivePrefetchHandler struct {
	controller *prefetch.Controller
	forecaster *prefetch.Forecaster
}

// NewPredictivePrefetchHandler creates a new handler.
func NewPredictivePrefetchHandler(controller *prefetch.Controller, forecaster *prefetch.Forecaster) *PredictivePrefetchHandler {
	return &PredictivePrefetchHandler{controller: controller, forecaster: forecaster}
}

// RegisterRoutes registers predictive prefetch API routes.
func (h *PredictivePrefetchHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/prefetch/predictive/record", h.handleRecordAccess)
	mux.HandleFunc("GET /v1/prefetch/predictive/predict/{entity}", h.handlePredict)
	mux.HandleFunc("GET /v1/prefetch/predictive/forecast/{entity}", h.handleForecast)
	mux.HandleFunc("GET /v1/prefetch/predictive/plan/{entity}", h.handleGetPlan)
	mux.HandleFunc("GET /v1/prefetch/predictive/patterns", h.handlePatterns)
	mux.HandleFunc("GET /v1/prefetch/predictive/stats", h.handleStats)
}

func (h *PredictivePrefetchHandler) handleRecordAccess(w http.ResponseWriter, r *http.Request) {
	var req struct {
		EntityKey string   `json:"entity_key"`
		Features  []string `json:"features"`
	}
	if err := strictDecode(r.Body, &req); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	if req.EntityKey == "" {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "entity_key required")
		return
	}
	h.controller.RecordAccess(req.EntityKey, req.Features)
	for _, f := range req.Features {
		h.forecaster.RecordAccess(req.EntityKey, f, time.Now())
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, SuccessResponse{Success: true, Message: "access recorded"})
}

func (h *PredictivePrefetchHandler) handlePredict(w http.ResponseWriter, r *http.Request) {
	entity := r.PathValue("entity")
	if entity == "" {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "entity required")
		return
	}
	candidates := h.controller.Predict(entity)
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"entity":     entity,
		"candidates": candidates,
	})
}

func (h *PredictivePrefetchHandler) handleForecast(w http.ResponseWriter, r *http.Request) {
	entity := r.PathValue("entity")
	if entity == "" {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "entity required")
		return
	}
	forecast := h.forecaster.Forecast(entity, time.Hour)
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"entity":   entity,
		"forecast": forecast,
	})
}

func (h *PredictivePrefetchHandler) handleGetPlan(w http.ResponseWriter, r *http.Request) {
	entity := r.PathValue("entity")
	if entity == "" {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "entity required")
		return
	}
	plan := h.controller.GetPrefetchPlan(entity)
	writeJSONResponse(r.Context(), w, http.StatusOK, plan)
}

func (h *PredictivePrefetchHandler) handlePatterns(w http.ResponseWriter, r *http.Request) {
	clusters := h.forecaster.ClusterEntities()
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"clusters": clusters,
		"total":    len(clusters),
	})
}

func (h *PredictivePrefetchHandler) handleStats(w http.ResponseWriter, r *http.Request) {
	controllerStats := h.controller.Stats()
	forecasterStats := h.forecaster.Stats()
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"controller": controllerStats,
		"forecaster": forecasterStats,
	})
}
