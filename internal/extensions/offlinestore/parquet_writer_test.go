package offlinestore

import (
	"path/filepath"
	"testing"
	"time"
)

func TestWriteAndReadRows(t *testing.T) {
	dir := t.TempDir()
	cfg := DefaultParquetConfig()
	cfg.BasePath = dir
	cfg.Partitioned = false

	pw, err := NewParquetWriter(cfg)
	if err != nil {
		t.Fatalf("NewParquetWriter: %v", err)
	}

	now := time.Now()
	rows := []FeatureRow{
		{EntityID: "u1", Features: map[string]interface{}{"age": float64(25)}, Timestamp: now.Add(-2 * time.Hour)},
		{EntityID: "u2", Features: map[string]interface{}{"age": float64(30)}, Timestamp: now.Add(-1 * time.Hour)},
		{EntityID: "u3", Features: map[string]interface{}{"age": float64(35)}, Timestamp: now},
	}

	path, size, err := pw.WriteRows("test_ds", "", rows)
	if err != nil {
		t.Fatalf("WriteRows: %v", err)
	}
	if size <= 0 {
		t.Errorf("expected positive size, got %d", size)
	}

	got, err := pw.ReadRows(path, 0)
	if err != nil {
		t.Fatalf("ReadRows: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(got))
	}
	for i, row := range got {
		if row.EntityID != rows[i].EntityID {
			t.Errorf("row %d: expected entity %s, got %s", i, rows[i].EntityID, row.EntityID)
		}
		if row.Features["age"] != rows[i].Features["age"] {
			t.Errorf("row %d: expected age %v, got %v", i, rows[i].Features["age"], row.Features["age"])
		}
	}

	// Test limit
	got, err = pw.ReadRows(path, 1)
	if err != nil {
		t.Fatalf("ReadRows with limit: %v", err)
	}
	if len(got) != 1 {
		t.Errorf("expected 1 row with limit, got %d", len(got))
	}
}

func TestWriteRowsEmpty(t *testing.T) {
	dir := t.TempDir()
	cfg := DefaultParquetConfig()
	cfg.BasePath = dir

	pw, err := NewParquetWriter(cfg)
	if err != nil {
		t.Fatalf("NewParquetWriter: %v", err)
	}

	_, _, err = pw.WriteRows("ds", "", nil)
	if err == nil {
		t.Fatal("expected error for empty rows")
	}
}

func TestPartitionedWrites(t *testing.T) {
	dir := t.TempDir()
	cfg := DefaultParquetConfig()
	cfg.BasePath = dir
	cfg.Partitioned = true

	pw, err := NewParquetWriter(cfg)
	if err != nil {
		t.Fatalf("NewParquetWriter: %v", err)
	}

	rows := []FeatureRow{
		{EntityID: "u1", Features: map[string]interface{}{"score": float64(0.9)}, Timestamp: time.Now()},
	}

	path, _, err := pw.WriteRows("ds", "user", rows)
	if err != nil {
		t.Fatalf("WriteRows partitioned: %v", err)
	}

	// Path should contain entity_type partition
	rel, _ := filepath.Rel(dir, path)
	if !filepath.IsAbs(path) && rel == path {
		t.Errorf("expected path under base dir, got %s", path)
	}

	partitions, err := pw.ListPartitions("ds")
	if err != nil {
		t.Fatalf("ListPartitions: %v", err)
	}
	if len(partitions) == 0 {
		t.Fatal("expected at least one partition")
	}
}

func TestExportToParquet(t *testing.T) {
	dir := t.TempDir()
	cfg := DefaultParquetConfig()
	cfg.BasePath = dir
	cfg.Partitioned = false

	pw, err := NewParquetWriter(cfg)
	if err != nil {
		t.Fatalf("NewParquetWriter: %v", err)
	}

	rows := []FeatureRow{
		{EntityID: "u1", Features: map[string]interface{}{"val": float64(1)}, Timestamp: time.Now()},
		{EntityID: "u2", Features: map[string]interface{}{"val": float64(2)}, Timestamp: time.Now()},
	}

	result, err := pw.ExportToParquet("export_ds", rows)
	if err != nil {
		t.Fatalf("ExportToParquet: %v", err)
	}
	if result.Dataset != "export_ds" {
		t.Errorf("expected dataset export_ds, got %s", result.Dataset)
	}
	if result.Format != "parquet" {
		t.Errorf("expected format parquet, got %s", result.Format)
	}
	if result.RowCount != 2 {
		t.Errorf("expected 2 rows, got %d", result.RowCount)
	}
	if result.SizeEstimate <= 0 {
		t.Errorf("expected positive size estimate, got %d", result.SizeEstimate)
	}
}

func TestListPartitionsEmpty(t *testing.T) {
	dir := t.TempDir()
	cfg := DefaultParquetConfig()
	cfg.BasePath = dir

	pw, err := NewParquetWriter(cfg)
	if err != nil {
		t.Fatalf("NewParquetWriter: %v", err)
	}

	partitions, err := pw.ListPartitions("nonexistent")
	if err != nil {
		t.Fatalf("ListPartitions: %v", err)
	}
	if partitions != nil {
		t.Errorf("expected nil for nonexistent dataset, got %v", partitions)
	}
}
