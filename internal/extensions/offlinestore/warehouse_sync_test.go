package offlinestore

import (
	"context"
	"testing"
	"time"
)

func TestWarehouseSyncerExport(t *testing.T) {
	t.Parallel()
	store := NewStore(DefaultStoreConfig())
	_, _ = store.CreateDataset(DatasetConfig{
		Name:       "test_ds",
		EntityType: "user",
	})
	_ = store.AppendRows("test_ds", []FeatureRow{
		{EntityID: "user:1", Features: map[string]interface{}{"clicks": 10}, Timestamp: time.Now()},
		{EntityID: "user:2", Features: map[string]interface{}{"clicks": 20}, Timestamp: time.Now()},
	})

	syncer := NewWarehouseSyncer(DefaultWarehouseSyncConfig(), store)
	job, err := syncer.ExportToWarehouse(context.Background(), "test_ds")
	if err != nil {
		t.Fatal(err)
	}
	if job.Status != "completed" {
		t.Errorf("expected completed, got %s", job.Status)
	}
	if job.RowsSynced != 2 {
		t.Errorf("expected 2 rows synced, got %d", job.RowsSynced)
	}
}

func TestWarehouseSyncerImport(t *testing.T) {
	t.Parallel()
	store := NewStore(DefaultStoreConfig())
	_, _ = store.CreateDataset(DatasetConfig{
		Name:       "import_ds",
		EntityType: "user",
	})

	syncer := NewWarehouseSyncer(DefaultWarehouseSyncConfig(), store)
	rows := []FeatureRow{
		{EntityID: "user:1", Features: map[string]interface{}{"score": 0.9}, Timestamp: time.Now()},
	}
	job, err := syncer.ImportFromWarehouse(context.Background(), "import_ds", rows)
	if err != nil {
		t.Fatal(err)
	}
	if job.Status != "completed" {
		t.Errorf("expected completed, got %s", job.Status)
	}
	if job.Direction != "import" {
		t.Errorf("expected import, got %s", job.Direction)
	}
}

func TestWarehouseSyncerPointInTimeJoin(t *testing.T) {
	t.Parallel()
	store := NewStore(DefaultStoreConfig())
	_, _ = store.CreateDataset(DatasetConfig{
		Name:       "features",
		EntityType: "user",
	})
	_ = store.AppendRows("features", []FeatureRow{
		{EntityID: "user:1", Features: map[string]interface{}{"clicks": 5}, Timestamp: time.Now()},
	})

	syncer := NewWarehouseSyncer(DefaultWarehouseSyncConfig(), store)
	result, err := syncer.PointInTimeJoin(context.Background(), PointInTimeJoinRequest{
		FeatureRefs:     []string{"clicks"},
		EntityColumn:    "user_id",
		TimestampColumn: "event_timestamp",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.RowCount == 0 {
		t.Error("expected non-zero row count")
	}
	if result.JoinStats.DurationMs < 0 {
		t.Error("expected non-negative duration")
	}
}

func TestWarehouseSyncerListJobs(t *testing.T) {
	t.Parallel()
	store := NewStore(DefaultStoreConfig())
	_, _ = store.CreateDataset(DatasetConfig{Name: "ds", EntityType: "user"})

	syncer := NewWarehouseSyncer(DefaultWarehouseSyncConfig(), store)
	_, _ = syncer.ExportToWarehouse(context.Background(), "ds")

	jobs := syncer.ListJobs()
	if len(jobs) != 1 {
		t.Errorf("expected 1 job, got %d", len(jobs))
	}
}

func TestWarehouseSyncerStats(t *testing.T) {
	t.Parallel()
	store := NewStore(DefaultStoreConfig())
	syncer := NewWarehouseSyncer(DefaultWarehouseSyncConfig(), store)

	stats := syncer.Stats()
	if stats.TotalSyncs != 0 {
		t.Errorf("expected 0 syncs initially")
	}
}

func TestWarehouseSyncerPITJoinDedup(t *testing.T) {
t.Parallel()
store := NewStore(DefaultStoreConfig())
_, _ = store.CreateDataset(DatasetConfig{Name: "features", EntityType: "user"})
now := time.Now()
// Add two rows for same entity at different times.
_ = store.AppendRows("features", []FeatureRow{
{EntityID: "user:1", Features: map[string]interface{}{"clicks": 5}, Timestamp: now.Add(-time.Hour)},
{EntityID: "user:1", Features: map[string]interface{}{"clicks": 10}, Timestamp: now},
})

syncer := NewWarehouseSyncer(DefaultWarehouseSyncConfig(), store)
result, err := syncer.PointInTimeJoin(context.Background(), PointInTimeJoinRequest{
FeatureRefs: []string{"clicks"},
})
if err != nil {
t.Fatal(err)
}
// Should return only 1 row (latest per entity).
if result.RowCount != 1 {
t.Errorf("expected 1 deduplicated row, got %d", result.RowCount)
}
if result.Rows[0].Features["clicks"] != 10 {
t.Errorf("expected latest value 10, got %v", result.Rows[0].Features["clicks"])
}
}

func TestWarehouseSyncerPITJoinEmptyFeatureRefs(t *testing.T) {
t.Parallel()
store := NewStore(DefaultStoreConfig())
syncer := NewWarehouseSyncer(DefaultWarehouseSyncConfig(), store)
_, err := syncer.PointInTimeJoin(context.Background(), PointInTimeJoinRequest{})
if err == nil {
t.Error("expected error for empty feature_refs")
}
}

func TestWarehouseSyncerExportEmptyDataset(t *testing.T) {
t.Parallel()
store := NewStore(DefaultStoreConfig())
syncer := NewWarehouseSyncer(DefaultWarehouseSyncConfig(), store)
_, err := syncer.ExportToWarehouse(context.Background(), "")
if err == nil {
t.Error("expected error for empty dataset name")
}
}
