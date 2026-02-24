package prefetch

import (
	"sync"
	"sync/atomic"
	"time"
)

// WarmerConfig configures the background pre-warming engine.
type WarmerConfig struct {
	Interval      time.Duration `json:"interval"`
	MaxConcurrent int           `json:"max_concurrent"`
	MemoryBudget  int64         `json:"memory_budget"` // bytes
	MinPriority   float64       `json:"min_priority"`
	EnableMetrics bool          `json:"enable_metrics"`
}

// DefaultWarmerConfig returns sensible defaults.
func DefaultWarmerConfig() WarmerConfig {
	return WarmerConfig{
		Interval:      30 * time.Second,
		MaxConcurrent: 4,
		MemoryBudget:  256 * 1024 * 1024,
		MinPriority:   0.1,
		EnableMetrics: true,
	}
}

// WarmingPlan describes a set of features to pre-warm.
type WarmingPlan struct {
	Candidates     []WarmingCandidate `json:"candidates"`
	EstimatedBytes int64              `json:"estimated_bytes"`
	BudgetBytes    int64              `json:"budget_bytes"`
	GeneratedAt    time.Time          `json:"generated_at"`
}

// WarmingCandidate is a single feature/entity to pre-warm.
type WarmingCandidate struct {
	Entity        string  `json:"entity"`
	Feature       string  `json:"feature"`
	Priority      float64 `json:"priority"`
	EstimatedSize int64   `json:"estimated_size_bytes"`
	Reason        string  `json:"reason"`
}

// WarmingResult records the outcome of executing a warming plan.
type WarmingResult struct {
	PlannedItems int           `json:"planned_items"`
	WarmedItems  int           `json:"warmed_items"`
	FailedItems  int           `json:"failed_items"`
	BytesWarmed  int64         `json:"bytes_warmed"`
	Duration     time.Duration `json:"duration"`
	Timestamp    time.Time     `json:"timestamp"`
}

// WarmerStats holds runtime statistics for the warmer.
type WarmerStats struct {
	TotalPlansExecuted int64 `json:"total_plans_executed"`
	TotalItemsWarmed   int64 `json:"total_items_warmed"`
	TotalItemsFailed   int64 `json:"total_items_failed"`
	TotalBytesWarmed   int64 `json:"total_bytes_warmed"`
	LastRunDuration    int64 `json:"last_run_duration_ms"`
}

// Warmer executes warming plans by pre-loading features based on forecasts.
type Warmer struct {
	config     WarmerConfig
	forecaster *Forecaster

	lastResult *WarmingResult

	totalPlansExecuted atomic.Int64
	totalItemsWarmed   atomic.Int64
	totalItemsFailed   atomic.Int64
	totalBytesWarmed   atomic.Int64
	lastRunDuration    atomic.Int64

	mu sync.RWMutex
}

// NewWarmer creates a new pre-warming engine.
func NewWarmer(cfg WarmerConfig, forecaster *Forecaster) *Warmer {
	if cfg.MaxConcurrent <= 0 {
		cfg.MaxConcurrent = 4
	}
	if cfg.MemoryBudget <= 0 {
		cfg.MemoryBudget = 256 * 1024 * 1024
	}
	if cfg.Interval <= 0 {
		cfg.Interval = 30 * time.Second
	}
	return &Warmer{
		config:     cfg,
		forecaster: forecaster,
	}
}

// ExecutePlan processes a warming plan, simulating feature loading.
// In a full integration, this would load features from warm tier to hot tier.
func (w *Warmer) ExecutePlan(plan *WarmingPlan) *WarmingResult {
	start := time.Now()

	if plan == nil || len(plan.Candidates) == 0 {
		result := &WarmingResult{
			PlannedItems: 0,
			WarmedItems:  0,
			Duration:     time.Since(start),
			Timestamp:    start,
		}
		w.mu.Lock()
		w.lastResult = result
		w.mu.Unlock()
		w.totalPlansExecuted.Add(1)
		w.lastRunDuration.Store(time.Since(start).Milliseconds())
		return result
	}

	var warmed, failed int
	var bytesWarmed int64
	budgetRemaining := plan.BudgetBytes

	// Process candidates respecting budget and priority filter.
	sem := make(chan struct{}, w.config.MaxConcurrent)
	var resultMu sync.Mutex
	var wg sync.WaitGroup

	for _, candidate := range plan.Candidates {
		if candidate.Priority < w.config.MinPriority {
			continue
		}
		if candidate.EstimatedSize > budgetRemaining {
			continue
		}
		budgetRemaining -= candidate.EstimatedSize

		wg.Add(1)
		sem <- struct{}{}
		go func(c WarmingCandidate) {
			defer wg.Done()
			defer func() { <-sem }()

			// Simulate warming: in production this would call store.Get
			// from warm tier and store.Put to hot tier.
			success := w.warmFeature(c)

			resultMu.Lock()
			if success {
				warmed++
				bytesWarmed += c.EstimatedSize
			} else {
				failed++
			}
			resultMu.Unlock()
		}(candidate)
	}

	wg.Wait()

	duration := time.Since(start)
	result := &WarmingResult{
		PlannedItems: len(plan.Candidates),
		WarmedItems:  warmed,
		FailedItems:  failed,
		BytesWarmed:  bytesWarmed,
		Duration:     duration,
		Timestamp:    start,
	}

	w.mu.Lock()
	w.lastResult = result
	w.mu.Unlock()

	w.totalPlansExecuted.Add(1)
	w.totalItemsWarmed.Add(int64(warmed))
	w.totalItemsFailed.Add(int64(failed))
	w.totalBytesWarmed.Add(bytesWarmed)
	w.lastRunDuration.Store(duration.Milliseconds())

	return result
}

// warmFeature simulates warming a single feature. Returns true on success.
func (w *Warmer) warmFeature(_ WarmingCandidate) bool {
	// In production, this would:
	// 1. Read feature from warm tier (BadgerDB)
	// 2. Write to hot tier (LRU cache)
	// For now, always succeed.
	return true
}

// GetLastResult returns the most recent warming result, or nil if none.
func (w *Warmer) GetLastResult() *WarmingResult {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.lastResult
}

// Stats returns warmer runtime statistics.
func (w *Warmer) Stats() WarmerStats {
	return WarmerStats{
		TotalPlansExecuted: w.totalPlansExecuted.Load(),
		TotalItemsWarmed:   w.totalItemsWarmed.Load(),
		TotalItemsFailed:   w.totalItemsFailed.Load(),
		TotalBytesWarmed:   w.totalBytesWarmed.Load(),
		LastRunDuration:    w.lastRunDuration.Load(),
	}
}
