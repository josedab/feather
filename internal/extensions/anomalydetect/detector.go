package anomalydetect

import (
	"fmt"
	"math"
	"sort"
	"sync"
	"time"
)

// AnomalyType describes the kind of anomaly detected.
type AnomalyType string

const (
	AnomalyZScore AnomalyType = "zscore"
	AnomalyIQR    AnomalyType = "iqr"
	AnomalyRange  AnomalyType = "range"
	AnomalyNone   AnomalyType = "none"
)

// AnomalyResult holds the outcome of an anomaly check.
type AnomalyResult struct {
	Feature     string
	Value       float64
	IsAnomaly   bool
	Type        AnomalyType
	Score       float64
	Threshold   float64
	Quarantined bool
	Timestamp   time.Time
	Message     string
}

// DetectorConfig configures the anomaly detector.
type DetectorConfig struct {
	ZScoreThreshold   float64
	IQRMultiplier     float64
	WindowSize        int
	QuarantineEnabled bool
	LearningPeriod    int
	MaxFeatures       int
}

// DefaultDetectorConfig returns sensible defaults.
func DefaultDetectorConfig() DetectorConfig {
	return DetectorConfig{
		ZScoreThreshold:   3.0,
		IQRMultiplier:     1.5,
		WindowSize:        1000,
		QuarantineEnabled: false,
		LearningPeriod:    100,
		MaxFeatures:       100000,
	}
}

// featureStats tracks rolling statistics for a single feature.
type featureStats struct {
	values       []float64
	mean         float64
	stddev       float64
	q1           float64
	q3           float64
	count        int64
	anomalyCount int64
	quarantined  bool
}

// DetectorStats holds aggregate detector statistics.
type DetectorStats struct {
	TotalFeatures       int64
	MonitoredFeatures   int64
	TotalChecks         int64
	TotalAnomalies      int64
	QuarantinedFeatures int64
	AnomalyRate         float64
}

// Detector monitors feature values and detects anomalies in real time.
type Detector struct {
	mu       sync.RWMutex
	config   DetectorConfig
	features map[string]*featureStats
	alerts   []AnomalyResult
}

// NewDetector creates a new Detector with the given configuration.
func NewDetector(config DetectorConfig) *Detector {
	return &Detector{
		config:   config,
		features: make(map[string]*featureStats),
	}
}

// RegisterFeature registers a feature for monitoring.
func (d *Detector) RegisterFeature(name string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if _, exists := d.features[name]; !exists {
		d.features[name] = &featureStats{}
	}
}

// Check evaluates a new value for the given feature and returns the result.
func (d *Detector) Check(name string, value float64) AnomalyResult {
	d.mu.Lock()
	defer d.mu.Unlock()

	fs, ok := d.features[name]
	if !ok {
		fs = &featureStats{}
		d.features[name] = fs
	}

	fs.count++

	// Add value to window
	if len(fs.values) >= d.config.WindowSize {
		fs.values = fs.values[1:]
	}
	fs.values = append(fs.values, value)

	// Update rolling statistics
	d.updateStats(fs)

	result := AnomalyResult{
		Feature:   name,
		Value:     value,
		Type:      AnomalyNone,
		Timestamp: time.Now(),
	}

	// Skip anomaly detection during learning period
	if fs.count < int64(d.config.LearningPeriod) {
		result.Message = "learning period"
		return result
	}

	// Z-score check
	if fs.stddev > 0 {
		zscore := math.Abs(value-fs.mean) / fs.stddev
		if zscore > d.config.ZScoreThreshold {
			result.IsAnomaly = true
			result.Type = AnomalyZScore
			result.Score = zscore
			result.Threshold = d.config.ZScoreThreshold
			result.Message = fmt.Sprintf("z-score %.2f exceeds threshold %.2f", zscore, d.config.ZScoreThreshold)
		}
	}

	// IQR check (if not already flagged)
	if !result.IsAnomaly {
		iqr := fs.q3 - fs.q1
		lower := fs.q1 - d.config.IQRMultiplier*iqr
		upper := fs.q3 + d.config.IQRMultiplier*iqr
		if value < lower || value > upper {
			result.IsAnomaly = true
			result.Type = AnomalyIQR
			result.Score = math.Max(math.Abs(value-lower), math.Abs(value-upper))
			result.Threshold = d.config.IQRMultiplier
			result.Message = fmt.Sprintf("value %.2f outside IQR range [%.2f, %.2f]", value, lower, upper)
		}
	}

	if result.IsAnomaly {
		fs.anomalyCount++
		d.alerts = append(d.alerts, result)

		if d.config.QuarantineEnabled {
			fs.quarantined = true
			result.Quarantined = true
		}
	}

	return result
}

// updateStats recalculates mean, stddev, q1, q3 from the current window.
func (d *Detector) updateStats(fs *featureStats) {
	n := len(fs.values)
	if n == 0 {
		return
	}

	sum := 0.0
	for _, v := range fs.values {
		sum += v
	}
	fs.mean = sum / float64(n)

	sumSq := 0.0
	for _, v := range fs.values {
		d := v - fs.mean
		sumSq += d * d
	}
	fs.stddev = math.Sqrt(sumSq / float64(n))

	sorted := make([]float64, n)
	copy(sorted, fs.values)
	sort.Float64s(sorted)

	fs.q1 = percentile(sorted, 25)
	fs.q3 = percentile(sorted, 75)
}

// percentile computes the p-th percentile of a sorted slice.
func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	idx := p / 100.0 * float64(len(sorted)-1)
	lower := int(idx)
	upper := lower + 1
	if upper >= len(sorted) {
		return sorted[len(sorted)-1]
	}
	frac := idx - float64(lower)
	return sorted[lower]*(1-frac) + sorted[upper]*frac
}

// GetAlerts returns anomaly alerts since the given time, up to limit.
func (d *Detector) GetAlerts(since time.Time, limit int) []AnomalyResult {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var results []AnomalyResult
	for i := len(d.alerts) - 1; i >= 0 && len(results) < limit; i-- {
		if !d.alerts[i].Timestamp.Before(since) {
			results = append(results, d.alerts[i])
		}
	}
	return results
}

// IsQuarantined reports whether the named feature is currently quarantined.
func (d *Detector) IsQuarantined(name string) bool {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if fs, ok := d.features[name]; ok {
		return fs.quarantined
	}
	return false
}

// ClearQuarantine removes quarantine from the named feature.
func (d *Detector) ClearQuarantine(name string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	fs, ok := d.features[name]
	if !ok {
		return ErrFeatureNotMonitored
	}
	fs.quarantined = false
	return nil
}

// GetFeatureStats returns statistics for the named feature.
func (d *Detector) GetFeatureStats(name string) (map[string]interface{}, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	fs, ok := d.features[name]
	if !ok {
		return nil, ErrFeatureNotMonitored
	}

	anomalyRate := 0.0
	if fs.count > 0 {
		anomalyRate = float64(fs.anomalyCount) / float64(fs.count)
	}

	return map[string]interface{}{
		"mean":         fs.mean,
		"stddev":       fs.stddev,
		"q1":           fs.q1,
		"q3":           fs.q3,
		"anomaly_rate": anomalyRate,
	}, nil
}

// Stats returns aggregate detector statistics.
func (d *Detector) Stats() DetectorStats {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var totalChecks, totalAnomalies, quarantined int64
	for _, fs := range d.features {
		totalChecks += fs.count
		totalAnomalies += fs.anomalyCount
		if fs.quarantined {
			quarantined++
		}
	}

	anomalyRate := 0.0
	if totalChecks > 0 {
		anomalyRate = float64(totalAnomalies) / float64(totalChecks)
	}

	return DetectorStats{
		TotalFeatures:       int64(len(d.features)),
		MonitoredFeatures:   int64(len(d.features)),
		TotalChecks:         totalChecks,
		TotalAnomalies:      totalAnomalies,
		QuarantinedFeatures: quarantined,
		AnomalyRate:         anomalyRate,
	}
}
