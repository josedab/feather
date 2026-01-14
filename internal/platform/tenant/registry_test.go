package tenant

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewTenantRegistry(t *testing.T) {
	registry := NewTenantRegistry()
	assert.NotNil(t, registry)
	assert.Empty(t, registry.ListTenants())
}

func TestTenantRegistry_CreateTenant(t *testing.T) {
	registry := NewTenantRegistry()

	tenant := &Tenant{
		ID:          "tenant-1",
		Name:        "Test Tenant",
		Description: "A test tenant",
		Tier:        TierStandard,
	}

	err := registry.CreateTenant(tenant)
	require.NoError(t, err)

	// Verify tenant was created
	retrieved, err := registry.GetTenant("tenant-1")
	require.NoError(t, err)
	assert.Equal(t, "tenant-1", retrieved.ID)
	assert.Equal(t, "Test Tenant", retrieved.Name)
	assert.True(t, retrieved.Enabled)
	assert.NotZero(t, retrieved.CreatedAt)
	assert.NotZero(t, retrieved.UpdatedAt)
}

func TestTenantRegistry_CreateTenant_Duplicate(t *testing.T) {
	registry := NewTenantRegistry()

	tenant := &Tenant{ID: "tenant-1", Name: "Test"}
	err := registry.CreateTenant(tenant)
	require.NoError(t, err)

	// Creating again should fail
	err = registry.CreateTenant(&Tenant{ID: "tenant-1", Name: "Test2"})
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrTenantExists)
}

func TestTenantRegistry_CreateTenant_EmptyID(t *testing.T) {
	registry := NewTenantRegistry()

	tenant := &Tenant{Name: "Test"}
	err := registry.CreateTenant(tenant)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ID is required")
}

func TestTenantRegistry_CreateTenant_DefaultTier(t *testing.T) {
	registry := NewTenantRegistry()

	tenant := &Tenant{ID: "tenant-1", Name: "Test"}
	err := registry.CreateTenant(tenant)
	require.NoError(t, err)

	retrieved, _ := registry.GetTenant("tenant-1")
	assert.Equal(t, TierStandard, retrieved.Tier)
}

func TestTenantRegistry_CreateTenant_DefaultQuotas(t *testing.T) {
	registry := NewTenantRegistry()

	tenant := &Tenant{ID: "tenant-1", Name: "Test", Tier: TierPremium}
	err := registry.CreateTenant(tenant)
	require.NoError(t, err)

	retrieved, _ := registry.GetTenant("tenant-1")
	// Premium tier should have higher quotas
	assert.Greater(t, retrieved.Quotas.MaxFeatures, 1000)
	assert.Greater(t, retrieved.Quotas.MaxRequestsPerSecond, 100)
}

func TestTenantRegistry_GetTenant_NotFound(t *testing.T) {
	registry := NewTenantRegistry()

	_, err := registry.GetTenant("nonexistent")
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrTenantNotFound)
}

func TestTenantRegistry_UpdateTenant(t *testing.T) {
	registry := NewTenantRegistry()

	tenant := &Tenant{ID: "tenant-1", Name: "Original", Tier: TierStandard}
	err := registry.CreateTenant(tenant)
	require.NoError(t, err)

	originalCreatedAt := tenant.CreatedAt

	// Update tenant
	updated := &Tenant{ID: "tenant-1", Name: "Updated", Tier: TierPremium}
	err = registry.UpdateTenant(updated)
	require.NoError(t, err)

	retrieved, _ := registry.GetTenant("tenant-1")
	assert.Equal(t, "Updated", retrieved.Name)
	assert.Equal(t, TierPremium, retrieved.Tier)
	assert.Equal(t, originalCreatedAt, retrieved.CreatedAt)
	assert.True(t, retrieved.UpdatedAt.After(originalCreatedAt))
}

func TestTenantRegistry_UpdateTenant_NotFound(t *testing.T) {
	registry := NewTenantRegistry()

	err := registry.UpdateTenant(&Tenant{ID: "nonexistent"})
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrTenantNotFound)
}

func TestTenantRegistry_DeleteTenant(t *testing.T) {
	registry := NewTenantRegistry()

	tenant := &Tenant{ID: "tenant-1", Name: "Test"}
	err := registry.CreateTenant(tenant)
	require.NoError(t, err)

	err = registry.DeleteTenant("tenant-1")
	require.NoError(t, err)

	_, err = registry.GetTenant("tenant-1")
	assert.Error(t, err)
}

func TestTenantRegistry_DeleteTenant_NotFound(t *testing.T) {
	registry := NewTenantRegistry()

	err := registry.DeleteTenant("nonexistent")
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrTenantNotFound)
}

func TestTenantRegistry_ListTenants(t *testing.T) {
	registry := NewTenantRegistry()

	for i := 0; i < 3; i++ {
		tenant := &Tenant{ID: string(rune('a' + i)), Name: "Test"}
		err := registry.CreateTenant(tenant)
		require.NoError(t, err)
	}

	tenants := registry.ListTenants()
	assert.Len(t, tenants, 3)
}

func TestTenantRegistry_EnableDisableTenant(t *testing.T) {
	registry := NewTenantRegistry()

	tenant := &Tenant{ID: "tenant-1", Name: "Test"}
	err := registry.CreateTenant(tenant)
	require.NoError(t, err)

	// Initially enabled
	retrieved, _ := registry.GetTenant("tenant-1")
	assert.True(t, retrieved.Enabled)

	// Disable
	err = registry.DisableTenant("tenant-1")
	require.NoError(t, err)
	retrieved, _ = registry.GetTenant("tenant-1")
	assert.False(t, retrieved.Enabled)

	// Enable
	err = registry.EnableTenant("tenant-1")
	require.NoError(t, err)
	retrieved, _ = registry.GetTenant("tenant-1")
	assert.True(t, retrieved.Enabled)
}

func TestTenantRegistry_EnableTenant_NotFound(t *testing.T) {
	registry := NewTenantRegistry()

	err := registry.EnableTenant("nonexistent")
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrTenantNotFound)
}

func TestTenantRegistry_DisableTenant_NotFound(t *testing.T) {
	registry := NewTenantRegistry()

	err := registry.DisableTenant("nonexistent")
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrTenantNotFound)
}

func TestTenantRegistry_CheckQuota(t *testing.T) {
	registry := NewTenantRegistry()

	tenant := &Tenant{
		ID:   "tenant-1",
		Name: "Test",
		Quotas: TenantQuotas{
			MaxFeatures:     10,
			MaxStorageBytes: 1000,
		},
	}
	err := registry.CreateTenant(tenant)
	require.NoError(t, err)

	// Within quota
	err = registry.CheckQuota("tenant-1", "features", 5)
	assert.NoError(t, err)

	// Update usage to approach limit
	registry.UpdateUsage("tenant-1", func(u *TenantUsage) {
		u.FeatureCount = 8
	})

	// Exceeds quota
	err = registry.CheckQuota("tenant-1", "features", 5)
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrQuotaExceeded)
}

func TestTenantRegistry_CheckQuota_Storage(t *testing.T) {
	registry := NewTenantRegistry()

	// Note: Must set MaxFeatures != 0 to prevent defaults from overwriting quotas
	tenant := &Tenant{
		ID:   "tenant-1",
		Name: "Test",
		Quotas: TenantQuotas{
			MaxFeatures:     100,
			MaxStorageBytes: 1000,
		},
	}
	err := registry.CreateTenant(tenant)
	require.NoError(t, err)

	// Update storage usage
	registry.UpdateUsage("tenant-1", func(u *TenantUsage) {
		u.StorageBytes = 900
	})

	// Exceeds storage quota (900 + 200 > 1000)
	err = registry.CheckQuota("tenant-1", "storage", 200)
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrStorageQuotaExceeded)
}

func TestTenantRegistry_CheckQuota_TenantDisabled(t *testing.T) {
	registry := NewTenantRegistry()

	tenant := &Tenant{ID: "tenant-1", Name: "Test"}
	err := registry.CreateTenant(tenant)
	require.NoError(t, err)

	registry.DisableTenant("tenant-1")

	err = registry.CheckQuota("tenant-1", "features", 1)
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrTenantDisabled)
}

func TestTenantRegistry_CheckQuota_NotFound(t *testing.T) {
	registry := NewTenantRegistry()

	err := registry.CheckQuota("nonexistent", "features", 1)
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrTenantNotFound)
}

func TestTenantRegistry_CheckRateLimit(t *testing.T) {
	registry := NewTenantRegistry()

	// Note: Must set MaxFeatures != 0 to prevent defaults from overwriting quotas
	tenant := &Tenant{
		ID:   "tenant-1",
		Name: "Test",
		Quotas: TenantQuotas{
			MaxFeatures:          100,
			MaxRequestsPerSecond: 5,
		},
	}
	err := registry.CreateTenant(tenant)
	require.NoError(t, err)

	// First few requests should pass
	for i := 0; i < 5; i++ {
		err = registry.CheckRateLimit("tenant-1")
		require.NoError(t, err)
	}

	// Next request should be rate limited
	err = registry.CheckRateLimit("tenant-1")
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrRateLimitExceeded)
}

func TestTenantRegistry_CheckRateLimit_Burst(t *testing.T) {
	registry := NewTenantRegistry()

	// Note: Must set MaxFeatures != 0 to prevent defaults from overwriting quotas
	tenant := &Tenant{
		ID:   "tenant-1",
		Name: "Test",
		Quotas: TenantQuotas{
			MaxFeatures:          100,
			MaxRequestsPerSecond: 5,
		},
		Settings: TenantSettings{
			AllowBurstTraffic: true,
			BurstMultiplier:   2.0,
		},
	}
	err := registry.CreateTenant(tenant)
	require.NoError(t, err)

	// With burst enabled, should allow 10 requests (5 * 2.0)
	for i := 0; i < 10; i++ {
		err = registry.CheckRateLimit("tenant-1")
		require.NoError(t, err)
	}

	// Should be rate limited after burst
	err = registry.CheckRateLimit("tenant-1")
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrRateLimitExceeded)
}

func TestTenantRegistry_CheckRateLimit_TenantDisabled(t *testing.T) {
	registry := NewTenantRegistry()

	tenant := &Tenant{ID: "tenant-1", Name: "Test"}
	err := registry.CreateTenant(tenant)
	require.NoError(t, err)

	registry.DisableTenant("tenant-1")

	err = registry.CheckRateLimit("tenant-1")
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrTenantDisabled)
}

func TestTenantRegistry_RecordRequest(t *testing.T) {
	registry := NewTenantRegistry()

	tenant := &Tenant{ID: "tenant-1", Name: "Test"}
	err := registry.CreateTenant(tenant)
	require.NoError(t, err)

	// Record successful request
	registry.RecordRequest("tenant-1", 100*time.Millisecond, false)

	// Record error request
	registry.RecordRequest("tenant-1", 50*time.Millisecond, true)

	metrics, _ := registry.GetMetrics("tenant-1")
	assert.Equal(t, int64(2), metrics.RequestCount)
	assert.Equal(t, int64(1), metrics.ErrorCount)
	assert.Greater(t, metrics.TotalLatencyNs, int64(0))
}

func TestTenantRegistry_UpdateUsage(t *testing.T) {
	registry := NewTenantRegistry()

	tenant := &Tenant{ID: "tenant-1", Name: "Test"}
	err := registry.CreateTenant(tenant)
	require.NoError(t, err)

	err = registry.UpdateUsage("tenant-1", func(u *TenantUsage) {
		u.FeatureCount = 50
		u.StorageBytes = 1024
	})
	require.NoError(t, err)

	usage, _ := registry.GetUsage("tenant-1")
	assert.Equal(t, int64(50), usage.FeatureCount)
	assert.Equal(t, int64(1024), usage.StorageBytes)
}

func TestTenantRegistry_UpdateUsage_NotFound(t *testing.T) {
	registry := NewTenantRegistry()

	err := registry.UpdateUsage("nonexistent", func(u *TenantUsage) {})
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrTenantNotFound)
}

func TestTenantRegistry_GetUsage_NotFound(t *testing.T) {
	registry := NewTenantRegistry()

	_, err := registry.GetUsage("nonexistent")
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrTenantNotFound)
}

func TestTenantRegistry_GetMetrics_NotFound(t *testing.T) {
	registry := NewTenantRegistry()

	_, err := registry.GetMetrics("nonexistent")
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrTenantNotFound)
}

func TestTenantRegistry_GetPriority(t *testing.T) {
	registry := NewTenantRegistry()

	tenant := &Tenant{
		ID:   "tenant-1",
		Name: "Test",
		Settings: TenantSettings{
			DefaultPriority: PriorityHigh,
		},
	}
	err := registry.CreateTenant(tenant)
	require.NoError(t, err)

	priority := registry.GetPriority("tenant-1")
	assert.Equal(t, PriorityHigh, priority)
}

func TestTenantRegistry_GetPriority_NotFound(t *testing.T) {
	registry := NewTenantRegistry()

	priority := registry.GetPriority("nonexistent")
	assert.Equal(t, PriorityNormal, priority) // Default priority
}

func TestTenantRegistry_Callbacks(t *testing.T) {
	registry := NewTenantRegistry()

	tenantCreated := false
	quotaExceededCh := make(chan struct{}, 1)

	registry.OnTenantCreated(func(t *Tenant) {
		tenantCreated = true
	})
	registry.OnQuotaExceeded(func(tenantID, quotaType string) {
		select {
		case quotaExceededCh <- struct{}{}:
		default:
		}
	})

	// Create tenant triggers callback (synchronous)
	tenant := &Tenant{ID: "tenant-1", Name: "Test", Quotas: TenantQuotas{MaxFeatures: 5}}
	registry.CreateTenant(tenant)
	assert.True(t, tenantCreated)

	// Exceed quota triggers callback (async)
	registry.UpdateUsage("tenant-1", func(u *TenantUsage) {
		u.FeatureCount = 10
	})
	_ = registry.CheckQuota("tenant-1", "features", 1)

	// Wait for async callback with timeout
	select {
	case <-quotaExceededCh:
		// Success - callback was invoked
	case <-time.After(100 * time.Millisecond):
		t.Fatal("quota exceeded callback was not invoked within timeout")
	}
}

func TestTenantRegistry_Stats(t *testing.T) {
	registry := NewTenantRegistry()

	// Create tenants with different tiers
	registry.CreateTenant(&Tenant{ID: "t1", Name: "T1", Tier: TierFree})
	registry.CreateTenant(&Tenant{ID: "t2", Name: "T2", Tier: TierStandard})
	registry.CreateTenant(&Tenant{ID: "t3", Name: "T3", Tier: TierPremium})

	// Disable one
	registry.DisableTenant("t1")

	stats := registry.Stats()
	assert.Equal(t, 3, stats["total_tenants"])
	assert.Equal(t, 2, stats["enabled_tenants"])

	byTier := stats["by_tier"].(map[string]int)
	assert.Equal(t, 1, byTier["free"])
	assert.Equal(t, 1, byTier["standard"])
	assert.Equal(t, 1, byTier["premium"])
}

func TestDefaultQuotasForTier(t *testing.T) {
	tests := []struct {
		tier             TenantTier
		expectedFeatures int
		expectedQPS      int
	}{
		{TierFree, 100, 10},
		{TierStandard, 1000, 100},
		{TierPremium, 10000, 1000},
		{TierEnterprise, 0, 0}, // Unlimited
	}

	for _, tt := range tests {
		t.Run(string(tt.tier), func(t *testing.T) {
			quotas := DefaultQuotasForTier(tt.tier)
			assert.Equal(t, tt.expectedFeatures, quotas.MaxFeatures)
			assert.Equal(t, tt.expectedQPS, quotas.MaxRequestsPerSecond)
		})
	}
}

func TestContextHelpers(t *testing.T) {
	ctx := context.Background()

	// Add tenant to context
	ctx = WithTenant(ctx, "tenant-123")

	// Retrieve tenant from context
	tenantID := TenantFromContext(ctx)
	assert.Equal(t, "tenant-123", tenantID)
}

func TestContextHelpers_EmptyContext(t *testing.T) {
	ctx := context.Background()

	tenantID := TenantFromContext(ctx)
	assert.Empty(t, tenantID)
}

func TestTenantTierConstants(t *testing.T) {
	assert.Equal(t, TenantTier("free"), TierFree)
	assert.Equal(t, TenantTier("standard"), TierStandard)
	assert.Equal(t, TenantTier("premium"), TierPremium)
	assert.Equal(t, TenantTier("enterprise"), TierEnterprise)
}

func TestPriorityClassConstants(t *testing.T) {
	assert.Equal(t, PriorityClass(0), PriorityLow)
	assert.Equal(t, PriorityClass(50), PriorityNormal)
	assert.Equal(t, PriorityClass(75), PriorityHigh)
	assert.Equal(t, PriorityClass(100), PriorityCritical)
}

func TestIsolationModeConstants(t *testing.T) {
	assert.Equal(t, IsolationMode("shared"), IsolationShared)
	assert.Equal(t, IsolationMode("partitioned"), IsolationPartitioned)
	assert.Equal(t, IsolationMode("dedicated"), IsolationDedicated)
}

func TestTenantRegistry_Sharing(t *testing.T) {
	r := NewTenantRegistry()
	_ = r.CreateTenant(&Tenant{ID: "t1", Name: "Tenant 1", Tier: TierStandard})
	_ = r.CreateTenant(&Tenant{ID: "t2", Name: "Tenant 2", Tier: TierStandard})

	grant := &ShareGrant{
		FromTenantID: "t1",
		ToTenantID:   "t2",
		Features:     []string{"clicks", "purchases"},
		Permission:   "read",
		GrantedBy:    "admin",
	}
	err := r.GrantShare(grant)
	assert.NoError(t, err)
	assert.NotEmpty(t, grant.ID)

	// Check access
	assert.True(t, r.CanAccess("t2", "t1", "clicks", "read"))
	assert.False(t, r.CanAccess("t2", "t1", "clicks", "read_write"))
	assert.False(t, r.CanAccess("t2", "t1", "other_feature", "read"))

	// List shares
	shares := r.ListShares("t1")
	assert.Len(t, shares, 1)

	// Revoke
	err = r.RevokeShare(grant.ID)
	assert.NoError(t, err)
	assert.False(t, r.CanAccess("t2", "t1", "clicks", "read"))
}

func TestTenantRegistry_AuditLog(t *testing.T) {
	r := NewTenantRegistry()
	_ = r.CreateTenant(&Tenant{ID: "t1", Name: "T1", Tier: TierStandard})

	entries := r.GetAuditLog("t1", 10)
	assert.NotEmpty(t, entries)
	assert.Equal(t, "tenant_created", entries[0].Action)
}
