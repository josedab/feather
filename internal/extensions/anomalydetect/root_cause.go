package anomalydetect

import (
	"fmt"
	"sort"
	"sync"
	"time"
)

// CorrelationResult represents a correlation between two features' anomalies.
type CorrelationResult struct {
	FeatureA      string  `json:"feature_a"`
	FeatureB      string  `json:"feature_b"`
	Correlation   float64 `json:"correlation"`
	CoOccurrences int     `json:"co_occurrences"`
	TimeWindowMs  int64   `json:"time_window_ms"`
}

// RootCause describes a potential root cause for an anomaly.
type RootCause struct {
	Feature       string    `json:"feature"`
	Confidence    float64   `json:"confidence"`
	Reason        string    `json:"reason"`
	RelatedAlerts int       `json:"related_alerts"`
	FirstSeen     time.Time `json:"first_seen"`
}

// IncidentReport summarizes an anomaly incident.
type IncidentReport struct {
	ID               string              `json:"id"`
	PrimaryFeature   string              `json:"primary_feature"`
	AffectedFeatures []string            `json:"affected_features"`
	RootCauses       []RootCause         `json:"root_causes"`
	Correlations     []CorrelationResult `json:"correlations"`
	TotalAlerts      int                 `json:"total_alerts"`
	Severity         string              `json:"severity"`
	StartTime        time.Time           `json:"start_time"`
	EndTime          time.Time           `json:"end_time"`
	GeneratedAt      time.Time           `json:"generated_at"`
}

// RootCauseAnalyzerConfig configures the root cause analyzer.
type RootCauseAnalyzerConfig struct {
	CorrelationWindow time.Duration `json:"correlation_window" yaml:"correlation_window"`
	MinCorrelation    float64       `json:"min_correlation" yaml:"min_correlation"`
	MaxIncidentAge    time.Duration `json:"max_incident_age" yaml:"max_incident_age"`
}

// DefaultRootCauseAnalyzerConfig returns sensible defaults.
func DefaultRootCauseAnalyzerConfig() RootCauseAnalyzerConfig {
	return RootCauseAnalyzerConfig{
		CorrelationWindow: 5 * time.Minute,
		MinCorrelation:    0.3,
		MaxIncidentAge:    1 * time.Hour,
	}
}

// RootCauseAnalyzer correlates anomalies across features to identify root causes.
type RootCauseAnalyzer struct {
	mu      sync.RWMutex
	config  RootCauseAnalyzerConfig
	alerts  []AnomalyResult
	reports []IncidentReport
	nextID  int
}

// NewRootCauseAnalyzer creates a new root cause analyzer.
func NewRootCauseAnalyzer(config RootCauseAnalyzerConfig) *RootCauseAnalyzer {
	return &RootCauseAnalyzer{
		config: config,
	}
}

// RecordAnomaly records an anomaly for correlation analysis.
func (a *RootCauseAnalyzer) RecordAnomaly(result AnomalyResult) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.alerts = append(a.alerts, result)

	// Trim old alerts
	cutoff := time.Now().Add(-a.config.MaxIncidentAge)
	var trimmed []AnomalyResult
	for _, alert := range a.alerts {
		if alert.Timestamp.After(cutoff) {
			trimmed = append(trimmed, alert)
		}
	}
	a.alerts = trimmed
}

// AnalyzeCorrelations finds correlated anomalies within the time window.
func (a *RootCauseAnalyzer) AnalyzeCorrelations() []CorrelationResult {
	a.mu.RLock()
	defer a.mu.RUnlock()

	// Group alerts by feature
	byFeature := make(map[string][]AnomalyResult)
	for _, alert := range a.alerts {
		if alert.IsAnomaly {
			byFeature[alert.Feature] = append(byFeature[alert.Feature], alert)
		}
	}

	features := make([]string, 0, len(byFeature))
	for f := range byFeature {
		features = append(features, f)
	}
	sort.Strings(features)

	var results []CorrelationResult

	for i := 0; i < len(features); i++ {
		for j := i + 1; j < len(features); j++ {
			fA, fB := features[i], features[j]
			coOccurrences := 0

			for _, alertA := range byFeature[fA] {
				for _, alertB := range byFeature[fB] {
					diff := alertA.Timestamp.Sub(alertB.Timestamp)
					if diff < 0 {
						diff = -diff
					}
					if diff <= a.config.CorrelationWindow {
						coOccurrences++
					}
				}
			}

			if coOccurrences == 0 {
				continue
			}

			// Simple correlation: co-occurrences / max(count_a, count_b)
			maxCount := len(byFeature[fA])
			if len(byFeature[fB]) > maxCount {
				maxCount = len(byFeature[fB])
			}
			correlation := float64(coOccurrences) / float64(maxCount)

			if correlation >= a.config.MinCorrelation {
				results = append(results, CorrelationResult{
					FeatureA:      fA,
					FeatureB:      fB,
					Correlation:   correlation,
					CoOccurrences: coOccurrences,
					TimeWindowMs:  a.config.CorrelationWindow.Milliseconds(),
				})
			}
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Correlation > results[j].Correlation
	})

	return results
}

// GenerateIncidentReport creates an incident report for a feature's anomalies.
func (a *RootCauseAnalyzer) GenerateIncidentReport(feature string) (*IncidentReport, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Find alerts for this feature
	var featureAlerts []AnomalyResult
	for _, alert := range a.alerts {
		if alert.Feature == feature && alert.IsAnomaly {
			featureAlerts = append(featureAlerts, alert)
		}
	}

	if len(featureAlerts) == 0 {
		return nil, fmt.Errorf("no anomalies found for feature %s", feature)
	}

	a.nextID++
	report := &IncidentReport{
		ID:             fmt.Sprintf("incident-%d", a.nextID),
		PrimaryFeature: feature,
		TotalAlerts:    len(featureAlerts),
		StartTime:      featureAlerts[0].Timestamp,
		EndTime:        featureAlerts[len(featureAlerts)-1].Timestamp,
		GeneratedAt:    time.Now(),
	}

	// Find correlated features
	correlatedFeatures := make(map[string]int)
	for _, alert := range a.alerts {
		if alert.Feature == feature || !alert.IsAnomaly {
			continue
		}
		for _, fa := range featureAlerts {
			diff := alert.Timestamp.Sub(fa.Timestamp)
			if diff < 0 {
				diff = -diff
			}
			if diff <= a.config.CorrelationWindow {
				correlatedFeatures[alert.Feature]++
			}
		}
	}

	// Build root causes from correlated features
	for f, count := range correlatedFeatures {
		confidence := float64(count) / float64(len(featureAlerts))
		if confidence > 1 {
			confidence = 1
		}
		report.RootCauses = append(report.RootCauses, RootCause{
			Feature:       f,
			Confidence:    confidence,
			Reason:        fmt.Sprintf("correlated anomalies (%d co-occurrences)", count),
			RelatedAlerts: count,
			FirstSeen:     featureAlerts[0].Timestamp,
		})
		report.AffectedFeatures = append(report.AffectedFeatures, f)
	}

	// Sort root causes by confidence
	sort.Slice(report.RootCauses, func(i, j int) bool {
		return report.RootCauses[i].Confidence > report.RootCauses[j].Confidence
	})

	// Determine severity
	switch {
	case len(featureAlerts) > 50 || len(report.AffectedFeatures) > 5:
		report.Severity = "critical"
	case len(featureAlerts) > 20 || len(report.AffectedFeatures) > 3:
		report.Severity = "high"
	case len(featureAlerts) > 5 || len(report.AffectedFeatures) > 1:
		report.Severity = "medium"
	default:
		report.Severity = "low"
	}

	a.reports = append(a.reports, *report)
	return report, nil
}

// GetReports returns generated incident reports.
func (a *RootCauseAnalyzer) GetReports(limit int) []IncidentReport {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if limit <= 0 || limit > len(a.reports) {
		limit = len(a.reports)
	}
	start := len(a.reports) - limit
	result := make([]IncidentReport, limit)
	copy(result, a.reports[start:])
	return result
}
