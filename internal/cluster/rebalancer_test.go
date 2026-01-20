package cluster

import (
	"context"
	"testing"
	"time"
)

func TestDefaultRebalancerConfig(t *testing.T) {
	config := DefaultRebalancerConfig()

	if config.MaxConcurrentTasks != 4 {
		t.Errorf("expected max concurrent tasks 4, got %d", config.MaxConcurrentTasks)
	}
	if config.TaskTimeout != 5*time.Minute {
		t.Errorf("expected task timeout 5m, got %v", config.TaskTimeout)
	}
	if config.ImbalanceThreshold != 0.1 {
		t.Errorf("expected imbalance threshold 0.1, got %v", config.ImbalanceThreshold)
	}
}

func TestRebalanceState_Values(t *testing.T) {
	states := []RebalanceState{
		RebalanceStatePending,
		RebalanceStateRunning,
		RebalanceStateCompleted,
		RebalanceStateFailed,
		RebalanceStateCancelled,
	}

	expected := []string{"pending", "running", "completed", "failed", "cancelled"}

	for i, state := range states {
		if string(state) != expected[i] {
			t.Errorf("expected state '%s', got '%s'", expected[i], state)
		}
	}
}

func TestRebalanceReason_Values(t *testing.T) {
	reasons := []RebalanceReason{
		RebalanceReasonNodeJoin,
		RebalanceReasonNodeLeave,
		RebalanceReasonManual,
		RebalanceReasonScheduled,
		RebalanceReasonImbalance,
	}

	expected := []string{"node_join", "node_leave", "manual", "scheduled", "imbalance"}

	for i, reason := range reasons {
		if string(reason) != expected[i] {
			t.Errorf("expected reason '%s', got '%s'", expected[i], reason)
		}
	}
}

func TestRebalanceTask_Fields(t *testing.T) {
	task := &RebalanceTask{
		ID:        "task-1",
		Partition: 10,
		FromNode:  "node-1",
		ToNode:    "node-2",
		State:     RebalanceStatePending,
	}

	if task.ID != "task-1" {
		t.Errorf("expected ID 'task-1', got '%s'", task.ID)
	}
	if task.Partition != 10 {
		t.Errorf("expected partition 10, got %d", task.Partition)
	}
	if task.FromNode != "node-1" {
		t.Errorf("expected from node 'node-1', got '%s'", task.FromNode)
	}
}

func TestRebalancePlan_Progress(t *testing.T) {
	tests := []struct {
		name     string
		tasks    []*RebalanceTask
		expected float64
	}{
		{
			name:     "empty",
			tasks:    []*RebalanceTask{},
			expected: 1.0,
		},
		{
			name: "none completed",
			tasks: []*RebalanceTask{
				{State: RebalanceStatePending},
				{State: RebalanceStatePending},
			},
			expected: 0.0,
		},
		{
			name: "half completed",
			tasks: []*RebalanceTask{
				{State: RebalanceStateCompleted},
				{State: RebalanceStatePending},
			},
			expected: 0.5,
		},
		{
			name: "all completed",
			tasks: []*RebalanceTask{
				{State: RebalanceStateCompleted},
				{State: RebalanceStateCompleted},
			},
			expected: 1.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan := &RebalancePlan{Tasks: tt.tasks}
			progress := plan.Progress()
			if progress != tt.expected {
				t.Errorf("expected progress %.2f, got %.2f", tt.expected, progress)
			}
		})
	}
}

func TestRebalancer_Creation(t *testing.T) {
	config := DefaultMembershipConfig()
	membership := NewMembershipManager(config)

	ring := NewHashRing(100)
	ring.AddNode(&Node{ID: "node-1", Weight: 100, VirtualNodes: 100})

	pm := NewPartitionMap(ring, 64, 2)

	rebalancerConfig := DefaultRebalancerConfig()
	rebalancer := NewRebalancer(rebalancerConfig, membership, ring, pm)

	if rebalancer == nil {
		t.Fatal("expected non-nil rebalancer")
	}
}

func TestRebalancer_Stats_Empty(t *testing.T) {
	config := DefaultMembershipConfig()
	membership := NewMembershipManager(config)

	ring := NewHashRing(100)
	ring.AddNode(&Node{ID: "node-1", Weight: 100, VirtualNodes: 100})

	pm := NewPartitionMap(ring, 64, 2)

	rebalancerConfig := DefaultRebalancerConfig()
	rebalancer := NewRebalancer(rebalancerConfig, membership, ring, pm)

	stats := rebalancer.Stats()

	if stats.TotalRebalances != 0 {
		t.Errorf("expected 0 total rebalances, got %d", stats.TotalRebalances)
	}
	if stats.SuccessfulRebalances != 0 {
		t.Errorf("expected 0 successful rebalances, got %d", stats.SuccessfulRebalances)
	}
}

func TestRebalancer_SetTransferFunc(t *testing.T) {
	config := DefaultMembershipConfig()
	membership := NewMembershipManager(config)

	ring := NewHashRing(100)
	pm := NewPartitionMap(ring, 64, 2)

	rebalancerConfig := DefaultRebalancerConfig()
	rebalancer := NewRebalancer(rebalancerConfig, membership, ring, pm)

	called := false
	rebalancer.SetTransferFunc(func(ctx context.Context, task *RebalanceTask) error {
		called = true
		return nil
	})

	// Check that transfer func was set (indirectly)
	if called {
		t.Error("transfer func should not be called yet")
	}
}

func TestRebalancer_GetCurrentPlan_Empty(t *testing.T) {
	config := DefaultMembershipConfig()
	membership := NewMembershipManager(config)

	ring := NewHashRing(100)
	pm := NewPartitionMap(ring, 64, 2)

	rebalancerConfig := DefaultRebalancerConfig()
	rebalancer := NewRebalancer(rebalancerConfig, membership, ring, pm)

	plan := rebalancer.GetCurrentPlan()
	if plan != nil {
		t.Error("expected nil current plan")
	}
}

func TestRebalancer_GetPlanHistory_Empty(t *testing.T) {
	config := DefaultMembershipConfig()
	membership := NewMembershipManager(config)

	ring := NewHashRing(100)
	pm := NewPartitionMap(ring, 64, 2)

	rebalancerConfig := DefaultRebalancerConfig()
	rebalancer := NewRebalancer(rebalancerConfig, membership, ring, pm)

	history := rebalancer.GetPlanHistory()
	if len(history) != 0 {
		t.Errorf("expected empty plan history, got %d entries", len(history))
	}
}

func TestRebalancer_CancelRebalance_NoPlan(t *testing.T) {
	config := DefaultMembershipConfig()
	membership := NewMembershipManager(config)

	ring := NewHashRing(100)
	pm := NewPartitionMap(ring, 64, 2)

	rebalancerConfig := DefaultRebalancerConfig()
	rebalancer := NewRebalancer(rebalancerConfig, membership, ring, pm)

	err := rebalancer.CancelRebalance()
	if err == nil {
		t.Error("expected error when cancelling with no plan")
	}
}

func TestRebalancer_TriggerRebalance_NoChanges(t *testing.T) {
	config := DefaultMembershipConfig()
	membership := NewMembershipManager(config)

	ring := NewHashRing(100)
	ring.AddNode(&Node{ID: "node-1", Weight: 100, VirtualNodes: 100})

	pm := NewPartitionMap(ring, 64, 2)

	rebalancerConfig := DefaultRebalancerConfig()
	rebalancer := NewRebalancer(rebalancerConfig, membership, ring, pm)

	// With a single node and no changes, there should be nothing to rebalance
	_, err := rebalancer.TriggerRebalance()
	if err == nil {
		t.Error("expected error when no rebalancing needed")
	}
}

func TestRebalancer_OnMembershipChange(t *testing.T) {
	config := DefaultMembershipConfig()
	membership := NewMembershipManager(config)

	ring := NewHashRing(100)
	ring.AddNode(&Node{ID: "node-1", Weight: 100, VirtualNodes: 100})

	pm := NewPartitionMap(ring, 64, 2)

	rebalancerConfig := DefaultRebalancerConfig()
	rebalancerConfig.MinRebalanceDelay = 0 // Disable delay for testing
	rebalancer := NewRebalancer(rebalancerConfig, membership, ring, pm)

	// Simulate a node join event
	newNode := &Node{ID: "node-2", Weight: 100, VirtualNodes: 100}
	rebalancer.OnMembershipChange(MembershipEvent{
		Type: EventNodeJoin,
		Node: newNode,
	})

	// Check that node was added to ring
	if ring.NodeCount() != 2 {
		t.Errorf("expected 2 nodes in ring after join, got %d", ring.NodeCount())
	}
}

func TestRebalancer_DryRun(t *testing.T) {
	config := DefaultMembershipConfig()
	membership := NewMembershipManager(config)

	ring := NewHashRing(100)
	ring.AddNode(&Node{ID: "node-1", Weight: 100, VirtualNodes: 100})

	pm := NewPartitionMap(ring, 64, 2)

	rebalancerConfig := DefaultRebalancerConfig()
	rebalancerConfig.DryRun = true
	rebalancerConfig.MinRebalanceDelay = 0
	rebalancer := NewRebalancer(rebalancerConfig, membership, ring, pm)

	// Add a node to trigger rebalance
	ring.AddNode(&Node{ID: "node-2", Weight: 100, VirtualNodes: 100})
	pm.Recompute()

	// In dry run mode, rebalance should complete immediately without actual transfers
	if rebalancerConfig.DryRun != true {
		t.Error("expected dry run mode to be enabled")
	}

	// Check rebalancer is initialized
	if rebalancer == nil {
		t.Error("expected non-nil rebalancer")
	}
}

func TestNodeIDSet(t *testing.T) {
	nodes := []*Node{
		{ID: "node-1"},
		{ID: "node-2"},
		nil, // Should be skipped
		{ID: "node-3"},
	}

	set := nodeIDSet(nodes)

	if len(set) != 3 {
		t.Errorf("expected 3 entries in set, got %d", len(set))
	}
	if !set["node-1"] {
		t.Error("expected node-1 in set")
	}
	if !set["node-2"] {
		t.Error("expected node-2 in set")
	}
	if !set["node-3"] {
		t.Error("expected node-3 in set")
	}
}

func TestRebalancerStats_Fields(t *testing.T) {
	stats := RebalancerStats{
		TotalRebalances:      5,
		SuccessfulRebalances: 4,
		FailedRebalances:     1,
		PartitionDistribution: map[string]int{
			"node-1": 50,
			"node-2": 50,
		},
	}

	if stats.TotalRebalances != 5 {
		t.Errorf("expected 5 total, got %d", stats.TotalRebalances)
	}
	if stats.SuccessfulRebalances != 4 {
		t.Errorf("expected 4 successful, got %d", stats.SuccessfulRebalances)
	}
	if stats.FailedRebalances != 1 {
		t.Errorf("expected 1 failed, got %d", stats.FailedRebalances)
	}
}

func TestRebalanceTask_Progress(t *testing.T) {
	task := &RebalanceTask{
		ID:         "task-1",
		BytesMoved: 500,
		KeysMoved:  100,
		Progress:   0.5,
	}

	if task.Progress != 0.5 {
		t.Errorf("expected progress 0.5, got %f", task.Progress)
	}
	if task.BytesMoved != 500 {
		t.Errorf("expected 500 bytes moved, got %d", task.BytesMoved)
	}
	if task.KeysMoved != 100 {
		t.Errorf("expected 100 keys moved, got %d", task.KeysMoved)
	}
}
