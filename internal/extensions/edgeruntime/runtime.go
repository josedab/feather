// Package edgeruntime provides a lightweight embedded feature serving
// runtime for edge devices, with offline-first sync and conflict resolution.
package edgeruntime

import (
	"fmt"
	"sync"
	"time"
)

// SyncState represents the synchronization state.
type SyncState string

const (
	// SyncStateSynced means all data is up-to-date with the central store.
	SyncStateSynced SyncState = "synced"
	// SyncStatePending means there are unsynced local writes.
	SyncStatePending SyncState = "pending"
	// SyncStateSyncing means a sync operation is in progress.
	SyncStateSyncing SyncState = "syncing"
	// SyncStateOffline means the central store is unreachable.
	SyncStateOffline SyncState = "offline"
	// SyncStateError means the last sync failed.
	SyncStateError SyncState = "error"
)

// ConflictStrategy controls how conflicts are resolved during sync.
type ConflictStrategy string

const (
	// ConflictLastWriteWins uses the most recent timestamp.
	ConflictLastWriteWins ConflictStrategy = "lww"
	// ConflictRemoteWins always prefers the remote value.
	ConflictRemoteWins ConflictStrategy = "remote_wins"
	// ConflictLocalWins always prefers the local value.
	ConflictLocalWins ConflictStrategy = "local_wins"
)

// RuntimeConfig configures the edge runtime.
type RuntimeConfig struct {
	DeviceID         string
	MaxFeatures      int
	MaxEntities      int
	SyncInterval     time.Duration
	ConflictStrategy ConflictStrategy
	CentralEndpoint  string
	OfflineQueueSize int
}

// DefaultRuntimeConfig returns sensible defaults.
func DefaultRuntimeConfig() RuntimeConfig {
	return RuntimeConfig{
		MaxFeatures:      1000,
		MaxEntities:      10000,
		SyncInterval:     30 * time.Second,
		ConflictStrategy: ConflictLastWriteWins,
		OfflineQueueSize: 10000,
	}
}

// FeatureValue represents a stored feature value on the edge.
type FeatureValue struct {
	Value     interface{} `json:"value"`
	Timestamp int64       `json:"timestamp"`
	Version   int64       `json:"version"`
	Source    string      `json:"source"` // "local" or "remote"
	Dirty     bool        `json:"dirty"`  // needs sync to central
}

// SyncOperation represents a queued write to sync to the central store.
type SyncOperation struct {
	EntityKey  string      `json:"entity_key"`
	FeatureKey string      `json:"feature_key"`
	Value      interface{} `json:"value"`
	Timestamp  int64       `json:"timestamp"`
	DeviceID   string      `json:"device_id"`
}

// SyncResult captures the outcome of a sync cycle.
type SyncResult struct {
	PushedCount   int           `json:"pushed_count"`
	PulledCount   int           `json:"pulled_count"`
	ConflictCount int           `json:"conflict_count"`
	ErrorCount    int           `json:"error_count"`
	Duration      time.Duration `json:"duration"`
	Timestamp     time.Time     `json:"timestamp"`
}

// Runtime is a lightweight feature store for edge devices.
type Runtime struct {
	config      RuntimeConfig
	store       map[string]map[string]*FeatureValue // entityKey -> featureKey -> value
	syncQueue   []SyncOperation
	syncState   SyncState
	lastSync    time.Time
	syncHistory []SyncResult
	mu          sync.RWMutex
}

// NewRuntime creates a new edge runtime.
func NewRuntime(cfg RuntimeConfig) *Runtime {
	return &Runtime{
		config:    cfg,
		store:     make(map[string]map[string]*FeatureValue),
		syncState: SyncStateOffline,
	}
}

// Get retrieves a feature value from the local store.
func (r *Runtime) Get(entityKey, featureKey string) (*FeatureValue, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	entity, ok := r.store[entityKey]
	if !ok {
		return nil, fmt.Errorf("entity %q not found", entityKey)
	}

	fv, ok := entity[featureKey]
	if !ok {
		return nil, fmt.Errorf("feature %q not found for entity %q", featureKey, entityKey)
	}
	return fv, nil
}

// GetBatch retrieves multiple features for an entity.
func (r *Runtime) GetBatch(entityKey string, featureKeys []string) map[string]*FeatureValue {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make(map[string]*FeatureValue)
	entity, ok := r.store[entityKey]
	if !ok {
		return result
	}

	for _, key := range featureKeys {
		if fv, exists := entity[key]; exists {
			result[key] = fv
		}
	}
	return result
}

// Put stores a feature value locally and queues it for sync.
func (r *Runtime) Put(entityKey, featureKey string, value interface{}) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Enforce limits
	if _, exists := r.store[entityKey]; !exists {
		if len(r.store) >= r.config.MaxEntities {
			return fmt.Errorf("max entities (%d) reached", r.config.MaxEntities)
		}
		r.store[entityKey] = make(map[string]*FeatureValue)
	}

	now := time.Now().UnixNano()
	fv := &FeatureValue{
		Value:     value,
		Timestamp: now,
		Version:   now,
		Source:    "local",
		Dirty:     true,
	}

	r.store[entityKey][featureKey] = fv

	// Queue for sync
	if len(r.syncQueue) < r.config.OfflineQueueSize {
		r.syncQueue = append(r.syncQueue, SyncOperation{
			EntityKey:  entityKey,
			FeatureKey: featureKey,
			Value:      value,
			Timestamp:  now,
			DeviceID:   r.config.DeviceID,
		})
		r.syncState = SyncStatePending
	}

	return nil
}

// ApplyRemote applies a feature update received from the central store.
func (r *Runtime) ApplyRemote(entityKey, featureKey string, value interface{}, timestamp int64) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.store[entityKey]; !exists {
		r.store[entityKey] = make(map[string]*FeatureValue)
	}

	existing, exists := r.store[entityKey][featureKey]
	if exists && existing.Dirty {
		// Conflict: local has unsent writes
		return r.resolveConflict(entityKey, featureKey, existing, value, timestamp)
	}

	r.store[entityKey][featureKey] = &FeatureValue{
		Value:     value,
		Timestamp: timestamp,
		Version:   timestamp,
		Source:    "remote",
		Dirty:     false,
	}
	return true
}

// GetPendingSync returns operations waiting to be synced.
func (r *Runtime) GetPendingSync() []SyncOperation {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]SyncOperation, len(r.syncQueue))
	copy(result, r.syncQueue)
	return result
}

// AcknowledgeSync marks sync operations as completed.
func (r *Runtime) AcknowledgeSync(count int) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if count >= len(r.syncQueue) {
		r.syncQueue = nil
		r.syncState = SyncStateSynced
	} else {
		r.syncQueue = r.syncQueue[count:]
		if len(r.syncQueue) > 0 {
			r.syncState = SyncStatePending
		} else {
			r.syncState = SyncStateSynced
		}
	}
	r.lastSync = time.Now()
}

// RecordSyncResult stores the result of a sync cycle.
func (r *Runtime) RecordSyncResult(result SyncResult) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.syncHistory = append(r.syncHistory, result)
	const maxHistory = 100
	if len(r.syncHistory) > maxHistory {
		r.syncHistory = r.syncHistory[len(r.syncHistory)-maxHistory:]
	}
}

// SetSyncState updates the sync state.
func (r *Runtime) SetSyncState(state SyncState) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.syncState = state
}

// Stats returns runtime statistics.
func (r *Runtime) Stats() map[string]interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()

	totalFeatures := 0
	dirtyFeatures := 0
	for _, entity := range r.store {
		for _, fv := range entity {
			totalFeatures++
			if fv.Dirty {
				dirtyFeatures++
			}
		}
	}

	return map[string]interface{}{
		"device_id":      r.config.DeviceID,
		"total_entities": len(r.store),
		"total_features": totalFeatures,
		"dirty_features": dirtyFeatures,
		"pending_sync":   len(r.syncQueue),
		"sync_state":     string(r.syncState),
		"last_sync":      r.lastSync,
		"sync_history":   len(r.syncHistory),
	}
}

// EntityCount returns the number of entities in the local store.
func (r *Runtime) EntityCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.store)
}

// Clear removes all data from the local store.
func (r *Runtime) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.store = make(map[string]map[string]*FeatureValue)
	r.syncQueue = nil
	r.syncState = SyncStateOffline
}

func (r *Runtime) resolveConflict(entityKey, featureKey string, local *FeatureValue, remoteValue interface{}, remoteTimestamp int64) bool {
	switch r.config.ConflictStrategy {
	case ConflictLastWriteWins:
		if remoteTimestamp > local.Timestamp {
			r.store[entityKey][featureKey] = &FeatureValue{
				Value: remoteValue, Timestamp: remoteTimestamp,
				Version: remoteTimestamp, Source: "remote",
			}
			return true
		}
		return false // local wins
	case ConflictRemoteWins:
		r.store[entityKey][featureKey] = &FeatureValue{
			Value: remoteValue, Timestamp: remoteTimestamp,
			Version: remoteTimestamp, Source: "remote",
		}
		return true
	case ConflictLocalWins:
		return false
	default:
		// Default to LWW
		if remoteTimestamp > local.Timestamp {
			r.store[entityKey][featureKey] = &FeatureValue{
				Value: remoteValue, Timestamp: remoteTimestamp,
				Version: remoteTimestamp, Source: "remote",
			}
			return true
		}
		return false
	}
}
