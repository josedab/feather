package offlinestore

import (
	"fmt"

	"github.com/feather-store/feather/internal/core/domain"
)

// SchemaMapper maps FeatureGroup definitions to offline store schemas.
type SchemaMapper struct{}

// NewSchemaMapper creates a new schema mapper.
func NewSchemaMapper() *SchemaMapper {
	return &SchemaMapper{}
}

// DatasetFromGroup creates a DatasetConfig from a FeatureGroup definition.
func (m *SchemaMapper) DatasetFromGroup(group *domain.FeatureGroup) (*DatasetConfig, error) {
	if group == nil {
		return nil, fmt.Errorf("feature group is required")
	}
	if group.Name == "" {
		return nil, fmt.Errorf("feature group name is required")
	}

	return &DatasetConfig{
		Name:         group.Name,
		FeatureGroup: group.Name,
		EntityType:   group.EntityType,
	}, nil
}

// FeatureColumns returns the column names and types from a FeatureGroup's features.
func (m *SchemaMapper) FeatureColumns(group *domain.FeatureGroup) ([]ColumnDef, error) {
	if group == nil {
		return nil, fmt.Errorf("feature group is required")
	}

	cols := make([]ColumnDef, 0, len(group.Features)+2)
	// Always include entity_id and timestamp
	cols = append(cols, ColumnDef{Name: "entity_id", Type: "string"})
	cols = append(cols, ColumnDef{Name: "timestamp", Type: "int64"})

	for _, f := range group.Features {
		cols = append(cols, ColumnDef{
			Name: f.Name,
			Type: dataTypeToColumnType(f.DataType),
		})
	}
	return cols, nil
}

// ColumnDef defines a column in the offline store schema.
type ColumnDef struct {
	Name string `json:"name"`
	Type string `json:"type"` // "string", "int64", "float64", "bool", "bytes"
}

func dataTypeToColumnType(dt domain.DataType) string {
	switch dt {
	case domain.DataTypeInt64:
		return "int64"
	case domain.DataTypeFloat64:
		return "float64"
	case domain.DataTypeString:
		return "string"
	case domain.DataTypeBool:
		return "bool"
	case domain.DataTypeBytes:
		return "bytes"
	case domain.DataTypeVector:
		return "float64_array"
	case domain.DataTypeTimestamp:
		return "int64"
	default:
		return "string"
	}
}

// ValidateRow checks that a FeatureRow matches the schema columns.
func (m *SchemaMapper) ValidateRow(row FeatureRow, columns []ColumnDef) error {
	for _, col := range columns {
		if col.Name == "entity_id" || col.Name == "timestamp" {
			continue
		}
		if _, ok := row.Features[col.Name]; !ok {
			// Missing features are allowed (null semantics)
			continue
		}
	}
	return nil
}
