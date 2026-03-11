package arrowflight

import (
	"context"
	"testing"
)

func TestPredicateEvaluator_Apply(t *testing.T) {
	batch := &RecordBatch{
		Schema: []ColumnSchema{
			{Name: "id", Type: DataTypeInt64},
			{Name: "score", Type: DataTypeFloat64},
		},
		Rows: 3,
		Columns: map[string][]interface{}{
			"id":    {1, 2, 3},
			"score": {0.5, 0.8, 0.3},
		},
	}

	pe := NewPredicateEvaluator()

	// Filter: score > 0.4
	result := pe.Apply(batch, []Predicate{
		{Column: "score", Operator: "gt", Value: 0.4},
	})
	if result.Rows != 2 {
		t.Errorf("expected 2 rows after filter, got %d", result.Rows)
	}
}

func TestPredicateEvaluator_ProjectColumns(t *testing.T) {
	batch := &RecordBatch{
		Schema: []ColumnSchema{
			{Name: "id", Type: DataTypeInt64},
			{Name: "score", Type: DataTypeFloat64},
			{Name: "name", Type: DataTypeString},
		},
		Rows: 2,
		Columns: map[string][]interface{}{
			"id":    {1, 2},
			"score": {0.5, 0.8},
			"name":  {"alice", "bob"},
		},
	}

	pe := NewPredicateEvaluator()
	result := pe.ProjectColumns(batch, []string{"id", "score"})
	if len(result.Columns) != 2 {
		t.Errorf("expected 2 columns, got %d", len(result.Columns))
	}
	if _, ok := result.Columns["name"]; ok {
		t.Error("name column should have been projected out")
	}
}

func TestPredicateEvaluator_InOperator(t *testing.T) {
	batch := &RecordBatch{
		Schema: []ColumnSchema{{Name: "city", Type: DataTypeString}},
		Rows:   3,
		Columns: map[string][]interface{}{
			"city": {"NYC", "LA", "SF"},
		},
	}

	pe := NewPredicateEvaluator()
	result := pe.Apply(batch, []Predicate{
		{Column: "city", Operator: "in", Values: []interface{}{"NYC", "SF"}},
	})
	if result.Rows != 2 {
		t.Errorf("expected 2 rows for IN filter, got %d", result.Rows)
	}
}

func TestServer_GetBatch(t *testing.T) {
	server := NewServer(DefaultConfig())

	req := BatchRequest{
		Descriptor: FlightDescriptor{
			Features: []string{"score"},
			Entities: []string{"user:1", "user:2"},
		},
	}

	resp, err := server.GetBatch(context.Background(), req)
	if err != nil {
		t.Fatalf("GetBatch failed: %v", err)
	}
	if resp == nil {
		t.Fatal("expected response")
	}
}

func TestPredicateEvaluator_CompareNumeric(t *testing.T) {
	t.Parallel()
	pe := NewPredicateEvaluator()

	tests := []struct {
		name string
		a, b interface{}
		op   string
		want bool
	}{
		{"gt float64", float64(10), float64(5), "gt", true},
		{"gt false", float64(5), float64(10), "gt", false},
		{"gte equal", float64(5), float64(5), "gte", true},
		{"gte less", float64(3), float64(5), "gte", false},
		{"lt", float64(3), float64(5), "lt", true},
		{"lt false", float64(5), float64(3), "lt", false},
		{"lte equal", float64(5), float64(5), "lte", true},
		{"lte greater", float64(7), float64(5), "lte", false},
		{"int types", int64(10), int64(5), "gt", true},
		{"int32", int32(3), int32(5), "lt", true},
		{"mixed", int(10), float64(5), "gt", true},
		{"string non-numeric", "abc", "def", "gt", false},
		{"unknown op", float64(5), float64(5), "unknown", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := pe.compareNumeric(tt.a, tt.b, tt.op); got != tt.want {
				t.Errorf("compareNumeric(%v, %v, %s) = %v, want %v", tt.a, tt.b, tt.op, got, tt.want)
			}
		})
	}
}

func TestToFloat(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input   interface{}
		wantVal float64
		wantOk  bool
	}{
		{float64(3.14), 3.14, true},
		{float32(2.5), 2.5, true},
		{int(42), 42.0, true},
		{int64(100), 100.0, true},
		{int32(7), 7.0, true},
		{"string", 0, false},
		{nil, 0, false},
		{true, 0, false},
	}
	for _, tt := range tests {
		v, ok := toFloat(tt.input)
		if ok != tt.wantOk || v != tt.wantVal {
			t.Errorf("toFloat(%v) = (%f, %v), want (%f, %v)", tt.input, v, ok, tt.wantVal, tt.wantOk)
		}
	}
}

func TestPredicateEvaluator_NotNull(t *testing.T) {
	t.Parallel()
	pe := NewPredicateEvaluator()

	batch := &RecordBatch{
		Schema:  []ColumnSchema{{Name: "val", Type: DataTypeFloat64, Nullable: true}},
		Rows:    3,
		Columns: map[string][]interface{}{"val": {float64(1), nil, float64(3)}},
	}

	result := pe.Apply(batch, []Predicate{{Column: "val", Operator: "not_null"}})
	if result.Rows != 2 {
		t.Errorf("expected 2 non-null rows, got %d", result.Rows)
	}
}

func TestPredicateEvaluator_MissingColumn(t *testing.T) {
	t.Parallel()
	pe := NewPredicateEvaluator()

	batch := &RecordBatch{
		Schema:  []ColumnSchema{{Name: "x", Type: DataTypeFloat64}},
		Rows:    2,
		Columns: map[string][]interface{}{"x": {1.0, 2.0}},
	}

	result := pe.Apply(batch, []Predicate{{Column: "nonexistent", Operator: "eq", Value: 1}})
	if result.Rows != 0 {
		t.Errorf("expected 0 rows for missing column, got %d", result.Rows)
	}
}

func TestGetBatch_NoFeatures(t *testing.T) {
	t.Parallel()
	server := NewServer(DefaultConfig())
	_, err := server.GetBatch(context.Background(), BatchRequest{
		Descriptor: FlightDescriptor{Type: "path"},
	})
	if err == nil {
		t.Error("expected error for no features")
	}
}

func TestGetBatch_WithLimitOffset(t *testing.T) {
	t.Parallel()
	reader := &testReader{
		data: &RecordBatch{
			Schema: []ColumnSchema{
				{Name: "entity_key", Type: DataTypeString},
				{Name: "score", Type: DataTypeFloat64},
			},
			Rows: 5,
			Columns: map[string][]interface{}{
				"entity_key": {"u:1", "u:2", "u:3", "u:4", "u:5"},
				"score":      {1.0, 2.0, 3.0, 4.0, 5.0},
			},
		},
	}
	server := NewServer(DefaultConfig())
	server.SetReader(reader)

	resp, err := server.GetBatch(context.Background(), BatchRequest{
		Descriptor: FlightDescriptor{
			Type:     "path",
			Features: []string{"score"},
			Entities: []string{"u:1", "u:2", "u:3", "u:4", "u:5"},
		},
		Limit:  2,
		Offset: 1,
	})
	if err != nil {
		t.Fatalf("GetBatch with limit/offset failed: %v", err)
	}
	if resp.Data.Rows != 2 {
		t.Errorf("expected 2 rows with limit, got %d", resp.Data.Rows)
	}
	if !resp.HasMore {
		t.Error("expected HasMore=true")
	}
}
