package governance

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultResidencyConfig(t *testing.T) {
	config := DefaultResidencyConfig()

	assert.True(t, config.Enabled)
	assert.Equal(t, RegionUSEast, config.CurrentRegion)
	assert.Equal(t, ZoneUS, config.CurrentZone)
	assert.Equal(t, RequirementNone, config.DefaultRequirement)
	assert.True(t, config.EnforceOnWrite)
	assert.False(t, config.EnforceOnRead)
	assert.True(t, config.EnforceOnExport)
	assert.True(t, config.AuditEnabled)
}

func TestRegion_Values(t *testing.T) {
	assert.Equal(t, Region("us-east"), RegionUSEast)
	assert.Equal(t, Region("us-west"), RegionUSWest)
	assert.Equal(t, Region("eu-west"), RegionEUWest)
	assert.Equal(t, Region("eu-central"), RegionEUCentral)
	assert.Equal(t, Region("ap-southeast"), RegionAPSoutheast)
	assert.Equal(t, Region("ap-northeast"), RegionAPNortheast)
	assert.Equal(t, Region("sa-east"), RegionSAEast)
	assert.Equal(t, Region("global"), RegionGlobal)
}

func TestRegionZone_Values(t *testing.T) {
	assert.Equal(t, RegionZone("us"), ZoneUS)
	assert.Equal(t, RegionZone("eu"), ZoneEU)
	assert.Equal(t, RegionZone("apac"), ZoneAPAC)
	assert.Equal(t, RegionZone("latam"), ZoneLATAM)
	assert.Equal(t, RegionZone("global"), ZoneGlobal)
}

func TestResidencyRequirement_Values(t *testing.T) {
	assert.Equal(t, ResidencyRequirement("none"), RequirementNone)
	assert.Equal(t, ResidencyRequirement("same_zone"), RequirementSameZone)
	assert.Equal(t, ResidencyRequirement("same_region"), RequirementSameRegion)
	assert.Equal(t, ResidencyRequirement("specific"), RequirementSpecific)
}

func TestDataClassification_Values(t *testing.T) {
	assert.Equal(t, DataClassification("public"), ClassificationPublic)
	assert.Equal(t, DataClassification("internal"), ClassificationInternal)
	assert.Equal(t, DataClassification("confidential"), ClassificationConfidential)
	assert.Equal(t, DataClassification("restricted"), ClassificationRestricted)
}

func TestNewResidencyController(t *testing.T) {
	config := DefaultResidencyConfig()
	controller := NewResidencyController(config, nil)

	require.NotNil(t, controller)
	assert.NotNil(t, controller.policies)
	assert.NotNil(t, controller.byFeature)
}

func TestResidencyController_AddPolicy(t *testing.T) {
	config := DefaultResidencyConfig()
	controller := NewResidencyController(config, nil)

	policy := &ResidencyPolicy{
		ID:             "policy-1",
		Name:           "EU Data Policy",
		FeatureNames:   []string{"user_data"},
		AllowedRegions: []Region{RegionEUWest, RegionEUCentral},
		Requirement:    RequirementSameZone,
		Enabled:        true,
	}

	err := controller.AddPolicy(policy)
	require.NoError(t, err)

	retrieved, err := controller.GetPolicy("policy-1")
	require.NoError(t, err)
	assert.Equal(t, "EU Data Policy", retrieved.Name)
	assert.NotZero(t, retrieved.CreatedAt)
}

func TestResidencyController_AddPolicy_InvalidID(t *testing.T) {
	config := DefaultResidencyConfig()
	controller := NewResidencyController(config, nil)

	policy := &ResidencyPolicy{
		ID: "", // Empty ID
	}

	err := controller.AddPolicy(policy)
	assert.ErrorIs(t, err, ErrInvalidResidencyPolicy)
}

func TestResidencyController_UpdatePolicy(t *testing.T) {
	config := DefaultResidencyConfig()
	controller := NewResidencyController(config, nil)

	policy := &ResidencyPolicy{
		ID:           "policy-1",
		Name:         "Original",
		FeatureNames: []string{"feature1"},
		Enabled:      true,
	}
	_ = controller.AddPolicy(policy)

	updatedPolicy := &ResidencyPolicy{
		ID:           "policy-1",
		Name:         "Updated",
		FeatureNames: []string{"feature2"},
		Enabled:      true,
	}

	err := controller.UpdatePolicy(updatedPolicy)
	require.NoError(t, err)

	retrieved, _ := controller.GetPolicy("policy-1")
	assert.Equal(t, "Updated", retrieved.Name)
}

func TestResidencyController_UpdatePolicy_NotFound(t *testing.T) {
	config := DefaultResidencyConfig()
	controller := NewResidencyController(config, nil)

	policy := &ResidencyPolicy{
		ID: "nonexistent",
	}

	err := controller.UpdatePolicy(policy)
	assert.ErrorIs(t, err, ErrInvalidResidencyPolicy)
}

func TestResidencyController_DeletePolicy(t *testing.T) {
	config := DefaultResidencyConfig()
	controller := NewResidencyController(config, nil)

	policy := &ResidencyPolicy{
		ID:           "policy-1",
		FeatureNames: []string{"feature1"},
		Enabled:      true,
	}
	_ = controller.AddPolicy(policy)

	err := controller.DeletePolicy("policy-1")
	require.NoError(t, err)

	_, err = controller.GetPolicy("policy-1")
	assert.ErrorIs(t, err, ErrInvalidResidencyPolicy)
}

func TestResidencyController_DeletePolicy_NotFound(t *testing.T) {
	config := DefaultResidencyConfig()
	controller := NewResidencyController(config, nil)

	err := controller.DeletePolicy("nonexistent")
	assert.ErrorIs(t, err, ErrInvalidResidencyPolicy)
}

func TestResidencyController_GetPolicy_NotFound(t *testing.T) {
	config := DefaultResidencyConfig()
	controller := NewResidencyController(config, nil)

	_, err := controller.GetPolicy("nonexistent")
	assert.ErrorIs(t, err, ErrInvalidResidencyPolicy)
}

func TestResidencyController_ListPolicies(t *testing.T) {
	config := DefaultResidencyConfig()
	controller := NewResidencyController(config, nil)

	_ = controller.AddPolicy(&ResidencyPolicy{ID: "policy-1", Enabled: true})
	_ = controller.AddPolicy(&ResidencyPolicy{ID: "policy-2", Enabled: true})
	_ = controller.AddPolicy(&ResidencyPolicy{ID: "policy-3", Enabled: true})

	policies := controller.ListPolicies()
	assert.Len(t, policies, 3)
}

func TestResidencyController_CheckWrite_Disabled(t *testing.T) {
	config := ResidencyConfig{
		Enabled: false,
	}
	controller := NewResidencyController(config, nil)

	ctx := context.Background()
	check := controller.CheckWrite(ctx, "feature", RegionEUWest)

	assert.True(t, check.Allowed)
}

func TestResidencyController_CheckWrite_AllowedRegion(t *testing.T) {
	config := ResidencyConfig{
		Enabled:       true,
		CurrentRegion: RegionUSEast,
		CurrentZone:   ZoneUS,
	}
	controller := NewResidencyController(config, nil)

	_ = controller.AddPolicy(&ResidencyPolicy{
		ID:                    "policy-1",
		FeatureNames:          []string{"us_data"},
		AllowedRegions:        []Region{RegionUSEast, RegionUSWest},
		AllowCrossRegionWrite: true, // Allow cross-region writes
		Enabled:               true,
	})

	ctx := context.Background()
	check := controller.CheckWrite(ctx, "us_data", RegionUSWest)

	assert.True(t, check.Allowed)
}

func TestResidencyController_CheckWrite_DeniedRegion(t *testing.T) {
	config := ResidencyConfig{
		Enabled:       true,
		CurrentRegion: RegionUSEast,
		CurrentZone:   ZoneUS,
	}
	controller := NewResidencyController(config, nil)

	_ = controller.AddPolicy(&ResidencyPolicy{
		ID:           "policy-1",
		FeatureNames: []string{"sensitive"},
		DenyRegions:  []Region{RegionEUWest, RegionEUCentral},
		Enabled:      true,
	})

	ctx := context.Background()
	check := controller.CheckWrite(ctx, "sensitive", RegionEUWest)

	assert.False(t, check.Allowed)
	assert.Contains(t, check.Violation, "denied")
}

func TestResidencyController_CheckWrite_NotAllowedRegion(t *testing.T) {
	config := ResidencyConfig{
		Enabled:       true,
		CurrentRegion: RegionEUWest,
		CurrentZone:   ZoneEU,
	}
	controller := NewResidencyController(config, nil)

	_ = controller.AddPolicy(&ResidencyPolicy{
		ID:             "policy-1",
		FeatureNames:   []string{"eu_only"},
		AllowedRegions: []Region{RegionEUWest, RegionEUCentral},
		Enabled:        true,
	})

	ctx := context.Background()
	check := controller.CheckWrite(ctx, "eu_only", RegionUSEast)

	assert.False(t, check.Allowed)
	assert.Contains(t, check.Violation, "not allowed")
}

func TestResidencyController_CheckWrite_AllowedZone(t *testing.T) {
	config := ResidencyConfig{
		Enabled:       true,
		CurrentRegion: RegionUSEast,
		CurrentZone:   ZoneUS,
	}
	controller := NewResidencyController(config, nil)

	_ = controller.AddPolicy(&ResidencyPolicy{
		ID:                    "policy-1",
		FeatureNames:          []string{"us_zone_data"},
		AllowedZones:          []RegionZone{ZoneUS},
		AllowCrossRegionWrite: true, // Allow cross-region writes within zone
		Enabled:               true,
	})

	ctx := context.Background()
	check := controller.CheckWrite(ctx, "us_zone_data", RegionUSWest)

	assert.True(t, check.Allowed)
}

func TestResidencyController_CheckWrite_NotAllowedZone(t *testing.T) {
	config := ResidencyConfig{
		Enabled:       true,
		CurrentRegion: RegionEUWest,
		CurrentZone:   ZoneEU,
	}
	controller := NewResidencyController(config, nil)

	_ = controller.AddPolicy(&ResidencyPolicy{
		ID:           "policy-1",
		FeatureNames: []string{"eu_zone_data"},
		AllowedZones: []RegionZone{ZoneEU},
		Enabled:      true,
	})

	ctx := context.Background()
	check := controller.CheckWrite(ctx, "eu_zone_data", RegionUSEast)

	assert.False(t, check.Allowed)
	assert.Contains(t, check.Violation, "zone")
}

func TestResidencyController_CheckWrite_GlobalZone(t *testing.T) {
	config := ResidencyConfig{
		Enabled:       true,
		CurrentRegion: RegionUSEast,
		CurrentZone:   ZoneUS,
	}
	controller := NewResidencyController(config, nil)

	_ = controller.AddPolicy(&ResidencyPolicy{
		ID:                    "policy-1",
		FeatureNames:          []string{"global_data"},
		AllowedZones:          []RegionZone{ZoneGlobal},
		AllowCrossRegionWrite: true, // Allow cross-region writes for global data
		Enabled:               true,
	})

	ctx := context.Background()

	// Should allow any region with global zone
	check := controller.CheckWrite(ctx, "global_data", RegionAPNortheast)
	assert.True(t, check.Allowed)
}

func TestResidencyController_CheckWrite_SameRegionRequirement(t *testing.T) {
	config := ResidencyConfig{
		Enabled:       true,
		CurrentRegion: RegionUSEast,
		CurrentZone:   ZoneUS,
	}
	controller := NewResidencyController(config, nil)

	_ = controller.AddPolicy(&ResidencyPolicy{
		ID:                    "policy-1",
		FeatureNames:          []string{"local_data"},
		Requirement:           RequirementSameRegion,
		AllowCrossRegionWrite: true, // Allow for checking same-region requirement
		Enabled:               true,
	})

	ctx := context.Background()

	// Same region - allowed
	check := controller.CheckWrite(ctx, "local_data", RegionUSEast)
	assert.True(t, check.Allowed)

	// Different region - not allowed (same region requirement fails)
	check = controller.CheckWrite(ctx, "local_data", RegionUSWest)
	assert.False(t, check.Allowed)
	assert.Contains(t, check.Violation, "same region")
}

func TestResidencyController_CheckWrite_SameZoneRequirement(t *testing.T) {
	config := ResidencyConfig{
		Enabled:       true,
		CurrentRegion: RegionUSEast,
		CurrentZone:   ZoneUS,
	}
	controller := NewResidencyController(config, nil)

	_ = controller.AddPolicy(&ResidencyPolicy{
		ID:                    "policy-1",
		FeatureNames:          []string{"zone_data"},
		Requirement:           RequirementSameZone,
		AllowCrossRegionWrite: true, // Allow for checking same-zone requirement
		Enabled:               true,
	})

	ctx := context.Background()

	// Same zone (different region within US) - allowed
	check := controller.CheckWrite(ctx, "zone_data", RegionUSWest)
	assert.True(t, check.Allowed)

	// Different zone - not allowed (same zone requirement fails)
	check = controller.CheckWrite(ctx, "zone_data", RegionEUWest)
	assert.False(t, check.Allowed)
	assert.Contains(t, check.Violation, "same zone")
}

func TestResidencyController_CheckWrite_CrossRegionNotAllowed(t *testing.T) {
	config := ResidencyConfig{
		Enabled:       true,
		CurrentRegion: RegionUSEast,
		CurrentZone:   ZoneUS,
	}
	controller := NewResidencyController(config, nil)

	_ = controller.AddPolicy(&ResidencyPolicy{
		ID:                    "policy-1",
		FeatureNames:          []string{"no_cross_region"},
		AllowCrossRegionWrite: false,
		Enabled:               true,
	})

	ctx := context.Background()

	// Cross-region write - not allowed
	check := controller.CheckWrite(ctx, "no_cross_region", RegionUSWest)
	assert.False(t, check.Allowed)
	assert.Contains(t, check.Violation, "Cross-region")
}

func TestResidencyController_CheckWrite_CrossRegionAllowed(t *testing.T) {
	config := ResidencyConfig{
		Enabled:       true,
		CurrentRegion: RegionUSEast,
		CurrentZone:   ZoneUS,
	}
	controller := NewResidencyController(config, nil)

	_ = controller.AddPolicy(&ResidencyPolicy{
		ID:                    "policy-1",
		FeatureNames:          []string{"cross_region_ok"},
		AllowCrossRegionWrite: true,
		Enabled:               true,
	})

	ctx := context.Background()

	check := controller.CheckWrite(ctx, "cross_region_ok", RegionUSWest)
	assert.True(t, check.Allowed)
}

func TestResidencyController_CheckRead(t *testing.T) {
	config := ResidencyConfig{
		Enabled:       true,
		CurrentRegion: RegionUSEast,
		CurrentZone:   ZoneUS,
	}
	controller := NewResidencyController(config, nil)

	_ = controller.AddPolicy(&ResidencyPolicy{
		ID:                   "policy-1",
		FeatureNames:         []string{"read_data"},
		AllowCrossRegionRead: false,
		Enabled:              true,
	})

	ctx := context.Background()

	// Cross-region read not allowed
	check := controller.CheckRead(ctx, "read_data", RegionEUWest)
	assert.False(t, check.Allowed)
}

func TestResidencyController_CheckExport(t *testing.T) {
	config := ResidencyConfig{
		Enabled:       true,
		CurrentRegion: RegionUSEast,
		CurrentZone:   ZoneUS,
	}
	controller := NewResidencyController(config, nil)

	_ = controller.AddPolicy(&ResidencyPolicy{
		ID:           "policy-1",
		FeatureNames: []string{"export_data"},
		AllowExport:  false,
		Enabled:      true,
	})

	ctx := context.Background()

	// Export not allowed
	check := controller.CheckExport(ctx, "export_data", RegionEUWest)
	assert.False(t, check.Allowed)
	assert.Contains(t, check.Violation, "Export")
}

func TestResidencyController_CheckExport_Allowed(t *testing.T) {
	config := ResidencyConfig{
		Enabled:       true,
		CurrentRegion: RegionUSEast,
		CurrentZone:   ZoneUS,
	}
	controller := NewResidencyController(config, nil)

	_ = controller.AddPolicy(&ResidencyPolicy{
		ID:           "policy-1",
		FeatureNames: []string{"exportable"},
		AllowExport:  true,
		Enabled:      true,
	})

	ctx := context.Background()

	check := controller.CheckExport(ctx, "exportable", RegionEUWest)
	assert.True(t, check.Allowed)
}

func TestResidencyController_CheckWrite_DisabledPolicy(t *testing.T) {
	config := ResidencyConfig{
		Enabled:       true,
		CurrentRegion: RegionUSEast,
	}
	controller := NewResidencyController(config, nil)

	_ = controller.AddPolicy(&ResidencyPolicy{
		ID:           "policy-1",
		FeatureNames: []string{"feature"},
		DenyRegions:  []Region{RegionEUWest},
		Enabled:      false, // Disabled
	})

	ctx := context.Background()

	// Disabled policy should be skipped
	check := controller.CheckWrite(ctx, "feature", RegionEUWest)
	assert.True(t, check.Allowed)
}

func TestResidencyController_CheckBatch(t *testing.T) {
	config := ResidencyConfig{
		Enabled:       true,
		CurrentRegion: RegionUSEast,
		CurrentZone:   ZoneUS,
	}
	controller := NewResidencyController(config, nil)

	_ = controller.AddPolicy(&ResidencyPolicy{
		ID:             "policy-1",
		FeatureNames:   []string{"us_only"},
		AllowedRegions: []Region{RegionUSEast, RegionUSWest},
		Enabled:        true,
	})

	_ = controller.AddPolicy(&ResidencyPolicy{
		ID:           "policy-2",
		FeatureNames: []string{"no_eu"},
		DenyRegions:  []Region{RegionEUWest, RegionEUCentral},
		Enabled:      true,
	})

	features := []string{"us_only", "no_eu", "unrestricted"}
	ctx := context.Background()

	checks := controller.CheckBatch(ctx, features, RegionUSEast, RegionEUWest, "export")

	assert.Len(t, checks, 3)
	assert.False(t, checks["us_only"].Allowed)
	assert.False(t, checks["no_eu"].Allowed)
	assert.True(t, checks["unrestricted"].Allowed)
}

func TestResidencyController_FilterAllowed(t *testing.T) {
	config := ResidencyConfig{
		Enabled:       true,
		CurrentRegion: RegionUSEast,
		CurrentZone:   ZoneUS,
	}
	controller := NewResidencyController(config, nil)

	_ = controller.AddPolicy(&ResidencyPolicy{
		ID:           "policy-1",
		FeatureNames: []string{"restricted"},
		DenyRegions:  []Region{RegionEUWest},
		Enabled:      true,
	})

	features := []string{"restricted", "allowed1", "allowed2"}
	ctx := context.Background()

	allowed := controller.FilterAllowed(ctx, features, RegionUSEast, RegionEUWest, "write")

	assert.Len(t, allowed, 2)
	assert.Contains(t, allowed, "allowed1")
	assert.Contains(t, allowed, "allowed2")
	assert.NotContains(t, allowed, "restricted")
}

func TestResidencyController_Stats(t *testing.T) {
	config := DefaultResidencyConfig()
	controller := NewResidencyController(config, nil)

	_ = controller.AddPolicy(&ResidencyPolicy{
		ID:           "policy-1",
		FeatureNames: []string{"feature"},
		Enabled:      true,
	})

	ctx := context.Background()
	_ = controller.CheckWrite(ctx, "feature", RegionUSEast)

	stats := controller.Stats()
	assert.True(t, stats["enabled"].(bool))
	assert.Equal(t, RegionUSEast, stats["current_region"].(Region))
	assert.Equal(t, 1, stats["policies"].(int))
	assert.GreaterOrEqual(t, stats["checks_performed"].(int64), int64(1))
}

func TestResidencyPolicy_Fields(t *testing.T) {
	policy := &ResidencyPolicy{
		ID:                    "policy-1",
		Name:                  "GDPR Policy",
		Description:           "EU data residency",
		FeaturePattern:        "eu_*",
		FeatureNames:          []string{"eu_data1", "eu_data2"},
		Classification:        ClassificationConfidential,
		Requirement:           RequirementSameZone,
		AllowedRegions:        []Region{RegionEUWest, RegionEUCentral},
		AllowedZones:          []RegionZone{ZoneEU},
		DenyRegions:           []Region{RegionUSEast},
		AllowCrossRegionRead:  false,
		AllowCrossRegionWrite: false,
		AllowExport:           false,
		TenantIDs:             []string{"tenant-1"},
		Enabled:               true,
		Priority:              100,
	}

	assert.Equal(t, "policy-1", policy.ID)
	assert.Equal(t, "GDPR Policy", policy.Name)
	assert.Len(t, policy.AllowedRegions, 2)
	assert.Len(t, policy.AllowedZones, 1)
	assert.Equal(t, ClassificationConfidential, policy.Classification)
}

func TestResidencyCheck_Fields(t *testing.T) {
	check := &ResidencyCheck{
		Allowed:      false,
		FeatureName:  "sensitive",
		SourceRegion: RegionUSEast,
		TargetRegion: RegionEUWest,
		MatchedPolicy: &ResidencyPolicy{
			ID: "policy-1",
		},
		Violation: "Region not allowed",
	}

	assert.False(t, check.Allowed)
	assert.Equal(t, "sensitive", check.FeatureName)
	assert.Equal(t, RegionUSEast, check.SourceRegion)
	assert.NotNil(t, check.MatchedPolicy)
}

func TestGetRegionZone(t *testing.T) {
	tests := []struct {
		region   Region
		expected RegionZone
	}{
		{RegionUSEast, ZoneUS},
		{RegionUSWest, ZoneUS},
		{RegionEUWest, ZoneEU},
		{RegionEUCentral, ZoneEU},
		{RegionAPSoutheast, ZoneAPAC},
		{RegionAPNortheast, ZoneAPAC},
		{RegionSAEast, ZoneLATAM},
		{RegionGlobal, ZoneGlobal},
		{Region("unknown"), ZoneGlobal},
	}

	for _, tt := range tests {
		t.Run(string(tt.region), func(t *testing.T) {
			result := getRegionZone(tt.region)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsGDPRRegion(t *testing.T) {
	assert.True(t, IsGDPRRegion(RegionEUWest))
	assert.True(t, IsGDPRRegion(RegionEUCentral))
	assert.False(t, IsGDPRRegion(RegionUSEast))
	assert.False(t, IsGDPRRegion(RegionAPSoutheast))
}

func TestRegionDisplayName(t *testing.T) {
	assert.Equal(t, "US East (N. Virginia)", RegionDisplayName(RegionUSEast))
	assert.Equal(t, "EU West (Ireland)", RegionDisplayName(RegionEUWest))
	assert.Equal(t, "Asia Pacific (Tokyo)", RegionDisplayName(RegionAPNortheast))
	assert.Equal(t, "Global", RegionDisplayName(RegionGlobal))

	// Unknown region
	assert.Equal(t, "unknown", RegionDisplayName(Region("unknown")))
}
