package ml

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"sync"
	"sync/atomic"
	"time"
)

// ValidationResult represents the outcome of a serving validation check.
type ValidationResult struct {
	// ModelID is the model being validated
	ModelID string `json:"model_id"`
	// ModelVersion is the version being validated
	ModelVersion string `json:"model_version"`
	// Valid indicates if all features passed validation
	Valid bool `json:"valid"`
	// Features contains per-feature validation results
	Features map[string]*FeatureValidation `json:"features"`
	// ValidationTime is how long validation took
	ValidationTime time.Duration `json:"validation_time_ns"`
	// Timestamp is when validation was performed
	Timestamp time.Time `json:"timestamp"`
}

// FeatureValidation represents validation result for a single feature.
type FeatureValidation struct {
	// Name is the feature name
	Name string `json:"name"`
	// Valid indicates if the feature passed validation
	Valid bool `json:"valid"`
	// ServingValue is the current serving value
	ServingValue interface{} `json:"serving_value,omitempty"`
	// Issues contains any validation issues found
	Issues []ValidationIssue `json:"issues,omitempty"`
}

// ValidationIssue represents a specific validation problem.
type ValidationIssue struct {
	// Type is the kind of issue detected
	Type ValidationIssueType `json:"type"`
	// Severity indicates how serious the issue is
	Severity IssueSeverity `json:"severity"`
	// Message describes the issue
	Message string `json:"message"`
	// ExpectedValue is the expected value/range from training
	ExpectedValue interface{} `json:"expected_value,omitempty"`
	// ActualValue is the actual serving value
	ActualValue interface{} `json:"actual_value,omitempty"`
	// Score is the quantified deviation (e.g., z-score, PSI)
	Score float64 `json:"score,omitempty"`
	// Threshold is the threshold that was exceeded
	Threshold float64 `json:"threshold,omitempty"`
}

// ValidationIssueType categorizes validation issues.
type ValidationIssueType string

// ValidationIssueType constants for serving validation.
const (
	IssueTypeOutOfRange        ValidationIssueType = "out_of_range"
	IssueTypeMeanShift         ValidationIssueType = "mean_shift"
	IssueTypeDistShift         ValidationIssueType = "distribution_shift"
	IssueTypeNewCategory       ValidationIssueType = "new_category"
	IssueTypeMissingFeature    ValidationIssueType = "missing_feature"
	IssueTypeTypeMismatch      ValidationIssueType = "type_mismatch"
	IssueTypeNullValue         ValidationIssueType = "null_value"
	IssueTypeCardinalityChange ValidationIssueType = "cardinality_change"
)

// IssueSeverity indicates the severity of a validation issue.
type IssueSeverity string

// IssueSeverity constants for validation severity.
const (
	SeverityInfo    IssueSeverity = "info"
	SeverityWarning IssueSeverity = "warning"
	SeverityError   IssueSeverity = "error"
)

// ValidatorConfig configures the serving validator.
type ValidatorConfig struct {
	// ZScoreThreshold for numeric outlier detection (default: 3.0)
	ZScoreThreshold float64
	// CategoryMismatchSeverity for new category detection (default: warning)
	CategoryMismatchSeverity IssueSeverity
	// NullValueSeverity for null value detection (default: warning)
	NullValueSeverity IssueSeverity
	// RangeCheckEnabled enables min/max range validation
	RangeCheckEnabled bool
	// DistributionCheckEnabled enables distribution shift detection
	DistributionCheckEnabled bool
	// SampleRate for async validation (0.0-1.0, default: 1.0)
	SampleRate float64
	// AsyncValidation enables non-blocking validation
	AsyncValidation bool
	// AlertCallback is called when validation fails
	AlertCallback func(*ValidationResult)
}

// DefaultValidatorConfig returns default configuration.
func DefaultValidatorConfig() ValidatorConfig {
	return ValidatorConfig{
		ZScoreThreshold:          3.0,
		CategoryMismatchSeverity: SeverityWarning,
		NullValueSeverity:        SeverityWarning,
		RangeCheckEnabled:        true,
		DistributionCheckEnabled: true,
		SampleRate:               1.0,
		AsyncValidation:          false,
	}
}

// ServingValidator validates feature values at serving time against training snapshots.
type ServingValidator struct {
	config        ValidatorConfig
	registry      *ModelRegistry
	snapshotStore *SnapshotStore

	// Metrics
	validationCount   int64
	validationErrors  int64
	featureErrors     int64
	validationLatency int64 // nanoseconds

	// Alert aggregation
	mu              sync.RWMutex
	recentAlerts    []*ValidationResult
	alertsByModel   map[string]*ValidationResult
	maxRecentAlerts int
	alertCooldown   time.Duration
	lastAlertTime   map[string]time.Time
}

// NewServingValidator creates a new serving validator.
func NewServingValidator(registry *ModelRegistry, snapshots *SnapshotStore, config ValidatorConfig) *ServingValidator {
	if config.ZScoreThreshold == 0 {
		config.ZScoreThreshold = 3.0
	}
	if config.SampleRate == 0 {
		config.SampleRate = 1.0
	}

	return &ServingValidator{
		config:          config,
		registry:        registry,
		snapshotStore:   snapshots,
		recentAlerts:    make([]*ValidationResult, 0),
		alertsByModel:   make(map[string]*ValidationResult),
		maxRecentAlerts: 100,
		alertCooldown:   time.Minute,
		lastAlertTime:   make(map[string]time.Time),
	}
}

// Validate checks if serving features are consistent with training.
func (v *ServingValidator) Validate(ctx context.Context, modelID string, features map[string]interface{}) (*ValidationResult, error) {
	start := time.Now()
	defer func() {
		atomic.AddInt64(&v.validationCount, 1)
		atomic.AddInt64(&v.validationLatency, int64(time.Since(start)))
	}()

	// Get active version
	activeVersion, err := v.registry.GetActiveVersion(modelID)
	if err != nil {
		return nil, fmt.Errorf("getting active version: %w", err)
	}

	// Get training snapshot
	snapshot, err := v.snapshotStore.GetSnapshotForModel(modelID, activeVersion.Version)
	if err != nil {
		// No snapshot available - skip validation
		return &ValidationResult{
			ModelID:        modelID,
			ModelVersion:   activeVersion.Version,
			Valid:          true,
			Features:       make(map[string]*FeatureValidation),
			ValidationTime: time.Since(start),
			Timestamp:      time.Now(),
		}, nil
	}

	result := &ValidationResult{
		ModelID:      modelID,
		ModelVersion: activeVersion.Version,
		Valid:        true,
		Features:     make(map[string]*FeatureValidation),
		Timestamp:    time.Now(),
	}

	// Check each required feature
	for _, featureName := range activeVersion.Features {
		featureVal, exists := features[featureName]
		fv := &FeatureValidation{
			Name:         featureName,
			Valid:        true,
			ServingValue: featureVal,
		}

		if !exists || featureVal == nil {
			fv.Valid = false
			fv.Issues = append(fv.Issues, ValidationIssue{
				Type:     IssueTypeMissingFeature,
				Severity: SeverityError,
				Message:  fmt.Sprintf("Required feature '%s' is missing", featureName),
			})
			result.Valid = false
			atomic.AddInt64(&v.featureErrors, 1)
		} else if trainingSnap, ok := snapshot.Features[featureName]; ok {
			// Validate against training snapshot
			issues := v.validateFeature(featureName, featureVal, trainingSnap)
			if len(issues) > 0 {
				fv.Issues = issues
				for _, issue := range issues {
					if issue.Severity == SeverityError {
						fv.Valid = false
						result.Valid = false
						atomic.AddInt64(&v.featureErrors, 1)
						break
					}
				}
			}
		}

		result.Features[featureName] = fv
	}

	result.ValidationTime = time.Since(start)

	// Track validation errors
	if !result.Valid {
		atomic.AddInt64(&v.validationErrors, 1)
		v.handleValidationFailure(result)
	}

	return result, nil
}

// ValidateAsync performs validation asynchronously without blocking.
func (v *ServingValidator) ValidateAsync(ctx context.Context, modelID string, features map[string]interface{}) {
	if !v.config.AsyncValidation {
		if _, err := v.Validate(ctx, modelID, features); err != nil {
			slog.Debug("sync validation failed in async path", "model", modelID, "error", err)
		}
		return
	}

	// Apply sampling
	if v.config.SampleRate < 1.0 {
		// Simple deterministic sampling based on time
		if float64(time.Now().UnixNano()%1000)/1000.0 > v.config.SampleRate {
			return
		}
	}

	go func() {
		// Use a detached context with timeout so this goroutine does not
		// outlive the parent request or leak when the caller's context is
		// cancelled.
		asyncCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if _, err := v.Validate(asyncCtx, modelID, features); err != nil {
			slog.Debug("async validation failed", "model", modelID, "error", err)
		}
	}()
}

func (v *ServingValidator) validateFeature(name string, value interface{}, training *FeatureSnapshot) []ValidationIssue {
	var issues []ValidationIssue

	if value == nil {
		issues = append(issues, ValidationIssue{
			Type:     IssueTypeNullValue,
			Severity: v.config.NullValueSeverity,
			Message:  fmt.Sprintf("Feature '%s' has null value", name),
		})
		return issues
	}

	switch training.Type {
	case DistTypeNumeric:
		issues = append(issues, v.validateNumeric(name, value, training)...)
	case DistTypeCategorical:
		issues = append(issues, v.validateCategorical(name, value, training)...)
	case DistTypeVector:
		issues = append(issues, v.validateVector(name, value, training)...)
	}

	return issues
}

func (v *ServingValidator) validateNumeric(name string, value interface{}, training *FeatureSnapshot) []ValidationIssue {
	var issues []ValidationIssue

	// Convert to float64
	var floatVal float64
	switch val := value.(type) {
	case float64:
		floatVal = val
	case float32:
		floatVal = float64(val)
	case int:
		floatVal = float64(val)
	case int64:
		floatVal = float64(val)
	case int32:
		floatVal = float64(val)
	default:
		issues = append(issues, ValidationIssue{
			Type:        IssueTypeTypeMismatch,
			Severity:    SeverityError,
			Message:     fmt.Sprintf("Feature '%s' expected numeric but got %T", name, value),
			ActualValue: value,
		})
		return issues
	}

	// Range check
	if v.config.RangeCheckEnabled {
		if floatVal < training.Min || floatVal > training.Max {
			issues = append(issues, ValidationIssue{
				Type:          IssueTypeOutOfRange,
				Severity:      SeverityWarning,
				Message:       fmt.Sprintf("Feature '%s' value %.4f outside training range [%.4f, %.4f]", name, floatVal, training.Min, training.Max),
				ExpectedValue: fmt.Sprintf("[%.4f, %.4f]", training.Min, training.Max),
				ActualValue:   floatVal,
			})
		}
	}

	// Z-score check for distribution shift
	if v.config.DistributionCheckEnabled && training.StdDev > 0 {
		zScore := math.Abs(floatVal-training.Mean) / training.StdDev
		if zScore > v.config.ZScoreThreshold {
			severity := SeverityWarning
			if zScore > v.config.ZScoreThreshold*2 {
				severity = SeverityError
			}
			issues = append(issues, ValidationIssue{
				Type:          IssueTypeMeanShift,
				Severity:      severity,
				Message:       fmt.Sprintf("Feature '%s' value %.4f has z-score %.2f (threshold: %.2f)", name, floatVal, zScore, v.config.ZScoreThreshold),
				ExpectedValue: fmt.Sprintf("mean=%.4f, std=%.4f", training.Mean, training.StdDev),
				ActualValue:   floatVal,
				Score:         zScore,
				Threshold:     v.config.ZScoreThreshold,
			})
		}
	}

	return issues
}

func (v *ServingValidator) validateCategorical(name string, value interface{}, training *FeatureSnapshot) []ValidationIssue {
	var issues []ValidationIssue

	strVal := fmt.Sprintf("%v", value)

	// Check if category was seen during training
	if training.Categories != nil {
		if _, exists := training.Categories[strVal]; !exists {
			issues = append(issues, ValidationIssue{
				Type:          IssueTypeNewCategory,
				Severity:      v.config.CategoryMismatchSeverity,
				Message:       fmt.Sprintf("Feature '%s' has new category '%s' not seen during training", name, strVal),
				ExpectedValue: fmt.Sprintf("%d known categories", len(training.Categories)),
				ActualValue:   strVal,
			})
		}
	}

	return issues
}

func (v *ServingValidator) validateVector(name string, value interface{}, training *FeatureSnapshot) []ValidationIssue {
	var issues []ValidationIssue

	// Check dimension
	var dimension int
	var norm float64

	switch vec := value.(type) {
	case []float32:
		dimension = len(vec)
		for _, x := range vec {
			norm += float64(x) * float64(x)
		}
	case []float64:
		dimension = len(vec)
		for _, x := range vec {
			norm += x * x
		}
	default:
		issues = append(issues, ValidationIssue{
			Type:        IssueTypeTypeMismatch,
			Severity:    SeverityError,
			Message:     fmt.Sprintf("Feature '%s' expected vector but got %T", name, value),
			ActualValue: value,
		})
		return issues
	}

	norm = math.Sqrt(norm)

	// Check dimension match
	if dimension != training.VectorDimension {
		issues = append(issues, ValidationIssue{
			Type:          IssueTypeTypeMismatch,
			Severity:      SeverityError,
			Message:       fmt.Sprintf("Feature '%s' dimension %d doesn't match training dimension %d", name, dimension, training.VectorDimension),
			ExpectedValue: training.VectorDimension,
			ActualValue:   dimension,
		})
	}

	// Check norm distribution
	if v.config.DistributionCheckEnabled && training.VectorNormStd > 0 {
		zScore := math.Abs(norm-training.VectorNormMean) / training.VectorNormStd
		if zScore > v.config.ZScoreThreshold {
			issues = append(issues, ValidationIssue{
				Type:          IssueTypeDistShift,
				Severity:      SeverityWarning,
				Message:       fmt.Sprintf("Feature '%s' vector norm %.4f has z-score %.2f", name, norm, zScore),
				ExpectedValue: fmt.Sprintf("mean=%.4f, std=%.4f", training.VectorNormMean, training.VectorNormStd),
				ActualValue:   norm,
				Score:         zScore,
				Threshold:     v.config.ZScoreThreshold,
			})
		}
	}

	return issues
}

func (v *ServingValidator) handleValidationFailure(result *ValidationResult) {
	v.mu.Lock()
	defer v.mu.Unlock()

	modelKey := fmt.Sprintf("%s:%s", result.ModelID, result.ModelVersion)

	// Check cooldown
	if lastTime, ok := v.lastAlertTime[modelKey]; ok {
		if time.Since(lastTime) < v.alertCooldown {
			return
		}
	}

	v.lastAlertTime[modelKey] = time.Now()
	v.alertsByModel[modelKey] = result

	// Add to recent alerts
	v.recentAlerts = append(v.recentAlerts, result)
	if len(v.recentAlerts) > v.maxRecentAlerts {
		v.recentAlerts = v.recentAlerts[1:]
	}

	// Call alert callback
	if v.config.AlertCallback != nil {
		go v.config.AlertCallback(result)
	}
}

// GetRecentAlerts returns recent validation failures.
func (v *ServingValidator) GetRecentAlerts(since time.Time) []*ValidationResult {
	v.mu.RLock()
	defer v.mu.RUnlock()

	var results []*ValidationResult
	for _, alert := range v.recentAlerts {
		if alert.Timestamp.After(since) {
			results = append(results, alert)
		}
	}
	return results
}

// GetAlertForModel returns the most recent alert for a model.
func (v *ServingValidator) GetAlertForModel(modelID, version string) *ValidationResult {
	v.mu.RLock()
	defer v.mu.RUnlock()

	key := fmt.Sprintf("%s:%s", modelID, version)
	return v.alertsByModel[key]
}

// Stats returns validator statistics.
func (v *ServingValidator) Stats() map[string]interface{} {
	totalCount := atomic.LoadInt64(&v.validationCount)
	totalErrors := atomic.LoadInt64(&v.validationErrors)
	featureErrors := atomic.LoadInt64(&v.featureErrors)
	totalLatency := atomic.LoadInt64(&v.validationLatency)

	var avgLatency float64
	if totalCount > 0 {
		avgLatency = float64(totalLatency) / float64(totalCount)
	}

	var errorRate float64
	if totalCount > 0 {
		errorRate = float64(totalErrors) / float64(totalCount)
	}

	return map[string]interface{}{
		"validation_count":   totalCount,
		"validation_errors":  totalErrors,
		"feature_errors":     featureErrors,
		"error_rate":         errorRate,
		"avg_latency_ns":     avgLatency,
		"recent_alert_count": len(v.recentAlerts),
	}
}

// Reset clears all metrics and alerts.
func (v *ServingValidator) Reset() {
	atomic.StoreInt64(&v.validationCount, 0)
	atomic.StoreInt64(&v.validationErrors, 0)
	atomic.StoreInt64(&v.featureErrors, 0)
	atomic.StoreInt64(&v.validationLatency, 0)

	v.mu.Lock()
	defer v.mu.Unlock()
	v.recentAlerts = make([]*ValidationResult, 0)
	v.alertsByModel = make(map[string]*ValidationResult)
	v.lastAlertTime = make(map[string]time.Time)
}
