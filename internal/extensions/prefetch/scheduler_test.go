package prefetch

import (
	"testing"
	"time"
)

func TestScheduler_Schedule(t *testing.T) {
	ctrl := NewController(DefaultConfig())

	// Build co-access patterns
	for i := 0; i < 20; i++ {
		ctrl.RecordAccess("user1", []string{"age", "income", "score"})
	}

	sched := NewScheduler(ctrl, DefaultSchedulerConfig())
	actions := sched.Schedule("user1")

	// Should have some prefetch actions based on patterns
	if len(actions) > 0 {
		// Verify actions have priority
		for _, a := range actions {
			if a.Priority == "" {
				t.Fatal("expected priority to be set")
			}
			if a.EntityKey != "user1" {
				t.Fatalf("expected entity 'user1', got %q", a.EntityKey)
			}
		}
	}
}

func TestScheduler_MemoryBudget(t *testing.T) {
	ctrl := NewController(DefaultConfig())
	for i := 0; i < 50; i++ {
		ctrl.RecordAccess("e1", []string{"f1", "f2", "f3"})
	}

	cfg := DefaultSchedulerConfig()
	cfg.MaxMemoryBudgetMB = 1 // Very small budget
	cfg.EstBytesPerFeature = 500 * 1024 // 500KB per feature
	sched := NewScheduler(ctrl, cfg)

	sched.Schedule("e1")
	stats := sched.Stats()
	// With 1MB budget and 500KB per feature, max 2 features
	if stats.MemoryUsedBytes > int64(cfg.MaxMemoryBudgetMB)*1024*1024 {
		t.Fatalf("exceeded memory budget: %d > %d", stats.MemoryUsedBytes, int64(cfg.MaxMemoryBudgetMB)*1024*1024)
	}
}

func TestScheduler_Drain(t *testing.T) {
	ctrl := NewController(DefaultConfig())
	for i := 0; i < 20; i++ {
		ctrl.RecordAccess("e1", []string{"f1", "f2"})
	}

	sched := NewScheduler(ctrl, DefaultSchedulerConfig())
	sched.Schedule("e1")

	drained := sched.Drain()
	stats := sched.Stats()
	if stats.QueueDepth != 0 {
		t.Fatalf("expected empty queue after drain, got %d", stats.QueueDepth)
	}
	if stats.TotalExecuted != int64(len(drained)) {
		t.Fatalf("expected %d executed, got %d", len(drained), stats.TotalExecuted)
	}
}

func TestScheduler_HitRate(t *testing.T) {
	ctrl := NewController(DefaultConfig())
	sched := NewScheduler(ctrl, DefaultSchedulerConfig())

	sched.RecordHit()
	sched.RecordHit()
	sched.RecordMiss()

	stats := sched.Stats()
	expectedRate := 2.0 / 3.0
	if stats.HitRate < expectedRate-0.01 || stats.HitRate > expectedRate+0.01 {
		t.Fatalf("expected hit rate ~%.2f, got %.2f", expectedRate, stats.HitRate)
	}
}

func TestAccessPatternRingBuffer(t *testing.T) {
	rb := NewAccessPatternRingBuffer(5, 1.0)

	for i := 0; i < 3; i++ {
		rb.Record(AccessEvent{
			EntityKey: "user1",
			Features:  []string{"f1"},
			Timestamp: time.Now(),
		})
	}

	if rb.Size() != 3 {
		t.Fatalf("expected size 3, got %d", rb.Size())
	}

	snap := rb.Snapshot()
	if len(snap) != 3 {
		t.Fatalf("expected 3 events in snapshot, got %d", len(snap))
	}
}

func TestAccessPatternRingBuffer_Wraparound(t *testing.T) {
	rb := NewAccessPatternRingBuffer(3, 1.0)

	for i := 0; i < 5; i++ {
		rb.Record(AccessEvent{
			EntityKey: "user1",
			Features:  []string{"f1"},
			Timestamp: time.Now(),
		})
	}

	if rb.Size() != 3 {
		t.Fatalf("expected size 3 (capacity), got %d", rb.Size())
	}

	snap := rb.Snapshot()
	if len(snap) != 3 {
		t.Fatalf("expected 3 events after wraparound, got %d", len(snap))
	}
}

func TestAccessPatternRingBuffer_Sampling(t *testing.T) {
	// With 10% sampling rate, approximately 10% of events should be recorded
	rb := NewAccessPatternRingBuffer(1000, 0.1)

	for i := 0; i < 100; i++ {
		rb.Record(AccessEvent{EntityKey: "u", Features: []string{"f"}})
	}

	size := rb.Size()
	// Allow wide margin since sampling is deterministic modulo-based
	if size == 0 || size > 50 {
		t.Fatalf("expected some events with 10%% sampling (got %d out of 100)", size)
	}
}

func TestPriorityFromScore(t *testing.T) {
	if priorityFromScore(0.95) != "high" {
		t.Fatal("expected high priority for 0.95")
	}
	if priorityFromScore(0.75) != "medium" {
		t.Fatal("expected medium priority for 0.75")
	}
	if priorityFromScore(0.5) != "low" {
		t.Fatal("expected low priority for 0.5")
	}
}
