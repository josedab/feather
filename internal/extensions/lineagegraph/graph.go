package lineagegraph

import (
	"fmt"
	"sync"
	"time"
)

// NodeType classifies lineage graph nodes.
type NodeType string

const (
	NodeSource   NodeType = "source"
	NodeFeature  NodeType = "feature"
	NodeModel    NodeType = "model"
	NodeConsumer NodeType = "consumer"
)

// FreshnessStatus indicates how current a node's data is.
type FreshnessStatus string

const (
	FreshnessCurrent FreshnessStatus = "current"
	FreshnessStale   FreshnessStatus = "stale"
	FreshnessUnknown FreshnessStatus = "unknown"
)

// Node represents a vertex in the lineage graph.
type Node struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Type        NodeType          `json:"type"`
	Description string            `json:"description,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	Freshness   FreshnessStatus   `json:"freshness"`
	LastUpdated time.Time         `json:"last_updated"`
	CreatedAt   time.Time         `json:"created_at"`
}

// Edge represents a directed dependency between two nodes.
type Edge struct {
	From      string    `json:"from"`
	To        string    `json:"to"`
	Label     string    `json:"label,omitempty"`
	Latency   float64   `json:"latency_ms,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// GraphView represents the entire lineage DAG for rendering.
type GraphView struct {
	Nodes      []Node    `json:"nodes"`
	Edges      []Edge    `json:"edges"`
	TotalNodes int       `json:"total_nodes"`
	TotalEdges int       `json:"total_edges"`
	Timestamp  time.Time `json:"timestamp"`
}

// ImpactReport describes what is affected by a node change.
type ImpactReport struct {
	SourceNode  string   `json:"source_node"`
	Downstream  []string `json:"downstream"`
	TotalImpact int      `json:"total_impact"`
}

// GraphConfig configures the lineage graph.
type GraphConfig struct {
	MaxNodes       int           `json:"max_nodes"`
	MaxEdges       int           `json:"max_edges"`
	StaleThreshold time.Duration `json:"stale_threshold"`
}

// DefaultGraphConfig returns sensible defaults.
func DefaultGraphConfig() GraphConfig {
	return GraphConfig{
		MaxNodes:       100000,
		MaxEdges:       500000,
		StaleThreshold: 1 * time.Hour,
	}
}

// Graph manages the feature lineage DAG.
type Graph struct {
	mu      sync.RWMutex
	config  GraphConfig
	nodes   map[string]*Node
	edges   map[string][]Edge // from -> edges
	inEdges map[string][]Edge // to -> edges (reverse index)
}

// NewGraph creates a new lineage graph.
func NewGraph(config GraphConfig) *Graph {
	if config.MaxNodes == 0 {
		config = DefaultGraphConfig()
	}
	return &Graph{
		config:  config,
		nodes:   make(map[string]*Node),
		edges:   make(map[string][]Edge),
		inEdges: make(map[string][]Edge),
	}
}

// AddNode adds a node to the graph.
func (g *Graph) AddNode(node Node) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if _, exists := g.nodes[node.ID]; exists {
		return ErrNodeExists
	}
	if len(g.nodes) >= g.config.MaxNodes {
		return fmt.Errorf("max nodes reached (%d)", g.config.MaxNodes)
	}

	now := time.Now()
	node.CreatedAt = now
	node.LastUpdated = now
	if node.Freshness == "" {
		node.Freshness = FreshnessUnknown
	}
	g.nodes[node.ID] = &node
	return nil
}

// UpdateNode updates a node's metadata and freshness.
func (g *Graph) UpdateNode(id string, freshness FreshnessStatus, metadata map[string]string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	node, exists := g.nodes[id]
	if !exists {
		return ErrNodeNotFound
	}

	node.Freshness = freshness
	node.LastUpdated = time.Now()
	if metadata != nil {
		if node.Metadata == nil {
			node.Metadata = make(map[string]string)
		}
		for k, v := range metadata {
			node.Metadata[k] = v
		}
	}
	return nil
}

// RemoveNode removes a node and all its edges.
func (g *Graph) RemoveNode(id string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if _, exists := g.nodes[id]; !exists {
		return ErrNodeNotFound
	}

	delete(g.nodes, id)
	delete(g.edges, id)
	delete(g.inEdges, id)

	// Clean edges referencing this node
	for from, edges := range g.edges {
		var filtered []Edge
		for _, e := range edges {
			if e.To != id {
				filtered = append(filtered, e)
			}
		}
		g.edges[from] = filtered
	}
	for to, edges := range g.inEdges {
		var filtered []Edge
		for _, e := range edges {
			if e.From != id {
				filtered = append(filtered, e)
			}
		}
		g.inEdges[to] = filtered
	}
	return nil
}

// AddEdge adds a directed edge from one node to another.
func (g *Graph) AddEdge(from, to, label string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if _, exists := g.nodes[from]; !exists {
		return fmt.Errorf("%w: %s", ErrNodeNotFound, from)
	}
	if _, exists := g.nodes[to]; !exists {
		return fmt.Errorf("%w: %s", ErrNodeNotFound, to)
	}

	// Check for duplicate
	for _, e := range g.edges[from] {
		if e.To == to {
			return ErrEdgeExists
		}
	}

	// Cycle detection via DFS
	if g.wouldCreateCycle(from, to) {
		return ErrCyclicDependency
	}

	edge := Edge{
		From:      from,
		To:        to,
		Label:     label,
		CreatedAt: time.Now(),
	}
	g.edges[from] = append(g.edges[from], edge)
	g.inEdges[to] = append(g.inEdges[to], edge)
	return nil
}

// GetNode returns a node by ID.
func (g *Graph) GetNode(id string) (*Node, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	node, exists := g.nodes[id]
	if !exists {
		return nil, ErrNodeNotFound
	}
	result := *node
	return &result, nil
}

// GetView returns the full graph for rendering.
func (g *Graph) GetView() GraphView {
	g.mu.RLock()
	defer g.mu.RUnlock()

	view := GraphView{
		Nodes:     make([]Node, 0, len(g.nodes)),
		Edges:     make([]Edge, 0),
		Timestamp: time.Now(),
	}

	for _, n := range g.nodes {
		view.Nodes = append(view.Nodes, *n)
	}
	for _, edges := range g.edges {
		view.Edges = append(view.Edges, edges...)
	}
	view.TotalNodes = len(view.Nodes)
	view.TotalEdges = len(view.Edges)
	return view
}

// GetUpstream returns all nodes that feed into the given node.
func (g *Graph) GetUpstream(id string) ([]Node, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if _, exists := g.nodes[id]; !exists {
		return nil, ErrNodeNotFound
	}

	visited := make(map[string]bool)
	var result []Node
	g.collectUpstream(id, visited, &result)
	return result, nil
}

// GetDownstream returns all nodes that depend on the given node.
func (g *Graph) GetDownstream(id string) ([]Node, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if _, exists := g.nodes[id]; !exists {
		return nil, ErrNodeNotFound
	}

	visited := make(map[string]bool)
	var result []Node
	g.collectDownstream(id, visited, &result)
	return result, nil
}

// GetImpact returns an impact report for a node change.
func (g *Graph) GetImpact(id string) (*ImpactReport, error) {
	downstream, err := g.GetDownstream(id)
	if err != nil {
		return nil, err
	}

	ids := make([]string, len(downstream))
	for i, n := range downstream {
		ids[i] = n.ID
	}

	return &ImpactReport{
		SourceNode:  id,
		Downstream:  ids,
		TotalImpact: len(ids),
	}, nil
}

// Stats returns graph statistics.
func (g *Graph) Stats() GraphStats {
	g.mu.RLock()
	defer g.mu.RUnlock()

	var stats GraphStats
	stats.TotalNodes = len(g.nodes)
	for _, edges := range g.edges {
		stats.TotalEdges += len(edges)
	}
	for _, n := range g.nodes {
		switch n.Type {
		case NodeSource:
			stats.Sources++
		case NodeFeature:
			stats.Features++
		case NodeModel:
			stats.Models++
		case NodeConsumer:
			stats.Consumers++
		}
		switch n.Freshness {
		case FreshnessCurrent:
			stats.Current++
		case FreshnessStale:
			stats.Stale++
		}
	}
	return stats
}

// GraphStats provides aggregate statistics.
type GraphStats struct {
	TotalNodes int `json:"total_nodes"`
	TotalEdges int `json:"total_edges"`
	Sources    int `json:"sources"`
	Features   int `json:"features"`
	Models     int `json:"models"`
	Consumers  int `json:"consumers"`
	Current    int `json:"current"`
	Stale      int `json:"stale"`
}

func (g *Graph) wouldCreateCycle(from, to string) bool {
	// Check if there's already a path from 'to' to 'from'
	visited := make(map[string]bool)
	return g.hasPath(to, from, visited)
}

func (g *Graph) hasPath(src, dst string, visited map[string]bool) bool {
	if src == dst {
		return true
	}
	visited[src] = true
	for _, e := range g.edges[src] {
		if !visited[e.To] {
			if g.hasPath(e.To, dst, visited) {
				return true
			}
		}
	}
	return false
}

func (g *Graph) collectUpstream(id string, visited map[string]bool, result *[]Node) {
	for _, e := range g.inEdges[id] {
		if !visited[e.From] {
			visited[e.From] = true
			if n, exists := g.nodes[e.From]; exists {
				*result = append(*result, *n)
			}
			g.collectUpstream(e.From, visited, result)
		}
	}
}

func (g *Graph) collectDownstream(id string, visited map[string]bool, result *[]Node) {
	for _, e := range g.edges[id] {
		if !visited[e.To] {
			visited[e.To] = true
			if n, exists := g.nodes[e.To]; exists {
				*result = append(*result, *n)
			}
			g.collectDownstream(e.To, visited, result)
		}
	}
}
