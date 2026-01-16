package materialization

import (
	"context"
	"testing"
)

func TestIncrementalEngine_RegisterNode(t *testing.T) {
	base := NewEngine(DefaultEngineConfig())
	ie := NewIncrementalEngine(base, DefaultIncrementalConfig())

	if err := ie.RegisterNode("source", nil); err != nil {
		t.Fatalf("registering root node: %v", err)
	}
	if err := ie.RegisterNode("transform", []string{"source"}); err != nil {
		t.Fatalf("registering child: %v", err)
	}
	if err := ie.RegisterNode("output", []string{"transform"}); err != nil {
		t.Fatalf("registering output: %v", err)
	}

	dag := ie.GetDAG()
	if len(dag) != 3 {
		t.Errorf("expected 3 nodes, got %d", len(dag))
	}
}

func TestIncrementalEngine_RegisterNodeCycle(t *testing.T) {
	base := NewEngine(DefaultEngineConfig())
	ie := NewIncrementalEngine(base, DefaultIncrementalConfig())

	ie.RegisterNode("a", nil)
	ie.RegisterNode("b", []string{"a"})
	err := ie.RegisterNode("a_cycle", []string{"b"})
	// a_cycle depends on b, which depends on a — no cycle unless a depends on a_cycle
	if err != nil {
		t.Fatalf("this should not be a cycle: %v", err)
	}
}

func TestIncrementalEngine_RegisterNodeMissingDep(t *testing.T) {
	base := NewEngine(DefaultEngineConfig())
	ie := NewIncrementalEngine(base, DefaultIncrementalConfig())

	err := ie.RegisterNode("child", []string{"nonexistent"})
	if err == nil {
		t.Error("expected error for missing dependency")
	}
}

func TestIncrementalEngine_NotifyChange(t *testing.T) {
	base := NewEngine(DefaultEngineConfig())
	ie := NewIncrementalEngine(base, DefaultIncrementalConfig())

	ie.RegisterNode("source", nil)
	ie.RegisterNode("transform", []string{"source"})
	ie.RegisterNode("output", []string{"transform"})

	if err := ie.NotifyChange("source"); err != nil {
		t.Fatalf("notify change: %v", err)
	}

	dirty := ie.GetDirtyNodes()
	if len(dirty) != 3 {
		t.Errorf("expected 3 dirty nodes (cascade), got %d", len(dirty))
	}
}

func TestIncrementalEngine_NotifyChangeNotFound(t *testing.T) {
	base := NewEngine(DefaultEngineConfig())
	ie := NewIncrementalEngine(base, DefaultIncrementalConfig())

	if err := ie.NotifyChange("nonexistent"); err == nil {
		t.Error("expected error for nonexistent node")
	}
}

func TestIncrementalEngine_ComputeIncremental(t *testing.T) {
	base := NewEngine(DefaultEngineConfig())
	ie := NewIncrementalEngine(base, DefaultIncrementalConfig())

	ie.RegisterNode("source", nil)
	ie.RegisterNode("transform", []string{"source"})
	ie.RegisterNode("output", []string{"transform"})
	ie.NotifyChange("source")

	result, err := ie.ComputeIncremental(context.Background())
	if err != nil {
		t.Fatalf("compute incremental: %v", err)
	}
	if result.NodesProcessed != 3 {
		t.Errorf("expected 3 processed, got %d", result.NodesProcessed)
	}

	// Second run should process 0 (nothing dirty).
	result2, err := ie.ComputeIncremental(context.Background())
	if err != nil {
		t.Fatalf("second compute: %v", err)
	}
	if result2.NodesProcessed != 0 {
		t.Errorf("expected 0 processed on clean DAG, got %d", result2.NodesProcessed)
	}
}

func TestIncrementalEngine_PartialDirty(t *testing.T) {
	base := NewEngine(DefaultEngineConfig())
	ie := NewIncrementalEngine(base, DefaultIncrementalConfig())

	ie.RegisterNode("a", nil)
	ie.RegisterNode("b", nil)
	ie.RegisterNode("c", []string{"a"})

	// Only mark 'a' as dirty — should cascade to 'c' but not 'b'.
	ie.NotifyChange("a")
	dirty := ie.GetDirtyNodes()
	if len(dirty) != 2 {
		t.Errorf("expected 2 dirty (a, c), got %d", len(dirty))
	}

	result, _ := ie.ComputeIncremental(context.Background())
	if result.NodesProcessed != 2 {
		t.Errorf("expected 2 processed, got %d", result.NodesProcessed)
	}
	if result.NodesSkipped != 1 {
		t.Errorf("expected 1 skipped (b), got %d", result.NodesSkipped)
	}
}

func TestIncrementalEngine_Checkpoint(t *testing.T) {
	base := NewEngine(DefaultEngineConfig())
	ie := NewIncrementalEngine(base, DefaultIncrementalConfig())

	ie.SaveCheckpoint("pipeline1", "step1", 42, map[string]interface{}{"key": "value"})

	cp := ie.GetCheckpoint("pipeline1", "step1")
	if cp == nil {
		t.Fatal("expected checkpoint")
	}
	if cp.Offset != 42 {
		t.Errorf("expected offset 42, got %d", cp.Offset)
	}
	if cp.State["key"] != "value" {
		t.Error("expected state to be preserved")
	}

	// Non-existent checkpoint.
	if ie.GetCheckpoint("pipeline1", "step99") != nil {
		t.Error("expected nil for missing checkpoint")
	}
}

func TestDefaultIncrementalConfig(t *testing.T) {
	cfg := DefaultIncrementalConfig()
	if cfg.CheckpointInterval <= 0 {
		t.Error("checkpoint interval should be positive")
	}
	if cfg.MaxBatchSize <= 0 {
		t.Error("max batch size should be positive")
	}
}
