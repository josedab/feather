package anomalydetect

import (
	"testing"
	"time"
)

func TestRootCauseAnalyzer_RecordAndCorrelate(t *testing.T) {
	config := DefaultRootCauseAnalyzerConfig()
	config.CorrelationWindow = 1 * time.Second
	config.MinCorrelation = 0.3
	analyzer := NewRootCauseAnalyzer(config)

	now := time.Now()

	// Record correlated anomalies for two features close in time
	for i := 0; i < 5; i++ {
		analyzer.RecordAnomaly(AnomalyResult{
			Feature:   "latency",
			IsAnomaly: true,
			Score:     3.0,
			Timestamp: now.Add(time.Duration(i) * 100 * time.Millisecond),
		})
		analyzer.RecordAnomaly(AnomalyResult{
			Feature:   "error_rate",
			IsAnomaly: true,
			Score:     2.5,
			Timestamp: now.Add(time.Duration(i)*100*time.Millisecond + 50*time.Millisecond),
		})
	}

	correlations := analyzer.AnalyzeCorrelations()
	if len(correlations) == 0 {
		t.Fatal("expected at least one correlation")
	}
	if correlations[0].FeatureA != "error_rate" && correlations[0].FeatureB != "error_rate" {
		t.Error("expected correlation to involve error_rate")
	}
	if correlations[0].Correlation <= 0 {
		t.Error("expected positive correlation")
	}
}

func TestRootCauseAnalyzer_NoCorrelation(t *testing.T) {
	config := DefaultRootCauseAnalyzerConfig()
	config.CorrelationWindow = 1 * time.Millisecond
	analyzer := NewRootCauseAnalyzer(config)

	// Record anomalies far apart in time
	analyzer.RecordAnomaly(AnomalyResult{
		Feature:   "latency",
		IsAnomaly: true,
		Score:     3.0,
		Timestamp: time.Now().Add(-10 * time.Minute),
	})
	analyzer.RecordAnomaly(AnomalyResult{
		Feature:   "throughput",
		IsAnomaly: true,
		Score:     2.0,
		Timestamp: time.Now(),
	})

	correlations := analyzer.AnalyzeCorrelations()
	if len(correlations) != 0 {
		t.Errorf("expected no correlations for distant anomalies, got %d", len(correlations))
	}
}

func TestRootCauseAnalyzer_NonAnomalyIgnored(t *testing.T) {
	analyzer := NewRootCauseAnalyzer(DefaultRootCauseAnalyzerConfig())

	analyzer.RecordAnomaly(AnomalyResult{
		Feature:   "latency",
		IsAnomaly: false,
		Timestamp: time.Now(),
	})

	correlations := analyzer.AnalyzeCorrelations()
	if len(correlations) != 0 {
		t.Errorf("expected no correlations for non-anomalies, got %d", len(correlations))
	}
}

func TestRootCauseAnalyzer_GenerateIncidentReport(t *testing.T) {
	config := DefaultRootCauseAnalyzerConfig()
	config.CorrelationWindow = 1 * time.Second
	analyzer := NewRootCauseAnalyzer(config)

	now := time.Now()
	for i := 0; i < 3; i++ {
		analyzer.RecordAnomaly(AnomalyResult{
			Feature:   "latency",
			IsAnomaly: true,
			Score:     3.0,
			Timestamp: now.Add(time.Duration(i) * 100 * time.Millisecond),
		})
	}

	report, err := analyzer.GenerateIncidentReport("latency")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.PrimaryFeature != "latency" {
		t.Errorf("expected primary feature 'latency', got %q", report.PrimaryFeature)
	}
	if report.TotalAlerts != 3 {
		t.Errorf("expected 3 alerts, got %d", report.TotalAlerts)
	}
	if report.Severity != "low" {
		t.Errorf("expected severity 'low' for 3 alerts, got %q", report.Severity)
	}
}

func TestRootCauseAnalyzer_IncidentSeverity(t *testing.T) {
	tests := []struct {
		name     string
		count    int
		affected int
		expected string
	}{
		{"low", 3, 0, "low"},
		{"medium_by_count", 6, 0, "medium"},
		{"high_by_count", 21, 0, "high"},
		{"critical_by_count", 51, 0, "critical"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := DefaultRootCauseAnalyzerConfig()
			config.CorrelationWindow = 1 * time.Second
			analyzer := NewRootCauseAnalyzer(config)

			now := time.Now()
			for i := 0; i < tt.count; i++ {
				analyzer.RecordAnomaly(AnomalyResult{
					Feature:   "target",
					IsAnomaly: true,
					Score:     3.0,
					Timestamp: now.Add(time.Duration(i) * time.Millisecond),
				})
			}

			report, err := analyzer.GenerateIncidentReport("target")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if report.Severity != tt.expected {
				t.Errorf("expected severity %q, got %q", tt.expected, report.Severity)
			}
		})
	}
}

func TestRootCauseAnalyzer_NoAnomaliesForFeature(t *testing.T) {
	analyzer := NewRootCauseAnalyzer(DefaultRootCauseAnalyzerConfig())

	_, err := analyzer.GenerateIncidentReport("nonexistent")
	if err == nil {
		t.Error("expected error for feature with no anomalies")
	}
}

func TestRootCauseAnalyzer_AlertTrimming(t *testing.T) {
	config := DefaultRootCauseAnalyzerConfig()
	config.MaxIncidentAge = 100 * time.Millisecond
	analyzer := NewRootCauseAnalyzer(config)

	// Record an old anomaly
	analyzer.RecordAnomaly(AnomalyResult{
		Feature:   "latency",
		IsAnomaly: true,
		Score:     3.0,
		Timestamp: time.Now().Add(-1 * time.Hour),
	})

	// Record a fresh anomaly — this triggers trimming
	analyzer.RecordAnomaly(AnomalyResult{
		Feature:   "latency",
		IsAnomaly: true,
		Score:     3.0,
		Timestamp: time.Now(),
	})

	// Old alert should have been trimmed; only 1 remains
	correlations := analyzer.AnalyzeCorrelations()
	_ = correlations // just ensure no panic

	report, err := analyzer.GenerateIncidentReport("latency")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.TotalAlerts != 1 {
		t.Errorf("expected 1 alert after trimming, got %d", report.TotalAlerts)
	}
}

func TestRootCauseAnalyzer_GetReports(t *testing.T) {
	config := DefaultRootCauseAnalyzerConfig()
	analyzer := NewRootCauseAnalyzer(config)

	now := time.Now()
	for _, feature := range []string{"a", "b"} {
		analyzer.RecordAnomaly(AnomalyResult{
			Feature:   feature,
			IsAnomaly: true,
			Score:     3.0,
			Timestamp: now,
		})
		_, _ = analyzer.GenerateIncidentReport(feature)
	}

	reports := analyzer.GetReports(0)
	if len(reports) != 2 {
		t.Errorf("expected 2 reports with limit=0, got %d", len(reports))
	}

	reports = analyzer.GetReports(1)
	if len(reports) != 1 {
		t.Errorf("expected 1 report with limit=1, got %d", len(reports))
	}
}

func TestRootCauseAnalyzer_DefaultConfig(t *testing.T) {
	config := DefaultRootCauseAnalyzerConfig()
	if config.CorrelationWindow != 5*time.Minute {
		t.Errorf("expected 5m correlation window, got %v", config.CorrelationWindow)
	}
	if config.MinCorrelation != 0.3 {
		t.Errorf("expected 0.3 min correlation, got %f", config.MinCorrelation)
	}
	if config.MaxIncidentAge != 1*time.Hour {
		t.Errorf("expected 1h max incident age, got %v", config.MaxIncidentAge)
	}
}
