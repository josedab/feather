package joins

import (
	"context"
	"testing"
	"time"
)

func TestDefaultEngineConfig(t *testing.T) {
	cfg := DefaultEngineConfig()
	if cfg.MaxPlans <= 0 {
		t.Errorf("MaxPlans should be positive, got %d", cfg.MaxPlans)
	}
	if cfg.DefaultWindow <= 0 {
		t.Errorf("DefaultWindow should be positive, got %v", cfg.DefaultWindow)
	}
	if cfg.DefaultWatermark <= 0 {
		t.Errorf("DefaultWatermark should be positive, got %v", cfg.DefaultWatermark)
	}
	if cfg.MaxBufferSize <= 0 {
		t.Errorf("MaxBufferSize should be positive, got %d", cfg.MaxBufferSize)
	}
}

func TestCreatePlan(t *testing.T) {
	tests := []struct {
		name    string
		cfg     JoinConfig
		wantErr bool
	}{
		{
			name: "valid config",
			cfg: JoinConfig{
				LeftEntity:  "users",
				RightEntity: "transactions",
				JoinKey:     "user_id",
				JoinType:    Inner,
				Window:      5 * time.Minute,
				Watermark:   1 * time.Minute,
			},
			wantErr: false,
		},
		{
			name: "missing left entity",
			cfg: JoinConfig{
				RightEntity: "transactions",
				JoinKey:     "user_id",
			},
			wantErr: true,
		},
		{
			name: "missing right entity",
			cfg: JoinConfig{
				LeftEntity: "users",
				JoinKey:    "user_id",
			},
			wantErr: true,
		},
		{
			name: "missing join key",
			cfg: JoinConfig{
				LeftEntity:  "users",
				RightEntity: "transactions",
			},
			wantErr: true,
		},
		{
			name: "defaults applied for zero window and watermark",
			cfg: JoinConfig{
				LeftEntity:  "users",
				RightEntity: "transactions",
				JoinKey:     "user_id",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine := NewEngine(DefaultEngineConfig())
			plan, err := engine.CreatePlan(tt.cfg)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if plan.ID == "" {
				t.Error("plan ID should not be empty")
			}
			if plan.Status != "active" {
				t.Errorf("expected status 'active', got %q", plan.Status)
			}
		})
	}
}

func TestListAndDeletePlans(t *testing.T) {
	engine := NewEngine(DefaultEngineConfig())

	cfg := JoinConfig{
		LeftEntity:  "users",
		RightEntity: "transactions",
		JoinKey:     "user_id",
	}

	plan1, err := engine.CreatePlan(cfg)
	if err != nil {
		t.Fatalf("creating plan1: %v", err)
	}
	plan2, err := engine.CreatePlan(cfg)
	if err != nil {
		t.Fatalf("creating plan2: %v", err)
	}

	plans := engine.ListPlans()
	if len(plans) != 2 {
		t.Fatalf("expected 2 plans, got %d", len(plans))
	}

	// GetPlan
	got, err := engine.GetPlan(plan1.ID)
	if err != nil {
		t.Fatalf("getting plan1: %v", err)
	}
	if got.ID != plan1.ID {
		t.Errorf("expected plan ID %q, got %q", plan1.ID, got.ID)
	}

	// GetPlan not found
	_, err = engine.GetPlan("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent plan")
	}

	// DeletePlan
	if err := engine.DeletePlan(plan2.ID); err != nil {
		t.Fatalf("deleting plan2: %v", err)
	}
	plans = engine.ListPlans()
	if len(plans) != 1 {
		t.Fatalf("expected 1 plan after delete, got %d", len(plans))
	}

	// Delete nonexistent
	if err := engine.DeletePlan("nonexistent"); err == nil {
		t.Fatal("expected error deleting nonexistent plan")
	}
}

func TestExecuteInnerJoin(t *testing.T) {
	engine := NewEngine(DefaultEngineConfig())
	now := time.Now().UnixMilli()

	plan, err := engine.CreatePlan(JoinConfig{
		LeftEntity:  "users",
		RightEntity: "transactions",
		JoinKey:     "user_id",
		JoinType:    Inner,
		Window:      10 * time.Minute,
		Watermark:   1 * time.Hour,
	})
	if err != nil {
		t.Fatalf("creating plan: %v", err)
	}

	leftData := map[string]map[string]*FeatureValue{
		"entity1": {"age": {Value: 25, Timestamp: now}},
		"entity2": {"age": {Value: 30, Timestamp: now}},
		"entity3": {"age": {Value: 35, Timestamp: now}},
	}
	rightData := map[string]map[string]*FeatureValue{
		"entity1": {"amount": {Value: 100.0, Timestamp: now}},
		"entity2": {"amount": {Value: 200.0, Timestamp: now}},
		"entity4": {"amount": {Value: 400.0, Timestamp: now}},
	}

	output, err := engine.ExecuteJoin(context.Background(), plan.ID, leftData, rightData)
	if err != nil {
		t.Fatalf("executing join: %v", err)
	}

	// Inner join: only entity1 and entity2 match
	if len(output.Results) != 2 {
		t.Errorf("expected 2 results, got %d", len(output.Results))
	}
	if output.LeftUnmatched != 1 {
		t.Errorf("expected 1 left unmatched, got %d", output.LeftUnmatched)
	}
	if output.RightUnmatched != 1 {
		t.Errorf("expected 1 right unmatched, got %d", output.RightUnmatched)
	}
}

func TestExecuteLeftJoin(t *testing.T) {
	engine := NewEngine(DefaultEngineConfig())
	now := time.Now().UnixMilli()

	plan, err := engine.CreatePlan(JoinConfig{
		LeftEntity:  "users",
		RightEntity: "transactions",
		JoinKey:     "user_id",
		JoinType:    Left,
		Window:      10 * time.Minute,
		Watermark:   1 * time.Hour,
	})
	if err != nil {
		t.Fatalf("creating plan: %v", err)
	}

	leftData := map[string]map[string]*FeatureValue{
		"entity1": {"age": {Value: 25, Timestamp: now}},
		"entity2": {"age": {Value: 30, Timestamp: now}},
	}
	rightData := map[string]map[string]*FeatureValue{
		"entity1": {"amount": {Value: 100.0, Timestamp: now}},
	}

	output, err := engine.ExecuteJoin(context.Background(), plan.ID, leftData, rightData)
	if err != nil {
		t.Fatalf("executing join: %v", err)
	}

	// Left join: entity1 matched, entity2 unmatched but included
	if len(output.Results) != 2 {
		t.Errorf("expected 2 results for left join, got %d", len(output.Results))
	}
}

func TestExecuteWithWatermarkFiltering(t *testing.T) {
	engine := NewEngine(DefaultEngineConfig())
	oldTimestamp := time.Now().Add(-2 * time.Hour).UnixMilli()
	newTimestamp := time.Now().UnixMilli()

	plan, err := engine.CreatePlan(JoinConfig{
		LeftEntity:  "users",
		RightEntity: "transactions",
		JoinKey:     "user_id",
		JoinType:    Inner,
		Window:      10 * time.Minute,
		Watermark:   30 * time.Minute,
	})
	if err != nil {
		t.Fatalf("creating plan: %v", err)
	}

	leftData := map[string]map[string]*FeatureValue{
		"old_entity":  {"age": {Value: 25, Timestamp: oldTimestamp}},
		"new_entity":  {"age": {Value: 30, Timestamp: newTimestamp}},
	}
	rightData := map[string]map[string]*FeatureValue{
		"old_entity":  {"amount": {Value: 100.0, Timestamp: oldTimestamp}},
		"new_entity":  {"amount": {Value: 200.0, Timestamp: newTimestamp}},
	}

	output, err := engine.ExecuteJoin(context.Background(), plan.ID, leftData, rightData)
	if err != nil {
		t.Fatalf("executing join: %v", err)
	}

	// Only new_entity should match; old_entity filtered by watermark
	if len(output.Results) != 1 {
		t.Errorf("expected 1 result after watermark filtering, got %d", len(output.Results))
	}
	if len(output.Results) > 0 && output.Results[0].EntityKey != "new_entity" {
		t.Errorf("expected new_entity, got %q", output.Results[0].EntityKey)
	}
}

func TestExecuteJoinPlanNotFound(t *testing.T) {
	engine := NewEngine(DefaultEngineConfig())
	_, err := engine.ExecuteJoin(context.Background(), "nonexistent", nil, nil)
	if err == nil {
		t.Fatal("expected error for nonexistent plan")
	}
}

func TestMaxPlansLimit(t *testing.T) {
	engine := NewEngine(EngineConfig{
		MaxPlans:         2,
		DefaultWindow:    5 * time.Minute,
		DefaultWatermark: 1 * time.Minute,
		MaxBufferSize:    100,
	})

	cfg := JoinConfig{
		LeftEntity:  "users",
		RightEntity: "transactions",
		JoinKey:     "user_id",
	}

	if _, err := engine.CreatePlan(cfg); err != nil {
		t.Fatalf("creating plan 1: %v", err)
	}
	if _, err := engine.CreatePlan(cfg); err != nil {
		t.Fatalf("creating plan 2: %v", err)
	}
	if _, err := engine.CreatePlan(cfg); err == nil {
		t.Fatal("expected error when exceeding max plans")
	}
}
