package lineage

import (
	"fmt"
	"sort"
	"time"
)

// GraphStats provides summary statistics about the lineage graph.
type GraphStats struct {
	TotalNodes      int            `json:"total_nodes"`
	TotalEdges      int            `json:"total_edges"`
	NodesByType     map[string]int `json:"nodes_by_type"`
	EdgesByType     map[string]int `json:"edges_by_type"`
	MaxDepth        int            `json:"max_depth"`
	HasCycles       bool           `json:"has_cycles"`
	IsolatedNodes   int            `json:"isolated_nodes"`
	ConnectedGroups int            `json:"connected_groups"`
}

// BlastRadius represents the impact blast radius of changing a node.
type BlastRadius struct {
	NodeID            string       `json:"node_id"`
	DirectImpact      []*GraphNode `json:"direct_impact"`
	TransitiveImpact  []*GraphNode `json:"transitive_impact"`
	AffectedSources   []*GraphNode `json:"affected_sources"`
	AffectedConsumers []*GraphNode `json:"affected_consumers"`
	TotalAffected     int          `json:"total_affected"`
	RiskScore         float64      `json:"risk_score"`
	ComputedAt        time.Time    `json:"computed_at"`
}

// PathResult represents a path between two nodes.
type PathResult struct {
	From     string     `json:"from"`
	To       string     `json:"to"`
	Paths    [][]string `json:"paths"`
	Shortest int        `json:"shortest_length"`
}

// Stats computes summary statistics about the graph.
func (g *DependencyGraph) Stats() *GraphStats {
	g.mu.RLock()
	defer g.mu.RUnlock()

	stats := &GraphStats{
		TotalNodes:  len(g.nodes),
		TotalEdges:  len(g.edges),
		NodesByType: make(map[string]int),
		EdgesByType: make(map[string]int),
	}

	connected := make(map[string]bool)
	for _, e := range g.edges {
		stats.EdgesByType[string(e.Type)]++
		connected[e.From] = true
		connected[e.To] = true
	}

	for _, n := range g.nodes {
		stats.NodesByType[string(n.Type)]++
		if !connected[n.ID] {
			stats.IsolatedNodes++
		}
	}

	cycles := g.detectCyclesUnlocked()
	stats.HasCycles = len(cycles) > 0

	stats.MaxDepth = g.computeMaxDepthUnlocked()
	stats.ConnectedGroups = g.countConnectedGroupsUnlocked()

	return stats
}

// ComputeBlastRadius calculates the full impact of changing a node.
func (g *DependencyGraph) ComputeBlastRadius(nodeID string) (*BlastRadius, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if _, ok := g.nodes[nodeID]; !ok {
		return nil, fmt.Errorf("computing blast radius: node %q not found", nodeID)
	}

	direct := g.getDownstreamUnlocked(nodeID)
	transitive := g.getFullDownstreamUnlocked(nodeID)

	// Remove direct from transitive to avoid duplicates.
	directSet := make(map[string]bool)
	for _, n := range direct {
		directSet[n.ID] = true
	}
	transitiveOnly := make([]*GraphNode, 0)
	for _, n := range transitive {
		if !directSet[n.ID] && n.ID != nodeID {
			transitiveOnly = append(transitiveOnly, n)
		}
	}

	var sources, consumers []*GraphNode
	for _, n := range transitive {
		switch n.Type {
		case NodeTypeSource:
			sources = append(sources, n)
		case NodeTypeConsumer:
			consumers = append(consumers, n)
		}
	}

	// Also check upstream sources.
	upstream := g.getFullUpstreamUnlocked(nodeID)
	for _, n := range upstream {
		if n.Type == NodeTypeSource {
			sources = append(sources, n)
		}
	}

	totalAffected := len(direct) + len(transitiveOnly)
	riskScore := float64(totalAffected) / float64(max(len(g.nodes), 1)) * 10.0
	if riskScore > 10.0 {
		riskScore = 10.0
	}

	return &BlastRadius{
		NodeID:            nodeID,
		DirectImpact:      direct,
		TransitiveImpact:  transitiveOnly,
		AffectedSources:   sources,
		AffectedConsumers: consumers,
		TotalAffected:     totalAffected,
		RiskScore:         riskScore,
		ComputedAt:        time.Now(),
	}, nil
}

// FindPaths finds all paths between two nodes (up to maxPaths).
func (g *DependencyGraph) FindPaths(fromID, toID string, maxPaths int) (*PathResult, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if _, ok := g.nodes[fromID]; !ok {
		return nil, fmt.Errorf("finding paths: source node %q not found", fromID)
	}
	if _, ok := g.nodes[toID]; !ok {
		return nil, fmt.Errorf("finding paths: target node %q not found", toID)
	}

	if maxPaths <= 0 {
		maxPaths = 10
	}

	adj := make(map[string][]string)
	for _, e := range g.edges {
		adj[e.From] = append(adj[e.From], e.To)
	}

	var allPaths [][]string
	var dfs func(current string, path []string, visited map[string]bool)
	dfs = func(current string, path []string, visited map[string]bool) {
		if len(allPaths) >= maxPaths {
			return
		}
		if current == toID {
			pathCopy := make([]string, len(path))
			copy(pathCopy, path)
			allPaths = append(allPaths, pathCopy)
			return
		}
		for _, next := range adj[current] {
			if !visited[next] {
				visited[next] = true
				dfs(next, append(path, next), visited)
				delete(visited, next)
			}
		}
	}

	visited := map[string]bool{fromID: true}
	dfs(fromID, []string{fromID}, visited)

	shortest := 0
	if len(allPaths) > 0 {
		shortest = len(allPaths[0])
		for _, p := range allPaths {
			if len(p) < shortest {
				shortest = len(p)
			}
		}
	}

	return &PathResult{
		From:     fromID,
		To:       toID,
		Paths:    allPaths,
		Shortest: shortest,
	}, nil
}

// SearchNodes finds nodes matching a query string in their ID or label.
func (g *DependencyGraph) SearchNodes(query string, nodeType NodeType) []*GraphNode {
	g.mu.RLock()
	defer g.mu.RUnlock()

	var results []*GraphNode
	for _, n := range g.nodes {
		if nodeType != "" && n.Type != nodeType {
			continue
		}
		if containsIgnoreCase(n.ID, query) || containsIgnoreCase(n.Label, query) {
			results = append(results, n)
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].ID < results[j].ID
	})
	return results
}

func containsIgnoreCase(s, substr string) bool {
	if substr == "" {
		return true
	}
	sl := len(s)
	tl := len(substr)
	if tl > sl {
		return false
	}
	for i := 0; i <= sl-tl; i++ {
		match := true
		for j := 0; j < tl; j++ {
			sc := s[i+j]
			tc := substr[j]
			if sc >= 'A' && sc <= 'Z' {
				sc += 32
			}
			if tc >= 'A' && tc <= 'Z' {
				tc += 32
			}
			if sc != tc {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

func (g *DependencyGraph) computeMaxDepthUnlocked() int {
	adj := make(map[string][]string)
	inDegree := make(map[string]int)
	for _, n := range g.nodes {
		inDegree[n.ID] = 0
	}
	for _, e := range g.edges {
		adj[e.From] = append(adj[e.From], e.To)
		inDegree[e.To]++
	}

	// BFS from roots (inDegree == 0).
	var roots []string
	for id, d := range inDegree {
		if d == 0 {
			roots = append(roots, id)
		}
	}

	if len(roots) == 0 {
		return 0
	}

	depth := 0
	visited := make(map[string]bool)
	current := roots
	for len(current) > 0 {
		depth++
		var next []string
		for _, id := range current {
			visited[id] = true
			for _, child := range adj[id] {
				if !visited[child] {
					next = append(next, child)
				}
			}
		}
		current = next
	}
	return depth
}

func (g *DependencyGraph) countConnectedGroupsUnlocked() int {
	if len(g.nodes) == 0 {
		return 0
	}

	adj := make(map[string][]string)
	for _, e := range g.edges {
		adj[e.From] = append(adj[e.From], e.To)
		adj[e.To] = append(adj[e.To], e.From)
	}

	visited := make(map[string]bool)
	groups := 0
	for id := range g.nodes {
		if !visited[id] {
			groups++
			queue := []string{id}
			for len(queue) > 0 {
				cur := queue[0]
				queue = queue[1:]
				if visited[cur] {
					continue
				}
				visited[cur] = true
				queue = append(queue, adj[cur]...)
			}
		}
	}
	return groups
}

func (g *DependencyGraph) detectCyclesUnlocked() [][]string {
	// Reuse public method logic but without locking.
	visited := make(map[string]int) // 0=unvisited, 1=in-progress, 2=done
	var cycles [][]string
	var stack []string

	var dfs func(nodeID string)
	dfs = func(nodeID string) {
		visited[nodeID] = 1
		stack = append(stack, nodeID)

		adj := make(map[string][]string)
		for _, e := range g.edges {
			adj[e.From] = append(adj[e.From], e.To)
		}

		for _, next := range adj[nodeID] {
			if visited[next] == 1 {
				// Found cycle - extract it.
				cycleStart := -1
				for i, s := range stack {
					if s == next {
						cycleStart = i
						break
					}
				}
				if cycleStart >= 0 {
					cycle := make([]string, len(stack)-cycleStart)
					copy(cycle, stack[cycleStart:])
					cycles = append(cycles, cycle)
				}
			} else if visited[next] == 0 {
				dfs(next)
			}
		}

		stack = stack[:len(stack)-1]
		visited[nodeID] = 2
	}

	for id := range g.nodes {
		if visited[id] == 0 {
			dfs(id)
		}
	}
	return cycles
}

func (g *DependencyGraph) getDownstreamUnlocked(nodeID string) []*GraphNode {
	var result []*GraphNode
	for _, e := range g.edges {
		if e.From == nodeID {
			if n, ok := g.nodes[e.To]; ok {
				result = append(result, n)
			}
		}
	}
	return result
}

func (g *DependencyGraph) getFullDownstreamUnlocked(nodeID string) []*GraphNode {
	visited := make(map[string]bool)
	var result []*GraphNode
	queue := []string{nodeID}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		if visited[current] {
			continue
		}
		visited[current] = true
		if current != nodeID {
			if n, ok := g.nodes[current]; ok {
				result = append(result, n)
			}
		}
		for _, e := range g.edges {
			if e.From == current && !visited[e.To] {
				queue = append(queue, e.To)
			}
		}
	}
	return result
}

func (g *DependencyGraph) getFullUpstreamUnlocked(nodeID string) []*GraphNode {
	visited := make(map[string]bool)
	var result []*GraphNode
	queue := []string{nodeID}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		if visited[current] {
			continue
		}
		visited[current] = true
		if current != nodeID {
			if n, ok := g.nodes[current]; ok {
				result = append(result, n)
			}
		}
		for _, e := range g.edges {
			if e.To == current && !visited[e.From] {
				queue = append(queue, e.From)
			}
		}
	}
	return result
}
