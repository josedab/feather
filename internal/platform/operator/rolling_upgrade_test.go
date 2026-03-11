package operator

import (
	"testing"
)

func TestDefaultUpgradeStrategy(t *testing.T) {
	s := DefaultUpgradeStrategy()
	if s.MaxUnavailable != 1 {
		t.Errorf("expected MaxUnavailable=1, got %d", s.MaxUnavailable)
	}
	if s.MaxSurge != 1 {
		t.Errorf("expected MaxSurge=1, got %d", s.MaxSurge)
	}
}

func TestUpgradeManager_StartAndAdvance(t *testing.T) {
	m := NewUpgradeManager()
	strategy := DefaultUpgradeStrategy()

	upgrade, err := m.StartUpgrade("store-1", "v1.0", "v1.1", 3, strategy)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if upgrade.Phase != UpgradeInProgress {
		t.Errorf("expected phase in_progress, got %s", upgrade.Phase)
	}

	// Advance 3 times to complete
	for i := 0; i < 3; i++ {
		upgrade, err = m.AdvanceUpgrade("store-1")
		if err != nil {
			t.Fatalf("advance %d: unexpected error: %v", i, err)
		}
	}
	if upgrade.Phase != UpgradeCompleted {
		t.Errorf("expected completed, got %s", upgrade.Phase)
	}
}

func TestUpgradeManager_DuplicateStart(t *testing.T) {
	m := NewUpgradeManager()
	strategy := DefaultUpgradeStrategy()

	_, err := m.StartUpgrade("store-1", "v1.0", "v1.1", 3, strategy)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_, err = m.StartUpgrade("store-1", "v1.1", "v1.2", 3, strategy)
	if err == nil {
		t.Fatal("expected error for duplicate upgrade")
	}
}

func TestUpgradeManager_PauseResume(t *testing.T) {
	m := NewUpgradeManager()
	strategy := DefaultUpgradeStrategy()

	_, _ = m.StartUpgrade("store-1", "v1.0", "v1.1", 3, strategy)

	if err := m.PauseUpgrade("store-1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	upgrade, _ := m.GetUpgrade("store-1")
	if upgrade.Phase != UpgradePaused {
		t.Errorf("expected paused, got %s", upgrade.Phase)
	}

	// Cannot advance while paused
	_, err := m.AdvanceUpgrade("store-1")
	if err == nil {
		t.Fatal("expected error advancing paused upgrade")
	}

	if err := m.ResumeUpgrade("store-1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	upgrade, _ = m.GetUpgrade("store-1")
	if upgrade.Phase != UpgradeInProgress {
		t.Errorf("expected in_progress, got %s", upgrade.Phase)
	}
}

func TestUpgradeManager_Rollback(t *testing.T) {
	m := NewUpgradeManager()
	strategy := DefaultUpgradeStrategy()

	_, _ = m.StartUpgrade("store-1", "v1.0", "v1.1", 3, strategy)
	_, _ = m.AdvanceUpgrade("store-1")

	upgrade, err := m.RollbackUpgrade("store-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if upgrade.Phase != UpgradeRolledBack {
		t.Errorf("expected rolled_back, got %s", upgrade.Phase)
	}

	// Should be in history now
	history := m.ListHistory()
	if len(history) != 1 {
		t.Errorf("expected 1 history entry, got %d", len(history))
	}
}

func TestUpgradeManager_GetUpgradeNotFound(t *testing.T) {
	m := NewUpgradeManager()
	_, err := m.GetUpgrade("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent upgrade")
	}
}

func TestUpgradeManager_ResumeNotPaused(t *testing.T) {
	m := NewUpgradeManager()
	strategy := DefaultUpgradeStrategy()
	_, _ = m.StartUpgrade("store-1", "v1.0", "v1.1", 3, strategy)

	err := m.ResumeUpgrade("store-1")
	if err == nil {
		t.Fatal("expected error resuming non-paused upgrade")
	}
}
