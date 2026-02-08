package export

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
	exporter := NewExporter(nil, nil)
	tmpDir := t.TempDir()

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
