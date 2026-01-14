package edgeruntime

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRuntime_PutAndGet(t *testing.T) {
	rt := NewRuntime(DefaultRuntimeConfig())

	err := rt.Put("user:123", "clicks", 42)
	require.NoError(t, err)

	fv, err := rt.Get("user:123", "clicks")
	require.NoError(t, err)
	assert.Equal(t, 42, fv.Value)
	assert.True(t, fv.Dirty)
	assert.Equal(t, "local", fv.Source)
}

func TestRuntime_GetNotFound(t *testing.T) {
	rt := NewRuntime(DefaultRuntimeConfig())

	_, err := rt.Get("user:123", "clicks")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestRuntime_GetBatch(t *testing.T) {
	rt := NewRuntime(DefaultRuntimeConfig())
	_ = rt.Put("user:1", "clicks", 10)
	_ = rt.Put("user:1", "revenue", 99.5)
	_ = rt.Put("user:1", "age", 25)

	batch := rt.GetBatch("user:1", []string{"clicks", "revenue", "nonexistent"})
	assert.Len(t, batch, 2)
	assert.Equal(t, 10, batch["clicks"].Value)
	assert.Equal(t, 99.5, batch["revenue"].Value)
}

func TestRuntime_MaxEntities(t *testing.T) {
	cfg := DefaultRuntimeConfig()
	cfg.MaxEntities = 2
	rt := NewRuntime(cfg)

	_ = rt.Put("user:1", "clicks", 1)
	_ = rt.Put("user:2", "clicks", 2)
	err := rt.Put("user:3", "clicks", 3)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "max entities")
}

func TestRuntime_SyncQueue(t *testing.T) {
	rt := NewRuntime(DefaultRuntimeConfig())

	_ = rt.Put("user:1", "clicks", 10)
	_ = rt.Put("user:2", "clicks", 20)

	pending := rt.GetPendingSync()
	assert.Len(t, pending, 2)
	assert.Equal(t, "user:1", pending[0].EntityKey)
}

func TestRuntime_AcknowledgeSync(t *testing.T) {
	rt := NewRuntime(DefaultRuntimeConfig())
	_ = rt.Put("user:1", "clicks", 10)
	_ = rt.Put("user:2", "clicks", 20)

	rt.AcknowledgeSync(1)
	pending := rt.GetPendingSync()
	assert.Len(t, pending, 1)

	rt.AcknowledgeSync(10) // more than remaining
	pending = rt.GetPendingSync()
	assert.Empty(t, pending)

	stats := rt.Stats()
	assert.Equal(t, "synced", stats["sync_state"])
}

func TestRuntime_ApplyRemote(t *testing.T) {
	rt := NewRuntime(DefaultRuntimeConfig())

	applied := rt.ApplyRemote("user:1", "clicks", 42, time.Now().UnixNano())
	assert.True(t, applied)

	fv, err := rt.Get("user:1", "clicks")
	require.NoError(t, err)
	assert.Equal(t, 42, fv.Value)
	assert.Equal(t, "remote", fv.Source)
	assert.False(t, fv.Dirty)
}

func TestRuntime_ConflictLWW(t *testing.T) {
	cfg := DefaultRuntimeConfig()
	cfg.ConflictStrategy = ConflictLastWriteWins
	rt := NewRuntime(cfg)

	// Write locally
	_ = rt.Put("user:1", "clicks", 10)

	// Apply older remote - should not overwrite
	applied := rt.ApplyRemote("user:1", "clicks", 5, 1) // very old timestamp
	assert.False(t, applied)

	fv, _ := rt.Get("user:1", "clicks")
	assert.Equal(t, 10, fv.Value)
}

func TestRuntime_ConflictRemoteWins(t *testing.T) {
	cfg := DefaultRuntimeConfig()
	cfg.ConflictStrategy = ConflictRemoteWins
	rt := NewRuntime(cfg)

	_ = rt.Put("user:1", "clicks", 10)

	applied := rt.ApplyRemote("user:1", "clicks", 5, 1) // even old remote wins
	assert.True(t, applied)

	fv, _ := rt.Get("user:1", "clicks")
	assert.Equal(t, 5, fv.Value)
}

func TestRuntime_ConflictLocalWins(t *testing.T) {
	cfg := DefaultRuntimeConfig()
	cfg.ConflictStrategy = ConflictLocalWins
	rt := NewRuntime(cfg)

	_ = rt.Put("user:1", "clicks", 10)

	applied := rt.ApplyRemote("user:1", "clicks", 999, time.Now().UnixNano()+1e15)
	assert.False(t, applied)

	fv, _ := rt.Get("user:1", "clicks")
	assert.Equal(t, 10, fv.Value)
}

func TestRuntime_Clear(t *testing.T) {
	rt := NewRuntime(DefaultRuntimeConfig())
	_ = rt.Put("user:1", "clicks", 10)

	rt.Clear()
	assert.Equal(t, 0, rt.EntityCount())
}

func TestRuntime_Stats(t *testing.T) {
	rt := NewRuntime(DefaultRuntimeConfig())
	_ = rt.Put("user:1", "clicks", 10)
	_ = rt.Put("user:1", "revenue", 50.0)
	_ = rt.Put("user:2", "clicks", 20)

	stats := rt.Stats()
	assert.Equal(t, 2, stats["total_entities"])
	assert.Equal(t, 3, stats["total_features"])
	assert.Equal(t, 3, stats["dirty_features"])
	assert.Equal(t, 3, stats["pending_sync"])
}

func TestRuntime_RecordSyncResult(t *testing.T) {
	rt := NewRuntime(DefaultRuntimeConfig())
	rt.RecordSyncResult(SyncResult{
		PushedCount: 5, PulledCount: 3,
		Duration: 100 * time.Millisecond,
	})

	stats := rt.Stats()
	assert.Equal(t, 1, stats["sync_history"])
}

func TestRuntime_SetSyncState(t *testing.T) {
	rt := NewRuntime(DefaultRuntimeConfig())
	rt.SetSyncState(SyncStateSyncing)

	stats := rt.Stats()
	assert.Equal(t, "syncing", stats["sync_state"])
}

func TestDefaultRuntimeConfig(t *testing.T) {
	cfg := DefaultRuntimeConfig()
	assert.Equal(t, 1000, cfg.MaxFeatures)
	assert.Equal(t, 10000, cfg.MaxEntities)
	assert.Equal(t, ConflictLastWriteWins, cfg.ConflictStrategy)
}
