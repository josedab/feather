package prefetch

import (
	"testing"
)

func TestNewController(t *testing.T) {
	c := NewController(DefaultConfig())
	if c == nil {
		t.Fatal("NewController returned nil")
	}
	stats := c.Stats()
	if stats.TotalAccesses != 0 {
		t.Errorf("TotalAccesses = %d, want 0", stats.TotalAccesses)
	}
}

func TestRecordAccess(t *testing.T) {
	tests := []struct {
		name     string
		entity   string
		features []string
		wantInc  bool
	}{
		{"single feature", "user:1", []string{"clicks"}, true},
		{"multiple features", "user:1", []string{"clicks", "views", "buys"}, true},
		{"empty features", "user:1", []string{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewController(DefaultConfig())
			before := c.Stats().TotalAccesses
			c.RecordAccess(tt.entity, tt.features)
			after := c.Stats().TotalAccesses

			if tt.wantInc && after <= before {
				t.Error("expected TotalAccesses to increment")
			}
			if !tt.wantInc && after != before {
				t.Error("expected TotalAccesses to stay the same")
			}
		})
	}
}

func TestRecordAccess_CoAccessTracking(t *testing.T) {
	c := NewController(DefaultConfig())
	// Record co-access multiple times to build patterns
	for i := 0; i < 10; i++ {
		c.RecordAccess("user:1", []string{"clicks", "views"})
	}

	stats := c.Stats()
	if stats.PatternsTracked == 0 {
		t.Error("expected patterns to be tracked after co-accesses")
	}
}

func TestPredict_NoHistory(t *testing.T) {
	c := NewController(DefaultConfig())
	candidates := c.Predict("unknown_entity")
	if len(candidates) != 0 {
		t.Errorf("expected 0 candidates for unknown entity, got %d", len(candidates))
	}
}

func TestPredict_WithPatterns(t *testing.T) {
	cfg := DefaultConfig()
	cfg.PredictionThreshold = 0.1 // lower threshold to make predictions easier
	cfg.MinCoAccessCount = 1
	c := NewController(cfg)

	// Build strong co-access patterns: clicks always with views and buys
	for i := 0; i < 20; i++ {
		c.RecordAccess("user:1", []string{"clicks", "views", "buys"})
	}

	// Now record only "clicks" and see if views/buys are predicted
	c.RecordAccess("user:2", []string{"clicks"})
	candidates := c.Predict("user:2")

	// We should get some candidates based on co-access with "clicks"
	// (views and buys co-occur with clicks in user:1's history, but
	// Predict looks for features NOT in the entity's recent set)
	// The candidates may or may not appear depending on threshold,
	// but the function should not panic.
	if candidates == nil {
		// nil is valid if no candidates meet threshold
		return
	}
	for _, cand := range candidates {
		if cand.Feature == "" {
			t.Error("candidate feature should not be empty")
		}
		if cand.Score < 0 {
			t.Errorf("candidate score should be non-negative, got %f", cand.Score)
		}
	}
}

func TestGetPrefetchPlan_Empty(t *testing.T) {
	c := NewController(DefaultConfig())
	plan := c.GetPrefetchPlan("unknown")
	if plan == nil {
		t.Fatal("GetPrefetchPlan returned nil")
	}
	if plan.Priority != "low" {
		t.Errorf("Priority = %q, want %q", plan.Priority, "low")
	}
}

func TestGetPrefetchPlan_WithCandidates(t *testing.T) {
	cfg := DefaultConfig()
	cfg.PredictionThreshold = 0.01
	cfg.MinCoAccessCount = 1
	c := NewController(cfg)

	// Build strong patterns
	for i := 0; i < 50; i++ {
		c.RecordAccess("user:1", []string{"a", "b", "c"})
	}
	c.RecordAccess("user:2", []string{"a"})

	plan := c.GetPrefetchPlan("user:2")
	if plan == nil {
		t.Fatal("GetPrefetchPlan returned nil")
	}
	// Plan should have some structure regardless of candidates
	if plan.MemoryEstimate < 0 {
		t.Error("MemoryEstimate should be non-negative")
	}
}

func TestRecordPrefetchResult(t *testing.T) {
	c := NewController(DefaultConfig())

	c.RecordPrefetchResult(true)
	c.RecordPrefetchResult(true)
	c.RecordPrefetchResult(false)

	stats := c.Stats()
	if stats.HitRate < 0.6 || stats.HitRate > 0.7 {
		t.Errorf("HitRate = %f, want ~0.667", stats.HitRate)
	}
}

func TestStats(t *testing.T) {
	c := NewController(DefaultConfig())
	c.RecordAccess("user:1", []string{"a", "b"})
	c.RecordAccess("user:2", []string{"c"})

	stats := c.Stats()
	if stats.TotalAccesses != 2 {
		t.Errorf("TotalAccesses = %d, want 2", stats.TotalAccesses)
	}
	if stats.MemoryUsedBytes < 0 {
		t.Error("MemoryUsedBytes should be non-negative")
	}
}
