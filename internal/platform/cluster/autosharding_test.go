package cluster

import (
	"context"
	"testing"
)

func TestAutoShardingAddRemoveNode(t *testing.T) {
	t.Parallel()
	engine := NewAutoShardingEngine(DefaultAutoShardingConfig())
	if err := engine.AddNode("node-1", "localhost:8080"); err != nil {
		t.Fatal(err)
	}
	if err := engine.AddNode("node-2", "localhost:8081"); err != nil {
		t.Fatal(err)
	}

	stats := engine.Stats()
	if stats.TotalNodes != 2 {
		t.Errorf("expected 2 nodes, got %d", stats.TotalNodes)
	}

	if err := engine.RemoveNode("node-1"); err != nil {
		t.Fatal(err)
	}
	stats = engine.Stats()
	if stats.TotalNodes != 1 {
		t.Errorf("expected 1 node, got %d", stats.TotalNodes)
	}
}

func TestAutoShardingGetOwner(t *testing.T) {
	t.Parallel()
	engine := NewAutoShardingEngine(DefaultAutoShardingConfig())
	_ = engine.AddNode("node-1", "localhost:8080")
	_ = engine.AddNode("node-2", "localhost:8081")

	owner, err := engine.GetOwner("user:123")
	if err != nil {
		t.Fatal(err)
	}
	if owner == "" {
		t.Error("expected non-empty owner")
	}
}

func TestAutoShardingGetReplicas(t *testing.T) {
	t.Parallel()
	engine := NewAutoShardingEngine(DefaultAutoShardingConfig())
	_ = engine.AddNode("node-1", "localhost:8080")
	_ = engine.AddNode("node-2", "localhost:8081")
	_ = engine.AddNode("node-3", "localhost:8082")

	replicas, err := engine.GetReplicas("user:123", 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(replicas) < 2 {
		t.Errorf("expected at least 2 replicas, got %d", len(replicas))
	}
}

func TestAutoShardingQuorumRead(t *testing.T) {
	t.Parallel()
	engine := NewAutoShardingEngine(DefaultAutoShardingConfig())
	_ = engine.AddNode("node-1", "localhost:8080")
	_ = engine.AddNode("node-2", "localhost:8081")
	_ = engine.AddNode("node-3", "localhost:8082")

	result, err := engine.QuorumRead(context.Background(), "user:123")
	if err != nil {
		t.Fatal(err)
	}
	if !result.Consistent {
		t.Error("expected consistent read")
	}
	if result.Quorum != 2 {
		t.Errorf("expected quorum 2, got %d", result.Quorum)
	}
}

func TestAutoShardingImbalance(t *testing.T) {
	t.Parallel()
	engine := NewAutoShardingEngine(DefaultAutoShardingConfig())
	imbalance := engine.CheckImbalance()
	if imbalance != 0 {
		t.Errorf("expected 0 imbalance with no assignments, got %f", imbalance)
	}
}

func TestAutoShardingAssignments(t *testing.T) {
t.Parallel()
engine := NewAutoShardingEngine(DefaultAutoShardingConfig())
_ = engine.AddNode("n1", "localhost:8080")
_ = engine.AddNode("n2", "localhost:8081")

assignments := engine.GetAssignments()
if len(assignments) != 2 {
t.Errorf("expected 2 assignments, got %d", len(assignments))
}
}

func TestAutoShardingRemoveNodeCleansAssignments(t *testing.T) {
t.Parallel()
engine := NewAutoShardingEngine(DefaultAutoShardingConfig())
_ = engine.AddNode("n1", "localhost:8080")
_ = engine.AddNode("n2", "localhost:8081")
_ = engine.RemoveNode("n1")

assignments := engine.GetAssignments()
if len(assignments) != 1 {
t.Errorf("expected 1 assignment after remove, got %d", len(assignments))
}
}
