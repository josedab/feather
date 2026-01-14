package composition

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestNewExecutor(t *testing.T) {
	config := DefaultExecutorConfig()
	executor := NewExecutor(config)

	if executor == nil {
		t.Fatal("Expected executor to be non-nil")
	}
	if executor.config.MaxParallel != 10 {
		t.Errorf("Expected MaxParallel 10, got %d", executor.config.MaxParallel)
	}
}

func TestDefaultExecutorConfig(t *testing.T) {
	config := DefaultExecutorConfig()

	if config.MaxParallel != 10 {
		t.Errorf("Expected MaxParallel 10, got %d", config.MaxParallel)
	}
	if config.DefaultTimeout != 30*time.Second {
		t.Errorf("Expected DefaultTimeout 30s, got %v", config.DefaultTimeout)
	}
	if !config.EnableCaching {
		t.Error("Expected EnableCaching to be true")
	}
	if config.CacheTTL != 5*time.Minute {
		t.Errorf("Expected CacheTTL 5m, got %v", config.CacheTTL)
	}
	if config.RetryAttempts != 3 {
		t.Errorf("Expected RetryAttempts 3, got %d", config.RetryAttempts)
	}
}

func TestExecutor_RegisterComputeFunc(t *testing.T) {
	executor := NewExecutor(DefaultExecutorConfig())

	executor.RegisterComputeFunc(NodeTypeTransform, func(ctx context.Context, node *Node, inputs map[string]interface{}) (interface{}, error) {
		return "result", nil
	})

	// Verify function was registered by checking stats
	stats := executor.Stats()
	if stats.ComputeFuncs != 1 {
		t.Errorf("Expected 1 compute func, got %d", stats.ComputeFuncs)
	}
}

func TestExecutor_SetSourceFunc(t *testing.T) {
	executor := NewExecutor(DefaultExecutorConfig())

	executor.SetSourceFunc(func(ctx context.Context, node *Node, entityID string) (interface{}, error) {
		return "source-value", nil
	})

	stats := executor.Stats()
	if !stats.HasSourceFunc {
		t.Error("Expected HasSourceFunc to be true")
	}
}

func TestExecutor_Plan(t *testing.T) {
	executor := NewExecutor(DefaultExecutorConfig())
	dag := NewDAG("test", "Test DAG")

	_ = dag.AddNode(&Node{ID: "source", Type: NodeTypeSource})
	_ = dag.AddNode(&Node{ID: "transform", Type: NodeTypeTransform, Inputs: []string{"source"}})
	_ = dag.SetOutputs([]string{"transform"})

	plan, err := executor.Plan(dag, "entity-1")
	if err != nil {
		t.Fatalf("Failed to create plan: %v", err)
	}

	if plan.EntityID != "entity-1" {
		t.Errorf("Expected EntityID 'entity-1', got '%s'", plan.EntityID)
	}
	if plan.TotalNodes != 2 {
		t.Errorf("Expected 2 nodes, got %d", plan.TotalNodes)
	}
	if len(plan.Levels) != 2 {
		t.Errorf("Expected 2 levels, got %d", len(plan.Levels))
	}
}

func TestExecutor_Plan_InvalidDAG(t *testing.T) {
	executor := NewExecutor(DefaultExecutorConfig())
	dag := NewDAG("test", "Test DAG")

	// Create invalid DAG with missing dependency
	_ = dag.AddNode(&Node{ID: "node1", Type: NodeTypeTransform, Inputs: []string{"nonexistent"}})

	_, err := executor.Plan(dag, "entity-1")
	if err == nil {
		t.Error("Expected error for invalid DAG")
	}
}

func TestExecutor_Execute(t *testing.T) {
	executor := NewExecutor(DefaultExecutorConfig())

	// Set up source function
	executor.SetSourceFunc(func(ctx context.Context, node *Node, entityID string) (interface{}, error) {
		return 10.0, nil
	})

	// Register transform function
	executor.RegisterComputeFunc(NodeTypeTransform, func(ctx context.Context, node *Node, inputs map[string]interface{}) (interface{}, error) {
		sum := 0.0
		for _, v := range inputs {
			if num, ok := v.(float64); ok {
				sum += num
			}
		}
		return sum * 2, nil
	})

	dag := NewDAG("test", "Test DAG")
	_ = dag.AddNode(&Node{ID: "source", Type: NodeTypeSource})
	_ = dag.AddNode(&Node{ID: "transform", Type: NodeTypeTransform, Inputs: []string{"source"}})
	_ = dag.SetOutputs([]string{"transform"})

	results, err := executor.Execute(context.Background(), dag, "entity-1")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if len(results) != 1 {
		t.Errorf("Expected 1 result, got %d", len(results))
	}

	result, ok := results["transform"]
	if !ok {
		t.Fatal("Expected transform result")
	}

	if result.Value != 20.0 {
		t.Errorf("Expected value 20.0, got %v", result.Value)
	}
}

func TestExecutor_Execute_NoSourceFunc(t *testing.T) {
	executor := NewExecutor(DefaultExecutorConfig())

	dag := NewDAG("test", "Test DAG")
	_ = dag.AddNode(&Node{ID: "source", Type: NodeTypeSource})
	_ = dag.SetOutputs([]string{"source"})

	results, _ := executor.Execute(context.Background(), dag, "entity-1")

	// Should have error in result
	if result, ok := results["source"]; ok {
		if result.Error == "" {
			t.Error("Expected error for missing source function")
		}
	}
}

func TestExecutor_Execute_NoComputeFunc(t *testing.T) {
	executor := NewExecutor(DefaultExecutorConfig())

	executor.SetSourceFunc(func(ctx context.Context, node *Node, entityID string) (interface{}, error) {
		return 10.0, nil
	})

	dag := NewDAG("test", "Test DAG")
	_ = dag.AddNode(&Node{ID: "source", Type: NodeTypeSource})
	_ = dag.AddNode(&Node{ID: "transform", Type: NodeTypeTransform, Inputs: []string{"source"}})
	_ = dag.SetOutputs([]string{"transform"})

	results, _ := executor.Execute(context.Background(), dag, "entity-1")

	// Should have error in transform result
	if result, ok := results["transform"]; ok {
		if result.Error == "" {
			t.Error("Expected error for missing compute function")
		}
	}
}

func TestExecutor_Execute_ParallelNodes(t *testing.T) {
	executor := NewExecutor(DefaultExecutorConfig())

	executor.SetSourceFunc(func(ctx context.Context, node *Node, entityID string) (interface{}, error) {
		// Small delay to test parallelism
		time.Sleep(10 * time.Millisecond)
		return 5.0, nil
	})

	executor.RegisterComputeFunc(NodeTypeAggregate, func(ctx context.Context, node *Node, inputs map[string]interface{}) (interface{}, error) {
		sum := 0.0
		for _, v := range inputs {
			if num, ok := v.(float64); ok {
				sum += num
			}
		}
		return sum, nil
	})

	// DAG with 3 parallel source nodes
	dag := NewDAG("test", "Test DAG")
	_ = dag.AddNode(&Node{ID: "s1", Type: NodeTypeSource})
	_ = dag.AddNode(&Node{ID: "s2", Type: NodeTypeSource})
	_ = dag.AddNode(&Node{ID: "s3", Type: NodeTypeSource})
	_ = dag.AddNode(&Node{ID: "agg", Type: NodeTypeAggregate, Inputs: []string{"s1", "s2", "s3"}})
	_ = dag.SetOutputs([]string{"agg"})

	start := time.Now()
	results, err := executor.Execute(context.Background(), dag, "entity-1")
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Should complete faster than sequential (3 * 10ms = 30ms)
	// With parallelism, should be closer to 10-20ms
	if elapsed > 50*time.Millisecond {
		t.Errorf("Expected parallel execution, took %v", elapsed)
	}

	result, ok := results["agg"]
	if !ok {
		t.Fatal("Expected aggregate result")
	}

	if result.Value != 15.0 {
		t.Errorf("Expected value 15.0, got %v", result.Value)
	}
}

func TestExecutor_Execute_WithCaching(t *testing.T) {
	executor := NewExecutor(ExecutorConfig{
		MaxParallel:    10,
		DefaultTimeout: 30 * time.Second,
		EnableCaching:  true,
		CacheTTL:       5 * time.Minute,
	})

	callCount := 0
	executor.SetSourceFunc(func(ctx context.Context, node *Node, entityID string) (interface{}, error) {
		callCount++
		return 42.0, nil
	})

	dag := NewDAG("test", "Test DAG")
	_ = dag.AddNode(&Node{ID: "source", Type: NodeTypeSource, CacheEnabled: true})
	_ = dag.SetOutputs([]string{"source"})

	// First execution
	results1, _ := executor.Execute(context.Background(), dag, "entity-1")
	if results1["source"].CacheHit {
		t.Error("First execution should not be a cache hit")
	}

	// Second execution - should hit cache
	results2, _ := executor.Execute(context.Background(), dag, "entity-1")
	if !results2["source"].CacheHit {
		t.Error("Second execution should be a cache hit")
	}

	// Source function should only be called once
	if callCount != 1 {
		t.Errorf("Expected 1 source call, got %d", callCount)
	}
}

func TestExecutor_ClearCache(t *testing.T) {
	executor := NewExecutor(ExecutorConfig{
		MaxParallel:    10,
		DefaultTimeout: 30 * time.Second,
		EnableCaching:  true,
		CacheTTL:       5 * time.Minute,
	})

	callCount := 0
	executor.SetSourceFunc(func(ctx context.Context, node *Node, entityID string) (interface{}, error) {
		callCount++
		return 42.0, nil
	})

	dag := NewDAG("test", "Test DAG")
	_ = dag.AddNode(&Node{ID: "source", Type: NodeTypeSource, CacheEnabled: true})
	_ = dag.SetOutputs([]string{"source"})

	// First execution
	executor.Execute(context.Background(), dag, "entity-1")

	// Clear cache
	executor.ClearCache()

	// Third execution - should not hit cache
	results, _ := executor.Execute(context.Background(), dag, "entity-1")
	if results["source"].CacheHit {
		t.Error("Should not hit cache after clear")
	}

	if callCount != 2 {
		t.Errorf("Expected 2 source calls, got %d", callCount)
	}
}

func TestExecutor_Execute_WithTimeout(t *testing.T) {
	executor := NewExecutor(ExecutorConfig{
		MaxParallel:    10,
		DefaultTimeout: 50 * time.Millisecond,
		EnableCaching:  false,
		CacheTTL:       5 * time.Minute, // Required even if caching disabled
	})

	executor.SetSourceFunc(func(ctx context.Context, node *Node, entityID string) (interface{}, error) {
		// Check for timeout
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(200 * time.Millisecond):
			return 42.0, nil
		}
	})

	dag := NewDAG("test", "Test DAG")
	_ = dag.AddNode(&Node{ID: "source", Type: NodeTypeSource, Timeout: 50 * time.Millisecond})
	_ = dag.SetOutputs([]string{"source"})

	results, _ := executor.Execute(context.Background(), dag, "entity-1")

	// Should have timeout error
	result, ok := results["source"]
	if !ok {
		t.Fatal("Expected source result")
	}
	if result.Error == "" {
		t.Error("Expected timeout error")
	}
}

func TestExecutor_Execute_WithRetry(t *testing.T) {
	executor := NewExecutor(ExecutorConfig{
		MaxParallel:    10,
		DefaultTimeout: 30 * time.Second,
		RetryAttempts:  3,
		RetryDelay:     10 * time.Millisecond,
		EnableCaching:  false,
		CacheTTL:       5 * time.Minute, // Required even if caching disabled
	})

	executor.SetSourceFunc(func(ctx context.Context, node *Node, entityID string) (interface{}, error) {
		return 42.0, nil
	})

	callCount := 0
	executor.RegisterComputeFunc(NodeTypeTransform, func(ctx context.Context, node *Node, inputs map[string]interface{}) (interface{}, error) {
		callCount++
		if callCount < 3 {
			return nil, fmt.Errorf("temporary error")
		}
		return 100.0, nil
	})

	dag := NewDAG("test", "Test DAG")
	_ = dag.AddNode(&Node{ID: "source", Type: NodeTypeSource})
	_ = dag.AddNode(&Node{ID: "transform", Type: NodeTypeTransform, Inputs: []string{"source"}})
	_ = dag.SetOutputs([]string{"transform"})

	results, err := executor.Execute(context.Background(), dag, "entity-1")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	result, ok := results["transform"]
	if !ok {
		t.Fatal("Expected transform result")
	}

	if result.Error != "" {
		t.Errorf("Expected success after retry, got error: %s", result.Error)
	}

	if result.Value != 100.0 {
		t.Errorf("Expected value 100.0, got %v", result.Value)
	}

	if callCount != 3 {
		t.Errorf("Expected 3 calls (with retries), got %d", callCount)
	}
}

func TestExecutor_Stats(t *testing.T) {
	executor := NewExecutor(DefaultExecutorConfig())

	executor.SetSourceFunc(func(ctx context.Context, node *Node, entityID string) (interface{}, error) {
		return nil, nil
	})
	executor.RegisterComputeFunc(NodeTypeTransform, func(ctx context.Context, node *Node, inputs map[string]interface{}) (interface{}, error) {
		return nil, nil
	})
	executor.RegisterComputeFunc(NodeTypeAggregate, func(ctx context.Context, node *Node, inputs map[string]interface{}) (interface{}, error) {
		return nil, nil
	})

	stats := executor.Stats()

	if stats.ComputeFuncs != 2 {
		t.Errorf("Expected 2 compute funcs, got %d", stats.ComputeFuncs)
	}
	if !stats.HasSourceFunc {
		t.Error("Expected HasSourceFunc to be true")
	}
	if stats.MaxParallel != 10 {
		t.Errorf("Expected MaxParallel 10, got %d", stats.MaxParallel)
	}
	if !stats.CacheEnabled {
		t.Error("Expected CacheEnabled to be true")
	}
}

func TestResultCache(t *testing.T) {
	cache := newResultCache(100 * time.Millisecond)

	// Set a value
	cache.set("key1", "value1", 100*time.Millisecond)

	// Get immediately
	val, ok := cache.get("key1")
	if !ok {
		t.Error("Expected to get cached value")
	}
	if val != "value1" {
		t.Errorf("Expected 'value1', got '%v'", val)
	}

	// Wait for expiration
	time.Sleep(150 * time.Millisecond)

	// Should be expired
	_, ok = cache.get("key1")
	if ok {
		t.Error("Expected cached value to be expired")
	}
}

func TestResultCache_Clear(t *testing.T) {
	cache := newResultCache(5 * time.Minute)

	cache.set("key1", "value1", 5*time.Minute)
	cache.set("key2", "value2", 5*time.Minute)

	cache.clear()

	_, ok1 := cache.get("key1")
	_, ok2 := cache.get("key2")

	if ok1 || ok2 {
		t.Error("Expected cache to be cleared")
	}
}

func TestResultCache_ClearPrefix(t *testing.T) {
	cache := newResultCache(5 * time.Minute)

	cache.set("dag1:node1:entity1", "v1", 5*time.Minute)
	cache.set("dag1:node2:entity1", "v2", 5*time.Minute)
	cache.set("dag2:node1:entity1", "v3", 5*time.Minute)

	cache.clearPrefix("dag1:")

	_, ok1 := cache.get("dag1:node1:entity1")
	_, ok2 := cache.get("dag1:node2:entity1")
	_, ok3 := cache.get("dag2:node1:entity1")

	if ok1 || ok2 {
		t.Error("Expected dag1 entries to be cleared")
	}
	if !ok3 {
		t.Error("Expected dag2 entry to remain")
	}
}

func TestExecutionResult(t *testing.T) {
	result := &ExecutionResult{
		NodeID: "node1",
		Value:  42.0,
	}

	if result.NodeID != "node1" {
		t.Errorf("Expected NodeID 'node1', got '%s'", result.NodeID)
	}
	if result.Value != 42.0 {
		t.Errorf("Expected Value 42.0, got %v", result.Value)
	}
}
