package offlinestore

import (
	"testing"

	"github.com/feather-store/feather/internal/core/domain"
)

func TestSchemaMapper_DatasetFromGroup(t *testing.T) {
	mapper := NewSchemaMapper()

	group := &domain.FeatureGroup{
		Name:       "user_features",
		EntityType: "user",
		Features: []domain.FeatureSpec{
			{Name: "age", DataType: domain.DataTypeInt64},
			{Name: "score", DataType: domain.DataTypeFloat64},
		},
	}

	cfg, err := mapper.DatasetFromGroup(group)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Name != "user_features" {
		t.Errorf("expected name user_features, got %s", cfg.Name)
	}
	if cfg.EntityType != "user" {
		t.Errorf("expected entity type user, got %s", cfg.EntityType)
	}
}

func TestSchemaMapper_DatasetFromGroup_NilGroup(t *testing.T) {
	mapper := NewSchemaMapper()
	_, err := mapper.DatasetFromGroup(nil)
	if err == nil {
		t.Fatal("expected error for nil group")
	}
}

func TestSchemaMapper_FeatureColumns(t *testing.T) {
	mapper := NewSchemaMapper()

	group := &domain.FeatureGroup{
		Name: "test",
		Features: []domain.FeatureSpec{
			{Name: "count", DataType: domain.DataTypeInt64},
			{Name: "ratio", DataType: domain.DataTypeFloat64},
			{Name: "label", DataType: domain.DataTypeString},
			{Name: "active", DataType: domain.DataTypeBool},
		},
	}

	cols, err := mapper.FeatureColumns(group)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// entity_id + timestamp + 4 features = 6
	if len(cols) != 6 {
		t.Fatalf("expected 6 columns, got %d", len(cols))
	}
	if cols[0].Name != "entity_id" {
		t.Errorf("first column should be entity_id, got %s", cols[0].Name)
	}
	if cols[2].Type != "int64" {
		t.Errorf("count column should be int64, got %s", cols[2].Type)
	}
	if cols[3].Type != "float64" {
		t.Errorf("ratio column should be float64, got %s", cols[3].Type)
	}
}

func TestSchemaMapper_ValidateRow(t *testing.T) {
	mapper := NewSchemaMapper()
	cols := []ColumnDef{
		{Name: "entity_id", Type: "string"},
		{Name: "timestamp", Type: "int64"},
		{Name: "count", Type: "int64"},
	}
	row := FeatureRow{
		EntityID: "user1",
		Features: map[string]interface{}{"count": 42},
	}
	if err := mapper.ValidateRow(row, cols); err != nil {
		t.Errorf("unexpected validation error: %v", err)
	}
}
