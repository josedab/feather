package notebooksdk

import (
	"testing"
)

func TestNewService(t *testing.T) {
	svc := NewService(DefaultConfig())
	if svc == nil {
		t.Fatal("NewService returned nil")
	}
	stats := svc.Stats()
	if stats.ActiveSessions != 0 || stats.TotalSessions != 0 {
		t.Errorf("fresh service should have zero stats, got active=%d total=%d",
			stats.ActiveSessions, stats.TotalSessions)
	}
}

func TestCreateSession(t *testing.T) {
	tests := []struct {
		name    string
		cfg     SessionConfig
		wantErr bool
	}{
		{
			name: "valid session",
			cfg:  SessionConfig{Notebook: "nb1", User: "alice", ConnectionURL: "http://localhost"},
		},
		{
			name: "minimal session",
			cfg:  SessionConfig{Notebook: "nb2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewService(DefaultConfig())
			sess, err := svc.CreateSession(tt.cfg)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if sess.ID == "" {
				t.Error("session ID should not be empty")
			}
			if sess.State != SessionStateActive {
				t.Errorf("State = %q, want %q", sess.State, SessionStateActive)
			}
		})
	}
}

func TestCreateSession_MaxSessions(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxSessions = 2
	svc := NewService(cfg)

	if _, err := svc.CreateSession(SessionConfig{Notebook: "n1"}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CreateSession(SessionConfig{Notebook: "n2"}); err != nil {
		t.Fatal(err)
	}
	// Third should fail
	if _, err := svc.CreateSession(SessionConfig{Notebook: "n3"}); err == nil {
		t.Error("expected error when max sessions reached")
	}
}

func TestGetSession(t *testing.T) {
	svc := NewService(DefaultConfig())
	sess, _ := svc.CreateSession(SessionConfig{Notebook: "nb1", User: "alice"})

	got, err := svc.GetSession(sess.ID)
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if got.ID != sess.ID {
		t.Errorf("ID = %q, want %q", got.ID, sess.ID)
	}

	// Not found
	if _, err := svc.GetSession("nonexistent"); err == nil {
		t.Error("expected error for nonexistent session")
	}
}

func TestListSessions(t *testing.T) {
	svc := NewService(DefaultConfig())

	if infos := svc.ListSessions(); len(infos) != 0 {
		t.Errorf("expected 0 sessions, got %d", len(infos))
	}

	svc.CreateSession(SessionConfig{Notebook: "n1", User: "alice"})
	svc.CreateSession(SessionConfig{Notebook: "n2", User: "bob"})

	infos := svc.ListSessions()
	if len(infos) != 2 {
		t.Errorf("expected 2 sessions, got %d", len(infos))
	}
}

func TestExecute(t *testing.T) {
	tests := []struct {
		name    string
		command string
		wantErr bool
	}{
		{"connect command", "%feather_connect http://localhost", false},
		{"get command", "%feather_get entity:123 feature1,feature2", false},
		{"search command", "%feather_search click", false},
		{"schema command", "%feather_schema", false},
		{"empty command", "", true},
		{"unsupported command", "%feather_badcmd arg", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewService(DefaultConfig())
			sess, err := svc.CreateSession(SessionConfig{Notebook: "nb", User: "u"})
			if err != nil {
				t.Fatal(err)
			}

			result, err := svc.Execute(sess.ID, tt.command)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.Output == "" {
				t.Error("expected non-empty output")
			}
		})
	}
}

func TestExecute_InvalidSession(t *testing.T) {
	svc := NewService(DefaultConfig())
	_, err := svc.Execute("nonexistent", "%feather_get entity:1 f1")
	if err == nil {
		t.Error("expected error for nonexistent session")
	}
}

func TestCloseSession(t *testing.T) {
	svc := NewService(DefaultConfig())
	sess, _ := svc.CreateSession(SessionConfig{Notebook: "nb1"})

	if err := svc.CloseSession(sess.ID); err != nil {
		t.Fatalf("CloseSession: %v", err)
	}

	// Should not be retrievable after close
	if _, err := svc.GetSession(sess.ID); err == nil {
		t.Error("expected error for closed session")
	}

	// Closing again should fail
	if err := svc.CloseSession(sess.ID); err == nil {
		t.Error("expected error for already-closed session")
	}
}

func TestStats(t *testing.T) {
	svc := NewService(DefaultConfig())
	sess, _ := svc.CreateSession(SessionConfig{Notebook: "nb", User: "u", ConnectionURL: "http://x"})
	svc.Execute(sess.ID, "%feather_connect")

	stats := svc.Stats()
	if stats.ActiveSessions != 1 {
		t.Errorf("ActiveSessions = %d, want 1", stats.ActiveSessions)
	}
	if stats.TotalSessions != 1 {
		t.Errorf("TotalSessions = %d, want 1", stats.TotalSessions)
	}
	if stats.TotalExecutions != 1 {
		t.Errorf("TotalExecutions = %d, want 1", stats.TotalExecutions)
	}
}
