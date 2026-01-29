package backfill

import (
	"errors"
	"fmt"
	"math"
	"sync"
	"time"
)

// DAGNodeStatus represents the execution state of a DAG node.
type DAGNodeStatus string

const (
	NodePending   DAGNodeStatus = "pending"
	NodeRunning   DAGNodeStatus = "running"
	NodeCompleted DAGNodeStatus = "completed"
	NodeFailed    DAGNodeStatus = "failed"
	NodeSkipped   DAGNodeStatus = "skipped"
)

// DAGNode represents a single node in a backfill DAG.
type DAGNode struct {
	ID           string        `json:"id"`
	FeatureID    string        `json:"feature_id"`
	Dependencies []string      `json:"dependencies"`
	Status       DAGNodeStatus `json:"status"`
	StartTime    time.Time     `json:"start_time,omitempty"`
	EndTime      time.Time     `json:"end_time,omitempty"`
	Error        string        `json:"error,omitempty"`
	RetryCount   int           `json:"retry_count"`
}

// DAG represents a directed acyclic graph of backfill nodes.
type DAG struct {
	ID        string              `json:"id"`
	Name      string              `json:"name"`
	Nodes     map[string]*DAGNode `json:"nodes"`
	CreatedAt time.Time           `json:"created_at"`
	Status    string              `json:"status"`
}

// CostEstimate provides resource estimates for executing a DAG.
type CostEstimate struct {
	DAGID                  string  `json:"dag_id"`
	TotalNodes             int     `json:"total_nodes"`
	EstimatedDurationSec   float64 `json:"estimated_duration_sec"`
	EstimatedStorageMB     float64 `json:"estimated_storage_mb"`
	EstimatedCPUHours      float64 `json:"estimated_cpu_hours"`
	RecommendedParallelism int     `json:"recommended_parallelism"`
}

// OrchestratorConfig configures the DAG orchestrator.
type OrchestratorConfig struct {
	MaxParallelism       int  `json:"max_parallelism"`
	RetryLimit           int  `json:"retry_limit"`
	EnableCostEstimation bool `json:"enable_cost_estimation"`
	PreferOffPeak        bool `json:"prefer_off_peak"`
}

// DefaultOrchestratorConfig returns sensible defaults.
func DefaultOrchestratorConfig() OrchestratorConfig {
	return OrchestratorConfig{
		MaxParallelism:       4,
		RetryLimit:           3,
		EnableCostEstimation: true,
		PreferOffPeak:        false,
	}
}

// OrchestratorStats contains aggregate statistics across all DAGs.
type OrchestratorStats struct {
	TotalDAGs      int     `json:"total_dags"`
	CompletedDAGs  int     `json:"completed_dags"`
	FailedDAGs     int     `json:"failed_dags"`
	TotalNodes     int     `json:"total_nodes"`
	CompletedNodes int     `json:"completed_nodes"`
	FailedNodes    int     `json:"failed_nodes"`
	AvgDurationSec float64 `json:"avg_duration_sec"`
}

// Orchestrator manages DAG-based backfill execution.
type Orchestrator struct {
	mu     sync.RWMutex
	config OrchestratorConfig
	dags   map[string]*DAG
	seq    int
}

// NewOrchestrator creates a new DAG orchestrator.
func NewOrchestrator(cfg OrchestratorConfig) *Orchestrator {
	return &Orchestrator{
		config: cfg,
		dags:   make(map[string]*DAG),
	}
}

// Orchestrator errors.
var (
	ErrDAGNotFound       = errors.New("DAG not found")
	ErrDuplicateNodeID   = errors.New("duplicate node ID")
	ErrMissingDep        = errors.New("dependency references non-existent node")
	ErrCycleDetected     = errors.New("cycle detected in DAG")
	ErrNodeNotFound      = errors.New("node not found")
	ErrDAGNotRunning     = errors.New("DAG is not running")
	ErrNodeNotRunning    = errors.New("node is not in running state")
	ErrDAGAlreadyRunning = errors.New("DAG is already running")
)

// CreateDAG creates a new DAG from the given nodes after validation.
func (o *Orchestrator) CreateDAG(name string, nodes []*DAGNode) (*DAG, error) {
	// Build node map and validate uniqueness
	nodeMap := make(map[string]*DAGNode, len(nodes))
	for _, n := range nodes {
		if _, exists := nodeMap[n.ID]; exists {
			return nil, fmt.Errorf("%w: %s", ErrDuplicateNodeID, n.ID)
		}
		nodeMap[n.ID] = &DAGNode{
			ID:           n.ID,
			FeatureID:    n.FeatureID,
			Dependencies: n.Dependencies,
			Status:       NodePending,
		}
	}

	// Validate all dependencies reference existing nodes
	for _, n := range nodeMap {
		for _, dep := range n.Dependencies {
			if _, ok := nodeMap[dep]; !ok {
				return nil, fmt.Errorf("%w: %s -> %s", ErrMissingDep, n.ID, dep)
			}
		}
	}

	// Detect cycles via topological sort
	if err := validateNoCycles(nodeMap); err != nil {
		return nil, err
	}

	o.mu.Lock()
	defer o.mu.Unlock()

	o.seq++
	dag := &DAG{
		ID:        fmt.Sprintf("dag-%d", o.seq),
		Name:      name,
		Nodes:     nodeMap,
		CreatedAt: time.Now(),
		Status:    "pending",
	}
	o.dags[dag.ID] = dag
	return dag, nil
}

// validateNoCycles uses Kahn's algorithm to detect cycles.
func validateNoCycles(nodes map[string]*DAGNode) error {
	inDegree := make(map[string]int, len(nodes))
	for id := range nodes {
		inDegree[id] = 0
	}
	for _, n := range nodes {
		for _, dep := range n.Dependencies {
			inDegree[n.ID]++
			_ = dep // dep is the prerequisite
		}
	}

	// Recompute properly: for each node, each dependency adds to its in-degree
	for id := range nodes {
		inDegree[id] = len(nodes[id].Dependencies)
	}

	queue := make([]string, 0)
	for id, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, id)
		}
	}

	visited := 0
	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]
		visited++

		// Find nodes that depend on curr and reduce their in-degree
		for id, n := range nodes {
			for _, dep := range n.Dependencies {
				if dep == curr {
					inDegree[id]--
					if inDegree[id] == 0 {
						queue = append(queue, id)
					}
				}
			}
		}
	}

	if visited != len(nodes) {
		return ErrCycleDetected
	}
	return nil
}

// GetDAG retrieves a DAG by ID.
func (o *Orchestrator) GetDAG(id string) (*DAG, error) {
	o.mu.RLock()
	defer o.mu.RUnlock()

	dag, ok := o.dags[id]
	if !ok {
		return nil, ErrDAGNotFound
	}
	return dag, nil
}

// ListDAGs returns all DAGs.
func (o *Orchestrator) ListDAGs() []*DAG {
	o.mu.RLock()
	defer o.mu.RUnlock()

	result := make([]*DAG, 0, len(o.dags))
	for _, dag := range o.dags {
		result = append(result, dag)
	}
	return result
}

// ExecuteDAG begins execution of a DAG by marking it as running
// and starting all nodes whose dependencies are already satisfied.
func (o *Orchestrator) ExecuteDAG(id string) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	dag, ok := o.dags[id]
	if !ok {
		return ErrDAGNotFound
	}
	if dag.Status == "running" {
		return ErrDAGAlreadyRunning
	}

	dag.Status = "running"

	// Mark ready nodes as running
	for _, node := range dag.Nodes {
		if node.Status != NodePending {
			continue
		}
		if o.allDepsCompleted(dag, node) {
			node.Status = NodeRunning
			node.StartTime = time.Now()
		}
	}
	return nil
}

// GetReadyNodes returns nodes whose dependencies are all completed
// and that are currently in pending state.
func (o *Orchestrator) GetReadyNodes(dagID string) []*DAGNode {
	o.mu.RLock()
	defer o.mu.RUnlock()

	dag, ok := o.dags[dagID]
	if !ok {
		return nil
	}

	var ready []*DAGNode
	for _, node := range dag.Nodes {
		if node.Status == NodePending && o.allDepsCompleted(dag, node) {
			ready = append(ready, node)
		}
	}
	return ready
}

func (o *Orchestrator) allDepsCompleted(dag *DAG, node *DAGNode) bool {
	for _, dep := range node.Dependencies {
		depNode, ok := dag.Nodes[dep]
		if !ok || depNode.Status != NodeCompleted {
			return false
		}
	}
	return true
}

// CompleteNode marks a node as completed and promotes newly-ready nodes to running.
func (o *Orchestrator) CompleteNode(dagID, nodeID string) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	dag, ok := o.dags[dagID]
	if !ok {
		return ErrDAGNotFound
	}
	if dag.Status != "running" {
		return ErrDAGNotRunning
	}

	node, ok := dag.Nodes[nodeID]
	if !ok {
		return ErrNodeNotFound
	}
	if node.Status != NodeRunning {
		return ErrNodeNotRunning
	}

	node.Status = NodeCompleted
	node.EndTime = time.Now()

	// Promote newly-ready nodes
	for _, n := range dag.Nodes {
		if n.Status == NodePending && o.allDepsCompleted(dag, n) {
			n.Status = NodeRunning
			n.StartTime = time.Now()
		}
	}

	o.updateDAGStatus(dag)
	return nil
}

// FailNode marks a node as failed and skips downstream dependents if retries exhausted.
func (o *Orchestrator) FailNode(dagID, nodeID, errMsg string) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	dag, ok := o.dags[dagID]
	if !ok {
		return ErrDAGNotFound
	}
	if dag.Status != "running" {
		return ErrDAGNotRunning
	}

	node, ok := dag.Nodes[nodeID]
	if !ok {
		return ErrNodeNotFound
	}
	if node.Status != NodeRunning {
		return ErrNodeNotRunning
	}

	node.RetryCount++
	if node.RetryCount < o.config.RetryLimit {
		// Allow retry: reset to running
		node.Status = NodeRunning
		node.StartTime = time.Now()
		return nil
	}

	node.Status = NodeFailed
	node.EndTime = time.Now()
	node.Error = errMsg

	// Skip downstream nodes that depend on this failed node
	o.skipDependents(dag, nodeID)
	o.updateDAGStatus(dag)
	return nil
}

// skipDependents recursively marks nodes depending on failedID as skipped.
func (o *Orchestrator) skipDependents(dag *DAG, failedID string) {
	for _, n := range dag.Nodes {
		if n.Status != NodePending {
			continue
		}
		for _, dep := range n.Dependencies {
			if dep == failedID {
				n.Status = NodeSkipped
				o.skipDependents(dag, n.ID)
				break
			}
		}
	}
}

func (o *Orchestrator) updateDAGStatus(dag *DAG) {
	allDone := true
	anyFailed := false
	for _, n := range dag.Nodes {
		switch n.Status {
		case NodePending, NodeRunning:
			allDone = false
		case NodeFailed:
			anyFailed = true
		}
	}

	if !allDone {
		return
	}

	if anyFailed {
		// Check if some completed — partial success
		anyCompleted := false
		for _, n := range dag.Nodes {
			if n.Status == NodeCompleted {
				anyCompleted = true
				break
			}
		}
		if anyCompleted {
			dag.Status = "partial"
		} else {
			dag.Status = "failed"
		}
	} else {
		dag.Status = "completed"
	}
}

// EstimateCost provides a resource estimate for executing a DAG.
func (o *Orchestrator) EstimateCost(dagID string) (*CostEstimate, error) {
	o.mu.RLock()
	defer o.mu.RUnlock()

	dag, ok := o.dags[dagID]
	if !ok {
		return nil, ErrDAGNotFound
	}

	totalNodes := len(dag.Nodes)

	// Compute depth (longest path) for duration estimate
	depth := o.computeDepth(dag)
	parallelism := o.config.MaxParallelism
	if parallelism > totalNodes {
		parallelism = totalNodes
	}
	if parallelism < 1 {
		parallelism = 1
	}

	// Heuristic: ~30s per node at full parallelism
	estimatedDuration := float64(depth) * 30.0
	estimatedStorage := float64(totalNodes) * 50.0 // 50 MB per node
	estimatedCPU := float64(totalNodes) * 0.25     // 0.25 CPU-hours per node

	return &CostEstimate{
		DAGID:                  dagID,
		TotalNodes:             totalNodes,
		EstimatedDurationSec:   estimatedDuration,
		EstimatedStorageMB:     estimatedStorage,
		EstimatedCPUHours:      estimatedCPU,
		RecommendedParallelism: parallelism,
	}, nil
}

func (o *Orchestrator) computeDepth(dag *DAG) int {
	memo := make(map[string]int, len(dag.Nodes))
	maxDepth := 0
	for id := range dag.Nodes {
		d := o.nodeDepth(dag, id, memo)
		if d > maxDepth {
			maxDepth = d
		}
	}
	return maxDepth
}

func (o *Orchestrator) nodeDepth(dag *DAG, id string, memo map[string]int) int {
	if v, ok := memo[id]; ok {
		return v
	}
	node := dag.Nodes[id]
	if len(node.Dependencies) == 0 {
		memo[id] = 1
		return 1
	}
	maxDep := 0
	for _, dep := range node.Dependencies {
		d := o.nodeDepth(dag, dep, memo)
		if d > maxDep {
			maxDep = d
		}
	}
	memo[id] = maxDep + 1
	return maxDep + 1
}

// DeleteDAG removes a DAG by ID.
func (o *Orchestrator) DeleteDAG(id string) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	if _, ok := o.dags[id]; !ok {
		return ErrDAGNotFound
	}
	delete(o.dags, id)
	return nil
}

// Stats returns aggregate orchestrator statistics.
func (o *Orchestrator) Stats() *OrchestratorStats {
	o.mu.RLock()
	defer o.mu.RUnlock()

	stats := &OrchestratorStats{}
	var totalDuration float64
	completedWithDuration := 0

	for _, dag := range o.dags {
		stats.TotalDAGs++
		switch dag.Status {
		case "completed":
			stats.CompletedDAGs++
		case "failed":
			stats.FailedDAGs++
		}

		for _, node := range dag.Nodes {
			stats.TotalNodes++
			switch node.Status {
			case NodeCompleted:
				stats.CompletedNodes++
				if !node.EndTime.IsZero() && !node.StartTime.IsZero() {
					totalDuration += node.EndTime.Sub(node.StartTime).Seconds()
					completedWithDuration++
				}
			case NodeFailed:
				stats.FailedNodes++
			}
		}
	}

	if completedWithDuration > 0 {
		stats.AvgDurationSec = math.Round(totalDuration/float64(completedWithDuration)*100) / 100
	}

	return stats
}
