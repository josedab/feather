package pipelinebuilder

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"
)

// NodeType classifies a pipeline node.
type NodeType string

const (
	NodeSource    NodeType = "source"
	NodeTransform NodeType = "transform"
	NodeSink      NodeType = "sink"
	NodeFilter    NodeType = "filter"
	NodeJoin      NodeType = "join"
	NodeAggregate NodeType = "aggregate"
	NodeCustom    NodeType = "custom"
)

// PipelineStatus tracks the lifecycle of a pipeline.
type PipelineStatus string

const (
	StatusDraft     PipelineStatus = "draft"
	StatusValidated PipelineStatus = "validated"
	StatusCompiled  PipelineStatus = "compiled"
	StatusDeployed  PipelineStatus = "deployed"
	StatusFailed    PipelineStatus = "failed"
)

// PipelineConfig holds constraints for pipeline construction.
type PipelineConfig struct {
	MaxNodes    int  `json:"max_nodes"`
	MaxDepth    int  `json:"max_depth"`
	AllowCycles bool `json:"allow_cycles"`
}

// DefaultPipelineConfig returns sensible defaults.
func DefaultPipelineConfig() PipelineConfig {
	return PipelineConfig{
		MaxNodes:    100,
		MaxDepth:    20,
		AllowCycles: false,
	}
}

// Position represents the visual placement of a node.
type Position struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// PipelineNode is a single node in the pipeline DAG.
type PipelineNode struct {
	ID       string                 `json:"id"`
	Type     NodeType               `json:"type"`
	Name     string                 `json:"name"`
	Config   map[string]interface{} `json:"config,omitempty"`
	Inputs   []string               `json:"inputs,omitempty"`
	Position Position               `json:"position"`
}

// ValidationError describes a problem found during validation.
type ValidationError struct {
	NodeID   string `json:"node_id,omitempty"`
	Field    string `json:"field,omitempty"`
	Message  string `json:"message"`
	Severity string `json:"severity"` // "error" | "warning"
}

// Pipeline is the top-level pipeline definition.
type Pipeline struct {
	ID          string                   `json:"id"`
	Name        string                   `json:"name"`
	Description string                   `json:"description"`
	Nodes       map[string]*PipelineNode `json:"nodes"`
	Status      PipelineStatus           `json:"status"`
	CreatedAt   time.Time                `json:"created_at"`
	UpdatedAt   time.Time                `json:"updated_at"`
	Version     int                      `json:"version"`
	Tags        []string                 `json:"tags,omitempty"`
	Author      string                   `json:"author,omitempty"`

	config PipelineConfig
	mu     sync.RWMutex
}

// NewPipeline creates a new draft pipeline.
func NewPipeline(name, description string) *Pipeline {
	now := time.Now()
	return &Pipeline{
		ID:          generateID(),
		Name:        name,
		Description: description,
		Nodes:       make(map[string]*PipelineNode),
		Status:      StatusDraft,
		CreatedAt:   now,
		UpdatedAt:   now,
		Version:     1,
		config:      DefaultPipelineConfig(),
	}
}

// AddNode adds a node to the pipeline.
func (p *Pipeline) AddNode(node *PipelineNode) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if node.ID == "" {
		return errors.New("node ID is required")
	}
	if _, exists := p.Nodes[node.ID]; exists {
		return fmt.Errorf("node %q already exists", node.ID)
	}
	if len(p.Nodes) >= p.config.MaxNodes {
		return fmt.Errorf("pipeline exceeds max nodes (%d)", p.config.MaxNodes)
	}
	p.Nodes[node.ID] = node
	p.UpdatedAt = time.Now()
	return nil
}

// RemoveNode removes a node and cleans up references from other nodes.
func (p *Pipeline) RemoveNode(id string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if _, exists := p.Nodes[id]; !exists {
		return fmt.Errorf("node %q not found", id)
	}
	delete(p.Nodes, id)
	// Remove references to the deleted node from other nodes' inputs.
	for _, n := range p.Nodes {
		filtered := n.Inputs[:0]
		for _, inp := range n.Inputs {
			if inp != id {
				filtered = append(filtered, inp)
			}
		}
		n.Inputs = filtered
	}
	p.UpdatedAt = time.Now()
	return nil
}

// Connect adds an edge from fromID to toID.
func (p *Pipeline) Connect(fromID, toID string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if _, ok := p.Nodes[fromID]; !ok {
		return fmt.Errorf("source node %q not found", fromID)
	}
	to, ok := p.Nodes[toID]
	if !ok {
		return fmt.Errorf("target node %q not found", toID)
	}
	for _, inp := range to.Inputs {
		if inp == fromID {
			return fmt.Errorf("connection from %q to %q already exists", fromID, toID)
		}
	}

	// Cycle detection when cycles are disallowed.
	if !p.config.AllowCycles {
		to.Inputs = append(to.Inputs, fromID)
		if p.hasCycle() {
			to.Inputs = to.Inputs[:len(to.Inputs)-1]
			return fmt.Errorf("connection from %q to %q would create a cycle", fromID, toID)
		}
	} else {
		to.Inputs = append(to.Inputs, fromID)
	}
	p.UpdatedAt = time.Now()
	return nil
}

// Validate checks the pipeline for structural problems.
func (p *Pipeline) Validate() []ValidationError {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var errs []ValidationError

	if p.Name == "" {
		errs = append(errs, ValidationError{Field: "name", Message: "pipeline name is required", Severity: "error"})
	}
	if len(p.Nodes) == 0 {
		errs = append(errs, ValidationError{Message: "pipeline has no nodes", Severity: "error"})
		return errs
	}

	// Check each node.
	for _, n := range p.Nodes {
		if n.Name == "" {
			errs = append(errs, ValidationError{NodeID: n.ID, Field: "name", Message: "node name is required", Severity: "error"})
		}
		for _, inp := range n.Inputs {
			if _, ok := p.Nodes[inp]; !ok {
				errs = append(errs, ValidationError{NodeID: n.ID, Field: "inputs", Message: fmt.Sprintf("input node %q does not exist", inp), Severity: "error"})
			}
		}
	}

	// Cycle detection.
	if !p.config.AllowCycles && p.hasCycle() {
		errs = append(errs, ValidationError{Message: "pipeline contains a cycle", Severity: "error"})
	}

	// Warn about disconnected nodes (no inputs and not referenced by anyone).
	referenced := make(map[string]bool)
	for _, n := range p.Nodes {
		for _, inp := range n.Inputs {
			referenced[inp] = true
		}
	}
	for id, n := range p.Nodes {
		if len(n.Inputs) == 0 && !referenced[id] && len(p.Nodes) > 1 {
			errs = append(errs, ValidationError{NodeID: id, Message: "node is disconnected", Severity: "warning"})
		}
	}

	return errs
}

// TopologicalSort returns node IDs in execution order.
func (p *Pipeline) TopologicalSort() ([]string, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	inDegree := make(map[string]int, len(p.Nodes))
	for id := range p.Nodes {
		inDegree[id] = 0
	}
	for _, n := range p.Nodes {
		for _, inp := range n.Inputs {
			if _, ok := p.Nodes[inp]; ok {
				inDegree[n.ID]++
			}
		}
	}

	var queue []string
	for id, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, id)
		}
	}

	var sorted []string
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		sorted = append(sorted, cur)

		for _, n := range p.Nodes {
			for _, inp := range n.Inputs {
				if inp == cur {
					inDegree[n.ID]--
					if inDegree[n.ID] == 0 {
						queue = append(queue, n.ID)
					}
				}
			}
		}
	}

	if len(sorted) != len(p.Nodes) {
		return nil, errors.New("pipeline contains a cycle")
	}
	return sorted, nil
}

// hasCycle returns true if the graph contains a cycle (DFS-based).
// Must be called with at least a read lock held.
func (p *Pipeline) hasCycle() bool {
	const (
		white = 0
		gray  = 1
		black = 2
	)
	color := make(map[string]int, len(p.Nodes))
	// Build adjacency: parent -> children.
	children := make(map[string][]string, len(p.Nodes))
	for _, n := range p.Nodes {
		for _, inp := range n.Inputs {
			children[inp] = append(children[inp], n.ID)
		}
	}

	var dfs func(id string) bool
	dfs = func(id string) bool {
		color[id] = gray
		for _, c := range children[id] {
			if color[c] == gray {
				return true
			}
			if color[c] == white && dfs(c) {
				return true
			}
		}
		color[id] = black
		return false
	}

	for id := range p.Nodes {
		if color[id] == white {
			if dfs(id) {
				return true
			}
		}
	}
	return false
}

func generateID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
