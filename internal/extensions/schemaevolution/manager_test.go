package schemaevolution

import (
	"testing"
)

func TestRegisterSchema(t *testing.T) {
	m := NewManager(DefaultManagerConfig())
	sv, err := m.RegisterSchema("users", map[string]string{"age": "int64", "name": "string"})
	if err != nil {
		t.Fatal(err)
	}
	if sv.Version != 1 {
		t.Errorf("expected version 1, got %d", sv.Version)
	}
}

func TestEvolveBackwardCompatible(t *testing.T) {
	m := NewManager(DefaultManagerConfig())
	_, _ = m.RegisterSchema("users", map[string]string{"age": "int64", "name": "string"})

	// Add a field (backward compatible)
	mig, err := m.Evolve("users", map[string]string{"age": "int64", "name": "string", "email": "string"}, map[string]string{"email": ""})
	if err != nil {
		t.Fatal(err)
	}
	if mig.Status != MigrationCompleted {
		t.Errorf("expected completed, got %s", mig.Status)
	}

	sv, _ := m.GetSchema("users")
	if sv.Version != 2 {
		t.Errorf("expected version 2, got %d", sv.Version)
	}
}

func TestEvolveIncompatible(t *testing.T) {
	m := NewManager(ManagerConfig{
		DefaultCompatibility: CompatFull,
		MaxVersionsPerGroup:  100,
		MaxMigrations:        100,
	})
	_, _ = m.RegisterSchema("users", map[string]string{"age": "int64", "name": "string"})

	// Remove a field (breaks forward compatibility in full mode)
	_, err := m.Evolve("users", map[string]string{"age": "int64"}, nil)
	if err == nil {
		t.Fatal("expected error for incompatible change")
	}
}

func TestTypeCoercion(t *testing.T) {
	m := NewManager(DefaultManagerConfig())
	_, _ = m.RegisterSchema("metrics", map[string]string{"value": "int64"})

	// int64 -> float64 is coercible
	mig, err := m.Evolve("metrics", map[string]string{"value": "float64"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !mig.Compatible {
		t.Error("expected compatible migration for int64->float64")
	}
}

func TestRollback(t *testing.T) {
	m := NewManager(DefaultManagerConfig())
	_, _ = m.RegisterSchema("users", map[string]string{"age": "int64"})
	_, _ = m.Evolve("users", map[string]string{"age": "int64", "name": "string"}, map[string]string{"name": ""})

	if err := m.Rollback("users"); err != nil {
		t.Fatal(err)
	}

	sv, _ := m.GetSchema("users")
	if sv.Version != 1 {
		t.Errorf("expected version 1 after rollback, got %d", sv.Version)
	}
}

func TestCheckCompatibility(t *testing.T) {
	m := NewManager(DefaultManagerConfig())
	_, _ = m.RegisterSchema("test", map[string]string{"x": "string"})

	report, err := m.CheckCompatibility("test", map[string]string{"x": "int64"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if report.Compatible {
		t.Error("expected incompatible for string->int64")
	}
}

func TestStats(t *testing.T) {
	m := NewManager(DefaultManagerConfig())
	_, _ = m.RegisterSchema("a", map[string]string{"x": "int64"})
	_, _ = m.RegisterSchema("b", map[string]string{"y": "string"})

	stats := m.Stats()
	if stats.TotalGroups != 2 {
		t.Errorf("expected 2 groups, got %d", stats.TotalGroups)
	}
}
