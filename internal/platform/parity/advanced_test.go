package parity

import (
	"math"
	"math/rand"
	"testing"
)

func TestAdvancedChecker_RecordAndRunTests(t *testing.T) {
	checker := NewAdvancedChecker(DefaultAdvancedConfig())

	// Record similar online/offline samples.
	rng := rand.New(rand.NewSource(42))
	for i := 0; i < 100; i++ {
		v := rng.Float64() * 100
		checker.RecordSample("click_count", v, v+rng.Float64()*0.01)
	}

	report, err := checker.RunTests("click_count")
	if err != nil {
		t.Fatalf("running tests: %v", err)
	}
	if report.FeatureName != "click_count" {
		t.Errorf("expected feature name 'click_count', got %q", report.FeatureName)
	}
	if report.SampleSize != 100 {
		t.Errorf("expected 100 samples, got %d", report.SampleSize)
	}
	if !report.InParity {
		t.Error("expected in-parity for similar distributions")
	}
	if len(report.Tests) < 2 {
		t.Errorf("expected at least 2 tests, got %d", len(report.Tests))
	}
}

func TestAdvancedChecker_DivergentDistributions(t *testing.T) {
	checker := NewAdvancedChecker(DefaultAdvancedConfig())

	rng := rand.New(rand.NewSource(42))
	for i := 0; i < 200; i++ {
		online := rng.NormFloat64()*10 + 50
		offline := rng.NormFloat64()*10 + 80 // Shifted mean
		checker.RecordSample("purchase_amount", online, offline)
	}

	report, err := checker.RunTests("purchase_amount")
	if err != nil {
		t.Fatalf("running tests: %v", err)
	}
	if report.InParity {
		t.Error("expected out-of-parity for shifted distributions")
	}
	if report.MeanDifference < 10 {
		t.Errorf("expected mean difference > 10, got %f", report.MeanDifference)
	}
}

func TestAdvancedChecker_InsufficientSamples(t *testing.T) {
	checker := NewAdvancedChecker(DefaultAdvancedConfig())
	checker.RecordSample("sparse", 1.0, 1.0)

	_, err := checker.RunTests("sparse")
	if err == nil {
		t.Error("expected error for insufficient samples")
	}
}

func TestAdvancedChecker_NoSamples(t *testing.T) {
	checker := NewAdvancedChecker(DefaultAdvancedConfig())
	_, err := checker.RunTests("nonexistent")
	if err == nil {
		t.Error("expected error for no samples")
	}
}

func TestAdvancedChecker_GetReport(t *testing.T) {
	checker := NewAdvancedChecker(DefaultAdvancedConfig())

	rng := rand.New(rand.NewSource(42))
	for i := 0; i < 50; i++ {
		v := rng.Float64()
		checker.RecordSample("f1", v, v)
	}
	checker.RunTests("f1")

	report := checker.GetReport("f1")
	if report == nil {
		t.Fatal("expected non-nil report")
	}
	if report.FeatureName != "f1" {
		t.Errorf("wrong feature: %q", report.FeatureName)
	}

	all := checker.GetAllReports()
	if len(all) != 1 {
		t.Errorf("expected 1 report, got %d", len(all))
	}
}

func TestKolmogorovSmirnovTest_Identical(t *testing.T) {
	data := []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	result := kolmogorovSmirnovTest(data, data, 0.15)
	if result.Significant {
		t.Error("identical data should not be significant")
	}
}

func TestPopulationStabilityIndex_Stable(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	a := make([]float64, 1000)
	b := make([]float64, 1000)
	for i := range a {
		a[i] = rng.NormFloat64()
		b[i] = rng.NormFloat64()
	}
	result := populationStabilityIndex(a, b, 20, 0.2)
	if result.Significant {
		t.Errorf("similar distributions should not trigger PSI alert (PSI=%f)", result.Statistic)
	}
}

func TestPopulationStabilityIndex_Shifted(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	a := make([]float64, 500)
	b := make([]float64, 500)
	for i := range a {
		a[i] = rng.NormFloat64()
		b[i] = rng.NormFloat64() + 5 // Large shift
	}
	result := populationStabilityIndex(a, b, 20, 0.2)
	if !result.Significant {
		t.Error("heavily shifted distributions should trigger PSI alert")
	}
}

func TestDefaultAdvancedConfig(t *testing.T) {
	cfg := DefaultAdvancedConfig()
	if cfg.KSThreshold <= 0 {
		t.Error("KS threshold should be positive")
	}
	if cfg.PSIThreshold <= 0 {
		t.Error("PSI threshold should be positive")
	}
	if cfg.NumBins <= 0 {
		t.Error("NumBins should be positive")
	}
}

func TestAdvancedChecker_Webhook(t *testing.T) {
	checker := NewAdvancedChecker(DefaultAdvancedConfig())
	checker.RegisterWebhook(WebhookConfig{
		URL:         "https://hooks.slack.com/test",
		MinSeverity: "warning",
	})

	_ = math.Pi // suppress import
}

func TestSampleMaxLimit(t *testing.T) {
	cfg := DefaultAdvancedConfig()
	cfg.MaxSamples = 20
	checker := NewAdvancedChecker(cfg)

	for i := 0; i < 50; i++ {
		checker.RecordSample("limited", float64(i), float64(i))
	}

	checker.mu.RLock()
	s := checker.samples["limited"]
	checker.mu.RUnlock()

	if len(s.online) > 20 {
		t.Errorf("expected max 20 samples, got %d", len(s.online))
	}
}
