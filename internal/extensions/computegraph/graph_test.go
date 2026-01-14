package computegraph

import (
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func newTestEngine() *Engine {
	return NewEngine(DefaultEngineConfig())
}

func mustAddNode(t *testing.T, e *Engine, node FeatureNode) {
	t.Helper()
	if err := e.AddNode(node); err != nil {
		t.Fatalf("AddNode(%q): %v", node.Name, err)
	}
}

// ---------------------------------------------------------------------------
// TestAddNode
// ---------------------------------------------------------------------------

func TestAddNode(t *testing.T) {
	tests := []struct {
		name    string
		nodes   []FeatureNode
		wantErr string
	}{
		{
			name: "add source node",
			nodes: []FeatureNode{
				{Name: "a", Kind: KindSource, Function: FuncIdentity, OutputType: "float64"},
			},
		},
		{
			name: "add derived node with valid input",
			nodes: []FeatureNode{
				{Name: "a", Kind: KindSource, Function: FuncIdentity, OutputType: "float64"},
				{Name: "b", Kind: KindDerived, Inputs: []string{"a"}, Function: FuncIdentity, OutputType: "float64"},
			},
		},
		{
			name: "empty name",
			nodes: []FeatureNode{
				{Name: "", Kind: KindSource},
			},
			wantErr: "must not be empty",
		},
		{
			name: "self-reference",
			nodes: []FeatureNode{
				{Name: "x", Kind: KindDerived, Inputs: []string{"x"}, Function: FuncSum, OutputType: "float64"},
			},
			wantErr: "references itself",
		},
		{
			name: "duplicate node",
			nodes: []FeatureNode{
				{Name: "a", Kind: KindSource, Function: FuncIdentity, OutputType: "float64"},
				{Name: "a", Kind: KindSource, Function: FuncIdentity, OutputType: "float64"},
			},
			wantErr: "already exists",
		},
		{
			name: "missing input",
			nodes: []FeatureNode{
				{Name: "b", Kind: KindDerived, Inputs: []string{"nope"}, Function: FuncSum, OutputType: "float64"},
			},
			wantErr: "not found",
		},
		{
			name: "source with inputs",
			nodes: []FeatureNode{
				{Name: "a", Kind: KindSource, Inputs: []string{"b"}, Function: FuncIdentity, OutputType: "float64"},
			},
			wantErr: "must not have inputs",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newTestEngine()
			var err error
			for _, n := range tt.nodes {
				if err = e.AddNode(n); err != nil {
					break
				}
			}
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			} else {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("expected error containing %q, got %q", tt.wantErr, err.Error())
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestCycleDetection
// ---------------------------------------------------------------------------

func TestCycleDetection(t *testing.T) {
	e := newTestEngine()
	mustAddNode(t, e, FeatureNode{Name: "a", Kind: KindSource, Function: FuncIdentity, OutputType: "float64"})
	mustAddNode(t, e, FeatureNode{Name: "b", Kind: KindDerived, Inputs: []string{"a"}, Function: FuncIdentity, OutputType: "float64"})
	mustAddNode(t, e, FeatureNode{Name: "c", Kind: KindDerived, Inputs: []string{"b"}, Function: FuncIdentity, OutputType: "float64"})

	// d depends on c, and also tries to feed back into a via a new node—but because a is a source
	// that is already registered, we cannot add a node that would form a cycle.
	// Instead, try adding a node "d" that depends on c and then try to add a node that
	// forms a real cycle: e.g., re-adding "a" with input "c" (won't work because a already exists).
	// We craft a cycle by having d -> c -> b -> a, then adding e that inputs d and is also an input of b.
	// Simpler: add d that depends on c. Then add e that inputs d. Then try adding f that inputs e
	// and is also used by b—but b already exists. So we test by creating a fresh graph.
	e2 := newTestEngine()
	mustAddNode(t, e2, FeatureNode{Name: "x", Kind: KindSource, Function: FuncIdentity, OutputType: "float64"})
	mustAddNode(t, e2, FeatureNode{Name: "y", Kind: KindDerived, Inputs: []string{"x"}, Function: FuncIdentity, OutputType: "float64"})

	// Try to add z that depends on y, and then a node that creates a cycle through z.
	mustAddNode(t, e2, FeatureNode{Name: "z", Kind: KindDerived, Inputs: []string{"y"}, Function: FuncIdentity, OutputType: "float64"})

	// Now in a separate engine, test indirect cycle: a -> b -> c -> a
	e3 := newTestEngine()
	mustAddNode(t, e3, FeatureNode{Name: "a", Kind: KindSource, Function: FuncIdentity, OutputType: "float64"})
	mustAddNode(t, e3, FeatureNode{Name: "b", Kind: KindDerived, Inputs: []string{"a"}, Function: FuncIdentity, OutputType: "float64"})
	mustAddNode(t, e3, FeatureNode{Name: "c", Kind: KindDerived, Inputs: []string{"b"}, Function: FuncIdentity, OutputType: "float64"})

	// Try to add d that depends on c AND has a as input which would be fine (no cycle)
	mustAddNode(t, e3, FeatureNode{Name: "d", Kind: KindDerived, Inputs: []string{"c", "a"}, Function: FuncSum, OutputType: "float64"})

	// Direct self-reference
	err := e3.AddNode(FeatureNode{Name: "selfref", Kind: KindDerived, Inputs: []string{"selfref"}, Function: FuncSum, OutputType: "float64"})
	if err == nil || !strings.Contains(err.Error(), "references itself") {
		t.Fatalf("expected self-reference error, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// TestRemoveNode
// ---------------------------------------------------------------------------

func TestRemoveNode(t *testing.T) {
	e := newTestEngine()
	mustAddNode(t, e, FeatureNode{Name: "a", Kind: KindSource, Function: FuncIdentity, OutputType: "float64"})
	mustAddNode(t, e, FeatureNode{Name: "b", Kind: KindDerived, Inputs: []string{"a"}, Function: FuncIdentity, OutputType: "float64"})

	// Cannot remove a because b depends on it.
	if err := e.RemoveNode("a"); err == nil {
		t.Fatal("expected error removing node with dependents")
	}

	// Can remove b (leaf).
	if err := e.RemoveNode("b"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Now a can be removed.
	if err := e.RemoveNode("a"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Removing non-existent node.
	if err := e.RemoveNode("nope"); err == nil {
		t.Fatal("expected error removing non-existent node")
	}
}

// ---------------------------------------------------------------------------
// TestTopologicalSort
// ---------------------------------------------------------------------------

func TestTopologicalSort(t *testing.T) {
	e := newTestEngine()
	mustAddNode(t, e, FeatureNode{Name: "a", Kind: KindSource, Function: FuncIdentity, OutputType: "float64"})
	mustAddNode(t, e, FeatureNode{Name: "b", Kind: KindSource, Function: FuncIdentity, OutputType: "float64"})
	mustAddNode(t, e, FeatureNode{Name: "c", Kind: KindDerived, Inputs: []string{"a", "b"}, Function: FuncSum, OutputType: "float64"})
	mustAddNode(t, e, FeatureNode{Name: "d", Kind: KindDerived, Inputs: []string{"c"}, Function: FuncIdentity, OutputType: "float64"})

	order, err := e.TopologicalSort()
	if err != nil {
		t.Fatalf("TopologicalSort: %v", err)
	}

	if len(order.Order) != 4 {
		t.Fatalf("expected 4 nodes in order, got %d", len(order.Order))
	}

	// a and b must appear before c; c before d.
	idx := make(map[string]int)
	for i, n := range order.Order {
		idx[n] = i
	}

	if idx["a"] >= idx["c"] || idx["b"] >= idx["c"] {
		t.Fatalf("a and b should precede c: %v", order.Order)
	}
	if idx["c"] >= idx["d"] {
		t.Fatalf("c should precede d: %v", order.Order)
	}

	// Levels: level 0 = {a, b}, level 1 = {c}, level 2 = {d}
	if len(order.Levels) != 3 {
		t.Fatalf("expected 3 levels, got %d", len(order.Levels))
	}
}

// ---------------------------------------------------------------------------
// TestCompute
// ---------------------------------------------------------------------------

func TestCompute(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(*Engine)
		target   string
		inputs   map[string]interface{}
		want     interface{}
		wantErr  string
	}{
		{
			name: "identity",
			setup: func(e *Engine) {
				mustAddNodeE(e, FeatureNode{Name: "x", Kind: KindSource, Function: FuncIdentity, OutputType: "float64"})
				mustAddNodeE(e, FeatureNode{Name: "y", Kind: KindDerived, Inputs: []string{"x"}, Function: FuncIdentity, OutputType: "float64"})
			},
			target: "y",
			inputs: map[string]interface{}{"x": 42.0},
			want:   42.0,
		},
		{
			name: "sum",
			setup: func(e *Engine) {
				mustAddNodeE(e, FeatureNode{Name: "a", Kind: KindSource, Function: FuncIdentity, OutputType: "float64"})
				mustAddNodeE(e, FeatureNode{Name: "b", Kind: KindSource, Function: FuncIdentity, OutputType: "float64"})
				mustAddNodeE(e, FeatureNode{Name: "total", Kind: KindDerived, Inputs: []string{"a", "b"}, Function: FuncSum, OutputType: "float64"})
			},
			target: "total",
			inputs: map[string]interface{}{"a": 10.0, "b": 20.0},
			want:   30.0,
		},
		{
			name: "avg",
			setup: func(e *Engine) {
				mustAddNodeE(e, FeatureNode{Name: "a", Kind: KindSource, Function: FuncIdentity, OutputType: "float64"})
				mustAddNodeE(e, FeatureNode{Name: "b", Kind: KindSource, Function: FuncIdentity, OutputType: "float64"})
				mustAddNodeE(e, FeatureNode{Name: "mean", Kind: KindAggregated, Inputs: []string{"a", "b"}, Function: FuncAvg, OutputType: "float64"})
			},
			target: "mean",
			inputs: map[string]interface{}{"a": 10.0, "b": 30.0},
			want:   20.0,
		},
		{
			name: "multiply",
			setup: func(e *Engine) {
				mustAddNodeE(e, FeatureNode{Name: "a", Kind: KindSource, Function: FuncIdentity, OutputType: "float64"})
				mustAddNodeE(e, FeatureNode{Name: "b", Kind: KindSource, Function: FuncIdentity, OutputType: "float64"})
				mustAddNodeE(e, FeatureNode{Name: "prod", Kind: KindDerived, Inputs: []string{"a", "b"}, Function: FuncMultiply, OutputType: "float64"})
			},
			target: "prod",
			inputs: map[string]interface{}{"a": 3.0, "b": 7.0},
			want:   21.0,
		},
		{
			name: "divide",
			setup: func(e *Engine) {
				mustAddNodeE(e, FeatureNode{Name: "num", Kind: KindSource, Function: FuncIdentity, OutputType: "float64"})
				mustAddNodeE(e, FeatureNode{Name: "den", Kind: KindSource, Function: FuncIdentity, OutputType: "float64"})
				mustAddNodeE(e, FeatureNode{Name: "ratio", Kind: KindDerived, Inputs: []string{"num", "den"}, Function: FuncDivide, OutputType: "float64"})
			},
			target: "ratio",
			inputs: map[string]interface{}{"num": 10.0, "den": 4.0},
			want:   2.5,
		},
		{
			name: "divide by zero",
			setup: func(e *Engine) {
				mustAddNodeE(e, FeatureNode{Name: "num", Kind: KindSource, Function: FuncIdentity, OutputType: "float64"})
				mustAddNodeE(e, FeatureNode{Name: "den", Kind: KindSource, Function: FuncIdentity, OutputType: "float64"})
				mustAddNodeE(e, FeatureNode{Name: "ratio", Kind: KindDerived, Inputs: []string{"num", "den"}, Function: FuncDivide, OutputType: "float64"})
			},
			target:  "ratio",
			inputs:  map[string]interface{}{"num": 10.0, "den": 0.0},
			wantErr: "division by zero",
		},
		{
			name: "concat",
			setup: func(e *Engine) {
				mustAddNodeE(e, FeatureNode{Name: "first", Kind: KindSource, Function: FuncIdentity, OutputType: "string"})
				mustAddNodeE(e, FeatureNode{Name: "last", Kind: KindSource, Function: FuncIdentity, OutputType: "string"})
				mustAddNodeE(e, FeatureNode{Name: "full", Kind: KindDerived, Inputs: []string{"first", "last"}, Function: FuncConcat, OutputType: "string"})
			},
			target: "full",
			inputs: map[string]interface{}{"first": "hello", "last": "world"},
			want:   "helloworld",
		},
		{
			name: "coalesce returns first non-nil",
			setup: func(e *Engine) {
				mustAddNodeE(e, FeatureNode{Name: "a", Kind: KindSource, Function: FuncIdentity, OutputType: "float64"})
				mustAddNodeE(e, FeatureNode{Name: "b", Kind: KindSource, Function: FuncIdentity, OutputType: "float64"})
				mustAddNodeE(e, FeatureNode{Name: "c", Kind: KindDerived, Inputs: []string{"a", "b"}, Function: FuncCoalesce, OutputType: "float64"})
			},
			target: "c",
			inputs: map[string]interface{}{"a": nil, "b": 5.0},
			want:   5.0,
		},
		{
			name: "multi-level DAG",
			setup: func(e *Engine) {
				mustAddNodeE(e, FeatureNode{Name: "x", Kind: KindSource, Function: FuncIdentity, OutputType: "float64"})
				mustAddNodeE(e, FeatureNode{Name: "y", Kind: KindSource, Function: FuncIdentity, OutputType: "float64"})
				mustAddNodeE(e, FeatureNode{Name: "sum_xy", Kind: KindDerived, Inputs: []string{"x", "y"}, Function: FuncSum, OutputType: "float64"})
				mustAddNodeE(e, FeatureNode{Name: "z", Kind: KindSource, Function: FuncIdentity, OutputType: "float64"})
				mustAddNodeE(e, FeatureNode{Name: "result", Kind: KindDerived, Inputs: []string{"sum_xy", "z"}, Function: FuncMultiply, OutputType: "float64"})
			},
			target: "result",
			inputs: map[string]interface{}{"x": 2.0, "y": 3.0, "z": 4.0},
			want:   20.0, // (2+3)*4
		},
		{
			name: "missing source input",
			setup: func(e *Engine) {
				mustAddNodeE(e, FeatureNode{Name: "a", Kind: KindSource, Function: FuncIdentity, OutputType: "float64"})
				mustAddNodeE(e, FeatureNode{Name: "b", Kind: KindDerived, Inputs: []string{"a"}, Function: FuncIdentity, OutputType: "float64"})
			},
			target:  "b",
			inputs:  map[string]interface{}{},
			wantErr: "missing input",
		},
		{
			name:    "node not found",
			setup:   func(e *Engine) {},
			target:  "nonexistent",
			inputs:  map[string]interface{}{},
			wantErr: "not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newTestEngine()
			tt.setup(e)

			result, err := e.Compute(tt.target, tt.inputs)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("expected error containing %q, got %q", tt.wantErr, err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.Value != tt.want {
				t.Fatalf("got %v, want %v", result.Value, tt.want)
			}
		})
	}
}

func mustAddNodeE(e *Engine, node FeatureNode) {
	if err := e.AddNode(node); err != nil {
		panic(err)
	}
}

// ---------------------------------------------------------------------------
// TestInvalidate
// ---------------------------------------------------------------------------

func TestInvalidate(t *testing.T) {
	e := newTestEngine()
	mustAddNode(t, e, FeatureNode{Name: "a", Kind: KindSource, Function: FuncIdentity, OutputType: "float64"})
	mustAddNode(t, e, FeatureNode{Name: "b", Kind: KindDerived, Inputs: []string{"a"}, Function: FuncIdentity, OutputType: "float64"})
	mustAddNode(t, e, FeatureNode{Name: "c", Kind: KindDerived, Inputs: []string{"b"}, Function: FuncIdentity, OutputType: "float64"})
	mustAddNode(t, e, FeatureNode{Name: "d", Kind: KindSource, Function: FuncIdentity, OutputType: "float64"})

	// Compute to populate cache.
	if _, err := e.Compute("c", map[string]interface{}{"a": 1.0}); err != nil {
		t.Fatal(err)
	}

	// Invalidate a → should dirty a, b, c (3 nodes).
	count := e.Invalidate("a")
	if count != 3 {
		t.Fatalf("expected 3 invalidated nodes, got %d", count)
	}

	// d is independent, should not be dirty.
	stats := e.Stats()
	if stats.DirtyNodes != 3 {
		t.Fatalf("expected 3 dirty nodes, got %d", stats.DirtyNodes)
	}

	// Invalidating non-existent node returns 0.
	if n := e.Invalidate("nope"); n != 0 {
		t.Fatalf("expected 0 for non-existent node, got %d", n)
	}
}

// ---------------------------------------------------------------------------
// TestUpstreamDownstream
// ---------------------------------------------------------------------------

func TestUpstreamDownstream(t *testing.T) {
	e := newTestEngine()
	mustAddNode(t, e, FeatureNode{Name: "a", Kind: KindSource, Function: FuncIdentity, OutputType: "float64"})
	mustAddNode(t, e, FeatureNode{Name: "b", Kind: KindSource, Function: FuncIdentity, OutputType: "float64"})
	mustAddNode(t, e, FeatureNode{Name: "c", Kind: KindDerived, Inputs: []string{"a", "b"}, Function: FuncSum, OutputType: "float64"})
	mustAddNode(t, e, FeatureNode{Name: "d", Kind: KindDerived, Inputs: []string{"c"}, Function: FuncIdentity, OutputType: "float64"})

	upstream, err := e.GetUpstream("d")
	if err != nil {
		t.Fatal(err)
	}
	// d -> c -> {a, b}
	if len(upstream) != 3 {
		t.Fatalf("expected 3 upstream nodes, got %d: %v", len(upstream), upstream)
	}

	downstream, err := e.GetDownstream("a")
	if err != nil {
		t.Fatal(err)
	}
	// a -> c -> d
	if len(downstream) != 2 {
		t.Fatalf("expected 2 downstream nodes, got %d: %v", len(downstream), downstream)
	}

	// Error for non-existent node.
	if _, err := e.GetUpstream("nope"); err == nil {
		t.Fatal("expected error for non-existent node")
	}
	if _, err := e.GetDownstream("nope"); err == nil {
		t.Fatal("expected error for non-existent node")
	}
}

// ---------------------------------------------------------------------------
// TestValidation
// ---------------------------------------------------------------------------

func TestValidation(t *testing.T) {
	e := NewEngine(DefaultEngineConfig())

	// Add a node with missing output_type via direct manipulation to test Validate.
	e.mu.Lock()
	e.nodes["bad"] = &FeatureNode{Name: "bad", Kind: KindSource}
	e.mu.Unlock()

	errs := e.Validate()
	if len(errs) == 0 {
		t.Fatal("expected validation errors")
	}

	found := false
	for _, ve := range errs {
		if ve.Node == "bad" && strings.Contains(ve.Message, "output_type") {
			found = true
		}
	}
	if !found {
		t.Fatal("expected output_type validation error for node 'bad'")
	}
}

// ---------------------------------------------------------------------------
// TestStats
// ---------------------------------------------------------------------------

func TestStats(t *testing.T) {
	e := newTestEngine()
	mustAddNode(t, e, FeatureNode{Name: "a", Kind: KindSource, Function: FuncIdentity, OutputType: "float64", Policy: PolicyEager})
	mustAddNode(t, e, FeatureNode{Name: "b", Kind: KindSource, Function: FuncIdentity, OutputType: "float64", Policy: PolicyLazy})
	mustAddNode(t, e, FeatureNode{Name: "c", Kind: KindDerived, Inputs: []string{"a", "b"}, Function: FuncSum, OutputType: "float64", Policy: PolicyScheduled})

	stats := e.Stats()
	if stats.TotalNodes != 3 {
		t.Fatalf("expected 3 total nodes, got %d", stats.TotalNodes)
	}
	if stats.SourceNodes != 2 {
		t.Fatalf("expected 2 source nodes, got %d", stats.SourceNodes)
	}
	if stats.DerivedNodes != 1 {
		t.Fatalf("expected 1 derived node, got %d", stats.DerivedNodes)
	}
	if stats.TotalEdges != 2 {
		t.Fatalf("expected 2 edges, got %d", stats.TotalEdges)
	}
	if stats.MaxDepth != 1 {
		t.Fatalf("expected max depth 1, got %d", stats.MaxDepth)
	}
	if stats.ByPolicy[string(PolicyEager)] != 1 {
		t.Fatalf("expected 1 eager node, got %d", stats.ByPolicy[string(PolicyEager)])
	}
	if stats.ByPolicy[string(PolicyLazy)] != 1 {
		t.Fatalf("expected 1 lazy node, got %d", stats.ByPolicy[string(PolicyLazy)])
	}
	if stats.ByPolicy[string(PolicyScheduled)] != 1 {
		t.Fatalf("expected 1 scheduled node, got %d", stats.ByPolicy[string(PolicyScheduled)])
	}
}

// ---------------------------------------------------------------------------
// TestGetDAG
// ---------------------------------------------------------------------------

func TestGetDAG(t *testing.T) {
	e := newTestEngine()
	mustAddNode(t, e, FeatureNode{Name: "a", Kind: KindSource, Function: FuncIdentity, OutputType: "float64"})
	mustAddNode(t, e, FeatureNode{Name: "b", Kind: KindSource, Function: FuncIdentity, OutputType: "float64"})
	mustAddNode(t, e, FeatureNode{Name: "c", Kind: KindDerived, Inputs: []string{"a", "b"}, Function: FuncSum, OutputType: "float64"})

	dag := e.GetDAG()
	if len(dag.Nodes) != 3 {
		t.Fatalf("expected 3 nodes, got %d", len(dag.Nodes))
	}
	if len(dag.Edges) != 2 {
		t.Fatalf("expected 2 edges, got %d", len(dag.Edges))
	}
	if dag.RootCount != 2 {
		t.Fatalf("expected 2 roots, got %d", dag.RootCount)
	}
	if dag.LeafCount != 1 {
		t.Fatalf("expected 1 leaf, got %d", dag.LeafCount)
	}
	if dag.Depth != 1 {
		t.Fatalf("expected depth 1, got %d", dag.Depth)
	}
	if !dag.IsValid {
		t.Fatal("expected valid DAG")
	}
}

// ---------------------------------------------------------------------------
// TestGetNode and TestListNodes
// ---------------------------------------------------------------------------

func TestGetNodeAndListNodes(t *testing.T) {
	e := newTestEngine()
	mustAddNode(t, e, FeatureNode{Name: "z", Kind: KindSource, Function: FuncIdentity, OutputType: "float64"})
	mustAddNode(t, e, FeatureNode{Name: "a", Kind: KindSource, Function: FuncIdentity, OutputType: "float64"})

	node, err := e.GetNode("a")
	if err != nil {
		t.Fatal(err)
	}
	if node.Name != "a" {
		t.Fatalf("expected node name 'a', got %q", node.Name)
	}

	if _, err := e.GetNode("nope"); err == nil {
		t.Fatal("expected error for non-existent node")
	}

	nodes := e.ListNodes()
	if len(nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(nodes))
	}
	// Should be sorted.
	if nodes[0].Name != "a" || nodes[1].Name != "z" {
		t.Fatalf("expected sorted [a, z], got [%s, %s]", nodes[0].Name, nodes[1].Name)
	}
}

// ---------------------------------------------------------------------------
// TestDefaultPolicy
// ---------------------------------------------------------------------------

func TestDefaultPolicy(t *testing.T) {
	e := newTestEngine()
	mustAddNode(t, e, FeatureNode{Name: "a", Kind: KindSource, Function: FuncIdentity, OutputType: "float64"})

	node, _ := e.GetNode("a")
	if node.Policy != PolicyLazy {
		t.Fatalf("expected default policy %q, got %q", PolicyLazy, node.Policy)
	}
}

// ---------------------------------------------------------------------------
// TestMaxNodesLimit
// ---------------------------------------------------------------------------

func TestMaxNodesLimit(t *testing.T) {
	cfg := DefaultEngineConfig()
	cfg.MaxNodes = 2
	e := NewEngine(cfg)

	mustAddNode(t, e, FeatureNode{Name: "a", Kind: KindSource, Function: FuncIdentity, OutputType: "float64"})
	mustAddNode(t, e, FeatureNode{Name: "b", Kind: KindSource, Function: FuncIdentity, OutputType: "float64"})

	err := e.AddNode(FeatureNode{Name: "c", Kind: KindSource, Function: FuncIdentity, OutputType: "float64"})
	if err == nil || !strings.Contains(err.Error(), "max nodes") {
		t.Fatalf("expected max nodes error, got %v", err)
	}
}
