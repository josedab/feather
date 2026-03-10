package anomalydetect

import (
	"fmt"
	"math"
	"sync"
	"time"
)

// MultivariateConfig configures multivariate anomaly detection.
type MultivariateConfig struct {
	CorrelationThreshold float64 `json:"correlation_threshold" yaml:"correlation_threshold"`
	WindowSize           int     `json:"window_size" yaml:"window_size"`
	MinSamples           int     `json:"min_samples" yaml:"min_samples"`
}

// DefaultMultivariateConfig returns sensible defaults.
func DefaultMultivariateConfig() MultivariateConfig {
	return MultivariateConfig{
		CorrelationThreshold: 0.7,
		WindowSize:           500,
		MinSamples:           50,
	}
}

// MultivariateResult reports a joint anomaly across multiple features.
type MultivariateResult struct {
	Features    []string  `json:"features"`
	IsAnomaly   bool      `json:"is_anomaly"`
	Score       float64   `json:"score"`
	Correlation float64   `json:"correlation"`
	Timestamp   time.Time `json:"timestamp"`
	Message     string    `json:"message"`
}

// MultivariateDetector detects anomalies by analyzing correlations between features.
type MultivariateDetector struct {
	mu     sync.RWMutex
	config MultivariateConfig
	// featureValues stores the last N values per feature
	featureValues map[string][]float64
}

// NewMultivariateDetector creates a new multivariate detector.
func NewMultivariateDetector(config MultivariateConfig) *MultivariateDetector {
	if config.WindowSize == 0 {
		config = DefaultMultivariateConfig()
	}
	return &MultivariateDetector{
		config:        config,
		featureValues: make(map[string][]float64),
	}
}

// Record adds a value for a feature into the sliding window.
func (d *MultivariateDetector) Record(feature string, value float64) {
	d.mu.Lock()
	defer d.mu.Unlock()

	vals := d.featureValues[feature]
	if len(vals) >= d.config.WindowSize {
		vals = vals[1:]
	}
	d.featureValues[feature] = append(vals, value)
}

// CheckPair checks whether two features exhibit a joint anomaly based on
// a sudden decorrelation or correlation spike compared to their historical pattern.
func (d *MultivariateDetector) CheckPair(featureA, featureB string) MultivariateResult {
	d.mu.RLock()
	defer d.mu.RUnlock()

	result := MultivariateResult{
		Features:  []string{featureA, featureB},
		Timestamp: time.Now(),
	}

	valsA, okA := d.featureValues[featureA]
	valsB, okB := d.featureValues[featureB]

	if !okA || !okB {
		result.Message = "insufficient data: one or both features not recorded"
		return result
	}

	n := len(valsA)
	if len(valsB) < n {
		n = len(valsB)
	}
	if n < d.config.MinSamples {
		result.Message = fmt.Sprintf("insufficient samples: %d < %d", n, d.config.MinSamples)
		return result
	}

	// Compute Pearson correlation over the full window
	fullCorr := pearsonCorrelation(valsA[len(valsA)-n:], valsB[len(valsB)-n:])

	// Compute correlation over the recent quarter
	recentN := n / 4
	if recentN < 2 {
		recentN = 2
	}
	recentCorr := pearsonCorrelation(
		valsA[len(valsA)-recentN:],
		valsB[len(valsB)-recentN:],
	)

	result.Correlation = recentCorr

	// Detect anomaly: significant deviation from historical correlation
	corrDiff := math.Abs(recentCorr - fullCorr)
	if corrDiff > d.config.CorrelationThreshold {
		result.IsAnomaly = true
		result.Score = corrDiff
		result.Message = fmt.Sprintf("correlation shift: historical=%.3f recent=%.3f diff=%.3f",
			fullCorr, recentCorr, corrDiff)
	} else {
		result.Message = fmt.Sprintf("normal: correlation=%.3f (diff=%.3f)", recentCorr, corrDiff)
	}

	return result
}

// CheckAll checks all recorded feature pairs for joint anomalies.
func (d *MultivariateDetector) CheckAll() []MultivariateResult {
	d.mu.RLock()
	features := make([]string, 0, len(d.featureValues))
	for f := range d.featureValues {
		features = append(features, f)
	}
	d.mu.RUnlock()

	var results []MultivariateResult
	for i := 0; i < len(features); i++ {
		for j := i + 1; j < len(features); j++ {
			result := d.CheckPair(features[i], features[j])
			if result.IsAnomaly {
				results = append(results, result)
			}
		}
	}
	return results
}

func pearsonCorrelation(x, y []float64) float64 {
	n := len(x)
	if n != len(y) || n < 2 {
		return 0
	}

	var sumX, sumY, sumXY, sumX2, sumY2 float64
	for i := 0; i < n; i++ {
		sumX += x[i]
		sumY += y[i]
		sumXY += x[i] * y[i]
		sumX2 += x[i] * x[i]
		sumY2 += y[i] * y[i]
	}

	nf := float64(n)
	numerator := nf*sumXY - sumX*sumY
	denominator := math.Sqrt((nf*sumX2 - sumX*sumX) * (nf*sumY2 - sumY*sumY))

	if denominator < 1e-10 {
		return 0
	}
	return numerator / denominator
}
