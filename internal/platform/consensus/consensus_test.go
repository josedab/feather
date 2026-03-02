package consensus

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"
)

// testStateMachine is a simple state machine for testing.
type testStateMachine struct {
	applied [][]byte
}

func (sm *testStateMachine) Apply(entry *LogEntry) error {
	sm.applied = append(sm.applied, entry.Command)
	return nil
}

func (sm *testStateMachine) Snapshot() ([]byte, error) {
	return json.Marshal(sm.applied)
}

func (sm *testStateMachine) Restore(data []byte) error {
	return json.Unmarshal(data, &sm.applied)
}

func TestRaftNodeCreation(t *testing.T) {
	config := DefaultRaftConfig()
	config.NodeID = "node-1"
	config.Peers = []string{"node-2", "node-3"}

	sm := &testStateMachine{}
	node := NewRaftNode(context.Background(), config, sm)

	if node == nil {
		t.Fatal("expected non-nil RaftNode")
	}

	ctx := context.Background()
	state, term, isLeader := node.GetState(ctx)
	if state != StateFollower {
		t.Errorf("expected initial state follower, got %s", state)
	}
	if term != 0 {
		t.Errorf("expected initial term 0, got %d", term)
	}
	if isLeader {
		t.Error("expected node not to be leader initially")
	}
	if node.GetLeader(ctx) != "" {
		t.Errorf("expected no leader initially, got %s", node.GetLeader(ctx))
	}

	stats := node.Stats(ctx)
	if stats.NodeID != "node-1" {
		t.Errorf("expected node ID node-1, got %s", stats.NodeID)
	}
	if stats.PeerCount != 2 {
		t.Errorf("expected 2 peers, got %d", stats.PeerCount)
	}
	if stats.LogLength != 0 {
		t.Errorf("expected empty log, got %d entries", stats.LogLength)
	}
}

func TestRaftLeaderElection(t *testing.T) {
	// A node with no peers should elect itself leader
	config := DefaultRaftConfig()
	config.NodeID = "solo-node"
	config.ElectionTimeout = 50 * time.Millisecond
	config.Peers = []string{}

	sm := &testStateMachine{}
	node := NewRaftNode(context.Background(), config, sm)

	ctx := context.Background()
	if err := node.Start(ctx); err != nil {
		t.Fatalf("failed to start node: %v", err)
	}
	defer func() {
		if err := node.Stop(ctx); err != nil {
			t.Errorf("failed to stop node: %v", err)
		}
	}()

	// Wait for election timeout to fire
	time.Sleep(200 * time.Millisecond)

	state, term, isLeader := node.GetState(ctx)
	if state != StateLeader {
		t.Errorf("expected leader state, got %s", state)
	}
	if term == 0 {
		t.Error("expected term > 0 after election")
	}
	if !isLeader {
		t.Error("expected IsLeader to return true")
	}
	if node.GetLeader(ctx) != "solo-node" {
		t.Errorf("expected leader to be solo-node, got %s", node.GetLeader(ctx))
	}

	stats := node.Stats(ctx)
	if stats.Metrics.ElectionsStarted == 0 {
		t.Error("expected at least one election started")
	}
	if stats.Metrics.ElectionsWon == 0 {
		t.Error("expected at least one election won")
	}
}

func TestRaftPropose(t *testing.T) {
	// Create a single-node cluster that becomes leader
	config := DefaultRaftConfig()
	config.NodeID = "leader-node"
	config.ElectionTimeout = 50 * time.Millisecond

	sm := &testStateMachine{}
	node := NewRaftNode(context.Background(), config, sm)

	ctx := context.Background()
	if err := node.Start(ctx); err != nil {
		t.Fatalf("failed to start node: %v", err)
	}
	defer func() {
		_ = node.Stop(ctx)
	}()

	// Wait for leader election
	time.Sleep(200 * time.Millisecond)

	if !node.IsLeader(ctx) {
		t.Fatal("node should be leader before proposing")
	}

	// Propose a command
	cmd := []byte(`{"key":"user:1","value":"alice"}`)
	entry, err := node.Propose(ctx, cmd)
	if err != nil {
		t.Fatalf("failed to propose: %v", err)
	}

	if entry.Type != EntryTypeCommand {
		t.Errorf("expected command entry type, got %s", entry.Type)
	}
	if entry.Term == 0 {
		t.Error("expected term > 0 for proposed entry")
	}
	if entry.Applied.IsZero() {
		t.Error("expected entry to be applied")
	}

	// Verify state machine received the command
	if len(sm.applied) != 1 {
		t.Fatalf("expected 1 applied command, got %d", len(sm.applied))
	}
	if string(sm.applied[0]) != string(cmd) {
		t.Errorf("expected applied command %s, got %s", cmd, sm.applied[0])
	}

	// Verify log
	log := node.GetLog(ctx)
	// Should have noop + proposed command
	if len(log) < 2 {
		t.Errorf("expected at least 2 log entries (noop + command), got %d", len(log))
	}

	// Propose should fail on a follower
	followerConfig := DefaultRaftConfig()
	followerConfig.NodeID = "follower-node"
	followerSM := &testStateMachine{}
	follower := NewRaftNode(context.Background(), followerConfig, followerSM)

	_, err = follower.Propose(ctx, cmd)
	if err == nil {
		t.Error("expected error when proposing to follower")
	}
}

func TestRaftVoteGrant(t *testing.T) {
	config := DefaultRaftConfig()
	config.NodeID = "voter"

	sm := &testStateMachine{}
	node := NewRaftNode(context.Background(), config, sm)
	ctx := context.Background()

	// Grant vote to a candidate with higher term
	term, granted := node.GrantVote(ctx, "candidate-1", 1)
	if !granted {
		t.Error("expected vote to be granted")
	}
	if term != 1 {
		t.Errorf("expected term 1, got %d", term)
	}

	// Deny vote to same-term different candidate
	term, granted = node.GrantVote(ctx, "candidate-2", 1)
	if granted {
		t.Error("expected vote to be denied for second candidate in same term")
	}

	// Grant vote to candidate with higher term
	term, granted = node.GrantVote(ctx, "candidate-2", 2)
	if !granted {
		t.Error("expected vote to be granted for higher term")
	}
	if term != 2 {
		t.Errorf("expected term 2, got %d", term)
	}
}

func TestRaftHeartbeat(t *testing.T) {
	config := DefaultRaftConfig()
	config.NodeID = "follower"

	sm := &testStateMachine{}
	node := NewRaftNode(context.Background(), config, sm)
	ctx := context.Background()

	// Receive heartbeat from leader
	term, ok := node.ReceiveHeartbeat(ctx, "leader-1", 1, nil)
	if !ok {
		t.Error("expected heartbeat to be accepted")
	}
	if term != 1 {
		t.Errorf("expected term 1, got %d", term)
	}

	state, _, _ := node.GetState(ctx)
	if state != StateFollower {
		t.Errorf("expected follower state after heartbeat, got %s", state)
	}
	if node.GetLeader(ctx) != "leader-1" {
		t.Errorf("expected leader leader-1, got %s", node.GetLeader(ctx))
	}

	// Reject heartbeat with older term
	term, ok = node.ReceiveHeartbeat(ctx, "old-leader", 0, nil)
	if ok {
		t.Error("expected heartbeat with older term to be rejected")
	}
}

func TestShardManager(t *testing.T) {
	sm := NewShardManager(16, nil)
	ctx := context.Background()

	if sm == nil {
		t.Fatal("expected non-nil ShardManager")
	}

	shards := sm.ListShards(ctx)
	if len(shards) != 16 {
		t.Errorf("expected 16 shards, got %d", len(shards))
	}

	// All shards should start inactive
	for _, shard := range shards {
		if shard.State != ShardStateInactive {
			t.Errorf("expected shard %d to be inactive, got %s", shard.ID, shard.State)
		}
	}

	stats := sm.Stats(ctx)
	if stats.TotalShards != 16 {
		t.Errorf("expected 16 total shards, got %d", stats.TotalShards)
	}
	if stats.InactiveShards != 16 {
		t.Errorf("expected 16 inactive shards, got %d", stats.InactiveShards)
	}
}

func TestShardAssignment(t *testing.T) {
	sm := NewShardManager(12, nil)
	ctx := context.Background()

	nodes := []string{"node-a", "node-b", "node-c"}
	if err := sm.AssignShards(ctx, nodes); err != nil {
		t.Fatalf("failed to assign shards: %v", err)
	}

	// Each node should get 4 shards (12 / 3)
	for _, nodeID := range nodes {
		nodeShards := sm.GetNodeShards(ctx, nodeID)
		if len(nodeShards) != 4 {
			t.Errorf("expected node %s to have 4 shards, got %d", nodeID, len(nodeShards))
		}
	}

	// All shards should be active with a primary
	shards := sm.ListShards(ctx)
	for _, shard := range shards {
		if shard.State != ShardStateActive {
			t.Errorf("expected shard %d to be active, got %s", shard.ID, shard.State)
		}
		if shard.Primary == "" {
			t.Errorf("expected shard %d to have a primary", shard.ID)
		}
		if len(shard.Replicas) != 2 {
			t.Errorf("expected shard %d to have 2 replicas, got %d", shard.ID, len(shard.Replicas))
		}
	}

	stats := sm.Stats(ctx)
	if stats.ActiveShards != 12 {
		t.Errorf("expected 12 active shards, got %d", stats.ActiveShards)
	}
	if stats.NodeCount != 3 {
		t.Errorf("expected 3 nodes, got %d", stats.NodeCount)
	}

	// Assign with empty nodes should fail
	if err := sm.AssignShards(ctx, []string{}); err == nil {
		t.Error("expected error when assigning with no nodes")
	}
}

func TestShardMigration(t *testing.T) {
	sm := NewShardManager(8, nil)
	ctx := context.Background()

	nodes := []string{"node-1", "node-2"}
	if err := sm.AssignShards(ctx, nodes); err != nil {
		t.Fatalf("failed to assign shards: %v", err)
	}

	// Find a shard owned by node-1
	node1Shards := sm.GetNodeShards(ctx, "node-1")
	if len(node1Shards) == 0 {
		t.Fatal("expected node-1 to have shards")
	}
	shardToMigrate := node1Shards[0]

	// Migrate to node-2
	if err := sm.MigrateShard(ctx, shardToMigrate, "node-1", "node-2"); err != nil {
		t.Fatalf("failed to migrate shard: %v", err)
	}

	// Verify ownership changed
	info, err := sm.GetShardInfo(ctx, shardToMigrate)
	if err != nil {
		t.Fatalf("failed to get shard info: %v", err)
	}
	if info.Primary != "node-2" {
		t.Errorf("expected shard primary to be node-2, got %s", info.Primary)
	}

	// Verify node shard counts updated
	node1ShardsAfter := sm.GetNodeShards(ctx, "node-1")
	node2ShardsAfter := sm.GetNodeShards(ctx, "node-2")
	if len(node1ShardsAfter) != len(node1Shards)-1 {
		t.Errorf("expected node-1 to have %d shards, got %d", len(node1Shards)-1, len(node1ShardsAfter))
	}
	if len(node2ShardsAfter) != 4+1 { // started with 4, gained 1
		t.Errorf("expected node-2 to have 5 shards, got %d", len(node2ShardsAfter))
	}

	// Migrating from wrong node should fail
	if err := sm.MigrateShard(ctx, shardToMigrate, "node-1", "node-2"); err == nil {
		t.Error("expected error when migrating from wrong node")
	}

	// Migrating non-existent shard should fail
	if err := sm.MigrateShard(ctx, 9999, "node-1", "node-2"); err == nil {
		t.Error("expected error for non-existent shard")
	}
}

func TestShardSplitAndMerge(t *testing.T) {
	sm := NewShardManager(4, nil)
	ctx := context.Background()

	nodes := []string{"node-a", "node-b"}
	if err := sm.AssignShards(ctx, nodes); err != nil {
		t.Fatalf("failed to assign: %v", err)
	}

	// Split shard 0
	newID1, newID2, err := sm.SplitShard(ctx, 0)
	if err != nil {
		t.Fatalf("failed to split shard: %v", err)
	}

	// Original shard should be inactive
	original, err := sm.GetShardInfo(ctx, 0)
	if err != nil {
		t.Fatalf("failed to get shard info: %v", err)
	}
	if original.State != ShardStateInactive {
		t.Errorf("expected original shard to be inactive, got %s", original.State)
	}

	// New shards should be active
	for _, newID := range []int{newID1, newID2} {
		info, err := sm.GetShardInfo(ctx, newID)
		if err != nil {
			t.Fatalf("failed to get new shard %d: %v", newID, err)
		}
		if info.State != ShardStateActive {
			t.Errorf("expected new shard %d to be active, got %s", newID, info.State)
		}
	}

	// Merge the two new shards
	mergedID, err := sm.MergeShards(ctx, newID1, newID2)
	if err != nil {
		t.Fatalf("failed to merge shards: %v", err)
	}

	merged, err := sm.GetShardInfo(ctx, mergedID)
	if err != nil {
		t.Fatalf("failed to get merged shard: %v", err)
	}
	if merged.State != ShardStateActive {
		t.Errorf("expected merged shard to be active, got %s", merged.State)
	}

	// Source shards should be inactive
	for _, srcID := range []int{newID1, newID2} {
		info, err := sm.GetShardInfo(ctx, srcID)
		if err != nil {
			t.Fatalf("failed to get source shard %d: %v", srcID, err)
		}
		if info.State != ShardStateInactive {
			t.Errorf("expected source shard %d to be inactive, got %s", srcID, info.State)
		}
	}

	// Splitting inactive shard should fail
	_, _, err = sm.SplitShard(ctx, 0)
	if err == nil {
		t.Error("expected error splitting inactive shard")
	}

	// Merging inactive shard should fail
	_, err = sm.MergeShards(ctx, 0, 1)
	if err == nil {
		t.Error("expected error merging inactive shard")
	}
}

func TestGetShardForKey(t *testing.T) {
	sm := NewShardManager(16, nil)
	ctx := context.Background()

	tests := []struct {
		key string
	}{
		{"user:1"},
		{"user:2"},
		{"feature:click_count"},
		{"entity:product:123"},
		{""},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("key=%s", tt.key), func(t *testing.T) {
			shardID := sm.GetShardForKey(ctx, tt.key)
			if shardID < 0 || shardID >= 16 {
				t.Errorf("shard ID %d out of range [0, 16)", shardID)
			}

			// Same key should always map to same shard
			shardID2 := sm.GetShardForKey(ctx, tt.key)
			if shardID != shardID2 {
				t.Errorf("inconsistent shard mapping: got %d then %d", shardID, shardID2)
			}
		})
	}

	// Different keys should distribute across shards
	distribution := make(map[int]int)
	for i := 0; i < 1000; i++ {
		key := fmt.Sprintf("key:%d", i)
		shardID := sm.GetShardForKey(ctx, key)
		distribution[shardID]++
	}

	// With 16 shards and 1000 keys, we expect all shards to be used
	if len(distribution) < 10 {
		t.Errorf("expected reasonable distribution, only %d shards used out of 16", len(distribution))
	}
}

func TestRaftSetPeers(t *testing.T) {
	config := DefaultRaftConfig()
	config.NodeID = "node-1"
	config.Peers = []string{"node-2"}

	sm := &testStateMachine{}
	node := NewRaftNode(context.Background(), config, sm)
	ctx := context.Background()

	stats := node.Stats(ctx)
	if stats.PeerCount != 1 {
		t.Errorf("expected 1 peer, got %d", stats.PeerCount)
	}

	// Update peers
	node.SetPeers(ctx, []string{"node-2", "node-3", "node-4"})

	stats = node.Stats(ctx)
	if stats.PeerCount != 3 {
		t.Errorf("expected 3 peers after update, got %d", stats.PeerCount)
	}
}

func TestShardRebalance(t *testing.T) {
	sm := NewShardManager(12, nil)
	ctx := context.Background()

	// Assign to 2 nodes
	if err := sm.AssignShards(ctx, []string{"node-1", "node-2"}); err != nil {
		t.Fatalf("failed to assign: %v", err)
	}

	// Rebalance to 3 nodes
	if err := sm.RebalanceShards(ctx, []string{"node-1", "node-2", "node-3"}); err != nil {
		t.Fatalf("failed to rebalance: %v", err)
	}

	// Each node should get 4 shards
	for _, nodeID := range []string{"node-1", "node-2", "node-3"} {
		shards := sm.GetNodeShards(ctx, nodeID)
		if len(shards) != 4 {
			t.Errorf("expected node %s to have 4 shards, got %d", nodeID, len(shards))
		}
	}

	// Rebalance with no nodes should fail
	if err := sm.RebalanceShards(ctx, []string{}); err == nil {
		t.Error("expected error rebalancing with no nodes")
	}
}

func TestLogEntrySerialization(t *testing.T) {
	entry := &LogEntry{
		Index:   1,
		Term:    2,
		Type:    EntryTypeCommand,
		Command: []byte(`{"key":"test"}`),
		Applied: time.Now().Truncate(time.Millisecond),
	}

	data, err := marshalLogEntry(entry)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	restored, err := unmarshalLogEntry(data)
	if err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if restored.Index != entry.Index {
		t.Errorf("index mismatch: %d vs %d", restored.Index, entry.Index)
	}
	if restored.Term != entry.Term {
		t.Errorf("term mismatch: %d vs %d", restored.Term, entry.Term)
	}
	if restored.Type != entry.Type {
		t.Errorf("type mismatch: %s vs %s", restored.Type, entry.Type)
	}
	if string(restored.Command) != string(entry.Command) {
		t.Errorf("command mismatch: %s vs %s", restored.Command, entry.Command)
	}
}

func TestDefaultConfigs(t *testing.T) {
	rc := DefaultRaftConfig()
	if rc.ElectionTimeout <= 0 {
		t.Error("expected positive election timeout")
	}
	if rc.HeartbeatInterval <= 0 {
		t.Error("expected positive heartbeat interval")
	}
	if rc.MaxLogEntries <= 0 {
		t.Error("expected positive max log entries")
	}

	sc := DefaultShardManagerConfig()
	if sc.TotalShards <= 0 {
		t.Error("expected positive total shards")
	}
	if sc.ReplicasPerShard <= 0 {
		t.Error("expected positive replicas per shard")
	}
}

func TestRaftNode_AddRemovePeer(t *testing.T) {
	config := DefaultRaftConfig()
	config.NodeID = "node1"
	node := NewRaftNode(context.Background(), config, nil)

	ctx := context.Background()

	// Add a peer
	if err := node.AddPeer(ctx, "node2"); err != nil {
		t.Fatalf("AddPeer failed: %v", err)
	}
	peers := node.ListPeers(ctx)
	if len(peers) != 1 {
		t.Fatalf("expected 1 peer, got %d", len(peers))
	}

	// Duplicate add should fail
	if err := node.AddPeer(ctx, "node2"); err == nil {
		t.Error("expected error for duplicate peer")
	}

	// Add self should fail
	if err := node.AddPeer(ctx, "node1"); err == nil {
		t.Error("expected error for adding self")
	}

	// Remove peer
	if err := node.RemovePeer(ctx, "node2"); err != nil {
		t.Fatalf("RemovePeer failed: %v", err)
	}
	if len(node.ListPeers(ctx)) != 0 {
		t.Error("expected 0 peers after removal")
	}

	// Remove non-existent should fail
	if err := node.RemovePeer(ctx, "node3"); err == nil {
		t.Error("expected error for non-existent peer")
	}
}

func TestRaftNode_ClusterHealth(t *testing.T) {
	config := DefaultRaftConfig()
	config.NodeID = "node1"
	config.Peers = []string{"node2", "node3"}
	node := NewRaftNode(context.Background(), config, nil)

	health := node.GetClusterHealth(context.Background())
	if health.NodeID != "node1" {
		t.Errorf("expected node1, got %s", health.NodeID)
	}
	if health.PeerCount != 2 {
		t.Errorf("expected 2 peers, got %d", health.PeerCount)
	}
	if !health.HasQuorum {
		t.Error("expected quorum with 3 nodes")
	}
	if len(health.PeerStatuses) != 2 {
		t.Errorf("expected 2 peer statuses, got %d", len(health.PeerStatuses))
	}
}
