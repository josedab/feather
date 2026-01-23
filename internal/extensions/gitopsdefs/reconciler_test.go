package gitopsdefs

import (
	"errors"
	"testing"
)

func TestNewReconciler(t *testing.T) {
	r := NewReconciler(DefaultReconcilerConfig())
	if r == nil {
		t.Fatal("expected non-nil reconciler")
	}
	defs := r.ListDefinitions()
	if len(defs) != 0 {
		t.Errorf("expected 0 definitions, got %d", len(defs))
	}
}

func TestLoadAndList(t *testing.T) {
	r := NewReconciler(DefaultReconcilerConfig())

	err := r.LoadDefinition(FeatureDefinition{
		Name:       "user-features",
		EntityType: "user",
		TTL:        "1h",
		Owner:      "ml-team",
		Version:    "v1",
		Features: []FieldDef{
			{Name: "age", Type: "int", Required: true},
			{Name: "score", Type: "float", Required: false, Default: "0.0"},
		},
	})
	if err != nil {
		t.Fatalf("LoadDefinition failed: %v", err)
	}

	defs := r.ListDefinitions()
	if len(defs) != 1 {
		t.Fatalf("expected 1 definition, got %d", len(defs))
	}
	if defs[0].Name != "user-features" {
		t.Errorf("expected name user-features, got %s", defs[0].Name)
	}
}

func TestLoadInvalid(t *testing.T) {
	r := NewReconciler(DefaultReconcilerConfig())

	err := r.LoadDefinition(FeatureDefinition{})
	if err == nil {
		t.Fatal("expected error for empty name")
	}
	if !errors.Is(err, ErrDefinitionInvalid) {
		t.Errorf("expected ErrDefinitionInvalid, got %v", err)
	}
}

func TestReconcileCreatesNew(t *testing.T) {
	r := NewReconciler(DefaultReconcilerConfig())

	_ = r.LoadDefinition(FeatureDefinition{
		Name:       "new-group",
		EntityType: "item",
		Version:    "v1",
	})

	results := r.Reconcile()
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Action != ActionCreate {
		t.Errorf("expected ActionCreate, got %s", results[0].Action)
	}
	if !results[0].Success {
		t.Error("expected success")
	}
}

func TestReconcileUpdates(t *testing.T) {
	r := NewReconciler(DefaultReconcilerConfig())

	_ = r.LoadDefinition(FeatureDefinition{
		Name:    "evolving-group",
		Version: "v1",
	})
	r.Reconcile()

	// Modify the definition
	_ = r.LoadDefinition(FeatureDefinition{
		Name:    "evolving-group",
		Version: "v2",
	})

	results := r.Reconcile()
	found := false
	for _, res := range results {
		if res.Definition == "evolving-group" && res.Action == ActionUpdate {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected ActionUpdate for evolving-group")
	}
}

func TestReconcileNoOp(t *testing.T) {
	r := NewReconciler(DefaultReconcilerConfig())

	_ = r.LoadDefinition(FeatureDefinition{
		Name:    "stable-group",
		Version: "v1",
	})
	r.Reconcile()

	// Reconcile again without changes
	results := r.Reconcile()
	for _, res := range results {
		if res.Definition == "stable-group" && res.Action != ActionNoOp {
			t.Errorf("expected ActionNoOp, got %s", res.Action)
		}
	}
}

func TestDiff(t *testing.T) {
	r := NewReconciler(DefaultReconcilerConfig())

	_ = r.LoadDefinition(FeatureDefinition{
		Name:    "diff-group",
		Version: "v1",
	})

	entries := r.Diff()
	if len(entries) != 1 {
		t.Fatalf("expected 1 diff entry, got %d", len(entries))
	}
	if entries[0].Action != ActionCreate {
		t.Errorf("expected ActionCreate, got %s", entries[0].Action)
	}
}

func TestDeleteDefinition(t *testing.T) {
	r := NewReconciler(DefaultReconcilerConfig())

	_ = r.LoadDefinition(FeatureDefinition{Name: "to-delete", Version: "v1"})

	err := r.DeleteDefinition("to-delete")
	if err != nil {
		t.Fatalf("DeleteDefinition failed: %v", err)
	}

	defs := r.ListDefinitions()
	if len(defs) != 0 {
		t.Errorf("expected 0 definitions after delete, got %d", len(defs))
	}

	err = r.DeleteDefinition("nonexistent")
	if !errors.Is(err, ErrDefinitionNotFound) {
		t.Errorf("expected ErrDefinitionNotFound, got %v", err)
	}
}

func TestStats(t *testing.T) {
	r := NewReconciler(DefaultReconcilerConfig())

	stats := r.Stats()
	if stats.TotalDefinitions != 0 {
		t.Errorf("expected 0 definitions, got %d", stats.TotalDefinitions)
	}

	_ = r.LoadDefinition(FeatureDefinition{Name: "s1", Version: "v1"})
	_ = r.LoadDefinition(FeatureDefinition{Name: "s2", Version: "v1"})

	stats = r.Stats()
	if stats.TotalDefinitions != 2 {
		t.Errorf("expected 2 definitions, got %d", stats.TotalDefinitions)
	}
	if stats.Pending != 2 {
		t.Errorf("expected 2 pending, got %d", stats.Pending)
	}

	r.Reconcile()

	stats = r.Stats()
	if stats.Applied != 2 {
		t.Errorf("expected 2 applied, got %d", stats.Applied)
	}
	if stats.Pending != 0 {
		t.Errorf("expected 0 pending, got %d", stats.Pending)
	}
}

func TestGetHistory(t *testing.T) {
	r := NewReconciler(DefaultReconcilerConfig())

	_ = r.LoadDefinition(FeatureDefinition{Name: "h1", Version: "v1"})
	r.Reconcile()

	history := r.GetHistory(10)
	if len(history) != 1 {
		t.Errorf("expected 1 history entry, got %d", len(history))
	}
}
