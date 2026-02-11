package mesh

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServiceRegistry_RegisterAndList(t *testing.T) {
	r := NewServiceRegistry()

	err := r.Register(&Node{ID: "n1", Address: ":8001", State: NodeStateActive})
	require.NoError(t, err)
	err = r.Register(&Node{ID: "n2", Address: ":8002", State: NodeStateActive})
	require.NoError(t, err)

	nodes := r.ListNodes()
	assert.Len(t, nodes, 2)
}

func TestServiceRegistry_RegisterValidation(t *testing.T) {
	r := NewServiceRegistry()

	tests := []struct {
		name string
		node *Node
	}{
		{"nil node", nil},
		{"empty ID", &Node{ID: ""}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := r.Register(tt.node)
			require.Error(t, err)
		})
	}
}

func TestServiceRegistry_Deregister(t *testing.T) {
	r := NewServiceRegistry()
	_ = r.Register(&Node{ID: "n1", Address: ":8001", State: NodeStateActive})

	err := r.Deregister("n1")
	require.NoError(t, err)
	assert.Empty(t, r.ListNodes())

	err = r.Deregister("nonexistent")
	require.Error(t, err)
}

func TestServiceRegistry_Discover(t *testing.T) {
	r := NewServiceRegistry()
	_ = r.Register(&Node{ID: "n1", State: NodeStateActive, Features: []string{"clicks", "views"}})
	_ = r.Register(&Node{ID: "n2", State: NodeStateActive, Features: []string{"views"}})
	_ = r.Register(&Node{ID: "n3", State: NodeStateDraining, Features: []string{"clicks"}})

	tests := []struct {
		feature string
		want    int
	}{
		{"clicks", 1},  // n3 is draining, only n1
		{"views", 2},   // n1 and n2
		{"unknown", 0},
	}
	for _, tt := range tests {
		t.Run(tt.feature, func(t *testing.T) {
			nodes := r.Discover(tt.feature)
			assert.Len(t, nodes, tt.want)
		})
	}
}

func TestServiceRegistry_GetNode(t *testing.T) {
	r := NewServiceRegistry()
	_ = r.Register(&Node{ID: "n1", Address: ":8001", State: NodeStateActive})

	node, err := r.GetNode("n1")
	require.NoError(t, err)
	assert.Equal(t, ":8001", node.Address)

	_, err = r.GetNode("nonexistent")
	require.Error(t, err)
}

func TestServiceRegistry_HealthCheck(t *testing.T) {
	r := NewServiceRegistry()
	_ = r.Register(&Node{ID: "n1", State: NodeStateActive})
	_ = r.Register(&Node{ID: "n2", State: NodeStateUnhealthy})

	health := r.HealthCheck()
	assert.True(t, health["n1"])
	assert.False(t, health["n2"])
}

func TestCircuit_ClosedState(t *testing.T) {
	c := NewCircuit(3)

	assert.True(t, c.Allow())
	assert.Equal(t, "closed", c.State())

	c.RecordSuccess()
	stats := c.Stats()
	assert.Equal(t, int64(1), stats.TotalRequests)
	assert.Equal(t, int64(1), stats.Successes)
}

func TestCircuit_OpensOnThreshold(t *testing.T) {
	c := NewCircuit(3)

	c.RecordFailure()
	c.RecordFailure()
	assert.Equal(t, "closed", c.State())

	c.RecordFailure() // hits threshold
	assert.Equal(t, "open", c.State())
	assert.False(t, c.Allow())
}

func TestCircuit_HalfOpenAfterTimeout(t *testing.T) {
	c := NewCircuit(1)
	c.halfOpenTimeout = 1 * time.Millisecond

	c.RecordFailure()
	assert.Equal(t, "open", c.State())

	time.Sleep(5 * time.Millisecond)
	assert.Equal(t, "half-open", c.State())
	assert.True(t, c.Allow())
}

func TestCircuit_HalfOpenRecovery(t *testing.T) {
	c := NewCircuit(1)
	c.halfOpenTimeout = 1 * time.Millisecond

	c.RecordFailure()
	time.Sleep(5 * time.Millisecond)

	// Allow transitions from open to half-open
	assert.True(t, c.Allow())
	assert.Equal(t, "half-open", c.State())

	// In half-open, success should close
	c.RecordSuccess()
	assert.Equal(t, "closed", c.State())
}

func TestCircuit_Reset(t *testing.T) {
	c := NewCircuit(1)
	c.RecordFailure()
	assert.Equal(t, "open", c.State())

	c.Reset()
	assert.Equal(t, "closed", c.State())
	stats := c.Stats()
	assert.Equal(t, int64(0), stats.TotalRequests)
}

func TestRouter_Route(t *testing.T) {
	rt := NewRouter()

	nodes := []*Node{
		{ID: "n1", State: NodeStateActive},
		{ID: "n2", State: NodeStateActive},
		{ID: "n3", State: NodeStateActive},
	}

	// Should return a node
	node := rt.Route("key1", nodes)
	assert.NotNil(t, node)

	// Should be deterministic
	node2 := rt.Route("key1", nodes)
	assert.Equal(t, node.ID, node2.ID)

	// Empty nodes returns nil
	assert.Nil(t, rt.Route("key1", nil))
}

func TestRouter_RouteWithFallback(t *testing.T) {
	rt := NewRouter()

	tests := []struct {
		name    string
		nodes   []*Node
		wantErr bool
	}{
		{
			name:    "no nodes",
			nodes:   nil,
			wantErr: true,
		},
		{
			name: "all unhealthy",
			nodes: []*Node{
				{ID: "n1", State: NodeStateUnhealthy},
				{ID: "n2", State: NodeStateDraining},
			},
			wantErr: true,
		},
		{
			name: "has active node",
			nodes: []*Node{
				{ID: "n1", State: NodeStateUnhealthy},
				{ID: "n2", State: NodeStateActive},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node, err := rt.RouteWithFallback("key", tt.nodes)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, NodeStateActive, node.State)
			}
		})
	}
}

func TestMeshManager_JoinAndLeave(t *testing.T) {
	m := NewMeshManager(DefaultMeshConfig())

	err := m.Join("127.0.0.1:7946")
	require.NoError(t, err)

	nodes := m.Registry().ListNodes()
	assert.Len(t, nodes, 1)

	err = m.Leave()
	require.NoError(t, err)
	assert.Empty(t, m.Registry().ListNodes())
}

func TestMeshManager_Stats(t *testing.T) {
	m := NewMeshManager(DefaultMeshConfig())
	_ = m.Join("127.0.0.1:7946")

	stats := m.Stats()
	assert.Equal(t, 1, stats.TotalNodes)
	assert.Equal(t, 1, stats.ActiveNodes)
}

func TestMeshManager_CircuitBreaker(t *testing.T) {
	m := NewMeshManager(DefaultMeshConfig())

	c := m.GetCircuit("n1")
	assert.Equal(t, "closed", c.State())

	// Same circuit returned
	c2 := m.GetCircuit("n1")
	assert.Equal(t, c, c2)

	// Reset
	c.RecordFailure()
	err := m.ResetCircuit("n1")
	require.NoError(t, err)
	assert.Equal(t, "closed", c.State())

	err = m.ResetCircuit("nonexistent")
	require.Error(t, err)
}

func TestMeshManager_CircuitBreakers(t *testing.T) {
	m := NewMeshManager(DefaultMeshConfig())
	m.GetCircuit("n1").RecordSuccess()
	m.GetCircuit("n2").RecordFailure()

	cbs := m.CircuitBreakers()
	assert.Len(t, cbs, 2)
	assert.Equal(t, int64(1), cbs["n1"].Successes)
	assert.Equal(t, int64(1), cbs["n2"].Failures)
}
