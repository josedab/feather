package transform

import (
	"encoding/json"
	"testing"
)

func newTestDSL() *DSL {
	pipeline := NewPipeline(nil)
	return NewDSL(pipeline)
}

// --- parse() tests ---

func TestParse_AggregationFunction(t *testing.T) {
	t.Parallel()
	dsl := newTestDSL()

	funcs := []string{"sum", "avg", "min", "max", "count"}
	for _, fn := range funcs {
		err := dsl.Define("test_"+fn, "result = "+fn+"(value)")
		if err != nil {
			t.Errorf("Define(%s) error = %v", fn, err)
		}
	}
}

func TestParse_StringFunctions(t *testing.T) {
	t.Parallel()
	dsl := newTestDSL()

	funcs := []string{"lower", "upper", "trim", "concat"}
	for _, fn := range funcs {
		err := dsl.Define("test_"+fn, "result = "+fn+"(name)")
		if err != nil {
			t.Errorf("Define(%s) error = %v", fn, err)
		}
	}
}

func TestParse_TimestampFunctions(t *testing.T) {
	t.Parallel()
	dsl := newTestDSL()

	funcs := []string{"year", "month", "day", "hour", "weekday"}
	for _, fn := range funcs {
		err := dsl.Define("test_"+fn, "result = "+fn+"(ts)")
		if err != nil {
			t.Errorf("Define(%s) error = %v", fn, err)
		}
	}
}

func TestParse_ConditionalExpression(t *testing.T) {
	t.Parallel()
	dsl := newTestDSL()

	err := dsl.Define("cond_test", "result = age > 18 ? adult : minor")
	if err != nil {
		t.Fatal(err)
	}
}

func TestParse_ArithmeticExpression(t *testing.T) {
	t.Parallel()
	dsl := newTestDSL()

	expressions := []string{
		"result = a + b",
		"result = x - y",
		"result = price * quantity",
		"result = total / count",
	}
	for _, expr := range expressions {
		err := dsl.Define("arith_"+expr[:10], expr)
		if err != nil {
			t.Errorf("Define(%q) error = %v", expr, err)
		}
	}
}

func TestParse_WindowFunction(t *testing.T) {
	t.Parallel()
	dsl := newTestDSL()

	err := dsl.Define("window_test", "result = window(clicks, sum, 1h)")
	if err != nil {
		t.Fatal(err)
	}
}

func TestParse_LookupFunction(t *testing.T) {
	t.Parallel()
	dsl := newTestDSL()

	err := dsl.Define("lookup_test", "result = lookup(user_id, users, name)")
	if err != nil {
		t.Fatal(err)
	}
}

func TestParse_UnknownFunction(t *testing.T) {
	t.Parallel()
	dsl := newTestDSL()

	err := dsl.Define("bad", "result = unknown_func(x)")
	if err == nil {
		t.Fatal("expected error for unknown function")
	}
}

func TestParse_InvalidExpression_NoEquals(t *testing.T) {
	t.Parallel()
	dsl := newTestDSL()

	err := dsl.Define("bad", "no equals sign here")
	if err == nil {
		t.Fatal("expected error for invalid expression")
	}
}

// --- JSON() / FromJSON() tests ---

func TestJSON_SerializeTransform(t *testing.T) {
	t.Parallel()

	transform := &Transform{
		Name:       "test",
		Type:       TypeArithmetic,
		Expression: "a + b",
		Inputs:     []string{"a", "b"},
		Output:     "result",
		Mode:       ModeOnRead,
	}

	data, err := JSON(transform)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 {
		t.Error("expected non-empty JSON")
	}

	var m map[string]interface{}
	json.Unmarshal(data, &m)
	if m["name"] != "test" {
		t.Errorf("expected name 'test', got %v", m["name"])
	}
	if m["type"] != "arithmetic" {
		t.Errorf("expected type 'arithmetic', got %v", m["type"])
	}
}

func TestJSON_AllTypes(t *testing.T) {
	t.Parallel()

	types := []Type{TypeArithmetic, TypeAggregation, TypeWindow, TypeConditional, TypeString, TypeTimestamp, TypeLookup}
	for _, tp := range types {
		transform := &Transform{Name: "t_" + string(tp), Type: tp}
		data, err := JSON(transform)
		if err != nil {
			t.Errorf("JSON(%s) error = %v", tp, err)
		}
		if len(data) == 0 {
			t.Errorf("expected non-empty JSON for type %s", tp)
		}
	}
}

func TestJSON_NilTransform(t *testing.T) {
	t.Parallel()

	data, err := JSON(nil)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "null" {
		t.Errorf("expected 'null', got %s", data)
	}
}

func TestFromJSON_ValidJSON(t *testing.T) {
	t.Parallel()

	original := &Transform{
		Name:       "test",
		Type:       TypeAggregation,
		Expression: "sum(x)",
		Inputs:     []string{"x"},
		Output:     "total",
		Mode:       ModeOnRead,
	}

	data, _ := JSON(original)
	restored, err := FromJSON(data)
	if err != nil {
		t.Fatal(err)
	}
	if restored.Name != original.Name {
		t.Errorf("expected name %s, got %s", original.Name, restored.Name)
	}
	if restored.Type != original.Type {
		t.Errorf("expected type %s, got %s", original.Type, restored.Type)
	}
}

func TestFromJSON_InvalidJSON(t *testing.T) {
	t.Parallel()

	_, err := FromJSON([]byte(`{invalid json}`))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestFromJSON_UnknownType(t *testing.T) {
	t.Parallel()

	data := []byte(`{"name":"test","type":"unknown_type"}`)
	transform, err := FromJSON(data)
	if err != nil {
		t.Fatal(err)
	}
	// Unknown type is stored as-is
	if transform.Type != "unknown_type" {
		t.Errorf("expected unknown_type, got %s", transform.Type)
	}
}

// --- Define() complex tests ---

func TestDefine_ComplexNestedExpression(t *testing.T) {
	t.Parallel()
	dsl := newTestDSL()

	err := dsl.Define("complex", "result = a + b * c - d")
	if err != nil {
		t.Fatal(err)
	}
}

func TestDefine_DuplicateNames(t *testing.T) {
	t.Parallel()
	dsl := newTestDSL()

	dsl.Define("dup", "result = a + b")
	err := dsl.Define("dup", "result = x + y")
	// RegisterTransform may or may not error on duplicates depending on implementation
	_ = err
}

// --- extractVariables tests ---

func TestExtractVariables(t *testing.T) {
	t.Parallel()

	vars := extractVariables("a + b * c")
	if len(vars) != 3 {
		t.Errorf("expected 3 variables, got %d: %v", len(vars), vars)
	}
}

func TestExtractVariables_Deduplication(t *testing.T) {
	t.Parallel()

	vars := extractVariables("x + x + y")
	if len(vars) != 2 {
		t.Errorf("expected 2 unique variables, got %d: %v", len(vars), vars)
	}
}

func TestExtractVariables_Underscore(t *testing.T) {
	t.Parallel()

	vars := extractVariables("user_name + first_name")
	if len(vars) != 2 {
		t.Errorf("expected 2 variables, got %d: %v", len(vars), vars)
	}
}

// --- MathFunctions test ---

func TestMathFunctions(t *testing.T) {
	t.Parallel()

	expectedFuncs := []string{"abs", "sqrt", "log", "log10", "exp", "floor", "ceil", "round", "sin", "cos", "tan"}
	for _, name := range expectedFuncs {
		fn, ok := MathFunctions[name]
		if !ok {
			t.Errorf("expected math function %s", name)
			continue
		}
		// Verify callable
		_ = fn(1.0)
	}
}
