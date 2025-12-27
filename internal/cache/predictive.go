package cache

import (
	"container/heap"
	"context"
	"math"
	"sort"
	"sync"
	"time"

	"github.com/feather-store/feather/internal/storage"
)

// AccessPattern represents the access pattern for a feature.
type AccessPattern struct {
	Feature       string    `json:"feature"`
	EntityID      string    `json:"entity_id"`
	AccessCount   int64     `json:"access_count"`
	LastAccess    time.Time `json:"last_access"`
	AvgInterval   float64   `json:"avg_interval_ms"`  // Average time between accesses
	PredictedNext time.Time `json:"predicted_next"`
	Score         float64   `json:"score"`            // Priority score for caching
}

// PatternTracker tracks access patterns for features.
type PatternTracker struct {
	patterns     map[string]*AccessPattern // key: entityID:feature
	accessTimes  map[string][]time.Time    // Recent access times
	maxHistory   int
	mu           sync.RWMutex
}

// NewPatternTracker creates a new pattern tracker.
func NewPatternTracker(maxHistory int) *PatternTracker {
	if maxHistory <= 0 {
		maxHistory = 100
	}
	return &PatternTracker{
		patterns:    make(map[string]*AccessPattern),
		accessTimes: make(map[string][]time.Time),
		maxHistory:  maxHistory,
	}
}

// RecordAccess records a feature access.
func (t *PatternTracker) RecordAccess(entityID, feature string) {
	key := entityID + ":" + feature
	now := time.Now()

	t.mu.Lock()
	defer t.mu.Unlock()

	// Update access times
	times := t.accessTimes[key]
	times = append(times, now)
	if len(times) > t.maxHistory {
		times = times[1:]
	}
	t.accessTimes[key] = times

	// Update pattern
	pattern, ok := t.patterns[key]
	if !ok {
		pattern = &AccessPattern{
			Feature:   feature,
			EntityID:  entityID,
		}
		t.patterns[key] = pattern
	}

	pattern.AccessCount++
	pattern.LastAccess = now

	// Calculate average interval
	if len(times) > 1 {
		var totalInterval float64
		for i := 1; i < len(times); i++ {
			interval := times[i].Sub(times[i-1]).Milliseconds()
			totalInterval += float64(interval)
		}
		pattern.AvgInterval = totalInterval / float64(len(times)-1)

		// Predict next access
		pattern.PredictedNext = now.Add(time.Duration(pattern.AvgInterval) * time.Millisecond)
	}

	// Calculate score based on frequency and recency
	pattern.Score = t.calculateScore(pattern, now)
}

func (t *PatternTracker) calculateScore(pattern *AccessPattern, now time.Time) float64 {
	// Decay factor for recency (half-life of 1 hour)
	age := now.Sub(pattern.LastAccess).Hours()
	recencyScore := math.Exp(-age * 0.693) // ln(2) ≈ 0.693

	// Frequency score (logarithmic)
	frequencyScore := math.Log10(float64(pattern.AccessCount) + 1)

	// Regularity score (lower interval variance = more predictable)
	regularityScore := 1.0
	if pattern.AvgInterval > 0 {
		regularityScore = 1.0 / (1.0 + math.Log10(pattern.AvgInterval+1))
	}

	return (recencyScore * 0.4) + (frequencyScore * 0.4) + (regularityScore * 0.2)
}

// GetTopPatterns returns the top N patterns by score.
func (t *PatternTracker) GetTopPatterns(n int) []*AccessPattern {
	t.mu.RLock()
	defer t.mu.RUnlock()

	patterns := make([]*AccessPattern, 0, len(t.patterns))
	for _, p := range t.patterns {
		patterns = append(patterns, p)
	}

	sort.Slice(patterns, func(i, j int) bool {
		return patterns[i].Score > patterns[j].Score
	})

	if len(patterns) > n {
		patterns = patterns[:n]
	}

	return patterns
}

// GetPredictedAccesses returns patterns predicted to be accessed soon.
func (t *PatternTracker) GetPredictedAccesses(window time.Duration) []*AccessPattern {
	t.mu.RLock()
	defer t.mu.RUnlock()

	now := time.Now()
	deadline := now.Add(window)

	var predicted []*AccessPattern
	for _, p := range t.patterns {
		if !p.PredictedNext.IsZero() && p.PredictedNext.Before(deadline) && p.PredictedNext.After(now) {
			predicted = append(predicted, p)
		}
	}

	// Sort by predicted time
	sort.Slice(predicted, func(i, j int) bool {
		return predicted[i].PredictedNext.Before(predicted[j].PredictedNext)
	})

	return predicted
}

// GetPattern returns the pattern for a specific entity/feature.
func (t *PatternTracker) GetPattern(entityID, feature string) *AccessPattern {
	key := entityID + ":" + feature
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.patterns[key]
}

// PredictiveCache provides intelligent cache warming.
type PredictiveCache struct {
	store         *storage.Store
	tracker       *PatternTracker
	warmingQueue  *warmingQueue
	config        PredictiveCacheConfig
	stopCh        chan struct{}
	mu            sync.RWMutex
}

// PredictiveCacheConfig configures the predictive cache.
type PredictiveCacheConfig struct {
	WarmingWindow    time.Duration // How far ahead to predict
	WarmingInterval  time.Duration // How often to run warming
	MaxWarmItems     int           // Maximum items to warm per cycle
	MinScore         float64       // Minimum score to consider for warming
	Enabled          bool
}

// DefaultPredictiveCacheConfig returns default configuration.
func DefaultPredictiveCacheConfig() PredictiveCacheConfig {
	return PredictiveCacheConfig{
		WarmingWindow:   5 * time.Minute,
		WarmingInterval: 30 * time.Second,
		MaxWarmItems:    100,
		MinScore:        0.1,
		Enabled:         true,
	}
}

// NewPredictiveCache creates a new predictive cache.
func NewPredictiveCache(store *storage.Store, config PredictiveCacheConfig) *PredictiveCache {
	return &PredictiveCache{
		store:        store,
		tracker:      NewPatternTracker(100),
		warmingQueue: newWarmingQueue(),
		config:       config,
		stopCh:       make(chan struct{}),
	}
}

// RecordAccess records a feature access for pattern learning.
func (c *PredictiveCache) RecordAccess(entityID, feature string) {
	c.tracker.RecordAccess(entityID, feature)
}

// GetTopPatterns returns the top N patterns by score.
func (c *PredictiveCache) GetTopPatterns(n int) []*AccessPattern {
	return c.tracker.GetTopPatterns(n)
}

// GetPattern returns the pattern for a specific entity/feature.
func (c *PredictiveCache) GetPattern(entityID, feature string) *AccessPattern {
	return c.tracker.GetPattern(entityID, feature)
}

// GetPredictedAccesses returns patterns predicted to be accessed soon.
func (c *PredictiveCache) GetPredictedAccesses(window time.Duration) []*AccessPattern {
	return c.tracker.GetPredictedAccesses(window)
}

// Start starts the background warming process.
func (c *PredictiveCache) Start(ctx context.Context) {
	if !c.config.Enabled {
		return
	}

	go func() {
		ticker := time.NewTicker(c.config.WarmingInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-c.stopCh:
				return
			case <-ticker.C:
				c.warmCache(ctx)
			}
		}
	}()
}

// Stop stops the background warming process.
func (c *PredictiveCache) Stop() {
	close(c.stopCh)
}

func (c *PredictiveCache) warmCache(ctx context.Context) {
	// Get predicted accesses
	predicted := c.tracker.GetPredictedAccesses(c.config.WarmingWindow)

	// Also include top patterns by score
	topPatterns := c.tracker.GetTopPatterns(c.config.MaxWarmItems)

	// Merge and deduplicate
	toWarm := make(map[string]*AccessPattern)

	for _, p := range predicted {
		if p.Score >= c.config.MinScore {
			key := p.EntityID + ":" + p.Feature
			toWarm[key] = p
		}
	}

	for _, p := range topPatterns {
		if len(toWarm) >= c.config.MaxWarmItems {
			break
		}
		key := p.EntityID + ":" + p.Feature
		if _, ok := toWarm[key]; !ok && p.Score >= c.config.MinScore {
			toWarm[key] = p
		}
	}

	// Warm the cache
	for _, p := range toWarm {
		select {
		case <-ctx.Done():
			return
		default:
			// Access the feature to warm it in cache
			c.store.Get(p.EntityID, []string{p.Feature})
		}
	}
}

// GetStats returns cache warming statistics.
func (c *PredictiveCache) GetStats() *PredictiveCacheStats {
	patterns := c.tracker.GetTopPatterns(1000)

	var totalAccess int64
	var avgScore float64
	for _, p := range patterns {
		totalAccess += p.AccessCount
		avgScore += p.Score
	}
	if len(patterns) > 0 {
		avgScore /= float64(len(patterns))
	}

	predicted := c.tracker.GetPredictedAccesses(c.config.WarmingWindow)

	return &PredictiveCacheStats{
		TrackedPatterns:    len(patterns),
		TotalAccesses:      totalAccess,
		AverageScore:       avgScore,
		PredictedNextHour:  len(predicted),
		WarmingEnabled:     c.config.Enabled,
		WarmingWindow:      c.config.WarmingWindow,
		WarmingInterval:    c.config.WarmingInterval,
		MaxWarmItems:       c.config.MaxWarmItems,
	}
}

// PredictiveCacheStats contains cache statistics.
type PredictiveCacheStats struct {
	TrackedPatterns   int           `json:"tracked_patterns"`
	TotalAccesses     int64         `json:"total_accesses"`
	AverageScore      float64       `json:"average_score"`
	PredictedNextHour int           `json:"predicted_next_hour"`
	WarmingEnabled    bool          `json:"warming_enabled"`
	WarmingWindow     time.Duration `json:"warming_window"`
	WarmingInterval   time.Duration `json:"warming_interval"`
	MaxWarmItems      int           `json:"max_warm_items"`
}

// warmingItem represents an item in the warming queue.
type warmingItem struct {
	entityID string
	feature  string
	priority float64
	index    int
}

// warmingQueue is a priority queue for cache warming.
type warmingQueue struct {
	items []*warmingItem
	mu    sync.Mutex
}

func newWarmingQueue() *warmingQueue {
	q := &warmingQueue{
		items: make([]*warmingItem, 0),
	}
	heap.Init(q)
	return q
}

func (q *warmingQueue) Len() int { return len(q.items) }

func (q *warmingQueue) Less(i, j int) bool {
	return q.items[i].priority > q.items[j].priority
}

func (q *warmingQueue) Swap(i, j int) {
	q.items[i], q.items[j] = q.items[j], q.items[i]
	q.items[i].index = i
	q.items[j].index = j
}

func (q *warmingQueue) Push(x interface{}) {
	item := x.(*warmingItem)
	item.index = len(q.items)
	q.items = append(q.items, item)
}

func (q *warmingQueue) Pop() interface{} {
	old := q.items
	n := len(old)
	item := old[n-1]
	old[n-1] = nil
	item.index = -1
	q.items = old[0 : n-1]
	return item
}

// Add adds an item to the warming queue.
func (q *warmingQueue) Add(entityID, feature string, priority float64) {
	q.mu.Lock()
	defer q.mu.Unlock()

	item := &warmingItem{
		entityID: entityID,
		feature:  feature,
		priority: priority,
	}
	heap.Push(q, item)
}

// Next returns the next item to warm.
func (q *warmingQueue) Next() (string, string, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.Len() == 0 {
		return "", "", false
	}

	item := heap.Pop(q).(*warmingItem)
	return item.entityID, item.feature, true
}

// CoAccessTracker tracks features that are commonly accessed together.
type CoAccessTracker struct {
	coAccess   map[string]map[string]int64 // feature -> co-accessed features -> count
	window     time.Duration
	lastAccess map[string]time.Time
	mu         sync.RWMutex
}

// NewCoAccessTracker creates a new co-access tracker.
func NewCoAccessTracker(window time.Duration) *CoAccessTracker {
	return &CoAccessTracker{
		coAccess:   make(map[string]map[string]int64),
		window:     window,
		lastAccess: make(map[string]time.Time),
	}
}

// RecordAccess records feature accesses for co-occurrence tracking.
func (t *CoAccessTracker) RecordAccess(entityID string, features []string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	now := time.Now()

	// Track co-occurrences
	for i, f1 := range features {
		for j, f2 := range features {
			if i != j {
				if t.coAccess[f1] == nil {
					t.coAccess[f1] = make(map[string]int64)
				}
				t.coAccess[f1][f2]++
			}
		}
		t.lastAccess[f1] = now
	}
}

// GetRelatedFeatures returns features commonly accessed with the given feature.
func (t *CoAccessTracker) GetRelatedFeatures(feature string, limit int) []string {
	t.mu.RLock()
	defer t.mu.RUnlock()

	coFeatures := t.coAccess[feature]
	if len(coFeatures) == 0 {
		return nil
	}

	type pair struct {
		feature string
		count   int64
	}

	pairs := make([]pair, 0, len(coFeatures))
	for f, count := range coFeatures {
		pairs = append(pairs, pair{f, count})
	}

	sort.Slice(pairs, func(i, j int) bool {
		return pairs[i].count > pairs[j].count
	})

	if len(pairs) > limit {
		pairs = pairs[:limit]
	}

	result := make([]string, len(pairs))
	for i, p := range pairs {
		result[i] = p.feature
	}

	return result
}
