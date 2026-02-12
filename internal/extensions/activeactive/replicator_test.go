package activeactive

import (
	"testing"
	"time"
)

func TestNewReplicator(t *testing.T) {
	cfg := DefaultReplicatorConfig()
	r := NewReplicator(cfg)
	if r == nil {
		t.Fatal("NewReplicator returned nil")
	}
	if r.config.NodeID != "node-1" {
		t.Errorf("NodeID = %q, want %q", r.config.NodeID, "node-1")
	}
}

func TestDefaultReplicatorConfig(t *testing.T) {
	cfg := DefaultReplicatorConfig()
	if cfg.NodeID != "node-1" {
		t.Errorf("NodeID = %q, want %q", cfg.NodeID, "node-1")
	}
	if cfg.ConflictStrategy != LWW {
		t.Errorf("ConflictStrategy = %q, want %q", cfg.ConflictStrategy, LWW)
	}
	if cfg.GossipIntervalMs != 1000 {
		t.Errorf("GossipIntervalMs = %d, want 1000", cfg.GossipIntervalMs)
	}
	if cfg.MaxMessageQueueSize != 10000 {
		t.Errorf("MaxMessageQueueSize = %d, want 10000", cfg.MaxMessageQueueSize)
	}
}

func TestReplicator_AddPeer(t *testing.T) {
	r := NewReplicator(DefaultReplicatorConfig())

	peer := &Peer{ID: "peer-1", Address: "10.0.0.1:5000", Region: "us-east-1"}
	if err := r.AddPeer(peer); err != nil {
		t.Fatalf("AddPeer: %v", err)
	}

	if peer.Status != PeerActive {
		t.Errorf("Status = %q, want %q", peer.Status, PeerActive)
	}
	if peer.LastSeen.IsZero() {
		t.Error("LastSeen should be set")
	}
}

func TestReplicator_AddPeer_EmptyID(t *testing.T) {
	r := NewReplicator(DefaultReplicatorConfig())
	err := r.AddPeer(&Peer{ID: ""})
	if err == nil {
		t.Error("expected error for empty peer ID")
	}
}

func TestReplicator_AddPeer_Duplicate(t *testing.T) {
	r := NewReplicator(DefaultReplicatorConfig())
	_ = r.AddPeer(&Peer{ID: "peer-1", Address: "10.0.0.1:5000"})

	err := r.AddPeer(&Peer{ID: "peer-1", Address: "10.0.0.2:5000"})
	if err == nil {
		t.Error("expected error for duplicate peer")
	}
}

func TestReplicator_RemovePeer(t *testing.T) {
	r := NewReplicator(DefaultReplicatorConfig())
	_ = r.AddPeer(&Peer{ID: "peer-1", Address: "10.0.0.1:5000"})

	if err := r.RemovePeer("peer-1"); err != nil {
		t.Fatalf("RemovePeer: %v", err)
	}

	peers := r.ListPeers()
	if len(peers) != 0 {
		t.Errorf("expected 0 peers, got %d", len(peers))
	}
}

func TestReplicator_RemovePeer_NotFound(t *testing.T) {
	r := NewReplicator(DefaultReplicatorConfig())
	err := r.RemovePeer("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent peer")
	}
}

func TestReplicator_GetPeer(t *testing.T) {
	r := NewReplicator(DefaultReplicatorConfig())
	_ = r.AddPeer(&Peer{ID: "peer-1", Address: "10.0.0.1:5000", Region: "us-east-1"})

	peer, err := r.GetPeer("peer-1")
	if err != nil {
		t.Fatalf("GetPeer: %v", err)
	}
	if peer.Address != "10.0.0.1:5000" {
		t.Errorf("Address = %q, want %q", peer.Address, "10.0.0.1:5000")
	}
}

func TestReplicator_GetPeer_NotFound(t *testing.T) {
	r := NewReplicator(DefaultReplicatorConfig())
	_, err := r.GetPeer("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent peer")
	}
}

func TestReplicator_ListPeers(t *testing.T) {
	r := NewReplicator(DefaultReplicatorConfig())
	_ = r.AddPeer(&Peer{ID: "peer-1", Address: "10.0.0.1:5000"})
	_ = r.AddPeer(&Peer{ID: "peer-2", Address: "10.0.0.2:5000"})
	_ = r.AddPeer(&Peer{ID: "peer-3", Address: "10.0.0.3:5000"})

	peers := r.ListPeers()
	if len(peers) != 3 {
		t.Errorf("expected 3 peers, got %d", len(peers))
	}
}

func TestReplicator_VectorClock_Increment(t *testing.T) {
	cfg := DefaultReplicatorConfig()
	cfg.NodeID = "node-A"
	r := NewReplicator(cfg)
	_ = r.AddPeer(&Peer{ID: "peer-B", Address: "10.0.0.2:5000"})

	msg := &ReplicationMessage{
		Type:       MessagePut,
		TargetPeer: "peer-B",
		Payload:    map[string]string{"key": "value"},
	}
	if err := r.Replicate(msg); err != nil {
		t.Fatalf("Replicate: %v", err)
	}

	// Vector clock should have been incremented for node-A
	r.mu.RLock()
	clockVal := r.clock["node-A"]
	r.mu.RUnlock()

	if clockVal != 1 {
		t.Errorf("vector clock for node-A = %d, want 1", clockVal)
	}
}

func TestReplicator_VectorClock_Merge(t *testing.T) {
	cfg := DefaultReplicatorConfig()
	cfg.NodeID = "node-A"
	r := NewReplicator(cfg)
	_ = r.AddPeer(&Peer{ID: "node-B", Address: "10.0.0.2:5000"})

	// Receive a message with a higher vector clock from node-B
	msg := &ReplicationMessage{
		Type:        MessagePut,
		SourcePeer:  "node-B",
		VectorClock: map[string]uint64{"node-B": 5, "node-C": 3},
	}
	if err := r.Receive(msg); err != nil {
		t.Fatalf("Receive: %v", err)
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	// node-B's clock should be merged (max of local and incoming)
	if r.clock["node-B"] != 5 {
		t.Errorf("clock[node-B] = %d, want 5", r.clock["node-B"])
	}
	// node-C's clock should be merged from incoming
	if r.clock["node-C"] != 3 {
		t.Errorf("clock[node-C] = %d, want 3", r.clock["node-C"])
	}
	// node-A should be incremented by the receive
	if r.clock["node-A"] != 1 {
		t.Errorf("clock[node-A] = %d, want 1", r.clock["node-A"])
	}
}

func TestReplicator_VectorClock_CausalOrder(t *testing.T) {
	cfg := DefaultReplicatorConfig()
	cfg.NodeID = "node-A"
	r := NewReplicator(cfg)
	_ = r.AddPeer(&Peer{ID: "node-B", Address: "10.0.0.2:5000"})

	// Send two messages - clock should increment each time
	for i := 0; i < 3; i++ {
		msg := &ReplicationMessage{
			Type:       MessagePut,
			TargetPeer: "node-B",
			Payload:    map[string]string{"key": "value"},
		}
		if err := r.Replicate(msg); err != nil {
			t.Fatalf("Replicate %d: %v", i, err)
		}
	}

	r.mu.RLock()
	clockVal := r.clock["node-A"]
	r.mu.RUnlock()

	if clockVal != 3 {
		t.Errorf("vector clock after 3 sends = %d, want 3", clockVal)
	}
}

func TestReplicator_Replicate_MissingTargetPeer(t *testing.T) {
	r := NewReplicator(DefaultReplicatorConfig())
	err := r.Replicate(&ReplicationMessage{TargetPeer: ""})
	if err == nil {
		t.Error("expected error for empty target peer")
	}
}

func TestReplicator_Replicate_UnknownTargetPeer(t *testing.T) {
	r := NewReplicator(DefaultReplicatorConfig())
	err := r.Replicate(&ReplicationMessage{TargetPeer: "unknown"})
	if err == nil {
		t.Error("expected error for unknown target peer")
	}
}

func TestReplicator_Replicate_SetsDefaults(t *testing.T) {
	cfg := DefaultReplicatorConfig()
	cfg.NodeID = "node-A"
	r := NewReplicator(cfg)
	_ = r.AddPeer(&Peer{ID: "peer-B", Address: "10.0.0.2:5000"})

	msg := &ReplicationMessage{
		Type:       MessagePut,
		TargetPeer: "peer-B",
		Payload:    map[string]string{"key": "value"},
	}
	if err := r.Replicate(msg); err != nil {
		t.Fatalf("Replicate: %v", err)
	}

	if msg.SourcePeer != "node-A" {
		t.Errorf("SourcePeer = %q, want %q", msg.SourcePeer, "node-A")
	}
	if msg.Timestamp.IsZero() {
		t.Error("Timestamp should be set")
	}
}

func TestReplicator_Receive_MissingSourcePeer(t *testing.T) {
	r := NewReplicator(DefaultReplicatorConfig())
	err := r.Receive(&ReplicationMessage{SourcePeer: ""})
	if err == nil {
		t.Error("expected error for empty source peer")
	}
}

func TestReplicator_Receive_UpdatesPeerStatus(t *testing.T) {
	r := NewReplicator(DefaultReplicatorConfig())
	_ = r.AddPeer(&Peer{ID: "peer-B", Address: "10.0.0.2:5000", Status: PeerSuspect})

	before := time.Now()
	msg := &ReplicationMessage{
		Type:       MessagePut,
		SourcePeer: "peer-B",
	}
	if err := r.Receive(msg); err != nil {
		t.Fatalf("Receive: %v", err)
	}

	peer, _ := r.GetPeer("peer-B")
	if peer.Status != PeerActive {
		t.Errorf("Status = %q, want %q", peer.Status, PeerActive)
	}
	if peer.LastSeen.Before(before) {
		t.Error("LastSeen should be updated")
	}
}

func TestReplicator_AntiEntropy(t *testing.T) {
	r := NewReplicator(DefaultReplicatorConfig())
	_ = r.AddPeer(&Peer{ID: "peer-B", Address: "10.0.0.2:5000"})

	result, err := r.AntiEntropy("peer-B")
	if err != nil {
		t.Fatalf("AntiEntropy: %v", err)
	}

	if result.PeerID != "peer-B" {
		t.Errorf("PeerID = %q, want %q", result.PeerID, "peer-B")
	}
	if result.KeysCompared <= 0 {
		t.Error("KeysCompared should be positive")
	}
}

func TestReplicator_AntiEntropy_NotFound(t *testing.T) {
	r := NewReplicator(DefaultReplicatorConfig())
	_, err := r.AntiEntropy("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent peer")
	}
}

func TestReplicator_GetGossipState(t *testing.T) {
	r := NewReplicator(DefaultReplicatorConfig())
	_ = r.AddPeer(&Peer{ID: "peer-1", Status: PeerActive})
	_ = r.AddPeer(&Peer{ID: "peer-2", Status: PeerSuspect})
	_ = r.AddPeer(&Peer{ID: "peer-3", Status: PeerUnreachable})

	state := r.GetGossipState()
	if state.Members != 3 {
		t.Errorf("Members = %d, want 3", state.Members)
	}
	if state.Suspects != 1 {
		t.Errorf("Suspects = %d, want 1", state.Suspects)
	}
	if state.Unreachable != 1 {
		t.Errorf("Unreachable = %d, want 1", state.Unreachable)
	}
	if state.VectorClock == nil {
		t.Error("VectorClock should not be nil")
	}
}

func TestReplicator_Stats(t *testing.T) {
	cfg := DefaultReplicatorConfig()
	cfg.NodeID = "node-A"
	r := NewReplicator(cfg)
	_ = r.AddPeer(&Peer{ID: "peer-B", Address: "10.0.0.2:5000"})

	// Send a message
	_ = r.Replicate(&ReplicationMessage{Type: MessagePut, TargetPeer: "peer-B", Payload: map[string]string{"k": "v"}})
	// Receive a message
	_ = r.Receive(&ReplicationMessage{Type: MessagePut, SourcePeer: "peer-B"})
	// Anti-entropy
	_, _ = r.AntiEntropy("peer-B")

	stats := r.Stats()
	if stats.PeersTotal != 1 {
		t.Errorf("PeersTotal = %d, want 1", stats.PeersTotal)
	}
	if stats.MessagesSent != 1 {
		t.Errorf("MessagesSent = %d, want 1", stats.MessagesSent)
	}
	if stats.MessagesReceived != 1 {
		t.Errorf("MessagesReceived = %d, want 1", stats.MessagesReceived)
	}
	if stats.ConflictsResolved != 1 {
		t.Errorf("ConflictsResolved = %d, want 1", stats.ConflictsResolved)
	}
	if stats.AntiEntropyRuns != 1 {
		t.Errorf("AntiEntropyRuns = %d, want 1", stats.AntiEntropyRuns)
	}
}

func TestReplicator_MessageTypes(t *testing.T) {
	r := NewReplicator(DefaultReplicatorConfig())
	_ = r.AddPeer(&Peer{ID: "peer-B"})

	types := []MessageType{MessagePut, MessageDelete, MessageSync}
	for _, mt := range types {
		msg := &ReplicationMessage{Type: mt, TargetPeer: "peer-B", Payload: map[string]string{"k": "v"}}
		if err := r.Replicate(msg); err != nil {
			t.Errorf("Replicate with type %q: %v", mt, err)
		}
	}

	stats := r.Stats()
	if stats.MessagesSent != 3 {
		t.Errorf("MessagesSent = %d, want 3", stats.MessagesSent)
	}
}

func TestReplicator_CloneClock(t *testing.T) {
	cfg := DefaultReplicatorConfig()
	cfg.NodeID = "node-A"
	r := NewReplicator(cfg)
	_ = r.AddPeer(&Peer{ID: "peer-B"})

	// Send to increment clock
	_ = r.Replicate(&ReplicationMessage{Type: MessagePut, TargetPeer: "peer-B", Payload: map[string]string{"k": "v"}})

	state := r.GetGossipState()

	// Modify returned clock - should not affect internal state
	state.VectorClock["node-A"] = 999

	r.mu.RLock()
	actual := r.clock["node-A"]
	r.mu.RUnlock()

	if actual == 999 {
		t.Error("modifying returned clock should not affect internal state")
	}
}

func TestPeerStatus_Constants(t *testing.T) {
	if PeerActive != "active" {
		t.Errorf("PeerActive = %q, want %q", PeerActive, "active")
	}
	if PeerSuspect != "suspect" {
		t.Errorf("PeerSuspect = %q, want %q", PeerSuspect, "suspect")
	}
	if PeerUnreachable != "unreachable" {
		t.Errorf("PeerUnreachable = %q, want %q", PeerUnreachable, "unreachable")
	}
}

func TestConflictResolutionStrategy_Constants(t *testing.T) {
	if LWW != "lww" {
		t.Errorf("LWW = %q, want %q", LWW, "lww")
	}
	if HighestVersion != "highest_version" {
		t.Errorf("HighestVersion = %q, want %q", HighestVersion, "highest_version")
	}
	if Custom != "custom" {
		t.Errorf("Custom = %q, want %q", Custom, "custom")
	}
}
