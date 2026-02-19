package incrmat

import (
	"fmt"
	"sync"
	"time"
)

// ChangeEvent captures an upstream data mutation.
type ChangeEvent struct {
	EntityID      string
	FeatureGroup  string
	ChangedFields []string
	Timestamp     time.Time
	Version       int64
}

// MaterializationNode represents a feature computation with dependencies.
type MaterializationNode struct {
	ID               string
	FeatureGroup     string
	Dependencies     []string
	Expression       string
	LastMaterialized time.Time
	Version          int64
	Dirty            bool
}

// MaterializationResult holds the outcome of materializing a single node.
type MaterializationResult struct {
	NodeID            string
	EntitiesProcessed int64
	Skipped           int64
	Duration          time.Duration
	Timestamp         time.Time
}

// EngineConfig configures the incremental materialization engine.
type EngineConfig struct {
	MaxNodes           int
	BatchSize          int
	MaxPendingChanges  int
	CheckpointInterval time.Duration
}

// DefaultEngineConfig returns sensible defaults.
func DefaultEngineConfig() EngineConfig {
	return EngineConfig{
		MaxNodes:           10000,
		BatchSize:          1000,
		MaxPendingChanges:  100000,
		CheckpointInterval: 30 * time.Second,
	}
}

// EngineStats holds aggregate engine statistics.
type EngineStats struct {
	TotalNodes     int
	DirtyNodes     int
	TotalChanges   int64
	TotalProcessed int64
	TotalSkipped   int64
}

// Engine manages a DAG of materialization nodes and processes changes.
type Engine struct {
	mu             sync.RWMutex
	config         EngineConfig
	nodes          map[string]*MaterializationNode
	dependents     map[string][]string // reverse dependency index
	pendingChanges []ChangeEvent
	results        []MaterializationResult
	totalProcessed int64
	totalSkipped   int64
	totalChanges   int64
}

// NewEngine creates a new Engine with the given configuration.
func NewEngine(config EngineConfig) *Engine {
	return &Engine{
		config:     config,
		nodes:      make(map[string]*MaterializationNode),
		dependents: make(map[string][]string),
	}
}

// RegisterNode adds a node to the DAG after validating no cycles would be created.
func (e *Engine) RegisterNode(node MaterializationNode) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if len(e.nodes) >= e.config.MaxNodes {
		return fmt.Errorf("maximum nodes (%d) reached", e.config.MaxNodes)
	}

	// Check that dependencies exist or will be satisfied
	// and detect cycles
	if err := e.detectCycleLocked(node.ID, node.Dependencies); err != nil {
		return err
	}

	cp := node
	e.nodes[node.ID] = &cp

	// Build reverse dependency index
	for _, dep := range node.Dependencies {
		e.dependents[dep] = append(e.dependents[dep], node.ID)
	}

	return nil
}

// detectCycleLocked checks if adding a node with given deps would create a cycle.
func (e *Engine) detectCycleLocked(nodeID string, deps []string) error {
	// Check if any dependency transitively depends on nodeID
	visited := make(map[string]bool)
	var hasCycle func(current string) bool
	hasCycle = func(current string) bool {
		if current == nodeID {
			return true
		}
		if visited[current] {
			return false
		}
		visited[current] = true
		for _, downstream := range e.dependents[current] {
			if hasCycle(downstream) {
				return true
			}
		}
		return false
	}

	for _, dep := range deps {
		if dep == nodeID {
			return ErrCyclicDependency
		}
		visited = make(map[string]bool)
		if hasCycle(dep) {
			return ErrCyclicDependency
		}
	}
	return nil
}

// RemoveNode removes a node from the DAG.
func (e *Engine) RemoveNode(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	node, exists := e.nodes[id]
	if !exists {
		return ErrNodeNotRegistered
	}

	// Remove from reverse dependency index
	for _, dep := range node.Dependencies {
		deps := e.dependents[dep]
		for i, d := range deps {
			if d == id {
				e.dependents[dep] = append(deps[:i], deps[i+1:]...)
				break
			}
		}
	}

	delete(e.nodes, id)
	return nil
}

// ListNodes returns all registered nodes.
func (e *Engine) ListNodes() []MaterializationNode {
	e.mu.RLock()
	defer e.mu.RUnlock()

	out := make([]MaterializationNode, 0, len(e.nodes))
	for _, n := range e.nodes {
		out = append(out, *n)
	}
	return out
}

// RecordChange marks upstream nodes dirty and propagates to dependents.
func (e *Engine) RecordChange(event ChangeEvent) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.totalChanges++
	if len(e.pendingChanges) < e.config.MaxPendingChanges {
		e.pendingChanges = append(e.pendingChanges, event)
	}

	// Mark matching nodes dirty and propagate
	for id, node := range e.nodes {
		if node.FeatureGroup == event.FeatureGroup {
			e.markDirtyLocked(id)
		}
	}
}

// markDirtyLocked marks a node and all its dependents as dirty.
func (e *Engine) markDirtyLocked(id string) {
	node, ok := e.nodes[id]
	if !ok || node.Dirty {
		return
	}
	node.Dirty = true
	for _, dep := range e.dependents[id] {
		e.markDirtyLocked(dep)
	}
}

// GetDirtyNodes returns all nodes that need materialization.
func (e *Engine) GetDirtyNodes() []MaterializationNode {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var dirty []MaterializationNode
	for _, n := range e.nodes {
		if n.Dirty {
			dirty = append(dirty, *n)
		}
	}
	return dirty
}

// Materialize processes all dirty nodes in topological order.
func (e *Engine) Materialize() []MaterializationResult {
	e.mu.Lock()
	defer e.mu.Unlock()

	order := e.topoSortLocked()
	var results []MaterializationResult

	for _, id := range order {
		node := e.nodes[id]
		if !node.Dirty {
			e.totalSkipped++
			continue
		}

		start := time.Now()

		// Simulate materialization
		node.Dirty = false
		node.LastMaterialized = time.Now()
		node.Version++
		e.totalProcessed++

		result := MaterializationResult{
			NodeID:            id,
			EntitiesProcessed: int64(e.config.BatchSize),
			Duration:          time.Since(start),
			Timestamp:         time.Now(),
		}
		results = append(results, result)
		e.results = append(e.results, result)
	}

	e.pendingChanges = nil
	return results
}

// topoSortLocked returns node IDs in topological order (dependencies first).
func (e *Engine) topoSortLocked() []string {
	inDegree := make(map[string]int)
	for id := range e.nodes {
		inDegree[id] = 0
	}
	for id, node := range e.nodes {
		for _, dep := range node.Dependencies {
			if _, ok := e.nodes[dep]; ok {
				inDegree[id]++
			}
		}
	}

	var queue []string
	for id, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, id)
		}
	}

	var order []string
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		order = append(order, current)

		for _, dep := range e.dependents[current] {
			inDegree[dep]--
			if inDegree[dep] == 0 {
				queue = append(queue, dep)
			}
		}
	}

	return order
}

// GetResults returns the most recent materialization results, up to limit.
func (e *Engine) GetResults(limit int) []MaterializationResult {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if limit > len(e.results) {
		limit = len(e.results)
	}
	start := len(e.results) - limit
	out := make([]MaterializationResult, limit)
	copy(out, e.results[start:])
	return out
}

// Stats returns aggregate engine statistics.
func (e *Engine) Stats() EngineStats {
	e.mu.RLock()
	defer e.mu.RUnlock()

	dirty := 0
	for _, n := range e.nodes {
		if n.Dirty {
			dirty++
		}
	}

	return EngineStats{
		TotalNodes:     len(e.nodes),
		DirtyNodes:     dirty,
		TotalChanges:   e.totalChanges,
		TotalProcessed: e.totalProcessed,
		TotalSkipped:   e.totalSkipped,
	}
}
