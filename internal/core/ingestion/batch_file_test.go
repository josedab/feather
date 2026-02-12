package ingestion

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestBatchImporter_ImportCSV_FileNotFound(t *testing.T) {
	b := newTestBatchImporter(t)

	config := ImportConfig{
		EntityKeyColumn: "entity_id",
		HasHeader:       true,
	}

	_, err := b.ImportCSV(context.Background(), "/nonexistent/path/file.csv", config)
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestBatchImporter_ImportCSV_ValidFile(t *testing.T) {
	b := newTestBatchImporter(t)

	dir := t.TempDir()
	path := filepath.Join(dir, "test.csv")
	content := "entity_id,score\nuser:1,0.95\nuser:2,0.85\n"
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	config := ImportConfig{
		EntityKeyColumn: "entity_id",
		HasHeader:       true,
	}

	result, err := b.ImportCSV(context.Background(), path, config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.RowsSuccess != 2 {
		t.Errorf("RowsSuccess = %d, want 2", result.RowsSuccess)
	}
}

func TestBatchImporter_ImportJSON_FileNotFound(t *testing.T) {
	b := newTestBatchImporter(t)

	config := ImportConfig{
		EntityKeyColumn: "entity_id",
	}

	_, err := b.ImportJSON(context.Background(), "/nonexistent/path/file.json", config)
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestBatchImporter_ImportJSON_ValidFile(t *testing.T) {
	b := newTestBatchImporter(t)

	dir := t.TempDir()
	path := filepath.Join(dir, "test.json")
	content := `[{"entity_id":"user:1","score":0.95},{"entity_id":"user:2","score":0.85}]`
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	config := ImportConfig{
		EntityKeyColumn: "entity_id",
	}

	result, err := b.ImportJSON(context.Background(), path, config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.RowsSuccess != 2 {
		t.Errorf("RowsSuccess = %d, want 2", result.RowsSuccess)
	}
}

func TestBatchImporter_ImportJSONL_FileNotFound(t *testing.T) {
	b := newTestBatchImporter(t)

	config := ImportConfig{
		EntityKeyColumn: "entity_id",
	}

	_, err := b.ImportJSONL(context.Background(), "/nonexistent/path/file.jsonl", config)
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestBatchImporter_ImportJSONL_ValidFile(t *testing.T) {
	b := newTestBatchImporter(t)

	dir := t.TempDir()
	path := filepath.Join(dir, "test.jsonl")
	content := "{\"entity_id\":\"user:1\",\"score\":0.95}\n{\"entity_id\":\"user:2\",\"score\":0.85}\n"
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	config := ImportConfig{
		EntityKeyColumn: "entity_id",
	}

	result, err := b.ImportJSONL(context.Background(), path, config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.RowsSuccess != 2 {
		t.Errorf("RowsSuccess = %d, want 2", result.RowsSuccess)
	}
}

func TestBatchImporter_ImportCSV_PermissionError(t *testing.T) {
	b := newTestBatchImporter(t)

	dir := t.TempDir()
	path := filepath.Join(dir, "noperm.csv")
	if err := os.WriteFile(path, []byte("entity_id,score\nuser:1,0.5\n"), 0600); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// Remove read permission
	if err := os.Chmod(path, 0000); err != nil {
		t.Fatalf("failed to chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0600) })

	config := ImportConfig{
		EntityKeyColumn: "entity_id",
		HasHeader:       true,
	}

	_, err := b.ImportCSV(context.Background(), path, config)
	if err == nil {
		t.Error("expected error for permission denied")
	}
}

func TestBatchImporter_ImportJSON_CorruptFile(t *testing.T) {
	b := newTestBatchImporter(t)

	dir := t.TempDir()
	path := filepath.Join(dir, "corrupt.json")
	if err := os.WriteFile(path, []byte("not valid json at all"), 0600); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	config := ImportConfig{
		EntityKeyColumn: "entity_id",
	}

	_, err := b.ImportJSON(context.Background(), path, config)
	if err == nil {
		t.Error("expected error for corrupt JSON file")
	}
}

func TestBatchImporter_ImportCSV_EmptyFile(t *testing.T) {
	b := newTestBatchImporter(t)

	dir := t.TempDir()
	path := filepath.Join(dir, "empty.csv")
	if err := os.WriteFile(path, []byte(""), 0600); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	config := ImportConfig{
		EntityKeyColumn: "entity_id",
		HasHeader:       true,
	}

	// Empty file with HasHeader=true should fail reading header
	_, err := b.ImportCSV(context.Background(), path, config)
	if err == nil {
		t.Error("expected error for empty CSV with HasHeader")
	}
}
