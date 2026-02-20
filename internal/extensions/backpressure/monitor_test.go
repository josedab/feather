package backpressure

import (
	"testing"
)

func TestNewMonitor(t *testing.T) {
	m := NewMonitor(DefaultMonitorConfig())
	if m == nil {
		t.Fatal("NewMonitor returned nil")
	}

	if m.currentLevel != None {
		t.Errorf("expected initial level None, got %s", m.currentLevel)
	}

	// Zero config should use defaults
	m2 := NewMonitor(MonitorConfig{})
	if m2.config.LatencyThresholdMs != 100 {
		t.Errorf("expected default LatencyThresholdMs 100, got %f", m2.config.LatencyThresholdMs)
	}
}

func TestNormalLoad(t *testing.T) {
	m := NewMonitor(MonitorConfig{
		QueueHighWatermark: 0.8,
		LatencyThresholdMs: 100,
		ErrorRateThreshold: 0.05,
		MaxSamples:         1000,
		CooldownPeriod:     0, // Disable cooldown for testing
	})

	// Record low values
	for i := 0; i < 20; i++ {
		m.RecordQueueDepth(0.1)
		m.RecordLatency(5)
		m.RecordErrorRate(0.001)
	}

	report := m.Evaluate()

	if report.Level != None {
		t.Errorf("expected None level under normal load, got %s", report.Level)
	}

	if m.GetCurrentLevel() != None {
		t.Errorf("GetCurrentLevel should be None, got %s", m.GetCurrentLevel())
	}
}

func TestHighLoad(t *testing.T) {
	m := NewMonitor(MonitorConfig{
		QueueHighWatermark: 0.8,
		LatencyThresholdMs: 100,
		ErrorRateThreshold: 0.05,
		MaxSamples:         1000,
		CooldownPeriod:     0,
	})

	// Record high queue depth and latency
	for i := 0; i < 20; i++ {
		m.RecordQueueDepth(0.95)
		m.RecordLatency(500)
		m.RecordErrorRate(0.1)
	}

	report := m.Evaluate()

	if report.Level != Critical {
		t.Errorf("expected Critical level under high load, got %s", report.Level)
	}

	if report.QueueDepth < 0.9 {
		t.Errorf("expected high queue depth, got %f", report.QueueDepth)
	}

	if report.LatencyP99 < 400 {
		t.Errorf("expected high latency, got %f", report.LatencyP99)
	}
}

func TestScaleRecommendation(t *testing.T) {
	m := NewMonitor(MonitorConfig{
		QueueHighWatermark: 0.8,
		LatencyThresholdMs: 100,
		ErrorRateThreshold: 0.05,
		MaxSamples:         1000,
		CooldownPeriod:     0,
	})

	// High load should recommend scale up
	for i := 0; i < 20; i++ {
		m.RecordQueueDepth(0.95)
		m.RecordLatency(500)
		m.RecordErrorRate(0.1)
	}

	report := m.Evaluate()

	if report.ScaleAction != "scale_up" {
		t.Errorf("expected scale_up action, got %s", report.ScaleAction)
	}

	if report.SuggestedReplicas <= 0 {
		t.Errorf("expected positive suggested replicas, got %d", report.SuggestedReplicas)
	}

	if report.Recommendation == "" {
		t.Error("expected non-empty recommendation")
	}
}

func TestStats(t *testing.T) {
	m := NewMonitor(MonitorConfig{
		QueueHighWatermark: 0.8,
		LatencyThresholdMs: 100,
		ErrorRateThreshold: 0.05,
		MaxSamples:         1000,
		CooldownPeriod:     0,
	})

	m.RecordQueueDepth(0.1)
	m.RecordLatency(5)
	m.RecordErrorRate(0.001)
	m.Evaluate()

	stats := m.Stats()

	if stats.TotalSamples != 3 {
		t.Errorf("expected 3 total samples, got %d", stats.TotalSamples)
	}

	if stats.TotalEvaluations != 1 {
		t.Errorf("expected 1 evaluation, got %d", stats.TotalEvaluations)
	}

	if stats.CurrentLevel != string(None) {
		t.Errorf("expected current level 'none', got %s", stats.CurrentLevel)
	}

	// Verify reports
	reports := m.GetReports(10)
	if len(reports) != 1 {
		t.Errorf("expected 1 report, got %d", len(reports))
	}
}

func TestLowPressure(t *testing.T) {
	m := NewMonitor(MonitorConfig{
		QueueHighWatermark: 0.8,
		LatencyThresholdMs: 100,
		ErrorRateThreshold: 0.05,
		MaxSamples:         1000,
		CooldownPeriod:     0,
	})

	// Record moderately elevated queue depth only
	for i := 0; i < 20; i++ {
		m.RecordQueueDepth(0.85)
		m.RecordLatency(10)
		m.RecordErrorRate(0.001)
	}

	report := m.Evaluate()
	if report.Level == None {
		t.Error("expected some pressure level when queue is above watermark")
	}
}

func TestSampleCapping(t *testing.T) {
	m := NewMonitor(MonitorConfig{
		QueueHighWatermark: 0.8,
		LatencyThresholdMs: 100,
		ErrorRateThreshold: 0.05,
		MaxSamples:         5,
		CooldownPeriod:     0,
	})

	// Record more samples than MaxSamples
	for i := 0; i < 10; i++ {
		m.RecordQueueDepth(0.1)
		m.RecordLatency(5)
		m.RecordErrorRate(0.001)
	}

	stats := m.Stats()
	if stats.TotalSamples != 30 {
		t.Errorf("expected 30 total samples recorded, got %d", stats.TotalSamples)
	}
}

func TestGetReportsLimit(t *testing.T) {
	m := NewMonitor(MonitorConfig{
		QueueHighWatermark: 0.8,
		LatencyThresholdMs: 100,
		ErrorRateThreshold: 0.05,
		MaxSamples:         1000,
		CooldownPeriod:     0,
	})

	// Generate multiple reports
	for i := 0; i < 5; i++ {
		m.RecordQueueDepth(0.5)
		m.Evaluate()
	}

	// Get limited reports
	reports := m.GetReports(3)
	if len(reports) != 3 {
		t.Errorf("expected 3 reports (limited), got %d", len(reports))
	}

	// Get all reports
	allReports := m.GetReports(100)
	if len(allReports) != 5 {
		t.Errorf("expected 5 total reports, got %d", len(allReports))
	}
}

func TestEvaluateWithNoSamples(t *testing.T) {
	m := NewMonitor(MonitorConfig{
		QueueHighWatermark: 0.8,
		LatencyThresholdMs: 100,
		ErrorRateThreshold: 0.05,
		MaxSamples:         1000,
		CooldownPeriod:     0,
	})

	report := m.Evaluate()
	if report.Level != None {
		t.Errorf("expected None level with no samples, got %s", report.Level)
	}
}
