package operator

import (
	"testing"
)

func TestMigrationGenerator_CreateGroup(t *testing.T) {
	g := NewMigrationGenerator()
	newGroup := &FeatureGroup{
		ObjectMeta: ObjectMeta{Name: "user-features"},
		Spec: FeatureGroupSpec{
			Features: []FeatureSpec{
				{Name: "age", Type: "int"},
				{Name: "name", Type: "string"},
			},
		},
	}

	m, err := g.GenerateMigration(nil, newGroup)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// 1 add_group + 2 add_field
	if len(m.Operations) != 3 {
		t.Errorf("expected 3 operations, got %d", len(m.Operations))
	}
	if m.Operations[0].Operation != MigrationAddGroup {
		t.Errorf("expected add_group, got %s", m.Operations[0].Operation)
	}
}

func TestMigrationGenerator_RemoveGroup(t *testing.T) {
	g := NewMigrationGenerator()
	oldGroup := &FeatureGroup{
		ObjectMeta: ObjectMeta{Name: "old-features"},
	}

	m, err := g.GenerateMigration(oldGroup, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(m.Operations) != 1 {
		t.Errorf("expected 1 operation, got %d", len(m.Operations))
	}
	if m.Operations[0].Operation != MigrationRemoveGroup {
		t.Errorf("expected remove_group, got %s", m.Operations[0].Operation)
	}
}

func TestMigrationGenerator_UpdateGroup(t *testing.T) {
	g := NewMigrationGenerator()
	oldGroup := &FeatureGroup{
		ObjectMeta: ObjectMeta{Name: "features"},
		Spec: FeatureGroupSpec{
			Features: []FeatureSpec{
				{Name: "age", Type: "int"},
				{Name: "email", Type: "string"},
			},
		},
	}
	newGroup := &FeatureGroup{
		ObjectMeta: ObjectMeta{Name: "features"},
		Spec: FeatureGroupSpec{
			Features: []FeatureSpec{
				{Name: "age", Type: "float"},  // changed type
				{Name: "score", Type: "float"}, // new field
			},
		},
	}

	m, err := g.GenerateMigration(oldGroup, newGroup)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var adds, removes, changes int
	for _, op := range m.Operations {
		switch op.Operation {
		case MigrationAddField:
			adds++
		case MigrationRemoveField:
			removes++
		case MigrationChangeType:
			changes++
		}
	}
	if adds != 1 {
		t.Errorf("expected 1 add, got %d", adds)
	}
	if removes != 1 {
		t.Errorf("expected 1 remove, got %d", removes)
	}
	if changes != 1 {
		t.Errorf("expected 1 type change, got %d", changes)
	}
}

func TestMigrationGenerator_BothNil(t *testing.T) {
	g := NewMigrationGenerator()
	_, err := g.GenerateMigration(nil, nil)
	if err == nil {
		t.Fatal("expected error when both groups nil")
	}
}

func TestMigrationGenerator_DryRun(t *testing.T) {
	g := NewMigrationGenerator()
	newGroup := &FeatureGroup{
		ObjectMeta: ObjectMeta{Name: "test"},
		Spec:       FeatureGroupSpec{Features: []FeatureSpec{{Name: "x", Type: "int"}}},
	}

	m, err := g.DryRun(nil, newGroup)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !m.DryRun {
		t.Error("expected DryRun=true")
	}
}

func TestMigrationGenerator_Apply(t *testing.T) {
	g := NewMigrationGenerator()
	newGroup := &FeatureGroup{
		ObjectMeta: ObjectMeta{Name: "test"},
		Spec:       FeatureGroupSpec{Features: []FeatureSpec{{Name: "x", Type: "int"}}},
	}

	m, _ := g.GenerateMigration(nil, newGroup)
	if err := g.Apply(m.ID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	migrations := g.ListMigrations()
	for _, mig := range migrations {
		if mig.ID == m.ID && !mig.Applied {
			t.Error("expected migration to be applied")
		}
	}
}

func TestMigrationGenerator_ApplyNotFound(t *testing.T) {
	g := NewMigrationGenerator()
	err := g.Apply("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent migration")
	}
}

func TestMigrationGenerator_ListMigrations(t *testing.T) {
	g := NewMigrationGenerator()
	group := &FeatureGroup{
		ObjectMeta: ObjectMeta{Name: "test"},
		Spec:       FeatureGroupSpec{Features: []FeatureSpec{{Name: "x", Type: "int"}}},
	}
	_, _ = g.GenerateMigration(nil, group)
	_, _ = g.GenerateMigration(group, nil)

	list := g.ListMigrations()
	if len(list) != 2 {
		t.Errorf("expected 2 migrations, got %d", len(list))
	}
}

func TestMigrationGenerator_ChangeTypeNotReversible(t *testing.T) {
	g := NewMigrationGenerator()
	oldGroup := &FeatureGroup{
		ObjectMeta: ObjectMeta{Name: "test"},
		Spec:       FeatureGroupSpec{Features: []FeatureSpec{{Name: "x", Type: "int"}}},
	}
	newGroup := &FeatureGroup{
		ObjectMeta: ObjectMeta{Name: "test"},
		Spec:       FeatureGroupSpec{Features: []FeatureSpec{{Name: "x", Type: "string"}}},
	}

	m, _ := g.GenerateMigration(oldGroup, newGroup)
	for _, op := range m.Operations {
		if op.Operation == MigrationChangeType && op.Reversible {
			t.Error("change_type operations should not be reversible")
		}
	}
}
