package saas

import (
	"context"
	"errors"
	"testing"
	"time"
)

func setupProvisioningManager() *ProvisioningManager {
	registry := NewPlanRegistry()
	billing := NewBillingManager(registry)
	return NewProvisioningManager(registry, billing)
}

func TestNewProvisioningManager(t *testing.T) {
	pm := setupProvisioningManager()
	if pm == nil {
		t.Fatal("Expected manager to be non-nil")
	}
}

func TestProvisioningManager_CreateInstance(t *testing.T) {
	pm := setupProvisioningManager()

	req := &ProvisioningRequest{
		OrganizationID: "org_1",
		Name:           "test-instance",
		Region:         "us-east-1",
		Size:           SizeMedium,
		Config: InstanceConfig{
			HotStorageGB:       10,
			WarmStorageGB:      100,
			EnableVectorSearch: true,
		},
	}

	instance, err := pm.CreateInstance(req)
	if err != nil {
		t.Fatalf("CreateInstance failed: %v", err)
	}

	if instance.Name != "test-instance" {
		t.Errorf("Expected name 'test-instance', got '%s'", instance.Name)
	}
	if instance.Region != "us-east-1" {
		t.Errorf("Expected region 'us-east-1', got '%s'", instance.Region)
	}
	if instance.Size != SizeMedium {
		t.Errorf("Expected size medium, got %s", instance.Size)
	}
	if instance.Status != InstancePending && instance.Status != InstanceProvisioning {
		t.Errorf("Expected pending or provisioning status, got %s", instance.Status)
	}
}

func TestProvisioningManager_CreateInstance_InvalidRegion(t *testing.T) {
	pm := setupProvisioningManager()

	req := &ProvisioningRequest{
		OrganizationID: "org_1",
		Name:           "test-instance",
		Region:         "invalid-region",
		Size:           SizeMedium,
	}

	_, err := pm.CreateInstance(req)
	if !errors.Is(err, ErrInvalidRegion) {
		t.Errorf("Expected ErrInvalidRegion, got %v", err)
	}
}

func TestProvisioningManager_CreateInstance_InvalidSize(t *testing.T) {
	pm := setupProvisioningManager()

	req := &ProvisioningRequest{
		OrganizationID: "org_1",
		Name:           "test-instance",
		Region:         "us-east-1",
		Size:           InstanceSize("invalid"),
	}

	_, err := pm.CreateInstance(req)
	if !errors.Is(err, ErrInvalidInstanceSize) {
		t.Errorf("Expected ErrInvalidInstanceSize, got %v", err)
	}
}

func TestProvisioningManager_CreateInstance_ProvisioningCompletes(t *testing.T) {
	pm := setupProvisioningManager()

	req := &ProvisioningRequest{
		OrganizationID: "org_1",
		Name:           "test-instance",
		Region:         "us-east-1",
		Size:           SizeSmall,
	}

	instance, _ := pm.CreateInstance(req)

	// Wait for provisioning to complete
	time.Sleep(200 * time.Millisecond)

	updated, _ := pm.GetInstance(instance.ID)
	if updated.Status != InstanceRunning {
		t.Errorf("Expected running status, got %s", updated.Status)
	}
	if updated.Endpoint == "" {
		t.Error("Expected endpoint to be set")
	}
	if updated.StartedAt == nil {
		t.Error("Expected StartedAt to be set")
	}
}

func TestProvisioningManager_GetInstance(t *testing.T) {
	pm := setupProvisioningManager()

	req := &ProvisioningRequest{
		OrganizationID: "org_1",
		Name:           "test-instance",
		Region:         "us-east-1",
		Size:           SizeSmall,
	}

	instance, _ := pm.CreateInstance(req)

	retrieved, err := pm.GetInstance(instance.ID)
	if err != nil {
		t.Fatalf("GetInstance failed: %v", err)
	}
	if retrieved.ID != instance.ID {
		t.Errorf("Expected ID %s, got %s", instance.ID, retrieved.ID)
	}
}

func TestProvisioningManager_GetInstance_NotFound(t *testing.T) {
	pm := setupProvisioningManager()

	_, err := pm.GetInstance("nonexistent")
	if !errors.Is(err, ErrInstanceNotFound) {
		t.Errorf("Expected ErrInstanceNotFound, got %v", err)
	}
}

func TestProvisioningManager_ListInstances(t *testing.T) {
	pm := setupProvisioningManager()

	// Create instances for different orgs
	pm.CreateInstance(&ProvisioningRequest{
		OrganizationID: "org_1",
		Name:           "instance-1",
		Region:         "us-east-1",
		Size:           SizeSmall,
	})
	pm.CreateInstance(&ProvisioningRequest{
		OrganizationID: "org_1",
		Name:           "instance-2",
		Region:         "us-west-2",
		Size:           SizeMedium,
	})
	pm.CreateInstance(&ProvisioningRequest{
		OrganizationID: "org_2",
		Name:           "instance-3",
		Region:         "eu-west-1",
		Size:           SizeSmall,
	})

	org1Instances := pm.ListInstances("org_1")
	if len(org1Instances) != 2 {
		t.Errorf("Expected 2 instances for org_1, got %d", len(org1Instances))
	}

	org2Instances := pm.ListInstances("org_2")
	if len(org2Instances) != 1 {
		t.Errorf("Expected 1 instance for org_2, got %d", len(org2Instances))
	}
}

func TestProvisioningManager_UpdateInstance(t *testing.T) {
	pm := setupProvisioningManager()

	req := &ProvisioningRequest{
		OrganizationID: "org_1",
		Name:           "test-instance",
		Region:         "us-east-1",
		Size:           SizeSmall,
		Config: InstanceConfig{
			HotStorageGB: 10,
		},
	}

	instance, _ := pm.CreateInstance(req)

	newConfig := InstanceConfig{
		HotStorageGB:       20,
		EnableVectorSearch: true,
	}

	err := pm.UpdateInstance(instance.ID, newConfig)
	if err != nil {
		t.Fatalf("UpdateInstance failed: %v", err)
	}

	updated, _ := pm.GetInstance(instance.ID)
	if updated.Config.HotStorageGB != 20 {
		t.Errorf("Expected HotStorageGB 20, got %d", updated.Config.HotStorageGB)
	}
	if !updated.Config.EnableVectorSearch {
		t.Error("Expected EnableVectorSearch to be true")
	}
}

func TestProvisioningManager_UpdateInstance_NotFound(t *testing.T) {
	pm := setupProvisioningManager()

	err := pm.UpdateInstance("nonexistent", InstanceConfig{})
	if !errors.Is(err, ErrInstanceNotFound) {
		t.Errorf("Expected ErrInstanceNotFound, got %v", err)
	}
}

func TestProvisioningManager_ResizeInstance(t *testing.T) {
	pm := setupProvisioningManager()

	req := &ProvisioningRequest{
		OrganizationID: "org_1",
		Name:           "test-instance",
		Region:         "us-east-1",
		Size:           SizeSmall,
	}

	instance, _ := pm.CreateInstance(req)

	err := pm.ResizeInstance(instance.ID, SizeLarge)
	if err != nil {
		t.Fatalf("ResizeInstance failed: %v", err)
	}

	updated, _ := pm.GetInstance(instance.ID)
	if updated.Size != SizeLarge {
		t.Errorf("Expected size large, got %s", updated.Size)
	}
}

func TestProvisioningManager_ResizeInstance_InvalidSize(t *testing.T) {
	pm := setupProvisioningManager()

	req := &ProvisioningRequest{
		OrganizationID: "org_1",
		Name:           "test-instance",
		Region:         "us-east-1",
		Size:           SizeSmall,
	}

	instance, _ := pm.CreateInstance(req)

	err := pm.ResizeInstance(instance.ID, InstanceSize("invalid"))
	if !errors.Is(err, ErrInvalidInstanceSize) {
		t.Errorf("Expected ErrInvalidInstanceSize, got %v", err)
	}
}

func TestProvisioningManager_StopInstance(t *testing.T) {
	pm := setupProvisioningManager()

	req := &ProvisioningRequest{
		OrganizationID: "org_1",
		Name:           "test-instance",
		Region:         "us-east-1",
		Size:           SizeSmall,
	}

	instance, _ := pm.CreateInstance(req)

	// Wait for provisioning
	time.Sleep(200 * time.Millisecond)

	err := pm.StopInstance(context.Background(), instance.ID)
	if err != nil {
		t.Fatalf("StopInstance failed: %v", err)
	}

	// Wait for stopping
	time.Sleep(100 * time.Millisecond)

	updated, _ := pm.GetInstance(instance.ID)
	if updated.Status != InstanceStopped {
		t.Errorf("Expected stopped status, got %s", updated.Status)
	}
}

func TestProvisioningManager_StartInstance(t *testing.T) {
	pm := setupProvisioningManager()

	req := &ProvisioningRequest{
		OrganizationID: "org_1",
		Name:           "test-instance",
		Region:         "us-east-1",
		Size:           SizeSmall,
	}

	instance, _ := pm.CreateInstance(req)

	// Wait for provisioning then stop
	time.Sleep(200 * time.Millisecond)
	pm.StopInstance(context.Background(), instance.ID)
	time.Sleep(100 * time.Millisecond)

	err := pm.StartInstance(instance.ID)
	if err != nil {
		t.Fatalf("StartInstance failed: %v", err)
	}

	updated, _ := pm.GetInstance(instance.ID)
	if updated.Status != InstanceRunning {
		t.Errorf("Expected running status, got %s", updated.Status)
	}
}

func TestProvisioningManager_TerminateInstance(t *testing.T) {
	pm := setupProvisioningManager()

	req := &ProvisioningRequest{
		OrganizationID: "org_1",
		Name:           "test-instance",
		Region:         "us-east-1",
		Size:           SizeSmall,
	}

	instance, _ := pm.CreateInstance(req)

	err := pm.TerminateInstance(context.Background(), instance.ID)
	if err != nil {
		t.Fatalf("TerminateInstance failed: %v", err)
	}

	// Wait for termination (longer to avoid race)
	time.Sleep(150 * time.Millisecond)

	updated, _ := pm.GetInstance(instance.ID)
	// Accept either terminating or terminated status (async processing)
	if updated.Status != InstanceTerminated && updated.Status != InstanceTerminating {
		t.Errorf("Expected terminated or terminating status, got %s", updated.Status)
	}
}

func TestProvisioningManager_TerminateInstance_NotFound(t *testing.T) {
	pm := setupProvisioningManager()

	err := pm.TerminateInstance(context.Background(), "nonexistent")
	if !errors.Is(err, ErrInstanceNotFound) {
		t.Errorf("Expected ErrInstanceNotFound, got %v", err)
	}
}

func TestProvisioningManager_RestartInstance(t *testing.T) {
	pm := setupProvisioningManager()

	req := &ProvisioningRequest{
		OrganizationID: "org_1",
		Name:           "test-instance",
		Region:         "us-east-1",
		Size:           SizeSmall,
	}

	instance, _ := pm.CreateInstance(req)

	// Wait for provisioning
	time.Sleep(200 * time.Millisecond)

	err := pm.RestartInstance(context.Background(), instance.ID)
	if err != nil {
		t.Fatalf("RestartInstance failed: %v", err)
	}

	// Wait for restart
	time.Sleep(200 * time.Millisecond)

	updated, _ := pm.GetInstance(instance.ID)
	if updated.Status != InstanceRunning {
		t.Errorf("Expected running status after restart, got %s", updated.Status)
	}
}

func TestProvisioningManager_GetRegions(t *testing.T) {
	pm := setupProvisioningManager()

	regions := pm.GetRegions()
	if len(regions) < 4 {
		t.Errorf("Expected at least 4 regions, got %d", len(regions))
	}

	// Check for specific regions
	foundUSEast := false
	foundEUWest := false
	for _, r := range regions {
		if r.ID == "us-east-1" {
			foundUSEast = true
		}
		if r.ID == "eu-west-1" {
			foundEUWest = true
		}
	}

	if !foundUSEast {
		t.Error("Expected us-east-1 region")
	}
	if !foundEUWest {
		t.Error("Expected eu-west-1 region")
	}
}

func TestProvisioningManager_GetInstanceSizes(t *testing.T) {
	pm := setupProvisioningManager()

	sizes := pm.GetInstanceSizes()
	if len(sizes) < 4 {
		t.Errorf("Expected at least 4 sizes, got %d", len(sizes))
	}

	smallSpec := sizes[SizeSmall]
	if smallSpec.VCPUs < 1 {
		t.Error("Expected small to have at least 1 vCPU")
	}

	largeSpec := sizes[SizeLarge]
	if largeSpec.MemoryGB <= smallSpec.MemoryGB {
		t.Error("Expected large to have more memory than small")
	}
}

func TestProvisioningManager_GetInstanceMetrics(t *testing.T) {
	pm := setupProvisioningManager()

	req := &ProvisioningRequest{
		OrganizationID: "org_1",
		Name:           "test-instance",
		Region:         "us-east-1",
		Size:           SizeMedium,
		Config: InstanceConfig{
			HotStorageGB: 10,
		},
	}

	instance, _ := pm.CreateInstance(req)

	// Wait for provisioning
	time.Sleep(200 * time.Millisecond)

	metrics, err := pm.GetInstanceMetrics(instance.ID)
	if err != nil {
		t.Fatalf("GetInstanceMetrics failed: %v", err)
	}

	if metrics.InstanceID != instance.ID {
		t.Errorf("Expected instance ID %s, got %s", instance.ID, metrics.InstanceID)
	}
	if metrics.CPUUtilization < 0 || metrics.CPUUtilization > 100 {
		t.Errorf("Expected valid CPU utilization, got %f", metrics.CPUUtilization)
	}
	if metrics.MemoryTotalGB <= 0 {
		t.Error("Expected positive memory total")
	}
}

func TestProvisioningManager_GetInstanceMetrics_NotFound(t *testing.T) {
	pm := setupProvisioningManager()

	_, err := pm.GetInstanceMetrics("nonexistent")
	if !errors.Is(err, ErrInstanceNotFound) {
		t.Errorf("Expected ErrInstanceNotFound, got %v", err)
	}
}

func TestInstanceSizeSpecs(t *testing.T) {
	// Verify all defined sizes have valid specs
	sizes := []InstanceSize{
		SizeXSmall,
		SizeSmall,
		SizeMedium,
		SizeLarge,
		SizeXLarge,
		Size2XLarge,
	}

	for _, size := range sizes {
		spec, ok := InstanceSizeSpecs[size]
		if !ok {
			t.Errorf("Missing spec for size %s", size)
			continue
		}
		if spec.VCPUs <= 0 {
			t.Errorf("Invalid vCPUs for size %s", size)
		}
		if spec.MemoryGB <= 0 {
			t.Errorf("Invalid memory for size %s", size)
		}
		if spec.PricePerHour <= 0 {
			t.Errorf("Invalid price for size %s", size)
		}
	}
}

func TestInstanceConfig_Defaults(t *testing.T) {
	config := InstanceConfig{}

	if config.TLSEnabled {
		t.Error("Expected TLS to be disabled by default")
	}
	if config.EnableVectorSearch {
		t.Error("Expected vector search to be disabled by default")
	}
}

func TestInstanceStatus_Values(t *testing.T) {
	statuses := []InstanceStatus{
		InstancePending,
		InstanceProvisioning,
		InstanceRunning,
		InstanceStopping,
		InstanceStopped,
		InstanceTerminating,
		InstanceTerminated,
		InstanceFailed,
	}

	for _, status := range statuses {
		if status == "" {
			t.Error("Expected non-empty status value")
		}
	}
}
