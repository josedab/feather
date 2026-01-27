package multiregion

import (
	"testing"
	"time"
)

func TestNewReplicationManager(t *testing.T) {
	rm := NewReplicationManager(DefaultFederationConfig())
	if rm == nil {
		t.Fatal("expected non-nil replication manager")
	}
}

func TestEnqueueAndDrain(t *testing.T) {
	rm := NewReplicationManager(DefaultFederationConfig())

	err := rm.EnqueueReplication(ReplicationEvent{
		FromRegion: "us-east-1", Entity: "e1", Feature: "age",
		Version: 1,
	})
	if err != nil {
		t.Fatal(err)
	}

	events := rm.DrainPending(10)
	if len(events) != 1 {
		t.Fatalf("expected 1 pending event, got %d", len(events))
	}

	// Should be empty after drain
	events = rm.DrainPending(10)
	if len(events) != 0 {
		t.Fatalf("expected 0 pending events after drain, got %d", len(events))
	}
}

func TestApplyReplicationNoConflict(t *testing.T) {
	rm := NewReplicationManager(DefaultFederationConfig())

	conflict, err := rm.ApplyReplication(ReplicationEvent{
		ID: "r1", FromRegion: "eu-west-1", Entity: "e1", Version: 1,
		Timestamp: time.Now(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if conflict != nil {
		t.Error("expected no conflict")
	}

	stats := rm.Stats()
	if stats.TotalReplicated != 1 {
		t.Errorf("expected 1 replicated, got %d", stats.TotalReplicated)
	}
}

func TestApplyReplicationWithConflict(t *testing.T) {
	rm := NewReplicationManager(DefaultFederationConfig())

	// Apply version 2
	rm.ApplyReplication(ReplicationEvent{
		ID: "r1", FromRegion: "eu-west-1", Entity: "e1", Version: 2,
		Timestamp: time.Now(),
	})

	// Apply version 1 (lower) - should conflict
	conflict, err := rm.ApplyReplication(ReplicationEvent{
		ID: "r2", FromRegion: "ap-southeast-1", Entity: "e1", Version: 1,
		Timestamp: time.Now(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if conflict == nil {
		t.Fatal("expected conflict")
	}

	stats := rm.Stats()
	if stats.TotalConflicts != 1 {
		t.Errorf("expected 1 conflict, got %d", stats.TotalConflicts)
	}
}

func TestGetConflicts(t *testing.T) {
	rm := NewReplicationManager(DefaultFederationConfig())

	// Create a conflict
	rm.ApplyReplication(ReplicationEvent{ID: "r1", Entity: "e1", Version: 5, Timestamp: time.Now()})
	rm.ApplyReplication(ReplicationEvent{ID: "r2", Entity: "e1", Version: 3, Timestamp: time.Now()})

	conflicts := rm.GetConflicts(10)
	if len(conflicts) != 1 {
		t.Fatalf("expected 1 conflict, got %d", len(conflicts))
	}
}

func TestMissingEntity(t *testing.T) {
	rm := NewReplicationManager(DefaultFederationConfig())
	err := rm.EnqueueReplication(ReplicationEvent{FromRegion: "us-east-1"})
	if err == nil {
		t.Fatal("expected error for missing entity")
	}
}
