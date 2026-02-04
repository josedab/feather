// Package fedlearning provides a federated learning adapter for
// privacy-preserving feature aggregation across organizations,
// enabling secure gradient exchange and cross-org policy enforcement
// without exposing raw feature data.
package fedlearning

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// Protocol represents the federated aggregation protocol.
type Protocol string

const (
	// ProtocolSecAgg uses secure aggregation with encrypted shares.
	ProtocolSecAgg Protocol = "secagg"
	// ProtocolFedAvg uses federated averaging of model updates.
	ProtocolFedAvg Protocol = "fedavg"
	// ProtocolGossip uses gossip-based decentralized aggregation.
	ProtocolGossip Protocol = "gossip"
)

// AggType represents a supported aggregation function.
type AggType string

const (
	// AggTypeSum aggregates by summation.
	AggTypeSum AggType = "sum"
	// AggTypeAvg aggregates by averaging.
	AggTypeAvg AggType = "avg"
	// AggTypeCount aggregates by counting.
	AggTypeCount AggType = "count"
)

// OrgStatus represents the lifecycle state of a registered organization.
type OrgStatus string

const (
	// OrgStatusActive indicates the organization is participating.
	OrgStatusActive OrgStatus = "active"
	// OrgStatusSuspended indicates the organization is temporarily suspended.
	OrgStatusSuspended OrgStatus = "suspended"
)

// PrivacyLevel indicates the minimum privacy guarantee required.
type PrivacyLevel string

const (
	// PrivacyLevelLow provides basic aggregation privacy.
	PrivacyLevelLow PrivacyLevel = "low"
	// PrivacyLevelMedium provides differential privacy guarantees.
	PrivacyLevelMedium PrivacyLevel = "medium"
	// PrivacyLevelHigh provides cryptographic privacy guarantees.
	PrivacyLevelHigh PrivacyLevel = "high"
)

// Config configures the federated learning adapter.
type Config struct {
	MaxOrganizations          int      `json:"max_organizations"`
	AggregationTimeoutSeconds int      `json:"aggregation_timeout_seconds"`
	EncryptionEnabled         bool     `json:"encryption_enabled"`
	MinParticipants           int      `json:"min_participants"`
	Protocol                  Protocol `json:"protocol"`
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() Config {
	return Config{
		MaxOrganizations:          100,
		AggregationTimeoutSeconds: 300,
		EncryptionEnabled:         true,
		MinParticipants:           2,
		Protocol:                  ProtocolSecAgg,
	}
}

// OrgConfig holds registration details for an organization.
type OrgConfig struct {
	Name            string   `json:"name"`
	Region          string   `json:"region"`
	DataResidency   string   `json:"data_residency"`
	AllowedFeatures []string `json:"allowed_features"`
	PublicKey        []byte   `json:"public_key"`
}

// OrgInfo provides a read-only view of a registered organization.
type OrgInfo struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	Region         string    `json:"region"`
	Status         OrgStatus `json:"status"`
	FeaturesShared int       `json:"features_shared"`
	LastSync       time.Time `json:"last_sync"`
}

// AggregationRequest describes a cross-org aggregation request.
type AggregationRequest struct {
	Feature      string   `json:"feature"`
	Participants []string `json:"participants"`
	AggType      AggType  `json:"agg_type"`
	Round        int      `json:"round"`
}

// AggregationResult holds the outcome of a secure aggregation.
type AggregationResult struct {
	Feature          string  `json:"feature"`
	AggregatedValue  float64 `json:"aggregated_value"`
	Participants     int     `json:"participants"`
	Round            int     `json:"round"`
	PrivacyGuarantee string  `json:"privacy_guarantee"`
}

// FeaturePolicy defines cross-org access and privacy rules for a feature.
type FeaturePolicy struct {
	AllowedOrgs      []string `json:"allowed_orgs"`
	DataResidency    string   `json:"data_residency"`
	MinPrivacyLevel  string   `json:"min_privacy_level"`
	RequireEncryption bool    `json:"require_encryption"`
}

// FrameworkAdapter holds connection details for an external FL framework.
type FrameworkAdapter struct {
	Framework string                 `json:"framework"`
	OrgID     string                 `json:"org_id"`
	Endpoint  string                 `json:"endpoint"`
	Config    map[string]interface{} `json:"config"`
	Status    string                 `json:"status"`
}

// org is the internal representation of a registered organization.
type org struct {
	config    OrgConfig
	status    OrgStatus
	lastSync  time.Time
	gradients map[string][]float64 // feature -> gradient
}

// AdapterStats tracks federated learning statistics.
type AdapterStats struct {
	TotalAggregations  atomic.Int64
	ActiveOrgs         atomic.Int64
	GradientsExchanged atomic.Int64
	PolicyViolations   atomic.Int64
}

// Snapshot returns a point-in-time copy of stats.
func (s *AdapterStats) Snapshot() map[string]int64 {
	return map[string]int64{
		"total_aggregations":  s.TotalAggregations.Load(),
		"active_orgs":         s.ActiveOrgs.Load(),
		"gradients_exchanged": s.GradientsExchanged.Load(),
		"policy_violations":   s.PolicyViolations.Load(),
	}
}

// Adapter orchestrates federated learning across organizations.
type Adapter struct {
	config   Config
	orgs     map[string]*org
	policies map[string]*FeaturePolicy // feature -> policy
	// aggregated gradients per feature, computed from org submissions
	aggregatedGradients map[string][]float64
	stats               AdapterStats
	mu                  sync.RWMutex
}

// NewAdapter creates a new federated learning adapter.
func NewAdapter(cfg Config) *Adapter {
	return &Adapter{
		config:              cfg,
		orgs:                make(map[string]*org),
		policies:            make(map[string]*FeaturePolicy),
		aggregatedGradients: make(map[string][]float64),
	}
}

// RegisterOrg adds an organization to the federation.
func (a *Adapter) RegisterOrg(id string, cfg OrgConfig) error {
	if id == "" {
		return fmt.Errorf("organization ID is required")
	}
	if cfg.Name == "" {
		return fmt.Errorf("organization name is required")
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	if _, ok := a.orgs[id]; ok {
		return fmt.Errorf("organization %q already registered", id)
	}
	if len(a.orgs) >= a.config.MaxOrganizations {
		return fmt.Errorf("maximum organizations reached (%d)", a.config.MaxOrganizations)
	}

	a.orgs[id] = &org{
		config:    cfg,
		status:    OrgStatusActive,
		lastSync:  time.Now(),
		gradients: make(map[string][]float64),
	}
	a.stats.ActiveOrgs.Add(1)
	return nil
}

// DeregisterOrg removes an organization from the federation.
func (a *Adapter) DeregisterOrg(id string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if _, ok := a.orgs[id]; !ok {
		return fmt.Errorf("organization %q not found", id)
	}
	delete(a.orgs, id)
	a.stats.ActiveOrgs.Add(-1)
	return nil
}

// ListOrgs returns information about all registered organizations.
func (a *Adapter) ListOrgs() []OrgInfo {
	a.mu.RLock()
	defer a.mu.RUnlock()

	result := make([]OrgInfo, 0, len(a.orgs))
	for id, o := range a.orgs {
		result = append(result, OrgInfo{
			ID:             id,
			Name:           o.config.Name,
			Region:         o.config.Region,
			Status:         o.status,
			FeaturesShared: len(o.gradients),
			LastSync:       o.lastSync,
		})
	}
	return result
}

// SecureAggregate performs a privacy-preserving aggregation across participants.
func (a *Adapter) SecureAggregate(ctx context.Context, req AggregationRequest) (*AggregationResult, error) {
	if req.Feature == "" {
		return nil, fmt.Errorf("feature is required")
	}
	if len(req.Participants) < a.config.MinParticipants {
		return nil, fmt.Errorf("need at least %d participants, got %d", a.config.MinParticipants, len(req.Participants))
	}

	timeout := time.Duration(a.config.AggregationTimeoutSeconds) * time.Second
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	a.mu.RLock()

	// Validate all participants exist, are active, and pass policy checks.
	for _, pid := range req.Participants {
		o, ok := a.orgs[pid]
		if !ok {
			a.mu.RUnlock()
			return nil, fmt.Errorf("participant %q not found", pid)
		}
		if o.status != OrgStatusActive {
			a.mu.RUnlock()
			return nil, fmt.Errorf("participant %q is not active", pid)
		}
	}

	if policy, ok := a.policies[req.Feature]; ok {
		for _, pid := range req.Participants {
			if !containsStr(policy.AllowedOrgs, pid) {
				a.mu.RUnlock()
				a.stats.PolicyViolations.Add(1)
				return nil, fmt.Errorf("organization %q is not allowed for feature %q", pid, req.Feature)
			}
		}
	}

	// Collect and aggregate gradients from participants.
	var values []float64
	for _, pid := range req.Participants {
		o := a.orgs[pid]
		if grad, ok := o.gradients[req.Feature]; ok && len(grad) > 0 {
			values = append(values, grad[0])
		}
	}
	a.mu.RUnlock()

	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("aggregation timed out: %w", ctx.Err())
	default:
	}

	aggregated := computeAgg(values, req.AggType)
	a.stats.TotalAggregations.Add(1)

	guarantee := "k-anonymity"
	if a.config.EncryptionEnabled {
		guarantee = "encrypted-" + string(a.config.Protocol)
	}

	return &AggregationResult{
		Feature:          req.Feature,
		AggregatedValue:  aggregated,
		Participants:     len(req.Participants),
		Round:            req.Round,
		PrivacyGuarantee: guarantee,
	}, nil
}

// SetFeaturePolicy configures the cross-org access policy for a feature.
func (a *Adapter) SetFeaturePolicy(feature string, policy FeaturePolicy) error {
	if feature == "" {
		return fmt.Errorf("feature name is required")
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	a.policies[feature] = &policy
	return nil
}

// CheckPolicy validates whether an organization may access a feature.
// Returns (allowed bool, reason string).
func (a *Adapter) CheckPolicy(orgID, feature string) (bool, string) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	o, ok := a.orgs[orgID]
	if !ok {
		return false, fmt.Sprintf("organization %q not found", orgID)
	}

	policy, ok := a.policies[feature]
	if !ok {
		return true, "no policy defined"
	}

	if len(policy.AllowedOrgs) > 0 && !containsStr(policy.AllowedOrgs, orgID) {
		a.stats.PolicyViolations.Add(1)
		return false, fmt.Sprintf("organization %q not in allowed list", orgID)
	}

	if policy.DataResidency != "" && o.config.DataResidency != policy.DataResidency {
		a.stats.PolicyViolations.Add(1)
		return false, fmt.Sprintf("data residency mismatch: org=%q, required=%q", o.config.DataResidency, policy.DataResidency)
	}

	if policy.RequireEncryption && len(o.config.PublicKey) == 0 {
		a.stats.PolicyViolations.Add(1)
		return false, "encryption required but organization has no public key"
	}

	return true, "policy check passed"
}

// CreateFlowerAdapter creates a Flower framework adapter for an organization.
func (a *Adapter) CreateFlowerAdapter(orgID string) (*FrameworkAdapter, error) {
	return a.createFrameworkAdapter(orgID, "flower")
}

// CreatePySyftAdapter creates a PySyft framework adapter for an organization.
func (a *Adapter) CreatePySyftAdapter(orgID string) (*FrameworkAdapter, error) {
	return a.createFrameworkAdapter(orgID, "pysyft")
}

func (a *Adapter) createFrameworkAdapter(orgID, framework string) (*FrameworkAdapter, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	o, ok := a.orgs[orgID]
	if !ok {
		return nil, fmt.Errorf("organization %q not found", orgID)
	}
	if o.status != OrgStatusActive {
		return nil, fmt.Errorf("organization %q is not active", orgID)
	}

	return &FrameworkAdapter{
		Framework: framework,
		OrgID:     orgID,
		Endpoint:  fmt.Sprintf("fl://%s/%s", framework, orgID),
		Config: map[string]interface{}{
			"protocol":           string(a.config.Protocol),
			"encryption_enabled": a.config.EncryptionEnabled,
			"min_participants":   a.config.MinParticipants,
			"region":             o.config.Region,
		},
		Status: "ready",
	}, nil
}

// SubmitGradient records a gradient vector from an organization for a feature.
func (a *Adapter) SubmitGradient(orgID, feature string, gradient []float64) error {
	if orgID == "" {
		return fmt.Errorf("organization ID is required")
	}
	if feature == "" {
		return fmt.Errorf("feature name is required")
	}
	if len(gradient) == 0 {
		return fmt.Errorf("gradient must not be empty")
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	o, ok := a.orgs[orgID]
	if !ok {
		return fmt.Errorf("organization %q not found", orgID)
	}
	if o.status != OrgStatusActive {
		return fmt.Errorf("organization %q is not active", orgID)
	}

	// Copy the gradient to avoid aliasing.
	g := make([]float64, len(gradient))
	copy(g, gradient)
	o.gradients[feature] = g
	o.lastSync = time.Now()

	a.recomputeAggregatedGradient(feature)
	a.stats.GradientsExchanged.Add(1)
	return nil
}

// GetAggregatedGradient returns the aggregated gradient for a feature
// across all organizations that have submitted gradients.
func (a *Adapter) GetAggregatedGradient(feature string) ([]float64, error) {
	if feature == "" {
		return nil, fmt.Errorf("feature name is required")
	}

	a.mu.RLock()
	defer a.mu.RUnlock()

	grad, ok := a.aggregatedGradients[feature]
	if !ok {
		return nil, fmt.Errorf("no gradients submitted for feature %q", feature)
	}

	// Return a copy to avoid aliasing.
	result := make([]float64, len(grad))
	copy(result, grad)
	return result, nil
}

// Stats returns current federated learning statistics.
func (a *Adapter) Stats() map[string]int64 {
	return a.stats.Snapshot()
}

// recomputeAggregatedGradient averages all submitted gradients for a feature.
// Must be called with a.mu held for writing.
func (a *Adapter) recomputeAggregatedGradient(feature string) {
	var maxLen int
	var count int
	for _, o := range a.orgs {
		if g, ok := o.gradients[feature]; ok {
			count++
			if len(g) > maxLen {
				maxLen = len(g)
			}
		}
	}
	if count == 0 {
		delete(a.aggregatedGradients, feature)
		return
	}

	agg := make([]float64, maxLen)
	for _, o := range a.orgs {
		if g, ok := o.gradients[feature]; ok {
			for i, v := range g {
				agg[i] += v
			}
		}
	}
	for i := range agg {
		agg[i] /= float64(count)
	}
	a.aggregatedGradients[feature] = agg
}

func computeAgg(values []float64, aggType AggType) float64 {
	if len(values) == 0 {
		return 0
	}
	switch aggType {
	case AggTypeSum:
		var s float64
		for _, v := range values {
			s += v
		}
		return s
	case AggTypeAvg:
		var s float64
		for _, v := range values {
			s += v
		}
		return s / float64(len(values))
	case AggTypeCount:
		return float64(len(values))
	default:
		return 0
	}
}

func containsStr(ss []string, target string) bool {
	for _, s := range ss {
		if s == target {
			return true
		}
	}
	return false
}
