package consistencyvalidator

import (
	"fmt"
	"math"
	"sort"
	"sync"
	"time"
)

// ValidatorConfig configures the consistency validator.
type ValidatorConfig struct {
	SampleSize          int           `json:"sample_size"`
	DivergenceThreshold float64       `json:"divergence_threshold"`
	CheckInterval       time.Duration `json:"check_interval"`
	MaxAlerts           int           `json:"max_alerts"`
	AlertCooldown       time.Duration `json:"alert_cooldown"`
}

// DefaultValidatorConfig returns sensible defaults.
func DefaultValidatorConfig() ValidatorConfig {
	return ValidatorConfig{
		SampleSize:          1000,
		DivergenceThreshold: 0.05,
		CheckInterval:       5 * time.Minute,
		MaxAlerts:           500,
		AlertCooldown:       10 * time.Minute,
	}
}

// SkewType indicates what kind of skew was detected.
type SkewType string

const (
	SkewNone         SkewType = "none"
	SkewDistribution SkewType = "distribution"
	SkewMean         SkewType = "mean"
	SkewMissing      SkewType = "missing_values"
	SkewRange        SkewType = "range"
)

// FeatureConsistency tracks online vs offline data for a single feature.
type FeatureConsistency struct {
	Name          string    `json:"name"`
	OnlineValues  []float64 `json:"-"`
	OfflineValues []float64 `json:"-"`
	LastChecked   time.Time `json:"last_checked"`
	LastAlerted   time.Time `json:"-"`
}

// ConsistencyReport represents the result of a consistency check.
type ConsistencyReport struct {
	Feature         string    `json:"feature"`
	Consistent      bool      `json:"consistent"`
	SkewType        SkewType  `json:"skew_type"`
	DivergenceScore float64   `json:"divergence_score"`
	OnlineMean      float64   `json:"online_mean"`
	OfflineMean     float64   `json:"offline_mean"`
	OnlineStdDev    float64   `json:"online_std_dev"`
	OfflineStdDev   float64   `json:"offline_std_dev"`
	OnlineMissing   float64   `json:"online_missing_rate"`
	OfflineMissing  float64   `json:"offline_missing_rate"`
	SampleSize      int       `json:"sample_size"`
	CheckedAt       time.Time `json:"checked_at"`
	Message         string    `json:"message,omitempty"`
}

// ConsistencyAlert represents a detected consistency violation.
type ConsistencyAlert struct {
	Feature         string    `json:"feature"`
	SkewType        SkewType  `json:"skew_type"`
	DivergenceScore float64   `json:"divergence_score"`
	Threshold       float64   `json:"threshold"`
	Timestamp       time.Time `json:"timestamp"`
	Message         string    `json:"message"`
}

// Validator continuously validates online vs offline feature consistency.
type Validator struct {
	mu       sync.RWMutex
	config   ValidatorConfig
	features map[string]*FeatureConsistency
	reports  []ConsistencyReport
	alerts   []ConsistencyAlert
}

// NewValidator creates a new consistency validator.
func NewValidator(config ValidatorConfig) *Validator {
	if config.SampleSize == 0 {
		config = DefaultValidatorConfig()
	}
	return &Validator{
		config:   config,
		features: make(map[string]*FeatureConsistency),
		reports:  make([]ConsistencyReport, 0),
		alerts:   make([]ConsistencyAlert, 0),
	}
}

// RegisterFeature registers a feature for consistency monitoring.
func (v *Validator) RegisterFeature(name string) {
	v.mu.Lock()
	defer v.mu.Unlock()

	v.features[name] = &FeatureConsistency{
		Name:          name,
		OnlineValues:  make([]float64, 0, v.config.SampleSize),
		OfflineValues: make([]float64, 0, v.config.SampleSize),
	}
}

// RecordOnline records an online feature value.
func (v *Validator) RecordOnline(name string, value float64) {
	v.mu.Lock()
	defer v.mu.Unlock()

	fc, exists := v.features[name]
	if !exists {
		// Auto-register
		fc = &FeatureConsistency{
			Name:          name,
			OnlineValues:  make([]float64, 0, v.config.SampleSize),
			OfflineValues: make([]float64, 0, v.config.SampleSize),
		}
		v.features[name] = fc
	}

	fc.OnlineValues = append(fc.OnlineValues, value)
	if len(fc.OnlineValues) > v.config.SampleSize {
		fc.OnlineValues = fc.OnlineValues[1:]
	}
}

// RecordOffline records an offline feature value.
func (v *Validator) RecordOffline(name string, value float64) {
	v.mu.Lock()
	defer v.mu.Unlock()

	fc, exists := v.features[name]
	if !exists {
		fc = &FeatureConsistency{
			Name:          name,
			OnlineValues:  make([]float64, 0, v.config.SampleSize),
			OfflineValues: make([]float64, 0, v.config.SampleSize),
		}
		v.features[name] = fc
	}

	fc.OfflineValues = append(fc.OfflineValues, value)
	if len(fc.OfflineValues) > v.config.SampleSize {
		fc.OfflineValues = fc.OfflineValues[1:]
	}
}

// CheckAll validates consistency for all registered features.
func (v *Validator) CheckAll() []ConsistencyReport {
	v.mu.Lock()
	defer v.mu.Unlock()

	now := time.Now()
	var reports []ConsistencyReport

	for name, fc := range v.features {
		report := v.checkFeature(name, fc, now)
		reports = append(reports, report)
		fc.LastChecked = now
	}

	v.reports = append(v.reports, reports...)
	if len(v.reports) > v.config.MaxAlerts*2 {
		v.reports = v.reports[len(v.reports)-v.config.MaxAlerts*2:]
	}

	return reports
}

// Check validates consistency for a specific feature.
func (v *Validator) Check(name string) (ConsistencyReport, error) {
	v.mu.Lock()
	defer v.mu.Unlock()

	fc, exists := v.features[name]
	if !exists {
		return ConsistencyReport{}, ErrFeatureNotRegistered
	}

	now := time.Now()
	report := v.checkFeature(name, fc, now)
	fc.LastChecked = now
	v.reports = append(v.reports, report)
	return report, nil
}

func (v *Validator) checkFeature(name string, fc *FeatureConsistency, now time.Time) ConsistencyReport {
	report := ConsistencyReport{
		Feature:    name,
		Consistent: true,
		SkewType:   SkewNone,
		CheckedAt:  now,
	}

	minSamples := 50
	if len(fc.OnlineValues) < minSamples || len(fc.OfflineValues) < minSamples {
		report.Message = "insufficient data"
		report.SampleSize = min(len(fc.OnlineValues), len(fc.OfflineValues))
		return report
	}

	report.SampleSize = min(len(fc.OnlineValues), len(fc.OfflineValues))

	// Compute statistics
	report.OnlineMean, report.OnlineStdDev = meanStdDev(fc.OnlineValues)
	report.OfflineMean, report.OfflineStdDev = meanStdDev(fc.OfflineValues)

	// KS test for distribution comparison
	ksScore := ksTest(fc.OnlineValues, fc.OfflineValues)
	report.DivergenceScore = ksScore

	if ksScore > v.config.DivergenceThreshold {
		report.Consistent = false
		report.SkewType = SkewDistribution
		report.Message = fmt.Sprintf("distribution divergence detected (KS=%.4f > %.4f)", ksScore, v.config.DivergenceThreshold)

		// Root cause: check mean shift
		if report.OfflineStdDev > 0 {
			zScore := math.Abs(report.OnlineMean-report.OfflineMean) / report.OfflineStdDev
			if zScore > 2.0 {
				report.SkewType = SkewMean
				report.Message = fmt.Sprintf("mean shift detected (z=%.2f, online=%.4f, offline=%.4f)", zScore, report.OnlineMean, report.OfflineMean)
			}
		}

		v.maybeAlert(fc, report, now)
	}

	return report
}

func (v *Validator) maybeAlert(fc *FeatureConsistency, report ConsistencyReport, now time.Time) {
	if now.Sub(fc.LastAlerted) < v.config.AlertCooldown {
		return
	}

	fc.LastAlerted = now
	alert := ConsistencyAlert{
		Feature:         report.Feature,
		SkewType:        report.SkewType,
		DivergenceScore: report.DivergenceScore,
		Threshold:       v.config.DivergenceThreshold,
		Timestamp:       now,
		Message:         report.Message,
	}
	v.alerts = append(v.alerts, alert)
	if len(v.alerts) > v.config.MaxAlerts {
		v.alerts = v.alerts[1:]
	}
}

// GetAlerts returns recent consistency alerts.
func (v *Validator) GetAlerts(since time.Time) []ConsistencyAlert {
	v.mu.RLock()
	defer v.mu.RUnlock()

	var result []ConsistencyAlert
	for _, a := range v.alerts {
		if a.Timestamp.After(since) {
			result = append(result, a)
		}
	}
	return result
}

// GetReports returns recent consistency reports.
func (v *Validator) GetReports(feature string, limit int) []ConsistencyReport {
	v.mu.RLock()
	defer v.mu.RUnlock()

	if limit <= 0 {
		limit = 50
	}

	var result []ConsistencyReport
	for _, r := range v.reports {
		if feature != "" && r.Feature != feature {
			continue
		}
		result = append(result, r)
	}

	if len(result) > limit {
		result = result[len(result)-limit:]
	}
	return result
}

// ListFeatures returns the names of all monitored features.
func (v *Validator) ListFeatures() []string {
	v.mu.RLock()
	defer v.mu.RUnlock()

	names := make([]string, 0, len(v.features))
	for name := range v.features {
		names = append(names, name)
	}
	return names
}

// Stats returns aggregate statistics.
func (v *Validator) Stats() ValidatorStats {
	v.mu.RLock()
	defer v.mu.RUnlock()

	var stats ValidatorStats
	stats.TotalFeatures = len(v.features)
	stats.TotalReports = len(v.reports)
	stats.TotalAlerts = len(v.alerts)

	for _, r := range v.reports {
		if !r.Consistent {
			stats.InconsistentFeatures++
		}
	}
	return stats
}

// ValidatorStats provides aggregate statistics.
type ValidatorStats struct {
	TotalFeatures        int `json:"total_features"`
	TotalReports         int `json:"total_reports"`
	TotalAlerts          int `json:"total_alerts"`
	InconsistentFeatures int `json:"inconsistent_features"`
}

func meanStdDev(values []float64) (float64, float64) {
	if len(values) == 0 {
		return 0, 0
	}
	var sum float64
	for _, v := range values {
		sum += v
	}
	mean := sum / float64(len(values))

	var variance float64
	for _, v := range values {
		diff := v - mean
		variance += diff * diff
	}
	variance /= float64(len(values))
	return mean, math.Sqrt(variance)
}

func ksTest(a, b []float64) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}

	sortedA := make([]float64, len(a))
	sortedB := make([]float64, len(b))
	copy(sortedA, a)
	copy(sortedB, b)
	sort.Float64s(sortedA)
	sort.Float64s(sortedB)

	n1, n2 := float64(len(sortedA)), float64(len(sortedB))
	var maxDiff float64
	var i, j int

	for i < len(sortedA) || j < len(sortedB) {
		var val float64
		if i >= len(sortedA) {
			val = sortedB[j]
		} else if j >= len(sortedB) {
			val = sortedA[i]
		} else if sortedA[i] < sortedB[j] {
			val = sortedA[i]
		} else {
			val = sortedB[j]
		}

		for i < len(sortedA) && sortedA[i] <= val {
			i++
		}
		for j < len(sortedB) && sortedB[j] <= val {
			j++
		}

		diff := math.Abs(float64(i)/n1 - float64(j)/n2)
		if diff > maxDiff {
			maxDiff = diff
		}
	}

	return maxDiff
}
