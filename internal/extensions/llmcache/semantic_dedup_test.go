package llmcache

import (
	"testing"
)

func TestEnhancedCostTracker(t *testing.T) {
	tracker := NewEnhancedCostTracker()

	tracker.RecordRequest(ProviderOpenAI, "gpt-4", 0.03, 100, 200, 500.0)
	tracker.RecordRequest(ProviderOpenAI, "gpt-4", 0.05, 150, 300, 600.0)
	tracker.RecordRequest(ProviderAnthropic, "claude-3", 0.02, 80, 150, 400.0)
	tracker.RecordCacheHit(ProviderOpenAI, "gpt-4", 0.03)
	tracker.RecordCacheHit(ProviderOpenAI, "gpt-4", 0.04)

	dashboard := tracker.Dashboard()

	if dashboard.TotalCost != 0.10 {
		t.Errorf("expected total cost 0.10, got %f", dashboard.TotalCost)
	}
	if dashboard.TotalSaved != 0.07 {
		t.Errorf("expected total saved 0.07, got %f", dashboard.TotalSaved)
	}
	if len(dashboard.ByProvider) != 2 {
		t.Errorf("expected 2 providers, got %d", len(dashboard.ByProvider))
	}
	if dashboard.SavingsRate <= 0 {
		t.Errorf("expected positive savings rate, got %f", dashboard.SavingsRate)
	}
}

func TestEnhancedCostTrackerEmpty(t *testing.T) {
	tracker := NewEnhancedCostTracker()
	dashboard := tracker.Dashboard()
	if dashboard.TotalCost != 0 {
		t.Errorf("expected 0 cost, got %f", dashboard.TotalCost)
	}
	if len(dashboard.ByProvider) != 0 {
		t.Errorf("expected 0 providers, got %d", len(dashboard.ByProvider))
	}
}

func TestEnhancedCostTrackerHitRate(t *testing.T) {
	tracker := NewEnhancedCostTracker()
	tracker.RecordRequest(ProviderOpenAI, "gpt-4", 0.01, 50, 50, 200.0)
	tracker.RecordCacheHit(ProviderOpenAI, "gpt-4", 0.01)
	tracker.RecordCacheHit(ProviderOpenAI, "gpt-4", 0.01)

	dashboard := tracker.Dashboard()
	for _, pr := range dashboard.ByProvider {
		if pr.Provider == ProviderOpenAI {
			// 2 hits / (2 hits + 1 miss) ≈ 0.667
			if pr.HitRate < 0.6 || pr.HitRate > 0.7 {
				t.Errorf("expected hit rate ~0.667, got %f", pr.HitRate)
			}
		}
	}
}

func TestDefaultSemanticDedupConfig(t *testing.T) {
	cfg := DefaultSemanticDedupConfig()
	if cfg.SimilarityThreshold <= 0 || cfg.SimilarityThreshold > 1 {
		t.Errorf("invalid threshold: %f", cfg.SimilarityThreshold)
	}
	if cfg.MaxCandidates <= 0 {
		t.Errorf("invalid max candidates: %d", cfg.MaxCandidates)
	}
}

func TestDefaultInvalidationPolicy(t *testing.T) {
	p := DefaultInvalidationPolicy()
	if p.MaxAge <= 0 {
		t.Error("MaxAge should be positive")
	}
	if p.MaxEntries <= 0 {
		t.Error("MaxEntries should be positive")
	}
}

func TestInvalidateExpired(t *testing.T) {
	embedder := &testEmbedder{}
	cache := NewCache(CacheConfig{
		MaxEntries: 100,
		DefaultTTL: 0, // 0 TTL means everything is "expired"
	}, embedder)

	cache.Put("prompt1", "response1", "model1", ProviderOpenAI, 10, 20, 0.01)
	cache.Put("prompt2", "response2", "model1", ProviderOpenAI, 10, 20, 0.01)

	if cache.Size() != 2 {
		t.Fatalf("expected 2 entries, got %d", cache.Size())
	}

	removed := cache.InvalidateExpired()
	if removed != 2 {
		t.Errorf("expected 2 removed, got %d", removed)
	}
}

type testEmbedder struct{}

func (e *testEmbedder) Embed(text string) ([]float64, error) {
	return []float64{0.1, 0.2, 0.3}, nil
}
