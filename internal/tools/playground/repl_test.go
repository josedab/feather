package playground

import (
	"context"
	"testing"
	"time"
)

type mockProvider struct{}

func (m *mockProvider) GetFeature(ctx context.Context, entity, feature string) (interface{}, time.Time, error) {
	return 42, time.Now(), nil
}
func (m *mockProvider) ListFeatures(ctx context.Context) ([]string, error) {
	return []string{"click_count", "purchase_total"}, nil
}
func (m *mockProvider) GetFeatureValues(ctx context.Context, feature string, limit int) ([]interface{}, error) {
	return []interface{}{1, 2, 3}, nil
}

func TestREPLEngine_CreateSession(t *testing.T) {
	engine := NewREPLEngine(&mockProvider{})
	session := engine.CreateSession()
	if session.ID == "" {
		t.Error("session ID should not be empty")
	}
	if len(session.Variables) != 0 {
		t.Error("new session should have empty variables")
	}
}

func TestREPLEngine_GetSession(t *testing.T) {
	engine := NewREPLEngine(&mockProvider{})
	session := engine.CreateSession()

	got, err := engine.GetSession(session.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != session.ID {
		t.Errorf("expected ID %q, got %q", session.ID, got.ID)
	}

	_, err = engine.GetSession("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent session")
	}
}

func TestREPLEngine_ListSessions(t *testing.T) {
	engine := NewREPLEngine(&mockProvider{})
	engine.CreateSession()
	engine.CreateSession()

	sessions := engine.ListSessions()
	if len(sessions) != 2 {
		t.Errorf("expected 2 sessions, got %d", len(sessions))
	}
}

func TestREPLEngine_ExecuteHelp(t *testing.T) {
	engine := NewREPLEngine(&mockProvider{})
	session := engine.CreateSession()

	cmd := engine.Execute(session.ID, "help")
	if cmd.Error != "" {
		t.Errorf("unexpected error: %s", cmd.Error)
	}
	if cmd.Output == nil {
		t.Error("expected help output")
	}
}

func TestREPLEngine_ExecuteGet(t *testing.T) {
	engine := NewREPLEngine(&mockProvider{})
	session := engine.CreateSession()

	cmd := engine.Execute(session.ID, "get user:123 click_count")
	if cmd.Error != "" {
		t.Errorf("unexpected error: %s", cmd.Error)
	}
}

func TestREPLEngine_ExecuteSet(t *testing.T) {
	engine := NewREPLEngine(&mockProvider{})
	session := engine.CreateSession()

	cmd := engine.Execute(session.ID, "set entity user:456")
	if cmd.Error != "" {
		t.Errorf("unexpected error: %s", cmd.Error)
	}

	got, _ := engine.GetSession(session.ID)
	if got.Variables["entity"] != "user:456" {
		t.Errorf("expected variable 'entity' = 'user:456', got %v", got.Variables["entity"])
	}
}

func TestREPLEngine_ExecuteList(t *testing.T) {
	engine := NewREPLEngine(&mockProvider{})
	session := engine.CreateSession()

	cmd := engine.Execute(session.ID, "list features")
	if cmd.Error != "" {
		t.Errorf("unexpected error: %s", cmd.Error)
	}
}

func TestREPLEngine_ExecuteDescribe(t *testing.T) {
	engine := NewREPLEngine(&mockProvider{})
	session := engine.CreateSession()

	cmd := engine.Execute(session.ID, "describe click_count")
	if cmd.Error != "" {
		t.Errorf("unexpected error: %s", cmd.Error)
	}
}

func TestREPLEngine_ExecuteHistory(t *testing.T) {
	engine := NewREPLEngine(&mockProvider{})
	session := engine.CreateSession()

	engine.Execute(session.ID, "help")
	cmd := engine.Execute(session.ID, "history")
	if cmd.Error != "" {
		t.Errorf("unexpected error: %s", cmd.Error)
	}
}

func TestREPLEngine_ExecuteClear(t *testing.T) {
	engine := NewREPLEngine(&mockProvider{})
	session := engine.CreateSession()

	engine.Execute(session.ID, "set x 1")
	engine.Execute(session.ID, "clear")

	got, _ := engine.GetSession(session.ID)
	// Clear resets history, but the clear command itself runs after reset,
	// so history has just the clear command.
	if len(got.History) != 1 {
		t.Errorf("expected 1 entry (the clear cmd itself), got %d", len(got.History))
	}
	if len(got.Variables) != 0 {
		t.Errorf("expected empty vars after clear, got %d", len(got.Variables))
	}
}

func TestREPLEngine_ExecuteUnknownCommand(t *testing.T) {
	engine := NewREPLEngine(&mockProvider{})
	session := engine.CreateSession()

	cmd := engine.Execute(session.ID, "foobar")
	if cmd.Error == "" {
		t.Error("expected error for unknown command")
	}
}

func TestREPLEngine_ExecuteEmptyInput(t *testing.T) {
	engine := NewREPLEngine(&mockProvider{})
	session := engine.CreateSession()

	cmd := engine.Execute(session.ID, "   ")
	if cmd.Error == "" {
		t.Error("expected error for empty input")
	}
}

func TestREPLEngine_DeleteSession(t *testing.T) {
	engine := NewREPLEngine(&mockProvider{})
	session := engine.CreateSession()

	if err := engine.DeleteSession(session.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if err := engine.DeleteSession(session.ID); err == nil {
		t.Error("expected error for double delete")
	}
}

func TestREPLEngine_ExportState(t *testing.T) {
	engine := NewREPLEngine(&mockProvider{})
	session := engine.CreateSession()

	engine.Execute(session.ID, "set x 1")
	engine.Execute(session.ID, "help")

	state, err := engine.ExportState(session.ID)
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	if len(state.Commands) != 2 {
		t.Errorf("expected 2 commands, got %d", len(state.Commands))
	}
}

func TestREPLEngine_ExportStateNotFound(t *testing.T) {
	engine := NewREPLEngine(&mockProvider{})
	_, err := engine.ExportState("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent session")
	}
}

func TestBuiltinTutorials(t *testing.T) {
	tutorials := BuiltinTutorials()
	if len(tutorials) < 2 {
		t.Errorf("expected at least 2 tutorials, got %d", len(tutorials))
	}
	for _, tut := range tutorials {
		if tut.Name == "" {
			t.Error("tutorial name should not be empty")
		}
		if len(tut.Steps) == 0 {
			t.Errorf("tutorial %q should have steps", tut.Name)
		}
	}
}

func TestREPLEngine_ExecuteInvalidSession(t *testing.T) {
	engine := NewREPLEngine(&mockProvider{})
	cmd := engine.Execute("bad-session", "help")
	if cmd.Error == "" {
		t.Error("expected error for invalid session")
	}
}
