package pythonsdk

import (
	"errors"
	"testing"
)

func TestNewRegistry(t *testing.T) {
	r := NewRegistry(DefaultRegistryConfig())
	if r == nil {
		t.Fatal("expected non-nil registry")
	}
}

func TestRegisterTransform(t *testing.T) {
	r := NewRegistry(DefaultRegistryConfig())
	def, err := r.Register(TransformDef{
		ID:         "user_age_bucket",
		Name:       "User Age Bucketing",
		SourceCode: "def transform(age): return age // 10 * 10",
		EntryPoint: "transform",
		Type:       TransformOnDemand,
		Inputs:     []FieldSchema{{Name: "age", DType: "int64", Required: true}},
		Outputs:    []FieldSchema{{Name: "age_bucket", DType: "int64"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if def.Version != 1 {
		t.Errorf("expected version 1, got %d", def.Version)
	}
	if def.Status != StatusRegistered {
		t.Errorf("expected registered, got %s", def.Status)
	}
}

func TestDuplicateTransform(t *testing.T) {
	r := NewRegistry(DefaultRegistryConfig())
	_, _ = r.Register(TransformDef{ID: "t1", Name: "T1", SourceCode: "pass"})
	_, err := r.Register(TransformDef{ID: "t1", Name: "T1 dup", SourceCode: "pass"})
	if !errors.Is(err, ErrTransformExists) {
		t.Fatalf("expected ErrTransformExists, got %v", err)
	}
}

func TestInvalidTransform(t *testing.T) {
	r := NewRegistry(DefaultRegistryConfig())
	_, err := r.Register(TransformDef{})
	if !errors.Is(err, ErrInvalidTransform) {
		t.Fatalf("expected ErrInvalidTransform, got %v", err)
	}
}

func TestGetTransform(t *testing.T) {
	r := NewRegistry(DefaultRegistryConfig())
	_, _ = r.Register(TransformDef{ID: "t1", Name: "T1", SourceCode: "pass"})

	def, err := r.Get("t1")
	if err != nil {
		t.Fatal(err)
	}
	if def.Name != "T1" {
		t.Errorf("expected T1, got %s", def.Name)
	}

	_, err = r.Get("nonexistent")
	if !errors.Is(err, ErrTransformNotFound) {
		t.Fatalf("expected ErrTransformNotFound, got %v", err)
	}
}

func TestUpdateTransform(t *testing.T) {
	r := NewRegistry(DefaultRegistryConfig())
	_, _ = r.Register(TransformDef{ID: "t1", Name: "T1", SourceCode: "v1"})

	updated, err := r.Update(TransformDef{ID: "t1", Name: "T1 Updated", SourceCode: "v2"})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Version != 2 {
		t.Errorf("expected version 2, got %d", updated.Version)
	}
}

func TestDeleteTransform(t *testing.T) {
	r := NewRegistry(DefaultRegistryConfig())
	_, _ = r.Register(TransformDef{ID: "t1", Name: "T1", SourceCode: "pass"})

	if err := r.Delete("t1"); err != nil {
		t.Fatal(err)
	}
	_, err := r.Get("t1")
	if !errors.Is(err, ErrTransformNotFound) {
		t.Fatalf("expected ErrTransformNotFound, got %v", err)
	}
}

func TestExecuteTransform(t *testing.T) {
	r := NewRegistry(DefaultRegistryConfig())
	_, _ = r.Register(TransformDef{
		ID: "t1", Name: "T1", SourceCode: "pass",
		Outputs: []FieldSchema{{Name: "result", DType: "float64"}},
	})

	result, err := r.Execute("t1", map[string]interface{}{"input": 42})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Success {
		t.Error("expected success")
	}
	if _, ok := result.Outputs["result"]; !ok {
		t.Error("expected result output")
	}
}

func TestDeployTransform(t *testing.T) {
	r := NewRegistry(DefaultRegistryConfig())
	_, _ = r.Register(TransformDef{ID: "t1", Name: "T1", SourceCode: "pass"})

	if err := r.Deploy("t1"); err != nil {
		t.Fatal(err)
	}
	def, _ := r.Get("t1")
	if def.Status != StatusDeployed {
		t.Errorf("expected deployed, got %s", def.Status)
	}
}

func TestRegistryStats(t *testing.T) {
	r := NewRegistry(DefaultRegistryConfig())
	_, _ = r.Register(TransformDef{ID: "t1", Name: "T1", SourceCode: "pass", Type: TransformOnDemand})
	_, _ = r.Register(TransformDef{ID: "t2", Name: "T2", SourceCode: "pass", Type: TransformBatch})
	_, _ = r.Execute("t1", nil)

	stats := r.Stats()
	if stats.TotalTransforms != 2 {
		t.Errorf("expected 2 transforms, got %d", stats.TotalTransforms)
	}
	if stats.TotalExecutions != 1 {
		t.Errorf("expected 1 execution, got %d", stats.TotalExecutions)
	}
}

func TestListTransforms(t *testing.T) {
	r := NewRegistry(DefaultRegistryConfig())
	_, _ = r.Register(TransformDef{ID: "t1", Name: "T1", SourceCode: "pass"})
	_, _ = r.Register(TransformDef{ID: "t2", Name: "T2", SourceCode: "pass"})

	transforms := r.List()
	if len(transforms) != 2 {
		t.Fatalf("expected 2 transforms, got %d", len(transforms))
	}
}
