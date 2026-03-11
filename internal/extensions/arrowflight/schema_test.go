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

func TestBatchBuilder_Rows(t *testing.T) {
	t.Parallel()
	schema := []ColumnSchema{
		{Name: "entity_key", Type: DataTypeString, Nullable: false},
		{Name: "score", Type: DataTypeFloat64, Nullable: true},
	}
	b := NewBatchBuilder(schema)

	if b.Rows() != 0 {
		t.Errorf("expected 0 rows initially, got %d", b.Rows())
	}
	_ = b.AddRow(map[string]interface{}{"entity_key": "u:1", "score": 0.9})
	_ = b.AddRow(map[string]interface{}{"entity_key": "u:2", "score": 0.8})
	if b.Rows() != 2 {
		t.Errorf("expected 2 rows, got %d", b.Rows())
	}
}

func TestEvalPredicate(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		val      interface{}
		pred     Predicate
		expected bool
	}{
		{"nil is_null", nil, Predicate{Operator: "is_null"}, true},
		{"nil other", nil, Predicate{Operator: "eq", Value: "x"}, false},
		{"eq match", "hello", Predicate{Operator: "eq", Value: "hello"}, true},
		{"eq =", "hello", Predicate{Operator: "=", Value: "hello"}, true},
		{"eq ==", "hello", Predicate{Operator: "==", Value: "hello"}, true},
		{"neq match", "hello", Predicate{Operator: "neq", Value: "world"}, true},
		{"neq !=", "hello", Predicate{Operator: "!=", Value: "world"}, true},
		{"gt >", int64(10), Predicate{Operator: ">", Value: int64(5)}, true},
		{"gte >=", int64(5), Predicate{Operator: ">=", Value: int64(5)}, true},
		{"lt <", int64(3), Predicate{Operator: "<", Value: int64(5)}, true},
		{"lte <=", int64(5), Predicate{Operator: "<=", Value: int64(5)}, true},
		{"contains", "hello world", Predicate{Operator: "contains", Value: "world"}, true},
		{"is_not_null val", "val", Predicate{Operator: "is_not_null"}, true},
		{"is_not_null nil", nil, Predicate{Operator: "is_not_null"}, false},
		{"unknown op", "val", Predicate{Operator: "unknown"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := evalPredicate(tt.val, tt.pred); got != tt.expected {
				t.Errorf("evalPredicate(%v, %s) = %v, want %v", tt.val, tt.pred.Operator, got, tt.expected)
			}
		})
	}
}

func TestToArrowFloat(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input    interface{}
		expected float64
	}{
		{float64(3.14), 3.14},
		{float32(2.5), 2.5},
		{int(42), 42.0},
		{int64(100), 100.0},
		{int32(7), 7.0},
		{"not a number", 0},
		{nil, 0},
		{true, 0},
	}
	for _, tt := range tests {
		if got := toArrowFloat(tt.input); got != tt.expected {
			t.Errorf("toArrowFloat(%v) = %f, want %f", tt.input, got, tt.expected)
		}
	}
}

func TestCompareNumeric(t *testing.T) {
	t.Parallel()
	if compareNumeric(int64(5), int64(3)) != 1 {
		t.Error("expected 5 > 3 = 1")
	}
	if compareNumeric(int64(3), int64(5)) != -1 {
		t.Error("expected 3 < 5 = -1")
	}
	if compareNumeric(int64(5), int64(5)) != 0 {
		t.Error("expected 5 == 5 = 0")
	}
	if compareNumeric(float64(1.5), float64(1.5)) != 0 {
		t.Error("expected 1.5 == 1.5 = 0")
	}
}
