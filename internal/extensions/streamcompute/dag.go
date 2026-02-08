package streamcompute

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// DAGNodeType defines the type of a DAG node.
type DAGNodeType string

const (
	DAGNodeSource    DAGNodeType = "source"
	DAGNodeTransform DAGNodeType = "transform"
	DAGNodeWindow    DAGNodeType = "window"
	DAGNodeSink      DAGNodeType = "sink"
)

// DAGNode represents a node in a processing DAG.
type DAGNode struct {
	ID          string      `json:"id"`
	Type        DAGNodeType `json:"type"`
	PipelineID  string      `json:"pipeline_id,omitempty"` // References a pipeline for window nodes
	Upstream    []string    `json:"upstream,omitempty"`
	Downstream  []string    `json:"downstream,omitempty"`
	Config      interface{} `json:"config,omitempty"`
	Status      string      `json:"status"` // "idle", "running", "backpressure", "error"
	EventsIn    int64       `json:"events_in"`
	EventsOut   int64       `json:"events_out"`
	LastEventAt time.Time   `json:"last_event_at,omitempty"`
}

// DAGConfig configures a processing DAG.
type DAGConfig struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Nodes       []DAGNode `json:"nodes"`
}

// DAGOrchestrator manages DAG-based pipeline orchestration with backpressure.
type DAGOrchestrator struct {
	mu                sync.RWMutex
	dags              map[string]*dagInstance
	engine            *Engine
	backpressureLimit int
}

type dagInstance struct {
	config DAGConfig
	nodes  map[string]*DAGNode
	status string // "created", "running", "stopped"
	stopCh chan struct{}
}

// NewDAGOrchestrator creates a new DAG orchestrator.
func NewDAGOrchestrator(engine *Engine, backpressureLimit int) *DAGOrchestrator {
	if backpressureLimit <= 0 {
		backpressureLimit = 10000
	}
	return &DAGOrchestrator{
		dags:              make(map[string]*dagInstance),
		engine:            engine,
		backpressureLimit: backpressureLimit,
	}
}

// CreateDAG creates a new processing DAG.
func (o *DAGOrchestrator) CreateDAG(cfg DAGConfig) error {
	if cfg.ID == "" {
		return fmt.Errorf("DAG ID is required")
	}
	if len(cfg.Nodes) == 0 {
		return fmt.Errorf("DAG must have at least one node")
	}
	if err := o.validateDAG(cfg); err != nil {
		return err
	}

	o.mu.Lock()
	defer o.mu.Unlock()

	if _, exists := o.dags[cfg.ID]; exists {
		return fmt.Errorf("DAG %q already exists", cfg.ID)
	}

	instance := &dagInstance{
		config: cfg,
		nodes:  make(map[string]*DAGNode),
		status: "created",
		stopCh: make(chan struct{}),
	}
	for i := range cfg.Nodes {
		node := cfg.Nodes[i]
		node.Status = "idle"
		instance.nodes[node.ID] = &node
	}

	o.dags[cfg.ID] = instance
	return nil
}

// StartDAG starts a DAG for processing.
func (o *DAGOrchestrator) StartDAG(ctx context.Context, dagID string) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	dag, exists := o.dags[dagID]
	if !exists {
		return fmt.Errorf("DAG %q not found", dagID)
	}

	dag.status = "running"
	for _, node := range dag.nodes {
		node.Status = "running"
	}
	return nil
}

// StopDAG stops a DAG.
func (o *DAGOrchestrator) StopDAG(dagID string) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	dag, exists := o.dags[dagID]
	if !exists {
		return fmt.Errorf("DAG %q not found", dagID)
	}

	close(dag.stopCh)
	dag.status = "stopped"
	for _, node := range dag.nodes {
		node.Status = "idle"
	}
	return nil
}

// IngestToDAG processes an event through the DAG, following edges in topological order.
func (o *DAGOrchestrator) IngestToDAG(dagID string, event Event) ([]WindowResult, error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	dag, exists := o.dags[dagID]
	if !exists {
		return nil, fmt.Errorf("DAG %q not found", dagID)
	}
	if dag.status != "running" {
		return nil, fmt.Errorf("DAG %q is not running", dagID)
	}

	// Find source nodes (no upstream)
	sources := make([]string, 0)
	for _, node := range dag.nodes {
		if len(node.Upstream) == 0 {
			sources = append(sources, node.ID)
		}
	}

	var allResults []WindowResult

	// Process in topological order starting from sources
	visited := make(map[string]bool)
	queue := make([]string, 0, len(sources))
	queue = append(queue, sources...)

	for len(queue) > 0 {
		nodeID := queue[0]
		queue = queue[1:]

		if visited[nodeID] {
			continue
		}
		visited[nodeID] = true

		node, ok := dag.nodes[nodeID]
		if !ok {
			continue
		}

		// Backpressure check
		if node.EventsIn-node.EventsOut > int64(o.backpressureLimit) {
			node.Status = "backpressure"
			continue
		}

		node.EventsIn++
		node.LastEventAt = event.Timestamp

		// For window nodes, delegate to the pipeline engine
		if node.Type == DAGNodeWindow && node.PipelineID != "" {
			results := o.engine.Ingest(event)
			allResults = append(allResults, results...)
			node.EventsOut += int64(len(results))
		} else {
			node.EventsOut++
		}

		// Enqueue downstream nodes
		for _, downID := range node.Downstream {
			if !visited[downID] {
				queue = append(queue, downID)
			}
		}
	}

	return allResults, nil
}

// GetDAG returns DAG info.
func (o *DAGOrchestrator) GetDAG(dagID string) (*DAGConfig, error) {
	o.mu.RLock()
	defer o.mu.RUnlock()

	dag, exists := o.dags[dagID]
	if !exists {
		return nil, fmt.Errorf("DAG %q not found", dagID)
	}

	// Build config with current node state
	cfg := dag.config
	cfg.Nodes = make([]DAGNode, 0, len(dag.nodes))
	for _, n := range dag.nodes {
		cfg.Nodes = append(cfg.Nodes, *n)
	}
	return &cfg, nil
}

// ListDAGs returns all DAGs.
func (o *DAGOrchestrator) ListDAGs() []DAGConfig {
	o.mu.RLock()
	defer o.mu.RUnlock()

	result := make([]DAGConfig, 0, len(o.dags))
	for _, dag := range o.dags {
		cfg := dag.config
		cfg.Nodes = make([]DAGNode, 0, len(dag.nodes))
		for _, n := range dag.nodes {
			cfg.Nodes = append(cfg.Nodes, *n)
		}
		result = append(result, cfg)
	}
	return result
}

// DeleteDAG removes a DAG.
func (o *DAGOrchestrator) DeleteDAG(dagID string) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	if _, exists := o.dags[dagID]; !exists {
		return fmt.Errorf("DAG %q not found", dagID)
	}
	delete(o.dags, dagID)
	return nil
}

func (o *DAGOrchestrator) validateDAG(cfg DAGConfig) error {
	nodeIDs := make(map[string]bool, len(cfg.Nodes))
	for _, n := range cfg.Nodes {
		if n.ID == "" {
			return fmt.Errorf("node ID is required")
		}
		if nodeIDs[n.ID] {
			return fmt.Errorf("duplicate node ID: %s", n.ID)
		}
		nodeIDs[n.ID] = true
	}

	// Validate edge references
	for _, n := range cfg.Nodes {
		for _, upID := range n.Upstream {
			if !nodeIDs[upID] {
				return fmt.Errorf("node %s references unknown upstream %s", n.ID, upID)
			}
		}
		for _, downID := range n.Downstream {
			if !nodeIDs[downID] {
				return fmt.Errorf("node %s references unknown downstream %s", n.ID, downID)
			}
		}
	}

	// Simple cycle detection via topological sort
	return o.detectCycle(cfg.Nodes)
}

func (o *DAGOrchestrator) detectCycle(nodes []DAGNode) error {
	inDegree := make(map[string]int)
	adj := make(map[string][]string)

	for _, n := range nodes {
		if _, ok := inDegree[n.ID]; !ok {
			inDegree[n.ID] = 0
		}
		for _, down := range n.Downstream {
			adj[n.ID] = append(adj[n.ID], down)
			inDegree[down]++
		}
	}

	queue := make([]string, 0)
	for id, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, id)
		}
	}

	visited := 0
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		visited++
		for _, next := range adj[id] {
			inDegree[next]--
			if inDegree[next] == 0 {
				queue = append(queue, next)
			}
		}
	}

	if visited != len(nodes) {
		return fmt.Errorf("DAG contains a cycle")
	}
	return nil
}
