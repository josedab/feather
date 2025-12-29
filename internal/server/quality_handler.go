package server

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/feather-store/feather/internal/quality"
)

// QualityHandler handles data quality API requests.
type QualityHandler struct {
	validator *quality.Validator
}

// NewQualityHandler creates a new quality handler.
func NewQualityHandler(validator *quality.Validator) *QualityHandler {
	return &QualityHandler{
		validator: validator,
	}
}

// RegisterRoutes registers quality API routes.
func (h *QualityHandler) RegisterRoutes(mux *http.ServeMux) {
	// Rule management
	mux.HandleFunc("GET /v1/quality/rules", h.handleListRules)
	mux.HandleFunc("GET /v1/quality/rules/{id}", h.handleGetRule)
	mux.HandleFunc("POST /v1/quality/rules", h.handleAddRule)
	mux.HandleFunc("DELETE /v1/quality/rules/{id}", h.handleRemoveRule)
	mux.HandleFunc("GET /v1/quality/rules/feature/{featureId}", h.handleGetRulesForFeature)

	// Validation
	mux.HandleFunc("POST /v1/quality/validate", h.handleValidateValue)
	mux.HandleFunc("POST /v1/quality/validate/batch", h.handleValidateBatch)

	// Quality scores and reports
	mux.HandleFunc("GET /v1/quality/score/{featureId}", h.handleGetQualityScore)
	mux.HandleFunc("GET /v1/quality/history", h.handleGetHistory)
	mux.HandleFunc("GET /v1/quality/stats", h.handleGetStats)
}

// ValidationRuleJSON represents a validation rule in JSON format.
type ValidationRuleJSON struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Type        string                 `json:"type"`
	FeatureID   string                 `json:"feature_id"`
	GroupID     string                 `json:"group_id,omitempty"`
	Severity    string                 `json:"severity"`
	Config      map[string]interface{} `json:"config,omitempty"`
	Enabled     bool                   `json:"enabled"`
	Tags        []string               `json:"tags,omitempty"`
	CreatedAt   string                 `json:"created_at,omitempty"`
	UpdatedAt   string                 `json:"updated_at,omitempty"`
}

// ValidateValueRequest represents a validation request.
type ValidateValueRequest struct {
	FeatureID string                 `json:"feature_id"`
	Value     interface{}            `json:"value"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// ValidateBatchRequest represents a batch validation request.
type ValidateBatchRequest struct {
	FeatureID string                 `json:"feature_id"`
	Values    []interface{}          `json:"values"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// handleListRules handles GET /v1/quality/rules
func (h *QualityHandler) handleListRules(w http.ResponseWriter, r *http.Request) {
	if h.validator == nil {
		h.writeError(w, http.StatusServiceUnavailable, "quality validator not configured")
		return
	}

	rules := h.validator.ListRules()
	response := make([]ValidationRuleJSON, len(rules))

	for i, rule := range rules {
		response[i] = h.ruleToJSON(rule)
	}

	h.writeJSON(w, http.StatusOK, map[string]interface{}{
		"rules": response,
		"count": len(response),
	})
}

// handleGetRule handles GET /v1/quality/rules/{id}
func (h *QualityHandler) handleGetRule(w http.ResponseWriter, r *http.Request) {
	if h.validator == nil {
		h.writeError(w, http.StatusServiceUnavailable, "quality validator not configured")
		return
	}

	ruleID := r.PathValue("id")
	if ruleID == "" {
		h.writeError(w, http.StatusBadRequest, "rule ID required")
		return
	}

	rule, err := h.validator.GetRule(ruleID)
	if err != nil {
		h.writeError(w, http.StatusNotFound, err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, h.ruleToJSON(rule))
}

// handleAddRule handles POST /v1/quality/rules
func (h *QualityHandler) handleAddRule(w http.ResponseWriter, r *http.Request) {
	if h.validator == nil {
		h.writeError(w, http.StatusServiceUnavailable, "quality validator not configured")
		return
	}

	var req ValidationRuleJSON
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	rule := h.jsonToRule(&req)

	if err := h.validator.AddRule(rule); err != nil {
		h.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	h.writeJSON(w, http.StatusCreated, map[string]interface{}{
		"success": true,
		"rule_id": rule.ID,
	})
}

// handleRemoveRule handles DELETE /v1/quality/rules/{id}
func (h *QualityHandler) handleRemoveRule(w http.ResponseWriter, r *http.Request) {
	if h.validator == nil {
		h.writeError(w, http.StatusServiceUnavailable, "quality validator not configured")
		return
	}

	ruleID := r.PathValue("id")
	if ruleID == "" {
		h.writeError(w, http.StatusBadRequest, "rule ID required")
		return
	}

	if err := h.validator.RemoveRule(ruleID); err != nil {
		h.writeError(w, http.StatusNotFound, err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
	})
}

// handleGetRulesForFeature handles GET /v1/quality/rules/feature/{featureId}
func (h *QualityHandler) handleGetRulesForFeature(w http.ResponseWriter, r *http.Request) {
	if h.validator == nil {
		h.writeError(w, http.StatusServiceUnavailable, "quality validator not configured")
		return
	}

	featureID := r.PathValue("featureId")
	if featureID == "" {
		h.writeError(w, http.StatusBadRequest, "feature ID required")
		return
	}

	rules := h.validator.GetRulesForFeature(featureID)
	response := make([]ValidationRuleJSON, len(rules))

	for i, rule := range rules {
		response[i] = h.ruleToJSON(rule)
	}

	h.writeJSON(w, http.StatusOK, map[string]interface{}{
		"feature_id": featureID,
		"rules":      response,
		"count":      len(response),
	})
}

// handleValidateValue handles POST /v1/quality/validate
func (h *QualityHandler) handleValidateValue(w http.ResponseWriter, r *http.Request) {
	if h.validator == nil {
		h.writeError(w, http.StatusServiceUnavailable, "quality validator not configured")
		return
	}

	var req ValidateValueRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.FeatureID == "" {
		h.writeError(w, http.StatusBadRequest, "feature_id is required")
		return
	}

	results := h.validator.ValidateValue(r.Context(), req.FeatureID, req.Value, req.Metadata)

	allPassed := true
	for _, result := range results {
		if !result.Passed {
			allPassed = false
			break
		}
	}

	h.writeJSON(w, http.StatusOK, map[string]interface{}{
		"feature_id": req.FeatureID,
		"passed":     allPassed,
		"results":    results,
	})
}

// handleValidateBatch handles POST /v1/quality/validate/batch
func (h *QualityHandler) handleValidateBatch(w http.ResponseWriter, r *http.Request) {
	if h.validator == nil {
		h.writeError(w, http.StatusServiceUnavailable, "quality validator not configured")
		return
	}

	var req ValidateBatchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.FeatureID == "" {
		h.writeError(w, http.StatusBadRequest, "feature_id is required")
		return
	}

	if len(req.Values) == 0 {
		h.writeError(w, http.StatusBadRequest, "values array is required")
		return
	}

	report := h.validator.ValidateBatch(r.Context(), req.FeatureID, req.Values, req.Metadata)

	h.writeJSON(w, http.StatusOK, report)
}

// handleGetQualityScore handles GET /v1/quality/score/{featureId}
func (h *QualityHandler) handleGetQualityScore(w http.ResponseWriter, r *http.Request) {
	if h.validator == nil {
		h.writeError(w, http.StatusServiceUnavailable, "quality validator not configured")
		return
	}

	featureID := r.PathValue("featureId")
	if featureID == "" {
		h.writeError(w, http.StatusBadRequest, "feature ID required")
		return
	}

	score := h.validator.CalculateQualityScore(featureID)

	h.writeJSON(w, http.StatusOK, score)
}

// handleGetHistory handles GET /v1/quality/history
func (h *QualityHandler) handleGetHistory(w http.ResponseWriter, r *http.Request) {
	if h.validator == nil {
		h.writeError(w, http.StatusServiceUnavailable, "quality validator not configured")
		return
	}

	limit := 100
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	history := h.validator.GetQualityHistory(limit)

	h.writeJSON(w, http.StatusOK, map[string]interface{}{
		"history": history,
		"count":   len(history),
	})
}

// handleGetStats handles GET /v1/quality/stats
func (h *QualityHandler) handleGetStats(w http.ResponseWriter, r *http.Request) {
	if h.validator == nil {
		h.writeError(w, http.StatusServiceUnavailable, "quality validator not configured")
		return
	}

	stats := h.validator.GetStats()

	h.writeJSON(w, http.StatusOK, stats)
}

func (h *QualityHandler) ruleToJSON(rule *quality.ValidationRule) ValidationRuleJSON {
	return ValidationRuleJSON{
		ID:          rule.ID,
		Name:        rule.Name,
		Description: rule.Description,
		Type:        string(rule.Type),
		FeatureID:   rule.FeatureID,
		GroupID:     rule.GroupID,
		Severity:    string(rule.Severity),
		Config:      rule.Config,
		Enabled:     rule.Enabled,
		Tags:        rule.Tags,
		CreatedAt:   rule.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:   rule.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}

func (h *QualityHandler) jsonToRule(j *ValidationRuleJSON) *quality.ValidationRule {
	return &quality.ValidationRule{
		ID:          j.ID,
		Name:        j.Name,
		Description: j.Description,
		Type:        quality.RuleType(j.Type),
		FeatureID:   j.FeatureID,
		GroupID:     j.GroupID,
		Severity:    quality.Severity(j.Severity),
		Config:      j.Config,
		Enabled:     true,
		Tags:        j.Tags,
	}
}

func (h *QualityHandler) writeJSON(w http.ResponseWriter, status int, data interface{}) {
	writeJSONResponse(w, status, data)
}

func (h *QualityHandler) writeError(w http.ResponseWriter, status int, message string) {
	writeJSONError(w, status, message)
}
