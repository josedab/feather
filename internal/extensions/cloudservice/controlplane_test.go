package cloudservice

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestControlPlane_Provision(t *testing.T) {
	cp := NewControlPlane()

	spec := DefaultSpecForTier(InstanceTierPro)
	inst, err := cp.Provision("tenant-1", "my-store", spec)
	require.NoError(t, err)
	assert.Equal(t, InstanceStatusProvisioning, inst.Status)
	assert.Equal(t, "tenant-1", inst.TenantID)
	assert.Contains(t, inst.Endpoint, "my-store")
	assert.Equal(t, 3, inst.Spec.Replicas)
}

func TestControlPlane_ProvisionValidation(t *testing.T) {
	cp := NewControlPlane()
	spec := DefaultSpecForTier(InstanceTierFree)

	_, err := cp.Provision("", "name", spec)
	require.Error(t, err)

	_, err = cp.Provision("tenant", "", spec)
	require.Error(t, err)
}

func TestControlPlane_Scale(t *testing.T) {
	cp := NewControlPlane()
	spec := DefaultSpecForTier(InstanceTierPro)
	inst, _ := cp.Provision("t1", "test", spec)

	err := cp.Scale(inst.ID, 5)
	require.NoError(t, err)

	got, _ := cp.Get(inst.ID)
	assert.Equal(t, 5, got.Spec.Replicas)
	assert.Equal(t, InstanceStatusScaling, got.Status)
}

func TestControlPlane_ScaleEnforcesBounds(t *testing.T) {
	cp := NewControlPlane()
	spec := DefaultSpecForTier(InstanceTierPro) // min=2, max=10
	inst, _ := cp.Provision("t1", "test", spec)

	_ = cp.Scale(inst.ID, 100)
	got, _ := cp.Get(inst.ID)
	assert.Equal(t, 10, got.Spec.Replicas)

	_ = cp.Scale(inst.ID, 0)
	got, _ = cp.Get(inst.ID)
	assert.Equal(t, 2, got.Spec.Replicas)
}

func TestControlPlane_EvaluateAutoScale(t *testing.T) {
	cp := NewControlPlane()
	spec := DefaultSpecForTier(InstanceTierPro)
	inst, _ := cp.Provision("t1", "test", spec)
	_ = cp.UpdateStatus(inst.ID, InstanceStatusRunning)

	// High CPU should trigger scale up
	_ = cp.UpdateMetrics(inst.ID, &InstanceMetrics{CPUUsagePct: 85, MemoryUsagePct: 50})
	decisions := cp.EvaluateAutoScale()
	require.Len(t, decisions, 1)
	assert.Equal(t, 4, decisions[0].DesiredScale)
}

func TestControlPlane_Terminate(t *testing.T) {
	cp := NewControlPlane()
	inst, _ := cp.Provision("t1", "test", DefaultSpecForTier(InstanceTierFree))

	err := cp.Terminate(inst.ID)
	require.NoError(t, err)

	got, _ := cp.Get(inst.ID)
	assert.Equal(t, InstanceStatusTerminated, got.Status)

	// Terminated instances not in list
	list := cp.List("")
	assert.Empty(t, list)
}

func TestControlPlane_List(t *testing.T) {
	cp := NewControlPlane()
	_, _ = cp.Provision("t1", "a", DefaultSpecForTier(InstanceTierFree))
	_, _ = cp.Provision("t2", "b", DefaultSpecForTier(InstanceTierFree))
	_, _ = cp.Provision("t1", "c", DefaultSpecForTier(InstanceTierFree))

	all := cp.List("")
	assert.Len(t, all, 3)

	t1 := cp.List("t1")
	assert.Len(t, t1, 2)
}

func TestDefaultSpecForTier(t *testing.T) {
	tests := []struct {
		tier     InstanceTier
		vcpus    int
		memoryGB int
	}{
		{InstanceTierFree, 1, 1},
		{InstanceTierStarter, 2, 4},
		{InstanceTierPro, 4, 16},
		{InstanceTierEnterprise, 16, 64},
	}

	for _, tt := range tests {
		t.Run(string(tt.tier), func(t *testing.T) {
			spec := DefaultSpecForTier(tt.tier)
			assert.Equal(t, tt.vcpus, spec.VCPUs)
			assert.Equal(t, tt.memoryGB, spec.MemoryGB)
		})
	}
}

func TestControlPlane_Stats(t *testing.T) {
	cp := NewControlPlane()
	inst, _ := cp.Provision("t1", "a", DefaultSpecForTier(InstanceTierFree))
	_ = cp.UpdateStatus(inst.ID, InstanceStatusRunning)

	stats := cp.Stats()
	assert.Equal(t, 1, stats["total_instances"])
	assert.Equal(t, 1, stats["running_instances"])
}
