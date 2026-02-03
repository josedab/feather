package queryplanner

import (
	"testing"
)

func TestNew(t *testing.T) {
	p := New(DefaultConfig())
	if p == nil {
		t.Fatal("New returned nil")
	}
	stats := p.Stats()
	if stats.TotalOptimizations != 0 {
		t.Errorf("TotalOptimizations = %d, want 0", stats.TotalOptimizations)
	}
}

func TestOptimize(t *testing.T) {
	tests := []struct {
		name    string
		query   Query
		wantErr bool
	}{
		{
			name: "single operation",
			query: Query{
				ID:         "q1",
				Operations: []Operation{{Type: OpLookup, Feature: "clicks"}},
				Features:   []string{"clicks"},
				Entities:   []string{"user:1"},
			},
		},
		{
			name: "multiple operations",
			query: Query{
				ID: "q2",
				Operations: []Operation{
					{Type: OpLookup, Feature: "clicks"},
					{Type: OpCompute, Feature: "ctr"},
					{Type: OpAggregate, Feature: "total"},
				},
				Features: []string{"clicks", "ctr", "total"},
				Entities: []string{"user:1"},
			},
		},
		{
			name: "with filters",
			query: Query{
				ID:         "q3",
				Operations: []Operation{{Type: OpLookup, Feature: "clicks"}},
				Features:   []string{"clicks"},
				Filters:    []Filter{{Feature: "clicks", Operator: ">", Value: 10}},
			},
		},
		{
			name:    "no operations",
			query:   Query{ID: "q4"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := New(DefaultConfig())
			plan, err := p.Optimize(tt.query)
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
			if len(plan.Steps) == 0 {
				t.Error("plan should have steps")
			}
			if plan.EstimatedCostMs <= 0 {
				t.Error("EstimatedCostMs should be positive")
			}
		})
	}
}

func TestOptimize_CacheHit(t *testing.T) {
	p := New(DefaultConfig())
	query := Query{
		ID:         "q1",
		Operations: []Operation{{Type: OpLookup, Feature: "clicks"}},
		Features:   []string{"clicks"},
	}

	// First call: cache miss
	plan1, err := p.Optimize(query)
	if err != nil {
		t.Fatal(err)
	}

	// Second call with same query: cache hit
	plan2, err := p.Optimize(query)
	if err != nil {
		t.Fatal(err)
	}

	if plan1.ID != plan2.ID {
		t.Error("expected same plan from cache")
	}

	stats := p.Stats()
	if stats.CacheHits < 1 {
		t.Errorf("CacheHits = %d, want >= 1", stats.CacheHits)
	}
}

func TestRecordOperationCost(t *testing.T) {
	p := New(DefaultConfig())

	// Record some costs
	p.RecordOperationCost("lookup", 0.3, 100)
	p.RecordOperationCost("lookup", 0.5, 200)
	p.RecordOperationCost("compute", 1.0, 50)

	// Estimated cost should reflect recorded data
	lookupCost := p.EstimateCost(Operation{Type: OpLookup})
	if lookupCost <= 0 {
		t.Errorf("lookup cost should be positive, got %f", lookupCost)
	}

	// Unknown operation should get default cost
	joinCost := p.EstimateCost(Operation{Type: OpJoin})
	if joinCost <= 0 {
		t.Errorf("join default cost should be positive, got %f", joinCost)
	}
}

func TestRecordExecutionResult(t *testing.T) {
	p := New(DefaultConfig())

	query := Query{
		ID:         "q1",
		Operations: []Operation{{Type: OpLookup, Feature: "clicks"}},
		Features:   []string{"clicks"},
	}
	plan, _ := p.Optimize(query)

	// Record execution result
	p.RecordExecutionResult(plan.ID, 10.0, 500)

	// Should not panic; result is stored internally
	// Verify through ShouldReplan
	if p.ShouldReplan("nonexistent") {
		t.Error("ShouldReplan should return false for unknown plan")
	}
}

func TestShouldReplan(t *testing.T) {
	tests := []struct {
		name          string
		adaptive      bool
		estimatedCost float64
		actualCost    float64
		wantReplan    bool
	}{
		{
			name:          "within threshold",
			adaptive:      true,
			estimatedCost: 0.5,
			actualCost:    0.6,
			wantReplan:    false,
		},
		{
			name:          "adaptive disabled",
			adaptive:      false,
			estimatedCost: 10.0,
			actualCost:    100.0,
			wantReplan:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig()
			cfg.EnableAdaptiveReplanning = tt.adaptive
			p := New(cfg)

			query := Query{
				ID:         "q1",
				Operations: []Operation{{Type: OpLookup, Feature: "f"}},
				Features:   []string{"f"},
			}
			plan, _ := p.Optimize(query)
			p.RecordExecutionResult(plan.ID, tt.actualCost, 100)

			got := p.ShouldReplan(plan.ID)
			if got != tt.wantReplan {
				t.Errorf("ShouldReplan = %v, want %v", got, tt.wantReplan)
			}
		})
	}
}

func TestStats(t *testing.T) {
	p := New(DefaultConfig())

	query := Query{
		ID:         "q1",
		Operations: []Operation{{Type: OpLookup, Feature: "clicks"}},
		Features:   []string{"clicks"},
	}
	p.Optimize(query)
	p.Optimize(query) // cache hit

	stats := p.Stats()
	if stats.TotalOptimizations != 2 {
		t.Errorf("TotalOptimizations = %d, want 2", stats.TotalOptimizations)
	}
	if stats.CacheHits < 1 {
		t.Errorf("CacheHits = %d, want >= 1", stats.CacheHits)
	}
	if stats.CacheMisses < 1 {
		t.Errorf("CacheMisses = %d, want >= 1", stats.CacheMisses)
	}
	if stats.ActivePlans < 1 {
		t.Errorf("ActivePlans = %d, want >= 1", stats.ActivePlans)
	}
}
