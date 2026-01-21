package migration

import (
	"testing"
	"time"

	"github.com/feather-store/feather/internal/domain"
)

func TestNewSchemaConverter(t *testing.T) {
	config := DefaultSchemaConverterConfig()
	converter := NewSchemaConverter(config)

	if converter == nil {
		t.Fatal("Expected converter to be non-nil")
	}
}

func TestDefaultSchemaConverterConfig(t *testing.T) {
	config := DefaultSchemaConverterConfig()

	if config.DefaultTTL != 5*time.Minute {
		t.Errorf("Expected default TTL 5m, got %v", config.DefaultTTL)
	}
	if !config.PreserveMetadata {
		t.Error("Expected PreserveMetadata to be true")
	}
}

func TestSchemaConverter_ConvertProject(t *testing.T) {
	converter := NewSchemaConverter(DefaultSchemaConverterConfig())

	ttl := 10 * time.Minute
	project := &FeastProject{
		Name: "test_project",
		Entities: []FeastEntity{
			{
				Name:      "user_id",
				ValueType: FeastTypeString,
			},
		},
		FeatureViews: []FeastFeatureView{
			{
				Name:     "user_features",
				Entities: []string{"user_id"},
				Features: []FeastFeature{
					{Name: "age", ValueType: FeastTypeInt64},
					{Name: "balance", ValueType: FeastTypeDouble},
				},
				TTL:    &ttl,
				Online: true,
			},
		},
	}

	result, err := converter.ConvertProject(project)
	if err != nil {
		t.Fatalf("ConvertProject failed: %v", err)
	}

	if len(result.FeatureGroups) != 1 {
		t.Errorf("Expected 1 feature group, got %d", len(result.FeatureGroups))
	}

	group := result.FeatureGroups[0]
	if group.Name != "user_features" {
		t.Errorf("Expected name 'user_features', got '%s'", group.Name)
	}
	if len(group.Features) != 2 {
		t.Errorf("Expected 2 features, got %d", len(group.Features))
	}
	if group.TTL != ttl {
		t.Errorf("Expected TTL %v, got %v", ttl, group.TTL)
	}
}

func TestSchemaConverter_ConvertProject_Nil(t *testing.T) {
	converter := NewSchemaConverter(DefaultSchemaConverterConfig())

	_, err := converter.ConvertProject(nil)
	if err != ErrInvalidFeastSchema {
		t.Errorf("Expected ErrInvalidFeastSchema, got %v", err)
	}
}

func TestSchemaConverter_ConvertFeatureView(t *testing.T) {
	converter := NewSchemaConverter(DefaultSchemaConverterConfig())

	ttl := 15 * time.Minute
	fv := &FeastFeatureView{
		Name:        "product_features",
		Description: "Product features",
		Entities:    []string{"product_id"},
		Features: []FeastFeature{
			{Name: "price", ValueType: FeastTypeDouble, Description: "Product price"},
			{Name: "in_stock", ValueType: FeastTypeBool},
		},
		TTL:    &ttl,
		Online: true,
		Tags:   map[string]string{"team": "pricing"},
	}

	entities := []FeastEntity{
		{Name: "product_id", ValueType: FeastTypeString},
	}

	group, warnings, err := converter.ConvertFeatureView(fv, entities)
	if err != nil {
		t.Fatalf("ConvertFeatureView failed: %v", err)
	}

	if group.Name != "product_features" {
		t.Errorf("Expected name 'product_features', got '%s'", group.Name)
	}
	if group.Description != "Product features" {
		t.Errorf("Expected description 'Product features', got '%s'", group.Description)
	}
	if len(group.Features) != 2 {
		t.Errorf("Expected 2 features, got %d", len(group.Features))
	}
	if group.TTL != ttl {
		t.Errorf("Expected TTL %v, got %v", ttl, group.TTL)
	}
	if len(*warnings) != 0 {
		t.Errorf("Expected no warnings, got %d", len(*warnings))
	}
}

func TestSchemaConverter_ConvertFeatureView_Nil(t *testing.T) {
	converter := NewSchemaConverter(DefaultSchemaConverterConfig())

	_, _, err := converter.ConvertFeatureView(nil, nil)
	if err != ErrInvalidFeastSchema {
		t.Errorf("Expected ErrInvalidFeastSchema, got %v", err)
	}
}

func TestSchemaConverter_ConvertOnDemandFeatureView(t *testing.T) {
	converter := NewSchemaConverter(DefaultSchemaConverterConfig())

	odfv := &FeastOnDemandFeatureView{
		Name:        "computed_features",
		Description: "Computed features",
		Features: []FeastFeature{
			{Name: "derived_score", ValueType: FeastTypeDouble},
		},
		UDF: "def transform(inputs): return {'derived_score': inputs['a'] + inputs['b']}",
	}

	group, warnings, err := converter.ConvertOnDemandFeatureView(odfv)
	if err != nil {
		t.Fatalf("ConvertOnDemandFeatureView failed: %v", err)
	}

	if group.Name != "computed_features" {
		t.Errorf("Expected name 'computed_features', got '%s'", group.Name)
	}

	// Should have warning about UDF
	if len(*warnings) == 0 {
		t.Error("Expected warning about UDF migration")
	}

	// Group should be marked as on_demand
	if group.Tags["feast_type"] != "on_demand" {
		t.Error("Expected group to be tagged as on_demand")
	}
}

func TestSchemaConverter_ConvertTypes(t *testing.T) {
	converter := NewSchemaConverter(DefaultSchemaConverterConfig())

	tests := []struct {
		feastType FeastValueType
		expected  domain.DataType
	}{
		{FeastTypeBool, domain.DataTypeBool},
		{FeastTypeInt32, domain.DataTypeInt64},
		{FeastTypeInt64, domain.DataTypeInt64},
		{FeastTypeFloat, domain.DataTypeFloat64},
		{FeastTypeDouble, domain.DataTypeFloat64},
		{FeastTypeString, domain.DataTypeString},
		{FeastTypeBytes, domain.DataTypeString},
		{FeastTypeUnixTimestamp, domain.DataTypeInt64},
	}

	for _, tt := range tests {
		fv := &FeastFeatureView{
			Name: "test",
			Features: []FeastFeature{
				{Name: "test_feature", ValueType: tt.feastType},
			},
		}

		group, _, _ := converter.ConvertFeatureView(fv, nil)
		if len(group.Features) > 0 && group.Features[0].DataType != tt.expected {
			t.Errorf("Type %s: expected %s, got %s", tt.feastType, tt.expected, group.Features[0].DataType)
		}
	}
}

func TestSchemaConverter_NameMapping(t *testing.T) {
	config := DefaultSchemaConverterConfig()
	config.NameMapping = map[string]string{
		"legacy_feature": "new_feature",
	}
	converter := NewSchemaConverter(config)

	fv := &FeastFeatureView{
		Name: "test",
		Features: []FeastFeature{
			{Name: "legacy_feature", ValueType: FeastTypeDouble},
		},
	}

	group, _, _ := converter.ConvertFeatureView(fv, nil)
	if len(group.Features) > 0 && group.Features[0].Name != "new_feature" {
		t.Errorf("Expected renamed feature 'new_feature', got '%s'", group.Features[0].Name)
	}
}

func TestValidateFeastProject(t *testing.T) {
	// Valid project
	validProject := &FeastProject{
		Name: "test_project",
		Entities: []FeastEntity{
			{Name: "user_id", ValueType: FeastTypeString},
		},
		FeatureViews: []FeastFeatureView{
			{
				Name:     "features",
				Entities: []string{"user_id"},
				Features: []FeastFeature{
					{Name: "age", ValueType: FeastTypeInt64},
				},
			},
		},
	}

	errors := ValidateFeastProject(validProject)
	if len(errors) != 0 {
		t.Errorf("Expected no errors, got %d: %v", len(errors), errors)
	}
}

func TestValidateFeastProject_Nil(t *testing.T) {
	errors := ValidateFeastProject(nil)
	if len(errors) != 1 {
		t.Errorf("Expected 1 error for nil project, got %d", len(errors))
	}
}

func TestValidateFeastProject_MissingName(t *testing.T) {
	project := &FeastProject{
		Name: "",
	}

	errors := ValidateFeastProject(project)
	if len(errors) != 1 {
		t.Errorf("Expected 1 error for missing name, got %d", len(errors))
	}
}

func TestValidateFeastProject_DuplicateEntity(t *testing.T) {
	project := &FeastProject{
		Name: "test",
		Entities: []FeastEntity{
			{Name: "user_id", ValueType: FeastTypeString},
			{Name: "user_id", ValueType: FeastTypeInt64},
		},
	}

	errors := ValidateFeastProject(project)
	if len(errors) == 0 {
		t.Error("Expected error for duplicate entity")
	}
}

func TestValidateFeastProject_InvalidEntityReference(t *testing.T) {
	project := &FeastProject{
		Name: "test",
		Entities: []FeastEntity{
			{Name: "user_id", ValueType: FeastTypeString},
		},
		FeatureViews: []FeastFeatureView{
			{
				Name:     "features",
				Entities: []string{"nonexistent_entity"},
			},
		},
	}

	errors := ValidateFeastProject(project)
	if len(errors) == 0 {
		t.Error("Expected error for invalid entity reference")
	}
}

func TestValidateFeastProject_DuplicateFeature(t *testing.T) {
	project := &FeastProject{
		Name: "test",
		FeatureViews: []FeastFeatureView{
			{
				Name: "features",
				Features: []FeastFeature{
					{Name: "age", ValueType: FeastTypeInt64},
					{Name: "age", ValueType: FeastTypeDouble},
				},
			},
		},
	}

	errors := ValidateFeastProject(project)
	if len(errors) == 0 {
		t.Error("Expected error for duplicate feature")
	}
}

func TestConvertProject_WithOnDemandViews(t *testing.T) {
	converter := NewSchemaConverter(DefaultSchemaConverterConfig())

	project := &FeastProject{
		Name: "test_project",
		FeatureViews: []FeastFeatureView{
			{
				Name: "base_features",
				Features: []FeastFeature{
					{Name: "value", ValueType: FeastTypeDouble},
				},
			},
		},
		OnDemandFeatureViews: []FeastOnDemandFeatureView{
			{
				Name: "derived_features",
				Features: []FeastFeature{
					{Name: "derived", ValueType: FeastTypeDouble},
				},
			},
		},
	}

	result, err := converter.ConvertProject(project)
	if err != nil {
		t.Fatalf("ConvertProject failed: %v", err)
	}

	if len(result.FeatureGroups) != 2 {
		t.Errorf("Expected 2 feature groups, got %d", len(result.FeatureGroups))
	}
}

func TestConvertProject_CollectsWarnings(t *testing.T) {
	converter := NewSchemaConverter(DefaultSchemaConverterConfig())

	project := &FeastProject{
		Name: "test_project",
		OnDemandFeatureViews: []FeastOnDemandFeatureView{
			{
				Name: "computed",
				Features: []FeastFeature{
					{Name: "x", ValueType: FeastTypeDouble},
				},
				UDF: "lambda x: x * 2",
			},
		},
	}

	result, _ := converter.ConvertProject(project)
	if len(result.Warnings) == 0 {
		t.Error("Expected warnings about UDF")
	}
}
