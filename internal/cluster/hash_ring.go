package cluster

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"sort"
	"sync"
)

// HashRing implements consistent hashing with virtual nodes.
type HashRing struct {
	nodes        map[string]*Node
	virtualNodes map[uint64]string  // hash -> nodeID
	sortedHashes []uint64
	replicas     int
	mu           sync.RWMutex
}

// NewHashRing creates a new consistent hash ring.
func NewHashRing(replicas int) *HashRing {
	if replicas <= 0 {
		replicas = 150
	}
	return &HashRing{
		nodes:        make(map[string]*Node),
		virtualNodes: make(map[uint64]string),
		sortedHashes: make([]uint64, 0),
		replicas:     replicas,
	}
}

// AddNode adds a node to the hash ring.
func (r *HashRing) AddNode(node *Node) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.nodes[node.ID]; exists {
		return
	}

	r.nodes[node.ID] = node

	// Add virtual nodes based on node's configured virtual node count
	vnodes := node.VirtualNodes
	if vnodes <= 0 {
		vnodes = r.replicas
	}

	// Adjust by weight
	adjustedVnodes := vnodes * node.Weight / 100
	if adjustedVnodes <= 0 {
		adjustedVnodes = 1
	}

	for i := 0; i < adjustedVnodes; i++ {
		hash := r.hash(fmt.Sprintf("%s-%d", node.ID, i))
		r.virtualNodes[hash] = node.ID
		r.sortedHashes = append(r.sortedHashes, hash)
	}

	sort.Slice(r.sortedHashes, func(i, j int) bool {
		return r.sortedHashes[i] < r.sortedHashes[j]
	})
}

// RemoveNode removes a node from the hash ring.
func (r *HashRing) RemoveNode(nodeID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	node, exists := r.nodes[nodeID]
	if !exists {
		return
	}

	vnodes := node.VirtualNodes
	if vnodes <= 0 {
		vnodes = r.replicas
	}
	adjustedVnodes := vnodes * node.Weight / 100
	if adjustedVnodes <= 0 {
		adjustedVnodes = 1
	}

	// Remove virtual nodes
	for i := 0; i < adjustedVnodes; i++ {
		hash := r.hash(fmt.Sprintf("%s-%d", nodeID, i))
		delete(r.virtualNodes, hash)
	}

	// Rebuild sorted hashes
	r.sortedHashes = make([]uint64, 0, len(r.virtualNodes))
	for hash := range r.virtualNodes {
		r.sortedHashes = append(r.sortedHashes, hash)
	}
	sort.Slice(r.sortedHashes, func(i, j int) bool {
		return r.sortedHashes[i] < r.sortedHashes[j]
	})

	delete(r.nodes, nodeID)
}

// GetNode returns the node responsible for the given key.
func (r *HashRing) GetNode(key string) (*Node, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if len(r.sortedHashes) == 0 {
		return nil, fmt.Errorf("no nodes in ring")
	}

	hash := r.hash(key)
	idx := r.search(hash)
	nodeID := r.virtualNodes[r.sortedHashes[idx]]

	return r.nodes[nodeID], nil
}

// GetNodes returns N nodes responsible for the given key (for replication).
func (r *HashRing) GetNodes(key string, count int) ([]*Node, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if len(r.sortedHashes) == 0 {
		return nil, fmt.Errorf("no nodes in ring")
	}

	if count > len(r.nodes) {
		count = len(r.nodes)
	}

	hash := r.hash(key)
	idx := r.search(hash)

	seen := make(map[string]bool)
	result := make([]*Node, 0, count)

	for i := 0; i < len(r.sortedHashes) && len(result) < count; i++ {
		actualIdx := (idx + i) % len(r.sortedHashes)
		nodeID := r.virtualNodes[r.sortedHashes[actualIdx]]

		if !seen[nodeID] {
			seen[nodeID] = true
			result = append(result, r.nodes[nodeID])
		}
	}

	return result, nil
}

// GetNodesInZones returns N nodes in different zones for the given key.
func (r *HashRing) GetNodesInZones(key string, count int) ([]*Node, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if len(r.sortedHashes) == 0 {
		return nil, fmt.Errorf("no nodes in ring")
	}

	hash := r.hash(key)
	idx := r.search(hash)

	seenNodes := make(map[string]bool)
	seenZones := make(map[string]bool)
	result := make([]*Node, 0, count)

	for i := 0; i < len(r.sortedHashes) && len(result) < count; i++ {
		actualIdx := (idx + i) % len(r.sortedHashes)
		nodeID := r.virtualNodes[r.sortedHashes[actualIdx]]

		if seenNodes[nodeID] {
			continue
		}

		node := r.nodes[nodeID]

		// Prefer nodes in different zones
		if seenZones[node.Zone] && len(result) > 0 {
			// Already have a node in this zone, skip unless we've exhausted all zones
			continue
		}

		seenNodes[nodeID] = true
		seenZones[node.Zone] = true
		result = append(result, node)
	}

	// If we couldn't find enough nodes in different zones, allow same-zone nodes
	if len(result) < count {
		for i := 0; i < len(r.sortedHashes) && len(result) < count; i++ {
			actualIdx := (idx + i) % len(r.sortedHashes)
			nodeID := r.virtualNodes[r.sortedHashes[actualIdx]]

			if !seenNodes[nodeID] {
				seenNodes[nodeID] = true
				result = append(result, r.nodes[nodeID])
			}
		}
	}

	return result, nil
}

// GetPartition returns the partition ID for a key.
func (r *HashRing) GetPartition(key string, totalPartitions int) int {
	hash := r.hash(key)
	return int(hash % uint64(totalPartitions))
}

// GetPartitionOwners returns the nodes responsible for a partition.
func (r *HashRing) GetPartitionOwners(partition, totalPartitions, replicationFactor int) []*Node {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if len(r.nodes) == 0 {
		return nil
	}

	// Generate a deterministic key for this partition
	key := fmt.Sprintf("partition-%d", partition)
	nodes, err := r.GetNodes(key, replicationFactor)
	if err != nil {
		return nil
	}
	return nodes
}

// NodeCount returns the number of nodes in the ring.
func (r *HashRing) NodeCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.nodes)
}

// VirtualNodeCount returns the total number of virtual nodes.
func (r *HashRing) VirtualNodeCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.virtualNodes)
}

// Nodes returns all nodes in the ring.
func (r *HashRing) Nodes() []*Node {
	r.mu.RLock()
	defer r.mu.RUnlock()

	nodes := make([]*Node, 0, len(r.nodes))
	for _, node := range r.nodes {
		nodes = append(nodes, node)
	}
	return nodes
}

// GetKeyDistribution returns the distribution of keys across nodes.
func (r *HashRing) GetKeyDistribution(sampleKeys []string) map[string]int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	distribution := make(map[string]int)
	for _, key := range sampleKeys {
		if len(r.sortedHashes) == 0 {
			continue
		}
		hash := r.hash(key)
		idx := r.search(hash)
		nodeID := r.virtualNodes[r.sortedHashes[idx]]
		distribution[nodeID]++
	}
	return distribution
}

// hash returns the hash of a key.
func (r *HashRing) hash(key string) uint64 {
	h := sha256.Sum256([]byte(key))
	return binary.BigEndian.Uint64(h[:8])
}

// search finds the index of the first hash >= the given hash.
func (r *HashRing) search(hash uint64) int {
	idx := sort.Search(len(r.sortedHashes), func(i int) bool {
		return r.sortedHashes[i] >= hash
	})
	if idx >= len(r.sortedHashes) {
		idx = 0
	}
	return idx
}

// PartitionMap manages the mapping of partitions to nodes.
type PartitionMap struct {
	ring              *HashRing
	totalPartitions   int
	replicationFactor int
	partitions        map[int][]*Node // partition -> owning nodes
	mu                sync.RWMutex
}

// NewPartitionMap creates a new partition map.
func NewPartitionMap(ring *HashRing, totalPartitions, replicationFactor int) *PartitionMap {
	pm := &PartitionMap{
		ring:              ring,
		totalPartitions:   totalPartitions,
		replicationFactor: replicationFactor,
		partitions:        make(map[int][]*Node),
	}
	pm.Recompute()
	return pm
}

// Recompute rebuilds the partition map.
func (pm *PartitionMap) Recompute() {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	pm.partitions = make(map[int][]*Node)
	for p := 0; p < pm.totalPartitions; p++ {
		pm.partitions[p] = pm.ring.GetPartitionOwners(p, pm.totalPartitions, pm.replicationFactor)
	}
}

// GetOwners returns the nodes responsible for a partition.
func (pm *PartitionMap) GetOwners(partition int) []*Node {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	if nodes, ok := pm.partitions[partition]; ok {
		result := make([]*Node, len(nodes))
		copy(result, nodes)
		return result
	}
	return nil
}

// GetPartitionForKey returns the partition for a key.
func (pm *PartitionMap) GetPartitionForKey(key string) int {
	return pm.ring.GetPartition(key, pm.totalPartitions)
}

// GetOwnersForKey returns the nodes responsible for a key.
func (pm *PartitionMap) GetOwnersForKey(key string) []*Node {
	partition := pm.GetPartitionForKey(key)
	return pm.GetOwners(partition)
}

// GetLocalPartitions returns partitions owned by the given node.
func (pm *PartitionMap) GetLocalPartitions(nodeID string) []int {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	var partitions []int
	for p, nodes := range pm.partitions {
		for _, node := range nodes {
			if node.ID == nodeID {
				partitions = append(partitions, p)
				break
			}
		}
	}
	return partitions
}

// GetPartitionDistribution returns the number of partitions per node.
func (pm *PartitionMap) GetPartitionDistribution() map[string]int {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	distribution := make(map[string]int)
	for _, nodes := range pm.partitions {
		for _, node := range nodes {
			distribution[node.ID]++
		}
	}
	return distribution
}

// TotalPartitions returns the total number of partitions.
func (pm *PartitionMap) TotalPartitions() int {
	return pm.totalPartitions
}

// ReplicationFactor returns the replication factor.
func (pm *PartitionMap) ReplicationFactor() int {
	return pm.replicationFactor
}
