package governance

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultACLConfig(t *testing.T) {
	config := DefaultACLConfig()

	assert.True(t, config.Enabled)
	assert.Equal(t, ACLEffectAllow, config.DefaultEffect)
	assert.True(t, config.EnforceOnRead)
	assert.True(t, config.EnforceOnWrite)
	assert.True(t, config.CacheEnabled)
	assert.Equal(t, 5*time.Minute, config.CacheTTL)
	assert.True(t, config.AuditEnabled)
}

func TestACLPermission_Values(t *testing.T) {
	assert.Equal(t, ACLPermission("read"), ACLPermissionRead)
	assert.Equal(t, ACLPermission("write"), ACLPermissionWrite)
	assert.Equal(t, ACLPermission("delete"), ACLPermissionDelete)
	assert.Equal(t, ACLPermission("admin"), ACLPermissionAdmin)
	assert.Equal(t, ACLPermission("all"), ACLPermissionAll)
}

func TestACLEffect_Values(t *testing.T) {
	assert.Equal(t, ACLEffect("allow"), ACLEffectAllow)
	assert.Equal(t, ACLEffect("deny"), ACLEffectDeny)
}

func TestNewColumnACLController(t *testing.T) {
	config := DefaultACLConfig()
	controller := NewColumnACLController(config, nil)

	require.NotNil(t, controller)
	assert.NotNil(t, controller.acls)
	assert.NotNil(t, controller.byFeature)
	assert.NotNil(t, controller.cache)
}

func TestColumnACLController_AddACL(t *testing.T) {
	config := DefaultACLConfig()
	controller := NewColumnACLController(config, nil)

	acl := &ColumnACL{
		ID:          "acl-1",
		FeatureName: "sensitive_data",
		Effect:      ACLEffectAllow,
		Permissions: []ACLPermission{ACLPermissionRead},
		Principals: []ACLPrincipal{
			{Type: "user", ID: "user-1"},
		},
		Enabled: true,
	}

	err := controller.AddACL(acl)
	require.NoError(t, err)

	// Verify ACL was added
	retrieved, err := controller.GetACL("acl-1")
	require.NoError(t, err)
	assert.Equal(t, "sensitive_data", retrieved.FeatureName)
	assert.NotZero(t, retrieved.CreatedAt)
	assert.NotZero(t, retrieved.UpdatedAt)
}

func TestColumnACLController_AddACL_InvalidID(t *testing.T) {
	config := DefaultACLConfig()
	controller := NewColumnACLController(config, nil)

	acl := &ColumnACL{
		ID: "", // Empty ID
	}

	err := controller.AddACL(acl)
	assert.ErrorIs(t, err, ErrInvalidACLConfig)
}

func TestColumnACLController_AddACL_Duplicate(t *testing.T) {
	config := DefaultACLConfig()
	controller := NewColumnACLController(config, nil)

	acl := &ColumnACL{
		ID:          "acl-1",
		FeatureName: "feature",
		Enabled:     true,
	}

	err := controller.AddACL(acl)
	require.NoError(t, err)

	// Try to add duplicate
	err = controller.AddACL(acl)
	assert.ErrorIs(t, err, ErrACLExists)
}

func TestColumnACLController_UpdateACL(t *testing.T) {
	config := DefaultACLConfig()
	controller := NewColumnACLController(config, nil)

	// Add initial ACL
	acl := &ColumnACL{
		ID:          "acl-1",
		FeatureName: "feature1",
		Effect:      ACLEffectAllow,
		Enabled:     true,
	}
	err := controller.AddACL(acl)
	require.NoError(t, err)

	// Update ACL
	updatedACL := &ColumnACL{
		ID:          "acl-1",
		FeatureName: "feature2",
		Effect:      ACLEffectDeny,
		Enabled:     true,
	}

	err = controller.UpdateACL(updatedACL)
	require.NoError(t, err)

	// Verify update
	retrieved, _ := controller.GetACL("acl-1")
	assert.Equal(t, "feature2", retrieved.FeatureName)
	assert.Equal(t, ACLEffectDeny, retrieved.Effect)
}

func TestColumnACLController_UpdateACL_NotFound(t *testing.T) {
	config := DefaultACLConfig()
	controller := NewColumnACLController(config, nil)

	acl := &ColumnACL{
		ID: "nonexistent",
	}

	err := controller.UpdateACL(acl)
	assert.ErrorIs(t, err, ErrACLNotFound)
}

func TestColumnACLController_DeleteACL(t *testing.T) {
	config := DefaultACLConfig()
	controller := NewColumnACLController(config, nil)

	acl := &ColumnACL{
		ID:          "acl-1",
		FeatureName: "feature",
		Enabled:     true,
	}
	_ = controller.AddACL(acl)

	err := controller.DeleteACL("acl-1")
	require.NoError(t, err)

	_, err = controller.GetACL("acl-1")
	assert.ErrorIs(t, err, ErrACLNotFound)
}

func TestColumnACLController_DeleteACL_NotFound(t *testing.T) {
	config := DefaultACLConfig()
	controller := NewColumnACLController(config, nil)

	err := controller.DeleteACL("nonexistent")
	assert.ErrorIs(t, err, ErrACLNotFound)
}

func TestColumnACLController_GetACL_NotFound(t *testing.T) {
	config := DefaultACLConfig()
	controller := NewColumnACLController(config, nil)

	_, err := controller.GetACL("nonexistent")
	assert.ErrorIs(t, err, ErrACLNotFound)
}

func TestColumnACLController_ListACLs(t *testing.T) {
	config := DefaultACLConfig()
	controller := NewColumnACLController(config, nil)

	_ = controller.AddACL(&ColumnACL{ID: "acl-1", FeatureName: "f1", Enabled: true})
	_ = controller.AddACL(&ColumnACL{ID: "acl-2", FeatureName: "f2", Enabled: true})
	_ = controller.AddACL(&ColumnACL{ID: "acl-3", FeatureName: "f3", Enabled: true})

	acls := controller.ListACLs()
	assert.Len(t, acls, 3)
}

func TestColumnACLController_ListACLsForFeature(t *testing.T) {
	config := DefaultACLConfig()
	controller := NewColumnACLController(config, nil)

	_ = controller.AddACL(&ColumnACL{ID: "acl-1", FeatureName: "feature1", Enabled: true})
	_ = controller.AddACL(&ColumnACL{ID: "acl-2", FeatureName: "feature1", Enabled: true})
	_ = controller.AddACL(&ColumnACL{ID: "acl-3", FeatureName: "feature2", Enabled: true})

	acls := controller.ListACLsForFeature("feature1")
	assert.Len(t, acls, 2)
}

func TestColumnACLController_Evaluate_Disabled(t *testing.T) {
	config := ACLConfig{
		Enabled: false,
	}
	controller := NewColumnACLController(config, nil)

	req := &ACLRequest{
		FeatureName: "feature",
		Permission:  ACLPermissionRead,
	}

	ctx := context.Background()
	decision := controller.Evaluate(ctx, req)

	assert.True(t, decision.Allowed)
	assert.Equal(t, "ACL enforcement disabled", decision.Reason)
}

func TestColumnACLController_Evaluate_AllowMatch(t *testing.T) {
	config := DefaultACLConfig()
	config.CacheEnabled = false
	controller := NewColumnACLController(config, nil)

	_ = controller.AddACL(&ColumnACL{
		ID:          "acl-1",
		FeatureName: "sensitive",
		Effect:      ACLEffectAllow,
		Permissions: []ACLPermission{ACLPermissionRead},
		Principals: []ACLPrincipal{
			{Type: "user", ID: "user-1"},
		},
		Enabled: true,
	})

	req := &ACLRequest{
		FeatureName: "sensitive",
		Permission:  ACLPermissionRead,
		Principal:   ACLPrincipal{Type: "user", ID: "user-1"},
	}

	ctx := context.Background()
	decision := controller.Evaluate(ctx, req)

	assert.True(t, decision.Allowed)
	assert.Equal(t, ACLEffectAllow, decision.Effect)
	assert.NotNil(t, decision.MatchedACL)
}

func TestColumnACLController_Evaluate_DenyMatch(t *testing.T) {
	config := DefaultACLConfig()
	config.CacheEnabled = false
	controller := NewColumnACLController(config, nil)

	_ = controller.AddACL(&ColumnACL{
		ID:          "acl-1",
		FeatureName: "restricted",
		Effect:      ACLEffectDeny,
		Permissions: []ACLPermission{ACLPermissionRead},
		Principals: []ACLPrincipal{
			{Type: "user", ID: "user-1"},
		},
		Enabled: true,
	})

	req := &ACLRequest{
		FeatureName: "restricted",
		Permission:  ACLPermissionRead,
		Principal:   ACLPrincipal{Type: "user", ID: "user-1"},
	}

	ctx := context.Background()
	decision := controller.Evaluate(ctx, req)

	assert.False(t, decision.Allowed)
	assert.Equal(t, ACLEffectDeny, decision.Effect)
}

func TestColumnACLController_Evaluate_NoMatchDefaultAllow(t *testing.T) {
	config := ACLConfig{
		Enabled:       true,
		DefaultEffect: ACLEffectAllow,
		CacheEnabled:  false,
	}
	controller := NewColumnACLController(config, nil)

	req := &ACLRequest{
		FeatureName: "feature",
		Permission:  ACLPermissionRead,
		Principal:   ACLPrincipal{Type: "user", ID: "user-1"},
	}

	ctx := context.Background()
	decision := controller.Evaluate(ctx, req)

	assert.True(t, decision.Allowed)
	assert.Contains(t, decision.Reason, "default effect")
}

func TestColumnACLController_Evaluate_NoMatchDefaultDeny(t *testing.T) {
	config := ACLConfig{
		Enabled:       true,
		DefaultEffect: ACLEffectDeny,
		CacheEnabled:  false,
	}
	controller := NewColumnACLController(config, nil)

	req := &ACLRequest{
		FeatureName: "feature",
		Permission:  ACLPermissionRead,
		Principal:   ACLPrincipal{Type: "user", ID: "user-1"},
	}

	ctx := context.Background()
	decision := controller.Evaluate(ctx, req)

	assert.False(t, decision.Allowed)
}

func TestColumnACLController_Evaluate_Priority(t *testing.T) {
	config := DefaultACLConfig()
	config.CacheEnabled = false
	controller := NewColumnACLController(config, nil)

	// Lower priority - allow
	_ = controller.AddACL(&ColumnACL{
		ID:          "acl-allow",
		FeatureName: "feature",
		Effect:      ACLEffectAllow,
		Permissions: []ACLPermission{ACLPermissionAll},
		Priority:    10,
		Enabled:     true,
	})

	// Higher priority - deny
	_ = controller.AddACL(&ColumnACL{
		ID:          "acl-deny",
		FeatureName: "feature",
		Effect:      ACLEffectDeny,
		Permissions: []ACLPermission{ACLPermissionAll},
		Priority:    100,
		Enabled:     true,
	})

	req := &ACLRequest{
		FeatureName: "feature",
		Permission:  ACLPermissionRead,
	}

	ctx := context.Background()
	decision := controller.Evaluate(ctx, req)

	// Higher priority deny should win
	assert.False(t, decision.Allowed)
	assert.Equal(t, "acl-deny", decision.MatchedACL.ID)
}

func TestColumnACLController_Evaluate_DisabledACL(t *testing.T) {
	config := DefaultACLConfig()
	config.CacheEnabled = false
	config.DefaultEffect = ACLEffectAllow
	controller := NewColumnACLController(config, nil)

	_ = controller.AddACL(&ColumnACL{
		ID:          "acl-1",
		FeatureName: "feature",
		Effect:      ACLEffectDeny,
		Permissions: []ACLPermission{ACLPermissionAll},
		Enabled:     false, // Disabled
	})

	req := &ACLRequest{
		FeatureName: "feature",
		Permission:  ACLPermissionRead,
	}

	ctx := context.Background()
	decision := controller.Evaluate(ctx, req)

	// Disabled ACL should be skipped, default allow
	assert.True(t, decision.Allowed)
}

func TestColumnACLController_Evaluate_WildcardPrincipal(t *testing.T) {
	config := DefaultACLConfig()
	config.CacheEnabled = false
	controller := NewColumnACLController(config, nil)

	_ = controller.AddACL(&ColumnACL{
		ID:          "acl-1",
		FeatureName: "feature",
		Effect:      ACLEffectAllow,
		Permissions: []ACLPermission{ACLPermissionRead},
		Principals: []ACLPrincipal{
			{Type: "user", ID: "*"}, // Wildcard
		},
		Enabled: true,
	})

	req := &ACLRequest{
		FeatureName: "feature",
		Permission:  ACLPermissionRead,
		Principal:   ACLPrincipal{Type: "user", ID: "any-user"},
	}

	ctx := context.Background()
	decision := controller.Evaluate(ctx, req)

	assert.True(t, decision.Allowed)
}

func TestColumnACLController_Evaluate_RoleMatch(t *testing.T) {
	config := DefaultACLConfig()
	config.CacheEnabled = false
	controller := NewColumnACLController(config, nil)

	_ = controller.AddACL(&ColumnACL{
		ID:          "acl-1",
		FeatureName: "feature",
		Effect:      ACLEffectAllow,
		Permissions: []ACLPermission{ACLPermissionRead},
		Principals: []ACLPrincipal{
			{Type: "role", ID: "admin"},
		},
		Enabled: true,
	})

	req := &ACLRequest{
		FeatureName: "feature",
		Permission:  ACLPermissionRead,
		Principal:   ACLPrincipal{Type: "user", ID: "user-1"},
		Context: ACLEvaluationContext{
			Roles: []string{"admin"},
		},
	}

	ctx := context.Background()
	decision := controller.Evaluate(ctx, req)

	assert.True(t, decision.Allowed)
}

func TestColumnACLController_Evaluate_GroupMatch(t *testing.T) {
	config := DefaultACLConfig()
	config.CacheEnabled = false
	controller := NewColumnACLController(config, nil)

	_ = controller.AddACL(&ColumnACL{
		ID:          "acl-1",
		FeatureName: "feature",
		Effect:      ACLEffectAllow,
		Permissions: []ACLPermission{ACLPermissionRead},
		Principals: []ACLPrincipal{
			{Type: "group", ID: "engineering"},
		},
		Enabled: true,
	})

	req := &ACLRequest{
		FeatureName: "feature",
		Permission:  ACLPermissionRead,
		Principal:   ACLPrincipal{Type: "user", ID: "user-1"},
		Context: ACLEvaluationContext{
			Groups: []string{"engineering"},
		},
	}

	ctx := context.Background()
	decision := controller.Evaluate(ctx, req)

	assert.True(t, decision.Allowed)
}

func TestColumnACLController_Evaluate_TenantMatch(t *testing.T) {
	config := DefaultACLConfig()
	config.CacheEnabled = false
	controller := NewColumnACLController(config, nil)

	_ = controller.AddACL(&ColumnACL{
		ID:          "acl-1",
		FeatureName: "feature",
		Effect:      ACLEffectAllow,
		Permissions: []ACLPermission{ACLPermissionRead},
		Principals: []ACLPrincipal{
			{Type: "tenant", ID: "tenant-1"},
		},
		Enabled: true,
	})

	req := &ACLRequest{
		FeatureName: "feature",
		Permission:  ACLPermissionRead,
		Principal:   ACLPrincipal{Type: "user", ID: "user-1"},
		Context: ACLEvaluationContext{
			TenantID: "tenant-1",
		},
	}

	ctx := context.Background()
	decision := controller.Evaluate(ctx, req)

	assert.True(t, decision.Allowed)
}

func TestColumnACLController_Evaluate_PermissionAll(t *testing.T) {
	config := DefaultACLConfig()
	config.CacheEnabled = false
	controller := NewColumnACLController(config, nil)

	_ = controller.AddACL(&ColumnACL{
		ID:          "acl-1",
		FeatureName: "feature",
		Effect:      ACLEffectAllow,
		Permissions: []ACLPermission{ACLPermissionAll},
		Enabled:     true,
	})

	ctx := context.Background()

	// Should match any permission
	for _, perm := range []ACLPermission{ACLPermissionRead, ACLPermissionWrite, ACLPermissionDelete} {
		req := &ACLRequest{
			FeatureName: "feature",
			Permission:  perm,
		}
		decision := controller.Evaluate(ctx, req)
		assert.True(t, decision.Allowed)
	}
}

func TestColumnACLController_Evaluate_TimeCondition(t *testing.T) {
	config := DefaultACLConfig()
	config.CacheEnabled = false
	config.DefaultEffect = ACLEffectDeny // Default deny when no ACL matches
	controller := NewColumnACLController(config, nil)

	_ = controller.AddACL(&ColumnACL{
		ID:          "acl-1",
		FeatureName: "feature",
		Effect:      ACLEffectAllow,
		Permissions: []ACLPermission{ACLPermissionRead},
		Conditions: []ACLCondition{
			{
				Type: "time",
				Value: map[string]interface{}{
					"start_hour": float64(9),
					"end_hour":   float64(17),
				},
			},
		},
		Enabled: true,
	})

	ctx := context.Background()

	// Within allowed hours
	req := &ACLRequest{
		FeatureName: "feature",
		Permission:  ACLPermissionRead,
		Context: ACLEvaluationContext{
			Time: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
		},
	}
	decision := controller.Evaluate(ctx, req)
	assert.True(t, decision.Allowed)

	// Outside allowed hours
	req.Context.Time = time.Date(2024, 1, 1, 20, 0, 0, 0, time.UTC)
	decision = controller.Evaluate(ctx, req)
	assert.False(t, decision.Allowed)
}

func TestColumnACLController_Evaluate_IPCondition(t *testing.T) {
	config := DefaultACLConfig()
	config.CacheEnabled = false
	config.DefaultEffect = ACLEffectDeny // Default deny when no ACL matches
	controller := NewColumnACLController(config, nil)

	_ = controller.AddACL(&ColumnACL{
		ID:          "acl-1",
		FeatureName: "feature",
		Effect:      ACLEffectAllow,
		Permissions: []ACLPermission{ACLPermissionRead},
		Conditions: []ACLCondition{
			{
				Type:     "ip",
				Operator: "in",
				Value:    []interface{}{"192.168.1.1", "192.168.1.2"},
			},
		},
		Enabled: true,
	})

	ctx := context.Background()

	// Allowed IP
	req := &ACLRequest{
		FeatureName: "feature",
		Permission:  ACLPermissionRead,
		Context: ACLEvaluationContext{
			SourceIP: "192.168.1.1",
		},
	}
	decision := controller.Evaluate(ctx, req)
	assert.True(t, decision.Allowed)

	// Not allowed IP
	req.Context.SourceIP = "10.0.0.1"
	decision = controller.Evaluate(ctx, req)
	assert.False(t, decision.Allowed)
}

func TestColumnACLController_Evaluate_PurposeCondition(t *testing.T) {
	config := DefaultACLConfig()
	config.CacheEnabled = false
	config.DefaultEffect = ACLEffectDeny // Default deny when no ACL matches
	controller := NewColumnACLController(config, nil)

	_ = controller.AddACL(&ColumnACL{
		ID:          "acl-1",
		FeatureName: "feature",
		Effect:      ACLEffectAllow,
		Permissions: []ACLPermission{ACLPermissionRead},
		Conditions: []ACLCondition{
			{
				Type:     "purpose",
				Operator: "equals",
				Value:    "analytics",
			},
		},
		Enabled: true,
	})

	ctx := context.Background()

	// Matching purpose
	req := &ACLRequest{
		FeatureName: "feature",
		Permission:  ACLPermissionRead,
		Context: ACLEvaluationContext{
			Purpose: "analytics",
		},
	}
	decision := controller.Evaluate(ctx, req)
	assert.True(t, decision.Allowed)

	// Non-matching purpose
	req.Context.Purpose = "marketing"
	decision = controller.Evaluate(ctx, req)
	assert.False(t, decision.Allowed)
}

func TestColumnACLController_Evaluate_SensitivityCondition(t *testing.T) {
	config := DefaultACLConfig()
	config.CacheEnabled = false
	config.DefaultEffect = ACLEffectDeny // Default deny when no ACL matches
	controller := NewColumnACLController(config, nil)

	_ = controller.AddACL(&ColumnACL{
		ID:          "acl-1",
		FeatureName: "feature",
		Effect:      ACLEffectAllow,
		Permissions: []ACLPermission{ACLPermissionRead},
		Conditions: []ACLCondition{
			{
				Type:     "sensitivity",
				Operator: "lt",
				Value:    "high",
			},
		},
		Enabled: true,
	})

	ctx := context.Background()

	// Low sensitivity (allowed)
	req := &ACLRequest{
		FeatureName: "feature",
		Permission:  ACLPermissionRead,
		Context: ACLEvaluationContext{
			Sensitivity: SensitivityLow,
		},
	}
	decision := controller.Evaluate(ctx, req)
	assert.True(t, decision.Allowed)

	// High sensitivity (not allowed)
	req.Context.Sensitivity = SensitivityHigh
	decision = controller.Evaluate(ctx, req)
	assert.False(t, decision.Allowed)
}

func TestColumnACLController_Evaluate_Cache(t *testing.T) {
	config := ACLConfig{
		Enabled:      true,
		CacheEnabled: true,
		CacheTTL:     5 * time.Minute,
	}
	controller := NewColumnACLController(config, nil)

	_ = controller.AddACL(&ColumnACL{
		ID:          "acl-1",
		FeatureName: "feature",
		Effect:      ACLEffectAllow,
		Permissions: []ACLPermission{ACLPermissionRead},
		Enabled:     true,
	})

	req := &ACLRequest{
		FeatureName: "feature",
		Permission:  ACLPermissionRead,
		Principal:   ACLPrincipal{Type: "user", ID: "user-1"},
	}

	ctx := context.Background()

	// First evaluation - cache miss
	decision1 := controller.Evaluate(ctx, req)
	assert.True(t, decision1.Allowed)

	// Second evaluation - cache hit
	decision2 := controller.Evaluate(ctx, req)
	assert.True(t, decision2.Allowed)

	stats := controller.Stats()
	assert.Equal(t, int64(1), stats["cache_hits"].(int64))
}

func TestColumnACLController_EvaluateBatch(t *testing.T) {
	config := DefaultACLConfig()
	config.CacheEnabled = false
	controller := NewColumnACLController(config, nil)

	_ = controller.AddACL(&ColumnACL{
		ID:          "acl-1",
		FeatureName: "feature1",
		Effect:      ACLEffectAllow,
		Permissions: []ACLPermission{ACLPermissionRead},
		Enabled:     true,
	})

	_ = controller.AddACL(&ColumnACL{
		ID:          "acl-2",
		FeatureName: "feature2",
		Effect:      ACLEffectDeny,
		Permissions: []ACLPermission{ACLPermissionRead},
		Enabled:     true,
	})

	features := []string{"feature1", "feature2", "feature3"}
	evalCtx := ACLEvaluationContext{
		UserID: "user-1",
	}

	ctx := context.Background()
	decisions := controller.EvaluateBatch(ctx, features, ACLPermissionRead, evalCtx)

	assert.Len(t, decisions, 3)
	assert.True(t, decisions["feature1"].Allowed)
	assert.False(t, decisions["feature2"].Allowed)
	// feature3 has no ACL, uses default (allow)
}

func TestColumnACLController_FilterAllowed(t *testing.T) {
	config := ACLConfig{
		Enabled:       true,
		DefaultEffect: ACLEffectDeny,
		CacheEnabled:  false,
	}
	controller := NewColumnACLController(config, nil)

	_ = controller.AddACL(&ColumnACL{
		ID:          "acl-1",
		FeatureName: "feature1",
		Effect:      ACLEffectAllow,
		Permissions: []ACLPermission{ACLPermissionRead},
		Enabled:     true,
	})

	_ = controller.AddACL(&ColumnACL{
		ID:          "acl-2",
		FeatureName: "feature2",
		Effect:      ACLEffectAllow,
		Permissions: []ACLPermission{ACLPermissionRead},
		Enabled:     true,
	})

	features := []string{"feature1", "feature2", "feature3"}
	evalCtx := ACLEvaluationContext{
		UserID: "user-1",
	}

	ctx := context.Background()
	allowed := controller.FilterAllowed(ctx, features, ACLPermissionRead, evalCtx)

	assert.Len(t, allowed, 2)
	assert.Contains(t, allowed, "feature1")
	assert.Contains(t, allowed, "feature2")
}

func TestColumnACLController_Stats(t *testing.T) {
	config := DefaultACLConfig()
	controller := NewColumnACLController(config, nil)

	_ = controller.AddACL(&ColumnACL{
		ID:          "acl-1",
		FeatureName: "feature",
		Effect:      ACLEffectAllow,
		Enabled:     true,
	})

	ctx := context.Background()
	req := &ACLRequest{
		FeatureName: "feature",
		Permission:  ACLPermissionRead,
	}
	_ = controller.Evaluate(ctx, req)

	stats := controller.Stats()
	assert.True(t, stats["enabled"].(bool))
	assert.Equal(t, 1, stats["acls"].(int))
	assert.GreaterOrEqual(t, stats["evaluations"].(int64), int64(1))
}

func TestColumnACL_Fields(t *testing.T) {
	acl := &ColumnACL{
		ID:          "acl-1",
		Permissions: []ACLPermission{ACLPermissionRead, ACLPermissionWrite},
		Principals: []ACLPrincipal{
			{Type: "user", ID: "user-1"},
			{Type: "role", ID: "admin"},
		},
		Conditions: []ACLCondition{
			{Type: "time", Operator: "in", Value: "business_hours"},
		},
	}

	assert.Equal(t, "acl-1", acl.ID)
	assert.Len(t, acl.Permissions, 2)
	assert.Len(t, acl.Principals, 2)
	assert.Len(t, acl.Conditions, 1)
}

func TestACLRequest_Fields(t *testing.T) {
	req := &ACLRequest{
		FeatureName: "feature",
		Permission:  ACLPermissionRead,
		Principal: ACLPrincipal{
			Type: "user",
			ID:   "user-1",
		},
		Context: ACLEvaluationContext{
			UserID:      "user-1",
			TenantID:    "tenant-1",
			Roles:       []string{"admin"},
			Groups:      []string{"engineering"},
			SourceIP:    "192.168.1.1",
			Time:        time.Now(),
			Purpose:     "analytics",
			Sensitivity: SensitivityMedium,
			Metadata:    map[string]interface{}{"key": "value"},
		},
	}

	assert.Equal(t, "feature", req.FeatureName)
	assert.Equal(t, ACLPermissionRead, req.Permission)
	assert.Equal(t, "user", req.Principal.Type)
}

func TestACLDecision_Fields(t *testing.T) {
	decision := &ACLDecision{
		Allowed: true,
		Effect:  ACLEffectAllow,
		MatchedACL: &ColumnACL{
			ID: "acl-1",
		},
	}

	assert.True(t, decision.Allowed)
	assert.Equal(t, ACLEffectAllow, decision.Effect)
	assert.NotNil(t, decision.MatchedACL)
}
