package edgeruntime

import (
	"sync"
	"time"
)

// DeltaSyncConfig configures the delta sync protocol.
type DeltaSyncConfig struct {
	MaxBatchSize       int           `json:"max_batch_size"`
	CompressionEnabled bool          `json:"compression_enabled"`
	DeltaThreshold     float64       `json:"delta_threshold"`
	MaxDeltaAge        time.Duration `json:"max_delta_age"`
}

// DefaultDeltaSyncConfig returns sensible defaults.
func DefaultDeltaSyncConfig() DeltaSyncConfig {
	return DeltaSyncConfig{
		MaxBatchSize:       1000,
		CompressionEnabled: true,
		DeltaThreshold:     0.1,
		MaxDeltaAge:        24 * time.Hour,
	}
}

// DeltaEntry records a single change.
type DeltaEntry struct {
	EntityKey  string      `json:"entity_key"`
	FeatureKey string      `json:"feature_key"`
	OldValue   interface{} `json:"old_value"`
	NewValue   interface{} `json:"new_value"`
	Timestamp  int64       `json:"timestamp"`
	Operation  string      `json:"operation"` // "put" or "delete"
}

// DeltaStats provides statistics about the delta log.
type DeltaStats struct {
	TotalEntries    int       `json:"total_entries"`
	CompactedEntries int      `json:"compacted_entries"`
	AvgBatchSize    int       `json:"avg_batch_size"`
	OldestEntry     time.Time `json:"oldest_entry"`
	NewestEntry     time.Time `json:"newest_entry"`
}

// DeltaLog maintains an append-only log of changes for delta synchronisation.
type DeltaLog struct {
	config           DeltaSyncConfig
	entries          []*DeltaEntry
	compactedEntries int
	mu               sync.RWMutex
}

// NewDeltaLog creates a new DeltaLog.
func NewDeltaLog(config DeltaSyncConfig) *DeltaLog {
	return &DeltaLog{
		config: config,
	}
}

// RecordDelta appends a change entry to the log.
func (dl *DeltaLog) RecordDelta(entry *DeltaEntry) {
	dl.mu.Lock()
	defer dl.mu.Unlock()

	if entry.Timestamp == 0 {
		entry.Timestamp = time.Now().UnixNano()
	}
	dl.entries = append(dl.entries, entry)
}

// GetDeltasSince returns all entries with a timestamp >= the given value.
func (dl *DeltaLog) GetDeltasSince(timestamp int64) []*DeltaEntry {
	dl.mu.RLock()
	defer dl.mu.RUnlock()

	var result []*DeltaEntry
	for _, e := range dl.entries {
		if e.Timestamp >= timestamp {
			result = append(result, e)
		}
	}
	return result
}

// GetDeltasForDevice returns deltas since the device's last sync timestamp.
// deviceID is accepted for future per-device filtering; currently it returns
// all deltas since lastSync.
func (dl *DeltaLog) GetDeltasForDevice(_ string, lastSync int64) []*DeltaEntry {
	return dl.GetDeltasSince(lastSync)
}

// Compact removes entries older than MaxDeltaAge.
func (dl *DeltaLog) Compact() {
	dl.mu.Lock()
	defer dl.mu.Unlock()

	cutoff := time.Now().Add(-dl.config.MaxDeltaAge).UnixNano()
	var kept []*DeltaEntry
	removed := 0
	for _, e := range dl.entries {
		if e.Timestamp >= cutoff {
			kept = append(kept, e)
		} else {
			removed++
		}
	}
	dl.entries = kept
	dl.compactedEntries += removed
}

// Stats returns delta log statistics.
func (dl *DeltaLog) Stats() *DeltaStats {
	dl.mu.RLock()
	defer dl.mu.RUnlock()

	s := &DeltaStats{
		TotalEntries:     len(dl.entries),
		CompactedEntries: dl.compactedEntries,
	}
	if len(dl.entries) > 0 {
		s.OldestEntry = time.Unix(0, dl.entries[0].Timestamp)
		s.NewestEntry = time.Unix(0, dl.entries[len(dl.entries)-1].Timestamp)
		s.AvgBatchSize = len(dl.entries)
	}
	return s
}
