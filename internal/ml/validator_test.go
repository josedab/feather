package ml

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupValidatorTestData creates a registry and snapshot store with test data.
func setupValidatorTestData(t *testing.T) (*ModelRegistry, *SnapshotStore) {
	t.Helper()

	registry := NewModelRegistry()
	snapshots := NewSnapshotStore()

	// Create a model with features
	model := &Model{ID: "test-model", Name: "Test Model"}
	err := registry.RegisterModel(model)
	require.NoError(t, err)

	// Register a version with features
	version := &ModelVersion{
		Version:  "v1.0",
		Features: []string{"feature_a", "feature_b", "category_feature"},
	}
	err = registry.RegisterVersion("test-model", version)
	require.NoError(t, err)

	// Activate the version
	err = registry.ActivateVersion("test-model", "v1.0")
	require.NoError(t, err)

	// Create a training snapshot using the builder
	builder := NewSnapshotBuilder("test-model", "v1.0", "Test snapshot")

	// Add numeric samples for feature_a (range 1-10, mean ~5.5)
	for i := 1; i <= 10; i++ {
		builder.AddSample("feature_a", float64(i))
	}

	// Add numeric samples for feature_b (range 100-500)
	builder.AddSamples("feature_b", []interface{}{100.0, 200.0, 300.0, 400.0, 500.0})

	// Add categorical samples
	builder.AddSample("category_feature", "A")
	builder.AddSample("category_feature", "B")
	builder.AddSample("category_feature", "A")
	builder.AddSample("category_feature", "C")

	snapshot := builder.Build()
	err = snapshots.CreateSnapshot(snapshot)
	require.NoError(t, err)

	return registry, snapshots
}

func TestNewServingValidator(t *testing.T) {
	registry := NewModelRegistry()
	snapshots := NewSnapshotStore()
	validator := NewServingValidator(registry, snapshots, DefaultValidatorConfig())
	assert.NotNil(t, validator)
}

func TestDefaultValidatorConfig(t *testing.T) {
	config := DefaultValidatorConfig()
	assert.Equal(t, 3.0, config.ZScoreThreshold)
	assert.True(t, config.RangeCheckEnabled)
	assert.True(t, config.DistributionCheckEnabled)
	assert.Equal(t, 1.0, config.SampleRate)
	assert.False(t, config.AsyncValidation)
}

func TestServingValidator_Validate_Valid(t *testing.T) {
	registry, snapshots := setupValidatorTestData(t)
	validator := NewServingValidator(registry, snapshots, DefaultValidatorConfig())
	ctx := context.Background()

	// Values within expected range
	features := map[string]interface{}{
		"feature_a":        5.5, // Within mean ± 3*std
		"feature_b":        300.0,
		"category_feature": "A",
	}

	result, err := validator.Validate(ctx, "test-model", features)
	require.NoError(t, err)
	assert.True(t, result.Valid)
	assert.Equal(t, "test-model", result.ModelID)
	assert.Equal(t, "v1.0", result.ModelVersion)
}

func TestServingValidator_Validate_OutOfRange(t *testing.T) {
	registry, snapshots := setupValidatorTestData(t)
	config := DefaultValidatorConfig()
	config.RangeCheckEnabled = true
	validator := NewServingValidator(registry, snapshots, config)
	ctx := context.Background()

	// Value well outside training range
	features := map[string]interface{}{
		"feature_a":        100.0, // Training range was 1-10
		"feature_b":        300.0,
		"category_feature": "A",
	}

	result, err := validator.Validate(ctx, "test-model", features)
	require.NoError(t, err)

	// Check that range violation was recorded
	fv := result.Features["feature_a"]
	require.NotNil(t, fv)

	found := false
	for _, issue := range fv.Issues {
		if issue.Type == IssueTypeOutOfRange {
			found = true
			break
		}
	}
	assert.True(t, found, "Expected out_of_range issue for feature_a")
}

func TestServingValidator_Validate_MeanShift(t *testing.T) {
	registry, snapshots := setupValidatorTestData(t)
	config := DefaultValidatorConfig()
	config.ZScoreThreshold = 2.0 // Lower threshold for easier testing
	config.DistributionCheckEnabled = true
	validator := NewServingValidator(registry, snapshots, config)
	ctx := context.Background()

	// Value that produces high z-score
	features := map[string]interface{}{
		"feature_a":        50.0, // Far from mean of ~5.5
		"feature_b":        300.0,
		"category_feature": "A",
	}

	result, err := validator.Validate(ctx, "test-model", features)
	require.NoError(t, err)

	// Check for mean shift issue
	fv := result.Features["feature_a"]
	require.NotNil(t, fv)

	found := false
	for _, issue := range fv.Issues {
		if issue.Type == IssueTypeMeanShift {
			found = true
			assert.True(t, issue.Score > config.ZScoreThreshold)
			break
		}
	}
	assert.True(t, found, "Expected mean_shift issue for feature_a")
}

func TestServingValidator_Validate_MissingFeature(t *testing.T) {
	registry, snapshots := setupValidatorTestData(t)
	validator := NewServingValidator(registry, snapshots, DefaultValidatorConfig())
	ctx := context.Background()

	// Missing category_feature
	features := map[string]interface{}{
		"feature_a": 5.0,
		"feature_b": 300.0,
		// "category_feature" is missing
	}

	result, err := validator.Validate(ctx, "test-model", features)
	require.NoError(t, err)
	assert.False(t, result.Valid)

	// Check for missing feature issue
	fv := result.Features["category_feature"]
	require.NotNil(t, fv)
	assert.False(t, fv.Valid)

	found := false
	for _, issue := range fv.Issues {
		if issue.Type == IssueTypeMissingFeature {
			found = true
			break
		}
	}
	assert.True(t, found, "Expected missing_feature issue")
}

func TestServingValidator_Validate_NullValue(t *testing.T) {
	registry, snapshots := setupValidatorTestData(t)
	config := DefaultValidatorConfig()
	config.NullValueSeverity = SeverityWarning
	validator := NewServingValidator(registry, snapshots, config)
	ctx := context.Background()

	// Null feature value
	features := map[string]interface{}{
		"feature_a":        nil, // Null value
		"feature_b":        300.0,
		"category_feature": "A",
	}

	result, err := validator.Validate(ctx, "test-model", features)
	require.NoError(t, err)

	// Should be invalid due to missing/null
	assert.False(t, result.Valid)
}

func TestServingValidator_Validate_NewCategory(t *testing.T) {
	registry, snapshots := setupValidatorTestData(t)
	config := DefaultValidatorConfig()
	config.CategoryMismatchSeverity = SeverityWarning
	validator := NewServingValidator(registry, snapshots, config)
	ctx := context.Background()

	// New category not seen during training
	features := map[string]interface{}{
		"feature_a":        5.0,
		"feature_b":        300.0,
		"category_feature": "X", // Not in training categories (A, B, C)
	}

	result, err := validator.Validate(ctx, "test-model", features)
	require.NoError(t, err)

	// Check for new category issue
	fv := result.Features["category_feature"]
	require.NotNil(t, fv)

	found := false
	for _, issue := range fv.Issues {
		if issue.Type == IssueTypeNewCategory {
			found = true
			assert.Equal(t, SeverityWarning, issue.Severity)
			break
		}
	}
	assert.True(t, found, "Expected new_category issue")
}

func TestServingValidator_Validate_TypeMismatch(t *testing.T) {
	registry, snapshots := setupValidatorTestData(t)
	validator := NewServingValidator(registry, snapshots, DefaultValidatorConfig())
	ctx := context.Background()

	// String value for numeric feature
	features := map[string]interface{}{
		"feature_a":        "not_a_number", // String instead of numeric
		"feature_b":        300.0,
		"category_feature": "A",
	}

	result, err := validator.Validate(ctx, "test-model", features)
	require.NoError(t, err)

	// Check for type mismatch issue
	fv := result.Features["feature_a"]
	require.NotNil(t, fv)

	found := false
	for _, issue := range fv.Issues {
		if issue.Type == IssueTypeTypeMismatch {
			found = true
			assert.Equal(t, SeverityError, issue.Severity)
			break
		}
	}
	assert.True(t, found, "Expected type_mismatch issue")
}

func TestServingValidator_Validate_NoActiveVersion(t *testing.T) {
	registry := NewModelRegistry()
	snapshots := NewSnapshotStore()

	// Register model but don't activate any version
	model := &Model{ID: "test-model", Name: "Test Model"}
	err := registry.RegisterModel(model)
	require.NoError(t, err)

	version := &ModelVersion{Version: "v1.0", Features: []string{"f1"}}
	err = registry.RegisterVersion("test-model", version)
	require.NoError(t, err)
	// Note: Not calling ActivateVersion

	validator := NewServingValidator(registry, snapshots, DefaultValidatorConfig())
	ctx := context.Background()

	features := map[string]interface{}{"f1": 5.0}
	_, err = validator.Validate(ctx, "test-model", features)
	assert.Error(t, err, "Should fail when no active version")
}

func TestServingValidator_Validate_ModelNotFound(t *testing.T) {
	registry := NewModelRegistry()
	snapshots := NewSnapshotStore()
	validator := NewServingValidator(registry, snapshots, DefaultValidatorConfig())
	ctx := context.Background()

	features := map[string]interface{}{"f1": 5.0}
	_, err := validator.Validate(ctx, "nonexistent", features)
	assert.Error(t, err)
}

func TestServingValidator_Validate_NoSnapshot(t *testing.T) {
	registry := NewModelRegistry()
	snapshots := NewSnapshotStore() // Empty - no snapshots

	// Create model and activate version
	model := &Model{ID: "test-model", Name: "Test Model"}
	registry.RegisterModel(model)
	version := &ModelVersion{Version: "v1.0", Features: []string{"f1"}}
	registry.RegisterVersion("test-model", version)
	registry.ActivateVersion("test-model", "v1.0")

	validator := NewServingValidator(registry, snapshots, DefaultValidatorConfig())
	ctx := context.Background()

	features := map[string]interface{}{"f1": 5.0}
	result, err := validator.Validate(ctx, "test-model", features)
	require.NoError(t, err)
	// Should return valid when no snapshot exists (skip validation)
	assert.True(t, result.Valid)
}

func TestServingValidator_Validate_DisabledChecks(t *testing.T) {
	registry, snapshots := setupValidatorTestData(t)
	config := DefaultValidatorConfig()
	config.RangeCheckEnabled = false
	config.DistributionCheckEnabled = false
	validator := NewServingValidator(registry, snapshots, config)
	ctx := context.Background()

	// Even extreme values should not produce range/distribution issues
	features := map[string]interface{}{
		"feature_a":        1000.0, // Way out of range
		"feature_b":        10000.0,
		"category_feature": "A",
	}

	result, err := validator.Validate(ctx, "test-model", features)
	require.NoError(t, err)

	// Check no range or mean shift issues
	for _, fv := range result.Features {
		for _, issue := range fv.Issues {
			assert.NotEqual(t, IssueTypeOutOfRange, issue.Type, "Should not have range issue when disabled")
			assert.NotEqual(t, IssueTypeMeanShift, issue.Type, "Should not have mean shift issue when disabled")
		}
	}
}

func TestServingValidator_ValidateAsync(t *testing.T) {
	registry, snapshots := setupValidatorTestData(t)
	config := DefaultValidatorConfig()
	config.AsyncValidation = true
	validator := NewServingValidator(registry, snapshots, config)
	ctx := context.Background()

	features := map[string]interface{}{
		"feature_a":        5.0,
		"feature_b":        300.0,
		"category_feature": "A",
	}

	// ValidateAsync doesn't return result, just ensure it doesn't panic
	validator.ValidateAsync(ctx, "test-model", features)

	// Give async validation time to complete
	time.Sleep(50 * time.Millisecond)

	// Check stats were updated
	stats := validator.Stats()
	assert.Greater(t, stats["validation_count"].(int64), int64(0))
}

func TestServingValidator_AlertCallback(t *testing.T) {
	registry, snapshots := setupValidatorTestData(t)
	config := DefaultValidatorConfig()

	alertReceived := make(chan *ValidationResult, 1)
	config.AlertCallback = func(result *ValidationResult) {
		alertReceived <- result
	}

	validator := NewServingValidator(registry, snapshots, config)
	ctx := context.Background()

	// Trigger validation failure
	features := map[string]interface{}{
		"feature_a": nil, // Missing required feature
		"feature_b": nil,
		// category_feature is also missing
	}

	_, err := validator.Validate(ctx, "test-model", features)
	require.NoError(t, err)

	// Wait for callback
	select {
	case alert := <-alertReceived:
		assert.False(t, alert.Valid)
	case <-time.After(100 * time.Millisecond):
		t.Error("Alert callback not received")
	}
}

func TestServingValidator_GetRecentAlerts(t *testing.T) {
	registry, snapshots := setupValidatorTestData(t)
	validator := NewServingValidator(registry, snapshots, DefaultValidatorConfig())
	ctx := context.Background()

	// Trigger validation failure
	features := map[string]interface{}{
		"feature_a": nil,
		"feature_b": nil,
	}

	_, err := validator.Validate(ctx, "test-model", features)
	require.NoError(t, err)

	// Get recent alerts
	alerts := validator.GetRecentAlerts(time.Now().Add(-time.Minute))
	assert.NotEmpty(t, alerts)
}

func TestServingValidator_GetAlertForModel(t *testing.T) {
	registry, snapshots := setupValidatorTestData(t)
	validator := NewServingValidator(registry, snapshots, DefaultValidatorConfig())
	ctx := context.Background()

	// Trigger validation failure
	features := map[string]interface{}{
		"feature_a": nil,
		"feature_b": nil,
	}

	_, err := validator.Validate(ctx, "test-model", features)
	require.NoError(t, err)

	// Get alert for model
	alert := validator.GetAlertForModel("test-model", "v1.0")
	assert.NotNil(t, alert)
	assert.Equal(t, "test-model", alert.ModelID)
}

func TestServingValidator_Stats(t *testing.T) {
	registry, snapshots := setupValidatorTestData(t)
	validator := NewServingValidator(registry, snapshots, DefaultValidatorConfig())
	ctx := context.Background()

	// Initial stats
	stats := validator.Stats()
	assert.Equal(t, int64(0), stats["validation_count"])

	// Perform validations
	features := map[string]interface{}{
		"feature_a":        5.0,
		"feature_b":        300.0,
		"category_feature": "A",
	}

	_, _ = validator.Validate(ctx, "test-model", features)
	_, _ = validator.Validate(ctx, "test-model", features)

	stats = validator.Stats()
	assert.Equal(t, int64(2), stats["validation_count"])
	assert.GreaterOrEqual(t, stats["avg_latency_ns"].(float64), float64(0))
}

func TestServingValidator_Reset(t *testing.T) {
	registry, snapshots := setupValidatorTestData(t)
	validator := NewServingValidator(registry, snapshots, DefaultValidatorConfig())
	ctx := context.Background()

	// Perform some validations
	features := map[string]interface{}{
		"feature_a": nil,
		"feature_b": nil,
	}
	_, _ = validator.Validate(ctx, "test-model", features)

	// Verify stats exist
	stats := validator.Stats()
	assert.Greater(t, stats["validation_count"].(int64), int64(0))

	// Reset
	validator.Reset()

	// Verify reset
	stats = validator.Stats()
	assert.Equal(t, int64(0), stats["validation_count"])
	assert.Equal(t, int64(0), stats["validation_errors"])
}

func TestValidationIssueTypes(t *testing.T) {
	// Test that all issue types are defined correctly
	assert.Equal(t, ValidationIssueType("out_of_range"), IssueTypeOutOfRange)
	assert.Equal(t, ValidationIssueType("mean_shift"), IssueTypeMeanShift)
	assert.Equal(t, ValidationIssueType("distribution_shift"), IssueTypeDistShift)
	assert.Equal(t, ValidationIssueType("new_category"), IssueTypeNewCategory)
	assert.Equal(t, ValidationIssueType("missing_feature"), IssueTypeMissingFeature)
	assert.Equal(t, ValidationIssueType("type_mismatch"), IssueTypeTypeMismatch)
	assert.Equal(t, ValidationIssueType("null_value"), IssueTypeNullValue)
	assert.Equal(t, ValidationIssueType("cardinality_change"), IssueTypeCardinalityChange)
}

func TestIssueSeverityTypes(t *testing.T) {
	// Test that all severity types are defined correctly
	assert.Equal(t, IssueSeverity("info"), SeverityInfo)
	assert.Equal(t, IssueSeverity("warning"), SeverityWarning)
	assert.Equal(t, IssueSeverity("error"), SeverityError)
}
