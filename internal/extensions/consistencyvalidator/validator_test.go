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
