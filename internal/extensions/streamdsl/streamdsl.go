package streamdsl

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// NodeType represents the type of a node in the execution DAG.
type NodeType string

const (
	NodeSource    NodeType = "source"
	NodeTransform NodeType = "transform"
	NodeWindow    NodeType = "window"
	NodeJoin      NodeType = "join"
	NodeSink      NodeType = "sink"
	NodeFilter    NodeType = "filter"
	NodeAggregate NodeType = "aggregate"
)

// WindowType represents the windowing strategy.
type WindowType string

const (
	WindowTumbling WindowType = "tumbling"
	WindowSliding  WindowType = "sliding"
	WindowSession  WindowType = "session"
)

// AggFunc represents a supported aggregation function.
type AggFunc string

const (
	AggCount AggFunc = "count"
	AggSum   AggFunc = "sum"
	AggAvg   AggFunc = "avg"
	AggMin   AggFunc = "min"
	AggMax   AggFunc = "max"
	AggLast  AggFunc = "last"
	AggFirst AggFunc = "first"
)

var validAggFuncs = map[AggFunc]bool{
	AggCount: true, AggSum: true, AggAvg: true,
	AggMin: true, AggMax: true, AggLast: true, AggFirst: true,
}

var validJoinTypes = map[string]bool{
	"inner": true, "left": true, "right": true,
}

// PipelineSpec is the YAML/JSON-serializable pipeline definition.
type PipelineSpec struct {
	Name         string            `json:"name" yaml:"name"`
	Version      string            `json:"version" yaml:"version"`
	Description  string            `json:"description,omitempty" yaml:"description,omitempty"`
	Sources      []SourceSpec      `json:"sources" yaml:"sources"`
	Transforms   []TransformSpec   `json:"transforms,omitempty" yaml:"transforms,omitempty"`
	Windows      []WindowSpec      `json:"windows,omitempty" yaml:"windows,omitempty"`
	Joins        []JoinSpec        `json:"joins,omitempty" yaml:"joins,omitempty"`
	Sinks        []SinkSpec        `json:"sinks" yaml:"sinks"`
	Filters      []FilterSpec      `json:"filters,omitempty" yaml:"filters,omitempty"`
	Aggregations []AggregationSpec `json:"aggregations,omitempty" yaml:"aggregations,omitempty"`
}

// SourceSpec defines an input data source.
type SourceSpec struct {
	Name   string            `json:"name" yaml:"name"`
	Type   string            `json:"type" yaml:"type"`
	Config map[string]string `json:"config,omitempty" yaml:"config,omitempty"`
	Schema map[string]string `json:"schema,omitempty" yaml:"schema,omitempty"`
}

// TransformSpec defines a transformation applied to a stream.
type TransformSpec struct {
	Name       string `json:"name" yaml:"name"`
	Input      string `json:"input" yaml:"input"`
	Expression string `json:"expression" yaml:"expression"`
	OutputType string `json:"output_type,omitempty" yaml:"output_type,omitempty"`
}

// WindowSpec defines a windowing operation.
type WindowSpec struct {
	Name  string        `json:"name" yaml:"name"`
	Input string        `json:"input" yaml:"input"`
	Type  WindowType    `json:"type" yaml:"type"`
	Size  time.Duration `json:"size" yaml:"size"`
	Slide time.Duration `json:"slide,omitempty" yaml:"slide,omitempty"`
	Gap   time.Duration `json:"gap,omitempty" yaml:"gap,omitempty"`
}

// JoinSpec defines a join between two streams.
type JoinSpec struct {
	Name   string        `json:"name" yaml:"name"`
	Left   string        `json:"left" yaml:"left"`
	Right  string        `json:"right" yaml:"right"`
	On     string        `json:"on" yaml:"on"`
	Type   string        `json:"type" yaml:"type"`
	Window time.Duration `json:"window,omitempty" yaml:"window,omitempty"`
}

// SinkSpec defines an output destination.
type SinkSpec struct {
	Name   string            `json:"name" yaml:"name"`
	Input  string            `json:"input" yaml:"input"`
	Type   string            `json:"type" yaml:"type"`
	Config map[string]string `json:"config,omitempty" yaml:"config,omitempty"`
}

// FilterSpec defines a filter condition on a stream.
type FilterSpec struct {
	Name      string `json:"name" yaml:"name"`
	Input     string `json:"input" yaml:"input"`
	Condition string `json:"condition" yaml:"condition"`
}

// AggregationSpec defines an aggregation on a stream.
type AggregationSpec struct {
	Name     string   `json:"name" yaml:"name"`
	Input    string   `json:"input" yaml:"input"`
	Function AggFunc  `json:"function" yaml:"function"`
	Field    string   `json:"field" yaml:"field"`
	GroupBy  []string `json:"group_by,omitempty" yaml:"group_by,omitempty"`
}

// ExecutionPlan is the compiled, validated pipeline ready for execution.
type ExecutionPlan struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Nodes       []ExecutionNode `json:"nodes"`
	Edges       []ExecutionEdge `json:"edges"`
	Status      string          `json:"status"`
	CompiledAt  time.Time       `json:"compiled_at"`
	Warnings    []string        `json:"warnings,omitempty"`
	Parallelism int             `json:"parallelism"`
}

// ExecutionNode is a single node in the execution DAG.
type ExecutionNode struct {
	ID      string            `json:"id"`
	Type    NodeType          `json:"type"`
	Name    string            `json:"name"`
	Config  map[string]string `json:"config,omitempty"`
	Inputs  []string          `json:"inputs"`
	Outputs []string          `json:"outputs"`
}

// ExecutionEdge is a directed edge in the execution DAG.
type ExecutionEdge struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// ValidationError describes a single validation problem.
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

func (v ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", v.Field, v.Message)
}

// PipelineStats holds aggregate statistics about managed pipelines.
type PipelineStats struct {
	TotalPipelines  int            `json:"total_pipelines"`
	ByStatus        map[string]int `json:"by_status"`
	TotalNodes      int            `json:"total_nodes"`
	AvgNodesPerPlan float64        `json:"avg_nodes_per_plan"`
}

// CompilerConfig controls compiler limits and allowed source/sink types.
type CompilerConfig struct {
	MaxNodes       int
	MaxJoins       int
	AllowedSources []string
	AllowedSinks   []string
}

// DefaultCompilerConfig returns sensible defaults for the compiler.
func DefaultCompilerConfig() CompilerConfig {
	return CompilerConfig{
		MaxNodes:       100,
		MaxJoins:       10,
		AllowedSources: []string{"kafka", "http", "feature_store"},
		AllowedSinks:   []string{"feature_store", "kafka", "stdout"},
	}
}

// Compiler compiles a PipelineSpec into an ExecutionPlan.
type Compiler struct {
	config CompilerConfig
}

// NewCompiler creates a Compiler with the given configuration.
func NewCompiler(cfg CompilerConfig) *Compiler {
	return &Compiler{config: cfg}
}

// Compile validates the spec and builds the execution DAG.
func (c *Compiler) Compile(spec PipelineSpec) (*ExecutionPlan, error) {
	if errs := c.validate(spec); len(errs) > 0 {
		return nil, fmt.Errorf("validating pipeline: %s", errs[0].Error())
	}

	nodes, edges := c.buildGraph(spec)

	order, err := topoSort(nodes, edges)
	if err != nil {
		return nil, fmt.Errorf("compiling pipeline: %w", err)
	}

	sorted := make([]ExecutionNode, len(order))
	nodeByID := make(map[string]ExecutionNode, len(nodes))
	for _, n := range nodes {
		nodeByID[n.ID] = n
	}
	for i, id := range order {
		sorted[i] = nodeByID[id]
	}

	plan := &ExecutionPlan{
		Name:        spec.Name,
		Nodes:       sorted,
		Edges:       edges,
		Status:      "compiled",
		CompiledAt:  time.Now(),
		Parallelism: 1,
	}
	return plan, nil
}

// validate checks the spec for structural errors.
func (c *Compiler) validate(spec PipelineSpec) []ValidationError {
	var errs []ValidationError

	if spec.Name == "" {
		errs = append(errs, ValidationError{Field: "name", Message: "pipeline name is required"})
	}
	if len(spec.Sources) == 0 {
		errs = append(errs, ValidationError{Field: "sources", Message: "at least one source is required"})
	}
	if len(spec.Sinks) == 0 {
		errs = append(errs, ValidationError{Field: "sinks", Message: "at least one sink is required"})
	}

	// Collect all node names for reference checking and duplicate detection.
	names := make(map[string]int)
	for _, s := range spec.Sources {
		if s.Name == "" {
			errs = append(errs, ValidationError{Field: "sources", Message: "source name is required"})
		}
		names[s.Name]++
		if c.config.AllowedSources != nil && !contains(c.config.AllowedSources, s.Type) {
			errs = append(errs, ValidationError{Field: "sources", Message: fmt.Sprintf("source type %q is not allowed", s.Type)})
		}
	}
	for _, t := range spec.Transforms {
		if t.Name == "" {
			errs = append(errs, ValidationError{Field: "transforms", Message: "transform name is required"})
		}
		names[t.Name]++
	}
	for _, w := range spec.Windows {
		if w.Name == "" {
			errs = append(errs, ValidationError{Field: "windows", Message: "window name is required"})
		}
		names[w.Name]++
		if w.Size <= 0 {
			errs = append(errs, ValidationError{Field: "windows", Message: fmt.Sprintf("window %q size must be > 0", w.Name)})
		}
	}
	for _, j := range spec.Joins {
		if j.Name == "" {
			errs = append(errs, ValidationError{Field: "joins", Message: "join name is required"})
		}
		names[j.Name]++
		if !validJoinTypes[j.Type] {
			errs = append(errs, ValidationError{Field: "joins", Message: fmt.Sprintf("join %q has invalid type %q", j.Name, j.Type)})
		}
	}
	for _, s := range spec.Sinks {
		if s.Name == "" {
			errs = append(errs, ValidationError{Field: "sinks", Message: "sink name is required"})
		}
		names[s.Name]++
		if c.config.AllowedSinks != nil && !contains(c.config.AllowedSinks, s.Type) {
			errs = append(errs, ValidationError{Field: "sinks", Message: fmt.Sprintf("sink type %q is not allowed", s.Type)})
		}
	}
	for _, f := range spec.Filters {
		if f.Name == "" {
			errs = append(errs, ValidationError{Field: "filters", Message: "filter name is required"})
		}
		names[f.Name]++
	}
	for _, a := range spec.Aggregations {
		if a.Name == "" {
			errs = append(errs, ValidationError{Field: "aggregations", Message: "aggregation name is required"})
		}
		names[a.Name]++
		if !validAggFuncs[a.Function] {
			errs = append(errs, ValidationError{Field: "aggregations", Message: fmt.Sprintf("aggregation %q has invalid function %q", a.Name, a.Function)})
		}
	}

	// Check for duplicate names.
	for name, count := range names {
		if count > 1 {
			errs = append(errs, ValidationError{Field: "names", Message: fmt.Sprintf("duplicate node name %q", name)})
		}
	}

	// Check that all input references exist.
	for _, t := range spec.Transforms {
		if _, ok := names[t.Input]; !ok {
			errs = append(errs, ValidationError{Field: "transforms", Message: fmt.Sprintf("transform %q references unknown input %q", t.Name, t.Input)})
		}
	}
	for _, w := range spec.Windows {
		if _, ok := names[w.Input]; !ok {
			errs = append(errs, ValidationError{Field: "windows", Message: fmt.Sprintf("window %q references unknown input %q", w.Name, w.Input)})
		}
	}
	for _, j := range spec.Joins {
		if _, ok := names[j.Left]; !ok {
			errs = append(errs, ValidationError{Field: "joins", Message: fmt.Sprintf("join %q references unknown left input %q", j.Name, j.Left)})
		}
		if _, ok := names[j.Right]; !ok {
			errs = append(errs, ValidationError{Field: "joins", Message: fmt.Sprintf("join %q references unknown right input %q", j.Name, j.Right)})
		}
	}
	for _, s := range spec.Sinks {
		if _, ok := names[s.Input]; !ok {
			errs = append(errs, ValidationError{Field: "sinks", Message: fmt.Sprintf("sink %q references unknown input %q", s.Name, s.Input)})
		}
	}
	for _, f := range spec.Filters {
		if _, ok := names[f.Input]; !ok {
			errs = append(errs, ValidationError{Field: "filters", Message: fmt.Sprintf("filter %q references unknown input %q", f.Name, f.Input)})
		}
	}
	for _, a := range spec.Aggregations {
		if _, ok := names[a.Input]; !ok {
			errs = append(errs, ValidationError{Field: "aggregations", Message: fmt.Sprintf("aggregation %q references unknown input %q", a.Name, a.Input)})
		}
	}

	// Check node/join limits.
	totalNodes := len(spec.Sources) + len(spec.Transforms) + len(spec.Windows) +
		len(spec.Joins) + len(spec.Sinks) + len(spec.Filters) + len(spec.Aggregations)
	if c.config.MaxNodes > 0 && totalNodes > c.config.MaxNodes {
		errs = append(errs, ValidationError{Field: "nodes", Message: fmt.Sprintf("total nodes (%d) exceeds max (%d)", totalNodes, c.config.MaxNodes)})
	}
	if c.config.MaxJoins > 0 && len(spec.Joins) > c.config.MaxJoins {
		errs = append(errs, ValidationError{Field: "joins", Message: fmt.Sprintf("total joins (%d) exceeds max (%d)", len(spec.Joins), c.config.MaxJoins)})
	}

	return errs
}

// buildGraph creates execution nodes and edges from the spec.
func (c *Compiler) buildGraph(spec PipelineSpec) ([]ExecutionNode, []ExecutionEdge) {
	var nodes []ExecutionNode
	var edges []ExecutionEdge

	for _, s := range spec.Sources {
		nodes = append(nodes, ExecutionNode{
			ID: s.Name, Type: NodeSource, Name: s.Name,
			Config: s.Config, Inputs: nil, Outputs: nil,
		})
	}
	for _, t := range spec.Transforms {
		nodes = append(nodes, ExecutionNode{
			ID: t.Name, Type: NodeTransform, Name: t.Name,
			Config: map[string]string{"expression": t.Expression},
			Inputs: []string{t.Input}, Outputs: nil,
		})
		edges = append(edges, ExecutionEdge{From: t.Input, To: t.Name})
	}
	for _, w := range spec.Windows {
		nodes = append(nodes, ExecutionNode{
			ID: w.Name, Type: NodeWindow, Name: w.Name,
			Config:  map[string]string{"type": string(w.Type), "size": w.Size.String()},
			Inputs:  []string{w.Input}, Outputs: nil,
		})
		edges = append(edges, ExecutionEdge{From: w.Input, To: w.Name})
	}
	for _, j := range spec.Joins {
		nodes = append(nodes, ExecutionNode{
			ID: j.Name, Type: NodeJoin, Name: j.Name,
			Config:  map[string]string{"on": j.On, "type": j.Type},
			Inputs:  []string{j.Left, j.Right}, Outputs: nil,
		})
		edges = append(edges, ExecutionEdge{From: j.Left, To: j.Name})
		edges = append(edges, ExecutionEdge{From: j.Right, To: j.Name})
	}
	for _, f := range spec.Filters {
		nodes = append(nodes, ExecutionNode{
			ID: f.Name, Type: NodeFilter, Name: f.Name,
			Config:  map[string]string{"condition": f.Condition},
			Inputs:  []string{f.Input}, Outputs: nil,
		})
		edges = append(edges, ExecutionEdge{From: f.Input, To: f.Name})
	}
	for _, a := range spec.Aggregations {
		nodes = append(nodes, ExecutionNode{
			ID: a.Name, Type: NodeAggregate, Name: a.Name,
			Config:  map[string]string{"function": string(a.Function), "field": a.Field},
			Inputs:  []string{a.Input}, Outputs: nil,
		})
		edges = append(edges, ExecutionEdge{From: a.Input, To: a.Name})
	}
	for _, s := range spec.Sinks {
		nodes = append(nodes, ExecutionNode{
			ID: s.Name, Type: NodeSink, Name: s.Name,
			Config: s.Config, Inputs: []string{s.Input}, Outputs: nil,
		})
		edges = append(edges, ExecutionEdge{From: s.Input, To: s.Name})
	}

	// Populate output references.
	outMap := make(map[string][]string)
	for _, e := range edges {
		outMap[e.From] = append(outMap[e.From], e.To)
	}
	for i := range nodes {
		if outs, ok := outMap[nodes[i].ID]; ok {
			nodes[i].Outputs = outs
		}
	}

	return nodes, edges
}

// topoSort performs Kahn's algorithm; returns an error if a cycle exists.
func topoSort(nodes []ExecutionNode, edges []ExecutionEdge) ([]string, error) {
	inDeg := make(map[string]int, len(nodes))
	adj := make(map[string][]string, len(nodes))
	for _, n := range nodes {
		inDeg[n.ID] = 0
	}
	for _, e := range edges {
		adj[e.From] = append(adj[e.From], e.To)
		inDeg[e.To]++
	}

	var queue []string
	for _, n := range nodes {
		if inDeg[n.ID] == 0 {
			queue = append(queue, n.ID)
		}
	}

	var order []string
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		order = append(order, cur)
		for _, next := range adj[cur] {
			inDeg[next]--
			if inDeg[next] == 0 {
				queue = append(queue, next)
			}
		}
	}

	if len(order) != len(nodes) {
		return nil, fmt.Errorf("cycle detected in pipeline graph")
	}
	return order, nil
}

// PipelineManager manages compiled pipelines.
type PipelineManager struct {
	mu        sync.RWMutex
	compiler  *Compiler
	pipelines map[string]*ExecutionPlan
	idCounter int64
}

// NewPipelineManager creates a PipelineManager with the given compiler config.
func NewPipelineManager(cfg CompilerConfig) *PipelineManager {
	return &PipelineManager{
		compiler:  NewCompiler(cfg),
		pipelines: make(map[string]*ExecutionPlan),
	}
}

// Compile compiles the spec and stores the resulting plan.
func (pm *PipelineManager) Compile(spec PipelineSpec) (*ExecutionPlan, error) {
	plan, err := pm.compiler.Compile(spec)
	if err != nil {
		return nil, err
	}

	id := fmt.Sprintf("p-%d", atomic.AddInt64(&pm.idCounter, 1))
	plan.ID = id

	pm.mu.Lock()
	pm.pipelines[id] = plan
	pm.mu.Unlock()

	return plan, nil
}

// Get returns a pipeline by ID.
func (pm *PipelineManager) Get(id string) (*ExecutionPlan, error) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	plan, ok := pm.pipelines[id]
	if !ok {
		return nil, fmt.Errorf("pipeline %q not found", id)
	}
	return plan, nil
}

// List returns all managed pipelines.
func (pm *PipelineManager) List() []*ExecutionPlan {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	out := make([]*ExecutionPlan, 0, len(pm.pipelines))
	for _, p := range pm.pipelines {
		out = append(out, p)
	}
	return out
}

// Delete removes a pipeline by ID.
func (pm *PipelineManager) Delete(id string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if _, ok := pm.pipelines[id]; !ok {
		return fmt.Errorf("pipeline %q not found", id)
	}
	delete(pm.pipelines, id)
	return nil
}

// Validate checks a spec without compiling it.
func (pm *PipelineManager) Validate(spec PipelineSpec) []ValidationError {
	return pm.compiler.validate(spec)
}

// Stats returns aggregate statistics about managed pipelines.
func (pm *PipelineManager) Stats() PipelineStats {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	stats := PipelineStats{
		TotalPipelines: len(pm.pipelines),
		ByStatus:       make(map[string]int),
	}
	for _, p := range pm.pipelines {
		stats.ByStatus[p.Status]++
		stats.TotalNodes += len(p.Nodes)
	}
	if stats.TotalPipelines > 0 {
		stats.AvgNodesPerPlan = float64(stats.TotalNodes) / float64(stats.TotalPipelines)
	}
	return stats
}

func contains(ss []string, s string) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}
