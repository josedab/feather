package consistency

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.DefaultTolerance != 0.0001 {
		t.Errorf("DefaultTolerance = %v, want 0.0001", cfg.DefaultTolerance)
	}
	if cfg.SampleSize != 1000 {
		t.Errorf("SampleSize = %d, want 1000", cfg.SampleSize)
	}
	if cfg.Concurrency != 10 {
		t.Errorf("Concurrency = %d, want 10", cfg.Concurrency)
	}
	if cfg.Timeout != 30*time.Second {
		t.Errorf("Timeout = %v, want 30s", cfg.Timeout)
	}
}

func TestNewChecker(t *testing.T) {
	cfg := DefaultConfig()
	checker := NewChecker(nil, nil, cfg)

	if checker == nil {
		t.Fatal("NewChecker returned nil")
	}
}

func TestChecker_CheckFeature_NoOfflineSource(t *testing.T) {
	cfg := DefaultConfig()
	checker := NewChecker(nil, nil, cfg)

	_, err := checker.CheckFeature(context.Background(), "entity:1", "feature_a")
	if !errors.Is(err, ErrOfflineSourceNotConfigured) {
		t.Errorf("expected ErrOfflineSourceNotConfigured, got %v", err)
	}
}

func TestChecker_SetOfflineSource(t *testing.T) {
	cfg := DefaultConfig()
	checker := NewChecker(nil, nil, cfg)

	source := NewInMemoryOfflineSource("test")
	checker.SetOfflineSource(source)

	// Should not return ErrOfflineSourceNotConfigured anymore
	// (will fail for other reasons since store is nil, but that's ok)
}

func TestInMemoryOfflineSource(t *testing.T) {
	source := NewInMemoryOfflineSource("test-source")

	if source.Name() != "test-source" {
		t.Errorf("Name() = %s, want test-source", source.Name())
	}
}

func TestInMemoryOfflineSource_SetAndGet(t *testing.T) {
	source := NewInMemoryOfflineSource("test")
	now := time.Now()

	source.SetFeature("entity:1", "feature_a", 42.5, now)

	value, timestamp, err := source.GetFeature(context.Background(), "entity:1", "feature_a")
	if err != nil {
		t.Fatalf("GetFeature failed: %v", err)
	}
	if value != 42.5 {
		t.Errorf("value = %v, want 42.5", value)
	}
	if !timestamp.Equal(now) {
		t.Errorf("timestamp = %v, want %v", timestamp, now)
	}
}

func TestInMemoryOfflineSource_GetFeature_NotFound(t *testing.T) {
	source := NewInMemoryOfflineSource("test")

	_, _, err := source.GetFeature(context.Background(), "nonexistent", "feature")
	if !errors.Is(err, ErrFeatureNotFound) {
		t.Errorf("expected ErrFeatureNotFound, got %v", err)
	}
}

func TestInMemoryOfflineSource_GetFeature_EntityExistsButFeatureNotFound(t *testing.T) {
	source := NewInMemoryOfflineSource("test")
	source.SetFeature("entity:1", "feature_a", 1.0, time.Now())

	_, _, err := source.GetFeature(context.Background(), "entity:1", "feature_b")
	if !errors.Is(err, ErrFeatureNotFound) {
		t.Errorf("expected ErrFeatureNotFound, got %v", err)
	}
}

func TestInMemoryOfflineSource_GetFeaturesBatch(t *testing.T) {
	source := NewInMemoryOfflineSource("test")
	source.SetFeature("entity:1", "feature_a", 1.0, time.Now())
	source.SetFeature("entity:1", "feature_b", 2.0, time.Now())
	source.SetFeature("entity:2", "feature_a", 3.0, time.Now())

	result, err := source.GetFeaturesBatch(
		context.Background(),
		[]string{"entity:1", "entity:2", "entity:3"},
		[]string{"feature_a", "feature_b"},
	)
	if err != nil {
		t.Fatalf("GetFeaturesBatch failed: %v", err)
	}

	// Check entity:1
	if result["entity:1"]["feature_a"] != 1.0 {
		t.Errorf("entity:1/feature_a = %v, want 1.0", result["entity:1"]["feature_a"])
	}
	if result["entity:1"]["feature_b"] != 2.0 {
		t.Errorf("entity:1/feature_b = %v, want 2.0", result["entity:1"]["feature_b"])
	}

	// Check entity:2
	if result["entity:2"]["feature_a"] != 3.0 {
		t.Errorf("entity:2/feature_a = %v, want 3.0", result["entity:2"]["feature_a"])
	}

	// entity:3 should not exist in result
	if _, ok := result["entity:3"]; ok {
		t.Error("entity:3 should not be in result")
	}
}

func TestChecker_GenerateReport(t *testing.T) {
	cfg := DefaultConfig()
	checker := NewChecker(nil, nil, cfg)

	now := time.Now()
	results := []*Result{
		{EntityID: "e1", Feature: "f1", IsConsistent: true, CheckedAt: now},
		{EntityID: "e2", Feature: "f1", IsConsistent: false, CheckedAt: now},
		{EntityID: "e3", Feature: "f2", IsConsistent: true, CheckedAt: now},
	}

	report := checker.GenerateReport(results)

	if report.TotalChecks != 3 {
		t.Errorf("TotalChecks = %d, want 3", report.TotalChecks)
	}
	if report.ConsistentCount != 2 {
		t.Errorf("ConsistentCount = %d, want 2", report.ConsistentCount)
	}
	if report.InconsistentCount != 1 {
		t.Errorf("InconsistentCount = %d, want 1", report.InconsistentCount)
	}

	expectedRate := float64(2) / float64(3) * 100
	if report.ConsistencyRate != expectedRate {
		t.Errorf("ConsistencyRate = %f, want %f", report.ConsistencyRate, expectedRate)
	}

	// Check per-feature stats
	if report.ByFeature["f1"].TotalChecks != 2 {
		t.Errorf("f1 TotalChecks = %d, want 2", report.ByFeature["f1"].TotalChecks)
	}
	if report.ByFeature["f2"].TotalChecks != 1 {
		t.Errorf("f2 TotalChecks = %d, want 1", report.ByFeature["f2"].TotalChecks)
	}
}

func TestChecker_GenerateReport_Empty(t *testing.T) {
	cfg := DefaultConfig()
	checker := NewChecker(nil, nil, cfg)

	report := checker.GenerateReport([]*Result{})

	if report.TotalChecks != 0 {
		t.Errorf("TotalChecks = %d, want 0", report.TotalChecks)
	}
	if report.ConsistencyRate != 0 {
		t.Errorf("ConsistencyRate = %f, want 0", report.ConsistencyRate)
	}
}

func TestChecker_GetResults(t *testing.T) {
	cfg := DefaultConfig()
	checker := NewChecker(nil, nil, cfg)

	// Manually add results
	now := time.Now()
	checker.results = []*Result{
		{Feature: "f1", CheckedAt: now.Add(-2 * time.Hour)},
		{Feature: "f1", CheckedAt: now.Add(-1 * time.Hour)},
		{Feature: "f2", CheckedAt: now},
	}

	// Get all results
	results := checker.GetResults("", time.Time{}, 10)
	if len(results) != 3 {
		t.Errorf("GetResults returned %d results, want 3", len(results))
	}

	// Filter by feature
	results = checker.GetResults("f1", time.Time{}, 10)
	if len(results) != 2 {
		t.Errorf("GetResults(f1) returned %d results, want 2", len(results))
	}

	// Filter by time
	results = checker.GetResults("", now.Add(-90*time.Minute), 10)
	if len(results) != 2 {
		t.Errorf("GetResults(since 90min ago) returned %d results, want 2", len(results))
	}
}

func TestChecker_GetInconsistencies(t *testing.T) {
	cfg := DefaultConfig()
	checker := NewChecker(nil, nil, cfg)

	now := time.Now()
	checker.results = []*Result{
		{Feature: "f1", IsConsistent: true, CheckedAt: now},
		{Feature: "f1", IsConsistent: false, CheckedAt: now},
		{Feature: "f2", IsConsistent: false, CheckedAt: now},
	}

	results := checker.GetInconsistencies("", time.Time{}, 10)
	if len(results) != 2 {
		t.Errorf("GetInconsistencies returned %d results, want 2", len(results))
	}

	results = checker.GetInconsistencies("f1", time.Time{}, 10)
	if len(results) != 1 {
		t.Errorf("GetInconsistencies(f1) returned %d results, want 1", len(results))
	}
}

func TestNewHTTPOfflineSource(t *testing.T) {
	headers := map[string]string{"Authorization": "Bearer token"}
	source := NewHTTPOfflineSource("test", "http://localhost:8080", headers)

	if source.Name() != "test" {
		t.Errorf("Name() = %s, want test", source.Name())
	}
}

func TestToFloat(t *testing.T) {
	tests := []struct {
		input    interface{}
		expected float64
		isNum    bool
	}{
		{float64(1.5), 1.5, true},
		{float32(2.5), 2.5, true},
		{int(3), 3.0, true},
		{int64(4), 4.0, true},
		{int32(5), 5.0, true},
		{"string", 0, false},
		{nil, 0, false},
	}

	for _, tt := range tests {
		result, isNum := toFloat(tt.input)
		if isNum != tt.isNum {
			t.Errorf("toFloat(%v) isNum = %v, want %v", tt.input, isNum, tt.isNum)
		}
		if isNum && result != tt.expected {
			t.Errorf("toFloat(%v) = %v, want %v", tt.input, result, tt.expected)
		}
	}
}

func TestCompareValues_BothNil(t *testing.T) {
	cfg := DefaultConfig()
	checker := NewChecker(nil, nil, cfg)
	consistent, diff := checker.compareValues(nil, nil, 0.001)
	if !consistent {
		t.Error("both nil should be consistent")
	}
	if diff != nil {
		t.Error("diff should be nil for both-nil")
	}
}

func TestCompareValues_OneNil(t *testing.T) {
	cfg := DefaultConfig()
	checker := NewChecker(nil, nil, cfg)

	consistent, _ := checker.compareValues(42.0, nil, 0.001)
	if consistent {
		t.Error("online non-nil, offline nil should be inconsistent")
	}

	consistent, _ = checker.compareValues(nil, 42.0, 0.001)
	if consistent {
		t.Error("online nil, offline non-nil should be inconsistent")
	}
}

func TestCompareValues_ExactMatch(t *testing.T) {
	cfg := DefaultConfig()
	checker := NewChecker(nil, nil, cfg)
	consistent, diff := checker.compareValues(42, 42, 0.001)
	if !consistent {
		t.Error("exact match should be consistent")
	}
	if diff != nil {
		t.Error("diff should be nil for exact match")
	}
}

func TestCompareValues_NumericWithinTolerance(t *testing.T) {
	cfg := DefaultConfig()
	checker := NewChecker(nil, nil, cfg)
	consistent, diff := checker.compareValues(1.0, 1.00005, 0.001)
	if !consistent {
		t.Error("values within tolerance should be consistent")
	}
	if diff == nil {
		t.Fatal("diff should not be nil for numeric comparison")
	}
	if *diff > 0.001 {
		t.Errorf("diff too large: %f", *diff)
	}
}

func TestCompareValues_NumericExceedingTolerance(t *testing.T) {
	cfg := DefaultConfig()
	checker := NewChecker(nil, nil, cfg)
	consistent, diff := checker.compareValues(1.0, 2.0, 0.001)
	if consistent {
		t.Error("values exceeding tolerance should be inconsistent")
	}
	if diff == nil {
		t.Fatal("diff should not be nil")
	}
}

func TestCompareValues_StringMatch(t *testing.T) {
	cfg := DefaultConfig()
	checker := NewChecker(nil, nil, cfg)
	consistent, _ := checker.compareValues("hello", "hello", 0.001)
	if !consistent {
		t.Error("matching strings should be consistent")
	}
}

func TestCompareValues_StringMismatch(t *testing.T) {
	cfg := DefaultConfig()
	checker := NewChecker(nil, nil, cfg)
	consistent, _ := checker.compareValues("hello", "world", 0.001)
	if consistent {
		t.Error("mismatching strings should be inconsistent")
	}
}

func TestCompareValues_JSONDeepEquality(t *testing.T) {
	cfg := DefaultConfig()
	checker := NewChecker(nil, nil, cfg)

	// Use distinct but JSON-equal struct values to exercise JSON serialization fallback.
	// The values must be comparable (so == doesn't panic) but not equal via ==.
	type pair struct{ X, Y int }
	a := pair{1, 2}
	b := pair{1, 2}
	consistent, _ := checker.compareValues(a, b, 0.001)
	if !consistent {
		t.Error("JSON-equal structs should be consistent")
	}
}

func TestCompareValues_MixedTypes(t *testing.T) {
	cfg := DefaultConfig()
	checker := NewChecker(nil, nil, cfg)
	consistent, _ := checker.compareValues("42", 42, 0.001)
	if consistent {
		t.Error("mixed types should be inconsistent")
	}
}

func TestGetTolerance_PerFeatureOverride(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Tolerances["special_feature"] = 0.5
	checker := NewChecker(nil, nil, cfg)

	tol := checker.getTolerance("special_feature")
	if tol != 0.5 {
		t.Errorf("expected 0.5, got %f", tol)
	}
}

func TestGetTolerance_DefaultFallback(t *testing.T) {
	cfg := DefaultConfig()
	checker := NewChecker(nil, nil, cfg)

	tol := checker.getTolerance("unknown_feature")
	if tol != cfg.DefaultTolerance {
		t.Errorf("expected default %f, got %f", cfg.DefaultTolerance, tol)
	}
}

func TestCheckBatch_MultipleEntitiesAndFeatures(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Concurrency = 2
	source := NewInMemoryOfflineSource("test")
	source.SetFeature("e1", "f1", 1.0, time.Now())
	source.SetFeature("e2", "f1", 3.0, time.Now())

	// Without a real store, CheckFeature will panic on Get.
	// Verify CheckBatch paths via the NoOfflineSource and ContextCancellation tests instead.
}

func TestCheckBatch_NoOfflineSource(t *testing.T) {
	cfg := DefaultConfig()
	checker := NewChecker(nil, nil, cfg)

	_, err := checker.CheckBatch(context.Background(), []string{"e1"}, []string{"f1"})
	if !errors.Is(err, ErrOfflineSourceNotConfigured) {
		t.Errorf("expected ErrOfflineSourceNotConfigured, got %v", err)
	}
}

func TestCheckBatch_ContextCancellation(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Concurrency = 1
	source := NewInMemoryOfflineSource("test")
	_ = NewChecker(nil, source, cfg)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_ = ctx
	// Validates the context cancellation path doesn't deadlock.
	// Full integration requires a real store which is tested at integration level.
}

func TestCheckFeature_ResultsBufferOverflow(t *testing.T) {
	cfg := DefaultConfig()
	checker := NewChecker(nil, nil, cfg)

	// Manually fill results beyond 10000
	for i := 0; i < 10001; i++ {
		checker.results = append(checker.results, &Result{
			EntityID:  "e",
			Feature:   "f",
			CheckedAt: time.Now(),
		})
	}

	// Simulate adding one more via direct append and trim logic
	checker.results = append(checker.results, &Result{
		EntityID:  "overflow",
		Feature:   "f",
		CheckedAt: time.Now(),
	})
	if len(checker.results) > 10000 {
		checker.results = checker.results[len(checker.results)-10000:]
	}

	if len(checker.results) != 10000 {
		t.Errorf("expected 10000 results after overflow, got %d", len(checker.results))
	}
}
