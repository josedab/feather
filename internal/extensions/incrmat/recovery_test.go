package incrmat

import (
	"testing"
	"time"
)

func TestRecoveryManager_Checkpoint(t *testing.T) {
	engine := NewEngine(DefaultEngineConfig())
	cdcMgr := NewCDCManager(engine, 1000)

	// Register a source
	err := cdcMgr.RegisterSource(CDCSourceConfig{
		ID:           "pg-source",
		Name:         "postgres-users",
		Type:         CDCPostgreSQL,
		FeatureGroup: "users",
		Enabled:      true,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Process some events
	_ = cdcMgr.ProcessCDCEvent(CDCEvent{
		SourceID:  "pg-source",
		Operation: OpInsert,
		EntityID:  "user:1",
		After:     map[string]interface{}{"name": "Alice"},
		Timestamp: time.Now(),
		LSN:       100,
	})

	// Create recovery manager and checkpoint
	rm := NewRecoveryManager(cdcMgr, DefaultRecoveryConfig())
	cp, err := rm.Checkpoint("pg-source")
	if err != nil {
		t.Fatal(err)
	}
	if cp.EventCount != 1 {
		t.Errorf("expected 1 event, got %d", cp.EventCount)
	}
}

func TestRecoveryManager_RecoverFrom(t *testing.T) {
	engine := NewEngine(DefaultEngineConfig())
	cdcMgr := NewCDCManager(engine, 1000)

	err := cdcMgr.RegisterSource(CDCSourceConfig{
		ID:           "mysql-source",
		Name:         "mysql-orders",
		Type:         CDCMySQL,
		FeatureGroup: "orders",
		Enabled:      true,
	})
	if err != nil {
		t.Fatal(err)
	}

	rm := NewRecoveryManager(cdcMgr, DefaultRecoveryConfig())

	// Manually set a checkpoint
	rm.mu.Lock()
	rm.checkpoints["mysql-source"] = &OffsetCheckpoint{
		SourceID:     "mysql-source",
		LSN:          500,
		EventCount:   100,
		CheckpointAt: time.Now(),
	}
	rm.mu.Unlock()

	// Recover
	cp, err := rm.RecoverFrom("mysql-source")
	if err != nil {
		t.Fatal(err)
	}
	if cp.LSN != 500 {
		t.Errorf("expected LSN 500, got %d", cp.LSN)
	}

	// Verify LSN tracker was updated
	positions := cdcMgr.GetLSNPositions()
	if positions["mysql-source"] != 500 {
		t.Errorf("expected LSN tracker at 500, got %d", positions["mysql-source"])
	}
}

func TestRecoveryManager_ExportImport(t *testing.T) {
	engine := NewEngine(DefaultEngineConfig())
	cdcMgr := NewCDCManager(engine, 1000)
	rm := NewRecoveryManager(cdcMgr, DefaultRecoveryConfig())

	rm.mu.Lock()
	rm.checkpoints["source-1"] = &OffsetCheckpoint{SourceID: "source-1", LSN: 42}
	rm.mu.Unlock()

	data, err := rm.ExportCheckpoints()
	if err != nil {
		t.Fatal(err)
	}

	// Import into a fresh manager
	rm2 := NewRecoveryManager(cdcMgr, DefaultRecoveryConfig())
	if err := rm2.ImportCheckpoints(data); err != nil {
		t.Fatal(err)
	}

	cp, exists := rm2.GetLatestCheckpoint("source-1")
	if !exists {
		t.Fatal("expected checkpoint after import")
	}
	if cp.LSN != 42 {
		t.Errorf("expected LSN 42, got %d", cp.LSN)
	}
}

func TestRecoveryManager_RecoverFromMissing(t *testing.T) {
	engine := NewEngine(DefaultEngineConfig())
	cdcMgr := NewCDCManager(engine, 1000)
	rm := NewRecoveryManager(cdcMgr, DefaultRecoveryConfig())

	_, err := rm.RecoverFrom("nonexistent-source")
	if err == nil {
		t.Fatal("expected error for missing checkpoint")
	}
}

func TestRecoveryManager_CheckpointHistory(t *testing.T) {
	engine := NewEngine(DefaultEngineConfig())
	cdcMgr := NewCDCManager(engine, 1000)

	err := cdcMgr.RegisterSource(CDCSourceConfig{
		ID:           "hist-source",
		Name:         "history-test",
		Type:         CDCPostgreSQL,
		FeatureGroup: "users",
		Enabled:      true,
	})
	if err != nil {
		t.Fatal(err)
	}

	rm := NewRecoveryManager(cdcMgr, RecoveryConfig{MaxHistory: 3})

	// Create multiple checkpoints
	for i := 0; i < 5; i++ {
		_, err := rm.Checkpoint("hist-source")
		if err != nil {
			t.Fatal(err)
		}
	}

	// History should be capped at MaxHistory
	history := rm.GetCheckpointHistory("hist-source")
	if len(history) != 3 {
		t.Errorf("expected 3 history entries (MaxHistory), got %d", len(history))
	}
}

func TestRecoveryManager_ListAllCheckpoints(t *testing.T) {
	engine := NewEngine(DefaultEngineConfig())
	cdcMgr := NewCDCManager(engine, 1000)
	rm := NewRecoveryManager(cdcMgr, DefaultRecoveryConfig())

	rm.mu.Lock()
	rm.checkpoints["src-a"] = &OffsetCheckpoint{SourceID: "src-a", LSN: 10}
	rm.checkpoints["src-b"] = &OffsetCheckpoint{SourceID: "src-b", LSN: 20}
	rm.mu.Unlock()

	all := rm.ListAllCheckpoints()
	if len(all) != 2 {
		t.Errorf("expected 2 checkpoints, got %d", len(all))
	}
	if all["src-a"].LSN != 10 {
		t.Errorf("expected LSN 10, got %d", all["src-a"].LSN)
	}
}

func TestRecoveryManager_ImportInvalidJSON(t *testing.T) {
	engine := NewEngine(DefaultEngineConfig())
	cdcMgr := NewCDCManager(engine, 1000)
	rm := NewRecoveryManager(cdcMgr, DefaultRecoveryConfig())

	err := rm.ImportCheckpoints([]byte("not valid json"))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestRecoveryManager_EmptyHistory(t *testing.T) {
	engine := NewEngine(DefaultEngineConfig())
	cdcMgr := NewCDCManager(engine, 1000)
	rm := NewRecoveryManager(cdcMgr, DefaultRecoveryConfig())

	history := rm.GetCheckpointHistory("nonexistent")
	if history != nil {
		t.Errorf("expected nil history for unknown source, got %v", history)
	}
}
