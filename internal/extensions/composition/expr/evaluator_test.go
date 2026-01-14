package expr

import (
	"math"
	"testing"
)

func TestEvaluate_NumberLiterals(t *testing.T) {
	tests := []struct {
		expr     string
		expected float64
	}{
		{"42", 42},
		{"3.14", 3.14},
		{"-5", -5},
		{"0", 0},
	}

	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			result, err := Evaluate(tt.expr, nil)
			if err != nil {
				t.Fatalf("evaluation error: %v", err)
			}
			num, ok := result.(float64)
			if !ok {
				t.Fatalf("expected float64, got %T", result)
			}
			if num != tt.expected {
				t.Errorf("got %v, want %v", num, tt.expected)
			}
		})
	}
}

func TestEvaluate_StringLiterals(t *testing.T) {
	result, err := Evaluate(`"hello world"`, nil)
	if err != nil {
		t.Fatalf("evaluation error: %v", err)
	}
	str, ok := result.(string)
	if !ok {
		t.Fatalf("expected string, got %T", result)
	}
	if str != "hello world" {
		t.Errorf("got %q, want %q", str, "hello world")
	}
}

func TestEvaluate_Variables(t *testing.T) {
	vars := map[string]interface{}{
		"x": 10.0,
		"y": 20.0,
		"s": "hello",
	}

	tests := []struct {
		expr     string
		expected interface{}
	}{
		{"x", 10.0},
		{"y", 20.0},
		{"s", "hello"},
	}

	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			result, err := Evaluate(tt.expr, vars)
			if err != nil {
				t.Fatalf("evaluation error: %v", err)
			}
			if result != tt.expected {
				t.Errorf("got %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestEvaluate_Arithmetic(t *testing.T) {
	vars := map[string]interface{}{"x": 10.0, "y": 3.0}

	tests := []struct {
		expr     string
		expected float64
	}{
		{"1 + 2", 3},
		{"5 - 3", 2},
		{"4 * 5", 20},
		{"10 / 2", 5},
		{"10 % 3", 1},
		{"x + y", 13},
		{"x - y", 7},
		{"x * y", 30},
		{"x / y", 10.0 / 3.0},
		{"2 + 3 * 4", 14}, // precedence test
		{"(2 + 3) * 4", 20},
		{"-x", -10},
	}

	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			result, err := Evaluate(tt.expr, vars)
			if err != nil {
				t.Fatalf("evaluation error: %v", err)
			}
			num, ok := result.(float64)
			if !ok {
				t.Fatalf("expected float64, got %T", result)
			}
			if math.Abs(num-tt.expected) > 0.0001 {
				t.Errorf("got %v, want %v", num, tt.expected)
			}
		})
	}
}

func TestEvaluate_Comparison(t *testing.T) {
	vars := map[string]interface{}{"x": 10.0, "y": 20.0, "z": 10.0}

	tests := []struct {
		expr     string
		expected float64
	}{
		{"x < y", 1},
		{"x > y", 0},
		{"x <= y", 1},
		{"x >= y", 0},
		{"x <= z", 1},
		{"x >= z", 1},
		{"x == z", 1},
		{"x == y", 0},
		{"x != y", 1},
		{"x != z", 0},
	}

	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			result, err := Evaluate(tt.expr, vars)
			if err != nil {
				t.Fatalf("evaluation error: %v", err)
			}
			num, ok := result.(float64)
			if !ok {
				t.Fatalf("expected float64, got %T", result)
			}
			if num != tt.expected {
				t.Errorf("got %v, want %v", num, tt.expected)
			}
		})
	}
}

func TestEvaluate_Logical(t *testing.T) {
	vars := map[string]interface{}{"a": 1.0, "b": 0.0}

	tests := []struct {
		expr     string
		expected float64
	}{
		{"a && a", 1},
		{"a && b", 0},
		{"b && b", 0},
		{"a || a", 1},
		{"a || b", 1},
		{"b || b", 0},
		{"!a", 0},
		{"!b", 1},
		{"!(a && b)", 1},
	}

	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			result, err := Evaluate(tt.expr, vars)
			if err != nil {
				t.Fatalf("evaluation error: %v", err)
			}
			num, ok := result.(float64)
			if !ok {
				t.Fatalf("expected float64, got %T", result)
			}
			if num != tt.expected {
				t.Errorf("got %v, want %v", num, tt.expected)
			}
		})
	}
}

func TestEvaluate_Ternary(t *testing.T) {
	vars := map[string]interface{}{"x": 5.0}

	tests := []struct {
		expr     string
		expected float64
	}{
		{"x > 0 ? 1 : 0", 1},
		{"x < 0 ? 1 : 0", 0},
		{"x > 0 ? x : -x", 5},
		{"x < 0 ? -x : x", 5},
	}

	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			result, err := Evaluate(tt.expr, vars)
			if err != nil {
				t.Fatalf("evaluation error: %v", err)
			}
			num, ok := result.(float64)
			if !ok {
				t.Fatalf("expected float64, got %T", result)
			}
			if num != tt.expected {
				t.Errorf("got %v, want %v", num, tt.expected)
			}
		})
	}
}

func TestEvaluate_MathFunctions(t *testing.T) {
	tests := []struct {
		expr     string
		expected float64
	}{
		{"abs(-5)", 5},
		{"abs(5)", 5},
		{"ceil(3.2)", 4},
		{"floor(3.8)", 3},
		{"round(3.5)", 4},
		{"round(3.4)", 3},
		{"sqrt(16)", 4},
		{"pow(2, 3)", 8},
		{"log(2.718281828)", 1},
		{"log10(100)", 2},
		{"exp(0)", 1},
		{"min(1, 2, 3)", 1},
		{"max(1, 2, 3)", 3},
	}

	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			result, err := Evaluate(tt.expr, nil)
			if err != nil {
				t.Fatalf("evaluation error: %v", err)
			}
			num, ok := result.(float64)
			if !ok {
				t.Fatalf("expected float64, got %T", result)
			}
			if math.Abs(num-tt.expected) > 0.0001 {
				t.Errorf("got %v, want %v", num, tt.expected)
			}
		})
	}
}

func TestEvaluate_AggregationFunctions(t *testing.T) {
	vars := map[string]interface{}{
		"arr": []interface{}{1.0, 2.0, 3.0, 4.0, 5.0},
	}

	tests := []struct {
		expr     string
		expected float64
	}{
		{"sum(1, 2, 3)", 6},
		{"sum(arr)", 15},
		{"avg(1, 2, 3, 4, 5)", 3},
		{"avg(arr)", 3},
		{"count(arr)", 5},
		{"count(1, 2, 3)", 3},
	}

	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			result, err := Evaluate(tt.expr, vars)
			if err != nil {
				t.Fatalf("evaluation error: %v", err)
			}
			num, ok := result.(float64)
			if !ok {
				t.Fatalf("expected float64, got %T", result)
			}
			if math.Abs(num-tt.expected) > 0.0001 {
				t.Errorf("got %v, want %v", num, tt.expected)
			}
		})
	}
}

func TestEvaluate_StringFunctions(t *testing.T) {
	tests := []struct {
		expr     string
		expected interface{}
	}{
		{`len("hello")`, 5.0},
		{`lower("HELLO")`, "hello"},
		{`upper("hello")`, "HELLO"},
		{`concat("hello", " ", "world")`, "hello world"},
		{`substr("hello", 1, 3)`, "ell"},
		{`substr("hello", 2)`, "llo"},
	}

	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			result, err := Evaluate(tt.expr, nil)
			if err != nil {
				t.Fatalf("evaluation error: %v", err)
			}
			if result != tt.expected {
				t.Errorf("got %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestEvaluate_ConditionalFunctions(t *testing.T) {
	vars := map[string]interface{}{"x": 10.0, "y": nil}

	tests := []struct {
		expr     string
		expected interface{}
	}{
		{"if(x > 5, 1, 0)", float64(1)},
		{"if(x < 5, 1, 0)", float64(0)},
		{"coalesce(y, x)", 10.0},
		{"coalesce(x, y)", 10.0},
	}

	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			result, err := Evaluate(tt.expr, vars)
			if err != nil {
				t.Fatalf("evaluation error: %v", err)
			}
			if result != tt.expected {
				t.Errorf("got %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestEvaluate_TypeConversionFunctions(t *testing.T) {
	tests := []struct {
		expr     string
		expected interface{}
	}{
		{"float(42)", 42.0},
		{"int(3.9)", 3.0},
		{`str(42)`, "42"},
		{"bool(0)", 0.0},
		{"bool(1)", 1.0},
	}

	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			result, err := Evaluate(tt.expr, nil)
			if err != nil {
				t.Fatalf("evaluation error: %v", err)
			}
			if result != tt.expected {
				t.Errorf("got %v (%T), want %v (%T)", result, result, tt.expected, tt.expected)
			}
		})
	}
}

func TestEvaluate_FeatureFunctions(t *testing.T) {
	tests := []struct {
		expr     string
		expected float64
		delta    float64
	}{
		{"zscore(10, 5, 2.5)", 2.0, 0.0001},
		{"normalize(5, 0, 10)", 0.5, 0.0001},
		{"normalize(0, 0, 10)", 0.0, 0.0001},
		{"normalize(10, 0, 10)", 1.0, 0.0001},
		{"clip(5, 0, 10)", 5.0, 0},
		{"clip(-5, 0, 10)", 0.0, 0},
		{"clip(15, 0, 10)", 10.0, 0},
		{"sigmoid(0)", 0.5, 0.0001},
	}

	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			result, err := Evaluate(tt.expr, nil)
			if err != nil {
				t.Fatalf("evaluation error: %v", err)
			}
			num, ok := result.(float64)
			if !ok {
				t.Fatalf("expected float64, got %T", result)
			}
			if math.Abs(num-tt.expected) > tt.delta {
				t.Errorf("got %v, want %v", num, tt.expected)
			}
		})
	}
}

func TestEvaluate_IndexAccess(t *testing.T) {
	vars := map[string]interface{}{
		"arr": []interface{}{10.0, 20.0, 30.0},
		"obj": map[string]interface{}{
			"a": 1.0,
			"b": 2.0,
		},
	}

	tests := []struct {
		expr     string
		expected interface{}
	}{
		{"arr[0]", 10.0},
		{"arr[1]", 20.0},
		{"arr[2]", 30.0},
		{`obj["a"]`, 1.0},
		{`obj["b"]`, 2.0},
	}

	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			result, err := Evaluate(tt.expr, vars)
			if err != nil {
				t.Fatalf("evaluation error: %v", err)
			}
			if result != tt.expected {
				t.Errorf("got %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestEvaluate_PropertyAccess(t *testing.T) {
	vars := map[string]interface{}{
		"user": map[string]interface{}{
			"name": "Alice",
			"age":  30.0,
			"address": map[string]interface{}{
				"city": "NYC",
			},
		},
	}

	tests := []struct {
		expr     string
		expected interface{}
	}{
		{"user.name", "Alice"},
		{"user.age", 30.0},
		{"user.address.city", "NYC"},
	}

	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			result, err := Evaluate(tt.expr, vars)
			if err != nil {
				t.Fatalf("evaluation error: %v", err)
			}
			if result != tt.expected {
				t.Errorf("got %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestEvaluate_StringConcatenation(t *testing.T) {
	result, err := Evaluate(`"hello" + " " + "world"`, nil)
	if err != nil {
		t.Fatalf("evaluation error: %v", err)
	}
	str, ok := result.(string)
	if !ok {
		t.Fatalf("expected string, got %T", result)
	}
	if str != "hello world" {
		t.Errorf("got %q, want %q", str, "hello world")
	}
}

func TestEvaluate_ComplexExpressions(t *testing.T) {
	vars := map[string]interface{}{
		"purchases":   150.0,
		"logins":      30.0,
		"account_age": 365.0,
		"mean":        100.0,
		"stddev":      25.0,
		"is_premium":  1.0,
		"feature_a":   10.0,
		"feature_b":   20.0,
		"weight_a":    0.3,
		"weight_b":    0.7,
	}

	tests := []struct {
		name     string
		expr     string
		expected float64
		delta    float64
	}{
		{
			"zscore of purchases",
			"zscore(purchases, mean, stddev)",
			2.0,
			0.0001,
		},
		{
			"weighted sum",
			"feature_a * weight_a + feature_b * weight_b",
			17.0,
			0.0001,
		},
		{
			"conditional multiplier",
			"purchases * (is_premium ? 1.5 : 1.0)",
			225.0,
			0.0001,
		},
		{
			"complex calculation",
			"(logins / account_age) * 365 * (is_premium + 1)",
			60.0,
			0.0001,
		},
		{
			"normalized clipped",
			"clip(normalize(purchases, 0, 200), 0.1, 0.9)",
			0.75,
			0.0001,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Evaluate(tt.expr, vars)
			if err != nil {
				t.Fatalf("evaluation error: %v", err)
			}
			num, ok := result.(float64)
			if !ok {
				t.Fatalf("expected float64, got %T", result)
			}
			if math.Abs(num-tt.expected) > tt.delta {
				t.Errorf("got %v, want %v", num, tt.expected)
			}
		})
	}
}

func TestEvaluator_CustomFunction(t *testing.T) {
	vars := map[string]interface{}{"x": 5.0}
	evaluator := NewEvaluator(vars)

	// Register custom function
	evaluator.RegisterFunction("double", func(args []interface{}) (interface{}, error) {
		if len(args) != 1 {
			return nil, nil
		}
		v, ok := toFloat(args[0])
		if !ok {
			return nil, nil
		}
		return v * 2, nil
	})

	parser := NewParser("double(x)")
	ast, err := parser.Parse()
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	result, err := evaluator.Eval(ast)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}

	num, ok := result.(float64)
	if !ok {
		t.Fatalf("expected float64, got %T", result)
	}
	if num != 10.0 {
		t.Errorf("got %v, want 10", num)
	}
}

func TestEvaluate_Softmax(t *testing.T) {
	result, err := Evaluate("softmax(1, 2, 3)", nil)
	if err != nil {
		t.Fatalf("evaluation error: %v", err)
	}
	arr, ok := result.([]float64)
	if !ok {
		t.Fatalf("expected []float64, got %T", result)
	}
	if len(arr) != 3 {
		t.Fatalf("expected 3 elements, got %d", len(arr))
	}
	// Check that they sum to 1
	sum := arr[0] + arr[1] + arr[2]
	if math.Abs(sum-1.0) > 0.0001 {
		t.Errorf("softmax sum = %v, want 1.0", sum)
	}
	// Check ordering (larger input should give larger probability)
	if arr[0] >= arr[1] || arr[1] >= arr[2] {
		t.Errorf("softmax ordering wrong: %v", arr)
	}
}

func TestEvaluate_DivisionByZero(t *testing.T) {
	result, err := Evaluate("1 / 0", nil)
	if err != nil {
		t.Fatalf("evaluation error: %v", err)
	}
	num, ok := result.(float64)
	if !ok {
		t.Fatalf("expected float64, got %T", result)
	}
	if !math.IsInf(num, 1) {
		t.Errorf("expected +Inf, got %v", num)
	}
}

func TestEvaluate_UndefinedVariable(t *testing.T) {
	_, err := Evaluate("undefined_var", nil)
	if err == nil {
		t.Error("expected error for undefined variable, got nil")
	}
}

func TestEvaluate_UndefinedFunction(t *testing.T) {
	_, err := Evaluate("undefined_func()", nil)
	if err == nil {
		t.Error("expected error for undefined function, got nil")
	}
}

func TestEvaluate_NullHandling(t *testing.T) {
	vars := map[string]interface{}{"n": nil}

	result, err := Evaluate("null", vars)
	if err != nil {
		t.Fatalf("evaluation error: %v", err)
	}
	if result != nil {
		t.Errorf("expected nil, got %v", result)
	}

	result, err = Evaluate("n", vars)
	if err != nil {
		t.Fatalf("evaluation error: %v", err)
	}
	if result != nil {
		t.Errorf("expected nil, got %v", result)
	}
}

func BenchmarkEvaluate_Simple(b *testing.B) {
	vars := map[string]interface{}{"x": 10.0, "y": 20.0}
	for i := 0; i < b.N; i++ {
		_, _ = Evaluate("x + y", vars)
	}
}

func BenchmarkEvaluate_Complex(b *testing.B) {
	vars := map[string]interface{}{
		"purchases":  150.0,
		"mean":       100.0,
		"stddev":     25.0,
		"is_premium": 1.0,
	}
	for i := 0; i < b.N; i++ {
		_, _ = Evaluate("zscore(purchases, mean, stddev) * (is_premium ? 1.5 : 1.0)", vars)
	}
}

func BenchmarkEvaluate_FunctionCall(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _ = Evaluate("max(min(10, 20), 5)", nil)
	}
}
