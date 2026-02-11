package autofe

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestOptimizer_RecordFeedback(t *testing.T) {
	opt := NewOptimizer()
	opt.RecordFeedback("feature_a", 0.85)
	opt.RecordFeedback("feature_b", 0.40)

	recs := opt.GetRecommendations()
	assert.NotEmpty(t, recs)
}

func TestOptimizer_GetRecommendations(t *testing.T) {
	tests := []struct {
		name     string
		feedback []struct {
			candidate string
			accuracy  float64
		}
		wantMin int
	}{
		{
			name:     "no feedback",
			feedback: nil,
			wantMin:  1, // the "collect_feedback" recommendation
		},
		{
			name: "low accuracy feature",
			feedback: []struct {
				candidate string
				accuracy  float64
			}{
				{"bad_feature", 0.3},
				{"bad_feature", 0.2},
			},
			wantMin: 1,
		},
		{
			name: "high accuracy feature",
			feedback: []struct {
				candidate string
				accuracy  float64
			}{
				{"good_feature", 0.9},
				{"good_feature", 0.95},
			},
			wantMin: 1,
		},
		{
			name: "mixed features",
			feedback: []struct {
				candidate string
				accuracy  float64
			}{
				{"good", 0.9},
				{"bad", 0.2},
				{"medium", 0.6},
			},
			wantMin: 2, // remove bad + promote good
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opt := NewOptimizer()
			for _, f := range tt.feedback {
				opt.RecordFeedback(f.candidate, f.accuracy)
			}
			recs := opt.GetRecommendations()
			assert.GreaterOrEqual(t, len(recs), tt.wantMin)
			for _, r := range recs {
				assert.NotEmpty(t, r.Action)
				assert.NotEmpty(t, r.Reason)
				assert.Greater(t, r.Priority, 0)
			}
		})
	}
}

func TestOptimizer_PruneUnused(t *testing.T) {
	tests := []struct {
		name      string
		setup     func(o *Optimizer)
		threshold time.Duration
		wantLen   int
	}{
		{
			name:      "no feedback",
			setup:     func(o *Optimizer) {},
			threshold: time.Hour,
			wantLen:   0,
		},
		{
			name: "recent feedback not pruned",
			setup: func(o *Optimizer) {
				o.RecordFeedback("recent", 0.8)
			},
			threshold: time.Hour,
			wantLen:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opt := NewOptimizer()
			tt.setup(opt)
			pruned := opt.PruneUnused(tt.threshold)
			assert.Len(t, pruned, tt.wantLen)
		})
	}
}

func TestOptimizer_RecommendationsSorted(t *testing.T) {
	opt := NewOptimizer()
	opt.RecordFeedback("good", 0.9)
	opt.RecordFeedback("bad", 0.1)

	recs := opt.GetRecommendations()
	for i := 1; i < len(recs); i++ {
		assert.LessOrEqual(t, recs[i-1].Priority, recs[i].Priority,
			"recommendations should be sorted by priority")
	}
}
