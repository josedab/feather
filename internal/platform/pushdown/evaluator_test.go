package pushdown

import (
	"math"
	"testing"
	"time"
)

func TestEvaluator_SimpleArithmetic(t *testing.T) {
	e := NewEvaluator()
	ctx := map[string]float64{"clicks": 100, "impressions": 1000}

	tests := []struct {
		name     string
		expr     string
		expected float64
	}{
		{"addition", "$clicks + $impressions", 1100},
		{"subtraction", "$impressions - $clicks", 900},
		{"multiplication", "$clicks * 2", 200},
		{"division", "$clicks / $impressions", 0.1},
		{"complex", "($clicks + 10) * 2", 220},
		{"literal", "42", 42},
		{"nested", "($clicks + $impressions) / 2", 550},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := e.EvaluateExpression(tt.expr, ctx)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if math.Abs(result-tt.expected) > 0.001 {
				t.Fatalf("expected %f, got %f", tt.expected, result)
			}
		})
	}
}

func TestEvaluator_Functions(t *testing.T) {
	e := NewEvaluator()
	ctx := map[string]float64{"a": -5, "b": 100, "c": 3}

	tests := []struct {
		name     string
		expr     string
		expected float64
	}{
		{"abs", "abs($a)", 5},
		{"sqrt", "sqrt($b)", 10},
		{"round", "round(3.7)", 4},
		{"min", "min($a, $c)", -5},
		{"max", "max($a, $c)", 3},
		{"log", "log($b)", math.Log(100)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := e.EvaluateExpression(tt.expr, ctx)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if math.Abs(result-tt.expected) > 0.001 {
				t.Fatalf("expected %f, got %f", tt.expected, result)
			}
		})
	}
}

func TestEvaluator_DivisionByZero(t *testing.T) {
	e := NewEvaluator()
	_, err := e.EvaluateExpression("10 / 0", nil)
	if err != ErrDivisionByZero {
		t.Fatalf("expected division by zero error, got %v", err)
	}
}

func TestEvaluator_FeatureNotFound(t *testing.T) {
	e := NewEvaluator()
	_, err := e.EvaluateExpression("$missing + 1", map[string]float64{})
	if err == nil {
		t.Fatal("expected error for missing feature")
	}
}

func TestEvaluator_RegisterDerived(t *testing.T) {
	e := NewEvaluator()

	df := &DerivedFeature{
		Name:       "ctr",
		Expression: "$clicks / $impressions",
		CacheTTL:   time.Minute,
	}
	if err := e.RegisterDerived(df); err != nil {
		t.Fatal(err)
	}

	if len(df.Inputs) != 2 {
		t.Fatalf("expected 2 inputs, got %d", len(df.Inputs))
	}

	ctx := map[string]float64{"clicks": 50, "impressions": 1000}
	result, err := e.Evaluate("user:1", "ctr", ctx)
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(result-0.05) > 0.001 {
		t.Fatalf("expected 0.05, got %f", result)
	}

	// Second eval should hit cache
	result2, err := e.Evaluate("user:1", "ctr", ctx)
	if err != nil {
		t.Fatal(err)
	}
	if result2 != result {
		t.Fatalf("cache miss: expected %f, got %f", result, result2)
	}
}

func TestEvaluator_ListAndGet(t *testing.T) {
	e := NewEvaluator()
	e.RegisterDerived(&DerivedFeature{Name: "f1", Expression: "1 + 2"})
	e.RegisterDerived(&DerivedFeature{Name: "f2", Expression: "3 * 4"})

	list := e.ListDerived()
	if len(list) != 2 {
		t.Fatalf("expected 2, got %d", len(list))
	}

	got, err := e.GetDerived("f1")
	if err != nil || got.Name != "f1" {
		t.Fatal("get failed")
	}

	_, err = e.GetDerived("nonexistent")
	if err != ErrFeatureNotFound {
		t.Fatal("expected not found")
	}

	e.UnregisterDerived("f1")
	list = e.ListDerived()
	if len(list) != 1 {
		t.Fatalf("expected 1 after unregister, got %d", len(list))
	}
}

func TestEvaluator_InvalidExpression(t *testing.T) {
	e := NewEvaluator()

	err := e.RegisterDerived(&DerivedFeature{Name: "bad", Expression: ""})
	if err == nil {
		t.Fatal("expected error for empty expression")
	}

	err = e.RegisterDerived(&DerivedFeature{Name: "", Expression: "1+1"})
	if err == nil {
		t.Fatal("expected error for empty name")
	}
}

func TestTokenize(t *testing.T) {
	tokens := tokenize("($a + $b) * 2")
	expected := []string{"(", "$a", "+", "$b", ")", "*", "2"}
	if len(tokens) != len(expected) {
		t.Fatalf("expected %d tokens, got %d: %v", len(expected), len(tokens), tokens)
	}
	for i, tok := range tokens {
		if tok != expected[i] {
			t.Fatalf("token %d: expected %q, got %q", i, expected[i], tok)
		}
	}
}
