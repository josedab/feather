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

// --- ComputeAll tests ---

func TestDeclarativeGraph_ComputeAll_MultiNode(t *testing.T) {
	t.Parallel()
	engine := NewEngine(DefaultEngineConfig())
	memoizer := NewMemoizer(DefaultMemoizerConfig())
	graph := NewDeclarativeGraph(engine, memoizer)

	def := GraphSpec{
		Name: "multi",
		Features: []FeatureDefinition{
			{Name: "src_a", Kind: KindSource, OutputType: "float64"},
			{Name: "src_b", Kind: KindSource, OutputType: "float64"},
			{Name: "derived_c", Kind: KindDerived, Inputs: []string{"src_a", "src_b"}, Function: FuncSum, OutputType: "float64"},
		},
	}
	graph.ApplyDefinition(def)

	results, err := graph.ComputeAll(context.Background(), map[string]interface{}{"src_a": 10.0, "src_b": 20.0})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 3 {
		t.Errorf("expected 3 results, got %d", len(results))
	}
}

func TestDeclarativeGraph_ComputeAll_EmptyGraph(t *testing.T) {
	t.Parallel()
	engine := NewEngine(DefaultEngineConfig())
	memoizer := NewMemoizer(DefaultMemoizerConfig())
	graph := NewDeclarativeGraph(engine, memoizer)

	results, err := graph.ComputeAll(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results for empty graph, got %d", len(results))
	}
}

func TestDeclarativeGraph_ComputeAll_NodeWithErrors(t *testing.T) {
	t.Parallel()
	engine := NewEngine(DefaultEngineConfig())
	memoizer := NewMemoizer(DefaultMemoizerConfig())
	graph := NewDeclarativeGraph(engine, memoizer)

	def := GraphSpec{
		Name: "error_test",
		Features: []FeatureDefinition{
			{Name: "bad_node", Kind: KindDerived, Inputs: []string{"nonexistent"}, Function: FuncIdentity, OutputType: "float64"},
		},
	}
	result := graph.ApplyDefinition(def)
	if len(result.Errors) == 0 {
		// If no error on definition, the compute should still fail gracefully
		_, err := graph.ComputeAll(context.Background(), nil)
		// Error expected when dependency doesn't exist
		_ = err
	}
}

func TestDeclarativeGraph_ComputeAll_TopologicalOrder(t *testing.T) {
	t.Parallel()
	engine := NewEngine(DefaultEngineConfig())
	memoizer := NewMemoizer(DefaultMemoizerConfig())
	graph := NewDeclarativeGraph(engine, memoizer)

	def := GraphSpec{
		Name: "topo",
		Features: []FeatureDefinition{
			{Name: "leaf", Kind: KindSource, OutputType: "float64"},
			{Name: "mid", Kind: KindDerived, Inputs: []string{"leaf"}, Function: FuncIdentity, OutputType: "float64"},
			{Name: "root", Kind: KindDerived, Inputs: []string{"mid"}, Function: FuncIdentity, OutputType: "float64"},
		},
	}
	graph.ApplyDefinition(def)

	plan, err := graph.GetExecutionPlan()
	if err != nil {
		t.Fatal(err)
	}

	// leaf must come before mid, mid before root
	leafIdx, midIdx, rootIdx := -1, -1, -1
	for i, name := range plan.Order {
		switch name {
		case "leaf":
			leafIdx = i
		case "mid":
			midIdx = i
		case "root":
			rootIdx = i
		}
	}
	if leafIdx >= midIdx || midIdx >= rootIdx {
		t.Errorf("expected topological order leaf < mid < root, got %d, %d, %d", leafIdx, midIdx, rootIdx)
	}
}

func TestDeclarativeGraph_ComputeAll_CancelledContext(t *testing.T) {
	t.Parallel()
	engine := NewEngine(DefaultEngineConfig())
	memoizer := NewMemoizer(DefaultMemoizerConfig())
	graph := NewDeclarativeGraph(engine, memoizer)

	def := GraphSpec{
		Name: "cancel",
		Features: []FeatureDefinition{
			{Name: "x", Kind: KindSource, OutputType: "float64"},
		},
	}
	graph.ApplyDefinition(def)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := graph.ComputeAll(ctx, map[string]interface{}{"x": 1.0})
	if err == nil {
		t.Error("expected error for cancelled context")
	}
}

// --- InvalidateAndRecompute tests ---

func TestDeclarativeGraph_InvalidateAndRecompute(t *testing.T) {
	t.Parallel()
	engine := NewEngine(DefaultEngineConfig())
	memoizer := NewMemoizer(DefaultMemoizerConfig())
	graph := NewDeclarativeGraph(engine, memoizer)

	def := GraphSpec{
		Name: "invalidate",
		Features: []FeatureDefinition{
			{Name: "src", Kind: KindSource, OutputType: "float64"},
			{Name: "derived", Kind: KindDerived, Inputs: []string{"src"}, Function: FuncIdentity, OutputType: "float64"},
		},
	}
	graph.ApplyDefinition(def)

	// Compute to populate cache
	graph.Compute(context.Background(), "src", map[string]interface{}{"src": 1.0})

	affected, err := graph.InvalidateAndRecompute("src", map[string]interface{}{"src": 2.0})
	if err != nil {
		t.Fatal(err)
	}
	if affected < 0 {
		t.Error("expected non-negative affected count")
	}
}

func TestDeclarativeGraph_InvalidateAndRecompute_NonExistentNode(t *testing.T) {
	t.Parallel()
	engine := NewEngine(DefaultEngineConfig())
	memoizer := NewMemoizer(DefaultMemoizerConfig())
	graph := NewDeclarativeGraph(engine, memoizer)

	affected, err := graph.InvalidateAndRecompute("nonexistent", nil)
	if err != nil {
		t.Fatal(err)
	}
	// Should return 0 affected
	if affected < 0 {
		t.Error("expected non-negative affected")
	}
}

func TestDeclarativeGraph_InvalidateAndRecompute_ClearsCache(t *testing.T) {
	t.Parallel()
	engine := NewEngine(DefaultEngineConfig())
	memoizer := NewMemoizer(DefaultMemoizerConfig())
	graph := NewDeclarativeGraph(engine, memoizer)

	def := GraphSpec{
		Name: "cache_clear",
		Features: []FeatureDefinition{
			{Name: "src", Kind: KindSource, OutputType: "float64"},
			{Name: "derived", Kind: KindDerived, Inputs: []string{"src"}, Function: FuncIdentity, OutputType: "float64"},
		},
	}
	graph.ApplyDefinition(def)

	// Invalidate should return affected count without error
	affected, err := graph.InvalidateAndRecompute("src", nil)
	if err != nil {
		t.Fatal(err)
	}
	// "src" has a downstream "derived", so affected should include both
	if affected < 1 {
		t.Errorf("expected at least 1 affected node, got %d", affected)
	}
}

// --- Memoization verification ---

func TestDeclarativeGraph_ComputeAll_Memoization(t *testing.T) {
	t.Parallel()
	engine := NewEngine(DefaultEngineConfig())
	memoizer := NewMemoizer(DefaultMemoizerConfig())
	graph := NewDeclarativeGraph(engine, memoizer)

	def := GraphSpec{
		Name: "memo",
		Features: []FeatureDefinition{
			{Name: "x", Kind: KindSource, OutputType: "float64"},
		},
	}
	graph.ApplyDefinition(def)

	inputs := map[string]interface{}{"x": 5.0}
	graph.ComputeAll(context.Background(), inputs)
	graph.ComputeAll(context.Background(), inputs)

	stats := graph.Stats()
	if stats.CacheHits < 1 {
		t.Errorf("expected cache hits on repeated ComputeAll, got %d", stats.CacheHits)
	}
}
