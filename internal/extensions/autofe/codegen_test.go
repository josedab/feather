package autofe

import (
	"strings"
	"testing"
)

func candidate(name string, transform TransformType, sources []string) *CandidateFeature {
	return &CandidateFeature{
		Name:           name,
		Expression:     name + "_expr",
		Transform:      transform,
		SourceFeatures: sources,
		Score:          0.85,
		Explanation:    "test transform",
	}
}

// --- GenerateGo tests ---

func TestGenerateGo_LogTransform(t *testing.T) {
	t.Parallel()
	gen := NewCodeGenerator()
	code := gen.GenerateGo([]*CandidateFeature{candidate("log_val", TransformLog, []string{"price"})})
	if !strings.Contains(code, "math.Log(price)") {
		t.Error("expected math.Log call in Go code")
	}
	if !strings.Contains(code, "func ComputeLogVal") {
		t.Error("expected function name ComputeLogVal")
	}
}

func TestGenerateGo_SqrtTransform(t *testing.T) {
	t.Parallel()
	gen := NewCodeGenerator()
	code := gen.GenerateGo([]*CandidateFeature{candidate("sqrt_val", TransformSqrt, []string{"x"})})
	if !strings.Contains(code, "math.Sqrt(x)") {
		t.Error("expected math.Sqrt call")
	}
}

func TestGenerateGo_SquareTransform(t *testing.T) {
	t.Parallel()
	gen := NewCodeGenerator()
	code := gen.GenerateGo([]*CandidateFeature{candidate("sq_val", TransformSquare, []string{"x"})})
	if !strings.Contains(code, "x * x") {
		t.Error("expected x * x in square transform")
	}
}

func TestGenerateGo_InteractionTransform(t *testing.T) {
	t.Parallel()
	gen := NewCodeGenerator()
	code := gen.GenerateGo([]*CandidateFeature{candidate("interact", TransformInteraction, []string{"a", "b"})})
	if !strings.Contains(code, "a * b") {
		t.Error("expected a * b in interaction transform")
	}
}

func TestGenerateGo_RatioTransform(t *testing.T) {
	t.Parallel()
	gen := NewCodeGenerator()
	code := gen.GenerateGo([]*CandidateFeature{candidate("ratio_val", TransformRatio, []string{"num", "den"})})
	if !strings.Contains(code, "num / den") {
		t.Error("expected num / den in ratio transform")
	}
	if !strings.Contains(code, "den == 0") {
		t.Error("expected zero division guard")
	}
}

func TestGenerateGo_EmptyCandidates(t *testing.T) {
	t.Parallel()
	gen := NewCodeGenerator()
	code := gen.GenerateGo(nil)
	if !strings.Contains(code, "package features") {
		t.Error("expected package header even with empty candidates")
	}
}

func TestGenerateGo_MultipleCandidates(t *testing.T) {
	t.Parallel()
	gen := NewCodeGenerator()
	candidates := []*CandidateFeature{
		candidate("log_a", TransformLog, []string{"a"}),
		candidate("sqrt_b", TransformSqrt, []string{"b"}),
	}
	code := gen.GenerateGo(candidates)
	if !strings.Contains(code, "ComputeLogA") || !strings.Contains(code, "ComputeSqrtB") {
		t.Error("expected both functions in output")
	}
}

// --- GeneratePython tests ---

func TestGeneratePython_LogTransform(t *testing.T) {
	t.Parallel()
	gen := NewCodeGenerator()
	code := gen.GeneratePython([]*CandidateFeature{candidate("log_val", TransformLog, []string{"price"})})
	if !strings.Contains(code, "math.log(price)") {
		t.Error("expected math.log call")
	}
	if !strings.Contains(code, "def compute_log_val") {
		t.Error("expected python function name")
	}
	if !strings.Contains(code, "-> float") {
		t.Error("expected return type annotation")
	}
}

func TestGeneratePython_SqrtTransform(t *testing.T) {
	t.Parallel()
	gen := NewCodeGenerator()
	code := gen.GeneratePython([]*CandidateFeature{candidate("sqrt_val", TransformSqrt, []string{"x"})})
	if !strings.Contains(code, "math.sqrt(x)") {
		t.Error("expected math.sqrt call")
	}
}

func TestGeneratePython_SquareTransform(t *testing.T) {
	t.Parallel()
	gen := NewCodeGenerator()
	code := gen.GeneratePython([]*CandidateFeature{candidate("sq", TransformSquare, []string{"x"})})
	if !strings.Contains(code, "x ** 2") {
		t.Error("expected x ** 2")
	}
}

func TestGeneratePython_InteractionTransform(t *testing.T) {
	t.Parallel()
	gen := NewCodeGenerator()
	code := gen.GeneratePython([]*CandidateFeature{candidate("interact", TransformInteraction, []string{"a", "b"})})
	if !strings.Contains(code, "a * b") {
		t.Error("expected a * b")
	}
}

func TestGeneratePython_RatioTransform(t *testing.T) {
	t.Parallel()
	gen := NewCodeGenerator()
	code := gen.GeneratePython([]*CandidateFeature{candidate("ratio", TransformRatio, []string{"num", "den"})})
	if !strings.Contains(code, "num / den") {
		t.Error("expected num / den")
	}
	if !strings.Contains(code, "den == 0") {
		t.Error("expected zero guard")
	}
}

func TestGeneratePython_ImportMath(t *testing.T) {
	t.Parallel()
	gen := NewCodeGenerator()
	code := gen.GeneratePython(nil)
	if !strings.Contains(code, "import math") {
		t.Error("expected import math header")
	}
}

// --- GenerateFeatherQL tests ---

func TestGenerateFeatherQL_CreateFeature(t *testing.T) {
	t.Parallel()
	gen := NewCodeGenerator()
	c := candidate("log_price", TransformLog, []string{"price"})
	code := gen.GenerateFeatherQL([]*CandidateFeature{c})

	if !strings.Contains(code, "CREATE FEATURE log_price") {
		t.Error("expected CREATE FEATURE statement")
	}
	if !strings.Contains(code, "FROM price") {
		t.Error("expected FROM clause")
	}
	if !strings.Contains(code, "score: 0.85") {
		t.Error("expected scoring annotation")
	}
}

func TestGenerateFeatherQL_MultipleCandidates(t *testing.T) {
	t.Parallel()
	gen := NewCodeGenerator()
	candidates := []*CandidateFeature{
		candidate("feat_a", TransformLog, []string{"a"}),
		candidate("feat_b", TransformSqrt, []string{"b"}),
	}
	code := gen.GenerateFeatherQL(candidates)
	if strings.Count(code, "CREATE FEATURE") != 2 {
		t.Error("expected 2 CREATE FEATURE statements")
	}
}

// --- goFuncName / pyFuncName tests ---

func TestGoFuncName_Basic(t *testing.T) {
	t.Parallel()
	name := goFuncName("click_count")
	if name != "ComputeClickCount" {
		t.Errorf("expected ComputeClickCount, got %s", name)
	}
}

func TestGoFuncName_SingleWord(t *testing.T) {
	t.Parallel()
	name := goFuncName("clicks")
	if name != "ComputeClicks" {
		t.Errorf("expected ComputeClicks, got %s", name)
	}
}

func TestGoFuncName_EmptyInput(t *testing.T) {
	t.Parallel()
	name := goFuncName("")
	if name != "Compute" {
		t.Errorf("expected Compute, got %s", name)
	}
}

func TestPyFuncName_Basic(t *testing.T) {
	t.Parallel()
	name := pyFuncName("click_count")
	if name != "compute_click_count" {
		t.Errorf("expected compute_click_count, got %s", name)
	}
}

func TestPyFuncName_WithSpaces(t *testing.T) {
	t.Parallel()
	name := pyFuncName("my feature")
	if name != "compute_my_feature" {
		t.Errorf("expected compute_my_feature, got %s", name)
	}
}

func TestPyFuncName_EmptyInput(t *testing.T) {
	t.Parallel()
	name := pyFuncName("")
	if name != "compute_" {
		t.Errorf("expected compute_, got %s", name)
	}
}
