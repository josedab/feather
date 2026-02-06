package governance

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

// Errors returned by residency operations.
var (
	ErrResidencyViolation     = errors.New("data residency violation")
	ErrUnknownRegion          = errors.New("unknown region")
	ErrRegionNotAllowed       = errors.New("region not allowed for this data")
	ErrCrossRegionTransfer    = errors.New("cross-region transfer not allowed")
	ErrInvalidResidencyPolicy = errors.New("invalid residency policy")
)

// Region represents a geographic region.
type Region string

// Region constants.
const (
	RegionUSEast      Region = "us-east"
	RegionUSWest      Region = "us-west"
	RegionEUWest      Region = "eu-west"
	RegionEUCentral   Region = "eu-central"
	RegionAPSoutheast Region = "ap-southeast"
	RegionAPNortheast Region = "ap-northeast"
	RegionSAEast      Region = "sa-east"
	RegionGlobal      Region = "global"
)

// RegionZone maps regions to compliance zones.
type RegionZone string

// RegionZone constants.
const (
	ZoneUS     RegionZone = "us"     // United States
	ZoneEU     RegionZone = "eu"     // European Union (GDPR)
	ZoneAPAC   RegionZone = "apac"   // Asia-Pacific
	ZoneLATAM  RegionZone = "latam"  // Latin America
	ZoneGlobal RegionZone = "global" // No restriction
)

// ResidencyRequirement defines data residency requirements.
type ResidencyRequirement string

// ResidencyRequirement constants.
const (
	RequirementNone       ResidencyRequirement = "none"        // No requirement
	RequirementSameZone   ResidencyRequirement = "same_zone"   // Must stay in same zone
	RequirementSameRegion ResidencyRequirement = "same_region" // Must stay in same region
	RequirementSpecific   ResidencyRequirement = "specific"    // Must be in specific regions
)

// DataClassification classifies data for residency purposes.
type DataClassification string

// DataClassification constants.
const (
	ClassificationPublic       DataClassification = "public"
	ClassificationInternal     DataClassification = "internal"
	ClassificationConfidential DataClassification = "confidential"
	ClassificationRestricted   DataClassification = "restricted"
)

// ResidencyPolicy defines a data residency policy.
type ResidencyPolicy struct {
	// ID is the unique policy identifier.
	ID string `json:"id"`

	// Name is the human-readable name.
	Name string `json:"name"`

	// Description describes the policy.
	Description string `json:"description,omitempty"`

	// FeaturePattern is a glob pattern for matching features.
	FeaturePattern string `json:"feature_pattern,omitempty"`

	// FeatureNames are specific features this policy applies to.
	FeatureNames []string `json:"feature_names,omitempty"`

	// Classification is the data classification level.
	Classification DataClassification `json:"classification"`

	// Requirement is the residency requirement type.
	Requirement ResidencyRequirement `json:"requirement"`

	// AllowedRegions lists regions where data can be stored.
	AllowedRegions []Region `json:"allowed_regions,omitempty"`

	// AllowedZones lists zones where data can be stored.
	AllowedZones []RegionZone `json:"allowed_zones,omitempty"`

	// DenyRegions lists regions where data cannot be stored.
	DenyRegions []Region `json:"deny_regions,omitempty"`

	// AllowCrossRegionRead allows reading from other regions.
	AllowCrossRegionRead bool `json:"allow_cross_region_read"`

	// AllowCrossRegionWrite allows writing from other regions.
	AllowCrossRegionWrite bool `json:"allow_cross_region_write"`

	// AllowExport allows data export outside the region.
	AllowExport bool `json:"allow_export"`

	// TenantIDs restricts policy to specific tenants.
	TenantIDs []string `json:"tenant_ids,omitempty"`

	// Enabled indicates if the policy is active.
	Enabled bool `json:"enabled"`

	// Priority determines evaluation order.
	Priority int `json:"priority"`

	// CreatedAt is when the policy was created.
	CreatedAt time.Time `json:"created_at"`

	// UpdatedAt is when the policy was last modified.
	UpdatedAt time.Time `json:"updated_at"`
}

// ResidencyConfig configures the residency controller.
type ResidencyConfig struct {
	// Enabled enables residency enforcement.
	Enabled bool `json:"enabled" yaml:"enabled"`

	// CurrentRegion is the region of this deployment.
	CurrentRegion Region `json:"current_region" yaml:"current_region"`

	// CurrentZone is the compliance zone of this deployment.
	CurrentZone RegionZone `json:"current_zone" yaml:"current_zone"`

	// DefaultRequirement is the default residency requirement.
	DefaultRequirement ResidencyRequirement `json:"default_requirement" yaml:"default_requirement"`

	// EnforceOnWrite enforces residency on write operations.
	EnforceOnWrite bool `json:"enforce_on_write" yaml:"enforce_on_write"`

	// EnforceOnRead enforces residency on read operations.
	EnforceOnRead bool `json:"enforce_on_read" yaml:"enforce_on_read"`

	// EnforceOnExport enforces residency on export operations.
	EnforceOnExport bool `json:"enforce_on_export" yaml:"enforce_on_export"`

	// AuditEnabled enables residency auditing.
	AuditEnabled bool `json:"audit_enabled" yaml:"audit_enabled"`
}

// DefaultResidencyConfig returns the default residency configuration.
func DefaultResidencyConfig() ResidencyConfig {
	return ResidencyConfig{
		Enabled:            true,
		CurrentRegion:      RegionUSEast,
		CurrentZone:        ZoneUS,
		DefaultRequirement: RequirementNone,
		EnforceOnWrite:     true,
		EnforceOnRead:      false,
		EnforceOnExport:    true,
		AuditEnabled:       true,
	}
}

// ResidencyCheck represents a residency check result.
type ResidencyCheck struct {
	// Allowed indicates if the operation is allowed.
	Allowed bool `json:"allowed"`

	// FeatureName is the feature checked.
	FeatureName string `json:"feature_name"`

	// SourceRegion is the source region.
	SourceRegion Region `json:"source_region"`

	// TargetRegion is the target region.
	TargetRegion Region `json:"target_region"`

	// MatchedPolicy is the policy that matched.
	MatchedPolicy *ResidencyPolicy `json:"matched_policy,omitempty"`

	// Violation describes any violation.
	Violation string `json:"violation,omitempty"`

	// CheckedAt is when the check was performed.
	CheckedAt time.Time `json:"checked_at"`
}

// ResidencyController enforces data residency requirements.
type ResidencyController struct {
	mu        sync.RWMutex
	config    ResidencyConfig
	policies  map[string]*ResidencyPolicy
	byFeature map[string][]*ResidencyPolicy
	auditor   *AuditLogger

	// Metrics
	checksPerformed   int64
	violationsBlocked int64
	allowedOps        int64
}

// NewResidencyController creates a new residency controller.
func NewResidencyController(config ResidencyConfig, auditor *AuditLogger) *ResidencyController {
	return &ResidencyController{
		config:    config,
		policies:  make(map[string]*ResidencyPolicy),
		byFeature: make(map[string][]*ResidencyPolicy),
		auditor:   auditor,
	}
}

// AddPolicy adds a residency policy.
func (c *ResidencyController) AddPolicy(policy *ResidencyPolicy) error {
	if policy.ID == "" {
		return ErrInvalidResidencyPolicy
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	policy.CreatedAt = now
	policy.UpdatedAt = now

	c.policies[policy.ID] = policy

	// Index by feature
	for _, feature := range policy.FeatureNames {
		c.byFeature[feature] = append(c.byFeature[feature], policy)
	}

	return nil
}

// UpdatePolicy updates a residency policy.
func (c *ResidencyController) UpdatePolicy(policy *ResidencyPolicy) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	existing, exists := c.policies[policy.ID]
	if !exists {
		return ErrInvalidResidencyPolicy
	}

	// Remove from old feature indices
	for _, feature := range existing.FeatureNames {
		c.removeFromIndex(feature, existing.ID)
	}

	policy.CreatedAt = existing.CreatedAt
	policy.UpdatedAt = time.Now()

	c.policies[policy.ID] = policy

	// Add to new feature indices
	for _, feature := range policy.FeatureNames {
		c.byFeature[feature] = append(c.byFeature[feature], policy)
	}

	return nil
}

// DeletePolicy removes a residency policy.
func (c *ResidencyController) DeletePolicy(policyID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	policy, exists := c.policies[policyID]
	if !exists {
		return ErrInvalidResidencyPolicy
	}

	// Remove from feature indices
	for _, feature := range policy.FeatureNames {
		c.removeFromIndex(feature, policyID)
	}

	delete(c.policies, policyID)

	return nil
}

// removeFromIndex removes a policy from the feature index.
func (c *ResidencyController) removeFromIndex(feature, policyID string) {
	policies := c.byFeature[feature]
	for i, p := range policies {
		if p.ID == policyID {
			c.byFeature[feature] = append(policies[:i], policies[i+1:]...)
			break
		}
	}
}

// GetPolicy returns a policy by ID.
func (c *ResidencyController) GetPolicy(policyID string) (*ResidencyPolicy, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	policy, exists := c.policies[policyID]
	if !exists {
		return nil, ErrInvalidResidencyPolicy
	}

	return policy, nil
}

// ListPolicies returns all policies.
func (c *ResidencyController) ListPolicies() []*ResidencyPolicy {
	c.mu.RLock()
	defer c.mu.RUnlock()

	policies := make([]*ResidencyPolicy, 0, len(c.policies))
	for _, p := range c.policies {
		policies = append(policies, p)
	}

	return policies
}

// CheckWrite checks if a write operation is allowed.
func (c *ResidencyController) CheckWrite(ctx context.Context, feature string, sourceRegion Region) *ResidencyCheck {
	return c.check(ctx, feature, sourceRegion, c.config.CurrentRegion, "write")
}

// CheckRead checks if a read operation is allowed.
func (c *ResidencyController) CheckRead(ctx context.Context, feature string, requestRegion Region) *ResidencyCheck {
	return c.check(ctx, feature, c.config.CurrentRegion, requestRegion, "read")
}

// CheckExport checks if an export operation is allowed.
func (c *ResidencyController) CheckExport(ctx context.Context, feature string, targetRegion Region) *ResidencyCheck {
	return c.check(ctx, feature, c.config.CurrentRegion, targetRegion, "export")
}

// check performs a residency check.
func (c *ResidencyController) check(ctx context.Context, feature string, sourceRegion, targetRegion Region, operation string) *ResidencyCheck {
	if !c.config.Enabled {
		return &ResidencyCheck{
			Allowed:      true,
			FeatureName:  feature,
			SourceRegion: sourceRegion,
			TargetRegion: targetRegion,
			CheckedAt:    time.Now(),
		}
	}

	atomic.AddInt64(&c.checksPerformed, 1)

	// Get applicable policies
	c.mu.RLock()
	policies := c.byFeature[feature]
	c.mu.RUnlock()

	// Check each policy
	for _, policy := range policies {
		if !policy.Enabled {
			continue
		}

		check := c.evaluatePolicy(policy, feature, sourceRegion, targetRegion, operation)
		if !check.Allowed {
			atomic.AddInt64(&c.violationsBlocked, 1)
			c.auditViolation(ctx, check)
			return check
		}
	}

	// No policy violation
	atomic.AddInt64(&c.allowedOps, 1)

	return &ResidencyCheck{
		Allowed:      true,
		FeatureName:  feature,
		SourceRegion: sourceRegion,
		TargetRegion: targetRegion,
		CheckedAt:    time.Now(),
	}
}

// evaluatePolicy evaluates a single policy.
func (c *ResidencyController) evaluatePolicy(policy *ResidencyPolicy, feature string, sourceRegion, targetRegion Region, operation string) *ResidencyCheck {
	check := &ResidencyCheck{
		Allowed:       true,
		FeatureName:   feature,
		SourceRegion:  sourceRegion,
		TargetRegion:  targetRegion,
		MatchedPolicy: policy,
		CheckedAt:     time.Now(),
	}

	// Check denied regions
	for _, denied := range policy.DenyRegions {
		if sourceRegion == denied || targetRegion == denied {
			check.Allowed = false
			check.Violation = "Region " + string(denied) + " is denied"
			return check
		}
	}

	// Check allowed regions
	if len(policy.AllowedRegions) > 0 {
		sourceAllowed := false
		targetAllowed := false

		for _, allowed := range policy.AllowedRegions {
			if sourceRegion == allowed {
				sourceAllowed = true
			}
			if targetRegion == allowed {
				targetAllowed = true
			}
		}

		if !sourceAllowed {
			check.Allowed = false
			check.Violation = "Source region " + string(sourceRegion) + " not allowed"
			return check
		}
		if !targetAllowed {
			check.Allowed = false
			check.Violation = "Target region " + string(targetRegion) + " not allowed"
			return check
		}
	}

	// Check allowed zones
	if len(policy.AllowedZones) > 0 {
		sourceZone := getRegionZone(sourceRegion)
		targetZone := getRegionZone(targetRegion)

		sourceZoneAllowed := false
		targetZoneAllowed := false

		for _, allowed := range policy.AllowedZones {
			if sourceZone == allowed || allowed == ZoneGlobal {
				sourceZoneAllowed = true
			}
			if targetZone == allowed || allowed == ZoneGlobal {
				targetZoneAllowed = true
			}
		}

		if !sourceZoneAllowed {
			check.Allowed = false
			check.Violation = "Source zone " + string(sourceZone) + " not allowed"
			return check
		}
		if !targetZoneAllowed {
			check.Allowed = false
			check.Violation = "Target zone " + string(targetZone) + " not allowed"
			return check
		}
	}

	// Check cross-region operations
	if sourceRegion != targetRegion {
		switch operation {
		case "read":
			if !policy.AllowCrossRegionRead {
				check.Allowed = false
				check.Violation = "Cross-region read not allowed"
				return check
			}
		case "write":
			if !policy.AllowCrossRegionWrite {
				check.Allowed = false
				check.Violation = "Cross-region write not allowed"
				return check
			}
		case "export":
			if !policy.AllowExport {
				check.Allowed = false
				check.Violation = "Export not allowed"
				return check
			}
		}
	}

	// Check same-zone requirement
	if policy.Requirement == RequirementSameZone {
		if getRegionZone(sourceRegion) != getRegionZone(targetRegion) {
			check.Allowed = false
			check.Violation = "Data must stay in same zone"
			return check
		}
	}

	// Check same-region requirement
	if policy.Requirement == RequirementSameRegion {
		if sourceRegion != targetRegion {
			check.Allowed = false
			check.Violation = "Data must stay in same region"
			return check
		}
	}

	return check
}

// auditViolation logs a residency violation.
func (c *ResidencyController) auditViolation(ctx context.Context, check *ResidencyCheck) {
	if c.auditor == nil || !c.config.AuditEnabled {
		return
	}

	_ = c.auditor.Log(&AuditEvent{
		Action:   ActionAdminOp,
		Outcome:  OutcomeDenied,
		Severity: SeverityWarning,
		Resource: check.FeatureName,
		Metadata: map[string]interface{}{
			"violation":     check.Violation,
			"source_region": check.SourceRegion,
			"target_region": check.TargetRegion,
			"policy_id":     check.MatchedPolicy.ID,
		},
	})
}

// CheckBatch checks multiple features for residency.
func (c *ResidencyController) CheckBatch(ctx context.Context, features []string, sourceRegion, targetRegion Region, operation string) map[string]*ResidencyCheck {
	results := make(map[string]*ResidencyCheck)

	for _, feature := range features {
		results[feature] = c.check(ctx, feature, sourceRegion, targetRegion, operation)
	}

	return results
}

// FilterAllowed returns only features that pass residency checks.
func (c *ResidencyController) FilterAllowed(ctx context.Context, features []string, sourceRegion, targetRegion Region, operation string) []string {
	checks := c.CheckBatch(ctx, features, sourceRegion, targetRegion, operation)

	var allowed []string
	for feature, check := range checks {
		if check.Allowed {
			allowed = append(allowed, feature)
		}
	}

	return allowed
}

// Stats returns residency controller statistics.
func (c *ResidencyController) Stats() map[string]interface{} {
	c.mu.RLock()
	policyCount := len(c.policies)
	c.mu.RUnlock()

	return map[string]interface{}{
		"enabled":            c.config.Enabled,
		"current_region":     c.config.CurrentRegion,
		"current_zone":       c.config.CurrentZone,
		"policies":           policyCount,
		"checks_performed":   atomic.LoadInt64(&c.checksPerformed),
		"violations_blocked": atomic.LoadInt64(&c.violationsBlocked),
		"allowed_ops":        atomic.LoadInt64(&c.allowedOps),
	}
}

// getRegionZone maps a region to its compliance zone.
func getRegionZone(region Region) RegionZone {
	switch region {
	case RegionUSEast, RegionUSWest:
		return ZoneUS
	case RegionEUWest, RegionEUCentral:
		return ZoneEU
	case RegionAPSoutheast, RegionAPNortheast:
		return ZoneAPAC
	case RegionSAEast:
		return ZoneLATAM
	case RegionGlobal:
		return ZoneGlobal
	default:
		return ZoneGlobal
	}
}

// IsGDPRRegion checks if a region is under GDPR jurisdiction.
func IsGDPRRegion(region Region) bool {
	return region == RegionEUWest || region == RegionEUCentral
}

// RegionDisplayName returns a human-readable region name.
func RegionDisplayName(region Region) string {
	names := map[Region]string{
		RegionUSEast:      "US East (N. Virginia)",
		RegionUSWest:      "US West (Oregon)",
		RegionEUWest:      "EU West (Ireland)",
		RegionEUCentral:   "EU Central (Frankfurt)",
		RegionAPSoutheast: "Asia Pacific (Singapore)",
		RegionAPNortheast: "Asia Pacific (Tokyo)",
		RegionSAEast:      "South America (São Paulo)",
		RegionGlobal:      "Global",
	}

	if name, ok := names[region]; ok {
		return name
	}
	return string(region)
}
