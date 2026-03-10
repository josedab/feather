package lineage

import (
	"fmt"
	"sort"
	"sync"
	"time"
)

// UnifiedConfig configures the unified lineage system.
type UnifiedConfig struct {
	MaxNodes        int           `json:"max_nodes" yaml:"max_nodes"`
	MaxEdges        int           `json:"max_edges" yaml:"max_edges"`
	StaleThreshold  time.Duration `json:"stale_threshold" yaml:"stale_threshold"`
	EnableQuality   bool          `json:"enable_quality" yaml:"enable_quality"`
	EnableFreshness bool          `json:"enable_freshness" yaml:"enable_freshness"`
}

// DefaultUnifiedConfig returns sensible defaults.
func DefaultUnifiedConfig() UnifiedConfig {
	return UnifiedConfig{
		MaxNodes:        10000,
		MaxEdges:        50000,
		StaleThreshold:  24 * time.Hour,
		EnableQuality:   true,
		EnableFreshness: true,
	}
}

// UnifiedNodeKind categorizes nodes in the unified lineage graph.
type UnifiedNodeKind string

const (
	UnifiedNodeSource    UnifiedNodeKind = "source"
	UnifiedNodeFeature   UnifiedNodeKind = "feature"
	UnifiedNodeModel     UnifiedNodeKind = "model"
	UnifiedNodeConsumer  UnifiedNodeKind = "consumer"
	UnifiedNodeTransform UnifiedNodeKind = "transform"
)

// LineageNode is a unified node in the lineage DAG.
type LineageNode struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Kind      UnifiedNodeKind   `json:"kind"`
	Owner     string            `json:"owner,omitempty"`
	Tags      map[string]string `json:"tags,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
	// Quality metrics
	FreshnessMs  int64   `json:"freshness_ms,omitempty"`
	QualityScore float64 `json:"quality_score,omitempty"`
	DriftScore   float64 `json:"drift_score,omitempty"`
	FreshnessSLA int64   `json:"freshness_sla_ms,omitempty"`
	SLAViolation bool    `json:"sla_violation,omitempty"`
}

// LineageEdge connects two nodes in the lineage graph.
type LineageEdge struct {
	From      string    `json:"from"`
	To        string    `json:"to"`
	Label     string    `json:"label,omitempty"`
	Weight    float64   `json:"weight,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// LineageGraph represents the full lineage DAG.
type LineageGraph struct {
	Nodes []LineageNode `json:"nodes"`
	Edges []LineageEdge `json:"edges"`
}

// ImpactResult contains the blast radius analysis for a node.
type ImpactResult struct {
	SourceNode    string         `json:"source_node"`
	AffectedNodes []AffectedNode `json:"affected_nodes"`
	BlastRadius   int            `json:"blast_radius"`
	MaxDepth      int            `json:"max_depth"`
	CriticalPaths [][]string     `json:"critical_paths,omitempty"`
}

// AffectedNode is a downstream node affected by a change.
type AffectedNode struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Kind  string `json:"kind"`
	Depth int    `json:"depth"`
}

// UnifiedLineage provides a merged API for all lineage operations.
type UnifiedLineage struct {
	mu     sync.RWMutex
	config UnifiedConfig
	nodes  map[string]*LineageNode
	edges  map[string]map[string]*LineageEdge // from -> to -> edge
}

// NewUnifiedLineage creates a new unified lineage system.
func NewUnifiedLineage(config UnifiedConfig) *UnifiedLineage {
	return &UnifiedLineage{
		config: config,
		nodes:  make(map[string]*LineageNode),
		edges:  make(map[string]map[string]*LineageEdge),
	}
}

// AddNode adds or updates a node in the lineage graph.
func (u *UnifiedLineage) AddNode(node LineageNode) error {
	u.mu.Lock()
	defer u.mu.Unlock()

	if node.ID == "" {
		return fmt.Errorf("node ID is required")
	}

	if _, exists := u.nodes[node.ID]; !exists {
		if len(u.nodes) >= u.config.MaxNodes {
			return fmt.Errorf("max nodes (%d) reached", u.config.MaxNodes)
		}
		node.CreatedAt = time.Now()
	}
	node.UpdatedAt = time.Now()
	if node.Tags == nil {
		node.Tags = make(map[string]string)
	}
	if node.Metadata == nil {
		node.Metadata = make(map[string]string)
	}
	u.nodes[node.ID] = &node
	return nil
}

// RemoveNode removes a node and its edges.
func (u *UnifiedLineage) RemoveNode(id string) error {
	u.mu.Lock()
	defer u.mu.Unlock()
	if _, exists := u.nodes[id]; !exists {
		return fmt.Errorf("node %s not found", id)
	}
	delete(u.nodes, id)
	delete(u.edges, id)
	for from := range u.edges {
		delete(u.edges[from], id)
	}
	return nil
}

// GetNode returns a node by ID.
func (u *UnifiedLineage) GetNode(id string) (*LineageNode, error) {
	u.mu.RLock()
	defer u.mu.RUnlock()
	node, exists := u.nodes[id]
	if !exists {
		return nil, fmt.Errorf("node %s not found", id)
	}
	copy := *node
	return &copy, nil
}

// AddEdge adds a directed edge between two nodes.
func (u *UnifiedLineage) AddEdge(edge LineageEdge) error {
	u.mu.Lock()
	defer u.mu.Unlock()

	if edge.From == "" || edge.To == "" {
		return fmt.Errorf("edge from and to are required")
	}
	if _, exists := u.nodes[edge.From]; !exists {
		return fmt.Errorf("source node %s not found", edge.From)
	}
	if _, exists := u.nodes[edge.To]; !exists {
		return fmt.Errorf("target node %s not found", edge.To)
	}

	// Check for cycles
	if u.wouldCreateCycle(edge.From, edge.To) {
		return fmt.Errorf("edge %s -> %s would create a cycle", edge.From, edge.To)
	}

	edge.CreatedAt = time.Now()
	if u.edges[edge.From] == nil {
		u.edges[edge.From] = make(map[string]*LineageEdge)
	}
	u.edges[edge.From][edge.To] = &edge
	return nil
}

func (u *UnifiedLineage) wouldCreateCycle(from, to string) bool {
	visited := make(map[string]bool)
	return u.dfs(to, from, visited)
}

func (u *UnifiedLineage) dfs(current, target string, visited map[string]bool) bool {
	if current == target {
		return true
	}
	if visited[current] {
		return false
	}
	visited[current] = true
	for next := range u.edges[current] {
		if u.dfs(next, target, visited) {
			return true
		}
	}
	return false
}

// GetUpstream returns all upstream nodes for a given node.
func (u *UnifiedLineage) GetUpstream(id string, maxDepth int) []LineageNode {
	u.mu.RLock()
	defer u.mu.RUnlock()
	visited := make(map[string]bool)
	var result []LineageNode
	u.collectUpstream(id, 0, maxDepth, visited, &result)
	return result
}

func (u *UnifiedLineage) collectUpstream(id string, depth, maxDepth int, visited map[string]bool, result *[]LineageNode) {
	if depth >= maxDepth || visited[id] {
		return
	}
	visited[id] = true
	for from, targets := range u.edges {
		if _, exists := targets[id]; exists {
			if node, ok := u.nodes[from]; ok {
				*result = append(*result, *node)
				u.collectUpstream(from, depth+1, maxDepth, visited, result)
			}
		}
	}
}

// GetDownstream returns all downstream nodes for a given node.
func (u *UnifiedLineage) GetDownstream(id string, maxDepth int) []LineageNode {
	u.mu.RLock()
	defer u.mu.RUnlock()
	visited := make(map[string]bool)
	var result []LineageNode
	u.collectDownstream(id, 0, maxDepth, visited, &result)
	return result
}

func (u *UnifiedLineage) collectDownstream(id string, depth, maxDepth int, visited map[string]bool, result *[]LineageNode) {
	if depth >= maxDepth || visited[id] {
		return
	}
	visited[id] = true
	for to := range u.edges[id] {
		if node, ok := u.nodes[to]; ok {
			*result = append(*result, *node)
			u.collectDownstream(to, depth+1, maxDepth, visited, result)
		}
	}
}

// AnalyzeImpact computes the blast radius for a node change.
func (u *UnifiedLineage) AnalyzeImpact(nodeID string) (*ImpactResult, error) {
	u.mu.RLock()
	defer u.mu.RUnlock()

	if _, exists := u.nodes[nodeID]; !exists {
		return nil, fmt.Errorf("node %s not found", nodeID)
	}

	result := &ImpactResult{
		SourceNode: nodeID,
	}

	visited := make(map[string]bool)
	u.collectImpact(nodeID, 0, visited, result)

	result.BlastRadius = len(result.AffectedNodes)
	return result, nil
}

func (u *UnifiedLineage) collectImpact(id string, depth int, visited map[string]bool, result *ImpactResult) {
	if visited[id] {
		return
	}
	visited[id] = true

	for to := range u.edges[id] {
		node, exists := u.nodes[to]
		if !exists {
			continue
		}
		affected := AffectedNode{
			ID:    node.ID,
			Name:  node.Name,
			Kind:  string(node.Kind),
			Depth: depth + 1,
		}
		result.AffectedNodes = append(result.AffectedNodes, affected)
		if depth+1 > result.MaxDepth {
			result.MaxDepth = depth + 1
		}
		u.collectImpact(to, depth+1, visited, result)
	}
}

// GetGraph returns the full lineage graph.
func (u *UnifiedLineage) GetGraph() *LineageGraph {
	u.mu.RLock()
	defer u.mu.RUnlock()

	graph := &LineageGraph{
		Nodes: make([]LineageNode, 0, len(u.nodes)),
		Edges: make([]LineageEdge, 0),
	}

	for _, node := range u.nodes {
		graph.Nodes = append(graph.Nodes, *node)
	}
	sort.Slice(graph.Nodes, func(i, j int) bool {
		return graph.Nodes[i].ID < graph.Nodes[j].ID
	})

	for _, targets := range u.edges {
		for _, edge := range targets {
			graph.Edges = append(graph.Edges, *edge)
		}
	}

	return graph
}

// SetFreshness updates freshness metrics for a node.
func (u *UnifiedLineage) SetFreshness(nodeID string, freshnessMs int64, slaMs int64) error {
	u.mu.Lock()
	defer u.mu.Unlock()
	node, exists := u.nodes[nodeID]
	if !exists {
		return fmt.Errorf("node %s not found", nodeID)
	}
	node.FreshnessMs = freshnessMs
	node.FreshnessSLA = slaMs
	node.SLAViolation = slaMs > 0 && freshnessMs > slaMs
	node.UpdatedAt = time.Now()
	return nil
}

// SetQuality updates quality and drift scores for a node.
func (u *UnifiedLineage) SetQuality(nodeID string, qualityScore, driftScore float64) error {
	u.mu.Lock()
	defer u.mu.Unlock()
	node, exists := u.nodes[nodeID]
	if !exists {
		return fmt.Errorf("node %s not found", nodeID)
	}
	node.QualityScore = qualityScore
	node.DriftScore = driftScore
	node.UpdatedAt = time.Now()
	return nil
}

// GetSLAViolations returns all nodes with SLA violations.
func (u *UnifiedLineage) GetSLAViolations() []LineageNode {
	u.mu.RLock()
	defer u.mu.RUnlock()
	var result []LineageNode
	for _, node := range u.nodes {
		if node.SLAViolation {
			result = append(result, *node)
		}
	}
	return result
}

// GetDriftingNodes returns nodes with drift scores above the threshold.
func (u *UnifiedLineage) GetDriftingNodes(threshold float64) []LineageNode {
	u.mu.RLock()
	defer u.mu.RUnlock()
	var result []LineageNode
	for _, node := range u.nodes {
		if node.DriftScore > threshold {
			result = append(result, *node)
		}
	}
	return result
}

// GetLowQualityNodes returns nodes with quality scores below the threshold.
func (u *UnifiedLineage) GetLowQualityNodes(threshold float64) []LineageNode {
	u.mu.RLock()
	defer u.mu.RUnlock()
	var result []LineageNode
	for _, node := range u.nodes {
		if node.QualityScore > 0 && node.QualityScore < threshold {
			result = append(result, *node)
		}
	}
	return result
}

// ExportDOT returns the graph in Graphviz DOT format.
func (u *UnifiedLineage) ExportDOT() string {
	u.mu.RLock()
	defer u.mu.RUnlock()

	var b []byte
	b = append(b, "digraph lineage {\n"...)
	b = append(b, "  rankdir=LR;\n"...)
	b = append(b, "  node [shape=box];\n"...)

	for _, node := range u.nodes {
		color := "white"
		switch node.Kind {
		case UnifiedNodeSource:
			color = "lightblue"
		case UnifiedNodeFeature:
			color = "lightyellow"
		case UnifiedNodeModel:
			color = "lightgreen"
		case UnifiedNodeConsumer:
			color = "lightsalmon"
		}
		if node.SLAViolation {
			color = "red"
		}
		label := fmt.Sprintf("%s\\n(%s)", node.Name, node.Kind)
		b = append(b, fmt.Sprintf("  %q [label=%q, style=filled, fillcolor=%q];\n", node.ID, label, color)...)
	}

	for from, targets := range u.edges {
		for to, edge := range targets {
			label := edge.Label
			if label == "" {
				label = ""
			}
			b = append(b, fmt.Sprintf("  %q -> %q [label=%q];\n", from, to, label)...)
		}
	}

	b = append(b, "}\n"...)
	return string(b)
}

// ExportMermaid returns the graph in Mermaid format.
func (u *UnifiedLineage) ExportMermaid() string {
	u.mu.RLock()
	defer u.mu.RUnlock()

	var b []byte
	b = append(b, "graph LR\n"...)

	for _, node := range u.nodes {
		shape := "[%s]"
		switch node.Kind {
		case UnifiedNodeSource:
			shape = "([%s])"
		case UnifiedNodeModel:
			shape = "{{%s}}"
		case UnifiedNodeConsumer:
			shape = ">%s]"
		}
		b = append(b, fmt.Sprintf("  %s"+shape+"\n", node.ID, node.Name)...)
	}

	for from, targets := range u.edges {
		for to, edge := range targets {
			if edge.Label != "" {
				b = append(b, fmt.Sprintf("  %s -->|%s| %s\n", from, edge.Label, to)...)
			} else {
				b = append(b, fmt.Sprintf("  %s --> %s\n", from, to)...)
			}
		}
	}

	return string(b)
}

// UnifiedStats returns aggregate lineage statistics.
type UnifiedStats struct {
	TotalNodes    int            `json:"total_nodes"`
	TotalEdges    int            `json:"total_edges"`
	NodesByKind   map[string]int `json:"nodes_by_kind"`
	SLAViolations int            `json:"sla_violations"`
	AvgQuality    float64        `json:"avg_quality"`
}

// Stats returns aggregate statistics.
func (u *UnifiedLineage) Stats() UnifiedStats {
	u.mu.RLock()
	defer u.mu.RUnlock()

	stats := UnifiedStats{
		TotalNodes:  len(u.nodes),
		NodesByKind: make(map[string]int),
	}
	var totalQuality float64
	qualityCount := 0
	for _, node := range u.nodes {
		stats.NodesByKind[string(node.Kind)]++
		if node.SLAViolation {
			stats.SLAViolations++
		}
		if node.QualityScore > 0 {
			totalQuality += node.QualityScore
			qualityCount++
		}
	}
	for _, targets := range u.edges {
		stats.TotalEdges += len(targets)
	}
	if qualityCount > 0 {
		stats.AvgQuality = totalQuality / float64(qualityCount)
	}
	return stats
}
