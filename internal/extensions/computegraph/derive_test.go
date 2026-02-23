package computegraph

import (
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// TestParseDeriveStatement
// ---------------------------------------------------------------------------

func TestParseDeriveStatement(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantName   string
		wantExpr   string
		wantInputs []string
		wantFunc   ComputeFunc
		wantPolicy MaterializePolicy
		wantWindow *WindowSpec
		wantErr    string
	}{
		{
			name:       "basic derive",
			input:      "DERIVE total AS price * quantity FROM price, quantity",
			wantName:   "total",
			wantExpr:   "price * quantity",
			wantInputs: []string{"price", "quantity"},
			wantFunc:   FuncIdentity,
		},
		{
			name:       "derive with USING",
			input:      "DERIVE total AS price * quantity FROM price, quantity USING multiply",
			wantName:   "total",
			wantExpr:   "price * quantity",
			wantInputs: []string{"price", "quantity"},
			wantFunc:   FuncMultiply,
		},
		{
			name:       "derive with POLICY",
			input:      "DERIVE avg_price AS avg(price) FROM price USING avg POLICY eager",
			wantName:   "avg_price",
			wantExpr:   "avg(price)",
			wantInputs: []string{"price"},
			wantFunc:   FuncAvg,
			wantPolicy: PolicyEager,
		},
		{
			name:       "derive with tumbling window",
			input:      "DERIVE count_5m AS count(events) FROM events USING sum WINDOW tumbling 5m",
			wantName:   "count_5m",
			wantExpr:   "count(events)",
			wantInputs: []string{"events"},
			wantFunc:   FuncSum,
			wantWindow: &WindowSpec{Type: "tumbling", Duration: 5 * time.Minute},
		},
		{
			name:       "derive with sliding window",
			input:      "DERIVE moving_avg AS avg(price) FROM price USING avg WINDOW sliding 10m SLIDE 1m",
			wantName:   "moving_avg",
			wantExpr:   "avg(price)",
			wantInputs: []string{"price"},
			wantFunc:   FuncAvg,
			wantWindow: &WindowSpec{Type: "sliding", Duration: 10 * time.Minute, Slide: 1 * time.Minute},
		},
		{
			name:    "missing DERIVE keyword",
			input:   "SELECT total FROM price",
			wantErr: "must start with DERIVE",
		},
		{
			name:    "missing AS clause",
			input:   "DERIVE total FROM price",
			wantErr: "expected AS and FROM",
		},
		{
			name:    "missing FROM clause",
			input:   "DERIVE total AS expr",
			wantErr: "expected AS and FROM",
		},
		{
			name:    "empty expression",
			input:   "DERIVE total AS FROM price",
			wantErr: "empty expression",
		},
		{
			name:    "no inputs after FROM",
			input:   "DERIVE total AS expr FROM USING sum",
			wantErr: "at least one input",
		},
		{
			name:    "invalid window type",
			input:   "DERIVE x AS expr FROM a WINDOW hopping 5m",
			wantErr: "unknown window type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec, err := ParseDeriveStatement(tt.input)

			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error %q does not contain %q", err.Error(), tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if spec.Name != tt.wantName {
				t.Errorf("name = %q, want %q", spec.Name, tt.wantName)
			}
			if spec.Expression != tt.wantExpr {
				t.Errorf("expression = %q, want %q", spec.Expression, tt.wantExpr)
			}
			if len(spec.Inputs) != len(tt.wantInputs) {
				t.Fatalf("inputs = %v, want %v", spec.Inputs, tt.wantInputs)
			}
			for i, inp := range spec.Inputs {
				if inp != tt.wantInputs[i] {
					t.Errorf("input[%d] = %q, want %q", i, inp, tt.wantInputs[i])
				}
			}
			if spec.Function != tt.wantFunc {
				t.Errorf("function = %q, want %q", spec.Function, tt.wantFunc)
			}
			if tt.wantPolicy != "" && spec.Policy != tt.wantPolicy {
				t.Errorf("policy = %q, want %q", spec.Policy, tt.wantPolicy)
			}
			if tt.wantWindow != nil {
				if spec.Window == nil {
					t.Fatal("expected window spec, got nil")
				}
				if spec.Window.Type != tt.wantWindow.Type {
					t.Errorf("window type = %q, want %q", spec.Window.Type, tt.wantWindow.Type)
				}
				if spec.Window.Duration != tt.wantWindow.Duration {
					t.Errorf("window duration = %v, want %v", spec.Window.Duration, tt.wantWindow.Duration)
				}
				if spec.Window.Slide != tt.wantWindow.Slide {
					t.Errorf("window slide = %v, want %v", spec.Window.Slide, tt.wantWindow.Slide)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestBuildDeriveGraph
// ---------------------------------------------------------------------------

func TestBuildDeriveGraph(t *testing.T) {
	tests := []struct {
		name       string
		setup      func(*Engine)
		specs      []DeriveSpec
		wantAdded  []string
		wantErrors int
	}{
		{
			name: "single derive from source",
			setup: func(e *Engine) {
				mustAddNode(t, e, FeatureNode{Name: "price", Kind: KindSource, Function: FuncIdentity, OutputType: "float64"})
			},
			specs: []DeriveSpec{
				{Name: "double_price", Expression: "price * 2", Inputs: []string{"price"}, Function: FuncMultiply, OutputType: "float64"},
			},
			wantAdded: []string{"double_price"},
		},
		{
			name: "chain of derived nodes",
			setup: func(e *Engine) {
				mustAddNode(t, e, FeatureNode{Name: "a", Kind: KindSource, Function: FuncIdentity, OutputType: "float64"})
				mustAddNode(t, e, FeatureNode{Name: "b", Kind: KindSource, Function: FuncIdentity, OutputType: "float64"})
			},
			specs: []DeriveSpec{
				{Name: "c", Expression: "a + b", Inputs: []string{"a", "b"}, Function: FuncSum, OutputType: "float64"},
				{Name: "d", Expression: "c * 2", Inputs: []string{"c"}, Function: FuncMultiply, OutputType: "float64"},
			},
			wantAdded: []string{"c", "d"},
		},
		{
			name:  "missing input produces error",
			setup: func(e *Engine) {},
			specs: []DeriveSpec{
				{Name: "x", Expression: "missing + 1", Inputs: []string{"missing"}, Function: FuncSum, OutputType: "float64"},
			},
			wantAdded:  nil,
			wantErrors: 1,
		},
		{
			name:  "empty name produces error",
			setup: func(e *Engine) {},
			specs: []DeriveSpec{
				{Name: "", Expression: "expr", Inputs: []string{"a"}, Function: FuncSum, OutputType: "float64"},
			},
			wantAdded:  nil,
			wantErrors: 1,
		},
		{
			name: "derive with window metadata",
			setup: func(e *Engine) {
				mustAddNode(t, e, FeatureNode{Name: "events", Kind: KindSource, Function: FuncIdentity, OutputType: "float64"})
			},
			specs: []DeriveSpec{
				{
					Name:       "event_count",
					Expression: "count(events)",
					Inputs:     []string{"events"},
					Function:   FuncSum,
					OutputType: "float64",
					Window:     &WindowSpec{Type: "tumbling", Duration: 5 * time.Minute},
				},
			},
			wantAdded: []string{"event_count"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newTestEngine()
			tt.setup(e)

			result, err := BuildDeriveGraph(e, tt.specs)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(result.NodesAdded) != len(tt.wantAdded) {
				t.Fatalf("added %v, want %v", result.NodesAdded, tt.wantAdded)
			}
			for i, name := range result.NodesAdded {
				if name != tt.wantAdded[i] {
					t.Errorf("added[%d] = %q, want %q", i, name, tt.wantAdded[i])
				}
			}
			if len(result.Errors) != tt.wantErrors {
				t.Fatalf("got %d errors %v, want %d", len(result.Errors), result.Errors, tt.wantErrors)
			}
		})
	}
}

func TestBuildDeriveGraph_NilEngine(t *testing.T) {
	_, err := BuildDeriveGraph(nil, []DeriveSpec{})
	if err == nil || !strings.Contains(err.Error(), "nil engine") {
		t.Fatalf("expected nil engine error, got %v", err)
	}
}

func TestBuildDeriveGraph_WindowMetadata(t *testing.T) {
	e := newTestEngine()
	mustAddNode(t, e, FeatureNode{Name: "src", Kind: KindSource, Function: FuncIdentity, OutputType: "float64"})

	specs := []DeriveSpec{
		{
			Name:       "windowed",
			Expression: "sum(src)",
			Inputs:     []string{"src"},
			Function:   FuncSum,
			OutputType: "float64",
			Window:     &WindowSpec{Type: "sliding", Duration: 10 * time.Minute, Slide: 1 * time.Minute},
		},
	}

	result, err := BuildDeriveGraph(e, specs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Fatalf("expected success, got errors: %v", result.Errors)
	}

	node, err := e.GetNode("windowed")
	if err != nil {
		t.Fatalf("GetNode: %v", err)
	}
	if node.Metadata["window_type"] != "sliding" {
		t.Errorf("window_type = %q, want 'sliding'", node.Metadata["window_type"])
	}
	if node.Metadata["window_duration"] != "10m0s" {
		t.Errorf("window_duration = %q, want '10m0s'", node.Metadata["window_duration"])
	}
	if node.Metadata["window_slide"] != "1m0s" {
		t.Errorf("window_slide = %q, want '1m0s'", node.Metadata["window_slide"])
	}
}

func TestDeriveSpecFromDSLNode(t *testing.T) {
	nd := NodeDefinition{
		Name:       "total",
		Kind:       "derived",
		Inputs:     []string{"price", "quantity"},
		Function:   "multiply",
		Expression: "price * quantity",
		OutputType: "float64",
		Policy:     "eager",
	}

	spec := DeriveSpecFromDSLNode(nd)

	if spec.Name != "total" {
		t.Errorf("name = %q, want 'total'", spec.Name)
	}
	if spec.Function != FuncMultiply {
		t.Errorf("function = %q, want %q", spec.Function, FuncMultiply)
	}
	if spec.Policy != PolicyEager {
		t.Errorf("policy = %q, want %q", spec.Policy, PolicyEager)
	}
	if len(spec.Inputs) != 2 {
		t.Errorf("inputs = %v, want [price quantity]", spec.Inputs)
	}
}
