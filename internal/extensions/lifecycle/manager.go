package lifecycle

import (
	"fmt"
	"sort"
	"sync"
	"time"
)

// FeatureState represents the lifecycle state of a feature.
type FeatureState string

const (
	StateActive     FeatureState = "active"
	StateWarning    FeatureState = "warning"
	StateDeprecated FeatureState = "deprecated"
	StateArchived   FeatureState = "archived"
)

// ManagerConfig configures the lifecycle manager.
type ManagerConfig struct {
	MaxFeatures           int           `json:"max_features"`
	DeprecationThresholdDays int        `json:"deprecation_threshold_days"`
	ArchivalThresholdDays int           `json:"archival_threshold_days"`
	DriftThreshold        float64       `json:"drift_threshold"`
	CostPerGBMonth        float64       `json:"cost_per_gb_month"`
	CheckInterval         time.Duration `json:"check_interval"`
}

// DefaultManagerConfig returns sensible defaults.
func DefaultManagerConfig() ManagerConfig {
	return ManagerConfig{
		MaxFeatures:              100000,
		DeprecationThresholdDays: 90,
		ArchivalThresholdDays:    180,
		DriftThreshold:           0.1,
		CostPerGBMonth:           0.023,
		CheckInterval:            time.Hour,
	}
}

// FeatureUsage tracks per-feature access metrics.
type FeatureUsage struct {
	Name           string       `json:"name"`
	State          FeatureState `json:"state"`
	AccessCount    int64        `json:"access_count"`
	LastAccessed   time.Time    `json:"last_accessed"`
	Consumers      map[string]int64 `json:"consumers"`
	StorageBytes   int64        `json:"storage_bytes"`
	DriftScore     float64      `json:"drift_score"`
	FreshnessScore float64      `json:"freshness_score"`
	CreatedAt      time.Time    `json:"created_at"`
	DeprecatedAt   *time.Time   `json:"deprecated_at,omitempty"`
}

// LifecycleRule defines a condition and action for automated management.
type LifecycleRule struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Condition   RuleCondition `json:"condition"`
	Action      RuleAction    `json:"action"`
	Enabled     bool          `json:"enabled"`
	CreatedAt   time.Time     `json:"created_at"`
}

// RuleCondition defines when a lifecycle rule triggers.
type RuleCondition struct {
	Type      string  `json:"type"` // "unused_days", "drift_threshold", "storage_threshold", "access_rate"
	Threshold float64 `json:"threshold"`
	Operator  string  `json:"operator"` // "gt", "lt", "eq"
}

// RuleAction defines what happens when a rule triggers.
type RuleAction struct {
	Type    string `json:"type"` // "deprecate", "archive", "alert", "tier_migrate", "compact"
	Target  string `json:"target,omitempty"`
	Message string `json:"message,omitempty"`
}

// LifecycleEvent records an automated action taken.
type LifecycleEvent struct {
	Feature   string       `json:"feature"`
	RuleID    string       `json:"rule_id"`
	Action    string       `json:"action"`
	OldState  FeatureState `json:"old_state"`
	NewState  FeatureState `json:"new_state"`
	Reason    string       `json:"reason"`
	Timestamp time.Time    `json:"timestamp"`
}

// CostReport summarizes storage costs.
type CostReport struct {
	TotalFeatures  int     `json:"total_features"`
	TotalStorageGB float64 `json:"total_storage_gb"`
	MonthlyEstUSD  float64 `json:"monthly_estimate_usd"`
	ByState        map[string]StateCost `json:"by_state"`
	TopCostly      []FeatureCost `json:"top_costly"`
}

// StateCost groups cost by feature state.
type StateCost struct {
	Count      int     `json:"count"`
	StorageGB  float64 `json:"storage_gb"`
	CostUSD    float64 `json:"cost_usd"`
}

// FeatureCost is a single feature's cost info.
type FeatureCost struct {
	Name      string  `json:"name"`
	StorageGB float64 `json:"storage_gb"`
	CostUSD   float64 `json:"cost_usd"`
	State     FeatureState `json:"state"`
}

// Manager provides autonomous feature lifecycle management.
type Manager struct {
	mu      sync.RWMutex
	config  ManagerConfig
	features map[string]*FeatureUsage
	rules    map[string]*LifecycleRule
	events   []LifecycleEvent
}

// NewManager creates a new lifecycle manager.
func NewManager(cfg ManagerConfig) *Manager {
	if cfg.MaxFeatures <= 0 {
		cfg = DefaultManagerConfig()
	}
	return &Manager{
		config:   cfg,
		features: make(map[string]*FeatureUsage),
		rules:    make(map[string]*LifecycleRule),
		events:   make([]LifecycleEvent, 0),
	}
}

// TrackFeature registers a feature for lifecycle tracking.
func (m *Manager) TrackFeature(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.features[name]; exists {
		return
	}
	m.features[name] = &FeatureUsage{
		Name:      name,
		State:     StateActive,
		Consumers: make(map[string]int64),
		CreatedAt: time.Now(),
	}
}

// RecordAccess records a feature access from a consumer.
func (m *Manager) RecordAccess(featureName, consumer string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	fu, ok := m.features[featureName]
	if !ok {
		fu = &FeatureUsage{
			Name:      featureName,
			State:     StateActive,
			Consumers: make(map[string]int64),
			CreatedAt: time.Now(),
		}
		m.features[featureName] = fu
	}

	fu.AccessCount++
	fu.LastAccessed = time.Now()
	if consumer != "" {
		fu.Consumers[consumer]++
	}
}

// UpdateMetrics updates drift, freshness, and storage metrics for a feature.
func (m *Manager) UpdateMetrics(name string, driftScore, freshnessScore float64, storageBytes int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	fu, ok := m.features[name]
	if !ok {
		return fmt.Errorf("feature %q not tracked", name)
	}

	fu.DriftScore = driftScore
	fu.FreshnessScore = freshnessScore
	fu.StorageBytes = storageBytes
	return nil
}

// AddRule adds a lifecycle automation rule.
func (m *Manager) AddRule(rule LifecycleRule) error {
	if rule.ID == "" || rule.Name == "" {
		return fmt.Errorf("rule id and name are required")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	rule.CreatedAt = time.Now()
	if !rule.Enabled {
		rule.Enabled = true
	}
	m.rules[rule.ID] = &rule
	return nil
}

// ListRules returns all lifecycle rules.
func (m *Manager) ListRules() []LifecycleRule {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]LifecycleRule, 0, len(m.rules))
	for _, r := range m.rules {
		result = append(result, *r)
	}
	return result
}

// RemoveRule removes a lifecycle rule.
func (m *Manager) RemoveRule(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.rules[id]; !ok {
		return fmt.Errorf("rule %q not found", id)
	}
	delete(m.rules, id)
	return nil
}

// Evaluate runs all enabled rules against all tracked features and returns events.
func (m *Manager) Evaluate() []LifecycleEvent {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	var events []LifecycleEvent

	for _, fu := range m.features {
		for _, rule := range m.rules {
			if !rule.Enabled {
				continue
			}

			if !m.conditionMet(fu, rule.Condition, now) {
				continue
			}

			event := m.applyAction(fu, rule, now)
			if event != nil {
				events = append(events, *event)
			}
		}

		// Built-in auto-deprecation based on config thresholds
		if fu.State == StateActive {
			daysSinceAccess := now.Sub(fu.LastAccessed).Hours() / 24
			if fu.AccessCount > 0 && daysSinceAccess > float64(m.config.DeprecationThresholdDays) {
				old := fu.State
				fu.State = StateDeprecated
				t := now
				fu.DeprecatedAt = &t
				events = append(events, LifecycleEvent{
					Feature:   fu.Name,
					Action:    "auto_deprecate",
					OldState:  old,
					NewState:  StateDeprecated,
					Reason:    fmt.Sprintf("unused for %.0f days (threshold: %d)", daysSinceAccess, m.config.DeprecationThresholdDays),
					Timestamp: now,
				})
			}
		}

		if fu.State == StateDeprecated && fu.DeprecatedAt != nil {
			daysSinceDeprecation := now.Sub(*fu.DeprecatedAt).Hours() / 24
			if daysSinceDeprecation > float64(m.config.ArchivalThresholdDays) {
				old := fu.State
				fu.State = StateArchived
				events = append(events, LifecycleEvent{
					Feature:   fu.Name,
					Action:    "auto_archive",
					OldState:  old,
					NewState:  StateArchived,
					Reason:    fmt.Sprintf("deprecated for %.0f days (threshold: %d)", daysSinceDeprecation, m.config.ArchivalThresholdDays),
					Timestamp: now,
				})
			}
		}
	}

	m.events = append(m.events, events...)
	if len(m.events) > 10000 {
		m.events = m.events[len(m.events)-10000:]
	}

	return events
}

func (m *Manager) conditionMet(fu *FeatureUsage, cond RuleCondition, now time.Time) bool {
	var value float64
	switch cond.Type {
	case "unused_days":
		if fu.LastAccessed.IsZero() {
			value = now.Sub(fu.CreatedAt).Hours() / 24
		} else {
			value = now.Sub(fu.LastAccessed).Hours() / 24
		}
	case "drift_threshold":
		value = fu.DriftScore
	case "storage_threshold":
		value = float64(fu.StorageBytes) / (1024 * 1024 * 1024) // GB
	case "access_rate":
		if !fu.LastAccessed.IsZero() {
			days := now.Sub(fu.CreatedAt).Hours() / 24
			if days > 0 {
				value = float64(fu.AccessCount) / days
			}
		}
	default:
		return false
	}

	switch cond.Operator {
	case "gt", ">":
		return value > cond.Threshold
	case "lt", "<":
		return value < cond.Threshold
	case "eq", "==":
		return value == cond.Threshold
	case "gte", ">=":
		return value >= cond.Threshold
	case "lte", "<=":
		return value <= cond.Threshold
	}
	return false
}

func (m *Manager) applyAction(fu *FeatureUsage, rule *LifecycleRule, now time.Time) *LifecycleEvent {
	oldState := fu.State
	var newState FeatureState

	switch rule.Action.Type {
	case "deprecate":
		if fu.State == StateActive || fu.State == StateWarning {
			newState = StateDeprecated
			t := now
			fu.DeprecatedAt = &t
		}
	case "archive":
		if fu.State != StateArchived {
			newState = StateArchived
		}
	case "alert":
		if fu.State == StateActive {
			newState = StateWarning
		}
	default:
		return nil
	}

	if newState == "" || newState == oldState {
		return nil
	}

	fu.State = newState
	return &LifecycleEvent{
		Feature:   fu.Name,
		RuleID:    rule.ID,
		Action:    rule.Action.Type,
		OldState:  oldState,
		NewState:  newState,
		Reason:    rule.Action.Message,
		Timestamp: now,
	}
}

// GetFeature returns usage info for a tracked feature.
func (m *Manager) GetFeature(name string) (*FeatureUsage, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	fu, ok := m.features[name]
	if !ok {
		return nil, fmt.Errorf("feature %q not tracked", name)
	}
	cp := *fu
	return &cp, nil
}

// ListFeatures returns all tracked features.
func (m *Manager) ListFeatures() []FeatureUsage {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]FeatureUsage, 0, len(m.features))
	for _, fu := range m.features {
		result = append(result, *fu)
	}
	return result
}

// GetEvents returns recent lifecycle events.
func (m *Manager) GetEvents(limit int) []LifecycleEvent {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if limit <= 0 || limit > len(m.events) {
		limit = len(m.events)
	}
	start := len(m.events) - limit
	result := make([]LifecycleEvent, limit)
	copy(result, m.events[start:])
	return result
}

// CostReport generates a storage cost report.
func (m *Manager) CostReport(topN int) *CostReport {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if topN <= 0 {
		topN = 10
	}

	report := &CostReport{
		TotalFeatures: len(m.features),
		ByState:       make(map[string]StateCost),
	}

	var allCosts []FeatureCost
	for _, fu := range m.features {
		gb := float64(fu.StorageBytes) / (1024 * 1024 * 1024)
		cost := gb * m.config.CostPerGBMonth

		report.TotalStorageGB += gb
		report.MonthlyEstUSD += cost

		sc := report.ByState[string(fu.State)]
		sc.Count++
		sc.StorageGB += gb
		sc.CostUSD += cost
		report.ByState[string(fu.State)] = sc

		allCosts = append(allCosts, FeatureCost{
			Name:      fu.Name,
			StorageGB: gb,
			CostUSD:   cost,
			State:     fu.State,
		})
	}

	sort.Slice(allCosts, func(i, j int) bool {
		return allCosts[i].CostUSD > allCosts[j].CostUSD
	})
	if len(allCosts) > topN {
		allCosts = allCosts[:topN]
	}
	report.TopCostly = allCosts

	return report
}

// ManagerStats holds aggregate statistics.
type ManagerStats struct {
	TotalFeatures    int                  `json:"total_features"`
	ActiveFeatures   int                  `json:"active_features"`
	DeprecatedCount  int                  `json:"deprecated_count"`
	ArchivedCount    int                  `json:"archived_count"`
	TotalRules       int                  `json:"total_rules"`
	TotalEvents      int                  `json:"total_events"`
	TotalConsumers   int                  `json:"total_consumers"`
}

// Stats returns manager statistics.
func (m *Manager) Stats() ManagerStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := ManagerStats{
		TotalFeatures: len(m.features),
		TotalRules:    len(m.rules),
		TotalEvents:   len(m.events),
	}

	consumers := make(map[string]bool)
	for _, fu := range m.features {
		switch fu.State {
		case StateActive:
			stats.ActiveFeatures++
		case StateDeprecated:
			stats.DeprecatedCount++
		case StateArchived:
			stats.ArchivedCount++
		}
		for c := range fu.Consumers {
			consumers[c] = true
		}
	}
	stats.TotalConsumers = len(consumers)

	return stats
}
