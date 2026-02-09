package computegraph

import (
	"strings"
	"testing"
)

func TestParseDSL_Basic(t *testing.T) {
	input := `
GRAPH test_pipeline
  SOURCE price AS float64
  SOURCE quantity AS float64
  DERIVE total FROM price, quantity USING multiply AS float64
  DERIVE avg_total FROM total USING avg AS float64 POLICY lazy
END
`
	def, err := ParseDSL(input)
	if err != nil {
		t.Fatalf("ParseDSL: %v", err)
	}

	if def.Name != "test_pipeline" {
		t.Fatalf("expected graph name 'test_pipeline', got %q", def.Name)
	}
	if len(def.Nodes) != 4 {
		t.Fatalf("expected 4 nodes, got %d", len(def.Nodes))
	}

	// Verify source nodes
	if def.Nodes[0].Kind != "source" || def.Nodes[0].Name != "price" {
		t.Fatalf("expected source node 'price', got %+v", def.Nodes[0])
	}
	if def.Nodes[1].Kind != "source" || def.Nodes[1].Name != "quantity" {
		t.Fatalf("expected source node 'quantity', got %+v", def.Nodes[1])
	}

	// Verify derived nodes
	if def.Nodes[2].Name != "total" || def.Nodes[2].Function != "multiply" {
		t.Fatalf("expected derived node 'total' with multiply, got %+v", def.Nodes[2])
	}
	if len(def.Nodes[2].Inputs) != 2 {
		t.Fatalf("expected 2 inputs for total, got %d", len(def.Nodes[2].Inputs))
	}

	if def.Nodes[3].Policy != "lazy" {
		t.Fatalf("expected policy 'lazy', got %q", def.Nodes[3].Policy)
	}
}

func TestParseDSL_Comments(t *testing.T) {
	input := `
# This is a comment
GRAPH demo
  -- Another comment
  SOURCE x AS float64
END
`
	def, err := ParseDSL(input)
	if err != nil {
		t.Fatalf("ParseDSL: %v", err)
	}
	if len(def.Nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(def.Nodes))
	}
}

func TestParseDSL_Errors(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr string
	}{
		{
			name:    "missing graph name",
			input:   "GRAPH\nEND",
			wantErr: "requires a name",
		},
		{
			name:    "unterminated graph",
			input:   "GRAPH test\n  SOURCE x AS float64",
			wantErr: "unterminated",
		},
		{
			name:    "bad source syntax",
			input:   "GRAPH test\n  SOURCE\nEND",
			wantErr: "SOURCE syntax",
		},
		{
			name:    "bad derive syntax",
			input:   "GRAPH test\n  DERIVE x\nEND",
			wantErr: "DERIVE syntax",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseDSL(tt.input)
			if err == nil {
				t.Fatalf("expected error containing %q", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error containing %q, got %q", tt.wantErr, err.Error())
			}
		})
	}
}

func TestApplyDefinition(t *testing.T) {
	e := newTestEngine()
	def := &GraphDefinition{
		Name: "test",
		Nodes: []NodeDefinition{
			{Name: "a", Kind: "source", Function: "identity", OutputType: "float64"},
			{Name: "b", Kind: "source", Function: "identity", OutputType: "float64"},
			{Name: "c", Kind: "derived", Inputs: []string{"a", "b"}, Function: "sum", OutputType: "float64"},
		},
	}

	result, err := e.ApplyDefinition(def)
	if err != nil {
		t.Fatalf("ApplyDefinition: %v", err)
	}
	if !result.Success {
		t.Fatalf("expected success, got errors: %v", result.Errors)
	}
	if len(result.NodesAdded) != 3 {
		t.Fatalf("expected 3 nodes added, got %d", len(result.NodesAdded))
	}

	// Verify nodes work
	res, err := e.Compute("c", map[string]interface{}{"a": 10.0, "b": 20.0})
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	if res.Value != 30.0 {
		t.Fatalf("expected 30.0, got %v", res.Value)
	}
}

func TestApplyDefinition_NilDef(t *testing.T) {
	e := newTestEngine()
	_, err := e.ApplyDefinition(nil)
	if err == nil {
		t.Fatal("expected error for nil definition")
	}
}

func TestParseDSLAndApply(t *testing.T) {
	dsl := `
GRAPH revenue
  SOURCE price AS float64
  SOURCE qty AS float64
  DERIVE revenue FROM price, qty USING multiply AS float64
END
`
	def, err := ParseDSL(dsl)
	if err != nil {
		t.Fatalf("ParseDSL: %v", err)
	}

	e := newTestEngine()
	result, err := e.ApplyDefinition(def)
	if err != nil {
		t.Fatalf("ApplyDefinition: %v", err)
	}
	if !result.Success {
		t.Fatalf("expected success, got errors: %v", result.Errors)
	}

	res, err := e.Compute("revenue", map[string]interface{}{"price": 9.99, "qty": 3.0})
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	if res.Value != 29.97 {
		t.Fatalf("expected 29.97, got %v", res.Value)
	}
}
