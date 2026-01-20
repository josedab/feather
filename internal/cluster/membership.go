// Package cluster provides distributed clustering capabilities
// using gossip-based membership and consistent hashing for sharding.
package cluster

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/google/uuid"
)

// NodeStatus represents the health status of a node.
type NodeStatus string

const (
	NodeStatusAlive    NodeStatus = "alive"
	NodeStatusSuspect  NodeStatus = "suspect"
	NodeStatusDead     NodeStatus = "dead"
	NodeStatusLeft     NodeStatus = "left"
	NodeStatusStarting NodeStatus = "starting"
)

// NodeRole represents the role of a node in the cluster.
type NodeRole string

const (
	NodeRoleLeader   NodeRole = "leader"
	NodeRoleFollower NodeRole = "follower"
	NodeRoleObserver NodeRole = "observer"
)

// Node represents a member of the cluster.
type Node struct {
	ID            string            `json:"id"`
	Name          string            `json:"name"`
	Address       string            `json:"address"`
	GossipPort    int               `json:"gossip_port"`
	DataPort      int               `json:"data_port"`
	Status        NodeStatus        `json:"status"`
	Role          NodeRole          `json:"role"`
	Zone          string            `json:"zone"`
	Region        string            `json:"region"`
	Weight        int               `json:"weight"`
	VirtualNodes  int               `json:"virtual_nodes"`
	Metadata      map[string]string `json:"metadata"`
	JoinedAt      time.Time         `json:"joined_at"`
	LastHeartbeat time.Time         `json:"last_heartbeat"`
	Generation    uint64            `json:"generation"`
}

// MembershipConfig configures the membership manager.
type MembershipConfig struct {
	NodeID           string
	NodeName         string
	BindAddress      string
	GossipPort       int
	DataPort         int
	Zone             string
	Region           string
	Weight           int
	VirtualNodes     int
	GossipInterval   time.Duration
	ProbeInterval    time.Duration
	ProbeTimeout     time.Duration
	SuspicionMult    int
	RetransmitMult   int
	Seeds            []string
	DeadNodeTimeout  time.Duration
}

// DefaultMembershipConfig returns sensible defaults.
func DefaultMembershipConfig() MembershipConfig {
	return MembershipConfig{
		NodeID:          uuid.New().String(),
		NodeName:        "",
		BindAddress:     "0.0.0.0",
		GossipPort:      7946,
		DataPort:        7947,
		Zone:            "default",
		Region:          "default",
		Weight:          100,
		VirtualNodes:    150,
		GossipInterval:  200 * time.Millisecond,
		ProbeInterval:   1 * time.Second,
		ProbeTimeout:    500 * time.Millisecond,
		SuspicionMult:   4,
		RetransmitMult:  4,
		Seeds:           []string{},
		DeadNodeTimeout: 30 * time.Second,
	}
}

// MembershipEvent represents a change in cluster membership.
type MembershipEvent struct {
	Type      MembershipEventType `json:"type"`
	Node      *Node               `json:"node"`
	Timestamp time.Time           `json:"timestamp"`
}

// MembershipEventType represents the type of membership event.
type MembershipEventType string

const (
	EventNodeJoin    MembershipEventType = "join"
	EventNodeLeave   MembershipEventType = "leave"
	EventNodeUpdate  MembershipEventType = "update"
	EventNodeSuspect MembershipEventType = "suspect"
	EventNodeDead    MembershipEventType = "dead"
	EventNodeAlive   MembershipEventType = "alive"
)

// MembershipListener is notified of membership changes.
type MembershipListener interface {
	OnMembershipChange(event MembershipEvent)
}

// MembershipManager manages cluster membership using SWIM-style gossip.
type MembershipManager struct {
	config    MembershipConfig
	localNode *Node
	members   map[string]*Node
	listeners []MembershipListener
	mu        sync.RWMutex

	gossipConn  *net.UDPConn
	gossipAddr  *net.UDPAddr
	incarnation uint64

	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
	started    bool
	eventsCh   chan MembershipEvent
}

// NewMembershipManager creates a new membership manager.
func NewMembershipManager(config MembershipConfig) *MembershipManager {
	if config.NodeID == "" {
		config.NodeID = uuid.New().String()
	}
	if config.NodeName == "" {
		config.NodeName = config.NodeID[:8]
	}

	localNode := &Node{
		ID:            config.NodeID,
		Name:          config.NodeName,
		Address:       config.BindAddress,
		GossipPort:    config.GossipPort,
		DataPort:      config.DataPort,
		Status:        NodeStatusStarting,
		Role:          NodeRoleFollower,
		Zone:          config.Zone,
		Region:        config.Region,
		Weight:        config.Weight,
		VirtualNodes:  config.VirtualNodes,
		Metadata:      make(map[string]string),
		JoinedAt:      time.Now(),
		LastHeartbeat: time.Now(),
		Generation:    1,
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &MembershipManager{
		config:    config,
		localNode: localNode,
		members:   make(map[string]*Node),
		listeners: []MembershipListener{},
		ctx:       ctx,
		cancel:    cancel,
		eventsCh:  make(chan MembershipEvent, 100),
	}
}

// Start begins the membership protocol.
func (m *MembershipManager) Start() error {
	m.mu.Lock()
	if m.started {
		m.mu.Unlock()
		return fmt.Errorf("membership manager already started")
	}
	m.started = true
	m.mu.Unlock()

	// Set up UDP listener for gossip
	addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", m.config.BindAddress, m.config.GossipPort))
	if err != nil {
		return fmt.Errorf("resolving gossip address: %w", err)
	}
	m.gossipAddr = addr

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return fmt.Errorf("listening on gossip port: %w", err)
	}
	m.gossipConn = conn

	// Update local node status
	m.mu.Lock()
	m.localNode.Status = NodeStatusAlive
	m.members[m.localNode.ID] = m.localNode
	m.mu.Unlock()

	// Start background goroutines
	m.wg.Add(3)
	go m.gossipLoop()
	go m.probeLoop()
	go m.eventLoop()

	// Join seeds if provided
	for _, seed := range m.config.Seeds {
		if err := m.joinSeed(seed); err != nil {
			// Log error but continue - seeds may not be available yet
			continue
		}
	}

	return nil
}

// Stop gracefully shuts down the membership manager.
func (m *MembershipManager) Stop() error {
	m.mu.Lock()
	if !m.started {
		m.mu.Unlock()
		return nil
	}
	m.started = false
	m.mu.Unlock()

	// Notify peers we're leaving
	m.broadcastLeave()

	m.cancel()
	if m.gossipConn != nil {
		m.gossipConn.Close()
	}

	m.wg.Wait()
	close(m.eventsCh)

	return nil
}

// LocalNode returns the local node.
func (m *MembershipManager) LocalNode() *Node {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.cloneNode(m.localNode)
}

// Members returns all known cluster members.
func (m *MembershipManager) Members() []*Node {
	m.mu.RLock()
	defer m.mu.RUnlock()

	nodes := make([]*Node, 0, len(m.members))
	for _, node := range m.members {
		nodes = append(nodes, m.cloneNode(node))
	}
	return nodes
}

// AliveMembers returns only alive members.
func (m *MembershipManager) AliveMembers() []*Node {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var nodes []*Node
	for _, node := range m.members {
		if node.Status == NodeStatusAlive {
			nodes = append(nodes, m.cloneNode(node))
		}
	}
	return nodes
}

// GetMember returns a specific member by ID.
func (m *MembershipManager) GetMember(nodeID string) (*Node, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	node, ok := m.members[nodeID]
	if !ok {
		return nil, false
	}
	return m.cloneNode(node), true
}

// AddListener registers a membership event listener.
func (m *MembershipManager) AddListener(listener MembershipListener) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.listeners = append(m.listeners, listener)
}

// RemoveListener removes a membership event listener.
func (m *MembershipManager) RemoveListener(listener MembershipListener) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i, l := range m.listeners {
		if l == listener {
			m.listeners = append(m.listeners[:i], m.listeners[i+1:]...)
			return
		}
	}
}

// UpdateMetadata updates the local node's metadata.
func (m *MembershipManager) UpdateMetadata(key, value string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.localNode.Metadata[key] = value
	m.localNode.Generation++
}

// SetRole sets the local node's role.
func (m *MembershipManager) SetRole(role NodeRole) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.localNode.Role = role
	m.localNode.Generation++
}

func (m *MembershipManager) cloneNode(node *Node) *Node {
	if node == nil {
		return nil
	}
	clone := *node
	clone.Metadata = make(map[string]string)
	for k, v := range node.Metadata {
		clone.Metadata[k] = v
	}
	return &clone
}

// gossipLoop periodically sends gossip messages to random peers.
func (m *MembershipManager) gossipLoop() {
	defer m.wg.Done()

	ticker := time.NewTicker(m.config.GossipInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.gossipRound()
		}
	}
}

// probeLoop periodically probes random members.
func (m *MembershipManager) probeLoop() {
	defer m.wg.Done()

	ticker := time.NewTicker(m.config.ProbeInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.probeRound()
		}
	}
}

// eventLoop dispatches membership events to listeners.
func (m *MembershipManager) eventLoop() {
	defer m.wg.Done()

	for {
		select {
		case <-m.ctx.Done():
			return
		case event := <-m.eventsCh:
			m.mu.RLock()
			listeners := make([]MembershipListener, len(m.listeners))
			copy(listeners, m.listeners)
			m.mu.RUnlock()

			for _, listener := range listeners {
				listener.OnMembershipChange(event)
			}
		}
	}
}

func (m *MembershipManager) gossipRound() {
	m.mu.RLock()
	localNode := m.cloneNode(m.localNode)
	var targets []*Node
	for _, node := range m.members {
		if node.ID != m.localNode.ID && node.Status == NodeStatusAlive {
			targets = append(targets, m.cloneNode(node))
		}
	}
	m.mu.RUnlock()

	if len(targets) == 0 {
		return
	}

	// Update heartbeat
	m.mu.Lock()
	m.localNode.LastHeartbeat = time.Now()
	m.mu.Unlock()

	// Send gossip to random target
	target := targets[time.Now().UnixNano()%int64(len(targets))]
	m.sendGossip(target, localNode)
}

func (m *MembershipManager) probeRound() {
	m.mu.RLock()
	var targets []*Node
	for _, node := range m.members {
		if node.ID != m.localNode.ID {
			targets = append(targets, m.cloneNode(node))
		}
	}
	m.mu.RUnlock()

	now := time.Now()
	for _, target := range targets {
		age := now.Sub(target.LastHeartbeat)

		switch target.Status {
		case NodeStatusAlive:
			if age > m.config.ProbeTimeout*time.Duration(m.config.SuspicionMult) {
				m.markSuspect(target.ID)
			}
		case NodeStatusSuspect:
			if age > m.config.DeadNodeTimeout {
				m.markDead(target.ID)
			}
		}
	}
}

func (m *MembershipManager) markSuspect(nodeID string) {
	m.mu.Lock()
	node, ok := m.members[nodeID]
	if !ok || node.Status != NodeStatusAlive {
		m.mu.Unlock()
		return
	}
	node.Status = NodeStatusSuspect
	m.mu.Unlock()

	m.emitEvent(MembershipEvent{
		Type:      EventNodeSuspect,
		Node:      m.cloneNode(node),
		Timestamp: time.Now(),
	})
}

func (m *MembershipManager) markDead(nodeID string) {
	m.mu.Lock()
	node, ok := m.members[nodeID]
	if !ok || node.Status == NodeStatusDead {
		m.mu.Unlock()
		return
	}
	node.Status = NodeStatusDead
	m.mu.Unlock()

	m.emitEvent(MembershipEvent{
		Type:      EventNodeDead,
		Node:      m.cloneNode(node),
		Timestamp: time.Now(),
	})
}

func (m *MembershipManager) markAlive(nodeID string) {
	m.mu.Lock()
	node, ok := m.members[nodeID]
	if !ok {
		m.mu.Unlock()
		return
	}
	wasNotAlive := node.Status != NodeStatusAlive
	node.Status = NodeStatusAlive
	node.LastHeartbeat = time.Now()
	m.mu.Unlock()

	if wasNotAlive {
		m.emitEvent(MembershipEvent{
			Type:      EventNodeAlive,
			Node:      m.cloneNode(node),
			Timestamp: time.Now(),
		})
	}
}

func (m *MembershipManager) sendGossip(target, local *Node) {
	msg := gossipMessage{
		Type:   gossipTypePing,
		Sender: local,
		Nodes:  m.sampleMembers(3),
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return
	}

	addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", target.Address, target.GossipPort))
	if err != nil {
		return
	}

	if m.gossipConn != nil {
		m.gossipConn.WriteToUDP(data, addr)
	}
}

func (m *MembershipManager) sampleMembers(count int) []*Node {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var nodes []*Node
	for _, node := range m.members {
		nodes = append(nodes, m.cloneNode(node))
		if len(nodes) >= count {
			break
		}
	}
	return nodes
}

func (m *MembershipManager) joinSeed(seed string) error {
	addr, err := net.ResolveUDPAddr("udp", seed)
	if err != nil {
		return err
	}

	msg := gossipMessage{
		Type:   gossipTypeJoin,
		Sender: m.cloneNode(m.localNode),
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	if m.gossipConn != nil {
		_, err = m.gossipConn.WriteToUDP(data, addr)
	}
	return err
}

func (m *MembershipManager) broadcastLeave() {
	m.mu.Lock()
	m.localNode.Status = NodeStatusLeft
	localNode := m.cloneNode(m.localNode)
	var targets []*Node
	for _, node := range m.members {
		if node.ID != m.localNode.ID && node.Status == NodeStatusAlive {
			targets = append(targets, m.cloneNode(node))
		}
	}
	m.mu.Unlock()

	msg := gossipMessage{
		Type:   gossipTypeLeave,
		Sender: localNode,
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return
	}

	for _, target := range targets {
		addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", target.Address, target.GossipPort))
		if err != nil {
			continue
		}
		if m.gossipConn != nil {
			m.gossipConn.WriteToUDP(data, addr)
		}
	}
}

func (m *MembershipManager) handleIncomingGossip(data []byte, addr *net.UDPAddr) {
	var msg gossipMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return
	}

	switch msg.Type {
	case gossipTypePing:
		m.handlePing(msg, addr)
	case gossipTypeAck:
		m.handleAck(msg)
	case gossipTypeJoin:
		m.handleJoin(msg, addr)
	case gossipTypeLeave:
		m.handleLeave(msg)
	}
}

func (m *MembershipManager) handlePing(msg gossipMessage, addr *net.UDPAddr) {
	// Merge received membership info
	if msg.Sender != nil {
		m.mergeNode(msg.Sender)
	}
	for _, node := range msg.Nodes {
		m.mergeNode(node)
	}

	// Send ack
	ack := gossipMessage{
		Type:   gossipTypeAck,
		Sender: m.cloneNode(m.localNode),
		Nodes:  m.sampleMembers(3),
	}

	data, err := json.Marshal(ack)
	if err != nil {
		return
	}

	if m.gossipConn != nil {
		m.gossipConn.WriteToUDP(data, addr)
	}
}

func (m *MembershipManager) handleAck(msg gossipMessage) {
	if msg.Sender != nil {
		m.markAlive(msg.Sender.ID)
		m.mergeNode(msg.Sender)
	}
	for _, node := range msg.Nodes {
		m.mergeNode(node)
	}
}

func (m *MembershipManager) handleJoin(msg gossipMessage, addr *net.UDPAddr) {
	if msg.Sender == nil {
		return
	}

	m.mu.Lock()
	_, exists := m.members[msg.Sender.ID]
	msg.Sender.Status = NodeStatusAlive
	msg.Sender.LastHeartbeat = time.Now()
	m.members[msg.Sender.ID] = msg.Sender
	m.mu.Unlock()

	if !exists {
		m.emitEvent(MembershipEvent{
			Type:      EventNodeJoin,
			Node:      m.cloneNode(msg.Sender),
			Timestamp: time.Now(),
		})
	}

	// Send current membership as response
	ack := gossipMessage{
		Type:   gossipTypeAck,
		Sender: m.cloneNode(m.localNode),
		Nodes:  m.sampleMembers(10),
	}

	data, err := json.Marshal(ack)
	if err != nil {
		return
	}

	if m.gossipConn != nil {
		m.gossipConn.WriteToUDP(data, addr)
	}
}

func (m *MembershipManager) handleLeave(msg gossipMessage) {
	if msg.Sender == nil {
		return
	}

	m.mu.Lock()
	node, exists := m.members[msg.Sender.ID]
	if exists {
		node.Status = NodeStatusLeft
	}
	m.mu.Unlock()

	if exists {
		m.emitEvent(MembershipEvent{
			Type:      EventNodeLeave,
			Node:      m.cloneNode(node),
			Timestamp: time.Now(),
		})
	}
}

func (m *MembershipManager) mergeNode(incoming *Node) {
	if incoming == nil || incoming.ID == m.localNode.ID {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	existing, exists := m.members[incoming.ID]
	if !exists {
		incoming.LastHeartbeat = time.Now()
		m.members[incoming.ID] = incoming

		go m.emitEvent(MembershipEvent{
			Type:      EventNodeJoin,
			Node:      m.cloneNode(incoming),
			Timestamp: time.Now(),
		})
		return
	}

	// Update if incoming has higher generation
	if incoming.Generation > existing.Generation {
		incoming.LastHeartbeat = time.Now()
		m.members[incoming.ID] = incoming

		go m.emitEvent(MembershipEvent{
			Type:      EventNodeUpdate,
			Node:      m.cloneNode(incoming),
			Timestamp: time.Now(),
		})
	} else if incoming.Generation == existing.Generation {
		// Same generation, update heartbeat
		existing.LastHeartbeat = time.Now()
	}
}

func (m *MembershipManager) emitEvent(event MembershipEvent) {
	select {
	case m.eventsCh <- event:
	default:
		// Channel full, drop event
	}
}

// gossipMessage is the message format for gossip protocol.
type gossipMessage struct {
	Type   gossipType `json:"type"`
	Sender *Node      `json:"sender"`
	Nodes  []*Node    `json:"nodes,omitempty"`
}

type gossipType string

const (
	gossipTypePing  gossipType = "ping"
	gossipTypeAck   gossipType = "ack"
	gossipTypeJoin  gossipType = "join"
	gossipTypeLeave gossipType = "leave"
)

// MembershipStats returns statistics about the membership.
type MembershipStats struct {
	TotalMembers   int            `json:"total_members"`
	AliveMembers   int            `json:"alive_members"`
	SuspectMembers int            `json:"suspect_members"`
	DeadMembers    int            `json:"dead_members"`
	ByZone         map[string]int `json:"by_zone"`
	ByRegion       map[string]int `json:"by_region"`
}

// Stats returns membership statistics.
func (m *MembershipManager) Stats() MembershipStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := MembershipStats{
		TotalMembers: len(m.members),
		ByZone:       make(map[string]int),
		ByRegion:     make(map[string]int),
	}

	for _, node := range m.members {
		switch node.Status {
		case NodeStatusAlive:
			stats.AliveMembers++
		case NodeStatusSuspect:
			stats.SuspectMembers++
		case NodeStatusDead:
			stats.DeadMembers++
		}
		stats.ByZone[node.Zone]++
		stats.ByRegion[node.Region]++
	}

	return stats
}
