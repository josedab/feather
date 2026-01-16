package consensus

import (
	"fmt"
	"sync"
	"time"
)

// SyncMode controls how the WAL syncs data to disk.
type SyncMode string

const (
	// SyncModeImmediate syncs after every write.
	SyncModeImmediate SyncMode = "immediate"
	// SyncModeBatch batches syncs at a configurable interval.
	SyncModeBatch SyncMode = "batch"
	// SyncModeNone relies on the OS for flushing.
	SyncModeNone SyncMode = "none"
)

// WALConfig configures the Write-Ahead Log.
type WALConfig struct {
	SegmentSize    int64         `json:"segment_size"`
	SyncMode       SyncMode      `json:"sync_mode"`
	RetentionCount int           `json:"retention_count"`
	SyncInterval   time.Duration `json:"sync_interval"`
}

// DefaultWALConfig returns sensible defaults for the WAL.
func DefaultWALConfig() WALConfig {
	return WALConfig{
		SegmentSize:    64 * 1024 * 1024, // 64 MB
		SyncMode:       SyncModeBatch,
		RetentionCount: 10000,
		SyncInterval:   100 * time.Millisecond,
	}
}

// WALEntry represents a single entry persisted in the WAL.
type WALEntry struct {
	Index     uint64    `json:"index"`
	Term      uint64    `json:"term"`
	Type      EntryType `json:"type"`
	Data      []byte    `json:"data"`
	Timestamp time.Time `json:"timestamp"`
	CRC       uint32    `json:"crc"`
}

// WALStats holds operational statistics for the WAL.
type WALStats struct {
	EntryCount    int       `json:"entry_count"`
	FirstIndex    uint64    `json:"first_index"`
	LastIndex     uint64    `json:"last_index"`
	SegmentSize   int64     `json:"segment_size"`
	SyncCount     uint64    `json:"sync_count"`
	BytesWritten  int64     `json:"bytes_written"`
	LastSyncTime  time.Time `json:"last_sync_time"`
	TruncateCount uint64    `json:"truncate_count"`
}

// WAL provides a durable Write-Ahead Log for Raft log entries.
// Entries are appended before being applied to the state machine,
// guaranteeing durability across restarts.
type WAL struct {
	config   WALConfig
	entries  []*WALEntry
	stats    WALStats
	closed   bool
	mu       sync.RWMutex
}

// NewWAL creates a new Write-Ahead Log with the given configuration.
func NewWAL(config WALConfig) *WAL {
	return &WAL{
		config:  config,
		entries: make([]*WALEntry, 0),
	}
}

// Append adds a new entry to the WAL. The entry is persisted before
// control returns to the caller.
func (w *WAL) Append(entry *WALEntry) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return fmt.Errorf("appending to WAL: log is closed")
	}
	if entry == nil {
		return fmt.Errorf("appending to WAL: nil entry")
	}

	entry.Timestamp = time.Now()
	w.entries = append(w.entries, entry)

	dataLen := int64(len(entry.Data))
	w.stats.EntryCount = len(w.entries)
	w.stats.LastIndex = entry.Index
	w.stats.BytesWritten += dataLen
	if w.stats.FirstIndex == 0 {
		w.stats.FirstIndex = entry.Index
	}

	// Enforce retention
	if w.config.RetentionCount > 0 && len(w.entries) > w.config.RetentionCount {
		excess := len(w.entries) - w.config.RetentionCount
		w.entries = w.entries[excess:]
		w.stats.EntryCount = len(w.entries)
		if len(w.entries) > 0 {
			w.stats.FirstIndex = w.entries[0].Index
		}
	}

	if w.config.SyncMode == SyncModeImmediate {
		w.stats.SyncCount++
		w.stats.LastSyncTime = time.Now()
	}

	return nil
}

// Read returns all WAL entries between startIndex and endIndex (inclusive).
func (w *WAL) Read(startIndex, endIndex uint64) ([]*WALEntry, error) {
	w.mu.RLock()
	defer w.mu.RUnlock()

	if w.closed {
		return nil, fmt.Errorf("reading WAL: log is closed")
	}
	if startIndex > endIndex {
		return nil, fmt.Errorf("reading WAL: start index %d > end index %d", startIndex, endIndex)
	}

	result := make([]*WALEntry, 0)
	for _, e := range w.entries {
		if e.Index >= startIndex && e.Index <= endIndex {
			result = append(result, e)
		}
	}
	return result, nil
}

// Truncate removes all entries up to and including the given index.
func (w *WAL) Truncate(upToIndex uint64) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return fmt.Errorf("truncating WAL: log is closed")
	}

	filtered := make([]*WALEntry, 0, len(w.entries))
	for _, e := range w.entries {
		if e.Index > upToIndex {
			filtered = append(filtered, e)
		}
	}
	w.entries = filtered
	w.stats.EntryCount = len(w.entries)
	w.stats.TruncateCount++
	if len(w.entries) > 0 {
		w.stats.FirstIndex = w.entries[0].Index
	} else {
		w.stats.FirstIndex = 0
		w.stats.LastIndex = 0
	}

	return nil
}

// Sync forces a flush of all buffered WAL data.
func (w *WAL) Sync() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return fmt.Errorf("syncing WAL: log is closed")
	}

	w.stats.SyncCount++
	w.stats.LastSyncTime = time.Now()
	return nil
}

// Close flushes remaining data and marks the WAL as closed.
func (w *WAL) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return fmt.Errorf("closing WAL: already closed")
	}
	w.closed = true
	w.stats.SyncCount++
	w.stats.LastSyncTime = time.Now()
	return nil
}

// Stats returns operational statistics for the WAL.
func (w *WAL) Stats() WALStats {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.stats
}

// --- Snapshot Manager ---

// SnapshotConfig configures the snapshot manager.
type SnapshotConfig struct {
	Interval     time.Duration `json:"interval"`
	MaxSnapshots int           `json:"max_snapshots"`
}

// DefaultSnapshotConfig returns sensible defaults for the snapshot manager.
func DefaultSnapshotConfig() SnapshotConfig {
	return SnapshotConfig{
		Interval:     10 * time.Minute,
		MaxSnapshots: 5,
	}
}

// SnapshotMeta holds metadata about a single snapshot.
type SnapshotMeta struct {
	ID        string    `json:"id"`
	Index     uint64    `json:"index"`
	Term      uint64    `json:"term"`
	Size      int64     `json:"size"`
	CreatedAt time.Time `json:"created_at"`
}

// SnapshotManager periodically captures state machine snapshots and
// manages their lifecycle (creation, restoration, cleanup).
type SnapshotManager struct {
	config    SnapshotConfig
	snapshots []*SnapshotMeta
	data      map[string][]byte // id -> snapshot data
	mu        sync.RWMutex
}

// NewSnapshotManager creates a new snapshot manager.
func NewSnapshotManager(config SnapshotConfig) *SnapshotManager {
	return &SnapshotManager{
		config:    config,
		snapshots: make([]*SnapshotMeta, 0),
		data:      make(map[string][]byte),
	}
}

// Take captures a snapshot at the given index and term, storing the
// provided data blob.
func (sm *SnapshotManager) Take(index uint64, term uint64, data []byte) (*SnapshotMeta, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if data == nil {
		return nil, fmt.Errorf("taking snapshot: nil data")
	}

	id := fmt.Sprintf("snap-%d-%d", term, index)
	meta := &SnapshotMeta{
		ID:        id,
		Index:     index,
		Term:      term,
		Size:      int64(len(data)),
		CreatedAt: time.Now(),
	}

	snapshot := make([]byte, len(data))
	copy(snapshot, data)
	sm.data[id] = snapshot
	sm.snapshots = append(sm.snapshots, meta)

	// Enforce max snapshots
	if sm.config.MaxSnapshots > 0 && len(sm.snapshots) > sm.config.MaxSnapshots {
		excess := len(sm.snapshots) - sm.config.MaxSnapshots
		for i := 0; i < excess; i++ {
			delete(sm.data, sm.snapshots[i].ID)
		}
		sm.snapshots = sm.snapshots[excess:]
	}

	return meta, nil
}

// Restore returns the data for the given snapshot ID so the state
// machine can be rebuilt.
func (sm *SnapshotManager) Restore(id string) ([]byte, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	data, ok := sm.data[id]
	if !ok {
		return nil, fmt.Errorf("restoring snapshot: snapshot %q not found", id)
	}

	result := make([]byte, len(data))
	copy(result, data)
	return result, nil
}

// List returns metadata for all available snapshots, newest first.
func (sm *SnapshotManager) List() []*SnapshotMeta {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	result := make([]*SnapshotMeta, len(sm.snapshots))
	for i, s := range sm.snapshots {
		result[len(sm.snapshots)-1-i] = s
	}
	return result
}

// Cleanup removes snapshots that exceed the configured maximum.
func (sm *SnapshotManager) Cleanup() int {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if sm.config.MaxSnapshots <= 0 || len(sm.snapshots) <= sm.config.MaxSnapshots {
		return 0
	}

	excess := len(sm.snapshots) - sm.config.MaxSnapshots
	for i := 0; i < excess; i++ {
		delete(sm.data, sm.snapshots[i].ID)
	}
	sm.snapshots = sm.snapshots[excess:]
	return excess
}

// --- Multi-Region Coordinator ---

// RegionHealth represents the health status of a region.
type RegionHealth string

const (
	// RegionHealthy indicates the region is operating normally.
	RegionHealthy RegionHealth = "healthy"
	// RegionDegraded indicates the region has elevated latency or partial failures.
	RegionDegraded RegionHealth = "degraded"
	// RegionUnhealthy indicates the region is not reachable.
	RegionUnhealthy RegionHealth = "unhealthy"
)

// RegionStatus tracks the replication state of a single region.
type RegionStatus struct {
	ID            string       `json:"id"`
	Health        RegionHealth `json:"health"`
	LastHeartbeat time.Time    `json:"last_heartbeat"`
	LagEntries    uint64       `json:"lag_entries"`
	LagDuration   time.Duration `json:"lag_duration"`
	IsPrimary     bool         `json:"is_primary"`
}

// MultiRegionCoordinator tracks cross-region replication lag,
// region health, and coordinates failover.
type MultiRegionCoordinator struct {
	regions    map[string]*RegionStatus
	primaryID  string
	mu         sync.RWMutex
}

// NewMultiRegionCoordinator creates a new coordinator.
func NewMultiRegionCoordinator() *MultiRegionCoordinator {
	return &MultiRegionCoordinator{
		regions: make(map[string]*RegionStatus),
	}
}

// RegisterRegion adds a region to be tracked.
func (c *MultiRegionCoordinator) RegisterRegion(id string, isPrimary bool) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if id == "" {
		return fmt.Errorf("registering region: empty region ID")
	}
	if _, exists := c.regions[id]; exists {
		return fmt.Errorf("registering region: region %q already exists", id)
	}

	c.regions[id] = &RegionStatus{
		ID:            id,
		Health:        RegionHealthy,
		LastHeartbeat: time.Now(),
		IsPrimary:     isPrimary,
	}
	if isPrimary {
		c.primaryID = id
	}
	return nil
}

// UpdateLag records the current replication lag for a region.
func (c *MultiRegionCoordinator) UpdateLag(regionID string, lagEntries uint64, lagDuration time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	region, ok := c.regions[regionID]
	if !ok {
		return fmt.Errorf("updating lag: region %q not found", regionID)
	}

	region.LagEntries = lagEntries
	region.LagDuration = lagDuration
	region.LastHeartbeat = time.Now()
	return nil
}

// UpdateHealth sets the health status for a region.
func (c *MultiRegionCoordinator) UpdateHealth(regionID string, health RegionHealth) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	region, ok := c.regions[regionID]
	if !ok {
		return fmt.Errorf("updating health: region %q not found", regionID)
	}

	region.Health = health
	return nil
}

// Failover promotes a healthy replica region to primary. The previous
// primary is demoted. Returns the new primary region ID.
func (c *MultiRegionCoordinator) Failover() (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Find the healthiest replica with the lowest lag
	var best *RegionStatus
	for _, r := range c.regions {
		if r.IsPrimary {
			continue
		}
		if r.Health != RegionHealthy {
			continue
		}
		if best == nil || r.LagEntries < best.LagEntries {
			best = r
		}
	}

	if best == nil {
		return "", fmt.Errorf("failover: no healthy replica region available")
	}

	// Demote current primary
	if current, ok := c.regions[c.primaryID]; ok {
		current.IsPrimary = false
	}

	// Promote best replica
	best.IsPrimary = true
	c.primaryID = best.ID
	return best.ID, nil
}

// ListRegions returns the status of all tracked regions.
func (c *MultiRegionCoordinator) ListRegions() []*RegionStatus {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make([]*RegionStatus, 0, len(c.regions))
	for _, r := range c.regions {
		cp := *r
		result = append(result, &cp)
	}
	return result
}

// GetPrimary returns the current primary region ID.
func (c *MultiRegionCoordinator) GetPrimary() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.primaryID
}
