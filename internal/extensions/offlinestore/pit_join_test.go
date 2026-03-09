package offlinestore

import (
	"context"
	"testing"
	"time"
)

func newTestStoreWithData(t *testing.T, datasetName string, rows []FeatureRow) *Store {
	t.Helper()
	s := NewStore(DefaultStoreConfig())
	_, err := s.CreateDataset(DatasetConfig{
		Name:         datasetName,
		FeatureGroup: "features",
		EntityType:   "user",
	})
	if err != nil {
		t.Fatalf("CreateDataset: %v", err)
	}
	if err := s.AppendRows(datasetName, rows); err != nil {
		t.Fatalf("AppendRows: %v", err)
	}
	return s
}

func TestPITJoinBasic(t *testing.T) {
	now := time.Now()
	t1 := now.Add(-3 * time.Hour)
	t2 := now.Add(-2 * time.Hour)
	t3 := now.Add(-1 * time.Hour)

	rows := []FeatureRow{
		{EntityID: "u1", Features: map[string]interface{}{"score": 0.5, "age": 25}, Timestamp: t1},
		{EntityID: "u1", Features: map[string]interface{}{"score": 0.7, "age": 26}, Timestamp: t2},
		{EntityID: "u1", Features: map[string]interface{}{"score": 0.9, "age": 27}, Timestamp: t3},
		{EntityID: "u2", Features: map[string]interface{}{"score": 0.3, "age": 30}, Timestamp: t2},
	}

	store := newTestStoreWithData(t, "ds", rows)
	engine := NewPITJoinEngine(DefaultPITJoinConfig(), store)

	entityTS := []EntityTimestamp{
		{EntityID: "u1", Timestamp: t2},
		{EntityID: "u2", Timestamp: t3},
	}

	result, err := engine.Join(context.Background(), entityTS, []string{"score"}, "ds")
	if err != nil {
		t.Fatalf("Join: %v", err)
	}

	if result.RowCount != 2 {
		t.Fatalf("expected 2 rows, got %d", result.RowCount)
	}

	// u1 at t2 should get score=0.7 (latest at or before t2)
	for _, row := range result.Rows {
		if row.EntityID == "u1" {
			if row.Features["score"] != 0.7 {
				t.Errorf("u1: expected score 0.7, got %v", row.Features["score"])
			}
			// "age" not requested, should not appear
			if _, ok := row.Features["age"]; ok {
				t.Error("u1: age should not be in projected features")
			}
		}
		if row.EntityID == "u2" {
			if row.Features["score"] != 0.3 {
				t.Errorf("u2: expected score 0.3, got %v", row.Features["score"])
			}
		}
	}

	if result.JoinStats.RowsMatched != 2 {
		t.Errorf("expected 2 matched, got %d", result.JoinStats.RowsMatched)
	}
	if result.JoinStats.RowsUnmatched != 0 {
		t.Errorf("expected 0 unmatched, got %d", result.JoinStats.RowsUnmatched)
	}
}

func TestPITJoinUnmatchedEntity(t *testing.T) {
	now := time.Now()
	rows := []FeatureRow{
		{EntityID: "u1", Features: map[string]interface{}{"score": 0.5}, Timestamp: now},
	}

	store := newTestStoreWithData(t, "ds", rows)
	engine := NewPITJoinEngine(DefaultPITJoinConfig(), store)

	entityTS := []EntityTimestamp{
		{EntityID: "u1", Timestamp: now},
		{EntityID: "unknown", Timestamp: now},
	}

	result, err := engine.Join(context.Background(), entityTS, []string{"score"}, "ds")
	if err != nil {
		t.Fatalf("Join: %v", err)
	}

	if result.JoinStats.RowsMatched != 1 {
		t.Errorf("expected 1 matched, got %d", result.JoinStats.RowsMatched)
	}
	if result.JoinStats.RowsUnmatched != 1 {
		t.Errorf("expected 1 unmatched, got %d", result.JoinStats.RowsUnmatched)
	}
}

func TestPITJoinLookbackEnforcement(t *testing.T) {
	now := time.Now()
	oldRow := now.Add(-48 * time.Hour)

	rows := []FeatureRow{
		{EntityID: "u1", Features: map[string]interface{}{"score": 0.5}, Timestamp: oldRow},
	}

	store := newTestStoreWithData(t, "ds", rows)

	cfg := DefaultPITJoinConfig()
	cfg.MaxLookback = 24 * time.Hour
	engine := NewPITJoinEngine(cfg, store)

	entityTS := []EntityTimestamp{
		{EntityID: "u1", Timestamp: now},
	}

	result, err := engine.Join(context.Background(), entityTS, []string{"score"}, "ds")
	if err != nil {
		t.Fatalf("Join: %v", err)
	}

	if result.RowCount != 0 {
		t.Errorf("expected 0 rows (lookback exceeded), got %d", result.RowCount)
	}
	if result.JoinStats.RowsUnmatched != 1 {
		t.Errorf("expected 1 unmatched, got %d", result.JoinStats.RowsUnmatched)
	}
}

func TestPITJoinTTL(t *testing.T) {
	now := time.Now()
	rows := []FeatureRow{
		{EntityID: "u1", Features: map[string]interface{}{"score": 0.5}, Timestamp: now.Add(-2 * time.Hour)},
	}

	store := newTestStoreWithData(t, "ds", rows)

	cfg := DefaultPITJoinConfig()
	cfg.TTL = 1 * time.Hour
	engine := NewPITJoinEngine(cfg, store)

	entityTS := []EntityTimestamp{
		{EntityID: "u1", Timestamp: now},
	}

	result, err := engine.Join(context.Background(), entityTS, []string{"score"}, "ds")
	if err != nil {
		t.Fatalf("Join: %v", err)
	}

	if result.RowCount != 0 {
		t.Errorf("expected 0 rows (TTL exceeded), got %d", result.RowCount)
	}
}

func TestPITJoinEmptyInputs(t *testing.T) {
	store := NewStore(DefaultStoreConfig())
	engine := NewPITJoinEngine(DefaultPITJoinConfig(), store)

	_, err := engine.Join(context.Background(), nil, []string{"score"}, "ds")
	if err == nil {
		t.Fatal("expected error for empty entity_timestamps")
	}

	_, err = engine.Join(context.Background(), []EntityTimestamp{{EntityID: "u1", Timestamp: time.Now()}}, nil, "ds")
	if err == nil {
		t.Fatal("expected error for empty feature_names")
	}
}

func TestGenerateTrainingSet(t *testing.T) {
	now := time.Now()
	rows := []FeatureRow{
		{EntityID: "u1", Features: map[string]interface{}{"score": 0.5}, Timestamp: now.Add(-2 * time.Hour)},
		{EntityID: "u1", Features: map[string]interface{}{"score": 0.9}, Timestamp: now},
		{EntityID: "u2", Features: map[string]interface{}{"score": 0.3}, Timestamp: now.Add(-1 * time.Hour)},
	}

	store := newTestStoreWithData(t, "ds", rows)
	engine := NewPITJoinEngine(DefaultPITJoinConfig(), store)

	result, err := engine.GenerateTrainingSet(context.Background(), "ds", []string{"score"})
	if err != nil {
		t.Fatalf("GenerateTrainingSet: %v", err)
	}

	if result.RowCount != 2 {
		t.Fatalf("expected 2 rows (one per entity), got %d", result.RowCount)
	}

	for _, row := range result.Rows {
		if row.EntityID == "u1" {
			if row.Features["score"] != 0.9 {
				t.Errorf("u1: expected latest score 0.9, got %v", row.Features["score"])
			}
		}
		if row.EntityID == "u2" {
			if row.Features["score"] != 0.3 {
				t.Errorf("u2: expected score 0.3, got %v", row.Features["score"])
			}
		}
	}
}

func TestPITJoinCancellation(t *testing.T) {
	now := time.Now()
	rows := []FeatureRow{
		{EntityID: "u1", Features: map[string]interface{}{"score": 0.5}, Timestamp: now},
	}

	store := newTestStoreWithData(t, "ds", rows)
	engine := NewPITJoinEngine(DefaultPITJoinConfig(), store)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := engine.Join(ctx, []EntityTimestamp{{EntityID: "u1", Timestamp: now}}, []string{"score"}, "ds")
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}
