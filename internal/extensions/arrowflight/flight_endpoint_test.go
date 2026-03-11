package arrowflight

import (
	"context"
	"testing"
)

func TestFlightEndpointDoGetBatch(t *testing.T) {
	t.Parallel()
	server := NewServer(DefaultConfig())
	reader := &testReader{
		data: &RecordBatch{
			Schema: []ColumnSchema{
				{Name: "entity", Type: DataTypeString},
				{Name: "score", Type: DataTypeFloat64},
			},
			Rows: 3,
			Columns: map[string][]interface{}{
				"entity": {"user:1", "user:2", "user:3"},
				"score":  {0.9, 0.8, 0.7},
			},
		},
	}
	server.SetReader(reader)
	batch := NewBatchServer(server, DefaultBatchConfig())
	endpoint := NewFlightServiceEndpoint(server, batch, DefaultFlightServiceConfig())

	result, err := endpoint.DoGetBatch(context.Background(), []string{"user:1", "user:2"}, []string{"score"})
	if err != nil {
		t.Fatal(err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}

	stats := endpoint.Stats()
	if stats.TotalRequests != 1 {
		t.Errorf("expected 1 request, got %d", stats.TotalRequests)
	}
}

func TestFlightEndpointStats(t *testing.T) {
	t.Parallel()
	server := NewServer(DefaultConfig())
	batch := NewBatchServer(server, DefaultBatchConfig())
	endpoint := NewFlightServiceEndpoint(server, batch, DefaultFlightServiceConfig())

	stats := endpoint.Stats()
	if stats.TotalRequests != 0 {
		t.Errorf("expected 0 requests initially")
	}
	if stats.ErrorCount != 0 {
		t.Errorf("expected 0 errors initially")
	}
}

func TestFlightEndpointDoPutBatch(t *testing.T) {
	t.Parallel()
	server := NewServer(DefaultConfig())
	writer := &testWriter{}
	server.SetWriter(writer)
	batch := NewBatchServer(server, DefaultBatchConfig())
	endpoint := NewFlightServiceEndpoint(server, batch, DefaultFlightServiceConfig())

	cb := &ColumnarBatch{
		Columns: []*Column{
			{Name: "entity", DataType: DataTypeString, Values: []interface{}{"u1"}},
			{Name: "score", DataType: DataTypeFloat64, Values: []interface{}{0.5}},
		},
		RowCount: 1,
		Schema: []ColumnSchema{
			{Name: "entity", Type: DataTypeString},
			{Name: "score", Type: DataTypeFloat64},
		},
	}
	result, err := endpoint.DoPutBatch(context.Background(), cb)
	if err != nil {
		t.Fatal(err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}

// testReader implements FeatureReader for testing.
type testReader struct {
	data *RecordBatch
}

func (r *testReader) ReadBatch(_ context.Context, _ []string, _ []string) (*RecordBatch, error) {
	return r.data, nil
}

// testWriter implements FeatureWriter for testing.
type testWriter struct {
	batches []*RecordBatch
}

func (w *testWriter) WriteBatch(_ context.Context, batch *RecordBatch) (*PutResult, error) {
	w.batches = append(w.batches, batch)
	return &PutResult{RowsWritten: int64(batch.Rows)}, nil
}

func TestFlightServiceEndpoint_GetSchema(t *testing.T) {
	t.Parallel()
	server := NewServer(DefaultConfig())
	batch := NewBatchServer(server, DefaultBatchConfig())
	endpoint := NewFlightServiceEndpoint(server, batch, DefaultFlightServiceConfig())

	desc := FlightDescriptor{
		Type:     "path",
		Features: []string{"age", "score"},
		Entities: []string{"user:1"},
	}

	schema, err := endpoint.GetSchema(context.Background(), desc)
	if err != nil {
		t.Fatalf("GetSchema failed: %v", err)
	}
	if len(schema) != 3 {
		t.Errorf("expected 3 columns (entity_key + 2 features), got %d", len(schema))
	}
	if schema[0].Name != "entity_key" {
		t.Errorf("expected first column entity_key, got %s", schema[0].Name)
	}
}

func TestFlightServiceEndpoint_GetSchema_Error(t *testing.T) {
	t.Parallel()
	server := NewServer(DefaultConfig())
	batch := NewBatchServer(server, DefaultBatchConfig())
	endpoint := NewFlightServiceEndpoint(server, batch, DefaultFlightServiceConfig())

	desc := FlightDescriptor{Type: "path"}
	_, err := endpoint.GetSchema(context.Background(), desc)
	if err == nil {
		t.Error("expected error for empty features")
	}

	stats := endpoint.Stats()
	if stats.ErrorCount != 1 {
		t.Errorf("expected 1 error, got %d", stats.ErrorCount)
	}
}

func TestFlightServiceEndpoint_DoGetRecordBatch(t *testing.T) {
	t.Parallel()
	server := NewServer(DefaultConfig())
	batch := NewBatchServer(server, DefaultBatchConfig())
	endpoint := NewFlightServiceEndpoint(server, batch, DefaultFlightServiceConfig())

	req := BatchRequest{
		Descriptor: FlightDescriptor{
			Type:     "path",
			Features: []string{"age"},
			Entities: []string{"user:1"},
		},
	}

	resp, err := endpoint.DoGetRecordBatch(context.Background(), req)
	if err != nil {
		t.Fatalf("DoGetRecordBatch failed: %v", err)
	}
	if resp == nil || resp.Data == nil {
		t.Fatal("expected non-nil response with data")
	}
}

func TestFlightServiceEndpoint_DoGetRecordBatch_WithFiltering(t *testing.T) {
	t.Parallel()
	server := NewServer(DefaultConfig())
	batch := NewBatchServer(server, DefaultBatchConfig())
	endpoint := NewFlightServiceEndpoint(server, batch, DefaultFlightServiceConfig())

	req := BatchRequest{
		Descriptor: FlightDescriptor{
			Type:     "path",
			Features: []string{"age", "score"},
			Entities: []string{"user:1"},
		},
		Predicates: []Predicate{{Column: "age", Operator: "gt", Value: 10}},
		Columns:    []string{"age"},
		Limit:      5,
		Offset:     0,
	}

	resp, err := endpoint.DoGetRecordBatch(context.Background(), req)
	if err != nil {
		t.Fatalf("DoGetRecordBatch with filtering failed: %v", err)
	}
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
}

func TestFlightServiceEndpoint_HealthCheck(t *testing.T) {
	t.Parallel()

	server := NewServer(DefaultConfig())
	batch := NewBatchServer(server, DefaultBatchConfig())

	endpoint := NewFlightServiceEndpoint(server, batch, DefaultFlightServiceConfig())
	if err := endpoint.HealthCheck(); err != nil {
		t.Errorf("expected healthy endpoint, got: %v", err)
	}

	if err := NewFlightServiceEndpoint(nil, batch, DefaultFlightServiceConfig()).HealthCheck(); err == nil {
		t.Error("expected error for nil server")
	}

	if err := NewFlightServiceEndpoint(server, nil, DefaultFlightServiceConfig()).HealthCheck(); err == nil {
		t.Error("expected error for nil batch server")
	}
}
