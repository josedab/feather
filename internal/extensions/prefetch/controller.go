// Package prefetch provides a predictive feature pre-fetching controller
// that learns co-access patterns and temporal signals to proactively warm
// features before they are requested, reducing tail latency.
package prefetch

import (
	"fmt"
	"math"
	"sync"
	"sync/atomic"
	"time"
)

// Config configures the predictive prefetch controller.
type Config struct {
	MaxMemoryBudgetMB int     // maximum memory budget for prefetch candidates
	PredictionThreshold float64 // minimum score to trigger a prefetch
	MaxPrefetchBatch  int     // maximum features per prefetch batch
	PatternWindowSize int     // sliding window size for pattern tracking
	DecayFactor       float64 // exponential decay applied to older observations
	MinCoAccessCount  int     // minimum co-access count before a pair is considered
}

// DefaultConfig returns production defaults for the prefetch controller.
func DefaultConfig() Config {
	return Config{
		MaxMemoryBudgetMB:   512,
		PredictionThreshold: 0.7,
		MaxPrefetchBatch:    100,
		PatternWindowSize:   1000,
		DecayFactor:         0.95,
		MinCoAccessCount:    3,
	}
}

// PrefetchCandidate represents a single feature predicted for prefetching.
type PrefetchCandidate struct {
	Feature    string  `json:"feature"`
	Score      float64 `json:"score"`
	Confidence float64 `json:"confidence"`
	Reason     string  `json:"reason"`
}

// PrefetchPlan is the set of candidates selected for a given entity.
type PrefetchPlan struct {
	Candidates     []PrefetchCandidate `json:"candidates"`
	MemoryEstimate int64               `json:"memory_estimate"` // bytes
	Priority       string              `json:"priority"`        // "high", "medium", "low"
}

// Stats holds runtime statistics for the controller.
type Stats struct {
	TotalAccesses    int64   `json:"total_accesses"`
	PatternsTracked  int64   `json:"patterns_tracked"`
	PrefetchesIssued int64   `json:"prefetches_issued"`
	HitRate          float64 `json:"hit_rate"`
	MemoryUsedBytes  int64   `json:"memory_used_bytes"`
}

// coAccessKey is the ordered pair used as a map key for co-access tracking.
type coAccessKey struct {
	a, b string
}

// coAccessEntry stores the decayed frequency for a co-access pair.
type coAccessEntry struct {
	count     float64
	lastSeen  time.Time
}

// accessRecord stores a single access event for temporal analysis.
type accessRecord struct {
	entityKey string
	features  []string
	timestamp time.Time
	hour      int // 0-23, for time-of-day patterns
}

// Controller is a predictive feature pre-fetching engine that tracks
// access patterns and produces prefetch plans for entity keys.
type Controller struct {
	config Config

	// co-access matrix: tracks how often feature pairs are fetched together
	coAccess map[coAccessKey]*coAccessEntry

	// per-entity recent feature history (bounded by PatternWindowSize)
	entityHistory map[string][]string

	// per-feature hourly access counts for temporal patterns
	hourlyAccess map[string][24]float64

	// sliding window of recent accesses
	window []accessRecord

	// counters
	totalAccesses    atomic.Int64
	patternsTracked  atomic.Int64
	prefetchesIssued atomic.Int64
	prefetchHits     atomic.Int64
	prefetchTotal    atomic.Int64

	mu sync.RWMutex
}

// NewController creates a new prefetch controller with the given config.
func NewController(cfg Config) *Controller {
	return &Controller{
		config:        cfg,
		coAccess:      make(map[coAccessKey]*coAccessEntry),
		entityHistory: make(map[string][]string),
		hourlyAccess:  make(map[string][24]float64),
		window:        make([]accessRecord, 0, cfg.PatternWindowSize),
	}
}

// RecordAccess records a co-access event for the given entity and features.
// It updates the co-access matrix, temporal patterns, and entity history.
func (c *Controller) RecordAccess(entityKey string, features []string) {
	if len(features) == 0 {
		return
	}

	now := time.Now()
	hour := now.Hour()
	c.totalAccesses.Add(1)

	c.mu.Lock()
	defer c.mu.Unlock()

	// Update co-access matrix for every pair of features.
	for i := 0; i < len(features); i++ {
		for j := i + 1; j < len(features); j++ {
			key := makeCoAccessKey(features[i], features[j])
			entry, ok := c.coAccess[key]
			if !ok {
				entry = &coAccessEntry{}
				c.coAccess[key] = entry
				c.patternsTracked.Add(1)
			}
			// Apply decay based on elapsed time, then increment.
			elapsed := now.Sub(entry.lastSeen).Hours()
			entry.count = entry.count*math.Pow(c.config.DecayFactor, elapsed) + 1
			entry.lastSeen = now
		}
	}

	// Update temporal (hourly) patterns for each feature.
	for _, f := range features {
		h := c.hourlyAccess[f]
		h[hour]++
		c.hourlyAccess[f] = h
	}

	// Update per-entity history (bounded ring).
	history := c.entityHistory[entityKey]
	history = append(history, features...)
	if len(history) > c.config.PatternWindowSize {
		history = history[len(history)-c.config.PatternWindowSize:]
	}
	c.entityHistory[entityKey] = history

	// Append to sliding window.
	rec := accessRecord{
		entityKey: entityKey,
		features:  features,
		timestamp: now,
		hour:      hour,
	}
	if len(c.window) >= c.config.PatternWindowSize {
		c.window = c.window[1:]
	}
	c.window = append(c.window, rec)
}

// Predict returns scored prefetch candidates for the given entity based on
// co-access frequency, temporal patterns, and recency.
func (c *Controller) Predict(entityKey string) []PrefetchCandidate {
	c.mu.RLock()
	defer c.mu.RUnlock()

	history := c.entityHistory[entityKey]
	if len(history) == 0 {
		return nil
	}

	now := time.Now()
	hour := now.Hour()

	// Collect the set of recently accessed features for this entity.
	recentSet := make(map[string]struct{})
	for _, f := range history {
		recentSet[f] = struct{}{}
	}

	// Score every candidate feature that co-occurs with any recent feature.
	type scored struct {
		feature    string
		coScore    float64
		tempScore  float64
		recency    float64
		coCount    float64
		bestReason string
	}
	candidates := make(map[string]*scored)

	for pair, entry := range c.coAccess {
		if entry.count < float64(c.config.MinCoAccessCount) {
			continue
		}
		var target, source string
		if _, ok := recentSet[pair.a]; ok {
			target, source = pair.b, pair.a
		} else if _, ok := recentSet[pair.b]; ok {
			target, source = pair.a, pair.b
		} else {
			continue
		}
		// Skip features already in the recent set.
		if _, ok := recentSet[target]; ok {
			continue
		}

		elapsed := now.Sub(entry.lastSeen).Hours()
		decayed := entry.count * math.Pow(c.config.DecayFactor, elapsed)

		sc, ok := candidates[target]
		if !ok {
			sc = &scored{feature: target}
			candidates[target] = sc
		}
		if decayed > sc.coScore {
			sc.coScore = decayed
			sc.coCount = entry.count
			sc.bestReason = fmt.Sprintf("co-accessed with %s (%.0fx)", source, entry.count)
		}
	}

	// Enrich candidates with temporal and recency signals.
	for _, sc := range candidates {
		// Temporal score: fraction of accesses in this hour vs. total.
		h := c.hourlyAccess[sc.feature]
		var total float64
		for _, v := range h {
			total += v
		}
		if total > 0 {
			sc.tempScore = h[hour] / total
		}

		// Recency: check how recently this feature appeared in the window.
		sc.recency = c.recencyScore(sc.feature, now)
	}

	// Combine signals into a final score and filter.
	const (
		wCoAccess  = 0.50
		wTemporal  = 0.25
		wRecency   = 0.25
	)

	var result []PrefetchCandidate
	for _, sc := range candidates {
		// Normalize co-access score to [0,1] via sigmoid.
		normCo := sigmoid(sc.coScore, 10)
		score := wCoAccess*normCo + wTemporal*sc.tempScore + wRecency*sc.recency

		if score < c.config.PredictionThreshold {
			continue
		}

		confidence := math.Min(1.0, sc.coCount/20)
		result = append(result, PrefetchCandidate{
			Feature:    sc.feature,
			Score:      math.Round(score*1000) / 1000,
			Confidence: math.Round(confidence*1000) / 1000,
			Reason:     sc.bestReason,
		})
	}

	// Sort by score descending (insertion sort for small N).
	for i := 1; i < len(result); i++ {
		for j := i; j > 0 && result[j].Score > result[j-1].Score; j-- {
			result[j], result[j-1] = result[j-1], result[j]
		}
	}

	if len(result) > c.config.MaxPrefetchBatch {
		result = result[:c.config.MaxPrefetchBatch]
	}

	return result
}

// GetPrefetchPlan produces a memory-aware prefetch plan for the entity.
func (c *Controller) GetPrefetchPlan(entityKey string) *PrefetchPlan {
	candidates := c.Predict(entityKey)
	if len(candidates) == 0 {
		return &PrefetchPlan{
			Priority: "low",
		}
	}

	c.prefetchesIssued.Add(1)

	const estimatedBytesPerFeature int64 = 256
	budgetBytes := int64(c.config.MaxMemoryBudgetMB) * 1024 * 1024
	var memEstimate int64
	var selected []PrefetchCandidate

	for _, cand := range candidates {
		cost := estimatedBytesPerFeature
		if memEstimate+cost > budgetBytes {
			break
		}
		memEstimate += cost
		selected = append(selected, cand)
	}

	priority := "low"
	if len(selected) > 0 {
		avg := avgScore(selected)
		switch {
		case avg >= 0.9:
			priority = "high"
		case avg >= 0.75:
			priority = "medium"
		}
	}

	return &PrefetchPlan{
		Candidates:     selected,
		MemoryEstimate: memEstimate,
		Priority:       priority,
	}
}

// RecordPrefetchResult records whether a prefetched feature was actually used,
// allowing the controller to track hit rate.
func (c *Controller) RecordPrefetchResult(hit bool) {
	c.prefetchTotal.Add(1)
	if hit {
		c.prefetchHits.Add(1)
	}
}

// Stats returns runtime statistics for the controller.
func (c *Controller) Stats() Stats {
	total := c.prefetchTotal.Load()
	var hitRate float64
	if total > 0 {
		hitRate = float64(c.prefetchHits.Load()) / float64(total)
	}

	c.mu.RLock()
	memUsed := c.estimateMemoryLocked()
	c.mu.RUnlock()

	return Stats{
		TotalAccesses:    c.totalAccesses.Load(),
		PatternsTracked:  c.patternsTracked.Load(),
		PrefetchesIssued: c.prefetchesIssued.Load(),
		HitRate:          math.Round(hitRate*1000) / 1000,
		MemoryUsedBytes:  memUsed,
	}
}

// estimateMemoryLocked returns a rough byte estimate of internal data structures.
// Caller must hold at least a read lock.
func (c *Controller) estimateMemoryLocked() int64 {
	const (
		coAccessEntrySize = 64
		historyEntrySize  = 48
		windowEntrySize   = 80
		hourlyEntrySize   = 24 * 8
	)

	var total int64
	total += int64(len(c.coAccess)) * coAccessEntrySize
	for _, h := range c.entityHistory {
		total += int64(len(h)) * historyEntrySize
	}
	total += int64(len(c.window)) * windowEntrySize
	total += int64(len(c.hourlyAccess)) * hourlyEntrySize
	return total
}

// recencyScore returns a [0,1] score based on how recently a feature was seen
// in the sliding window. Caller must hold at least a read lock.
func (c *Controller) recencyScore(feature string, now time.Time) float64 {
	for i := len(c.window) - 1; i >= 0; i-- {
		for _, f := range c.window[i].features {
			if f == feature {
				age := now.Sub(c.window[i].timestamp).Minutes()
				return math.Exp(-age / 60) // half-life ~1 hour
			}
		}
	}
	return 0
}

// makeCoAccessKey returns a deterministic ordered key for a feature pair.
func makeCoAccessKey(a, b string) coAccessKey {
	if a > b {
		a, b = b, a
	}
	return coAccessKey{a: a, b: b}
}

// sigmoid maps x to (0,1) with a midpoint at mid.
func sigmoid(x, mid float64) float64 {
	return 1 / (1 + math.Exp(-(x-mid)/mid))
}

// avgScore computes the mean score of a candidate slice.
func avgScore(cs []PrefetchCandidate) float64 {
	if len(cs) == 0 {
		return 0
	}
	var sum float64
	for _, c := range cs {
		sum += c.Score
	}
	return sum / float64(len(cs))
}
