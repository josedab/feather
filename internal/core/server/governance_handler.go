package server

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/feather-store/feather/internal/platform/governance"
)

// GovernanceHandler handles enterprise governance API requests.
type GovernanceHandler struct {
	audit     *governance.AuditLogger
	pii       *governance.PIIDetector
	masking   *governance.DataMasker
	acl       *governance.ColumnACLController
	residency *governance.ResidencyController
	logger    *slog.Logger
}

// GovernanceHandlerConfig configures the governance handler.
type GovernanceHandlerConfig struct {
	Audit     *governance.AuditLogger
	PII       *governance.PIIDetector
	Masking   *governance.DataMasker
	ACL       *governance.ColumnACLController
	Residency *governance.ResidencyController
	Logger    *slog.Logger
}

// NewGovernanceHandler creates a new governance handler.
func NewGovernanceHandler(cfg GovernanceHandlerConfig) *GovernanceHandler {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	return &GovernanceHandler{
		audit:     cfg.Audit,
		pii:       cfg.PII,
		masking:   cfg.Masking,
		acl:       cfg.ACL,
		residency: cfg.Residency,
		logger:    logger,
	}
}

// RegisterRoutes registers governance API routes.
func (h *GovernanceHandler) RegisterRoutes(mux *http.ServeMux) {
	// Audit logging routes
	mux.HandleFunc("GET /v1/governance/audit", h.handleListAuditLogs)
	mux.HandleFunc("GET /v1/governance/audit/{id}", h.handleGetAuditLog)
	mux.HandleFunc("GET /v1/governance/audit/stats", h.handleAuditStats)

	// PII detection routes
	mux.HandleFunc("POST /v1/governance/pii/detect", h.handleDetectPII)
	mux.HandleFunc("POST /v1/governance/pii/scan", h.handleScanPII)
	mux.HandleFunc("GET /v1/governance/pii/patterns", h.handleListPIIPatterns)
	mux.HandleFunc("POST /v1/governance/pii/patterns", h.handleAddPIIPattern)
	mux.HandleFunc("DELETE /v1/governance/pii/patterns/{name}", h.handleRemovePIIPattern)

	// Data masking routes
	mux.HandleFunc("POST /v1/governance/mask", h.handleMaskData)
	mux.HandleFunc("POST /v1/governance/mask/batch", h.handleMaskBatch)
	mux.HandleFunc("GET /v1/governance/mask/rules", h.handleListMaskingRules)
	mux.HandleFunc("POST /v1/governance/mask/rules", h.handleAddMaskingRule)
	mux.HandleFunc("DELETE /v1/governance/mask/rules/{id}", h.handleRemoveMaskingRule)

	// Access control routes
	mux.HandleFunc("GET /v1/governance/acl", h.handleListACLs)
	mux.HandleFunc("POST /v1/governance/acl", h.handleCreateACL)
	mux.HandleFunc("GET /v1/governance/acl/{id}", h.handleGetACL)
	mux.HandleFunc("PUT /v1/governance/acl/{id}", h.handleUpdateACL)
	mux.HandleFunc("DELETE /v1/governance/acl/{id}", h.handleDeleteACL)
	mux.HandleFunc("POST /v1/governance/acl/check", h.handleCheckAccess)

	// Data residency routes
	mux.HandleFunc("GET /v1/governance/residency/policies", h.handleListResidencyPolicies)
	mux.HandleFunc("POST /v1/governance/residency/policies", h.handleAddResidencyPolicy)
	mux.HandleFunc("GET /v1/governance/residency/policies/{id}", h.handleGetResidencyPolicy)
	mux.HandleFunc("DELETE /v1/governance/residency/policies/{id}", h.handleDeleteResidencyPolicy)
	mux.HandleFunc("POST /v1/governance/residency/validate", h.handleValidateResidency)

	// Stats
	mux.HandleFunc("GET /v1/governance/stats", h.handleGetStats)
}

// Request/Response types

// PIIDetectionRequest represents a PII detection request.
type PIIDetectionRequest struct {
	Content    string   `json:"content"`
	Categories []string `json:"categories,omitempty"`
}

// PIIScanRequest represents a batch PII scan request.
type PIIScanRequest struct {
	Contents   []string `json:"contents"`
	Categories []string `json:"categories,omitempty"`
}

// PIIPatternRequest represents a PII pattern registration request.
type PIIPatternRequest struct {
	Name        string `json:"name"`
	Category    string `json:"category"`
	Pattern     string `json:"pattern"`
	Sensitivity string `json:"sensitivity,omitempty"`
	Description string `json:"description,omitempty"`
}

// MaskRequest represents a data masking request.
type MaskRequest struct {
	Value     string            `json:"value"`
	Field     string            `json:"field"`
	Type      string            `json:"type,omitempty"` // redact, partial, hash, tokenize
	Principal *PrincipalContext `json:"principal,omitempty"`
}

// MaskBatchRequest represents a batch masking request.
type MaskBatchRequest struct {
	Items     []MaskRequest     `json:"items"`
	Principal *PrincipalContext `json:"principal,omitempty"`
}

// MaskingRuleRequest represents a masking rule registration request.
type MaskingRuleRequest struct {
	ID          string   `json:"id"`
	Field       string   `json:"field"`
	Type        string   `json:"type"`
	Categories  []string `json:"categories,omitempty"`
	Sensitivity string   `json:"sensitivity,omitempty"`
	Priority    int      `json:"priority,omitempty"`
}

// ACLRequest represents an ACL creation/update request.
type ACLRequest struct {
	ID          string         `json:"id"`
	Resource    string         `json:"resource"`
	Principal   string         `json:"principal"`
	Permissions []string       `json:"permissions"`
	Effect      string         `json:"effect"` // allow or deny
	Conditions  *ACLConditions `json:"conditions,omitempty"`
	Priority    int            `json:"priority,omitempty"`
}

// ACLConditions represents ACL conditions.
type ACLConditions struct {
	TimeWindow     *TimeWindowCondition `json:"time_window,omitempty"`
	IPRange        string               `json:"ip_range,omitempty"`
	Purpose        []string             `json:"purpose,omitempty"`
	MaxSensitivity string               `json:"max_sensitivity,omitempty"`
}

// TimeWindowCondition represents a time-based condition.
type TimeWindowCondition struct {
	Start string `json:"start"`          // HH:MM
	End   string `json:"end"`            // HH:MM
	Days  []int  `json:"days,omitempty"` // 0=Sunday, 6=Saturday
}

// PrincipalContext represents the requesting principal.
type PrincipalContext struct {
	ID      string   `json:"id"`
	Roles   []string `json:"roles,omitempty"`
	Groups  []string `json:"groups,omitempty"`
	IP      string   `json:"ip,omitempty"`
	Purpose string   `json:"purpose,omitempty"`
}

// AccessCheckRequest represents an access check request.
type AccessCheckRequest struct {
	Resource    string            `json:"resource"`
	Permission  string            `json:"permission"`
	Principal   *PrincipalContext `json:"principal"`
	Sensitivity string            `json:"sensitivity,omitempty"`
}

// ResidencyPolicyRequest represents a residency policy request.
type ResidencyPolicyRequest struct {
	ID                    string   `json:"id"`
	Regions               []string `json:"regions"`
	Zones                 []string `json:"zones,omitempty"`
	RequireSameRegion     bool     `json:"require_same_region,omitempty"`
	RequireSameZone       bool     `json:"require_same_zone,omitempty"`
	AllowCrossRegionRead  bool     `json:"allow_cross_region_read,omitempty"`
	AllowCrossRegionWrite bool     `json:"allow_cross_region_write,omitempty"`
}

// ResidencyValidationRequest represents a residency validation request.
type ResidencyValidationRequest struct {
	SourceRegion string `json:"source_region"`
	SourceZone   string `json:"source_zone,omitempty"`
	TargetRegion string `json:"target_region"`
	TargetZone   string `json:"target_zone,omitempty"`
	PolicyID     string `json:"policy_id,omitempty"`
	Operation    string `json:"operation"` // read or write
}

// Audit handlers

// handleListAuditLogs handles GET /v1/governance/audit
func (h *GovernanceHandler) handleListAuditLogs(w http.ResponseWriter, r *http.Request) {
	if h.audit == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "audit logging not configured")
		return
	}

	limit := 100
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	var since time.Time
	if s := r.URL.Query().Get("since"); s != "" {
		if parsed, err := time.Parse(time.RFC3339, s); err == nil {
			since = parsed
		}
	}

	action := r.URL.Query().Get("action")
	resource := r.URL.Query().Get("resource")
	principal := r.URL.Query().Get("principal")

	filter := &governance.AuditFilter{
		Limit: limit,
	}

	// Set StartTime if since is provided
	if !since.IsZero() {
		filter.StartTime = &since
	}

	// Filter by action if provided
	if action != "" {
		filter.Actions = []governance.AuditAction{governance.AuditAction(action)}
	}

	// Filter by resource if provided
	if resource != "" {
		filter.Resources = []string{resource}
	}

	// Filter by principal (user) if provided
	if principal != "" {
		filter.UserIDs = []string{principal}
	}

	logs, err := h.audit.Query(r.Context(), filter)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"logs":  logs,
		"count": len(logs),
	})
}

// handleGetAuditLog handles GET /v1/governance/audit/{id}
// Note: Individual audit log retrieval by ID is not supported in the current implementation.
// Audit logs should be queried using the list endpoint with appropriate filters.
func (h *GovernanceHandler) handleGetAuditLog(w http.ResponseWriter, r *http.Request) {
	if h.audit == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "audit logging not configured")
		return
	}

	logID := r.PathValue("id")
	if logID == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "log ID required")
		return
	}

	// The audit logger doesn't support direct retrieval by ID.
	// In production, this would query a database with the log ID.
	h.writeError(r.Context(), w, http.StatusNotImplemented, "individual audit log retrieval not implemented; use query filters instead")
}

// handleAuditStats handles GET /v1/governance/audit/stats
func (h *GovernanceHandler) handleAuditStats(w http.ResponseWriter, r *http.Request) {
	if h.audit == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "audit logging not configured")
		return
	}

	stats := h.audit.Stats()
	h.writeJSON(r.Context(), w, http.StatusOK, stats)
}

// PII handlers

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

// ACL handlers

// handleListACLs handles GET /v1/governance/acl
func (h *GovernanceHandler) handleListACLs(w http.ResponseWriter, r *http.Request) {
	if h.acl == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "ACL controller not configured")
		return
	}

	// Get all ACLs and optionally filter by feature name
	allACLs := h.acl.ListACLs()
	featureFilter := r.URL.Query().Get("resource")

	var acls []*governance.ColumnACL
	if featureFilter != "" {
		// Filter by feature name
		for _, acl := range allACLs {
			if acl.FeatureName == featureFilter {
				acls = append(acls, acl)
			}
		}
	} else {
		acls = allACLs
	}

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"acls":  acls,
		"count": len(acls),
	})
}

// handleCreateACL handles POST /v1/governance/acl
func (h *GovernanceHandler) handleCreateACL(w http.ResponseWriter, r *http.Request) {
	if h.acl == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "ACL controller not configured")
		return
	}

	var req ACLRequest
	if err := decodeJSONBody(r, &req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.ID == "" || req.Resource == "" || req.Principal == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "id, resource, and principal are required")
		return
	}

	permissions := make([]governance.ACLPermission, len(req.Permissions))
	for i, p := range req.Permissions {
		permissions[i] = governance.ACLPermission(p)
	}

	// Build principals list from the single principal string
	principals := []governance.ACLPrincipal{
		{Type: "user", ID: req.Principal},
	}

	acl := &governance.ColumnACL{
		ID:          req.ID,
		FeatureName: req.Resource,
		Permissions: permissions,
		Principals:  principals,
		Effect:      governance.ACLEffect(req.Effect),
		Priority:    req.Priority,
		Enabled:     true,
	}

	// Convert conditions if provided
	if req.Conditions != nil {
		acl.Conditions = h.convertACLConditions(req.Conditions)
	}

	if err := h.acl.AddACL(acl); err != nil {
		h.writeError(r.Context(), w, http.StatusConflict, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusCreated, map[string]interface{}{
		"success": true,
		"acl_id":  req.ID,
	})
}

// handleGetACL handles GET /v1/governance/acl/{id}
func (h *GovernanceHandler) handleGetACL(w http.ResponseWriter, r *http.Request) {
	if h.acl == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "ACL controller not configured")
		return
	}

	aclID := r.PathValue("id")
	if aclID == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "ACL ID required")
		return
	}

	acl, err := h.acl.GetACL(aclID)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, acl)
}

// handleUpdateACL handles PUT /v1/governance/acl/{id}
func (h *GovernanceHandler) handleUpdateACL(w http.ResponseWriter, r *http.Request) {
	if h.acl == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "ACL controller not configured")
		return
	}

	aclID := r.PathValue("id")
	if aclID == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "ACL ID required")
		return
	}

	var req ACLRequest
	if err := decodeJSONBody(r, &req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	permissions := make([]governance.ACLPermission, len(req.Permissions))
	for i, p := range req.Permissions {
		permissions[i] = governance.ACLPermission(p)
	}

	// Build principals list from the single principal string
	var principals []governance.ACLPrincipal
	if req.Principal != "" {
		principals = []governance.ACLPrincipal{
			{Type: "user", ID: req.Principal},
		}
	}

	acl := &governance.ColumnACL{
		ID:          aclID,
		FeatureName: req.Resource,
		Permissions: permissions,
		Principals:  principals,
		Effect:      governance.ACLEffect(req.Effect),
		Priority:    req.Priority,
		Enabled:     true,
	}

	if req.Conditions != nil {
		acl.Conditions = h.convertACLConditions(req.Conditions)
	}

	if err := h.acl.UpdateACL(acl); err != nil {
		h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success": true,
	})
}

// handleDeleteACL handles DELETE /v1/governance/acl/{id}
func (h *GovernanceHandler) handleDeleteACL(w http.ResponseWriter, r *http.Request) {
	if h.acl == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "ACL controller not configured")
		return
	}

	aclID := r.PathValue("id")
	if aclID == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "ACL ID required")
		return
	}

	if err := h.acl.DeleteACL(aclID); err != nil {
		h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success": true,
	})
}

// handleCheckAccess handles POST /v1/governance/acl/check
func (h *GovernanceHandler) handleCheckAccess(w http.ResponseWriter, r *http.Request) {
	if h.acl == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "ACL controller not configured")
		return
	}

	var req AccessCheckRequest
	if err := decodeJSONBody(r, &req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Resource == "" || req.Permission == "" || req.Principal == nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "resource, permission, and principal are required")
		return
	}

	evalReq := &governance.ACLRequest{
		FeatureName: req.Resource,
		Permission:  governance.ACLPermission(req.Permission),
		Principal: governance.ACLPrincipal{
			Type: "user",
			ID:   req.Principal.ID,
		},
		Context: governance.ACLEvaluationContext{
			UserID:   req.Principal.ID,
			Roles:    req.Principal.Roles,
			Groups:   req.Principal.Groups,
			SourceIP: req.Principal.IP,
			Purpose:  req.Principal.Purpose,
			Time:     time.Now(),
		},
	}

	if req.Sensitivity != "" {
		evalReq.Context.Sensitivity = governance.PIISensitivity(req.Sensitivity)
	}

	result := h.acl.Evaluate(r.Context(), evalReq)

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"allowed": result.Allowed,
		"reason":  result.Reason,
		"effect":  result.Effect,
	})
}

// Residency handlers

// handleListResidencyPolicies handles GET /v1/governance/residency/policies
func (h *GovernanceHandler) handleListResidencyPolicies(w http.ResponseWriter, r *http.Request) {
	if h.residency == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "residency enforcer not configured")
		return
	}

	policies := h.residency.ListPolicies()

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"policies": policies,
		"count":    len(policies),
	})
}

// handleAddResidencyPolicy handles POST /v1/governance/residency/policies
func (h *GovernanceHandler) handleAddResidencyPolicy(w http.ResponseWriter, r *http.Request) {
	if h.residency == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "residency enforcer not configured")
		return
	}

	var req ResidencyPolicyRequest
	if err := decodeJSONBody(r, &req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.ID == "" || len(req.Regions) == 0 {
		h.writeError(r.Context(), w, http.StatusBadRequest, "id and regions are required")
		return
	}

	// Convert string slices to Region/RegionZone types
	allowedRegions := make([]governance.Region, len(req.Regions))
	for i, r := range req.Regions {
		allowedRegions[i] = governance.Region(r)
	}

	allowedZones := make([]governance.RegionZone, len(req.Zones))
	for i, z := range req.Zones {
		allowedZones[i] = governance.RegionZone(z)
	}

	// Determine requirement based on flags
	var requirement governance.ResidencyRequirement
	if req.RequireSameRegion {
		requirement = governance.RequirementSameRegion
	} else if req.RequireSameZone {
		requirement = governance.RequirementSameZone
	} else {
		requirement = governance.RequirementSpecific
	}

	policy := &governance.ResidencyPolicy{
		ID:                    req.ID,
		AllowedRegions:        allowedRegions,
		AllowedZones:          allowedZones,
		Requirement:           requirement,
		AllowCrossRegionRead:  req.AllowCrossRegionRead,
		AllowCrossRegionWrite: req.AllowCrossRegionWrite,
		Enabled:               true,
	}

	if err := h.residency.AddPolicy(policy); err != nil {
		h.writeError(r.Context(), w, http.StatusConflict, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusCreated, map[string]interface{}{
		"success":   true,
		"policy_id": req.ID,
	})
}

// handleGetResidencyPolicy handles GET /v1/governance/residency/policies/{id}
func (h *GovernanceHandler) handleGetResidencyPolicy(w http.ResponseWriter, r *http.Request) {
	if h.residency == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "residency enforcer not configured")
		return
	}

	policyID := r.PathValue("id")
	if policyID == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "policy ID required")
		return
	}

	policy, err := h.residency.GetPolicy(policyID)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, policy)
}

// handleDeleteResidencyPolicy handles DELETE /v1/governance/residency/policies/{id}
func (h *GovernanceHandler) handleDeleteResidencyPolicy(w http.ResponseWriter, r *http.Request) {
	if h.residency == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "residency enforcer not configured")
		return
	}

	policyID := r.PathValue("id")
	if policyID == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "policy ID required")
		return
	}

	if err := h.residency.DeletePolicy(policyID); err != nil {
		h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success": true,
	})
}

// handleValidateResidency handles POST /v1/governance/residency/validate
func (h *GovernanceHandler) handleValidateResidency(w http.ResponseWriter, r *http.Request) {
	if h.residency == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "residency enforcer not configured")
		return
	}

	var req ResidencyValidationRequest
	if err := decodeJSONBody(r, &req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.SourceRegion == "" || req.TargetRegion == "" || req.Operation == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "source_region, target_region, and operation are required")
		return
	}

	sourceRegion := governance.Region(req.SourceRegion)
	targetRegion := governance.Region(req.TargetRegion)

	// Use generic feature name if not provided
	feature := req.PolicyID
	if feature == "" {
		feature = "_residency_check"
	}

	// Use CheckBatch to validate the transfer between regions
	checks := h.residency.CheckBatch(r.Context(), []string{feature}, sourceRegion, targetRegion, req.Operation)
	check := checks[feature]

	var violation string
	if check != nil && !check.Allowed {
		violation = check.Violation
	}

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"valid":         check != nil && check.Allowed,
		"violation":     violation,
		"operation":     req.Operation,
		"source_region": req.SourceRegion,
		"target_region": req.TargetRegion,
	})
}

// handleGetStats handles GET /v1/governance/stats
func (h *GovernanceHandler) handleGetStats(w http.ResponseWriter, r *http.Request) {
	stats := make(map[string]interface{})

	if h.audit != nil {
		stats["audit"] = h.audit.Stats()
	}
	if h.pii != nil {
		stats["pii"] = h.pii.Stats()
	}
	if h.masking != nil {
		stats["masking"] = h.masking.Stats()
	}
	if h.acl != nil {
		stats["acl"] = h.acl.Stats()
	}
	if h.residency != nil {
		stats["residency"] = h.residency.Stats()
	}

	h.writeJSON(r.Context(), w, http.StatusOK, stats)
}

// Helper methods

// convertACLConditions converts the API request conditions to governance ACL conditions.
func (h *GovernanceHandler) convertACLConditions(req *ACLConditions) []governance.ACLCondition {
	var conditions []governance.ACLCondition

	// Convert IP range condition
	if req.IPRange != "" {
		conditions = append(conditions, governance.ACLCondition{
			Type:     "ip",
			Operator: "in",
			Value:    req.IPRange,
		})
	}

	// Convert purpose conditions
	if len(req.Purpose) > 0 {
		conditions = append(conditions, governance.ACLCondition{
			Type:     "purpose",
			Operator: "in",
			Value:    req.Purpose,
		})
	}

	// Convert sensitivity condition
	if req.MaxSensitivity != "" {
		conditions = append(conditions, governance.ACLCondition{
			Type:     "sensitivity",
			Operator: "lte",
			Value:    req.MaxSensitivity,
		})
	}

	// Convert time window condition
	if req.TimeWindow != nil {
		conditions = append(conditions, governance.ACLCondition{
			Type:     "time",
			Operator: "in",
			Value: map[string]interface{}{
				"start_hour": req.TimeWindow.Start,
				"end_hour":   req.TimeWindow.End,
				"days":       req.TimeWindow.Days,
			},
		})
	}

	return conditions
}

func (h *GovernanceHandler) writeJSON(ctx context.Context, w http.ResponseWriter, status int, data interface{}) {
	writeJSONResponse(ctx, w, status, data)
}

func (h *GovernanceHandler) writeError(ctx context.Context, w http.ResponseWriter, status int, message string) {
	writeJSONError(ctx, w, status, message)
}
