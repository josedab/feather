package sharding

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/feather-store/feather/internal/platform/cluster"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockReplicaClient struct {
	writeErr error
	readResp *ReadResponse
	readErr  error
	mu       sync.Mutex
	calls    int
}

func (m *mockReplicaClient) WriteFeature(_ context.Context, _ string, _ *WriteRequest) error {
	m.mu.Lock()
	m.calls++
	m.mu.Unlock()
	return m.writeErr
}

func (m *mockReplicaClient) ReadFeature(_ context.Context, _ string, _ *ReadRequest) (*ReadResponse, error) {
	m.mu.Lock()
	m.calls++
	m.mu.Unlock()
	if m.readErr != nil {
		return nil, m.readErr
	}
	return m.readResp, nil
}

func newTestRing(nodeCount int) *cluster.HashRing {
	ring := cluster.NewHashRing(50)
	for i := 0; i < nodeCount; i++ {
		ring.AddNode(&cluster.Node{
			ID:           nodeIDForIndex(i),
			Name:         nodeIDForIndex(i),
			Address:      "127.0.0.1",
			DataPort:     8080 + i,
			Weight:       100,
			VirtualNodes: 50,
			Zone:         "us-east-1a",
			Region:       "us-east-1",
		})
	}
	return ring
}

func nodeIDForIndex(i int) string {
	return "node-" + string(rune('a'+i))
}

func TestNewRouter(t *testing.T) {
	ring := newTestRing(3)
	client := &mockReplicaClient{}
	cfg := DefaultRouterConfig()
	cfg.LocalNodeID = "node-a"

	router := NewRouter(cfg, ring, client)
	require.NotNil(t, router)
	assert.NotNil(t, router.PartitionMap())
}

func TestRouteWrite_Quorum(t *testing.T) {
	ring := newTestRing(3)
	client := &mockReplicaClient{}
	cfg := DefaultRouterConfig()
	cfg.LocalNodeID = "node-a"
	cfg.WriteConsistency = WriteConsistencyQuorum
	cfg.ReplicationFactor = 3

	router := NewRouter(cfg, ring, client)
	err := router.RouteWrite(context.Background(), &WriteRequest{
		EntityKey:  "user:123",
		FeatureKey: "clicks",
		Value:      42,
		Timestamp:  time.Now().UnixNano(),
	})
	require.NoError(t, err)

	stats := router.Stats()
	assert.Equal(t, int64(1), stats["total_writes"])
}

func TestRouteWrite_NoOwners(t *testing.T) {
	ring := cluster.NewHashRing(50) // empty ring
	client := &mockReplicaClient{}
	cfg := DefaultRouterConfig()

	router := NewRouter(cfg, ring, client)
	err := router.RouteWrite(context.Background(), &WriteRequest{EntityKey: "user:123"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no owners")
}

func TestRouteRead_Local(t *testing.T) {
	ring := newTestRing(3)
	client := &mockReplicaClient{}
	cfg := DefaultRouterConfig()
	cfg.ReadConsistency = ReadConsistencyLocal

	// Find which node owns "user:123" and set it as local
	pm := cluster.NewPartitionMap(ring, cfg.TotalPartitions, cfg.ReplicationFactor)
	owners := pm.GetOwnersForKey("user:123")
	require.NotEmpty(t, owners)
	cfg.LocalNodeID = owners[0].ID

	router := NewRouter(cfg, ring, client)
	resp, err := router.RouteRead(context.Background(), &ReadRequest{
		EntityKey:   "user:123",
		FeatureKeys: []string{"clicks"},
	})
	require.NoError(t, err)
	assert.Equal(t, owners[0].ID, resp.NodeID)
}

func TestRouteRead_NoOwners(t *testing.T) {
	ring := cluster.NewHashRing(50)
	client := &mockReplicaClient{}
	cfg := DefaultRouterConfig()

	router := NewRouter(cfg, ring, client)
	_, err := router.RouteRead(context.Background(), &ReadRequest{EntityKey: "user:123"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no owners")
}

func TestIsLocalKey(t *testing.T) {
	ring := newTestRing(3)
	cfg := DefaultRouterConfig()
	cfg.LocalNodeID = "node-a"

	router := NewRouter(cfg, ring, &mockReplicaClient{})

	// At least some keys should be local
	localCount := 0
	for i := 0; i < 100; i++ {
		if router.IsLocalKey("key-" + string(rune('0'+i))) {
			localCount++
		}
	}
	assert.Greater(t, localCount, 0, "should own at least some keys")
}

func TestGetPartitionForKey(t *testing.T) {
	ring := newTestRing(3)
	cfg := DefaultRouterConfig()
	router := NewRouter(cfg, ring, &mockReplicaClient{})

	p1 := router.GetPartitionForKey("user:123")
	p2 := router.GetPartitionForKey("user:123")
	assert.Equal(t, p1, p2, "same key should map to same partition")
	assert.GreaterOrEqual(t, p1, 0)
	assert.Less(t, p1, cfg.TotalPartitions)
}

func TestResolveConflicts(t *testing.T) {
	router := &Router{}

	responses := []*ReadResponse{
		{
			NodeID: "node-a",
			Features: map[string]FeatureResult{
				"clicks": {Value: 10, Timestamp: 100, Found: true},
			},
		},
		{
			NodeID: "node-b",
			Features: map[string]FeatureResult{
				"clicks": {Value: 20, Timestamp: 200, Found: true},
			},
		},
	}

	merged := router.resolveConflicts(responses)
	assert.Equal(t, 20, merged.Features["clicks"].Value)
	assert.Equal(t, int64(200), merged.Features["clicks"].Timestamp)
}

func TestDefaultRouterConfig(t *testing.T) {
	cfg := DefaultRouterConfig()
	assert.Equal(t, 3, cfg.ReplicationFactor)
	assert.Equal(t, 64, cfg.TotalPartitions)
	assert.Equal(t, ReadConsistencyLocal, cfg.ReadConsistency)
	assert.Equal(t, WriteConsistencyQuorum, cfg.WriteConsistency)
}

func TestReadQuorum_SuccessfulQuorum(t *testing.T) {
	ring := newTestRing(3)
	client := &mockReplicaClient{
		readResp: &ReadResponse{
			NodeID: "node-b",
			Features: map[string]FeatureResult{
				"clicks": {Value: 42, Timestamp: 200, Found: true},
			},
		},
	}
	cfg := DefaultRouterConfig()
	cfg.ReadConsistency = ReadConsistencyQuorum
	cfg.ReplicationFactor = 3
	cfg.ReadTimeout = 5 * time.Second

	// Set local node to one of the owners
	pm := cluster.NewPartitionMap(ring, cfg.TotalPartitions, cfg.ReplicationFactor)
	owners := pm.GetOwnersForKey("user:123")
	require.NotEmpty(t, owners)
	cfg.LocalNodeID = owners[0].ID

	router := NewRouter(cfg, ring, client)
	resp, err := router.RouteRead(context.Background(), &ReadRequest{
		EntityKey:   "user:123",
		FeatureKeys: []string{"clicks"},
	})
	require.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestReadQuorum_Failure(t *testing.T) {
	ring := newTestRing(3)
	client := &mockReplicaClient{
		readErr: fmt.Errorf("node unavailable"),
	}
	cfg := DefaultRouterConfig()
	cfg.ReadConsistency = ReadConsistencyQuorum
	cfg.ReplicationFactor = 3
	cfg.ReadTimeout = 1 * time.Second
	cfg.LocalNodeID = "nonexistent-node"

	router := NewRouter(cfg, ring, client)
	_, err := router.RouteRead(context.Background(), &ReadRequest{
		EntityKey:   "user:123",
		FeatureKeys: []string{"clicks"},
	})
	require.Error(t, err)
}

func TestReadQuorum_Timeout(t *testing.T) {
	ring := newTestRing(3)
	// Slow client that blocks
	slowClient := &slowReplicaClient{delay: 5 * time.Second}
	cfg := DefaultRouterConfig()
	cfg.ReadConsistency = ReadConsistencyQuorum
	cfg.ReplicationFactor = 3
	cfg.ReadTimeout = 100 * time.Millisecond
	cfg.LocalNodeID = "nonexistent-node"

	router := NewRouter(cfg, ring, slowClient)
	_, err := router.RouteRead(context.Background(), &ReadRequest{
		EntityKey:   "user:timeout",
		FeatureKeys: []string{"clicks"},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "timeout")
}

func TestReadAll_AllRespond(t *testing.T) {
	ring := newTestRing(3)
	client := &mockReplicaClient{
		readResp: &ReadResponse{
			NodeID: "node-b",
			Features: map[string]FeatureResult{
				"clicks": {Value: 10, Timestamp: 100, Found: true},
			},
		},
	}
	cfg := DefaultRouterConfig()
	cfg.ReadConsistency = ReadConsistencyAll
	cfg.ReplicationFactor = 3
	cfg.ReadTimeout = 5 * time.Second

	pm := cluster.NewPartitionMap(ring, cfg.TotalPartitions, cfg.ReplicationFactor)
	owners := pm.GetOwnersForKey("user:123")
	require.NotEmpty(t, owners)
	cfg.LocalNodeID = owners[0].ID

	router := NewRouter(cfg, ring, client)
	resp, err := router.RouteRead(context.Background(), &ReadRequest{
		EntityKey:   "user:123",
		FeatureKeys: []string{"clicks"},
	})
	require.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestReadAll_PartialFailure(t *testing.T) {
	ring := newTestRing(3)
	// All remotes fail, but local succeeds
	client := &mockReplicaClient{
		readErr: fmt.Errorf("node down"),
	}
	cfg := DefaultRouterConfig()
	cfg.ReadConsistency = ReadConsistencyAll
	cfg.ReplicationFactor = 3
	cfg.ReadTimeout = 1 * time.Second

	pm := cluster.NewPartitionMap(ring, cfg.TotalPartitions, cfg.ReplicationFactor)
	owners := pm.GetOwnersForKey("user:123")
	require.NotEmpty(t, owners)
	cfg.LocalNodeID = owners[0].ID

	router := NewRouter(cfg, ring, client)
	resp, err := router.RouteRead(context.Background(), &ReadRequest{
		EntityKey:   "user:123",
		FeatureKeys: []string{"clicks"},
	})
	// Should succeed with at least the local response
	require.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestGetOwnersForKey(t *testing.T) {
	ring := newTestRing(3)
	cfg := DefaultRouterConfig()
	cfg.ReplicationFactor = 3
	router := NewRouter(cfg, ring, &mockReplicaClient{})

	owners := router.GetOwnersForKey("user:123")
	assert.NotEmpty(t, owners)
	assert.LessOrEqual(t, len(owners), 3)
}

func TestGetOwnersForKey_SingleNode(t *testing.T) {
	ring := newTestRing(1)
	cfg := DefaultRouterConfig()
	cfg.ReplicationFactor = 3
	router := NewRouter(cfg, ring, &mockReplicaClient{})

	owners := router.GetOwnersForKey("user:123")
	assert.NotEmpty(t, owners)
}

func TestRecompute(t *testing.T) {
	ring := newTestRing(3)
	cfg := DefaultRouterConfig()
	router := NewRouter(cfg, ring, &mockReplicaClient{})

	p1 := router.GetPartitionForKey("user:123")

	// Add a node and recompute
	ring.AddNode(&cluster.Node{
		ID: "node-d", Name: "node-d", Address: "127.0.0.1",
		DataPort: 8083, Weight: 100, VirtualNodes: 50,
		Zone: "us-east-1a", Region: "us-east-1",
	})
	router.Recompute()

	p2 := router.GetPartitionForKey("user:123")
	// Partition should still be deterministic for same key
	assert.Equal(t, p1, p2)
}

func TestRequiredAcks_ConsistencyOne(t *testing.T) {
	router := &Router{}
	acks := router.requiredAcks(3, "one")
	assert.Equal(t, 1, acks)
}

func TestRequiredAcks_ConsistencyAll(t *testing.T) {
	router := &Router{}
	acks := router.requiredAcks(3, "all")
	assert.Equal(t, 3, acks)
}

func TestRequiredAcks_ConsistencyQuorum(t *testing.T) {
	router := &Router{}
	acks := router.requiredAcks(3, "quorum")
	assert.Equal(t, 2, acks)
}

func TestRouteWrite_Timeout(t *testing.T) {
	ring := newTestRing(3)
	slowClient := &slowReplicaClient{delay: 5 * time.Second}
	cfg := DefaultRouterConfig()
	cfg.WriteConsistency = WriteConsistencyAll
	cfg.WriteTimeout = 100 * time.Millisecond
	cfg.LocalNodeID = "nonexistent-node"
	cfg.ReplicationFactor = 3

	router := NewRouter(cfg, ring, slowClient)
	err := router.RouteWrite(context.Background(), &WriteRequest{
		EntityKey:  "user:timeout",
		FeatureKey: "clicks",
		Value:      1,
	})
	require.Error(t, err)
}

func TestRouteWrite_AllFail(t *testing.T) {
	ring := newTestRing(3)
	client := &mockReplicaClient{writeErr: fmt.Errorf("disk full")}
	cfg := DefaultRouterConfig()
	cfg.WriteConsistency = WriteConsistencyQuorum
	cfg.WriteTimeout = 1 * time.Second
	cfg.LocalNodeID = "nonexistent-node"
	cfg.ReplicationFactor = 3

	router := NewRouter(cfg, ring, client)
	err := router.RouteWrite(context.Background(), &WriteRequest{
		EntityKey:  "user:fail",
		FeatureKey: "clicks",
		Value:      1,
	})
	require.Error(t, err)
}

// slowReplicaClient simulates a slow network.
type slowReplicaClient struct {
	delay time.Duration
}

func (s *slowReplicaClient) WriteFeature(ctx context.Context, _ string, _ *WriteRequest) error {
	select {
	case <-time.After(s.delay):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *slowReplicaClient) ReadFeature(ctx context.Context, _ string, _ *ReadRequest) (*ReadResponse, error) {
	select {
	case <-time.After(s.delay):
		return &ReadResponse{Features: make(map[string]FeatureResult)}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
