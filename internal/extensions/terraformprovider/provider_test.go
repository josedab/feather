package terraformprovider

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewProvider(t *testing.T) {
	cfg := DefaultProviderConfig()
	p := NewProvider(cfg)
	require.NotNil(t, p)
	assert.Equal(t, "http://localhost:8080", p.config.ServerURL)
	assert.Equal(t, 100000, p.config.MaxResources)
}

func TestCreateResource(t *testing.T) {
	p := NewProvider(DefaultProviderConfig())

	attrs := map[string]interface{}{"name": "user_features", "ttl": 3600}
	state, err := p.CreateResource(ResourceFeatureGroup, "fg-1", attrs)
	require.NoError(t, err)
	assert.Equal(t, "fg-1", state.ID)
	assert.Equal(t, string(ResourceFeatureGroup), state.Type)
	assert.Equal(t, 1, state.Version)
	assert.Equal(t, "user_features", state.Attributes["name"])
}

func TestReadResource(t *testing.T) {
	p := NewProvider(DefaultProviderConfig())

	_, err := p.CreateResource(ResourceSchema, "sch-1", map[string]interface{}{"format": "avro"})
	require.NoError(t, err)

	state, err := p.ReadResource("sch-1")
	require.NoError(t, err)
	assert.Equal(t, "sch-1", state.ID)
	assert.Equal(t, "avro", state.Attributes["format"])

	_, err = p.ReadResource("nonexistent")
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrResourceNotFound))
}

func TestUpdateResource(t *testing.T) {
	p := NewProvider(DefaultProviderConfig())

	_, err := p.CreateResource(ResourceSLA, "sla-1", map[string]interface{}{"latency_ms": 10})
	require.NoError(t, err)

	state, err := p.UpdateResource("sla-1", map[string]interface{}{"latency_ms": 5})
	require.NoError(t, err)
	assert.Equal(t, 2, state.Version)
	assert.Equal(t, 5, state.Attributes["latency_ms"])
}

func TestDeleteResource(t *testing.T) {
	p := NewProvider(DefaultProviderConfig())

	_, err := p.CreateResource(ResourceContract, "ct-1", map[string]interface{}{})
	require.NoError(t, err)
	require.NoError(t, p.DeleteResource("ct-1"))

	_, err = p.ReadResource("ct-1")
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrResourceNotFound))

	err = p.DeleteResource("ct-1")
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrResourceNotFound))
}

func TestPlan(t *testing.T) {
	p := NewProvider(DefaultProviderConfig())

	// Create an existing resource
	_, err := p.CreateResource(ResourceFeatureGroup, "fg-existing", map[string]interface{}{"name": "old"})
	require.NoError(t, err)

	desired := []ResourceState{
		{ID: "fg-new", Attributes: map[string]interface{}{"name": "new_group"}},
		{ID: "fg-existing", Attributes: map[string]interface{}{"name": "updated"}},
	}

	plan := p.Plan(desired)
	require.Len(t, plan, 2)

	actionMap := make(map[string]PlanAction)
	for _, pr := range plan {
		actionMap[pr.ResourceID] = pr.Action
	}

	assert.Equal(t, PlanCreate, actionMap["fg-new"])
	assert.Equal(t, PlanUpdate, actionMap["fg-existing"])
}

func TestApply(t *testing.T) {
	p := NewProvider(DefaultProviderConfig())

	plan := []PlanResult{
		{ResourceID: "r1", Action: PlanCreate, After: map[string]interface{}{"name": "res1"}},
		{ResourceID: "r2", Action: PlanCreate, After: map[string]interface{}{"name": "res2"}},
	}

	results := p.Apply(plan)
	require.Len(t, results, 2)
	for _, r := range results {
		assert.True(t, r.Success)
	}

	state, err := p.ReadResource("r1")
	require.NoError(t, err)
	assert.Equal(t, "res1", state.Attributes["name"])
}

func TestImportResource(t *testing.T) {
	p := NewProvider(DefaultProviderConfig())

	state, err := p.ImportResource(ResourceFeatureGroup, "imported-1")
	require.NoError(t, err)
	assert.Equal(t, "imported-1", state.ID)
	assert.Equal(t, string(ResourceFeatureGroup), state.Type)
	assert.Equal(t, 1, state.Version)

	// verify it's readable
	got, err := p.ReadResource("imported-1")
	require.NoError(t, err)
	assert.Equal(t, "imported-1", got.ID)
}

func TestDuplicateResource(t *testing.T) {
	p := NewProvider(DefaultProviderConfig())

	_, err := p.CreateResource(ResourceFeatureGroup, "dup-1", map[string]interface{}{})
	require.NoError(t, err)

	_, err = p.CreateResource(ResourceFeatureGroup, "dup-1", map[string]interface{}{})
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrResourceExists))
}

func TestStats(t *testing.T) {
	p := NewProvider(DefaultProviderConfig())

	_, _ = p.CreateResource(ResourceFeatureGroup, "s1", map[string]interface{}{})
	_, _ = p.CreateResource(ResourceFeatureGroup, "s2", map[string]interface{}{})
	_, _ = p.CreateResource(ResourceSchema, "s3", map[string]interface{}{})

	stats := p.Stats()
	assert.Equal(t, 3, stats.TotalResources)
	assert.Equal(t, 2, stats.ResourcesByType[string(ResourceFeatureGroup)])
	assert.Equal(t, 1, stats.ResourcesByType[string(ResourceSchema)])
}
