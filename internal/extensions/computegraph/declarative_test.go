package computegraph

import (
	"context"
	"testing"
)

func TestDeclarativeGraphApplyDefinition(t *testing.T) {
	t.Parallel()
	engine := NewEngine(DefaultEngineConfig())
	memoizer := NewMemoizer(DefaultMemoizerConfig())
	graph := NewDeclarativeGraph(engine, memoizer)

	def := GraphSpec{
		Name:    "user_features",
		Version: "1.0",
		Features: []FeatureDefinition{
			{Name: "raw_clicks", Kind: KindSource, OutputType: "int64"},
			{Name: "raw_views", Kind: KindSource, OutputType: "int64"},
			{Name: "ctr", Kind: KindDerived, Inputs: []string{"raw_clicks", "raw_views"}, Function: FuncDivide, OutputType: "float64"},
		},
	}

	result := graph.ApplyDefinition(def)
	if result.NodesAdded != 3 {
		t.Errorf("expected 3 nodes added, got %d", result.NodesAdded)
	}
	if len(result.Errors) > 0 {
		t.Errorf("unexpected errors: %v", result.Errors)
	}
}

func TestDeclarativeGraphCycleDetection(t *testing.T) {
	t.Parallel()
	engine := NewEngine(DefaultEngineConfig())
	memoizer := NewMemoizer(DefaultMemoizerConfig())
	graph := NewDeclarativeGraph(engine, memoizer)

	def := GraphSpec{
		Name: "cyclic",
		Features: []FeatureDefinition{
			{Name: "a", Kind: KindDerived, Inputs: []string{"b"}, OutputType: "int64"},
			{Name: "b", Kind: KindDerived, Inputs: []string{"a"}, OutputType: "int64"},
		},
	}

	result := graph.ApplyDefinition(def)
	// One of the nodes should fail with cycle detection.
	if len(result.Errors) == 0 {
		t.Error("expected cycle detection error")
	}
}

func TestDeclarativeGraphCompute(t *testing.T) {
	t.Parallel()
	engine := NewEngine(DefaultEngineConfig())
	memoizer := NewMemoizer(DefaultMemoizerConfig())
	graph := NewDeclarativeGraph(engine, memoizer)

	def := GraphSpec{
		Name: "simple",
		Features: []FeatureDefinition{
			{Name: "x", Kind: KindSource, OutputType: "float64"},
		},
	}
	graph.ApplyDefinition(def)

	result, err := graph.Compute(context.Background(), "x", map[string]interface{}{"x": 42.0})
	if err != nil {
		t.Fatal(err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}

func TestDeclarativeGraphMemoization(t *testing.T) {
	t.Parallel()
	engine := NewEngine(DefaultEngineConfig())
	memoizer := NewMemoizer(DefaultMemoizerConfig())
	graph := NewDeclarativeGraph(engine, memoizer)

	def := GraphSpec{
		Name: "memo_test",
		Features: []FeatureDefinition{
			{Name: "val", Kind: KindSource, OutputType: "float64"},
		},
	}
	graph.ApplyDefinition(def)

	inputs := map[string]interface{}{"val": 1.0}
	_, _ = graph.Compute(context.Background(), "val", inputs)
	_, _ = graph.Compute(context.Background(), "val", inputs)

	stats := graph.Stats()
	if stats.CacheHits < 1 {
		t.Errorf("expected at least 1 cache hit, got %d", stats.CacheHits)
	}
}

func TestDeclarativeGraphGetLineage(t *testing.T) {
	t.Parallel()
	engine := NewEngine(DefaultEngineConfig())
	memoizer := NewMemoizer(DefaultMemoizerConfig())
	graph := NewDeclarativeGraph(engine, memoizer)

	def := GraphSpec{
		Name: "lineage_test",
		Features: []FeatureDefinition{
			{Name: "source1", Kind: KindSource, OutputType: "float64"},
			{Name: "derived1", Kind: KindDerived, Inputs: []string{"source1"}, Function: FuncIdentity, OutputType: "float64"},
		},
	}
	graph.ApplyDefinition(def)

	lineage := graph.GetLineage("derived1")
	upstream, ok := lineage["upstream"].([]string)
	if !ok || len(upstream) == 0 {
		t.Error("expected upstream dependencies")
	}
}

func TestDeclarativeGraphExecutionPlan(t *testing.T) {
	t.Parallel()
	engine := NewEngine(DefaultEngineConfig())
	memoizer := NewMemoizer(DefaultMemoizerConfig())
	graph := NewDeclarativeGraph(engine, memoizer)

	def := GraphSpec{
		Name: "plan_test",
		Features: []FeatureDefinition{
			{Name: "a", Kind: KindSource, OutputType: "int64"},
			{Name: "b", Kind: KindDerived, Inputs: []string{"a"}, Function: FuncIdentity, OutputType: "int64"},
		},
	}
	graph.ApplyDefinition(def)

	plan, err := graph.GetExecutionPlan()
	if err != nil {
		t.Fatal(err)
	}
	if plan == nil {
		t.Fatal("expected non-nil execution plan")
	}
	if len(plan.Order) != 2 {
		t.Errorf("expected 2 nodes in order, got %d", len(plan.Order))
	}
}
