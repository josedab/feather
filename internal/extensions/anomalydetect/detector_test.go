package anomalydetect

import (
	"testing"
	"time"
)

func TestNewDetector(t *testing.T) {
	cfg := DefaultDetectorConfig()
	d := NewDetector(cfg)
	if d == nil {
		t.Fatal("expected non-nil detector")
	}
	if d.config.ZScoreThreshold != 3.0 {
		t.Errorf("expected ZScoreThreshold=3.0, got %f", d.config.ZScoreThreshold)
	}
}

func TestNormalValues(t *testing.T) {
	cfg := DefaultDetectorConfig()
	cfg.LearningPeriod = 10
	d := NewDetector(cfg)

	// Feed 100 values in a tight normal range
	for i := 0; i < 100; i++ {
		result := d.Check("feature1", 50.0+float64(i%5))
		if result.IsAnomaly {
			t.Errorf("value %f should not be anomaly at step %d", result.Value, i)
		}
	}
}

func TestZScoreAnomaly(t *testing.T) {
	cfg := DefaultDetectorConfig()
	cfg.LearningPeriod = 20
	cfg.ZScoreThreshold = 3.0
	d := NewDetector(cfg)

	// Build a normal distribution around 100
	for i := 0; i < 50; i++ {
		d.Check("feature1", 100.0+float64(i%3)-1.0)
	}

	// Inject a massive outlier
	result := d.Check("feature1", 10000.0)
	if !result.IsAnomaly {
		t.Error("expected outlier 10000.0 to be detected as anomaly")
	}
	if result.Type != AnomalyZScore {
		t.Errorf("expected AnomalyZScore, got %s", result.Type)
	}
}

func TestIQRAnomaly(t *testing.T) {
	cfg := DefaultDetectorConfig()
	cfg.LearningPeriod = 10
	cfg.ZScoreThreshold = 100 // Disable z-score for this test
	cfg.IQRMultiplier = 1.5
	d := NewDetector(cfg)

	// Tight distribution: values 1-20
	for i := 0; i < 50; i++ {
		d.Check("feature1", float64(i%20)+1)
	}

	// Value far outside IQR
	result := d.Check("feature1", 500.0)
	if !result.IsAnomaly {
		t.Error("expected IQR anomaly for value 500.0")
	}
	if result.Type != AnomalyIQR {
		t.Errorf("expected AnomalyIQR, got %s", result.Type)
	}
}

func TestQuarantine(t *testing.T) {
	cfg := DefaultDetectorConfig()
	cfg.LearningPeriod = 10
	cfg.QuarantineEnabled = true
	d := NewDetector(cfg)

	for i := 0; i < 50; i++ {
		d.Check("feature1", 10.0)
	}

	// Trigger anomaly
	d.Check("feature1", 100000.0)

	if !d.IsQuarantined("feature1") {
		t.Error("expected feature1 to be quarantined after anomaly")
	}
}

func TestClearQuarantine(t *testing.T) {
	cfg := DefaultDetectorConfig()
	cfg.LearningPeriod = 10
	cfg.QuarantineEnabled = true
	d := NewDetector(cfg)

	for i := 0; i < 50; i++ {
		d.Check("feature1", 10.0)
	}
	d.Check("feature1", 100000.0)

	if !d.IsQuarantined("feature1") {
		t.Fatal("expected quarantine before clear")
	}

	if err := d.ClearQuarantine("feature1"); err != nil {
		t.Fatalf("ClearQuarantine failed: %v", err)
	}

	if d.IsQuarantined("feature1") {
		t.Error("expected feature1 to not be quarantined after clear")
	}

	// Non-existent feature
	if err := d.ClearQuarantine("nonexistent"); err != ErrFeatureNotMonitored {
		t.Errorf("expected ErrFeatureNotMonitored, got %v", err)
	}
}

func TestAlerts(t *testing.T) {
	cfg := DefaultDetectorConfig()
	cfg.LearningPeriod = 10
	d := NewDetector(cfg)

	for i := 0; i < 50; i++ {
		d.Check("feature1", 10.0)
	}

	before := time.Now().Add(-1 * time.Second)
	d.Check("feature1", 100000.0)

	alerts := d.GetAlerts(before, 10)
	if len(alerts) == 0 {
		t.Error("expected at least one alert")
	}
}

func TestStats(t *testing.T) {
	cfg := DefaultDetectorConfig()
	cfg.LearningPeriod = 5
	d := NewDetector(cfg)

	for i := 0; i < 20; i++ {
		d.Check("feature1", 10.0)
	}

	stats := d.Stats()
	if stats.TotalFeatures != 1 {
		t.Errorf("expected 1 feature, got %d", stats.TotalFeatures)
	}
	if stats.TotalChecks != 20 {
		t.Errorf("expected 20 checks, got %d", stats.TotalChecks)
	}
}

func TestDetector_RegisterFeature(t *testing.T) {
	t.Parallel()
	d := NewDetector(DefaultDetectorConfig())

	d.RegisterFeature("clicks")
	d.RegisterFeature("revenue")

	stats := d.Stats()
	if stats.TotalFeatures != 2 {
		t.Errorf("expected 2 features, got %d", stats.TotalFeatures)
	}

	// Registering the same feature again should not create a duplicate.
	d.RegisterFeature("clicks")
	stats = d.Stats()
	if stats.TotalFeatures != 2 {
		t.Errorf("expected 2 features after duplicate register, got %d", stats.TotalFeatures)
	}
}

func TestDetector_GetFeatureStats(t *testing.T) {
	t.Parallel()
	d := NewDetector(DefaultDetectorConfig())

	// Unregistered feature should return error.
	_, err := d.GetFeatureStats("nonexistent")
	if err != ErrFeatureNotMonitored {
		t.Errorf("expected ErrFeatureNotMonitored, got %v", err)
	}

	// Register and feed some values.
	d.RegisterFeature("clicks")
	for i := 0; i < 10; i++ {
		d.Check("clicks", float64(i))
	}

	fs, err := d.GetFeatureStats("clicks")
	if err != nil {
		t.Fatalf("GetFeatureStats failed: %v", err)
	}
	if fs["mean"] == nil {
		t.Error("expected mean in feature stats")
	}
	if fs["stddev"] == nil {
		t.Error("expected stddev in feature stats")
	}
	if fs["anomaly_rate"] == nil {
		t.Error("expected anomaly_rate in feature stats")
	}
}
