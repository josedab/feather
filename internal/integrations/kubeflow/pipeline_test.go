package kubeflow

import (
	"testing"
	"time"
)

func newTestManager() *Manager {
	return NewManager(DefaultConfig())
}

func validComponent(id, name, compType string) *Component {
	return &Component{
		ID:   id,
		Name: name,
		Type: compType,
	}
}

// --- RegisterComponent ---

func TestRegisterComponent_Valid(t *testing.T) {
	m := newTestManager()
	err := m.RegisterComponent(validComponent("c1", "Component 1", "feature_source"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRegisterComponent_AllTypes(t *testing.T) {
	m := newTestManager()
	for _, typ := range []string{"feature_source", "feature_sink", "transform"} {
		err := m.RegisterComponent(validComponent("c-"+typ, "Comp", typ))
		if err != nil {
			t.Fatalf("unexpected error for type %s: %v", typ, err)
		}
	}
}

func TestRegisterComponent_InvalidType(t *testing.T) {
	m := newTestManager()
	err := m.RegisterComponent(validComponent("c1", "Comp", "invalid"))
	if err == nil {
		t.Fatal("expected error for invalid type")
	}
}

func TestRegisterComponent_MissingID(t *testing.T) {
	m := newTestManager()
	err := m.RegisterComponent(&Component{ID: "", Name: "Comp", Type: "feature_source"})
	if err == nil {
		t.Fatal("expected error for missing ID")
	}
}

func TestRegisterComponent_MissingName(t *testing.T) {
	m := newTestManager()
	err := m.RegisterComponent(&Component{ID: "c1", Name: "", Type: "feature_source"})
	if err == nil {
		t.Fatal("expected error for missing name")
	}
}

func TestRegisterComponent_Duplicate(t *testing.T) {
	m := newTestManager()
	_ = m.RegisterComponent(validComponent("c1", "Comp1", "feature_source"))
	err := m.RegisterComponent(validComponent("c1", "Comp2", "feature_source"))
	if err == nil {
		t.Fatal("expected error for duplicate component")
	}
}

func TestRegisterComponent_SetsCreatedAt(t *testing.T) {
	m := newTestManager()
	comp := validComponent("c1", "Comp1", "feature_source")
	_ = m.RegisterComponent(comp)
	if comp.CreatedAt.IsZero() {
		t.Fatal("expected created_at to be set")
	}
}

// --- GetComponent ---

func TestGetComponent_Found(t *testing.T) {
	m := newTestManager()
	_ = m.RegisterComponent(validComponent("c1", "Comp1", "feature_source"))
	comp, err := m.GetComponent("c1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if comp.ID != "c1" {
		t.Fatalf("expected c1, got %s", comp.ID)
	}
}

func TestGetComponent_NotFound(t *testing.T) {
	m := newTestManager()
	_, err := m.GetComponent("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent component")
	}
}

// --- ListComponents ---

func TestListComponents(t *testing.T) {
	m := newTestManager()
	_ = m.RegisterComponent(validComponent("c1", "C1", "feature_source"))
	_ = m.RegisterComponent(validComponent("c2", "C2", "feature_sink"))

	comps := m.ListComponents()
	if len(comps) != 2 {
		t.Fatalf("expected 2 components, got %d", len(comps))
	}
}

func TestListComponents_Empty(t *testing.T) {
	m := newTestManager()
	comps := m.ListComponents()
	if len(comps) != 0 {
		t.Fatalf("expected 0 components, got %d", len(comps))
	}
}

// --- CreatePipelineRun ---

func TestCreatePipelineRun_Valid(t *testing.T) {
	m := newTestManager()
	_ = m.RegisterComponent(validComponent("c1", "C1", "feature_source"))
	_ = m.RegisterComponent(validComponent("c2", "C2", "transform"))

	run, err := m.CreatePipelineRun("test-pipeline", []string{"c1", "c2"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if run.Name != "test-pipeline" {
		t.Fatalf("expected name test-pipeline, got %s", run.Name)
	}
	if run.Status != "running" {
		t.Fatalf("expected running, got %s", run.Status)
	}
	if len(run.Components) != 2 {
		t.Fatalf("expected 2 components, got %d", len(run.Components))
	}
}

func TestCreatePipelineRun_EmptyName(t *testing.T) {
	m := newTestManager()
	_, err := m.CreatePipelineRun("", nil)
	if err == nil {
		t.Fatal("expected error for empty name")
	}
}

func TestCreatePipelineRun_MissingComponent(t *testing.T) {
	m := newTestManager()
	_, err := m.CreatePipelineRun("test", []string{"nonexistent"})
	if err == nil {
		t.Fatal("expected error for missing component")
	}
}

func TestCreatePipelineRun_EmptyComponents(t *testing.T) {
	m := newTestManager()
	run, err := m.CreatePipelineRun("test", []string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(run.Components) != 0 {
		t.Fatalf("expected 0 components, got %d", len(run.Components))
	}
}

// --- GetPipelineRun ---

func TestGetPipelineRun_Found(t *testing.T) {
	m := newTestManager()
	run, _ := m.CreatePipelineRun("test", []string{})
	got, err := m.GetPipelineRun(run.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != run.ID {
		t.Fatalf("expected %s, got %s", run.ID, got.ID)
	}
}

func TestGetPipelineRun_NotFound(t *testing.T) {
	m := newTestManager()
	_, err := m.GetPipelineRun("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent run")
	}
}

// --- CompletePipelineRun ---

func TestCompletePipelineRun_Completed(t *testing.T) {
	m := newTestManager()
	run, _ := m.CreatePipelineRun("test", []string{})
	err := m.CompletePipelineRun(run.ID, "completed")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if run.Status != "completed" {
		t.Fatalf("expected completed, got %s", run.Status)
	}
	if run.EndTime.IsZero() {
		t.Fatal("expected end time to be set")
	}
}

func TestCompletePipelineRun_Failed(t *testing.T) {
	m := newTestManager()
	run, _ := m.CreatePipelineRun("test", []string{})
	err := m.CompletePipelineRun(run.ID, "failed")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if run.Status != "failed" {
		t.Fatalf("expected failed, got %s", run.Status)
	}
}

func TestCompletePipelineRun_InvalidStatus(t *testing.T) {
	m := newTestManager()
	run, _ := m.CreatePipelineRun("test", []string{})
	err := m.CompletePipelineRun(run.ID, "invalid")
	if err == nil {
		t.Fatal("expected error for invalid status")
	}
}

func TestCompletePipelineRun_NotFound(t *testing.T) {
	m := newTestManager()
	err := m.CompletePipelineRun("nonexistent", "completed")
	if err == nil {
		t.Fatal("expected error for nonexistent run")
	}
}

func TestCompletePipelineRun_AlreadyCompleted(t *testing.T) {
	m := newTestManager()
	run, _ := m.CreatePipelineRun("test", []string{})
	_ = m.CompletePipelineRun(run.ID, "completed")
	err := m.CompletePipelineRun(run.ID, "failed")
	if err == nil {
		t.Fatal("expected error for already completed run")
	}
}

// --- ListPipelineRuns ---

func TestListPipelineRuns(t *testing.T) {
	m := newTestManager()
	m.CreatePipelineRun("r1", []string{})
	time.Sleep(time.Millisecond)
	m.CreatePipelineRun("r2", []string{})

	runs := m.ListPipelineRuns()
	if len(runs) != 2 {
		t.Fatalf("expected 2 runs, got %d", len(runs))
	}
}

func TestListPipelineRuns_Empty(t *testing.T) {
	m := newTestManager()
	runs := m.ListPipelineRuns()
	if len(runs) != 0 {
		t.Fatalf("expected 0 runs, got %d", len(runs))
	}
}

// --- Stats ---

func TestStats(t *testing.T) {
	m := newTestManager()
	_ = m.RegisterComponent(validComponent("c1", "C1", "feature_source"))
	_ = m.RegisterComponent(validComponent("c2", "C2", "transform"))

	run1, _ := m.CreatePipelineRun("r1", []string{"c1"})
	time.Sleep(time.Millisecond)
	m.CreatePipelineRun("r2", []string{"c2"})

	_ = m.CompletePipelineRun(run1.ID, "completed")

	stats := m.Stats()
	if stats.TotalComponents != 2 {
		t.Fatalf("expected 2 components, got %d", stats.TotalComponents)
	}
	if stats.TotalPipelines != 2 {
		t.Fatalf("expected 2 pipelines, got %d", stats.TotalPipelines)
	}
	if stats.ActiveRuns != 1 {
		t.Fatalf("expected 1 active run, got %d", stats.ActiveRuns)
	}
}

func TestStats_Empty(t *testing.T) {
	m := newTestManager()
	stats := m.Stats()
	if stats.TotalComponents != 0 || stats.TotalPipelines != 0 || stats.ActiveRuns != 0 {
		t.Fatal("expected all zeros for empty manager")
	}
}
