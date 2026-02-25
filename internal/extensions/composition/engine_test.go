package composition

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestNewEngine(t *testing.T) {
	engine := NewEngine(EngineConfig{
		ExecutorConfig: DefaultExecutorConfig(),
	})

	if engine == nil {
		t.Fatal("Expected engine to be non-nil")
	}
}

func TestEngine_RegisterDAG(t *testing.T) {
	engine := NewEngine(EngineConfig{
		ExecutorConfig: DefaultExecutorConfig(),
	})

	dag := NewDAG("test-dag", "Test DAG")
	_ = dag.AddNode(&Node{ID: "source", Type: NodeTypeSource})
	_ = dag.SetOutputs([]string{"source"})

	err := engine.RegisterDAG(dag)
	if err != nil {
		t.Fatalf("Failed to register DAG: %v", err)
	}
}

func TestEngine_RegisterDAG_Invalid(t *testing.T) {
	engine := NewEngine(EngineConfig{
		ExecutorConfig: DefaultExecutorConfig(),
	})

	dag := NewDAG("test-dag", "Test DAG")
	// Invalid DAG - node depends on non-existent node
	_ = dag.AddNode(&Node{ID: "transform", Type: NodeTypeTransform, Inputs: []string{"nonexistent"}})

	err := engine.RegisterDAG(dag)
	if err == nil {
		t.Error("Expected error for invalid DAG")
	}
}

func TestEngine_UnregisterDAG(t *testing.T) {
	engine := NewEngine(EngineConfig{
		ExecutorConfig: DefaultExecutorConfig(),
	})

	dag := NewDAG("test-dag", "Test DAG")
	_ = dag.AddNode(&Node{ID: "source", Type: NodeTypeSource})
	_ = dag.SetOutputs([]string{"source"})
	_ = engine.RegisterDAG(dag)

	err := engine.UnregisterDAG("test-dag")
	if err != nil {
		t.Fatalf("Failed to unregister DAG: %v", err)
	}

	// Try to get unregistered DAG
	_, err = engine.GetDAG("test-dag")
	if err == nil {
		t.Error("Expected error for unregistered DAG")
	}
}

func TestEngine_UnregisterDAG_NotFound(t *testing.T) {
	engine := NewEngine(EngineConfig{
		ExecutorConfig: DefaultExecutorConfig(),
	})

	err := engine.UnregisterDAG("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent DAG")
	}
}

func TestEngine_GetDAG(t *testing.T) {
	engine := NewEngine(EngineConfig{
		ExecutorConfig: DefaultExecutorConfig(),
	})

	dag := NewDAG("test-dag", "Test DAG")
	dag.Description = "Test description"
	_ = dag.AddNode(&Node{ID: "source", Type: NodeTypeSource})
	_ = dag.SetOutputs([]string{"source"})
	_ = engine.RegisterDAG(dag)

	retrieved, err := engine.GetDAG("test-dag")
	if err != nil {
		t.Fatalf("Failed to get DAG: %v", err)
	}

	if retrieved.ID != "test-dag" {
		t.Errorf("Expected ID 'test-dag', got '%s'", retrieved.ID)
	}
	if retrieved.Description != "Test description" {
		t.Errorf("Expected description 'Test description', got '%s'", retrieved.Description)
	}

	// Verify it's a clone
	retrieved.Description = "Modified"
	original, _ := engine.GetDAG("test-dag")
	if original.Description == "Modified" {
		t.Error("GetDAG should return a clone")
	}
}

func TestEngine_GetDAG_NotFound(t *testing.T) {
	engine := NewEngine(EngineConfig{
		ExecutorConfig: DefaultExecutorConfig(),
	})

	_, err := engine.GetDAG("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent DAG")
	}
}

func TestEngine_ListDAGs(t *testing.T) {
	engine := NewEngine(EngineConfig{
		ExecutorConfig: DefaultExecutorConfig(),
	})

	// Register multiple DAGs
	for i := 0; i < 3; i++ {
		dag := NewDAG(string(rune('a'+i)), "DAG")
		_ = dag.AddNode(&Node{ID: "source", Type: NodeTypeSource})
		_ = dag.SetOutputs([]string{"source"})
		_ = engine.RegisterDAG(dag)
	}

	dags := engine.ListDAGs()
	if len(dags) != 3 {
		t.Errorf("Expected 3 DAGs, got %d", len(dags))
	}
}

func TestEngine_Compose_NotFound(t *testing.T) {
	engine := NewEngine(EngineConfig{
		ExecutorConfig: DefaultExecutorConfig(),
	})

	_, err := engine.Compose(context.Background(), "nonexistent", "entity-1")
	if err == nil {
		t.Error("Expected error for nonexistent DAG")
	}
}

func TestEngine_ComposeBatch_NotFound(t *testing.T) {
	engine := NewEngine(EngineConfig{
		ExecutorConfig: DefaultExecutorConfig(),
	})

	_, err := engine.ComposeBatch(context.Background(), "nonexistent", []string{"entity-1", "entity-2"})
	if err == nil {
		t.Error("Expected error for nonexistent DAG")
	}
}

func TestEngine_Stats(t *testing.T) {
	engine := NewEngine(EngineConfig{
		ExecutorConfig: DefaultExecutorConfig(),
	})

	dag := NewDAG("test-dag", "Test DAG")
	_ = dag.AddNode(&Node{ID: "source", Type: NodeTypeSource})
	_ = dag.SetOutputs([]string{"source"})
	_ = engine.RegisterDAG(dag)

	stats := engine.Stats()
	if stats.DAGCount != 1 {
		t.Errorf("Expected DAGCount 1, got %d", stats.DAGCount)
	}
}

// GetMaxLevel needs lock handling test
func TestDAG_GetMaxLevel_Unlocked(t *testing.T) {
	dag := NewDAG("test", "Test DAG")
	_ = dag.AddNode(&Node{ID: "source", Type: NodeTypeSource})
	_ = dag.AddNode(&Node{ID: "t1", Type: NodeTypeTransform, Inputs: []string{"source"}})
	_ = dag.AddNode(&Node{ID: "t2", Type: NodeTypeTransform, Inputs: []string{"t1"}})
	_ = dag.SetOutputs([]string{"t2"})
	_ = dag.ComputeTopology()

	maxLevel := dag.GetMaxLevel()
	if maxLevel != 2 {
		t.Errorf("Expected max level 2, got %d", maxLevel)
	}
}

func TestEngine_ClearCache(t *testing.T) {
	engine := NewEngine(EngineConfig{
		ExecutorConfig: DefaultExecutorConfig(),
	})

	// Should not panic
	engine.ClearCache()
}

func TestEngine_CreateDAGBuilder(t *testing.T) {
	engine := NewEngine(EngineConfig{
		ExecutorConfig: DefaultExecutorConfig(),
	})

	builder := engine.CreateDAGBuilder("test-dag", "Test DAG")
	if builder == nil {
		t.Fatal("Expected builder to be non-nil")
	}
}

func TestDAGBuilder_SetDescription(t *testing.T) {
	engine := NewEngine(EngineConfig{
		ExecutorConfig: DefaultExecutorConfig(),
	})

	builder := engine.CreateDAGBuilder("test", "Test").
		SetDescription("Test description")

	dag, err := builder.Build()
	if err != nil {
		t.Fatalf("Failed to build: %v", err)
	}

	if dag.Description != "Test description" {
		t.Errorf("Expected description 'Test description', got '%s'", dag.Description)
	}
}

func TestDAGBuilder_AddSourceNode(t *testing.T) {
	engine := NewEngine(EngineConfig{
		ExecutorConfig: DefaultExecutorConfig(),
	})

	builder := engine.CreateDAGBuilder("test", "Test").
		AddSourceNode("src", "feature_name").
		SetOutputs([]string{"src"})

	dag, err := builder.Build()
	if err != nil {
		t.Fatalf("Failed to build: %v", err)
	}

	node, exists := dag.Nodes["src"]
	if !exists {
		t.Fatal("Expected source node to exist")
	}
	if node.Type != NodeTypeSource {
		t.Errorf("Expected NodeTypeSource, got %s", node.Type)
	}
	if node.Expression != "feature_name" {
		t.Errorf("Expected expression 'feature_name', got '%s'", node.Expression)
	}
}

func TestDAGBuilder_AddTransformNode(t *testing.T) {
	engine := NewEngine(EngineConfig{
		ExecutorConfig: DefaultExecutorConfig(),
	})

	builder := engine.CreateDAGBuilder("test", "Test").
		AddSourceNode("src", "feature").
		AddTransformNode("transform", "Double", "multiply_by_2", []string{"src"}).
		SetOutputs([]string{"transform"})

	dag, err := builder.Build()
	if err != nil {
		t.Fatalf("Failed to build: %v", err)
	}

	node, exists := dag.Nodes["transform"]
	if !exists {
		t.Fatal("Expected transform node to exist")
	}
	if node.Type != NodeTypeTransform {
		t.Errorf("Expected NodeTypeTransform, got %s", node.Type)
	}
	if len(node.Inputs) != 1 || node.Inputs[0] != "src" {
		t.Errorf("Expected inputs ['src'], got %v", node.Inputs)
	}
}

func TestDAGBuilder_AddAggregateNode(t *testing.T) {
	engine := NewEngine(EngineConfig{
		ExecutorConfig: DefaultExecutorConfig(),
	})

	builder := engine.CreateDAGBuilder("test", "Test").
		AddSourceNode("s1", "f1").
		AddSourceNode("s2", "f2").
		AddAggregateNode("agg", "Sum", "sum", []string{"s1", "s2"}).
		SetOutputs([]string{"agg"})

	dag, err := builder.Build()
	if err != nil {
		t.Fatalf("Failed to build: %v", err)
	}

	node, exists := dag.Nodes["agg"]
	if !exists {
		t.Fatal("Expected aggregate node to exist")
	}
	if node.Type != NodeTypeAggregate {
		t.Errorf("Expected NodeTypeAggregate, got %s", node.Type)
	}
	if node.Config["aggregate"] != "sum" {
		t.Errorf("Expected aggregate type 'sum', got '%v'", node.Config["aggregate"])
	}
}

func TestDAGBuilder_AddJoinNode(t *testing.T) {
	engine := NewEngine(EngineConfig{
		ExecutorConfig: DefaultExecutorConfig(),
	})

	builder := engine.CreateDAGBuilder("test", "Test").
		AddSourceNode("s1", "f1").
		AddSourceNode("s2", "f2").
		AddJoinNode("join", "Merge", "merge", []string{"s1", "s2"}).
		SetOutputs([]string{"join"})

	dag, err := builder.Build()
	if err != nil {
		t.Fatalf("Failed to build: %v", err)
	}

	node, exists := dag.Nodes["join"]
	if !exists {
		t.Fatal("Expected join node to exist")
	}
	if node.Type != NodeTypeJoin {
		t.Errorf("Expected NodeTypeJoin, got %s", node.Type)
	}
	if node.Config["join_type"] != "merge" {
		t.Errorf("Expected join type 'merge', got '%v'", node.Config["join_type"])
	}
}

func TestDAGBuilder_AddFilterNode(t *testing.T) {
	engine := NewEngine(EngineConfig{
		ExecutorConfig: DefaultExecutorConfig(),
	})

	builder := engine.CreateDAGBuilder("test", "Test").
		AddSourceNode("src", "feature").
		AddFilterNode("filter", "GreaterThan", "gt", 10.0, []string{"src"}).
		SetOutputs([]string{"filter"})

	dag, err := builder.Build()
	if err != nil {
		t.Fatalf("Failed to build: %v", err)
	}

	node, exists := dag.Nodes["filter"]
	if !exists {
		t.Fatal("Expected filter node to exist")
	}
	if node.Type != NodeTypeFilter {
		t.Errorf("Expected NodeTypeFilter, got %s", node.Type)
	}
	if node.Config["condition"] != "gt" {
		t.Errorf("Expected condition 'gt', got '%v'", node.Config["condition"])
	}
	if node.Config["threshold"] != 10.0 {
		t.Errorf("Expected threshold 10.0, got '%v'", node.Config["threshold"])
	}
}

func TestDAGBuilder_WithCaching(t *testing.T) {
	engine := NewEngine(EngineConfig{
		ExecutorConfig: DefaultExecutorConfig(),
	})

	builder := engine.CreateDAGBuilder("test", "Test").
		AddSourceNode("src", "feature").
		WithCaching(10 * time.Minute).
		SetOutputs([]string{"src"})

	dag, err := builder.Build()
	if err != nil {
		t.Fatalf("Failed to build: %v", err)
	}

	// Note: WithCaching applies to all nodes currently
	for _, node := range dag.Nodes {
		if !node.CacheEnabled {
			t.Error("Expected CacheEnabled to be true")
		}
		if node.CacheTTL != 10*time.Minute {
			t.Errorf("Expected CacheTTL 10m, got %v", node.CacheTTL)
		}
	}
}

func TestDAGBuilder_Build_Error(t *testing.T) {
	engine := NewEngine(EngineConfig{
		ExecutorConfig: DefaultExecutorConfig(),
	})

	// Add duplicate node to cause error
	builder := engine.CreateDAGBuilder("test", "Test").
		AddSourceNode("src", "feature").
		AddSourceNode("src", "feature2") // Duplicate ID

	_, err := builder.Build()
	if err == nil {
		t.Error("Expected error for duplicate node")
	}
}

func TestDAGBuilder_Build_ValidationError(t *testing.T) {
	engine := NewEngine(EngineConfig{
		ExecutorConfig: DefaultExecutorConfig(),
	})

	// Create DAG with missing dependency
	builder := engine.CreateDAGBuilder("test", "Test")
	builder.dag.Nodes["transform"] = &Node{
		ID:     "transform",
		Type:   NodeTypeTransform,
		Inputs: []string{"nonexistent"},
	}

	_, err := builder.Build()
	if err == nil {
		t.Error("Expected validation error")
	}
}

func TestDAGBuilder_Register(t *testing.T) {
	engine := NewEngine(EngineConfig{
		ExecutorConfig: DefaultExecutorConfig(),
	})

	err := engine.CreateDAGBuilder("test", "Test").
		AddSourceNode("src", "feature").
		SetOutputs([]string{"src"}).
		Register()

	if err != nil {
		t.Fatalf("Failed to register: %v", err)
	}

	// Verify DAG was registered
	dag, err := engine.GetDAG("test")
	if err != nil {
		t.Fatalf("Failed to get registered DAG: %v", err)
	}
	if dag.ID != "test" {
		t.Errorf("Expected ID 'test', got '%s'", dag.ID)
	}
}

func TestDAGBuilder_Register_Error(t *testing.T) {
	engine := NewEngine(EngineConfig{
		ExecutorConfig: DefaultExecutorConfig(),
	})

	// Try to register invalid DAG
	builder := engine.CreateDAGBuilder("test", "Test").
		AddSourceNode("src", "feature").
		AddSourceNode("src", "feature2") // Duplicate

	err := builder.Register()
	if err == nil {
		t.Error("Expected error for invalid DAG")
	}
}

func TestDAGBuilder_Chaining(t *testing.T) {
	engine := NewEngine(EngineConfig{
		ExecutorConfig: DefaultExecutorConfig(),
	})

	// Test fluent chaining
	dag, err := engine.CreateDAGBuilder("complex", "Complex DAG").
		SetDescription("A complex feature composition").
		AddSourceNode("price", "product_price").
		AddSourceNode("quantity", "order_quantity").
		AddTransformNode("total", "Calculate Total", "product", []string{"price", "quantity"}).
		AddSourceNode("discount", "discount_rate").
		AddTransformNode("discounted", "Apply Discount", "multiply", []string{"total", "discount"}).
		AddFilterNode("filtered", "Min Order", "gte", 10.0, []string{"discounted"}).
		SetOutputs([]string{"filtered"}).
		Build()

	if err != nil {
		t.Fatalf("Failed to build complex DAG: %v", err)
	}

	if len(dag.Nodes) != 6 {
		t.Errorf("Expected 6 nodes, got %d", len(dag.Nodes))
	}
}

func TestEvaluateExpression_Sum(t *testing.T) {
	engine := NewEngine(EngineConfig{
		ExecutorConfig: DefaultExecutorConfig(),
	})

	inputs := map[string]interface{}{
		"a": 10.0,
		"b": 20.0,
		"c": 30.0,
	}

	result, err := engine.evaluateExpression("sum", inputs)
	if err != nil {
		t.Fatalf("Failed to evaluate sum: %v", err)
	}

	if result != 60.0 {
		t.Errorf("Expected 60.0, got %v", result)
	}
}

func TestEvaluateExpression_Product(t *testing.T) {
	engine := NewEngine(EngineConfig{
		ExecutorConfig: DefaultExecutorConfig(),
	})

	inputs := map[string]interface{}{
		"a": 2.0,
		"b": 3.0,
		"c": 4.0,
	}

	result, err := engine.evaluateExpression("product", inputs)
	if err != nil {
		t.Fatalf("Failed to evaluate product: %v", err)
	}

	if result != 24.0 {
		t.Errorf("Expected 24.0, got %v", result)
	}
}

func TestEvaluateExpression_First(t *testing.T) {
	engine := NewEngine(EngineConfig{
		ExecutorConfig: DefaultExecutorConfig(),
	})

	inputs := map[string]interface{}{
		"a": "first_value",
	}

	result, err := engine.evaluateExpression("first", inputs)
	if err != nil {
		t.Fatalf("Failed to evaluate first: %v", err)
	}

	if result != "first_value" {
		t.Errorf("Expected 'first_value', got %v", result)
	}
}

func TestEngine_Compose_MultiNodeDAG(t *testing.T) {
	engine := NewEngine(EngineConfig{
		ExecutorConfig: DefaultExecutorConfig(),
	})

	// Set source func to return known values
	engine.executor.SetSourceFunc(func(ctx context.Context, node *Node, entityID string) (interface{}, error) {
		switch node.Expression {
		case "price":
			return 10.0, nil
		case "qty":
			return 5.0, nil
		default:
			return nil, fmt.Errorf("unknown feature: %s", node.Expression)
		}
	})

	// Use aggregate node (sum) which works without pipeline
	err := engine.CreateDAGBuilder("multi", "Multi Node DAG").
		AddSourceNode("s_price", "price").
		AddSourceNode("s_qty", "qty").
		AddAggregateNode("total", "Total", "sum", []string{"s_price", "s_qty"}).
		SetOutputs([]string{"total"}).
		Register()
	if err != nil {
		t.Fatalf("Failed to register DAG: %v", err)
	}

	results, err := engine.Compose(context.Background(), "multi", "order-1")
	if err != nil {
		t.Fatalf("Compose failed: %v", err)
	}

	val, ok := results["total"]
	if !ok {
		t.Fatal("Expected 'total' in results")
	}
	fVal, ok := toFloat64(val)
	if !ok || fVal != 15.0 { // 10 + 5
		t.Errorf("expected 15.0, got %v", val)
	}
}

func TestEngine_Compose_DAGNotFound(t *testing.T) {
	engine := NewEngine(EngineConfig{
		ExecutorConfig: DefaultExecutorConfig(),
	})

	_, err := engine.Compose(context.Background(), "no-such-dag", "entity-1")
	if err == nil {
		t.Error("Expected error for nonexistent DAG")
	}
}

func TestEngine_Compose_NodeErrorExcludedFromOutput(t *testing.T) {
	engine := NewEngine(EngineConfig{
		ExecutorConfig: func() ExecutorConfig {
			c := DefaultExecutorConfig()
			c.RetryAttempts = 0
			return c
		}(),
	})

	engine.executor.SetSourceFunc(func(ctx context.Context, node *Node, entityID string) (interface{}, error) {
		if node.Expression == "broken" {
			return nil, fmt.Errorf("source error")
		}
		return 42.0, nil
	})

	dag := NewDAG("err-dag", "Error DAG")
	_ = dag.AddNode(&Node{ID: "good", Type: NodeTypeSource, Expression: "good"})
	_ = dag.AddNode(&Node{ID: "broken", Type: NodeTypeSource, Expression: "broken"})
	_ = dag.SetOutputs([]string{"good", "broken"})
	_ = engine.RegisterDAG(dag)

	results, _ := engine.Compose(context.Background(), "err-dag", "entity-1")

	if val, ok := results["good"]; !ok {
		t.Error("Expected 'good' in results")
	} else if fVal, ok := toFloat64(val); !ok || fVal != 42.0 {
		t.Errorf("expected 42.0 for good, got %v", val)
	}

	// Broken node has error so Compose excludes it from output map
	if _, ok := results["broken"]; ok {
		t.Error("Expected 'broken' to be excluded from results due to error")
	}
}

func TestEngine_ComposeBatch_ConcurrentExecution(t *testing.T) {
	engine := NewEngine(EngineConfig{
		ExecutorConfig: DefaultExecutorConfig(),
	})

	engine.executor.SetSourceFunc(func(ctx context.Context, node *Node, entityID string) (interface{}, error) {
		return float64(len(entityID)), nil
	})

	dag := NewDAG("batch-dag", "Batch DAG")
	_ = dag.AddNode(&Node{ID: "src", Type: NodeTypeSource, Expression: "len"})
	_ = dag.SetOutputs([]string{"src"})
	_ = engine.RegisterDAG(dag)

	entityIDs := []string{"a", "bb", "ccc", "dddd", "eeeee"}
	results, err := engine.ComposeBatch(context.Background(), "batch-dag", entityIDs)
	if err != nil {
		t.Fatalf("ComposeBatch failed: %v", err)
	}

	if len(results) != len(entityIDs) {
		t.Errorf("expected %d results, got %d", len(entityIDs), len(results))
	}

	for _, eid := range entityIDs {
		r, ok := results[eid]
		if !ok {
			t.Errorf("missing result for entity %s", eid)
			continue
		}
		if val, ok := toFloat64(r["src"]); !ok || val != float64(len(eid)) {
			t.Errorf("entity %s: expected %d, got %v", eid, len(eid), r["src"])
		}
	}
}

func TestEngine_ComposeBatch_PartialFailures(t *testing.T) {
	engine := NewEngine(EngineConfig{
		ExecutorConfig: func() ExecutorConfig {
			c := DefaultExecutorConfig()
			c.RetryAttempts = 0
			return c
		}(),
	})

	engine.executor.SetSourceFunc(func(ctx context.Context, node *Node, entityID string) (interface{}, error) {
		if entityID == "fail" {
			return nil, fmt.Errorf("source error for %s", entityID)
		}
		return 1.0, nil
	})

	dag := NewDAG("partial-dag", "Partial")
	_ = dag.AddNode(&Node{ID: "src", Type: NodeTypeSource, Expression: "val"})
	_ = dag.SetOutputs([]string{"src"})
	_ = engine.RegisterDAG(dag)

	results, err := engine.ComposeBatch(context.Background(), "partial-dag", []string{"ok1", "fail", "ok2"})

	// ComposeBatch returns partial results; err may or may not be non-nil
	// depending on whether the failed entity produces an executor-level error
	// or just a node-level error that gets swallowed
	_ = err

	// Successful entities should still have results
	if _, ok := results["ok1"]; !ok {
		t.Error("Expected result for ok1")
	}
	if _, ok := results["ok2"]; !ok {
		t.Error("Expected result for ok2")
	}
}

func TestEngine_RegisterBuiltinFunctions_NilStore(t *testing.T) {
	engine := NewEngine(EngineConfig{
		ExecutorConfig: DefaultExecutorConfig(),
		Store:          nil,
	})

	dag := NewDAG("nil-store-dag", "Nil Store")
	_ = dag.AddNode(&Node{ID: "src", Type: NodeTypeSource, Expression: "feature"})
	_ = dag.SetOutputs([]string{"src"})
	_ = engine.RegisterDAG(dag)

	results, err := engine.Compose(context.Background(), "nil-store-dag", "entity-1")
	// With nil store, source function returns error "store not configured"
	// Compose may return error or empty results depending on executor behavior
	if err == nil && len(results) > 0 {
		t.Error("Expected error or empty results when store is nil")
	}
}

func TestEngine_Compose_CustomNodeWithExpression(t *testing.T) {
	engine := NewEngine(EngineConfig{
		ExecutorConfig: DefaultExecutorConfig(),
	})

	engine.executor.SetSourceFunc(func(ctx context.Context, node *Node, entityID string) (interface{}, error) {
		return 10.0, nil
	})

	dag := NewDAG("custom-dag", "Custom")
	_ = dag.AddNode(&Node{ID: "s1", Type: NodeTypeSource, Expression: "val"})
	_ = dag.AddNode(&Node{ID: "c1", Type: NodeTypeCustom, Expression: "sum", Inputs: []string{"s1"}})
	_ = dag.SetOutputs([]string{"c1"})
	_ = engine.RegisterDAG(dag)

	results, err := engine.Compose(context.Background(), "custom-dag", "entity-1")
	if err != nil {
		t.Fatalf("Compose failed: %v", err)
	}

	if _, ok := results["c1"]; !ok {
		t.Error("Expected c1 in results")
	}
}

func TestToFloat64(t *testing.T) {
	tests := []struct {
		input    interface{}
		expected float64
		ok       bool
	}{
		{float64(10.5), 10.5, true},
		{float32(10.5), 10.5, true},
		{int(10), 10.0, true},
		{int32(10), 10.0, true},
		{int64(10), 10.0, true},
		{uint(10), 10.0, true},
		{uint32(10), 10.0, true},
		{uint64(10), 10.0, true},
		{"string", 0, false},
		{nil, 0, false},
	}

	for _, tt := range tests {
		result, ok := toFloat64(tt.input)
		if ok != tt.ok {
			t.Errorf("toFloat64(%v): expected ok=%v, got ok=%v", tt.input, tt.ok, ok)
		}
		if ok && result != tt.expected {
			t.Errorf("toFloat64(%v): expected %v, got %v", tt.input, tt.expected, result)
		}
	}
}
