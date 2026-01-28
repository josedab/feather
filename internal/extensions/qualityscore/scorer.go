package qualityscore

import (
	"fmt"
	"math"
	"sort"
	"sync"
	"time"
)

// SignalType represents the type of quality signal.
type SignalType string

const (
	SignalUsage        SignalType = "usage"
	SignalStability    SignalType = "stability"
	SignalFreshness    SignalType = "freshness"
	SignalCompleteness SignalType = "completeness"
	SignalUniqueness   SignalType = "uniqueness"
	SignalDrift        SignalType = "drift"
)

// Signal represents a quality measurement for a feature.
type Signal struct {
	Type      SignalType `json:"type"`
	Value     float64    `json:"value"`
	Weight    float64    `json:"weight"`
	Timestamp time.Time  `json:"timestamp"`
}

// FeatureScore represents the computed quality score for a feature.
type FeatureScore struct {
	FeatureID       string                 `json:"feature_id"`
	OverallScore    float64                `json:"overall_score"`
	Signals         map[SignalType]*Signal `json:"signals"`
	Grade           string                 `json:"grade"`
	Recommendations []string               `json:"recommendations"`
	ScoredAt        time.Time              `json:"scored_at"`
}

// ScoringConfig holds weights and thresholds for scoring.
type ScoringConfig struct {
	Weights         map[SignalType]float64 `json:"weights"`
	GradeThresholds GradeThresholds        `json:"grade_thresholds"`
}

// GradeThresholds defines minimum scores for each grade.
type GradeThresholds struct {
	A float64 `json:"a"`
	B float64 `json:"b"`
	C float64 `json:"c"`
	D float64 `json:"d"`
}

// DeprecationCandidate represents a feature that may be deprecated.
type DeprecationCandidate struct {
	FeatureID     string    `json:"feature_id"`
	Score         float64   `json:"score"`
	Reason        string    `json:"reason"`
	LastUsed      time.Time `json:"last_used"`
	StorageCostGB float64   `json:"storage_cost_gb"`
}

// ScorerStats provides summary statistics about scoring.
type ScorerStats struct {
	FeaturesScored        int     `json:"features_scored"`
	DeprecationCandidates int     `json:"deprecation_candidates"`
	AverageScore          float64 `json:"average_score"`
	StorageSavingsGB      float64 `json:"storage_savings_gb"`
}

// DefaultScoringConfig returns the default scoring configuration.
func DefaultScoringConfig() ScoringConfig {
	return ScoringConfig{
		Weights: map[SignalType]float64{
			SignalUsage:        0.30,
			SignalStability:    0.20,
			SignalFreshness:    0.20,
			SignalCompleteness: 0.15,
			SignalUniqueness:   0.10,
			SignalDrift:        0.05,
		},
		GradeThresholds: GradeThresholds{
			A: 0.9,
			B: 0.75,
			C: 0.6,
			D: 0.4,
		},
	}
}

// Scorer computes automated quality scores for features.
type Scorer struct {
	mu      sync.RWMutex
	config  ScoringConfig
	scores  map[string]*FeatureScore
	signals map[string]map[SignalType]*Signal
}

// NewScorer creates a new Scorer with the given configuration.
func NewScorer(cfg ScoringConfig) *Scorer {
	return &Scorer{
		config:  cfg,
		scores:  make(map[string]*FeatureScore),
		signals: make(map[string]map[SignalType]*Signal),
	}
}

// RecordSignal records a quality signal for a feature.
func (s *Scorer) RecordSignal(featureID string, signal *Signal) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.signals[featureID]; !ok {
		s.signals[featureID] = make(map[SignalType]*Signal)
	}
	s.signals[featureID][signal.Type] = signal
}

// Score computes the quality score for a single feature.
func (s *Scorer) Score(featureID string) (*FeatureScore, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	signals, ok := s.signals[featureID]
	if !ok {
		return nil, fmt.Errorf("no signals recorded for feature %q", featureID)
	}

	score := s.computeScore(featureID, signals)
	s.scores[featureID] = score
	return score, nil
}

// ScoreAll computes quality scores for all tracked features.
func (s *Scorer) ScoreAll() []*FeatureScore {
	s.mu.Lock()
	defer s.mu.Unlock()

	results := make([]*FeatureScore, 0, len(s.signals))
	for featureID, signals := range s.signals {
		score := s.computeScore(featureID, signals)
		s.scores[featureID] = score
		results = append(results, score)
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].OverallScore > results[j].OverallScore
	})
	return results
}

// GetScore returns the cached score for a feature.
func (s *Scorer) GetScore(featureID string) (*FeatureScore, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	score, ok := s.scores[featureID]
	if !ok {
		return nil, fmt.Errorf("no score computed for feature %q", featureID)
	}
	return score, nil
}

// TopN returns the top N highest-scoring features.
func (s *Scorer) TopN(n int) []*FeatureScore {
	s.mu.RLock()
	defer s.mu.RUnlock()

	all := s.sortedScores()
	if n > len(all) {
		n = len(all)
	}
	return all[:n]
}

// BottomN returns the bottom N lowest-scoring features.
func (s *Scorer) BottomN(n int) []*FeatureScore {
	s.mu.RLock()
	defer s.mu.RUnlock()

	all := s.sortedScores()
	if n > len(all) {
		n = len(all)
	}
	// Return from the end (lowest scores)
	return all[len(all)-n:]
}

// GetDeprecationCandidates returns features scoring below the threshold.
func (s *Scorer) GetDeprecationCandidates(threshold float64) []*DeprecationCandidate {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var candidates []*DeprecationCandidate
	for featureID, score := range s.scores {
		if score.OverallScore < threshold {
			reason := fmt.Sprintf("score %.2f below threshold %.2f", score.OverallScore, threshold)
			candidates = append(candidates, &DeprecationCandidate{
				FeatureID:     featureID,
				Score:         score.OverallScore,
				Reason:        reason,
				LastUsed:      score.ScoredAt,
				StorageCostGB: 0.01, // placeholder estimate
			})
		}
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Score < candidates[j].Score
	})
	return candidates
}

// Stats returns summary statistics about the scorer.
func (s *Scorer) Stats() *ScorerStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := &ScorerStats{
		FeaturesScored: len(s.scores),
	}

	if len(s.scores) == 0 {
		return stats
	}

	var totalScore float64
	for _, score := range s.scores {
		totalScore += score.OverallScore
		if score.OverallScore < 0.4 {
			stats.DeprecationCandidates++
			stats.StorageSavingsGB += 0.01
		}
	}
	stats.AverageScore = totalScore / float64(len(s.scores))
	return stats
}

// computeScore calculates the weighted score from signals (must be called with lock held).
func (s *Scorer) computeScore(featureID string, signals map[SignalType]*Signal) *FeatureScore {
	var weightedSum, totalWeight float64
	for signalType, weight := range s.config.Weights {
		sig, ok := signals[signalType]
		if !ok {
			continue
		}
		value := math.Max(0, math.Min(1, sig.Value))
		weightedSum += value * weight
		totalWeight += weight
	}

	overall := 0.0
	if totalWeight > 0 {
		overall = weightedSum / totalWeight
	}

	grade := s.computeGrade(overall)
	recommendations := s.generateRecommendations(signals)

	return &FeatureScore{
		FeatureID:       featureID,
		OverallScore:    overall,
		Signals:         signals,
		Grade:           grade,
		Recommendations: recommendations,
		ScoredAt:        time.Now(),
	}
}

// computeGrade returns the letter grade for a score.
func (s *Scorer) computeGrade(score float64) string {
	t := s.config.GradeThresholds
	switch {
	case score >= t.A:
		return "A"
	case score >= t.B:
		return "B"
	case score >= t.C:
		return "C"
	case score >= t.D:
		return "D"
	default:
		return "F"
	}
}

// generateRecommendations suggests improvements based on lowest signals.
func (s *Scorer) generateRecommendations(signals map[SignalType]*Signal) []string {
	type ranked struct {
		t SignalType
		v float64
	}
	var items []ranked
	for t, sig := range signals {
		items = append(items, ranked{t, sig.Value})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].v < items[j].v })

	var recs []string
	for _, item := range items {
		if item.v >= 0.7 {
			break
		}
		switch item.t {
		case SignalUsage:
			recs = append(recs, "Increase feature usage or consider deprecation")
		case SignalStability:
			recs = append(recs, "Investigate value instability and add validation")
		case SignalFreshness:
			recs = append(recs, "Update feature data pipeline for fresher data")
		case SignalCompleteness:
			recs = append(recs, "Reduce null/missing values in feature data")
		case SignalUniqueness:
			recs = append(recs, "Check for duplicate or highly correlated features")
		case SignalDrift:
			recs = append(recs, "Investigate distribution drift from baseline")
		}
	}
	return recs
}

// sortedScores returns all scores sorted descending by overall score (must be called with read lock held).
func (s *Scorer) sortedScores() []*FeatureScore {
	all := make([]*FeatureScore, 0, len(s.scores))
	for _, score := range s.scores {
		all = append(all, score)
	}
	sort.Slice(all, func(i, j int) bool {
		return all[i].OverallScore > all[j].OverallScore
	})
	return all
}
