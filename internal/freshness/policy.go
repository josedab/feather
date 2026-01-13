package freshness

import (
	"errors"
	"sync"
	"time"
)

// Common errors
var (
	ErrPolicyNotFound = errors.New("policy not found")
	ErrPolicyExists   = errors.New("policy already exists")
	ErrInvalidPolicy  = errors.New("invalid policy configuration")
)

// PolicyType represents the type of freshness policy.
type PolicyType string

const (
	// PolicyTypeFixed uses a static TTL.
	PolicyTypeFixed PolicyType = "fixed"
	// PolicyTypeAdaptive uses ML-driven adaptive TTL.
	PolicyTypeAdaptive PolicyType = "adaptive"
	// PolicyTypeTime uses time-of-day based TTL.
	PolicyTypeTime PolicyType = "time"
	// PolicyTypeThreshold uses metric threshold based TTL.
	PolicyTypeThreshold PolicyType = "threshold"
)

// Policy defines a freshness policy for features.
type Policy struct {
	ID             string       `json:"id"`
	Name           string       `json:"name"`
	Type           PolicyType   `json:"type"`
	FeaturePattern string       `json:"feature_pattern"` // Glob pattern for matching features
	Priority       int          `json:"priority"`        // Higher priority takes precedence
	Enabled        bool         `json:"enabled"`
	Config         PolicyConfig `json:"config"`
	CreatedAt      time.Time    `json:"created_at"`
	UpdatedAt      time.Time    `json:"updated_at"`
}

// PolicyConfig holds type-specific configuration.
type PolicyConfig struct {
	// Fixed policy config
	FixedTTL time.Duration `json:"fixed_ttl,omitempty"`

	// Adaptive policy config
	MinTTL           time.Duration `json:"min_ttl,omitempty"`
	MaxTTL           time.Duration `json:"max_ttl,omitempty"`
	AccessWeight     float64       `json:"access_weight,omitempty"`
	VolatilityWeight float64       `json:"volatility_weight,omitempty"`
	DriftWeight      float64       `json:"drift_weight,omitempty"`

	// Time-based policy config
	PeakHoursStart int           `json:"peak_hours_start,omitempty"` // Hour of day (0-23)
	PeakHoursEnd   int           `json:"peak_hours_end,omitempty"`   // Hour of day (0-23)
	PeakTTL        time.Duration `json:"peak_ttl,omitempty"`
	OffPeakTTL     time.Duration `json:"off_peak_ttl,omitempty"`

	// Threshold-based policy config
	AccessRateThreshold float64       `json:"access_rate_threshold,omitempty"`
	HighAccessTTL       time.Duration `json:"high_access_ttl,omitempty"`
	LowAccessTTL        time.Duration `json:"low_access_ttl,omitempty"`
	DriftThreshold      float64       `json:"drift_threshold,omitempty"`
	HighDriftTTL        time.Duration `json:"high_drift_ttl,omitempty"`
}

// PolicyRegistry manages freshness policies.
type PolicyRegistry struct {
	policies map[string]*Policy
	mu       sync.RWMutex
}

// NewPolicyRegistry creates a new policy registry.
func NewPolicyRegistry() *PolicyRegistry {
	return &PolicyRegistry{
		policies: make(map[string]*Policy),
	}
}

// Register adds a new policy.
func (r *PolicyRegistry) Register(policy *Policy) error {
	if policy.ID == "" {
		return ErrInvalidPolicy
	}

	if err := r.validatePolicy(policy); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.policies[policy.ID]; exists {
		return ErrPolicyExists
	}

	policy.CreatedAt = time.Now()
	policy.UpdatedAt = policy.CreatedAt
	r.policies[policy.ID] = policy

	return nil
}

// Update updates an existing policy.
func (r *PolicyRegistry) Update(policy *Policy) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	existing, exists := r.policies[policy.ID]
	if !exists {
		return ErrPolicyNotFound
	}

	if err := r.validatePolicy(policy); err != nil {
		return err
	}

	policy.CreatedAt = existing.CreatedAt
	policy.UpdatedAt = time.Now()
	r.policies[policy.ID] = policy

	return nil
}

// Delete removes a policy.
func (r *PolicyRegistry) Delete(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.policies[id]; !exists {
		return ErrPolicyNotFound
	}

	delete(r.policies, id)
	return nil
}

// Get retrieves a policy by ID.
func (r *PolicyRegistry) Get(id string) (*Policy, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	policy, exists := r.policies[id]
	if !exists {
		return nil, ErrPolicyNotFound
	}

	return policy, nil
}

// List returns all policies.
func (r *PolicyRegistry) List() []*Policy {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*Policy, 0, len(r.policies))
	for _, policy := range r.policies {
		result = append(result, policy)
	}
	return result
}

// FindPolicies returns policies matching a feature name, sorted by priority.
func (r *PolicyRegistry) FindPolicies(featureName string) []*Policy {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var matching []*Policy
	for _, policy := range r.policies {
		if !policy.Enabled {
			continue
		}
		if matchPattern(policy.FeaturePattern, featureName) {
			matching = append(matching, policy)
		}
	}

	// Sort by priority (higher first)
	sortPolicies(matching)

	return matching
}

// GetEffectivePolicy returns the highest priority enabled policy for a feature.
func (r *PolicyRegistry) GetEffectivePolicy(featureName string) *Policy {
	policies := r.FindPolicies(featureName)
	if len(policies) == 0 {
		return nil
	}
	return policies[0]
}

func (r *PolicyRegistry) validatePolicy(policy *Policy) error {
	switch policy.Type {
	case PolicyTypeFixed:
		if policy.Config.FixedTTL <= 0 {
			return errors.New("fixed_ttl must be positive")
		}
	case PolicyTypeAdaptive:
		if policy.Config.MinTTL >= policy.Config.MaxTTL {
			return errors.New("min_ttl must be less than max_ttl")
		}
	case PolicyTypeTime:
		if policy.Config.PeakHoursStart < 0 || policy.Config.PeakHoursStart > 23 {
			return errors.New("peak_hours_start must be 0-23")
		}
		if policy.Config.PeakHoursEnd < 0 || policy.Config.PeakHoursEnd > 23 {
			return errors.New("peak_hours_end must be 0-23")
		}
	case PolicyTypeThreshold:
		// No specific validation needed
	default:
		return errors.New("unknown policy type")
	}
	return nil
}

// PolicyEvaluator evaluates policies to determine TTL.
type PolicyEvaluator struct {
	registry  *PolicyRegistry
	monitor   *Monitor
	predictor *Predictor
}

// NewPolicyEvaluator creates a new policy evaluator.
func NewPolicyEvaluator(registry *PolicyRegistry, monitor *Monitor, predictor *Predictor) *PolicyEvaluator {
	return &PolicyEvaluator{
		registry:  registry,
		monitor:   monitor,
		predictor: predictor,
	}
}

// EvaluationResult contains the result of policy evaluation.
type EvaluationResult struct {
	FeatureName string        `json:"feature_name"`
	TTL         time.Duration `json:"ttl"`
	PolicyID    string        `json:"policy_id,omitempty"`
	PolicyName  string        `json:"policy_name,omitempty"`
	PolicyType  PolicyType    `json:"policy_type,omitempty"`
	Reason      string        `json:"reason"`
	EvaluatedAt time.Time     `json:"evaluated_at"`
}

// Evaluate determines the appropriate TTL for a feature.
func (e *PolicyEvaluator) Evaluate(featureName string) *EvaluationResult {
	policy := e.registry.GetEffectivePolicy(featureName)

	if policy == nil {
		// No policy - use predictor default
		if e.predictor != nil {
			pred := e.predictor.Predict(featureName)
			return &EvaluationResult{
				FeatureName: featureName,
				TTL:         pred.RecommendedTTL,
				Reason:      "adaptive prediction (no policy): " + pred.Reason,
				EvaluatedAt: time.Now(),
			}
		}
		return &EvaluationResult{
			FeatureName: featureName,
			TTL:         5 * time.Minute, // Default
			Reason:      "default (no policy or predictor)",
			EvaluatedAt: time.Now(),
		}
	}

	result := &EvaluationResult{
		FeatureName: featureName,
		PolicyID:    policy.ID,
		PolicyName:  policy.Name,
		PolicyType:  policy.Type,
		EvaluatedAt: time.Now(),
	}

	switch policy.Type {
	case PolicyTypeFixed:
		result.TTL = policy.Config.FixedTTL
		result.Reason = "fixed TTL from policy"

	case PolicyTypeAdaptive:
		if e.predictor != nil {
			pred := e.predictor.Predict(featureName)
			// Clamp to policy limits
			result.TTL = clampDuration(pred.RecommendedTTL, policy.Config.MinTTL, policy.Config.MaxTTL)
			result.Reason = "adaptive prediction: " + pred.Reason
		} else {
			result.TTL = policy.Config.MinTTL + (policy.Config.MaxTTL-policy.Config.MinTTL)/2
			result.Reason = "adaptive midpoint (no predictor)"
		}

	case PolicyTypeTime:
		result.TTL, result.Reason = e.evaluateTimePolicy(policy)

	case PolicyTypeThreshold:
		result.TTL, result.Reason = e.evaluateThresholdPolicy(policy, featureName)

	default:
		result.TTL = 5 * time.Minute
		result.Reason = "unknown policy type, using default"
	}

	return result
}

func (e *PolicyEvaluator) evaluateTimePolicy(policy *Policy) (time.Duration, string) {
	hour := time.Now().Hour()

	var isPeak bool
	if policy.Config.PeakHoursStart <= policy.Config.PeakHoursEnd {
		// Normal case: peak hours don't wrap around midnight
		isPeak = hour >= policy.Config.PeakHoursStart && hour < policy.Config.PeakHoursEnd
	} else {
		// Wrap around case: e.g., 22:00 to 06:00
		isPeak = hour >= policy.Config.PeakHoursStart || hour < policy.Config.PeakHoursEnd
	}

	if isPeak {
		return policy.Config.PeakTTL, "peak hours TTL"
	}
	return policy.Config.OffPeakTTL, "off-peak hours TTL"
}

func (e *PolicyEvaluator) evaluateThresholdPolicy(policy *Policy, featureName string) (time.Duration, string) {
	accessMetrics, hasAccess := e.monitor.GetAccessMetrics(featureName)
	changeMetrics, hasChange := e.monitor.GetChangeMetrics(featureName)

	// Check drift threshold first (more important)
	if hasChange && policy.Config.DriftThreshold > 0 {
		if changeMetrics.DriftScore >= policy.Config.DriftThreshold {
			return policy.Config.HighDriftTTL, "high drift detected"
		}
	}

	// Check access rate threshold
	if hasAccess && policy.Config.AccessRateThreshold > 0 {
		if accessMetrics.AccessRate >= policy.Config.AccessRateThreshold {
			return policy.Config.HighAccessTTL, "high access rate"
		}
		return policy.Config.LowAccessTTL, "low access rate"
	}

	// Default to low access TTL
	return policy.Config.LowAccessTTL, "default threshold"
}

// EvaluateAll evaluates policies for all tracked features.
func (e *PolicyEvaluator) EvaluateAll() []*EvaluationResult {
	metrics := e.monitor.GetAllAccessMetrics()

	results := make([]*EvaluationResult, 0, len(metrics))
	for _, m := range metrics {
		results = append(results, e.Evaluate(m.FeatureName))
	}
	return results
}

// Helper functions

func matchPattern(pattern, name string) bool {
	// Simple glob matching: * matches any sequence
	if pattern == "" || pattern == "*" {
		return true
	}

	// Exact match
	if pattern == name {
		return true
	}

	// Prefix match with *
	if len(pattern) > 0 && pattern[len(pattern)-1] == '*' {
		prefix := pattern[:len(pattern)-1]
		return len(name) >= len(prefix) && name[:len(prefix)] == prefix
	}

	// Suffix match with *
	if len(pattern) > 0 && pattern[0] == '*' {
		suffix := pattern[1:]
		return len(name) >= len(suffix) && name[len(name)-len(suffix):] == suffix
	}

	return false
}

func sortPolicies(policies []*Policy) {
	// Simple insertion sort (usually few policies)
	for i := 1; i < len(policies); i++ {
		key := policies[i]
		j := i - 1
		for j >= 0 && policies[j].Priority < key.Priority {
			policies[j+1] = policies[j]
			j--
		}
		policies[j+1] = key
	}
}

func clampDuration(d, minD, maxD time.Duration) time.Duration {
	if d < minD {
		return minD
	}
	if d > maxD {
		return maxD
	}
	return d
}
