package validation

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// ValidationReport contains the full report for a set of validation results.
type ValidationReport struct {
	ID              string                `json:"id"`
	Name            string                `json:"name"`
	Summary         *ReportSummary        `json:"summary"`
	FeatureResults  []*FeatureReportEntry `json:"feature_results"`
	Recommendations []string              `json:"recommendations"`
	GeneratedAt     time.Time             `json:"generated_at"`
}

// ReportSummary provides high-level metrics across all validated features.
type ReportSummary struct {
	TotalFeatures     int     `json:"total_features"`
	ConsistentCount   int     `json:"consistent_count"`
	InconsistentCount int     `json:"inconsistent_count"`
	OverallScore      float64 `json:"overall_score"`
	AvgMatchRate      float64 `json:"avg_match_rate"`
}

// FeatureReportEntry contains validation results for a single feature.
type FeatureReportEntry struct {
	Feature      string              `json:"feature"`
	RuleName     string              `json:"rule_name"`
	IsConsistent bool                `json:"is_consistent"`
	Metrics      *ConsistencyMetrics `json:"metrics"`
	SampleSize   int                 `json:"sample_size"`
}

// ValidatorStats exposes aggregate statistics about the validator's state.
type ValidatorStats struct {
	TotalRules      int `json:"total_rules"`
	EnabledRules    int `json:"enabled_rules"`
	TotalResults    int `json:"total_results"`
	TotalReports    int `json:"total_reports"`
	ConsistentCount int `json:"consistent_count"`
	FailedCount     int `json:"failed_count"`
}

// generateReport builds a ValidationReport from a set of results.
func generateReport(_ context.Context, results []*ValidationResult) (*ValidationReport, error) {
	if results == nil {
		return nil, fmt.Errorf("generating report: %w", ErrNoResults)
	}

	report := &ValidationReport{
		ID:          uuid.New().String(),
		Name:        fmt.Sprintf("validation-report-%s", time.Now().Format("20060102-150405")),
		GeneratedAt: time.Now(),
	}

	summary := &ReportSummary{
		TotalFeatures: len(results),
	}

	var totalMatchRate float64
	entries := make([]*FeatureReportEntry, 0, len(results))

	for _, r := range results {
		entry := &FeatureReportEntry{
			Feature:      r.Feature,
			RuleName:     r.RuleName,
			IsConsistent: r.IsConsistent,
			Metrics:      r.Metrics,
			SampleSize:   r.SampleSize,
		}
		entries = append(entries, entry)

		if r.IsConsistent {
			summary.ConsistentCount++
		} else {
			summary.InconsistentCount++
		}

		if r.Metrics != nil {
			totalMatchRate += r.Metrics.ExactMatchRate
		}
	}

	if summary.TotalFeatures > 0 {
		summary.OverallScore = float64(summary.ConsistentCount) / float64(summary.TotalFeatures)
		summary.AvgMatchRate = totalMatchRate / float64(summary.TotalFeatures)
	}

	report.Summary = summary
	report.FeatureResults = entries
	report.Recommendations = buildRecommendations(results)

	return report, nil
}

// buildRecommendations generates actionable suggestions from validation results.
func buildRecommendations(results []*ValidationResult) []string {
	var recs []string

	for _, r := range results {
		if r.IsConsistent {
			continue
		}
		if r.Metrics == nil {
			continue
		}
		if r.Metrics.MaxAbsError > 1.0 {
			recs = append(recs, fmt.Sprintf("Feature %q has high max absolute error (%.4f); investigate pipeline lag or transformation bugs", r.Feature, r.Metrics.MaxAbsError))
		}
		if r.Metrics.ExactMatchRate < 0.5 {
			recs = append(recs, fmt.Sprintf("Feature %q has low exact match rate (%.2f%%); check for type coercion or rounding differences", r.Feature, r.Metrics.ExactMatchRate*100))
		}
		if r.Metrics.KSPValue > 0 && r.Metrics.KSPValue < 0.01 {
			recs = append(recs, fmt.Sprintf("Feature %q shows significant distribution drift (KS p-value=%.4f); consider retraining or recalibrating", r.Feature, r.Metrics.KSPValue))
		}
	}

	if len(recs) == 0 {
		recs = append(recs, "All validated features are within acceptable thresholds")
	}

	return recs
}
