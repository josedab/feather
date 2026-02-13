package backfill

import (
	"testing"
)

func newTestOrchestrator() *Orchestrator {
	return NewOrchestrator(DefaultOrchestratorConfig())
}

// --- CreateDAG / validateNoCycles ---

func TestCreateDAG_SingleNode(t *testing.T) {
	o := newTestOrchestrator()
	dag, err := o.CreateDAG("single", []*DAGNode{
		{ID: "a", FeatureID: "f1"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(dag.Nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(dag.Nodes))
	}
	if dag.Status != "pending" {
		t.Fatalf("expected status pending, got %s", dag.Status)
	}
}

func TestCreateDAG_EmptyDAG(t *testing.T) {
	o := newTestOrchestrator()
	dag, err := o.CreateDAG("empty", []*DAGNode{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(dag.Nodes) != 0 {
		t.Fatalf("expected 0 nodes, got %d", len(dag.Nodes))
	}
}

func TestCreateDAG_LinearChain(t *testing.T) {
	o := newTestOrchestrator()
	dag, err := o.CreateDAG("linear", []*DAGNode{
		{ID: "a", FeatureID: "f1"},
		{ID: "b", FeatureID: "f2", Dependencies: []string{"a"}},
		{ID: "c", FeatureID: "f3", Dependencies: []string{"b"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(dag.Nodes) != 3 {
		t.Fatalf("expected 3 nodes, got %d", len(dag.Nodes))
	}
}

func TestCreateDAG_DiamondShape(t *testing.T) {
	o := newTestOrchestrator()
	// A -> B, A -> C, B -> D, C -> D
	_, err := o.CreateDAG("diamond", []*DAGNode{
		{ID: "a", FeatureID: "f1"},
		{ID: "b", FeatureID: "f2", Dependencies: []string{"a"}},
		{ID: "c", FeatureID: "f3", Dependencies: []string{"a"}},
		{ID: "d", FeatureID: "f4", Dependencies: []string{"b", "c"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCreateDAG_DuplicateNodeID(t *testing.T) {
	o := newTestOrchestrator()
	_, err := o.CreateDAG("dup", []*DAGNode{
		{ID: "a", FeatureID: "f1"},
		{ID: "a", FeatureID: "f2"},
	})
	if err == nil {
		t.Fatal("expected duplicate node error")
	}
}

func TestCreateDAG_MissingDependency(t *testing.T) {
	o := newTestOrchestrator()
	_, err := o.CreateDAG("missing", []*DAGNode{
		{ID: "a", FeatureID: "f1", Dependencies: []string{"nonexistent"}},
	})
	if err == nil {
		t.Fatal("expected missing dependency error")
	}
}

func TestCreateDAG_CycleDetected(t *testing.T) {
	o := newTestOrchestrator()
	_, err := o.CreateDAG("cycle", []*DAGNode{
		{ID: "a", FeatureID: "f1", Dependencies: []string{"c"}},
		{ID: "b", FeatureID: "f2", Dependencies: []string{"a"}},
		{ID: "c", FeatureID: "f3", Dependencies: []string{"b"}},
	})
	if err == nil {
		t.Fatal("expected cycle error")
	}
}

func TestCreateDAG_SelfCycle(t *testing.T) {
	o := newTestOrchestrator()
	_, err := o.CreateDAG("self", []*DAGNode{
		{ID: "a", FeatureID: "f1", Dependencies: []string{"a"}},
	})
	if err == nil {
		t.Fatal("expected cycle error")
	}
}

// --- ExecuteDAG ---

func TestExecuteDAG_NotFound(t *testing.T) {
	o := newTestOrchestrator()
	err := o.ExecuteDAG("nonexistent")
	if err != ErrDAGNotFound {
		t.Fatalf("expected ErrDAGNotFound, got %v", err)
	}
}

func TestExecuteDAG_AlreadyRunning(t *testing.T) {
	o := newTestOrchestrator()
	dag, _ := o.CreateDAG("test", []*DAGNode{{ID: "a", FeatureID: "f1"}})
	_ = o.ExecuteDAG(dag.ID)
	err := o.ExecuteDAG(dag.ID)
	if err != ErrDAGAlreadyRunning {
		t.Fatalf("expected ErrDAGAlreadyRunning, got %v", err)
	}
}

func TestExecuteDAG_StartsReadyNodes(t *testing.T) {
	o := newTestOrchestrator()
	dag, _ := o.CreateDAG("test", []*DAGNode{
		{ID: "a", FeatureID: "f1"},
		{ID: "b", FeatureID: "f2", Dependencies: []string{"a"}},
	})
	_ = o.ExecuteDAG(dag.ID)

	if dag.Nodes["a"].Status != NodeRunning {
		t.Fatalf("expected node a to be running, got %s", dag.Nodes["a"].Status)
	}
	if dag.Nodes["b"].Status != NodePending {
		t.Fatalf("expected node b to be pending, got %s", dag.Nodes["b"].Status)
	}
}

func TestExecuteDAG_EmptyDAG(t *testing.T) {
	o := newTestOrchestrator()
	dag, _ := o.CreateDAG("empty", []*DAGNode{})
	err := o.ExecuteDAG(dag.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- GetReadyNodes ---

func TestGetReadyNodes_SatisfiedDeps(t *testing.T) {
	o := newTestOrchestrator()
	dag, _ := o.CreateDAG("test", []*DAGNode{
		{ID: "a", FeatureID: "f1"},
		{ID: "b", FeatureID: "f2", Dependencies: []string{"a"}},
	})

	ready := o.GetReadyNodes(dag.ID)
	if len(ready) != 1 || ready[0].ID != "a" {
		t.Fatalf("expected node a to be ready, got %v", ready)
	}
}

func TestGetReadyNodes_UnsatisfiedDeps(t *testing.T) {
	o := newTestOrchestrator()
	dag, _ := o.CreateDAG("test", []*DAGNode{
		{ID: "a", FeatureID: "f1"},
		{ID: "b", FeatureID: "f2", Dependencies: []string{"a"}},
	})
	_ = o.ExecuteDAG(dag.ID)

	// b should not be ready since a is running, not completed
	ready := o.GetReadyNodes(dag.ID)
	if len(ready) != 0 {
		t.Fatalf("expected no ready nodes, got %d", len(ready))
	}
}

func TestGetReadyNodes_NonexistentDAG(t *testing.T) {
	o := newTestOrchestrator()
	ready := o.GetReadyNodes("nonexistent")
	if ready != nil {
		t.Fatalf("expected nil, got %v", ready)
	}
}

// --- CompleteNode / FailNode ---

func TestCompleteNode_PromotesDownstream(t *testing.T) {
	o := newTestOrchestrator()
	dag, _ := o.CreateDAG("test", []*DAGNode{
		{ID: "a", FeatureID: "f1"},
		{ID: "b", FeatureID: "f2", Dependencies: []string{"a"}},
	})
	_ = o.ExecuteDAG(dag.ID)

	err := o.CompleteNode(dag.ID, "a")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dag.Nodes["a"].Status != NodeCompleted {
		t.Fatalf("expected node a completed, got %s", dag.Nodes["a"].Status)
	}
	if dag.Nodes["b"].Status != NodeRunning {
		t.Fatalf("expected node b promoted to running, got %s", dag.Nodes["b"].Status)
	}
}

func TestCompleteNode_DAGCompletes(t *testing.T) {
	o := newTestOrchestrator()
	dag, _ := o.CreateDAG("test", []*DAGNode{
		{ID: "a", FeatureID: "f1"},
	})
	_ = o.ExecuteDAG(dag.ID)
	_ = o.CompleteNode(dag.ID, "a")

	if dag.Status != "completed" {
		t.Fatalf("expected DAG status completed, got %s", dag.Status)
	}
}

func TestCompleteNode_DAGNotFound(t *testing.T) {
	o := newTestOrchestrator()
	err := o.CompleteNode("nonexistent", "a")
	if err != ErrDAGNotFound {
		t.Fatalf("expected ErrDAGNotFound, got %v", err)
	}
}

func TestCompleteNode_DAGNotRunning(t *testing.T) {
	o := newTestOrchestrator()
	dag, _ := o.CreateDAG("test", []*DAGNode{{ID: "a", FeatureID: "f1"}})
	err := o.CompleteNode(dag.ID, "a")
	if err != ErrDAGNotRunning {
		t.Fatalf("expected ErrDAGNotRunning, got %v", err)
	}
}

func TestCompleteNode_NodeNotFound(t *testing.T) {
	o := newTestOrchestrator()
	dag, _ := o.CreateDAG("test", []*DAGNode{{ID: "a", FeatureID: "f1"}})
	_ = o.ExecuteDAG(dag.ID)
	err := o.CompleteNode(dag.ID, "nonexistent")
	if err != ErrNodeNotFound {
		t.Fatalf("expected ErrNodeNotFound, got %v", err)
	}
}

func TestCompleteNode_NodeNotRunning(t *testing.T) {
	o := newTestOrchestrator()
	dag, _ := o.CreateDAG("test", []*DAGNode{
		{ID: "a", FeatureID: "f1"},
		{ID: "b", FeatureID: "f2", Dependencies: []string{"a"}},
	})
	_ = o.ExecuteDAG(dag.ID)
	// b is pending, not running
	err := o.CompleteNode(dag.ID, "b")
	if err != ErrNodeNotRunning {
		t.Fatalf("expected ErrNodeNotRunning, got %v", err)
	}
}

func TestFailNode_Retries(t *testing.T) {
	cfg := DefaultOrchestratorConfig()
	cfg.RetryLimit = 3
	o := NewOrchestrator(cfg)
	dag, _ := o.CreateDAG("test", []*DAGNode{{ID: "a", FeatureID: "f1"}})
	_ = o.ExecuteDAG(dag.ID)

	// First two failures should retry
	_ = o.FailNode(dag.ID, "a", "err1")
	if dag.Nodes["a"].Status != NodeRunning {
		t.Fatalf("expected retry (running), got %s", dag.Nodes["a"].Status)
	}
	_ = o.FailNode(dag.ID, "a", "err2")
	if dag.Nodes["a"].Status != NodeRunning {
		t.Fatalf("expected retry (running), got %s", dag.Nodes["a"].Status)
	}

	// Third failure should mark as failed
	_ = o.FailNode(dag.ID, "a", "err3")
	if dag.Nodes["a"].Status != NodeFailed {
		t.Fatalf("expected failed, got %s", dag.Nodes["a"].Status)
	}
}

func TestFailNode_SkipsDependents(t *testing.T) {
	cfg := DefaultOrchestratorConfig()
	cfg.RetryLimit = 1
	o := NewOrchestrator(cfg)
	dag, _ := o.CreateDAG("test", []*DAGNode{
		{ID: "a", FeatureID: "f1"},
		{ID: "b", FeatureID: "f2", Dependencies: []string{"a"}},
		{ID: "c", FeatureID: "f3", Dependencies: []string{"b"}},
	})
	_ = o.ExecuteDAG(dag.ID)

	_ = o.FailNode(dag.ID, "a", "fail")
	if dag.Nodes["b"].Status != NodeSkipped {
		t.Fatalf("expected node b skipped, got %s", dag.Nodes["b"].Status)
	}
	if dag.Nodes["c"].Status != NodeSkipped {
		t.Fatalf("expected node c skipped, got %s", dag.Nodes["c"].Status)
	}
}

func TestFailNode_DAGStatusFailed(t *testing.T) {
	cfg := DefaultOrchestratorConfig()
	cfg.RetryLimit = 1
	o := NewOrchestrator(cfg)
	dag, _ := o.CreateDAG("test", []*DAGNode{{ID: "a", FeatureID: "f1"}})
	_ = o.ExecuteDAG(dag.ID)
	_ = o.FailNode(dag.ID, "a", "fail")

	if dag.Status != "failed" {
		t.Fatalf("expected DAG status failed, got %s", dag.Status)
	}
}

func TestFailNode_DAGStatusPartial(t *testing.T) {
	cfg := DefaultOrchestratorConfig()
	cfg.RetryLimit = 1
	o := NewOrchestrator(cfg)
	dag, _ := o.CreateDAG("test", []*DAGNode{
		{ID: "a", FeatureID: "f1"},
		{ID: "b", FeatureID: "f2"},
	})
	_ = o.ExecuteDAG(dag.ID)
	_ = o.CompleteNode(dag.ID, "a")
	_ = o.FailNode(dag.ID, "b", "fail")

	if dag.Status != "partial" {
		t.Fatalf("expected DAG status partial, got %s", dag.Status)
	}
}

func TestAllNodesFail(t *testing.T) {
	cfg := DefaultOrchestratorConfig()
	cfg.RetryLimit = 1
	o := NewOrchestrator(cfg)
	dag, _ := o.CreateDAG("test", []*DAGNode{
		{ID: "a", FeatureID: "f1"},
		{ID: "b", FeatureID: "f2"},
	})
	_ = o.ExecuteDAG(dag.ID)
	_ = o.FailNode(dag.ID, "a", "fail")
	_ = o.FailNode(dag.ID, "b", "fail")

	if dag.Status != "failed" {
		t.Fatalf("expected DAG status failed, got %s", dag.Status)
	}
}

// --- EstimateCost ---

func TestEstimateCost_SingleNode(t *testing.T) {
	o := newTestOrchestrator()
	dag, _ := o.CreateDAG("test", []*DAGNode{{ID: "a", FeatureID: "f1"}})
	est, err := o.EstimateCost(dag.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if est.TotalNodes != 1 {
		t.Fatalf("expected 1 node, got %d", est.TotalNodes)
	}
	if est.EstimatedDurationSec != 30.0 {
		t.Fatalf("expected 30s duration, got %f", est.EstimatedDurationSec)
	}
	if est.RecommendedParallelism != 1 {
		t.Fatalf("expected parallelism 1, got %d", est.RecommendedParallelism)
	}
}

func TestEstimateCost_DiamondDAG(t *testing.T) {
	o := newTestOrchestrator()
	dag, _ := o.CreateDAG("diamond", []*DAGNode{
		{ID: "a", FeatureID: "f1"},
		{ID: "b", FeatureID: "f2", Dependencies: []string{"a"}},
		{ID: "c", FeatureID: "f3", Dependencies: []string{"a"}},
		{ID: "d", FeatureID: "f4", Dependencies: []string{"b", "c"}},
	})
	est, err := o.EstimateCost(dag.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if est.TotalNodes != 4 {
		t.Fatalf("expected 4 nodes, got %d", est.TotalNodes)
	}
	// Depth is 3: a->b->d or a->c->d
	if est.EstimatedDurationSec != 90.0 {
		t.Fatalf("expected 90s duration for depth 3, got %f", est.EstimatedDurationSec)
	}
}

func TestEstimateCost_NotFound(t *testing.T) {
	o := newTestOrchestrator()
	_, err := o.EstimateCost("nonexistent")
	if err != ErrDAGNotFound {
		t.Fatalf("expected ErrDAGNotFound, got %v", err)
	}
}

func TestEstimateCost_EmptyDAG(t *testing.T) {
	o := newTestOrchestrator()
	dag, _ := o.CreateDAG("empty", []*DAGNode{})
	est, err := o.EstimateCost(dag.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if est.TotalNodes != 0 {
		t.Fatalf("expected 0 nodes, got %d", est.TotalNodes)
	}
}

// --- DiamondDAG full execution ---

func TestExecuteDAG_DiamondFullExecution(t *testing.T) {
	o := newTestOrchestrator()
	dag, _ := o.CreateDAG("diamond", []*DAGNode{
		{ID: "a", FeatureID: "f1"},
		{ID: "b", FeatureID: "f2", Dependencies: []string{"a"}},
		{ID: "c", FeatureID: "f3", Dependencies: []string{"a"}},
		{ID: "d", FeatureID: "f4", Dependencies: []string{"b", "c"}},
	})
	_ = o.ExecuteDAG(dag.ID)

	// Only a should be running
	if dag.Nodes["a"].Status != NodeRunning {
		t.Fatalf("expected a running")
	}

	_ = o.CompleteNode(dag.ID, "a")
	// b and c should now be running
	if dag.Nodes["b"].Status != NodeRunning {
		t.Fatalf("expected b running after a complete")
	}
	if dag.Nodes["c"].Status != NodeRunning {
		t.Fatalf("expected c running after a complete")
	}

	_ = o.CompleteNode(dag.ID, "b")
	// d still pending since c not done
	if dag.Nodes["d"].Status != NodePending {
		t.Fatalf("expected d pending, got %s", dag.Nodes["d"].Status)
	}

	_ = o.CompleteNode(dag.ID, "c")
	// d should now be running
	if dag.Nodes["d"].Status != NodeRunning {
		t.Fatalf("expected d running after b,c complete")
	}

	_ = o.CompleteNode(dag.ID, "d")
	if dag.Status != "completed" {
		t.Fatalf("expected DAG completed, got %s", dag.Status)
	}
}

// --- Stats ---

func TestStats(t *testing.T) {
	cfg := DefaultOrchestratorConfig()
	cfg.RetryLimit = 1
	o := NewOrchestrator(cfg)

	dag1, _ := o.CreateDAG("d1", []*DAGNode{{ID: "a", FeatureID: "f1"}})
	_ = o.ExecuteDAG(dag1.ID)
	_ = o.CompleteNode(dag1.ID, "a")

	dag2, _ := o.CreateDAG("d2", []*DAGNode{{ID: "b", FeatureID: "f2"}})
	_ = o.ExecuteDAG(dag2.ID)
	_ = o.FailNode(dag2.ID, "b", "err")

	stats := o.Stats()
	if stats.TotalDAGs != 2 {
		t.Fatalf("expected 2 total DAGs, got %d", stats.TotalDAGs)
	}
	if stats.CompletedDAGs != 1 {
		t.Fatalf("expected 1 completed DAG, got %d", stats.CompletedDAGs)
	}
	if stats.FailedDAGs != 1 {
		t.Fatalf("expected 1 failed DAG, got %d", stats.FailedDAGs)
	}
	if stats.CompletedNodes != 1 {
		t.Fatalf("expected 1 completed node, got %d", stats.CompletedNodes)
	}
	if stats.FailedNodes != 1 {
		t.Fatalf("expected 1 failed node, got %d", stats.FailedNodes)
	}
}

// --- DeleteDAG ---

func TestDeleteDAG(t *testing.T) {
	o := newTestOrchestrator()
	dag, _ := o.CreateDAG("test", []*DAGNode{{ID: "a", FeatureID: "f1"}})
	err := o.DeleteDAG(dag.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_, err = o.GetDAG(dag.ID)
	if err != ErrDAGNotFound {
		t.Fatalf("expected ErrDAGNotFound after delete, got %v", err)
	}
}

func TestDeleteDAG_NotFound(t *testing.T) {
	o := newTestOrchestrator()
	err := o.DeleteDAG("nonexistent")
	if err != ErrDAGNotFound {
		t.Fatalf("expected ErrDAGNotFound, got %v", err)
	}
}

// --- ListDAGs ---

func TestListDAGs(t *testing.T) {
	o := newTestOrchestrator()
	o.CreateDAG("d1", []*DAGNode{{ID: "a", FeatureID: "f1"}})
	o.CreateDAG("d2", []*DAGNode{{ID: "b", FeatureID: "f2"}})

	dags := o.ListDAGs()
	if len(dags) != 2 {
		t.Fatalf("expected 2 DAGs, got %d", len(dags))
	}
}
