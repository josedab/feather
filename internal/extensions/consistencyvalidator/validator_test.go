package consistencyvalidator

import (
	"testing"
	"time"
)

func TestNewValidator(t *testing.T) {
	v := NewValidator(DefaultValidatorConfig())
	if v == nil {
		t.Fatal("expected non-nil validator")
	}
}

func TestConsistentFeatures(t *testing.T) {
	v := NewValidator(DefaultValidatorConfig())
	v.RegisterFeature("price")

	// Same distribution online and offline
	for i := 0; i < 100; i++ {
		val := float64(i) * 0.1
		v.RecordOnline("price", val)
		v.RecordOffline("price", val)
	}

	reports := v.CheckAll()
	if len(reports) != 1 {
		t.Fatalf("expected 1 report, got %d", len(reports))
	}
	if !reports[0].Consistent {
		t.Errorf("expected consistent, got skew: %s (score=%.4f)", reports[0].SkewType, reports[0].DivergenceScore)
	}
}

func TestInconsistentFeatures(t *testing.T) {
	v := NewValidator(ValidatorConfig{
		SampleSize:          1000,
		DivergenceThreshold: 0.05,
		MaxAlerts:           100,
		AlertCooldown:       0,
	})
	v.RegisterFeature("price")

	// Different distributions
	for i := 0; i < 100; i++ {
		v.RecordOnline("price", float64(i))
		v.RecordOffline("price", float64(i)+500)
	}

	reports := v.CheckAll()
	if len(reports) != 1 {
		t.Fatalf("expected 1 report, got %d", len(reports))
	}
	if reports[0].Consistent {
		t.Error("expected inconsistent")
	}
}

func TestAlerts(t *testing.T) {
	v := NewValidator(ValidatorConfig{
		SampleSize:          1000,
		DivergenceThreshold: 0.05,
		MaxAlerts:           100,
		AlertCooldown:       0,
	})

	for i := 0; i < 100; i++ {
		v.RecordOnline("price", float64(i))
		v.RecordOffline("price", float64(i)+1000)
	}

	v.CheckAll()
	alerts := v.GetAlerts(time.Time{})
	if len(alerts) == 0 {
		t.Error("expected at least one alert")
	}
}

func TestInsufficientData(t *testing.T) {
	v := NewValidator(DefaultValidatorConfig())
	v.RegisterFeature("sparse")

	v.RecordOnline("sparse", 1.0)
	v.RecordOffline("sparse", 1.0)

	reports := v.CheckAll()
	if len(reports) != 1 {
		t.Fatalf("expected 1 report, got %d", len(reports))
	}
	if reports[0].Message != "insufficient data" {
		t.Errorf("expected 'insufficient data' message, got %q", reports[0].Message)
	}
}

func TestStats(t *testing.T) {
	v := NewValidator(DefaultValidatorConfig())
	v.RegisterFeature("f1")
	v.RegisterFeature("f2")

	stats := v.Stats()
	if stats.TotalFeatures != 2 {
		t.Errorf("expected 2 features, got %d", stats.TotalFeatures)
	}
}

func TestCheck_UnregisteredFeature(t *testing.T) {
	v := NewValidator(DefaultValidatorConfig())
	_, err := v.Check("nonexistent")
	if err != ErrFeatureNotRegistered {
		t.Errorf("expected ErrFeatureNotRegistered, got %v", err)
	}
}

func TestCheck_InsufficientData(t *testing.T) {
	v := NewValidator(DefaultValidatorConfig())
	v.RegisterFeature("sparse")
	for i := 0; i < 10; i++ {
		v.RecordOnline("sparse", float64(i))
		v.RecordOffline("sparse", float64(i))
	}
	report, err := v.Check("sparse")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.Message != "insufficient data" {
		t.Errorf("expected 'insufficient data', got %q", report.Message)
	}
}

func TestCheck_ConsistentDistributions(t *testing.T) {
	v := NewValidator(DefaultValidatorConfig())
	v.RegisterFeature("price")
	for i := 0; i < 100; i++ {
		val := float64(i) * 0.1
		v.RecordOnline("price", val)
		v.RecordOffline("price", val)
	}
	report, err := v.Check("price")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !report.Consistent {
		t.Errorf("same distribution should be consistent, got skew=%s score=%.4f", report.SkewType, report.DivergenceScore)
	}
}

func TestCheck_DivergentDistributions(t *testing.T) {
	v := NewValidator(ValidatorConfig{
		SampleSize:          1000,
		DivergenceThreshold: 0.05,
		MaxAlerts:           100,
		AlertCooldown:       0,
	})
	v.RegisterFeature("price")
	for i := 0; i < 100; i++ {
		v.RecordOnline("price", float64(i))
		v.RecordOffline("price", float64(i)+500)
	}
	report, err := v.Check("price")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.Consistent {
		t.Error("divergent distributions should be inconsistent")
	}
	alerts := v.GetAlerts(time.Time{})
	if len(alerts) == 0 {
		t.Error("expected alert for divergent distribution")
	}
}

func TestCheckExtended_AllPass(t *testing.T) {
	v := NewValidator(DefaultValidatorConfig())
	v.RegisterFeature("feature_a")
	for i := 0; i < 100; i++ {
		val := float64(i) * 0.1
		v.RecordOnline("feature_a", val)
		v.RecordOffline("feature_a", val)
	}
	report, err := v.CheckExtended("feature_a")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !report.Consistent {
		t.Error("identical distributions should pass all tests")
	}
	if len(report.Tests) == 0 {
		t.Error("expected test results")
	}
}

func TestCheckExtended_SomeFail(t *testing.T) {
	v := NewValidator(DefaultValidatorConfig())
	v.RegisterFeature("feature_b")
	for i := 0; i < 100; i++ {
		v.RecordOnline("feature_b", float64(i))
		v.RecordOffline("feature_b", float64(i)+500)
	}
	report, err := v.CheckExtended("feature_b")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.Consistent {
		t.Error("divergent distributions should fail some tests")
	}
}

func TestCheckExtended_PerFeatureConfig(t *testing.T) {
	v := NewValidator(DefaultValidatorConfig())
	v.RegisterFeature("custom")
	// Set a very loose threshold so everything passes
	v.SetFeatureConfig("custom", PerFeatureConfig{
		KSThreshold:         1.0,
		PSIThreshold:        10.0,
		ChiSquaredThreshold: 1.0,
		JSThreshold:         1.0,
		EnabledTests:        []StatisticalTest{TestKS},
	})
	for i := 0; i < 100; i++ {
		v.RecordOnline("custom", float64(i))
		v.RecordOffline("custom", float64(i)+100)
	}
	report, err := v.CheckExtended("custom")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !report.Consistent {
		t.Error("very loose threshold should pass")
	}
}

func TestCheckExtended_InsufficientData(t *testing.T) {
	v := NewValidator(DefaultValidatorConfig())
	v.RegisterFeature("sparse")
	v.RecordOnline("sparse", 1.0)
	v.RecordOffline("sparse", 1.0)
	report, err := v.CheckExtended("sparse")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.Message != "insufficient data" {
		t.Errorf("expected 'insufficient data', got %q", report.Message)
	}
}

func TestCheckExtended_Unregistered(t *testing.T) {
	v := NewValidator(DefaultValidatorConfig())
	_, err := v.CheckExtended("nonexistent")
	if err != ErrFeatureNotRegistered {
		t.Errorf("expected ErrFeatureNotRegistered, got %v", err)
	}
}

func TestGetReports_FilterByFeature(t *testing.T) {
	v := NewValidator(DefaultValidatorConfig())
	v.RegisterFeature("f1")
	v.RegisterFeature("f2")
	for i := 0; i < 100; i++ {
		v.RecordOnline("f1", float64(i))
		v.RecordOffline("f1", float64(i))
		v.RecordOnline("f2", float64(i))
		v.RecordOffline("f2", float64(i))
	}
	v.CheckAll()

	reports := v.GetReports("f1", 50)
	for _, r := range reports {
		if r.Feature != "f1" {
			t.Errorf("expected only f1 reports, got %s", r.Feature)
		}
	}
}

func TestGetReports_Limit(t *testing.T) {
	v := NewValidator(DefaultValidatorConfig())
	v.RegisterFeature("f1")
	for i := 0; i < 100; i++ {
		v.RecordOnline("f1", float64(i))
		v.RecordOffline("f1", float64(i))
	}
	// Generate multiple reports
	v.CheckAll()
	v.CheckAll()
	v.CheckAll()

	reports := v.GetReports("", 2)
	if len(reports) > 2 {
		t.Errorf("expected at most 2 reports, got %d", len(reports))
	}
}

func TestGetReports_DefaultLimit(t *testing.T) {
	v := NewValidator(DefaultValidatorConfig())
	reports := v.GetReports("", 0) // 0 should use default of 50
	_ = reports                    // Just verify no panic
}

func TestGetReports_Empty(t *testing.T) {
	v := NewValidator(DefaultValidatorConfig())
	reports := v.GetReports("", 50)
	if len(reports) != 0 {
		t.Errorf("expected 0 reports, got %d", len(reports))
	}
}

func TestSetGetFeatureConfig(t *testing.T) {
	v := NewValidator(DefaultValidatorConfig())
	cfg := PerFeatureConfig{
		KSThreshold:  0.1,
		PSIThreshold: 0.5,
	}
	v.SetFeatureConfig("my_feature", cfg)
	got := v.GetFeatureConfig("my_feature")
	if got.KSThreshold != 0.1 {
		t.Errorf("expected KS threshold 0.1, got %f", got.KSThreshold)
	}
	if got.PSIThreshold != 0.5 {
		t.Errorf("expected PSI threshold 0.5, got %f", got.PSIThreshold)
	}
}

func TestGetFeatureConfig_FallbackToDefaults(t *testing.T) {
	v := NewValidator(DefaultValidatorConfig())
	got := v.GetFeatureConfig("unknown")
	defaults := DefaultPerFeatureConfig()
	if got.KSThreshold != defaults.KSThreshold {
		t.Errorf("expected default KS threshold, got %f", got.KSThreshold)
	}
}

func TestSnapshot_MultipleFeatures(t *testing.T) {
	v := NewValidator(DefaultValidatorConfig())
	v.RegisterFeature("f1")
	v.RegisterFeature("f2")
	for i := 0; i < 10; i++ {
		v.RecordOnline("f1", float64(i))
		v.RecordOffline("f1", float64(i))
		v.RecordOnline("f2", float64(i)*2)
		v.RecordOffline("f2", float64(i)*2)
	}
	snapshots := v.Snapshot()
	// 2 features × 2 sources (online+offline) = 4 snapshots
	if len(snapshots) != 4 {
		t.Errorf("expected 4 snapshots, got %d", len(snapshots))
	}
}

func TestSnapshot_Empty(t *testing.T) {
	v := NewValidator(DefaultValidatorConfig())
	snapshots := v.Snapshot()
	if len(snapshots) != 0 {
		t.Errorf("expected 0 snapshots, got %d", len(snapshots))
	}
}

func TestListFeatures(t *testing.T) {
	v := NewValidator(DefaultValidatorConfig())
	v.RegisterFeature("alpha")
	v.RegisterFeature("beta")
	features := v.ListFeatures()
	if len(features) != 2 {
		t.Errorf("expected 2 features, got %d", len(features))
	}
}
