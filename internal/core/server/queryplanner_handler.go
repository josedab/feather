package server

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/feather-store/feather/internal/extensions/queryplanner"
)

// QueryPlannerHandler handles query planner API requests.
type QueryPlannerHandler struct {
	planner *queryplanner.Planner
}

// NewQueryPlannerHandler creates a new query planner handler.
func NewQueryPlannerHandler(planner *queryplanner.Planner) *QueryPlannerHandler {
	return &QueryPlannerHandler{planner: planner}
}

// RegisterRoutes registers query planner API routes.
func (h *QueryPlannerHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/planner/optimize", h.handleOptimize)
	mux.HandleFunc("POST /v1/planner/cost", h.handleRecordCost)
	mux.HandleFunc("POST /v1/planner/result", h.handleRecordResult)
	mux.HandleFunc("GET /v1/planner/replan/{id}", h.handleShouldReplan)
	mux.HandleFunc("GET /v1/planner/stats", h.handleGetStats)
}

// handleOptimize handles POST /v1/planner/optimize
func (h *QueryPlannerHandler) handleOptimize(w http.ResponseWriter, r *http.Request) {
	var query queryplanner.Query
	if err := json.NewDecoder(r.Body).Decode(&query); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	plan, err := h.planner.Optimize(query)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, plan)
}

// handleRecordCost handles POST /v1/planner/cost
func (h *QueryPlannerHandler) handleRecordCost(w http.ResponseWriter, r *http.Request) {
	var req struct {
		OpType     string  `json:"op_type"`
		DurationMs float64 `json:"duration_ms"`
		RowCount   int64   `json:"row_count"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.OpType == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "op_type required")
		return
	}

	h.planner.RecordOperationCost(req.OpType, req.DurationMs, req.RowCount)
	h.writeJSON(r.Context(), w, http.StatusOK, SuccessResponse{Success: true, Message: "cost recorded"})
}

// handleRecordResult handles POST /v1/planner/result
func (h *QueryPlannerHandler) handleRecordResult(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PlanID       string  `json:"plan_id"`
		ActualCostMs float64 `json:"actual_cost_ms"`
		ActualRows   int64   `json:"actual_rows"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.PlanID == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "plan_id required")
		return
	}

	h.planner.RecordExecutionResult(req.PlanID, req.ActualCostMs, req.ActualRows)
	h.writeJSON(r.Context(), w, http.StatusOK, SuccessResponse{Success: true, Message: "result recorded"})
}

// handleShouldReplan handles GET /v1/planner/replan/{id}
func (h *QueryPlannerHandler) handleShouldReplan(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "plan id required")
		return
	}

	shouldReplan := h.planner.ShouldReplan(id)
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"plan_id":       id,
		"should_replan": shouldReplan,
	})
}

// handleGetStats handles GET /v1/planner/stats
func (h *QueryPlannerHandler) handleGetStats(w http.ResponseWriter, r *http.Request) {
	stats := h.planner.Stats()
	h.writeJSON(r.Context(), w, http.StatusOK, stats)
}

func (h *QueryPlannerHandler) writeJSON(ctx context.Context, w http.ResponseWriter, status int, data interface{}) {
	writeJSONResponse(ctx, w, status, data)
}

func (h *QueryPlannerHandler) writeError(ctx context.Context, w http.ResponseWriter, status int, message string) {
	writeJSONError(ctx, w, status, message)
}
