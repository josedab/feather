package tenant

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupMiddlewareTest(t *testing.T) (*TenantRegistry, *Middleware) {
	t.Helper()
	registry := NewTenantRegistry()
	middleware := NewTenantMiddleware(registry)

	// Create a test tenant
	tenant := &Tenant{
		ID:   "test-tenant",
		Name: "Test Tenant",
		Quotas: TenantQuotas{
			MaxRequestsPerSecond:  100,
			MaxConcurrentRequests: 10,
			MaxStorageBytes:       1024 * 1024,
		},
	}
	err := registry.CreateTenant(tenant)
	require.NoError(t, err)

	return registry, middleware
}

func TestNewTenantMiddleware(t *testing.T) {
	registry := NewTenantRegistry()
	middleware := NewTenantMiddleware(registry)
	assert.NotNil(t, middleware)
}

func TestTenantMiddleware_ExtractTenant_Header(t *testing.T) {
	_, middleware := setupMiddlewareTest(t)

	handler := middleware.ExtractTenant(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tenantID := TenantFromContext(r.Context())
		w.Write([]byte(tenantID))
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Tenant-ID", "test-tenant")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, "test-tenant", rr.Body.String())
}

func TestTenantMiddleware_ExtractTenant_Query(t *testing.T) {
	_, middleware := setupMiddlewareTest(t)

	handler := middleware.ExtractTenant(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tenantID := TenantFromContext(r.Context())
		w.Write([]byte(tenantID))
	}))

	req := httptest.NewRequest("GET", "/test?tenant_id=query-tenant", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, "query-tenant", rr.Body.String())
}

func TestTenantMiddleware_ExtractTenant_Default(t *testing.T) {
	_, middleware := setupMiddlewareTest(t)

	handler := middleware.ExtractTenant(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tenantID := TenantFromContext(r.Context())
		w.Write([]byte(tenantID))
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, "default", rr.Body.String())
}

func TestTenantMiddleware_EnforceTenant_Valid(t *testing.T) {
	_, middleware := setupMiddlewareTest(t)

	handler := middleware.EnforceTenant(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	ctx := WithTenant(req.Context(), "test-tenant")
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestTenantMiddleware_EnforceTenant_Missing(t *testing.T) {
	_, middleware := setupMiddlewareTest(t)

	handler := middleware.EnforceTenant(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestTenantMiddleware_EnforceTenant_Invalid(t *testing.T) {
	_, middleware := setupMiddlewareTest(t)

	handler := middleware.EnforceTenant(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	ctx := WithTenant(req.Context(), "nonexistent")
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestTenantMiddleware_EnforceTenant_Disabled(t *testing.T) {
	registry, middleware := setupMiddlewareTest(t)
	registry.DisableTenant("test-tenant")

	handler := middleware.EnforceTenant(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	ctx := WithTenant(req.Context(), "test-tenant")
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusForbidden, rr.Code)
}

func TestTenantMiddleware_EnforceRateLimit_Allowed(t *testing.T) {
	_, middleware := setupMiddlewareTest(t)

	handler := middleware.EnforceRateLimit(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	ctx := WithTenant(req.Context(), "test-tenant")
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestTenantMiddleware_EnforceRateLimit_Exceeded(t *testing.T) {
	registry := NewTenantRegistry()
	middleware := NewTenantMiddleware(registry)

	// Create tenant with very low rate limit
	// Note: Must set MaxFeatures != 0 to prevent defaults from overwriting quotas
	tenant := &Tenant{
		ID:   "limited-tenant",
		Name: "Limited Tenant",
		Quotas: TenantQuotas{
			MaxFeatures:          100,
			MaxRequestsPerSecond: 2,
		},
	}
	registry.CreateTenant(tenant)

	handler := middleware.EnforceRateLimit(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Exhaust rate limit
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		ctx := WithTenant(req.Context(), "limited-tenant")
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		if i < 2 {
			assert.Equal(t, http.StatusOK, rr.Code)
		} else {
			assert.Equal(t, http.StatusTooManyRequests, rr.Code)
			assert.Equal(t, "1", rr.Header().Get("Retry-After"))
		}
	}
}

func TestTenantMiddleware_EnforceRateLimit_NoTenant(t *testing.T) {
	_, middleware := setupMiddlewareTest(t)

	handler := middleware.EnforceRateLimit(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	// No tenant in context
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	// Should allow request without tenant
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestTenantMiddleware_EnforceQuota(t *testing.T) {
	_, middleware := setupMiddlewareTest(t)

	handler := middleware.EnforceQuota("storage", 100)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("POST", "/test", nil)
	ctx := WithTenant(req.Context(), "test-tenant")
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestTenantMiddleware_EnforceQuota_Exceeded(t *testing.T) {
	registry := NewTenantRegistry()
	middleware := NewTenantMiddleware(registry)

	// Create tenant with specific storage quota
	// Note: Must set MaxFeatures != 0 to prevent defaults from overwriting quotas
	tenant := &Tenant{
		ID:   "quota-tenant",
		Name: "Quota Tenant",
		Quotas: TenantQuotas{
			MaxFeatures:     100,
			MaxStorageBytes: 1000, // 1KB limit for testing
		},
	}
	registry.CreateTenant(tenant)

	// Set storage near limit
	registry.UpdateUsage("quota-tenant", func(u *TenantUsage) {
		u.StorageBytes = 950 // Near the 1000 byte limit
	})

	handler := middleware.EnforceQuota("storage", 100)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("POST", "/test", nil)
	ctx := WithTenant(req.Context(), "quota-tenant")
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	// 950 + 100 > 1000, should fail
	assert.Equal(t, http.StatusTooManyRequests, rr.Code)
}

func TestTenantMiddleware_RecordMetrics(t *testing.T) {
	registry, middleware := setupMiddlewareTest(t)

	handler := middleware.RecordMetrics(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(10 * time.Millisecond) // Add some latency
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	ctx := WithTenant(req.Context(), "test-tenant")
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	metrics, _ := registry.GetMetrics("test-tenant")
	assert.Equal(t, int64(1), metrics.RequestCount)
	assert.Greater(t, metrics.TotalLatencyNs, int64(0))
}

func TestTenantMiddleware_RecordMetrics_Error(t *testing.T) {
	registry, middleware := setupMiddlewareTest(t)

	handler := middleware.RecordMetrics(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	ctx := WithTenant(req.Context(), "test-tenant")
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	metrics, _ := registry.GetMetrics("test-tenant")
	assert.Equal(t, int64(1), metrics.RequestCount)
	assert.Equal(t, int64(1), metrics.ErrorCount)
}

func TestNewConcurrencyLimiter(t *testing.T) {
	registry := NewTenantRegistry()
	limiter := NewConcurrencyLimiter(registry)
	assert.NotNil(t, limiter)
}

func TestConcurrencyLimiter_Limit_Allowed(t *testing.T) {
	registry, _ := setupMiddlewareTest(t)
	limiter := NewConcurrencyLimiter(registry)

	handler := limiter.Limit(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	ctx := WithTenant(req.Context(), "test-tenant")
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestConcurrencyLimiter_NoTenant(t *testing.T) {
	registry := NewTenantRegistry()
	limiter := NewConcurrencyLimiter(registry)

	handler := limiter.Limit(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	// No tenant
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestStatusCapture_WriteHeader(t *testing.T) {
	rr := httptest.NewRecorder()
	capture := &statusCapture{ResponseWriter: rr, status: http.StatusOK}

	capture.WriteHeader(http.StatusCreated)

	assert.Equal(t, http.StatusCreated, capture.status)
	assert.Equal(t, http.StatusCreated, rr.Code)
}

func TestNewTenantAwareStore(t *testing.T) {
	registry := NewTenantRegistry()
	store := NewTenantAwareStore(registry)
	assert.NotNil(t, store)
}

func TestTenantAwareStore_PrefixEntityKey(t *testing.T) {
	registry := NewTenantRegistry()
	store := NewTenantAwareStore(registry)

	// With tenant
	ctx := WithTenant(context.Background(), "tenant-123")
	key := store.PrefixEntityKey(ctx, "entity-456")
	assert.Equal(t, "tenant-123:entity-456", key)

	// Without tenant
	ctx = context.Background()
	key = store.PrefixEntityKey(ctx, "entity-456")
	assert.Equal(t, "entity-456", key)

	// Default tenant
	ctx = WithTenant(context.Background(), "default")
	key = store.PrefixEntityKey(ctx, "entity-456")
	assert.Equal(t, "entity-456", key)
}

func TestTenantAwareStore_StripTenantPrefix(t *testing.T) {
	registry := NewTenantRegistry()
	store := NewTenantAwareStore(registry)

	// With tenant
	ctx := WithTenant(context.Background(), "tenant-123")
	key := store.StripTenantPrefix(ctx, "tenant-123:entity-456")
	assert.Equal(t, "entity-456", key)

	// Without tenant
	ctx = context.Background()
	key = store.StripTenantPrefix(ctx, "entity-456")
	assert.Equal(t, "entity-456", key)

	// Key without prefix
	ctx = WithTenant(context.Background(), "tenant-123")
	key = store.StripTenantPrefix(ctx, "other-prefix:entity-456")
	assert.Equal(t, "other-prefix:entity-456", key)
}

func TestTenantAwareStore_CheckWriteAccess(t *testing.T) {
	registry := NewTenantRegistry()
	store := NewTenantAwareStore(registry)

	// Note: Must set MaxFeatures != 0 to prevent defaults from overwriting quotas
	tenant := &Tenant{
		ID:   "tenant-1",
		Name: "Test",
		Quotas: TenantQuotas{
			MaxFeatures:     100,
			MaxStorageBytes: 1000,
		},
	}
	registry.CreateTenant(tenant)

	// Within quota
	ctx := WithTenant(context.Background(), "tenant-1")
	err := store.CheckWriteAccess(ctx, 500)
	assert.NoError(t, err)

	// Update to near limit
	registry.UpdateUsage("tenant-1", func(u *TenantUsage) {
		u.StorageBytes = 900
	})

	// Exceeds quota (900 + 200 > 1000)
	err = store.CheckWriteAccess(ctx, 200)
	assert.Error(t, err)
}

func TestTenantAwareStore_CheckWriteAccess_NoTenant(t *testing.T) {
	registry := NewTenantRegistry()
	store := NewTenantAwareStore(registry)

	// Without tenant should allow
	ctx := context.Background()
	err := store.CheckWriteAccess(ctx, 1000000)
	assert.NoError(t, err)
}

func TestTenantAwareStore_RecordStorageUsage(t *testing.T) {
	registry := NewTenantRegistry()
	store := NewTenantAwareStore(registry)

	tenant := &Tenant{ID: "tenant-1", Name: "Test"}
	registry.CreateTenant(tenant)

	ctx := WithTenant(context.Background(), "tenant-1")
	store.RecordStorageUsage(ctx, 1024)

	usage, _ := registry.GetUsage("tenant-1")
	assert.Equal(t, int64(1024), usage.StorageBytes)
}

func TestTenantAwareStore_RecordEntityUsage(t *testing.T) {
	registry := NewTenantRegistry()
	store := NewTenantAwareStore(registry)

	tenant := &Tenant{ID: "tenant-1", Name: "Test"}
	registry.CreateTenant(tenant)

	ctx := WithTenant(context.Background(), "tenant-1")
	store.RecordEntityUsage(ctx, 5)

	usage, _ := registry.GetUsage("tenant-1")
	assert.Equal(t, int64(5), usage.EntityCount)
}

func TestNewPriorityQueue(t *testing.T) {
	registry := NewTenantRegistry()
	pq := NewPriorityQueue(registry, 5)
	assert.NotNil(t, pq)
}

func TestPriorityQueue_StartStop(t *testing.T) {
	registry := NewTenantRegistry()
	pq := NewPriorityQueue(registry, 2)

	pq.Start()
	// Starting again should be no-op
	pq.Start()

	pq.Stop()
	// Stopping again should be no-op
	pq.Stop()
}

func TestPriorityQueue_Middleware(t *testing.T) {
	registry := NewTenantRegistry()
	pq := NewPriorityQueue(registry, 2)
	pq.Start()
	defer pq.Stop()

	tenant := &Tenant{
		ID:   "tenant-1",
		Name: "Test",
		Settings: TenantSettings{
			DefaultPriority: PriorityHigh,
		},
	}
	registry.CreateTenant(tenant)

	handler := pq.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	ctx := WithTenant(req.Context(), "tenant-1")
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}
