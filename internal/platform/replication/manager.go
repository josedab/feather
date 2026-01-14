package replication

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// ConsistencyLevel defines the replication consistency guarantee.
type ConsistencyLevel string

const (
	ConsistencyEventual ConsistencyLevel = "eventual"
	ConsistencySession  ConsistencyLevel = "session"
	ConsistencyStrong   ConsistencyLevel = "strong"
)

// RegionStatus represents the health of a region.
type RegionStatus string

const (
	RegionActive   RegionStatus = "active"
	RegionDraining RegionStatus = "draining"
	RegionFailed   RegionStatus = "failed"
)

var (
	ErrRegionNotFound   = errors.New("region not found")
	ErrRegionExists     = errors.New("region already exists")
	ErrNoHealthyRegions = errors.New("no healthy regions available")
	ErrConflict         = errors.New("write conflict detected")
)

// VectorClock tracks causality across regions.
type VectorClock map[string]uint64

// Merge combines two vector clocks taking the max of each entry.
func (vc VectorClock) Merge(other VectorClock) VectorClock {
	merged := make(VectorClock)
	for k, v := range vc {
		merged[k] = v
	}
	for k, v := range other {
		if v > merged[k] {
			merged[k] = v
		}
	}
	return merged
}

// Increment advances the clock for a given region.
func (vc VectorClock) Increment(region string) VectorClock {
	next := make(VectorClock)
	for k, v := range vc {
		next[k] = v
	}
	next[region]++
	return next
}

// HappensBefore returns true if vc causally precedes other.
func (vc VectorClock) HappensBefore(other VectorClock) bool {
	atLeastOneLess := false
	for k, v := range vc {
		if v > other[k] {
			return false
		}
		if v < other[k] {
			atLeastOneLess = true
		}
	}
	for k, v := range other {
		if _, ok := vc[k]; !ok && v > 0 {
			atLeastOneLess = true
		}
	}
	return atLeastOneLess
}

// IsConcurrent returns true if neither clock causally precedes the other.
func (vc VectorClock) IsConcurrent(other VectorClock) bool {
	return !vc.HappensBefore(other) && !other.HappensBefore(vc)
}

// ReplicatedValue wraps a feature value with replication metadata.
type ReplicatedValue struct {
	Value     interface{} `json:"value"`
	Clock     VectorClock `json:"clock"`
	Origin    string      `json:"origin"`
	Timestamp time.Time   `json:"timestamp"`
}

// Region represents a replication peer.
type Region struct {
	// ID uniquely identifies this region.
	ID string `json:"id"`
	// Name is a human-readable name.
	Name string `json:"name"`
	// Endpoint is the replication API address.
	Endpoint string `json:"endpoint"`
	// Status is the current health state.
	Status RegionStatus `json:"status"`
	// Latency is the last measured RTT.
	Latency time.Duration `json:"latency"`
	// LastSeen is when the region last reported health.
	LastSeen time.Time `json:"last_seen"`
	// ReplicationLag is the observed lag.
	ReplicationLag time.Duration `json:"replication_lag"`
}

// ConflictPolicy determines how concurrent writes are resolved.
type ConflictPolicy string

const (
	// PolicyLastWriterWins resolves by timestamp.
	PolicyLastWriterWins ConflictPolicy = "lww"
	// PolicyHighestVersion resolves by vector clock sum.
	PolicyHighestVersion ConflictPolicy = "highest_version"
)

// ManagerConfig configures the replication manager.
type ManagerConfig struct {
	// LocalRegion is the ID of this region.
	LocalRegion string
	// Consistency is the default consistency level.
	Consistency ConsistencyLevel
	// ConflictPolicy determines conflict resolution.
	ConflictPolicy ConflictPolicy
	// ReplicateInterval is how often to push changes.
	ReplicateInterval time.Duration
	// HealthCheckInterval is how often to check peer health.
	HealthCheckInterval time.Duration
	// FailoverTimeout is the max time to wait before marking a region failed.
	FailoverTimeout time.Duration
}

// DefaultManagerConfig returns sensible defaults.
func DefaultManagerConfig() ManagerConfig {
	return ManagerConfig{
		LocalRegion:         "local",
		Consistency:         ConsistencyEventual,
		ConflictPolicy:      PolicyLastWriterWins,
		ReplicateInterval:   time.Second,
		HealthCheckInterval: 10 * time.Second,
		FailoverTimeout:     30 * time.Second,
	}
}

// Manager coordinates multi-region replication.
type Manager struct {
	mu       sync.RWMutex
	regions  map[string]*Region
	values   map[string]*ReplicatedValue // key -> replicated value
	config   ManagerConfig
	pending  []ReplicationEvent
	stopCh   chan struct{}
}

// ReplicationEvent records a value change to be propagated.
type ReplicationEvent struct {
	Key       string          `json:"key"`
	Value     *ReplicatedValue `json:"value"`
	TargetID  string          `json:"target_id"`
	CreatedAt time.Time       `json:"created_at"`
	Delivered bool            `json:"delivered"`
}

// NewManager creates a new replication manager.
func NewManager(config ManagerConfig) *Manager {
	if config.LocalRegion == "" {
		config = DefaultManagerConfig()
	}
	return &Manager{
		regions: make(map[string]*Region),
		values:  make(map[string]*ReplicatedValue),
		config:  config,
		pending: make([]ReplicationEvent, 0),
		stopCh:  make(chan struct{}),
	}
}

// AddRegion registers a peer region.
func (m *Manager) AddRegion(region *Region) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.regions[region.ID]; exists {
		return ErrRegionExists
	}

	region.Status = RegionActive
	region.LastSeen = time.Now()
	m.regions[region.ID] = region
	return nil
}

// RemoveRegion deregisters a peer region.
func (m *Manager) RemoveRegion(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.regions[id]; !exists {
		return ErrRegionNotFound
	}
	delete(m.regions, id)
	return nil
}

// GetRegion retrieves a region by ID.
func (m *Manager) GetRegion(id string) (*Region, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	r, exists := m.regions[id]
	if !exists {
		return nil, ErrRegionNotFound
	}
	return r, nil
}

// ListRegions returns all registered regions.
func (m *Manager) ListRegions() []*Region {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Region, 0, len(m.regions))
	for _, r := range m.regions {
		result = append(result, r)
	}
	return result
}

// Write stores a value with vector clock tracking and queues replication.
func (m *Manager) Write(key string, value interface{}) (*ReplicatedValue, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	existing := m.values[key]
	var clock VectorClock
	if existing != nil {
		clock = existing.Clock.Increment(m.config.LocalRegion)
	} else {
		clock = VectorClock{m.config.LocalRegion: 1}
	}

	rv := &ReplicatedValue{
		Value:     value,
		Clock:     clock,
		Origin:    m.config.LocalRegion,
		Timestamp: time.Now(),
	}
	m.values[key] = rv

	// Queue replication to all active peers
	for _, region := range m.regions {
		if region.ID != m.config.LocalRegion && region.Status == RegionActive {
			m.pending = append(m.pending, ReplicationEvent{
				Key:       key,
				Value:     rv,
				TargetID:  region.ID,
				CreatedAt: time.Now(),
			})
		}
	}

	return rv, nil
}

// Read retrieves a replicated value.
func (m *Manager) Read(key string) (*ReplicatedValue, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rv, exists := m.values[key]
	if !exists {
		return nil, fmt.Errorf("key %q not found", key)
	}
	return rv, nil
}

// ReceiveReplica handles an incoming replicated value from a peer.
func (m *Manager) ReceiveReplica(key string, incoming *ReplicatedValue) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	existing, exists := m.values[key]
	if !exists {
		m.values[key] = incoming
		return nil
	}

	// Resolve conflict
	if incoming.Clock.HappensBefore(existing.Clock) {
		// Incoming is older, ignore
		return nil
	}

	if existing.Clock.HappensBefore(incoming.Clock) {
		// Incoming is newer, accept
		m.values[key] = incoming
		return nil
	}

	// Concurrent writes - apply conflict policy
	switch m.config.ConflictPolicy {
	case PolicyLastWriterWins:
		if incoming.Timestamp.After(existing.Timestamp) {
			m.values[key] = incoming
		}
	case PolicyHighestVersion:
		incomingSum := clockSum(incoming.Clock)
		existingSum := clockSum(existing.Clock)
		if incomingSum > existingSum {
			m.values[key] = incoming
		}
	}

	// Merge clocks
	merged := existing.Clock.Merge(incoming.Clock)
	m.values[key].Clock = merged

	return nil
}

// DrainRegion puts a region in draining mode (no new traffic).
func (m *Manager) DrainRegion(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	r, exists := m.regions[id]
	if !exists {
		return ErrRegionNotFound
	}
	r.Status = RegionDraining
	return nil
}

// ActivateRegion restores a region to active status.
func (m *Manager) ActivateRegion(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	r, exists := m.regions[id]
	if !exists {
		return ErrRegionNotFound
	}
	r.Status = RegionActive
	r.LastSeen = time.Now()
	return nil
}

// GetPendingEvents returns undelivered replication events.
func (m *Manager) GetPendingEvents() []ReplicationEvent {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var pending []ReplicationEvent
	for _, e := range m.pending {
		if !e.Delivered {
			pending = append(pending, e)
		}
	}
	return pending
}

// MarkDelivered marks replication events as delivered for a target.
func (m *Manager) MarkDelivered(targetID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i := range m.pending {
		if m.pending[i].TargetID == targetID && !m.pending[i].Delivered {
			m.pending[i].Delivered = true
		}
	}
}

// Stats returns replication statistics.
func (m *Manager) Stats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	active := 0
	for _, r := range m.regions {
		if r.Status == RegionActive {
			active++
		}
	}

	pending := 0
	for _, e := range m.pending {
		if !e.Delivered {
			pending++
		}
	}

	return map[string]interface{}{
		"local_region":   m.config.LocalRegion,
		"consistency":    string(m.config.Consistency),
		"total_regions":  len(m.regions),
		"active_regions": active,
		"total_keys":     len(m.values),
		"pending_events": pending,
	}
}

// Start begins the replication loop.
func (m *Manager) Start(ctx context.Context) {
	go m.replicationLoop(ctx)
}

// Stop halts the replication loop.
func (m *Manager) Stop() {
	close(m.stopCh)
}

func (m *Manager) replicationLoop(ctx context.Context) {
	ticker := time.NewTicker(m.config.ReplicateInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-m.stopCh:
			return
		case <-ticker.C:
			// In production, this would push pending events to peers
		}
	}
}

func clockSum(vc VectorClock) uint64 {
	var sum uint64
	for _, v := range vc {
		sum += v
	}
	return sum
}
