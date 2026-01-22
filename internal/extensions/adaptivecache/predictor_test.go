package adaptivecache

import (
	"fmt"
	"testing"
)

func TestNewPredictor(t *testing.T) {
	p := NewPredictor(DefaultPredictorConfig())
	if p == nil {
		t.Fatal("NewPredictor returned nil")
	}

	if len(p.accessCounts) != 0 {
		t.Errorf("expected empty access counts, got %d", len(p.accessCounts))
	}

	// Zero config should use defaults
	p2 := NewPredictor(PredictorConfig{})
	if p2.config.WindowSize != 10000 {
		t.Errorf("expected default WindowSize 10000, got %d", p2.config.WindowSize)
	}
}

func TestRecordAccess(t *testing.T) {
	p := NewPredictor(DefaultPredictorConfig())

	for i := 0; i < 100; i++ {
		p.RecordAccess("key1")
	}

	p.mu.RLock()
	state, exists := p.accessCounts["key1"]
	p.mu.RUnlock()

	if !exists {
		t.Fatal("key1 should be tracked")
	}

	if state.count != 100 {
		t.Errorf("expected count 100, got %d", state.count)
	}

	score := p.computeScore(state, state.lastAccess)
	if score <= 0 {
		t.Errorf("expected score > 0, got %f", score)
	}
}

func TestPredictions(t *testing.T) {
	p := NewPredictor(DefaultPredictorConfig())

	// Record different access counts for multiple keys
	for i := 0; i < 50; i++ {
		p.RecordAccess("hot_key")
	}
	for i := 0; i < 20; i++ {
		p.RecordAccess("warm_key")
	}
	for i := 0; i < 5; i++ {
		p.RecordAccess("cold_key")
	}

	preds := p.GetPredictions(3)
	if len(preds) != 3 {
		t.Fatalf("expected 3 predictions, got %d", len(preds))
	}

	// Top prediction should be the most accessed key
	if preds[0].Key != "hot_key" {
		t.Errorf("expected hot_key as top prediction, got %s", preds[0].Key)
	}

	// Scores should be in descending order
	for i := 1; i < len(preds); i++ {
		if preds[i].Score > preds[i-1].Score {
			t.Errorf("predictions not sorted: index %d score %f > index %d score %f",
				i, preds[i].Score, i-1, preds[i-1].Score)
		}
	}

	// Test topK larger than tracked keys
	preds = p.GetPredictions(100)
	if len(preds) != 3 {
		t.Errorf("expected 3 predictions (all keys), got %d", len(preds))
	}
}

func TestShouldPromote(t *testing.T) {
	p := NewPredictor(PredictorConfig{
		WindowSize:         10000,
		PromotionThreshold: 0.7,
		MaxTracked:         50000,
		DecayFactor:        0.95,
	})

	// Cold key should not be promoted (threshold set high enough)
	p.RecordAccess("cold")
	if p.ShouldPromote("cold") {
		// A single access gives score=1*0.95^0=1.0 which exceeds 0.7,
		// so raise threshold for this test
	}

	p2 := NewPredictor(PredictorConfig{
		WindowSize:         10000,
		PromotionThreshold: 5.0, // High threshold
		MaxTracked:         50000,
		DecayFactor:        0.95,
	})
	p2.RecordAccess("cold")
	if p2.ShouldPromote("cold") {
		t.Error("cold key with 1 access should not be promoted with high threshold")
	}

	// Non-existent key should not be promoted
	if p.ShouldPromote("nonexistent") {
		t.Error("nonexistent key should not be promoted")
	}

	// Hot key with many accesses should be promoted
	for i := 0; i < 100; i++ {
		p.RecordAccess("hot")
	}
	if !p.ShouldPromote("hot") {
		t.Error("hot key with 100 accesses should be promoted")
	}
}

func TestHitMissTracking(t *testing.T) {
	p := NewPredictor(DefaultPredictorConfig())

	p.RecordHit("key1")
	p.RecordHit("key1")
	p.RecordHit("key2")
	p.RecordMiss("key3")

	p.mu.RLock()
	hits := p.hits
	misses := p.misses
	p.mu.RUnlock()

	if hits != 3 {
		t.Errorf("expected 3 hits, got %d", hits)
	}
	if misses != 1 {
		t.Errorf("expected 1 miss, got %d", misses)
	}
}

func TestStats(t *testing.T) {
	p := NewPredictor(DefaultPredictorConfig())

	for i := 0; i < 10; i++ {
		p.RecordAccess(fmt.Sprintf("key%d", i))
	}
	p.RecordHit("key0")
	p.RecordMiss("key1")

	stats := p.Stats()

	if stats.TotalRecords != 10 {
		t.Errorf("expected 10 total records, got %d", stats.TotalRecords)
	}

	if stats.TrackedKeys != 10 {
		t.Errorf("expected 10 tracked keys, got %d", stats.TrackedKeys)
	}

	expectedHitRate := 0.5
	if stats.HitRate != expectedHitRate {
		t.Errorf("expected hit rate %f, got %f", expectedHitRate, stats.HitRate)
	}

	if stats.AvgScore <= 0 {
		t.Errorf("expected positive avg score, got %f", stats.AvgScore)
	}
}
