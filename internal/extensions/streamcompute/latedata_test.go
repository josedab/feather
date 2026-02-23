package streamcompute

import (
	"testing"
	"time"
)

func TestLateDataHandler_OnTimeEvent(t *testing.T) {
	h := NewLateDataHandler(LateDataDrop, 5*time.Second)

	watermark := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	event := Event{Key: "k", Value: 1, Timestamp: watermark.Add(time.Second)}

	action, err := h.HandleEvent(event, watermark)
	if err != nil {
		t.Fatal(err)
	}
	if action.IsLate {
		t.Error("expected on-time event")
	}

	stats := h.Stats()
	if stats.OnTimeEvents != 1 {
		t.Errorf("expected 1 on-time event, got %d", stats.OnTimeEvents)
	}
}

func TestLateDataHandler_DropPolicy(t *testing.T) {
	h := NewLateDataHandler(LateDataDrop, 2*time.Second)

	watermark := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	// Late by 10 seconds, beyond 2s allowed lateness
	event := Event{Key: "k", Value: 1, Timestamp: watermark.Add(-10 * time.Second)}

	action, err := h.HandleEvent(event, watermark)
	if err != nil {
		t.Fatal(err)
	}
	if !action.IsLate {
		t.Error("expected late event")
	}
	if !action.Dropped {
		t.Error("expected dropped event")
	}

	stats := h.Stats()
	if stats.DroppedEvents != 1 {
		t.Errorf("expected 1 dropped event, got %d", stats.DroppedEvents)
	}
}

func TestLateDataHandler_UpdatePolicy(t *testing.T) {
	h := NewLateDataHandler(LateDataUpdate, 2*time.Second)

	watermark := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	event := Event{Key: "k", Value: 1, Timestamp: watermark.Add(-10 * time.Second)}

	action, err := h.HandleEvent(event, watermark)
	if err != nil {
		t.Fatal(err)
	}
	if !action.IsLate {
		t.Error("expected late event")
	}
	if action.Dropped {
		t.Error("expected not dropped for update policy")
	}

	stats := h.Stats()
	if stats.UpdatedEvents != 1 {
		t.Errorf("expected 1 updated event, got %d", stats.UpdatedEvents)
	}
}

func TestLateDataHandler_SideOutputPolicy(t *testing.T) {
	h := NewLateDataHandler(LateDataSideOutput, 2*time.Second)

	watermark := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	event := Event{Key: "k", Value: 42, Timestamp: watermark.Add(-10 * time.Second)}

	action, err := h.HandleEvent(event, watermark)
	if err != nil {
		t.Fatal(err)
	}
	if !action.IsLate {
		t.Error("expected late event")
	}

	sideOutput := h.GetSideOutput()
	if len(sideOutput) != 1 {
		t.Fatalf("expected 1 side output event, got %d", len(sideOutput))
	}
	if sideOutput[0].Value != 42 {
		t.Errorf("expected value 42, got %f", sideOutput[0].Value)
	}

	// GetSideOutput should drain the buffer
	sideOutput2 := h.GetSideOutput()
	if len(sideOutput2) != 0 {
		t.Errorf("expected empty side output after drain, got %d", len(sideOutput2))
	}

	stats := h.Stats()
	if stats.SideOutputEvents != 1 {
		t.Errorf("expected 1 side output event, got %d", stats.SideOutputEvents)
	}
}

func TestLateDataHandler_WithinAllowedLateness(t *testing.T) {
	h := NewLateDataHandler(LateDataDrop, 5*time.Second)

	watermark := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	// Late by 3 seconds, within 5s allowed lateness
	event := Event{Key: "k", Value: 1, Timestamp: watermark.Add(-3 * time.Second)}

	action, err := h.HandleEvent(event, watermark)
	if err != nil {
		t.Fatal(err)
	}
	if !action.IsLate {
		t.Error("expected late event")
	}
	if action.Dropped {
		t.Error("expected not dropped (within allowed lateness)")
	}

	stats := h.Stats()
	if stats.UpdatedEvents != 1 {
		t.Errorf("expected 1 updated event (within lateness), got %d", stats.UpdatedEvents)
	}
	if stats.DroppedEvents != 0 {
		t.Errorf("expected 0 dropped events, got %d", stats.DroppedEvents)
	}
}

func TestLateDataHandler_ZeroAllowedLateness(t *testing.T) {
	h := NewLateDataHandler(LateDataDrop, 0)

	watermark := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	event := Event{Key: "k", Value: 1, Timestamp: watermark.Add(-1 * time.Second)}

	action, err := h.HandleEvent(event, watermark)
	if err != nil {
		t.Fatal(err)
	}
	if !action.IsLate {
		t.Error("expected late event")
	}
	if !action.Dropped {
		t.Error("expected dropped with zero allowed lateness")
	}
}

func TestLateDataHandler_Stats(t *testing.T) {
	h := NewLateDataHandler(LateDataDrop, 2*time.Second)

	watermark := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)

	// On-time event
	h.HandleEvent(Event{Key: "k", Value: 1, Timestamp: watermark.Add(time.Second)}, watermark)
	// Within lateness
	h.HandleEvent(Event{Key: "k", Value: 2, Timestamp: watermark.Add(-1 * time.Second)}, watermark)
	// Beyond lateness (dropped)
	h.HandleEvent(Event{Key: "k", Value: 3, Timestamp: watermark.Add(-10 * time.Second)}, watermark)

	stats := h.Stats()
	if stats.TotalEvents != 3 {
		t.Errorf("expected 3 total events, got %d", stats.TotalEvents)
	}
	if stats.OnTimeEvents != 1 {
		t.Errorf("expected 1 on-time, got %d", stats.OnTimeEvents)
	}
	if stats.LateEvents != 2 {
		t.Errorf("expected 2 late events, got %d", stats.LateEvents)
	}
	if stats.DroppedEvents != 1 {
		t.Errorf("expected 1 dropped, got %d", stats.DroppedEvents)
	}
	if stats.UpdatedEvents != 1 {
		t.Errorf("expected 1 updated (within lateness), got %d", stats.UpdatedEvents)
	}
}
