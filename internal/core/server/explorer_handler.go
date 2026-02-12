package server

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/feather-store/feather/internal/tools/dashboard"
)

// ExplorerHandler exposes feature exploration APIs.
type ExplorerHandler struct {
	explorer *dashboard.Explorer
}

// NewExplorerHandler creates a new ExplorerHandler.
func NewExplorerHandler(explorer *dashboard.Explorer) *ExplorerHandler {
	return &ExplorerHandler{explorer: explorer}
}

// RegisterRoutes registers the explorer API routes on the given mux.
func (h *ExplorerHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/explorer/insights", h.handleRecordInsight)
	mux.HandleFunc("GET /v1/explorer/insights", h.handleListInsights)
	mux.HandleFunc("GET /v1/explorer/insights/{featureId}", h.handleGetInsight)
	mux.HandleFunc("GET /v1/explorer/search", h.handleSearchInsights)
	mux.HandleFunc("POST /v1/explorer/correlations", h.handleComputeCorrelation)
	mux.HandleFunc("GET /v1/explorer/correlations", h.handleListCorrelations)
	mux.HandleFunc("POST /v1/explorer/usage", h.handleRecordUsage)
	mux.HandleFunc("GET /v1/explorer/usage", h.handleListUsage)
	mux.HandleFunc("GET /v1/explorer/usage/{featureId}", h.handleGetUsage)
	mux.HandleFunc("POST /v1/explorer/costs", h.handleRecordCost)
	mux.HandleFunc("GET /v1/explorer/costs", h.handleGetTotalCosts)
	mux.HandleFunc("GET /v1/explorer/costs/{featureId}", h.handleGetCost)
	mux.HandleFunc("GET /v1/explorer/stats", h.handleStats)
}

func (h *ExplorerHandler) handleRecordInsight(w http.ResponseWriter, r *http.Request) {
	var insight dashboard.FeatureInsight
	if err := json.NewDecoder(r.Body).Decode(&insight); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	h.explorer.RecordInsight(&insight)
	writeJSONResponse(r.Context(), w, http.StatusCreated, map[string]string{"status": "recorded"})
}

func (h *ExplorerHandler) handleListInsights(w http.ResponseWriter, r *http.Request) {
	writeJSONResponse(r.Context(), w, http.StatusOK, h.explorer.ListInsights())
}

func (h *ExplorerHandler) handleGetInsight(w http.ResponseWriter, r *http.Request) {
	featureID := r.PathValue("featureId")
	insight, err := h.explorer.GetInsight(featureID)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, insight)
}

func (h *ExplorerHandler) handleSearchInsights(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	if q == "" {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "query parameter 'q' is required")
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, h.explorer.SearchInsights(q))
}

type correlationRequest struct {
	FeatureA string    `json:"feature_a"`
	FeatureB string    `json:"feature_b"`
	ValuesA  []float64 `json:"values_a"`
	ValuesB  []float64 `json:"values_b"`
}

func (h *ExplorerHandler) handleComputeCorrelation(w http.ResponseWriter, r *http.Request) {
	var req correlationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	result, err := h.explorer.ComputeCorrelation(req.FeatureA, req.FeatureB, req.ValuesA, req.ValuesB)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, result)
}

func (h *ExplorerHandler) handleListCorrelations(w http.ResponseWriter, r *http.Request) {
	minStr := r.URL.Query().Get("min")
	minCorrelation := 0.5
	if minStr != "" {
		if parsed, err := strconv.ParseFloat(minStr, 64); err == nil {
			minCorrelation = parsed
		}
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, h.explorer.ListCorrelations(minCorrelation))
}

func (h *ExplorerHandler) handleRecordUsage(w http.ResponseWriter, r *http.Request) {
	var pattern dashboard.UsagePattern
	if err := json.NewDecoder(r.Body).Decode(&pattern); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	h.explorer.RecordUsagePattern(&pattern)
	writeJSONResponse(r.Context(), w, http.StatusCreated, map[string]string{"status": "recorded"})
}

func (h *ExplorerHandler) handleListUsage(w http.ResponseWriter, r *http.Request) {
	writeJSONResponse(r.Context(), w, http.StatusOK, h.explorer.ListUsagePatterns())
}

func (h *ExplorerHandler) handleGetUsage(w http.ResponseWriter, r *http.Request) {
	featureID := r.PathValue("featureId")
	pattern, err := h.explorer.GetUsagePattern(featureID)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, pattern)
}

func (h *ExplorerHandler) handleRecordCost(w http.ResponseWriter, r *http.Request) {
	var cost dashboard.CostBreakdown
	if err := json.NewDecoder(r.Body).Decode(&cost); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	h.explorer.RecordCost(&cost)
	writeJSONResponse(r.Context(), w, http.StatusCreated, map[string]string{"status": "recorded"})
}

func (h *ExplorerHandler) handleGetTotalCosts(w http.ResponseWriter, r *http.Request) {
	writeJSONResponse(r.Context(), w, http.StatusOK, h.explorer.GetTotalCosts())
}

func (h *ExplorerHandler) handleGetCost(w http.ResponseWriter, r *http.Request) {
	featureID := r.PathValue("featureId")
	cost, err := h.explorer.GetCostBreakdown(featureID)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, cost)
}

func (h *ExplorerHandler) handleStats(w http.ResponseWriter, r *http.Request) {
	writeJSONResponse(r.Context(), w, http.StatusOK, h.explorer.Stats())
}
