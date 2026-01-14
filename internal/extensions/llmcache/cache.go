// Package llmcache provides semantic caching for LLM prompts and responses
// with exact-match and embedding-based similarity lookup, TTL management,
// and per-provider cost tracking.
package llmcache

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"sync"
	"sync/atomic"
	"time"
)

// Provider identifies an LLM provider.
type Provider string

const (
	// ProviderOpenAI is the OpenAI provider.
	ProviderOpenAI Provider = "openai"
	// ProviderAnthropic is the Anthropic provider.
	ProviderAnthropic Provider = "anthropic"
	// ProviderCohere is the Cohere provider.
	ProviderCohere Provider = "cohere"
	// ProviderLocal is a local/self-hosted model.
	ProviderLocal Provider = "local"
)

// CacheConfig configures the LLM cache.
type CacheConfig struct {
	MaxEntries          int
	DefaultTTL          time.Duration
	SimilarityThreshold float64 // 0.0-1.0 threshold for semantic match
	EnableSemantic      bool
	EmbeddingDimension  int
}

// DefaultCacheConfig returns sensible defaults.
func DefaultCacheConfig() CacheConfig {
	return CacheConfig{
		MaxEntries:          10000,
		DefaultTTL:          1 * time.Hour,
		SimilarityThreshold: 0.92,
		EnableSemantic:      true,
		EmbeddingDimension:  256,
	}
}

// CacheEntry represents a cached LLM response.
type CacheEntry struct {
	Key        string    `json:"key"`
	Prompt     string    `json:"prompt"`
	Response   string    `json:"response"`
	Model      string    `json:"model"`
	Provider   Provider  `json:"provider"`
	Embedding  []float64 `json:"embedding,omitempty"`
	TokensIn   int       `json:"tokens_in"`
	TokensOut  int       `json:"tokens_out"`
	CostUSD    float64   `json:"cost_usd"`
	CreatedAt  time.Time `json:"created_at"`
	ExpiresAt  time.Time `json:"expires_at"`
	HitCount   int64     `json:"hit_count"`
	LastAccess time.Time `json:"last_access"`
}

// CacheStats tracks cache performance.
type CacheStats struct {
	Hits          atomic.Int64
	Misses        atomic.Int64
	ExactHits     atomic.Int64
	SemanticHits  atomic.Int64
	Evictions     atomic.Int64
	TotalSavedUSD atomic.Int64 // stored as microdollars (1e-6)
}

// Snapshot returns a point-in-time copy of stats.
func (s *CacheStats) Snapshot() map[string]interface{} {
	hits := s.Hits.Load()
	misses := s.Misses.Load()
	total := hits + misses
	hitRate := float64(0)
	if total > 0 {
		hitRate = float64(hits) / float64(total)
	}
	return map[string]interface{}{
		"hits":            hits,
		"misses":          misses,
		"exact_hits":      s.ExactHits.Load(),
		"semantic_hits":   s.SemanticHits.Load(),
		"evictions":       s.Evictions.Load(),
		"hit_rate":        hitRate,
		"total_saved_usd": float64(s.TotalSavedUSD.Load()) / 1e6,
	}
}

// Embedder generates vector embeddings for prompts.
type Embedder interface {
	Embed(text string) ([]float64, error)
}

// Cache provides semantic and exact-match caching for LLM calls.
type Cache struct {
	config   CacheConfig
	entries  map[string]*CacheEntry   // hash -> entry (exact match)
	semantic []*CacheEntry            // all entries with embeddings
	embedder Embedder
	stats    CacheStats
	mu       sync.RWMutex
}

// NewCache creates a new LLM cache.
func NewCache(cfg CacheConfig, embedder Embedder) *Cache {
	return &Cache{
		config:   cfg,
		entries:  make(map[string]*CacheEntry),
		embedder: embedder,
	}
}

// Get looks up a cached response, first by exact match then by semantic similarity.
func (c *Cache) Get(prompt, model string) (*CacheEntry, bool) {
	// Try exact match first
	key := c.hashKey(prompt, model)

	c.mu.RLock()
	entry, ok := c.entries[key]
	c.mu.RUnlock()

	if ok && time.Now().Before(entry.ExpiresAt) {
		c.stats.Hits.Add(1)
		c.stats.ExactHits.Add(1)
		c.stats.TotalSavedUSD.Add(int64(entry.CostUSD * 1e6))

		c.mu.Lock()
		entry.HitCount++
		entry.LastAccess = time.Now()
		c.mu.Unlock()

		return entry, true
	}

	// Try semantic match if enabled
	if c.config.EnableSemantic && c.embedder != nil {
		if entry := c.semanticLookup(prompt, model); entry != nil {
			c.stats.Hits.Add(1)
			c.stats.SemanticHits.Add(1)
			c.stats.TotalSavedUSD.Add(int64(entry.CostUSD * 1e6))
			return entry, true
		}
	}

	c.stats.Misses.Add(1)
	return nil, false
}

// Put stores a prompt-response pair in the cache.
func (c *Cache) Put(prompt, response, model string, provider Provider, tokensIn, tokensOut int, costUSD float64) error {
	key := c.hashKey(prompt, model)

	entry := &CacheEntry{
		Key:        key,
		Prompt:     prompt,
		Response:   response,
		Model:      model,
		Provider:   provider,
		TokensIn:   tokensIn,
		TokensOut:  tokensOut,
		CostUSD:    costUSD,
		CreatedAt:  time.Now(),
		ExpiresAt:  time.Now().Add(c.config.DefaultTTL),
		LastAccess: time.Now(),
	}

	// Generate embedding if semantic caching is enabled
	if c.config.EnableSemantic && c.embedder != nil {
		embedding, err := c.embedder.Embed(prompt)
		if err == nil {
			entry.Embedding = embedding
		}
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Evict if at capacity
	if len(c.entries) >= c.config.MaxEntries {
		c.evictOldest()
	}

	c.entries[key] = entry
	if len(entry.Embedding) > 0 {
		c.semantic = append(c.semantic, entry)
	}

	return nil
}

// Invalidate removes a specific entry.
func (c *Cache) Invalidate(prompt, model string) {
	key := c.hashKey(prompt, model)

	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.entries, key)
	c.rebuildSemantic()
}

// Clear removes all entries.
func (c *Cache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries = make(map[string]*CacheEntry)
	c.semantic = nil
}

// Size returns the number of cached entries.
func (c *Cache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.entries)
}

// Stats returns cache statistics.
func (c *Cache) Stats() map[string]interface{} {
	stats := c.stats.Snapshot()
	stats["size"] = c.Size()
	stats["max_entries"] = c.config.MaxEntries
	return stats
}

// CostByProvider returns total cached cost savings by provider.
func (c *Cache) CostByProvider() map[Provider]float64 {
	c.mu.RLock()
	defer c.mu.RUnlock()

	costs := make(map[Provider]float64)
	for _, entry := range c.entries {
		costs[entry.Provider] += entry.CostUSD * float64(entry.HitCount)
	}
	return costs
}

// semanticLookup finds the best semantic match above the threshold.
func (c *Cache) semanticLookup(prompt, model string) *CacheEntry {
	embedding, err := c.embedder.Embed(prompt)
	if err != nil || len(embedding) == 0 {
		return nil
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	var bestEntry *CacheEntry
	bestSimilarity := float64(0)

	now := time.Now()
	for _, entry := range c.semantic {
		if entry.Model != model || now.After(entry.ExpiresAt) {
			continue
		}

		sim := cosineSimilarity(embedding, entry.Embedding)
		if sim > bestSimilarity && sim >= c.config.SimilarityThreshold {
			bestSimilarity = sim
			bestEntry = entry
		}
	}

	if bestEntry != nil {
		bestEntry.HitCount++
		bestEntry.LastAccess = now
	}
	return bestEntry
}

func (c *Cache) evictOldest() {
	var oldestKey string
	var oldestTime time.Time

	for key, entry := range c.entries {
		if oldestKey == "" || entry.LastAccess.Before(oldestTime) {
			oldestKey = key
			oldestTime = entry.LastAccess
		}
	}

	if oldestKey != "" {
		delete(c.entries, oldestKey)
		c.stats.Evictions.Add(1)
		c.rebuildSemantic()
	}
}

func (c *Cache) rebuildSemantic() {
	c.semantic = make([]*CacheEntry, 0, len(c.entries))
	for _, entry := range c.entries {
		if len(entry.Embedding) > 0 {
			c.semantic = append(c.semantic, entry)
		}
	}
}

func (c *Cache) hashKey(prompt, model string) string {
	h := sha256.New()
	h.Write([]byte(model))
	h.Write([]byte{0})
	h.Write([]byte(prompt))
	return hex.EncodeToString(h.Sum(nil))
}

func cosineSimilarity(a, b []float64) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}

	var dot, normA, normB float64
	for i := range a {
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	denom := math.Sqrt(normA) * math.Sqrt(normB)
	if denom == 0 {
		return 0
	}
	return dot / denom
}

// ProviderPricing holds token pricing for a provider/model.
type ProviderPricing struct {
	Provider     Provider `json:"provider"`
	Model        string   `json:"model"`
	InputPer1K   float64  `json:"input_per_1k"`
	OutputPer1K  float64  `json:"output_per_1k"`
}

// EstimateCost estimates the cost of an LLM call.
func EstimateCost(pricing ProviderPricing, tokensIn, tokensOut int) float64 {
	return (float64(tokensIn) / 1000.0 * pricing.InputPer1K) +
		(float64(tokensOut) / 1000.0 * pricing.OutputPer1K)
}

// DefaultPricing returns pricing for common models.
func DefaultPricing() []ProviderPricing {
	return []ProviderPricing{
		{Provider: ProviderOpenAI, Model: "gpt-4", InputPer1K: 0.03, OutputPer1K: 0.06},
		{Provider: ProviderOpenAI, Model: "gpt-3.5-turbo", InputPer1K: 0.0005, OutputPer1K: 0.0015},
		{Provider: ProviderAnthropic, Model: "claude-3-sonnet", InputPer1K: 0.003, OutputPer1K: 0.015},
		{Provider: ProviderAnthropic, Model: "claude-3-haiku", InputPer1K: 0.00025, OutputPer1K: 0.00125},
	}
}

// CostTracker tracks LLM API costs.
type CostTracker struct {
	mu       sync.RWMutex
	costs    map[string]float64 // provider:model -> total cost
	savings  map[string]float64 // provider:model -> total savings from cache
}

// NewCostTracker creates a new cost tracker.
func NewCostTracker() *CostTracker {
	return &CostTracker{
		costs:   make(map[string]float64),
		savings: make(map[string]float64),
	}
}

// RecordCost records an actual LLM API call cost.
func (ct *CostTracker) RecordCost(provider Provider, model string, costUSD float64) {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	key := fmt.Sprintf("%s:%s", provider, model)
	ct.costs[key] += costUSD
}

// RecordSaving records a cache hit saving.
func (ct *CostTracker) RecordSaving(provider Provider, model string, savedUSD float64) {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	key := fmt.Sprintf("%s:%s", provider, model)
	ct.savings[key] += savedUSD
}

// Summary returns cost tracking summary.
func (ct *CostTracker) Summary() map[string]interface{} {
	ct.mu.RLock()
	defer ct.mu.RUnlock()

	totalCost := float64(0)
	totalSaved := float64(0)
	for _, c := range ct.costs {
		totalCost += c
	}
	for _, s := range ct.savings {
		totalSaved += s
	}

	return map[string]interface{}{
		"total_cost_usd":    totalCost,
		"total_saved_usd":   totalSaved,
		"savings_pct":       safeDivide(totalSaved, totalCost+totalSaved) * 100,
		"costs_by_model":    ct.costs,
		"savings_by_model":  ct.savings,
	}
}

func safeDivide(a, b float64) float64 {
	if b == 0 {
		return 0
	}
	return a / b
}
