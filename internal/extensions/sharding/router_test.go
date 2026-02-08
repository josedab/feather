package sharding

import (
	"context"
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
