package cluster

import (
	"fmt"
	"testing"
)

func TestHashRing_Creation(t *testing.T) {
	ring := NewHashRing(150)

	if ring.NodeCount() != 0 {
		t.Errorf("expected 0 nodes, got %d", ring.NodeCount())
	}
	if ring.VirtualNodeCount() != 0 {
		t.Errorf("expected 0 virtual nodes, got %d", ring.VirtualNodeCount())
	}
}

func TestHashRing_AddNode(t *testing.T) {
	ring := NewHashRing(100)

	node := &Node{
		ID:           "node-1",
		Name:         "Node 1",
		Address:      "192.168.1.1",
		Weight:       100,
		VirtualNodes: 100,
	}

	ring.AddNode(node)

	if ring.NodeCount() != 1 {
		t.Errorf("expected 1 node, got %d", ring.NodeCount())
	}
	if ring.VirtualNodeCount() != 100 {
		t.Errorf("expected 100 virtual nodes, got %d", ring.VirtualNodeCount())
	}
}

func TestHashRing_AddNode_Duplicate(t *testing.T) {
	ring := NewHashRing(100)

	node := &Node{
		ID:           "node-1",
		Weight:       100,
		VirtualNodes: 100,
	}

	ring.AddNode(node)
	ring.AddNode(node) // Add again

	if ring.NodeCount() != 1 {
		t.Errorf("expected 1 node after duplicate add, got %d", ring.NodeCount())
	}
}

func TestHashRing_RemoveNode(t *testing.T) {
	ring := NewHashRing(100)

	node := &Node{
		ID:           "node-1",
		Weight:       100,
		VirtualNodes: 100,
	}

	ring.AddNode(node)
	ring.RemoveNode("node-1")

	if ring.NodeCount() != 0 {
		t.Errorf("expected 0 nodes after removal, got %d", ring.NodeCount())
	}
	if ring.VirtualNodeCount() != 0 {
		t.Errorf("expected 0 virtual nodes after removal, got %d", ring.VirtualNodeCount())
	}
}

func TestHashRing_RemoveNode_NotFound(t *testing.T) {
	ring := NewHashRing(100)

	// Should not panic
	ring.RemoveNode("nonexistent")

	if ring.NodeCount() != 0 {
		t.Errorf("expected 0 nodes, got %d", ring.NodeCount())
	}
}

func TestHashRing_GetNode(t *testing.T) {
	ring := NewHashRing(100)

	node := &Node{
		ID:           "node-1",
		Name:         "Node 1",
		Weight:       100,
		VirtualNodes: 100,
	}

	ring.AddNode(node)

	result, err := ring.GetNode("test-key")
	if err != nil {
		t.Fatalf("GetNode failed: %v", err)
	}
	if result.ID != "node-1" {
		t.Errorf("expected node-1, got %s", result.ID)
	}
}

func TestHashRing_GetNode_Empty(t *testing.T) {
	ring := NewHashRing(100)

	_, err := ring.GetNode("test-key")
	if err == nil {
		t.Error("expected error for empty ring")
	}
}

func TestHashRing_GetNode_Consistency(t *testing.T) {
	ring := NewHashRing(100)

	for i := 1; i <= 3; i++ {
		ring.AddNode(&Node{
			ID:           fmt.Sprintf("node-%d", i),
			Weight:       100,
			VirtualNodes: 100,
		})
	}

	// Same key should always map to same node
	key := "consistent-key"
	node1, _ := ring.GetNode(key)
	node2, _ := ring.GetNode(key)

	if node1.ID != node2.ID {
		t.Errorf("consistency violation: got %s and %s for same key", node1.ID, node2.ID)
	}
}

func TestHashRing_GetNodes(t *testing.T) {
	ring := NewHashRing(100)

	for i := 1; i <= 5; i++ {
		ring.AddNode(&Node{
			ID:           fmt.Sprintf("node-%d", i),
			Weight:       100,
			VirtualNodes: 100,
		})
	}

	nodes, err := ring.GetNodes("test-key", 3)
	if err != nil {
		t.Fatalf("GetNodes failed: %v", err)
	}
	if len(nodes) != 3 {
		t.Errorf("expected 3 nodes, got %d", len(nodes))
	}

	// Check uniqueness
	seen := make(map[string]bool)
	for _, node := range nodes {
		if seen[node.ID] {
			t.Errorf("duplicate node in result: %s", node.ID)
		}
		seen[node.ID] = true
	}
}

func TestHashRing_GetNodes_LimitedNodes(t *testing.T) {
	ring := NewHashRing(100)

	ring.AddNode(&Node{ID: "node-1", Weight: 100, VirtualNodes: 100})
	ring.AddNode(&Node{ID: "node-2", Weight: 100, VirtualNodes: 100})

	// Request more nodes than available
	nodes, err := ring.GetNodes("test-key", 5)
	if err != nil {
		t.Fatalf("GetNodes failed: %v", err)
	}
	if len(nodes) != 2 {
		t.Errorf("expected 2 nodes (max available), got %d", len(nodes))
	}
}

func TestHashRing_GetNodesInZones(t *testing.T) {
	ring := NewHashRing(100)

	ring.AddNode(&Node{ID: "node-1", Zone: "zone-a", Weight: 100, VirtualNodes: 100})
	ring.AddNode(&Node{ID: "node-2", Zone: "zone-a", Weight: 100, VirtualNodes: 100})
	ring.AddNode(&Node{ID: "node-3", Zone: "zone-b", Weight: 100, VirtualNodes: 100})
	ring.AddNode(&Node{ID: "node-4", Zone: "zone-c", Weight: 100, VirtualNodes: 100})

	nodes, err := ring.GetNodesInZones("test-key", 3)
	if err != nil {
		t.Fatalf("GetNodesInZones failed: %v", err)
	}
	if len(nodes) != 3 {
		t.Errorf("expected 3 nodes, got %d", len(nodes))
	}

	// Check that we got nodes from different zones
	zones := make(map[string]bool)
	for _, node := range nodes {
		zones[node.Zone] = true
	}
	if len(zones) < 2 {
		t.Errorf("expected nodes from at least 2 zones, got %d", len(zones))
	}
}

func TestHashRing_GetPartition(t *testing.T) {
	ring := NewHashRing(100)

	partitions := 256
	p1 := ring.GetPartition("key1", partitions)
	p2 := ring.GetPartition("key1", partitions)

	if p1 != p2 {
		t.Errorf("same key should map to same partition: %d vs %d", p1, p2)
	}

	if p1 < 0 || p1 >= partitions {
		t.Errorf("partition out of range: %d", p1)
	}
}

func TestHashRing_Weight(t *testing.T) {
	ring := NewHashRing(100)

	// Node with double weight should get more virtual nodes
	ring.AddNode(&Node{ID: "node-1", Weight: 100, VirtualNodes: 100})
	ring.AddNode(&Node{ID: "node-2", Weight: 200, VirtualNodes: 100})

	// Distribute many keys
	distribution := make(map[string]int)
	for i := 0; i < 10000; i++ {
		key := fmt.Sprintf("key-%d", i)
		node, _ := ring.GetNode(key)
		distribution[node.ID]++
	}

	// Node 2 should have roughly twice the keys
	ratio := float64(distribution["node-2"]) / float64(distribution["node-1"])
	if ratio < 1.5 || ratio > 2.5 {
		t.Errorf("expected ratio ~2.0, got %.2f", ratio)
	}
}

func TestHashRing_Distribution(t *testing.T) {
	ring := NewHashRing(150)

	for i := 1; i <= 5; i++ {
		ring.AddNode(&Node{
			ID:           fmt.Sprintf("node-%d", i),
			Weight:       100,
			VirtualNodes: 150,
		})
	}

	// Generate sample keys
	keys := make([]string, 1000)
	for i := 0; i < 1000; i++ {
		keys[i] = fmt.Sprintf("sample-key-%d", i)
	}

	distribution := ring.GetKeyDistribution(keys)

	// Check that all nodes got some keys
	for i := 1; i <= 5; i++ {
		nodeID := fmt.Sprintf("node-%d", i)
		if distribution[nodeID] == 0 {
			t.Errorf("node %s got no keys", nodeID)
		}
	}

	// Check relatively even distribution
	avgKeys := 1000.0 / 5.0
	for nodeID, count := range distribution {
		deviation := float64(count) / avgKeys
		if deviation < 0.5 || deviation > 1.5 {
			t.Errorf("node %s has uneven distribution: %d keys (expected ~%.0f)", nodeID, count, avgKeys)
		}
	}
}

func TestHashRing_Nodes(t *testing.T) {
	ring := NewHashRing(100)

	for i := 1; i <= 3; i++ {
		ring.AddNode(&Node{
			ID:           fmt.Sprintf("node-%d", i),
			Weight:       100,
			VirtualNodes: 100,
		})
	}

	nodes := ring.Nodes()
	if len(nodes) != 3 {
		t.Errorf("expected 3 nodes, got %d", len(nodes))
	}
}

func TestPartitionMap_Creation(t *testing.T) {
	ring := NewHashRing(100)
	ring.AddNode(&Node{ID: "node-1", Weight: 100, VirtualNodes: 100})
	ring.AddNode(&Node{ID: "node-2", Weight: 100, VirtualNodes: 100})

	pm := NewPartitionMap(ring, 256, 2)

	if pm.TotalPartitions() != 256 {
		t.Errorf("expected 256 partitions, got %d", pm.TotalPartitions())
	}
	if pm.ReplicationFactor() != 2 {
		t.Errorf("expected replication factor 2, got %d", pm.ReplicationFactor())
	}
}

func TestPartitionMap_GetOwners(t *testing.T) {
	ring := NewHashRing(100)
	ring.AddNode(&Node{ID: "node-1", Weight: 100, VirtualNodes: 100})
	ring.AddNode(&Node{ID: "node-2", Weight: 100, VirtualNodes: 100})
	ring.AddNode(&Node{ID: "node-3", Weight: 100, VirtualNodes: 100})

	pm := NewPartitionMap(ring, 256, 2)

	owners := pm.GetOwners(0)
	if len(owners) != 2 {
		t.Errorf("expected 2 owners, got %d", len(owners))
	}
}

func TestPartitionMap_GetOwnersForKey(t *testing.T) {
	ring := NewHashRing(100)
	ring.AddNode(&Node{ID: "node-1", Weight: 100, VirtualNodes: 100})
	ring.AddNode(&Node{ID: "node-2", Weight: 100, VirtualNodes: 100})

	pm := NewPartitionMap(ring, 256, 2)

	owners := pm.GetOwnersForKey("test-key")
	if len(owners) == 0 {
		t.Error("expected at least one owner")
	}
}

func TestPartitionMap_GetLocalPartitions(t *testing.T) {
	ring := NewHashRing(100)
	ring.AddNode(&Node{ID: "node-1", Weight: 100, VirtualNodes: 100})
	ring.AddNode(&Node{ID: "node-2", Weight: 100, VirtualNodes: 100})

	pm := NewPartitionMap(ring, 64, 2)

	partitions := pm.GetLocalPartitions("node-1")
	if len(partitions) == 0 {
		t.Error("expected node-1 to own some partitions")
	}

	// With RF=2 and 2 nodes, all partitions should be owned by both
	if len(partitions) != 64 {
		t.Errorf("with RF=2 and 2 nodes, expected 64 partitions, got %d", len(partitions))
	}
}

func TestPartitionMap_GetPartitionDistribution(t *testing.T) {
	ring := NewHashRing(100)
	ring.AddNode(&Node{ID: "node-1", Weight: 100, VirtualNodes: 100})
	ring.AddNode(&Node{ID: "node-2", Weight: 100, VirtualNodes: 100})
	ring.AddNode(&Node{ID: "node-3", Weight: 100, VirtualNodes: 100})

	pm := NewPartitionMap(ring, 128, 2)

	dist := pm.GetPartitionDistribution()

	// Each partition has RF=2 owners, so total should be 128*2 = 256
	total := 0
	for _, count := range dist {
		total += count
	}
	if total != 256 {
		t.Errorf("expected total 256, got %d", total)
	}
}

func TestPartitionMap_Recompute(t *testing.T) {
	ring := NewHashRing(100)
	ring.AddNode(&Node{ID: "node-1", Weight: 100, VirtualNodes: 100})

	pm := NewPartitionMap(ring, 64, 2)

	// Add another node
	ring.AddNode(&Node{ID: "node-2", Weight: 100, VirtualNodes: 100})

	// Recompute
	pm.Recompute()

	dist := pm.GetPartitionDistribution()
	if len(dist) != 2 {
		t.Errorf("expected 2 nodes in distribution, got %d", len(dist))
	}
}

func BenchmarkHashRing_GetNode(b *testing.B) {
	ring := NewHashRing(150)
	for i := 1; i <= 10; i++ {
		ring.AddNode(&Node{
			ID:           fmt.Sprintf("node-%d", i),
			Weight:       100,
			VirtualNodes: 150,
		})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("key-%d", i)
		_, _ = ring.GetNode(key)
	}
}

func BenchmarkHashRing_GetNodes(b *testing.B) {
	ring := NewHashRing(150)
	for i := 1; i <= 10; i++ {
		ring.AddNode(&Node{
			ID:           fmt.Sprintf("node-%d", i),
			Weight:       100,
			VirtualNodes: 150,
		})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("key-%d", i)
		_, _ = ring.GetNodes(key, 3)
	}
}
