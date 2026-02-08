package server

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/feather-store/feather/internal/platform/validation"
)

// ValidationHandler handles online-offline consistency validation API requests.
type ValidationHandler struct {
	validator *validation.Validator
}

// NewValidationHandler creates a new validation handler.
func NewValidationHandler(validator *validation.Validator) *ValidationHandler {
	return &ValidationHandler{validator: validator}
}

// RegisterRoutes registers validation API routes.
func (h *ValidationHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/validation/rules", h.handleListRules)
	mux.HandleFunc("POST /v1/validation/rules", h.handleAddRule)
	mux.HandleFunc("DELETE /v1/validation/rules/{name}", h.handleRemoveRule)
	mux.HandleFunc("POST /v1/validation/validate", h.handleValidate)
	mux.HandleFunc("GET /v1/validation/results", h.handleGetResults)
	mux.HandleFunc("GET /v1/validation/reports", h.handleGetReports)
	mux.HandleFunc("GET /v1/validation/stats", h.handleStats)
}

func (h *ValidationHandler) handleListRules(w http.ResponseWriter, r *http.Request) {
	rules := h.validator.ListRules(r.Context())
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{"rules": rules})
}

func (h *ValidationHandler) handleAddRule(w http.ResponseWriter, r *http.Request) {
	var rule validation.ValidationRule
	if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.validator.AddRule(r.Context(), &rule); err != nil {
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusCreated, map[string]interface{}{"success": true, "name": rule.Name})
}

func (h *ValidationHandler) handleRemoveRule(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "rule name required")
		return
	}
	if err := h.validator.RemoveRule(r.Context(), name); err != nil {
		h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{"success": true})
}

type validateRequest struct {
	RuleName      string    `json:"rule_name"`
	OnlineValues  []float64 `json:"online_values"`
	OfflineValues []float64 `json:"offline_values"`
}

func (h *ValidationHandler) handleValidate(w http.ResponseWriter, r *http.Request) {
	var req validateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.RuleName == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "rule_name required")
		return
	}
	result, err := h.validator.Validate(r.Context(), req.RuleName, req.OnlineValues, req.OfflineValues)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusOK, result)
}

func (h *ValidationHandler) handleGetResults(w http.ResponseWriter, r *http.Request) {
	limit := 100
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	results := h.validator.GetResults(r.Context(), limit)
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{"results": results})
}

func (h *ValidationHandler) handleGetReports(w http.ResponseWriter, r *http.Request) {
	limit := 100
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	results := h.validator.GetResults(r.Context(), limit)
	report, err := h.validator.GenerateReport(r.Context(), results)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusOK, report)
}

func (h *ValidationHandler) handleStats(w http.ResponseWriter, r *http.Request) {
	stats := h.validator.Stats(r.Context())
	h.writeJSON(r.Context(), w, http.StatusOK, stats)
}

func (h *ValidationHandler) writeJSON(ctx context.Context, w http.ResponseWriter, status int, data interface{}) {
	writeJSONResponse(ctx, w, status, data)
}

func (h *ValidationHandler) writeError(ctx context.Context, w http.ResponseWriter, status int, message string) {
	writeJSONError(ctx, w, status, message)
}
