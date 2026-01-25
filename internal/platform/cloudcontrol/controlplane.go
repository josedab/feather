package cloudcontrol

import (
	"fmt"
	"sync"
	"time"
)

// InstanceStatus represents the lifecycle state of a managed instance.
type InstanceStatus string

const (
	InstanceProvisioning InstanceStatus = "provisioning"
	InstanceRunning      InstanceStatus = "running"
	InstanceScaling      InstanceStatus = "scaling"
	InstanceStopped      InstanceStatus = "stopped"
	InstanceTerminating  InstanceStatus = "terminating"
	InstanceTerminated   InstanceStatus = "terminated"
	InstanceFailed       InstanceStatus = "failed"
)

// InstanceTier defines the instance size/capabilities.
type InstanceTier string

const (
	TierFree       InstanceTier = "free"
	TierStarter    InstanceTier = "starter"
	TierPro        InstanceTier = "pro"
	TierEnterprise InstanceTier = "enterprise"
)

// Instance represents a managed Feather deployment.
type Instance struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	TenantID    string         `json:"tenant_id"`
	Region      string         `json:"region"`
	Tier        InstanceTier   `json:"tier"`
	Status      InstanceStatus `json:"status"`
	Version     string         `json:"version"`
	Replicas    int            `json:"replicas"`
	CPULimit    string         `json:"cpu_limit"`
	MemoryLimit string         `json:"memory_limit"`
	Endpoint    string         `json:"endpoint,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	Autoscale   *AutoscalePolicy `json:"autoscale,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// AutoscalePolicy defines autoscaling rules for an instance.
type AutoscalePolicy struct {
	Enabled     bool    `json:"enabled"`
	MinReplicas int     `json:"min_replicas"`
	MaxReplicas int     `json:"max_replicas"`
	TargetCPU   float64 `json:"target_cpu_percent"`
	TargetQPS   int     `json:"target_qps,omitempty"`
	ScaleUpCooldown   time.Duration `json:"scale_up_cooldown_ns"`
	ScaleDownCooldown time.Duration `json:"scale_down_cooldown_ns"`
}

// Tenant represents an isolated customer environment.
type Tenant struct {
	ID            string         `json:"id"`
	Name          string         `json:"name"`
	Plan          InstanceTier   `json:"plan"`
	MaxInstances  int            `json:"max_instances"`
	MaxReplicas   int            `json:"max_replicas_per_instance"`
	MaxStorage    string         `json:"max_storage"`
	Instances     []string       `json:"instance_ids"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	ContactEmail  string         `json:"contact_email,omitempty"`
	Metadata      map[string]string `json:"metadata,omitempty"`
}

// ScaleRequest defines a scaling operation.
type ScaleRequest struct {
	Replicas int    `json:"replicas"`
	Reason   string `json:"reason,omitempty"`
}

// ControlPlaneConfig configures the control plane.
type ControlPlaneConfig struct {
	MaxTenantsTotal    int `json:"max_tenants"`
	MaxInstancesTotal  int `json:"max_instances"`
	DefaultReplicas    int `json:"default_replicas"`
	DefaultTier        InstanceTier `json:"default_tier"`
}

// DefaultControlPlaneConfig returns sensible defaults.
func DefaultControlPlaneConfig() ControlPlaneConfig {
	return ControlPlaneConfig{
		MaxTenantsTotal:   1000,
		MaxInstancesTotal: 10000,
		DefaultReplicas:   2,
		DefaultTier:       TierStarter,
	}
}

// ControlPlaneStats holds aggregate statistics.
type ControlPlaneStats struct {
	TotalTenants   int            `json:"total_tenants"`
	TotalInstances int            `json:"total_instances"`
	ByStatus       map[string]int `json:"by_status"`
	ByTier         map[string]int `json:"by_tier"`
	ByRegion       map[string]int `json:"by_region"`
}

// ControlPlane orchestrates instance lifecycle and tenant management.
type ControlPlane struct {
	mu        sync.RWMutex
	config    ControlPlaneConfig
	instances map[string]*Instance
	tenants   map[string]*Tenant
}

// NewControlPlane creates a new control plane.
func NewControlPlane(config ControlPlaneConfig) *ControlPlane {
	if config.MaxTenantsTotal == 0 {
		config = DefaultControlPlaneConfig()
	}
	return &ControlPlane{
		config:    config,
		instances: make(map[string]*Instance),
		tenants:   make(map[string]*Tenant),
	}
}

// CreateTenant registers a new tenant.
func (cp *ControlPlane) CreateTenant(t Tenant) (*Tenant, error) {
	if t.ID == "" || t.Name == "" {
		return nil, fmt.Errorf("%w: id and name are required", ErrInvalidConfig)
	}

	cp.mu.Lock()
	defer cp.mu.Unlock()

	if _, exists := cp.tenants[t.ID]; exists {
		return nil, ErrTenantExists
	}
	if len(cp.tenants) >= cp.config.MaxTenantsTotal {
		return nil, ErrQuotaExceeded
	}

	now := time.Now()
	t.CreatedAt = now
	t.UpdatedAt = now
	if t.MaxInstances == 0 {
		t.MaxInstances = 5
	}
	if t.MaxReplicas == 0 {
		t.MaxReplicas = 10
	}
	if t.Instances == nil {
		t.Instances = []string{}
	}

	cp.tenants[t.ID] = &t
	return &t, nil
}

// GetTenant returns a tenant by ID.
func (cp *ControlPlane) GetTenant(id string) (*Tenant, error) {
	cp.mu.RLock()
	defer cp.mu.RUnlock()

	t, exists := cp.tenants[id]
	if !exists {
		return nil, ErrTenantNotFound
	}
	return t, nil
}

// ListTenants returns all tenants.
func (cp *ControlPlane) ListTenants() []Tenant {
	cp.mu.RLock()
	defer cp.mu.RUnlock()

	result := make([]Tenant, 0, len(cp.tenants))
	for _, t := range cp.tenants {
		result = append(result, *t)
	}
	return result
}

// ProvisionInstance creates a new managed Feather instance.
func (cp *ControlPlane) ProvisionInstance(inst Instance) (*Instance, error) {
	if inst.ID == "" || inst.Name == "" || inst.TenantID == "" {
		return nil, fmt.Errorf("%w: id, name, and tenant_id are required", ErrInvalidConfig)
	}

	cp.mu.Lock()
	defer cp.mu.Unlock()

	if _, exists := cp.instances[inst.ID]; exists {
		return nil, ErrInstanceExists
	}

	tenant, exists := cp.tenants[inst.TenantID]
	if !exists {
		return nil, ErrTenantNotFound
	}

	if len(tenant.Instances) >= tenant.MaxInstances {
		return nil, fmt.Errorf("%w: max instances for tenant %s", ErrQuotaExceeded, inst.TenantID)
	}

	now := time.Now()
	inst.Status = InstanceProvisioning
	inst.CreatedAt = now
	inst.UpdatedAt = now
	if inst.Replicas <= 0 {
		inst.Replicas = cp.config.DefaultReplicas
	}
	if inst.Tier == "" {
		inst.Tier = cp.config.DefaultTier
	}
	if inst.Region == "" {
		inst.Region = "us-east-1"
	}

	// Simulate provisioning completion
	inst.Status = InstanceRunning
	inst.Endpoint = fmt.Sprintf("https://%s.feather.cloud", inst.ID)

	cp.instances[inst.ID] = &inst
	tenant.Instances = append(tenant.Instances, inst.ID)
	tenant.UpdatedAt = now

	return &inst, nil
}

// GetInstance returns an instance by ID.
func (cp *ControlPlane) GetInstance(id string) (*Instance, error) {
	cp.mu.RLock()
	defer cp.mu.RUnlock()

	inst, exists := cp.instances[id]
	if !exists {
		return nil, ErrInstanceNotFound
	}
	return inst, nil
}

// ListInstances returns all instances, optionally filtered by tenant.
func (cp *ControlPlane) ListInstances(tenantID string) []Instance {
	cp.mu.RLock()
	defer cp.mu.RUnlock()

	result := make([]Instance, 0, len(cp.instances))
	for _, inst := range cp.instances {
		if tenantID == "" || inst.TenantID == tenantID {
			result = append(result, *inst)
		}
	}
	return result
}

// ScaleInstance changes the replica count for an instance.
func (cp *ControlPlane) ScaleInstance(id string, req ScaleRequest) (*Instance, error) {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	inst, exists := cp.instances[id]
	if !exists {
		return nil, ErrInstanceNotFound
	}

	if inst.Status != InstanceRunning {
		return nil, ErrInstanceNotReady
	}

	if req.Replicas < 1 {
		return nil, fmt.Errorf("%w: replicas must be >= 1", ErrInvalidConfig)
	}

	// Validate against tenant quota
	tenant := cp.tenants[inst.TenantID]
	if tenant != nil && req.Replicas > tenant.MaxReplicas {
		return nil, fmt.Errorf("%w: max replicas for tenant is %d", ErrQuotaExceeded, tenant.MaxReplicas)
	}

	inst.Replicas = req.Replicas
	inst.UpdatedAt = time.Now()
	return inst, nil
}

// TerminateInstance terminates an instance.
func (cp *ControlPlane) TerminateInstance(id string) error {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	inst, exists := cp.instances[id]
	if !exists {
		return ErrInstanceNotFound
	}

	inst.Status = InstanceTerminated
	inst.UpdatedAt = time.Now()

	// Remove from tenant
	if tenant, ok := cp.tenants[inst.TenantID]; ok {
		for i, iid := range tenant.Instances {
			if iid == id {
				tenant.Instances = append(tenant.Instances[:i], tenant.Instances[i+1:]...)
				break
			}
		}
	}

	delete(cp.instances, id)
	return nil
}

// SetAutoscalePolicy sets the autoscaling policy for an instance.
func (cp *ControlPlane) SetAutoscalePolicy(id string, policy AutoscalePolicy) (*Instance, error) {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	inst, exists := cp.instances[id]
	if !exists {
		return nil, ErrInstanceNotFound
	}

	if policy.MinReplicas < 1 {
		policy.MinReplicas = 1
	}
	if policy.MaxReplicas < policy.MinReplicas {
		policy.MaxReplicas = policy.MinReplicas
	}

	inst.Autoscale = &policy
	inst.UpdatedAt = time.Now()
	return inst, nil
}

// Stats returns aggregate control plane statistics.
func (cp *ControlPlane) Stats() ControlPlaneStats {
	cp.mu.RLock()
	defer cp.mu.RUnlock()

	stats := ControlPlaneStats{
		TotalTenants:   len(cp.tenants),
		TotalInstances: len(cp.instances),
		ByStatus:       make(map[string]int),
		ByTier:         make(map[string]int),
		ByRegion:       make(map[string]int),
	}
	for _, inst := range cp.instances {
		stats.ByStatus[string(inst.Status)]++
		stats.ByTier[string(inst.Tier)]++
		stats.ByRegion[inst.Region]++
	}
	return stats
}
