package computegraph

import (
	"fmt"
	"sort"
	"sync"
	"time"
)

// NodeKind classifies the role of a node in the computation graph.
type NodeKind string

const (
	KindSource     NodeKind = "source"
	KindDerived    NodeKind = "derived"
	KindAggregated NodeKind = "aggregated"
)

// ComputeFunc identifies the built-in function applied at a node.
type ComputeFunc string

const (
	FuncIdentity   ComputeFunc = "identity"
	FuncSum        ComputeFunc = "sum"
	FuncAvg        ComputeFunc = "avg"
	FuncMultiply   ComputeFunc = "multiply"
	FuncDivide     ComputeFunc = "divide"
	FuncConcat     ComputeFunc = "concat"
	FuncCoalesce   ComputeFunc = "coalesce"
	FuncCustomExpr ComputeFunc = "custom_expr"
)

// MaterializePolicy controls when a node's value is recomputed.
type MaterializePolicy string

const (
	PolicyEager     MaterializePolicy = "eager"     // compute immediately on input change
	PolicyLazy      MaterializePolicy = "lazy"      // compute on read
	PolicyScheduled MaterializePolicy = "scheduled" // compute on schedule
)

// EngineConfig holds tunables for the compute graph engine.
type EngineConfig struct {
	MaxNodes      int
	MaxDepth      int
	CacheResults  bool
	DefaultPolicy MaterializePolicy
}

// DefaultEngineConfig returns sensible defaults.
func DefaultEngineConfig() EngineConfig {
	return EngineConfig{
		MaxNodes:      1000,
		MaxDepth:      20,
		CacheResults:  true,
		DefaultPolicy: PolicyLazy,
	}
}

// Engine manages a DAG of feature computation nodes.
type Engine struct {
	config EngineConfig
	mu     sync.RWMutex
	nodes  map[string]*FeatureNode
	cache  map[string]*ComputeResult
	dirty  map[string]bool
}

// FeatureNode is a single vertex in the computation graph.
type FeatureNode struct {
	Name        string            `json:"name"`
	Kind        NodeKind          `json:"kind"`
	Inputs      []string          `json:"inputs"`
	Function    ComputeFunc       `json:"function"`
	Expression  string            `json:"expression,omitempty"`
	Policy      MaterializePolicy `json:"materialize_policy"`
	OutputType  string            `json:"output_type"`
	Description string            `json:"description,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// ComputeResult holds the outcome of evaluating a single node.
type ComputeResult struct {
	NodeName   string      `json:"node_name"`
	Value      interface{} `json:"value"`
	ComputedAt time.Time   `json:"computed_at"`
	InputHash  string      `json:"input_hash"`
	FromCache  bool        `json:"from_cache"`
	DurationMs float64     `json:"duration_ms"`
}

// DAGInfo represents the full graph topology.
type DAGInfo struct {
	Nodes     []FeatureNode `json:"nodes"`
	Edges     []Edge        `json:"edges"`
	Depth     int           `json:"depth"`
	LeafCount int           `json:"leaf_count"`
	RootCount int           `json:"root_count"`
	IsValid   bool          `json:"is_valid"`
}

// Edge is a directed dependency between two nodes.
type Edge struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// TopologicalOrder contains the result of a topological sort.
type TopologicalOrder struct {
	Levels [][]string `json:"levels"`
	Order  []string   `json:"order"`
}

// ValidationError describes a single problem detected during validation.
type ValidationError struct {
	Node    string `json:"node"`
	Message string `json:"message"`
}

// GraphStats reports high-level engine statistics.
type GraphStats struct {
	TotalNodes    int            `json:"total_nodes"`
	SourceNodes   int            `json:"source_nodes"`
	DerivedNodes  int            `json:"derived_nodes"`
	TotalEdges    int            `json:"total_edges"`
	MaxDepth      int            `json:"max_depth"`
	CachedResults int            `json:"cached_results"`
	DirtyNodes    int            `json:"dirty_nodes"`
	ByPolicy      map[string]int `json:"by_policy"`
}

// NewEngine creates a new compute graph engine.
func NewEngine(cfg EngineConfig) *Engine {
	return &Engine{
		config: cfg,
		nodes:  make(map[string]*FeatureNode),
		cache:  make(map[string]*ComputeResult),
		dirty:  make(map[string]bool),
	}
}

// AddNode registers a feature node in the graph after validation.
func (e *Engine) AddNode(node FeatureNode) error {
	if node.Name == "" {
		return fmt.Errorf("node name must not be empty")
	}

	for _, inp := range node.Inputs {
		if inp == node.Name {
			return fmt.Errorf("node %q references itself", node.Name)
		}
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.nodes[node.Name]; exists {
		return fmt.Errorf("node %q already exists", node.Name)
	}

	if len(e.nodes) >= e.config.MaxNodes {
		return fmt.Errorf("max nodes (%d) reached", e.config.MaxNodes)
	}

	// Source nodes must have no inputs; derived/aggregated must reference existing nodes.
	if node.Kind == KindSource {
		if len(node.Inputs) > 0 {
			return fmt.Errorf("source node %q must not have inputs", node.Name)
		}
	} else {
		for _, inp := range node.Inputs {
			if _, ok := e.nodes[inp]; !ok {
				return fmt.Errorf("input %q of node %q not found", inp, node.Name)
			}
		}
	}

	// Cycle detection via DFS.
	if e.wouldCycle(node.Name, node.Inputs) {
		return fmt.Errorf("cycle detected: adding %q would create a cycle", node.Name)
	}

	now := time.Now()
	node.CreatedAt = now
	node.UpdatedAt = now
	if node.Policy == "" {
		node.Policy = e.config.DefaultPolicy
	}

	e.nodes[node.Name] = &node
	return nil
}

// wouldCycle returns true if adding a node with the given inputs would form a cycle.
// It checks whether any transitive input eventually references name.
func (e *Engine) wouldCycle(name string, inputs []string) bool {
	visited := make(map[string]bool)
	var dfs func(string) bool
	dfs = func(cur string) bool {
		if cur == name {
			return true
		}
		if visited[cur] {
			return false
		}
		visited[cur] = true
		if n, ok := e.nodes[cur]; ok {
			for _, inp := range n.Inputs {
				if dfs(inp) {
					return true
				}
			}
		}
		return false
	}

	for _, inp := range inputs {
		if dfs(inp) {
			return true
		}
	}
	return false
}

// RemoveNode removes a node only if no other node depends on it.
func (e *Engine) RemoveNode(name string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.nodes[name]; !exists {
		return fmt.Errorf("node %q not found", name)
	}

	for n, node := range e.nodes {
		if n == name {
			continue
		}
		for _, inp := range node.Inputs {
			if inp == name {
				return fmt.Errorf("cannot remove %q: node %q depends on it", name, n)
			}
		}
	}

	delete(e.nodes, name)
	delete(e.cache, name)
	delete(e.dirty, name)
	return nil
}

// GetNode returns a copy of the named node.
func (e *Engine) GetNode(name string) (*FeatureNode, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	n, ok := e.nodes[name]
	if !ok {
		return nil, fmt.Errorf("node %q not found", name)
	}
	cp := *n
	return &cp, nil
}

// ListNodes returns all nodes sorted by name.
func (e *Engine) ListNodes() []*FeatureNode {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := make([]*FeatureNode, 0, len(e.nodes))
	for _, n := range e.nodes {
		cp := *n
		result = append(result, &cp)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result
}

// GetUpstream returns all transitive input nodes for the named node.
func (e *Engine) GetUpstream(name string) ([]string, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if _, ok := e.nodes[name]; !ok {
		return nil, fmt.Errorf("node %q not found", name)
	}

	visited := make(map[string]bool)
	e.collectUpstream(name, visited)
	delete(visited, name)

	result := make([]string, 0, len(visited))
	for n := range visited {
		result = append(result, n)
	}
	sort.Strings(result)
	return result, nil
}

func (e *Engine) collectUpstream(name string, visited map[string]bool) {
	if visited[name] {
		return
	}
	visited[name] = true
	if n, ok := e.nodes[name]; ok {
		for _, inp := range n.Inputs {
			e.collectUpstream(inp, visited)
		}
	}
}

// GetDownstream returns all transitive dependent nodes for the named node.
func (e *Engine) GetDownstream(name string) ([]string, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if _, ok := e.nodes[name]; !ok {
		return nil, fmt.Errorf("node %q not found", name)
	}

	visited := make(map[string]bool)
	e.collectDownstream(name, visited)
	delete(visited, name)

	result := make([]string, 0, len(visited))
	for n := range visited {
		result = append(result, n)
	}
	sort.Strings(result)
	return result, nil
}

func (e *Engine) collectDownstream(name string, visited map[string]bool) {
	if visited[name] {
		return
	}
	visited[name] = true
	for n, node := range e.nodes {
		for _, inp := range node.Inputs {
			if inp == name {
				e.collectDownstream(n, visited)
			}
		}
	}
}

// TopologicalSort returns a topological ordering of the graph using Kahn's algorithm.
func (e *Engine) TopologicalSort() (*TopologicalOrder, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	inDegree := make(map[string]int)
	dependents := make(map[string][]string) // node -> nodes that depend on it

	for name := range e.nodes {
		inDegree[name] = 0
	}
	for name, node := range e.nodes {
		for _, inp := range node.Inputs {
			dependents[inp] = append(dependents[inp], name)
			inDegree[name]++
		}
	}

	// Seed the queue with zero-in-degree nodes.
	var queue []string
	for name, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, name)
		}
	}
	sort.Strings(queue) // deterministic ordering

	var order []string
	var levels [][]string

	for len(queue) > 0 {
		sort.Strings(queue)
		levels = append(levels, append([]string(nil), queue...))

		var next []string
		for _, name := range queue {
			order = append(order, name)
			for _, dep := range dependents[name] {
				inDegree[dep]--
				if inDegree[dep] == 0 {
					next = append(next, dep)
				}
			}
		}
		queue = next
	}

	if len(order) != len(e.nodes) {
		return nil, fmt.Errorf("cycle detected in graph")
	}

	return &TopologicalOrder{
		Levels: levels,
		Order:  order,
	}, nil
}

// Compute evaluates the named node, recursively computing its subgraph.
func (e *Engine) Compute(name string, inputs map[string]interface{}) (*ComputeResult, error) {
	start := time.Now()

	e.mu.RLock()
	node, ok := e.nodes[name]
	if !ok {
		e.mu.RUnlock()
		return nil, fmt.Errorf("node %q not found", name)
	}
	e.mu.RUnlock()

	// Build subgraph topological order.
	subOrder, err := e.subgraphOrder(name)
	if err != nil {
		return nil, fmt.Errorf("computing %q: %w", name, err)
	}

	// Intermediate values keyed by node name.
	values := make(map[string]interface{})
	for k, v := range inputs {
		values[k] = v
	}

	e.mu.RLock()
	defer e.mu.RUnlock()

	for _, n := range subOrder {
		nd := e.nodes[n]

		// If it's a source node, its value must be in inputs.
		if nd.Kind == KindSource {
			if _, ok := values[n]; !ok {
				return nil, fmt.Errorf("missing input for source node %q", n)
			}
			continue
		}

		// Check cached result.
		if e.config.CacheResults && !e.dirty[n] {
			if cr, ok := e.cache[n]; ok {
				values[n] = cr.Value
				continue
			}
		}

		// Gather input values.
		inputVals := make([]interface{}, 0, len(nd.Inputs))
		for _, inp := range nd.Inputs {
			v, ok := values[inp]
			if !ok {
				return nil, fmt.Errorf("missing value for input %q of node %q", inp, n)
			}
			inputVals = append(inputVals, v)
		}

		val, err := applyFunc(nd.Function, inputVals)
		if err != nil {
			return nil, fmt.Errorf("computing node %q: %w", n, err)
		}
		values[n] = val
	}

	result := &ComputeResult{
		NodeName:   name,
		Value:      values[name],
		ComputedAt: time.Now(),
		InputHash:  computeInputHash(inputs),
		FromCache:  false,
		DurationMs: float64(time.Since(start).Microseconds()) / 1000.0,
	}

	if e.config.CacheResults {
		e.cache[name] = result
		delete(e.dirty, name)
	}

	_ = node // suppress unused warning
	return result, nil
}

// subgraphOrder returns a topological ordering of the subgraph reachable from the named node.
func (e *Engine) subgraphOrder(name string) ([]string, error) {
	// Collect all upstream nodes including name itself.
	visited := make(map[string]bool)
	e.collectUpstream(name, visited)

	// Kahn's on the subgraph.
	inDegree := make(map[string]int)
	dependents := make(map[string][]string)
	for n := range visited {
		inDegree[n] = 0
	}
	for n := range visited {
		nd := e.nodes[n]
		for _, inp := range nd.Inputs {
			if visited[inp] {
				dependents[inp] = append(dependents[inp], n)
				inDegree[n]++
			}
		}
	}

	var queue []string
	for n, d := range inDegree {
		if d == 0 {
			queue = append(queue, n)
		}
	}
	sort.Strings(queue)

	var order []string
	for len(queue) > 0 {
		sort.Strings(queue)
		var next []string
		for _, n := range queue {
			order = append(order, n)
			for _, dep := range dependents[n] {
				inDegree[dep]--
				if inDegree[dep] == 0 {
					next = append(next, dep)
				}
			}
		}
		queue = next
	}

	if len(order) != len(visited) {
		return nil, fmt.Errorf("cycle detected in subgraph of %q", name)
	}
	return order, nil
}

// applyFunc evaluates a built-in function over the given input values.
func applyFunc(fn ComputeFunc, vals []interface{}) (interface{}, error) {
	switch fn {
	case FuncIdentity:
		if len(vals) == 0 {
			return nil, fmt.Errorf("identity requires at least one input")
		}
		return vals[0], nil

	case FuncSum:
		sum := 0.0
		for _, v := range vals {
			n, ok := toFloat64(v)
			if !ok {
				return nil, fmt.Errorf("sum: non-numeric input %v", v)
			}
			sum += n
		}
		return sum, nil

	case FuncAvg:
		if len(vals) == 0 {
			return nil, fmt.Errorf("avg requires at least one input")
		}
		sum := 0.0
		for _, v := range vals {
			n, ok := toFloat64(v)
			if !ok {
				return nil, fmt.Errorf("avg: non-numeric input %v", v)
			}
			sum += n
		}
		return sum / float64(len(vals)), nil

	case FuncMultiply:
		product := 1.0
		for _, v := range vals {
			n, ok := toFloat64(v)
			if !ok {
				return nil, fmt.Errorf("multiply: non-numeric input %v", v)
			}
			product *= n
		}
		return product, nil

	case FuncDivide:
		if len(vals) != 2 {
			return nil, fmt.Errorf("divide requires exactly 2 inputs")
		}
		a, ok := toFloat64(vals[0])
		if !ok {
			return nil, fmt.Errorf("divide: non-numeric numerator %v", vals[0])
		}
		b, ok2 := toFloat64(vals[1])
		if !ok2 {
			return nil, fmt.Errorf("divide: non-numeric denominator %v", vals[1])
		}
		if b == 0 {
			return nil, fmt.Errorf("divide: division by zero")
		}
		return a / b, nil

	case FuncConcat:
		result := ""
		for _, v := range vals {
			result += fmt.Sprintf("%v", v)
		}
		return result, nil

	case FuncCoalesce:
		for _, v := range vals {
			if v != nil {
				return v, nil
			}
		}
		return nil, nil

	default:
		return nil, fmt.Errorf("unknown function %q", fn)
	}
}

// toFloat64 converts numeric types to float64.
func toFloat64(v interface{}) (float64, bool) {
	switch val := v.(type) {
	case float64:
		return val, true
	case float32:
		return float64(val), true
	case int:
		return float64(val), true
	case int32:
		return float64(val), true
	case int64:
		return float64(val), true
	case uint:
		return float64(val), true
	case uint32:
		return float64(val), true
	case uint64:
		return float64(val), true
	default:
		return 0, false
	}
}

// Invalidate marks the named node and all downstream nodes as dirty,
// returning the total count of invalidated nodes.
func (e *Engine) Invalidate(name string) int {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, ok := e.nodes[name]; !ok {
		return 0
	}

	visited := make(map[string]bool)
	e.markDirty(name, visited)
	for n := range visited {
		e.dirty[n] = true
		delete(e.cache, n)
	}
	return len(visited)
}

func (e *Engine) markDirty(name string, visited map[string]bool) {
	if visited[name] {
		return
	}
	visited[name] = true
	for n, node := range e.nodes {
		for _, inp := range node.Inputs {
			if inp == name {
				e.markDirty(n, visited)
			}
		}
	}
}

// GetDAG returns the full graph topology.
func (e *Engine) GetDAG() *DAGInfo {
	e.mu.RLock()
	defer e.mu.RUnlock()

	nodes := make([]FeatureNode, 0, len(e.nodes))
	var edges []Edge
	hasDependent := make(map[string]bool)
	isInput := make(map[string]bool)

	for _, n := range e.nodes {
		nodes = append(nodes, *n)
		for _, inp := range n.Inputs {
			edges = append(edges, Edge{From: inp, To: n.Name})
			hasDependent[inp] = true
			isInput[n.Name] = true
		}
	}

	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].Name < nodes[j].Name
	})

	// Leaves: nodes with no dependents. Roots: nodes with no inputs.
	leafCount := 0
	rootCount := 0
	for name, n := range e.nodes {
		if !hasDependent[name] {
			leafCount++
		}
		if len(n.Inputs) == 0 {
			rootCount++
		}
	}

	depth := e.maxDepth()
	valid := len(e.Validate()) == 0

	return &DAGInfo{
		Nodes:     nodes,
		Edges:     edges,
		Depth:     depth,
		LeafCount: leafCount,
		RootCount: rootCount,
		IsValid:   valid,
	}
}

func (e *Engine) maxDepth() int {
	max := 0
	for name := range e.nodes {
		d := e.nodeDepth(name, make(map[string]bool))
		if d > max {
			max = d
		}
	}
	return max
}

func (e *Engine) nodeDepth(name string, visited map[string]bool) int {
	if visited[name] {
		return 0
	}
	visited[name] = true
	n, ok := e.nodes[name]
	if !ok || len(n.Inputs) == 0 {
		return 0
	}
	max := 0
	for _, inp := range n.Inputs {
		d := e.nodeDepth(inp, visited)
		if d+1 > max {
			max = d + 1
		}
	}
	return max
}

// Validate checks all nodes for structural problems.
func (e *Engine) Validate() []ValidationError {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var errs []ValidationError
	for name, node := range e.nodes {
		if node.Name == "" {
			errs = append(errs, ValidationError{Node: name, Message: "empty name"})
		}
		if node.OutputType == "" {
			errs = append(errs, ValidationError{Node: name, Message: "missing output_type"})
		}
		for _, inp := range node.Inputs {
			if _, ok := e.nodes[inp]; !ok {
				errs = append(errs, ValidationError{Node: name, Message: fmt.Sprintf("input %q not found", inp)})
			}
		}
	}
	return errs
}

// Stats returns aggregate statistics about the graph.
func (e *Engine) Stats() GraphStats {
	e.mu.RLock()
	defer e.mu.RUnlock()

	stats := GraphStats{
		TotalNodes: len(e.nodes),
		ByPolicy:   make(map[string]int),
	}

	totalEdges := 0
	for _, n := range e.nodes {
		totalEdges += len(n.Inputs)
		switch n.Kind {
		case KindSource:
			stats.SourceNodes++
		case KindDerived, KindAggregated:
			stats.DerivedNodes++
		}
		stats.ByPolicy[string(n.Policy)]++
	}

	stats.TotalEdges = totalEdges
	stats.MaxDepth = e.maxDepth()
	stats.CachedResults = len(e.cache)
	stats.DirtyNodes = len(e.dirty)
	return stats
}

// computeInputHash creates a deterministic hash string from input values.
func computeInputHash(inputs map[string]interface{}) string {
	keys := make([]string, 0, len(inputs))
	for k := range inputs {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	hash := ""
	for _, k := range keys {
		hash += fmt.Sprintf("%s=%v;", k, inputs[k])
	}
	return hash
}
