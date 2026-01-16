package edgeruntime

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDeltaLog_RecordAndGet(t *testing.T) {
	dl := NewDeltaLog(DefaultDeltaSyncConfig())

	now := time.Now().UnixNano()
	dl.RecordDelta(&DeltaEntry{
		EntityKey:  "user:1",
		FeatureKey: "clicks",
		NewValue:   42,
		Timestamp:  now,
		Operation:  "put",
	})

	entries := dl.GetDeltasSince(now)
	assert.Len(t, entries, 1)
	assert.Equal(t, "user:1", entries[0].EntityKey)
	assert.Equal(t, 42, entries[0].NewValue)
}

func TestDeltaLog_GetDeltasSinceFilters(t *testing.T) {
	dl := NewDeltaLog(DefaultDeltaSyncConfig())

	dl.RecordDelta(&DeltaEntry{EntityKey: "a", Timestamp: 100, Operation: "put"})
	dl.RecordDelta(&DeltaEntry{EntityKey: "b", Timestamp: 200, Operation: "put"})
	dl.RecordDelta(&DeltaEntry{EntityKey: "c", Timestamp: 300, Operation: "put"})

	entries := dl.GetDeltasSince(200)
	assert.Len(t, entries, 2)
	assert.Equal(t, "b", entries[0].EntityKey)
	assert.Equal(t, "c", entries[1].EntityKey)
}

func TestDeltaLog_GetDeltasForDevice(t *testing.T) {
	dl := NewDeltaLog(DefaultDeltaSyncConfig())

	dl.RecordDelta(&DeltaEntry{EntityKey: "a", Timestamp: 100, Operation: "put"})
	dl.RecordDelta(&DeltaEntry{EntityKey: "b", Timestamp: 200, Operation: "delete"})

	entries := dl.GetDeltasForDevice("device-1", 200)
	assert.Len(t, entries, 1)
	assert.Equal(t, "b", entries[0].EntityKey)
}

func TestDeltaLog_AutoTimestamp(t *testing.T) {
	dl := NewDeltaLog(DefaultDeltaSyncConfig())

	dl.RecordDelta(&DeltaEntry{EntityKey: "a", Operation: "put"})

	entries := dl.GetDeltasSince(0)
	assert.Len(t, entries, 1)
	assert.NotZero(t, entries[0].Timestamp)
}

func TestDeltaLog_Compact(t *testing.T) {
	cfg := DefaultDeltaSyncConfig()
	cfg.MaxDeltaAge = 1 * time.Millisecond
	dl := NewDeltaLog(cfg)

	dl.RecordDelta(&DeltaEntry{EntityKey: "old", Timestamp: 1, Operation: "put"})
	time.Sleep(5 * time.Millisecond)
	now := time.Now().UnixNano()
	dl.RecordDelta(&DeltaEntry{EntityKey: "new", Timestamp: now, Operation: "put"})

	dl.Compact()

	entries := dl.GetDeltasSince(0)
	assert.Len(t, entries, 1)
	assert.Equal(t, "new", entries[0].EntityKey)

	stats := dl.Stats()
	assert.Equal(t, 1, stats.CompactedEntries)
}

func TestDeltaLog_Stats(t *testing.T) {
	dl := NewDeltaLog(DefaultDeltaSyncConfig())

	dl.RecordDelta(&DeltaEntry{EntityKey: "a", Timestamp: 100, Operation: "put"})
	dl.RecordDelta(&DeltaEntry{EntityKey: "b", Timestamp: 200, Operation: "put"})

	s := dl.Stats()
	assert.Equal(t, 2, s.TotalEntries)
	assert.Equal(t, 0, s.CompactedEntries)
	assert.Equal(t, time.Unix(0, 100), s.OldestEntry)
	assert.Equal(t, time.Unix(0, 200), s.NewestEntry)
}

func TestDeltaLog_StatsEmpty(t *testing.T) {
	dl := NewDeltaLog(DefaultDeltaSyncConfig())
	s := dl.Stats()
	assert.Equal(t, 0, s.TotalEntries)
}

func TestDefaultDeltaSyncConfig(t *testing.T) {
	cfg := DefaultDeltaSyncConfig()
	assert.Equal(t, 1000, cfg.MaxBatchSize)
	assert.True(t, cfg.CompressionEnabled)
	assert.Equal(t, 0.1, cfg.DeltaThreshold)
	assert.Equal(t, 24*time.Hour, cfg.MaxDeltaAge)
}
