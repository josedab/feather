package dashboard

import (
	"fmt"
	"math"
	"strings"
	"sync"
	"time"
)

// ExplorerConfig configures the feature explorer.
type ExplorerConfig struct {
	MaxResults        int     `json:"max_results"`
	EnableCorrelation bool    `json:"enable_correlation"`
	SamplingRate      float64 `json:"sampling_rate"`
}

// DefaultExplorerConfig returns sensible defaults for the explorer.
func DefaultExplorerConfig() ExplorerConfig {
	return ExplorerConfig{
		MaxResults:        100,
		EnableCorrelation: true,
		SamplingRate:      1.0,
	}
}

// FeatureInsight holds computed statistics for a single feature.
type FeatureInsight struct {
	FeatureID         string           `json:"feature_id"`
	EntityCount       int64            `json:"entity_count"`
	ValueDistribution map[string]int64 `json:"value_distribution"`
	NullRate          float64          `json:"null_rate"`
	Cardinality       int              `json:"cardinality"`
	MinValue          float64          `json:"min_value"`
	MaxValue          float64          `json:"max_value"`
	MeanValue         float64          `json:"mean_value"`
	StdDev            float64          `json:"std_dev"`
	LastUpdated       time.Time        `json:"last_updated"`
}

// CorrelationResult stores the Pearson correlation between two features.
type CorrelationResult struct {
	FeatureA    string    `json:"feature_a"`
	FeatureB    string    `json:"feature_b"`
	Correlation float64   `json:"correlation"`
	SampleSize  int       `json:"sample_size"`
	ComputedAt  time.Time `json:"computed_at"`
}

// UsagePattern captures access patterns for a feature.
type UsagePattern struct {
	FeatureID      string    `json:"feature_id"`
	HourlyAccess   [24]int64 `json:"hourly_access"`
	PeakHour       int       `json:"peak_hour"`
	TotalAccesses  int64     `json:"total_accesses"`
	UniqueEntities int64     `json:"unique_entities"`
	AvgLatencyUs   float64   `json:"avg_latency_us"`
}

// CostBreakdown estimates the resource cost for a feature.
type CostBreakdown struct {
	FeatureID               string  `json:"feature_id"`
	StorageMB               float64 `json:"storage_mb"`
	ReadOps                 int64   `json:"read_ops"`
	WriteOps                int64   `json:"write_ops"`
	EstimatedMonthlyCostUSD float64 `json:"estimated_monthly_cost_usd"`
}

// ExplorerStats reports aggregate explorer state.
type ExplorerStats struct {
	TotalInsights        int       `json:"total_insights"`
	CorrelationsComputed int       `json:"correlations_computed"`
	LastFullScanAt       time.Time `json:"last_full_scan_at"`
}

// Explorer provides feature exploration and insight capabilities.
type Explorer struct {
	mu            sync.RWMutex
	config        ExplorerConfig
	insights      map[string]*FeatureInsight
	correlations  []*CorrelationResult
	usagePatterns map[string]*UsagePattern
	costs         map[string]*CostBreakdown
}

// NewExplorer creates an Explorer with the given configuration.
func NewExplorer(cfg ExplorerConfig) *Explorer {
	return &Explorer{
		config:        cfg,
		insights:      make(map[string]*FeatureInsight),
		correlations:  make([]*CorrelationResult, 0),
		usagePatterns: make(map[string]*UsagePattern),
		costs:         make(map[string]*CostBreakdown),
	}
}

// RecordInsight stores or updates an insight for a feature.
func (e *Explorer) RecordInsight(insight *FeatureInsight) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.insights[insight.FeatureID] = insight
}

// GetInsight returns the insight for the given feature.
func (e *Explorer) GetInsight(featureID string) (*FeatureInsight, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	ins, ok := e.insights[featureID]
	if !ok {
		return nil, fmt.Errorf("insight not found for feature %q", featureID)
	}
	return ins, nil
}

// ListInsights returns all recorded insights.
func (e *Explorer) ListInsights() []*FeatureInsight {
	e.mu.RLock()
	defer e.mu.RUnlock()
	result := make([]*FeatureInsight, 0, len(e.insights))
	for _, ins := range e.insights {
		result = append(result, ins)
	}
	return result
}

// SearchInsights returns insights whose FeatureID contains the query substring.
func (e *Explorer) SearchInsights(query string) []*FeatureInsight {
	e.mu.RLock()
	defer e.mu.RUnlock()
	var result []*FeatureInsight
	q := strings.ToLower(query)
	for _, ins := range e.insights {
		if strings.Contains(strings.ToLower(ins.FeatureID), q) {
			result = append(result, ins)
			if len(result) >= e.config.MaxResults {
				break
			}
		}
	}
	return result
}

// ComputeCorrelation calculates the Pearson correlation coefficient between two value slices.
func (e *Explorer) ComputeCorrelation(featureA, featureB string, valuesA, valuesB []float64) (*CorrelationResult, error) {
	if !e.config.EnableCorrelation {
		return nil, fmt.Errorf("correlation computation is disabled")
	}
	n := len(valuesA)
	if n != len(valuesB) {
		return nil, fmt.Errorf("value slices must have equal length (got %d and %d)", len(valuesA), len(valuesB))
	}
	if n < 2 {
		return nil, fmt.Errorf("need at least 2 data points, got %d", n)
	}

	// Compute means
	var sumA, sumB float64
	for i := 0; i < n; i++ {
		sumA += valuesA[i]
		sumB += valuesB[i]
	}
	meanA := sumA / float64(n)
	meanB := sumB / float64(n)

	// Pearson: r = Σ((xi-x̄)(yi-ȳ)) / √(Σ(xi-x̄)² × Σ(yi-ȳ)²)
	var num, denomA, denomB float64
	for i := 0; i < n; i++ {
		dA := valuesA[i] - meanA
		dB := valuesB[i] - meanB
		num += dA * dB
		denomA += dA * dA
		denomB += dB * dB
	}

	denom := math.Sqrt(denomA * denomB)
	if denom == 0 {
		return nil, fmt.Errorf("zero variance in one or both features")
	}

	r := num / denom

	result := &CorrelationResult{
		FeatureA:    featureA,
		FeatureB:    featureB,
		Correlation: r,
		SampleSize:  n,
		ComputedAt:  time.Now(),
	}

	e.mu.Lock()
	e.correlations = append(e.correlations, result)
	e.mu.Unlock()

	return result, nil
}

// ListCorrelations returns correlations with absolute value >= minCorrelation.
func (e *Explorer) ListCorrelations(minCorrelation float64) []*CorrelationResult {
	e.mu.RLock()
	defer e.mu.RUnlock()
	var result []*CorrelationResult
	for _, c := range e.correlations {
		if math.Abs(c.Correlation) >= minCorrelation {
			result = append(result, c)
		}
	}
	return result
}

// RecordUsagePattern stores a usage pattern for a feature.
func (e *Explorer) RecordUsagePattern(pattern *UsagePattern) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.usagePatterns[pattern.FeatureID] = pattern
}

// GetUsagePattern returns the usage pattern for the given feature.
func (e *Explorer) GetUsagePattern(featureID string) (*UsagePattern, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	p, ok := e.usagePatterns[featureID]
	if !ok {
		return nil, fmt.Errorf("usage pattern not found for feature %q", featureID)
	}
	return p, nil
}

// ListUsagePatterns returns all recorded usage patterns.
func (e *Explorer) ListUsagePatterns() []*UsagePattern {
	e.mu.RLock()
	defer e.mu.RUnlock()
	result := make([]*UsagePattern, 0, len(e.usagePatterns))
	for _, p := range e.usagePatterns {
		result = append(result, p)
	}
	return result
}

// RecordCost stores a cost breakdown for a feature.
func (e *Explorer) RecordCost(cost *CostBreakdown) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.costs[cost.FeatureID] = cost
}

// GetCostBreakdown returns the cost breakdown for the given feature.
func (e *Explorer) GetCostBreakdown(featureID string) (*CostBreakdown, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	c, ok := e.costs[featureID]
	if !ok {
		return nil, fmt.Errorf("cost breakdown not found for feature %q", featureID)
	}
	return c, nil
}

// GetTotalCosts returns all recorded cost breakdowns.
func (e *Explorer) GetTotalCosts() []*CostBreakdown {
	e.mu.RLock()
	defer e.mu.RUnlock()
	result := make([]*CostBreakdown, 0, len(e.costs))
	for _, c := range e.costs {
		result = append(result, c)
	}
	return result
}

// Stats returns aggregate statistics about the explorer state.
func (e *Explorer) Stats() *ExplorerStats {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return &ExplorerStats{
		TotalInsights:        len(e.insights),
		CorrelationsComputed: len(e.correlations),
	}
}
