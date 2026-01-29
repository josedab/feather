package costopt

import (
	"sort"
	"sync"
	"time"
)

// AnalyzerConfig controls how access patterns are analyzed.
type AnalyzerConfig struct {
	AnalysisWindow     time.Duration
	MinSamples         int
	HotThresholdQPS    float64
	ColdThresholdHours float64
}

// DefaultAnalyzerConfig returns sensible defaults.
func DefaultAnalyzerConfig() AnalyzerConfig {
	return AnalyzerConfig{
		AnalysisWindow:     24 * time.Hour,
		MinSamples:         100,
		HotThresholdQPS:    10,
		ColdThresholdHours: 168,
	}
}

// AccessPattern describes observed access behaviour for a feature group.
type AccessPattern struct {
	FeatureGroup   string        `json:"feature_group"`
	Entity         string        `json:"entity"`
	AccessCount    int64         `json:"access_count"`
	LastAccess     time.Time     `json:"last_access"`
	AvgLatency     time.Duration `json:"avg_latency_ns"`
	P99Latency     time.Duration `json:"p99_latency_ns"`
	ReadWriteRatio float64       `json:"read_write_ratio"`
	CurrentTier    string        `json:"current_tier"`
}

// accessEvent is a single recorded access.
type accessEvent struct {
	featureGroup string
	entity       string
	tier         string
	latency      time.Duration
	isWrite      bool
	timestamp    time.Time
}

// Analyzer collects access events and derives access patterns.
type Analyzer struct {
	mu     sync.RWMutex
	config AnalyzerConfig
	events []accessEvent
}

// NewAnalyzer creates an Analyzer with the given configuration.
func NewAnalyzer(config AnalyzerConfig) *Analyzer {
	return &Analyzer{
		config: config,
		events: make([]accessEvent, 0, 1024),
	}
}

// RecordAccess records a single access event.
func (a *Analyzer) RecordAccess(featureGroup, entity, tier string, latency time.Duration, isWrite bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.events = append(a.events, accessEvent{
		featureGroup: featureGroup,
		entity:       entity,
		tier:         tier,
		latency:      latency,
		isWrite:      isWrite,
		timestamp:    time.Now(),
	})
}

// GetPattern returns the computed access pattern for a single feature group.
func (a *Analyzer) GetPattern(featureGroup string) *AccessPattern {
	patterns := a.computePatterns()
	if p, ok := patterns[featureGroup]; ok {
		return p
	}
	return nil
}

// ListPatterns returns all patterns sorted by access count descending.
func (a *Analyzer) ListPatterns() []*AccessPattern {
	patterns := a.computePatterns()
	out := make([]*AccessPattern, 0, len(patterns))
	for _, p := range patterns {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].AccessCount > out[j].AccessCount
	})
	return out
}

// AnalyzeAll computes and returns all patterns (alias for ListPatterns).
func (a *Analyzer) AnalyzeAll() []*AccessPattern {
	return a.ListPatterns()
}

func (a *Analyzer) computePatterns() map[string]*AccessPattern {
	a.mu.RLock()
	defer a.mu.RUnlock()

	cutoff := time.Now().Add(-a.config.AnalysisWindow)

	type groupStats struct {
		entity     string
		tier       string
		count      int64
		reads      int64
		writes     int64
		lastAccess time.Time
		latencies  []time.Duration
	}

	groups := make(map[string]*groupStats)
	for _, e := range a.events {
		if e.timestamp.Before(cutoff) {
			continue
		}
		gs, ok := groups[e.featureGroup]
		if !ok {
			gs = &groupStats{entity: e.entity, tier: e.tier}
			groups[e.featureGroup] = gs
		}
		gs.count++
		if e.isWrite {
			gs.writes++
		} else {
			gs.reads++
		}
		if e.timestamp.After(gs.lastAccess) {
			gs.lastAccess = e.timestamp
			gs.entity = e.entity
			gs.tier = e.tier
		}
		gs.latencies = append(gs.latencies, e.latency)
	}

	patterns := make(map[string]*AccessPattern, len(groups))
	for fg, gs := range groups {
		var totalLat time.Duration
		for _, l := range gs.latencies {
			totalLat += l
		}
		avgLat := time.Duration(0)
		if gs.count > 0 {
			avgLat = totalLat / time.Duration(gs.count)
		}

		p99Lat := computeP99(gs.latencies)

		rwRatio := float64(0)
		if gs.writes > 0 {
			rwRatio = float64(gs.reads) / float64(gs.writes)
		} else if gs.reads > 0 {
			rwRatio = float64(gs.reads)
		}

		patterns[fg] = &AccessPattern{
			FeatureGroup:   fg,
			Entity:         gs.entity,
			AccessCount:    gs.count,
			LastAccess:     gs.lastAccess,
			AvgLatency:     avgLat,
			P99Latency:     p99Lat,
			ReadWriteRatio: rwRatio,
			CurrentTier:    gs.tier,
		}
	}
	return patterns
}

func computeP99(latencies []time.Duration) time.Duration {
	if len(latencies) == 0 {
		return 0
	}
	sorted := make([]time.Duration, len(latencies))
	copy(sorted, latencies)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	idx := int(float64(len(sorted)) * 0.99)
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}
