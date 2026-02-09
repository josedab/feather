package computegraph

import (
	"sync/atomic"
	"testing"
)

func TestIncrementalEngine_PropagateChange(t *testing.T) {
	e := newTestEngine()
	mustAddNodeE(e, FeatureNode{Name: "a", Kind: KindSource, Function: FuncIdentity, OutputType: "float64"})
	mustAddNodeE(e, FeatureNode{Name: "b", Kind: KindSource, Function: FuncIdentity, OutputType: "float64"})
	mustAddNodeE(e, FeatureNode{Name: "c", Kind: KindDerived, Inputs: []string{"a", "b"}, Function: FuncSum, OutputType: "float64"})

	ie := NewIncrementalEngine(e, DefaultIncrementalConfig())

	// Initial compute
	inputs := map[string]interface{}{"a": 10.0, "b": 20.0}
	recomputed, err := ie.PropagateChange("a", 10.0, inputs)
	if err != nil {
		t.Fatalf("PropagateChange: %v", err)
	}

	if len(recomputed) < 1 {
		t.Fatalf("expected at least 1 recomputed node, got %d", len(recomputed))
	}

	// Propagate a change
	inputs2 := map[string]interface{}{"a": 50.0, "b": 20.0}
	recomputed2, err := ie.PropagateChange("a", 50.0, inputs2)
	if err != nil {
		t.Fatalf("PropagateChange: %v", err)
	}

	// Should recompute a and c
	if len(recomputed2) < 2 {
		t.Fatalf("expected at least 2 recomputed nodes, got %d: %v", len(recomputed2), recomputed2)
	}
}

func TestIncrementalEngine_ChangeListener(t *testing.T) {
	e := newTestEngine()
	mustAddNodeE(e, FeatureNode{Name: "x", Kind: KindSource, Function: FuncIdentity, OutputType: "float64"})

	ie := NewIncrementalEngine(e, DefaultIncrementalConfig())

	var callCount int32
	ie.OnChange("x", func(event ChangeEvent) {
		atomic.AddInt32(&callCount, 1)
	})

	inputs := map[string]interface{}{"x": 1.0}
	if _, err := ie.PropagateChange("x", 1.0, inputs); err != nil {
		t.Fatalf("PropagateChange: %v", err)
	}

	if atomic.LoadInt32(&callCount) != 1 {
		t.Fatalf("expected listener called once, got %d", callCount)
	}
}

func TestIncrementalEngine_NonSourceError(t *testing.T) {
	e := newTestEngine()
	mustAddNodeE(e, FeatureNode{Name: "a", Kind: KindSource, Function: FuncIdentity, OutputType: "float64"})
	mustAddNodeE(e, FeatureNode{Name: "b", Kind: KindDerived, Inputs: []string{"a"}, Function: FuncIdentity, OutputType: "float64"})

	ie := NewIncrementalEngine(e, DefaultIncrementalConfig())

	_, err := ie.PropagateChange("b", 1.0, map[string]interface{}{})
	if err == nil {
		t.Fatal("expected error for non-source node")
	}
}

func TestIncrementalEngine_GetChangeLog(t *testing.T) {
	e := newTestEngine()
	mustAddNodeE(e, FeatureNode{Name: "a", Kind: KindSource, Function: FuncIdentity, OutputType: "float64"})

	ie := NewIncrementalEngine(e, DefaultIncrementalConfig())

	inputs := map[string]interface{}{"a": 1.0}
	ie.PropagateChange("a", 1.0, inputs)
	ie.PropagateChange("a", 2.0, inputs)

	log := ie.GetChangeLog("", 10)
	if len(log) != 2 {
		t.Fatalf("expected 2 changes, got %d", len(log))
	}

	log = ie.GetChangeLog("a", 1)
	if len(log) != 1 {
		t.Fatalf("expected 1 change with limit, got %d", len(log))
	}
}

func TestIncrementalEngine_Stats(t *testing.T) {
	e := newTestEngine()
	mustAddNodeE(e, FeatureNode{Name: "a", Kind: KindSource, Function: FuncIdentity, OutputType: "float64"})

	ie := NewIncrementalEngine(e, DefaultIncrementalConfig())
	ie.OnChange("a", func(event ChangeEvent) {})

	stats := ie.Stats()
	if stats.ListenerCount != 1 {
		t.Fatalf("expected 1 listener, got %d", stats.ListenerCount)
	}
	if stats.GraphStats.TotalNodes != 1 {
		t.Fatalf("expected 1 node, got %d", stats.GraphStats.TotalNodes)
	}
}

func TestComputeParallel(t *testing.T) {
	e := newTestEngine()
	mustAddNodeE(e, FeatureNode{Name: "a", Kind: KindSource, Function: FuncIdentity, OutputType: "float64"})
	mustAddNodeE(e, FeatureNode{Name: "b", Kind: KindSource, Function: FuncIdentity, OutputType: "float64"})
	mustAddNodeE(e, FeatureNode{Name: "c", Kind: KindDerived, Inputs: []string{"a"}, Function: FuncIdentity, OutputType: "float64"})
	mustAddNodeE(e, FeatureNode{Name: "d", Kind: KindDerived, Inputs: []string{"b"}, Function: FuncIdentity, OutputType: "float64"})

	inputs := map[string]interface{}{"a": 10.0, "b": 20.0}
	results, err := e.ComputeParallel([]string{"c", "d"}, inputs)
	if err != nil {
		t.Fatalf("ComputeParallel: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results["c"].Value != 10.0 {
		t.Fatalf("expected c=10.0, got %v", results["c"].Value)
	}
	if results["d"].Value != 20.0 {
		t.Fatalf("expected d=20.0, got %v", results["d"].Value)
	}
}
