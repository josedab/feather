package offlinestore

import (
	"errors"
	"testing"
	"time"
)

func TestNewStore(t *testing.T) {
	s := NewStore(DefaultStoreConfig())
	if s == nil {
		t.Fatal("NewStore returned nil")
	}

	if len(s.datasets) != 0 {
		t.Errorf("expected empty datasets, got %d", len(s.datasets))
	}

	// Zero config should use defaults
	s2 := NewStore(StoreConfig{})
	if s2.config.MaxDatasets != 1000 {
		t.Errorf("expected default MaxDatasets 1000, got %d", s2.config.MaxDatasets)
	}
}

func TestCreateAndListDatasets(t *testing.T) {
	s := NewStore(DefaultStoreConfig())

	cfg := DatasetConfig{
		Name:         "training_v1",
		FeatureGroup: "user_features",
		EntityType:   "user",
		StartTime:    time.Now().Add(-24 * time.Hour),
		EndTime:      time.Now(),
	}

	info, err := s.CreateDataset(cfg)
	if err != nil {
		t.Fatalf("CreateDataset failed: %v", err)
	}

	if info.Config.Name != "training_v1" {
		t.Errorf("expected name training_v1, got %s", info.Config.Name)
	}

	if info.Status != "pending" {
		t.Errorf("expected status pending, got %s", info.Status)
	}

	// List datasets
	datasets := s.ListDatasets()
	if len(datasets) != 1 {
		t.Fatalf("expected 1 dataset, got %d", len(datasets))
	}

	// Get dataset
	got, err := s.GetDataset("training_v1")
	if err != nil {
		t.Fatalf("GetDataset failed: %v", err)
	}
	if got.Config.FeatureGroup != "user_features" {
		t.Errorf("expected feature group user_features, got %s", got.Config.FeatureGroup)
	}

	// Get non-existent
	_, err = s.GetDataset("nonexistent")
	if !errors.Is(err, ErrDatasetNotFound) {
		t.Errorf("expected ErrDatasetNotFound, got %v", err)
	}
}

func TestAppendAndGetRows(t *testing.T) {
	s := NewStore(DefaultStoreConfig())

	cfg := DatasetConfig{
		Name:         "test_ds",
		FeatureGroup: "features",
		EntityType:   "user",
	}
	_, _ = s.CreateDataset(cfg)

	rows := []FeatureRow{
		{EntityID: "user1", Features: map[string]interface{}{"age": 25}, Timestamp: time.Now()},
		{EntityID: "user2", Features: map[string]interface{}{"age": 30}, Timestamp: time.Now()},
		{EntityID: "user3", Features: map[string]interface{}{"age": 35}, Timestamp: time.Now()},
	}

	if err := s.AppendRows("test_ds", rows); err != nil {
		t.Fatalf("AppendRows failed: %v", err)
	}

	// Get all rows
	got, err := s.GetRows("test_ds", 100, 0)
	if err != nil {
		t.Fatalf("GetRows failed: %v", err)
	}

	if len(got) != 3 {
		t.Errorf("expected 3 rows, got %d", len(got))
	}

	// Pagination
	got, err = s.GetRows("test_ds", 2, 0)
	if err != nil {
		t.Fatalf("GetRows with limit failed: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("expected 2 rows, got %d", len(got))
	}

	// Offset beyond data
	got, err = s.GetRows("test_ds", 10, 100)
	if err != nil {
		t.Fatalf("GetRows with large offset failed: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected 0 rows, got %d", len(got))
	}

	// Non-existent dataset
	_, err = s.GetRows("nonexistent", 10, 0)
	if !errors.Is(err, ErrDatasetNotFound) {
		t.Errorf("expected ErrDatasetNotFound, got %v", err)
	}

	// Append to non-existent dataset
	if err := s.AppendRows("nonexistent", rows); !errors.Is(err, ErrDatasetNotFound) {
		t.Errorf("expected ErrDatasetNotFound, got %v", err)
	}
}

func TestPointInTimeQuery(t *testing.T) {
	s := NewStore(DefaultStoreConfig())

	cfg := DatasetConfig{
		Name:         "pit_ds",
		FeatureGroup: "features",
		EntityType:   "user",
	}
	_, _ = s.CreateDataset(cfg)

	now := time.Now()
	t1 := now.Add(-3 * time.Hour)
	t2 := now.Add(-2 * time.Hour)
	t3 := now.Add(-1 * time.Hour)

	rows := []FeatureRow{
		{EntityID: "user1", Features: map[string]interface{}{"score": 0.5}, Timestamp: t1},
		{EntityID: "user1", Features: map[string]interface{}{"score": 0.7}, Timestamp: t2},
		{EntityID: "user1", Features: map[string]interface{}{"score": 0.9}, Timestamp: t3},
		{EntityID: "user2", Features: map[string]interface{}{"score": 0.3}, Timestamp: t2},
	}
	_ = s.AppendRows("pit_ds", rows)

	// Query as of t2 for user1 — should return t1 and t2 rows
	results, err := s.GetPointInTime("pit_ds", "user1", t2)
	if err != nil {
		t.Fatalf("GetPointInTime failed: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 rows as of t2, got %d", len(results))
	}

	// Results should be sorted by timestamp descending
	if results[0].Timestamp.Before(results[1].Timestamp) {
		t.Error("expected results sorted by timestamp descending")
	}

	// Most recent should have score 0.7
	if results[0].Features["score"] != 0.7 {
		t.Errorf("expected score 0.7 for most recent, got %v", results[0].Features["score"])
	}

	// Query for non-existent entity
	results, err = s.GetPointInTime("pit_ds", "nonexistent", now)
	if err != nil {
		t.Fatalf("GetPointInTime failed: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 rows for nonexistent entity, got %d", len(results))
	}

	// Query non-existent dataset
	_, err = s.GetPointInTime("nonexistent", "user1", now)
	if !errors.Is(err, ErrDatasetNotFound) {
		t.Errorf("expected ErrDatasetNotFound, got %v", err)
	}
}

func TestExportDataset(t *testing.T) {
	s := NewStore(DefaultStoreConfig())

	cfg := DatasetConfig{
		Name:         "export_ds",
		FeatureGroup: "features",
		EntityType:   "user",
	}
	_, _ = s.CreateDataset(cfg)

	rows := make([]FeatureRow, 100)
	for i := range rows {
		rows[i] = FeatureRow{
			EntityID:  "user1",
			Features:  map[string]interface{}{"val": i},
			Timestamp: time.Now(),
		}
	}
	_ = s.AppendRows("export_ds", rows)

	result, err := s.ExportDataset("export_ds", ExportConfig{
		Format:      "parquet",
		Compression: "snappy",
		MaxRows:     50,
	})
	if err != nil {
		t.Fatalf("ExportDataset failed: %v", err)
	}

	if result.Dataset != "export_ds" {
		t.Errorf("expected dataset export_ds, got %s", result.Dataset)
	}

	if result.Format != "parquet" {
		t.Errorf("expected format parquet, got %s", result.Format)
	}

	if result.RowCount != 50 {
		t.Errorf("expected 50 rows (capped by MaxRows), got %d", result.RowCount)
	}

	if result.SizeEstimate <= 0 {
		t.Errorf("expected positive size estimate, got %d", result.SizeEstimate)
	}

	// Non-existent dataset
	_, err = s.ExportDataset("nonexistent", DefaultExportConfig())
	if !errors.Is(err, ErrDatasetNotFound) {
		t.Errorf("expected ErrDatasetNotFound, got %v", err)
	}
}

func TestDeleteDataset(t *testing.T) {
	s := NewStore(DefaultStoreConfig())

	cfg := DatasetConfig{
		Name:         "del_ds",
		FeatureGroup: "features",
		EntityType:   "user",
	}
	_, _ = s.CreateDataset(cfg)

	if err := s.DeleteDataset("del_ds"); err != nil {
		t.Fatalf("DeleteDataset failed: %v", err)
	}

	_, err := s.GetDataset("del_ds")
	if !errors.Is(err, ErrDatasetNotFound) {
		t.Errorf("expected ErrDatasetNotFound after deletion, got %v", err)
	}

	// Delete non-existent
	if err := s.DeleteDataset("nonexistent"); !errors.Is(err, ErrDatasetNotFound) {
		t.Errorf("expected ErrDatasetNotFound, got %v", err)
	}
}

func TestDuplicateDataset(t *testing.T) {
	s := NewStore(DefaultStoreConfig())

	cfg := DatasetConfig{
		Name:         "dup_ds",
		FeatureGroup: "features",
		EntityType:   "user",
	}

	_, err := s.CreateDataset(cfg)
	if err != nil {
		t.Fatalf("first CreateDataset failed: %v", err)
	}

	_, err = s.CreateDataset(cfg)
	if !errors.Is(err, ErrDatasetExists) {
		t.Errorf("expected ErrDatasetExists, got %v", err)
	}
}

func TestStats(t *testing.T) {
	s := NewStore(DefaultStoreConfig())

	cfg1 := DatasetConfig{Name: "ds1", FeatureGroup: "f1", EntityType: "user"}
	cfg2 := DatasetConfig{Name: "ds2", FeatureGroup: "f2", EntityType: "user"}
	_, _ = s.CreateDataset(cfg1)
	_, _ = s.CreateDataset(cfg2)

	rows := []FeatureRow{
		{EntityID: "u1", Features: map[string]interface{}{"a": 1}, Timestamp: time.Now()},
		{EntityID: "u2", Features: map[string]interface{}{"a": 2}, Timestamp: time.Now()},
	}
	_ = s.AppendRows("ds1", rows)

	stats := s.Stats()

	if stats.TotalDatasets != 2 {
		t.Errorf("expected 2 datasets, got %d", stats.TotalDatasets)
	}

	if stats.TotalRows != 2 {
		t.Errorf("expected 2 rows, got %d", stats.TotalRows)
	}

	if stats.TotalSizeBytes <= 0 {
		t.Errorf("expected positive total size, got %d", stats.TotalSizeBytes)
	}
}
