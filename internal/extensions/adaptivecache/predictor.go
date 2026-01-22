package adaptivecache

import (
	"math"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// AccessRecord represents a single key access event.
type AccessRecord struct {
	Key       string
	Timestamp time.Time
}

// PredictorConfig configures the adaptive cache predictor.
type PredictorConfig struct {
	// WindowSize is the maximum number of access records to consider.
	WindowSize int

	// PromotionThreshold is the minimum score for a key to be promoted.
	PromotionThreshold float64

	// MaxTracked is the maximum number of keys to track.
	MaxTracked int

	// DecayFactor controls the exponential decay rate.
	DecayFactor float64
}

// DefaultPredictorConfig returns sensible defaults.
func DefaultPredictorConfig() PredictorConfig {
	return PredictorConfig{
		WindowSize:         10000,
		PromotionThreshold: 0.7,
		MaxTracked:         50000,
		DecayFactor:        0.95,
	}
}

// Prediction represents a predicted-hot key.
type Prediction struct {
	Key        string
	Score      float64
	Confidence float64
}

// PredictorStats contains predictor statistics.
type PredictorStats struct {
	TotalRecords    int64
	TotalPredictions int64
	HitRate         float64
	AvgScore        float64
	TrackedKeys     int
}

// keyState tracks the access state for a single key.
type keyState struct {
	count      int64
	lastAccess time.Time
	score      float64
}

// Predictor predicts feature access patterns using exponential smoothing.
type Predictor struct {
	mu           sync.RWMutex
	config       PredictorConfig
	accessCounts map[string]*keyState
	predictions  []Prediction
	hits         int64
	misses       int64
	totalRecords atomic.Int64
	totalPreds   atomic.Int64
}

// NewPredictor creates a new adaptive cache predictor.
func NewPredictor(config PredictorConfig) *Predictor {
	if config.WindowSize == 0 {
		config = DefaultPredictorConfig()
	}

	return &Predictor{
		config:       config,
		accessCounts: make(map[string]*keyState),
		predictions:  make([]Prediction, 0),
	}
}

// RecordAccess records an access to the given key.
func (p *Predictor) RecordAccess(key string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	now := time.Now()
	p.totalRecords.Add(1)

	state, exists := p.accessCounts[key]
	if !exists {
		// Evict least-scored key if at capacity
		if len(p.accessCounts) >= p.config.MaxTracked {
			p.evictLowest()
		}
		state = &keyState{
			lastAccess: now,
		}
		p.accessCounts[key] = state
	}

	state.count++
	state.lastAccess = now
	state.score = p.computeScore(state, now)
}

// GetPredictions returns the top-K keys sorted by score.
func (p *Predictor) GetPredictions(topK int) []Prediction {
	p.mu.RLock()
	defer p.mu.RUnlock()

	now := time.Now()
	preds := make([]Prediction, 0, len(p.accessCounts))

	for key, state := range p.accessCounts {
		score := p.computeScore(state, now)
		confidence := math.Min(1.0, float64(state.count)/100.0)
		preds = append(preds, Prediction{
			Key:        key,
			Score:      score,
			Confidence: confidence,
		})
	}

	sort.Slice(preds, func(i, j int) bool {
		return preds[i].Score > preds[j].Score
	})

	if topK > len(preds) {
		topK = len(preds)
	}

	result := make([]Prediction, topK)
	copy(result, preds[:topK])

	p.totalPreds.Add(int64(topK))

	return result
}

// ShouldPromote returns true if the key's score exceeds the promotion threshold.
func (p *Predictor) ShouldPromote(key string) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()

	state, exists := p.accessCounts[key]
	if !exists {
		return false
	}

	score := p.computeScore(state, time.Now())
	return score > p.config.PromotionThreshold
}

// RecordHit records a cache hit for hit rate tracking.
func (p *Predictor) RecordHit(key string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.hits++
	_ = key
}

// RecordMiss records a cache miss for hit rate tracking.
func (p *Predictor) RecordMiss(key string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.misses++
	_ = key
}

// Stats returns predictor statistics.
func (p *Predictor) Stats() PredictorStats {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var hitRate float64
	total := p.hits + p.misses
	if total > 0 {
		hitRate = float64(p.hits) / float64(total)
	}

	var avgScore float64
	now := time.Now()
	if len(p.accessCounts) > 0 {
		var sum float64
		for _, state := range p.accessCounts {
			sum += p.computeScore(state, now)
		}
		avgScore = sum / float64(len(p.accessCounts))
	}

	return PredictorStats{
		TotalRecords:    p.totalRecords.Load(),
		TotalPredictions: p.totalPreds.Load(),
		HitRate:         hitRate,
		AvgScore:        avgScore,
		TrackedKeys:     len(p.accessCounts),
	}
}

// computeScore calculates the score for a key using exponential decay.
// score = count * decayFactor^(secondsSinceLastAccess/3600)
func (p *Predictor) computeScore(state *keyState, now time.Time) float64 {
	elapsed := now.Sub(state.lastAccess).Seconds()
	decay := math.Pow(p.config.DecayFactor, elapsed/3600.0)
	return float64(state.count) * decay
}

// evictLowest removes the key with the lowest score.
func (p *Predictor) evictLowest() {
	var lowestKey string
	lowestScore := math.MaxFloat64
	now := time.Now()

	for key, state := range p.accessCounts {
		score := p.computeScore(state, now)
		if score < lowestScore {
			lowestScore = score
			lowestKey = key
		}
	}

	if lowestKey != "" {
		delete(p.accessCounts, lowestKey)
	}
}
