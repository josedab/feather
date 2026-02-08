package abrollout

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestManager_CreateVersion(t *testing.T) {
	m := NewManager()

	v1, err := m.CreateVersion("clicks", "raw_clicks", "alice")
	require.NoError(t, err)
	assert.Equal(t, 1, v1.Version)
	assert.Equal(t, "clicks", v1.FeatureName)

	v2, err := m.CreateVersion("clicks", "log(raw_clicks)", "bob")
	require.NoError(t, err)
	assert.Equal(t, 2, v2.Version)
}

func TestManager_CreateVersionValidation(t *testing.T) {
	m := NewManager()
	_, err := m.CreateVersion("", "expr", "user")
	require.Error(t, err)
}

func TestManager_GetVersion(t *testing.T) {
	m := NewManager()
	_, _ = m.CreateVersion("clicks", "v1", "a")
	_, _ = m.CreateVersion("clicks", "v2", "a")

	v, err := m.GetVersion("clicks", 2)
	require.NoError(t, err)
	assert.Equal(t, "v2", v.Expression)

	_, err = m.GetVersion("clicks", 99)
	require.Error(t, err)
}

func TestManager_ListVersions(t *testing.T) {
	m := NewManager()
	_, _ = m.CreateVersion("clicks", "v1", "a")
	_, _ = m.CreateVersion("clicks", "v2", "a")

	versions := m.ListVersions("clicks")
	assert.Len(t, versions, 2)
}

func TestManager_StartRollout(t *testing.T) {
	m := NewManager()
	_, _ = m.CreateVersion("clicks", "v1", "a")
	_, _ = m.CreateVersion("clicks", "v2", "a")

	rollout, err := m.StartRollout("clicks", 1, 2, nil, true, true)
	require.NoError(t, err)
	assert.Equal(t, RolloutStatusCanary, rollout.Status)
	assert.Equal(t, float64(1), rollout.TrafficPct) // first step from DefaultRolloutSteps
}

func TestManager_StartRollout_InvalidVersion(t *testing.T) {
	m := NewManager()
	_, _ = m.CreateVersion("clicks", "v1", "a")

	_, err := m.StartRollout("clicks", 1, 99, nil, false, false)
	require.Error(t, err)
}

func TestManager_StartRollout_AlreadyActive(t *testing.T) {
	m := NewManager()
	_, _ = m.CreateVersion("clicks", "v1", "a")
	_, _ = m.CreateVersion("clicks", "v2", "a")
	_, _ = m.StartRollout("clicks", 1, 2, nil, false, false)

	_, err := m.StartRollout("clicks", 1, 2, nil, false, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already has an active rollout")
}

func TestManager_Advance(t *testing.T) {
	m := NewManager()
	_, _ = m.CreateVersion("clicks", "v1", "a")
	_, _ = m.CreateVersion("clicks", "v2", "a")

	steps := []RolloutStep{
		{TrafficPct: 10},
		{TrafficPct: 50},
		{TrafficPct: 100},
	}
	rollout, _ := m.StartRollout("clicks", 1, 2, steps, false, false)

	// Advance to 50%
	err := m.Advance(rollout.ID)
	require.NoError(t, err)
	assert.Equal(t, float64(50), m.rollouts[rollout.ID].TrafficPct)

	// Advance to 100%
	err = m.Advance(rollout.ID)
	require.NoError(t, err)
	assert.Equal(t, float64(100), m.rollouts[rollout.ID].TrafficPct)

	// Advance past last step -> complete
	err = m.Advance(rollout.ID)
	require.NoError(t, err)
	assert.Equal(t, RolloutStatusComplete, m.rollouts[rollout.ID].Status)

	// No active rollout anymore
	assert.Nil(t, m.GetActiveRollout("clicks"))
}

func TestManager_Rollback(t *testing.T) {
	m := NewManager()
	_, _ = m.CreateVersion("clicks", "v1", "a")
	_, _ = m.CreateVersion("clicks", "v2", "a")
	rollout, _ := m.StartRollout("clicks", 1, 2, nil, false, false)

	err := m.Rollback(rollout.ID)
	require.NoError(t, err)
	assert.Equal(t, RolloutStatusRolledBack, m.rollouts[rollout.ID].Status)
	assert.Nil(t, m.GetActiveRollout("clicks"))
}

func TestManager_Pause(t *testing.T) {
	m := NewManager()
	_, _ = m.CreateVersion("clicks", "v1", "a")
	_, _ = m.CreateVersion("clicks", "v2", "a")
	rollout, _ := m.StartRollout("clicks", 1, 2, nil, false, false)

	err := m.Pause(rollout.ID)
	require.NoError(t, err)
	assert.Equal(t, RolloutStatusPaused, m.rollouts[rollout.ID].Status)
}

func TestManager_EvaluateQualityGates_Pass(t *testing.T) {
	m := NewManager()
	_, _ = m.CreateVersion("clicks", "v1", "a")
	_, _ = m.CreateVersion("clicks", "v2", "a")
	rollout, _ := m.StartRollout("clicks", 1, 2, nil, false, false)

	_ = m.UpdateMetrics(rollout.ID, &RolloutMetrics{
		CanaryErrorRate: 0.01,
		CanaryLatencyMs: 50,
		CanaryDrift:     0.05,
	})

	healthy, reason := m.EvaluateQualityGates(rollout.ID)
	assert.True(t, healthy)
	assert.Contains(t, reason, "passed")
}

func TestManager_EvaluateQualityGates_Fail(t *testing.T) {
	m := NewManager()
	_, _ = m.CreateVersion("clicks", "v1", "a")
	_, _ = m.CreateVersion("clicks", "v2", "a")
	rollout, _ := m.StartRollout("clicks", 1, 2, nil, false, false)

	_ = m.UpdateMetrics(rollout.ID, &RolloutMetrics{
		CanaryErrorRate: 0.10, // exceeds 0.05 threshold
		CanaryLatencyMs: 50,
		CanaryDrift:     0.05,
	})

	healthy, reason := m.EvaluateQualityGates(rollout.ID)
	assert.False(t, healthy)
	assert.Contains(t, reason, "error rate")
}

func TestManager_ResolveVersion(t *testing.T) {
	m := NewManager()
	_, _ = m.CreateVersion("clicks", "v1", "a")
	_, _ = m.CreateVersion("clicks", "v2", "a")

	// No rollout: should return latest version
	v := m.ResolveVersion("clicks", "user:123")
	assert.Equal(t, 2, v)

	// With rollout at 50%: different entities should get different versions
	_, _ = m.StartRollout("clicks", 1, 2, []RolloutStep{{TrafficPct: 50}}, false, false)

	v1Count, v2Count := 0, 0
	for i := 0; i < 100; i++ {
		entity := fmt.Sprintf("user:%d", i)
		if m.ResolveVersion("clicks", entity) == 1 {
			v1Count++
		} else {
			v2Count++
		}
	}
	// With 50% split, both should have meaningful traffic
	assert.Greater(t, v1Count, 10)
	assert.Greater(t, v2Count, 10)
}

func TestManager_ResolveVersion_DeterministicHashing(t *testing.T) {
	m := NewManager()
	_, _ = m.CreateVersion("clicks", "v1", "a")
	_, _ = m.CreateVersion("clicks", "v2", "a")
	_, _ = m.StartRollout("clicks", 1, 2, []RolloutStep{{TrafficPct: 50}}, false, false)

	// Same entity should always get the same version
	v1 := m.ResolveVersion("clicks", "user:42")
	v2 := m.ResolveVersion("clicks", "user:42")
	assert.Equal(t, v1, v2)
}

func TestManager_Stats(t *testing.T) {
	m := NewManager()
	_, _ = m.CreateVersion("clicks", "v1", "a")
	_, _ = m.CreateVersion("clicks", "v2", "a")
	_, _ = m.StartRollout("clicks", 1, 2, nil, false, false)

	stats := m.Stats()
	assert.Equal(t, 1, stats["total_features"])
	assert.Equal(t, 1, stats["total_rollouts"])
	assert.Equal(t, 1, stats["active_rollouts"])
}

func TestDefaultRolloutSteps(t *testing.T) {
	steps := DefaultRolloutSteps()
	assert.Len(t, steps, 5)
	assert.Equal(t, float64(1), steps[0].TrafficPct)
	assert.Equal(t, float64(100), steps[4].TrafficPct)
}
