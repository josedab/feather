package server

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/feather-store/feather/internal/extensions/diffprivacy"
)

// DiffPrivacyHandler handles differential privacy API requests.
type DiffPrivacyHandler struct {
	engine *diffprivacy.Engine
}

// NewDiffPrivacyHandler creates a new differential privacy handler.
func NewDiffPrivacyHandler(engine *diffprivacy.Engine) *DiffPrivacyHandler {
	return &DiffPrivacyHandler{engine: engine}
}

// RegisterRoutes registers differential privacy API routes.
func (h *DiffPrivacyHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/privacy/register", h.handleRegister)
	mux.HandleFunc("POST /v1/privacy/noise", h.handleAddNoise)
	mux.HandleFunc("GET /v1/privacy/budget/{feature}", h.handleBudgetStatus)
	mux.HandleFunc("POST /v1/privacy/aggregate", h.handleNoisyAggregate)
	mux.HandleFunc("GET /v1/privacy/stats", h.handleGetStats)
}

// handleRegister handles POST /v1/privacy/register
func (h *DiffPrivacyHandler) handleRegister(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name   string                       `json:"name"`
		Config diffprivacy.FeaturePrivacyConfig `json:"config"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "feature name required")
		return
	}

	if err := h.engine.RegisterFeature(req.Name, req.Config); err != nil {
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusCreated, SuccessResponse{Success: true, Message: "feature registered with privacy config"})
}

// handleAddNoise handles POST /v1/privacy/noise
func (h *DiffPrivacyHandler) handleAddNoise(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Feature string  `json:"feature"`
		Value   float64 `json:"value"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Feature == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "feature name required")
		return
	}

	noisy, err := h.engine.AddNoise(req.Feature, req.Value)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"original": req.Value,
		"noisy":    noisy,
	})
}

// handleBudgetStatus handles GET /v1/privacy/budget/{feature}
func (h *DiffPrivacyHandler) handleBudgetStatus(w http.ResponseWriter, r *http.Request) {
	feature := r.PathValue("feature")
	if feature == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "feature name required")
		return
	}

	budget, err := h.engine.BudgetStatus(feature)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, budget)
}

// handleNoisyAggregate handles POST /v1/privacy/aggregate
func (h *DiffPrivacyHandler) handleNoisyAggregate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Feature string    `json:"feature"`
		Values  []float64 `json:"values"`
		AggType string    `json:"agg_type"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Feature == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "feature name required")
		return
	}

	var (
		result diffprivacy.NoisyAggregation
		err    error
	)

	switch req.AggType {
	case "count":
		result, err = h.engine.NoisyCount(req.Feature, int64(len(req.Values)))
	case "sum":
		var sum float64
		for _, v := range req.Values {
			sum += v
		}
		result, err = h.engine.NoisySum(req.Feature, sum)
	case "avg":
		var sum float64
		for _, v := range req.Values {
			sum += v
		}
		result, err = h.engine.NoisyAvg(req.Feature, sum, int64(len(req.Values)))
	case "min":
		if len(req.Values) == 0 {
			h.writeError(r.Context(), w, http.StatusBadRequest, "values required for min aggregation")
			return
		}
		min := req.Values[0]
		for _, v := range req.Values[1:] {
			if v < min {
				min = v
			}
		}
		result, err = h.engine.NoisyMin(req.Feature, min)
	case "max":
		if len(req.Values) == 0 {
			h.writeError(r.Context(), w, http.StatusBadRequest, "values required for max aggregation")
			return
		}
		max := req.Values[0]
		for _, v := range req.Values[1:] {
			if v > max {
				max = v
			}
		}
		result, err = h.engine.NoisyMax(req.Feature, max)
	default:
		h.writeError(r.Context(), w, http.StatusBadRequest, "unsupported agg_type: use count, sum, avg, min, or max")
		return
	}

	if err != nil {
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, result)
}

// handleGetStats handles GET /v1/privacy/stats
func (h *DiffPrivacyHandler) handleGetStats(w http.ResponseWriter, r *http.Request) {
	stats := h.engine.Stats()
	h.writeJSON(r.Context(), w, http.StatusOK, stats)
}

func (h *DiffPrivacyHandler) writeJSON(ctx context.Context, w http.ResponseWriter, status int, data interface{}) {
	writeJSONResponse(ctx, w, status, data)
}

func (h *DiffPrivacyHandler) writeError(ctx context.Context, w http.ResponseWriter, status int, message string) {
	writeJSONError(ctx, w, status, message)
}
