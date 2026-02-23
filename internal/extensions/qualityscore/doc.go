// Package qualityscore provides a multi-signal quality scoring system for features.
//
// It computes composite quality scores based on weighted signals including
// usage frequency, stability, freshness, completeness, uniqueness, and drift.
// Scores are used by the semantic search ranker and feature marketplace to
// surface high-quality features.
//
// Usage:
//
//	scorer := qualityscore.NewScorer(qualityscore.DefaultConfig())
//	scorer.RecordSignal("user_click_count", qualityscore.Signal{
//	    Type:  qualityscore.SignalUsage,
//	    Value: 0.95,
//	})
//	score := scorer.ComputeScore("user_click_count")
package qualityscore
