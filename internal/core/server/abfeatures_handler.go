package server

import (
	"context"
	"errors"
	"net/http"

	"github.com/feather-store/feather/internal/extensions/abfeatures"
)

// ABFeaturesHandler handles A/B experiment API requests.
type ABFeaturesHandler struct {
	manager *abfeatures.Manager
}

// NewABFeaturesHandler creates a new A/B features handler.
func NewABFeaturesHandler(manager *abfeatures.Manager) *ABFeaturesHandler {
	return &ABFeaturesHandler{manager: manager}
}

// RegisterRoutes registers A/B experiment API routes.
func (h *ABFeaturesHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/experiments", h.handleListExperiments)
	mux.HandleFunc("POST /v1/experiments", h.handleCreateExperiment)
	mux.HandleFunc("GET /v1/experiments/{id}", h.handleGetExperiment)
	mux.HandleFunc("POST /v1/experiments/{id}/start", h.handleStartExperiment)
	mux.HandleFunc("POST /v1/experiments/{id}/stop", h.handleStopExperiment)
	mux.HandleFunc("GET /v1/experiments/{id}/resolve", h.handleResolveVariant)
	mux.HandleFunc("POST /v1/experiments/{id}/metric", h.handleRecordMetric)
	mux.HandleFunc("POST /v1/experiments/{id}/score", h.handleRecordScore)
	mux.HandleFunc("GET /v1/experiments/{id}/significance", h.handleEvaluateSignificance)
	mux.HandleFunc("GET /v1/experiments/stats", h.handleGetStats)
}

// handleListExperiments handles GET /v1/experiments
func (h *ABFeaturesHandler) handleListExperiments(w http.ResponseWriter, r *http.Request) {
	if h.manager == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "experiment manager not configured")
		return
	}

	experiments := h.manager.ListExperiments()

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"experiments": experiments,
	})
}

// handleCreateExperiment handles POST /v1/experiments
func (h *ABFeaturesHandler) handleCreateExperiment(w http.ResponseWriter, r *http.Request) {
	if h.manager == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "experiment manager not configured")
		return
	}

	var exp abfeatures.Experiment
	if err := strictDecode(r.Body, &exp); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.manager.CreateExperiment(exp); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusCreated, map[string]interface{}{
		"success": true,
		"id":      exp.ID,
	})
}

// handleGetExperiment handles GET /v1/experiments/{id}
func (h *ABFeaturesHandler) handleGetExperiment(w http.ResponseWriter, r *http.Request) {
	if h.manager == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "experiment manager not configured")
		return
	}

	id := r.PathValue("id")
	if id == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "experiment id is required")
		return
	}

	exp, err := h.manager.GetExperiment(id)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, exp)
}

// handleStartExperiment handles POST /v1/experiments/{id}/start
func (h *ABFeaturesHandler) handleStartExperiment(w http.ResponseWriter, r *http.Request) {
	if h.manager == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "experiment manager not configured")
		return
	}

	id := r.PathValue("id")
	if id == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "experiment id is required")
		return
	}

	if err := h.manager.StartExperiment(id); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, SuccessResponse{Success: true, Message: "experiment started"})
}

// handleStopExperiment handles POST /v1/experiments/{id}/stop
func (h *ABFeaturesHandler) handleStopExperiment(w http.ResponseWriter, r *http.Request) {
	if h.manager == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "experiment manager not configured")
		return
	}

	id := r.PathValue("id")
	if id == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "experiment id is required")
		return
	}

	if err := h.manager.StopExperiment(id); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, SuccessResponse{Success: true, Message: "experiment stopped"})
}

// handleResolveVariant handles GET /v1/experiments/{id}/resolve
func (h *ABFeaturesHandler) handleResolveVariant(w http.ResponseWriter, r *http.Request) {
	if h.manager == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "experiment manager not configured")
		return
	}

	id := r.PathValue("id")
	if id == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "experiment id is required")
		return
	}

	entityID := r.URL.Query().Get("entity_id")
	if entityID == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "entity_id is required")
		return
	}

	variant, err := h.manager.ResolveVariant(id, entityID)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"experiment_id": id,
		"entity_id":     entityID,
		"variant":       variant,
	})
}

// handleRecordMetric handles POST /v1/experiments/{id}/metric
func (h *ABFeaturesHandler) handleRecordMetric(w http.ResponseWriter, r *http.Request) {
	if h.manager == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "experiment manager not configured")
		return
	}

	id := r.PathValue("id")
	if id == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "experiment id is required")
		return
	}

	var req struct {
		VariantID string  `json:"variant_id"`
		LatencyMs float64 `json:"latency_ms"`
		Error     *string `json:"error,omitempty"`
	}
	if err := strictDecode(r.Body, &req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	var reqErr error
	if req.Error != nil {
		reqErr = errFromString(*req.Error)
	}

	h.manager.RecordMetric(id, req.VariantID, req.LatencyMs, reqErr)

	h.writeJSON(r.Context(), w, http.StatusOK, SuccessResponse{Success: true})
}

// handleRecordScore handles POST /v1/experiments/{id}/score
func (h *ABFeaturesHandler) handleRecordScore(w http.ResponseWriter, r *http.Request) {
	if h.manager == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "experiment manager not configured")
		return
	}

	id := r.PathValue("id")
	if id == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "experiment id is required")
		return
	}

	var req struct {
		VariantID string  `json:"variant_id"`
		Score     float64 `json:"score"`
	}
	if err := strictDecode(r.Body, &req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	h.manager.RecordScore(id, req.VariantID, req.Score)

	h.writeJSON(r.Context(), w, http.StatusOK, SuccessResponse{Success: true})
}

// handleEvaluateSignificance handles GET /v1/experiments/{id}/significance
func (h *ABFeaturesHandler) handleEvaluateSignificance(w http.ResponseWriter, r *http.Request) {
	if h.manager == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "experiment manager not configured")
		return
	}

	id := r.PathValue("id")
	if id == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "experiment id is required")
		return
	}

	result, err := h.manager.EvaluateSignificance(id)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, result)
}

// handleGetStats handles GET /v1/experiments/stats
func (h *ABFeaturesHandler) handleGetStats(w http.ResponseWriter, r *http.Request) {
	if h.manager == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "experiment manager not configured")
		return
	}

	stats := h.manager.Stats()

	h.writeJSON(r.Context(), w, http.StatusOK, stats)
}

// errFromString creates an error from a string.
func errFromString(s string) error {
	if s == "" {
		return nil
	}
	return errors.New(s)
}

func (h *ABFeaturesHandler) writeJSON(ctx context.Context, w http.ResponseWriter, status int, data interface{}) {
	writeJSONResponse(ctx, w, status, data)
}

func (h *ABFeaturesHandler) writeError(ctx context.Context, w http.ResponseWriter, status int, message string) {
	writeJSONError(ctx, w, status, message)
}
