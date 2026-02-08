package incrmat

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// OffsetCheckpoint stores CDC source offsets for exactly-once recovery.
type OffsetCheckpoint struct {
	SourceID     string            `json:"source_id"`
	LSN          int64             `json:"lsn"`
	BinlogFile   string            `json:"binlog_file,omitempty"`
	BinlogPos    int64             `json:"binlog_pos,omitempty"`
	EventCount   int64             `json:"event_count"`
	CheckpointAt time.Time         `json:"checkpoint_at"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

// RecoveryManager handles CDC offset checkpointing and recovery.
type RecoveryManager struct {
	mu          sync.RWMutex
	checkpoints map[string]*OffsetCheckpoint // sourceID -> latest checkpoint
	history     map[string][]OffsetCheckpoint // sourceID -> checkpoint history
	maxHistory  int
	cdcManager  *CDCManager
}

// RecoveryConfig configures the recovery manager.
type RecoveryConfig struct {
	MaxHistory       int           `json:"max_history"`
	CheckpointFreq   time.Duration `json:"checkpoint_freq"`
}

// DefaultRecoveryConfig returns sensible defaults.
func DefaultRecoveryConfig() RecoveryConfig {
	return RecoveryConfig{
		MaxHistory:     100,
		CheckpointFreq: 30 * time.Second,
	}
}

// NewRecoveryManager creates a new recovery manager.
func NewRecoveryManager(cdcManager *CDCManager, config RecoveryConfig) *RecoveryManager {
	if config.MaxHistory <= 0 {
		config.MaxHistory = 100
	}
	return &RecoveryManager{
		checkpoints: make(map[string]*OffsetCheckpoint),
		history:     make(map[string][]OffsetCheckpoint),
		maxHistory:  config.MaxHistory,
		cdcManager:  cdcManager,
	}
}

// Checkpoint saves the current offset for a CDC source.
func (rm *RecoveryManager) Checkpoint(sourceID string) (*OffsetCheckpoint, error) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	var lsn int64
	var eventCount int64

	if rm.cdcManager != nil {
		status, err := rm.cdcManager.GetSourceStatus(sourceID)
		if err != nil {
			return nil, fmt.Errorf("getting source status: %w", err)
		}
		eventCount = status.EventsCapture

		positions := rm.cdcManager.GetLSNPositions()
		lsn = positions[sourceID]
	}

	cp := &OffsetCheckpoint{
		SourceID:      sourceID,
		LSN:           lsn,
		EventCount:    eventCount,
		CheckpointAt:  time.Now(),
	}

	rm.checkpoints[sourceID] = cp

	// Append to history
	rm.history[sourceID] = append(rm.history[sourceID], *cp)
	if len(rm.history[sourceID]) > rm.maxHistory {
		rm.history[sourceID] = rm.history[sourceID][len(rm.history[sourceID])-rm.maxHistory:]
	}

	return cp, nil
}

// GetLatestCheckpoint returns the most recent checkpoint for a source.
func (rm *RecoveryManager) GetLatestCheckpoint(sourceID string) (*OffsetCheckpoint, bool) {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	cp, exists := rm.checkpoints[sourceID]
	if !exists {
		return nil, false
	}
	return cp, true
}

// GetCheckpointHistory returns the checkpoint history for a source.
func (rm *RecoveryManager) GetCheckpointHistory(sourceID string) []OffsetCheckpoint {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	h, exists := rm.history[sourceID]
	if !exists {
		return nil
	}
	result := make([]OffsetCheckpoint, len(h))
	copy(result, h)
	return result
}

// RecoverFrom restores CDC processing from a checkpoint.
func (rm *RecoveryManager) RecoverFrom(sourceID string) (*OffsetCheckpoint, error) {
	rm.mu.RLock()
	cp, exists := rm.checkpoints[sourceID]
	rm.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("no checkpoint for source %s", sourceID)
	}

	// Restore LSN position in the CDC manager
	if rm.cdcManager != nil {
		rm.cdcManager.mu.Lock()
		if rm.cdcManager.lsnTracker == nil {
			rm.cdcManager.lsnTracker = NewLSNTracker()
		}
		rm.cdcManager.lsnTracker.Advance(sourceID, cp.LSN)
		rm.cdcManager.mu.Unlock()
	}

	return cp, nil
}

// ExportCheckpoints serializes all checkpoints to JSON for external storage.
func (rm *RecoveryManager) ExportCheckpoints() ([]byte, error) {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	return json.Marshal(rm.checkpoints)
}

// ImportCheckpoints restores checkpoints from JSON.
func (rm *RecoveryManager) ImportCheckpoints(data []byte) error {
	var imported map[string]*OffsetCheckpoint
	if err := json.Unmarshal(data, &imported); err != nil {
		return fmt.Errorf("importing checkpoints: %w", err)
	}

	rm.mu.Lock()
	defer rm.mu.Unlock()

	for k, v := range imported {
		rm.checkpoints[k] = v
	}
	return nil
}

// ListAllCheckpoints returns the latest checkpoint for every source.
func (rm *RecoveryManager) ListAllCheckpoints() map[string]OffsetCheckpoint {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	result := make(map[string]OffsetCheckpoint, len(rm.checkpoints))
	for k, v := range rm.checkpoints {
		result[k] = *v
	}
	return result
}
