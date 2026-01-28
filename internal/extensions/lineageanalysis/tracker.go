package lineageanalysis

import (
	"fmt"
	"sync"
	"time"
)

// NodeType classifies lineage nodes.
type NodeType string

const (
	NodeSource         NodeType = "source"         // Raw data source (DB, API, file)
	NodeTransformation NodeType = "transformation" // Processing step
	NodeFeature        NodeType = "feature"        // Computed feature
	NodeConsumer       NodeType = "consumer"       // Downstream consumer (model, API)
)

// LineageNode represents a node in the lineage graph.
type LineageNode struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Type        NodeType          `json:"type"`
	Description string            `json:"description,omitempty"`
	Owner       string            `json:"owner,omitempty"`
	Tags        map[string]string `json:"tags,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// LineageEdge represents a dependency between two nodes.
type LineageEdge struct {
	FromID      string    `json:"from_id"`
	ToID        string    `json:"to_id"`
	Type        string    `json:"type,omitempty"` // "derives_from", "feeds_into", "transforms"
	Description string    `json:"description,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

// ImpactReport shows the downstream effects of changing an upstream node.
type ImpactReport struct {
	SourceNode    string         `json:"source_node"`
	AffectedNodes []AffectedNode `json:"affected_nodes"`
	TotalAffected int            `json:"total_affected"`
	MaxDepth      int            `json:"max_depth"`
	GeneratedAt   time.Time      `json:"generated_at"`
}

// AffectedNode describes a node affected by an upstream change.
type AffectedNode struct {
	ID    string   `json:"id"`
	Name  string   `json:"name"`
	Type  NodeType `json:"type"`
	Depth int      `json:"depth"` // distance from source
	Path  []string `json:"path"`  // path from source to this node
}

// LineagePath represents a full lineage path from source to consumer.
type LineagePath struct {
	Nodes []string `json:"nodes"`
	Depth int      `json:"depth"`
}

// TrackerConfig configures the lineage tracker.
type TrackerConfig struct {
	MaxNodes int `json:"max_nodes"`
	MaxEdges int `json:"max_edges"`
	MaxDepth int `json:"max_depth"`
}

// DefaultTrackerConfig returns sensible defaults.
func DefaultTrackerConfig() TrackerConfig {
	return TrackerConfig{
		MaxNodes: 100000,
		MaxEdges: 500000,
		MaxDepth: 50,
	}
}

// Tracker manages the lineage graph and dependency tracking.
type Tracker struct {
	mu     sync.RWMutex
	config TrackerConfig
	nodes  map[string]*LineageNode
	edges  map[string][]string // fromID -> []toID
	redges map[string][]string // toID -> []fromID (reverse edges)
}

// NewTracker creates a new lineage tracker.
func NewTracker(config TrackerConfig) *Tracker {
	if config.MaxNodes == 0 {
		config = DefaultTrackerConfig()
	}
	return &Tracker{
		config: config,
		nodes:  make(map[string]*LineageNode),
		edges:  make(map[string][]string),
		redges: make(map[string][]string),
	}
}

// AddNode adds a node to the lineage graph.
func (t *Tracker) AddNode(node LineageNode) error {
	if node.ID == "" || node.Name == "" {
		return fmt.Errorf("%w: id and name are required", ErrInvalidNode)
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	if _, exists := t.nodes[node.ID]; exists {
		return ErrNodeExists
	}
	if len(t.nodes) >= t.config.MaxNodes {
		return fmt.Errorf("max nodes reached (%d)", t.config.MaxNodes)
	}

	now := time.Now()
	node.CreatedAt = now
	node.UpdatedAt = now
	t.nodes[node.ID] = &node
	return nil
}

// GetNode returns a node by ID.
func (t *Tracker) GetNode(id string) (*LineageNode, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	node, exists := t.nodes[id]
	if !exists {
		return nil, ErrNodeNotFound
	}
	return node, nil
}

// ListNodes returns all nodes, optionally filtered by type.
func (t *Tracker) ListNodes(nodeType string) []LineageNode {
	t.mu.RLock()
	defer t.mu.RUnlock()

	result := make([]LineageNode, 0, len(t.nodes))
	for _, n := range t.nodes {
		if nodeType == "" || string(n.Type) == nodeType {
			result = append(result, *n)
		}
	}
	return result
}

// AddEdge adds a dependency edge between two nodes.
func (t *Tracker) AddEdge(edge LineageEdge) error {
	if edge.FromID == "" || edge.ToID == "" {
		return fmt.Errorf("from_id and to_id are required")
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	if _, exists := t.nodes[edge.FromID]; !exists {
		return fmt.Errorf("%w: from node %s", ErrNodeNotFound, edge.FromID)
	}
	if _, exists := t.nodes[edge.ToID]; !exists {
		return fmt.Errorf("%w: to node %s", ErrNodeNotFound, edge.ToID)
	}

	// Check for duplicate edge
	for _, toID := range t.edges[edge.FromID] {
		if toID == edge.ToID {
			return ErrEdgeExists
		}
	}

	// Check for cycle
	if t.wouldCreateCycle(edge.FromID, edge.ToID) {
		return ErrCyclicLineage
	}

	edge.CreatedAt = time.Now()
	t.edges[edge.FromID] = append(t.edges[edge.FromID], edge.ToID)
	t.redges[edge.ToID] = append(t.redges[edge.ToID], edge.FromID)
	return nil
}

// wouldCreateCycle checks if adding from->to would create a cycle.
func (t *Tracker) wouldCreateCycle(from, to string) bool {
	visited := make(map[string]bool)
	var dfs func(current string) bool
	dfs = func(current string) bool {
		if current == from {
			return true
		}
		if visited[current] {
			return false
		}
		visited[current] = true
		for _, next := range t.edges[current] {
			if dfs(next) {
				return true
			}
		}
		return false
	}
	return dfs(to)
}

// GetUpstream returns all upstream dependencies of a node.
func (t *Tracker) GetUpstream(id string) []LineageNode {
	t.mu.RLock()
	defer t.mu.RUnlock()

	visited := make(map[string]bool)
	var result []LineageNode
	t.collectUpstream(id, visited, &result)
	return result
}

func (t *Tracker) collectUpstream(id string, visited map[string]bool, result *[]LineageNode) {
	for _, fromID := range t.redges[id] {
		if !visited[fromID] {
			visited[fromID] = true
			if node, ok := t.nodes[fromID]; ok {
				*result = append(*result, *node)
			}
			t.collectUpstream(fromID, visited, result)
		}
	}
}

// GetDownstream returns all downstream consumers of a node.
func (t *Tracker) GetDownstream(id string) []LineageNode {
	t.mu.RLock()
	defer t.mu.RUnlock()

	visited := make(map[string]bool)
	var result []LineageNode
	t.collectDownstream(id, visited, &result)
	return result
}

func (t *Tracker) collectDownstream(id string, visited map[string]bool, result *[]LineageNode) {
	for _, toID := range t.edges[id] {
		if !visited[toID] {
			visited[toID] = true
			if node, ok := t.nodes[toID]; ok {
				*result = append(*result, *node)
			}
			t.collectDownstream(toID, visited, result)
		}
	}
}

// AnalyzeImpact generates an impact report for changing a source node.
func (t *Tracker) AnalyzeImpact(sourceID string) (*ImpactReport, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if _, exists := t.nodes[sourceID]; !exists {
		return nil, ErrNodeNotFound
	}

	report := &ImpactReport{
		SourceNode:  sourceID,
		GeneratedAt: time.Now(),
	}

	visited := make(map[string]bool)
	t.bfsImpact(sourceID, visited, report, []string{sourceID}, 0)

	report.TotalAffected = len(report.AffectedNodes)
	return report, nil
}

func (t *Tracker) bfsImpact(id string, visited map[string]bool, report *ImpactReport, path []string, depth int) {
	if depth > t.config.MaxDepth {
		return
	}
	if depth > report.MaxDepth {
		report.MaxDepth = depth
	}

	for _, toID := range t.edges[id] {
		if !visited[toID] {
			visited[toID] = true
			node := t.nodes[toID]
			newPath := make([]string, len(path)+1)
			copy(newPath, path)
			newPath[len(path)] = toID

			report.AffectedNodes = append(report.AffectedNodes, AffectedNode{
				ID:    toID,
				Name:  node.Name,
				Type:  node.Type,
				Depth: depth + 1,
				Path:  newPath,
			})

			t.bfsImpact(toID, visited, report, newPath, depth+1)
		}
	}
}

// Stats returns tracker statistics.
func (t *Tracker) Stats() map[string]interface{} {
	t.mu.RLock()
	defer t.mu.RUnlock()

	edgeCount := 0
	for _, targets := range t.edges {
		edgeCount += len(targets)
	}

	byType := make(map[string]int)
	for _, n := range t.nodes {
		byType[string(n.Type)]++
	}

	return map[string]interface{}{
		"total_nodes": len(t.nodes),
		"total_edges": edgeCount,
		"by_type":     byType,
	}
}
