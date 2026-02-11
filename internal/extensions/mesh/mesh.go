package mesh

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"sort"
	"sync"
	"time"
)

// NodeState represents the current state of a mesh node.
type NodeState string

const (
	NodeStateActive       NodeState = "active"
	NodeStateDraining     NodeState = "draining"
	NodeStateUnhealthy    NodeState = "unhealthy"
	NodeStateDisconnected NodeState = "disconnected"
)

// MeshConfig holds configuration for the mesh network.
type MeshConfig struct {
	NodeID                  string        `json:"node_id"`
	ListenAddr              string        `json:"listen_addr"`
	AdvertiseAddr           string        `json:"advertise_addr"`
	HealthCheckInterval     time.Duration `json:"health_check_interval"`
	CircuitBreakerThreshold int           `json:"circuit_breaker_threshold"`
	RetryPolicy             string        `json:"retry_policy"`
	MaxRetries              int           `json:"max_retries"`
	RetryBackoff            time.Duration `json:"retry_backoff"`
	MTLSEnabled             bool          `json:"mtls_enabled"`
}

// DefaultMeshConfig returns sensible defaults for mesh configuration.
func DefaultMeshConfig() MeshConfig {
	return MeshConfig{
		NodeID:                  "node-1",
		ListenAddr:              ":7946",
		AdvertiseAddr:           "127.0.0.1:7946",
		HealthCheckInterval:     5 * time.Second,
		CircuitBreakerThreshold: 5,
		RetryPolicy:             "exponential",
		MaxRetries:              3,
		RetryBackoff:            100 * time.Millisecond,
		MTLSEnabled:             false,
	}
}

// Node represents a member of the feature store mesh.
type Node struct {
	ID       string            `json:"id"`
	Address  string            `json:"address"`
	State    NodeState         `json:"state"`
	Features []string          `json:"features"`
	Latency  time.Duration     `json:"latency"`
	LastSeen time.Time         `json:"last_seen"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// CircuitStats holds circuit breaker statistics.
type CircuitStats struct {
	TotalRequests       int64     `json:"total_requests"`
	Failures            int64     `json:"failures"`
	Successes           int64     `json:"successes"`
	ConsecutiveFailures int       `json:"consecutive_failures"`
	State               string    `json:"state"`
	LastFailure         time.Time `json:"last_failure,omitempty"`
}

// Circuit implements the circuit breaker pattern with states: closed, open, half-open.
type Circuit struct {
	mu                  sync.Mutex
	state               string
	threshold           int
	consecutiveFailures int
	totalRequests       int64
	failures            int64
	successes           int64
	lastFailure         time.Time
	openExpiry          time.Time
	halfOpenTimeout     time.Duration
}

// NewCircuit creates a new circuit breaker with the given failure threshold.
func NewCircuit(threshold int) *Circuit {
	return &Circuit{
		state:           "closed",
		threshold:       threshold,
		halfOpenTimeout: 10 * time.Second,
	}
}

// Allow returns true if a request is allowed through the circuit.
func (c *Circuit) Allow() bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	switch c.state {
	case "open":
		if time.Now().After(c.openExpiry) {
			c.state = "half-open"
			return true
		}
		return false
	case "half-open":
		return true
	default: // closed
		return true
	}
}

// RecordSuccess records a successful request.
func (c *Circuit) RecordSuccess() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.totalRequests++
	c.successes++
	c.consecutiveFailures = 0

	if c.state == "half-open" {
		c.state = "closed"
	}
}

// RecordFailure records a failed request.
func (c *Circuit) RecordFailure() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.totalRequests++
	c.failures++
	c.consecutiveFailures++
	c.lastFailure = time.Now()

	if c.consecutiveFailures >= c.threshold {
		c.state = "open"
		c.openExpiry = time.Now().Add(c.halfOpenTimeout)
	}
}

// State returns the current circuit state.
func (c *Circuit) State() string {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.state == "open" && time.Now().After(c.openExpiry) {
		c.state = "half-open"
	}
	return c.state
}

// Reset resets the circuit breaker to its initial closed state.
func (c *Circuit) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.state = "closed"
	c.consecutiveFailures = 0
	c.totalRequests = 0
	c.failures = 0
	c.successes = 0
	c.lastFailure = time.Time{}
	c.openExpiry = time.Time{}
}

// Stats returns circuit breaker statistics.
func (c *Circuit) Stats() CircuitStats {
	c.mu.Lock()
	defer c.mu.Unlock()

	return CircuitStats{
		TotalRequests:       c.totalRequests,
		Failures:            c.failures,
		Successes:           c.successes,
		ConsecutiveFailures: c.consecutiveFailures,
		State:               c.state,
		LastFailure:         c.lastFailure,
	}
}

// ServiceRegistry provides thread-safe service discovery for mesh nodes.
type ServiceRegistry struct {
	mu    sync.RWMutex
	nodes map[string]*Node
}

// NewServiceRegistry creates a new service registry.
func NewServiceRegistry() *ServiceRegistry {
	return &ServiceRegistry{
		nodes: make(map[string]*Node),
	}
}

// Register adds a node to the registry.
func (r *ServiceRegistry) Register(node *Node) error {
	if node == nil || node.ID == "" {
		return fmt.Errorf("node with valid ID is required")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	node.LastSeen = time.Now()
	r.nodes[node.ID] = node
	return nil
}

// Deregister removes a node from the registry.
func (r *ServiceRegistry) Deregister(nodeID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.nodes[nodeID]; !ok {
		return fmt.Errorf("node %q not found", nodeID)
	}
	delete(r.nodes, nodeID)
	return nil
}

// Discover returns nodes that serve the given feature.
func (r *ServiceRegistry) Discover(featureName string) []*Node {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []*Node
	for _, node := range r.nodes {
		if node.State != NodeStateActive {
			continue
		}
		for _, f := range node.Features {
			if f == featureName {
				result = append(result, node)
				break
			}
		}
	}
	return result
}

// GetNode retrieves a node by ID.
func (r *ServiceRegistry) GetNode(nodeID string) (*Node, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	node, ok := r.nodes[nodeID]
	if !ok {
		return nil, fmt.Errorf("node %q not found", nodeID)
	}
	return node, nil
}

// ListNodes returns all registered nodes.
func (r *ServiceRegistry) ListNodes() []*Node {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*Node, 0, len(r.nodes))
	for _, node := range r.nodes {
		result = append(result, node)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].ID < result[j].ID
	})
	return result
}

// HealthCheck returns the health status of all nodes.
func (r *ServiceRegistry) HealthCheck() map[string]bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	status := make(map[string]bool, len(r.nodes))
	for id, node := range r.nodes {
		status[id] = node.State == NodeStateActive
	}
	return status
}

// Router provides consistent-hash-based routing across mesh nodes.
type Router struct{}

// NewRouter creates a new router.
func NewRouter() *Router {
	return &Router{}
}

// Route selects a node for the given key using consistent hashing.
func (rt *Router) Route(key string, nodes []*Node) *Node {
	if len(nodes) == 0 {
		return nil
	}

	h := sha256.Sum256([]byte(key))
	idx := binary.BigEndian.Uint64(h[:8]) % uint64(len(nodes))
	return nodes[idx]
}

// RouteWithFallback routes a key and falls back if the primary node is unavailable.
func (rt *Router) RouteWithFallback(key string, nodes []*Node) (*Node, error) {
	if len(nodes) == 0 {
		return nil, fmt.Errorf("no nodes available")
	}

	h := sha256.Sum256([]byte(key))
	startIdx := binary.BigEndian.Uint64(h[:8]) % uint64(len(nodes))

	for i := 0; i < len(nodes); i++ {
		idx := (int(startIdx) + i) % len(nodes)
		if nodes[idx].State == NodeStateActive {
			return nodes[idx], nil
		}
	}
	return nil, fmt.Errorf("no active nodes available")
}

// MeshStats holds mesh-wide statistics.
type MeshStats struct {
	TotalNodes      int                       `json:"total_nodes"`
	ActiveNodes     int                       `json:"active_nodes"`
	TotalRoutes     int                       `json:"total_routes"`
	CircuitBreakers map[string]*CircuitStats  `json:"circuit_breakers"`
}

// MeshManager orchestrates the mesh network components.
type MeshManager struct {
	config   MeshConfig
	registry *ServiceRegistry
	router   *Router
	circuits map[string]*Circuit
	mu       sync.RWMutex
}

// NewMeshManager creates a new mesh manager.
func NewMeshManager(config MeshConfig) *MeshManager {
	return &MeshManager{
		config:   config,
		registry: NewServiceRegistry(),
		router:   NewRouter(),
		circuits: make(map[string]*Circuit),
	}
}

// Registry returns the service registry.
func (m *MeshManager) Registry() *ServiceRegistry {
	return m.registry
}

// Router returns the mesh router.
func (m *MeshManager) Router() *Router {
	return m.router
}

// GetCircuit returns (or creates) a circuit breaker for the given node ID.
func (m *MeshManager) GetCircuit(nodeID string) *Circuit {
	m.mu.Lock()
	defer m.mu.Unlock()

	if c, ok := m.circuits[nodeID]; ok {
		return c
	}
	c := NewCircuit(m.config.CircuitBreakerThreshold)
	m.circuits[nodeID] = c
	return c
}

// ResetCircuit resets the circuit breaker for the given node ID.
func (m *MeshManager) ResetCircuit(nodeID string) error {
	m.mu.RLock()
	c, ok := m.circuits[nodeID]
	m.mu.RUnlock()

	if !ok {
		return fmt.Errorf("circuit breaker for node %q not found", nodeID)
	}
	c.Reset()
	return nil
}

// CircuitBreakers returns stats for all circuit breakers.
func (m *MeshManager) CircuitBreakers() map[string]*CircuitStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]*CircuitStats, len(m.circuits))
	for id, c := range m.circuits {
		stats := c.Stats()
		result[id] = &stats
	}
	return result
}

// Join adds this node to the mesh at the given address.
func (m *MeshManager) Join(addr string) error {
	node := &Node{
		ID:       m.config.NodeID,
		Address:  addr,
		State:    NodeStateActive,
		LastSeen: time.Now(),
		Metadata: map[string]string{"advertise": m.config.AdvertiseAddr},
	}
	return m.registry.Register(node)
}

// Leave removes this node from the mesh.
func (m *MeshManager) Leave() error {
	return m.registry.Deregister(m.config.NodeID)
}

// Stats returns mesh-wide statistics.
func (m *MeshManager) Stats() *MeshStats {
	nodes := m.registry.ListNodes()
	active := 0
	for _, n := range nodes {
		if n.State == NodeStateActive {
			active++
		}
	}

	return &MeshStats{
		TotalNodes:      len(nodes),
		ActiveNodes:     active,
		TotalRoutes:     0,
		CircuitBreakers: m.CircuitBreakers(),
	}
}
