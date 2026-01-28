package saascontrol

import (
	"fmt"
	"sync"
	"time"
)

// Plan represents a subscription plan.
type Plan struct {
	ID                string  `json:"id"`
	Name              string  `json:"name"`
	MaxInstances      int     `json:"max_instances"`
	MaxFeatures       int     `json:"max_features"`
	MaxRequestsPerSec int     `json:"max_requests_per_sec"`
	MaxStorageGB      int     `json:"max_storage_gb"`
	PricePerMonth     float64 `json:"price_per_month"`
}

// Tenant represents a customer tenant.
type Tenant struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	PlanID    string    `json:"plan_id"`
	Status    string    `json:"status"` // active, suspended, terminated
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Instance represents a provisioned Feather instance for a tenant.
type Instance struct {
	ID        string    `json:"id"`
	TenantID  string    `json:"tenant_id"`
	Region    string    `json:"region"`
	Status    string    `json:"status"` // provisioning, running, stopped, terminated
	Replicas  int       `json:"replicas"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// UsageRecord tracks resource consumption.
type UsageRecord struct {
	TenantID   string    `json:"tenant_id"`
	Requests   int64     `json:"requests"`
	Features   int       `json:"features"`
	StorageMB  int64     `json:"storage_mb"`
	Instances  int       `json:"instances"`
	Period     string    `json:"period"`
	RecordedAt time.Time `json:"recorded_at"`
}

// Invoice represents a billing invoice.
type Invoice struct {
	ID        string    `json:"id"`
	TenantID  string    `json:"tenant_id"`
	PlanID    string    `json:"plan_id"`
	Amount    float64   `json:"amount"`
	Period    string    `json:"period"`
	Status    string    `json:"status"` // pending, paid, overdue
	CreatedAt time.Time `json:"created_at"`
}

// ControlPlaneConfig configures the SaaS control plane.
type ControlPlaneConfig struct {
	MaxTenants    int    `json:"max_tenants"`
	MaxInstances  int    `json:"max_instances_total"`
	DefaultRegion string `json:"default_region"`
}

// DefaultControlPlaneConfig returns sensible defaults.
func DefaultControlPlaneConfig() ControlPlaneConfig {
	return ControlPlaneConfig{
		MaxTenants:    10000,
		MaxInstances:  50000,
		DefaultRegion: "us-east-1",
	}
}

// ControlPlane manages the multi-tenant SaaS environment.
type ControlPlane struct {
	mu        sync.RWMutex
	config    ControlPlaneConfig
	plans     map[string]*Plan
	tenants   map[string]*Tenant
	instances map[string]*Instance
	usage     map[string]*UsageRecord // tenant_id -> current usage
	invoices  []Invoice
}

// NewControlPlane creates a new SaaS control plane.
func NewControlPlane(config ControlPlaneConfig) *ControlPlane {
	if config.MaxTenants == 0 {
		config = DefaultControlPlaneConfig()
	}

	cp := &ControlPlane{
		config:    config,
		plans:     make(map[string]*Plan),
		tenants:   make(map[string]*Tenant),
		instances: make(map[string]*Instance),
		usage:     make(map[string]*UsageRecord),
		invoices:  make([]Invoice, 0),
	}

	// Register default plans
	cp.plans["free"] = &Plan{ID: "free", Name: "Free", MaxInstances: 1, MaxFeatures: 100, MaxRequestsPerSec: 10, MaxStorageGB: 1, PricePerMonth: 0}
	cp.plans["starter"] = &Plan{ID: "starter", Name: "Starter", MaxInstances: 3, MaxFeatures: 1000, MaxRequestsPerSec: 100, MaxStorageGB: 10, PricePerMonth: 49}
	cp.plans["pro"] = &Plan{ID: "pro", Name: "Pro", MaxInstances: 10, MaxFeatures: 10000, MaxRequestsPerSec: 1000, MaxStorageGB: 100, PricePerMonth: 299}
	cp.plans["enterprise"] = &Plan{ID: "enterprise", Name: "Enterprise", MaxInstances: 100, MaxFeatures: 100000, MaxRequestsPerSec: 10000, MaxStorageGB: 1000, PricePerMonth: 999}

	return cp
}

// ListPlans returns all available plans.
func (cp *ControlPlane) ListPlans() []Plan {
	cp.mu.RLock()
	defer cp.mu.RUnlock()

	result := make([]Plan, 0, len(cp.plans))
	for _, p := range cp.plans {
		result = append(result, *p)
	}
	return result
}

// CreateTenant creates a new tenant.
func (cp *ControlPlane) CreateTenant(id, name, email, planID string) (*Tenant, error) {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	if _, exists := cp.tenants[id]; exists {
		return nil, ErrTenantExists
	}

	if _, exists := cp.plans[planID]; !exists {
		return nil, ErrInvalidPlan
	}

	if len(cp.tenants) >= cp.config.MaxTenants {
		return nil, fmt.Errorf("max tenants reached (%d)", cp.config.MaxTenants)
	}

	now := time.Now()
	tenant := &Tenant{
		ID:        id,
		Name:      name,
		Email:     email,
		PlanID:    planID,
		Status:    "active",
		CreatedAt: now,
		UpdatedAt: now,
	}

	cp.tenants[id] = tenant
	cp.usage[id] = &UsageRecord{TenantID: id, RecordedAt: now}
	return tenant, nil
}

// GetTenant returns a tenant.
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

// SuspendTenant suspends a tenant.
func (cp *ControlPlane) SuspendTenant(id string) error {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	t, exists := cp.tenants[id]
	if !exists {
		return ErrTenantNotFound
	}
	t.Status = "suspended"
	t.UpdatedAt = time.Now()
	return nil
}

// ProvisionInstance provisions a new instance for a tenant.
func (cp *ControlPlane) ProvisionInstance(tenantID, region string, replicas int) (*Instance, error) {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	tenant, exists := cp.tenants[tenantID]
	if !exists {
		return nil, ErrTenantNotFound
	}

	plan := cp.plans[tenant.PlanID]

	// Count existing instances for tenant
	count := 0
	for _, inst := range cp.instances {
		if inst.TenantID == tenantID && inst.Status != "terminated" {
			count++
		}
	}
	if count >= plan.MaxInstances {
		return nil, fmt.Errorf("%w: max instances for plan %s (%d)", ErrQuotaExceeded, plan.Name, plan.MaxInstances)
	}

	if region == "" {
		region = cp.config.DefaultRegion
	}
	if replicas <= 0 {
		replicas = 1
	}

	now := time.Now()
	instanceID := fmt.Sprintf("%s-%s-%d", tenantID, region, now.UnixMilli())
	inst := &Instance{
		ID:        instanceID,
		TenantID:  tenantID,
		Region:    region,
		Status:    "provisioning",
		Replicas:  replicas,
		CreatedAt: now,
		UpdatedAt: now,
	}

	cp.instances[instanceID] = inst

	// Simulate quick provisioning
	inst.Status = "running"

	return inst, nil
}

// GetInstance returns an instance.
func (cp *ControlPlane) GetInstance(id string) (*Instance, error) {
	cp.mu.RLock()
	defer cp.mu.RUnlock()

	inst, exists := cp.instances[id]
	if !exists {
		return nil, ErrInstanceNotFound
	}
	return inst, nil
}

// ListInstances returns instances for a tenant.
func (cp *ControlPlane) ListInstances(tenantID string) []Instance {
	cp.mu.RLock()
	defer cp.mu.RUnlock()

	var result []Instance
	for _, inst := range cp.instances {
		if tenantID == "" || inst.TenantID == tenantID {
			result = append(result, *inst)
		}
	}
	return result
}

// ScaleInstance changes the replica count.
func (cp *ControlPlane) ScaleInstance(id string, replicas int) error {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	inst, exists := cp.instances[id]
	if !exists {
		return ErrInstanceNotFound
	}

	inst.Replicas = replicas
	inst.UpdatedAt = time.Now()
	return nil
}

// TerminateInstance terminates an instance.
func (cp *ControlPlane) TerminateInstance(id string) error {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	inst, exists := cp.instances[id]
	if !exists {
		return ErrInstanceNotFound
	}

	inst.Status = "terminated"
	inst.UpdatedAt = time.Now()
	return nil
}

// GetUsage returns current usage for a tenant.
func (cp *ControlPlane) GetUsage(tenantID string) (*UsageRecord, error) {
	cp.mu.RLock()
	defer cp.mu.RUnlock()

	u, exists := cp.usage[tenantID]
	if !exists {
		return nil, ErrTenantNotFound
	}
	return u, nil
}

// RecordUsage records usage for a tenant.
func (cp *ControlPlane) RecordUsage(tenantID string, requests int64, features int, storageMB int64) {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	u, exists := cp.usage[tenantID]
	if !exists {
		return
	}
	u.Requests += requests
	u.Features = features
	u.StorageMB = storageMB
	u.RecordedAt = time.Now()
}

// Stats returns aggregate statistics.
func (cp *ControlPlane) Stats() ControlPlaneStats {
	cp.mu.RLock()
	defer cp.mu.RUnlock()

	var stats ControlPlaneStats
	stats.TotalTenants = len(cp.tenants)
	stats.TotalInstances = len(cp.instances)
	stats.TotalPlans = len(cp.plans)

	for _, t := range cp.tenants {
		if t.Status == "active" {
			stats.ActiveTenants++
		}
	}
	for _, inst := range cp.instances {
		if inst.Status == "running" {
			stats.RunningInstances++
		}
	}
	return stats
}

// ControlPlaneStats provides aggregate statistics.
type ControlPlaneStats struct {
	TotalTenants     int `json:"total_tenants"`
	ActiveTenants    int `json:"active_tenants"`
	TotalInstances   int `json:"total_instances"`
	RunningInstances int `json:"running_instances"`
	TotalPlans       int `json:"total_plans"`
}
