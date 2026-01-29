package parity

import (
	"fmt"
	"math"
	"sync"
	"time"
)

// Config configures the parity checker.
type Config struct {
	// MaxSamples is the maximum number of value pairs to retain per feature.
	MaxSamples int `json:"max_samples"`
	// AbsoluteTolerance is the maximum allowed absolute difference for numeric values.
	AbsoluteTolerance float64 `json:"absolute_tolerance"`
	// RelativeTolerance is the maximum allowed relative difference (0-1).
	RelativeTolerance float64 `json:"relative_tolerance"`
	// AlertThreshold is the mismatch rate (0-1) that triggers an alert.
	AlertThreshold float64 `json:"alert_threshold"`
	// CheckInterval is how often periodic checks run.
	CheckInterval time.Duration `json:"check_interval"`
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() Config {
	return Config{
		MaxSamples:        10000,
		AbsoluteTolerance: 0.001,
		RelativeTolerance: 0.01,
		AlertThreshold:    0.05,
		CheckInterval:     5 * time.Minute,
	}
}

// ValuePair represents an online value paired with an offline value.
type ValuePair struct {
	OnlineValue  interface{} `json:"online_value"`
	OfflineValue interface{} `json:"offline_value"`
	EntityKey    string      `json:"entity_key"`
	Timestamp    time.Time   `json:"timestamp"`
	Match        bool        `json:"match"`
}

// FeatureParityStatus holds parity statistics for a single feature.
type FeatureParityStatus struct {
	FeatureName   string    `json:"feature_name"`
	TotalPairs    int64     `json:"total_pairs"`
	MatchCount    int64     `json:"match_count"`
	MismatchCount int64     `json:"mismatch_count"`
	MatchRate     float64   `json:"match_rate"`
	MismatchRate  float64   `json:"mismatch_rate"`
	MeanAbsDiff   float64   `json:"mean_abs_diff"`
	MaxAbsDiff    float64   `json:"max_abs_diff"`
	HasSkew       bool      `json:"has_skew"`
	LastChecked   time.Time `json:"last_checked"`
}

// Alert represents a parity violation alert.
type Alert struct {
	ID           string    `json:"id"`
	FeatureName  string    `json:"feature_name"`
	Severity     string    `json:"severity"` // "warning", "critical"
	Message      string    `json:"message"`
	MismatchRate float64   `json:"mismatch_rate"`
	CreatedAt    time.Time `json:"created_at"`
}

type featureState struct {
	pairs      []ValuePair
	totalPairs int64
	matchCount int64
	maxAbsDiff float64
	sumAbsDiff float64
}

// Checker validates online/offline feature parity.
type Checker struct {
	config Config
	states map[string]*featureState
	alerts []Alert
	mu     sync.RWMutex
}

// NewChecker creates a new parity checker.
func NewChecker(config Config) *Checker {
	return &Checker{
		config: config,
		states: make(map[string]*featureState),
		alerts: make([]Alert, 0),
	}
}

// RecordPair records an online/offline value pair for comparison.
func (c *Checker) RecordPair(featureName, entityKey string, onlineValue, offlineValue interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()

	state, ok := c.states[featureName]
	if !ok {
		state = &featureState{
			pairs: make([]ValuePair, 0, c.config.MaxSamples),
		}
		c.states[featureName] = state
	}

	match, absDiff := c.compareValues(onlineValue, offlineValue)

	pair := ValuePair{
		OnlineValue:  onlineValue,
		OfflineValue: offlineValue,
		EntityKey:    entityKey,
		Timestamp:    time.Now(),
		Match:        match,
	}

	if len(state.pairs) >= c.config.MaxSamples {
		// Evict oldest
		state.pairs = state.pairs[1:]
	}
	state.pairs = append(state.pairs, pair)

	state.totalPairs++
	if match {
		state.matchCount++
	}
	state.sumAbsDiff += absDiff
	if absDiff > state.maxAbsDiff {
		state.maxAbsDiff = absDiff
	}

	// Check if we should alert
	mismatchRate := 1.0 - float64(state.matchCount)/float64(state.totalPairs)
	if state.totalPairs >= 100 && mismatchRate > c.config.AlertThreshold {
		severity := "warning"
		if mismatchRate > c.config.AlertThreshold*2 {
			severity = "critical"
		}
		c.alerts = append(c.alerts, Alert{
			ID:           fmt.Sprintf("parity-%s-%d", featureName, time.Now().UnixNano()),
			FeatureName:  featureName,
			Severity:     severity,
			Message:      fmt.Sprintf("Feature %q mismatch rate %.2f%% exceeds threshold %.2f%%", featureName, mismatchRate*100, c.config.AlertThreshold*100),
			MismatchRate: mismatchRate,
			CreatedAt:    time.Now(),
		})
	}
}

func (c *Checker) compareValues(online, offline interface{}) (bool, float64) {
	onlineFloat, okOn := toFloat(online)
	offlineFloat, okOff := toFloat(offline)

	if okOn && okOff {
		absDiff := math.Abs(onlineFloat - offlineFloat)
		if absDiff <= c.config.AbsoluteTolerance {
			return true, absDiff
		}
		if offlineFloat != 0 {
			relDiff := absDiff / math.Abs(offlineFloat)
			if relDiff <= c.config.RelativeTolerance {
				return true, absDiff
			}
		}
		return false, absDiff
	}

	// String/bool/other: exact match
	if fmt.Sprintf("%v", online) == fmt.Sprintf("%v", offline) {
		return true, 0
	}
	return false, 1.0
}

func toFloat(v interface{}) (float64, bool) {
	switch val := v.(type) {
	case float64:
		return val, true
	case float32:
		return float64(val), true
	case int:
		return float64(val), true
	case int64:
		return float64(val), true
	case int32:
		return float64(val), true
	default:
		return 0, false
	}
}

// GetStatus returns parity status for a specific feature.
func (c *Checker) GetStatus(featureName string) *FeatureParityStatus {
	c.mu.RLock()
	defer c.mu.RUnlock()

	state, ok := c.states[featureName]
	if !ok {
		return nil
	}
	return c.buildStatus(featureName, state)
}

// GetAllStatuses returns parity status for all tracked features.
func (c *Checker) GetAllStatuses() []*FeatureParityStatus {
	c.mu.RLock()
	defer c.mu.RUnlock()

	statuses := make([]*FeatureParityStatus, 0, len(c.states))
	for name, state := range c.states {
		statuses = append(statuses, c.buildStatus(name, state))
	}
	return statuses
}

func (c *Checker) buildStatus(name string, state *featureState) *FeatureParityStatus {
	total := state.totalPairs
	matchCount := state.matchCount
	mismatchCount := total - matchCount

	matchRate := 0.0
	mismatchRate := 0.0
	meanAbsDiff := 0.0
	if total > 0 {
		matchRate = float64(matchCount) / float64(total)
		mismatchRate = float64(mismatchCount) / float64(total)
		meanAbsDiff = state.sumAbsDiff / float64(total)
	}

	return &FeatureParityStatus{
		FeatureName:   name,
		TotalPairs:    total,
		MatchCount:    matchCount,
		MismatchCount: mismatchCount,
		MatchRate:     matchRate,
		MismatchRate:  mismatchRate,
		MeanAbsDiff:   meanAbsDiff,
		MaxAbsDiff:    state.maxAbsDiff,
		HasSkew:       mismatchRate > c.config.AlertThreshold,
		LastChecked:   time.Now(),
	}
}

// GetAlerts returns all parity alerts, optionally filtered by time.
func (c *Checker) GetAlerts(since time.Time) []Alert {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make([]Alert, 0)
	for _, a := range c.alerts {
		if a.CreatedAt.After(since) || a.CreatedAt.Equal(since) {
			result = append(result, a)
		}
	}
	return result
}

// Reset clears parity data for a feature.
func (c *Checker) Reset(featureName string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, ok := c.states[featureName]; !ok {
		return fmt.Errorf("feature %q not tracked", featureName)
	}
	delete(c.states, featureName)
	return nil
}

// Summary provides overall parity health.
type Summary struct {
	TotalFeatures    int     `json:"total_features"`
	HealthyFeatures  int     `json:"healthy_features"`
	SkewedFeatures   int     `json:"skewed_features"`
	OverallMatchRate float64 `json:"overall_match_rate"`
	AlertCount       int     `json:"alert_count"`
}

// GetSummary returns a high-level parity summary.
func (c *Checker) GetSummary() *Summary {
	c.mu.RLock()
	defer c.mu.RUnlock()

	summary := &Summary{
		TotalFeatures: len(c.states),
		AlertCount:    len(c.alerts),
	}

	var totalPairs, totalMatches int64
	for _, state := range c.states {
		totalPairs += state.totalPairs
		totalMatches += state.matchCount
		mismatchRate := 1.0 - float64(state.matchCount)/max(float64(state.totalPairs), 1)
		if mismatchRate > c.config.AlertThreshold {
			summary.SkewedFeatures++
		} else {
			summary.HealthyFeatures++
		}
	}
	if totalPairs > 0 {
		summary.OverallMatchRate = float64(totalMatches) / float64(totalPairs)
	}
	return summary
}
