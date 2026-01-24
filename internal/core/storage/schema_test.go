package storage

import (
	"testing"
	"time"

	"github.com/feather-store/feather/internal/core/domain"
)

func TestRegistry_RegisterGroup(t *testing.T) {
	r := NewRegistry()

	group := &domain.FeatureGroup{
		Name:       "user_features",
		EntityType: "user",
		TTL:        time.Hour,
		Features: []domain.FeatureSpec{
			{Name: "age", DataType: domain.DataTypeInt64},
			{Name: "name", DataType: domain.DataTypeString},
		},
	}

	err := r.RegisterGroup(group)
	if err != nil {
		t.Fatalf("RegisterGroup failed: %v", err)
	}

	got, err := r.GetGroup("user_features")
	if err != nil {
		t.Fatalf("GetGroup failed: %v", err)
	}
	if got.Name != "user_features" {
		t.Errorf("Expected group name user_features, got %s", got.Name)
	}
	if len(got.Features) != 2 {
		t.Errorf("Expected 2 features, got %d", len(got.Features))
	}
}

func TestRegistry_RegisterGroup_Duplicate(t *testing.T) {
	r := NewRegistry()

	group := &domain.FeatureGroup{
		Name:     "test",
		Features: []domain.FeatureSpec{{Name: "f1", DataType: domain.DataTypeInt64}},
	}

	err := r.RegisterGroup(group)
	if err != nil {
		t.Fatalf("First register failed: %v", err)
	}

	err = r.RegisterGroup(group)
	if err == nil {
		t.Error("Expected error for duplicate group registration")
	}
}

func TestRegistry_RegisterGroup_DuplicateFeatureName(t *testing.T) {
	r := NewRegistry()

	group1 := &domain.FeatureGroup{
		Name:     "group1",
		Features: []domain.FeatureSpec{{Name: "shared_feature", DataType: domain.DataTypeInt64}},
	}
	err := r.RegisterGroup(group1)
	if err != nil {
		t.Fatalf("First register failed: %v", err)
	}

	group2 := &domain.FeatureGroup{
		Name:     "group2",
		Features: []domain.FeatureSpec{{Name: "shared_feature", DataType: domain.DataTypeString}},
	}
	err = r.RegisterGroup(group2)
	if err == nil {
		t.Error("Expected error for duplicate feature name across groups")
	}
}

func TestRegistry_GetGroup_NotFound(t *testing.T) {
	r := NewRegistry()

	_, err := r.GetGroup("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent group")
	}
	if !domain.IsNotFound(err) {
		t.Errorf("Expected not-found error, got %v", err)
	}
}

func TestRegistry_UpdateGroup(t *testing.T) {
	r := NewRegistry()

	group := &domain.FeatureGroup{
		Name:     "updatable",
		Features: []domain.FeatureSpec{{Name: "f1", DataType: domain.DataTypeInt64}},
	}
	err := r.RegisterGroup(group)
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	updated := &domain.FeatureGroup{
		Name:     "updatable",
		Features: []domain.FeatureSpec{{Name: "f2", DataType: domain.DataTypeString}},
	}
	err = r.UpdateGroup(updated)
	if err != nil {
		t.Fatalf("UpdateGroup failed: %v", err)
	}

	// Old feature should be gone
	_, err = r.GetFeatureSpec("f1")
	if err == nil {
		t.Error("Expected f1 to be removed after update")
	}

	// New feature should exist
	spec, err := r.GetFeatureSpec("f2")
	if err != nil {
		t.Fatalf("GetFeatureSpec(f2) failed: %v", err)
	}
	if spec.DataType != domain.DataTypeString {
		t.Errorf("Expected DataTypeString, got %v", spec.DataType)
	}
}

func TestRegistry_UpdateGroup_NotFound(t *testing.T) {
	r := NewRegistry()

	err := r.UpdateGroup(&domain.FeatureGroup{Name: "nonexistent"})
	if err == nil {
		t.Error("Expected error for updating nonexistent group")
	}
}

func TestRegistry_RemoveGroup(t *testing.T) {
	r := NewRegistry()

	group := &domain.FeatureGroup{
		Name:     "removable",
		Features: []domain.FeatureSpec{{Name: "f1", DataType: domain.DataTypeInt64}},
	}
	r.RegisterGroup(group)

	err := r.RemoveGroup("removable")
	if err != nil {
		t.Fatalf("RemoveGroup failed: %v", err)
	}

	_, err = r.GetGroup("removable")
	if err == nil {
		t.Error("Expected error after removal")
	}

	_, err = r.GetFeatureSpec("f1")
	if err == nil {
		t.Error("Expected feature index to be cleaned up after group removal")
	}
}

func TestRegistry_RemoveGroup_NotFound(t *testing.T) {
	r := NewRegistry()

	err := r.RemoveGroup("nonexistent")
	if err == nil {
		t.Error("Expected error for removing nonexistent group")
	}
}

func TestRegistry_GetFeatureSpec(t *testing.T) {
	r := NewRegistry()
	r.RegisterGroup(&domain.FeatureGroup{
		Name: "grp",
		Features: []domain.FeatureSpec{
			{Name: "age", DataType: domain.DataTypeInt64},
		},
	})

	spec, err := r.GetFeatureSpec("age")
	if err != nil {
		t.Fatalf("GetFeatureSpec failed: %v", err)
	}
	if spec.Name != "age" {
		t.Errorf("Expected name=age, got %s", spec.Name)
	}
}

func TestRegistry_GetFeatureSpec_NotFound(t *testing.T) {
	r := NewRegistry()

	_, err := r.GetFeatureSpec("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent feature")
	}
}

func TestRegistry_GetFeatureGroup(t *testing.T) {
	r := NewRegistry()
	r.RegisterGroup(&domain.FeatureGroup{
		Name: "grp",
		Features: []domain.FeatureSpec{
			{Name: "age", DataType: domain.DataTypeInt64},
		},
	})

	group, err := r.GetFeatureGroup("age")
	if err != nil {
		t.Fatalf("GetFeatureGroup failed: %v", err)
	}
	if group.Name != "grp" {
		t.Errorf("Expected group name=grp, got %s", group.Name)
	}
}

func TestRegistry_GetFeatureGroup_NotFound(t *testing.T) {
	r := NewRegistry()

	_, err := r.GetFeatureGroup("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent feature")
	}
}

func TestRegistry_ListGroups(t *testing.T) {
	r := NewRegistry()
	r.RegisterGroup(&domain.FeatureGroup{Name: "g1", Features: []domain.FeatureSpec{{Name: "f1"}}})
	r.RegisterGroup(&domain.FeatureGroup{Name: "g2", Features: []domain.FeatureSpec{{Name: "f2"}}})

	groups := r.ListGroups()
	if len(groups) != 2 {
		t.Errorf("Expected 2 groups, got %d", len(groups))
	}
}

func TestRegistry_ListFeatures(t *testing.T) {
	r := NewRegistry()
	r.RegisterGroup(&domain.FeatureGroup{
		Name: "grp",
		Features: []domain.FeatureSpec{
			{Name: "f1"},
			{Name: "f2"},
		},
	})

	features := r.ListFeatures()
	if len(features) != 2 {
		t.Errorf("Expected 2 features, got %d", len(features))
	}
}

func TestRegistry_ListEntityTypes(t *testing.T) {
	r := NewRegistry()
	r.RegisterGroup(&domain.FeatureGroup{Name: "g1", EntityType: "user", Features: []domain.FeatureSpec{{Name: "f1"}}})
	r.RegisterGroup(&domain.FeatureGroup{Name: "g2", EntityType: "product", Features: []domain.FeatureSpec{{Name: "f2"}}})
	r.RegisterGroup(&domain.FeatureGroup{Name: "g3", EntityType: "user", Features: []domain.FeatureSpec{{Name: "f3"}}})

	types := r.ListEntityTypes()
	if len(types) != 2 {
		t.Errorf("Expected 2 entity types, got %d", len(types))
	}
}

func TestRegistry_ListFeaturesForEntityType(t *testing.T) {
	r := NewRegistry()
	r.RegisterGroup(&domain.FeatureGroup{Name: "g1", EntityType: "user", Features: []domain.FeatureSpec{{Name: "f1"}, {Name: "f2"}}})
	r.RegisterGroup(&domain.FeatureGroup{Name: "g2", EntityType: "product", Features: []domain.FeatureSpec{{Name: "f3"}}})

	features := r.ListFeaturesForEntityType("user")
	if len(features) != 2 {
		t.Errorf("Expected 2 user features, got %d", len(features))
	}

	features = r.ListFeaturesForEntityType("nonexistent")
	if len(features) != 0 {
		t.Errorf("Expected 0 features for nonexistent entity type, got %d", len(features))
	}
}

func TestRegistry_Validate_NumericRange(t *testing.T) {
	r := NewRegistry()
	min := 0.0
	max := 100.0
	r.RegisterGroup(&domain.FeatureGroup{
		Name: "grp",
		Features: []domain.FeatureSpec{
			{
				Name:     "score",
				DataType: domain.DataTypeFloat64,
				Validation: &domain.ValidationSpec{
					Min: &min,
					Max: &max,
				},
			},
		},
	})

	// Valid value
	if err := r.Validate("score", 50.0); err != nil {
		t.Errorf("Expected valid, got %v", err)
	}

	// Below min
	if err := r.Validate("score", -1.0); err == nil {
		t.Error("Expected error for value below min")
	}

	// Above max
	if err := r.Validate("score", 101.0); err == nil {
		t.Error("Expected error for value above max")
	}
}

func TestRegistry_Validate_NotNull(t *testing.T) {
	r := NewRegistry()
	r.RegisterGroup(&domain.FeatureGroup{
		Name: "grp",
		Features: []domain.FeatureSpec{
			{
				Name:     "required",
				DataType: domain.DataTypeString,
				Validation: &domain.ValidationSpec{
					NotNull: true,
				},
			},
		},
	})

	if err := r.Validate("required", nil); err == nil {
		t.Error("Expected error for null value")
	}
	if err := r.Validate("required", "valid"); err != nil {
		t.Errorf("Expected valid, got %v", err)
	}
}

func TestRegistry_Validate_NilValidation(t *testing.T) {
	r := NewRegistry()
	r.RegisterGroup(&domain.FeatureGroup{
		Name: "grp",
		Features: []domain.FeatureSpec{
			{Name: "novalidation", DataType: domain.DataTypeString},
		},
	})

	// No validation rules = always valid
	if err := r.Validate("novalidation", nil); err != nil {
		t.Errorf("Expected valid with no validation rules, got %v", err)
	}
}

func TestRegistry_Validate_NullPassesWhenNotRequired(t *testing.T) {
	r := NewRegistry()
	min := 0.0
	r.RegisterGroup(&domain.FeatureGroup{
		Name: "grp",
		Features: []domain.FeatureSpec{
			{
				Name:     "optional",
				DataType: domain.DataTypeFloat64,
				Validation: &domain.ValidationSpec{
					Min: &min,
				},
			},
		},
	})

	// nil should pass when NotNull is false
	if err := r.Validate("optional", nil); err != nil {
		t.Errorf("Expected nil to pass when NotNull is false, got %v", err)
	}
}

func TestRegistry_Validate_OneOf(t *testing.T) {
	r := NewRegistry()
	r.RegisterGroup(&domain.FeatureGroup{
		Name: "grp",
		Features: []domain.FeatureSpec{
			{
				Name:     "status",
				DataType: domain.DataTypeString,
				Validation: &domain.ValidationSpec{
					OneOf: []string{"active", "inactive"},
				},
			},
		},
	})

	if err := r.Validate("status", "active"); err != nil {
		t.Errorf("Expected valid, got %v", err)
	}
	if err := r.Validate("status", "unknown"); err == nil {
		t.Error("Expected error for value not in OneOf")
	}
}

func TestRegistry_Validate_TypeMismatch(t *testing.T) {
	r := NewRegistry()
	r.RegisterGroup(&domain.FeatureGroup{
		Name: "grp",
		Features: []domain.FeatureSpec{
			{Name: "num", DataType: domain.DataTypeFloat64, Validation: &domain.ValidationSpec{NotNull: true}},
			{Name: "str", DataType: domain.DataTypeString, Validation: &domain.ValidationSpec{NotNull: true}},
			{Name: "flag", DataType: domain.DataTypeBool, Validation: &domain.ValidationSpec{NotNull: true}},
			{Name: "data", DataType: domain.DataTypeBytes, Validation: &domain.ValidationSpec{NotNull: true}},
			{Name: "vec", DataType: domain.DataTypeVector, Validation: &domain.ValidationSpec{NotNull: true}},
			{Name: "ts", DataType: domain.DataTypeTimestamp, Validation: &domain.ValidationSpec{NotNull: true}},
		},
	})

	// Wrong types should fail
	if err := r.Validate("num", "not a number"); err == nil {
		t.Error("Expected error for string when float64 expected")
	}
	if err := r.Validate("str", 42); err == nil {
		t.Error("Expected error for int when string expected")
	}
	if err := r.Validate("flag", "not a bool"); err == nil {
		t.Error("Expected error for string when bool expected")
	}
	if err := r.Validate("data", "not bytes"); err == nil {
		t.Error("Expected error for string when bytes expected")
	}
	if err := r.Validate("vec", "not a vector"); err == nil {
		t.Error("Expected error for string when vector expected")
	}
	if err := r.Validate("ts", 42); err == nil {
		t.Error("Expected error for int when timestamp expected")
	}

	// Correct types should pass
	if err := r.Validate("num", 42.0); err != nil {
		t.Errorf("Expected valid float64, got %v", err)
	}
	if err := r.Validate("num", int64(42)); err != nil {
		t.Errorf("Expected valid int64 for numeric, got %v", err)
	}
	if err := r.Validate("str", "hello"); err != nil {
		t.Errorf("Expected valid string, got %v", err)
	}
	if err := r.Validate("flag", true); err != nil {
		t.Errorf("Expected valid bool, got %v", err)
	}
	if err := r.Validate("data", []byte{1, 2, 3}); err != nil {
		t.Errorf("Expected valid bytes, got %v", err)
	}
	if err := r.Validate("vec", []float32{1.0, 2.0}); err != nil {
		t.Errorf("Expected valid vector, got %v", err)
	}
	if err := r.Validate("ts", time.Now()); err != nil {
		t.Errorf("Expected valid time.Time for timestamp, got %v", err)
	}
	if err := r.Validate("ts", int64(1234567890)); err != nil {
		t.Errorf("Expected valid int64 for timestamp, got %v", err)
	}
	if err := r.Validate("ts", "2024-01-01T00:00:00Z"); err != nil {
		t.Errorf("Expected valid string for timestamp, got %v", err)
	}
}

func TestRegistry_Validate_NonexistentFeature(t *testing.T) {
	r := NewRegistry()

	err := r.Validate("nonexistent", "value")
	if err == nil {
		t.Error("Expected error for nonexistent feature")
	}
}

func TestRegistry_Validate_IntForNumeric(t *testing.T) {
	r := NewRegistry()
	min := 0.0
	r.RegisterGroup(&domain.FeatureGroup{
		Name: "grp",
		Features: []domain.FeatureSpec{
			{
				Name:     "count",
				DataType: domain.DataTypeInt64,
				Validation: &domain.ValidationSpec{
					Min: &min,
				},
			},
		},
	})

	if err := r.Validate("count", int64(5)); err != nil {
		t.Errorf("Expected valid int64, got %v", err)
	}
	if err := r.Validate("count", int(10)); err != nil {
		t.Errorf("Expected valid int, got %v", err)
	}
	if err := r.Validate("count", int64(-1)); err == nil {
		t.Error("Expected error for value below min")
	}
}
