package gitops

import (
	"testing"
	"time"
)

func TestReconciler_NewAndStatus(t *testing.T) {
	r := NewReconciler(nil, DefaultReconcilerConfig())

	if r.GetStatus() != ReconcileIdle {
		t.Fatalf("expected idle, got %s", r.GetStatus())
	}

	if r.GetVersion() != "v0" {
		t.Fatalf("expected v0, got %s", r.GetVersion())
	}
}

func TestReconciler_ApplyMigration(t *testing.T) {
	r := NewReconciler(nil, DefaultReconcilerConfig())

	step := MigrationStep{
		FromVersion: "v0",
		ToVersion:   "v1",
		Description: "Add click_count feature",
		Changes: []SchemaChange{
			{Type: "add_feature", Group: "user_engagement", Feature: "click_count"},
		},
	}

	if err := r.ApplyMigration(step); err != nil {
		t.Fatal(err)
	}

	if r.GetVersion() != "v1" {
		t.Fatalf("expected v1, got %s", r.GetVersion())
	}

	migrations := r.GetMigrations()
	if len(migrations) != 1 {
		t.Fatalf("expected 1 migration, got %d", len(migrations))
	}
	if migrations[0].Status != "applied" {
		t.Fatalf("expected applied, got %s", migrations[0].Status)
	}
	if migrations[0].AppliedAt.IsZero() {
		t.Fatal("expected non-zero applied timestamp")
	}
}

func TestReconciler_History(t *testing.T) {
	r := NewReconciler(nil, DefaultReconcilerConfig())

	// Manually add events
	r.mu.Lock()
	r.history = append(r.history, ReconcileEvent{
		StartedAt: time.Now().Add(-time.Minute),
		EndedAt:   time.Now(),
		Status:    ReconcileSuccess,
		Changes:   5,
	})
	r.mu.Unlock()

	history := r.GetHistory(10)
	if len(history) != 1 {
		t.Fatalf("expected 1 event, got %d", len(history))
	}
}

func TestComputeSchemaDiff(t *testing.T) {
	old := []FeatureDefinition{
		{Metadata: DefinitionMeta{Name: "user_clicks"}, Spec: FeatureSpec{EntityType: "user"}},
		{Metadata: DefinitionMeta{Name: "user_spend"}, Spec: FeatureSpec{EntityType: "user"}},
		{Metadata: DefinitionMeta{Name: "removed_group"}, Spec: FeatureSpec{EntityType: "user"}},
	}

	new := []FeatureDefinition{
		{Metadata: DefinitionMeta{Name: "user_clicks"}, Spec: FeatureSpec{EntityType: "user"}},  // unchanged
		{Metadata: DefinitionMeta{Name: "user_spend"}, Spec: FeatureSpec{EntityType: "order"}},  // modified
		{Metadata: DefinitionMeta{Name: "new_group"}, Spec: FeatureSpec{EntityType: "product"}}, // added
	}

	changes := ComputeSchemaDiff(old, new)

	typeCount := make(map[string]int)
	for _, c := range changes {
		typeCount[c.Type]++
	}

	if typeCount["add_group"] != 1 {
		t.Fatalf("expected 1 addition, got %d", typeCount["add_group"])
	}
	if typeCount["modify_type"] != 1 {
		t.Fatalf("expected 1 modification, got %d", typeCount["modify_type"])
	}
	if typeCount["remove_group"] != 1 {
		t.Fatalf("expected 1 removal, got %d", typeCount["remove_group"])
	}
}

func TestReconciler_Summary(t *testing.T) {
	r := NewReconciler(nil, DefaultReconcilerConfig())
	r.ApplyMigration(MigrationStep{
		FromVersion: "v0",
		ToVersion:   "v1",
	})

	summary := r.Summary()
	if summary["version"] != "v1" {
		t.Fatal("expected v1 in summary")
	}
	if summary["total_migrations"] != 1 {
		t.Fatalf("expected 1 migration, got %v", summary["total_migrations"])
	}
}
