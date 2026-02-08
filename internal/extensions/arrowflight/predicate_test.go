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
