package controlplane

import (
	"context"
	"testing"
)

func setupManager(t *testing.T) (*Manager, context.Context) {
	t.Helper()
	mgr := NewManager(DefaultManagerConfig())
	ctx := context.Background()

	// Add a default region so instance registration succeeds.
	if err := mgr.AddRegion(ctx, &Region{
		Name:     "us-east-1",
		Provider: "aws",
		Primary:  true,
	}); err != nil {
		t.Fatalf("setup: adding region: %v", err)
	}
	return mgr, ctx
}

func registerTestInstance(t *testing.T, mgr *Manager, ctx context.Context, name string) *Instance {
	t.Helper()
	inst := &Instance{
		Name:     name,
		Region:   "us-east-1",
		Endpoint: "https://" + name + ".example.com:8080",
		Version:  "1.0.0",
	}
	if err := mgr.RegisterInstance(ctx, inst); err != nil {
		t.Fatalf("registering instance %q: %v", name, err)
	}
	return inst
}

func TestRegisterInstance(t *testing.T) {
	mgr, ctx := setupManager(t)
	defer mgr.Close()

	inst := registerTestInstance(t, mgr, ctx, "feather-1")

	if inst.ID == "" {
		t.Fatal("expected instance ID to be assigned")
	}
	if inst.Status != InstanceStatusProvisioning {
		t.Fatalf("expected status %q, got %q", InstanceStatusProvisioning, inst.Status)
	}
	if inst.CreatedAt.IsZero() {
		t.Fatal("expected CreatedAt to be set")
	}

	// Verify it appears in the region's instance list.
	region := mgr.regions["us-east-1"]
	if len(region.Instances) != 1 || region.Instances[0] != inst.ID {
		t.Fatalf("expected region to contain instance %q", inst.ID)
	}

	// Duplicate registration should succeed with a new ID.
	inst2 := registerTestInstance(t, mgr, ctx, "feather-2")
	if inst2.ID == inst.ID {
		t.Fatal("expected different IDs for different instances")
	}

	// Registration without a region should fail.
	err := mgr.RegisterInstance(ctx, &Instance{Name: "bad", Region: "nonexistent"})
	if err == nil {
		t.Fatal("expected error for unknown region")
	}

	// Registration without a name should fail.
	err = mgr.RegisterInstance(ctx, &Instance{Region: "us-east-1"})
	if err == nil {
		t.Fatal("expected error for missing name")
	}
}

func TestDeregisterInstance(t *testing.T) {
	mgr, ctx := setupManager(t)
	defer mgr.Close()

	inst := registerTestInstance(t, mgr, ctx, "feather-1")

	if err := mgr.DeregisterInstance(ctx, inst.ID); err != nil {
		t.Fatalf("deregistering instance: %v", err)
	}

	// Verify the instance is gone.
	_, err := mgr.GetInstance(ctx, inst.ID)
	if err == nil {
		t.Fatal("expected error getting deregistered instance")
	}

	// Verify the instance is removed from the region.
	region := mgr.regions["us-east-1"]
	if len(region.Instances) != 0 {
		t.Fatalf("expected region instances to be empty, got %d", len(region.Instances))
	}

	// Deregistering again should fail.
	err = mgr.DeregisterInstance(ctx, inst.ID)
	if err == nil {
		t.Fatal("expected error deregistering unknown instance")
	}
}

func TestListInstances(t *testing.T) {
	mgr, ctx := setupManager(t)
	defer mgr.Close()

	// Empty list.
	if got := mgr.ListInstances(ctx); len(got) != 0 {
		t.Fatalf("expected 0 instances, got %d", len(got))
	}

	registerTestInstance(t, mgr, ctx, "feather-1")
	registerTestInstance(t, mgr, ctx, "feather-2")

	got := mgr.ListInstances(ctx)
	if len(got) != 2 {
		t.Fatalf("expected 2 instances, got %d", len(got))
	}

	// Filter by region.
	byRegion := mgr.ListInstancesByRegion(ctx, "us-east-1")
	if len(byRegion) != 2 {
		t.Fatalf("expected 2 instances in us-east-1, got %d", len(byRegion))
	}

	// Non-existent region returns empty.
	byRegion = mgr.ListInstancesByRegion(ctx, "eu-west-1")
	if len(byRegion) != 0 {
		t.Fatalf("expected 0 instances in eu-west-1, got %d", len(byRegion))
	}
}

func TestRegions(t *testing.T) {
	mgr := NewManager(DefaultManagerConfig())
	ctx := context.Background()
	defer mgr.Close()

	err := mgr.AddRegion(ctx, &Region{
		Name:     "us-east-1",
		Provider: "aws",
		Primary:  true,
	})
	if err != nil {
		t.Fatalf("adding region: %v", err)
	}

	err = mgr.AddRegion(ctx, &Region{
		Name:     "eu-west-1",
		Provider: "aws",
	})
	if err != nil {
		t.Fatalf("adding region: %v", err)
	}

	regions := mgr.ListRegions(ctx)
	if len(regions) != 2 {
		t.Fatalf("expected 2 regions, got %d", len(regions))
	}

	// Duplicate region should fail.
	err = mgr.AddRegion(ctx, &Region{Name: "us-east-1", Provider: "aws"})
	if err == nil {
		t.Fatal("expected error for duplicate region")
	}

	// Nil region should fail.
	err = mgr.AddRegion(ctx, nil)
	if err == nil {
		t.Fatal("expected error for nil region")
	}

	// Empty name should fail.
	err = mgr.AddRegion(ctx, &Region{Provider: "aws"})
	if err == nil {
		t.Fatal("expected error for empty region name")
	}
}

func TestPolicies(t *testing.T) {
	mgr := NewManager(DefaultManagerConfig())
	ctx := context.Background()
	defer mgr.Close()

	policy := &Policy{
		Name:            "prod",
		ReplicationMode: ReplicationSync,
		Regions:         []string{"us-east-1", "eu-west-1"},
		MaxInstances:    10,
		AutoScale:       true,
	}

	if err := mgr.AddPolicy(ctx, policy); err != nil {
		t.Fatalf("adding policy: %v", err)
	}

	got, err := mgr.GetPolicy(ctx, "prod")
	if err != nil {
		t.Fatalf("getting policy: %v", err)
	}
	if got.Name != "prod" {
		t.Fatalf("expected policy name %q, got %q", "prod", got.Name)
	}
	if got.ReplicationMode != ReplicationSync {
		t.Fatalf("expected replication mode %q, got %q", ReplicationSync, got.ReplicationMode)
	}

	// Missing policy should fail.
	_, err = mgr.GetPolicy(ctx, "nonexistent")
	if err == nil {
		t.Fatal("expected error for missing policy")
	}

	// Nil policy should fail.
	err = mgr.AddPolicy(ctx, nil)
	if err == nil {
		t.Fatal("expected error for nil policy")
	}

	// List policies.
	policies := mgr.ListPolicies(ctx)
	if len(policies) != 1 {
		t.Fatalf("expected 1 policy, got %d", len(policies))
	}
}

func TestFleetStatus(t *testing.T) {
	mgr, ctx := setupManager(t)
	defer mgr.Close()

	inst1 := registerTestInstance(t, mgr, ctx, "feather-1")
	inst2 := registerTestInstance(t, mgr, ctx, "feather-2")
	inst3 := registerTestInstance(t, mgr, ctx, "feather-3")

	// Set varied statuses.
	mgr.UpdateInstanceStatus(ctx, inst1.ID, InstanceStatusHealthy)
	mgr.UpdateInstanceStatus(ctx, inst2.ID, InstanceStatusDegraded)
	mgr.UpdateInstanceStatus(ctx, inst3.ID, InstanceStatusUnhealthy)

	status := mgr.GetFleetStatus(ctx)

	if status.TotalInstances != 3 {
		t.Fatalf("expected 3 total instances, got %d", status.TotalInstances)
	}
	if status.HealthyInstances != 1 {
		t.Fatalf("expected 1 healthy instance, got %d", status.HealthyInstances)
	}
	if status.DegradedInstances != 1 {
		t.Fatalf("expected 1 degraded instance, got %d", status.DegradedInstances)
	}
	if status.UnhealthyInstances != 1 {
		t.Fatalf("expected 1 unhealthy instance, got %d", status.UnhealthyInstances)
	}
	if status.TotalRegions != 1 {
		t.Fatalf("expected 1 region, got %d", status.TotalRegions)
	}
	if status.ReplicationMode != ReplicationAsync {
		t.Fatalf("expected replication mode %q, got %q", ReplicationAsync, status.ReplicationMode)
	}
}

func TestReplicationRules(t *testing.T) {
	rm := NewReplicationManager(DefaultReplicationConfig())
	ctx := context.Background()

	rule := &ReplicationRule{
		SourceRegion: "us-east-1",
		TargetRegion: "eu-west-1",
		Mode:         ReplicationAsync,
		Enabled:      true,
	}

	if err := rm.AddRule(ctx, rule); err != nil {
		t.Fatalf("adding rule: %v", err)
	}
	if rule.ID == "" {
		t.Fatal("expected rule ID to be assigned")
	}

	// Verify rule is retrievable.
	got, err := rm.GetRule(ctx, rule.ID)
	if err != nil {
		t.Fatalf("getting rule: %v", err)
	}
	if got.SourceRegion != "us-east-1" {
		t.Fatalf("expected source %q, got %q", "us-east-1", got.SourceRegion)
	}

	// Verify initial status.
	status, err := rm.GetStatus(ctx, rule.ID)
	if err != nil {
		t.Fatalf("getting status: %v", err)
	}
	if status.State != "idle" {
		t.Fatalf("expected state %q, got %q", "idle", status.State)
	}

	// List rules.
	rules := rm.ListRules(ctx)
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(rules))
	}

	// Same source and target should fail.
	err = rm.AddRule(ctx, &ReplicationRule{
		SourceRegion: "us-east-1",
		TargetRegion: "us-east-1",
		Mode:         ReplicationSync,
	})
	if err == nil {
		t.Fatal("expected error for same source and target")
	}

	// Nil rule should fail.
	err = rm.AddRule(ctx, nil)
	if err == nil {
		t.Fatal("expected error for nil rule")
	}

	// Remove rule.
	if err := rm.RemoveRule(ctx, rule.ID); err != nil {
		t.Fatalf("removing rule: %v", err)
	}
	if len(rm.ListRules(ctx)) != 0 {
		t.Fatal("expected 0 rules after removal")
	}

	// Remove unknown rule should fail.
	err = rm.RemoveRule(ctx, "nonexistent")
	if err == nil {
		t.Fatal("expected error removing unknown rule")
	}
}

func TestUpdateMetrics(t *testing.T) {
	mgr, ctx := setupManager(t)
	defer mgr.Close()

	inst := registerTestInstance(t, mgr, ctx, "feather-1")

	metrics := &InstanceMetrics{
		CPUUsage:       45.5,
		MemoryUsage:    72.3,
		RequestsPerSec: 1500.0,
		AvgLatencyMs:   0.8,
		ErrorRate:      0.01,
		FeatureCount:   10000,
		EntityCount:    50000,
	}

	if err := mgr.UpdateInstanceMetrics(ctx, inst.ID, metrics); err != nil {
		t.Fatalf("updating metrics: %v", err)
	}

	got, err := mgr.GetInstance(ctx, inst.ID)
	if err != nil {
		t.Fatalf("getting instance: %v", err)
	}
	if got.Metrics == nil {
		t.Fatal("expected metrics to be set")
	}
	if got.Metrics.CPUUsage != 45.5 {
		t.Fatalf("expected CPU usage 45.5, got %f", got.Metrics.CPUUsage)
	}
	if got.Metrics.FeatureCount != 10000 {
		t.Fatalf("expected feature count 10000, got %d", got.Metrics.FeatureCount)
	}

	// Nil metrics should fail.
	err = mgr.UpdateInstanceMetrics(ctx, inst.ID, nil)
	if err == nil {
		t.Fatal("expected error for nil metrics")
	}

	// Unknown instance should fail.
	err = mgr.UpdateInstanceMetrics(ctx, "nonexistent", metrics)
	if err == nil {
		t.Fatal("expected error for unknown instance")
	}
}

func TestInstanceLifecycle(t *testing.T) {
	mgr, ctx := setupManager(t)
	defer mgr.Close()

	// Register.
	inst := registerTestInstance(t, mgr, ctx, "feather-lifecycle")
	if inst.Status != InstanceStatusProvisioning {
		t.Fatalf("expected provisioning status, got %q", inst.Status)
	}

	// Transition to healthy.
	if err := mgr.UpdateInstanceStatus(ctx, inst.ID, InstanceStatusHealthy); err != nil {
		t.Fatalf("updating status to healthy: %v", err)
	}
	got, _ := mgr.GetInstance(ctx, inst.ID)
	if got.Status != InstanceStatusHealthy {
		t.Fatalf("expected healthy status, got %q", got.Status)
	}

	// Degrade.
	if err := mgr.UpdateInstanceStatus(ctx, inst.ID, InstanceStatusDegraded); err != nil {
		t.Fatalf("updating status to degraded: %v", err)
	}
	got, _ = mgr.GetInstance(ctx, inst.ID)
	if got.Status != InstanceStatusDegraded {
		t.Fatalf("expected degraded status, got %q", got.Status)
	}

	// Decommission.
	if err := mgr.UpdateInstanceStatus(ctx, inst.ID, InstanceStatusDecommissioned); err != nil {
		t.Fatalf("updating status to decommissioned: %v", err)
	}
	got, _ = mgr.GetInstance(ctx, inst.ID)
	if got.Status != InstanceStatusDecommissioned {
		t.Fatalf("expected decommissioned status, got %q", got.Status)
	}

	// Deregister.
	if err := mgr.DeregisterInstance(ctx, inst.ID); err != nil {
		t.Fatalf("deregistering instance: %v", err)
	}

	// Confirm gone.
	_, err := mgr.GetInstance(ctx, inst.ID)
	if err == nil {
		t.Fatal("expected error getting deregistered instance")
	}

	// Updating status on unknown instance should fail.
	err = mgr.UpdateInstanceStatus(ctx, "nonexistent", InstanceStatusHealthy)
	if err == nil {
		t.Fatal("expected error updating unknown instance status")
	}
}
