package consensus

import (
	"fmt"
	"testing"
	"time"
)

func TestDefaultWALConfig(t *testing.T) {
	cfg := DefaultWALConfig()
	if cfg.SegmentSize <= 0 {
		t.Error("expected positive segment size")
	}
	if cfg.SyncMode == "" {
		t.Error("expected non-empty sync mode")
	}
	if cfg.RetentionCount <= 0 {
		t.Error("expected positive retention count")
	}
	if cfg.SyncInterval <= 0 {
		t.Error("expected positive sync interval")
	}
}

func TestDefaultSnapshotConfig(t *testing.T) {
	cfg := DefaultSnapshotConfig()
	if cfg.Interval <= 0 {
		t.Error("expected positive interval")
	}
	if cfg.MaxSnapshots <= 0 {
		t.Error("expected positive max snapshots")
	}
}

func TestWAL_AppendAndRead(t *testing.T) {
	tests := []struct {
		name      string
		entries   []*WALEntry
		readStart uint64
		readEnd   uint64
		wantCount int
		wantErr   bool
		appendErr bool
	}{
		{
			name: "single entry",
			entries: []*WALEntry{
				{Index: 1, Term: 1, Type: EntryTypeCommand, Data: []byte("cmd1")},
			},
			readStart: 1, readEnd: 1, wantCount: 1,
		},
		{
			name: "multiple entries",
			entries: []*WALEntry{
				{Index: 1, Term: 1, Type: EntryTypeCommand, Data: []byte("cmd1")},
				{Index: 2, Term: 1, Type: EntryTypeCommand, Data: []byte("cmd2")},
				{Index: 3, Term: 2, Type: EntryTypeConfig, Data: []byte("cfg1")},
			},
			readStart: 1, readEnd: 3, wantCount: 3,
		},
		{
			name: "partial read",
			entries: []*WALEntry{
				{Index: 1, Term: 1, Type: EntryTypeCommand, Data: []byte("cmd1")},
				{Index: 2, Term: 1, Type: EntryTypeCommand, Data: []byte("cmd2")},
				{Index: 3, Term: 1, Type: EntryTypeCommand, Data: []byte("cmd3")},
			},
			readStart: 2, readEnd: 3, wantCount: 2,
		},
		{
			name: "read beyond range",
			entries: []*WALEntry{
				{Index: 1, Term: 1, Type: EntryTypeCommand, Data: []byte("cmd1")},
			},
			readStart: 5, readEnd: 10, wantCount: 0,
		},
		{
			name:      "invalid range",
			entries:   []*WALEntry{},
			readStart: 5, readEnd: 1, wantErr: true,
		},
		{
			name:      "nil entry",
			entries:   []*WALEntry{nil},
			appendErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wal := NewWAL(DefaultWALConfig())

			var appendFailed bool
			for _, e := range tt.entries {
				if err := wal.Append(e); err != nil {
					if !tt.appendErr {
						t.Fatalf("unexpected append error: %v", err)
					}
					appendFailed = true
					break
				}
			}
			if tt.appendErr {
				if !appendFailed {
					t.Fatal("expected append error")
				}
				return
			}

			result, err := wal.Read(tt.readStart, tt.readEnd)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected read error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected read error: %v", err)
			}
			if len(result) != tt.wantCount {
				t.Errorf("expected %d entries, got %d", tt.wantCount, len(result))
			}
		})
	}
}

func TestWAL_Truncate(t *testing.T) {
	tests := []struct {
		name       string
		entries    []*WALEntry
		truncateAt uint64
		wantRemain int
		wantFirst  uint64
	}{
		{
			name: "truncate first half",
			entries: []*WALEntry{
				{Index: 1, Term: 1, Data: []byte("a")},
				{Index: 2, Term: 1, Data: []byte("b")},
				{Index: 3, Term: 1, Data: []byte("c")},
				{Index: 4, Term: 1, Data: []byte("d")},
			},
			truncateAt: 2, wantRemain: 2, wantFirst: 3,
		},
		{
			name: "truncate all",
			entries: []*WALEntry{
				{Index: 1, Term: 1, Data: []byte("a")},
				{Index: 2, Term: 1, Data: []byte("b")},
			},
			truncateAt: 5, wantRemain: 0, wantFirst: 0,
		},
		{
			name: "truncate none",
			entries: []*WALEntry{
				{Index: 5, Term: 1, Data: []byte("e")},
				{Index: 6, Term: 1, Data: []byte("f")},
			},
			truncateAt: 1, wantRemain: 2, wantFirst: 5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wal := NewWAL(DefaultWALConfig())
			for _, e := range tt.entries {
				if err := wal.Append(e); err != nil {
					t.Fatalf("append: %v", err)
				}
			}

			if err := wal.Truncate(tt.truncateAt); err != nil {
				t.Fatalf("truncate: %v", err)
			}

			stats := wal.Stats()
			if stats.EntryCount != tt.wantRemain {
				t.Errorf("expected %d entries remaining, got %d", tt.wantRemain, stats.EntryCount)
			}
			if stats.FirstIndex != tt.wantFirst {
				t.Errorf("expected first index %d, got %d", tt.wantFirst, stats.FirstIndex)
			}
		})
	}
}

func TestWAL_SyncAndClose(t *testing.T) {
	wal := NewWAL(DefaultWALConfig())

	if err := wal.Sync(); err != nil {
		t.Fatalf("sync failed: %v", err)
	}

	stats := wal.Stats()
	if stats.SyncCount != 1 {
		t.Errorf("expected 1 sync, got %d", stats.SyncCount)
	}
	if stats.LastSyncTime.IsZero() {
		t.Error("expected non-zero last sync time")
	}

	if err := wal.Close(); err != nil {
		t.Fatalf("close failed: %v", err)
	}

	// Operations on closed WAL should fail
	if err := wal.Append(&WALEntry{Index: 1, Data: []byte("x")}); err == nil {
		t.Error("expected error appending to closed WAL")
	}
	if _, err := wal.Read(1, 1); err == nil {
		t.Error("expected error reading closed WAL")
	}
	if err := wal.Truncate(1); err == nil {
		t.Error("expected error truncating closed WAL")
	}
	if err := wal.Sync(); err == nil {
		t.Error("expected error syncing closed WAL")
	}
	if err := wal.Close(); err == nil {
		t.Error("expected error double-closing WAL")
	}
}

func TestWAL_Retention(t *testing.T) {
	cfg := DefaultWALConfig()
	cfg.RetentionCount = 3
	wal := NewWAL(cfg)

	for i := uint64(1); i <= 5; i++ {
		if err := wal.Append(&WALEntry{Index: i, Term: 1, Data: []byte("x")}); err != nil {
			t.Fatalf("append: %v", err)
		}
	}

	stats := wal.Stats()
	if stats.EntryCount != 3 {
		t.Errorf("expected 3 entries after retention, got %d", stats.EntryCount)
	}
	if stats.FirstIndex != 3 {
		t.Errorf("expected first index 3, got %d", stats.FirstIndex)
	}
	if stats.LastIndex != 5 {
		t.Errorf("expected last index 5, got %d", stats.LastIndex)
	}
}

func TestWAL_Stats(t *testing.T) {
	wal := NewWAL(DefaultWALConfig())
	for i := uint64(1); i <= 3; i++ {
		_ = wal.Append(&WALEntry{Index: i, Term: 1, Data: []byte(fmt.Sprintf("data-%d", i))})
	}

	stats := wal.Stats()
	if stats.EntryCount != 3 {
		t.Errorf("expected 3 entries, got %d", stats.EntryCount)
	}
	if stats.FirstIndex != 1 {
		t.Errorf("expected first index 1, got %d", stats.FirstIndex)
	}
	if stats.LastIndex != 3 {
		t.Errorf("expected last index 3, got %d", stats.LastIndex)
	}
	if stats.BytesWritten <= 0 {
		t.Error("expected positive bytes written")
	}
}

func TestWAL_ImmediateSync(t *testing.T) {
	cfg := DefaultWALConfig()
	cfg.SyncMode = SyncModeImmediate
	wal := NewWAL(cfg)

	_ = wal.Append(&WALEntry{Index: 1, Term: 1, Data: []byte("x")})
	_ = wal.Append(&WALEntry{Index: 2, Term: 1, Data: []byte("y")})

	stats := wal.Stats()
	if stats.SyncCount != 2 {
		t.Errorf("expected 2 immediate syncs, got %d", stats.SyncCount)
	}
}

func TestSnapshotManager_TakeAndRestore(t *testing.T) {
	tests := []struct {
		name    string
		index   uint64
		term    uint64
		data    []byte
		wantErr bool
	}{
		{name: "valid snapshot", index: 100, term: 5, data: []byte(`{"state":"ok"}`)},
		{name: "empty data", index: 1, term: 1, data: []byte{}},
		{name: "nil data", index: 1, term: 1, data: nil, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sm := NewSnapshotManager(DefaultSnapshotConfig())

			meta, err := sm.Take(tt.index, tt.term, tt.data)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if meta.Index != tt.index {
				t.Errorf("expected index %d, got %d", tt.index, meta.Index)
			}
			if meta.Term != tt.term {
				t.Errorf("expected term %d, got %d", tt.term, meta.Term)
			}

			restored, err := sm.Restore(meta.ID)
			if err != nil {
				t.Fatalf("restore failed: %v", err)
			}
			if string(restored) != string(tt.data) {
				t.Errorf("restored data mismatch: got %s", restored)
			}
		})
	}
}

func TestSnapshotManager_RestoreNotFound(t *testing.T) {
	sm := NewSnapshotManager(DefaultSnapshotConfig())
	_, err := sm.Restore("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent snapshot")
	}
}

func TestSnapshotManager_List(t *testing.T) {
	sm := NewSnapshotManager(DefaultSnapshotConfig())

	for i := uint64(1); i <= 3; i++ {
		_, err := sm.Take(i*10, i, []byte(fmt.Sprintf("snap-%d", i)))
		if err != nil {
			t.Fatalf("take: %v", err)
		}
	}

	list := sm.List()
	if len(list) != 3 {
		t.Fatalf("expected 3 snapshots, got %d", len(list))
	}

	// Newest first
	if list[0].Index != 30 {
		t.Errorf("expected newest snapshot first (index 30), got %d", list[0].Index)
	}
	if list[2].Index != 10 {
		t.Errorf("expected oldest snapshot last (index 10), got %d", list[2].Index)
	}
}

func TestSnapshotManager_MaxSnapshots(t *testing.T) {
	cfg := DefaultSnapshotConfig()
	cfg.MaxSnapshots = 2
	sm := NewSnapshotManager(cfg)

	for i := uint64(1); i <= 4; i++ {
		_, _ = sm.Take(i*10, i, []byte("data"))
	}

	list := sm.List()
	if len(list) != 2 {
		t.Errorf("expected 2 snapshots after max enforcement, got %d", len(list))
	}
}

func TestSnapshotManager_Cleanup(t *testing.T) {
	cfg := DefaultSnapshotConfig()
	cfg.MaxSnapshots = 2
	sm := NewSnapshotManager(cfg)

	// Cleanup with no excess
	removed := sm.Cleanup()
	if removed != 0 {
		t.Errorf("expected 0 removed, got %d", removed)
	}

	// MaxSnapshots is enforced in Take, so Cleanup should also enforce
	// Force-insert snapshots beyond limit by temporarily using a higher max
	cfg2 := DefaultSnapshotConfig()
	cfg2.MaxSnapshots = 100
	sm2 := NewSnapshotManager(cfg2)
	for i := uint64(1); i <= 5; i++ {
		_, _ = sm2.Take(i, 1, []byte("x"))
	}
	sm2.config.MaxSnapshots = 2
	removed = sm2.Cleanup()
	if removed != 3 {
		t.Errorf("expected 3 removed, got %d", removed)
	}
	if len(sm2.List()) != 2 {
		t.Errorf("expected 2 remaining, got %d", len(sm2.List()))
	}
}

func TestMultiRegionCoordinator_RegisterAndList(t *testing.T) {
	c := NewMultiRegionCoordinator()

	if err := c.RegisterRegion("us-east-1", true); err != nil {
		t.Fatalf("register: %v", err)
	}
	if err := c.RegisterRegion("eu-west-1", false); err != nil {
		t.Fatalf("register: %v", err)
	}

	regions := c.ListRegions()
	if len(regions) != 2 {
		t.Fatalf("expected 2 regions, got %d", len(regions))
	}
	if c.GetPrimary() != "us-east-1" {
		t.Errorf("expected primary us-east-1, got %s", c.GetPrimary())
	}

	// Duplicate registration should fail
	if err := c.RegisterRegion("us-east-1", false); err == nil {
		t.Error("expected error for duplicate region")
	}

	// Empty ID should fail
	if err := c.RegisterRegion("", false); err == nil {
		t.Error("expected error for empty region ID")
	}
}

func TestMultiRegionCoordinator_UpdateLag(t *testing.T) {
	c := NewMultiRegionCoordinator()
	_ = c.RegisterRegion("us-east-1", true)
	_ = c.RegisterRegion("eu-west-1", false)

	if err := c.UpdateLag("eu-west-1", 50, 2*time.Second); err != nil {
		t.Fatalf("update lag: %v", err)
	}

	// Unknown region should fail
	if err := c.UpdateLag("ap-south-1", 10, time.Second); err == nil {
		t.Error("expected error for unknown region")
	}
}

func TestMultiRegionCoordinator_UpdateHealth(t *testing.T) {
	c := NewMultiRegionCoordinator()
	_ = c.RegisterRegion("us-east-1", true)

	if err := c.UpdateHealth("us-east-1", RegionDegraded); err != nil {
		t.Fatalf("update health: %v", err)
	}

	regions := c.ListRegions()
	for _, r := range regions {
		if r.ID == "us-east-1" && r.Health != RegionDegraded {
			t.Errorf("expected degraded, got %s", r.Health)
		}
	}

	// Unknown region should fail
	if err := c.UpdateHealth("unknown", RegionHealthy); err == nil {
		t.Error("expected error for unknown region")
	}
}

func TestMultiRegionCoordinator_Failover(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(c *MultiRegionCoordinator)
		wantID  string
		wantErr bool
	}{
		{
			name: "failover to healthy replica",
			setup: func(c *MultiRegionCoordinator) {
				_ = c.RegisterRegion("us-east-1", true)
				_ = c.RegisterRegion("eu-west-1", false)
				_ = c.RegisterRegion("ap-south-1", false)
				_ = c.UpdateLag("eu-west-1", 100, 5*time.Second)
				_ = c.UpdateLag("ap-south-1", 10, time.Second)
			},
			wantID: "ap-south-1",
		},
		{
			name: "no healthy replicas",
			setup: func(c *MultiRegionCoordinator) {
				_ = c.RegisterRegion("us-east-1", true)
				_ = c.RegisterRegion("eu-west-1", false)
				_ = c.UpdateHealth("eu-west-1", RegionUnhealthy)
			},
			wantErr: true,
		},
		{
			name: "only primary exists",
			setup: func(c *MultiRegionCoordinator) {
				_ = c.RegisterRegion("us-east-1", true)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewMultiRegionCoordinator()
			tt.setup(c)

			newPrimary, err := c.Failover()
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected failover error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if newPrimary != tt.wantID {
				t.Errorf("expected new primary %s, got %s", tt.wantID, newPrimary)
			}
			if c.GetPrimary() != tt.wantID {
				t.Errorf("expected GetPrimary() = %s, got %s", tt.wantID, c.GetPrimary())
			}
		})
	}
}
