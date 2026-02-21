package crd

import "testing"

func TestFeatherClusterDefaults(t *testing.T) {
	spec := DefaultFeatherClusterSpec()
	if spec.Replicas != 1 {
		t.Errorf("got replicas %d, want 1", spec.Replicas)
	}
	if spec.Storage.HotMemoryGB != 4 {
		t.Errorf("got hot memory %d GB, want 4", spec.Storage.HotMemoryGB)
	}
	if spec.Storage.WarmDiskGB != 50 {
		t.Errorf("got warm disk %d GB, want 50", spec.Storage.WarmDiskGB)
	}
	if spec.Storage.StorageClass != "standard" {
		t.Errorf("got storage class %q, want %q", spec.Storage.StorageClass, "standard")
	}
	if spec.Image == "" {
		t.Error("default image should not be empty")
	}
}

func TestFeatureGroupValidation(t *testing.T) {
	// empty spec should produce errors
	errs := ValidateFeatureGroup(FeatureGroupSpec{})
	if len(errs) == 0 {
		t.Error("expected validation errors for empty spec")
	}

	// valid spec
	errs = ValidateFeatureGroup(FeatureGroupSpec{
		Name:       "user_features",
		EntityType: "user",
		Features: []FeatureFieldSpec{
			{Name: "login_count", Type: "INT64"},
		},
	})
	if len(errs) != 0 {
		t.Errorf("unexpected validation errors: %v", errs)
	}

	// feature with missing type
	errs = ValidateFeatureGroup(FeatureGroupSpec{
		Name:       "test",
		EntityType: "user",
		Features: []FeatureFieldSpec{
			{Name: "f1", Type: ""},
		},
	})
	hasTypeErr := false
	for _, e := range errs {
		if e == "feature type is required for f1" {
			hasTypeErr = true
		}
	}
	if !hasTypeErr {
		t.Errorf("expected type error, got: %v", errs)
	}
}
