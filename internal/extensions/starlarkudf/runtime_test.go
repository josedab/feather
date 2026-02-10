package starlarkudf

import (
	"math"
	"strings"
	"testing"
)

func TestRegisterAndExecute(t *testing.T) {
	reg := NewRegistry(DefaultRegistryConfig())

	udf, err := reg.Register(UDF{
		Name:       "double",
		Expression: "x*2",
		OutputType: "float64",
	})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if udf.Version != 1 {
		t.Fatalf("expected version 1, got %d", udf.Version)
	}

	result, err := reg.Execute("double", map[string]interface{}{"x": 5.0})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !result.Success {
		t.Fatalf("expected success, got error: %s", result.Error)
	}
	if result.Value != 10.0 {
		t.Fatalf("expected 10.0, got %v", result.Value)
	}
}

func TestVersioning(t *testing.T) {
	reg := NewRegistry(DefaultRegistryConfig())

	reg.Register(UDF{Name: "f", Expression: "x+1"})
	udf, _ := reg.Register(UDF{Name: "f", Expression: "x+2"})
	if udf.Version != 2 {
		t.Fatalf("expected version 2, got %d", udf.Version)
	}

	result, _ := reg.Execute("f", map[string]interface{}{"x": 10.0})
	if result.Value != 12.0 {
		t.Fatalf("expected 12.0, got %v", result.Value)
	}
}

func TestRemove(t *testing.T) {
	reg := NewRegistry(DefaultRegistryConfig())
	reg.Register(UDF{Name: "f", Expression: "x"})
	if err := reg.Remove("f"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if err := reg.Remove("f"); err == nil {
		t.Fatal("expected error removing non-existent UDF")
	}
}

func TestList(t *testing.T) {
	reg := NewRegistry(DefaultRegistryConfig())
	reg.Register(UDF{Name: "a", Expression: "x"})
	reg.Register(UDF{Name: "b", Expression: "x"})
	list := reg.List()
	if len(list) != 2 {
		t.Fatalf("expected 2 UDFs, got %d", len(list))
	}
}

func TestStats(t *testing.T) {
	reg := NewRegistry(DefaultRegistryConfig())
	reg.Register(UDF{Name: "f", Expression: "x+1"})
	reg.Execute("f", map[string]interface{}{"x": 1.0})
	reg.Execute("f", map[string]interface{}{"x": 2.0})

	stats := reg.Stats()
	if stats.TotalExecs != 2 {
		t.Fatalf("expected 2 executions, got %d", stats.TotalExecs)
	}
	if stats.TotalUDFs != 1 {
		t.Fatalf("expected 1 UDF, got %d", stats.TotalUDFs)
	}
}

func TestMaxUDFs(t *testing.T) {
	cfg := DefaultRegistryConfig()
	cfg.MaxUDFs = 1
	reg := NewRegistry(cfg)
	reg.Register(UDF{Name: "a", Expression: "x"})
	_, err := reg.Register(UDF{Name: "b", Expression: "x"})
	if err == nil {
		t.Fatal("expected max UDFs error")
	}
}

func TestValidation(t *testing.T) {
	reg := NewRegistry(DefaultRegistryConfig())
	_, err := reg.Register(UDF{Name: "", Expression: "x"})
	if err == nil {
		t.Fatal("expected error for empty name")
	}
	_, err = reg.Register(UDF{Name: "f", Expression: ""})
	if err == nil {
		t.Fatal("expected error for empty expression")
	}
}

func TestEvalExpression_Arithmetic(t *testing.T) {
	tests := []struct {
		expr string
		vars map[string]interface{}
		want float64
	}{
		{"x+y", map[string]interface{}{"x": 3.0, "y": 4.0}, 7.0},
		{"x-y", map[string]interface{}{"x": 10.0, "y": 3.0}, 7.0},
		{"x*y", map[string]interface{}{"x": 3.0, "y": 5.0}, 15.0},
		{"x/y", map[string]interface{}{"x": 10.0, "y": 4.0}, 2.5},
	}

	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			result, err := EvalExpression(tt.expr, tt.vars)
			if err != nil {
				t.Fatalf("EvalExpression: %v", err)
			}
			f, ok := result.(float64)
			if !ok {
				t.Fatalf("expected float64, got %T", result)
			}
			if math.Abs(f-tt.want) > 0.001 {
				t.Fatalf("expected %f, got %f", tt.want, f)
			}
		})
	}
}

func TestEvalExpression_Comparison(t *testing.T) {
	result, err := EvalExpression("x>y", map[string]interface{}{"x": 5.0, "y": 3.0})
	if err != nil {
		t.Fatalf("EvalExpression: %v", err)
	}
	if result != true {
		t.Fatalf("expected true, got %v", result)
	}
}

func TestEvalExpression_Ternary(t *testing.T) {
	result, err := EvalExpression("'high' if x>5 else 'low'", map[string]interface{}{"x": 10.0})
	if err != nil {
		t.Fatalf("EvalExpression: %v", err)
	}
	if result != "high" {
		t.Fatalf("expected 'high', got %v", result)
	}

	result2, err := EvalExpression("'high' if x>5 else 'low'", map[string]interface{}{"x": 2.0})
	if err != nil {
		t.Fatalf("EvalExpression: %v", err)
	}
	if result2 != "low" {
		t.Fatalf("expected 'low', got %v", result2)
	}
}

func TestEvalExpression_Functions(t *testing.T) {
	tests := []struct {
		expr string
		vars map[string]interface{}
		want float64
	}{
		{"abs(x)", map[string]interface{}{"x": -5.0}, 5.0},
		{"sqrt(x)", map[string]interface{}{"x": 16.0}, 4.0},
		{"round(x)", map[string]interface{}{"x": 3.7}, 4.0},
	}

	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			result, err := EvalExpression(tt.expr, tt.vars)
			if err != nil {
				t.Fatalf("EvalExpression: %v", err)
			}
			f, ok := result.(float64)
			if !ok {
				t.Fatalf("expected float64, got %T", result)
			}
			if math.Abs(f-tt.want) > 0.001 {
				t.Fatalf("expected %f, got %f", tt.want, f)
			}
		})
	}
}

func TestEvalExpression_StringFunctions(t *testing.T) {
	result, err := EvalExpression("upper(x)", map[string]interface{}{"x": "hello"})
	if err != nil {
		t.Fatalf("EvalExpression: %v", err)
	}
	if result != "HELLO" {
		t.Fatalf("expected 'HELLO', got %v", result)
	}
}

func TestEvalExpression_Empty(t *testing.T) {
	_, err := EvalExpression("", nil)
	if err == nil || !strings.Contains(err.Error(), "empty") {
		t.Fatalf("expected empty expression error, got %v", err)
	}
}

func TestExecuteNotFound(t *testing.T) {
	reg := NewRegistry(DefaultRegistryConfig())
	_, err := reg.Execute("nonexistent", nil)
	if err == nil {
		t.Fatal("expected error for non-existent UDF")
	}
}
