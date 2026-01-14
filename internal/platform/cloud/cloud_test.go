package cloud

import (
	"errors"
	"testing"
)

func setupControlPlane() *ControlPlane {
	return NewControlPlane(DefaultConfig())
}

func provisionDefault(t *testing.T, cp *ControlPlane) *Instance {
	t.Helper()
	inst, err := cp.Provision(ProvisionRequest{
		Name:     "test-instance",
		TenantID: "tenant-1",
		Tier:     TierStarter,
		Region:   "us-east-1",
	})
	if err != nil {
		t.Fatalf("unexpected error provisioning: %v", err)
	}
	return inst
}

// --- Provision tests ---

func TestControlPlane_Provision(t *testing.T) {
	cp := setupControlPlane()
	inst := provisionDefault(t, cp)

	if inst.ID == "" {
		t.Error("expected non-empty instance ID")
	}
	if inst.Status != StatusProvisioning {
		t.Errorf("expected status %s, got %s", StatusProvisioning, inst.Status)
	}
	if inst.Tier != TierStarter {
		t.Errorf("expected tier %s, got %s", TierStarter, inst.Tier)
	}
	if inst.Replicas != 1 {
		t.Errorf("expected 1 replica, got %d", inst.Replicas)
	}
	if inst.MaxReplicas != 3 {
		t.Errorf("expected max replicas 3, got %d", inst.MaxReplicas)
	}
	if inst.Endpoint == "" {
		t.Error("expected non-empty endpoint")
	}
}

func TestControlPlane_Provision_AllTiers(t *testing.T) {
	tiers := []struct {
		tier        Tier
		maxReplicas int
	}{
		{TierFree, 1},
		{TierStarter, 3},
		{TierPro, 10},
		{TierEnterprise, 50},
	}
	for _, tc := range tiers {
		t.Run(string(tc.tier), func(t *testing.T) {
			cp := setupControlPlane()
			inst, err := cp.Provision(ProvisionRequest{
				Name:     "inst",
				TenantID: "t1",
				Tier:     tc.tier,
				Region:   "us-east-1",
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if inst.MaxReplicas != tc.maxReplicas {
				t.Errorf("tier %s: expected max replicas %d, got %d", tc.tier, tc.maxReplicas, inst.MaxReplicas)
			}
		})
	}
}

func TestControlPlane_Provision_MissingName(t *testing.T) {
	cp := setupControlPlane()
	_, err := cp.Provision(ProvisionRequest{
		TenantID: "t1",
		Tier:     TierStarter,
		Region:   "us-east-1",
	})
	if err == nil {
		t.Fatal("expected error for missing name")
	}
}

func TestControlPlane_Provision_MissingTenantID(t *testing.T) {
	cp := setupControlPlane()
	_, err := cp.Provision(ProvisionRequest{
		Name:   "inst",
		Tier:   TierStarter,
		Region: "us-east-1",
	})
	if err == nil {
		t.Fatal("expected error for missing tenant_id")
	}
}

func TestControlPlane_Provision_InvalidTier(t *testing.T) {
	cp := setupControlPlane()
	_, err := cp.Provision(ProvisionRequest{
		Name:     "inst",
		TenantID: "t1",
		Tier:     "invalid",
		Region:   "us-east-1",
	})
	if !errors.Is(err, ErrInvalidTier) {
		t.Errorf("expected ErrInvalidTier, got %v", err)
	}
}

func TestControlPlane_Provision_InvalidRegion(t *testing.T) {
	cp := setupControlPlane()
	_, err := cp.Provision(ProvisionRequest{
		Name:     "inst",
		TenantID: "t1",
		Tier:     TierStarter,
		Region:   "mars-1",
	})
	if !errors.Is(err, ErrInvalidRegion) {
		t.Errorf("expected ErrInvalidRegion, got %v", err)
	}
}

func TestControlPlane_Provision_TenantLimit(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxInstancesPerTenant = 2
	cp := NewControlPlane(cfg)

	for i := 0; i < 2; i++ {
		_, err := cp.Provision(ProvisionRequest{
			Name:     "inst",
			TenantID: "t1",
			Tier:     TierStarter,
			Region:   "us-east-1",
		})
		if err != nil {
			t.Fatalf("unexpected error on provision %d: %v", i, err)
		}
	}

	_, err := cp.Provision(ProvisionRequest{
		Name:     "inst",
		TenantID: "t1",
		Tier:     TierStarter,
		Region:   "us-east-1",
	})
	if !errors.Is(err, ErrTenantLimitExceeded) {
		t.Errorf("expected ErrTenantLimitExceeded, got %v", err)
	}
}

// --- GetInstance tests ---

func TestControlPlane_GetInstance(t *testing.T) {
	cp := setupControlPlane()
	inst := provisionDefault(t, cp)

	got, err := cp.GetInstance(inst.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != inst.ID {
		t.Errorf("expected ID %s, got %s", inst.ID, got.ID)
	}
}

func TestControlPlane_GetInstance_NotFound(t *testing.T) {
	cp := setupControlPlane()
	_, err := cp.GetInstance("nonexistent")
	if !errors.Is(err, ErrInstanceNotFound) {
		t.Errorf("expected ErrInstanceNotFound, got %v", err)
	}
}

// --- ListInstances tests ---

func TestControlPlane_ListInstances(t *testing.T) {
	cp := setupControlPlane()
	provisionDefault(t, cp)
	_, _ = cp.Provision(ProvisionRequest{
		Name:     "other",
		TenantID: "tenant-2",
		Tier:     TierPro,
		Region:   "eu-west-1",
	})

	list := cp.ListInstances("tenant-1")
	if len(list) != 1 {
		t.Errorf("expected 1 instance for tenant-1, got %d", len(list))
	}
}

func TestControlPlane_ListInstances_Empty(t *testing.T) {
	cp := setupControlPlane()
	list := cp.ListInstances("nonexistent")
	if len(list) != 0 {
		t.Errorf("expected empty list, got %d", len(list))
	}
}

// --- Scale tests ---

func TestControlPlane_Scale(t *testing.T) {
	cp := setupControlPlane()
	inst := provisionDefault(t, cp)

	// Set instance to running so scaling is allowed.
	cp.mu.Lock()
	cp.instances[inst.ID].Status = StatusRunning
	cp.mu.Unlock()

	scaled, err := cp.Scale(inst.ID, ScaleRequest{Replicas: 3})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if scaled.Replicas != 3 {
		t.Errorf("expected 3 replicas, got %d", scaled.Replicas)
	}
	if scaled.Status != StatusScaling {
		t.Errorf("expected status %s, got %s", StatusScaling, scaled.Status)
	}
}

func TestControlPlane_Scale_ExceedsTierLimit(t *testing.T) {
	cp := setupControlPlane()
	inst := provisionDefault(t, cp)

	cp.mu.Lock()
	cp.instances[inst.ID].Status = StatusRunning
	cp.mu.Unlock()

	// Starter tier max is 3.
	_, err := cp.Scale(inst.ID, ScaleRequest{Replicas: 10})
	if !errors.Is(err, ErrInvalidReplicas) {
		t.Errorf("expected ErrInvalidReplicas, got %v", err)
	}
}

func TestControlPlane_Scale_ZeroReplicas(t *testing.T) {
	cp := setupControlPlane()
	inst := provisionDefault(t, cp)

	cp.mu.Lock()
	cp.instances[inst.ID].Status = StatusRunning
	cp.mu.Unlock()

	_, err := cp.Scale(inst.ID, ScaleRequest{Replicas: 0})
	if !errors.Is(err, ErrInvalidReplicas) {
		t.Errorf("expected ErrInvalidReplicas, got %v", err)
	}
}

func TestControlPlane_Scale_NotRunning(t *testing.T) {
	cp := setupControlPlane()
	inst := provisionDefault(t, cp)

	cp.mu.Lock()
	cp.instances[inst.ID].Status = StatusStopped
	cp.mu.Unlock()

	_, err := cp.Scale(inst.ID, ScaleRequest{Replicas: 2})
	if !errors.Is(err, ErrInstanceNotRunning) {
		t.Errorf("expected ErrInstanceNotRunning, got %v", err)
	}
}

func TestControlPlane_Scale_NotFound(t *testing.T) {
	cp := setupControlPlane()
	_, err := cp.Scale("nonexistent", ScaleRequest{Replicas: 2})
	if !errors.Is(err, ErrInstanceNotFound) {
		t.Errorf("expected ErrInstanceNotFound, got %v", err)
	}
}

func TestControlPlane_Scale_WithResourceOverrides(t *testing.T) {
	cp := setupControlPlane()
	inst := provisionDefault(t, cp)

	cp.mu.Lock()
	cp.instances[inst.ID].Status = StatusRunning
	cp.mu.Unlock()

	scaled, err := cp.Scale(inst.ID, ScaleRequest{
		Replicas:    2,
		CPULimit:    "1",
		MemoryLimit: "2Gi",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if scaled.CPULimit != "1" {
		t.Errorf("expected cpu_limit '1', got %s", scaled.CPULimit)
	}
	if scaled.MemoryLimit != "2Gi" {
		t.Errorf("expected memory_limit '2Gi', got %s", scaled.MemoryLimit)
	}
}

// --- Terminate tests ---

func TestControlPlane_Terminate(t *testing.T) {
	cp := setupControlPlane()
	inst := provisionDefault(t, cp)

	if err := cp.Terminate(inst.ID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, err := cp.GetInstance(inst.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Status != StatusTerminating {
		t.Errorf("expected status %s, got %s", StatusTerminating, got.Status)
	}
}

func TestControlPlane_Terminate_NotFound(t *testing.T) {
	cp := setupControlPlane()
	err := cp.Terminate("nonexistent")
	if !errors.Is(err, ErrInstanceNotFound) {
		t.Errorf("expected ErrInstanceNotFound, got %v", err)
	}
}

func TestControlPlane_Terminate_AlreadyTerminating(t *testing.T) {
	cp := setupControlPlane()
	inst := provisionDefault(t, cp)

	_ = cp.Terminate(inst.ID)
	err := cp.Terminate(inst.ID)
	if !errors.Is(err, ErrInstanceNotTerminated) {
		t.Errorf("expected ErrInstanceNotTerminated, got %v", err)
	}
}

// --- Usage tests ---

func TestControlPlane_GetUsage(t *testing.T) {
	cp := setupControlPlane()
	provisionDefault(t, cp)

	usage, err := cp.GetUsage("tenant-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if usage.TenantID != "tenant-1" {
		t.Errorf("expected tenant ID tenant-1, got %s", usage.TenantID)
	}
}

func TestControlPlane_GetUsage_NotFound(t *testing.T) {
	cp := setupControlPlane()
	_, err := cp.GetUsage("nonexistent")
	if !errors.Is(err, ErrUsageNotFound) {
		t.Errorf("expected ErrUsageNotFound, got %v", err)
	}
}

func TestControlPlane_RecordUsage(t *testing.T) {
	cp := setupControlPlane()
	provisionDefault(t, cp)

	cp.RecordUsage("tenant-1", 1000, 500)
	cp.RecordUsage("tenant-1", 500, 250)

	usage, err := cp.GetUsage("tenant-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if usage.FeatureReads != 1500 {
		t.Errorf("expected 1500 reads, got %d", usage.FeatureReads)
	}
	if usage.FeatureWrites != 750 {
		t.Errorf("expected 750 writes, got %d", usage.FeatureWrites)
	}
	if usage.EstimatedCost <= 0 {
		t.Errorf("expected positive estimated cost, got %f", usage.EstimatedCost)
	}
}

func TestControlPlane_RecordUsage_NewTenant(t *testing.T) {
	cp := setupControlPlane()

	cp.RecordUsage("new-tenant", 100, 50)

	usage, err := cp.GetUsage("new-tenant")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if usage.FeatureReads != 100 {
		t.Errorf("expected 100 reads, got %d", usage.FeatureReads)
	}
}

// --- Autoscale tests ---

func TestControlPlane_EvaluateAutoscale_HighCPU(t *testing.T) {
	cp := setupControlPlane()
	inst := provisionDefault(t, cp)

	cp.mu.Lock()
	cp.instances[inst.ID].Status = StatusRunning
	cp.instances[inst.ID].Metrics.CPUUsage = 90.0
	cp.mu.Unlock()

	actions := cp.EvaluateAutoscale()
	if len(actions) == 0 {
		t.Fatal("expected at least one scale action")
	}
	if actions[0].DesiredReps <= actions[0].CurrentReps {
		t.Error("expected desired replicas to exceed current")
	}
}

func TestControlPlane_EvaluateAutoscale_NormalMetrics(t *testing.T) {
	cp := setupControlPlane()
	inst := provisionDefault(t, cp)

	cp.mu.Lock()
	cp.instances[inst.ID].Status = StatusRunning
	cp.instances[inst.ID].Metrics.CPUUsage = 30.0
	cp.instances[inst.ID].Metrics.MemoryUsage = 40.0
	cp.mu.Unlock()

	actions := cp.EvaluateAutoscale()
	if len(actions) != 0 {
		t.Errorf("expected no scale actions, got %d", len(actions))
	}
}

func TestControlPlane_EvaluateAutoscale_AtMaxReplicas(t *testing.T) {
	cp := setupControlPlane()
	inst, _ := cp.Provision(ProvisionRequest{
		Name:     "free-inst",
		TenantID: "t1",
		Tier:     TierFree,
		Region:   "us-east-1",
	})

	cp.mu.Lock()
	cp.instances[inst.ID].Status = StatusRunning
	cp.instances[inst.ID].Replicas = 1 // already at max for free
	cp.instances[inst.ID].Metrics.CPUUsage = 95.0
	cp.mu.Unlock()

	actions := cp.EvaluateAutoscale()
	if len(actions) != 0 {
		t.Errorf("expected no actions when at max replicas, got %d", len(actions))
	}
}

func TestControlPlane_EvaluateAutoscale_SkipNonRunning(t *testing.T) {
	cp := setupControlPlane()
	inst := provisionDefault(t, cp)

	// Instance is still in provisioning state, not running.
	cp.mu.Lock()
	cp.instances[inst.ID].Metrics.CPUUsage = 95.0
	cp.mu.Unlock()

	actions := cp.EvaluateAutoscale()
	if len(actions) != 0 {
		t.Errorf("expected no actions for non-running instance, got %d", len(actions))
	}
}

// --- Stats tests ---

func TestControlPlane_Stats(t *testing.T) {
	cp := setupControlPlane()
	provisionDefault(t, cp)
	_, _ = cp.Provision(ProvisionRequest{
		Name:     "other",
		TenantID: "tenant-2",
		Tier:     TierPro,
		Region:   "eu-west-1",
	})

	// Mark one as running.
	cp.mu.Lock()
	for _, inst := range cp.instances {
		if inst.TenantID == "tenant-1" {
			inst.Status = StatusRunning
		}
	}
	cp.mu.Unlock()

	stats := cp.Stats()
	if stats.TotalInstances != 2 {
		t.Errorf("expected 2 total instances, got %d", stats.TotalInstances)
	}
	if stats.RunningCount != 1 {
		t.Errorf("expected 1 running, got %d", stats.RunningCount)
	}
	if stats.TotalTenants != 2 {
		t.Errorf("expected 2 tenants, got %d", stats.TotalTenants)
	}
	if stats.ByTier["starter"] != 1 || stats.ByTier["pro"] != 1 {
		t.Errorf("unexpected tier distribution: %v", stats.ByTier)
	}
	if stats.ByRegion["us-east-1"] != 1 || stats.ByRegion["eu-west-1"] != 1 {
		t.Errorf("unexpected region distribution: %v", stats.ByRegion)
	}
}

func TestControlPlane_Stats_Empty(t *testing.T) {
	cp := setupControlPlane()
	stats := cp.Stats()
	if stats.TotalInstances != 0 {
		t.Errorf("expected 0 instances, got %d", stats.TotalInstances)
	}
	if stats.TotalTenants != 0 {
		t.Errorf("expected 0 tenants, got %d", stats.TotalTenants)
	}
}

// --- DefaultConfig tests ---

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.MaxInstancesPerTenant != 5 {
		t.Errorf("expected max 5, got %d", cfg.MaxInstancesPerTenant)
	}
	if cfg.DefaultTier != TierStarter {
		t.Errorf("expected default tier starter, got %s", cfg.DefaultTier)
	}
	if !cfg.AutoscaleEnabled {
		t.Error("expected autoscale enabled by default")
	}
}
