package cloud

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

// Control plane errors.
var (
	ErrInstanceNotFound      = errors.New("instance not found")
	ErrTenantLimitExceeded   = errors.New("tenant instance limit exceeded")
	ErrInvalidTier           = errors.New("invalid tier")
	ErrInvalidRegion         = errors.New("invalid region")
	ErrInvalidReplicas       = errors.New("replica count exceeds tier limit")
	ErrInstanceNotRunning    = errors.New("instance is not running")
	ErrUsageNotFound         = errors.New("usage record not found")
	ErrInstanceNotTerminated = errors.New("instance cannot be terminated in current state")
)

// Tier represents a pricing tier.
type Tier string

// Tier constants.
const (
	TierFree       Tier = "free"
	TierStarter    Tier = "starter"
	TierPro        Tier = "pro"
	TierEnterprise Tier = "enterprise"
)

// InstanceStatus represents the state of a managed instance.
type InstanceStatus string

// InstanceStatus constants.
const (
	StatusProvisioning InstanceStatus = "provisioning"
	StatusRunning      InstanceStatus = "running"
	StatusScaling      InstanceStatus = "scaling"
	StatusStopped      InstanceStatus = "stopped"
	StatusError        InstanceStatus = "error"
	StatusTerminating  InstanceStatus = "terminating"
)

// TierLimits defines resource limits per tier.
type TierLimits struct {
	MaxReplicas int
	CPULimit    string
	MemoryLimit string
}

// tierLimitsMap maps each tier to its resource limits.
var tierLimitsMap = map[Tier]TierLimits{
	TierFree:       {MaxReplicas: 1, CPULimit: "0.5", MemoryLimit: "512Mi"},
	TierStarter:    {MaxReplicas: 3, CPULimit: "2", MemoryLimit: "4Gi"},
	TierPro:        {MaxReplicas: 10, CPULimit: "8", MemoryLimit: "32Gi"},
	TierEnterprise: {MaxReplicas: 50, CPULimit: "32", MemoryLimit: "128Gi"},
}

// validRegions lists accepted deployment regions.
var validRegions = map[string]bool{
	"us-east-1":      true,
	"us-west-2":      true,
	"eu-west-1":      true,
	"eu-central-1":   true,
	"ap-northeast-1": true,
	"ap-southeast-1": true,
}

// Config holds control plane configuration.
type Config struct {
	MaxInstancesPerTenant int           `json:"max_instances_per_tenant"`
	DefaultTier           Tier          `json:"default_tier"`
	AutoscaleEnabled      bool          `json:"autoscale_enabled"`
	AutoscaleInterval     time.Duration `json:"autoscale_interval"`
	MetricsRetentionDays  int           `json:"metrics_retention_days"`
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() Config {
	return Config{
		MaxInstancesPerTenant: 5,
		DefaultTier:           TierStarter,
		AutoscaleEnabled:      true,
		AutoscaleInterval:     30 * time.Second,
		MetricsRetentionDays:  90,
	}
}

// Instance represents a managed Feather instance.
type Instance struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	TenantID    string          `json:"tenant_id"`
	Tier        Tier            `json:"tier"`
	Status      InstanceStatus  `json:"status"`
	Region      string          `json:"region"`
	Replicas    int             `json:"replicas"`
	MaxReplicas int             `json:"max_replicas"`
	CPULimit    string          `json:"cpu_limit"`
	MemoryLimit string          `json:"memory_limit"`
	Endpoint    string          `json:"endpoint"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
	Metrics     InstanceMetrics `json:"metrics"`
}

// InstanceMetrics holds runtime metrics for an instance.
type InstanceMetrics struct {
	CPUUsage       float64 `json:"cpu_usage_pct"`
	MemoryUsage    float64 `json:"memory_usage_pct"`
	RequestsPerSec float64 `json:"requests_per_sec"`
	P99LatencyMs   float64 `json:"p99_latency_ms"`
	ErrorRate      float64 `json:"error_rate_pct"`
	StorageUsedGB  float64 `json:"storage_used_gb"`
}

// ProvisionRequest describes a new instance to create.
type ProvisionRequest struct {
	Name     string `json:"name"`
	TenantID string `json:"tenant_id"`
	Tier     Tier   `json:"tier"`
	Region   string `json:"region"`
}

// ScaleRequest describes a scaling operation.
type ScaleRequest struct {
	Replicas    int    `json:"replicas"`
	CPULimit    string `json:"cpu_limit,omitempty"`
	MemoryLimit string `json:"memory_limit,omitempty"`
}

// UsageMeter tracks per-tenant resource usage for billing.
type UsageMeter struct {
	TenantID       string    `json:"tenant_id"`
	FeatureReads   int64     `json:"feature_reads"`
	FeatureWrites  int64     `json:"feature_writes"`
	StorageBytes   int64     `json:"storage_bytes"`
	ComputeMinutes float64   `json:"compute_minutes"`
	PeriodStart    time.Time `json:"period_start"`
	PeriodEnd      time.Time `json:"period_end"`
	EstimatedCost  float64   `json:"estimated_cost_usd"`
}

// Autoscaler monitors instances and scales based on metric rules.
type Autoscaler struct {
	mu        sync.Mutex
	rules     []ScaleRule
	cooldown  time.Duration
	lastScale map[string]time.Time
}

// ScaleRule defines a threshold-based scaling rule.
type ScaleRule struct {
	Metric    string  `json:"metric"`
	Threshold float64 `json:"threshold"`
	ScaleUp   int     `json:"scale_up"`
	ScaleDown int     `json:"scale_down"`
}

// ScaleAction represents a recommended scaling action.
type ScaleAction struct {
	InstanceID  string `json:"instance_id"`
	CurrentReps int    `json:"current_replicas"`
	DesiredReps int    `json:"desired_replicas"`
	Reason      string `json:"reason"`
}

// ControlPlaneStats holds aggregate statistics.
type ControlPlaneStats struct {
	TotalInstances int            `json:"total_instances"`
	RunningCount   int            `json:"running_count"`
	ByTier         map[string]int `json:"by_tier"`
	ByRegion       map[string]int `json:"by_region"`
	TotalTenants   int            `json:"total_tenants"`
}

// ControlPlane manages multi-tenant Feather instances.
type ControlPlane struct {
	config     Config
	mu         sync.RWMutex
	instances  map[string]*Instance
	usage      map[string]*UsageMeter
	autoscaler *Autoscaler
	idCounter  int64
}

// NewControlPlane creates a new control plane with the given configuration.
func NewControlPlane(cfg Config) *ControlPlane {
	return &ControlPlane{
		config:    cfg,
		instances: make(map[string]*Instance),
		usage:     make(map[string]*UsageMeter),
		autoscaler: &Autoscaler{
			rules: []ScaleRule{
				{Metric: "cpu_usage_pct", Threshold: 80, ScaleUp: 1, ScaleDown: 0},
				{Metric: "memory_usage_pct", Threshold: 85, ScaleUp: 1, ScaleDown: 0},
				{Metric: "requests_per_sec", Threshold: 1000, ScaleUp: 2, ScaleDown: 0},
			},
			cooldown:  2 * time.Minute,
			lastScale: make(map[string]time.Time),
		},
	}
}

func (cp *ControlPlane) nextID() string {
	cp.idCounter++
	return fmt.Sprintf("inst_%s_%d", time.Now().Format("20060102"), cp.idCounter)
}

// Provision creates a new managed instance.
func (cp *ControlPlane) Provision(req ProvisionRequest) (*Instance, error) {
	if req.Name == "" {
		return nil, fmt.Errorf("provisioning instance: name is required")
	}
	if req.TenantID == "" {
		return nil, fmt.Errorf("provisioning instance: tenant_id is required")
	}

	limits, ok := tierLimitsMap[req.Tier]
	if !ok {
		return nil, fmt.Errorf("provisioning instance: %w", ErrInvalidTier)
	}

	if !validRegions[req.Region] {
		return nil, fmt.Errorf("provisioning instance: %w", ErrInvalidRegion)
	}

	cp.mu.Lock()
	defer cp.mu.Unlock()

	// Check tenant instance limit.
	count := 0
	for _, inst := range cp.instances {
		if inst.TenantID == req.TenantID && inst.Status != StatusTerminating {
			count++
		}
	}
	if count >= cp.config.MaxInstancesPerTenant {
		return nil, fmt.Errorf("provisioning instance: %w", ErrTenantLimitExceeded)
	}

	now := time.Now()
	id := cp.nextID()

	instance := &Instance{
		ID:          id,
		Name:        req.Name,
		TenantID:    req.TenantID,
		Tier:        req.Tier,
		Status:      StatusProvisioning,
		Region:      req.Region,
		Replicas:    1,
		MaxReplicas: limits.MaxReplicas,
		CPULimit:    limits.CPULimit,
		MemoryLimit: limits.MemoryLimit,
		Endpoint:    fmt.Sprintf("https://%s.feather.io", id),
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	cp.instances[id] = instance

	// Initialise usage meter for the tenant if absent.
	if _, exists := cp.usage[req.TenantID]; !exists {
		cp.usage[req.TenantID] = &UsageMeter{
			TenantID:    req.TenantID,
			PeriodStart: now,
			PeriodEnd:   now.AddDate(0, 1, 0),
		}
	}

	instanceCopy := *instance
	return &instanceCopy, nil
}

// GetInstance retrieves an instance by ID.
func (cp *ControlPlane) GetInstance(id string) (*Instance, error) {
	cp.mu.RLock()
	defer cp.mu.RUnlock()

	instance, exists := cp.instances[id]
	if !exists {
		return nil, fmt.Errorf("getting instance %s: %w", id, ErrInstanceNotFound)
	}
	instanceCopy := *instance
	return &instanceCopy, nil
}

// ListInstances returns all instances belonging to a tenant.
func (cp *ControlPlane) ListInstances(tenantID string) []*Instance {
	cp.mu.RLock()
	defer cp.mu.RUnlock()

	result := make([]*Instance, 0)
	for _, inst := range cp.instances {
		if inst.TenantID == tenantID {
			copyInst := *inst
			result = append(result, &copyInst)
		}
	}
	return result
}

// Scale adjusts the replica count and optionally resource limits.
func (cp *ControlPlane) Scale(id string, req ScaleRequest) (*Instance, error) {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	instance, exists := cp.instances[id]
	if !exists {
		return nil, fmt.Errorf("scaling instance %s: %w", id, ErrInstanceNotFound)
	}

	if instance.Status != StatusRunning && instance.Status != StatusProvisioning {
		return nil, fmt.Errorf("scaling instance %s: %w", id, ErrInstanceNotRunning)
	}

	limits := tierLimitsMap[instance.Tier]
	if req.Replicas < 1 || req.Replicas > limits.MaxReplicas {
		return nil, fmt.Errorf("scaling instance %s: %w (max %d for tier %s)",
			id, ErrInvalidReplicas, limits.MaxReplicas, instance.Tier)
	}

	instance.Replicas = req.Replicas
	if req.CPULimit != "" {
		instance.CPULimit = req.CPULimit
	}
	if req.MemoryLimit != "" {
		instance.MemoryLimit = req.MemoryLimit
	}
	instance.Status = StatusScaling
	instance.UpdatedAt = time.Now()

	instanceCopy := *instance
	return &instanceCopy, nil
}

// Terminate marks an instance for termination.
func (cp *ControlPlane) Terminate(id string) error {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	instance, exists := cp.instances[id]
	if !exists {
		return fmt.Errorf("terminating instance %s: %w", id, ErrInstanceNotFound)
	}

	if instance.Status == StatusTerminating {
		return fmt.Errorf("terminating instance %s: %w", id, ErrInstanceNotTerminated)
	}

	instance.Status = StatusTerminating
	instance.UpdatedAt = time.Now()
	return nil
}

// GetUsage returns the usage meter for a tenant.
func (cp *ControlPlane) GetUsage(tenantID string) (*UsageMeter, error) {
	cp.mu.RLock()
	defer cp.mu.RUnlock()

	meter, exists := cp.usage[tenantID]
	if !exists {
		return nil, fmt.Errorf("getting usage for tenant %s: %w", tenantID, ErrUsageNotFound)
	}
	meterCopy := *meter
	return &meterCopy, nil
}

// RecordUsage increments feature read/write counters for a tenant.
func (cp *ControlPlane) RecordUsage(tenantID string, reads, writes int64) {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	meter, exists := cp.usage[tenantID]
	if !exists {
		now := time.Now()
		meter = &UsageMeter{
			TenantID:    tenantID,
			PeriodStart: now,
			PeriodEnd:   now.AddDate(0, 1, 0),
		}
		cp.usage[tenantID] = meter
	}
	meter.FeatureReads += reads
	meter.FeatureWrites += writes

	// Estimate cost: $0.01 per 1000 reads, $0.05 per 1000 writes.
	meter.EstimatedCost = float64(meter.FeatureReads)/1000*0.01 +
		float64(meter.FeatureWrites)/1000*0.05
}

// EvaluateAutoscale checks all running instances against autoscale rules.
func (cp *ControlPlane) EvaluateAutoscale() []ScaleAction {
	cp.mu.RLock()
	defer cp.mu.RUnlock()

	cp.autoscaler.mu.Lock()
	defer cp.autoscaler.mu.Unlock()

	var actions []ScaleAction
	now := time.Now()

	for _, inst := range cp.instances {
		if inst.Status != StatusRunning {
			continue
		}

		// Respect cooldown.
		if last, ok := cp.autoscaler.lastScale[inst.ID]; ok {
			if now.Sub(last) < cp.autoscaler.cooldown {
				continue
			}
		}

		for _, rule := range cp.autoscaler.rules {
			var metricVal float64
			switch rule.Metric {
			case "cpu_usage_pct":
				metricVal = inst.Metrics.CPUUsage
			case "memory_usage_pct":
				metricVal = inst.Metrics.MemoryUsage
			case "requests_per_sec":
				metricVal = inst.Metrics.RequestsPerSec
			default:
				continue
			}

			limits := tierLimitsMap[inst.Tier]
			if metricVal > rule.Threshold && rule.ScaleUp > 0 {
				desired := inst.Replicas + rule.ScaleUp
				if desired > limits.MaxReplicas {
					desired = limits.MaxReplicas
				}
				if desired != inst.Replicas {
					actions = append(actions, ScaleAction{
						InstanceID:  inst.ID,
						CurrentReps: inst.Replicas,
						DesiredReps: desired,
						Reason:      fmt.Sprintf("%s %.1f%% exceeds threshold %.1f%%", rule.Metric, metricVal, rule.Threshold),
					})
					cp.autoscaler.lastScale[inst.ID] = now
				}
			}
		}
	}
	return actions
}

// Stats returns aggregate statistics about the control plane.
func (cp *ControlPlane) Stats() ControlPlaneStats {
	cp.mu.RLock()
	defer cp.mu.RUnlock()

	stats := ControlPlaneStats{
		ByTier:   make(map[string]int),
		ByRegion: make(map[string]int),
	}

	tenants := make(map[string]struct{})
	for _, inst := range cp.instances {
		stats.TotalInstances++
		stats.ByTier[string(inst.Tier)]++
		stats.ByRegion[inst.Region]++
		tenants[inst.TenantID] = struct{}{}

		if inst.Status == StatusRunning {
			stats.RunningCount++
		}
	}
	stats.TotalTenants = len(tenants)
	return stats
}
