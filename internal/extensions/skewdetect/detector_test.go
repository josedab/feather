package skewdetect

import (
	"errors"
	"math"
	"testing"
	"time"
)

func TestRegisterFeature(t *testing.T) {
	d := NewDetector(DefaultDetectorConfig())

	if err := d.RegisterFeature("f1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Duplicate registration should fail.
	if err := d.RegisterFeature("f1"); !errors.Is(err, ErrFeatureExists) {
		t.Fatalf("expected ErrFeatureExists, got %v", err)
	}
}

func TestRecordOnlineOffline(t *testing.T) {
	d := NewDetector(DetectorConfig{MaxSamples: 100})

	if err := d.RegisterFeature("f1"); err != nil {
		t.Fatal(err)
	}

	vals := make([]float64, 200)
	for i := range vals {
		vals[i] = float64(i)
	}

	if err := d.RecordOnline("f1", vals); err != nil {
		t.Fatal(err)
	}
	if err := d.RecordOffline("f1", vals); err != nil {
		t.Fatal(err)
	}

	fp, err := d.GetProfile("f1")
	if err != nil {
		t.Fatal(err)
	}

	// Samples should be capped at MaxSamples.
	if fp.OnlineStats.Count != 100 {
		t.Errorf("expected 100 online samples, got %d", fp.OnlineStats.Count)
	}
	if fp.OfflineStats.Count != 100 {
		t.Errorf("expected 100 offline samples, got %d", fp.OfflineStats.Count)
	}
}

func TestRecordUnknownFeature(t *testing.T) {
	d := NewDetector(DefaultDetectorConfig())

	if err := d.RecordOnline("unknown", []float64{1}); !errors.Is(err, ErrFeatureNotFound) {
		t.Fatalf("expected ErrFeatureNotFound, got %v", err)
	}
	if err := d.RecordOffline("unknown", []float64{1}); !errors.Is(err, ErrFeatureNotFound) {
		t.Fatalf("expected ErrFeatureNotFound, got %v", err)
	}
}

func TestCheckHealthy(t *testing.T) {
	d := NewDetector(DefaultDetectorConfig())
	_ = d.RegisterFeature("f1")

	data := make([]float64, 500)
	for i := range data {
		data[i] = float64(i)
	}

	// Both sides get the same data → no skew.
	_ = d.RecordOnline("f1", data)
	_ = d.RecordOffline("f1", data)

	fp, err := d.Check("f1")
	if err != nil {
		t.Fatal(err)
	}

	if fp.Status != "healthy" {
		t.Errorf("expected healthy, got %s (KS=%.4f PSI=%.4f JS=%.4f)",
			fp.Status, fp.KSStatistic, fp.PSIScore, fp.JSScore)
	}
	if fp.KSStatistic > 0.01 {
		t.Errorf("KS should be ~0 for identical data, got %.4f", fp.KSStatistic)
	}
}

func TestCheckSkewed(t *testing.T) {
	d := NewDetector(DetectorConfig{
		KSThreshold:  0.1,
		PSIThreshold: 0.25,
		JSThreshold:  0.1,
		MaxSamples:   10000,
	})
	_ = d.RegisterFeature("f1")

	offline := make([]float64, 1000)
	online := make([]float64, 1000)
	for i := 0; i < 1000; i++ {
		offline[i] = float64(i)        // 0-999
		online[i] = float64(i) + 10000 // 10000-10999, completely shifted
	}

	_ = d.RecordOffline("f1", offline)
	_ = d.RecordOnline("f1", online)

	fp, err := d.Check("f1")
	if err != nil {
		t.Fatal(err)
	}

	if fp.Status != "skewed" {
		t.Errorf("expected skewed, got %s", fp.Status)
	}
	if fp.KSStatistic < 0.9 {
		t.Errorf("KS should be ~1.0 for non-overlapping data, got %.4f", fp.KSStatistic)
	}
}

func TestCheckAll(t *testing.T) {
	d := NewDetector(DefaultDetectorConfig())
	_ = d.RegisterFeature("a")
	_ = d.RegisterFeature("b")

	data := []float64{1, 2, 3, 4, 5}
	_ = d.RecordOnline("a", data)
	_ = d.RecordOffline("a", data)
	_ = d.RecordOnline("b", data)
	_ = d.RecordOffline("b", data)

	profiles := d.CheckAll()
	if len(profiles) != 2 {
		t.Fatalf("expected 2 profiles, got %d", len(profiles))
	}
}

func TestAlerts(t *testing.T) {
	d := NewDetector(DetectorConfig{
		KSThreshold:  0.1,
		PSIThreshold: 0.25,
		JSThreshold:  0.1,
		MaxSamples:   10000,
	})
	_ = d.RegisterFeature("f1")

	offline := make([]float64, 500)
	online := make([]float64, 500)
	for i := 0; i < 500; i++ {
		offline[i] = float64(i)
		online[i] = float64(i) + 50000
	}
	_ = d.RecordOffline("f1", offline)
	_ = d.RecordOnline("f1", online)

	_, _ = d.Check("f1")

	alerts := d.GetAlerts(time.Now().Add(-1 * time.Hour))
	if len(alerts) == 0 {
		t.Fatal("expected at least one alert for heavily skewed data")
	}
	if alerts[0].Feature != "f1" {
		t.Errorf("expected feature f1, got %s", alerts[0].Feature)
	}

	// Severity should be critical for extremely shifted data.
	hasCritical := false
	for _, a := range alerts {
		if a.Severity == SeverityCritical {
			hasCritical = true
		}
	}
	if !hasCritical {
		t.Error("expected at least one critical alert for extreme skew")
	}
}

func TestContracts(t *testing.T) {
	d := NewDetector(DefaultDetectorConfig())
	_ = d.RegisterFeature("f1")

	_ = d.RecordOnline("f1", []float64{-5, 0, 5, 10, 15})
	_ = d.RecordOffline("f1", []float64{0, 5, 10})

	minVal := 0.0
	maxVal := 10.0
	err := d.SetContract(DataContract{
		Feature:  "f1",
		MinValue: &minVal,
		MaxValue: &maxVal,
		MaxSkew:  0.05,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Run a check so SkewScore is populated.
	_, _ = d.Check("f1")

	violations, err := d.ValidateContract("f1")
	if err != nil {
		t.Fatal(err)
	}

	// We expect min_value violation (-5 < 0) and max_value violation (15 > 10).
	ruleSet := make(map[string]bool)
	for _, v := range violations {
		ruleSet[v.Rule] = true
	}
	if !ruleSet["min_value"] {
		t.Error("expected min_value violation")
	}
	if !ruleSet["max_value"] {
		t.Error("expected max_value violation")
	}
}

func TestContractUnknownFeature(t *testing.T) {
	d := NewDetector(DefaultDetectorConfig())

	err := d.SetContract(DataContract{Feature: "unknown"})
	if !errors.Is(err, ErrFeatureNotFound) {
		t.Fatalf("expected ErrFeatureNotFound, got %v", err)
	}

	_, err = d.ValidateContract("unknown")
	if !errors.Is(err, ErrFeatureNotFound) {
		t.Fatalf("expected ErrFeatureNotFound, got %v", err)
	}
}

func TestStats(t *testing.T) {
	d := NewDetector(DetectorConfig{
		KSThreshold:  0.1,
		PSIThreshold: 0.25,
		JSThreshold:  0.1,
		MaxSamples:   10000,
	})
	_ = d.RegisterFeature("healthy")
	_ = d.RegisterFeature("skewed")

	data := make([]float64, 200)
	for i := range data {
		data[i] = float64(i)
	}
	_ = d.RecordOnline("healthy", data)
	_ = d.RecordOffline("healthy", data)

	shifted := make([]float64, 200)
	for i := range shifted {
		shifted[i] = float64(i) + 100000
	}
	_ = d.RecordOnline("skewed", shifted)
	_ = d.RecordOffline("skewed", data)

	_ = d.CheckAll()

	stats := d.Stats()
	if stats.TrackedFeatures != 2 {
		t.Errorf("expected 2 tracked features, got %d", stats.TrackedFeatures)
	}
	if stats.HealthyCount < 1 {
		t.Errorf("expected at least 1 healthy feature, got %d", stats.HealthyCount)
	}
	if stats.SkewedCount < 1 {
		t.Errorf("expected at least 1 skewed feature, got %d", stats.SkewedCount)
	}
	if stats.TotalAlerts == 0 {
		t.Error("expected at least one alert")
	}
}

func TestListProfiles(t *testing.T) {
	d := NewDetector(DefaultDetectorConfig())
	_ = d.RegisterFeature("a")
	_ = d.RegisterFeature("b")

	profiles := d.ListProfiles()
	if len(profiles) != 2 {
		t.Errorf("expected 2 profiles, got %d", len(profiles))
	}
}

func TestKSStatisticIdentical(t *testing.T) {
	data := []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	ks := ksStatistic(data, data)
	if ks > 0.01 {
		t.Errorf("KS for identical data should be ~0, got %f", ks)
	}
}

func TestKSStatisticDisjoint(t *testing.T) {
	a := []float64{1, 2, 3, 4, 5}
	b := []float64{100, 101, 102, 103, 104}
	ks := ksStatistic(a, b)
	if ks < 0.9 {
		t.Errorf("KS for disjoint data should be ~1, got %f", ks)
	}
}

func TestPSIIdentical(t *testing.T) {
	data := []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	psi := psiScore(data, data, 10)
	if psi > 0.01 {
		t.Errorf("PSI for identical data should be ~0, got %f", psi)
	}
}

func TestJSDivergenceIdentical(t *testing.T) {
	data := []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	js := jsDivergence(data, data, 10)
	if js > 0.01 {
		t.Errorf("JS for identical data should be ~0, got %f", js)
	}
}

func TestJSDivergenceShifted(t *testing.T) {
	a := make([]float64, 1000)
	b := make([]float64, 1000)
	for i := 0; i < 1000; i++ {
		a[i] = float64(i)
		b[i] = float64(i) + 50000
	}
	js := jsDivergence(a, b, 10)
	if js < 0.1 {
		t.Errorf("JS for shifted data should be large, got %f", js)
	}
}

func TestComputeStats(t *testing.T) {
	data := []float64{1, 2, 3, 4, 5}
	s := computeStats(data)

	if s.Count != 5 {
		t.Errorf("Count = %d, want 5", s.Count)
	}
	if s.Mean != 3.0 {
		t.Errorf("Mean = %f, want 3.0", s.Mean)
	}
	if s.Min != 1.0 {
		t.Errorf("Min = %f, want 1.0", s.Min)
	}
	if s.Max != 5.0 {
		t.Errorf("Max = %f, want 5.0", s.Max)
	}
	if s.P50 != 3.0 {
		t.Errorf("P50 = %f, want 3.0", s.P50)
	}

	expectedStdDev := math.Sqrt(2.0)
	if math.Abs(s.StdDev-expectedStdDev) > 0.001 {
		t.Errorf("StdDev = %f, want ~%f", s.StdDev, expectedStdDev)
	}
}

func TestScoreSeverity(t *testing.T) {
	tests := []struct {
		score, threshold float64
		want             Severity
	}{
		{0.05, 0.1, SeverityLow},
		{0.12, 0.1, SeverityMedium},
		{0.16, 0.1, SeverityHigh},
		{0.25, 0.1, SeverityCritical},
	}
	for _, tc := range tests {
		got := scoreSeverity(tc.score, tc.threshold)
		if got != tc.want {
			t.Errorf("scoreSeverity(%.2f, %.2f) = %s, want %s", tc.score, tc.threshold, got, tc.want)
		}
	}
}
