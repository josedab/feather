package governance

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

// Errors returned by ACL operations.
var (
	ErrACLNotFound       = errors.New("ACL not found")
	ErrACLExists         = errors.New("ACL already exists")
	ErrACLAccessDenied   = errors.New("access denied")
	ErrInvalidACLConfig  = errors.New("invalid ACL configuration")
	ErrFeatureNotCovered = errors.New("feature not covered by any ACL")
)

// ACLPermission represents a specific permission.
type ACLPermission string

const (
	ACLPermissionRead   ACLPermission = "read"
	ACLPermissionWrite  ACLPermission = "write"
	ACLPermissionDelete ACLPermission = "delete"
	ACLPermissionAdmin  ACLPermission = "admin"
	ACLPermissionAll    ACLPermission = "all"
)

// ACLEffect is the result of an ACL evaluation.
type ACLEffect string

const (
	ACLEffectAllow ACLEffect = "allow"
	ACLEffectDeny  ACLEffect = "deny"
)

// ColumnACL defines access control for a specific feature/column.
type ColumnACL struct {
	// ID is the unique ACL identifier.
	ID string `json:"id"`

	// FeatureName is the feature this ACL applies to.
	FeatureName string `json:"feature_name"`

	// FeaturePattern is a glob pattern for matching features.
	FeaturePattern string `json:"feature_pattern,omitempty"`

	// Effect is whether to allow or deny access.
	Effect ACLEffect `json:"effect"`

	// Permissions are the permissions this ACL grants/denies.
	Permissions []ACLPermission `json:"permissions"`

	// Principals are the users/roles this ACL applies to.
	Principals []ACLPrincipal `json:"principals"`

	// Conditions are additional conditions for evaluation.
	Conditions []ACLCondition `json:"conditions,omitempty"`

	// Priority determines evaluation order (higher = first).
	Priority int `json:"priority"`

	// Enabled indicates if the ACL is active.
	Enabled bool `json:"enabled"`

	// Description describes the ACL.
	Description string `json:"description,omitempty"`

	// CreatedAt is when the ACL was created.
	CreatedAt time.Time `json:"created_at"`

	// UpdatedAt is when the ACL was last modified.
	UpdatedAt time.Time `json:"updated_at"`

	// CreatedBy is who created the ACL.
	CreatedBy string `json:"created_by,omitempty"`
}

// ACLPrincipal identifies who an ACL applies to.
type ACLPrincipal struct {
	// Type is the principal type (user, role, group, tenant).
	Type string `json:"type"`

	// ID is the principal identifier.
	ID string `json:"id"`
}

// ACLCondition is a condition for ACL evaluation.
type ACLCondition struct {
	// Type is the condition type.
	Type string `json:"type"` // "time", "ip", "purpose", "sensitivity"

	// Operator is the comparison operator.
	Operator string `json:"operator"` // "equals", "not_equals", "in", "not_in", "gt", "lt"

	// Value is the condition value.
	Value interface{} `json:"value"`
}

// ACLRequest represents a request for ACL evaluation.
type ACLRequest struct {
	// FeatureName is the feature being accessed.
	FeatureName string

	// Permission is the requested permission.
	Permission ACLPermission

	// Principal is the requesting principal.
	Principal ACLPrincipal

	// Context provides additional evaluation context.
	Context ACLEvaluationContext
}

// ACLEvaluationContext provides context for ACL evaluation.
type ACLEvaluationContext struct {
	// UserID is the requesting user.
	UserID string

	// TenantID is the tenant context.
	TenantID string

	// Roles are the user's roles.
	Roles []string

	// Groups are the user's groups.
	Groups []string

	// SourceIP is the request source IP.
	SourceIP string

	// Time is the request time.
	Time time.Time

	// Purpose is the access purpose.
	Purpose string

	// Sensitivity is the data sensitivity level.
	Sensitivity PIISensitivity

	// Metadata contains additional context.
	Metadata map[string]interface{}
}

// ACLDecision represents the result of an ACL evaluation.
type ACLDecision struct {
	// Allowed indicates if access is allowed.
	Allowed bool `json:"allowed"`

	// Effect is the resulting effect.
	Effect ACLEffect `json:"effect"`

	// MatchedACL is the ACL that matched.
	MatchedACL *ColumnACL `json:"matched_acl,omitempty"`

	// Reason explains the decision.
	Reason string `json:"reason,omitempty"`

	// EvaluatedAt is when the decision was made.
	EvaluatedAt time.Time `json:"evaluated_at"`

	// EvaluationTime is how long evaluation took.
	EvaluationTime time.Duration `json:"evaluation_time_ns"`
}

// ACLConfig configures the ACL system.
type ACLConfig struct {
	// Enabled enables ACL enforcement.
	Enabled bool `json:"enabled" yaml:"enabled"`

	// DefaultEffect is the default effect when no ACL matches.
	DefaultEffect ACLEffect `json:"default_effect" yaml:"default_effect"`

	// EnforceOnRead enables enforcement on read operations.
	EnforceOnRead bool `json:"enforce_on_read" yaml:"enforce_on_read"`

	// EnforceOnWrite enables enforcement on write operations.
	EnforceOnWrite bool `json:"enforce_on_write" yaml:"enforce_on_write"`

	// CacheEnabled enables decision caching.
	CacheEnabled bool `json:"cache_enabled" yaml:"cache_enabled"`

	// CacheTTL is the cache entry TTL.
	CacheTTL time.Duration `json:"cache_ttl" yaml:"cache_ttl"`

	// AuditEnabled enables decision auditing.
	AuditEnabled bool `json:"audit_enabled" yaml:"audit_enabled"`
}

// DefaultACLConfig returns the default ACL configuration.
func DefaultACLConfig() ACLConfig {
	return ACLConfig{
		Enabled:        true,
		DefaultEffect:  ACLEffectAllow, // Fail-open by default
		EnforceOnRead:  true,
		EnforceOnWrite: true,
		CacheEnabled:   true,
		CacheTTL:       5 * time.Minute,
		AuditEnabled:   true,
	}
}

// ColumnACLController manages column-level access control.
type ColumnACLController struct {
	mu        sync.RWMutex
	config    ACLConfig
	acls      map[string]*ColumnACL      // by ACL ID
	byFeature map[string][]*ColumnACL    // by feature name
	cache     map[string]*cachedDecision // decision cache
	auditor   *AuditLogger

	// Metrics
	evaluations  int64
	allowedCount int64
	deniedCount  int64
	cacheHits    int64
	cacheMisses  int64
}

type cachedDecision struct {
	decision  *ACLDecision
	expiresAt time.Time
}

// NewColumnACLController creates a new ACL controller.
func NewColumnACLController(config ACLConfig, auditor *AuditLogger) *ColumnACLController {
	return &ColumnACLController{
		config:    config,
		acls:      make(map[string]*ColumnACL),
		byFeature: make(map[string][]*ColumnACL),
		cache:     make(map[string]*cachedDecision),
		auditor:   auditor,
	}
}

// AddACL adds a new ACL.
func (c *ColumnACLController) AddACL(acl *ColumnACL) error {
	if acl.ID == "" {
		return ErrInvalidACLConfig
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.acls[acl.ID]; exists {
		return ErrACLExists
	}

	now := time.Now()
	acl.CreatedAt = now
	acl.UpdatedAt = now

	c.acls[acl.ID] = acl

	// Index by feature
	if acl.FeatureName != "" {
		c.byFeature[acl.FeatureName] = append(c.byFeature[acl.FeatureName], acl)
	}

	// Clear cache
	c.cache = make(map[string]*cachedDecision)

	return nil
}

// UpdateACL updates an existing ACL.
func (c *ColumnACLController) UpdateACL(acl *ColumnACL) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	existing, exists := c.acls[acl.ID]
	if !exists {
		return ErrACLNotFound
	}

	// Remove from old feature index
	if existing.FeatureName != "" {
		c.removeFromIndex(existing.FeatureName, existing.ID)
	}

	acl.CreatedAt = existing.CreatedAt
	acl.UpdatedAt = time.Now()

	c.acls[acl.ID] = acl

	// Add to new feature index
	if acl.FeatureName != "" {
		c.byFeature[acl.FeatureName] = append(c.byFeature[acl.FeatureName], acl)
	}

	// Clear cache
	c.cache = make(map[string]*cachedDecision)

	return nil
}

// DeleteACL removes an ACL.
func (c *ColumnACLController) DeleteACL(aclID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	acl, exists := c.acls[aclID]
	if !exists {
		return ErrACLNotFound
	}

	// Remove from feature index
	if acl.FeatureName != "" {
		c.removeFromIndex(acl.FeatureName, aclID)
	}

	delete(c.acls, aclID)

	// Clear cache
	c.cache = make(map[string]*cachedDecision)

	return nil
}

// removeFromIndex removes an ACL from the feature index.
func (c *ColumnACLController) removeFromIndex(featureName, aclID string) {
	acls := c.byFeature[featureName]
	for i, a := range acls {
		if a.ID == aclID {
			c.byFeature[featureName] = append(acls[:i], acls[i+1:]...)
			break
		}
	}
}

// GetACL returns an ACL by ID.
func (c *ColumnACLController) GetACL(aclID string) (*ColumnACL, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	acl, exists := c.acls[aclID]
	if !exists {
		return nil, ErrACLNotFound
	}

	return acl, nil
}

// ListACLs returns all ACLs.
func (c *ColumnACLController) ListACLs() []*ColumnACL {
	c.mu.RLock()
	defer c.mu.RUnlock()

	acls := make([]*ColumnACL, 0, len(c.acls))
	for _, acl := range c.acls {
		acls = append(acls, acl)
	}

	return acls
}

// ListACLsForFeature returns ACLs for a specific feature.
func (c *ColumnACLController) ListACLsForFeature(featureName string) []*ColumnACL {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.byFeature[featureName]
}

// Evaluate evaluates an ACL request.
func (c *ColumnACLController) Evaluate(ctx context.Context, req *ACLRequest) *ACLDecision {
	if !c.config.Enabled {
		return &ACLDecision{
			Allowed:     true,
			Effect:      ACLEffectAllow,
			Reason:      "ACL enforcement disabled",
			EvaluatedAt: time.Now(),
		}
	}

	start := time.Now()
	atomic.AddInt64(&c.evaluations, 1)

	// Check cache
	cacheKey := c.buildCacheKey(req)
	if c.config.CacheEnabled {
		if decision := c.checkCache(cacheKey); decision != nil {
			atomic.AddInt64(&c.cacheHits, 1)
			return decision
		}
		atomic.AddInt64(&c.cacheMisses, 1)
	}

	// Get applicable ACLs
	c.mu.RLock()
	acls := c.byFeature[req.FeatureName]
	c.mu.RUnlock()

	// Evaluate ACLs in priority order
	decision := c.evaluateACLs(acls, req)
	decision.EvaluatedAt = time.Now()
	decision.EvaluationTime = time.Since(start)

	// Update metrics
	if decision.Allowed {
		atomic.AddInt64(&c.allowedCount, 1)
	} else {
		atomic.AddInt64(&c.deniedCount, 1)
	}

	// Cache decision
	if c.config.CacheEnabled {
		c.cacheDecision(cacheKey, decision)
	}

	// Audit
	if c.config.AuditEnabled && c.auditor != nil {
		c.auditDecision(ctx, req, decision)
	}

	return decision
}

// evaluateACLs evaluates a list of ACLs against a request.
func (c *ColumnACLController) evaluateACLs(acls []*ColumnACL, req *ACLRequest) *ACLDecision {
	// Sort by priority (highest first)
	sortedACLs := make([]*ColumnACL, len(acls))
	copy(sortedACLs, acls)
	// Simple bubble sort for small lists
	for i := 0; i < len(sortedACLs); i++ {
		for j := i + 1; j < len(sortedACLs); j++ {
			if sortedACLs[j].Priority > sortedACLs[i].Priority {
				sortedACLs[i], sortedACLs[j] = sortedACLs[j], sortedACLs[i]
			}
		}
	}

	// Evaluate each ACL
	for _, acl := range sortedACLs {
		if !acl.Enabled {
			continue
		}

		// Check if principal matches
		if !c.principalMatches(acl, req) {
			continue
		}

		// Check if permission matches
		if !c.permissionMatches(acl, req) {
			continue
		}

		// Check conditions
		if !c.conditionsMatch(acl, req) {
			continue
		}

		// ACL matches
		return &ACLDecision{
			Allowed:    acl.Effect == ACLEffectAllow,
			Effect:     acl.Effect,
			MatchedACL: acl,
			Reason:     "Matched ACL: " + acl.ID,
		}
	}

	// No ACL matched, use default effect
	return &ACLDecision{
		Allowed: c.config.DefaultEffect == ACLEffectAllow,
		Effect:  c.config.DefaultEffect,
		Reason:  "No matching ACL, using default effect",
	}
}

// principalMatches checks if the request principal matches the ACL.
func (c *ColumnACLController) principalMatches(acl *ColumnACL, req *ACLRequest) bool {
	if len(acl.Principals) == 0 {
		return true // No principal restriction
	}

	for _, p := range acl.Principals {
		// Check direct match
		if p.Type == req.Principal.Type && p.ID == req.Principal.ID {
			return true
		}

		// Check wildcard
		if p.ID == "*" && p.Type == req.Principal.Type {
			return true
		}

		// Check role match
		if p.Type == "role" {
			for _, role := range req.Context.Roles {
				if role == p.ID {
					return true
				}
			}
		}

		// Check group match
		if p.Type == "group" {
			for _, group := range req.Context.Groups {
				if group == p.ID {
					return true
				}
			}
		}

		// Check tenant match
		if p.Type == "tenant" && p.ID == req.Context.TenantID {
			return true
		}
	}

	return false
}

// permissionMatches checks if the request permission matches the ACL.
func (c *ColumnACLController) permissionMatches(acl *ColumnACL, req *ACLRequest) bool {
	if len(acl.Permissions) == 0 {
		return true
	}

	for _, p := range acl.Permissions {
		if p == ACLPermissionAll || p == req.Permission {
			return true
		}
	}

	return false
}

// conditionsMatch checks if all ACL conditions are satisfied.
func (c *ColumnACLController) conditionsMatch(acl *ColumnACL, req *ACLRequest) bool {
	if len(acl.Conditions) == 0 {
		return true
	}

	for _, cond := range acl.Conditions {
		if !c.evaluateCondition(cond, req) {
			return false
		}
	}

	return true
}

// evaluateCondition evaluates a single condition.
func (c *ColumnACLController) evaluateCondition(cond ACLCondition, req *ACLRequest) bool {
	switch cond.Type {
	case "time":
		return c.evaluateTimeCondition(cond, req.Context.Time)
	case "ip":
		return c.evaluateIPCondition(cond, req.Context.SourceIP)
	case "purpose":
		return c.evaluatePurposeCondition(cond, req.Context.Purpose)
	case "sensitivity":
		return c.evaluateSensitivityCondition(cond, req.Context.Sensitivity)
	default:
		return true // Unknown condition type, pass
	}
}

func (c *ColumnACLController) evaluateTimeCondition(cond ACLCondition, t time.Time) bool {
	// Simple time window check
	if rangeVal, ok := cond.Value.(map[string]interface{}); ok {
		if startHour, ok := rangeVal["start_hour"].(float64); ok {
			if endHour, ok := rangeVal["end_hour"].(float64); ok {
				hour := float64(t.Hour())
				return hour >= startHour && hour < endHour
			}
		}
	}
	return true
}

func (c *ColumnACLController) evaluateIPCondition(cond ACLCondition, ip string) bool {
	if cond.Operator == "equals" {
		return ip == cond.Value.(string)
	}
	if cond.Operator == "in" {
		if ips, ok := cond.Value.([]interface{}); ok {
			for _, allowed := range ips {
				if ip == allowed.(string) {
					return true
				}
			}
		}
	}
	return false
}

func (c *ColumnACLController) evaluatePurposeCondition(cond ACLCondition, purpose string) bool {
	if cond.Operator == "equals" {
		return purpose == cond.Value.(string)
	}
	if cond.Operator == "in" {
		if purposes, ok := cond.Value.([]interface{}); ok {
			for _, allowed := range purposes {
				if purpose == allowed.(string) {
					return true
				}
			}
		}
	}
	return false
}

func (c *ColumnACLController) evaluateSensitivityCondition(cond ACLCondition, sensitivity PIISensitivity) bool {
	sensitivityOrder := map[PIISensitivity]int{
		SensitivityLow:      0,
		SensitivityMedium:   1,
		SensitivityHigh:     2,
		SensitivityCritical: 3,
	}

	if condSensitivity, ok := cond.Value.(string); ok {
		condLevel := sensitivityOrder[PIISensitivity(condSensitivity)]
		reqLevel := sensitivityOrder[sensitivity]

		switch cond.Operator {
		case "lt":
			return reqLevel < condLevel
		case "lte":
			return reqLevel <= condLevel
		case "gt":
			return reqLevel > condLevel
		case "gte":
			return reqLevel >= condLevel
		case "equals":
			return reqLevel == condLevel
		}
	}

	return true
}

// buildCacheKey creates a cache key for a request.
func (c *ColumnACLController) buildCacheKey(req *ACLRequest) string {
	return req.FeatureName + ":" + string(req.Permission) + ":" +
		req.Principal.Type + ":" + req.Principal.ID + ":" +
		req.Context.TenantID
}

// checkCache checks the decision cache.
func (c *ColumnACLController) checkCache(key string) *ACLDecision {
	c.mu.RLock()
	cached, ok := c.cache[key]
	c.mu.RUnlock()

	if !ok {
		return nil
	}

	if time.Now().After(cached.expiresAt) {
		return nil
	}

	return cached.decision
}

// cacheDecision adds a decision to the cache.
func (c *ColumnACLController) cacheDecision(key string, decision *ACLDecision) {
	c.mu.Lock()
	c.cache[key] = &cachedDecision{
		decision:  decision,
		expiresAt: time.Now().Add(c.config.CacheTTL),
	}
	c.mu.Unlock()
}

// auditDecision audits an ACL decision.
func (c *ColumnACLController) auditDecision(ctx context.Context, req *ACLRequest, decision *ACLDecision) {
	if c.auditor == nil {
		return
	}

	action := ActionRead
	if req.Permission == ACLPermissionWrite {
		action = ActionWrite
	} else if req.Permission == ACLPermissionDelete {
		action = ActionDelete
	}

	outcome := OutcomeSuccess
	if !decision.Allowed {
		outcome = OutcomeDenied
	}

	_ = c.auditor.Log(&AuditEvent{
		Action:   action,
		Outcome:  outcome,
		UserID:   req.Context.UserID,
		TenantID: req.Context.TenantID,
		Resource: req.FeatureName,
		SourceIP: req.Context.SourceIP,
		Metadata: map[string]interface{}{
			"permission":  req.Permission,
			"decision":    decision.Effect,
			"matched_acl": decision.MatchedACL != nil,
			"reason":      decision.Reason,
		},
	})
}

// EvaluateBatch evaluates multiple features in a batch.
func (c *ColumnACLController) EvaluateBatch(ctx context.Context, features []string, permission ACLPermission, evalCtx ACLEvaluationContext) map[string]*ACLDecision {
	results := make(map[string]*ACLDecision)

	for _, feature := range features {
		req := &ACLRequest{
			FeatureName: feature,
			Permission:  permission,
			Principal: ACLPrincipal{
				Type: "user",
				ID:   evalCtx.UserID,
			},
			Context: evalCtx,
		}

		results[feature] = c.Evaluate(ctx, req)
	}

	return results
}

// FilterAllowed returns only features that are allowed.
func (c *ColumnACLController) FilterAllowed(ctx context.Context, features []string, permission ACLPermission, evalCtx ACLEvaluationContext) []string {
	decisions := c.EvaluateBatch(ctx, features, permission, evalCtx)

	var allowed []string
	for feature, decision := range decisions {
		if decision.Allowed {
			allowed = append(allowed, feature)
		}
	}

	return allowed
}

// Stats returns ACL controller statistics.
func (c *ColumnACLController) Stats() map[string]interface{} {
	c.mu.RLock()
	aclCount := len(c.acls)
	cacheSize := len(c.cache)
	c.mu.RUnlock()

	return map[string]interface{}{
		"enabled":      c.config.Enabled,
		"acls":         aclCount,
		"cache_size":   cacheSize,
		"evaluations":  atomic.LoadInt64(&c.evaluations),
		"allowed":      atomic.LoadInt64(&c.allowedCount),
		"denied":       atomic.LoadInt64(&c.deniedCount),
		"cache_hits":   atomic.LoadInt64(&c.cacheHits),
		"cache_misses": atomic.LoadInt64(&c.cacheMisses),
	}
}
