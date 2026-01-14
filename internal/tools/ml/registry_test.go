package ml

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewModelRegistry(t *testing.T) {
	registry := NewModelRegistry()
	assert.NotNil(t, registry)
	assert.Empty(t, registry.ListModels())
}

func TestModelRegistry_RegisterModel(t *testing.T) {
	registry := NewModelRegistry()

	model := &Model{
		ID:          "model-1",
		Name:        "Test Model",
		Description: "A test model",
		Team:        "ml-team",
		Owner:       "owner@example.com",
		Tags:        []string{"production", "v1"},
	}

	err := registry.RegisterModel(model)
	require.NoError(t, err)

	// Verify model was registered
	retrieved, err := registry.GetModel("model-1")
	require.NoError(t, err)
	assert.Equal(t, "model-1", retrieved.ID)
	assert.Equal(t, "Test Model", retrieved.Name)
	assert.NotZero(t, retrieved.CreatedAt)
	assert.NotZero(t, retrieved.UpdatedAt)
}

func TestModelRegistry_RegisterModel_Duplicate(t *testing.T) {
	registry := NewModelRegistry()

	model := &Model{ID: "model-1", Name: "Test"}
	err := registry.RegisterModel(model)
	require.NoError(t, err)

	// Registering again should fail
	err = registry.RegisterModel(&Model{ID: "model-1", Name: "Test2"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}

func TestModelRegistry_RegisterModel_EmptyID(t *testing.T) {
	registry := NewModelRegistry()

	model := &Model{Name: "Test"}
	err := registry.RegisterModel(model)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ID is required")
}

func TestModelRegistry_RegisterModel_EmptyName(t *testing.T) {
	registry := NewModelRegistry()

	model := &Model{ID: "model-1"}
	err := registry.RegisterModel(model)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "name is required")
}

func TestModelRegistry_GetModel_NotFound(t *testing.T) {
	registry := NewModelRegistry()

	_, err := registry.GetModel("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestModelRegistry_GetModelByName(t *testing.T) {
	registry := NewModelRegistry()

	model := &Model{ID: "model-1", Name: "My Model"}
	err := registry.RegisterModel(model)
	require.NoError(t, err)

	retrieved, err := registry.GetModelByName("My Model")
	require.NoError(t, err)
	assert.Equal(t, "model-1", retrieved.ID)
}

func TestModelRegistry_GetModelByName_NotFound(t *testing.T) {
	registry := NewModelRegistry()

	_, err := registry.GetModelByName("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestModelRegistry_DeleteModel(t *testing.T) {
	registry := NewModelRegistry()

	model := &Model{ID: "model-1", Name: "Test"}
	err := registry.RegisterModel(model)
	require.NoError(t, err)

	err = registry.DeleteModel("model-1")
	require.NoError(t, err)

	_, err = registry.GetModel("model-1")
	assert.Error(t, err)
}

func TestModelRegistry_DeleteModel_NotFound(t *testing.T) {
	registry := NewModelRegistry()

	err := registry.DeleteModel("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestModelRegistry_ListModels(t *testing.T) {
	registry := NewModelRegistry()

	for i := 0; i < 3; i++ {
		model := &Model{ID: "model-" + string(rune('1'+i)), Name: "Test" + string(rune('1'+i))}
		err := registry.RegisterModel(model)
		require.NoError(t, err)
	}

	models := registry.ListModels()
	assert.Len(t, models, 3)
}

func TestModelRegistry_RegisterVersion(t *testing.T) {
	registry := NewModelRegistry()

	model := &Model{ID: "model-1", Name: "Test"}
	err := registry.RegisterModel(model)
	require.NoError(t, err)

	version := &ModelVersion{
		Version:         "v1.0.0",
		Features:        []string{"feature_a", "feature_b"},
		ServingEndpoint: "http://localhost:8080/predict",
	}

	err = registry.RegisterVersion("model-1", version)
	require.NoError(t, err)

	// Verify version was registered
	retrieved, err := registry.GetVersion("model-1", "v1.0.0")
	require.NoError(t, err)
	assert.Equal(t, "v1.0.0", retrieved.Version)
	assert.Equal(t, []string{"feature_a", "feature_b"}, retrieved.Features)
	assert.NotZero(t, retrieved.CreatedAt)
	assert.Equal(t, ModelStatusDraft, retrieved.Status)
}

func TestModelRegistry_RegisterVersion_ModelNotFound(t *testing.T) {
	registry := NewModelRegistry()

	version := &ModelVersion{Version: "v1.0.0", Features: []string{"f1"}}
	err := registry.RegisterVersion("nonexistent", version)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestModelRegistry_RegisterVersion_Duplicate(t *testing.T) {
	registry := NewModelRegistry()

	model := &Model{ID: "model-1", Name: "Test"}
	registry.RegisterModel(model)

	version := &ModelVersion{Version: "v1.0.0", Features: []string{"f1"}}
	err := registry.RegisterVersion("model-1", version)
	require.NoError(t, err)

	err = registry.RegisterVersion("model-1", &ModelVersion{Version: "v1.0.0", Features: []string{"f2"}})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}

func TestModelRegistry_RegisterVersion_EmptyFeatures(t *testing.T) {
	registry := NewModelRegistry()

	model := &Model{ID: "model-1", Name: "Test"}
	registry.RegisterModel(model)

	version := &ModelVersion{Version: "v1.0.0", Features: []string{}}
	err := registry.RegisterVersion("model-1", version)
	assert.Error(t, err)
}

func TestModelRegistry_GetVersion_NotFound(t *testing.T) {
	registry := NewModelRegistry()

	model := &Model{ID: "model-1", Name: "Test"}
	registry.RegisterModel(model)

	_, err := registry.GetVersion("model-1", "nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestModelRegistry_ActivateVersion(t *testing.T) {
	registry := NewModelRegistry()

	model := &Model{ID: "model-1", Name: "Test"}
	registry.RegisterModel(model)

	v1 := &ModelVersion{Version: "v1.0", Features: []string{"f1"}}
	v2 := &ModelVersion{Version: "v2.0", Features: []string{"f1", "f2"}}
	registry.RegisterVersion("model-1", v1)
	registry.RegisterVersion("model-1", v2)

	err := registry.ActivateVersion("model-1", "v2.0")
	require.NoError(t, err)

	active, err := registry.GetActiveVersion("model-1")
	require.NoError(t, err)
	assert.Equal(t, "v2.0", active.Version)
	assert.Equal(t, ModelStatusProduction, active.Status)
	assert.NotNil(t, active.ActivatedAt)
}

func TestModelRegistry_ActivateVersion_DeactivatesPrevious(t *testing.T) {
	registry := NewModelRegistry()

	model := &Model{ID: "model-1", Name: "Test"}
	registry.RegisterModel(model)

	v1 := &ModelVersion{Version: "v1.0", Features: []string{"f1"}}
	v2 := &ModelVersion{Version: "v2.0", Features: []string{"f1"}}
	registry.RegisterVersion("model-1", v1)
	registry.RegisterVersion("model-1", v2)

	// Activate v1
	registry.ActivateVersion("model-1", "v1.0")

	// Activate v2 (should deactivate v1)
	registry.ActivateVersion("model-1", "v2.0")

	// Check v1 is archived
	v1Retrieved, _ := registry.GetVersion("model-1", "v1.0")
	assert.Equal(t, ModelStatusArchived, v1Retrieved.Status)
	assert.NotNil(t, v1Retrieved.ArchivedAt)
}

func TestModelRegistry_GetActiveVersion_NotSet(t *testing.T) {
	registry := NewModelRegistry()

	model := &Model{ID: "model-1", Name: "Test"}
	registry.RegisterModel(model)

	_, err := registry.GetActiveVersion("model-1")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no active")
}

func TestModelRegistry_GetFeaturesForModel(t *testing.T) {
	registry := NewModelRegistry()

	model := &Model{ID: "model-1", Name: "Test"}
	registry.RegisterModel(model)

	version := &ModelVersion{
		Version:  "v1.0",
		Features: []string{"feature_a", "feature_b", "feature_c"},
	}
	registry.RegisterVersion("model-1", version)
	registry.ActivateVersion("model-1", "v1.0")

	features, err := registry.GetFeaturesForModel("model-1")
	require.NoError(t, err)
	assert.Equal(t, []string{"feature_a", "feature_b", "feature_c"}, features)
}

func TestModelRegistry_GetFeaturesForModel_NoActiveVersion(t *testing.T) {
	registry := NewModelRegistry()

	model := &Model{ID: "model-1", Name: "Test"}
	registry.RegisterModel(model)

	_, err := registry.GetFeaturesForModel("model-1")
	assert.Error(t, err)
}

func TestModelRegistry_GetModelsUsingFeature(t *testing.T) {
	registry := NewModelRegistry()

	// Create multiple models with overlapping features
	for _, m := range []struct {
		id       string
		name     string
		features []string
	}{
		{"model-1", "Model1", []string{"feature_a", "feature_b"}},
		{"model-2", "Model2", []string{"feature_b", "feature_c"}},
		{"model-3", "Model3", []string{"feature_d"}},
	} {
		model := &Model{ID: m.id, Name: m.name}
		registry.RegisterModel(model)
		version := &ModelVersion{Version: "v1", Features: m.features}
		registry.RegisterVersion(m.id, version)
	}

	// Find models using feature_b
	models := registry.GetModelsUsingFeature("feature_b")
	assert.Len(t, models, 2)

	// Find models using feature_d
	models = registry.GetModelsUsingFeature("feature_d")
	assert.Len(t, models, 1)
	assert.Equal(t, "model-3", models[0].ID)

	// Find models using non-existent feature
	models = registry.GetModelsUsingFeature("nonexistent")
	assert.Empty(t, models)
}

func TestModelRegistry_Stats(t *testing.T) {
	registry := NewModelRegistry()

	model := &Model{ID: "model-1", Name: "Test"}
	registry.RegisterModel(model)

	v1 := &ModelVersion{Version: "v1.0", Features: []string{"a", "b"}}
	v2 := &ModelVersion{Version: "v2.0", Features: []string{"a", "b", "c"}}
	registry.RegisterVersion("model-1", v1)
	registry.RegisterVersion("model-1", v2)
	registry.ActivateVersion("model-1", "v1.0")

	stats := registry.Stats()
	assert.Equal(t, 1, stats["total_models"])
	assert.Equal(t, 1, stats["active_models"])
	assert.Equal(t, 2, stats["total_versions"])
}

func TestModelRegistry_Callbacks(t *testing.T) {
	registry := NewModelRegistry()

	modelRegistered := false
	versionActivated := false
	versionDeactivated := false

	registry.OnModelRegistered(func(m *Model) {
		modelRegistered = true
	})
	registry.OnVersionActivated(func(m *Model, v *ModelVersion) {
		versionActivated = true
	})
	registry.OnVersionDeactivated(func(m *Model, v *ModelVersion) {
		versionDeactivated = true
	})

	model := &Model{ID: "model-1", Name: "Test"}
	registry.RegisterModel(model)
	assert.True(t, modelRegistered)

	v1 := &ModelVersion{Version: "v1.0", Features: []string{"f1"}}
	v2 := &ModelVersion{Version: "v2.0", Features: []string{"f1"}}
	registry.RegisterVersion("model-1", v1)
	registry.RegisterVersion("model-1", v2)

	registry.ActivateVersion("model-1", "v1.0")
	assert.True(t, versionActivated)

	registry.ActivateVersion("model-1", "v2.0")
	assert.True(t, versionDeactivated)
}

func TestModelRegistry_DeleteModel_CleansFeatureIndex(t *testing.T) {
	registry := NewModelRegistry()

	model := &Model{ID: "model-1", Name: "Test"}
	registry.RegisterModel(model)
	version := &ModelVersion{Version: "v1.0", Features: []string{"unique_feature"}}
	registry.RegisterVersion("model-1", version)

	// Verify feature is indexed
	models := registry.GetModelsUsingFeature("unique_feature")
	assert.Len(t, models, 1)

	// Delete model
	registry.DeleteModel("model-1")

	// Verify feature index is cleaned
	models = registry.GetModelsUsingFeature("unique_feature")
	assert.Empty(t, models)
}
