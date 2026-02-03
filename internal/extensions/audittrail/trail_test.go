package audittrail

import (
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	trail := New(DefaultConfig())
	if trail == nil {
		t.Fatal("New returned nil")
	}
	stats := trail.Stats()
	if stats.TotalEvents != 0 {
		t.Errorf("TotalEvents = %d, want 0", stats.TotalEvents)
	}
}

func TestRecord(t *testing.T) {
	tests := []struct {
		name    string
		events  []Event
		wantErr bool
	}{
		{
			name: "single event",
			events: []Event{
				{ID: "e1", Action: ActionCreate, Entity: "user:1", Feature: "clicks", Actor: "alice"},
			},
		},
		{
			name: "multiple events",
			events: []Event{
				{ID: "e1", Action: ActionCreate, Entity: "user:1", Feature: "clicks", Actor: "alice"},
				{ID: "e2", Action: ActionRead, Entity: "user:1", Feature: "clicks", Actor: "bob"},
				{ID: "e3", Action: ActionUpdate, Entity: "user:1", Feature: "clicks", Actor: "alice"},
			},
		},
		{
			name: "duplicate ID",
			events: []Event{
				{ID: "e1", Action: ActionCreate, Entity: "user:1", Feature: "f", Actor: "a"},
				{ID: "e1", Action: ActionRead, Entity: "user:1", Feature: "f", Actor: "a"},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			trail := New(DefaultConfig())
			var lastErr error
			for _, e := range tt.events {
				if err := trail.Record(e); err != nil {
					lastErr = err
				}
			}
			if tt.wantErr {
				if lastErr == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if lastErr != nil {
				t.Fatalf("unexpected error: %v", lastErr)
			}
		})
	}
}

func TestRecord_MaxEvents(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxEvents = 2
	trail := New(cfg)

	trail.Record(Event{ID: "e1", Action: ActionCreate, Entity: "u", Feature: "f", Actor: "a"})
	trail.Record(Event{ID: "e2", Action: ActionCreate, Entity: "u", Feature: "f", Actor: "a"})
	err := trail.Record(Event{ID: "e3", Action: ActionCreate, Entity: "u", Feature: "f", Actor: "a"})
	if err == nil {
		t.Error("expected error when max events reached")
	}
}

func TestRecord_HashChaining(t *testing.T) {
	trail := New(DefaultConfig())
	trail.Record(Event{ID: "e1", Action: ActionCreate, Entity: "u", Feature: "f", Actor: "a"})
	trail.Record(Event{ID: "e2", Action: ActionRead, Entity: "u", Feature: "f", Actor: "b"})

	valid, err := trail.VerifyChain()
	if err != nil {
		t.Fatalf("VerifyChain: %v", err)
	}
	if !valid {
		t.Error("chain should be valid")
	}
}

func TestQueryByEntity(t *testing.T) {
	trail := New(DefaultConfig())
	now := time.Now().UTC()

	trail.Record(Event{ID: "e1", Action: ActionCreate, Entity: "user:1", Feature: "f", Actor: "a", Timestamp: now})
	trail.Record(Event{ID: "e2", Action: ActionCreate, Entity: "user:2", Feature: "f", Actor: "a", Timestamp: now})
	trail.Record(Event{ID: "e3", Action: ActionRead, Entity: "user:1", Feature: "f", Actor: "b", Timestamp: now})

	start := now.Add(-time.Minute)
	end := now.Add(time.Minute)

	events := trail.QueryByEntity("user:1", start, end)
	if len(events) != 2 {
		t.Errorf("expected 2 events for user:1, got %d", len(events))
	}

	events = trail.QueryByEntity("user:2", start, end)
	if len(events) != 1 {
		t.Errorf("expected 1 event for user:2, got %d", len(events))
	}
}

func TestQueryByTimeRange(t *testing.T) {
	trail := New(DefaultConfig())
	t1 := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)
	t2 := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	t3 := time.Date(2024, 1, 1, 14, 0, 0, 0, time.UTC)

	trail.Record(Event{ID: "e1", Action: ActionCreate, Entity: "u", Feature: "f", Actor: "a", Timestamp: t1})
	trail.Record(Event{ID: "e2", Action: ActionRead, Entity: "u", Feature: "f", Actor: "a", Timestamp: t2})
	trail.Record(Event{ID: "e3", Action: ActionUpdate, Entity: "u", Feature: "f", Actor: "a", Timestamp: t3})

	events := trail.QueryByTimeRange(t1, t2)
	if len(events) != 2 {
		t.Errorf("expected 2 events in range, got %d", len(events))
	}

	events = trail.QueryByTimeRange(t3, t3.Add(time.Hour))
	if len(events) != 1 {
		t.Errorf("expected 1 event in range, got %d", len(events))
	}
}

func TestVerifyChain(t *testing.T) {
	trail := New(DefaultConfig())

	// Empty trail should verify
	valid, err := trail.VerifyChain()
	if err != nil {
		t.Fatalf("VerifyChain on empty: %v", err)
	}
	if !valid {
		t.Error("empty chain should be valid")
	}

	trail.Record(Event{ID: "e1", Action: ActionCreate, Entity: "u", Feature: "f", Actor: "a"})
	trail.Record(Event{ID: "e2", Action: ActionUpdate, Entity: "u", Feature: "f", Actor: "a"})

	valid, err = trail.VerifyChain()
	if err != nil {
		t.Fatalf("VerifyChain: %v", err)
	}
	if !valid {
		t.Error("chain should be valid")
	}
}

func TestGetProof(t *testing.T) {
	trail := New(DefaultConfig())
	trail.Record(Event{ID: "e1", Action: ActionCreate, Entity: "u", Feature: "f", Actor: "a"})
	trail.Record(Event{ID: "e2", Action: ActionRead, Entity: "u", Feature: "f", Actor: "b"})

	proof, err := trail.GetProof("e1")
	if err != nil {
		t.Fatalf("GetProof: %v", err)
	}
	if proof.EventID != "e1" {
		t.Errorf("EventID = %q, want %q", proof.EventID, "e1")
	}
	if !proof.Verified {
		t.Error("proof should be verified")
	}
	if proof.Hash == "" {
		t.Error("hash should not be empty")
	}
	if proof.MerkleRoot == "" {
		t.Error("MerkleRoot should not be empty")
	}

	// Not found
	if _, err := trail.GetProof("nonexistent"); err == nil {
		t.Error("expected error for nonexistent event")
	}
}

func TestVerifyProof(t *testing.T) {
	trail := New(DefaultConfig())
	trail.Record(Event{ID: "e1", Action: ActionCreate, Entity: "u", Feature: "f", Actor: "a"})

	proof, _ := trail.GetProof("e1")
	if !trail.VerifyProof(proof) {
		t.Error("valid proof should verify")
	}

	// Tampered proof should fail
	tampered := *proof
	tampered.Hash = "0000000000000000000000000000000000000000000000000000000000000000"
	if trail.VerifyProof(&tampered) {
		t.Error("tampered proof should not verify")
	}
}

func TestGenerateComplianceReport(t *testing.T) {
	tests := []struct {
		name       string
		reportType string
		wantErr    bool
	}{
		{"sox report", "sox", false},
		{"hipaa report", "hipaa", false},
		{"gdpr report", "gdpr", false},
		{"unsupported type", "pci", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			trail := New(DefaultConfig())
			now := time.Now().UTC()
			trail.Record(Event{ID: "e1", Action: ActionRead, Entity: "u", Feature: "f", Actor: "a", Timestamp: now})
			trail.Record(Event{ID: "e2", Action: ActionUpdate, Entity: "u", Feature: "f", Actor: "a", Timestamp: now})

			start := now.Add(-time.Hour)
			end := now.Add(time.Hour)

			report, err := trail.GenerateComplianceReport(tt.reportType, start, end)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if report.Type != tt.reportType {
				t.Errorf("Type = %q, want %q", report.Type, tt.reportType)
			}
			if report.IntegrityStatus != "verified" {
				t.Errorf("IntegrityStatus = %q, want %q", report.IntegrityStatus, "verified")
			}
			if report.TotalEvents != 2 {
				t.Errorf("TotalEvents = %d, want 2", report.TotalEvents)
			}
		})
	}
}

func TestTrailStats(t *testing.T) {
	trail := New(DefaultConfig())
	trail.Record(Event{ID: "e1", Action: ActionCreate, Entity: "u", Feature: "f", Actor: "a"})
	trail.Record(Event{ID: "e2", Action: ActionRead, Entity: "u", Feature: "f", Actor: "b"})

	stats := trail.Stats()
	if stats.TotalEvents != 2 {
		t.Errorf("TotalEvents = %d, want 2", stats.TotalEvents)
	}
	if stats.ChainLength != 2 {
		t.Errorf("ChainLength = %d, want 2", stats.ChainLength)
	}
	if stats.MerkleRoot == "" {
		t.Error("MerkleRoot should not be empty")
	}
	if !stats.IntegrityVerified {
		t.Error("IntegrityVerified should be true")
	}
}
