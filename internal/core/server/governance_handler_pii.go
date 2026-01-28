package server

import (
"net/http"
"strconv"

"github.com/feather-store/feather/internal/platform/governance"
)
// handleDetectPII handles POST /v1/governance/pii/detect
func (h *GovernanceHandler) handleDetectPII(w http.ResponseWriter, r *http.Request) {
	if h.pii == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "PII detection not configured")
		return
	}

	var req PIIDetectionRequest
	if err := decodeJSONBody(r, &req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Content == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "content is required")
		return
	}

	// Scan uses featureName + value; use a generic feature name for single-value detection
	detections, err := h.pii.Scan(r.Context(), "_detect", req.Content)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}

	// Filter by categories if specified
	if len(req.Categories) > 0 {
		categorySet := make(map[string]bool)
		for _, cat := range req.Categories {
			categorySet[cat] = true
		}
		filtered := make([]*governance.PIIDetection, 0)
		for _, d := range detections {
			if categorySet[string(d.Category)] {
				filtered = append(filtered, d)
			}
		}
		detections = filtered
	}

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"detections": detections,
		"count":      len(detections),
		"has_pii":    len(detections) > 0,
	})
}

// handleScanPII handles POST /v1/governance/pii/scan
func (h *GovernanceHandler) handleScanPII(w http.ResponseWriter, r *http.Request) {
	if h.pii == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "PII detection not configured")
		return
	}

	var req PIIScanRequest
	if err := decodeJSONBody(r, &req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	if len(req.Contents) == 0 {
		h.writeError(r.Context(), w, http.StatusBadRequest, "contents is required")
		return
	}

	// Convert []string to map[string]interface{} for ScanBatch
	features := make(map[string]interface{})
	for i, content := range req.Contents {
		features[strconv.Itoa(i)] = content
	}

	results, err := h.pii.ScanBatch(r.Context(), features)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"results": results,
		"count":   len(results),
	})
}

// handleListPIIPatterns handles GET /v1/governance/pii/patterns
// Note: Returns pattern statistics and detected PII rather than the raw pattern list.
func (h *GovernanceHandler) handleListPIIPatterns(w http.ResponseWriter, r *http.Request) {
	if h.pii == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "PII detection not configured")
		return
	}

	// Get stats which includes pattern count
	stats := h.pii.Stats()

	// Get detections to show what PII has been found
	detections := h.pii.GetDetections()

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"pattern_count": stats["patterns"],
		"detections":    detections,
		"stats":         stats,
	})
}

// handleAddPIIPattern handles POST /v1/governance/pii/patterns
func (h *GovernanceHandler) handleAddPIIPattern(w http.ResponseWriter, r *http.Request) {
	if h.pii == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "PII detection not configured")
		return
	}

	var req PIIPatternRequest
	if err := decodeJSONBody(r, &req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name == "" || req.Pattern == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "name and pattern are required")
		return
	}

	pattern := &governance.PIIPattern{
		Name:        req.Name,
		Category:    governance.PIICategory(req.Category),
		Regex:       req.Pattern,
		Sensitivity: governance.PIISensitivity(req.Sensitivity),
		Description: req.Description,
		Enabled:     true,
	}

	if err := h.pii.AddPattern(pattern); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusCreated, map[string]interface{}{
		"success": true,
		"name":    req.Name,
	})
}

// handleRemovePIIPattern handles DELETE /v1/governance/pii/patterns/{name}
// Note: Pattern removal is not supported in the current implementation.
// Patterns can be disabled by adding a new pattern with Enabled=false.
func (h *GovernanceHandler) handleRemovePIIPattern(w http.ResponseWriter, r *http.Request) {
	if h.pii == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "PII detection not configured")
		return
	}

	name := r.PathValue("name")
	if name == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "pattern name required")
		return
	}

	// The PIIDetector doesn't support pattern removal.
	// In production, this would update the pattern to disabled.
	h.writeError(r.Context(), w, http.StatusNotImplemented, "pattern removal not implemented; patterns are immutable once added")
}

// Masking handlers

// handleMaskData handles POST /v1/governance/mask
func (h *GovernanceHandler) handleMaskData(w http.ResponseWriter, r *http.Request) {
	if h.masking == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "masking engine not configured")
		return
	}

	var req MaskRequest
	if err := decodeJSONBody(r, &req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Value == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "value is required")
		return
	}

	// Build masking context if principal is provided
	var maskCtx *governance.MaskingContext
	if req.Principal != nil {
		maskCtx = &governance.MaskingContext{
			UserID:  req.Principal.ID,
			Roles:   req.Principal.Roles,
			Purpose: req.Principal.Purpose,
		}
	}

	// Mask the value
	masked, _, err := h.masking.MaskWithError(r.Context(), req.Field, req.Value, maskCtx)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"original": req.Value,
		"masked":   masked,
		"field":    req.Field,
	})
}

// handleMaskBatch handles POST /v1/governance/mask/batch
func (h *GovernanceHandler) handleMaskBatch(w http.ResponseWriter, r *http.Request) {
	if h.masking == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "masking engine not configured")
		return
	}

	var req MaskBatchRequest
	if err := decodeJSONBody(r, &req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	if len(req.Items) == 0 {
		h.writeError(r.Context(), w, http.StatusBadRequest, "items is required")
		return
	}

	// Build masking context if principal is provided
	var maskCtx *governance.MaskingContext
	if req.Principal != nil {
		maskCtx = &governance.MaskingContext{
			UserID:  req.Principal.ID,
			Roles:   req.Principal.Roles,
			Purpose: req.Principal.Purpose,
		}
	}

	results := make([]map[string]interface{}, len(req.Items))
	for i, item := range req.Items {
		masked, _, err := h.masking.MaskWithError(r.Context(), item.Field, item.Value, maskCtx)
		if err != nil {
			h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
			return
		}
		results[i] = map[string]interface{}{
			"original": item.Value,
			"masked":   masked,
			"field":    item.Field,
		}
	}

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"results": results,
		"count":   len(results),
	})
}

// handleListMaskingRules handles GET /v1/governance/mask/rules
func (h *GovernanceHandler) handleListMaskingRules(w http.ResponseWriter, r *http.Request) {
	if h.masking == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "masking engine not configured")
		return
	}

	// Return stats which includes rule counts
	stats := h.masking.Stats()

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"stats": stats,
	})
}

// handleAddMaskingRule handles POST /v1/governance/mask/rules
func (h *GovernanceHandler) handleAddMaskingRule(w http.ResponseWriter, r *http.Request) {
	if h.masking == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "masking engine not configured")
		return
	}

	var req MaskingRuleRequest
	if err := decodeJSONBody(r, &req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Field == "" || req.Type == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "field and type are required")
		return
	}

	// Build PIICategory from first category if provided
	var piiCategory governance.PIICategory
	if len(req.Categories) > 0 {
		piiCategory = governance.PIICategory(req.Categories[0])
	}

	rule := &governance.MaskingRule{
		FeatureName: req.Field,
		MaskingType: governance.MaskingType(req.Type),
		PIICategory: piiCategory,
		Sensitivity: governance.PIISensitivity(req.Sensitivity),
		Enabled:     true,
	}

	h.masking.AddRule(rule)

	h.writeJSON(r.Context(), w, http.StatusCreated, map[string]interface{}{
		"success":      true,
		"feature_name": req.Field,
	})
}

// handleRemoveMaskingRule handles DELETE /v1/governance/mask/rules/{id}
// Note: The id parameter is treated as the feature name for masking rules.
func (h *GovernanceHandler) handleRemoveMaskingRule(w http.ResponseWriter, r *http.Request) {
	if h.masking == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "masking engine not configured")
		return
	}

	featureName := r.PathValue("id")
	if featureName == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "feature name required")
		return
	}

	h.masking.RemoveRule(featureName)

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success": true,
	})
}
