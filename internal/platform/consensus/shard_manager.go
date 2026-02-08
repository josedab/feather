package consensus

import (
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"sync"
	"time"
)

// ShardState represents the current state of a shard.
type ShardState string

const (
	// ShardStateActive indicates the shard is serving traffic.
	ShardStateActive ShardState = "active"
	// ShardStateMigrating indicates the shard is being moved to another node.
	ShardStateMigrating ShardState = "migrating"
	// ShardStateInactive indicates the shard is not serving traffic.
	ShardStateInactive ShardState = "inactive"
	// ShardStateSplitting indicates the shard is being split into smaller shards.
	ShardStateSplitting ShardState = "splitting"
)

// ShardInfo describes the metadata and ownership of a single shard.
type ShardInfo struct {
	ID        int        `json:"id"`
	State     ShardState `json:"state"`
	Primary   string     `json:"primary"`
	Replicas  []string   `json:"replicas"`
	Version   uint64     `json:"version"`
	KeyCount  int64      `json:"key_count"`
	SizeBytes int64      `json:"size_bytes"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

// ShardManagerConfig configures the shard manager.
type ShardManagerConfig struct {
	TotalShards      int `json:"total_shards"`
	ReplicasPerShard int `json:"replicas_per_shard"`
}

// DefaultShardManagerConfig returns sensible defaults.
func DefaultShardManagerConfig() ShardManagerConfig {
	return ShardManagerConfig{
		TotalShards:      64,
		ReplicasPerShard: 2,
	}
}

// ShardManager manages shard assignment and routing across cluster nodes.
// It coordinates shard ownership through the Raft log so that all nodes
// converge on the same shard-to-node mapping.
type ShardManager struct {
	shards      map[int]*ShardInfo
	nodeShards  map[string][]int // node -> shard IDs
	totalShards int
	raftNode    *RaftNode
	mu          sync.RWMutex
}

// NewShardManager creates a new shard manager backed by the given Raft node.
func NewShardManager(totalShards int, raftNode *RaftNode) *ShardManager {
	if totalShards <= 0 {
		totalShards = DefaultShardManagerConfig().TotalShards
	}

	sm := &ShardManager{
		shards:      make(map[int]*ShardInfo, totalShards),
		nodeShards:  make(map[string][]int),
		totalShards: totalShards,
		raftNode:    raftNode,
	}

	// Initialize all shards as inactive
	now := time.Now()
	for i := 0; i < totalShards; i++ {
		sm.shards[i] = &ShardInfo{
			ID:        i,
			State:     ShardStateInactive,
			Replicas:  []string{},
			CreatedAt: now,
			UpdatedAt: now,
		}
	}

	return sm
}

// AssignShards distributes shards evenly across the given set of node IDs.
// Each shard is assigned a primary owner and the mapping is replicated
// through Raft.
func (sm *ShardManager) AssignShards(ctx context.Context, nodeIDs []string) error {
	if len(nodeIDs) == 0 {
		return fmt.Errorf("assigning shards: no nodes provided")
	}

	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Clear existing assignments
	sm.nodeShards = make(map[string][]int, len(nodeIDs))
	for _, id := range nodeIDs {
		sm.nodeShards[id] = make([]int, 0)
	}

	now := time.Now()
	for i := 0; i < sm.totalShards; i++ {
		nodeIdx := i % len(nodeIDs)
		primary := nodeIDs[nodeIdx]

		shard := sm.shards[i]
		shard.Primary = primary
		shard.State = ShardStateActive
		shard.Version++
		shard.UpdatedAt = now

		// Assign replicas from other nodes
		replicas := make([]string, 0)
		for j := 1; j < len(nodeIDs) && len(replicas) < 2; j++ {
			replicaIdx := (nodeIdx + j) % len(nodeIDs)
			replicas = append(replicas, nodeIDs[replicaIdx])
		}
		shard.Replicas = replicas

		sm.nodeShards[primary] = append(sm.nodeShards[primary], i)
	}

	// Replicate assignment through Raft if available
	if sm.raftNode != nil {
		cmd, err := json.Marshal(map[string]interface{}{
			"action":    "assign_shards",
			"node_ids":  nodeIDs,
			"timestamp": now,
		})
		if err != nil {
			return fmt.Errorf("marshaling shard assignment: %w", err)
		}
		if _, err := sm.raftNode.Propose(ctx, cmd); err != nil {
			// Non-fatal: local assignment still applied
			_ = err
		}
	}

	return nil
}

// GetShardForKey returns the shard ID that owns the given key using
// FNV-1a hashing.
func (sm *ShardManager) GetShardForKey(ctx context.Context, key string) int {
	sm.mu.RLock()
	total := sm.totalShards
	sm.mu.RUnlock()

	h := fnv.New32a()
	_, _ = h.Write([]byte(key))
	return int(h.Sum32()) % total
}

// GetNodeShards returns the list of shard IDs assigned to the given node.
func (sm *ShardManager) GetNodeShards(ctx context.Context, nodeID string) []int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	shards, ok := sm.nodeShards[nodeID]
	if !ok {
		return nil
	}
	result := make([]int, len(shards))
	copy(result, shards)
	return result
}

// MigrateShard moves a shard from one node to another. The shard is marked
// as migrating during the transition and active once complete.
func (sm *ShardManager) MigrateShard(ctx context.Context, shardID int, fromNode, toNode string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	shard, ok := sm.shards[shardID]
	if !ok {
		return fmt.Errorf("migrating shard: shard %d not found", shardID)
	}

	if shard.Primary != fromNode {
		return fmt.Errorf("migrating shard: shard %d is owned by %s, not %s", shardID, shard.Primary, fromNode)
	}

	if shard.State == ShardStateMigrating {
		return fmt.Errorf("migrating shard: shard %d is already migrating", shardID)
	}

	// Mark as migrating
	shard.State = ShardStateMigrating
	shard.Version++
	shard.UpdatedAt = time.Now()

	// Update ownership
	shard.Primary = toNode
	shard.State = ShardStateActive

	// Update node-shard mappings
	sm.removeNodeShard(fromNode, shardID)
	sm.nodeShards[toNode] = append(sm.nodeShards[toNode], shardID)

	// Replicate through Raft
	if sm.raftNode != nil {
		cmd, err := json.Marshal(map[string]interface{}{
			"action":    "migrate_shard",
			"shard_id":  shardID,
			"from_node": fromNode,
			"to_node":   toNode,
		})
		if err == nil {
			_, _ = sm.raftNode.Propose(ctx, cmd)
		}
	}

	return nil
}

// SplitShard splits a shard into two new shards. The original shard becomes
// inactive and two new shards are created at the end of the shard range.
func (sm *ShardManager) SplitShard(ctx context.Context, shardID int) (int, int, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	shard, ok := sm.shards[shardID]
	if !ok {
		return 0, 0, fmt.Errorf("splitting shard: shard %d not found", shardID)
	}

	if shard.State != ShardStateActive {
		return 0, 0, fmt.Errorf("splitting shard: shard %d is not active (state: %s)", shardID, shard.State)
	}

	shard.State = ShardStateSplitting
	shard.Version++
	shard.UpdatedAt = time.Now()

	now := time.Now()
	newID1 := sm.totalShards
	newID2 := sm.totalShards + 1

	sm.shards[newID1] = &ShardInfo{
		ID:        newID1,
		State:     ShardStateActive,
		Primary:   shard.Primary,
		Replicas:  append([]string{}, shard.Replicas...),
		Version:   1,
		CreatedAt: now,
		UpdatedAt: now,
	}

	sm.shards[newID2] = &ShardInfo{
		ID:        newID2,
		State:     ShardStateActive,
		Primary:   shard.Primary,
		Replicas:  append([]string{}, shard.Replicas...),
		Version:   1,
		CreatedAt: now,
		UpdatedAt: now,
	}

	// Update node shard mappings
	sm.nodeShards[shard.Primary] = append(sm.nodeShards[shard.Primary], newID1, newID2)

	// Mark original shard inactive and remove from node
	shard.State = ShardStateInactive
	sm.removeNodeShard(shard.Primary, shardID)

	sm.totalShards += 2

	return newID1, newID2, nil
}

// MergeShards merges two shards into a single new shard. Both source shards
// become inactive and a new shard is created.
func (sm *ShardManager) MergeShards(ctx context.Context, shardID1, shardID2 int) (int, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	shard1, ok1 := sm.shards[shardID1]
	shard2, ok2 := sm.shards[shardID2]

	if !ok1 {
		return 0, fmt.Errorf("merging shards: shard %d not found", shardID1)
	}
	if !ok2 {
		return 0, fmt.Errorf("merging shards: shard %d not found", shardID2)
	}

	if shard1.State != ShardStateActive {
		return 0, fmt.Errorf("merging shards: shard %d is not active", shardID1)
	}
	if shard2.State != ShardStateActive {
		return 0, fmt.Errorf("merging shards: shard %d is not active", shardID2)
	}

	now := time.Now()
	newID := sm.totalShards

	sm.shards[newID] = &ShardInfo{
		ID:        newID,
		State:     ShardStateActive,
		Primary:   shard1.Primary,
		Replicas:  append([]string{}, shard1.Replicas...),
		Version:   1,
		KeyCount:  shard1.KeyCount + shard2.KeyCount,
		SizeBytes: shard1.SizeBytes + shard2.SizeBytes,
		CreatedAt: now,
		UpdatedAt: now,
	}

	// Deactivate source shards
	shard1.State = ShardStateInactive
	shard1.Version++
	shard1.UpdatedAt = now
	shard2.State = ShardStateInactive
	shard2.Version++
	shard2.UpdatedAt = now

	// Update node-shard mappings
	sm.nodeShards[shard1.Primary] = append(sm.nodeShards[shard1.Primary], newID)
	sm.removeNodeShard(shard1.Primary, shardID1)
	sm.removeNodeShard(shard2.Primary, shardID2)

	sm.totalShards++

	return newID, nil
}

// RebalanceShards redistributes shards evenly across the given nodes.
func (sm *ShardManager) RebalanceShards(ctx context.Context, nodeIDs []string) error {
	if len(nodeIDs) == 0 {
		return fmt.Errorf("rebalancing shards: no nodes provided")
	}

	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Collect active shards
	activeShards := make([]int, 0)
	for id, shard := range sm.shards {
		if shard.State == ShardStateActive {
			activeShards = append(activeShards, id)
		}
	}

	// Clear existing assignments
	sm.nodeShards = make(map[string][]int, len(nodeIDs))
	for _, id := range nodeIDs {
		sm.nodeShards[id] = make([]int, 0)
	}

	// Redistribute
	now := time.Now()
	for i, shardID := range activeShards {
		nodeIdx := i % len(nodeIDs)
		primary := nodeIDs[nodeIdx]

		shard := sm.shards[shardID]
		shard.Primary = primary
		shard.Version++
		shard.UpdatedAt = now

		// Assign replicas
		replicas := make([]string, 0)
		for j := 1; j < len(nodeIDs) && len(replicas) < 2; j++ {
			replicaIdx := (nodeIdx + j) % len(nodeIDs)
			replicas = append(replicas, nodeIDs[replicaIdx])
		}
		shard.Replicas = replicas

		sm.nodeShards[primary] = append(sm.nodeShards[primary], shardID)
	}

	return nil
}

// GetShardInfo returns the metadata for a specific shard.
func (sm *ShardManager) GetShardInfo(ctx context.Context, shardID int) (*ShardInfo, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	shard, ok := sm.shards[shardID]
	if !ok {
		return nil, fmt.Errorf("getting shard info: shard %d not found", shardID)
	}

	// Return a copy
	info := *shard
	info.Replicas = append([]string{}, shard.Replicas...)
	return &info, nil
}

// ListShards returns all shards managed by this shard manager.
func (sm *ShardManager) ListShards(ctx context.Context) []*ShardInfo {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	result := make([]*ShardInfo, 0, len(sm.shards))
	for _, shard := range sm.shards {
		info := *shard
		info.Replicas = append([]string{}, shard.Replicas...)
		result = append(result, &info)
	}
	return result
}

// ShardManagerStats holds statistics about the shard manager.
type ShardManagerStats struct {
	TotalShards    int            `json:"total_shards"`
	ActiveShards   int            `json:"active_shards"`
	MigratingShards int           `json:"migrating_shards"`
	InactiveShards int            `json:"inactive_shards"`
	SplittingShards int           `json:"splitting_shards"`
	NodeCount      int            `json:"node_count"`
	ShardsPerNode  map[string]int `json:"shards_per_node"`
}

// Stats returns statistics about shard distribution and states.
func (sm *ShardManager) Stats(ctx context.Context) ShardManagerStats {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	stats := ShardManagerStats{
		TotalShards:   len(sm.shards),
		NodeCount:     len(sm.nodeShards),
		ShardsPerNode: make(map[string]int, len(sm.nodeShards)),
	}

	for _, shard := range sm.shards {
		switch shard.State {
		case ShardStateActive:
			stats.ActiveShards++
		case ShardStateMigrating:
			stats.MigratingShards++
		case ShardStateInactive:
			stats.InactiveShards++
		case ShardStateSplitting:
			stats.SplittingShards++
		}
	}

	for nodeID, shards := range sm.nodeShards {
		stats.ShardsPerNode[nodeID] = len(shards)
	}

	return stats
}

// removeNodeShard removes a shard ID from a node's assignment list.
// Must be called with sm.mu held.
func (sm *ShardManager) removeNodeShard(nodeID string, shardID int) {
	shards, ok := sm.nodeShards[nodeID]
	if !ok {
		return
	}

	for i, id := range shards {
		if id == shardID {
			sm.nodeShards[nodeID] = append(shards[:i], shards[i+1:]...)
			return
		}
	}
}
