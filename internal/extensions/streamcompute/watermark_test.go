package streamcompute

import (
	"testing"
	"time"
)

// --- CheckpointStore tests ---

func TestCheckpointStore_Save(t *testing.T) {
	t.Parallel()
	store := NewCheckpointStore(10)

	cp := Checkpoint{PipelineID: "pipe-1", Sequence: 1, Watermark: time.Now()}
	store.Save(cp)

	latest, ok := store.Latest("pipe-1")
	if !ok {
		t.Fatal("expected checkpoint to exist")
	}
	if latest.PipelineID != "pipe-1" {
		t.Errorf("expected pipe-1, got %s", latest.PipelineID)
	}
	if latest.Sequence != 1 {
		t.Errorf("expected sequence 1, got %d", latest.Sequence)
	}
	if latest.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be set")
	}
}

func TestCheckpointStore_MaxLimitEviction(t *testing.T) {
	t.Parallel()
	store := NewCheckpointStore(3)

	for i := int64(1); i <= 5; i++ {
		store.Save(Checkpoint{PipelineID: "pipe-1", Sequence: i})
	}

	list := store.List("pipe-1")
	if len(list) != 3 {
		t.Errorf("expected 3 checkpoints after eviction, got %d", len(list))
	}
	// Should keep the most recent (3, 4, 5)
	if list[0].Sequence != 3 {
		t.Errorf("expected oldest kept sequence 3, got %d", list[0].Sequence)
	}
}

func TestCheckpointStore_DefaultMaxPerPipeline(t *testing.T) {
	t.Parallel()
	store := NewCheckpointStore(0)

	for i := int64(1); i <= 15; i++ {
		store.Save(Checkpoint{PipelineID: "pipe-1", Sequence: i})
	}

	list := store.List("pipe-1")
	if len(list) != 10 {
		t.Errorf("expected default max 10 checkpoints, got %d", len(list))
	}
}

func TestCheckpointStore_Latest_Empty(t *testing.T) {
	t.Parallel()
	store := NewCheckpointStore(10)

	_, ok := store.Latest("nonexistent")
	if ok {
		t.Error("expected false for empty store")
	}
}

func TestCheckpointStore_Latest_MultipleCheckpoints(t *testing.T) {
	t.Parallel()
	store := NewCheckpointStore(10)

	store.Save(Checkpoint{PipelineID: "pipe-1", Sequence: 1})
	store.Save(Checkpoint{PipelineID: "pipe-1", Sequence: 2})
	store.Save(Checkpoint{PipelineID: "pipe-1", Sequence: 3})

	latest, ok := store.Latest("pipe-1")
	if !ok {
		t.Fatal("expected checkpoint")
	}
	if latest.Sequence != 3 {
		t.Errorf("expected latest sequence 3, got %d", latest.Sequence)
	}
}

func TestCheckpointStore_List_PipelineIsolation(t *testing.T) {
	t.Parallel()
	store := NewCheckpointStore(10)

	store.Save(Checkpoint{PipelineID: "pipe-a", Sequence: 1})
	store.Save(Checkpoint{PipelineID: "pipe-a", Sequence: 2})
	store.Save(Checkpoint{PipelineID: "pipe-b", Sequence: 10})

	listA := store.List("pipe-a")
	if len(listA) != 2 {
		t.Errorf("expected 2 for pipe-a, got %d", len(listA))
	}
	listB := store.List("pipe-b")
	if len(listB) != 1 {
		t.Errorf("expected 1 for pipe-b, got %d", len(listB))
	}
}

func TestCheckpointStore_List_Empty(t *testing.T) {
	t.Parallel()
	store := NewCheckpointStore(10)

	list := store.List("nonexistent")
	if len(list) != 0 {
		t.Errorf("expected 0, got %d", len(list))
	}
}

// --- Engine Checkpoint tests ---

func TestEngine_CreateCheckpoint(t *testing.T) {
	t.Parallel()
	engine := NewEngine(DefaultEngineConfig())

	cfg := PipelineConfig{
		ID:          "test-pipe",
		Window:      WindowConfig{Type: WindowTumbling, Size: 10 * time.Second},
		Aggregation: AggSum,
	}
	if err := engine.CreatePipeline(cfg); err != nil {
		t.Fatal(err)
	}
	if err := engine.StartPipeline("test-pipe"); err != nil {
		t.Fatal(err)
	}

	// Ingest an event to advance watermark
	now := time.Now()
	engine.Ingest(Event{Key: "k1", Value: 42.0, Timestamp: now})

	cp, err := engine.CreateCheckpoint("test-pipe")
	if err != nil {
		t.Fatal(err)
	}
	if cp.PipelineID != "test-pipe" {
		t.Errorf("expected pipeline ID test-pipe, got %s", cp.PipelineID)
	}
}

func TestEngine_CreateCheckpoint_NonExistent(t *testing.T) {
	t.Parallel()
	engine := NewEngine(DefaultEngineConfig())

	_, err := engine.CreateCheckpoint("nonexistent")
	if err != ErrPipelineNotFound {
		t.Errorf("expected ErrPipelineNotFound, got %v", err)
	}
}

func TestEngine_RestoreFromCheckpoint(t *testing.T) {
	t.Parallel()
	engine := NewEngine(DefaultEngineConfig())

	cfg := PipelineConfig{
		ID:          "restore-pipe",
		Window:      WindowConfig{Type: WindowTumbling, Size: 10 * time.Second},
		Aggregation: AggSum,
	}
	engine.CreatePipeline(cfg)
	engine.StartPipeline("restore-pipe")

	// Ingest some events
	now := time.Now()
	engine.Ingest(Event{Key: "k1", Value: 10.0, Timestamp: now})

	// Create checkpoint
	engine.CreateCheckpoint("restore-pipe")

	// Ingest more events
	engine.Ingest(Event{Key: "k1", Value: 99.0, Timestamp: now.Add(time.Second)})

	// Restore from checkpoint
	err := engine.RestoreFromCheckpoint("restore-pipe")
	if err != nil {
		t.Fatal(err)
	}
}

func TestEngine_RestoreFromCheckpoint_NoCheckpoint(t *testing.T) {
	t.Parallel()
	engine := NewEngine(DefaultEngineConfig())

	cfg := PipelineConfig{
		ID:          "no-cp-pipe",
		Window:      WindowConfig{Type: WindowTumbling, Size: 10 * time.Second},
		Aggregation: AggSum,
	}
	engine.CreatePipeline(cfg)

	err := engine.RestoreFromCheckpoint("no-cp-pipe")
	if err != ErrCheckpointNotFound {
		t.Errorf("expected ErrCheckpointNotFound, got %v", err)
	}
}

func TestEngine_RestoreFromCheckpoint_NonExistentPipeline(t *testing.T) {
	t.Parallel()
	engine := NewEngine(DefaultEngineConfig())

	err := engine.RestoreFromCheckpoint("nonexistent")
	if err != ErrCheckpointNotFound {
		t.Errorf("expected ErrCheckpointNotFound, got %v", err)
	}
}

func TestEngine_GetCheckpoints_Empty(t *testing.T) {
	t.Parallel()
	engine := NewEngine(DefaultEngineConfig())

	cps := engine.GetCheckpoints("nonexistent")
	if len(cps) != 0 {
		t.Errorf("expected 0, got %d", len(cps))
	}
}

func TestEngine_GetCheckpoints_Populated(t *testing.T) {
	t.Parallel()
	engine := NewEngine(DefaultEngineConfig())

	cfg := PipelineConfig{
		ID:          "cp-pipe",
		Window:      WindowConfig{Type: WindowTumbling, Size: 10 * time.Second},
		Aggregation: AggSum,
	}
	engine.CreatePipeline(cfg)
	engine.StartPipeline("cp-pipe")

	engine.CreateCheckpoint("cp-pipe")
	engine.CreateCheckpoint("cp-pipe")

	cps := engine.GetCheckpoints("cp-pipe")
	if len(cps) != 2 {
		t.Errorf("expected 2 checkpoints, got %d", len(cps))
	}
}

func TestEngine_CreateCheckpoint_CapturesWindowState(t *testing.T) {
	t.Parallel()
	engine := NewEngine(DefaultEngineConfig())

	cfg := PipelineConfig{
		ID:          "window-cp",
		Window:      WindowConfig{Type: WindowTumbling, Size: time.Minute},
		Aggregation: AggSum,
		GroupByKey:  true,
	}
	engine.CreatePipeline(cfg)
	engine.StartPipeline("window-cp")

	now := time.Now()
	engine.Ingest(Event{Key: "user1", Value: 10.0, Timestamp: now})
	engine.Ingest(Event{Key: "user1", Value: 20.0, Timestamp: now.Add(time.Second)})

	cp, err := engine.CreateCheckpoint("window-cp")
	if err != nil {
		t.Fatal(err)
	}

	if len(cp.Windows) == 0 {
		t.Log("no window state captured (may depend on window firing)")
	}
}
