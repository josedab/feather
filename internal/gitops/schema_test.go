package gitops

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

func TestDuration_MarshalJSON(t *testing.T) {
	d := Duration{Duration: 5 * time.Minute}
	data, err := json.Marshal(d)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	if string(data) != `"5m0s"` {
		t.Errorf("Expected \"5m0s\", got %s", string(data))
	}
}

func TestDuration_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected time.Duration
		wantErr  bool
	}{
		{
			name:     "string duration",
			input:    `"1h30m"`,
			expected: 90 * time.Minute,
		},
		{
			name:     "numeric nanoseconds",
			input:    `1000000000`,
			expected: time.Second,
		},
		{
			name:    "invalid string",
			input:   `"invalid"`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var d Duration
			err := json.Unmarshal([]byte(tt.input), &d)
			if tt.wantErr {
				if err == nil {
					t.Error("Expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}
			if d.Duration != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, d.Duration)
			}
		})
	}
}

func TestDuration_MarshalYAML(t *testing.T) {
	d := Duration{Duration: 24 * time.Hour}
	data, err := yaml.Marshal(d)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	expected := "24h0m0s\n"
	if string(data) != expected {
		t.Errorf("Expected %q, got %q", expected, string(data))
	}
}

func TestDuration_UnmarshalYAML(t *testing.T) {
	input := "ttl: 30m"
	var result struct {
		TTL Duration `yaml:"ttl"`
	}
	if err := yaml.Unmarshal([]byte(input), &result); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if result.TTL.Duration != 30*time.Minute {
		t.Errorf("Expected 30m, got %v", result.TTL.Duration)
	}
}

func TestSchemaLoader_LoadDefinition_YAML(t *testing.T) {
	tmpDir := t.TempDir()
	loader := NewSchemaLoader(tmpDir)

	yamlContent := `
apiVersion: feather.io/v1
kind: FeatureGroup
metadata:
  name: user_features
  namespace: production
  owner: ml-team
  team: data-science
  labels:
    env: prod
spec:
  entityType: user
  description: User profile features
  features:
    - name: age
      dataType: int64
      description: User age in years
    - name: country
      dataType: string
  ttl: 24h
  tags:
    - user
    - profile
`

	defPath := filepath.Join(tmpDir, "user_features.yaml")
	if err := os.WriteFile(defPath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}

	def, err := loader.LoadDefinition("user_features.yaml")
	if err != nil {
		t.Fatalf("LoadDefinition failed: %v", err)
	}

	if def.APIVersion != "feather.io/v1" {
		t.Errorf("Expected apiVersion 'feather.io/v1', got '%s'", def.APIVersion)
	}
	if def.Kind != "FeatureGroup" {
		t.Errorf("Expected kind 'FeatureGroup', got '%s'", def.Kind)
	}
	if def.Metadata.Name != "user_features" {
		t.Errorf("Expected name 'user_features', got '%s'", def.Metadata.Name)
	}
	if def.Metadata.Namespace != "production" {
		t.Errorf("Expected namespace 'production', got '%s'", def.Metadata.Namespace)
	}
	if def.Spec.EntityType != "user" {
		t.Errorf("Expected entityType 'user', got '%s'", def.Spec.EntityType)
	}
	if len(def.Spec.Features) != 2 {
		t.Errorf("Expected 2 features, got %d", len(def.Spec.Features))
	}
	if def.Spec.TTL == nil || def.Spec.TTL.Duration != 24*time.Hour {
		t.Errorf("Expected TTL 24h, got %v", def.Spec.TTL)
	}
}

func TestSchemaLoader_LoadDefinition_JSON(t *testing.T) {
	tmpDir := t.TempDir()
	loader := NewSchemaLoader(tmpDir)

	jsonContent := `{
		"apiVersion": "feather.io/v1",
		"kind": "FeatureGroup",
		"metadata": {
			"name": "product_features",
			"owner": "product-team"
		},
		"spec": {
			"entityType": "product",
			"features": [
				{"name": "price", "dataType": "float64"},
				{"name": "category", "dataType": "string"}
			]
		}
	}`

	defPath := filepath.Join(tmpDir, "product.json")
	if err := os.WriteFile(defPath, []byte(jsonContent), 0644); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}

	def, err := loader.LoadDefinition("product.json")
	if err != nil {
		t.Fatalf("LoadDefinition failed: %v", err)
	}

	if def.Metadata.Name != "product_features" {
		t.Errorf("Expected name 'product_features', got '%s'", def.Metadata.Name)
	}
	if len(def.Spec.Features) != 2 {
		t.Errorf("Expected 2 features, got %d", len(def.Spec.Features))
	}
}

func TestSchemaLoader_LoadDefinition_UnsupportedFormat(t *testing.T) {
	tmpDir := t.TempDir()
	loader := NewSchemaLoader(tmpDir)

	defPath := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(defPath, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}

	_, err := loader.LoadDefinition("test.txt")
	if err == nil {
		t.Error("Expected error for unsupported format")
	}
}

func TestSchemaLoader_LoadDefinition_ValidationErrors(t *testing.T) {
	tmpDir := t.TempDir()
	loader := NewSchemaLoader(tmpDir)

	tests := []struct {
		name    string
		content string
		errMsg  string
	}{
		{
			name:    "missing apiVersion",
			content: `kind: FeatureGroup\nmetadata:\n  name: test\nspec:\n  entityType: user\n  features:\n    - name: age\n      dataType: int64`,
			errMsg:  "apiVersion is required",
		},
		{
			name:    "missing kind",
			content: `apiVersion: feather.io/v1\nmetadata:\n  name: test\nspec:\n  entityType: user\n  features:\n    - name: age\n      dataType: int64`,
			errMsg:  "kind is required",
		},
		{
			name:    "missing name",
			content: `apiVersion: feather.io/v1\nkind: FeatureGroup\nmetadata: {}\nspec:\n  entityType: user\n  features:\n    - name: age\n      dataType: int64`,
			errMsg:  "metadata.name is required",
		},
		{
			name:    "missing entityType",
			content: `apiVersion: feather.io/v1\nkind: FeatureGroup\nmetadata:\n  name: test\nspec:\n  features:\n    - name: age\n      dataType: int64`,
			errMsg:  "spec.entityType is required",
		},
		{
			name:    "empty features",
			content: `apiVersion: feather.io/v1\nkind: FeatureGroup\nmetadata:\n  name: test\nspec:\n  entityType: user\n  features: []`,
			errMsg:  "spec.features must not be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defPath := filepath.Join(tmpDir, tt.name+".yaml")
			if err := os.WriteFile(defPath, []byte(tt.content), 0644); err != nil {
				t.Fatalf("Failed to write file: %v", err)
			}

			_, err := loader.LoadDefinition(tt.name + ".yaml")
			if err == nil {
				t.Errorf("Expected validation error for %s", tt.name)
			}
		})
	}
}

func TestSchemaLoader_LoadAllDefinitions(t *testing.T) {
	tmpDir := t.TempDir()
	loader := NewSchemaLoader(tmpDir)

	// Create multiple definition files
	files := []struct {
		name    string
		content string
	}{
		{
			name: "user.yaml",
			content: `
apiVersion: feather.io/v1
kind: FeatureGroup
metadata:
  name: user_features
spec:
  entityType: user
  features:
    - name: age
      dataType: int64`,
		},
		{
			name: "product.yaml",
			content: `
apiVersion: feather.io/v1
kind: FeatureGroup
metadata:
  name: product_features
spec:
  entityType: product
  features:
    - name: price
      dataType: float64`,
		},
	}

	for _, f := range files {
		if err := os.WriteFile(filepath.Join(tmpDir, f.name), []byte(f.content), 0644); err != nil {
			t.Fatalf("Failed to write file: %v", err)
		}
	}

	defs, err := loader.LoadAllDefinitions("*.yaml")
	if err != nil {
		t.Fatalf("LoadAllDefinitions failed: %v", err)
	}

	if len(defs) != 2 {
		t.Errorf("Expected 2 definitions, got %d", len(defs))
	}
}

func TestSchemaLoader_SaveDefinition(t *testing.T) {
	tmpDir := t.TempDir()
	loader := NewSchemaLoader(tmpDir)

	def := &FeatureDefinition{
		APIVersion: "feather.io/v1",
		Kind:       "FeatureGroup",
		Metadata: DefinitionMeta{
			Name:      "test_features",
			Namespace: "test",
		},
		Spec: FeatureSpec{
			EntityType: "test",
			Features: []FeatureField{
				{Name: "value", DataType: "float64"},
			},
		},
	}

	// Save as YAML
	if err := loader.SaveDefinition(def, "test.yaml"); err != nil {
		t.Fatalf("SaveDefinition YAML failed: %v", err)
	}

	// Verify file exists and can be loaded
	loaded, err := loader.LoadDefinition("test.yaml")
	if err != nil {
		t.Fatalf("Failed to load saved definition: %v", err)
	}
	if loaded.Metadata.Name != "test_features" {
		t.Errorf("Expected name 'test_features', got '%s'", loaded.Metadata.Name)
	}

	// Save as JSON
	if err := loader.SaveDefinition(def, "subdir/test.json"); err != nil {
		t.Fatalf("SaveDefinition JSON failed: %v", err)
	}

	// Verify JSON file
	loaded, err = loader.LoadDefinition("subdir/test.json")
	if err != nil {
		t.Fatalf("Failed to load saved JSON definition: %v", err)
	}
	if loaded.Metadata.Name != "test_features" {
		t.Errorf("Expected name 'test_features', got '%s'", loaded.Metadata.Name)
	}
}

func TestIsValidDataType(t *testing.T) {
	validTypes := []string{
		"string", "int64", "float64", "bool", "bytes",
		"timestamp", "string_list", "int64_list", "float64_list", "map",
	}

	for _, dt := range validTypes {
		if !isValidDataType(dt) {
			t.Errorf("Expected %s to be valid", dt)
		}
	}

	invalidTypes := []string{"integer", "number", "array", "object", "invalid"}
	for _, dt := range invalidTypes {
		if isValidDataType(dt) {
			t.Errorf("Expected %s to be invalid", dt)
		}
	}
}

func TestFeatureDefinition_WithConstraints(t *testing.T) {
	tmpDir := t.TempDir()
	loader := NewSchemaLoader(tmpDir)

	yamlContent := `
apiVersion: feather.io/v1
kind: FeatureGroup
metadata:
  name: constrained_features
spec:
  entityType: user
  features:
    - name: age
      dataType: int64
      constraints:
        required: true
        minValue: 0
        maxValue: 150
    - name: email
      dataType: string
      constraints:
        required: true
        pattern: "^[a-zA-Z0-9_.+-]+@[a-zA-Z0-9-]+\\.[a-zA-Z0-9-.]+$"
    - name: status
      dataType: string
      constraints:
        enum:
          - active
          - inactive
          - pending
`

	defPath := filepath.Join(tmpDir, "constrained.yaml")
	if err := os.WriteFile(defPath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}

	def, err := loader.LoadDefinition("constrained.yaml")
	if err != nil {
		t.Fatalf("LoadDefinition failed: %v", err)
	}

	// Check age constraints
	ageFeature := def.Spec.Features[0]
	if ageFeature.Constraints == nil {
		t.Fatal("Expected constraints on age feature")
	}
	if !ageFeature.Constraints.Required {
		t.Error("Expected age to be required")
	}
	if ageFeature.Constraints.MinValue == nil || *ageFeature.Constraints.MinValue != 0 {
		t.Error("Expected minValue 0 for age")
	}
	if ageFeature.Constraints.MaxValue == nil || *ageFeature.Constraints.MaxValue != 150 {
		t.Error("Expected maxValue 150 for age")
	}

	// Check email constraints
	emailFeature := def.Spec.Features[1]
	if emailFeature.Constraints.Pattern == "" {
		t.Error("Expected pattern on email feature")
	}

	// Check status constraints
	statusFeature := def.Spec.Features[2]
	if len(statusFeature.Constraints.Enum) != 3 {
		t.Errorf("Expected 3 enum values, got %d", len(statusFeature.Constraints.Enum))
	}
}

func TestFeatureDefinition_WithDeprecation(t *testing.T) {
	tmpDir := t.TempDir()
	loader := NewSchemaLoader(tmpDir)

	yamlContent := `
apiVersion: feather.io/v1
kind: FeatureGroup
metadata:
  name: deprecated_features
spec:
  entityType: user
  features:
    - name: old_field
      dataType: string
  deprecation:
    deprecated: true
    message: This feature group is deprecated
    replacement: new_user_features
    sunsetDate: "2025-12-31T00:00:00Z"
`

	defPath := filepath.Join(tmpDir, "deprecated.yaml")
	if err := os.WriteFile(defPath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}

	def, err := loader.LoadDefinition("deprecated.yaml")
	if err != nil {
		t.Fatalf("LoadDefinition failed: %v", err)
	}

	if def.Spec.Deprecation == nil {
		t.Fatal("Expected deprecation spec")
	}
	if !def.Spec.Deprecation.Deprecated {
		t.Error("Expected deprecated to be true")
	}
	if def.Spec.Deprecation.Replacement != "new_user_features" {
		t.Errorf("Expected replacement 'new_user_features', got '%s'", def.Spec.Deprecation.Replacement)
	}
}

func TestFeatureDefinition_WithSources(t *testing.T) {
	tmpDir := t.TempDir()
	loader := NewSchemaLoader(tmpDir)

	yamlContent := `
apiVersion: feather.io/v1
kind: FeatureGroup
metadata:
  name: sourced_features
spec:
  entityType: user
  features:
    - name: transaction_count
      dataType: int64
  sources:
    - type: kafka
      topic: user-events
      config:
        groupId: feature-ingestion
    - type: batch
      query: "SELECT user_id, COUNT(*) FROM transactions GROUP BY user_id"
      schedule: "0 0 * * *"
`

	defPath := filepath.Join(tmpDir, "sourced.yaml")
	if err := os.WriteFile(defPath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}

	def, err := loader.LoadDefinition("sourced.yaml")
	if err != nil {
		t.Fatalf("LoadDefinition failed: %v", err)
	}

	if len(def.Spec.Sources) != 2 {
		t.Fatalf("Expected 2 sources, got %d", len(def.Spec.Sources))
	}

	kafkaSource := def.Spec.Sources[0]
	if kafkaSource.Type != "kafka" {
		t.Errorf("Expected type 'kafka', got '%s'", kafkaSource.Type)
	}
	if kafkaSource.Topic != "user-events" {
		t.Errorf("Expected topic 'user-events', got '%s'", kafkaSource.Topic)
	}

	batchSource := def.Spec.Sources[1]
	if batchSource.Type != "batch" {
		t.Errorf("Expected type 'batch', got '%s'", batchSource.Type)
	}
	if batchSource.Schedule != "0 0 * * *" {
		t.Errorf("Expected schedule '0 0 * * *', got '%s'", batchSource.Schedule)
	}
}

func TestFeatureDefinition_WithFreshness(t *testing.T) {
	tmpDir := t.TempDir()
	loader := NewSchemaLoader(tmpDir)

	yamlContent := `
apiVersion: feather.io/v1
kind: FeatureGroup
metadata:
  name: fresh_features
spec:
  entityType: user
  features:
    - name: score
      dataType: float64
  freshness:
    maxAge: 1h
    onStale: default
    default: 0.5
`

	defPath := filepath.Join(tmpDir, "fresh.yaml")
	if err := os.WriteFile(defPath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}

	def, err := loader.LoadDefinition("fresh.yaml")
	if err != nil {
		t.Fatalf("LoadDefinition failed: %v", err)
	}

	if def.Spec.Freshness == nil {
		t.Fatal("Expected freshness spec")
	}
	if def.Spec.Freshness.MaxAge.Duration != time.Hour {
		t.Errorf("Expected maxAge 1h, got %v", def.Spec.Freshness.MaxAge.Duration)
	}
	if def.Spec.Freshness.OnStale != "default" {
		t.Errorf("Expected onStale 'default', got '%s'", def.Spec.Freshness.OnStale)
	}
}

func TestFeatureField_WithSensitivity(t *testing.T) {
	tmpDir := t.TempDir()
	loader := NewSchemaLoader(tmpDir)

	yamlContent := `
apiVersion: feather.io/v1
kind: FeatureGroup
metadata:
  name: sensitive_features
spec:
  entityType: user
  features:
    - name: public_name
      dataType: string
      sensitivity: public
    - name: email
      dataType: string
      sensitivity: internal
    - name: ssn
      dataType: string
      sensitivity: restricted
`

	defPath := filepath.Join(tmpDir, "sensitive.yaml")
	if err := os.WriteFile(defPath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}

	def, err := loader.LoadDefinition("sensitive.yaml")
	if err != nil {
		t.Fatalf("LoadDefinition failed: %v", err)
	}

	sensitivities := []string{"public", "internal", "restricted"}
	for i, f := range def.Spec.Features {
		if f.Sensitivity != sensitivities[i] {
			t.Errorf("Feature %d: expected sensitivity '%s', got '%s'", i, sensitivities[i], f.Sensitivity)
		}
	}
}
