package multiregion

import (
	"fmt"
	"sync"
	"time"
)

// ReplicationMode controls how data is replicated between regions.
type ReplicationMode string

const (
	ReplicationSync  ReplicationMode = "sync"
	ReplicationAsync ReplicationMode = "async"
)

// ConflictRecord logs a resolved conflict for audit purposes.
type ConflictRecord struct {
	EventID       string    `json:"event_id"`
	Entity        string    `json:"entity"`
	LocalVersion  int64     `json:"local_version"`
	RemoteVersion int64     `json:"remote_version"`
	Winner        string    `json:"winner"` // "local" or "remote"
	Strategy      string    `json:"strategy"`
	ResolvedAt    time.Time `json:"resolved_at"`
}

// ReplicationStats tracks replication health.
type ReplicationStats struct {
	TotalReplicated  int64            `json:"total_replicated"`
	TotalConflicts   int64            `json:"total_conflicts"`
	ConflictsLocal   int64            `json:"conflicts_resolved_local"`
	ConflictsRemote  int64            `json:"conflicts_resolved_remote"`
	PendingEvents    int              `json:"pending_events"`
	ByRegion         map[string]int64 `json:"by_region"`
	LastReplicatedAt time.Time        `json:"last_replicated_at"`
}

// ReplicationManager handles cross-region data replication and conflict resolution.
type ReplicationManager struct {
	mu           sync.RWMutex
	config       FederationConfig
	pending      []ReplicationEvent
	conflicts    []ConflictRecord
	versions     map[string]int64 // entity -> latest version
	stats        ReplicationStats
	maxPending   int
	maxConflicts int
}

// NewReplicationManager creates a new replication manager.
func NewReplicationManager(config FederationConfig) *ReplicationManager {
	return &ReplicationManager{
		config:       config,
		versions:     make(map[string]int64),
		stats:        ReplicationStats{ByRegion: make(map[string]int64)},
		maxPending:   100000,
		maxConflicts: 10000,
	}
}

// EnqueueReplication enqueues a replication event for sending to other regions.
func (rm *ReplicationManager) EnqueueReplication(event ReplicationEvent) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if event.Entity == "" {
		return fmt.Errorf("entity is required")
	}

	if len(rm.pending) >= rm.maxPending {
		rm.pending = rm.pending[1:]
	}

	event.Timestamp = time.Now()
	rm.pending = append(rm.pending, event)
	rm.stats.PendingEvents = len(rm.pending)
	return nil
}

// ApplyReplication applies an incoming replication event, resolving conflicts.
func (rm *ReplicationManager) ApplyReplication(event ReplicationEvent) (*ConflictRecord, error) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	localVersion := rm.versions[event.Entity]

	// No conflict: remote version is newer
	if event.Version > localVersion {
		rm.versions[event.Entity] = event.Version
		rm.stats.TotalReplicated++
		rm.stats.ByRegion[event.FromRegion]++
		rm.stats.LastReplicatedAt = time.Now()
		return nil, nil
	}

	// Conflict: resolve based on configured strategy
	rm.stats.TotalConflicts++
	conflict := &ConflictRecord{
		EventID:       event.ID,
		Entity:        event.Entity,
		LocalVersion:  localVersion,
		RemoteVersion: event.Version,
		Strategy:      string(rm.config.ConflictStrategy),
		ResolvedAt:    time.Now(),
	}

	switch rm.config.ConflictStrategy {
	case ConflictLWW:
		if event.Timestamp.After(time.Now().Add(-rm.config.MaxLagDuration)) {
			conflict.Winner = "remote"
			rm.versions[event.Entity] = event.Version
			rm.stats.ConflictsRemote++
		} else {
			conflict.Winner = "local"
			rm.stats.ConflictsLocal++
		}
	case ConflictHighestVersion:
		if event.Version > localVersion {
			conflict.Winner = "remote"
			rm.versions[event.Entity] = event.Version
			rm.stats.ConflictsRemote++
		} else {
			conflict.Winner = "local"
			rm.stats.ConflictsLocal++
		}
	default:
		conflict.Winner = "local"
		rm.stats.ConflictsLocal++
	}

	if len(rm.conflicts) >= rm.maxConflicts {
		rm.conflicts = rm.conflicts[1:]
	}
	rm.conflicts = append(rm.conflicts, *conflict)

	rm.stats.TotalReplicated++
	rm.stats.ByRegion[event.FromRegion]++
	rm.stats.LastReplicatedAt = time.Now()

	return conflict, nil
}

// DrainPending returns and clears all pending replication events.
func (rm *ReplicationManager) DrainPending(limit int) []ReplicationEvent {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if limit <= 0 || limit > len(rm.pending) {
		limit = len(rm.pending)
	}

	events := make([]ReplicationEvent, limit)
	copy(events, rm.pending[:limit])
	rm.pending = rm.pending[limit:]
	rm.stats.PendingEvents = len(rm.pending)
	return events
}

// GetConflicts returns recent conflict records.
func (rm *ReplicationManager) GetConflicts(limit int) []ConflictRecord {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	if limit > len(rm.conflicts) {
		limit = len(rm.conflicts)
	}
	start := len(rm.conflicts) - limit
	result := make([]ConflictRecord, limit)
	copy(result, rm.conflicts[start:])
	return result
}

// Stats returns replication statistics.
func (rm *ReplicationManager) Stats() ReplicationStats {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	return rm.stats
}
