package embedding

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultVersionConfig(t *testing.T) {
	config := DefaultVersionConfig()

	assert.False(t, config.AutoMigrate)
	assert.True(t, config.StrictCompatibility)
	assert.True(t, config.WarnOnDeprecated)
}

func TestNewVersionManager(t *testing.T) {
	config := DefaultVersionConfig()
	manager := NewVersionManager(config)

	require.NotNil(t, manager)
	assert.NotNil(t, manager.models)
	assert.NotNil(t, manager.versions)
}

func TestVersionManager_RegisterModel(t *testing.T) {
	config := DefaultVersionConfig()
	manager := NewVersionManager(config)

	model := &ModelInfo{
		ID:        "model-1",
		Name:      "Test Model",
		Provider:  "test",
		Dimension: 1536,
		MaxTokens: 8191,
	}

	err := manager.RegisterModel(model)
	require.NoError(t, err)

	// Retrieve model
	retrieved, err := manager.GetModel("model-1")
	require.NoError(t, err)
	assert.Equal(t, "Test Model", retrieved.Name)
}

func TestVersionManager_RegisterModel_EmptyID(t *testing.T) {
	config := DefaultVersionConfig()
	manager := NewVersionManager(config)

	model := &ModelInfo{
		Name: "Test Model",
	}

	err := manager.RegisterModel(model)
	assert.Error(t, err)
}

func TestVersionManager_GetModel_NotFound(t *testing.T) {
	config := DefaultVersionConfig()
	manager := NewVersionManager(config)

	_, err := manager.GetModel("nonexistent")
	assert.ErrorIs(t, err, ErrModelNotRegistered)
}

func TestVersionManager_RegisterVersion(t *testing.T) {
	config := DefaultVersionConfig()
	manager := NewVersionManager(config)

	// Register model first
	model := &ModelInfo{
		ID:        "model-1",
		Name:      "Test Model",
		Dimension: 1536,
	}
	_ = manager.RegisterModel(model)

	// Register version
	version := &ModelVersion{
		Version:    "v1.0",
		ModelID:    "model-1",
		Dimension:  1536,
		Compatible: []string{},
		IsDefault:  true,
	}

	err := manager.RegisterVersion(version)
	require.NoError(t, err)

	// Retrieve version
	retrieved, err := manager.GetVersion("model-1", "v1.0")
	require.NoError(t, err)
	assert.Equal(t, "v1.0", retrieved.Version)
	assert.True(t, retrieved.IsDefault)
}

func TestVersionManager_RegisterVersion_ModelNotRegistered(t *testing.T) {
	config := DefaultVersionConfig()
	manager := NewVersionManager(config)

	version := &ModelVersion{
		Version: "v1.0",
		ModelID: "nonexistent",
	}

	err := manager.RegisterVersion(version)
	assert.ErrorIs(t, err, ErrModelNotRegistered)
}

func TestVersionManager_RegisterVersion_AlreadyExists(t *testing.T) {
	config := DefaultVersionConfig()
	manager := NewVersionManager(config)

	model := &ModelInfo{ID: "model-1", Dimension: 1536}
	_ = manager.RegisterModel(model)

	version := &ModelVersion{Version: "v1.0", ModelID: "model-1"}
	_ = manager.RegisterVersion(version)

	err := manager.RegisterVersion(version)
	assert.ErrorIs(t, err, ErrVersionAlreadyExists)
}

func TestVersionManager_GetVersion_NotFound(t *testing.T) {
	config := DefaultVersionConfig()
	manager := NewVersionManager(config)

	model := &ModelInfo{ID: "model-1", Dimension: 1536}
	_ = manager.RegisterModel(model)

	_, err := manager.GetVersion("model-1", "nonexistent")
	assert.ErrorIs(t, err, ErrVersionNotFound)
}

func TestVersionManager_GetDefaultVersion(t *testing.T) {
	config := DefaultVersionConfig()
	manager := NewVersionManager(config)

	model := &ModelInfo{ID: "model-1", Dimension: 1536}
	_ = manager.RegisterModel(model)

	v1 := &ModelVersion{Version: "v1.0", ModelID: "model-1", IsDefault: false}
	v2 := &ModelVersion{Version: "v2.0", ModelID: "model-1", IsDefault: true}
	_ = manager.RegisterVersion(v1)
	_ = manager.RegisterVersion(v2)

	defaultVersion, err := manager.GetDefaultVersion("model-1")
	require.NoError(t, err)
	assert.Equal(t, "v2.0", defaultVersion.Version)
}

func TestVersionManager_GetDefaultVersion_NoDefault(t *testing.T) {
	config := DefaultVersionConfig()
	manager := NewVersionManager(config)

	model := &ModelInfo{ID: "model-1", Dimension: 1536}
	_ = manager.RegisterModel(model)

	v1 := &ModelVersion{Version: "v1.0", ModelID: "model-1", ReleasedAt: time.Now().Add(-time.Hour)}
	v2 := &ModelVersion{Version: "v2.0", ModelID: "model-1", ReleasedAt: time.Now()}
	_ = manager.RegisterVersion(v1)
	_ = manager.RegisterVersion(v2)

	// Should return latest
	defaultVersion, err := manager.GetDefaultVersion("model-1")
	require.NoError(t, err)
	assert.Equal(t, "v2.0", defaultVersion.Version)
}

func TestVersionManager_ListVersions(t *testing.T) {
	config := DefaultVersionConfig()
	manager := NewVersionManager(config)

	model := &ModelInfo{ID: "model-1", Dimension: 1536}
	_ = manager.RegisterModel(model)

	_ = manager.RegisterVersion(&ModelVersion{Version: "v1.0", ModelID: "model-1"})
	_ = manager.RegisterVersion(&ModelVersion{Version: "v2.0", ModelID: "model-1"})
	_ = manager.RegisterVersion(&ModelVersion{Version: "v3.0", ModelID: "model-1"})

	versions, err := manager.ListVersions("model-1")
	require.NoError(t, err)
	assert.Len(t, versions, 3)
}

func TestVersionManager_ListModels(t *testing.T) {
	config := DefaultVersionConfig()
	manager := NewVersionManager(config)

	_ = manager.RegisterModel(&ModelInfo{ID: "model-1", Dimension: 1536})
	_ = manager.RegisterModel(&ModelInfo{ID: "model-2", Dimension: 768})
	_ = manager.RegisterModel(&ModelInfo{ID: "model-3", Dimension: 3072})

	models := manager.ListModels()
	assert.Len(t, models, 3)
}

func TestVersionManager_CheckCompatibility_SameVersion(t *testing.T) {
	config := DefaultVersionConfig()
	manager := NewVersionManager(config)

	model := &ModelInfo{ID: "model-1", Dimension: 1536}
	_ = manager.RegisterModel(model)

	_ = manager.RegisterVersion(&ModelVersion{Version: "v1.0", ModelID: "model-1", Dimension: 1536})

	compatible, err := manager.CheckCompatibility("model-1", "v1.0", "v1.0")
	require.NoError(t, err)
	assert.True(t, compatible)
}

func TestVersionManager_CheckCompatibility_SameDimension(t *testing.T) {
	config := DefaultVersionConfig()
	config.StrictCompatibility = false // Non-strict mode
	manager := NewVersionManager(config)

	model := &ModelInfo{ID: "model-1", Dimension: 1536}
	_ = manager.RegisterModel(model)

	_ = manager.RegisterVersion(&ModelVersion{Version: "v1.0", ModelID: "model-1", Dimension: 1536})
	_ = manager.RegisterVersion(&ModelVersion{Version: "v2.0", ModelID: "model-1", Dimension: 1536})

	compatible, err := manager.CheckCompatibility("model-1", "v1.0", "v2.0")
	require.NoError(t, err)
	assert.True(t, compatible) // Same dimension, non-strict mode
}

func TestVersionManager_CheckCompatibility_DifferentDimension(t *testing.T) {
	config := DefaultVersionConfig()
	manager := NewVersionManager(config)

	model := &ModelInfo{ID: "model-1", Dimension: 1536}
	_ = manager.RegisterModel(model)

	_ = manager.RegisterVersion(&ModelVersion{Version: "v1.0", ModelID: "model-1", Dimension: 1536})
	_ = manager.RegisterVersion(&ModelVersion{Version: "v2.0", ModelID: "model-1", Dimension: 3072})

	compatible, err := manager.CheckCompatibility("model-1", "v1.0", "v2.0")
	require.NoError(t, err)
	assert.False(t, compatible) // Different dimensions
}

func TestVersionManager_CheckCompatibility_ExplicitlyCompatible(t *testing.T) {
	config := DefaultVersionConfig()
	config.StrictCompatibility = true
	manager := NewVersionManager(config)

	model := &ModelInfo{ID: "model-1", Dimension: 1536}
	_ = manager.RegisterModel(model)

	_ = manager.RegisterVersion(&ModelVersion{
		Version:    "v1.0",
		ModelID:    "model-1",
		Dimension:  1536,
		Compatible: []string{"v2.0"},
	})
	_ = manager.RegisterVersion(&ModelVersion{
		Version:   "v2.0",
		ModelID:   "model-1",
		Dimension: 1536,
	})

	compatible, err := manager.CheckCompatibility("model-1", "v1.0", "v2.0")
	require.NoError(t, err)
	assert.True(t, compatible) // Explicitly compatible
}

func TestVersionManager_ValidateEmbedding(t *testing.T) {
	config := DefaultVersionConfig()
	manager := NewVersionManager(config)

	model := &ModelInfo{ID: "model-1", Dimension: 1536}
	_ = manager.RegisterModel(model)
	_ = manager.RegisterVersion(&ModelVersion{Version: "v1.0", ModelID: "model-1", Dimension: 1536})

	emb := &Embedding{
		ModelID:      "model-1",
		ModelVersion: "v1.0",
		Dimension:    1536,
	}

	ctx := context.Background()
	err := manager.ValidateEmbedding(ctx, emb)
	require.NoError(t, err)
}

func TestVersionManager_ValidateEmbedding_ModelNotRegistered(t *testing.T) {
	config := DefaultVersionConfig()
	manager := NewVersionManager(config)

	emb := &Embedding{
		ModelID:      "nonexistent",
		ModelVersion: "v1.0",
		Dimension:    1536,
	}

	ctx := context.Background()
	err := manager.ValidateEmbedding(ctx, emb)
	assert.ErrorIs(t, err, ErrModelNotRegistered)
}

func TestVersionManager_ValidateEmbedding_VersionNotFound(t *testing.T) {
	config := DefaultVersionConfig()
	manager := NewVersionManager(config)

	model := &ModelInfo{ID: "model-1", Dimension: 1536}
	_ = manager.RegisterModel(model)

	emb := &Embedding{
		ModelID:      "model-1",
		ModelVersion: "nonexistent",
		Dimension:    1536,
	}

	ctx := context.Background()
	err := manager.ValidateEmbedding(ctx, emb)
	assert.ErrorIs(t, err, ErrVersionNotFound)
}

func TestVersionManager_ValidateEmbedding_DimensionMismatch(t *testing.T) {
	config := DefaultVersionConfig()
	manager := NewVersionManager(config)

	model := &ModelInfo{ID: "model-1", Dimension: 1536}
	_ = manager.RegisterModel(model)
	_ = manager.RegisterVersion(&ModelVersion{Version: "v1.0", ModelID: "model-1", Dimension: 1536})

	emb := &Embedding{
		ModelID:      "model-1",
		ModelVersion: "v1.0",
		Dimension:    768, // Wrong dimension
	}

	ctx := context.Background()
	err := manager.ValidateEmbedding(ctx, emb)
	assert.ErrorIs(t, err, ErrDimensionMismatch)
}

func TestVersionManager_IsDeprecated(t *testing.T) {
	config := DefaultVersionConfig()
	manager := NewVersionManager(config)

	model := &ModelInfo{ID: "model-1", Dimension: 1536}
	_ = manager.RegisterModel(model)

	deprecated := time.Now().Add(-time.Hour)
	_ = manager.RegisterVersion(&ModelVersion{
		Version:      "v1.0",
		ModelID:      "model-1",
		DeprecatedAt: &deprecated,
	})
	_ = manager.RegisterVersion(&ModelVersion{
		Version: "v2.0",
		ModelID: "model-1",
	})

	isDeprecated, err := manager.IsDeprecated("model-1", "v1.0")
	require.NoError(t, err)
	assert.True(t, isDeprecated)

	isDeprecated, err = manager.IsDeprecated("model-1", "v2.0")
	require.NoError(t, err)
	assert.False(t, isDeprecated)
}

func TestVersionManager_DeprecateVersion(t *testing.T) {
	config := DefaultVersionConfig()
	manager := NewVersionManager(config)

	model := &ModelInfo{ID: "model-1", Dimension: 1536}
	_ = manager.RegisterModel(model)
	_ = manager.RegisterVersion(&ModelVersion{Version: "v1.0", ModelID: "model-1"})

	err := manager.DeprecateVersion("model-1", "v1.0")
	require.NoError(t, err)

	isDeprecated, _ := manager.IsDeprecated("model-1", "v1.0")
	assert.True(t, isDeprecated)
}

func TestVersionManager_SetDefaultVersion(t *testing.T) {
	config := DefaultVersionConfig()
	manager := NewVersionManager(config)

	model := &ModelInfo{ID: "model-1", Dimension: 1536}
	_ = manager.RegisterModel(model)
	_ = manager.RegisterVersion(&ModelVersion{Version: "v1.0", ModelID: "model-1", IsDefault: true})
	_ = manager.RegisterVersion(&ModelVersion{Version: "v2.0", ModelID: "model-1", IsDefault: false})

	err := manager.SetDefaultVersion("model-1", "v2.0")
	require.NoError(t, err)

	v1, _ := manager.GetVersion("model-1", "v1.0")
	v2, _ := manager.GetVersion("model-1", "v2.0")

	assert.False(t, v1.IsDefault)
	assert.True(t, v2.IsDefault)
}

func TestVersionManager_Stats(t *testing.T) {
	config := DefaultVersionConfig()
	manager := NewVersionManager(config)

	model := &ModelInfo{ID: "model-1", Dimension: 1536}
	_ = manager.RegisterModel(model)
	_ = manager.RegisterVersion(&ModelVersion{Version: "v1.0", ModelID: "model-1", Dimension: 1536})
	_ = manager.RegisterVersion(&ModelVersion{Version: "v2.0", ModelID: "model-1", Dimension: 1536})

	_, _ = manager.CheckCompatibility("model-1", "v1.0", "v2.0")

	stats := manager.Stats()
	assert.Equal(t, 1, stats["model_count"].(int))
	assert.Equal(t, 2, stats["total_versions"].(int))
	assert.GreaterOrEqual(t, stats["version_checks"].(int64), int64(1))
}

func TestModelInfo_Fields(t *testing.T) {
	model := &ModelInfo{
		ID:        "text-embedding-ada-002",
		Dimension: 1536,
		MaxTokens: 8191,
	}

	assert.Equal(t, "text-embedding-ada-002", model.ID)
	assert.Equal(t, 1536, model.Dimension)
	assert.Equal(t, 8191, model.MaxTokens)
}

func TestModelVersion_Fields(t *testing.T) {
	version := &ModelVersion{
		Version:    "v1.0",
		Compatible: []string{"v1.1", "v1.2"},
		IsDefault:  true,
	}

	assert.Equal(t, "v1.0", version.Version)
	assert.Len(t, version.Compatible, 2)
	assert.True(t, version.IsDefault)
}

func TestPredefinedModels(t *testing.T) {
	assert.Equal(t, "text-embedding-ada-002", ModelOpenAIAda002.ID)
	assert.Equal(t, 1536, ModelOpenAIAda002.Dimension)

	assert.Equal(t, "text-embedding-3-small", ModelOpenAI3Small.ID)
	assert.Equal(t, 1536, ModelOpenAI3Small.Dimension)

	assert.Equal(t, "text-embedding-3-large", ModelOpenAI3Large.ID)
	assert.Equal(t, 3072, ModelOpenAI3Large.Dimension)

	assert.Equal(t, "embed-english-v3.0", ModelCohereEnglish.ID)
	assert.Equal(t, 1024, ModelCohereEnglish.Dimension)
}
