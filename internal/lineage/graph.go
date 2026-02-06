package lineage

import (
	"encoding/json"
	"sync"
)

// DependencyGraph represents the feature dependency graph.
type DependencyGraph struct {
	mu    sync.RWMutex
	nodes map[string]*GraphNode
	edges []*GraphEdge
}

// GraphNode represents a node in the dependency graph.
type GraphNode struct {
	ID       string            `json:"id"`
	Type     NodeType          `json:"type"`
	Label    string            `json:"label,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// NodeType indicates the type of graph node.
type NodeType string

// NodeType constants for graph nodes.
const (
	NodeTypeFeature  NodeType = "feature"
	NodeTypeSource   NodeType = "source"
	NodeTypeConsumer NodeType = "consumer"
)

// GraphEdge represents an edge in the dependency graph.
type GraphEdge struct {
	From  string   `json:"from"`
	To    string   `json:"to"`
	Type  EdgeType `json:"type"`
	Label string   `json:"label,omitempty"`
}

// EdgeType indicates the type of relationship.
type EdgeType string

// EdgeType constants for graph relationships.
const (
	EdgeTypeDependsOn  EdgeType = "depends_on"
	EdgeTypeSourceOf   EdgeType = "source_of"
	EdgeTypeConsumedBy EdgeType = "consumed_by"
)

// NewDependencyGraph creates a new dependency graph.
func NewDependencyGraph() *DependencyGraph {
	return &DependencyGraph{
		nodes: make(map[string]*GraphNode),
		edges: make([]*GraphEdge, 0),
	}
}

// AddNode adds a node to the graph.
func (g *DependencyGraph) AddNode(id string, nodeType NodeType) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if _, exists := g.nodes[id]; !exists {
		g.nodes[id] = &GraphNode{
			ID:       id,
			Type:     nodeType,
			Metadata: make(map[string]string),
		}
	}
}

// SetNodeLabel sets the label for a node.
func (g *DependencyGraph) SetNodeLabel(id, label string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if node, ok := g.nodes[id]; ok {
		node.Label = label
	}
}

// AddEdge adds an edge to the graph.
func (g *DependencyGraph) AddEdge(from, to string, edgeType EdgeType) {
	g.mu.Lock()
	defer g.mu.Unlock()

	// Check for duplicate
	for _, e := range g.edges {
		if e.From == from && e.To == to && e.Type == edgeType {
			return
		}
	}

	g.edges = append(g.edges, &GraphEdge{
		From: from,
		To:   to,
		Type: edgeType,
	})
}

// GetNode returns a node by ID.
func (g *DependencyGraph) GetNode(id string) *GraphNode {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.nodes[id]
}

// GetNodes returns all nodes.
func (g *DependencyGraph) GetNodes() []*GraphNode {
	g.mu.RLock()
	defer g.mu.RUnlock()

	nodes := make([]*GraphNode, 0, len(g.nodes))
	for _, n := range g.nodes {
		nodes = append(nodes, n)
	}
	return nodes
}

// GetEdges returns all edges.
func (g *DependencyGraph) GetEdges() []*GraphEdge {
	g.mu.RLock()
	defer g.mu.RUnlock()

	edges := make([]*GraphEdge, len(g.edges))
	copy(edges, g.edges)
	return edges
}

// GetUpstream returns all nodes that feed into the given node.
func (g *DependencyGraph) GetUpstream(nodeID string) []*GraphNode {
	g.mu.RLock()
	defer g.mu.RUnlock()

	result := make([]*GraphNode, 0)
	for _, e := range g.edges {
		if e.To == nodeID {
			if node, ok := g.nodes[e.From]; ok {
				result = append(result, node)
			}
		}
	}
	return result
}

// GetDownstream returns all nodes that the given node feeds into.
func (g *DependencyGraph) GetDownstream(nodeID string) []*GraphNode {
	g.mu.RLock()
	defer g.mu.RUnlock()

	result := make([]*GraphNode, 0)
	for _, e := range g.edges {
		if e.From == nodeID {
			if node, ok := g.nodes[e.To]; ok {
				result = append(result, node)
			}
		}
	}
	return result
}

// GetFullUpstream returns all transitive upstream nodes (BFS).
func (g *DependencyGraph) GetFullUpstream(nodeID string) []*GraphNode {
	g.mu.RLock()
	defer g.mu.RUnlock()

	visited := make(map[string]bool)
	result := make([]*GraphNode, 0)
	queue := []string{nodeID}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		if visited[current] {
			continue
		}
		visited[current] = true

		for _, e := range g.edges {
			if e.To == current && !visited[e.From] {
				if node, ok := g.nodes[e.From]; ok {
					result = append(result, node)
					queue = append(queue, e.From)
				}
			}
		}
	}

	return result
}

// GetFullDownstream returns all transitive downstream nodes (BFS).
func (g *DependencyGraph) GetFullDownstream(nodeID string) []*GraphNode {
	g.mu.RLock()
	defer g.mu.RUnlock()

	visited := make(map[string]bool)
	result := make([]*GraphNode, 0)
	queue := []string{nodeID}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		if visited[current] {
			continue
		}
		visited[current] = true

		for _, e := range g.edges {
			if e.From == current && !visited[e.To] {
				if node, ok := g.nodes[e.To]; ok {
					result = append(result, node)
					queue = append(queue, e.To)
				}
			}
		}
	}

	return result
}

// DetectCycles detects if there are any cycles in the graph.
func (g *DependencyGraph) DetectCycles() [][]string {
	g.mu.RLock()
	defer g.mu.RUnlock()

	cycles := make([][]string, 0)
	visited := make(map[string]bool)
	recStack := make(map[string]bool)

	var dfs func(node string, path []string) bool
	dfs = func(node string, path []string) bool {
		visited[node] = true
		recStack[node] = true
		path = append(path, node)

		for _, e := range g.edges {
			if e.From != node {
				continue
			}

			if !visited[e.To] {
				if dfs(e.To, path) {
					return true
				}
			} else if recStack[e.To] {
				// Found cycle - extract it
				cycleStart := -1
				for i, n := range path {
					if n == e.To {
						cycleStart = i
						break
					}
				}
				if cycleStart >= 0 {
					cycle := make([]string, len(path)-cycleStart+1)
					copy(cycle, path[cycleStart:])
					cycle[len(cycle)-1] = e.To // Close the cycle
					cycles = append(cycles, cycle)
				}
			}
		}

		recStack[node] = false
		return false
	}

	for id := range g.nodes {
		if !visited[id] {
			dfs(id, []string{})
		}
	}

	return cycles
}

// TopologicalSort returns nodes in topological order.
func (g *DependencyGraph) TopologicalSort() ([]string, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	// Calculate in-degrees
	inDegree := make(map[string]int)
	for id := range g.nodes {
		inDegree[id] = 0
	}
	for _, e := range g.edges {
		inDegree[e.To]++
	}

	// Start with nodes that have no incoming edges
	queue := make([]string, 0)
	for id, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, id)
		}
	}

	result := make([]string, 0, len(g.nodes))
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		result = append(result, node)

		for _, e := range g.edges {
			if e.From == node {
				inDegree[e.To]--
				if inDegree[e.To] == 0 {
					queue = append(queue, e.To)
				}
			}
		}
	}

	if len(result) != len(g.nodes) {
		return nil, ErrCycleDetected
	}

	return result, nil
}

// ExportDOT exports the graph in DOT format for visualization.
func (g *DependencyGraph) ExportDOT() string {
	g.mu.RLock()
	defer g.mu.RUnlock()

	dot := "digraph FeatureLineage {\n"
	dot += "  rankdir=LR;\n"
	dot += "  node [shape=box];\n\n"

	// Node styles by type
	for _, node := range g.nodes {
		label := node.ID
		if node.Label != "" {
			label = node.Label
		}

		style := ""
		switch node.Type {
		case NodeTypeSource:
			style = "shape=cylinder,color=blue"
		case NodeTypeFeature:
			style = "shape=box,color=green"
		case NodeTypeConsumer:
			style = "shape=ellipse,color=orange"
		}

		dot += "  \"" + node.ID + "\" [label=\"" + label + "\"," + style + "];\n"
	}

	dot += "\n"

	// Edges
	for _, edge := range g.edges {
		style := ""
		switch edge.Type {
		case EdgeTypeSourceOf:
			style = "style=dashed,color=blue"
		case EdgeTypeDependsOn:
			style = "color=green"
		case EdgeTypeConsumedBy:
			style = "style=dotted,color=orange"
		}

		dot += "  \"" + edge.From + "\" -> \"" + edge.To + "\" [" + style + "];\n"
	}

	dot += "}\n"
	return dot
}

// ExportJSON exports the graph as JSON.
func (g *DependencyGraph) ExportJSON() ([]byte, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	data := struct {
		Nodes []*GraphNode `json:"nodes"`
		Edges []*GraphEdge `json:"edges"`
	}{
		Nodes: make([]*GraphNode, 0, len(g.nodes)),
		Edges: g.edges,
	}

	for _, n := range g.nodes {
		data.Nodes = append(data.Nodes, n)
	}

	return json.Marshal(data)
}

// ExportMermaid exports the graph in Mermaid format.
func (g *DependencyGraph) ExportMermaid() string {
	g.mu.RLock()
	defer g.mu.RUnlock()

	mermaid := "graph LR\n"

	// Define node styles
	for _, node := range g.nodes {
		label := node.ID
		if node.Label != "" {
			label = node.Label
		}

		switch node.Type {
		case NodeTypeSource:
			mermaid += "  " + node.ID + "[(" + label + ")]\n"
		case NodeTypeFeature:
			mermaid += "  " + node.ID + "[" + label + "]\n"
		case NodeTypeConsumer:
			mermaid += "  " + node.ID + "((" + label + "))\n"
		}
	}

	mermaid += "\n"

	// Edges
	for _, edge := range g.edges {
		arrow := "-->"
		switch edge.Type {
		case EdgeTypeSourceOf:
			arrow = "-.->|source|"
		case EdgeTypeDependsOn:
			arrow = "-->|depends|"
		case EdgeTypeConsumedBy:
			arrow = "==>|consumes|"
		}

		mermaid += "  " + edge.From + " " + arrow + " " + edge.To + "\n"
	}

	return mermaid
}
