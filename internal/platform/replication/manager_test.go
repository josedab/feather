package replication

import (
	"testing"
	"time"
)

func TestVectorClock_Merge(t *testing.T) {
	vc1 := VectorClock{"a": 2, "b": 1}
	vc2 := VectorClock{"a": 1, "b": 3, "c": 1}
	merged := vc1.Merge(vc2)

	if merged["a"] != 2 || merged["b"] != 3 || merged["c"] != 1 {
		t.Fatalf("unexpected merge: %v", merged)
	}
}

func TestVectorClock_HappensBefore(t *testing.T) {
	vc1 := VectorClock{"a": 1, "b": 1}
	vc2 := VectorClock{"a": 2, "b": 1}

	if !vc1.HappensBefore(vc2) {
		t.Fatal("vc1 should happen before vc2")
	}
	if vc2.HappensBefore(vc1) {
		t.Fatal("vc2 should not happen before vc1")
	}
}

func TestVectorClock_IsConcurrent(t *testing.T) {
	vc1 := VectorClock{"a": 2, "b": 1}
	vc2 := VectorClock{"a": 1, "b": 2}

	if !vc1.IsConcurrent(vc2) {
		t.Fatal("clocks should be concurrent")
	}
}

func TestManager_AddRemoveRegion(t *testing.T) {
	m := NewManager(DefaultManagerConfig())

	r := &Region{ID: "us-east-1", Name: "US East", Endpoint: "http://us-east:8080"}
	if err := m.AddRegion(r); err != nil {
		t.Fatal(err)
	}

	if err := m.AddRegion(r); err != ErrRegionExists {
		t.Fatalf("expected ErrRegionExists, got %v", err)
	}

	got, err := m.GetRegion("us-east-1")
	if err != nil || got.Name != "US East" {
		t.Fatalf("get region failed: %v", err)
	}

	list := m.ListRegions()
	if len(list) != 1 {
		t.Fatalf("expected 1, got %d", len(list))
	}

	if err := m.RemoveRegion("us-east-1"); err != nil {
		t.Fatal(err)
	}

	if err := m.RemoveRegion("nonexistent"); err != ErrRegionNotFound {
		t.Fatalf("expected not found, got %v", err)
	}
}

func TestManager_WriteAndRead(t *testing.T) {
	m := NewManager(ManagerConfig{LocalRegion: "us-east"})

	rv, err := m.Write("user:123:clicks", 42)
	if err != nil {
		t.Fatal(err)
	}
	if rv.Origin != "us-east" {
		t.Fatalf("expected origin us-east, got %s", rv.Origin)
	}
	if rv.Clock["us-east"] != 1 {
		t.Fatalf("expected clock 1, got %d", rv.Clock["us-east"])
	}

	got, err := m.Read("user:123:clicks")
	if err != nil {
		t.Fatal(err)
	}
	if got.Value != 42 {
		t.Fatalf("expected 42, got %v", got.Value)
	}

	// Write again - clock should increment
	rv2, _ := m.Write("user:123:clicks", 43)
	if rv2.Clock["us-east"] != 2 {
		t.Fatalf("expected clock 2, got %d", rv2.Clock["us-east"])
	}
}

func TestManager_ReceiveReplica_NewerWins(t *testing.T) {
	m := NewManager(ManagerConfig{LocalRegion: "us-east", ConflictPolicy: PolicyLastWriterWins})

	// Write local
	m.Write("key1", "local-value")

	// Receive newer replica
	incoming := &ReplicatedValue{
		Value:     "remote-value",
		Clock:     VectorClock{"us-east": 1, "eu-west": 1},
		Origin:    "eu-west",
		Timestamp: time.Now().Add(time.Second),
	}

	if err := m.ReceiveReplica("key1", incoming); err != nil {
		t.Fatal(err)
	}

	got, _ := m.Read("key1")
	if got.Value != "remote-value" {
		t.Fatalf("expected remote-value, got %v", got.Value)
	}
}

func TestManager_ReceiveReplica_ConcurrentLWW(t *testing.T) {
	m := NewManager(ManagerConfig{LocalRegion: "us-east", ConflictPolicy: PolicyLastWriterWins})

	// Write local value
	local := &ReplicatedValue{
		Value:     "local",
		Clock:     VectorClock{"us-east": 1},
		Origin:    "us-east",
		Timestamp: time.Now(),
	}
	m.mu.Lock()
	m.values["key1"] = local
	m.mu.Unlock()

	// Receive concurrent remote value with later timestamp
	remote := &ReplicatedValue{
		Value:     "remote",
		Clock:     VectorClock{"eu-west": 1},
		Origin:    "eu-west",
		Timestamp: time.Now().Add(time.Second),
	}

	m.ReceiveReplica("key1", remote)

	got, _ := m.Read("key1")
	if got.Value != "remote" {
		t.Fatalf("LWW should pick remote (later timestamp), got %v", got.Value)
	}
}

func TestManager_DrainActivate(t *testing.T) {
	m := NewManager(DefaultManagerConfig())
	m.AddRegion(&Region{ID: "r1"})

	if err := m.DrainRegion("r1"); err != nil {
		t.Fatal(err)
	}
	r, _ := m.GetRegion("r1")
	if r.Status != RegionDraining {
		t.Fatalf("expected draining, got %s", r.Status)
	}

	if err := m.ActivateRegion("r1"); err != nil {
		t.Fatal(err)
	}
	r, _ = m.GetRegion("r1")
	if r.Status != RegionActive {
		t.Fatalf("expected active, got %s", r.Status)
	}
}

func TestManager_PendingEvents(t *testing.T) {
	m := NewManager(ManagerConfig{LocalRegion: "us-east"})
	m.AddRegion(&Region{ID: "eu-west", Status: RegionActive})

	m.Write("key1", "value1")

	pending := m.GetPendingEvents()
	if len(pending) != 1 {
		t.Fatalf("expected 1 pending, got %d", len(pending))
	}

	m.MarkDelivered("eu-west")
	pending = m.GetPendingEvents()
	if len(pending) != 0 {
		t.Fatalf("expected 0 pending after delivery, got %d", len(pending))
	}
}

func TestManager_Stats(t *testing.T) {
	m := NewManager(ManagerConfig{LocalRegion: "us-east"})
	m.AddRegion(&Region{ID: "eu-west"})
	m.Write("k1", "v1")

	stats := m.Stats()
	if stats["local_region"] != "us-east" {
		t.Fatal("unexpected local region")
	}
	if stats["total_keys"] != 1 {
		t.Fatalf("expected 1 key, got %v", stats["total_keys"])
	}
}
