package server

import (
	"encoding/json"
	"net/http"

	"github.com/feather-store/feather/internal/platform/pushdown"
)

// PushdownHandler provides HTTP endpoints for feature transformation pushdown.
type PushdownHandler struct {
	evaluator *pushdown.Evaluator
}

// NewPushdownHandler creates a new pushdown handler.
func NewPushdownHandler(evaluator *pushdown.Evaluator) *PushdownHandler {
	return &PushdownHandler{evaluator: evaluator}
}

// RegisterRoutes registers pushdown API routes.
func (h *PushdownHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/pushdown/derived", h.handleListDerived)
	mux.HandleFunc("POST /v1/pushdown/derived", h.handleRegisterDerived)
	mux.HandleFunc("GET /v1/pushdown/derived/{name}", h.handleGetDerived)
	mux.HandleFunc("DELETE /v1/pushdown/derived/{name}", h.handleUnregisterDerived)
	mux.HandleFunc("POST /v1/pushdown/evaluate", h.handleEvaluate)
	mux.HandleFunc("POST /v1/pushdown/evaluate/adhoc", h.handleEvaluateAdhoc)
}

func (h *PushdownHandler) handleListDerived(w http.ResponseWriter, r *http.Request) {
	if h.evaluator == nil {
		writeJSONError(r.Context(), w, http.StatusServiceUnavailable, "pushdown not configured")
		return
	}
	derived := h.evaluator.ListDerived()
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success": true, "derived_features": derived, "count": len(derived),
	})
}

func (h *PushdownHandler) handleRegisterDerived(w http.ResponseWriter, r *http.Request) {
	if h.evaluator == nil {
		writeJSONError(r.Context(), w, http.StatusServiceUnavailable, "pushdown not configured")
		return
	}
	var df pushdown.DerivedFeature
	if err := json.NewDecoder(r.Body).Decode(&df); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.evaluator.RegisterDerived(&df); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusCreated, map[string]interface{}{
		"success": true, "derived_feature": df,
	})
}

func (h *PushdownHandler) handleGetDerived(w http.ResponseWriter, r *http.Request) {
	if h.evaluator == nil {
		writeJSONError(r.Context(), w, http.StatusServiceUnavailable, "pushdown not configured")
		return
	}
	df, err := h.evaluator.GetDerived(r.PathValue("name"))
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{"success": true, "derived_feature": df})
}

func (h *PushdownHandler) handleUnregisterDerived(w http.ResponseWriter, r *http.Request) {
	if h.evaluator == nil {
		writeJSONError(r.Context(), w, http.StatusServiceUnavailable, "pushdown not configured")
		return
	}
	h.evaluator.UnregisterDerived(r.PathValue("name"))
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{"success": true, "message": "derived feature removed"})
}

func (h *PushdownHandler) handleEvaluate(w http.ResponseWriter, r *http.Request) {
	if h.evaluator == nil {
		writeJSONError(r.Context(), w, http.StatusServiceUnavailable, "pushdown not configured")
		return
	}
	var req struct {
		Entity  string             `json:"entity"`
		Feature string             `json:"feature"`
		Context map[string]float64 `json:"context"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}
	result, err := h.evaluator.Evaluate(req.Entity, req.Feature, req.Context)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success": true, "result": result,
	})
}

func (h *PushdownHandler) handleEvaluateAdhoc(w http.ResponseWriter, r *http.Request) {
	if h.evaluator == nil {
		writeJSONError(r.Context(), w, http.StatusServiceUnavailable, "pushdown not configured")
		return
	}
	var req struct {
		Expression string             `json:"expression"`
		Context    map[string]float64 `json:"context"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}
	result, err := h.evaluator.EvaluateExpression(req.Expression, req.Context)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success": true, "result": result,
	})
}
