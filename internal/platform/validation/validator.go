package validation

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sync"
	"time"
)

// Errors for the validation package.
var (
	ErrRuleNotFound  = errors.New("validation rule not found")
	ErrRuleExists    = errors.New("validation rule already exists")
	ErrEmptyRuleName = errors.New("rule name must not be empty")
	ErrNoData        = errors.New("no data provided for validation")
	ErrDataMismatch  = errors.New("online and offline data length mismatch")
	ErrNoResults     = errors.New("no results to report")
)

// CompareMethod defines how feature values are compared.
type CompareMethod string

const (
	// CompareExact requires exact equality between values.
	CompareExact CompareMethod = "exact"
	// CompareNumeric allows numeric differences within a tolerance.
	CompareNumeric CompareMethod = "numeric"
	// CompareStatistical uses statistical hypothesis tests.
	CompareStatistical CompareMethod = "statistical"
	// CompareDistribution compares full value distributions.
	CompareDistribution CompareMethod = "distribution"
)

// ValidatorConfig configures the validation engine.
type ValidatorConfig struct {
	SampleSize       int           // Default sample size for validation
	Tolerance        float64       // Numerical tolerance for comparison
	StatisticalAlpha float64       // Significance level for stat tests
	MaxResults       int           // Maximum stored results
	CheckInterval    time.Duration // Interval between automated checks
	EnableAlerts     bool          // Whether to generate alerts
}

// DefaultValidatorConfig returns sensible defaults for the validator.
func DefaultValidatorConfig() ValidatorConfig {
	return ValidatorConfig{
		SampleSize:       1000,
		Tolerance:        0.0001,
		StatisticalAlpha: 0.05,
		MaxResults:       10000,
		CheckInterval:    5 * time.Minute,
		EnableAlerts:     true,
	}
}

// ValidationRule defines how a feature should be validated.
type ValidationRule struct {
	Name          string            `json:"name"`
	Feature       string            `json:"feature"`
	OnlineSource  string            `json:"online_source"`
	OfflineSource string            `json:"offline_source"`
	CompareMethod CompareMethod     `json:"compare_method"`
	Tolerance     float64           `json:"tolerance"`
	SampleRate    float64           `json:"sample_rate"`
	Enabled       bool              `json:"enabled"`
	Tags          map[string]string `json:"tags,omitempty"`
	CreatedAt     time.Time         `json:"created_at"`
}

// ValidationResult contains the outcome of a single validation run.
type ValidationResult struct {
	RuleName     string              `json:"rule_name"`
	Feature      string              `json:"feature"`
	IsConsistent bool                `json:"is_consistent"`
	Metrics      *ConsistencyMetrics `json:"metrics"`
	SampleSize   int                 `json:"sample_size"`
	Errors       []string            `json:"errors,omitempty"`
	CheckedAt    time.Time           `json:"checked_at"`
	Duration     time.Duration       `json:"duration"`
}

// ConsistencyMetrics holds computed metrics for a validation run.
type ConsistencyMetrics struct {
	ExactMatchRate   float64 `json:"exact_match_rate"`
	MeanAbsError     float64 `json:"mean_abs_error"`
	MaxAbsError      float64 `json:"max_abs_error"`
	RootMeanSqError  float64 `json:"root_mean_sq_error"`
	CorrelationCoeff float64 `json:"correlation_coeff"`
	KSStatistic      float64 `json:"ks_statistic,omitempty"`
	KSPValue         float64 `json:"ks_p_value,omitempty"`
	MismatchCount    int     `json:"mismatch_count"`
	TotalCompared    int     `json:"total_compared"`
}

// Validator is the main validation engine for online-offline consistency.
type Validator struct {
	config  ValidatorConfig
	rules   map[string]*ValidationRule
	results []*ValidationResult
	reports []*ValidationReport
	mu      sync.RWMutex
	ctx     context.Context
	cancel  context.CancelFunc
}

// NewValidator creates a new Validator with the given configuration.
func NewValidator(config ValidatorConfig) *Validator {
	ctx, cancel := context.WithCancel(context.Background())
	return &Validator{
		config:  config,
		rules:   make(map[string]*ValidationRule),
		results: make([]*ValidationResult, 0),
		reports: make([]*ValidationReport, 0),
		ctx:     ctx,
		cancel:  cancel,
	}
}

// AddRule registers a new validation rule.
func (v *Validator) AddRule(_ context.Context, rule *ValidationRule) error {
	if rule.Name == "" {
		return fmt.Errorf("adding rule: %w", ErrEmptyRuleName)
	}

	v.mu.Lock()
	defer v.mu.Unlock()

	if _, exists := v.rules[rule.Name]; exists {
		return fmt.Errorf("adding rule %q: %w", rule.Name, ErrRuleExists)
	}

	rule.CreatedAt = time.Now()
	v.rules[rule.Name] = rule
	return nil
}

// RemoveRule deletes a validation rule by name.
func (v *Validator) RemoveRule(_ context.Context, name string) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	if _, exists := v.rules[name]; !exists {
		return fmt.Errorf("removing rule %q: %w", name, ErrRuleNotFound)
	}

	delete(v.rules, name)
	return nil
}

// ListRules returns all registered validation rules.
func (v *Validator) ListRules(_ context.Context) []*ValidationRule {
	v.mu.RLock()
	defer v.mu.RUnlock()

	rules := make([]*ValidationRule, 0, len(v.rules))
	for _, r := range v.rules {
		rules = append(rules, r)
	}
	return rules
}

// GetRule returns a single validation rule by name.
func (v *Validator) GetRule(_ context.Context, name string) (*ValidationRule, error) {
	v.mu.RLock()
	defer v.mu.RUnlock()

	rule, exists := v.rules[name]
	if !exists {
		return nil, fmt.Errorf("getting rule %q: %w", name, ErrRuleNotFound)
	}
	return rule, nil
}

// Validate runs a single validation using the named rule against the provided
// online and offline value slices.
func (v *Validator) Validate(_ context.Context, ruleName string, onlineValues, offlineValues []float64) (*ValidationResult, error) {
	v.mu.RLock()
	rule, exists := v.rules[ruleName]
	v.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("validating rule %q: %w", ruleName, ErrRuleNotFound)
	}

	if len(onlineValues) == 0 || len(offlineValues) == 0 {
		return nil, fmt.Errorf("validating rule %q: %w", ruleName, ErrNoData)
	}

	if len(onlineValues) != len(offlineValues) {
		return nil, fmt.Errorf("validating rule %q: %w", ruleName, ErrDataMismatch)
	}

	start := time.Now()
	n := len(onlineValues)

	tolerance := rule.Tolerance
	if tolerance == 0 {
		tolerance = v.config.Tolerance
	}

	metrics := computeMetrics(onlineValues, offlineValues, tolerance)

	isConsistent := determineConsistency(rule.CompareMethod, metrics, tolerance, v.config.StatisticalAlpha)

	result := &ValidationResult{
		RuleName:     ruleName,
		Feature:      rule.Feature,
		IsConsistent: isConsistent,
		Metrics:      metrics,
		SampleSize:   n,
		CheckedAt:    time.Now(),
		Duration:     time.Since(start),
	}

	v.mu.Lock()
	v.results = append(v.results, result)
	if len(v.results) > v.config.MaxResults {
		v.results = v.results[len(v.results)-v.config.MaxResults:]
	}
	v.mu.Unlock()

	return result, nil
}

// ValidateBatch runs validations for multiple rules in a single call.
func (v *Validator) ValidateBatch(ctx context.Context, rules []string, onlineData, offlineData map[string][]float64) ([]*ValidationResult, error) {
	results := make([]*ValidationResult, 0, len(rules))

	for _, ruleName := range rules {
		online, ok := onlineData[ruleName]
		if !ok {
			continue
		}
		offline, ok := offlineData[ruleName]
		if !ok {
			continue
		}

		result, err := v.Validate(ctx, ruleName, online, offline)
		if err != nil {
			return results, fmt.Errorf("batch validating rule %q: %w", ruleName, err)
		}
		results = append(results, result)
	}

	return results, nil
}

// GenerateReport creates a ValidationReport from the given results.
func (v *Validator) GenerateReport(ctx context.Context, results []*ValidationResult) (*ValidationReport, error) {
	report, err := generateReport(ctx, results)
	if err != nil {
		return nil, err
	}

	v.mu.Lock()
	v.reports = append(v.reports, report)
	v.mu.Unlock()

	return report, nil
}

// GetResults returns the most recent validation results up to the given limit.
func (v *Validator) GetResults(_ context.Context, limit int) []*ValidationResult {
	v.mu.RLock()
	defer v.mu.RUnlock()

	if limit <= 0 || limit > len(v.results) {
		limit = len(v.results)
	}

	out := make([]*ValidationResult, limit)
	copy(out, v.results[len(v.results)-limit:])
	return out
}

// Stats returns aggregate statistics about the validator.
func (v *Validator) Stats(_ context.Context) *ValidatorStats {
	v.mu.RLock()
	defer v.mu.RUnlock()

	stats := &ValidatorStats{
		TotalRules:   len(v.rules),
		TotalResults: len(v.results),
		TotalReports: len(v.reports),
	}

	for _, r := range v.rules {
		if r.Enabled {
			stats.EnabledRules++
		}
	}

	for _, r := range v.results {
		if r.IsConsistent {
			stats.ConsistentCount++
		} else {
			stats.FailedCount++
		}
	}

	return stats
}

// Close shuts down the validator and releases resources.
func (v *Validator) Close() error {
	v.cancel()
	return nil
}

// computeMetrics calculates all consistency metrics between two value slices.
func computeMetrics(online, offline []float64, tolerance float64) *ConsistencyMetrics {
	n := len(online)
	m := &ConsistencyMetrics{
		TotalCompared: n,
	}

	var exactMatches int
	var maxAbs float64

	for i := 0; i < n; i++ {
		diff := math.Abs(online[i] - offline[i])
		if diff <= tolerance {
			exactMatches++
		} else {
			m.MismatchCount++
		}
		if diff > maxAbs {
			maxAbs = diff
		}
	}

	m.ExactMatchRate = float64(exactMatches) / float64(n)
	m.MaxAbsError = maxAbs
	m.MeanAbsError = MeanAbsoluteError(online, offline)
	m.RootMeanSqError = RootMeanSquaredError(online, offline)
	m.CorrelationCoeff = PearsonCorrelation(online, offline)
	m.KSStatistic, m.KSPValue = KolmogorovSmirnov(online, offline)

	return m
}

// determineConsistency decides whether the comparison passes based on the
// configured compare method and computed metrics.
func determineConsistency(method CompareMethod, metrics *ConsistencyMetrics, tolerance, alpha float64) bool {
	switch method {
	case CompareExact:
		return metrics.ExactMatchRate == 1.0
	case CompareNumeric:
		return metrics.MaxAbsError <= tolerance
	case CompareStatistical:
		return metrics.KSPValue >= alpha
	case CompareDistribution:
		return metrics.KSPValue >= alpha && metrics.CorrelationCoeff >= 0.9
	default:
		return metrics.ExactMatchRate == 1.0
	}
}
