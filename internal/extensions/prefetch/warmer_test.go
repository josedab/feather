package prefetch

import (
	"testing"
	"time"
)

func TestNewWarmer(t *testing.T) {
	f := NewForecaster(DefaultForecasterConfig())
	w := NewWarmer(DefaultWarmerConfig(), f)
	if w == nil {
		t.Fatal("NewWarmer returned nil")
	}
}

func TestNewWarmer_InvalidConfig(t *testing.T) {
	f := NewForecaster(DefaultForecasterConfig())
	cfg := WarmerConfig{
		MaxConcurrent: 0,
		MemoryBudget:  0,
		Interval:      0,
	}
	w := NewWarmer(cfg, f)
	if w == nil {
		t.Fatal("NewWarmer should handle invalid config")
	}
}

func TestWarmer_ExecutePlan_Nil(t *testing.T) {
	f := NewForecaster(DefaultForecasterConfig())
	w := NewWarmer(DefaultWarmerConfig(), f)

	result := w.ExecutePlan(nil)
	if result == nil {
		t.Fatal("ExecutePlan returned nil for nil plan")
	}
	if result.PlannedItems != 0 {
		t.Errorf("PlannedItems = %d, want 0", result.PlannedItems)
	}
	if result.WarmedItems != 0 {
		t.Errorf("WarmedItems = %d, want 0", result.WarmedItems)
	}
}

func TestWarmer_ExecutePlan_Empty(t *testing.T) {
	f := NewForecaster(DefaultForecasterConfig())
	w := NewWarmer(DefaultWarmerConfig(), f)

	plan := &WarmingPlan{
		Candidates:  []WarmingCandidate{},
		BudgetBytes: 1024,
		GeneratedAt: time.Now(),
	}
	result := w.ExecutePlan(plan)
	if result.PlannedItems != 0 {
		t.Errorf("PlannedItems = %d, want 0", result.PlannedItems)
	}
}

func TestWarmer_ExecutePlan_WithCandidates(t *testing.T) {
	f := NewForecaster(DefaultForecasterConfig())
	w := NewWarmer(DefaultWarmerConfig(), f)

	plan := &WarmingPlan{
		Candidates: []WarmingCandidate{
			{Entity: "user:1", Feature: "clicks", Priority: 0.9, EstimatedSize: 256, Reason: "forecast"},
			{Entity: "user:2", Feature: "views", Priority: 0.8, EstimatedSize: 256, Reason: "forecast"},
			{Entity: "user:3", Feature: "buys", Priority: 0.7, EstimatedSize: 256, Reason: "forecast"},
		},
		EstimatedBytes: 768,
		BudgetBytes:    4096,
		GeneratedAt:    time.Now(),
	}

	result := w.ExecutePlan(plan)
	if result == nil {
		t.Fatal("ExecutePlan returned nil")
	}
	if result.PlannedItems != 3 {
		t.Errorf("PlannedItems = %d, want 3", result.PlannedItems)
	}
	if result.WarmedItems != 3 {
		t.Errorf("WarmedItems = %d, want 3", result.WarmedItems)
	}
	if result.FailedItems != 0 {
		t.Errorf("FailedItems = %d, want 0", result.FailedItems)
	}
	if result.BytesWarmed != 768 {
		t.Errorf("BytesWarmed = %d, want 768", result.BytesWarmed)
	}
	if result.Duration <= 0 {
		t.Error("Duration should be positive")
	}
}

func TestWarmer_ExecutePlan_MinPriority(t *testing.T) {
	f := NewForecaster(DefaultForecasterConfig())
	cfg := DefaultWarmerConfig()
	cfg.MinPriority = 0.5
	w := NewWarmer(cfg, f)

	plan := &WarmingPlan{
		Candidates: []WarmingCandidate{
			{Entity: "user:1", Feature: "clicks", Priority: 0.9, EstimatedSize: 256, Reason: "high"},
			{Entity: "user:2", Feature: "views", Priority: 0.3, EstimatedSize: 256, Reason: "low"},
			{Entity: "user:3", Feature: "buys", Priority: 0.1, EstimatedSize: 256, Reason: "very low"},
		},
		EstimatedBytes: 768,
		BudgetBytes:    4096,
		GeneratedAt:    time.Now(),
	}

	result := w.ExecutePlan(plan)
	if result.WarmedItems != 1 {
		t.Errorf("WarmedItems = %d, want 1 (only high priority)", result.WarmedItems)
	}
}

func TestWarmer_ExecutePlan_BudgetConstraint(t *testing.T) {
	f := NewForecaster(DefaultForecasterConfig())
	cfg := DefaultWarmerConfig()
	cfg.MinPriority = 0
	w := NewWarmer(cfg, f)

	plan := &WarmingPlan{
		Candidates: []WarmingCandidate{
			{Entity: "user:1", Feature: "clicks", Priority: 0.9, EstimatedSize: 500, Reason: "a"},
			{Entity: "user:2", Feature: "views", Priority: 0.8, EstimatedSize: 500, Reason: "b"},
			{Entity: "user:3", Feature: "buys", Priority: 0.7, EstimatedSize: 500, Reason: "c"},
		},
		EstimatedBytes: 1500,
		BudgetBytes:    800, // only fits 1 candidate
		GeneratedAt:    time.Now(),
	}

	result := w.ExecutePlan(plan)
	if result.WarmedItems > 1 {
		t.Errorf("WarmedItems = %d, should be <=1 with budget 800 and 500/item", result.WarmedItems)
	}
}

func TestWarmer_GetLastResult(t *testing.T) {
	f := NewForecaster(DefaultForecasterConfig())
	w := NewWarmer(DefaultWarmerConfig(), f)

	if w.GetLastResult() != nil {
		t.Error("GetLastResult should be nil before any execution")
	}

	plan := &WarmingPlan{
		Candidates: []WarmingCandidate{
			{Entity: "user:1", Feature: "clicks", Priority: 0.9, EstimatedSize: 256, Reason: "test"},
		},
		BudgetBytes: 4096,
		GeneratedAt: time.Now(),
	}
	w.ExecutePlan(plan)

	last := w.GetLastResult()
	if last == nil {
		t.Fatal("GetLastResult should not be nil after execution")
	}
	if last.WarmedItems != 1 {
		t.Errorf("WarmedItems = %d, want 1", last.WarmedItems)
	}
}

func TestWarmer_Stats(t *testing.T) {
	f := NewForecaster(DefaultForecasterConfig())
	w := NewWarmer(DefaultWarmerConfig(), f)

	plan := &WarmingPlan{
		Candidates: []WarmingCandidate{
			{Entity: "user:1", Feature: "clicks", Priority: 0.9, EstimatedSize: 256, Reason: "test"},
			{Entity: "user:2", Feature: "views", Priority: 0.8, EstimatedSize: 128, Reason: "test"},
		},
		BudgetBytes: 4096,
		GeneratedAt: time.Now(),
	}

	w.ExecutePlan(plan)
	w.ExecutePlan(plan)

	stats := w.Stats()
	if stats.TotalPlansExecuted != 2 {
		t.Errorf("TotalPlansExecuted = %d, want 2", stats.TotalPlansExecuted)
	}
	if stats.TotalItemsWarmed != 4 {
		t.Errorf("TotalItemsWarmed = %d, want 4", stats.TotalItemsWarmed)
	}
	if stats.TotalBytesWarmed != 768 {
		t.Errorf("TotalBytesWarmed = %d, want 768", stats.TotalBytesWarmed)
	}
	if stats.TotalItemsFailed != 0 {
		t.Errorf("TotalItemsFailed = %d, want 0", stats.TotalItemsFailed)
	}
}

func TestWarmer_ExecutePlan_Concurrent(t *testing.T) {
	f := NewForecaster(DefaultForecasterConfig())
	cfg := DefaultWarmerConfig()
	cfg.MaxConcurrent = 2
	w := NewWarmer(cfg, f)

	var candidates []WarmingCandidate
	for i := 0; i < 10; i++ {
		candidates = append(candidates, WarmingCandidate{
			Entity:        "user:" + string(rune('A'+i)),
			Feature:       "f" + string(rune('0'+i)),
			Priority:      0.9,
			EstimatedSize: 100,
			Reason:        "concurrent test",
		})
	}

	plan := &WarmingPlan{
		Candidates:  candidates,
		BudgetBytes: 10000,
		GeneratedAt: time.Now(),
	}

	result := w.ExecutePlan(plan)
	if result.WarmedItems != 10 {
		t.Errorf("WarmedItems = %d, want 10", result.WarmedItems)
	}
}

func TestWarmer_IntegrationWithForecaster(t *testing.T) {
	cfg := DefaultForecasterConfig()
	cfg.MinDataPoints = 3
	f := NewForecaster(cfg)

	now := time.Now()
	for i := 0; i < 10; i++ {
		f.RecordAccess("user:1", "clicks", now.Add(time.Duration(i)*time.Second))
	}

	plan := f.GetWarmingPlan(1024 * 1024)
	if plan == nil {
		t.Fatal("GetWarmingPlan returned nil")
	}

	w := NewWarmer(DefaultWarmerConfig(), f)
	result := w.ExecutePlan(plan)
	if result == nil {
		t.Fatal("ExecutePlan returned nil")
	}
	if result.FailedItems != 0 {
		t.Errorf("FailedItems = %d, want 0", result.FailedItems)
	}
}
