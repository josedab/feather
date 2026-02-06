package freshness

import "time"

// Manager provides the main interface for adaptive feature freshness.
type Manager struct {
	monitor   *Monitor
	predictor *Predictor
	registry  *PolicyRegistry
	evaluator *PolicyEvaluator
	config    ManagerConfig
}

// ManagerConfig configures the freshness manager.
type ManagerConfig struct {
	Monitor   MonitorConfig
	Predictor PredictorConfig
}

// DefaultManagerConfig returns sensible defaults.
func DefaultManagerConfig() ManagerConfig {
	return ManagerConfig{
		Monitor:   DefaultMonitorConfig(),
		Predictor: DefaultPredictorConfig(),
	}
}

// NewManager creates a new freshness manager.
func NewManager(config ManagerConfig) *Manager {
	monitor := NewMonitor(config.Monitor)
	predictor := NewPredictor(config.Predictor, monitor)
	registry := NewPolicyRegistry()
	evaluator := NewPolicyEvaluator(registry, monitor, predictor)

	return &Manager{
		monitor:   monitor,
		predictor: predictor,
		registry:  registry,
		evaluator: evaluator,
		config:    config,
	}
}

// RecordAccess records a feature access event.
func (m *Manager) RecordAccess(featureName string, latency time.Duration, cacheHit bool) {
	m.monitor.RecordAccess(featureName, latency, cacheHit)
}

// RecordStaleServe records when a stale value was served.
func (m *Manager) RecordStaleServe(featureName string) {
	m.monitor.RecordStaleServe(featureName)
}

// RecordChange records a feature value change.
func (m *Manager) RecordChange(featureName string, oldValue, newValue float64) {
	m.monitor.RecordChange(featureName, oldValue, newValue)
}

// RecordDriftScore records a drift score for a feature.
func (m *Manager) RecordDriftScore(featureName string, score float64) {
	m.monitor.RecordDriftScore(featureName, score)
}

// GetRecommendedTTL returns the recommended TTL for a feature.
func (m *Manager) GetRecommendedTTL(featureName string) time.Duration {
	result := m.evaluator.Evaluate(featureName)
	return result.TTL
}

// GetTTLWithReason returns the recommended TTL with explanation.
func (m *Manager) GetTTLWithReason(featureName string) *EvaluationResult {
	return m.evaluator.Evaluate(featureName)
}

// GetPrediction returns the ML-based prediction for a feature.
func (m *Manager) GetPrediction(featureName string) *Prediction {
	return m.predictor.Predict(featureName)
}

// GetAccessMetrics returns access metrics for a feature.
func (m *Manager) GetAccessMetrics(featureName string) (*AccessMetrics, bool) {
	return m.monitor.GetAccessMetrics(featureName)
}

// GetChangeMetrics returns change metrics for a feature.
func (m *Manager) GetChangeMetrics(featureName string) (*ChangeMetrics, bool) {
	return m.monitor.GetChangeMetrics(featureName)
}

// GetAllMetrics returns metrics for all tracked features.
func (m *Manager) GetAllMetrics() map[string]FeatureMetrics {
	accessMetrics := m.monitor.GetAllAccessMetrics()
	changeMetrics := m.monitor.GetAllChangeMetrics()

	// Build a map for change metrics
	changeMap := make(map[string]ChangeMetrics)
	for _, cm := range changeMetrics {
		changeMap[cm.FeatureName] = cm
	}

	result := make(map[string]FeatureMetrics)
	for _, am := range accessMetrics {
		fm := FeatureMetrics{
			FeatureName: am.FeatureName,
			Access:      am,
		}
		if cm, ok := changeMap[am.FeatureName]; ok {
			fm.Change = &cm
		}
		result[am.FeatureName] = fm
	}

	return result
}

// FeatureMetrics combines access and change metrics for a feature.
type FeatureMetrics struct {
	FeatureName string         `json:"feature_name"`
	Access      AccessMetrics  `json:"access"`
	Change      *ChangeMetrics `json:"change,omitempty"`
}

// RegisterPolicy registers a new freshness policy.
func (m *Manager) RegisterPolicy(policy *Policy) error {
	return m.registry.Register(policy)
}

// UpdatePolicy updates an existing policy.
func (m *Manager) UpdatePolicy(policy *Policy) error {
	return m.registry.Update(policy)
}

// DeletePolicy removes a policy.
func (m *Manager) DeletePolicy(id string) error {
	return m.registry.Delete(id)
}

// GetPolicy retrieves a policy by ID.
func (m *Manager) GetPolicy(id string) (*Policy, error) {
	return m.registry.Get(id)
}

// ListPolicies returns all policies.
func (m *Manager) ListPolicies() []*Policy {
	return m.registry.List()
}

// EvaluateAll evaluates policies for all tracked features.
func (m *Manager) EvaluateAll() []*EvaluationResult {
	return m.evaluator.EvaluateAll()
}

// GetAllPredictions returns predictions for all tracked features.
func (m *Manager) GetAllPredictions() []*Prediction {
	return m.predictor.GetAllPredictions()
}

// ManagerStats contains statistics about the manager.
type ManagerStats struct {
	Monitor   MonitorStats   `json:"monitor"`
	Predictor PredictorStats `json:"predictor"`
	Policies  int            `json:"policies"`
}

// Stats returns manager statistics.
func (m *Manager) Stats() ManagerStats {
	return ManagerStats{
		Monitor:   m.monitor.Stats(),
		Predictor: m.predictor.Stats(),
		Policies:  len(m.registry.List()),
	}
}

// Stop stops the manager and its components.
func (m *Manager) Stop() {
	m.predictor.Stop()
}

// Helper functions for common policy configurations

// NewFixedPolicy creates a fixed TTL policy.
func NewFixedPolicy(id, name, pattern string, ttl time.Duration, priority int) *Policy {
	return &Policy{
		ID:             id,
		Name:           name,
		Type:           PolicyTypeFixed,
		FeaturePattern: pattern,
		Priority:       priority,
		Enabled:        true,
		Config: PolicyConfig{
			FixedTTL: ttl,
		},
	}
}

// NewAdaptivePolicy creates an adaptive TTL policy.
func NewAdaptivePolicy(id, name, pattern string, minTTL, maxTTL time.Duration, priority int) *Policy {
	return &Policy{
		ID:             id,
		Name:           name,
		Type:           PolicyTypeAdaptive,
		FeaturePattern: pattern,
		Priority:       priority,
		Enabled:        true,
		Config: PolicyConfig{
			MinTTL:           minTTL,
			MaxTTL:           maxTTL,
			AccessWeight:     0.3,
			VolatilityWeight: 0.4,
			DriftWeight:      0.3,
		},
	}
}

// NewTimePolicy creates a time-based TTL policy.
func NewTimePolicy(id, name, pattern string, peakStart, peakEnd int, peakTTL, offPeakTTL time.Duration, priority int) *Policy {
	return &Policy{
		ID:             id,
		Name:           name,
		Type:           PolicyTypeTime,
		FeaturePattern: pattern,
		Priority:       priority,
		Enabled:        true,
		Config: PolicyConfig{
			PeakHoursStart: peakStart,
			PeakHoursEnd:   peakEnd,
			PeakTTL:        peakTTL,
			OffPeakTTL:     offPeakTTL,
		},
	}
}

// NewThresholdPolicy creates a threshold-based TTL policy.
func NewThresholdPolicy(id, name, pattern string, accessThreshold float64, highAccessTTL, lowAccessTTL time.Duration, driftThreshold float64, highDriftTTL time.Duration, priority int) *Policy {
	return &Policy{
		ID:             id,
		Name:           name,
		Type:           PolicyTypeThreshold,
		FeaturePattern: pattern,
		Priority:       priority,
		Enabled:        true,
		Config: PolicyConfig{
			AccessRateThreshold: accessThreshold,
			HighAccessTTL:       highAccessTTL,
			LowAccessTTL:        lowAccessTTL,
			DriftThreshold:      driftThreshold,
			HighDriftTTL:        highDriftTTL,
		},
	}
}
