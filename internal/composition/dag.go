// Package composition provides real-time feature composition capabilities
// through DAG-based execution with parallel processing and caching.
package composition

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

// Errors for the composition package.
var (
	ErrNodeNotFound     = errors.New("node not found")
	ErrCycleDetected    = errors.New("cycle detected in DAG")
	ErrInvalidDAG       = errors.New("invalid DAG structure")
	ErrExecutionFailed  = errors.New("execution failed")
	ErrComputationError = errors.New("computation error")
	ErrTimeout          = errors.New("execution timeout")
)

// NodeType represents the type of computation node.
type NodeType string

const (
	// NodeTypeSource represents a raw feature from the store.
	NodeTypeSource NodeType = "source"
	// NodeTypeTransform represents a single transformation.
	NodeTypeTransform NodeType = "transform"
	// NodeTypeAggregate represents aggregation over multiple features.
	NodeTypeAggregate NodeType = "aggregate"
	// NodeTypeJoin represents joining features from different sources.
	NodeTypeJoin NodeType = "join"
	// NodeTypeFilter represents conditional filtering.
	NodeTypeFilter NodeType = "filter"
	// NodeTypeCustom represents a custom function node.
	NodeTypeCustom NodeType = "custom"
)

// Node represents a computation node in the DAG.
type Node struct {
	ID           string                 `json:"id"`
	Name         string                 `json:"name"`
	Type         NodeType               `json:"type"`
	Inputs       []string               `json:"inputs"`      // IDs of input nodes
	Expression   string                 `json:"expression"`  // Computation expression
	Config       map[string]interface{} `json:"config"`      // Node-specific configuration
	OutputType   string                 `json:"output_type"` // Expected output type
	CacheEnabled bool                   `json:"cache_enabled"`
	CacheTTL     time.Duration          `json:"cache_ttl"`
	Priority     int                    `json:"priority"` // Execution priority (higher = earlier)
	Timeout      time.Duration          `json:"timeout"`
	Retries      int                    `json:"retries"`
}

// DAG represents a Directed Acyclic Graph for feature composition.
type DAG struct {
	ID          string           `json:"id"`
	Name        string           `json:"name"`
	Description string           `json:"description"`
	Nodes       map[string]*Node `json:"nodes"`
	Outputs     []string         `json:"outputs"` // IDs of output nodes
	Version     int              `json:"version"`
	CreatedAt   time.Time        `json:"created_at"`
	UpdatedAt   time.Time        `json:"updated_at"`

	// Internal: computed topology
	topoOrder []string
	levels    map[string]int // Node ID -> level (for parallel execution)
	mu        sync.RWMutex
}

// NewDAG creates a new DAG with the given ID and name.
func NewDAG(id, name string) *DAG {
	return &DAG{
		ID:        id,
		Name:      name,
		Nodes:     make(map[string]*Node),
		Outputs:   []string{},
		Version:   1,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		levels:    make(map[string]int),
	}
}

// AddNode adds a node to the DAG.
func (d *DAG) AddNode(node *Node) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if node.ID == "" {
		return fmt.Errorf("%w: node ID cannot be empty", ErrInvalidDAG)
	}

	if _, exists := d.Nodes[node.ID]; exists {
		return fmt.Errorf("%w: node %s already exists", ErrInvalidDAG, node.ID)
	}

	// Set defaults
	if node.Timeout == 0 {
		node.Timeout = 30 * time.Second
	}
	if node.CacheTTL == 0 {
		node.CacheTTL = 5 * time.Minute
	}

	d.Nodes[node.ID] = node
	d.UpdatedAt = time.Now()

	return nil
}

// RemoveNode removes a node from the DAG.
func (d *DAG) RemoveNode(nodeID string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if _, exists := d.Nodes[nodeID]; !exists {
		return ErrNodeNotFound
	}

	// Check if any other node depends on this one
	for _, node := range d.Nodes {
		for _, input := range node.Inputs {
			if input == nodeID {
				return fmt.Errorf("%w: node %s is used by %s", ErrInvalidDAG, nodeID, node.ID)
			}
		}
	}

	delete(d.Nodes, nodeID)
	d.UpdatedAt = time.Now()

	return nil
}

// SetOutputs sets the output nodes of the DAG.
func (d *DAG) SetOutputs(outputs []string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Validate all outputs exist
	for _, output := range outputs {
		if _, exists := d.Nodes[output]; !exists {
			return fmt.Errorf("%w: output node %s not found", ErrNodeNotFound, output)
		}
	}

	d.Outputs = outputs
	d.UpdatedAt = time.Now()

	return nil
}

// Validate checks if the DAG is valid (no cycles, all dependencies exist).
func (d *DAG) Validate() error {
	d.mu.RLock()
	defer d.mu.RUnlock()

	return d.validateUnlocked()
}

func (d *DAG) validateUnlocked() error {
	// Check all dependencies exist
	for _, node := range d.Nodes {
		for _, input := range node.Inputs {
			if _, exists := d.Nodes[input]; !exists {
				return fmt.Errorf("%w: node %s depends on non-existent node %s", ErrInvalidDAG, node.ID, input)
			}
		}
	}

	// Check for cycles using DFS
	visited := make(map[string]bool)
	recStack := make(map[string]bool)

	for nodeID := range d.Nodes {
		if !visited[nodeID] {
			if err := d.detectCycle(nodeID, visited, recStack); err != nil {
				return err
			}
		}
	}

	return nil
}

func (d *DAG) detectCycle(nodeID string, visited, recStack map[string]bool) error {
	visited[nodeID] = true
	recStack[nodeID] = true

	node := d.Nodes[nodeID]
	for _, input := range node.Inputs {
		if !visited[input] {
			if err := d.detectCycle(input, visited, recStack); err != nil {
				return err
			}
		} else if recStack[input] {
			return fmt.Errorf("%w: cycle involves nodes %s and %s", ErrCycleDetected, nodeID, input)
		}
	}

	recStack[nodeID] = false
	return nil
}

// ComputeTopology computes the topological order and levels for execution.
func (d *DAG) ComputeTopology() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if err := d.validateUnlocked(); err != nil {
		return err
	}

	// Compute topological order using Kahn's algorithm
	inDegree := make(map[string]int)
	for nodeID := range d.Nodes {
		inDegree[nodeID] = 0
	}

	for _, node := range d.Nodes {
		for _, input := range node.Inputs {
			inDegree[node.ID]++
			_ = input // input contributes to this node's in-degree
		}
	}

	// Queue nodes with in-degree 0
	var queue []string
	for nodeID, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, nodeID)
			d.levels[nodeID] = 0
		}
	}

	var topoOrder []string
	for len(queue) > 0 {
		// Dequeue
		nodeID := queue[0]
		queue = queue[1:]
		topoOrder = append(topoOrder, nodeID)

		// For each dependent node
		for _, node := range d.Nodes {
			for _, input := range node.Inputs {
				if input == nodeID {
					inDegree[node.ID]--
					if inDegree[node.ID] == 0 {
						queue = append(queue, node.ID)
						// Level is max of input levels + 1
						if level := d.levels[nodeID] + 1; level > d.levels[node.ID] {
							d.levels[node.ID] = level
						}
					}
				}
			}
		}
	}

	if len(topoOrder) != len(d.Nodes) {
		return ErrCycleDetected
	}

	d.topoOrder = topoOrder
	return nil
}

// GetTopologicalOrder returns the computed topological order.
func (d *DAG) GetTopologicalOrder() []string {
	d.mu.RLock()
	defer d.mu.RUnlock()

	result := make([]string, len(d.topoOrder))
	copy(result, d.topoOrder)
	return result
}

// GetLevel returns the execution level of a node.
func (d *DAG) GetLevel(nodeID string) int {
	d.mu.RLock()
	defer d.mu.RUnlock()

	return d.levels[nodeID]
}

// GetNodesAtLevel returns all nodes at a specific execution level.
func (d *DAG) GetNodesAtLevel(level int) []*Node {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var nodes []*Node
	for nodeID, nodeLevel := range d.levels {
		if nodeLevel == level {
			nodes = append(nodes, d.Nodes[nodeID])
		}
	}

	return nodes
}

// GetMaxLevel returns the maximum execution level in the DAG.
func (d *DAG) GetMaxLevel() int {
	d.mu.RLock()
	defer d.mu.RUnlock()

	maxLevel := 0
	for _, level := range d.levels {
		if level > maxLevel {
			maxLevel = level
		}
	}
	return maxLevel
}

// GetNode retrieves a node by ID.
func (d *DAG) GetNode(nodeID string) (*Node, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	node, exists := d.Nodes[nodeID]
	if !exists {
		return nil, ErrNodeNotFound
	}
	return node, nil
}

// GetDependencies returns the direct dependencies of a node.
func (d *DAG) GetDependencies(nodeID string) ([]string, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	node, exists := d.Nodes[nodeID]
	if !exists {
		return nil, ErrNodeNotFound
	}

	result := make([]string, len(node.Inputs))
	copy(result, node.Inputs)
	return result, nil
}

// GetDependents returns nodes that depend on the given node.
func (d *DAG) GetDependents(nodeID string) []string {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var dependents []string
	for _, node := range d.Nodes {
		for _, input := range node.Inputs {
			if input == nodeID {
				dependents = append(dependents, node.ID)
				break
			}
		}
	}
	return dependents
}

// Clone creates a deep copy of the DAG.
func (d *DAG) Clone() *DAG {
	d.mu.RLock()
	defer d.mu.RUnlock()

	clone := &DAG{
		ID:          d.ID,
		Name:        d.Name,
		Description: d.Description,
		Nodes:       make(map[string]*Node, len(d.Nodes)),
		Outputs:     make([]string, len(d.Outputs)),
		Version:     d.Version,
		CreatedAt:   d.CreatedAt,
		UpdatedAt:   d.UpdatedAt,
		topoOrder:   make([]string, len(d.topoOrder)),
		levels:      make(map[string]int, len(d.levels)),
	}

	for id, node := range d.Nodes {
		clonedNode := &Node{
			ID:           node.ID,
			Name:         node.Name,
			Type:         node.Type,
			Inputs:       make([]string, len(node.Inputs)),
			Expression:   node.Expression,
			Config:       make(map[string]interface{}),
			OutputType:   node.OutputType,
			CacheEnabled: node.CacheEnabled,
			CacheTTL:     node.CacheTTL,
			Priority:     node.Priority,
			Timeout:      node.Timeout,
			Retries:      node.Retries,
		}
		copy(clonedNode.Inputs, node.Inputs)
		for k, v := range node.Config {
			clonedNode.Config[k] = v
		}
		clone.Nodes[id] = clonedNode
	}

	copy(clone.Outputs, d.Outputs)
	copy(clone.topoOrder, d.topoOrder)
	for k, v := range d.levels {
		clone.levels[k] = v
	}

	return clone
}

// DAGStats reports statistics about the DAG.
type DAGStats struct {
	NodeCount   int            `json:"node_count"`
	MaxLevel    int            `json:"max_level"`
	OutputCount int            `json:"output_count"`
	NodeTypes   map[string]int `json:"node_types"`
	CacheCount  int            `json:"cache_enabled_count"`
}

// Stats returns statistics about the DAG.
func (d *DAG) Stats() DAGStats {
	d.mu.RLock()
	defer d.mu.RUnlock()

	stats := DAGStats{
		NodeCount:   len(d.Nodes),
		MaxLevel:    d.GetMaxLevel(),
		OutputCount: len(d.Outputs),
		NodeTypes:   make(map[string]int),
	}

	for _, node := range d.Nodes {
		stats.NodeTypes[string(node.Type)]++
		if node.CacheEnabled {
			stats.CacheCount++
		}
	}

	return stats
}
