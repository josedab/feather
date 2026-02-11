package autofe

import (
	"fmt"
	"sort"
	"sync"
	"time"
)

// Recommendation represents an optimization recommendation.
type Recommendation struct {
	Action          string  `json:"action"`
	Reason          string  `json:"reason"`
	Priority        int     `json:"priority"` // 1 = highest
	EstimatedImpact float64 `json:"estimated_impact"`
}

// feedbackEntry records a single feedback observation.
type feedbackEntry struct {
	CandidateName string    `json:"candidate_name"`
	ModelAccuracy float64   `json:"model_accuracy"`
	RecordedAt    time.Time `json:"recorded_at"`
}

// Optimizer provides a continuous learning feedback loop for feature candidates.
type Optimizer struct {
	mu       sync.RWMutex
	feedback []feedbackEntry
}

// NewOptimizer creates a new Optimizer.
func NewOptimizer() *Optimizer {
	return &Optimizer{}
}

// RecordFeedback records model performance for a candidate feature.
func (o *Optimizer) RecordFeedback(candidateName string, modelAccuracy float64) {
	o.mu.Lock()
	defer o.mu.Unlock()

	o.feedback = append(o.feedback, feedbackEntry{
		CandidateName: candidateName,
		ModelAccuracy: modelAccuracy,
		RecordedAt:    time.Now(),
	})
}

// GetRecommendations analyzes collected feedback and returns optimization recommendations.
func (o *Optimizer) GetRecommendations() []*Recommendation {
	o.mu.RLock()
	defer o.mu.RUnlock()

	if len(o.feedback) == 0 {
		return []*Recommendation{{
			Action:          "collect_feedback",
			Reason:          "No feedback recorded yet — start recording model accuracy for features",
			Priority:        1,
			EstimatedImpact: 0,
		}}
	}

	// Aggregate accuracy per candidate.
	type aggStats struct {
		totalAccuracy float64
		count         int
		lastSeen      time.Time
	}
	perCandidate := make(map[string]*aggStats)
	for _, f := range o.feedback {
		s, ok := perCandidate[f.CandidateName]
		if !ok {
			s = &aggStats{}
			perCandidate[f.CandidateName] = s
		}
		s.totalAccuracy += f.ModelAccuracy
		s.count++
		if f.RecordedAt.After(s.lastSeen) {
			s.lastSeen = f.RecordedAt
		}
	}

	var recs []*Recommendation

	for name, stats := range perCandidate {
		avgAccuracy := stats.totalAccuracy / float64(stats.count)

		if avgAccuracy < 0.5 {
			recs = append(recs, &Recommendation{
				Action:          fmt.Sprintf("remove_%s", name),
				Reason:          fmt.Sprintf("Feature %q has low average accuracy (%.2f) — consider removing", name, avgAccuracy),
				Priority:        1,
				EstimatedImpact: 0.5 - avgAccuracy,
			})
		} else if avgAccuracy > 0.8 {
			recs = append(recs, &Recommendation{
				Action:          fmt.Sprintf("promote_%s", name),
				Reason:          fmt.Sprintf("Feature %q has high average accuracy (%.2f) — promote to production", name, avgAccuracy),
				Priority:        2,
				EstimatedImpact: avgAccuracy - 0.8,
			})
		}
	}

	// Sort by priority then by estimated impact descending.
	sort.Slice(recs, func(i, j int) bool {
		if recs[i].Priority != recs[j].Priority {
			return recs[i].Priority < recs[j].Priority
		}
		return recs[i].EstimatedImpact > recs[j].EstimatedImpact
	})

	return recs
}

// PruneUnused identifies features that have not received feedback within the threshold duration.
func (o *Optimizer) PruneUnused(threshold time.Duration) []string {
	o.mu.RLock()
	defer o.mu.RUnlock()

	lastSeen := make(map[string]time.Time)
	for _, f := range o.feedback {
		if t, ok := lastSeen[f.CandidateName]; !ok || f.RecordedAt.After(t) {
			lastSeen[f.CandidateName] = f.RecordedAt
		}
	}

	cutoff := time.Now().Add(-threshold)
	var unused []string
	for name, t := range lastSeen {
		if t.Before(cutoff) {
			unused = append(unused, name)
		}
	}

	sort.Strings(unused)
	return unused
}
