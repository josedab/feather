package auditlog

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewLogger(t *testing.T) {
	cfg := DefaultLoggerConfig()
	l := NewLogger(cfg)
	require.NotNil(t, l)
	assert.Equal(t, 10000000, l.config.MaxEntries)
	assert.Len(t, l.config.EnabledActions, 6)
}

func TestLog(t *testing.T) {
	l := NewLogger(DefaultLoggerConfig())

	entries := []AuditEntry{
		{ID: "a1", Action: ActionRead, Actor: "alice", Resource: "user_age", Timestamp: time.Now()},
		{ID: "a2", Action: ActionWrite, Actor: "bob", Resource: "user_score", Timestamp: time.Now()},
		{ID: "a3", Action: ActionDelete, Actor: "carol", Resource: "old_feature", Timestamp: time.Now()},
	}

	for _, e := range entries {
		require.NoError(t, l.Log(e))
	}

	stats := l.Stats()
	assert.Equal(t, int64(3), stats.TotalEntries)
	assert.Equal(t, int64(3), stats.TotalLogged)
}

func TestQuery(t *testing.T) {
	l := NewLogger(DefaultLoggerConfig())

	now := time.Now()
	require.NoError(t, l.Log(AuditEntry{ID: "q1", Action: ActionRead, Actor: "alice", Timestamp: now}))
	require.NoError(t, l.Log(AuditEntry{ID: "q2", Action: ActionWrite, Actor: "bob", Timestamp: now}))
	require.NoError(t, l.Log(AuditEntry{ID: "q3", Action: ActionRead, Actor: "carol", Timestamp: now}))

	reads := l.Query(QueryFilter{Action: ActionRead})
	assert.Len(t, reads, 2)
	for _, e := range reads {
		assert.Equal(t, ActionRead, e.Action)
	}
}

func TestQueryByActor(t *testing.T) {
	l := NewLogger(DefaultLoggerConfig())

	require.NoError(t, l.Log(AuditEntry{ID: "b1", Action: ActionRead, Actor: "alice", Timestamp: time.Now()}))
	require.NoError(t, l.Log(AuditEntry{ID: "b2", Action: ActionWrite, Actor: "bob", Timestamp: time.Now()}))
	require.NoError(t, l.Log(AuditEntry{ID: "b3", Action: ActionRead, Actor: "alice", Timestamp: time.Now()}))

	aliceEntries := l.Query(QueryFilter{Actor: "alice"})
	assert.Len(t, aliceEntries, 2)
}

func TestQueryByTimeRange(t *testing.T) {
	l := NewLogger(DefaultLoggerConfig())

	t1 := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	t2 := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)
	t3 := time.Date(2024, 12, 1, 0, 0, 0, 0, time.UTC)

	require.NoError(t, l.Log(AuditEntry{ID: "t1", Action: ActionRead, Timestamp: t1}))
	require.NoError(t, l.Log(AuditEntry{ID: "t2", Action: ActionWrite, Timestamp: t2}))
	require.NoError(t, l.Log(AuditEntry{ID: "t3", Action: ActionDelete, Timestamp: t3}))

	results := l.Query(QueryFilter{
		StartTime: time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC),
		EndTime:   time.Date(2024, 9, 1, 0, 0, 0, 0, time.UTC),
	})
	assert.Len(t, results, 1)
	assert.Equal(t, "t2", results[0].ID)
}

func TestExportJSON(t *testing.T) {
	l := NewLogger(DefaultLoggerConfig())

	require.NoError(t, l.Log(AuditEntry{ID: "j1", Action: ActionRead, Actor: "alice", Timestamp: time.Now()}))
	require.NoError(t, l.Log(AuditEntry{ID: "j2", Action: ActionWrite, Actor: "bob", Timestamp: time.Now()}))

	out, err := l.Export(QueryFilter{}, ExportJSON)
	require.NoError(t, err)

	var entries []AuditEntry
	require.NoError(t, json.Unmarshal([]byte(out), &entries))
	assert.Len(t, entries, 2)
}

func TestExportCSV(t *testing.T) {
	l := NewLogger(DefaultLoggerConfig())

	require.NoError(t, l.Log(AuditEntry{ID: "c1", Action: ActionRead, Actor: "alice", Timestamp: time.Now()}))

	out, err := l.Export(QueryFilter{}, ExportCSV)
	require.NoError(t, err)

	lines := strings.Split(strings.TrimSpace(out), "\n")
	assert.Len(t, lines, 2) // header + 1 entry
	assert.True(t, strings.HasPrefix(lines[0], "id,"))
}

func TestPurge(t *testing.T) {
	l := NewLogger(DefaultLoggerConfig())

	old := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
	recent := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)

	require.NoError(t, l.Log(AuditEntry{ID: "p1", Action: ActionRead, Timestamp: old}))
	require.NoError(t, l.Log(AuditEntry{ID: "p2", Action: ActionWrite, Timestamp: old}))
	require.NoError(t, l.Log(AuditEntry{ID: "p3", Action: ActionRead, Timestamp: recent}))

	cutoff := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	purged := l.Purge(cutoff)
	assert.Equal(t, 2, purged)
	assert.Equal(t, int64(1), l.Stats().TotalEntries)
}

func TestStats(t *testing.T) {
	l := NewLogger(DefaultLoggerConfig())

	now := time.Now()
	require.NoError(t, l.Log(AuditEntry{ID: "s1", Action: ActionRead, Timestamp: now.Add(-time.Hour)}))
	require.NoError(t, l.Log(AuditEntry{ID: "s2", Action: ActionRead, Timestamp: now}))
	require.NoError(t, l.Log(AuditEntry{ID: "s3", Action: ActionWrite, Timestamp: now}))

	stats := l.Stats()
	assert.Equal(t, int64(3), stats.TotalEntries)
	assert.Equal(t, int64(3), stats.TotalLogged)
	assert.Equal(t, int64(2), stats.ActionCounts["read"])
	assert.Equal(t, int64(1), stats.ActionCounts["write"])
	assert.False(t, stats.OldestEntry.IsZero())
	assert.False(t, stats.NewestEntry.IsZero())
}

func TestFilePersistence(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "audit.log")

	cfg := DefaultLoggerConfig()
	cfg.FilePath = fp
	l := NewLogger(cfg)
	defer l.Close()

	now := time.Now()
	require.NoError(t, l.Log(AuditEntry{ID: "f1", Action: ActionRead, Actor: "alice", Timestamp: now}))
	require.NoError(t, l.Log(AuditEntry{ID: "f2", Action: ActionWrite, Actor: "bob", Timestamp: now}))

	// Verify in-memory still works
	assert.Equal(t, int64(2), l.Stats().TotalEntries)

	// Verify file content
	f, err := os.Open(fp)
	require.NoError(t, err)
	defer f.Close()

	scanner := bufio.NewScanner(f)
	var entries []AuditEntry
	for scanner.Scan() {
		var e AuditEntry
		require.NoError(t, json.Unmarshal(scanner.Bytes(), &e))
		entries = append(entries, e)
	}
	require.NoError(t, scanner.Err())
	assert.Len(t, entries, 2)
	assert.Equal(t, "f1", entries[0].ID)
	assert.Equal(t, "f2", entries[1].ID)
}

func TestFilePersistenceBadPath(t *testing.T) {
	cfg := DefaultLoggerConfig()
	cfg.FilePath = "/nonexistent/dir/audit.log"
	l := NewLogger(cfg)
	defer l.Close()

	// Should still work in-memory
	require.NoError(t, l.Log(AuditEntry{ID: "b1", Action: ActionRead, Timestamp: time.Now()}))
	assert.Equal(t, int64(1), l.Stats().TotalEntries)
}

func TestClose(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "audit.log")

	cfg := DefaultLoggerConfig()
	cfg.FilePath = fp
	l := NewLogger(cfg)
	require.NoError(t, l.Close())

	// Close again is safe
	require.NoError(t, l.Close())
}

func TestGetEntry(t *testing.T) {
	t.Parallel()
	l := NewLogger(DefaultLoggerConfig())
	defer l.Close()

	entry := AuditEntry{
		ID:       "entry-1",
		Action:   ActionRead,
		Actor:    "user:admin",
		Resource: "feature:clicks",
	}
	require.NoError(t, l.Log(entry))

	got, err := l.GetEntry("entry-1")
	require.NoError(t, err)
	require.Equal(t, "user:admin", got.Actor)
	require.Equal(t, ActionRead, got.Action)
	require.Equal(t, "feature:clicks", got.Resource)

	// Nonexistent entry.
	_, err = l.GetEntry("nonexistent")
	require.Error(t, err)
}

func TestGetEntry_AutoID(t *testing.T) {
	t.Parallel()
	l := NewLogger(DefaultLoggerConfig())
	defer l.Close()

	require.NoError(t, l.Log(AuditEntry{
		Action:   ActionWrite,
		Actor:    "user:writer",
		Resource: "feature:revenue",
	}))

	got, err := l.GetEntry("audit-1")
	require.NoError(t, err)
	require.Equal(t, "user:writer", got.Actor)
}
