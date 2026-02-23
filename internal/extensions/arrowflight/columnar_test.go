package arrowflight

import (
	"fmt"
	"testing"
)

func TestColumnBuilder_Int64(t *testing.T) {
	cb := NewColumnBuilder("age", DataTypeInt64)
	cb.Append(int64(25))
	cb.Append(int64(30))
	cb.Append(int64(35))

	col := cb.Build()
	if col.Name != "age" {
		t.Errorf("Name = %q, want %q", col.Name, "age")
	}
	if col.DataType != DataTypeInt64 {
		t.Errorf("DataType = %q, want %q", col.DataType, DataTypeInt64)
	}
	if len(col.Values) != 3 {
		t.Fatalf("expected 3 values, got %d", len(col.Values))
	}
	if col.NullMap != nil {
		t.Error("expected nil NullMap when no nulls")
	}
	if col.Stats.NullCount != 0 {
		t.Errorf("NullCount = %d, want 0", col.Stats.NullCount)
	}
	if col.Stats.Cardinality != 3 {
		t.Errorf("Cardinality = %d, want 3", col.Stats.Cardinality)
	}
}

func TestColumnBuilder_Float64(t *testing.T) {
	cb := NewColumnBuilder("score", DataTypeFloat64)
	cb.Append(0.5)
	cb.Append(0.9)

	col := cb.Build()
	if len(col.Values) != 2 {
		t.Fatalf("expected 2 values, got %d", len(col.Values))
	}
	minVal, ok := toFloat(col.Stats.MinValue)
	if !ok || minVal != 0.5 {
		t.Errorf("MinValue = %v, want 0.5", col.Stats.MinValue)
	}
	maxVal, ok := toFloat(col.Stats.MaxValue)
	if !ok || maxVal != 0.9 {
		t.Errorf("MaxValue = %v, want 0.9", col.Stats.MaxValue)
	}
}

func TestColumnBuilder_String(t *testing.T) {
	cb := NewColumnBuilder("name", DataTypeString)
	cb.Append("alice")
	cb.Append("bob")
	cb.Append("alice") // duplicate

	col := cb.Build()
	if len(col.Values) != 3 {
		t.Fatalf("expected 3 values, got %d", len(col.Values))
	}
	if col.Stats.Cardinality != 2 {
		t.Errorf("Cardinality = %d, want 2 (deduplicated)", col.Stats.Cardinality)
	}
}

func TestColumnBuilder_Bool(t *testing.T) {
	cb := NewColumnBuilder("active", DataTypeBool)
	cb.Append(true)
	cb.Append(false)

	col := cb.Build()
	if len(col.Values) != 2 {
		t.Fatalf("expected 2 values, got %d", len(col.Values))
	}
}

func TestColumnBuilder_Bytes(t *testing.T) {
	cb := NewColumnBuilder("data", DataTypeBytes)
	cb.Append([]byte{0x01, 0x02})
	cb.Append([]byte{0x03})

	col := cb.Build()
	if len(col.Values) != 2 {
		t.Fatalf("expected 2 values, got %d", len(col.Values))
	}
}

func TestColumnBuilder_NullHandling(t *testing.T) {
	cb := NewColumnBuilder("score", DataTypeFloat64)
	cb.Append(1.0)
	cb.AppendNull()
	cb.Append(3.0)
	cb.AppendNull()

	col := cb.Build()
	if len(col.Values) != 4 {
		t.Fatalf("expected 4 values, got %d", len(col.Values))
	}
	if col.NullMap == nil {
		t.Fatal("expected NullMap to be set")
	}
	if col.Stats.NullCount != 2 {
		t.Errorf("NullCount = %d, want 2", col.Stats.NullCount)
	}
	if !col.NullMap[1] || !col.NullMap[3] {
		t.Error("expected nulls at index 1 and 3")
	}
	if col.NullMap[0] || col.NullMap[2] {
		t.Error("expected non-null at index 0 and 2")
	}
}

func TestColumnBuilder_Empty(t *testing.T) {
	cb := NewColumnBuilder("empty", DataTypeInt64)
	col := cb.Build()
	if len(col.Values) != 0 {
		t.Errorf("expected 0 values, got %d", len(col.Values))
	}
	if col.Stats.NullCount != 0 {
		t.Errorf("NullCount = %d, want 0", col.Stats.NullCount)
	}
	if col.Stats.Cardinality != 0 {
		t.Errorf("Cardinality = %d, want 0", col.Stats.Cardinality)
	}
}

func TestBatchConverter_FromRows(t *testing.T) {
	schema := []ColumnSchema{
		{Name: "entity_key", Type: DataTypeString, Nullable: false},
		{Name: "age", Type: DataTypeInt64, Nullable: true},
		{Name: "score", Type: DataTypeFloat64, Nullable: true},
	}

	entities := []string{"user:1", "user:2", "user:3"}
	features := map[string]map[string]interface{}{
		"user:1": {"age": int64(25), "score": 0.9},
		"user:2": {"age": int64(30), "score": 0.8},
		"user:3": {"age": int64(35), "score": 0.7},
	}

	bc := NewBatchConverter()
	batch, err := bc.FromRows(entities, features, schema)
	if err != nil {
		t.Fatalf("FromRows: %v", err)
	}
	if batch.RowCount != 3 {
		t.Errorf("RowCount = %d, want 3", batch.RowCount)
	}
	if len(batch.Columns) != 3 {
		t.Errorf("Columns = %d, want 3", len(batch.Columns))
	}
	if batch.ByteSize <= 0 {
		t.Errorf("ByteSize = %d, want > 0", batch.ByteSize)
	}
}

func TestBatchConverter_FromRows_EmptySchema(t *testing.T) {
	bc := NewBatchConverter()
	_, err := bc.FromRows([]string{"u1"}, nil, nil)
	if err == nil {
		t.Fatal("expected error for empty schema")
	}
}

func TestBatchConverter_FromRows_MissingFeatures(t *testing.T) {
	schema := []ColumnSchema{
		{Name: "entity_key", Type: DataTypeString, Nullable: false},
		{Name: "score", Type: DataTypeFloat64, Nullable: true},
	}

	entities := []string{"user:1", "user:2"}
	features := map[string]map[string]interface{}{
		"user:1": {"score": 0.9},
		// user:2 has no features
	}

	bc := NewBatchConverter()
	batch, err := bc.FromRows(entities, features, schema)
	if err != nil {
		t.Fatalf("FromRows: %v", err)
	}
	if batch.RowCount != 2 {
		t.Errorf("RowCount = %d, want 2", batch.RowCount)
	}

	// user:2's score should be null
	scoreCol := findColumn(batch, "score")
	if scoreCol == nil {
		t.Fatal("missing score column")
	}
	if scoreCol.Stats.NullCount != 1 {
		t.Errorf("score NullCount = %d, want 1", scoreCol.Stats.NullCount)
	}
}

func TestBatchConverter_ToRows(t *testing.T) {
	schema := []ColumnSchema{
		{Name: "entity_key", Type: DataTypeString, Nullable: false},
		{Name: "age", Type: DataTypeInt64, Nullable: true},
	}

	batch := &ColumnarBatch{
		Columns: []*Column{
			{Name: "entity_key", DataType: DataTypeString, Values: []interface{}{"u1", "u2"}},
			{Name: "age", DataType: DataTypeInt64, Values: []interface{}{int64(25), int64(30)}},
		},
		RowCount: 2,
		Schema:   schema,
	}

	bc := NewBatchConverter()
	rows, err := bc.ToRows(batch)
	if err != nil {
		t.Fatalf("ToRows: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	if rows[0]["entity_key"] != "u1" {
		t.Errorf("row[0].entity_key = %v, want %q", rows[0]["entity_key"], "u1")
	}
	if rows[1]["age"] != int64(30) {
		t.Errorf("row[1].age = %v, want 30", rows[1]["age"])
	}
}

func TestBatchConverter_ToRows_Nil(t *testing.T) {
	bc := NewBatchConverter()
	_, err := bc.ToRows(nil)
	if err == nil {
		t.Fatal("expected error for nil batch")
	}
}

func TestBatchConverter_ToRows_WithNulls(t *testing.T) {
	batch := &ColumnarBatch{
		Columns: []*Column{
			{Name: "key", DataType: DataTypeString, Values: []interface{}{"a", "b"}},
			{Name: "val", DataType: DataTypeFloat64, Values: []interface{}{1.0, nil}, NullMap: []bool{false, true}},
		},
		RowCount: 2,
	}

	bc := NewBatchConverter()
	rows, err := bc.ToRows(batch)
	if err != nil {
		t.Fatalf("ToRows: %v", err)
	}
	if rows[1]["val"] != nil {
		t.Errorf("expected nil for null value, got %v", rows[1]["val"])
	}
}

func TestBatchConverter_RoundTrip(t *testing.T) {
	schema := []ColumnSchema{
		{Name: "entity_key", Type: DataTypeString, Nullable: false},
		{Name: "score", Type: DataTypeFloat64, Nullable: true},
		{Name: "name", Type: DataTypeString, Nullable: true},
	}

	entities := []string{"u1", "u2"}
	features := map[string]map[string]interface{}{
		"u1": {"score": 0.9, "name": "alice"},
		"u2": {"score": 0.8, "name": "bob"},
	}

	bc := NewBatchConverter()
	batch, err := bc.FromRows(entities, features, schema)
	if err != nil {
		t.Fatalf("FromRows: %v", err)
	}

	rows, err := bc.ToRows(batch)
	if err != nil {
		t.Fatalf("ToRows: %v", err)
	}

	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	if rows[0]["entity_key"] != "u1" {
		t.Errorf("row[0].entity_key = %v, want u1", rows[0]["entity_key"])
	}
	if rows[0]["score"] != 0.9 {
		t.Errorf("row[0].score = %v, want 0.9", rows[0]["score"])
	}
	if rows[1]["name"] != "bob" {
		t.Errorf("row[1].name = %v, want bob", rows[1]["name"])
	}
}

func TestBatchConverter_ProjectColumns(t *testing.T) {
	batch := &ColumnarBatch{
		Columns: []*Column{
			{Name: "entity_key", DataType: DataTypeString, Values: []interface{}{"u1", "u2"}},
			{Name: "age", DataType: DataTypeInt64, Values: []interface{}{int64(25), int64(30)}},
			{Name: "score", DataType: DataTypeFloat64, Values: []interface{}{0.9, 0.8}},
		},
		RowCount: 2,
		Schema: []ColumnSchema{
			{Name: "entity_key", Type: DataTypeString},
			{Name: "age", Type: DataTypeInt64},
			{Name: "score", Type: DataTypeFloat64},
		},
	}

	bc := NewBatchConverter()
	projected, err := bc.ProjectColumns(batch, []string{"entity_key", "score"})
	if err != nil {
		t.Fatalf("ProjectColumns: %v", err)
	}
	if len(projected.Columns) != 2 {
		t.Fatalf("expected 2 columns, got %d", len(projected.Columns))
	}
	if projected.Columns[0].Name != "entity_key" {
		t.Errorf("expected entity_key, got %s", projected.Columns[0].Name)
	}
	if projected.Columns[1].Name != "score" {
		t.Errorf("expected score, got %s", projected.Columns[1].Name)
	}
	if projected.RowCount != 2 {
		t.Errorf("RowCount = %d, want 2", projected.RowCount)
	}
}

func TestBatchConverter_ProjectColumns_Nil(t *testing.T) {
	bc := NewBatchConverter()
	_, err := bc.ProjectColumns(nil, []string{"a"})
	if err == nil {
		t.Fatal("expected error for nil batch")
	}
}

func TestBatchConverter_ProjectColumns_Empty(t *testing.T) {
	batch := &ColumnarBatch{RowCount: 1}
	bc := NewBatchConverter()
	result, err := bc.ProjectColumns(batch, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != batch {
		t.Error("expected same batch returned for empty columns")
	}
}

func TestBatchConverter_EmptyBatch(t *testing.T) {
	schema := []ColumnSchema{
		{Name: "entity_key", Type: DataTypeString, Nullable: false},
		{Name: "val", Type: DataTypeFloat64, Nullable: true},
	}

	bc := NewBatchConverter()
	batch, err := bc.FromRows([]string{}, map[string]map[string]interface{}{}, schema)
	if err != nil {
		t.Fatalf("FromRows: %v", err)
	}
	if batch.RowCount != 0 {
		t.Errorf("RowCount = %d, want 0", batch.RowCount)
	}

	rows, err := bc.ToRows(batch)
	if err != nil {
		t.Fatalf("ToRows: %v", err)
	}
	if len(rows) != 0 {
		t.Errorf("expected 0 rows, got %d", len(rows))
	}
}

func TestBatchConverter_SingleRow(t *testing.T) {
	schema := []ColumnSchema{
		{Name: "entity_key", Type: DataTypeString, Nullable: false},
		{Name: "val", Type: DataTypeFloat64, Nullable: true},
	}

	entities := []string{"u1"}
	features := map[string]map[string]interface{}{
		"u1": {"val": 42.0},
	}

	bc := NewBatchConverter()
	batch, err := bc.FromRows(entities, features, schema)
	if err != nil {
		t.Fatalf("FromRows: %v", err)
	}
	if batch.RowCount != 1 {
		t.Errorf("RowCount = %d, want 1", batch.RowCount)
	}
}

func TestBatchConverter_LargeBatch(t *testing.T) {
	schema := []ColumnSchema{
		{Name: "entity_key", Type: DataTypeString, Nullable: false},
		{Name: "val", Type: DataTypeFloat64, Nullable: true},
	}

	n := 1000
	entities := make([]string, n)
	features := make(map[string]map[string]interface{}, n)
	for i := 0; i < n; i++ {
		key := fmt.Sprintf("entity:%d", i)
		entities[i] = key
		features[key] = map[string]interface{}{"val": float64(i)}
	}

	bc := NewBatchConverter()
	batch, err := bc.FromRows(entities, features, schema)
	if err != nil {
		t.Fatalf("FromRows: %v", err)
	}
	if batch.RowCount != n {
		t.Errorf("RowCount = %d, want %d", batch.RowCount, n)
	}
	if batch.ByteSize <= 0 {
		t.Errorf("ByteSize = %d, want > 0", batch.ByteSize)
	}
}

func TestBatchConverter_RecordBatchRoundTrip(t *testing.T) {
	rb := &RecordBatch{
		Schema: []ColumnSchema{
			{Name: "entity_key", Type: DataTypeString, Nullable: false},
			{Name: "score", Type: DataTypeFloat64, Nullable: true},
		},
		Rows: 2,
		Columns: map[string][]interface{}{
			"entity_key": {"u1", "u2"},
			"score":      {0.5, 0.9},
		},
	}

	bc := NewBatchConverter()
	columnar := bc.FromRecordBatch(rb)
	if columnar.RowCount != 2 {
		t.Fatalf("RowCount = %d, want 2", columnar.RowCount)
	}

	back := bc.ToRecordBatch(columnar)
	if back.Rows != 2 {
		t.Fatalf("Rows = %d, want 2", back.Rows)
	}
	if len(back.Columns["entity_key"]) != 2 {
		t.Fatalf("expected 2 entity_key values")
	}
}

func TestBatchConverter_FromRecordBatch_Nil(t *testing.T) {
	bc := NewBatchConverter()
	if bc.FromRecordBatch(nil) != nil {
		t.Fatal("expected nil for nil RecordBatch")
	}
}

func TestBatchConverter_ToRecordBatch_Nil(t *testing.T) {
	bc := NewBatchConverter()
	if bc.ToRecordBatch(nil) != nil {
		t.Fatal("expected nil for nil ColumnarBatch")
	}
}

func findColumn(batch *ColumnarBatch, name string) *Column {
	for _, col := range batch.Columns {
		if col.Name == name {
			return col
		}
	}
	return nil
}
