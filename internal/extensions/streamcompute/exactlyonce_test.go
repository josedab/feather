package streamcompute

import (
	"testing"
	"time"
)

func TestExactlyOnceProcessor_BasicTransaction(t *testing.T) {
	proc := NewExactlyOnceProcessor(DefaultExactlyOnceConfig())

	tx, err := proc.BeginTransaction()
	if err != nil {
		t.Fatal(err)
	}
	if tx.Status != TxPending {
		t.Errorf("expected pending, got %s", tx.Status)
	}

	event := Event{Key: "user1", Value: 10.0, Timestamp: time.Now()}
	if err := proc.ProcessEvent(tx, event); err != nil {
		t.Fatal(err)
	}
	if len(tx.Events) != 1 {
		t.Errorf("expected 1 event, got %d", len(tx.Events))
	}

	if err := proc.Commit(tx); err != nil {
		t.Fatal(err)
	}
	if tx.Status != TxCommitted {
		t.Errorf("expected committed, got %s", tx.Status)
	}
}

func TestExactlyOnceProcessor_Rollback(t *testing.T) {
	proc := NewExactlyOnceProcessor(DefaultExactlyOnceConfig())

	tx, err := proc.BeginTransaction()
	if err != nil {
		t.Fatal(err)
	}

	event := Event{Key: "user1", Value: 5.0, Timestamp: time.Now()}
	if err := proc.ProcessEvent(tx, event); err != nil {
		t.Fatal(err)
	}

	if err := proc.Rollback(tx); err != nil {
		t.Fatal(err)
	}
	if tx.Status != TxRolledBack {
		t.Errorf("expected rolled_back, got %s", tx.Status)
	}
	if len(tx.Events) != 0 {
		t.Errorf("expected 0 events after rollback, got %d", len(tx.Events))
	}
}

func TestExactlyOnceProcessor_Deduplication(t *testing.T) {
	proc := NewExactlyOnceProcessor(DefaultExactlyOnceConfig())
	ts := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	event := Event{Key: "user1", Value: 10.0, Timestamp: ts}

	// First transaction: commit the event
	tx1, _ := proc.BeginTransaction()
	proc.ProcessEvent(tx1, event)
	proc.Commit(tx1)

	// Second transaction: same event should be deduplicated
	tx2, _ := proc.BeginTransaction()
	proc.ProcessEvent(tx2, event)
	if len(tx2.Events) != 0 {
		t.Errorf("expected 0 events (deduplicated), got %d", len(tx2.Events))
	}

	stats := proc.Stats()
	if stats.DuplicatesDetected != 1 {
		t.Errorf("expected 1 duplicate, got %d", stats.DuplicatesDetected)
	}
}

func TestExactlyOnceProcessor_MaxPendingTransactions(t *testing.T) {
	cfg := ExactlyOnceConfig{
		DeduplicationWindowSize: 100,
		CheckpointInterval:      time.Second,
		MaxPendingTransactions:  2,
	}
	proc := NewExactlyOnceProcessor(cfg)

	_, err := proc.BeginTransaction()
	if err != nil {
		t.Fatal(err)
	}
	_, err = proc.BeginTransaction()
	if err != nil {
		t.Fatal(err)
	}

	_, err = proc.BeginTransaction()
	if err == nil {
		t.Fatal("expected error for exceeding max pending transactions")
	}
}

func TestExactlyOnceProcessor_CommitNonPending(t *testing.T) {
	proc := NewExactlyOnceProcessor(DefaultExactlyOnceConfig())

	tx, _ := proc.BeginTransaction()
	proc.Commit(tx)

	if err := proc.Commit(tx); err == nil {
		t.Fatal("expected error committing non-pending transaction")
	}
}

func TestExactlyOnceProcessor_RollbackNonPending(t *testing.T) {
	proc := NewExactlyOnceProcessor(DefaultExactlyOnceConfig())

	tx, _ := proc.BeginTransaction()
	proc.Commit(tx)

	if err := proc.Rollback(tx); err == nil {
		t.Fatal("expected error rolling back non-pending transaction")
	}
}

func TestExactlyOnceProcessor_Stats(t *testing.T) {
	proc := NewExactlyOnceProcessor(DefaultExactlyOnceConfig())

	tx1, _ := proc.BeginTransaction()
	proc.ProcessEvent(tx1, Event{Key: "a", Value: 1, Timestamp: time.Now()})
	proc.Commit(tx1)

	tx2, _ := proc.BeginTransaction()
	proc.Rollback(tx2)

	stats := proc.Stats()
	if stats.TotalTransactions != 2 {
		t.Errorf("expected 2 total, got %d", stats.TotalTransactions)
	}
	if stats.CommittedTransactions != 1 {
		t.Errorf("expected 1 committed, got %d", stats.CommittedTransactions)
	}
	if stats.RolledBackTransactions != 1 {
		t.Errorf("expected 1 rolled back, got %d", stats.RolledBackTransactions)
	}
}

func TestExactlyOnceProcessor_NilTransaction(t *testing.T) {
	proc := NewExactlyOnceProcessor(DefaultExactlyOnceConfig())

	if err := proc.ProcessEvent(nil, Event{}); err == nil {
		t.Fatal("expected error for nil transaction")
	}
	if err := proc.Commit(nil); err == nil {
		t.Fatal("expected error for nil transaction")
	}
	if err := proc.Rollback(nil); err == nil {
		t.Fatal("expected error for nil transaction")
	}
}

func TestExactlyOnceProcessor_IsDuplicate(t *testing.T) {
	proc := NewExactlyOnceProcessor(DefaultExactlyOnceConfig())

	eventID := "test-event-1"
	if proc.IsDuplicate(eventID) {
		t.Error("expected not duplicate before commit")
	}

	// Manually add to seenIDs to test
	proc.mu.Lock()
	proc.seenIDs[eventID] = time.Now()
	proc.mu.Unlock()

	if !proc.IsDuplicate(eventID) {
		t.Error("expected duplicate after recording")
	}
}

func TestExactlyOnceProcessor_DeduplicationEviction(t *testing.T) {
	cfg := ExactlyOnceConfig{
		DeduplicationWindowSize: 3,
		CheckpointInterval:      time.Second,
		MaxPendingTransactions:  100,
	}
	proc := NewExactlyOnceProcessor(cfg)

	base := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 5; i++ {
		tx, _ := proc.BeginTransaction()
		proc.ProcessEvent(tx, Event{
			Key:       "k",
			Value:     float64(i),
			Timestamp: base.Add(time.Duration(i) * time.Second),
		})
		proc.Commit(tx)
	}

	stats := proc.Stats()
	if stats.DeduplicationSize > 3 {
		t.Errorf("expected dedup size <= 3, got %d", stats.DeduplicationSize)
	}
}
