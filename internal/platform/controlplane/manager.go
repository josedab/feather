package controlplane

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// InstanceStatus represents the current state of a Feather instance.
type InstanceStatus string

const (
	InstanceStatusProvisioning   InstanceStatus = "provisioning"
	InstanceStatusHealthy        InstanceStatus = "healthy"
	InstanceStatusDegraded       InstanceStatus = "degraded"
	InstanceStatusUnhealthy      InstanceStatus = "unhealthy"
	InstanceStatusDecommissioned InstanceStatus = "decommissioned"
)

// ReplicationMode defines how data is replicated across regions.
type ReplicationMode string

const (
	ReplicationSync  ReplicationMode = "sync"
	ReplicationAsync ReplicationMode = "async"
	ReplicationNone  ReplicationMode = "none"
)

// ManagerConfig holds configuration for the control plane Manager.
type ManagerConfig struct {
	MaxInstances        int             `json:"max_instances"`
	MaxRegions          int             `json:"max_regions"`
	HealthCheckInterval time.Duration   `json:"health_check_interval"`
	ReplicationMode     ReplicationMode `json:"replication_mode"`
	DefaultPolicy       string          `json:"default_policy"`
}

// DefaultManagerConfig returns a ManagerConfig with sensible defaults.
func DefaultManagerConfig() ManagerConfig {
	return ManagerConfig{
		MaxInstances:        100,
		MaxRegions:          10,
		HealthCheckInterval: 30 * time.Second,
		ReplicationMode:     ReplicationAsync,
		DefaultPolicy:       "default",
	}
}

// Instance represents a single running Feather feature store instance.
type Instance struct {
	ID              string            `json:"id"`
	Name            string            `json:"name"`
	Region          string            `json:"region"`
	Endpoint        string            `json:"endpoint"`
	Status          InstanceStatus    `json:"status"`
	Version         string            `json:"version"`
	Config          map[string]string `json:"config,omitempty"`
	Metrics         *InstanceMetrics  `json:"metrics"`
	Tags            map[string]string `json:"tags,omitempty"`
	CreatedAt       time.Time         `json:"created_at"`
	UpdatedAt       time.Time         `json:"updated_at"`
	LastHealthCheck time.Time         `json:"last_health_check"`
}

// InstanceMetrics holds runtime metrics for a Feather instance.
type InstanceMetrics struct {
	CPUUsage       float64 `json:"cpu_usage"`
	MemoryUsage    float64 `json:"memory_usage"`
	RequestsPerSec float64 `json:"requests_per_sec"`
	AvgLatencyMs   float64 `json:"avg_latency_ms"`
	ErrorRate      float64 `json:"error_rate"`
	FeatureCount   int64   `json:"feature_count"`
	EntityCount    int64   `json:"entity_count"`
}

// Region represents a cloud provider region hosting Feather instances.
type Region struct {
	Name      string   `json:"name"`
	Provider  string   `json:"provider"`
	Instances []string `json:"instances"`
	Primary   bool     `json:"primary"`
}

// Policy defines deployment and replication rules for a set of regions.
type Policy struct {
	Name            string            `json:"name"`
	ReplicationMode ReplicationMode   `json:"replication_mode"`
	Regions         []string          `json:"regions"`
	MaxInstances    int               `json:"max_instances"`
	AutoScale       bool              `json:"auto_scale"`
	Tags            map[string]string `json:"tags,omitempty"`
}

// FleetStatus provides a summary of all instances across the control plane.
type FleetStatus struct {
	TotalInstances     int             `json:"total_instances"`
	HealthyInstances   int             `json:"healthy_instances"`
	DegradedInstances  int             `json:"degraded_instances"`
	UnhealthyInstances int             `json:"unhealthy_instances"`
	TotalRegions       int             `json:"total_regions"`
	TotalPolicies      int             `json:"total_policies"`
	ReplicationMode    ReplicationMode `json:"replication_mode"`
}

// Manager is the central coordinator for managing Feather instances across
// multiple clouds and regions.
type Manager struct {
	config      ManagerConfig
	instances   map[string]*Instance
	regions     map[string]*Region
	policies    map[string]*Policy
	replication *ReplicationManager
	mu          sync.RWMutex
	ctx         context.Context
	cancel      context.CancelFunc
}

// NewManager creates a new control plane Manager with the given configuration.
func NewManager(ctx context.Context, config ManagerConfig) *Manager {
	ctx, cancel := context.WithCancel(ctx)
	return &Manager{
		config:      config,
		instances:   make(map[string]*Instance),
		regions:     make(map[string]*Region),
		policies:    make(map[string]*Policy),
		replication: NewReplicationManager(DefaultReplicationConfig()),
		ctx:         ctx,
		cancel:      cancel,
	}
}

// RegisterInstance adds a new Feather instance to the control plane. An ID is
// assigned automatically if not set. The instance status is set to provisioning.
func (m *Manager) RegisterInstance(ctx context.Context, inst *Instance) error {
	if inst == nil {
		return errors.New("instance must not be nil")
	}
	if inst.Name == "" {
		return errors.New("instance name is required")
	}
	if inst.Region == "" {
		return errors.New("instance region is required")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.instances) >= m.config.MaxInstances {
		return fmt.Errorf("registering instance %q: maximum instances (%d) reached", inst.Name, m.config.MaxInstances)
	}

	if _, ok := m.regions[inst.Region]; !ok {
		return fmt.Errorf("registering instance %q: region %q not found", inst.Name, inst.Region)
	}

	if inst.ID == "" {
		inst.ID = uuid.New().String()
	}

	now := time.Now()
	inst.Status = InstanceStatusProvisioning
	inst.CreatedAt = now
	inst.UpdatedAt = now

	m.instances[inst.ID] = inst

	// Track the instance in its region.
	region := m.regions[inst.Region]
	region.Instances = append(region.Instances, inst.ID)

	return nil
}

// DeregisterInstance removes an instance from the control plane by ID.
func (m *Manager) DeregisterInstance(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	inst, ok := m.instances[id]
	if !ok {
		return fmt.Errorf("deregistering instance: instance %q not found", id)
	}

	// Remove the instance from its region's list.
	if region, ok := m.regions[inst.Region]; ok {
		for i, iid := range region.Instances {
			if iid == id {
				region.Instances = append(region.Instances[:i], region.Instances[i+1:]...)
				break
			}
		}
	}

	delete(m.instances, id)
	return nil
}

// GetInstance returns an instance by its ID.
func (m *Manager) GetInstance(ctx context.Context, id string) (*Instance, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	inst, ok := m.instances[id]
	if !ok {
		return nil, fmt.Errorf("getting instance: instance %q not found", id)
	}
	return inst, nil
}

// ListInstances returns all registered instances.
func (m *Manager) ListInstances(ctx context.Context) []*Instance {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Instance, 0, len(m.instances))
	for _, inst := range m.instances {
		result = append(result, inst)
	}
	return result
}

// ListInstancesByRegion returns all instances in the given region.
func (m *Manager) ListInstancesByRegion(ctx context.Context, region string) []*Instance {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*Instance
	for _, inst := range m.instances {
		if inst.Region == region {
			result = append(result, inst)
		}
	}
	return result
}

// UpdateInstanceStatus changes the status of an instance.
func (m *Manager) UpdateInstanceStatus(ctx context.Context, id string, status InstanceStatus) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	inst, ok := m.instances[id]
	if !ok {
		return fmt.Errorf("updating instance status: instance %q not found", id)
	}

	inst.Status = status
	inst.UpdatedAt = time.Now()
	return nil
}

// UpdateInstanceMetrics updates the runtime metrics for an instance.
func (m *Manager) UpdateInstanceMetrics(ctx context.Context, id string, metrics *InstanceMetrics) error {
	if metrics == nil {
		return errors.New("metrics must not be nil")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	inst, ok := m.instances[id]
	if !ok {
		return fmt.Errorf("updating instance metrics: instance %q not found", id)
	}

	inst.Metrics = metrics
	inst.UpdatedAt = time.Now()
	return nil
}

// AddRegion registers a new cloud region in the control plane.
func (m *Manager) AddRegion(ctx context.Context, region *Region) error {
	if region == nil {
		return errors.New("region must not be nil")
	}
	if region.Name == "" {
		return errors.New("region name is required")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.regions) >= m.config.MaxRegions {
		return fmt.Errorf("adding region %q: maximum regions (%d) reached", region.Name, m.config.MaxRegions)
	}

	if _, ok := m.regions[region.Name]; ok {
		return fmt.Errorf("adding region: region %q already exists", region.Name)
	}

	if region.Instances == nil {
		region.Instances = []string{}
	}

	m.regions[region.Name] = region
	return nil
}

// ListRegions returns all registered regions.
func (m *Manager) ListRegions(ctx context.Context) []*Region {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Region, 0, len(m.regions))
	for _, r := range m.regions {
		result = append(result, r)
	}
	return result
}

// AddPolicy registers a deployment policy.
func (m *Manager) AddPolicy(ctx context.Context, policy *Policy) error {
	if policy == nil {
		return errors.New("policy must not be nil")
	}
	if policy.Name == "" {
		return errors.New("policy name is required")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.policies[policy.Name] = policy
	return nil
}

// GetPolicy returns a policy by name.
func (m *Manager) GetPolicy(ctx context.Context, name string) (*Policy, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	policy, ok := m.policies[name]
	if !ok {
		return nil, fmt.Errorf("getting policy: policy %q not found", name)
	}
	return policy, nil
}

// ListPolicies returns all registered policies.
func (m *Manager) ListPolicies(ctx context.Context) []*Policy {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Policy, 0, len(m.policies))
	for _, p := range m.policies {
		result = append(result, p)
	}
	return result
}

// GetFleetStatus returns an aggregated status summary of the entire fleet.
func (m *Manager) GetFleetStatus(ctx context.Context) *FleetStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	status := &FleetStatus{
		TotalInstances:  len(m.instances),
		TotalRegions:    len(m.regions),
		TotalPolicies:   len(m.policies),
		ReplicationMode: m.config.ReplicationMode,
	}

	for _, inst := range m.instances {
		switch inst.Status {
		case InstanceStatusHealthy:
			status.HealthyInstances++
		case InstanceStatusDegraded:
			status.DegradedInstances++
		case InstanceStatusUnhealthy:
			status.UnhealthyInstances++
		}
	}

	return status
}

// Close shuts down the control plane manager and releases resources.
func (m *Manager) Close() error {
	m.cancel()
	return nil
}
