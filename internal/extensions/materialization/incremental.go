package materialization

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// IncrementalConfig configures the incremental computation engine.
type IncrementalConfig struct {
	CheckpointInterval time.Duration `json:"checkpoint_interval"`
	MaxBatchSize       int           `json:"max_batch_size"`
	ChangeBufferSize   int           `json:"change_buffer_size"`
	RateLimitPerSec    int           `json:"rate_limit_per_sec"`
}

// DefaultIncrementalConfig returns sensible defaults for incremental computation.
func DefaultIncrementalConfig() IncrementalConfig {
	return IncrementalConfig{
		CheckpointInterval: 30 * time.Second,
		MaxBatchSize:       1000,
		ChangeBufferSize:   10000,
		RateLimitPerSec:    5000,
	}
}

// ChangeEvent represents an upstream feature change.
type ChangeEvent struct {
	EntityKey   string            `json:"entity_key"`
	FeatureName string            `json:"feature_name"`
	OldValue    interface{}       `json:"old_value,omitempty"`
	NewValue    interface{}       `json:"new_value"`
	Timestamp   time.Time         `json:"timestamp"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// Checkpoint stores the state of incremental processing.
type Checkpoint struct {
	PipelineName string                 `json:"pipeline_name"`
	StepName     string                 `json:"step_name"`
	Offset       int64                  `json:"offset"`
	ProcessedAt  time.Time              `json:"processed_at"`
	RecordCount  int64                  `json:"record_count"`
	State        map[string]interface{} `json:"state,omitempty"`
}

// DAGNode represents a node in the materialization DAG.
type DAGNode struct {
	Name         string    `json:"name"`
	Dependencies []string  `json:"dependencies"`
	Dirty        bool      `json:"dirty"`
	LastComputed time.Time `json:"last_computed"`
	ComputeCount int64     `json:"compute_count"`
}

// IncrementalEngine extends the base Engine with incremental processing.
type IncrementalEngine struct {
	mu           sync.RWMutex
	config       IncrementalConfig
	base         *Engine
	dagNodes     map[string]*DAGNode
	checkpoints  map[string]*Checkpoint
	changeBuffer chan ChangeEvent
	dirty        map[string]bool
}

// NewIncrementalEngine creates an incremental materialization engine.
func NewIncrementalEngine(base *Engine, config IncrementalConfig) *IncrementalEngine {
	return &IncrementalEngine{
		config:       config,
		base:         base,
		dagNodes:     make(map[string]*DAGNode),
		checkpoints:  make(map[string]*Checkpoint),
		changeBuffer: make(chan ChangeEvent, config.ChangeBufferSize),
		dirty:        make(map[string]bool),
	}
}

// RegisterNode adds a node to the DAG.
func (e *IncrementalEngine) RegisterNode(name string, dependencies []string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if name == "" {
		return fmt.Errorf("registering DAG node: name is required")
	}

	// Verify dependencies exist.
	for _, dep := range dependencies {
		if _, ok := e.dagNodes[dep]; !ok && dep != "" {
			return fmt.Errorf("registering DAG node %q: dependency %q not found", name, dep)
		}
	}

	// Check for cycles.
	e.dagNodes[name] = &DAGNode{Name: name, Dependencies: dependencies}
	if e.hasCycleUnlocked() {
		delete(e.dagNodes, name)
		return fmt.Errorf("registering DAG node %q: would create a cycle", name)
	}

	return nil
}

// NotifyChange marks a node and its downstream dependents as dirty.
func (e *IncrementalEngine) NotifyChange(nodeName string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, ok := e.dagNodes[nodeName]; !ok {
		return fmt.Errorf("notifying change: node %q not found", nodeName)
	}

	e.markDirtyDownstream(nodeName)
	return nil
}

// GetDirtyNodes returns all nodes that need recomputation.
func (e *IncrementalEngine) GetDirtyNodes() []string {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var result []string
	for name := range e.dirty {
		result = append(result, name)
	}
	return result
}

// ComputeIncremental recomputes only dirty nodes in topological order.
func (e *IncrementalEngine) ComputeIncremental(ctx context.Context) (*IncrementalResult, error) {
	e.mu.Lock()
	dirtyList := make([]string, 0, len(e.dirty))
	for name := range e.dirty {
		dirtyList = append(dirtyList, name)
	}
	e.mu.Unlock()

	if len(dirtyList) == 0 {
		return &IncrementalResult{
			NodesProcessed: 0,
			NodesSkipped:   len(e.dagNodes),
		}, nil
	}

	sorted, err := e.topologicalSortDirty(dirtyList)
	if err != nil {
		return nil, fmt.Errorf("computing incremental: %w", err)
	}

	start := time.Now()
	processed := 0
	for _, name := range sorted {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		e.mu.Lock()
		node := e.dagNodes[name]
		if node != nil {
			node.LastComputed = time.Now()
			node.ComputeCount++
			node.Dirty = false
			delete(e.dirty, name)
		}
		e.mu.Unlock()
		processed++
	}

	return &IncrementalResult{
		NodesProcessed: processed,
		NodesSkipped:   len(e.dagNodes) - processed,
		ExecutionTime:  time.Since(start),
	}, nil
}

// IncrementalResult captures the outcome of incremental computation.
type IncrementalResult struct {
	NodesProcessed int           `json:"nodes_processed"`
	NodesSkipped   int           `json:"nodes_skipped"`
	ExecutionTime  time.Duration `json:"execution_time"`
}

// SaveCheckpoint saves the current processing state for a pipeline step.
func (e *IncrementalEngine) SaveCheckpoint(pipelineName, stepName string, offset int64, state map[string]interface{}) {
	e.mu.Lock()
	defer e.mu.Unlock()

	key := pipelineName + "/" + stepName
	e.checkpoints[key] = &Checkpoint{
		PipelineName: pipelineName,
		StepName:     stepName,
		Offset:       offset,
		ProcessedAt:  time.Now(),
		State:        state,
	}
}

// GetCheckpoint retrieves the last checkpoint for a pipeline step.
func (e *IncrementalEngine) GetCheckpoint(pipelineName, stepName string) *Checkpoint {
	e.mu.RLock()
	defer e.mu.RUnlock()

	key := pipelineName + "/" + stepName
	return e.checkpoints[key]
}

// GetDAG returns a snapshot of the DAG nodes.
func (e *IncrementalEngine) GetDAG() []*DAGNode {
	e.mu.RLock()
	defer e.mu.RUnlock()

	nodes := make([]*DAGNode, 0, len(e.dagNodes))
	for _, n := range e.dagNodes {
		cp := *n
		cp.Dirty = e.dirty[n.Name]
		nodes = append(nodes, &cp)
	}
	return nodes
}

func (e *IncrementalEngine) markDirtyDownstream(name string) {
	e.dirty[name] = true
	if node, ok := e.dagNodes[name]; ok {
		node.Dirty = true
	}

	// Find nodes that depend on this one.
	for nName, node := range e.dagNodes {
		for _, dep := range node.Dependencies {
			if dep == name && !e.dirty[nName] {
				e.markDirtyDownstream(nName)
			}
		}
	}
}

func (e *IncrementalEngine) hasCycleUnlocked() bool {
	visited := make(map[string]int) // 0=unvisited, 1=in-progress, 2=done
	var hasCycle bool

	var dfs func(name string)
	dfs = func(name string) {
		if hasCycle {
			return
		}
		visited[name] = 1
		if node, ok := e.dagNodes[name]; ok {
			for _, dep := range node.Dependencies {
				switch visited[dep] {
				case 1:
					hasCycle = true
					return
				case 0:
					dfs(dep)
				}
			}
		}
		visited[name] = 2
	}

	for name := range e.dagNodes {
		if visited[name] == 0 {
			dfs(name)
		}
	}
	return hasCycle
}

func (e *IncrementalEngine) topologicalSortDirty(dirty []string) ([]string, error) {
	dirtySet := make(map[string]bool)
	for _, n := range dirty {
		dirtySet[n] = true
	}

	inDegree := make(map[string]int)
	for _, name := range dirty {
		if _, ok := inDegree[name]; !ok {
			inDegree[name] = 0
		}
		if node, ok := e.dagNodes[name]; ok {
			for _, dep := range node.Dependencies {
				if dirtySet[dep] {
					inDegree[name]++
				}
			}
		}
	}

	var queue []string
	for name, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, name)
		}
	}

	var result []string
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		result = append(result, cur)

		for name := range dirtySet {
			if node, ok := e.dagNodes[name]; ok {
				for _, dep := range node.Dependencies {
					if dep == cur {
						inDegree[name]--
						if inDegree[name] == 0 {
							queue = append(queue, name)
						}
					}
				}
			}
		}
	}

	if len(result) != len(dirty) {
		return nil, fmt.Errorf("cycle detected in dirty nodes")
	}
	return result, nil
}
