package qualitygates

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"
)

// ChangeType describes the kind of schema change.
type ChangeType string

const (
	ChangeAdd    ChangeType = "add"
	ChangeRemove ChangeType = "remove"
	ChangeModify ChangeType = "modify"
)

// CheckStatus represents the outcome of a single check.
type CheckStatus string

const (
	CheckPassed  CheckStatus = "passed"
	CheckFailed  CheckStatus = "failed"
	CheckWarning CheckStatus = "warning"
)

// Config holds settings for the quality gates validator.
type Config struct {
	MinQualityScore      float64
	MaxNullRate          float64
	MaxDistributionDrift float64
	EnableMergeBlocking  bool
	ReportFormat         string
}

// DefaultConfig returns sensible defaults for quality gate validation.
func DefaultConfig() Config {
	return Config{
		MinQualityScore:      0.7,
		MaxNullRate:          0.1,
		MaxDistributionDrift: 0.3,
		EnableMergeBlocking:  true,
		ReportFormat:         "markdown",
	}
}

// FeatureDefinition describes a single feature within a schema.
type FeatureDefinition struct {
	Name        string                 `json:"name"`
	DataType    string                 `json:"data_type"`
	Nullable    bool                   `json:"nullable"`
	Constraints map[string]interface{} `json:"constraints"`
}

// SchemaDefinition describes a feature group schema.
type SchemaDefinition struct {
	Name       string              `json:"name"`
	EntityType string              `json:"entity_type"`
	Features   []FeatureDefinition `json:"features"`
	Version    string              `json:"version"`
}

// ValidationReport holds the result of a schema validation.
type ValidationReport struct {
	Valid    bool
	Errors   []string
	Warnings []string
}

// DistributionStats holds basic distribution metrics for a sample.
type DistributionStats struct {
	Mean   float64
	StdDev float64
	Min    float64
	Max    float64
	P50    float64
	P95    float64
	P99    float64
}

// QualityReport holds the result of a data quality assertion.
type QualityReport struct {
	Feature           string
	NullRate          float64
	Completeness      float64
	Uniqueness        float64
	DistributionStats DistributionStats
	QualityScore      float64
	Passed            bool
}

// SchemaChange describes a single change to a feature schema.
type SchemaChange struct {
	Feature    string
	ChangeType ChangeType
	OldSpec    *FeatureDefinition
	NewSpec    *FeatureDefinition
}

// CheckResult holds the outcome of a single validation check.
type CheckResult struct {
	Name    string
	Status  CheckStatus
	Message string
}

// PRValidationRequest is the input for PR validation.
type PRValidationRequest struct {
	SchemaChanges []SchemaChange
	DataSamples   map[string][]float64
}

// PRValidationResult holds the full PR validation outcome.
type PRValidationResult struct {
	Passed  bool
	Score   float64
	Checks  []CheckResult
	Comment string
}

// MergeDecision captures the final merge/block decision.
type MergeDecision struct {
	Allowed        bool
	Reason         string
	BlockingChecks []string
}

// ValidatorStats holds aggregate validation statistics.
type ValidatorStats struct {
	TotalValidations   int64
	SchemaValidations  int64
	QualityAssertions  int64
	PRValidations      int64
	TotalPassed        int64
	TotalFailed        int64
	AverageScore       float64
}

// Validator provides schema validation and CI/CD quality gate integration.
type Validator struct {
	mu     sync.RWMutex
	config Config
	stats  ValidatorStats
	scores []float64
}

// NewValidator creates a new Validator with the given configuration.
func NewValidator(config Config) *Validator {
	return &Validator{
		config: config,
	}
}

// ValidateSchema checks a schema definition for correctness.
func (v *Validator) ValidateSchema(schema SchemaDefinition) (*ValidationReport, error) {
	v.mu.Lock()
	defer v.mu.Unlock()

	v.stats.TotalValidations++
	v.stats.SchemaValidations++

	report := &ValidationReport{Valid: true}

	if schema.Name == "" {
		report.Errors = append(report.Errors, "schema name is required")
	}
	if schema.EntityType == "" {
		report.Errors = append(report.Errors, "entity type is required")
	}
	if len(schema.Features) == 0 {
		report.Errors = append(report.Errors, "at least one feature is required")
	}
	if schema.Version == "" {
		report.Warnings = append(report.Warnings, "schema version is not set")
	}

	seen := make(map[string]bool)
	for _, f := range schema.Features {
		if f.Name == "" {
			report.Errors = append(report.Errors, "feature name is required")
			continue
		}
		if seen[f.Name] {
			report.Errors = append(report.Errors, fmt.Sprintf("duplicate feature name: %s", f.Name))
		}
		seen[f.Name] = true

		if f.DataType == "" {
			report.Errors = append(report.Errors, fmt.Sprintf("feature %s: data type is required", f.Name))
		}
	}

	if len(report.Errors) > 0 {
		report.Valid = false
		v.stats.TotalFailed++
	} else {
		v.stats.TotalPassed++
	}

	return report, nil
}

// AssertQuality computes data quality metrics for a feature's sample values.
func (v *Validator) AssertQuality(feature string, samples []float64) (*QualityReport, error) {
	v.mu.Lock()
	defer v.mu.Unlock()

	v.stats.TotalValidations++
	v.stats.QualityAssertions++

	if len(samples) == 0 {
		return nil, fmt.Errorf("asserting quality for %s: no samples provided", feature)
	}

	// Count NaN values as nulls
	nullCount := 0
	var valid []float64
	for _, s := range samples {
		if math.IsNaN(s) {
			nullCount++
		} else {
			valid = append(valid, s)
		}
	}

	total := float64(len(samples))
	nullRate := float64(nullCount) / total
	completeness := 1.0 - nullRate

	// Uniqueness
	unique := make(map[float64]bool)
	for _, val := range valid {
		unique[val] = true
	}
	uniqueness := 0.0
	if len(valid) > 0 {
		uniqueness = float64(len(unique)) / float64(len(valid))
	}

	dist := computeDistribution(valid)

	// Quality score: weighted combination
	score := 0.4*completeness + 0.3*uniqueness + 0.3*clamp(1.0-nullRate, 0, 1)
	passed := score >= v.config.MinQualityScore && nullRate <= v.config.MaxNullRate

	v.scores = append(v.scores, score)
	if passed {
		v.stats.TotalPassed++
	} else {
		v.stats.TotalFailed++
	}

	return &QualityReport{
		Feature:           feature,
		NullRate:          nullRate,
		Completeness:      completeness,
		Uniqueness:        uniqueness,
		DistributionStats: dist,
		QualityScore:      score,
		Passed:            passed,
	}, nil
}

// ValidatePR runs all quality checks for a pull request.
func (v *Validator) ValidatePR(req PRValidationRequest) (*PRValidationResult, error) {
	v.mu.Lock()
	defer v.mu.Unlock()

	v.stats.TotalValidations++
	v.stats.PRValidations++

	result := &PRValidationResult{Passed: true}

	// Validate schema changes
	for _, sc := range req.SchemaChanges {
		check := v.validateSchemaChange(sc)
		result.Checks = append(result.Checks, check)
		if check.Status == CheckFailed {
			result.Passed = false
		}
	}

	// Validate data samples
	for feature, samples := range req.DataSamples {
		check := v.validateDataSample(feature, samples)
		result.Checks = append(result.Checks, check)
		if check.Status == CheckFailed {
			result.Passed = false
		}
	}

	// Compute aggregate score
	if len(result.Checks) > 0 {
		passed := 0
		for _, c := range result.Checks {
			if c.Status == CheckPassed {
				passed++
			}
		}
		result.Score = float64(passed) / float64(len(result.Checks))
	}

	v.scores = append(v.scores, result.Score)
	if result.Passed {
		v.stats.TotalPassed++
	} else {
		v.stats.TotalFailed++
	}

	result.Comment = v.generateMarkdown(result)
	return result, nil
}

// EvaluateRules determines whether a PR may be merged based on validation results.
func (v *Validator) EvaluateRules(result *PRValidationResult) *MergeDecision {
	v.mu.RLock()
	defer v.mu.RUnlock()

	decision := &MergeDecision{Allowed: true}

	if !v.config.EnableMergeBlocking {
		decision.Reason = "merge blocking is disabled"
		return decision
	}

	for _, c := range result.Checks {
		if c.Status == CheckFailed {
			decision.BlockingChecks = append(decision.BlockingChecks, c.Name)
		}
	}

	if result.Score < v.config.MinQualityScore {
		decision.Allowed = false
		decision.Reason = fmt.Sprintf("quality score %.2f is below threshold %.2f", result.Score, v.config.MinQualityScore)
		return decision
	}

	if len(decision.BlockingChecks) > 0 {
		decision.Allowed = false
		decision.Reason = fmt.Sprintf("%d blocking check(s) failed", len(decision.BlockingChecks))
		return decision
	}

	decision.Reason = "all quality gates passed"
	return decision
}

// GenerateReport produces a markdown CI report from a PR validation result.
func (v *Validator) GenerateReport(result *PRValidationResult) string {
	v.mu.RLock()
	defer v.mu.RUnlock()

	return v.generateMarkdown(result)
}

// Stats returns aggregate validation statistics.
func (v *Validator) Stats() ValidatorStats {
	v.mu.RLock()
	defer v.mu.RUnlock()

	stats := v.stats
	if len(v.scores) > 0 {
		sum := 0.0
		for _, s := range v.scores {
			sum += s
		}
		stats.AverageScore = sum / float64(len(v.scores))
	}
	return stats
}

// validateSchemaChange checks a single schema change for issues.
func (v *Validator) validateSchemaChange(sc SchemaChange) CheckResult {
	name := fmt.Sprintf("schema:%s:%s", sc.ChangeType, sc.Feature)

	switch sc.ChangeType {
	case ChangeRemove:
		return CheckResult{
			Name:    name,
			Status:  CheckFailed,
			Message: fmt.Sprintf("removing feature %s is a breaking change", sc.Feature),
		}
	case ChangeModify:
		if sc.OldSpec != nil && sc.NewSpec != nil && sc.OldSpec.DataType != sc.NewSpec.DataType {
			return CheckResult{
				Name:    name,
				Status:  CheckFailed,
				Message: fmt.Sprintf("changing data type of %s from %s to %s is a breaking change", sc.Feature, sc.OldSpec.DataType, sc.NewSpec.DataType),
			}
		}
		if sc.OldSpec != nil && sc.NewSpec != nil && sc.OldSpec.Nullable && !sc.NewSpec.Nullable {
			return CheckResult{
				Name:    name,
				Status:  CheckWarning,
				Message: fmt.Sprintf("making %s non-nullable may break existing data", sc.Feature),
			}
		}
		return CheckResult{
			Name:    name,
			Status:  CheckPassed,
			Message: fmt.Sprintf("schema modification for %s is compatible", sc.Feature),
		}
	case ChangeAdd:
		return CheckResult{
			Name:    name,
			Status:  CheckPassed,
			Message: fmt.Sprintf("adding feature %s is a safe change", sc.Feature),
		}
	default:
		return CheckResult{
			Name:    name,
			Status:  CheckWarning,
			Message: fmt.Sprintf("unknown change type for %s", sc.Feature),
		}
	}
}

// validateDataSample checks a feature's sample data against quality thresholds.
func (v *Validator) validateDataSample(feature string, samples []float64) CheckResult {
	name := fmt.Sprintf("quality:%s", feature)

	if len(samples) == 0 {
		return CheckResult{
			Name:    name,
			Status:  CheckWarning,
			Message: fmt.Sprintf("no samples provided for %s", feature),
		}
	}

	nullCount := 0
	for _, s := range samples {
		if math.IsNaN(s) {
			nullCount++
		}
	}
	nullRate := float64(nullCount) / float64(len(samples))

	if nullRate > v.config.MaxNullRate {
		return CheckResult{
			Name:    name,
			Status:  CheckFailed,
			Message: fmt.Sprintf("%s null rate %.2f exceeds threshold %.2f", feature, nullRate, v.config.MaxNullRate),
		}
	}

	return CheckResult{
		Name:    name,
		Status:  CheckPassed,
		Message: fmt.Sprintf("%s data quality within acceptable range", feature),
	}
}

// generateMarkdown builds a markdown-formatted CI report.
func (v *Validator) generateMarkdown(result *PRValidationResult) string {
	var b strings.Builder

	status := "✅ Passed"
	if !result.Passed {
		status = "❌ Failed"
	}

	b.WriteString(fmt.Sprintf("## Feature Quality Gate Report\n\n"))
	b.WriteString(fmt.Sprintf("**Status:** %s\n", status))
	b.WriteString(fmt.Sprintf("**Score:** %.2f / 1.00\n", result.Score))
	b.WriteString(fmt.Sprintf("**Time:** %s\n\n", time.Now().UTC().Format(time.RFC3339)))

	if len(result.Checks) > 0 {
		b.WriteString("### Checks\n\n")
		b.WriteString("| Check | Status | Details |\n")
		b.WriteString("|-------|--------|---------|\n")
		for _, c := range result.Checks {
			icon := "✅"
			switch c.Status {
			case CheckFailed:
				icon = "❌"
			case CheckWarning:
				icon = "⚠️"
			}
			b.WriteString(fmt.Sprintf("| %s | %s %s | %s |\n", c.Name, icon, c.Status, c.Message))
		}
	}

	return b.String()
}

// computeDistribution calculates distribution statistics for a slice of values.
func computeDistribution(values []float64) DistributionStats {
	if len(values) == 0 {
		return DistributionStats{}
	}

	sorted := make([]float64, len(values))
	copy(sorted, values)
	sort.Float64s(sorted)

	sum := 0.0
	for _, v := range sorted {
		sum += v
	}
	mean := sum / float64(len(sorted))

	sumSq := 0.0
	for _, v := range sorted {
		d := v - mean
		sumSq += d * d
	}
	stddev := math.Sqrt(sumSq / float64(len(sorted)))

	return DistributionStats{
		Mean:   mean,
		StdDev: stddev,
		Min:    sorted[0],
		Max:    sorted[len(sorted)-1],
		P50:    percentile(sorted, 50),
		P95:    percentile(sorted, 95),
		P99:    percentile(sorted, 99),
	}
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

// clamp restricts v to the range [lo, hi].
func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
