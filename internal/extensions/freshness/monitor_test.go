package freshness

import (
	"testing"
	"time"
)

func TestNewMonitor(t *testing.T) {
	monitor := NewMonitor(DefaultMonitorConfig())

	if monitor == nil {
		t.Fatal("Expected monitor to be non-nil")
	}
}

func TestMonitor_RecordAccess(t *testing.T) {
	config := DefaultMonitorConfig()
	config.CleanupInterval = 1 * time.Hour // Disable cleanup for test
	monitor := NewMonitor(config)

	// Record some accesses
	for i := 0; i < 10; i++ {
		monitor.RecordAccess("feature1", 10*time.Millisecond, true)
	}
	for i := 0; i < 5; i++ {
		monitor.RecordAccess("feature1", 50*time.Millisecond, false)
	}

	metrics, found := monitor.GetAccessMetrics("feature1")
	if !found {
		t.Fatal("Expected to find metrics for feature1")
	}

	if metrics.TotalAccesses != 15 {
		t.Errorf("Expected 15 total accesses, got %d", metrics.TotalAccesses)
	}
}

func TestMonitor_RecordAccess_CacheHitRate(t *testing.T) {
	config := DefaultMonitorConfig()
	config.CleanupInterval = 1 * time.Hour
	monitor := NewMonitor(config)

	// 8 hits, 2 misses = 80% hit rate
	for i := 0; i < 8; i++ {
		monitor.RecordAccess("feature1", 10*time.Millisecond, true)
	}
	for i := 0; i < 2; i++ {
		monitor.RecordAccess("feature1", 10*time.Millisecond, false)
	}

	metrics, _ := monitor.GetAccessMetrics("feature1")

	if metrics.CacheHitRate < 0.79 || metrics.CacheHitRate > 0.81 {
		t.Errorf("Expected cache hit rate ~0.8, got %f", metrics.CacheHitRate)
	}
}

func TestMonitor_RecordStaleServe(t *testing.T) {
	config := DefaultMonitorConfig()
	config.CleanupInterval = 1 * time.Hour
	monitor := NewMonitor(config)

	monitor.RecordAccess("feature1", 10*time.Millisecond, true)
	monitor.RecordStaleServe("feature1")
	monitor.RecordStaleServe("feature1")

	metrics, _ := monitor.GetAccessMetrics("feature1")

	if metrics.StaleServes != 2 {
		t.Errorf("Expected 2 stale serves, got %d", metrics.StaleServes)
	}
}

func TestMonitor_RecordChange(t *testing.T) {
	config := DefaultMonitorConfig()
	config.CleanupInterval = 1 * time.Hour
	monitor := NewMonitor(config)

	// Record changes
	monitor.RecordChange("feature1", 100.0, 110.0) // +10
	monitor.RecordChange("feature1", 110.0, 105.0) // -5
	monitor.RecordChange("feature1", 105.0, 120.0) // +15

	metrics, found := monitor.GetChangeMetrics("feature1")
	if !found {
		t.Fatal("Expected to find change metrics for feature1")
	}

	if metrics.TotalUpdates != 3 {
		t.Errorf("Expected 3 updates, got %d", metrics.TotalUpdates)
	}

	// Average change magnitude: (10 + 5 + 15) / 3 = 10
	if metrics.AvgChangeMagnitude < 9.9 || metrics.AvgChangeMagnitude > 10.1 {
		t.Errorf("Expected avg change magnitude ~10, got %f", metrics.AvgChangeMagnitude)
	}
}

func TestMonitor_RecordDriftScore(t *testing.T) {
	config := DefaultMonitorConfig()
	config.CleanupInterval = 1 * time.Hour
	monitor := NewMonitor(config)

	monitor.RecordDriftScore("feature1", 0.75)

	metrics, found := monitor.GetChangeMetrics("feature1")
	if !found {
		t.Fatal("Expected to find change metrics for feature1")
	}

	if metrics.DriftScore != 0.75 {
		t.Errorf("Expected drift score 0.75, got %f", metrics.DriftScore)
	}
}

func TestMonitor_GetAccessMetrics_NotFound(t *testing.T) {
	monitor := NewMonitor(DefaultMonitorConfig())

	_, found := monitor.GetAccessMetrics("nonexistent")
	if found {
		t.Error("Expected not to find metrics for nonexistent feature")
	}
}

func TestMonitor_GetChangeMetrics_NotFound(t *testing.T) {
	monitor := NewMonitor(DefaultMonitorConfig())

	_, found := monitor.GetChangeMetrics("nonexistent")
	if found {
		t.Error("Expected not to find metrics for nonexistent feature")
	}
}

func TestMonitor_GetAllAccessMetrics(t *testing.T) {
	config := DefaultMonitorConfig()
	config.CleanupInterval = 1 * time.Hour
	monitor := NewMonitor(config)

	monitor.RecordAccess("feature1", 10*time.Millisecond, true)
	monitor.RecordAccess("feature2", 20*time.Millisecond, false)
	monitor.RecordAccess("feature3", 30*time.Millisecond, true)

	metrics := monitor.GetAllAccessMetrics()

	if len(metrics) != 3 {
		t.Errorf("Expected 3 features, got %d", len(metrics))
	}
}

func TestMonitor_GetAllChangeMetrics(t *testing.T) {
	config := DefaultMonitorConfig()
	config.CleanupInterval = 1 * time.Hour
	monitor := NewMonitor(config)

	monitor.RecordChange("feature1", 100.0, 110.0)
	monitor.RecordChange("feature2", 200.0, 190.0)

	metrics := monitor.GetAllChangeMetrics()

	if len(metrics) != 2 {
		t.Errorf("Expected 2 features, got %d", len(metrics))
	}
}

func TestMonitor_Stats(t *testing.T) {
	config := DefaultMonitorConfig()
	config.CleanupInterval = 1 * time.Hour
	monitor := NewMonitor(config)

	monitor.RecordAccess("feature1", 10*time.Millisecond, true)
	monitor.RecordAccess("feature1", 10*time.Millisecond, true)
	monitor.RecordChange("feature1", 100.0, 110.0)

	stats := monitor.Stats()

	if stats.TrackedFeatures != 1 {
		t.Errorf("Expected 1 tracked feature, got %d", stats.TrackedFeatures)
	}
	if stats.TotalAccesses != 2 {
		t.Errorf("Expected 2 total accesses, got %d", stats.TotalAccesses)
	}
	if stats.TotalUpdates != 1 {
		t.Errorf("Expected 1 total update, got %d", stats.TotalUpdates)
	}
}

func TestMonitor_LatencyPercentiles(t *testing.T) {
	config := DefaultMonitorConfig()
	config.CleanupInterval = 1 * time.Hour
	monitor := NewMonitor(config)

	// Record varying latencies
	latencies := []time.Duration{
		10 * time.Millisecond,
		20 * time.Millisecond,
		30 * time.Millisecond,
		40 * time.Millisecond,
		50 * time.Millisecond,
		60 * time.Millisecond,
		70 * time.Millisecond,
		80 * time.Millisecond,
		90 * time.Millisecond,
		100 * time.Millisecond,
	}

	for _, lat := range latencies {
		monitor.RecordAccess("feature1", lat, true)
	}

	metrics, _ := monitor.GetAccessMetrics("feature1")

	// P50 should be around 50ms, P99 around 100ms
	if metrics.P50Latency < 40*time.Millisecond || metrics.P50Latency > 60*time.Millisecond {
		t.Errorf("Expected P50 around 50ms, got %v", metrics.P50Latency)
	}
	if metrics.P99Latency < 90*time.Millisecond {
		t.Errorf("Expected P99 >= 90ms, got %v", metrics.P99Latency)
	}
}

func TestMonitor_Volatility(t *testing.T) {
	config := DefaultMonitorConfig()
	config.CleanupInterval = 1 * time.Hour
	monitor := NewMonitor(config)

	// Record changes with known volatility
	// Changes: 10, 10, 10 (no variance)
	monitor.RecordChange("stable", 100.0, 110.0)
	monitor.RecordChange("stable", 110.0, 100.0)
	monitor.RecordChange("stable", 100.0, 110.0)

	stableMetrics, _ := monitor.GetChangeMetrics("stable")
	// Volatility should be 0 for identical changes
	if stableMetrics.Volatility > 0.01 {
		t.Errorf("Expected low volatility for stable feature, got %f", stableMetrics.Volatility)
	}

	// Changes: 1, 10, 100 (high variance)
	monitor.RecordChange("volatile", 100.0, 101.0) // 1
	monitor.RecordChange("volatile", 101.0, 111.0) // 10
	monitor.RecordChange("volatile", 111.0, 211.0) // 100

	volatileMetrics, _ := monitor.GetChangeMetrics("volatile")
	// Volatility should be high
	if volatileMetrics.Volatility < 1.0 {
		t.Errorf("Expected high volatility for volatile feature, got %f", volatileMetrics.Volatility)
	}
}

func TestDefaultMonitorConfig(t *testing.T) {
	config := DefaultMonitorConfig()

	if config.WindowSize != 5*time.Minute {
		t.Errorf("Expected window size 5m, got %v", config.WindowSize)
	}
	if config.MaxHistoryEntries != 1000 {
		t.Errorf("Expected max history 1000, got %d", config.MaxHistoryEntries)
	}
	if config.CleanupInterval != 1*time.Minute {
		t.Errorf("Expected cleanup interval 1m, got %v", config.CleanupInterval)
	}
}

func TestHelperFunctions(t *testing.T) {
	// Test abs
	if abs(-5.0) != 5.0 {
		t.Error("abs(-5.0) should be 5.0")
	}
	if abs(5.0) != 5.0 {
		t.Error("abs(5.0) should be 5.0")
	}

	// Test sqrt
	result := sqrt(16.0)
	if result < 3.99 || result > 4.01 {
		t.Errorf("sqrt(16.0) should be ~4.0, got %f", result)
	}

	// Test sortDurations
	durations := []time.Duration{30, 10, 20}
	sortDurations(durations)
	if durations[0] != 10 || durations[1] != 20 || durations[2] != 30 {
		t.Errorf("sortDurations failed: %v", durations)
	}

	// Test percentile
	sorted := []time.Duration{10, 20, 30, 40, 50, 60, 70, 80, 90, 100}
	p50 := percentile(sorted, 50)
	if p50 < 50 || p50 > 60 {
		t.Errorf("percentile(50) should be around 50-60, got %v", p50)
	}
}
