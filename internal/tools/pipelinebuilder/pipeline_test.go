package pipelinebuilder

import (
	"strings"
	"testing"
)

// --- Pipeline tests ---

func TestNewPipeline(t *testing.T) {
	p, _ := NewPipeline("test", "desc")
	if p.Name != "test" || p.Description != "desc" {
		t.Fatal("unexpected pipeline fields")
	}
	if p.Status != StatusDraft {
		t.Fatalf("expected draft, got %s", p.Status)
	}
	if len(p.ID) == 0 {
		t.Fatal("expected generated ID")
	}
}

func TestAddRemoveNode(t *testing.T) {
	p, _ := NewPipeline("p", "")
	n := &PipelineNode{ID: "n1", Type: NodeSource, Name: "src"}
	if err := p.AddNode(n); err != nil {
		t.Fatalf("AddNode: %v", err)
	}
	if _, ok := p.Nodes["n1"]; !ok {
		t.Fatal("node not found after add")
	}

	// Duplicate
	if err := p.AddNode(n); err == nil {
		t.Fatal("expected error on duplicate add")
	}

	// Remove
	if err := p.RemoveNode("n1"); err != nil {
		t.Fatalf("RemoveNode: %v", err)
	}
	if _, ok := p.Nodes["n1"]; ok {
		t.Fatal("node still present after remove")
	}

	// Remove non-existent
	if err := p.RemoveNode("n1"); err == nil {
		t.Fatal("expected error removing non-existent node")
	}
}

func TestConnect(t *testing.T) {
	p, _ := NewPipeline("p", "")
	_ = p.AddNode(&PipelineNode{ID: "a", Type: NodeSource, Name: "A"})
	_ = p.AddNode(&PipelineNode{ID: "b", Type: NodeTransform, Name: "B"})

	if err := p.Connect("a", "b"); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	if len(p.Nodes["b"].Inputs) != 1 || p.Nodes["b"].Inputs[0] != "a" {
		t.Fatal("connection not recorded")
	}

	// Duplicate connection
	if err := p.Connect("a", "b"); err == nil {
		t.Fatal("expected error on duplicate connection")
	}

	// Non-existent source
	if err := p.Connect("x", "b"); err == nil {
		t.Fatal("expected error for missing source")
	}
}

func TestConnectCycleDetection(t *testing.T) {
	p, _ := NewPipeline("p", "")
	_ = p.AddNode(&PipelineNode{ID: "a", Type: NodeSource, Name: "A"})
	_ = p.AddNode(&PipelineNode{ID: "b", Type: NodeTransform, Name: "B"})
	_ = p.AddNode(&PipelineNode{ID: "c", Type: NodeTransform, Name: "C"})
	_ = p.Connect("a", "b")
	_ = p.Connect("b", "c")

	if err := p.Connect("c", "a"); err == nil {
		t.Fatal("expected cycle error")
	}
}

func TestValidate(t *testing.T) {
	p, _ := NewPipeline("", "")
	errs := p.Validate()
	hasNameErr := false
	for _, e := range errs {
		if e.Field == "name" {
			hasNameErr = true
		}
	}
	if !hasNameErr {
		t.Fatal("expected name validation error")
	}

	p2, _ := NewPipeline("valid", "desc")
	_ = p2.AddNode(&PipelineNode{ID: "a", Type: NodeSource, Name: "A"})
	_ = p2.AddNode(&PipelineNode{ID: "b", Type: NodeTransform, Name: "B"})
	_ = p2.Connect("a", "b")
	errs2 := p2.Validate()
	for _, e := range errs2 {
		if e.Severity == "error" {
			t.Fatalf("unexpected validation error: %s", e.Message)
		}
	}
}

func TestTopologicalSort(t *testing.T) {
	p, _ := NewPipeline("p", "")
	_ = p.AddNode(&PipelineNode{ID: "a", Type: NodeSource, Name: "A"})
	_ = p.AddNode(&PipelineNode{ID: "b", Type: NodeTransform, Name: "B"})
	_ = p.AddNode(&PipelineNode{ID: "c", Type: NodeSink, Name: "C"})
	_ = p.Connect("a", "b")
	_ = p.Connect("b", "c")

	order, err := p.TopologicalSort()
	if err != nil {
		t.Fatalf("TopologicalSort: %v", err)
	}
	if len(order) != 3 {
		t.Fatalf("expected 3 nodes, got %d", len(order))
	}
	// a must come before b, b before c
	idx := map[string]int{}
	for i, id := range order {
		idx[id] = i
	}
	if idx["a"] >= idx["b"] || idx["b"] >= idx["c"] {
		t.Fatalf("unexpected order: %v", order)
	}
}

func TestRemoveNodeCleansEdges(t *testing.T) {
	p, _ := NewPipeline("p", "")
	_ = p.AddNode(&PipelineNode{ID: "a", Type: NodeSource, Name: "A"})
	_ = p.AddNode(&PipelineNode{ID: "b", Type: NodeTransform, Name: "B"})
	_ = p.AddNode(&PipelineNode{ID: "c", Type: NodeSink, Name: "C"})
	_ = p.Connect("a", "b")
	_ = p.Connect("b", "c")
	_ = p.RemoveNode("b")

	if len(p.Nodes["c"].Inputs) != 0 {
		t.Fatal("expected inputs to be cleaned up after removal")
	}
}

// --- TransformRegistry tests ---

func TestTransformRegistryBuiltins(t *testing.T) {
	r := NewTransformRegistry()
	all := r.List()
	if len(all) < 20 {
		t.Fatalf("expected at least 20 built-in transforms, got %d", len(all))
	}
}

func TestTransformRegistryGetAndSearch(t *testing.T) {
	r := NewTransformRegistry()
	tf, err := r.Get("math_log")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if tf.Name != "Log" {
		t.Fatalf("unexpected name %s", tf.Name)
	}

	_, err = r.Get("nonexistent")
	if err == nil {
		t.Fatal("expected error for missing transform")
	}

	results := r.Search("sqrt")
	if len(results) == 0 {
		t.Fatal("expected search results for 'sqrt'")
	}

	mathResults := r.ListByCategory("math")
	if len(mathResults) < 7 {
		t.Fatalf("expected at least 7 math transforms, got %d", len(mathResults))
	}
}

func TestTransformRegistryRegister(t *testing.T) {
	r := NewTransformRegistry()
	err := r.Register(&TransformDef{ID: "custom_1", Name: "Custom", Category: "custom"})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	// Duplicate
	err = r.Register(&TransformDef{ID: "custom_1", Name: "Custom", Category: "custom"})
	if err == nil {
		t.Fatal("expected error on duplicate register")
	}
}

// --- CodeGenerator tests ---

func TestCodeGeneratorGo(t *testing.T) {
	p, _ := NewPipeline("TestPipeline", "A test pipeline")
	_ = p.AddNode(&PipelineNode{ID: "src", Type: NodeSource, Name: "Source"})
	_ = p.AddNode(&PipelineNode{ID: "xform", Type: NodeTransform, Name: "Transform"})
	_ = p.Connect("src", "xform")

	reg := NewTransformRegistry()
	gen := NewCodeGenerator(CodeGenConfig{Language: "go", IncludeComments: true, PackageName: "mypkg"})
	code, err := gen.Generate(p, reg)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if !strings.Contains(code, "package mypkg") {
		t.Fatal("expected package declaration")
	}
	if !strings.Contains(code, "RunPipeline") {
		t.Fatal("expected RunPipeline function")
	}
}

func TestCodeGeneratorPython(t *testing.T) {
	p, _ := NewPipeline("TestPipeline", "desc")
	_ = p.AddNode(&PipelineNode{ID: "src", Type: NodeSource, Name: "Source"})
	reg := NewTransformRegistry()
	gen := NewCodeGenerator(CodeGenConfig{Language: "python", IncludeComments: true})
	code, err := gen.Generate(p, reg)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if !strings.Contains(code, "def run_pipeline") {
		t.Fatal("expected run_pipeline function")
	}
}

func TestCodeGeneratorFeatherQL(t *testing.T) {
	p, _ := NewPipeline("TestPipeline", "desc")
	_ = p.AddNode(&PipelineNode{ID: "src", Type: NodeSource, Name: "Source"})
	reg := NewTransformRegistry()
	gen := NewCodeGenerator(CodeGenConfig{Language: "featherql"})
	code, err := gen.Generate(p, reg)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if !strings.Contains(code, "CREATE PIPELINE") {
		t.Fatal("expected CREATE PIPELINE statement")
	}
}

func TestCodeGeneratorUnsupported(t *testing.T) {
	p, _ := NewPipeline("p", "")
	_ = p.AddNode(&PipelineNode{ID: "src", Type: NodeSource, Name: "Source"})
	gen := NewCodeGenerator(CodeGenConfig{Language: "ruby"})
	_, err := gen.Generate(p, NewTransformRegistry())
	if err == nil {
		t.Fatal("expected error for unsupported language")
	}
}

// --- TemplateStore tests ---

func TestTemplateStoreBuiltins(t *testing.T) {
	s := NewTemplateStore()
	all := s.List()
	if len(all) != 5 {
		t.Fatalf("expected 5 built-in templates, got %d", len(all))
	}
}

func TestTemplateStoreGetAndSearch(t *testing.T) {
	s := NewTemplateStore()
	tmpl, err := s.Get("fraud-detection")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if tmpl.Name != "Fraud Detection" {
		t.Fatalf("unexpected name %s", tmpl.Name)
	}

	_, err = s.Get("nonexistent")
	if err == nil {
		t.Fatal("expected error for missing template")
	}

	results := s.Search("fraud")
	if len(results) == 0 {
		t.Fatal("expected search results for 'fraud'")
	}
}

func TestTemplateStoreCreate(t *testing.T) {
	s := NewTemplateStore()
	err := s.Create(&Template{ID: "custom", Name: "Custom"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	err = s.Create(&Template{ID: "custom", Name: "Custom"})
	if err == nil {
		t.Fatal("expected error on duplicate create")
	}
}

func TestTemplateStoreIncrementUsage(t *testing.T) {
	s := NewTemplateStore()
	s.IncrementUsage("fraud-detection")
	tmpl, _ := s.Get("fraud-detection")
	if tmpl.UsageCount != 1 {
		t.Fatalf("expected usage count 1, got %d", tmpl.UsageCount)
	}
}
