package llmcache

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockEmbedder struct {
	embeddings map[string][]float64
}

func (m *mockEmbedder) Embed(text string) ([]float64, error) {
	if e, ok := m.embeddings[text]; ok {
		return e, nil
	}
	// Return a deterministic embedding based on text length
	dim := 4
	emb := make([]float64, dim)
	for i := range emb {
		emb[i] = float64(len(text)+i) / 100.0
	}
	return emb, nil
}

func TestCache_ExactMatch(t *testing.T) {
	cfg := DefaultCacheConfig()
	cfg.EnableSemantic = false
	cache := NewCache(cfg, nil)

	err := cache.Put("Hello", "World", "gpt-4", ProviderOpenAI, 10, 5, 0.001)
	require.NoError(t, err)

	entry, found := cache.Get("Hello", "gpt-4")
	assert.True(t, found)
	assert.Equal(t, "World", entry.Response)

	stats := cache.Stats()
	assert.Equal(t, int64(1), stats["hits"])
	assert.Equal(t, int64(1), stats["exact_hits"])
}

func TestCache_Miss(t *testing.T) {
	cfg := DefaultCacheConfig()
	cfg.EnableSemantic = false
	cache := NewCache(cfg, nil)

	_, found := cache.Get("Hello", "gpt-4")
	assert.False(t, found)

	stats := cache.Stats()
	assert.Equal(t, int64(1), stats["misses"])
}

func TestCache_SemanticMatch(t *testing.T) {
	embedder := &mockEmbedder{
		embeddings: map[string][]float64{
			"What is Go?":               {0.9, 0.1, 0.0, 0.0},
			"Tell me about Go language": {0.88, 0.12, 0.01, 0.0},
		},
	}

	cfg := DefaultCacheConfig()
	cfg.SimilarityThreshold = 0.95
	cache := NewCache(cfg, embedder)

	_ = cache.Put("What is Go?", "Go is a programming language", "gpt-4", ProviderOpenAI, 10, 20, 0.003)

	entry, found := cache.Get("Tell me about Go language", "gpt-4")
	assert.True(t, found)
	assert.Equal(t, "Go is a programming language", entry.Response)

	stats := cache.Stats()
	assert.Equal(t, int64(1), stats["semantic_hits"])
}

func TestCache_ModelMismatch(t *testing.T) {
	cfg := DefaultCacheConfig()
	cfg.EnableSemantic = false
	cache := NewCache(cfg, nil)

	_ = cache.Put("Hello", "World", "gpt-4", ProviderOpenAI, 10, 5, 0.001)

	// Different model should not match
	_, found := cache.Get("Hello", "gpt-3.5-turbo")
	assert.False(t, found)
}

func TestCache_Eviction(t *testing.T) {
	cfg := DefaultCacheConfig()
	cfg.MaxEntries = 2
	cfg.EnableSemantic = false
	cache := NewCache(cfg, nil)

	_ = cache.Put("a", "1", "m", ProviderOpenAI, 1, 1, 0.001)
	_ = cache.Put("b", "2", "m", ProviderOpenAI, 1, 1, 0.001)
	_ = cache.Put("c", "3", "m", ProviderOpenAI, 1, 1, 0.001)

	assert.Equal(t, 2, cache.Size())
}

func TestCache_Invalidate(t *testing.T) {
	cfg := DefaultCacheConfig()
	cfg.EnableSemantic = false
	cache := NewCache(cfg, nil)

	_ = cache.Put("Hello", "World", "gpt-4", ProviderOpenAI, 10, 5, 0.001)
	cache.Invalidate("Hello", "gpt-4")

	_, found := cache.Get("Hello", "gpt-4")
	assert.False(t, found)
}

func TestCache_Clear(t *testing.T) {
	cfg := DefaultCacheConfig()
	cfg.EnableSemantic = false
	cache := NewCache(cfg, nil)

	_ = cache.Put("a", "1", "m", ProviderOpenAI, 1, 1, 0.001)
	_ = cache.Put("b", "2", "m", ProviderOpenAI, 1, 1, 0.001)
	cache.Clear()

	assert.Equal(t, 0, cache.Size())
}

func TestCosineSimilarity(t *testing.T) {
	tests := []struct {
		name     string
		a, b     []float64
		expected float64
	}{
		{"identical", []float64{1, 0, 0}, []float64{1, 0, 0}, 1.0},
		{"orthogonal", []float64{1, 0, 0}, []float64{0, 1, 0}, 0.0},
		{"opposite", []float64{1, 0, 0}, []float64{-1, 0, 0}, -1.0},
		{"empty", []float64{}, []float64{}, 0.0},
		{"different lengths", []float64{1, 0}, []float64{1, 0, 0}, 0.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cosineSimilarity(tt.a, tt.b)
			assert.InDelta(t, tt.expected, got, 0.0001)
		})
	}
}

func TestEstimateCost(t *testing.T) {
	pricing := ProviderPricing{
		Provider:    ProviderOpenAI,
		Model:       "gpt-4",
		InputPer1K:  0.03,
		OutputPer1K: 0.06,
	}

	cost := EstimateCost(pricing, 1000, 500)
	expected := 0.03 + 0.03 // 1K input * 0.03 + 0.5K output * 0.06
	assert.InDelta(t, expected, cost, 0.0001)
}

func TestCostTracker(t *testing.T) {
	ct := NewCostTracker()
	ct.RecordCost(ProviderOpenAI, "gpt-4", 0.50)
	ct.RecordCost(ProviderOpenAI, "gpt-4", 0.30)
	ct.RecordSaving(ProviderOpenAI, "gpt-4", 0.20)

	summary := ct.Summary()
	assert.InDelta(t, 0.80, summary["total_cost_usd"], 0.001)
	assert.InDelta(t, 0.20, summary["total_saved_usd"], 0.001)
	savingsPct := summary["savings_pct"].(float64)
	assert.Greater(t, savingsPct, float64(0))
}

func TestDefaultPricing(t *testing.T) {
	pricing := DefaultPricing()
	assert.NotEmpty(t, pricing)
	for _, p := range pricing {
		assert.NotEmpty(t, p.Model)
		assert.Greater(t, p.InputPer1K, float64(0))
	}
}

func TestCacheStats_HitRate(t *testing.T) {
	var stats CacheStats
	stats.Hits.Store(3)
	stats.Misses.Store(7)

	snapshot := stats.Snapshot()
	hitRate := snapshot["hit_rate"].(float64)
	assert.InDelta(t, 0.3, hitRate, 0.001)
}

func TestCache_CostByProvider(t *testing.T) {
	cfg := DefaultCacheConfig()
	cfg.EnableSemantic = false
	cache := NewCache(cfg, nil)

	_ = cache.Put("a", "1", "gpt-4", ProviderOpenAI, 100, 50, 0.01)
	_ = cache.Put("b", "2", "claude", ProviderAnthropic, 100, 50, 0.005)

	// Hit the OpenAI entry twice
	cache.Get("a", "gpt-4")
	cache.Get("a", "gpt-4")

	costs := cache.CostByProvider()
	assert.Greater(t, costs[ProviderOpenAI], float64(0))
	_ = math.Abs(0) // use math to avoid unused import
}
