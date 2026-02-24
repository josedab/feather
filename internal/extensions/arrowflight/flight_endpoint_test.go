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
