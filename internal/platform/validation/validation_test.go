package validation

import (
	"context"
	"errors"
	"math"
	"testing"
)

func TestAddRemoveRule(t *testing.T) {
	v := NewValidator(DefaultValidatorConfig())
	defer v.Close()
	ctx := context.Background()

	rule := &ValidationRule{
		Name:          "test_rule",
		Feature:       "feature_a",
		CompareMethod: CompareNumeric,
		Tolerance:     0.01,
		SampleRate:    1.0,
		Enabled:       true,
	}

	// Add rule
	if err := v.AddRule(ctx, rule); err != nil {
		t.Fatalf("AddRule failed: %v", err)
	}

	// Duplicate should fail
	if err := v.AddRule(ctx, rule); !errors.Is(err, ErrRuleExists) {
		t.Errorf("expected ErrRuleExists, got %v", err)
	}

	// Get rule
	got, err := v.GetRule(ctx, "test_rule")
	if err != nil {
		t.Fatalf("GetRule failed: %v", err)
	}
	if got.Feature != "feature_a" {
		t.Errorf("Feature = %q, want %q", got.Feature, "feature_a")
	}

	// List rules
	rules := v.ListRules(ctx)
	if len(rules) != 1 {
		t.Errorf("ListRules returned %d rules, want 1", len(rules))
	}

	// Remove rule
	if err := v.RemoveRule(ctx, "test_rule"); err != nil {
		t.Fatalf("RemoveRule failed: %v", err)
	}

	// Remove non-existent should fail
	if err := v.RemoveRule(ctx, "test_rule"); !errors.Is(err, ErrRuleNotFound) {
		t.Errorf("expected ErrRuleNotFound, got %v", err)
	}

	// Get non-existent should fail
	if _, err := v.GetRule(ctx, "test_rule"); !errors.Is(err, ErrRuleNotFound) {
		t.Errorf("expected ErrRuleNotFound, got %v", err)
	}

	// Empty name should fail
	if err := v.AddRule(ctx, &ValidationRule{}); !errors.Is(err, ErrEmptyRuleName) {
		t.Errorf("expected ErrEmptyRuleName, got %v", err)
	}
}

func TestValidateExact(t *testing.T) {
	v := NewValidator(DefaultValidatorConfig())
	defer v.Close()
	ctx := context.Background()

	rule := &ValidationRule{
		Name:          "exact_rule",
		Feature:       "clicks",
		CompareMethod: CompareExact,
		Tolerance:     0.0001,
		Enabled:       true,
	}
	if err := v.AddRule(ctx, rule); err != nil {
		t.Fatalf("AddRule: %v", err)
	}

	// Identical values should be consistent
	online := []float64{1.0, 2.0, 3.0, 4.0, 5.0}
	offline := []float64{1.0, 2.0, 3.0, 4.0, 5.0}

	result, err := v.Validate(ctx, "exact_rule", online, offline)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if !result.IsConsistent {
		t.Error("expected consistent result for identical values")
	}
	if result.Metrics.ExactMatchRate != 1.0 {
		t.Errorf("ExactMatchRate = %f, want 1.0", result.Metrics.ExactMatchRate)
	}

	// Different values should be inconsistent
	offlineDiff := []float64{1.0, 2.0, 3.5, 4.0, 5.0}
	result, err = v.Validate(ctx, "exact_rule", online, offlineDiff)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if result.IsConsistent {
		t.Error("expected inconsistent result for different values")
	}

	// Validate with non-existent rule should fail
	if _, err := v.Validate(ctx, "no_such_rule", online, offline); !errors.Is(err, ErrRuleNotFound) {
		t.Errorf("expected ErrRuleNotFound, got %v", err)
	}

	// Empty data should fail
	if _, err := v.Validate(ctx, "exact_rule", nil, offline); !errors.Is(err, ErrNoData) {
		t.Errorf("expected ErrNoData, got %v", err)
	}

	// Mismatched lengths should fail
	if _, err := v.Validate(ctx, "exact_rule", online, []float64{1.0}); !errors.Is(err, ErrDataMismatch) {
		t.Errorf("expected ErrDataMismatch, got %v", err)
	}
}

func TestValidateNumeric(t *testing.T) {
	v := NewValidator(DefaultValidatorConfig())
	defer v.Close()
	ctx := context.Background()

	rule := &ValidationRule{
		Name:          "numeric_rule",
		Feature:       "score",
		CompareMethod: CompareNumeric,
		Tolerance:     0.1,
		Enabled:       true,
	}
	if err := v.AddRule(ctx, rule); err != nil {
		t.Fatalf("AddRule: %v", err)
	}

	// Within tolerance
	online := []float64{1.0, 2.0, 3.0}
	offline := []float64{1.05, 2.08, 3.02}

	result, err := v.Validate(ctx, "numeric_rule", online, offline)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if !result.IsConsistent {
		t.Errorf("expected consistent, MaxAbsError=%f, tolerance=0.1", result.Metrics.MaxAbsError)
	}

	// Beyond tolerance
	offlineFar := []float64{1.0, 2.5, 3.0}
	result, err = v.Validate(ctx, "numeric_rule", online, offlineFar)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if result.IsConsistent {
		t.Error("expected inconsistent for values beyond tolerance")
	}
}

func TestValidateStatistical(t *testing.T) {
	v := NewValidator(DefaultValidatorConfig())
	defer v.Close()
	ctx := context.Background()

	rule := &ValidationRule{
		Name:          "stat_rule",
		Feature:       "latency",
		CompareMethod: CompareStatistical,
		Enabled:       true,
	}
	if err := v.AddRule(ctx, rule); err != nil {
		t.Fatalf("AddRule: %v", err)
	}

	// Same distribution should pass
	online := make([]float64, 100)
	offline := make([]float64, 100)
	for i := 0; i < 100; i++ {
		online[i] = float64(i)
		offline[i] = float64(i) + 0.001
	}

	result, err := v.Validate(ctx, "stat_rule", online, offline)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if !result.IsConsistent {
		t.Errorf("expected consistent for similar distributions, KS p-value=%f", result.Metrics.KSPValue)
	}

	// Very different distributions should fail
	for i := 0; i < 100; i++ {
		offline[i] = float64(i) * 100
	}
	result, err = v.Validate(ctx, "stat_rule", online, offline)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if result.IsConsistent {
		t.Errorf("expected inconsistent for very different distributions, KS p-value=%f", result.Metrics.KSPValue)
	}
}

func TestKolmogorovSmirnov(t *testing.T) {
	// Identical samples: statistic should be 0
	s := []float64{1, 2, 3, 4, 5}
	stat, pval := KolmogorovSmirnov(s, s)
	if stat != 0 {
		t.Errorf("KS statistic for identical samples = %f, want 0", stat)
	}
	if pval < 0.99 {
		t.Errorf("KS p-value for identical samples = %f, want ~1.0", pval)
	}

	// Completely different samples
	a := []float64{1, 2, 3, 4, 5}
	b := []float64{100, 200, 300, 400, 500}
	stat, pval = KolmogorovSmirnov(a, b)
	if stat != 1.0 {
		t.Errorf("KS statistic for disjoint samples = %f, want 1.0", stat)
	}
	if pval > 0.05 {
		t.Errorf("KS p-value for disjoint samples = %f, want < 0.05", pval)
	}

	// Empty samples
	stat, pval = KolmogorovSmirnov(nil, s)
	if stat != 0 || pval != 1.0 {
		t.Errorf("KS for empty sample: stat=%f pval=%f, want 0/1.0", stat, pval)
	}
}

func TestPearsonCorrelation(t *testing.T) {
	// Perfect positive correlation
	x := []float64{1, 2, 3, 4, 5}
	y := []float64{2, 4, 6, 8, 10}
	r := PearsonCorrelation(x, y)
	if math.Abs(r-1.0) > 1e-10 {
		t.Errorf("PearsonCorrelation(perfect positive) = %f, want 1.0", r)
	}

	// Perfect negative correlation
	y2 := []float64{10, 8, 6, 4, 2}
	r = PearsonCorrelation(x, y2)
	if math.Abs(r-(-1.0)) > 1e-10 {
		t.Errorf("PearsonCorrelation(perfect negative) = %f, want -1.0", r)
	}

	// Zero correlation (constant y)
	y3 := []float64{5, 5, 5, 5, 5}
	r = PearsonCorrelation(x, y3)
	if r != 0 {
		t.Errorf("PearsonCorrelation(constant y) = %f, want 0", r)
	}

	// Too few elements
	r = PearsonCorrelation([]float64{1}, []float64{2})
	if r != 0 {
		t.Errorf("PearsonCorrelation(single element) = %f, want 0", r)
	}
}

func TestMeanAbsoluteError(t *testing.T) {
	predicted := []float64{1.0, 2.0, 3.0}
	actual := []float64{1.5, 2.5, 3.5}

	mae := MeanAbsoluteError(predicted, actual)
	if math.Abs(mae-0.5) > 1e-10 {
		t.Errorf("MAE = %f, want 0.5", mae)
	}

	// Identical values
	mae = MeanAbsoluteError(predicted, predicted)
	if mae != 0 {
		t.Errorf("MAE(identical) = %f, want 0", mae)
	}

	// Empty slices
	mae = MeanAbsoluteError(nil, actual)
	if mae != 0 {
		t.Errorf("MAE(empty) = %f, want 0", mae)
	}

	// RMSE check
	rmse := RootMeanSquaredError(predicted, actual)
	if math.Abs(rmse-0.5) > 1e-10 {
		t.Errorf("RMSE = %f, want 0.5", rmse)
	}

	rmse = RootMeanSquaredError(nil, actual)
	if rmse != 0 {
		t.Errorf("RMSE(empty) = %f, want 0", rmse)
	}
}

func TestGenerateReport(t *testing.T) {
	v := NewValidator(DefaultValidatorConfig())
	defer v.Close()
	ctx := context.Background()

	results := []*ValidationResult{
		{
			RuleName:     "rule_a",
			Feature:      "feature_a",
			IsConsistent: true,
			Metrics: &ConsistencyMetrics{
				ExactMatchRate: 1.0,
				TotalCompared:  100,
			},
			SampleSize: 100,
		},
		{
			RuleName:     "rule_b",
			Feature:      "feature_b",
			IsConsistent: false,
			Metrics: &ConsistencyMetrics{
				ExactMatchRate: 0.3,
				MaxAbsError:    2.5,
				KSPValue:       0.001,
				TotalCompared:  100,
			},
			SampleSize: 100,
		},
	}

	report, err := v.GenerateReport(ctx, results)
	if err != nil {
		t.Fatalf("GenerateReport: %v", err)
	}

	if report.ID == "" {
		t.Error("report ID should not be empty")
	}
	if report.Summary.TotalFeatures != 2 {
		t.Errorf("TotalFeatures = %d, want 2", report.Summary.TotalFeatures)
	}
	if report.Summary.ConsistentCount != 1 {
		t.Errorf("ConsistentCount = %d, want 1", report.Summary.ConsistentCount)
	}
	if report.Summary.InconsistentCount != 1 {
		t.Errorf("InconsistentCount = %d, want 1", report.Summary.InconsistentCount)
	}
	if report.Summary.OverallScore != 0.5 {
		t.Errorf("OverallScore = %f, want 0.5", report.Summary.OverallScore)
	}
	if len(report.FeatureResults) != 2 {
		t.Errorf("FeatureResults length = %d, want 2", len(report.FeatureResults))
	}
	if len(report.Recommendations) == 0 {
		t.Error("expected at least one recommendation")
	}

	// Nil results should error
	if _, err := v.GenerateReport(ctx, nil); !errors.Is(err, ErrNoResults) {
		t.Errorf("expected ErrNoResults, got %v", err)
	}
}

func TestValidateBatch(t *testing.T) {
	v := NewValidator(DefaultValidatorConfig())
	defer v.Close()
	ctx := context.Background()

	ruleA := &ValidationRule{
		Name:          "batch_rule_a",
		Feature:       "feature_a",
		CompareMethod: CompareExact,
		Tolerance:     0.0001,
		Enabled:       true,
	}
	ruleB := &ValidationRule{
		Name:          "batch_rule_b",
		Feature:       "feature_b",
		CompareMethod: CompareNumeric,
		Tolerance:     0.5,
		Enabled:       true,
	}
	if err := v.AddRule(ctx, ruleA); err != nil {
		t.Fatalf("AddRule: %v", err)
	}
	if err := v.AddRule(ctx, ruleB); err != nil {
		t.Fatalf("AddRule: %v", err)
	}

	onlineData := map[string][]float64{
		"batch_rule_a": {1.0, 2.0, 3.0},
		"batch_rule_b": {10.0, 20.0, 30.0},
	}
	offlineData := map[string][]float64{
		"batch_rule_a": {1.0, 2.0, 3.0},
		"batch_rule_b": {10.2, 20.3, 30.1},
	}

	results, err := v.ValidateBatch(ctx, []string{"batch_rule_a", "batch_rule_b"}, onlineData, offlineData)
	if err != nil {
		t.Fatalf("ValidateBatch: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	// Rule A (exact) should be consistent (identical values)
	if !results[0].IsConsistent {
		t.Error("batch_rule_a: expected consistent")
	}
	// Rule B (numeric, tolerance 0.5) should be consistent
	if !results[1].IsConsistent {
		t.Error("batch_rule_b: expected consistent within tolerance")
	}

	// Verify stats
	stats := v.Stats(ctx)
	if stats.TotalRules != 2 {
		t.Errorf("TotalRules = %d, want 2", stats.TotalRules)
	}
	if stats.TotalResults < 2 {
		t.Errorf("TotalResults = %d, want >= 2", stats.TotalResults)
	}

	// Verify GetResults
	got := v.GetResults(ctx, 2)
	if len(got) != 2 {
		t.Errorf("GetResults(2) = %d, want 2", len(got))
	}
}

func TestPopulationStabilityIndex(t *testing.T) {
	// Identical distributions should have PSI near 0
	a := []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	psi := PopulationStabilityIndex(a, a, 5)
	if psi > 0.01 {
		t.Errorf("PSI(identical) = %f, want ~0", psi)
	}

	// Very different distributions should have higher PSI
	b := []float64{100, 200, 300, 400, 500, 600, 700, 800, 900, 1000}
	psi = PopulationStabilityIndex(a, b, 5)
	if psi < 0.1 {
		t.Errorf("PSI(very different) = %f, want > 0.1", psi)
	}

	// Empty slices
	psi = PopulationStabilityIndex(nil, a, 5)
	if psi != 0 {
		t.Errorf("PSI(empty) = %f, want 0", psi)
	}
}
