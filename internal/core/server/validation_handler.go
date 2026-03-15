package server

import (
	"context"
	"net/http"
	"strconv"

	"github.com/feather-store/feather/internal/platform/validation"
)

// ValidationHandler handles online-offline consistency validation API requests.
type ValidationHandler struct {
	validator   *validation.Validator
	requireAuth func(http.Handler) http.Handler
}

// NewValidationHandler creates a new validation handler.
func NewValidationHandler(validator *validation.Validator) *ValidationHandler {
	return &ValidationHandler{validator: validator}
}

// RegisterRoutes registers validation API routes.
func (h *ValidationHandler) RegisterRoutes(mux *http.ServeMux) {
	wrap := h.requireAuth
	if wrap == nil {
		wrap = func(next http.Handler) http.Handler { return next }
	}
	if wrap == nil {
		wrap = func(next http.Handler) http.Handler { return next }
	}
	mux.Handle("GET /v1/validation/rules", wrap(http.HandlerFunc(h.handleListRules)))
	mux.Handle("POST /v1/validation/rules", wrap(http.HandlerFunc(h.handleAddRule)))
	mux.Handle("DELETE /v1/validation/rules/{name}", wrap(http.HandlerFunc(h.handleRemoveRule)))
	mux.Handle("POST /v1/validation/validate", wrap(http.HandlerFunc(h.handleValidate)))
	mux.Handle("GET /v1/validation/results", wrap(http.HandlerFunc(h.handleGetResults)))
	mux.Handle("GET /v1/validation/reports", wrap(http.HandlerFunc(h.handleGetReports)))
	mux.Handle("GET /v1/validation/stats", wrap(http.HandlerFunc(h.handleStats)))
}

func (h *ValidationHandler) handleListRules(w http.ResponseWriter, r *http.Request) {
	rules := h.validator.ListRules(r.Context())
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{"rules": rules})
}

func (h *ValidationHandler) handleAddRule(w http.ResponseWriter, r *http.Request) {
	var rule validation.ValidationRule
	if err := strictDecode(r.Body, &rule); err != nil {
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
	h.writeJSON(r.Context(), w, http.StatusOK, SuccessResponse{Success: true})
}

type validateRequest struct {
	RuleName      string    `json:"rule_name"`
	OnlineValues  []float64 `json:"online_values"`
	OfflineValues []float64 `json:"offline_values"`
}

func (h *ValidationHandler) handleValidate(w http.ResponseWriter, r *http.Request) {
	var req validateRequest
	if err := strictDecode(r.Body, &req); err != nil {
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
