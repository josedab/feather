package lineage

import "fmt"

// ImpactAnalyzer provides impact analysis for feature changes.
type ImpactAnalyzer struct {
	tracker *Tracker
}

// ImpactReport describes the impact of changing a feature.
type ImpactReport struct {
	FeatureID        string           `json:"feature_id"`
	DirectDependents []string         `json:"direct_dependents"`
	AllDependents    []string         `json:"all_dependents"`
	AffectedModels   []string         `json:"affected_models"`
	AffectedSources  []string         `json:"affected_sources"`
	BreakingChanges  []BreakingChange `json:"breaking_changes"`
	RiskLevel        RiskLevel        `json:"risk_level"`
	Summary          string           `json:"summary"`
}

// BreakingChange represents a potential breaking change.
type BreakingChange struct {
	FeatureID string `json:"feature_id"`
	Reason    string `json:"reason"`
	Severity  string `json:"severity"` // "low", "medium", "high", "critical"
}

// RiskLevel categorizes the overall risk.
type RiskLevel string

const (
	RiskLow      RiskLevel = "low"
	RiskMedium   RiskLevel = "medium"
	RiskHigh     RiskLevel = "high"
	RiskCritical RiskLevel = "critical"
)

// NewImpactAnalyzer creates a new ImpactAnalyzer.
func NewImpactAnalyzer(tracker *Tracker) *ImpactAnalyzer {
	return &ImpactAnalyzer{tracker: tracker}
}

// Analyze performs a full impact analysis for a feature.
func (a *ImpactAnalyzer) Analyze(featureID string) (*ImpactReport, error) {
	lineage, err := a.tracker.GetFeatureLineage(featureID)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrFeatureNotFound, featureID)
	}

	graph := a.tracker.GetDependencyGraph()

	report := &ImpactReport{
		FeatureID:        featureID,
		DirectDependents: make([]string, 0),
		AllDependents:    make([]string, 0),
		AffectedModels:   make([]string, 0),
		AffectedSources:  make([]string, 0),
		BreakingChanges:  make([]BreakingChange, 0),
	}

	// Direct dependents
	for _, node := range graph.GetDownstream(featureID) {
		report.DirectDependents = append(report.DirectDependents, node.ID)
	}

	// All transitive dependents
	for _, node := range graph.GetFullDownstream(featureID) {
		report.AllDependents = append(report.AllDependents, node.ID)
	}

	// Affected models from consumers
	for _, cref := range lineage.Consumers {
		report.AffectedModels = append(report.AffectedModels, cref.ConsumerID)
	}
	// Also check transitive downstream consumers
	for _, depID := range report.AllDependents {
		depLineage, err := a.tracker.GetFeatureLineage(depID)
		if err != nil {
			continue
		}
		for _, cref := range depLineage.Consumers {
			if !contains(report.AffectedModels, cref.ConsumerID) {
				report.AffectedModels = append(report.AffectedModels, cref.ConsumerID)
			}
		}
	}

	// Affected sources
	for _, sref := range lineage.Sources {
		report.AffectedSources = append(report.AffectedSources, sref.SourceID)
	}

	// Check for breaking changes: PII propagation to downstream
	if lineage.PIILevel >= PIIHigh {
		for _, depID := range report.AllDependents {
			report.BreakingChanges = append(report.BreakingChanges, BreakingChange{
				FeatureID: depID,
				Reason:    fmt.Sprintf("upstream feature %s has %s PII level", featureID, lineage.PIILevel.String()),
				Severity:  "high",
			})
		}
	}

	report.RiskLevel = a.calculateRiskLevel(report)
	report.Summary = a.generateSummary(report)

	return report, nil
}

// CompareVersions compares impact between two versions of a feature.
func (a *ImpactAnalyzer) CompareVersions(featureID string, oldVersion, newVersion int) (*ImpactReport, error) {
	// Validate the feature exists
	lineage, err := a.tracker.GetFeatureLineage(featureID)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrFeatureNotFound, featureID)
	}

	// Build base impact report
	report, err := a.Analyze(featureID)
	if err != nil {
		return nil, err
	}

	// If version changed, flag all dependents as potentially broken
	if oldVersion != newVersion {
		for _, depID := range report.AllDependents {
			report.BreakingChanges = append(report.BreakingChanges, BreakingChange{
				FeatureID: depID,
				Reason:    fmt.Sprintf("feature %s version changed from %d to %d", featureID, oldVersion, newVersion),
				Severity:  "medium",
			})
		}
	}

	_ = lineage // used for validation above
	report.RiskLevel = a.calculateRiskLevel(report)
	report.Summary = a.generateSummary(report)

	return report, nil
}

func (a *ImpactAnalyzer) calculateRiskLevel(report *ImpactReport) RiskLevel {
	totalDependents := len(report.AllDependents)

	// Check for high-PII dependents
	hasHighPII := false
	hasMediumPII := false
	for _, depID := range report.AllDependents {
		depLineage, err := a.tracker.GetFeatureLineage(depID)
		if err != nil {
			continue
		}
		if depLineage.PIILevel >= PIIHigh {
			hasHighPII = true
		}
		if depLineage.PIILevel >= PIIMedium {
			hasMediumPII = true
		}
	}

	switch {
	case totalDependents > 20 || hasHighPII:
		return RiskCritical
	case totalDependents > 10 || hasMediumPII:
		return RiskHigh
	case totalDependents > 3:
		return RiskMedium
	default:
		return RiskLow
	}
}

func (a *ImpactAnalyzer) generateSummary(report *ImpactReport) string {
	return fmt.Sprintf(
		"Feature %s has %d direct and %d total dependents, %d affected models, risk level: %s",
		report.FeatureID,
		len(report.DirectDependents),
		len(report.AllDependents),
		len(report.AffectedModels),
		report.RiskLevel,
	)
}
