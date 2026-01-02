package saas

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

// Provisioning errors
var (
	ErrInstanceNotFound      = errors.New("instance not found")
	ErrInstanceExists        = errors.New("instance already exists")
	ErrInvalidRegion         = errors.New("invalid region")
	ErrInvalidInstanceSize   = errors.New("invalid instance size")
	ErrProvisioningFailed    = errors.New("provisioning failed")
	ErrInstanceLimitExceeded = errors.New("instance limit exceeded")
)

// InstanceStatus represents the state of an instance.
type InstanceStatus string

const (
	InstancePending      InstanceStatus = "pending"
	InstanceProvisioning InstanceStatus = "provisioning"
	InstanceRunning      InstanceStatus = "running"
	InstanceStopping     InstanceStatus = "stopping"
	InstanceStopped      InstanceStatus = "stopped"
	InstanceTerminating  InstanceStatus = "terminating"
	InstanceTerminated   InstanceStatus = "terminated"
	InstanceFailed       InstanceStatus = "failed"
)

// InstanceSize defines the compute configuration.
type InstanceSize string

const (
	SizeXSmall  InstanceSize = "xs"      // 0.5 vCPU, 1GB RAM
	SizeSmall   InstanceSize = "small"   // 1 vCPU, 2GB RAM
	SizeMedium  InstanceSize = "medium"  // 2 vCPU, 4GB RAM
	SizeLarge   InstanceSize = "large"   // 4 vCPU, 8GB RAM
	SizeXLarge  InstanceSize = "xlarge"  // 8 vCPU, 16GB RAM
	Size2XLarge InstanceSize = "2xlarge" // 16 vCPU, 32GB RAM
)

// InstanceSizeSpec defines the resources for a size.
var InstanceSizeSpecs = map[InstanceSize]struct {
	VCPUs        int
	MemoryGB     int
	PricePerHour float64
}{
	SizeXSmall:  {VCPUs: 1, MemoryGB: 1, PricePerHour: 0.01},
	SizeSmall:   {VCPUs: 1, MemoryGB: 2, PricePerHour: 0.02},
	SizeMedium:  {VCPUs: 2, MemoryGB: 4, PricePerHour: 0.04},
	SizeLarge:   {VCPUs: 4, MemoryGB: 8, PricePerHour: 0.08},
	SizeXLarge:  {VCPUs: 8, MemoryGB: 16, PricePerHour: 0.16},
	Size2XLarge: {VCPUs: 16, MemoryGB: 32, PricePerHour: 0.32},
}

// Region represents a cloud region.
type Region struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Location  string `json:"location"`
	Provider  string `json:"provider"` // aws, gcp, azure
	Available bool   `json:"available"`
}

// DefaultRegions returns available regions.
var DefaultRegions = []Region{
	{ID: "us-east-1", Name: "US East (N. Virginia)", Location: "Virginia, USA", Provider: "aws", Available: true},
	{ID: "us-west-2", Name: "US West (Oregon)", Location: "Oregon, USA", Provider: "aws", Available: true},
	{ID: "eu-west-1", Name: "EU (Ireland)", Location: "Dublin, Ireland", Provider: "aws", Available: true},
	{ID: "eu-central-1", Name: "EU (Frankfurt)", Location: "Frankfurt, Germany", Provider: "aws", Available: true},
	{ID: "ap-northeast-1", Name: "Asia Pacific (Tokyo)", Location: "Tokyo, Japan", Provider: "aws", Available: true},
	{ID: "ap-southeast-1", Name: "Asia Pacific (Singapore)", Location: "Singapore", Provider: "aws", Available: true},
}

// Instance represents a Feather instance.
type Instance struct {
	ID             string            `json:"id"`
	OrganizationID string            `json:"organization_id"`
	Name           string            `json:"name"`
	Region         string            `json:"region"`
	Size           InstanceSize      `json:"size"`
	Status         InstanceStatus    `json:"status"`
	Version        string            `json:"version"`
	Endpoint       string            `json:"endpoint,omitempty"`
	InternalIP     string            `json:"internal_ip,omitempty"`
	PublicIP       string            `json:"public_ip,omitempty"`
	Config         InstanceConfig    `json:"config"`
	Tags           map[string]string `json:"tags,omitempty"`
	CreatedAt      time.Time         `json:"created_at"`
	UpdatedAt      time.Time         `json:"updated_at"`
	StartedAt      *time.Time        `json:"started_at,omitempty"`
	StoppedAt      *time.Time        `json:"stopped_at,omitempty"`
}

// InstanceConfig holds instance configuration.
type InstanceConfig struct {
	HotStorageGB       int            `json:"hot_storage_gb"`
	WarmStorageGB      int            `json:"warm_storage_gb"`
	EnableVectorSearch bool           `json:"enable_vector_search"`
	EnableDrift        bool           `json:"enable_drift"`
	EnableMetrics      bool           `json:"enable_metrics"`
	CustomDomain       string         `json:"custom_domain,omitempty"`
	TLSEnabled         bool           `json:"tls_enabled"`
	AuthConfig         *AuthConfig    `json:"auth_config,omitempty"`
	NetworkConfig      *NetworkConfig `json:"network_config,omitempty"`
}

// AuthConfig holds authentication settings.
type AuthConfig struct {
	Method     string   `json:"method"` // api_key, oauth, saml
	AllowedIPs []string `json:"allowed_ips,omitempty"`
	SSOEnabled bool     `json:"sso_enabled"`
}

// NetworkConfig holds network settings.
type NetworkConfig struct {
	VPCEnabled         bool     `json:"vpc_enabled"`
	VPCID              string   `json:"vpc_id,omitempty"`
	SubnetIDs          []string `json:"subnet_ids,omitempty"`
	SecurityGroups     []string `json:"security_groups,omitempty"`
	PrivateLinkEnabled bool     `json:"private_link_enabled"`
}

// ProvisioningRequest represents a request to create an instance.
type ProvisioningRequest struct {
	OrganizationID string            `json:"organization_id"`
	Name           string            `json:"name"`
	Region         string            `json:"region"`
	Size           InstanceSize      `json:"size"`
	Version        string            `json:"version,omitempty"`
	Config         InstanceConfig    `json:"config"`
	Tags           map[string]string `json:"tags,omitempty"`
}

// ProvisioningManager manages instance provisioning.
type ProvisioningManager struct {
	planRegistry   *PlanRegistry
	billingManager *BillingManager
	instances      map[string]*Instance
	regions        map[string]Region
	mu             sync.RWMutex
}

// NewProvisioningManager creates a new provisioning manager.
func NewProvisioningManager(planRegistry *PlanRegistry, billingManager *BillingManager) *ProvisioningManager {
	pm := &ProvisioningManager{
		planRegistry:   planRegistry,
		billingManager: billingManager,
		instances:      make(map[string]*Instance),
		regions:        make(map[string]Region),
	}

	// Initialize regions
	for _, r := range DefaultRegions {
		pm.regions[r.ID] = r
	}

	return pm
}

// CreateInstance provisions a new instance.
func (pm *ProvisioningManager) CreateInstance(req *ProvisioningRequest) (*Instance, error) {
	// Validate region
	region, ok := pm.regions[req.Region]
	if !ok || !region.Available {
		return nil, ErrInvalidRegion
	}

	// Validate size
	if _, ok := InstanceSizeSpecs[req.Size]; !ok {
		return nil, ErrInvalidInstanceSize
	}

	// Check instance limits based on subscription
	subs := pm.billingManager.GetSubscriptionByOrg(req.OrganizationID)
	if len(subs) > 0 {
		plan, err := pm.planRegistry.GetPlan(subs[0].PlanID)
		if err == nil && plan.Quotas.MaxInstances > 0 {
			currentCount := pm.countInstancesForOrg(req.OrganizationID)
			if currentCount >= plan.Quotas.MaxInstances {
				return nil, ErrInstanceLimitExceeded
			}
		}
	}

	pm.mu.Lock()
	defer pm.mu.Unlock()

	now := time.Now()
	instanceID := generateID("inst")

	version := req.Version
	if version == "" {
		version = "latest"
	}

	instance := &Instance{
		ID:             instanceID,
		OrganizationID: req.OrganizationID,
		Name:           req.Name,
		Region:         req.Region,
		Size:           req.Size,
		Status:         InstancePending,
		Version:        version,
		Config:         req.Config,
		Tags:           req.Tags,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	pm.instances[instanceID] = instance

	// Simulate async provisioning
	go pm.provisionInstance(instance)

	return instance, nil
}

func (pm *ProvisioningManager) provisionInstance(instance *Instance) {
	pm.mu.Lock()
	// Check if instance was terminated before we start provisioning
	if instance.Status == InstanceTerminating || instance.Status == InstanceTerminated {
		pm.mu.Unlock()
		return
	}
	instance.Status = InstanceProvisioning
	instance.UpdatedAt = time.Now()
	pm.mu.Unlock()

	// Simulate provisioning time
	time.Sleep(100 * time.Millisecond)

	pm.mu.Lock()
	defer pm.mu.Unlock()

	// Check again if instance was terminated during provisioning
	if instance.Status == InstanceTerminating || instance.Status == InstanceTerminated {
		return
	}

	now := time.Now()
	instance.Status = InstanceRunning
	instance.StartedAt = &now
	instance.UpdatedAt = now
	instance.Endpoint = fmt.Sprintf("https://%s.feather.io", instance.ID)
	instance.InternalIP = fmt.Sprintf("10.0.%d.%d", now.Unix()%256, now.UnixNano()%256)
}

func (pm *ProvisioningManager) countInstancesForOrg(orgID string) int {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	count := 0
	for _, inst := range pm.instances {
		if inst.OrganizationID == orgID && inst.Status != InstanceTerminated {
			count++
		}
	}
	return count
}

// GetInstance retrieves an instance.
func (pm *ProvisioningManager) GetInstance(id string) (*Instance, error) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	instance, exists := pm.instances[id]
	if !exists {
		return nil, ErrInstanceNotFound
	}
	return instance, nil
}

// ListInstances returns instances for an organization.
func (pm *ProvisioningManager) ListInstances(orgID string) []*Instance {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	result := make([]*Instance, 0)
	for _, inst := range pm.instances {
		if inst.OrganizationID == orgID {
			result = append(result, inst)
		}
	}
	return result
}

// UpdateInstance updates an instance configuration.
func (pm *ProvisioningManager) UpdateInstance(id string, config InstanceConfig) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	instance, exists := pm.instances[id]
	if !exists {
		return ErrInstanceNotFound
	}

	instance.Config = config
	instance.UpdatedAt = time.Now()
	return nil
}

// ResizeInstance changes the instance size.
func (pm *ProvisioningManager) ResizeInstance(id string, newSize InstanceSize) error {
	if _, ok := InstanceSizeSpecs[newSize]; !ok {
		return ErrInvalidInstanceSize
	}

	pm.mu.Lock()
	defer pm.mu.Unlock()

	instance, exists := pm.instances[id]
	if !exists {
		return ErrInstanceNotFound
	}

	instance.Size = newSize
	instance.UpdatedAt = time.Now()
	return nil
}

// StartInstance starts a stopped instance.
func (pm *ProvisioningManager) StartInstance(id string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	instance, exists := pm.instances[id]
	if !exists {
		return ErrInstanceNotFound
	}

	if instance.Status != InstanceStopped {
		return fmt.Errorf("instance is not in stopped state")
	}

	now := time.Now()
	instance.Status = InstanceRunning
	instance.StartedAt = &now
	instance.UpdatedAt = now
	return nil
}

// StopInstance stops a running instance.
func (pm *ProvisioningManager) StopInstance(id string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	instance, exists := pm.instances[id]
	if !exists {
		return ErrInstanceNotFound
	}

	if instance.Status != InstanceRunning {
		return fmt.Errorf("instance is not running")
	}

	now := time.Now()
	instance.Status = InstanceStopping
	instance.UpdatedAt = now

	// Simulate stopping
	go func() {
		time.Sleep(50 * time.Millisecond)
		pm.mu.Lock()
		defer pm.mu.Unlock()
		now := time.Now()
		instance.Status = InstanceStopped
		instance.StoppedAt = &now
		instance.UpdatedAt = now
	}()

	return nil
}

// TerminateInstance permanently terminates an instance.
func (pm *ProvisioningManager) TerminateInstance(id string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	instance, exists := pm.instances[id]
	if !exists {
		return ErrInstanceNotFound
	}

	instance.Status = InstanceTerminating
	instance.UpdatedAt = time.Now()

	// Simulate termination
	go func() {
		time.Sleep(50 * time.Millisecond)
		pm.mu.Lock()
		defer pm.mu.Unlock()
		instance.Status = InstanceTerminated
		instance.UpdatedAt = time.Now()
	}()

	return nil
}

// RestartInstance restarts an instance.
func (pm *ProvisioningManager) RestartInstance(id string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	instance, exists := pm.instances[id]
	if !exists {
		return ErrInstanceNotFound
	}

	if instance.Status != InstanceRunning {
		return fmt.Errorf("instance is not running")
	}

	// Simulate restart
	instance.Status = InstanceStopping
	instance.UpdatedAt = time.Now()

	go func() {
		time.Sleep(100 * time.Millisecond)
		pm.mu.Lock()
		defer pm.mu.Unlock()
		now := time.Now()
		instance.Status = InstanceRunning
		instance.StartedAt = &now
		instance.UpdatedAt = now
	}()

	return nil
}

// GetRegions returns available regions.
func (pm *ProvisioningManager) GetRegions() []Region {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	result := make([]Region, 0, len(pm.regions))
	for _, r := range pm.regions {
		result = append(result, r)
	}
	return result
}

// GetInstanceSizes returns available instance sizes.
func (pm *ProvisioningManager) GetInstanceSizes() map[InstanceSize]struct {
	VCPUs        int
	MemoryGB     int
	PricePerHour float64
} {
	return InstanceSizeSpecs
}

// GetInstanceMetrics returns instance metrics.
func (pm *ProvisioningManager) GetInstanceMetrics(id string) (*InstanceMetrics, error) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	instance, exists := pm.instances[id]
	if !exists {
		return nil, ErrInstanceNotFound
	}

	// Return simulated metrics
	spec := InstanceSizeSpecs[instance.Size]
	return &InstanceMetrics{
		InstanceID:     id,
		CPUUtilization: 25.5,
		MemoryUsedGB:   float64(spec.MemoryGB) * 0.6,
		MemoryTotalGB:  float64(spec.MemoryGB),
		RequestsPerSec: 1000,
		LatencyP50Ms:   0.5,
		LatencyP99Ms:   2.5,
		StorageUsedGB:  float64(instance.Config.HotStorageGB) * 0.3,
		StorageTotalGB: float64(instance.Config.HotStorageGB),
		Timestamp:      time.Now(),
	}, nil
}

// InstanceMetrics represents instance resource metrics.
type InstanceMetrics struct {
	InstanceID     string    `json:"instance_id"`
	CPUUtilization float64   `json:"cpu_utilization_percent"`
	MemoryUsedGB   float64   `json:"memory_used_gb"`
	MemoryTotalGB  float64   `json:"memory_total_gb"`
	RequestsPerSec float64   `json:"requests_per_sec"`
	LatencyP50Ms   float64   `json:"latency_p50_ms"`
	LatencyP99Ms   float64   `json:"latency_p99_ms"`
	StorageUsedGB  float64   `json:"storage_used_gb"`
	StorageTotalGB float64   `json:"storage_total_gb"`
	Timestamp      time.Time `json:"timestamp"`
}
