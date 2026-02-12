package costopt

import (
	"fmt"
	"sync"
	"time"
)

// RecommendationType identifies the kind of optimization.
type RecommendationType string

const (
	RecommendTierMove    RecommendationType = "tier_move"
	RecommendTTLAdjust   RecommendationType = "ttl_adjust"
	RecommendCompression RecommendationType = "compression"
	RecommendEviction    RecommendationType = "eviction"
	RecommendScaling     RecommendationType = "scaling"
)

// Recommendation represents a single cost optimization suggestion.
type Recommendation struct {
	ID                  string             `json:"id"`
	Type                RecommendationType `json:"type"`
	FeatureGroup        string             `json:"feature_group"`
	CurrentState        string             `json:"current_state"`
	RecommendedState    string             `json:"recommended_state"`
	Reason              string             `json:"reason"`
	EstimatedSavingsPct float64            `json:"estimated_savings_pct"`
	EstimatedSavingsUSD float64            `json:"estimated_savings_usd"`
	Risk                string             `json:"risk"`
	Priority            int                `json:"priority"`
	CreatedAt           time.Time          `json:"created_at"`
	Applied             bool               `json:"applied"`
	Dismissed           bool               `json:"dismissed"`
}

// RecommenderConfig controls recommendation generation.
type RecommenderConfig struct {
	MinSavingsThreshold float64
	MaxRiskLevel        string
	AutoApplyLowRisk    bool
}

// DefaultRecommenderConfig returns sensible defaults.
func DefaultRecommenderConfig() RecommenderConfig {
	return RecommenderConfig{
		MinSavingsThreshold: 5.0,
		MaxRiskLevel:        "medium",
		AutoApplyLowRisk:    false,
	}
}

// RecommenderStats summarises recommendation outcomes.
type RecommenderStats struct {
	TotalRecommendations int     `json:"total_recommendations"`
	Applied              int     `json:"applied"`
	Dismissed            int     `json:"dismissed"`
	Pending              int     `json:"pending"`
	EstimatedTotalSavings float64 `json:"estimated_total_savings"`
}

// Recommender generates cost optimization recommendations from access patterns.
type Recommender struct {
	mu       sync.RWMutex
	analyzer *Analyzer
	config   RecommenderConfig
	recs     map[string]*Recommendation
	nextID   int
}

// NewRecommender creates a Recommender backed by the given Analyzer.
func NewRecommender(analyzer *Analyzer, config RecommenderConfig) *Recommender {
	return &Recommender{
		analyzer: analyzer,
		config:   config,
		recs:     make(map[string]*Recommendation),
	}
}

// GenerateRecommendations analyses current access patterns and produces recommendations.
func (r *Recommender) GenerateRecommendations() []*Recommendation {
	patterns := r.analyzer.AnalyzeAll()
	now := time.Now()

	var generated []*Recommendation
	for _, p := range patterns {
		recs := r.recommendForPattern(p, now)
		generated = append(generated, recs...)
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	for _, rec := range generated {
		r.recs[rec.ID] = rec
	}
	return generated
}

func (r *Recommender) recommendForPattern(p *AccessPattern, now time.Time) []*Recommendation {
	var recs []*Recommendation
	cfg := r.analyzer.config

	hoursSinceAccess := now.Sub(p.LastAccess).Hours()

	// Low-access features → move to warm tier
	if p.CurrentTier == "hot" && p.AccessCount < int64(cfg.MinSamples) {
		recs = append(recs, r.newRec(RecommendTierMove, p.FeatureGroup,
			"hot", "warm",
			"Low access count suggests warm tier is sufficient",
			15, 0.50, "low", 5))
	}

	// Features not accessed beyond cold threshold → compress or evict
	if hoursSinceAccess >= cfg.ColdThresholdHours {
		recs = append(recs, r.newRec(RecommendEviction, p.FeatureGroup,
			p.CurrentTier, "evicted",
			fmt.Sprintf("No access for %.0f hours exceeds cold threshold", hoursSinceAccess),
			25, 1.00, "medium", 7))
		recs = append(recs, r.newRec(RecommendCompression, p.FeatureGroup,
			"uncompressed", "compressed",
			"Compressing cold data reduces storage cost",
			10, 0.30, "low", 4))
	}

	// High P99 latency → recommend scaling
	if p.P99Latency > 50*time.Millisecond {
		recs = append(recs, r.newRec(RecommendScaling, p.FeatureGroup,
			"current", "scaled",
			fmt.Sprintf("P99 latency of %s exceeds 50ms threshold", p.P99Latency),
			0, 0, "medium", 8))
	}

	// High-access features in warm tier → pre-warm to hot
	windowSec := cfg.AnalysisWindow.Seconds()
	if windowSec > 0 {
		qps := float64(p.AccessCount) / windowSec
		if qps >= cfg.HotThresholdQPS && p.CurrentTier == "warm" {
			recs = append(recs, r.newRec(RecommendTierMove, p.FeatureGroup,
				"warm", "hot",
				fmt.Sprintf("QPS %.1f exceeds hot threshold; pre-warm recommended", qps),
				0, 0, "low", 9))
		}
	}

	return recs
}

func (r *Recommender) newRec(typ RecommendationType, fg, cur, rec, reason string, savPct, savUSD float64, risk string, pri int) *Recommendation {
	r.nextID++
	return &Recommendation{
		ID:                  fmt.Sprintf("rec-%d", r.nextID),
		Type:                typ,
		FeatureGroup:        fg,
		CurrentState:        cur,
		RecommendedState:    rec,
		Reason:              reason,
		EstimatedSavingsPct: savPct,
		EstimatedSavingsUSD: savUSD,
		Risk:                risk,
		Priority:            pri,
		CreatedAt:           time.Now(),
	}
}

// GetRecommendation returns a recommendation by ID.
func (r *Recommender) GetRecommendation(id string) (*Recommendation, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	rec, ok := r.recs[id]
	if !ok {
		return nil, fmt.Errorf("recommendation %q not found", id)
	}
	return rec, nil
}

// ApplyRecommendation marks a recommendation as applied.
func (r *Recommender) ApplyRecommendation(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	rec, ok := r.recs[id]
	if !ok {
		return fmt.Errorf("recommendation %q not found", id)
	}
	rec.Applied = true
	rec.Dismissed = false
	return nil
}

// DismissRecommendation marks a recommendation as dismissed.
func (r *Recommender) DismissRecommendation(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	rec, ok := r.recs[id]
	if !ok {
		return fmt.Errorf("recommendation %q not found", id)
	}
	rec.Dismissed = true
	rec.Applied = false
	return nil
}

// Stats returns summary statistics for all recommendations.
func (r *Recommender) Stats() *RecommenderStats {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s := &RecommenderStats{}
	for _, rec := range r.recs {
		s.TotalRecommendations++
		switch {
		case rec.Applied:
			s.Applied++
			s.EstimatedTotalSavings += rec.EstimatedSavingsUSD
		case rec.Dismissed:
			s.Dismissed++
		default:
			s.Pending++
		}
	}
	return s
}
