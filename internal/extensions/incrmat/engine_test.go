package incrmat

import (
	"testing"
	"time"
)

func TestNewEngine(t *testing.T) {
	cfg := DefaultEngineConfig()
	e := NewEngine(cfg)
	if e == nil {
		t.Fatal("expected non-nil engine")
	}
	if e.config.MaxNodes != 10000 {
		t.Errorf("expected MaxNodes=10000, got %d", e.config.MaxNodes)
	}
}

func TestRegisterNode(t *testing.T) {
	e := NewEngine(DefaultEngineConfig())

	err := e.RegisterNode(MaterializationNode{
		ID:           "nodeA",
		FeatureGroup: "group1",
		Expression:   "SELECT * FROM raw",
	})
	if err != nil {
		t.Fatalf("RegisterNode failed: %v", err)
	}

	nodes := e.ListNodes()
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}
	if nodes[0].ID != "nodeA" {
		t.Errorf("expected nodeA, got %s", nodes[0].ID)
	}
}

func TestRecordChange(t *testing.T) {
	e := NewEngine(DefaultEngineConfig())

	// Register A -> B -> C chain
	_ = e.RegisterNode(MaterializationNode{ID: "A", FeatureGroup: "raw"})
	_ = e.RegisterNode(MaterializationNode{ID: "B", FeatureGroup: "derived", Dependencies: []string{"A"}})
	_ = e.RegisterNode(MaterializationNode{ID: "C", FeatureGroup: "final", Dependencies: []string{"B"}})

	// Change in "raw" group should dirty A, then propagate to B and C
	e.RecordChange(ChangeEvent{
		EntityID:     "entity1",
		FeatureGroup: "raw",
		Timestamp:    time.Now(),
	})

	dirty := e.GetDirtyNodes()
	dirtyIDs := make(map[string]bool)
	for _, d := range dirty {
		dirtyIDs[d.ID] = true
	}

	if !dirtyIDs["A"] {
		t.Error("expected A to be dirty")
	}
	if !dirtyIDs["B"] {
		t.Error("expected B to be dirty (transitive)")
	}
	if !dirtyIDs["C"] {
		t.Error("expected C to be dirty (transitive)")
	}
}

func TestMaterialize(t *testing.T) {
	e := NewEngine(DefaultEngineConfig())

	_ = e.RegisterNode(MaterializationNode{ID: "A", FeatureGroup: "raw"})
	_ = e.RegisterNode(MaterializationNode{ID: "B", FeatureGroup: "derived", Dependencies: []string{"A"}})

	// Mark dirty
	e.RecordChange(ChangeEvent{FeatureGroup: "raw", Timestamp: time.Now()})

	results := e.Materialize()
	if len(results) < 2 {
		t.Fatalf("expected at least 2 results, got %d", len(results))
	}

	// After materialization, no nodes should be dirty
	dirty := e.GetDirtyNodes()
	if len(dirty) != 0 {
		t.Errorf("expected 0 dirty nodes after materialize, got %d", len(dirty))
	}

	// Verify topological order: A should come before B
	aIdx, bIdx := -1, -1
	for i, r := range results {
		if r.NodeID == "A" {
			aIdx = i
		}
		if r.NodeID == "B" {
			bIdx = i
		}
	}
	if aIdx >= 0 && bIdx >= 0 && aIdx > bIdx {
		t.Error("expected A to be materialized before B")
	}
}

func TestCycleDetection(t *testing.T) {
	e := NewEngine(DefaultEngineConfig())

	_ = e.RegisterNode(MaterializationNode{ID: "A", FeatureGroup: "g1"})
	_ = e.RegisterNode(MaterializationNode{ID: "B", FeatureGroup: "g2", Dependencies: []string{"A"}})

	// C depends on B, B depends on A; adding A->C->A would create cycle
	// But actually: adding a node that depends on B, then trying to make A depend on it
	err := e.RegisterNode(MaterializationNode{ID: "C", FeatureGroup: "g3", Dependencies: []string{"B"}})
	if err != nil {
		t.Fatalf("C->B should be fine: %v", err)
	}

	// Self-cycle
	err = e.RegisterNode(MaterializationNode{ID: "D", FeatureGroup: "g4", Dependencies: []string{"D"}})
	if err != ErrCyclicDependency {
		t.Errorf("expected ErrCyclicDependency for self-cycle, got %v", err)
	}
}

func TestSkipCleanNodes(t *testing.T) {
	e := NewEngine(DefaultEngineConfig())

	_ = e.RegisterNode(MaterializationNode{ID: "A", FeatureGroup: "raw"})
	_ = e.RegisterNode(MaterializationNode{ID: "B", FeatureGroup: "other"})

	// Only dirty "raw" group, so only A should be dirty
	e.RecordChange(ChangeEvent{FeatureGroup: "raw", Timestamp: time.Now()})

	results := e.Materialize()

	// Only A should have been processed
	processed := make(map[string]bool)
	for _, r := range results {
		processed[r.NodeID] = true
	}
	if !processed["A"] {
		t.Error("expected A to be processed")
	}
	if processed["B"] {
		t.Error("expected B to be skipped (not dirty)")
	}

	stats := e.Stats()
	if stats.TotalSkipped == 0 {
		t.Error("expected at least one skipped node")
	}
}

func TestStats(t *testing.T) {
	e := NewEngine(DefaultEngineConfig())

	_ = e.RegisterNode(MaterializationNode{ID: "A", FeatureGroup: "raw"})
	_ = e.RegisterNode(MaterializationNode{ID: "B", FeatureGroup: "derived", Dependencies: []string{"A"}})

	stats := e.Stats()
	if stats.TotalNodes != 2 {
		t.Errorf("expected 2 nodes, got %d", stats.TotalNodes)
	}
	if stats.DirtyNodes != 0 {
		t.Errorf("expected 0 dirty nodes, got %d", stats.DirtyNodes)
	}

	e.RecordChange(ChangeEvent{FeatureGroup: "raw", Timestamp: time.Now()})

	stats = e.Stats()
	if stats.DirtyNodes != 2 {
		t.Errorf("expected 2 dirty nodes, got %d", stats.DirtyNodes)
	}
	if stats.TotalChanges != 1 {
		t.Errorf("expected 1 total change, got %d", stats.TotalChanges)
	}
}
