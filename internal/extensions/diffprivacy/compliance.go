package diffprivacy

import (
	"fmt"
	"time"
)

// ComplianceFramework identifies the privacy regulation.
type ComplianceFramework string

const (
	FrameworkGDPR  ComplianceFramework = "GDPR"
	FrameworkCCPA  ComplianceFramework = "CCPA"
	FrameworkHIPAA ComplianceFramework = "HIPAA"
)

// ComplianceReport contains a GDPR/CCPA compliance assessment.
type ComplianceReport struct {
	Framework       ComplianceFramework `json:"framework"`
	GeneratedAt     time.Time           `json:"generated_at"`
	Period          ReportPeriod        `json:"period"`
	Summary         ComplianceSummary   `json:"summary"`
	FeatureDetails  []FeatureCompliance `json:"feature_details"`
	Recommendations []string            `json:"recommendations"`
}

// ReportPeriod defines the time range of a compliance report.
type ReportPeriod struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

// ComplianceSummary provides high-level compliance metrics.
type ComplianceSummary struct {
	TotalFeatures       int     `json:"total_features"`
	ProtectedFeatures   int     `json:"protected_features"`
	UnprotectedFeatures int     `json:"unprotected_features"`
	TotalQueries        int64   `json:"total_queries"`
	RejectedQueries     int64   `json:"rejected_queries"`
	AvgEpsilonUsed      float64 `json:"avg_epsilon_used"`
	MaxEpsilonUsed      float64 `json:"max_epsilon_used"`
	ComplianceScore     float64 `json:"compliance_score"` // 0-100
	Status              string  `json:"status"`           // "compliant", "at_risk", "non_compliant"
}

// FeatureCompliance describes the privacy posture of a single feature.
type FeatureCompliance struct {
	Feature         string  `json:"feature"`
	EntityType      string  `json:"entity_type"`
	Mechanism       string  `json:"mechanism"`
	EpsilonBudget   float64 `json:"epsilon_budget"`
	EpsilonConsumed float64 `json:"epsilon_consumed"`
	QueryCount      int64   `json:"query_count"`
	HasBudget       bool    `json:"has_budget"`
	Status          string  `json:"status"` // "compliant", "warning", "violation"
}

// ComplianceReporter generates privacy compliance reports.
type ComplianceReporter struct {
	engine        *Engine
	budgetManager *BudgetManager
}

// NewComplianceReporter creates a new compliance reporter.
func NewComplianceReporter(engine *Engine, budgetMgr *BudgetManager) *ComplianceReporter {
	return &ComplianceReporter{
		engine:        engine,
		budgetManager: budgetMgr,
	}
}

// GenerateReport creates a compliance report for the specified framework.
func (r *ComplianceReporter) GenerateReport(framework ComplianceFramework, period ReportPeriod) *ComplianceReport {
	report := &ComplianceReport{
		Framework:   framework,
		GeneratedAt: time.Now(),
		Period:      period,
	}

	// Collect feature details from engine
	engineStats := r.engine.Stats()
	r.engine.mu.RLock()
	type snapshotState struct {
		mechanism       Mechanism
		maxBudget       float64
		consumedEpsilon float64
		queryCount      int64
	}
	features := make(map[string]snapshotState)
	for k, v := range r.engine.features {
		features[k] = snapshotState{
			mechanism:       v.config.Mechanism,
			maxBudget:       v.config.MaxBudget,
			consumedEpsilon: v.consumedEpsilon,
			queryCount:      v.queryCount,
		}
	}
	r.engine.mu.RUnlock()

	summary := ComplianceSummary{
		TotalFeatures: engineStats.RegisteredFeatures,
		TotalQueries:  engineStats.TotalQueries,
	}

	var maxEpsilon float64
	var totalEpsilon float64
	epsilonCount := 0

	for name, fs := range features {
		fc := FeatureCompliance{
			Feature:         name,
			Mechanism:       string(fs.mechanism),
			EpsilonBudget:   fs.maxBudget,
			EpsilonConsumed: fs.consumedEpsilon,
			QueryCount:      fs.queryCount,
			HasBudget:       fs.maxBudget > 0,
		}

		if fs.consumedEpsilon > maxEpsilon {
			maxEpsilon = fs.consumedEpsilon
		}
		totalEpsilon += fs.consumedEpsilon
		epsilonCount++

		// Determine compliance status
		if fs.maxBudget <= 0 {
			fc.Status = "violation"
			summary.UnprotectedFeatures++
		} else if fs.consumedEpsilon >= fs.maxBudget*0.9 {
			fc.Status = "warning"
			summary.ProtectedFeatures++
		} else {
			fc.Status = "compliant"
			summary.ProtectedFeatures++
		}

		report.FeatureDetails = append(report.FeatureDetails, fc)
	}

	// Add budget manager data if available
	if r.budgetManager != nil {
		budgets := r.budgetManager.ListBudgets()
		for _, b := range budgets {
			// Check if we already have this feature
			found := false
			for i, fd := range report.FeatureDetails {
				if fd.Feature == b.Key.Feature {
					report.FeatureDetails[i].EntityType = b.Key.EntityType
					found = true
					break
				}
			}
			if !found {
				report.FeatureDetails = append(report.FeatureDetails, FeatureCompliance{
					Feature:         b.Key.Feature,
					EntityType:      b.Key.EntityType,
					EpsilonBudget:   b.MaxEpsilon,
					EpsilonConsumed: b.ConsumedEpsilon,
					QueryCount:      b.QueryCount,
					HasBudget:       true,
					Status:          "compliant",
				})
			}
		}

		bmStats := r.budgetManager.Stats()
		summary.RejectedQueries = bmStats.RejectedQueries
	}

	if epsilonCount > 0 {
		summary.AvgEpsilonUsed = totalEpsilon / float64(epsilonCount)
	}
	summary.MaxEpsilonUsed = maxEpsilon

	// Calculate compliance score
	if summary.TotalFeatures > 0 {
		protectedRatio := float64(summary.ProtectedFeatures) / float64(summary.TotalFeatures)
		budgetHealthRatio := 1.0
		if maxEpsilon > 0 {
			budgetHealthRatio = 1.0 - (summary.AvgEpsilonUsed / maxEpsilon)
			if budgetHealthRatio < 0 {
				budgetHealthRatio = 0
			}
		}
		summary.ComplianceScore = (protectedRatio*70 + budgetHealthRatio*30)
	}

	if summary.ComplianceScore >= 80 {
		summary.Status = "compliant"
	} else if summary.ComplianceScore >= 50 {
		summary.Status = "at_risk"
	} else {
		summary.Status = "non_compliant"
	}

	report.Summary = summary

	// Generate framework-specific recommendations
	report.Recommendations = r.generateRecommendations(framework, summary)

	return report
}

func (r *ComplianceReporter) generateRecommendations(framework ComplianceFramework, summary ComplianceSummary) []string {
	var recs []string

	if summary.UnprotectedFeatures > 0 {
		recs = append(recs, fmt.Sprintf("Register privacy budgets for %d unprotected features", summary.UnprotectedFeatures))
	}

	if summary.MaxEpsilonUsed > 5.0 {
		recs = append(recs, "Consider lowering epsilon values for high-consumption features")
	}

	switch framework {
	case FrameworkGDPR:
		recs = append(recs, "Ensure data subject access requests include privacy budget usage")
		if summary.ComplianceScore < 80 {
			recs = append(recs, "Review Data Protection Impact Assessment (DPIA) for features with high epsilon usage")
		}
	case FrameworkCCPA:
		recs = append(recs, "Document privacy mechanisms in consumer privacy notice")
		if summary.UnprotectedFeatures > 0 {
			recs = append(recs, "Apply differential privacy to all consumer-facing features")
		}
	case FrameworkHIPAA:
		recs = append(recs, "Verify de-identification meets Safe Harbor or Expert Determination requirements")
	}

	if summary.RejectedQueries > 0 {
		recs = append(recs, fmt.Sprintf("Review %d rejected queries for potential budget allocation adjustments", summary.RejectedQueries))
	}

	return recs
}
