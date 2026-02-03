package server

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/feather-store/feather/internal/extensions/qualitygates"
)

// QualityGatesHandler handles quality gates API requests.
type QualityGatesHandler struct {
	validator *qualitygates.Validator
}

// NewQualityGatesHandler creates a new quality gates handler.
func NewQualityGatesHandler(validator *qualitygates.Validator) *QualityGatesHandler {
	return &QualityGatesHandler{validator: validator}
}

// RegisterRoutes registers quality gates API routes.
func (h *QualityGatesHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/quality/validate/schema", h.handleValidateSchema)
	mux.HandleFunc("POST /v1/quality/validate/data", h.handleAssertQuality)
	mux.HandleFunc("POST /v1/quality/validate/pr", h.handleValidatePR)
	mux.HandleFunc("POST /v1/quality/rules/evaluate", h.handleEvaluateRules)
	mux.HandleFunc("POST /v1/quality/report", h.handleGenerateReport)
	mux.HandleFunc("GET /v1/quality/gates/stats", h.handleGetStats)
}

// handleValidateSchema handles POST /v1/quality/validate/schema
func (h *QualityGatesHandler) handleValidateSchema(w http.ResponseWriter, r *http.Request) {
	var schema qualitygates.SchemaDefinition
	if err := json.NewDecoder(r.Body).Decode(&schema); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	report, err := h.validator.ValidateSchema(schema)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, report)
}

// handleAssertQuality handles POST /v1/quality/validate/data
func (h *QualityGatesHandler) handleAssertQuality(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Feature string    `json:"feature"`
		Samples []float64 `json:"samples"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Feature == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "feature name required")
		return
	}

	report, err := h.validator.AssertQuality(req.Feature, req.Samples)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, report)
}

// handleValidatePR handles POST /v1/quality/validate/pr
func (h *QualityGatesHandler) handleValidatePR(w http.ResponseWriter, r *http.Request) {
	var req qualitygates.PRValidationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	result, err := h.validator.ValidatePR(req)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, result)
}

// handleEvaluateRules handles POST /v1/quality/rules/evaluate
func (h *QualityGatesHandler) handleEvaluateRules(w http.ResponseWriter, r *http.Request) {
	var result qualitygates.PRValidationResult
	if err := json.NewDecoder(r.Body).Decode(&result); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	decision := h.validator.EvaluateRules(&result)
	h.writeJSON(r.Context(), w, http.StatusOK, decision)
}

// handleGenerateReport handles POST /v1/quality/report
func (h *QualityGatesHandler) handleGenerateReport(w http.ResponseWriter, r *http.Request) {
	var result qualitygates.PRValidationResult
	if err := json.NewDecoder(r.Body).Decode(&result); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	report := h.validator.GenerateReport(&result)
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"report": report,
	})
}

// handleGetStats handles GET /v1/quality/gates/stats
func (h *QualityGatesHandler) handleGetStats(w http.ResponseWriter, r *http.Request) {
	stats := h.validator.Stats()
	h.writeJSON(r.Context(), w, http.StatusOK, stats)
}

func (h *QualityGatesHandler) writeJSON(ctx context.Context, w http.ResponseWriter, status int, data interface{}) {
	writeJSONResponse(ctx, w, status, data)
}

func (h *QualityGatesHandler) writeError(ctx context.Context, w http.ResponseWriter, status int, message string) {
	writeJSONError(ctx, w, status, message)
}
