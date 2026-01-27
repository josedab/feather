package incrmat

import (
	"testing"
	"time"
)

func TestNewCDCManager(t *testing.T) {
	engine := NewEngine(DefaultEngineConfig())
	mgr := NewCDCManager(engine, 1000)
	if mgr == nil {
		t.Fatal("expected non-nil CDC manager")
	}
}

func TestRegisterSource(t *testing.T) {
	engine := NewEngine(DefaultEngineConfig())
	mgr := NewCDCManager(engine, 1000)

	err := mgr.RegisterSource(CDCSourceConfig{
		ID: "pg1", Name: "Users DB", Type: CDCPostgreSQL,
		Database: "users", Table: "users", FeatureGroup: "user_features",
		Enabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	sources := mgr.ListSources()
	if len(sources) != 1 {
		t.Fatalf("expected 1 source, got %d", len(sources))
	}

	stats := mgr.Stats()
	if stats.TotalSources != 1 || stats.ActiveSources != 1 {
		t.Errorf("expected 1 total/1 active, got %d/%d", stats.TotalSources, stats.ActiveSources)
	}
}

func TestDuplicateSource(t *testing.T) {
	engine := NewEngine(DefaultEngineConfig())
	mgr := NewCDCManager(engine, 1000)

	_ = mgr.RegisterSource(CDCSourceConfig{
		ID: "s1", Name: "S1", FeatureGroup: "fg1",
	})
	err := mgr.RegisterSource(CDCSourceConfig{
		ID: "s1", Name: "S1 dup", FeatureGroup: "fg1",
	})
	if err == nil {
		t.Fatal("expected error for duplicate source")
	}
}

func TestProcessCDCEvent(t *testing.T) {
	engine := NewEngine(DefaultEngineConfig())
	_ = engine.RegisterNode(MaterializationNode{
		ID: "n1", FeatureGroup: "user_features",
	})

	mgr := NewCDCManager(engine, 1000)
	_ = mgr.RegisterSource(CDCSourceConfig{
		ID: "pg1", Name: "Users DB", Type: CDCPostgreSQL,
		FeatureGroup: "user_features", Enabled: true,
	})

	err := mgr.ProcessCDCEvent(CDCEvent{
		SourceID:  "pg1",
		Operation: OpUpdate,
		Table:     "users",
		EntityID:  "user1",
		After:     map[string]interface{}{"age": 30, "name": "Alice"},
		Timestamp: time.Now(),
		Version:   1,
	})
	if err != nil {
		t.Fatal(err)
	}

	stats := mgr.Stats()
	if stats.TotalCaptured != 1 {
		t.Errorf("expected 1 captured, got %d", stats.TotalCaptured)
	}
	if stats.TotalProcessed != 1 {
		t.Errorf("expected 1 processed, got %d", stats.TotalProcessed)
	}

	// Verify engine got the change
	dirty := engine.GetDirtyNodes()
	if len(dirty) != 1 {
		t.Errorf("expected 1 dirty node, got %d", len(dirty))
	}
}

func TestProcessBatch(t *testing.T) {
	engine := NewEngine(DefaultEngineConfig())
	mgr := NewCDCManager(engine, 1000)
	_ = mgr.RegisterSource(CDCSourceConfig{
		ID: "s1", Name: "S1", FeatureGroup: "fg1", Enabled: true,
	})

	events := []CDCEvent{
		{SourceID: "s1", Operation: OpInsert, EntityID: "e1", After: map[string]interface{}{"a": 1}, Timestamp: time.Now()},
		{SourceID: "s1", Operation: OpUpdate, EntityID: "e2", After: map[string]interface{}{"b": 2}, Timestamp: time.Now()},
		{SourceID: "invalid", Operation: OpDelete, EntityID: "e3", Timestamp: time.Now()},
	}

	processed, errCount, _ := mgr.ProcessBatch(events)
	if processed != 2 {
		t.Errorf("expected 2 processed, got %d", processed)
	}
	if errCount != 1 {
		t.Errorf("expected 1 error, got %d", errCount)
	}
}

func TestFieldMapping(t *testing.T) {
	engine := NewEngine(DefaultEngineConfig())
	_ = engine.RegisterNode(MaterializationNode{
		ID: "n1", FeatureGroup: "user_features",
	})

	mgr := NewCDCManager(engine, 1000)
	_ = mgr.RegisterSource(CDCSourceConfig{
		ID: "pg1", Name: "Users DB", Type: CDCPostgreSQL,
		FeatureGroup: "user_features", Enabled: true,
		FieldMapping: map[string]string{"user_age": "age", "user_name": "name"},
	})

	err := mgr.ProcessCDCEvent(CDCEvent{
		SourceID:  "pg1",
		Operation: OpUpdate,
		EntityID:  "user1",
		After:     map[string]interface{}{"user_age": 30},
		Timestamp: time.Now(),
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestGetRecentEvents(t *testing.T) {
	engine := NewEngine(DefaultEngineConfig())
	mgr := NewCDCManager(engine, 1000)
	_ = mgr.RegisterSource(CDCSourceConfig{
		ID: "s1", Name: "S1", FeatureGroup: "fg1", Enabled: true,
	})

	for i := 0; i < 5; i++ {
		_ = mgr.ProcessCDCEvent(CDCEvent{
			SourceID: "s1", Operation: OpInsert, EntityID: "e1",
			After: map[string]interface{}{"v": i}, Timestamp: time.Now(),
		})
	}

	events := mgr.GetRecentEvents(3)
	if len(events) != 3 {
		t.Fatalf("expected 3 recent events, got %d", len(events))
	}
}

func TestRemoveSource(t *testing.T) {
	engine := NewEngine(DefaultEngineConfig())
	mgr := NewCDCManager(engine, 1000)
	_ = mgr.RegisterSource(CDCSourceConfig{
		ID: "s1", Name: "S1", FeatureGroup: "fg1", Enabled: true,
	})

	if err := mgr.RemoveSource("s1"); err != nil {
		t.Fatal(err)
	}
	if err := mgr.RemoveSource("s1"); err == nil {
		t.Fatal("expected error removing nonexistent source")
	}
}

func TestSourceStatus(t *testing.T) {
	engine := NewEngine(DefaultEngineConfig())
	mgr := NewCDCManager(engine, 1000)
	_ = mgr.RegisterSource(CDCSourceConfig{
		ID: "s1", Name: "S1", FeatureGroup: "fg1", Enabled: true,
	})

	status, err := mgr.GetSourceStatus("s1")
	if err != nil {
		t.Fatal(err)
	}
	if !status.Connected {
		t.Error("expected connected status")
	}

	_, err = mgr.GetSourceStatus("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent source")
	}
}
