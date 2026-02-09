package arrowflight

import (
	"testing"
)

func TestArrowSchema(t *testing.T) {
	features := map[string]string{
		"age":    "int64",
		"score":  "float64",
		"name":   "string",
		"active": "bool",
	}

	schema := ArrowSchema(features)
	// +1 for entity_key
	if len(schema) != 5 {
		t.Fatalf("expected 5 columns, got %d", len(schema))
	}

	// entity_key should be first and non-nullable
	if schema[0].Name != "entity_key" || schema[0].Nullable {
		t.Fatalf("expected non-nullable entity_key first, got %+v", schema[0])
	}
}

func TestArrowSchema_UnknownType(t *testing.T) {
	schema := ArrowSchema(map[string]string{"x": "unknown_type"})
	// Should fallback to string
	if schema[1].Type != DataTypeString {
		t.Fatalf("expected string fallback, got %s", schema[1].Type)
	}
}

func TestBatchBuilder(t *testing.T) {
	schema := []ColumnSchema{
		{Name: "entity_key", Type: DataTypeString, Nullable: false},
		{Name: "age", Type: DataTypeInt64, Nullable: true},
		{Name: "score", Type: DataTypeFloat64, Nullable: true},
	}

	builder := NewBatchBuilder(schema)
	err := builder.AddRow(map[string]interface{}{
		"entity_key": "user1",
		"age":        25,
		"score":      0.95,
	})
	if err != nil {
		t.Fatalf("AddRow: %v", err)
	}

	err = builder.AddRow(map[string]interface{}{
		"entity_key": "user2",
		"age":        30,
		"score":      0.88,
	})
	if err != nil {
		t.Fatalf("AddRow: %v", err)
	}

	batch := builder.Build()
	if batch.Rows != 2 {
		t.Fatalf("expected 2 rows, got %d", batch.Rows)
	}
	if len(batch.Columns["entity_key"]) != 2 {
		t.Fatalf("expected 2 entity_keys, got %d", len(batch.Columns["entity_key"]))
	}
}

func TestBatchBuilder_MissingRequired(t *testing.T) {
	schema := []ColumnSchema{
		{Name: "entity_key", Type: DataTypeString, Nullable: false},
	}

	builder := NewBatchBuilder(schema)
	err := builder.AddRow(map[string]interface{}{})
	if err == nil {
		t.Fatal("expected error for missing required column")
	}
}

func TestBatchBuilder_NullableColumn(t *testing.T) {
	schema := []ColumnSchema{
		{Name: "key", Type: DataTypeString, Nullable: false},
		{Name: "opt", Type: DataTypeFloat64, Nullable: true},
	}

	builder := NewBatchBuilder(schema)
	err := builder.AddRow(map[string]interface{}{"key": "a"})
	if err != nil {
		t.Fatalf("expected nullable column to accept nil, got %v", err)
	}

	batch := builder.Build()
	if batch.Columns["opt"][0] != nil {
		t.Fatal("expected nil for missing nullable column")
	}
}

func TestPredicateFilter(t *testing.T) {
	batch := &RecordBatch{
		Schema: []ColumnSchema{
			{Name: "name", Type: DataTypeString},
			{Name: "age", Type: DataTypeInt64},
		},
		Rows: 3,
		Columns: map[string][]interface{}{
			"name": {"alice", "bob", "charlie"},
			"age":  {25, 30, 35},
		},
	}

	// Filter age > 28
	filtered := PredicateFilter(batch, []Predicate{
		{Column: "age", Operator: ">", Value: 28},
	})

	if filtered.Rows != 2 {
		t.Fatalf("expected 2 rows after filter, got %d", filtered.Rows)
	}
}

func TestPredicateFilter_Eq(t *testing.T) {
	batch := &RecordBatch{
		Schema: []ColumnSchema{{Name: "name", Type: DataTypeString}},
		Rows:   3,
		Columns: map[string][]interface{}{
			"name": {"alice", "bob", "charlie"},
		},
	}

	filtered := PredicateFilter(batch, []Predicate{
		{Column: "name", Operator: "eq", Value: "bob"},
	})

	if filtered.Rows != 1 {
		t.Fatalf("expected 1 row, got %d", filtered.Rows)
	}
}

func TestPredicateFilter_Empty(t *testing.T) {
	batch := &RecordBatch{Rows: 5}
	result := PredicateFilter(batch, nil)
	if result != batch {
		t.Fatal("expected same batch for empty predicates")
	}
}

func TestColumnProjection(t *testing.T) {
	batch := &RecordBatch{
		Schema: []ColumnSchema{
			{Name: "a", Type: DataTypeString},
			{Name: "b", Type: DataTypeInt64},
			{Name: "c", Type: DataTypeFloat64},
		},
		Rows: 2,
		Columns: map[string][]interface{}{
			"a": {"x", "y"},
			"b": {1, 2},
			"c": {1.0, 2.0},
		},
	}

	projected := ColumnProjection(batch, []string{"a", "c"})
	if len(projected.Schema) != 2 {
		t.Fatalf("expected 2 columns, got %d", len(projected.Schema))
	}
	if _, ok := projected.Columns["b"]; ok {
		t.Fatal("column 'b' should be projected out")
	}
}

func TestColumnProjection_Nil(t *testing.T) {
	if ColumnProjection(nil, []string{"a"}) != nil {
		t.Fatal("expected nil for nil batch")
	}
}
