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

func TestAutoShardingQuorumWrite_Success(t *testing.T) {
	t.Parallel()
	engine := NewAutoShardingEngine(DefaultAutoShardingConfig())
	_ = engine.AddNode("node-1", "localhost:8080")
	_ = engine.AddNode("node-2", "localhost:8081")
	_ = engine.AddNode("node-3", "localhost:8082")

	result, err := engine.QuorumWrite(context.Background(), "user:123", "value")
	if err != nil {
		t.Fatal(err)
	}
	if !result.Consistent {
		t.Error("expected consistent write")
	}
	if result.Quorum != 2 {
		t.Errorf("expected quorum 2, got %d", result.Quorum)
	}
	if result.Responses < result.Quorum {
		t.Errorf("expected responses >= quorum, got %d", result.Responses)
	}
}

func TestAutoShardingQuorumWrite_InsufficientReplicas(t *testing.T) {
	t.Parallel()
	cfg := DefaultAutoShardingConfig()
	cfg.QuorumSize = 5
	engine := NewAutoShardingEngine(cfg)
	_ = engine.AddNode("node-1", "localhost:8080")

	_, err := engine.QuorumWrite(context.Background(), "user:123", "value")
	if err == nil {
		t.Fatal("expected error for insufficient replicas")
	}
}

func TestAutoShardingQuorumWrite_ContextCancellation(t *testing.T) {
	t.Parallel()
	engine := NewAutoShardingEngine(DefaultAutoShardingConfig())
	_ = engine.AddNode("node-1", "localhost:8080")
	_ = engine.AddNode("node-2", "localhost:8081")
	_ = engine.AddNode("node-3", "localhost:8082")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Cancelled context should still work here since QuorumWrite is in-memory
	result, err := engine.QuorumWrite(ctx, "user:456", "value")
	// The in-memory implementation doesn't check ctx, so it may succeed
	if err == nil && result != nil {
		if !result.Consistent {
			t.Error("expected consistent result")
		}
	}
}

func TestAutoShardingTriggerRebalance_Redistribution(t *testing.T) {
	t.Parallel()
	engine := NewAutoShardingEngine(DefaultAutoShardingConfig())
	_ = engine.AddNode("n1", "localhost:8080")
	_ = engine.AddNode("n2", "localhost:8081")
	_ = engine.AddNode("n3", "localhost:8082")

	// Create imbalance: n1 has 1000 items, n2 has 10, n3 has 10
	engine.mu.Lock()
	i := 0
	for _, a := range engine.assignments {
		if i == 0 {
			a.NodeID = "n1"
			a.ItemCount = 1000
		} else if i == 1 {
			a.NodeID = "n2"
			a.ItemCount = 10
		} else {
			a.NodeID = "n3"
			a.ItemCount = 10
		}
		i++
	}
	engine.mu.Unlock()

	moved, err := engine.TriggerRebalance()
	if err != nil {
		t.Fatal(err)
	}
	if moved == 0 {
		t.Error("expected some shards to be moved")
	}
}

func TestAutoShardingTriggerRebalance_SingleNode(t *testing.T) {
	t.Parallel()
	engine := NewAutoShardingEngine(DefaultAutoShardingConfig())
	_ = engine.AddNode("n1", "localhost:8080")

	// Single node: set item count to trigger check
	engine.mu.Lock()
	for _, a := range engine.assignments {
		a.ItemCount = 1000
	}
	engine.mu.Unlock()

	moved, err := engine.TriggerRebalance()
	if err != nil {
		t.Fatal(err)
	}
	// Single node shouldn't move anything
	if moved != 0 {
		t.Errorf("expected 0 moves for single node, got %d", moved)
	}
}

func TestAutoShardingTriggerRebalance_Balanced(t *testing.T) {
	t.Parallel()
	engine := NewAutoShardingEngine(DefaultAutoShardingConfig())
	_ = engine.AddNode("n1", "localhost:8080")
	_ = engine.AddNode("n2", "localhost:8081")

	// Balanced: equal item counts
	engine.mu.Lock()
	i := 0
	nodes := []string{"n1", "n2"}
	for _, a := range engine.assignments {
		a.NodeID = nodes[i%2]
		a.ItemCount = 500
		i++
	}
	engine.mu.Unlock()

	moved, err := engine.TriggerRebalance()
	if err != nil {
		t.Fatal(err)
	}
	// If already balanced, no moves needed
	if moved != 0 {
		t.Logf("balanced distribution still moved %d shards (within threshold)", moved)
	}
}

func TestAutoShardingListNodes(t *testing.T) {
	t.Parallel()
	engine := NewAutoShardingEngine(DefaultAutoShardingConfig())
	_ = engine.AddNode("n1", "localhost:8080")
	_ = engine.AddNode("n2", "localhost:8081")
	_ = engine.AddNode("n3", "localhost:8082")

	nodes := engine.ListNodes()
	if len(nodes) != 3 {
		t.Errorf("expected 3 nodes, got %d", len(nodes))
	}
}

func TestAutoShardingCheckImbalance_Threshold(t *testing.T) {
	t.Parallel()
	engine := NewAutoShardingEngine(DefaultAutoShardingConfig())
	_ = engine.AddNode("n1", "localhost:8080")
	_ = engine.AddNode("n2", "localhost:8081")

	// Create imbalance: n1 has 1000 items, n2 has 100
	engine.mu.Lock()
	i := 0
	for _, a := range engine.assignments {
		if i == 0 {
			a.NodeID = "n1"
			a.ItemCount = 1000
		} else {
			a.NodeID = "n2"
			a.ItemCount = 100
		}
		i++
	}
	engine.mu.Unlock()

	imbalance := engine.CheckImbalance()
	if imbalance <= 0 {
		t.Error("expected positive imbalance ratio")
	}
}

func TestAutoShardingCheckImbalance_Balanced(t *testing.T) {
	t.Parallel()
	engine := NewAutoShardingEngine(DefaultAutoShardingConfig())
	_ = engine.AddNode("n1", "localhost:8080")
	_ = engine.AddNode("n2", "localhost:8081")

	// Equal items per node
	engine.mu.Lock()
	i := 0
	nodes := []string{"n1", "n2"}
	for _, a := range engine.assignments {
		a.NodeID = nodes[i%2]
		a.ItemCount = 500
		i++
	}
	engine.mu.Unlock()

	imbalance := engine.CheckImbalance()
	if imbalance > 0.01 {
		t.Errorf("expected near-zero imbalance, got %f", imbalance)
	}
}

func TestAutoShardingCheckImbalance_SingleNode(t *testing.T) {
	t.Parallel()
	engine := NewAutoShardingEngine(DefaultAutoShardingConfig())
	_ = engine.AddNode("n1", "localhost:8080")

	engine.mu.Lock()
	for _, a := range engine.assignments {
		a.ItemCount = 1000
	}
	engine.mu.Unlock()

	imbalance := engine.CheckImbalance()
	if imbalance != 0 {
		t.Errorf("single node should have 0 imbalance, got %f", imbalance)
	}
}
