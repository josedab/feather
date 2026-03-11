package operator

import (
	"testing"
)

func TestBackupManager_CreateAndGet(t *testing.T) {
	m := NewBackupManager()

	backup, err := m.CreateBackup("store-1", BackupFull, "/my/path")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if backup.Status != "completed" {
		t.Errorf("expected completed, got %s", backup.Status)
	}
	if backup.StoragePath != "/my/path" {
		t.Errorf("expected /my/path, got %s", backup.StoragePath)
	}
	if backup.SizeBytes != 1024*1024*100 {
		t.Errorf("expected 100MB for full backup, got %d", backup.SizeBytes)
	}

	got, err := m.GetBackup(backup.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != backup.ID {
		t.Errorf("expected ID %s, got %s", backup.ID, got.ID)
	}
}

func TestBackupManager_CreateDefaultPath(t *testing.T) {
	m := NewBackupManager()
	backup, err := m.CreateBackup("store-1", BackupIncremental, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if backup.StoragePath == "" {
		t.Error("expected default storage path")
	}
	if backup.SizeBytes != 1024*1024*10 {
		t.Errorf("expected 10MB for incremental, got %d", backup.SizeBytes)
	}
}

func TestBackupManager_CreateRequiresFeatureStore(t *testing.T) {
	m := NewBackupManager()
	_, err := m.CreateBackup("", BackupFull, "")
	if err == nil {
		t.Fatal("expected error for empty feature store")
	}
}

func TestBackupManager_ListBackups(t *testing.T) {
	m := NewBackupManager()
	_, _ = m.CreateBackup("store-1", BackupFull, "")
	_, _ = m.CreateBackup("store-2", BackupSnapshot, "")
	_, _ = m.CreateBackup("store-1", BackupIncremental, "")

	all := m.ListBackups("")
	if len(all) != 3 {
		t.Errorf("expected 3 backups, got %d", len(all))
	}

	filtered := m.ListBackups("store-1")
	if len(filtered) != 2 {
		t.Errorf("expected 2 backups for store-1, got %d", len(filtered))
	}
}

func TestBackupManager_DeleteBackup(t *testing.T) {
	m := NewBackupManager()
	backup, _ := m.CreateBackup("store-1", BackupFull, "")

	if err := m.DeleteBackup(backup.ID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_, err := m.GetBackup(backup.ID)
	if err == nil {
		t.Fatal("expected error after delete")
	}
}

func TestBackupManager_DeleteNotFound(t *testing.T) {
	m := NewBackupManager()
	err := m.DeleteBackup("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent backup")
	}
}

func TestBackupManager_RestoreFromBackup(t *testing.T) {
	m := NewBackupManager()
	backup, _ := m.CreateBackup("store-1", BackupFull, "")

	restore, err := m.RestoreFromBackup(backup.ID, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if restore.Status != "completed" {
		t.Errorf("expected completed, got %s", restore.Status)
	}
	if restore.FeatureStore != "store-1" {
		t.Errorf("expected store-1, got %s", restore.FeatureStore)
	}

	got, err := m.GetRestore(restore.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.BackupID != backup.ID {
		t.Errorf("expected backup ID %s, got %s", backup.ID, got.BackupID)
	}
}

func TestBackupManager_RestoreNotFound(t *testing.T) {
	m := NewBackupManager()
	_, err := m.RestoreFromBackup("nonexistent", "")
	if err == nil {
		t.Fatal("expected error for nonexistent backup")
	}
}

func TestBackupManager_RestoreToCustomStore(t *testing.T) {
	m := NewBackupManager()
	backup, _ := m.CreateBackup("store-1", BackupFull, "")

	restore, err := m.RestoreFromBackup(backup.ID, "store-2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if restore.FeatureStore != "store-2" {
		t.Errorf("expected store-2, got %s", restore.FeatureStore)
	}
}

func TestBackupManager_ListRestores(t *testing.T) {
	m := NewBackupManager()
	backup, _ := m.CreateBackup("store-1", BackupFull, "")
	_, _ = m.RestoreFromBackup(backup.ID, "")
	_, _ = m.RestoreFromBackup(backup.ID, "store-2")

	restores := m.ListRestores()
	if len(restores) != 2 {
		t.Errorf("expected 2 restores, got %d", len(restores))
	}
}

func TestBackupManager_GetRestoreNotFound(t *testing.T) {
	m := NewBackupManager()
	_, err := m.GetRestore("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent restore")
	}
}

func TestBackupManager_SnapshotSize(t *testing.T) {
	m := NewBackupManager()
	backup, _ := m.CreateBackup("store-1", BackupSnapshot, "")
	if backup.SizeBytes != 1024*1024*50 {
		t.Errorf("expected 50MB for snapshot, got %d", backup.SizeBytes)
	}
}
