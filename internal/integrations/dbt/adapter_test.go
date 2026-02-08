package dbt

import (
	"strings"
	"testing"
)

func TestParseManifest(t *testing.T) {
	manifestJSON := `{
		"metadata": {
			"dbt_schema_version": "https://schemas.getdbt.com/dbt/manifest/v11.json",
			"dbt_version": "1.7.0",
			"project_name": "test_project"
		},
		"nodes": {
			"model.test.user_features": {
				"unique_id": "model.test.user_features",
				"name": "user_features",
				"resource_type": "model",
				"package_name": "test",
				"description": "User feature table",
				"columns": {
					"user_id": {
						"name": "user_id",
						"description": "User identifier",
						"data_type": "string"
					},
					"score": {
						"name": "score",
						"description": "User score",
						"data_type": "float64"
					}
				},
				"config": {
					"enabled": true
				},
				"tags": ["entity:user"],
				"meta": {},
				"depends_on": {
					"macros": [],
					"nodes": []
				}
			}
		},
		"sources": {},
		"metrics": {}
	}`

	manifest, err := ParseManifestFromBytes([]byte(manifestJSON))
	if err != nil {
		t.Fatalf("failed to parse manifest: %v", err)
	}

	if manifest.Metadata.ProjectName != "test_project" {
		t.Errorf("expected project name 'test_project', got '%s'", manifest.Metadata.ProjectName)
	}

	models := manifest.GetModels()
	if len(models) != 1 {
		t.Errorf("expected 1 model, got %d", len(models))
	}

	if models[0].Name != "user_features" {
		t.Errorf("expected model name 'user_features', got '%s'", models[0].Name)
	}
}

func TestAdapterSyncManifest(t *testing.T) {
	manifestJSON := `{
		"metadata": {
			"dbt_schema_version": "https://schemas.getdbt.com/dbt/manifest/v11.json",
			"dbt_version": "1.7.0",
			"project_name": "test_project"
		},
		"nodes": {
			"model.test.user_features": {
				"unique_id": "model.test.user_features",
				"name": "user_features",
				"resource_type": "model",
				"package_name": "test",
				"description": "User feature table",
				"columns": {
					"user_id": {
						"name": "user_id",
						"description": "User identifier",
						"data_type": "string"
					},
					"score": {
						"name": "score",
						"description": "User score",
						"data_type": "float64"
					},
					"purchases": {
						"name": "purchases",
						"description": "Number of purchases",
						"data_type": "int"
					}
				},
				"config": {
					"enabled": true
				},
				"tags": ["entity:user"],
				"meta": {
					"owner": "data-team"
				},
				"depends_on": {
					"macros": [],
					"nodes": []
				}
			}
		},
		"sources": {},
		"metrics": {}
	}`

	manifest, err := ParseManifestFromBytes([]byte(manifestJSON))
	if err != nil {
		t.Fatalf("failed to parse manifest: %v", err)
	}

	adapter := NewAdapter(&SyncOptions{
		DefaultEntityType: "unknown",
		Owner:             "default-owner",
	})

	result, err := adapter.SyncManifest(manifest)
	if err != nil {
		t.Fatalf("failed to sync manifest: %v", err)
	}

	if !result.Success {
		t.Errorf("expected sync to succeed")
	}

	if len(result.Features) != 3 {
		t.Errorf("expected 3 features, got %d", len(result.Features))
	}

	// Check feature properties
	var scoreFeature *FeatureDefinition
	for i, f := range result.Features {
		if f.Name == "user_features.score" {
			scoreFeature = &result.Features[i]
			break
		}
	}

	if scoreFeature == nil {
		t.Fatal("expected to find feature 'user_features.score'")
	}

	if scoreFeature.EntityType != "user" {
		t.Errorf("expected entity type 'user', got '%s'", scoreFeature.EntityType)
	}

	if scoreFeature.DataType != "float64" {
		t.Errorf("expected data type 'float64', got '%s'", scoreFeature.DataType)
	}

	if scoreFeature.Owner != "data-team" {
		t.Errorf("expected owner 'data-team', got '%s'", scoreFeature.Owner)
	}
}

func TestMapDataType(t *testing.T) {
	adapter := NewAdapter(nil)

	tests := []struct {
		input    string
		expected string
	}{
		{"INTEGER", "int64"},
		{"BIGINT", "int64"},
		{"float", "float64"},
		{"DOUBLE PRECISION", "float64"},
		{"DECIMAL(10,2)", "float64"},
		{"BOOLEAN", "bool"},
		{"TIMESTAMP", "timestamp"},
		{"VARCHAR(255)", "string"},
		{"TEXT", "string"},
		{"ARRAY<FLOAT>", "vector"},
		{"BINARY", "bytes"},
		{"", "string"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := adapter.mapDataType(tt.input)
			if result != tt.expected {
				t.Errorf("mapDataType(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestDetermineEntityType(t *testing.T) {
	adapter := NewAdapter(&SyncOptions{
		DefaultEntityType: "default",
		EntityTypeMapping: map[string]string{
			"user_features": "user",
			"item_features": "item",
		},
	})

	tests := []struct {
		tags     []string
		expected string
	}{
		{[]string{"entity:customer"}, "customer"},
		{[]string{"user_features", "important"}, "user"},
		{[]string{"item_features"}, "item"},
		{[]string{"other"}, "default"},
		{[]string{}, "default"},
	}

	for _, tt := range tests {
		result := adapter.determineEntityType(tt.tags)
		if result != tt.expected {
			t.Errorf("determineEntityType(%v) = %q, want %q", tt.tags, result, tt.expected)
		}
	}
}

func TestManifestValidation(t *testing.T) {
	// Test missing schema version
	invalidManifest := &Manifest{
		Metadata: ManifestMetadata{},
	}

	err := invalidManifest.Validate()
	if err == nil {
		t.Error("expected validation error for missing schema version")
	}

	// Test circular dependency detection
	circularManifest := &Manifest{
		Metadata: ManifestMetadata{
			DBTSchemaVersion: "v1",
		},
		Nodes: map[string]Node{
			"model.a": {
				UniqueID: "model.a",
				DependsOn: DependsOn{
					Nodes: []string{"model.b"},
				},
			},
			"model.b": {
				UniqueID: "model.b",
				DependsOn: DependsOn{
					Nodes: []string{"model.c"},
				},
			},
			"model.c": {
				UniqueID: "model.c",
				DependsOn: DependsOn{
					Nodes: []string{"model.a"},
				},
			},
		},
	}

	err = circularManifest.Validate()
	if err == nil {
		t.Error("expected validation error for circular dependency")
	}
	if !strings.Contains(err.Error(), "circular dependency") {
		t.Errorf("expected circular dependency error, got: %v", err)
	}
}

func TestFilterModels(t *testing.T) {
	manifest := &Manifest{
		Metadata: ManifestMetadata{
			DBTSchemaVersion: "v1",
		},
		Nodes: map[string]Node{
			"model.user_features": {
				UniqueID:     "model.user_features",
				Name:         "user_features",
				ResourceType: "model",
				Tags:         []string{"ml", "user"},
			},
			"model.item_features": {
				UniqueID:     "model.item_features",
				Name:         "item_features",
				ResourceType: "model",
				Tags:         []string{"ml", "item"},
			},
			"model.other": {
				UniqueID:     "model.other",
				Name:         "other",
				ResourceType: "model",
				Tags:         []string{"reporting"},
			},
		},
	}

	// Filter by tag
	filtered := manifest.FilterModels([]string{"ml"}, nil)
	if len(filtered) != 2 {
		t.Errorf("expected 2 models with tag 'ml', got %d", len(filtered))
	}

	// Filter by pattern
	filtered = manifest.FilterModels(nil, []string{"*features"})
	if len(filtered) != 2 {
		t.Errorf("expected 2 models matching '*features', got %d", len(filtered))
	}

	// Filter by tag and pattern
	filtered = manifest.FilterModels([]string{"user"}, []string{"user*"})
	if len(filtered) != 1 {
		t.Errorf("expected 1 model matching tag 'user' and pattern 'user*', got %d", len(filtered))
	}
}
