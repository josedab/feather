package server

import (
	"net/http"
	"time"

	"github.com/feather-store/feather/internal/platform/governance"
)

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
