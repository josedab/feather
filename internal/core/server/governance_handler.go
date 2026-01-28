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
