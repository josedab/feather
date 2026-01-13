package cluster

import (
	"testing"
	"time"
)

func TestNode_Creation(t *testing.T) {
	node := &Node{
		ID:     "node-1",
		Status: NodeStatusAlive,
		Zone:   "us-east-1a",
	}

	if node.ID != "node-1" {
		t.Errorf("expected ID 'node-1', got '%s'", node.ID)
	}
	if node.Status != NodeStatusAlive {
		t.Errorf("expected status 'alive', got '%s'", node.Status)
	}
	if node.Zone != "us-east-1a" {
		t.Errorf("expected zone 'us-east-1a', got '%s'", node.Zone)
	}
}

func TestDefaultMembershipConfig(t *testing.T) {
	config := DefaultMembershipConfig()

	if config.GossipPort != 7946 {
		t.Errorf("expected gossip port 7946, got %d", config.GossipPort)
	}
	if config.DataPort != 7947 {
		t.Errorf("expected data port 7947, got %d", config.DataPort)
	}
	if config.GossipInterval != 200*time.Millisecond {
		t.Errorf("expected gossip interval 200ms, got %v", config.GossipInterval)
	}
	if config.VirtualNodes != 150 {
		t.Errorf("expected 150 virtual nodes, got %d", config.VirtualNodes)
	}
	if config.NodeID == "" {
		t.Error("expected non-empty node ID")
	}
}

func TestMembershipManager_Creation(t *testing.T) {
	config := DefaultMembershipConfig()
	config.NodeName = "test-node"

	m := NewMembershipManager(config)

	if m == nil {
		t.Fatal("expected non-nil membership manager")
	}

	local := m.LocalNode()
	if local == nil {
		t.Fatal("expected non-nil local node")
	}
	if local.Name != "test-node" {
		t.Errorf("expected name 'test-node', got '%s'", local.Name)
	}
	if local.Status != NodeStatusStarting {
		t.Errorf("expected status 'starting', got '%s'", local.Status)
	}
}

func TestMembershipManager_LocalNode(t *testing.T) {
	config := DefaultMembershipConfig()
	config.Zone = "us-west-2a"
	config.Region = "us-west-2"
	config.Weight = 150

	m := NewMembershipManager(config)

	local := m.LocalNode()
	if local.Zone != "us-west-2a" {
		t.Errorf("expected zone 'us-west-2a', got '%s'", local.Zone)
	}
	if local.Region != "us-west-2" {
		t.Errorf("expected region 'us-west-2', got '%s'", local.Region)
	}
	if local.Weight != 150 {
		t.Errorf("expected weight 150, got %d", local.Weight)
	}
}

func TestMembershipManager_UpdateMetadata(t *testing.T) {
	config := DefaultMembershipConfig()
	m := NewMembershipManager(config)

	initialGen := m.LocalNode().Generation

	m.UpdateMetadata("key1", "value1")
	m.UpdateMetadata("key2", "value2")

	local := m.LocalNode()
	if local.Metadata["key1"] != "value1" {
		t.Errorf("expected metadata key1='value1', got '%s'", local.Metadata["key1"])
	}
	if local.Metadata["key2"] != "value2" {
		t.Errorf("expected metadata key2='value2', got '%s'", local.Metadata["key2"])
	}
	if local.Generation <= initialGen {
		t.Error("expected generation to increase after metadata update")
	}
}

func TestMembershipManager_SetRole(t *testing.T) {
	config := DefaultMembershipConfig()
	m := NewMembershipManager(config)

	initialGen := m.LocalNode().Generation

	m.SetRole(NodeRoleLeader)

	local := m.LocalNode()
	if local.Role != NodeRoleLeader {
		t.Errorf("expected role 'leader', got '%s'", local.Role)
	}
	if local.Generation <= initialGen {
		t.Error("expected generation to increase after role change")
	}
}

func TestMembershipManager_Stats(t *testing.T) {
	config := DefaultMembershipConfig()
	m := NewMembershipManager(config)

	// Stats before starting
	stats := m.Stats()
	if stats.TotalMembers != 0 {
		t.Errorf("expected 0 total members, got %d", stats.TotalMembers)
	}
}

func TestMembershipEvent_Types(t *testing.T) {
	events := []MembershipEventType{
		EventNodeJoin,
		EventNodeLeave,
		EventNodeUpdate,
		EventNodeSuspect,
		EventNodeDead,
		EventNodeAlive,
	}

	expected := []string{"join", "leave", "update", "suspect", "dead", "alive"}

	for i, event := range events {
		if string(event) != expected[i] {
			t.Errorf("expected event type '%s', got '%s'", expected[i], event)
		}
	}
}

func TestNodeStatus_Values(t *testing.T) {
	statuses := []NodeStatus{
		NodeStatusAlive,
		NodeStatusSuspect,
		NodeStatusDead,
		NodeStatusLeft,
		NodeStatusStarting,
	}

	expected := []string{"alive", "suspect", "dead", "left", "starting"}

	for i, status := range statuses {
		if string(status) != expected[i] {
			t.Errorf("expected status '%s', got '%s'", expected[i], status)
		}
	}
}

func TestNodeRole_Values(t *testing.T) {
	roles := []NodeRole{
		NodeRoleLeader,
		NodeRoleFollower,
		NodeRoleObserver,
	}

	expected := []string{"leader", "follower", "observer"}

	for i, role := range roles {
		if string(role) != expected[i] {
			t.Errorf("expected role '%s', got '%s'", expected[i], role)
		}
	}
}

// mockListener implements MembershipListener for testing.
type mockListener struct {
	events []MembershipEvent
}

func (l *mockListener) OnMembershipChange(event MembershipEvent) {
	l.events = append(l.events, event)
}

func TestMembershipManager_AddRemoveListener(t *testing.T) {
	config := DefaultMembershipConfig()
	m := NewMembershipManager(config)

	listener := &mockListener{}
	m.AddListener(listener)

	// Check listener was added (indirectly by removing)
	m.RemoveListener(listener)

	// Removing again should not panic
	m.RemoveListener(listener)
}

func TestMembershipStats_Empty(t *testing.T) {
	config := DefaultMembershipConfig()
	m := NewMembershipManager(config)

	stats := m.Stats()

	if stats.TotalMembers != 0 {
		t.Errorf("expected 0 total members, got %d", stats.TotalMembers)
	}
	if stats.AliveMembers != 0 {
		t.Errorf("expected 0 alive members, got %d", stats.AliveMembers)
	}
	if len(stats.ByZone) != 0 {
		t.Errorf("expected empty zone map, got %d entries", len(stats.ByZone))
	}
	if len(stats.ByRegion) != 0 {
		t.Errorf("expected empty region map, got %d entries", len(stats.ByRegion))
	}
}

func TestGossipMessage_Types(t *testing.T) {
	types := []gossipType{
		gossipTypePing,
		gossipTypeAck,
		gossipTypeJoin,
		gossipTypeLeave,
	}

	expected := []string{"ping", "ack", "join", "leave"}

	for i, gt := range types {
		if string(gt) != expected[i] {
			t.Errorf("expected gossip type '%s', got '%s'", expected[i], gt)
		}
	}
}

func TestMembershipManager_Members_Empty(t *testing.T) {
	config := DefaultMembershipConfig()
	m := NewMembershipManager(config)

	members := m.Members()
	if len(members) != 0 {
		t.Errorf("expected 0 members, got %d", len(members))
	}

	alive := m.AliveMembers()
	if len(alive) != 0 {
		t.Errorf("expected 0 alive members, got %d", len(alive))
	}
}

func TestMembershipManager_GetMember_NotFound(t *testing.T) {
	config := DefaultMembershipConfig()
	m := NewMembershipManager(config)

	node, ok := m.GetMember("nonexistent")
	if ok {
		t.Error("expected GetMember to return false for nonexistent node")
	}
	if node != nil {
		t.Error("expected nil node for nonexistent member")
	}
}

func TestMembershipConfig_Defaults(t *testing.T) {
	config := DefaultMembershipConfig()

	if config.SuspicionMult != 4 {
		t.Errorf("expected suspicion mult 4, got %d", config.SuspicionMult)
	}
	if config.RetransmitMult != 4 {
		t.Errorf("expected retransmit mult 4, got %d", config.RetransmitMult)
	}
	if config.DeadNodeTimeout != 30*time.Second {
		t.Errorf("expected dead node timeout 30s, got %v", config.DeadNodeTimeout)
	}
	if config.ProbeTimeout != 500*time.Millisecond {
		t.Errorf("expected probe timeout 500ms, got %v", config.ProbeTimeout)
	}
}
