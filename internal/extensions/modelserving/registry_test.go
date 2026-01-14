package modelserving

import (
	"errors"
	"testing"
)

func newTestModel(id, name string) *Model {
	return &Model{
		ID:        id,
		Name:      name,
		Framework: "mlflow",
		Status:    ModelStatusActive,
	}
}

func newTestVersion(modelID string, version int, features []string) *ModelVersion {
	return &ModelVersion{
		ModelID:    modelID,
		Version:    version,
		Features:   features,
		EntityType: "user",
		Status:     ModelStatusActive,
	}
}

func TestRegistry_RegisterModel(t *testing.T) {
	reg := NewRegistry(DefaultRegistryConfig())
	model := newTestModel("m1", "fraud-detector")

	if err := reg.RegisterModel(model); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, err := reg.GetModel("m1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Name != "fraud-detector" {
		t.Errorf("got name %q, want %q", got.Name, "fraud-detector")
	}
	if got.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be set")
	}
}

func TestRegistry_RegisterModel_Duplicate(t *testing.T) {
	reg := NewRegistry(DefaultRegistryConfig())
	model := newTestModel("m1", "fraud-detector")

	if err := reg.RegisterModel(model); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err := reg.RegisterModel(newTestModel("m1", "another"))
	if err == nil {
		t.Fatal("expected error for duplicate model")
	}
	if !errors.Is(err, ErrAlreadyExists) {
		t.Errorf("got error %v, want %v", err, ErrAlreadyExists)
	}
}

func TestRegistry_RegisterVersion(t *testing.T) {
	reg := NewRegistry(DefaultRegistryConfig())
	if err := reg.RegisterModel(newTestModel("m1", "fraud-detector")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	v := newTestVersion("m1", 1, []string{"amount", "ip_country"})
	if err := reg.RegisterVersion(v); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, err := reg.GetVersion("m1", 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Version != 1 {
		t.Errorf("got version %d, want 1", got.Version)
	}
	if len(got.Features) != 2 {
		t.Errorf("got %d features, want 2", len(got.Features))
	}
}

func TestRegistry_GetLatestVersion(t *testing.T) {
	reg := NewRegistry(DefaultRegistryConfig())
	if err := reg.RegisterModel(newTestModel("m1", "fraud-detector")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, v := range []int{1, 3, 2} {
		if err := reg.RegisterVersion(newTestVersion("m1", v, []string{"f1"})); err != nil {
			t.Fatalf("unexpected error registering v%d: %v", v, err)
		}
	}

	latest, err := reg.GetLatestVersion("m1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if latest.Version != 3 {
		t.Errorf("got version %d, want 3", latest.Version)
	}
}

func TestRegistry_ResolveFeatures(t *testing.T) {
	reg := NewRegistry(DefaultRegistryConfig())
	if err := reg.RegisterModel(newTestModel("m1", "fraud-detector")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	features := []string{"amount", "ip_country", "device_type"}
	if err := reg.RegisterVersion(newTestVersion("m1", 1, features)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	values := map[string]interface{}{
		"amount":      99.99,
		"ip_country":  "US",
		"device_type": "mobile",
	}
	bundle, err := reg.ResolveFeatures("m1", 1, "user:123", values)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if bundle.ModelID != "m1" {
		t.Errorf("got model ID %q, want %q", bundle.ModelID, "m1")
	}
	if len(bundle.Features) != 3 {
		t.Errorf("got %d features, want 3", len(bundle.Features))
	}
	if len(bundle.MissingFeatures) != 0 {
		t.Errorf("got %d missing features, want 0", len(bundle.MissingFeatures))
	}
	if bundle.CacheHit {
		t.Error("expected first resolve to not be a cache hit")
	}
}

func TestRegistry_ResolveFeatures_Missing(t *testing.T) {
	reg := NewRegistry(DefaultRegistryConfig())
	if err := reg.RegisterModel(newTestModel("m1", "fraud-detector")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	features := []string{"amount", "ip_country", "device_type"}
	if err := reg.RegisterVersion(newTestVersion("m1", 1, features)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Only provide one of three features.
	values := map[string]interface{}{
		"amount": 50.0,
	}
	bundle, err := reg.ResolveFeatures("m1", 1, "user:456", values)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(bundle.MissingFeatures) != 2 {
		t.Errorf("got %d missing features, want 2", len(bundle.MissingFeatures))
	}
	if len(bundle.Features) != 1 {
		t.Errorf("got %d resolved features, want 1", len(bundle.Features))
	}
}

func TestRegistry_RemoveModel(t *testing.T) {
	reg := NewRegistry(DefaultRegistryConfig())
	if err := reg.RegisterModel(newTestModel("m1", "fraud-detector")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := reg.RegisterVersion(newTestVersion("m1", 1, []string{"f1"})); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := reg.RemoveModel("m1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err := reg.GetModel("m1")
	if !errors.Is(err, ErrModelNotFound) {
		t.Errorf("expected ErrModelNotFound after removal, got: %v", err)
	}

	versions := reg.ListVersions("m1")
	if len(versions) != 0 {
		t.Errorf("expected 0 versions after removal, got %d", len(versions))
	}
}

func TestRegistry_RemoveModel_NotFound(t *testing.T) {
	reg := NewRegistry(DefaultRegistryConfig())

	err := reg.RemoveModel("nonexistent")
	if !errors.Is(err, ErrModelNotFound) {
		t.Errorf("expected ErrModelNotFound, got: %v", err)
	}
}

func TestRegistry_Stats(t *testing.T) {
	reg := NewRegistry(DefaultRegistryConfig())

	for _, id := range []string{"m1", "m2"} {
		if err := reg.RegisterModel(newTestModel(id, id+"-name")); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}
	if err := reg.RegisterVersion(newTestVersion("m1", 1, []string{"f1"})); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := reg.RegisterVersion(newTestVersion("m1", 2, []string{"f1", "f2"})); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := reg.RegisterVersion(newTestVersion("m2", 1, []string{"f3"})); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	stats := reg.Stats()
	if stats.ModelCount != 2 {
		t.Errorf("got model count %d, want 2", stats.ModelCount)
	}
	if stats.VersionCount != 3 {
		t.Errorf("got version count %d, want 3", stats.VersionCount)
	}

	// Trigger a resolve to populate cache and test hit rate.
	if _, err := reg.ResolveFeatures("m1", 1, "user:1", map[string]interface{}{"f1": 1}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Second call should be a cache hit.
	bundle, err := reg.ResolveFeatures("m1", 1, "user:1", map[string]interface{}{"f1": 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bundle.CacheHit {
		t.Error("expected second resolve to be a cache hit")
	}

	stats = reg.Stats()
	if stats.BundleCount != 1 {
		t.Errorf("got bundle count %d, want 1", stats.BundleCount)
	}
	if stats.CacheHitRate != 0.5 {
		t.Errorf("got cache hit rate %f, want 0.5", stats.CacheHitRate)
	}
}
