package operator

import (
	"fmt"
	"sync"
	"time"
)

// BackupType identifies the backup method.
type BackupType string

const (
	BackupFull        BackupType = "full"
	BackupIncremental BackupType = "incremental"
	BackupSnapshot    BackupType = "snapshot"
)

// BackupRecord represents a completed backup.
type BackupRecord struct {
	ID           string            `json:"id"`
	FeatureStore string            `json:"feature_store"`
	Type         BackupType        `json:"type"`
	SizeBytes    int64             `json:"size_bytes"`
	Status       string            `json:"status"` // "completed", "failed", "in_progress"
	StartedAt    time.Time         `json:"started_at"`
	CompletedAt  *time.Time        `json:"completed_at,omitempty"`
	StoragePath  string            `json:"storage_path"`
	Error        string            `json:"error,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

// RestoreRecord represents a restore operation.
type RestoreRecord struct {
	ID           string     `json:"id"`
	BackupID     string     `json:"backup_id"`
	FeatureStore string     `json:"feature_store"`
	Status       string     `json:"status"`
	StartedAt    time.Time  `json:"started_at"`
	CompletedAt  *time.Time `json:"completed_at,omitempty"`
	Error        string     `json:"error,omitempty"`
}

// BackupManager manages backup and restore operations.
type BackupManager struct {
	mu       sync.RWMutex
	backups  map[string]*BackupRecord
	restores map[string]*RestoreRecord
	nextID   int
}

// NewBackupManager creates a new backup manager.
func NewBackupManager() *BackupManager {
	return &BackupManager{
		backups:  make(map[string]*BackupRecord),
		restores: make(map[string]*RestoreRecord),
	}
}

// CreateBackup initiates a backup operation.
func (m *BackupManager) CreateBackup(featureStore string, backupType BackupType, storagePath string) (*BackupRecord, error) {
	if featureStore == "" {
		return nil, fmt.Errorf("feature_store is required")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.nextID++
	record := &BackupRecord{
		ID:           fmt.Sprintf("backup-%d", m.nextID),
		FeatureStore: featureStore,
		Type:         backupType,
		Status:       "completed",
		StartedAt:    time.Now(),
		StoragePath:  storagePath,
		Metadata:     make(map[string]string),
	}

	if storagePath == "" {
		record.StoragePath = fmt.Sprintf("/backups/%s/%s", featureStore, record.ID)
	}

	// Simulate backup size based on type
	switch backupType {
	case BackupFull:
		record.SizeBytes = 1024 * 1024 * 100 // 100MB estimate
	case BackupIncremental:
		record.SizeBytes = 1024 * 1024 * 10 // 10MB estimate
	case BackupSnapshot:
		record.SizeBytes = 1024 * 1024 * 50 // 50MB estimate
	}

	now := time.Now()
	record.CompletedAt = &now
	m.backups[record.ID] = record
	return record, nil
}

// GetBackup returns a backup record by ID.
func (m *BackupManager) GetBackup(id string) (*BackupRecord, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	record, exists := m.backups[id]
	if !exists {
		return nil, fmt.Errorf("backup %s not found", id)
	}
	result := *record
	return &result, nil
}

// ListBackups returns all backups for a FeatureStore.
func (m *BackupManager) ListBackups(featureStore string) []BackupRecord {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []BackupRecord
	for _, record := range m.backups {
		if featureStore == "" || record.FeatureStore == featureStore {
			result = append(result, *record)
		}
	}
	return result
}

// DeleteBackup removes a backup record.
func (m *BackupManager) DeleteBackup(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.backups[id]; !exists {
		return fmt.Errorf("backup %s not found", id)
	}
	delete(m.backups, id)
	return nil
}

// RestoreFromBackup initiates a restore operation.
func (m *BackupManager) RestoreFromBackup(backupID, featureStore string) (*RestoreRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	backup, exists := m.backups[backupID]
	if !exists {
		return nil, fmt.Errorf("backup %s not found", backupID)
	}
	if backup.Status != "completed" {
		return nil, fmt.Errorf("backup %s is not completed", backupID)
	}

	if featureStore == "" {
		featureStore = backup.FeatureStore
	}

	m.nextID++
	record := &RestoreRecord{
		ID:           fmt.Sprintf("restore-%d", m.nextID),
		BackupID:     backupID,
		FeatureStore: featureStore,
		Status:       "completed",
		StartedAt:    time.Now(),
	}
	now := time.Now()
	record.CompletedAt = &now
	m.restores[record.ID] = record
	return record, nil
}

// GetRestore returns a restore record by ID.
func (m *BackupManager) GetRestore(id string) (*RestoreRecord, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	record, exists := m.restores[id]
	if !exists {
		return nil, fmt.Errorf("restore %s not found", id)
	}
	result := *record
	return &result, nil
}

// ListRestores returns all restore operations.
func (m *BackupManager) ListRestores() []RestoreRecord {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]RestoreRecord, 0, len(m.restores))
	for _, r := range m.restores {
		result = append(result, *r)
	}
	return result
}
