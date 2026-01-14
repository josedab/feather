// Package cloudservice provides a managed cloud service control plane
// for Feather, including tenant provisioning, auto-scaling, and lifecycle management.
package cloudservice

import (
	"fmt"
	"sync"
	"time"
)

// InstanceStatus represents the lifecycle state of a managed instance.
type InstanceStatus string

const (
	// InstanceStatusProvisioning means the instance is being created.
	InstanceStatusProvisioning InstanceStatus = "provisioning"
	// InstanceStatusRunning means the instance is healthy and serving.
	InstanceStatusRunning InstanceStatus = "running"
	// InstanceStatusScaling means the instance is scaling up or down.
	InstanceStatusScaling InstanceStatus = "scaling"
	// InstanceStatusSuspended means the instance is paused (e.g., billing issue).
	InstanceStatusSuspended InstanceStatus = "suspended"
	// InstanceStatusTerminating means the instance is being destroyed.
	InstanceStatusTerminating InstanceStatus = "terminating"
	// InstanceStatusTerminated means the instance has been destroyed.
	InstanceStatusTerminated InstanceStatus = "terminated"
	// InstanceStatusError means the instance is in a failed state.
	InstanceStatusError InstanceStatus = "error"
)

// InstanceTier defines the compute tier of an instance.
type InstanceTier string

const (
	// InstanceTierFree is the free tier with limited resources.
	InstanceTierFree InstanceTier = "free"
	// InstanceTierStarter is a paid tier for small workloads.
	InstanceTierStarter InstanceTier = "starter"
	// InstanceTierPro is for production workloads.
	InstanceTierPro InstanceTier = "pro"
	// InstanceTierEnterprise is for large-scale deployments.
	InstanceTierEnterprise InstanceTier = "enterprise"
)

// InstanceSpec defines the desired configuration for an instance.
type InstanceSpec struct {
	Tier         InstanceTier `json:"tier"`
	Region       string       `json:"region"`
	VCPUs        int          `json:"vcpus"`
	MemoryGB     int          `json:"memory_gb"`
	StorageGB    int          `json:"storage_gb"`
	Replicas     int          `json:"replicas"`
	AutoScale    bool         `json:"auto_scale"`
	MinReplicas  int          `json:"min_replicas"`
	MaxReplicas  int          `json:"max_replicas"`
	MultiRegion  bool         `json:"multi_region"`
	VPCPeering   bool         `json:"vpc_peering"`
	CustomDomain string       `json:"custom_domain,omitempty"`
}

// DefaultSpecForTier returns the default spec for a tier.
func DefaultSpecForTier(tier InstanceTier) InstanceSpec {
	switch tier {
	case InstanceTierFree:
		return InstanceSpec{Tier: tier, VCPUs: 1, MemoryGB: 1, StorageGB: 5, Replicas: 1}
	case InstanceTierStarter:
		return InstanceSpec{Tier: tier, VCPUs: 2, MemoryGB: 4, StorageGB: 50, Replicas: 1, AutoScale: true, MinReplicas: 1, MaxReplicas: 3}
	case InstanceTierPro:
		return InstanceSpec{Tier: tier, VCPUs: 4, MemoryGB: 16, StorageGB: 200, Replicas: 3, AutoScale: true, MinReplicas: 2, MaxReplicas: 10}
	case InstanceTierEnterprise:
		return InstanceSpec{Tier: tier, VCPUs: 16, MemoryGB: 64, StorageGB: 1000, Replicas: 3, AutoScale: true, MinReplicas: 3, MaxReplicas: 50, MultiRegion: true, VPCPeering: true}
	default:
		return InstanceSpec{Tier: InstanceTierFree, VCPUs: 1, MemoryGB: 1, StorageGB: 5, Replicas: 1}
	}
}

// Instance represents a managed Feather instance.
type Instance struct {
	ID          string            `json:"id"`
	TenantID    string            `json:"tenant_id"`
	Name        string            `json:"name"`
	Spec        InstanceSpec      `json:"spec"`
	Status      InstanceStatus    `json:"status"`
	Endpoint    string            `json:"endpoint,omitempty"`
	GRPCAddr    string            `json:"grpc_addr,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
	Labels      map[string]string `json:"labels,omitempty"`
	Metrics     *InstanceMetrics  `json:"metrics,omitempty"`
	LastError   string            `json:"last_error,omitempty"`
}

// InstanceMetrics tracks runtime metrics for an instance.
type InstanceMetrics struct {
	CPUUsagePct     float64 `json:"cpu_usage_pct"`
	MemoryUsagePct  float64 `json:"memory_usage_pct"`
	StorageUsedGB   float64 `json:"storage_used_gb"`
	RequestsPerSec  float64 `json:"requests_per_sec"`
	P99LatencyMs    float64 `json:"p99_latency_ms"`
	ActiveReplicas  int     `json:"active_replicas"`
	FeatureCount    int64   `json:"feature_count"`
	EntityCount     int64   `json:"entity_count"`
}

// ScaleDecision represents an auto-scaling decision.
type ScaleDecision struct {
	InstanceID   string    `json:"instance_id"`
	CurrentScale int       `json:"current_scale"`
	DesiredScale int       `json:"desired_scale"`
	Reason       string    `json:"reason"`
	Timestamp    time.Time `json:"timestamp"`
}

// ControlPlane manages the lifecycle of Feather Cloud instances.
type ControlPlane struct {
	instances map[string]*Instance
	history   []ScaleDecision
	mu        sync.RWMutex
}

// NewControlPlane creates a new cloud service control plane.
func NewControlPlane() *ControlPlane {
	return &ControlPlane{
		instances: make(map[string]*Instance),
	}
}

// Provision creates a new managed instance.
func (cp *ControlPlane) Provision(tenantID, name string, spec InstanceSpec) (*Instance, error) {
	if tenantID == "" {
		return nil, fmt.Errorf("tenant ID is required")
	}
	if name == "" {
		return nil, fmt.Errorf("instance name is required")
	}

	cp.mu.Lock()
	defer cp.mu.Unlock()

	id := fmt.Sprintf("inst-%s-%d", tenantID, time.Now().UnixNano())
	now := time.Now()

	inst := &Instance{
		ID:        id,
		TenantID:  tenantID,
		Name:      name,
		Spec:      spec,
		Status:    InstanceStatusProvisioning,
		Endpoint:  fmt.Sprintf("https://%s.feather.cloud", name),
		GRPCAddr:  fmt.Sprintf("%s.grpc.feather.cloud:443", name),
		CreatedAt: now,
		UpdatedAt: now,
		Labels:    make(map[string]string),
	}

	cp.instances[id] = inst
	return inst, nil
}

// Get retrieves an instance by ID.
func (cp *ControlPlane) Get(id string) (*Instance, error) {
	cp.mu.RLock()
	defer cp.mu.RUnlock()

	inst, ok := cp.instances[id]
	if !ok {
		return nil, fmt.Errorf("instance %q not found", id)
	}
	return inst, nil
}

// List returns all instances, optionally filtered by tenant.
func (cp *ControlPlane) List(tenantID string) []*Instance {
	cp.mu.RLock()
	defer cp.mu.RUnlock()

	var result []*Instance
	for _, inst := range cp.instances {
		if tenantID != "" && inst.TenantID != tenantID {
			continue
		}
		if inst.Status != InstanceStatusTerminated {
			result = append(result, inst)
		}
	}
	return result
}

// Scale adjusts the replica count for an instance.
func (cp *ControlPlane) Scale(id string, replicas int) error {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	inst, ok := cp.instances[id]
	if !ok {
		return fmt.Errorf("instance %q not found", id)
	}

	if inst.Spec.AutoScale {
		if replicas < inst.Spec.MinReplicas {
			replicas = inst.Spec.MinReplicas
		}
		if replicas > inst.Spec.MaxReplicas {
			replicas = inst.Spec.MaxReplicas
		}
	}

	old := inst.Spec.Replicas
	inst.Spec.Replicas = replicas
	inst.Status = InstanceStatusScaling
	inst.UpdatedAt = time.Now()

	cp.history = append(cp.history, ScaleDecision{
		InstanceID:   id,
		CurrentScale: old,
		DesiredScale: replicas,
		Reason:       "manual",
		Timestamp:    time.Now(),
	})
	return nil
}

// UpdateStatus sets the status of an instance.
func (cp *ControlPlane) UpdateStatus(id string, status InstanceStatus) error {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	inst, ok := cp.instances[id]
	if !ok {
		return fmt.Errorf("instance %q not found", id)
	}
	inst.Status = status
	inst.UpdatedAt = time.Now()
	return nil
}

// UpdateMetrics updates the runtime metrics for an instance.
func (cp *ControlPlane) UpdateMetrics(id string, metrics *InstanceMetrics) error {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	inst, ok := cp.instances[id]
	if !ok {
		return fmt.Errorf("instance %q not found", id)
	}
	inst.Metrics = metrics
	inst.UpdatedAt = time.Now()
	return nil
}

// EvaluateAutoScale checks if any instances need scaling.
func (cp *ControlPlane) EvaluateAutoScale() []ScaleDecision {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	var decisions []ScaleDecision
	for _, inst := range cp.instances {
		if !inst.Spec.AutoScale || inst.Metrics == nil || inst.Status != InstanceStatusRunning {
			continue
		}

		desired := inst.Spec.Replicas

		// Scale up if CPU > 70% or memory > 80%
		if inst.Metrics.CPUUsagePct > 70 || inst.Metrics.MemoryUsagePct > 80 {
			desired = inst.Spec.Replicas + 1
		}

		// Scale down if both CPU < 30% and memory < 40%
		if inst.Metrics.CPUUsagePct < 30 && inst.Metrics.MemoryUsagePct < 40 && inst.Spec.Replicas > inst.Spec.MinReplicas {
			desired = inst.Spec.Replicas - 1
		}

		// Enforce bounds
		if desired < inst.Spec.MinReplicas {
			desired = inst.Spec.MinReplicas
		}
		if desired > inst.Spec.MaxReplicas {
			desired = inst.Spec.MaxReplicas
		}

		if desired != inst.Spec.Replicas {
			decision := ScaleDecision{
				InstanceID:   inst.ID,
				CurrentScale: inst.Spec.Replicas,
				DesiredScale: desired,
				Reason:       "auto_scale",
				Timestamp:    time.Now(),
			}
			decisions = append(decisions, decision)
			inst.Spec.Replicas = desired
			inst.Status = InstanceStatusScaling
			inst.UpdatedAt = time.Now()
		}
	}

	cp.history = append(cp.history, decisions...)
	return decisions
}

// Terminate destroys a managed instance.
func (cp *ControlPlane) Terminate(id string) error {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	inst, ok := cp.instances[id]
	if !ok {
		return fmt.Errorf("instance %q not found", id)
	}
	inst.Status = InstanceStatusTerminated
	inst.UpdatedAt = time.Now()
	return nil
}

// ScaleHistory returns recent scale decisions.
func (cp *ControlPlane) ScaleHistory(limit int) []ScaleDecision {
	cp.mu.RLock()
	defer cp.mu.RUnlock()

	if limit <= 0 || limit > len(cp.history) {
		limit = len(cp.history)
	}
	start := len(cp.history) - limit
	result := make([]ScaleDecision, limit)
	copy(result, cp.history[start:])
	return result
}

// Stats returns control plane statistics.
func (cp *ControlPlane) Stats() map[string]interface{} {
	cp.mu.RLock()
	defer cp.mu.RUnlock()

	running := 0
	total := 0
	for _, inst := range cp.instances {
		if inst.Status != InstanceStatusTerminated {
			total++
		}
		if inst.Status == InstanceStatusRunning {
			running++
		}
	}
	return map[string]interface{}{
		"total_instances":   total,
		"running_instances": running,
		"scale_decisions":   len(cp.history),
	}
}
