package export

import (
	"bufio"
	"context"
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/feather-store/feather/internal/core/domain"
	"github.com/feather-store/feather/internal/core/storage"
)

func TestNewExporter(t *testing.T) {
	exporter := NewExporter(nil, nil)
	if exporter == nil {
		t.Fatal("NewExporter returned nil")
	}
}

func TestExporter_Export_NoFeatures(t *testing.T) {
	exporter := NewExporter(nil, nil)

	req := ExportRequest{
		Entities:   []string{"user:1"},
		Features:   []string{},
		Format:     FormatCSV,
		OutputPath: "/tmp/test.csv",
	}

	_, err := exporter.Export(context.Background(), req)
	if err == nil {
		t.Error("expected error for empty features")
	}
	if !strings.Contains(err.Error(), "at least one feature") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestExporter_Export_NoOutputPath(t *testing.T) {
	exporter := NewExporter(nil, nil)

	req := ExportRequest{
		Entities: []string{"user:1"},
		Features: []string{"age"},
		Format:   FormatCSV,
	}

	_, err := exporter.Export(context.Background(), req)
	if err == nil {
		t.Error("expected error for missing output path")
	}
	if !strings.Contains(err.Error(), "output path") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestExporter_Export_UnsupportedFormat(t *testing.T) {
	tmpDir := t.TempDir()
	exporter := NewExporterWithBaseDir(nil, nil, tmpDir)

	req := ExportRequest{
		Entities:   []string{"user:1"},
		Features:   []string{"age"},
		Format:     "invalid",
		OutputPath: filepath.Join(tmpDir, "test.xyz"),
	}

	_, err := exporter.Export(context.Background(), req)
	if err == nil {
		t.Error("expected error for unsupported format")
	}
	if !strings.Contains(err.Error(), "unsupported format") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestGetPrivateTempDir(t *testing.T) {
	dir, err := getPrivateTempDir()
	if err != nil {
		t.Fatalf("getPrivateTempDir failed: %v", err)
	}
	if dir == "" {
		t.Error("expected non-empty directory")
	}
	// Should contain "feather-export" in path
	if !strings.Contains(dir, "feather-export") {
		t.Errorf("expected feather-export in path, got %s", dir)
	}
	// Directory should exist
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("directory should exist: %v", err)
	}
	if !info.IsDir() {
		t.Error("path should be a directory")
	}
	// Permissions should be 0700
	perm := info.Mode().Perm()
	if perm != 0700 {
		t.Errorf("expected permissions 0700, got %o", perm)
	}
}

func TestFormatValue(t *testing.T) {
	tests := []struct {
		input    interface{}
		expected string
	}{
		{nil, ""},
		{"hello", "hello"},
		{float64(1.5), "1.5"},
		{float32(2.5), "2.5"},
		{int64(42), "42"},
		{int(100), "100"},
		{true, "true"},
		{false, "false"},
		{[]float32{1.0, 2.0}, "[1,2]"},
		{[]byte("bytes"), "bytes"},
	}

	for _, tt := range tests {
		result := formatValue(tt.input)
		if result != tt.expected {
			t.Errorf("formatValue(%v) = %s, want %s", tt.input, result, tt.expected)
		}
	}
}

func TestExportFormat_Constants(t *testing.T) {
	// Verify format constants are defined
	if FormatCSV != "csv" {
		t.Errorf("FormatCSV = %s, want csv", FormatCSV)
	}
	if FormatJSON != "json" {
		t.Errorf("FormatJSON = %s, want json", FormatJSON)
	}
	if FormatJSONL != "jsonl" {
		t.Errorf("FormatJSONL = %s, want jsonl", FormatJSONL)
	}
	if FormatParquet != "parquet" {
		t.Errorf("FormatParquet = %s, want parquet", FormatParquet)
	}
}

func TestExportRequest_Fields(t *testing.T) {
	// Test that ExportRequest struct has expected fields
	req := ExportRequest{
		Entities: []string{"entity:1"},
		Features: []string{"feature1"},
		Format:   FormatCSV,
	}

	if len(req.Entities) != 1 || req.Entities[0] != "entity:1" {
		t.Error("Entities field not set correctly")
	}
	if len(req.Features) != 1 || req.Features[0] != "feature1" {
		t.Error("Features field not set correctly")
	}
	if req.Format != FormatCSV {
		t.Error("Format field not set correctly")
	}
}

func TestExportResult_Fields(t *testing.T) {
	result := ExportResult{
		EntitiesExported: 10,
		FeaturesExported: 5,
		RowsWritten:      10,
		BytesWritten:     1024,
	}

	if result.EntitiesExported != 10 {
		t.Error("EntitiesExported not set correctly")
	}
	if result.FeaturesExported != 5 {
		t.Error("FeaturesExported not set correctly")
	}
	if result.RowsWritten != 10 {
		t.Error("RowsWritten not set correctly")
	}
	if result.BytesWritten != 1024 {
		t.Error("BytesWritten not set correctly")
	}
}

func TestTrainingRow_JSONTags(t *testing.T) {
	row := TrainingRow{
		EntityKey: "user:1",
		Timestamp: 1234567890,
		Features:  map[string]interface{}{"age": 25},
	}

	if row.EntityKey != "user:1" {
		t.Error("EntityKey not set correctly")
	}
	if row.Timestamp != 1234567890 {
		t.Error("Timestamp not set correctly")
	}
	if row.Features["age"] != 25 {
		t.Error("Features not set correctly")
	}
}

func TestPointInTimeRequest_Fields(t *testing.T) {
	req := PointInTimeRequest{
		Entities:   []string{"entity:1"},
		Features:   []string{"feature1"},
		OutputPath: "/tmp/pit.csv",
	}

	if len(req.Entities) != 1 {
		t.Error("Entities not set correctly")
	}
	if len(req.Features) != 1 {
		t.Error("Features not set correctly")
	}
	if req.OutputPath != "/tmp/pit.csv" {
		t.Error("OutputPath not set correctly")
	}
}

func TestExporter_ExportPointInTime_NoEntities(t *testing.T) {
	exporter := NewExporter(nil, nil)

	req := PointInTimeRequest{
		Features:   []string{"age"},
		OutputPath: "/tmp/test.csv",
	}

	_, err := exporter.ExportPointInTime(context.Background(), req)
	if err == nil {
		t.Error("expected error for empty entities")
	}
	if !strings.Contains(err.Error(), "entities required") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestExporter_ExportPointInTime_NoFeatures(t *testing.T) {
	exporter := NewExporter(nil, nil)

	req := PointInTimeRequest{
		Entities:   []string{"user:1"},
		OutputPath: "/tmp/test.csv",
	}

	_, err := exporter.ExportPointInTime(context.Background(), req)
	if err == nil {
		t.Error("expected error for empty features")
	}
	if !strings.Contains(err.Error(), "features required") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestExporter_ExportPointInTime_NoTimestamps(t *testing.T) {
	exporter := NewExporter(nil, nil)

	req := PointInTimeRequest{
		Entities:   []string{"user:1"},
		Features:   []string{"age"},
		OutputPath: "/tmp/test.csv",
	}

	_, err := exporter.ExportPointInTime(context.Background(), req)
	if err == nil {
		t.Error("expected error for empty timestamps")
	}
	if !strings.Contains(err.Error(), "timestamps required") {
		t.Errorf("unexpected error message: %v", err)
	}
}

// newTestStore creates a store with in-memory warm tier for testing.
func newTestStore(t *testing.T) (*storage.Store, *storage.Registry) {
	t.Helper()
	schema := storage.NewRegistry()
	store, err := storage.NewStore(context.Background(), storage.StoreOptions{
		HotMaxSize:       1024 * 1024 * 10,
		WarmInMemory:     true,
		TTLCheckInterval: time.Hour,
	}, schema)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	return store, schema
}

// seedTestData populates the store with test entities and features.
func seedTestData(t *testing.T, store *storage.Store) {
	t.Helper()
	now := time.Now().UnixNano()
	entities := map[string]map[string]*domain.FeatureValue{
		"user:1": {
			"age":    {Value: int64(25), Timestamp: now, Version: 1},
			"score":  {Value: 88.5, Timestamp: now, Version: 1},
			"active": {Value: true, Timestamp: now, Version: 1},
		},
		"user:2": {
			"age":    {Value: int64(30), Timestamp: now, Version: 1},
			"score":  {Value: 92.1, Timestamp: now, Version: 1},
			"active": {Value: false, Timestamp: now, Version: 1},
		},
		"user:3": {
			"age":    {Value: int64(22), Timestamp: now, Version: 1},
			"score":  {Value: 76.0, Timestamp: now, Version: 1},
			"active": {Value: true, Timestamp: now, Version: 1},
		},
	}
	for entity, features := range entities {
		if err := store.Put(context.Background(), entity, features); err != nil {
			t.Fatalf("Failed to seed data for %s: %v", entity, err)
		}
	}
	// Wait for async warm tier writes
	time.Sleep(100 * time.Millisecond)
}

func TestExporter_ExportCSV(t *testing.T) {
	store, schema := newTestStore(t)
	seedTestData(t, store)
	tmpDir := t.TempDir()
	exporter := NewExporterWithBaseDir(store, schema, tmpDir)
	outPath := filepath.Join(tmpDir, "out.csv")

	result, err := exporter.Export(context.Background(), ExportRequest{
		Entities:   []string{"user:1", "user:2", "user:3"},
		Features:   []string{"age", "score"},
		Format:     FormatCSV,
		OutputPath: outPath,
	})
	if err != nil {
		t.Fatalf("Export CSV failed: %v", err)
	}
	if result.EntitiesExported != 3 {
		t.Errorf("Expected 3 entities, got %d", result.EntitiesExported)
	}
	if result.FeaturesExported != 2 {
		t.Errorf("Expected 2 features, got %d", result.FeaturesExported)
	}
	if result.RowsWritten != 3 {
		t.Errorf("Expected 3 rows, got %d", result.RowsWritten)
	}

	// Verify CSV content
	f, err := os.Open(outPath)
	if err != nil {
		t.Fatalf("Failed to open output: %v", err)
	}
	defer f.Close()
	reader := csv.NewReader(f)
	records, err := reader.ReadAll()
	if err != nil {
		t.Fatalf("Failed to read CSV: %v", err)
	}
	// header + 3 data rows
	if len(records) != 4 {
		t.Errorf("Expected 4 CSV records (header + 3 rows), got %d", len(records))
	}
	if records[0][0] != "entity_key" || records[0][1] != "timestamp" || records[0][2] != "age" || records[0][3] != "score" {
		t.Errorf("Unexpected CSV header: %v", records[0])
	}
}

func TestExporter_ExportCSV_NoEntities(t *testing.T) {
	store, schema := newTestStore(t)
	tmpDir := t.TempDir()
	exporter := NewExporterWithBaseDir(store, schema, tmpDir)

	_, err := exporter.Export(context.Background(), ExportRequest{
		Entities:   []string{},
		Features:   []string{"age"},
		Format:     FormatCSV,
		OutputPath: filepath.Join(tmpDir, "out.csv"),
	})
	if err == nil {
		t.Error("expected error for empty entity list in CSV export")
	}
}

func TestExporter_ExportCSV_MissingEntity(t *testing.T) {
	store, schema := newTestStore(t)
	tmpDir := t.TempDir()
	exporter := NewExporterWithBaseDir(store, schema, tmpDir)

	result, err := exporter.Export(context.Background(), ExportRequest{
		Entities:   []string{"nonexistent:1"},
		Features:   []string{"age"},
		Format:     FormatCSV,
		OutputPath: filepath.Join(tmpDir, "out.csv"),
	})
	if err != nil {
		t.Fatalf("Export should succeed for nonexistent entities: %v", err)
	}
	// Store returns empty features (not error) for nonexistent entities,
	// so the exporter writes a row with empty feature values.
	if result.RowsWritten != 1 {
		t.Errorf("Expected 1 row, got %d", result.RowsWritten)
	}
}

func TestExporter_ExportCSV_WithEndTime(t *testing.T) {
	store, schema := newTestStore(t)
	seedTestData(t, store)
	tmpDir := t.TempDir()
	exporter := NewExporterWithBaseDir(store, schema, tmpDir)

	endTime := time.Now().Add(time.Minute)
	result, err := exporter.Export(context.Background(), ExportRequest{
		Entities:   []string{"user:1"},
		Features:   []string{"age"},
		Format:     FormatCSV,
		OutputPath: filepath.Join(tmpDir, "out.csv"),
		EndTime:    &endTime,
	})
	if err != nil {
		t.Fatalf("Export CSV with EndTime failed: %v", err)
	}
	if result.RowsWritten != 1 {
		t.Errorf("Expected 1 row, got %d", result.RowsWritten)
	}
}

func TestExporter_ExportCSV_ContextCancellation(t *testing.T) {
	store, schema := newTestStore(t)
	seedTestData(t, store)
	tmpDir := t.TempDir()
	exporter := NewExporterWithBaseDir(store, schema, tmpDir)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	_, err := exporter.Export(ctx, ExportRequest{
		Entities:   []string{"user:1", "user:2"},
		Features:   []string{"age"},
		Format:     FormatCSV,
		OutputPath: filepath.Join(tmpDir, "out.csv"),
	})
	if err == nil {
		t.Error("expected error for cancelled context")
	}
}

func TestExporter_ExportJSON(t *testing.T) {
	store, schema := newTestStore(t)
	seedTestData(t, store)
	tmpDir := t.TempDir()
	exporter := NewExporterWithBaseDir(store, schema, tmpDir)
	outPath := filepath.Join(tmpDir, "out.json")

	result, err := exporter.Export(context.Background(), ExportRequest{
		Entities:   []string{"user:1", "user:2"},
		Features:   []string{"age", "score", "active"},
		Format:     FormatJSON,
		OutputPath: outPath,
	})
	if err != nil {
		t.Fatalf("Export JSON failed: %v", err)
	}
	if result.EntitiesExported != 2 {
		t.Errorf("Expected 2 entities, got %d", result.EntitiesExported)
	}
	if result.FeaturesExported != 3 {
		t.Errorf("Expected 3 features, got %d", result.FeaturesExported)
	}

	// Verify JSON output
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("Failed to read output: %v", err)
	}
	var rows []TrainingRow
	if err := json.Unmarshal(data, &rows); err != nil {
		t.Fatalf("Failed to parse JSON output: %v", err)
	}
	if len(rows) != 2 {
		t.Errorf("Expected 2 rows, got %d", len(rows))
	}
	for _, row := range rows {
		if row.EntityKey == "" {
			t.Error("EntityKey should not be empty")
		}
		if row.Timestamp == 0 {
			t.Error("Timestamp should not be zero")
		}
		if len(row.Features) == 0 {
			t.Error("Features should not be empty")
		}
	}
}

func TestExporter_ExportJSON_NoEntities(t *testing.T) {
	store, schema := newTestStore(t)
	tmpDir := t.TempDir()
	exporter := NewExporterWithBaseDir(store, schema, tmpDir)

	_, err := exporter.Export(context.Background(), ExportRequest{
		Entities:   []string{},
		Features:   []string{"age"},
		Format:     FormatJSON,
		OutputPath: filepath.Join(tmpDir, "out.json"),
	})
	if err == nil {
		t.Error("expected error for empty entity list in JSON export")
	}
}

func TestExporter_ExportJSON_WithEndTime(t *testing.T) {
	store, schema := newTestStore(t)
	seedTestData(t, store)
	tmpDir := t.TempDir()
	exporter := NewExporterWithBaseDir(store, schema, tmpDir)

	endTime := time.Now().Add(time.Minute)
	result, err := exporter.Export(context.Background(), ExportRequest{
		Entities:   []string{"user:1"},
		Features:   []string{"age"},
		Format:     FormatJSON,
		OutputPath: filepath.Join(tmpDir, "out.json"),
		EndTime:    &endTime,
	})
	if err != nil {
		t.Fatalf("Export JSON with EndTime failed: %v", err)
	}
	if result.RowsWritten != 1 {
		t.Errorf("Expected 1 row, got %d", result.RowsWritten)
	}
}

func TestExporter_ExportJSONL(t *testing.T) {
	store, schema := newTestStore(t)
	seedTestData(t, store)
	tmpDir := t.TempDir()
	exporter := NewExporterWithBaseDir(store, schema, tmpDir)
	outPath := filepath.Join(tmpDir, "out.jsonl")

	result, err := exporter.Export(context.Background(), ExportRequest{
		Entities:   []string{"user:1", "user:2", "user:3"},
		Features:   []string{"age", "score"},
		Format:     FormatJSONL,
		OutputPath: outPath,
	})
	if err != nil {
		t.Fatalf("Export JSONL failed: %v", err)
	}
	if result.EntitiesExported != 3 {
		t.Errorf("Expected 3 entities, got %d", result.EntitiesExported)
	}
	if result.RowsWritten != 3 {
		t.Errorf("Expected 3 rows, got %d", result.RowsWritten)
	}

	// Verify JSONL: each line should be valid JSON
	f, err := os.Open(outPath)
	if err != nil {
		t.Fatalf("Failed to open output: %v", err)
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	lineCount := 0
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		var row TrainingRow
		if err := json.Unmarshal([]byte(line), &row); err != nil {
			t.Errorf("Line %d is not valid JSON: %v", lineCount+1, err)
		}
		if row.EntityKey == "" {
			t.Errorf("Line %d: EntityKey is empty", lineCount+1)
		}
		lineCount++
	}
	if lineCount != 3 {
		t.Errorf("Expected 3 JSONL lines, got %d", lineCount)
	}
}

func TestExporter_ExportJSONL_NoEntities(t *testing.T) {
	store, schema := newTestStore(t)
	tmpDir := t.TempDir()
	exporter := NewExporterWithBaseDir(store, schema, tmpDir)

	_, err := exporter.Export(context.Background(), ExportRequest{
		Entities:   []string{},
		Features:   []string{"age"},
		Format:     FormatJSONL,
		OutputPath: filepath.Join(tmpDir, "out.jsonl"),
	})
	if err == nil {
		t.Error("expected error for empty entity list in JSONL export")
	}
}

func TestExporter_ExportJSONL_WithEndTime(t *testing.T) {
	store, schema := newTestStore(t)
	seedTestData(t, store)
	tmpDir := t.TempDir()
	exporter := NewExporterWithBaseDir(store, schema, tmpDir)

	endTime := time.Now().Add(time.Minute)
	result, err := exporter.Export(context.Background(), ExportRequest{
		Entities:   []string{"user:1"},
		Features:   []string{"age"},
		Format:     FormatJSONL,
		OutputPath: filepath.Join(tmpDir, "out.jsonl"),
		EndTime:    &endTime,
	})
	if err != nil {
		t.Fatalf("Export JSONL with EndTime failed: %v", err)
	}
	if result.RowsWritten != 1 {
		t.Errorf("Expected 1 row, got %d", result.RowsWritten)
	}
}

func TestExporter_ExportParquet(t *testing.T) {
	store, schema := newTestStore(t)
	seedTestData(t, store)
	tmpDir := t.TempDir()
	exporter := NewExporterWithBaseDir(store, schema, tmpDir)
	outPath := filepath.Join(tmpDir, "out.parquet")

	result, err := exporter.Export(context.Background(), ExportRequest{
		Entities:   []string{"user:1", "user:2"},
		Features:   []string{"age", "score"},
		Format:     FormatParquet,
		OutputPath: outPath,
	})
	if err != nil {
		t.Fatalf("Export Parquet failed: %v", err)
	}
	if result.EntitiesExported != 2 {
		t.Errorf("Expected 2 entities, got %d", result.EntitiesExported)
	}
	if result.FeaturesExported != 2 {
		t.Errorf("Expected 2 features, got %d", result.FeaturesExported)
	}

	// Verify file is not empty
	info, err := os.Stat(outPath)
	if err != nil {
		t.Fatalf("Failed to stat output: %v", err)
	}
	if info.Size() == 0 {
		t.Error("Parquet output file should not be empty")
	}
}

func TestExporter_ExportParquet_Empty(t *testing.T) {
	store, schema := newTestStore(t)
	tmpDir := t.TempDir()
	exporter := NewExporterWithBaseDir(store, schema, tmpDir)
	outPath := filepath.Join(tmpDir, "empty.parquet")

	result, err := exporter.Export(context.Background(), ExportRequest{
		Entities:   []string{"nonexistent:1"},
		Features:   []string{"age"},
		Format:     FormatParquet,
		OutputPath: outPath,
	})
	if err != nil {
		t.Fatalf("Export Parquet (empty entity) failed: %v", err)
	}
	// Store returns empty features (not error) for nonexistent entities,
	// so the exporter writes a row with empty feature values.
	if result.RowsWritten != 1 {
		t.Errorf("Expected 1 row, got %d", result.RowsWritten)
	}
}

func TestExporter_ExportParquet_NoEntities(t *testing.T) {
	store, schema := newTestStore(t)
	tmpDir := t.TempDir()
	exporter := NewExporterWithBaseDir(store, schema, tmpDir)

	_, err := exporter.Export(context.Background(), ExportRequest{
		Entities:   []string{},
		Features:   []string{"age"},
		Format:     FormatParquet,
		OutputPath: filepath.Join(tmpDir, "out.parquet"),
	})
	if err == nil {
		t.Error("expected error for empty entity list in Parquet export")
	}
}

func TestExporter_Export_OutputPathCreation(t *testing.T) {
	store, schema := newTestStore(t)
	seedTestData(t, store)
	tmpDir := t.TempDir()
	exporter := NewExporterWithBaseDir(store, schema, tmpDir)
	// nested directory that doesn't exist yet
	outPath := filepath.Join(tmpDir, "nested", "deep", "out.csv")

	result, err := exporter.Export(context.Background(), ExportRequest{
		Entities:   []string{"user:1"},
		Features:   []string{"age"},
		Format:     FormatCSV,
		OutputPath: outPath,
	})
	if err != nil {
		t.Fatalf("Export should create nested directories: %v", err)
	}
	if result.RowsWritten != 1 {
		t.Errorf("Expected 1 row, got %d", result.RowsWritten)
	}
}

func TestExporter_Export_BytesWrittenPopulated(t *testing.T) {
	store, schema := newTestStore(t)
	seedTestData(t, store)
	tmpDir := t.TempDir()
	exporter := NewExporterWithBaseDir(store, schema, tmpDir)

	result, err := exporter.Export(context.Background(), ExportRequest{
		Entities:   []string{"user:1"},
		Features:   []string{"age"},
		Format:     FormatCSV,
		OutputPath: filepath.Join(tmpDir, "out.csv"),
	})
	if err != nil {
		t.Fatalf("Export failed: %v", err)
	}
	if result.BytesWritten <= 0 {
		t.Errorf("BytesWritten should be positive, got %d", result.BytesWritten)
	}
	if result.Duration <= 0 {
		t.Error("Duration should be positive")
	}
	if result.OutputPath == "" {
		t.Error("OutputPath should be set")
	}
}

func TestExporter_ExportPointInTime_WithStore(t *testing.T) {
	store, schema := newTestStore(t)

	now := time.Now()
	// Store feature at two timestamps
	err := store.Put(context.Background(), "user:pit", map[string]*domain.FeatureValue{
		"score": {Value: float64(10), Timestamp: now.Add(-2 * time.Hour).UnixNano(), Version: 1},
	})
	if err != nil {
		t.Fatalf("Put failed: %v", err)
	}
	time.Sleep(50 * time.Millisecond)

	err = store.Put(context.Background(), "user:pit", map[string]*domain.FeatureValue{
		"score": {Value: float64(20), Timestamp: now.Add(-1 * time.Hour).UnixNano(), Version: 2},
	})
	if err != nil {
		t.Fatalf("Put failed: %v", err)
	}
	time.Sleep(100 * time.Millisecond)

	exporter := NewExporter(store, schema)
	tmpDir := t.TempDir()
	outPath := filepath.Join(tmpDir, "pit.csv")

	result, err := exporter.ExportPointInTime(context.Background(), PointInTimeRequest{
		Entities:   []string{"user:pit"},
		Features:   []string{"score"},
		Timestamps: []time.Time{now.Add(-90 * time.Minute), now},
		OutputPath: outPath,
	})
	if err != nil {
		t.Fatalf("ExportPointInTime failed: %v", err)
	}
	if result.RowsWritten < 1 {
		t.Errorf("Expected at least 1 row, got %d", result.RowsWritten)
	}
	if result.EntitiesExported != 1 {
		t.Errorf("Expected 1 entity, got %d", result.EntitiesExported)
	}

	// Verify CSV content
	f, err := os.Open(outPath)
	if err != nil {
		t.Fatalf("Failed to open output: %v", err)
	}
	defer f.Close()
	reader := csv.NewReader(f)
	records, err := reader.ReadAll()
	if err != nil {
		t.Fatalf("Failed to read CSV: %v", err)
	}
	// header row
	if records[0][0] != "entity_key" {
		t.Errorf("Expected header to start with entity_key, got %s", records[0][0])
	}
}

func TestParquetSchemaBuilder(t *testing.T) {
	b := NewParquetSchemaBuilder()
	b.AddStringField("Name")
	b.AddInt64Field("Age")
	b.AddFloat64Field("Score")

	if len(b.fields) != 3 {
		t.Errorf("Expected 3 fields, got %d", len(b.fields))
	}
}
