package federation

import (
	"testing"
)

func TestSecureAggregator_CreateSession(t *testing.T) {
	sa := NewSecureAggregator(DefaultSecureAggConfig())

	session, err := sa.CreateSession("sess-1", []string{"client-a", "client-b", "client-c"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if session.ID != "sess-1" {
		t.Errorf("expected session ID sess-1, got %s", session.ID)
	}
	if session.Status != AggStatusWaiting {
		t.Errorf("expected status waiting, got %s", session.Status)
	}
	if len(session.Participants) != 3 {
		t.Errorf("expected 3 participants, got %d", len(session.Participants))
	}
}

func TestSecureAggregator_CreateSession_Duplicate(t *testing.T) {
	sa := NewSecureAggregator(DefaultSecureAggConfig())

	_, _ = sa.CreateSession("sess-1", []string{"client-a"})
	_, err := sa.CreateSession("sess-1", []string{"client-b"})
	if err == nil {
		t.Fatal("expected error for duplicate session")
	}
}

func TestSecureAggregator_CreateSession_EmptyID(t *testing.T) {
	sa := NewSecureAggregator(DefaultSecureAggConfig())

	_, err := sa.CreateSession("", []string{"client-a"})
	if err == nil {
		t.Fatal("expected error for empty session ID")
	}
}

func TestSecureAggregator_CreateSession_NoParticipants(t *testing.T) {
	sa := NewSecureAggregator(DefaultSecureAggConfig())

	_, err := sa.CreateSession("sess-1", []string{})
	if err == nil {
		t.Fatal("expected error for empty participants")
	}
}

func TestSecureAggregator_SubmitContribution(t *testing.T) {
	sa := NewSecureAggregator(DefaultSecureAggConfig())
	_, _ = sa.CreateSession("sess-1", []string{"client-a", "client-b"})

	err := sa.SubmitContribution("sess-1", "client-a", []float64{1.0, 2.0, 3.0})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestSecureAggregator_SubmitContribution_InvalidSession(t *testing.T) {
	sa := NewSecureAggregator(DefaultSecureAggConfig())

	err := sa.SubmitContribution("nonexistent", "client-a", []float64{1.0})
	if err == nil {
		t.Fatal("expected error for nonexistent session")
	}
}

func TestSecureAggregator_SubmitContribution_InvalidParticipant(t *testing.T) {
	sa := NewSecureAggregator(DefaultSecureAggConfig())
	_, _ = sa.CreateSession("sess-1", []string{"client-a"})

	err := sa.SubmitContribution("sess-1", "unknown", []float64{1.0})
	if err == nil {
		t.Fatal("expected error for invalid participant")
	}
}

func TestSecureAggregator_SubmitContribution_Duplicate(t *testing.T) {
	sa := NewSecureAggregator(DefaultSecureAggConfig())
	_, _ = sa.CreateSession("sess-1", []string{"client-a", "client-b"})

	_ = sa.SubmitContribution("sess-1", "client-a", []float64{1.0})
	err := sa.SubmitContribution("sess-1", "client-a", []float64{2.0})
	if err == nil {
		t.Fatal("expected error for duplicate contribution")
	}
}

func TestSecureAggregator_Aggregate(t *testing.T) {
	sa := NewSecureAggregator(DefaultSecureAggConfig())
	_, _ = sa.CreateSession("sess-1", []string{"client-a", "client-b"})

	_ = sa.SubmitContribution("sess-1", "client-a", []float64{2.0, 4.0, 6.0})
	_ = sa.SubmitContribution("sess-1", "client-b", []float64{4.0, 6.0, 8.0})

	result, err := sa.Aggregate("sess-1")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if result.ParticipantCount != 2 {
		t.Errorf("expected 2 participants, got %d", result.ParticipantCount)
	}

	// Average: (2+4)/2=3, (4+6)/2=5, (6+8)/2=7
	expected := []float64{3.0, 5.0, 7.0}
	for i, v := range expected {
		if result.Values[i] != v {
			t.Errorf("expected value[%d] = %f, got %f", i, v, result.Values[i])
		}
	}

	// Session should be marked as aggregated
	session, _ := sa.GetSession("sess-1")
	if session.Status != AggStatusAggregated {
		t.Errorf("expected status aggregated, got %s", session.Status)
	}
}

func TestSecureAggregator_Aggregate_NotEnough(t *testing.T) {
	sa := NewSecureAggregator(DefaultSecureAggConfig())
	_, _ = sa.CreateSession("sess-1", []string{"client-a", "client-b"})

	_ = sa.SubmitContribution("sess-1", "client-a", []float64{1.0})

	_, err := sa.Aggregate("sess-1")
	if err == nil {
		t.Fatal("expected error for insufficient contributions")
	}
}

func TestSecureAggregator_Aggregate_AlreadyAggregated(t *testing.T) {
	sa := NewSecureAggregator(DefaultSecureAggConfig())
	_, _ = sa.CreateSession("sess-1", []string{"client-a", "client-b"})
	_ = sa.SubmitContribution("sess-1", "client-a", []float64{1.0})
	_ = sa.SubmitContribution("sess-1", "client-b", []float64{2.0})
	_, _ = sa.Aggregate("sess-1")

	_, err := sa.Aggregate("sess-1")
	if err == nil {
		t.Fatal("expected error for already aggregated session")
	}
}

func TestSecureAggregator_Aggregate_NonexistentSession(t *testing.T) {
	sa := NewSecureAggregator(DefaultSecureAggConfig())

	_, err := sa.Aggregate("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent session")
	}
}

func TestSecureAggregator_GetSession(t *testing.T) {
	sa := NewSecureAggregator(DefaultSecureAggConfig())
	_, _ = sa.CreateSession("sess-1", []string{"client-a"})

	session, err := sa.GetSession("sess-1")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if session.ID != "sess-1" {
		t.Errorf("expected session ID sess-1, got %s", session.ID)
	}
}

func TestSecureAggregator_GetSession_NotFound(t *testing.T) {
	sa := NewSecureAggregator(DefaultSecureAggConfig())

	_, err := sa.GetSession("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent session")
	}
}

func TestSecureAggregator_ListSessions(t *testing.T) {
	sa := NewSecureAggregator(DefaultSecureAggConfig())
	_, _ = sa.CreateSession("sess-1", []string{"client-a"})
	_, _ = sa.CreateSession("sess-2", []string{"client-b"})

	sessions := sa.ListSessions()
	if len(sessions) != 2 {
		t.Errorf("expected 2 sessions, got %d", len(sessions))
	}
}

func TestSecureAggregator_StatusTransition(t *testing.T) {
	sa := NewSecureAggregator(DefaultSecureAggConfig())
	_, _ = sa.CreateSession("sess-1", []string{"client-a", "client-b"})

	session, _ := sa.GetSession("sess-1")
	if session.Status != AggStatusWaiting {
		t.Errorf("expected waiting, got %s", session.Status)
	}

	_ = sa.SubmitContribution("sess-1", "client-a", []float64{1.0})
	session, _ = sa.GetSession("sess-1")
	if session.Status != AggStatusWaiting {
		t.Errorf("expected waiting after first contribution, got %s", session.Status)
	}

	_ = sa.SubmitContribution("sess-1", "client-b", []float64{2.0})
	session, _ = sa.GetSession("sess-1")
	if session.Status != AggStatusReady {
		t.Errorf("expected ready after threshold met, got %s", session.Status)
	}

	_, _ = sa.Aggregate("sess-1")
	session, _ = sa.GetSession("sess-1")
	if session.Status != AggStatusAggregated {
		t.Errorf("expected aggregated, got %s", session.Status)
	}
}

func TestDefaultSecureAggConfig(t *testing.T) {
	config := DefaultSecureAggConfig()

	if config.Protocol != AggProtocolMasked {
		t.Errorf("expected protocol masked, got %s", config.Protocol)
	}
	if config.Threshold != 2 {
		t.Errorf("expected threshold 2, got %d", config.Threshold)
	}
}
