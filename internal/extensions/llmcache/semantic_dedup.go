package llmcache

import (
	"fmt"
	"sort"
	"sync"
	"time"
)

// SemanticDedupConfig configures semantic deduplication.
type SemanticDedupConfig struct {
	SimilarityThreshold float64       `json:"similarity_threshold"`
	MaxCandidates       int           `json:"max_candidates"`
	DedupWindow         time.Duration `json:"dedup_window"`
}

// DefaultSemanticDedupConfig returns reasonable defaults for semantic dedup.
func DefaultSemanticDedupConfig() SemanticDedupConfig {
	return SemanticDedupConfig{
		SimilarityThreshold: 0.92,
		MaxCandidates:       50,
		DedupWindow:         24 * time.Hour,
	}
}

// DedupResult captures the outcome of a semantic dedup check.
type DedupResult struct {
	IsDuplicate  bool    `json:"is_duplicate"`
	OriginalKey  string  `json:"original_key,omitempty"`
	Similarity   float64 `json:"similarity"`
	SavedTokens  int     `json:"saved_tokens"`
	SavedCostUSD float64 `json:"saved_cost_usd"`
}

// ProviderCostReport provides per-provider cost breakdown.
type ProviderCostReport struct {
	Provider     Provider  `json:"provider"`
	TotalCost    float64   `json:"total_cost_usd"`
	TotalSaved   float64   `json:"total_saved_usd"`
	TotalTokens  int64     `json:"total_tokens"`
	CacheHits    int64     `json:"cache_hits"`
	CacheMisses  int64     `json:"cache_misses"`
	HitRate      float64   `json:"hit_rate"`
	SavingsRate  float64   `json:"savings_rate"`
	Models       []ModelCost `json:"models"`
}

// ModelCost breaks down costs by model within a provider.
type ModelCost struct {
	Model       string  `json:"model"`
	Cost        float64 `json:"cost_usd"`
	Saved       float64 `json:"saved_usd"`
	Requests    int64   `json:"requests"`
	AvgLatencyMs float64 `json:"avg_latency_ms"`
}

// CostDashboard provides a comprehensive cost overview.
type CostDashboard struct {
	TotalCost     float64              `json:"total_cost_usd"`
	TotalSaved    float64              `json:"total_saved_usd"`
	SavingsRate   float64              `json:"savings_rate"`
	ByProvider    []ProviderCostReport `json:"by_provider"`
	DailyTrend    []DailyCost          `json:"daily_trend"`
	TopExpensive  []ExpensiveQuery     `json:"top_expensive_queries"`
	GeneratedAt   time.Time            `json:"generated_at"`
}

// DailyCost tracks cost per day.
type DailyCost struct {
	Date  string  `json:"date"`
	Cost  float64 `json:"cost_usd"`
	Saved float64 `json:"saved_usd"`
	Hits  int64   `json:"hits"`
}

// ExpensiveQuery tracks the most expensive queries.
type ExpensiveQuery struct {
	PromptHash string   `json:"prompt_hash"`
	Model      string   `json:"model"`
	Provider   Provider `json:"provider"`
	CostUSD    float64  `json:"cost_usd"`
	TokensIn   int      `json:"tokens_in"`
	TokensOut  int      `json:"tokens_out"`
	Count      int      `json:"count"`
}

// EnhancedCostTracker extends cost tracking with time-series and per-model detail.
type EnhancedCostTracker struct {
	mu          sync.RWMutex
	byProvider  map[Provider]*providerDetail
	dailyCosts  []DailyCost
	expensive   []ExpensiveQuery
	totalCost   float64
	totalSaved  float64
}

type providerDetail struct {
	cost     float64
	saved    float64
	tokens   int64
	hits     int64
	misses   int64
	models   map[string]*modelDetail
}

type modelDetail struct {
	cost        float64
	saved       float64
	requests    int64
	totalLatMs  float64
}

// NewEnhancedCostTracker creates a new enhanced cost tracker.
func NewEnhancedCostTracker() *EnhancedCostTracker {
	return &EnhancedCostTracker{
		byProvider: make(map[Provider]*providerDetail),
	}
}

// RecordRequest records a cache miss (actual API call).
func (t *EnhancedCostTracker) RecordRequest(provider Provider, model string, costUSD float64, tokensIn, tokensOut int, latencyMs float64) {
	t.mu.Lock()
	defer t.mu.Unlock()

	pd := t.getOrCreateProvider(provider)
	pd.cost += costUSD
	pd.tokens += int64(tokensIn + tokensOut)
	pd.misses++
	t.totalCost += costUSD

	md := t.getOrCreateModel(pd, model)
	md.cost += costUSD
	md.requests++
	md.totalLatMs += latencyMs

	t.trackExpensive(provider, model, costUSD, tokensIn, tokensOut)
}

// RecordCacheHit records a cache hit (saved API call).
func (t *EnhancedCostTracker) RecordCacheHit(provider Provider, model string, savedUSD float64) {
	t.mu.Lock()
	defer t.mu.Unlock()

	pd := t.getOrCreateProvider(provider)
	pd.saved += savedUSD
	pd.hits++
	t.totalSaved += savedUSD

	md := t.getOrCreateModel(pd, model)
	md.saved += savedUSD
}

// Dashboard generates a comprehensive cost dashboard.
func (t *EnhancedCostTracker) Dashboard() *CostDashboard {
	t.mu.RLock()
	defer t.mu.RUnlock()

	dashboard := &CostDashboard{
		TotalCost:   t.totalCost,
		TotalSaved:  t.totalSaved,
		SavingsRate: safeDivide(t.totalSaved, t.totalCost+t.totalSaved),
		GeneratedAt: time.Now(),
	}

	for provider, pd := range t.byProvider {
		report := ProviderCostReport{
			Provider:    provider,
			TotalCost:   pd.cost,
			TotalSaved:  pd.saved,
			TotalTokens: pd.tokens,
			CacheHits:   pd.hits,
			CacheMisses: pd.misses,
			HitRate:     safeDivide(float64(pd.hits), float64(pd.hits+pd.misses)),
			SavingsRate: safeDivide(pd.saved, pd.cost+pd.saved),
		}
		for modelName, md := range pd.models {
			avgLat := 0.0
			if md.requests > 0 {
				avgLat = md.totalLatMs / float64(md.requests)
			}
			report.Models = append(report.Models, ModelCost{
				Model:        modelName,
				Cost:         md.cost,
				Saved:        md.saved,
				Requests:     md.requests,
				AvgLatencyMs: avgLat,
			})
		}
		sort.Slice(report.Models, func(i, j int) bool {
			return report.Models[i].Cost > report.Models[j].Cost
		})
		dashboard.ByProvider = append(dashboard.ByProvider, report)
	}

	sort.Slice(dashboard.ByProvider, func(i, j int) bool {
		return dashboard.ByProvider[i].TotalCost > dashboard.ByProvider[j].TotalCost
	})

	// Return top expensive queries.
	dashboard.TopExpensive = make([]ExpensiveQuery, len(t.expensive))
	copy(dashboard.TopExpensive, t.expensive)
	sort.Slice(dashboard.TopExpensive, func(i, j int) bool {
		return dashboard.TopExpensive[i].CostUSD > dashboard.TopExpensive[j].CostUSD
	})
	if len(dashboard.TopExpensive) > 20 {
		dashboard.TopExpensive = dashboard.TopExpensive[:20]
	}

	return dashboard
}

func (t *EnhancedCostTracker) getOrCreateProvider(p Provider) *providerDetail {
	pd, ok := t.byProvider[p]
	if !ok {
		pd = &providerDetail{models: make(map[string]*modelDetail)}
		t.byProvider[p] = pd
	}
	return pd
}

func (t *EnhancedCostTracker) getOrCreateModel(pd *providerDetail, model string) *modelDetail {
	md, ok := pd.models[model]
	if !ok {
		md = &modelDetail{}
		pd.models[model] = md
	}
	return md
}

func (t *EnhancedCostTracker) trackExpensive(provider Provider, model string, costUSD float64, tokensIn, tokensOut int) {
	key := fmt.Sprintf("%s:%s", provider, model)
	found := false
	for i := range t.expensive {
		if t.expensive[i].PromptHash == key {
			t.expensive[i].CostUSD += costUSD
			t.expensive[i].Count++
			found = true
			break
		}
	}
	if !found {
		t.expensive = append(t.expensive, ExpensiveQuery{
			PromptHash: key,
			Model:      model,
			Provider:   provider,
			CostUSD:    costUSD,
			TokensIn:   tokensIn,
			TokensOut:  tokensOut,
			Count:      1,
		})
	}
}

// InvalidationPolicy defines rules for cache invalidation.
type InvalidationPolicy struct {
	MaxAge       time.Duration `json:"max_age"`
	MaxEntries   int           `json:"max_entries"`
	InvalidateOn []string      `json:"invalidate_on,omitempty"`
}

// DefaultInvalidationPolicy returns a sensible default policy.
func DefaultInvalidationPolicy() InvalidationPolicy {
	return InvalidationPolicy{
		MaxAge:     24 * time.Hour,
		MaxEntries: 10000,
	}
}

// InvalidateExpired removes entries older than the policy's MaxAge from the cache.
func (c *Cache) InvalidateExpired() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	cutoff := time.Now().Add(-c.config.DefaultTTL)
	removed := 0
	for key, entry := range c.entries {
		if entry.CreatedAt.Before(cutoff) {
			delete(c.entries, key)
			removed++
		}
	}
	return removed
}

// SimilaritySearch finds cached entries semantically similar to the given prompt.
func (c *Cache) SimilaritySearch(prompt string, threshold float64, maxResults int) []CacheEntry {
	if c.embedder == nil {
		return nil
	}

	embedding, err := c.embedder.Embed(prompt)
	if err != nil || len(embedding) == 0 {
		return nil
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	type scored struct {
		entry CacheEntry
		score float64
	}

	var candidates []scored
	for _, entry := range c.entries {
		if len(entry.Embedding) == 0 {
			continue
		}
		sim := cosineSimilarity(embedding, entry.Embedding)
		if sim >= threshold {
			candidates = append(candidates, scored{entry: *entry, score: sim})
		}
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].score > candidates[j].score
	})

	if maxResults > 0 && len(candidates) > maxResults {
		candidates = candidates[:maxResults]
	}

	results := make([]CacheEntry, len(candidates))
	for i, c := range candidates {
		results[i] = c.entry
	}
	return results
}
