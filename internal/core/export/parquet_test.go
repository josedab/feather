package export

import (
	"bytes"
	"testing"
)

func TestWriteEmptyParquet(t *testing.T) {
	var buf bytes.Buffer
	err := writeEmptyParquet(&buf)
	if err != nil {
		t.Fatalf("writeEmptyParquet() error: %v", err)
	}

	// Parquet files have a magic number: "PAR1"
	data := buf.Bytes()
	if len(data) < 4 {
		t.Fatalf("output too small: %d bytes", len(data))
	}
	if string(data[:4]) != "PAR1" {
		t.Errorf("missing PAR1 header, got %q", string(data[:4]))
	}

	// Should also end with PAR1
	if string(data[len(data)-4:]) != "PAR1" {
		t.Errorf("missing PAR1 footer, got %q", string(data[len(data)-4:]))
	}
}

func TestWriteParquet_EmptyRows(t *testing.T) {
	var buf bytes.Buffer
	err := writeParquet(&buf, nil, []string{"feature1", "feature2"})
	if err != nil {
		t.Fatalf("writeParquet() with empty rows error: %v", err)
	}

	// Should produce a valid empty parquet file
	data := buf.Bytes()
	if len(data) < 4 {
		t.Fatalf("output too small: %d bytes", len(data))
	}
	if string(data[:4]) != "PAR1" {
		t.Errorf("missing PAR1 header")
	}
}

func TestWriteParquet_WithRows(t *testing.T) {
	rows := []ParquetRow{
		{
			EntityKey: "user:1",
			Timestamp: 1704067200000000000,
			Features:  map[string]interface{}{"score": 0.95},
		},
		{
			EntityKey: "user:2",
			Timestamp: 1704067200000000000,
			Features:  map[string]interface{}{"score": 0.85},
		},
	}

	var buf bytes.Buffer
	err := writeParquet(&buf, rows, []string{"score"})
	if err != nil {
		t.Fatalf("writeParquet() error: %v", err)
	}

	data := buf.Bytes()
	if len(data) < 4 {
		t.Fatalf("output too small: %d bytes", len(data))
	}
	if string(data[:4]) != "PAR1" {
		t.Errorf("missing PAR1 header")
	}
}

func TestParquetSchemaBuilder_Fields(t *testing.T) {
	b := NewParquetSchemaBuilder()
	if b == nil {
		t.Fatal("NewParquetSchemaBuilder returned nil")
	}

	b.AddStringField("Name")
	b.AddInt64Field("Count")
	b.AddFloat64Field("Score")

	if len(b.fields) != 3 {
		t.Errorf("expected 3 fields, got %d", len(b.fields))
	}
}
