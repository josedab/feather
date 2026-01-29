// Package tenant provides multi-tenant isolation for Feather feature store.
// It implements per-tenant resource quotas, workload prioritization, and SLO tracking.
package tenant

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// Tenant-specific errors.
var (
	ErrTenantNotFound       = errors.New("tenant not found")
	ErrTenantExists         = errors.New("tenant already exists")
	ErrTenantDisabled       = errors.New("tenant is disabled")
	ErrQuotaExceeded        = errors.New("quota exceeded")
	ErrRateLimitExceeded    = errors.New("rate limit exceeded")
	ErrStorageQuotaExceeded = errors.New("storage quota exceeded")
)

//revive:disable:exported

// TenantTier represents the service tier for a tenant.
type TenantTier string

const (
	// TierFree is the free service tier.
	TierFree TenantTier = "free"
	// TierStandard is the standard service tier.
	TierStandard TenantTier = "standard"
	// TierPremium is the premium service tier.
	TierPremium TenantTier = "premium"
	// TierEnterprise is the enterprise service tier.
	TierEnterprise TenantTier = "enterprise"
)

// PriorityClass represents request priority levels.
type PriorityClass int

const (
	// PriorityLow is the lowest priority class.
	PriorityLow PriorityClass = 0
	// PriorityNormal is the default priority class.
	PriorityNormal PriorityClass = 50
	// PriorityHigh is a high priority class.
	PriorityHigh PriorityClass = 75
	// PriorityCritical is the highest priority class.
	PriorityCritical PriorityClass = 100
)

// Tenant represents a tenant in the multi-tenant system.
type Tenant struct {
	// ID is the unique tenant identifier
	ID string `json:"id"`
	// Name is the human-readable tenant name
	Name string `json:"name"`
	// Description provides context about the tenant
	Description string `json:"description"`
	// Tier is the service tier
	Tier TenantTier `json:"tier"`
	// Enabled indicates if the tenant is active
	Enabled bool `json:"enabled"`
	// Quotas defines resource limits
	Quotas TenantQuotas `json:"quotas"`
	// Settings contains tenant-specific settings
	Settings TenantSettings `json:"settings"`
	// Metadata for custom key-value pairs
	Metadata map[string]string `json:"metadata"`
	// CreatedAt is the tenant creation timestamp
	CreatedAt time.Time `json:"created_at"`
	// UpdatedAt is the last modification timestamp
	UpdatedAt time.Time `json:"updated_at"`
}

// TenantQuotas defines resource limits for a tenant.
type TenantQuotas struct {
	// MaxFeatures is the maximum number of features
	MaxFeatures int `json:"max_features"`
	// MaxNamespaces is the maximum number of namespaces
	MaxNamespaces int `json:"max_namespaces"`
	// MaxAPIKeys is the maximum number of API keys
	MaxAPIKeys int `json:"max_api_keys"`
	// MaxRequestsPerSecond is the rate limit (QPS)
	MaxRequestsPerSecond int `json:"max_requests_per_second"`
	// MaxRequestsPerMinute is the rate limit per minute
	MaxRequestsPerMinute int `json:"max_requests_per_minute"`
	// MaxStorageBytes is the storage quota
	MaxStorageBytes int64 `json:"max_storage_bytes"`
	// MaxHotTierBytes is the hot tier memory quota
	MaxHotTierBytes int64 `json:"max_hot_tier_bytes"`
	// MaxEntityCount is the maximum number of entities
	MaxEntityCount int64 `json:"max_entity_count"`
	// MaxBatchSize is the maximum batch request size
	MaxBatchSize int `json:"max_batch_size"`
	// MaxConcurrentRequests is the max concurrent requests
	MaxConcurrentRequests int `json:"max_concurrent_requests"`
}

// TenantSettings contains tenant-specific configuration.
type TenantSettings struct {
	// DefaultPriority is the default request priority
	DefaultPriority PriorityClass `json:"default_priority"`
	// AllowBurstTraffic enables burst traffic handling
	AllowBurstTraffic bool `json:"allow_burst_traffic"`
	// BurstMultiplier is the burst rate multiplier
	BurstMultiplier float64 `json:"burst_multiplier"`
	// DefaultTTL is the default feature TTL
	DefaultTTL time.Duration `json:"default_ttl"`
	// EnableAuditLogging enables audit logging
	EnableAuditLogging bool `json:"enable_audit_logging"`
	// EnableMetrics enables metrics collection
	EnableMetrics bool `json:"enable_metrics"`
	// AllowedNamespaces restricts accessible namespaces
	AllowedNamespaces []string `json:"allowed_namespaces,omitempty"`
	// DataIsolationMode determines isolation level
	DataIsolationMode IsolationMode `json:"data_isolation_mode"`
}

// IsolationMode determines how tenant data is isolated.
type IsolationMode string

const (
	// IsolationShared uses shared storage with tenant key prefixing.
	IsolationShared IsolationMode = "shared"
	// IsolationPartitioned uses a partitioned hot tier.
	IsolationPartitioned IsolationMode = "partitioned"
	// IsolationDedicated uses a dedicated storage instance.
	IsolationDedicated IsolationMode = "dedicated"
)

// TenantUsage tracks current resource usage for a tenant.
type TenantUsage struct {
	// TenantID is the tenant identifier
	TenantID string `json:"tenant_id"`
	// FeatureCount is the current feature count
	FeatureCount int64 `json:"feature_count"`
	// NamespaceCount is the current namespace count
	NamespaceCount int64 `json:"namespace_count"`
	// APIKeyCount is the current API key count
	APIKeyCount int64 `json:"api_key_count"`
	// StorageBytes is the current storage usage
	StorageBytes int64 `json:"storage_bytes"`
	// HotTierBytes is the current hot tier usage
	HotTierBytes int64 `json:"hot_tier_bytes"`
	// EntityCount is the current entity count
	EntityCount int64 `json:"entity_count"`
	// RequestCount is the request count (rolling window)
	RequestCount int64 `json:"request_count"`
	// ConcurrentRequests is the current concurrent requests
	ConcurrentRequests int64 `json:"concurrent_requests"`
	// LastUpdated is when usage was last calculated
	LastUpdated time.Time `json:"last_updated"`
}

// TenantMetrics tracks SLO and performance metrics for a tenant.
type TenantMetrics struct {
	// TenantID is the tenant identifier
	TenantID string
	// RequestCount is the total request count
	RequestCount int64
	// ErrorCount is the total error count
	ErrorCount int64
	// TotalLatencyNs is the cumulative latency
	TotalLatencyNs int64
	// P50LatencyNs is the 50th percentile latency
	P50LatencyNs int64
	// P99LatencyNs is the 99th percentile latency
	P99LatencyNs int64
	// QuotaExceededCount is the quota exceeded count
	QuotaExceededCount int64
	// RateLimitedCount is the rate limited count
	RateLimitedCount int64
	// LastReset is when metrics were last reset
	LastReset time.Time
}

// TenantRegistry manages tenants and their quotas.
type TenantRegistry struct {
	mu sync.RWMutex

	// tenants maps tenant ID to tenant
	tenants map[string]*Tenant
	// usage maps tenant ID to usage
	usage map[string]*TenantUsage
	// metrics maps tenant ID to metrics
	metrics map[string]*TenantMetrics
	// rateLimiters maps tenant ID to rate limiter
	rateLimiters map[string]*rateLimiter

	// Callbacks
	onTenantCreated []func(*Tenant)
	onQuotaExceeded []func(string, string)

	// Cross-tenant sharing
	shares   []*ShareGrant
	auditLog []AuditEntry
}

// rateLimiter implements a sliding window rate limiter.
type rateLimiter struct {
	mu           sync.Mutex
	windowStart  time.Time
	windowCount  int64
	maxPerSecond int
	maxPerMinute int
}

// NewTenantRegistry creates a new tenant registry.
func NewTenantRegistry() *TenantRegistry {
	return &TenantRegistry{
		tenants:      make(map[string]*Tenant),
		usage:        make(map[string]*TenantUsage),
		metrics:      make(map[string]*TenantMetrics),
		rateLimiters: make(map[string]*rateLimiter),
		shares:       make([]*ShareGrant, 0),
		auditLog:     make([]AuditEntry, 0),
	}
}

// CreateTenant creates a new tenant.
func (r *TenantRegistry) CreateTenant(tenant *Tenant) error {
	if tenant.ID == "" {
		return errors.New("tenant ID is required")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.tenants[tenant.ID]; exists {
		return fmt.Errorf("%w: %s", ErrTenantExists, tenant.ID)
	}

	now := time.Now()
	tenant.CreatedAt = now
	tenant.UpdatedAt = now
	tenant.Enabled = true

	if tenant.Tier == "" {
		tenant.Tier = TierStandard
	}

	// Apply default quotas based on tier
	if tenant.Quotas.MaxFeatures == 0 {
		tenant.Quotas = DefaultQuotasForTier(tenant.Tier)
	}

	if tenant.Settings.DefaultPriority == 0 {
		tenant.Settings.DefaultPriority = PriorityNormal
	}
	if tenant.Settings.DataIsolationMode == "" {
		tenant.Settings.DataIsolationMode = IsolationShared
	}

	if tenant.Metadata == nil {
		tenant.Metadata = make(map[string]string)
	}

	r.tenants[tenant.ID] = tenant
	r.usage[tenant.ID] = &TenantUsage{TenantID: tenant.ID, LastUpdated: now}
	r.metrics[tenant.ID] = &TenantMetrics{TenantID: tenant.ID, LastReset: now}
	r.rateLimiters[tenant.ID] = &rateLimiter{
		maxPerSecond: tenant.Quotas.MaxRequestsPerSecond,
		maxPerMinute: tenant.Quotas.MaxRequestsPerMinute,
		windowStart:  now,
	}

	// Notify callbacks
	for _, cb := range r.onTenantCreated {
		cb(tenant)
	}

	r.addAuditEntry(tenant.ID, "tenant_created", fmt.Sprintf("created tenant %s (tier: %s)", tenant.Name, tenant.Tier))

	return nil
}

// GetTenant retrieves a tenant by ID.
func (r *TenantRegistry) GetTenant(tenantID string) (*Tenant, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tenant, exists := r.tenants[tenantID]
	if !exists {
		return nil, fmt.Errorf("%w: %s", ErrTenantNotFound, tenantID)
	}
	return tenant, nil
}

// UpdateTenant updates an existing tenant.
func (r *TenantRegistry) UpdateTenant(tenant *Tenant) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	existing, exists := r.tenants[tenant.ID]
	if !exists {
		return fmt.Errorf("%w: %s", ErrTenantNotFound, tenant.ID)
	}

	tenant.CreatedAt = existing.CreatedAt
	tenant.UpdatedAt = time.Now()
	r.tenants[tenant.ID] = tenant

	// Update rate limiter if quotas changed
	if rl, ok := r.rateLimiters[tenant.ID]; ok {
		rl.mu.Lock()
		rl.maxPerSecond = tenant.Quotas.MaxRequestsPerSecond
		rl.maxPerMinute = tenant.Quotas.MaxRequestsPerMinute
		rl.mu.Unlock()
	}

	return nil
}

// DeleteTenant removes a tenant.
func (r *TenantRegistry) DeleteTenant(tenantID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.tenants[tenantID]; !exists {
		return fmt.Errorf("%w: %s", ErrTenantNotFound, tenantID)
	}

	delete(r.tenants, tenantID)
	delete(r.usage, tenantID)
	delete(r.metrics, tenantID)
	delete(r.rateLimiters, tenantID)

	return nil
}

// ListTenants returns all tenants.
func (r *TenantRegistry) ListTenants() []*Tenant {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tenants := make([]*Tenant, 0, len(r.tenants))
	for _, t := range r.tenants {
		tenants = append(tenants, t)
	}
	return tenants
}

// EnableTenant enables a tenant.
func (r *TenantRegistry) EnableTenant(tenantID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	tenant, exists := r.tenants[tenantID]
	if !exists {
		return fmt.Errorf("%w: %s", ErrTenantNotFound, tenantID)
	}

	tenant.Enabled = true
	tenant.UpdatedAt = time.Now()
	return nil
}

// DisableTenant disables a tenant.
func (r *TenantRegistry) DisableTenant(tenantID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	tenant, exists := r.tenants[tenantID]
	if !exists {
		return fmt.Errorf("%w: %s", ErrTenantNotFound, tenantID)
	}

	tenant.Enabled = false
	tenant.UpdatedAt = time.Now()
	return nil
}

// CheckQuota checks if a tenant has quota available.
func (r *TenantRegistry) CheckQuota(tenantID string, quotaType string, amount int64) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tenant, exists := r.tenants[tenantID]
	if !exists {
		return fmt.Errorf("%w: %s", ErrTenantNotFound, tenantID)
	}

	if !tenant.Enabled {
		return ErrTenantDisabled
	}

	usage := r.usage[tenantID]
	if usage == nil {
		return nil
	}

	switch quotaType {
	case "features":
		if tenant.Quotas.MaxFeatures > 0 && usage.FeatureCount+amount > int64(tenant.Quotas.MaxFeatures) {
			r.notifyQuotaExceeded(tenantID, quotaType)
			return fmt.Errorf("%w: features (%d/%d)", ErrQuotaExceeded, usage.FeatureCount, tenant.Quotas.MaxFeatures)
		}
	case "storage":
		if tenant.Quotas.MaxStorageBytes > 0 && usage.StorageBytes+amount > tenant.Quotas.MaxStorageBytes {
			r.notifyQuotaExceeded(tenantID, quotaType)
			return fmt.Errorf("%w: storage (%d/%d bytes)", ErrStorageQuotaExceeded, usage.StorageBytes, tenant.Quotas.MaxStorageBytes)
		}
	case "hot_tier":
		if tenant.Quotas.MaxHotTierBytes > 0 && usage.HotTierBytes+amount > tenant.Quotas.MaxHotTierBytes {
			r.notifyQuotaExceeded(tenantID, quotaType)
			return fmt.Errorf("%w: hot tier (%d/%d bytes)", ErrQuotaExceeded, usage.HotTierBytes, tenant.Quotas.MaxHotTierBytes)
		}
	case "entities":
		if tenant.Quotas.MaxEntityCount > 0 && usage.EntityCount+amount > tenant.Quotas.MaxEntityCount {
			r.notifyQuotaExceeded(tenantID, quotaType)
			return fmt.Errorf("%w: entities (%d/%d)", ErrQuotaExceeded, usage.EntityCount, tenant.Quotas.MaxEntityCount)
		}
	case "concurrent":
		if tenant.Quotas.MaxConcurrentRequests > 0 && usage.ConcurrentRequests+amount > int64(tenant.Quotas.MaxConcurrentRequests) {
			return fmt.Errorf("%w: concurrent requests (%d/%d)", ErrQuotaExceeded, usage.ConcurrentRequests, tenant.Quotas.MaxConcurrentRequests)
		}
	}

	return nil
}

// CheckRateLimit checks if a request is within rate limits.
func (r *TenantRegistry) CheckRateLimit(tenantID string) error {
	r.mu.RLock()
	tenant, exists := r.tenants[tenantID]
	rl := r.rateLimiters[tenantID]
	r.mu.RUnlock()

	if !exists {
		return fmt.Errorf("%w: %s", ErrTenantNotFound, tenantID)
	}

	if !tenant.Enabled {
		return ErrTenantDisabled
	}

	if rl == nil {
		return nil
	}

	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()

	// Reset window if needed
	if now.Sub(rl.windowStart) >= time.Second {
		rl.windowStart = now
		rl.windowCount = 0
	}

	// Check rate limit
	if rl.maxPerSecond > 0 && rl.windowCount >= int64(rl.maxPerSecond) {
		// Check if burst is allowed
		if tenant.Settings.AllowBurstTraffic && tenant.Settings.BurstMultiplier > 0 {
			burstLimit := int64(float64(rl.maxPerSecond) * tenant.Settings.BurstMultiplier)
			if rl.windowCount >= burstLimit {
				r.recordRateLimited(tenantID)
				return ErrRateLimitExceeded
			}
		} else {
			r.recordRateLimited(tenantID)
			return ErrRateLimitExceeded
		}
	}

	rl.windowCount++
	return nil
}

// RecordRequest records a request for a tenant.
func (r *TenantRegistry) RecordRequest(tenantID string, latency time.Duration, isError bool) {
	r.mu.RLock()
	metrics := r.metrics[tenantID]
	r.mu.RUnlock()

	if metrics == nil {
		return
	}

	atomic.AddInt64(&metrics.RequestCount, 1)
	atomic.AddInt64(&metrics.TotalLatencyNs, int64(latency))

	if isError {
		atomic.AddInt64(&metrics.ErrorCount, 1)
	}
}

// UpdateUsage updates usage metrics for a tenant.
func (r *TenantRegistry) UpdateUsage(tenantID string, updates func(*TenantUsage)) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	usage, exists := r.usage[tenantID]
	if !exists {
		return fmt.Errorf("%w: %s", ErrTenantNotFound, tenantID)
	}

	updates(usage)
	usage.LastUpdated = time.Now()
	return nil
}

// GetUsage returns the current usage for a tenant.
func (r *TenantRegistry) GetUsage(tenantID string) (*TenantUsage, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	usage, exists := r.usage[tenantID]
	if !exists {
		return nil, fmt.Errorf("%w: %s", ErrTenantNotFound, tenantID)
	}
	return usage, nil
}

// GetMetrics returns metrics for a tenant.
func (r *TenantRegistry) GetMetrics(tenantID string) (*TenantMetrics, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	metrics, exists := r.metrics[tenantID]
	if !exists {
		return nil, fmt.Errorf("%w: %s", ErrTenantNotFound, tenantID)
	}
	return metrics, nil
}

// GetPriority returns the priority class for a tenant.
func (r *TenantRegistry) GetPriority(tenantID string) PriorityClass {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tenant, exists := r.tenants[tenantID]
	if !exists {
		return PriorityNormal
	}
	return tenant.Settings.DefaultPriority
}

// OnTenantCreated registers a callback for tenant creation.
func (r *TenantRegistry) OnTenantCreated(cb func(*Tenant)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.onTenantCreated = append(r.onTenantCreated, cb)
}

// OnQuotaExceeded registers a callback for quota exceeded events.
func (r *TenantRegistry) OnQuotaExceeded(cb func(string, string)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.onQuotaExceeded = append(r.onQuotaExceeded, cb)
}

func (r *TenantRegistry) notifyQuotaExceeded(tenantID, quotaType string) {
	r.mu.RLock()
	callbacks := r.onQuotaExceeded
	r.mu.RUnlock()

	for _, cb := range callbacks {
		go cb(tenantID, quotaType)
	}

	// Update metrics
	if metrics, ok := r.metrics[tenantID]; ok {
		atomic.AddInt64(&metrics.QuotaExceededCount, 1)
	}
}

func (r *TenantRegistry) recordRateLimited(tenantID string) {
	r.mu.RLock()
	metrics := r.metrics[tenantID]
	r.mu.RUnlock()

	if metrics != nil {
		atomic.AddInt64(&metrics.RateLimitedCount, 1)
	}
}

// Stats returns registry statistics.
func (r *TenantRegistry) Stats() map[string]interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()

	byTier := make(map[string]int)
	enabledCount := 0

	for _, t := range r.tenants {
		byTier[string(t.Tier)]++
		if t.Enabled {
			enabledCount++
		}
	}

	return map[string]interface{}{
		"total_tenants":   len(r.tenants),
		"enabled_tenants": enabledCount,
		"by_tier":         byTier,
	}
}

// DefaultQuotasForTier returns default quotas for a tier.
func DefaultQuotasForTier(tier TenantTier) TenantQuotas {
	switch tier {
	case TierFree:
		return TenantQuotas{
			MaxFeatures:           100,
			MaxNamespaces:         5,
			MaxAPIKeys:            3,
			MaxRequestsPerSecond:  10,
			MaxRequestsPerMinute:  100,
			MaxStorageBytes:       100 * 1024 * 1024, // 100 MB
			MaxHotTierBytes:       10 * 1024 * 1024,  // 10 MB
			MaxEntityCount:        10000,
			MaxBatchSize:          100,
			MaxConcurrentRequests: 5,
		}
	case TierStandard:
		return TenantQuotas{
			MaxFeatures:           1000,
			MaxNamespaces:         20,
			MaxAPIKeys:            10,
			MaxRequestsPerSecond:  100,
			MaxRequestsPerMinute:  1000,
			MaxStorageBytes:       1024 * 1024 * 1024, // 1 GB
			MaxHotTierBytes:       100 * 1024 * 1024,  // 100 MB
			MaxEntityCount:        1000000,
			MaxBatchSize:          500,
			MaxConcurrentRequests: 20,
		}
	case TierPremium:
		return TenantQuotas{
			MaxFeatures:           10000,
			MaxNamespaces:         100,
			MaxAPIKeys:            50,
			MaxRequestsPerSecond:  1000,
			MaxRequestsPerMinute:  10000,
			MaxStorageBytes:       10 * 1024 * 1024 * 1024, // 10 GB
			MaxHotTierBytes:       1024 * 1024 * 1024,      // 1 GB
			MaxEntityCount:        100000000,
			MaxBatchSize:          2000,
			MaxConcurrentRequests: 100,
		}
	case TierEnterprise:
		return TenantQuotas{
			MaxFeatures:           0, // Unlimited
			MaxNamespaces:         0,
			MaxAPIKeys:            0,
			MaxRequestsPerSecond:  0,
			MaxRequestsPerMinute:  0,
			MaxStorageBytes:       0,
			MaxHotTierBytes:       0,
			MaxEntityCount:        0,
			MaxBatchSize:          10000,
			MaxConcurrentRequests: 0,
		}
	default:
		return DefaultQuotasForTier(TierStandard)
	}
}

// Context helpers

type tenantContextKey struct{}

// WithTenant adds tenant ID to context.
func WithTenant(ctx context.Context, tenantID string) context.Context {
	return context.WithValue(ctx, tenantContextKey{}, tenantID)
}

// TenantFromContext retrieves tenant ID from context.
func TenantFromContext(ctx context.Context) string {
	tenantID, _ := ctx.Value(tenantContextKey{}).(string)
	return tenantID
}

// ShareGrant represents a cross-tenant feature sharing permission.
type ShareGrant struct {
	ID           string     `json:"id"`
	FromTenantID string     `json:"from_tenant_id"`
	ToTenantID   string     `json:"to_tenant_id"`
	Features     []string   `json:"features"`   // empty means all features
	Permission   string     `json:"permission"` // "read", "read_write"
	GrantedBy    string     `json:"granted_by"`
	GrantedAt    time.Time  `json:"granted_at"`
	ExpiresAt    *time.Time `json:"expires_at,omitempty"`
}

// GrantShare creates a cross-tenant sharing permission.
func (r *TenantRegistry) GrantShare(grant *ShareGrant) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.tenants[grant.FromTenantID]; !ok {
		return ErrTenantNotFound
	}
	if _, ok := r.tenants[grant.ToTenantID]; !ok {
		return ErrTenantNotFound
	}
	if grant.Permission != "read" && grant.Permission != "read_write" {
		return errors.New("permission must be 'read' or 'read_write'")
	}

	grant.ID = fmt.Sprintf("share-%d", time.Now().UnixNano())
	grant.GrantedAt = time.Now()
	r.shares = append(r.shares, grant)

	r.addAuditEntry(grant.FromTenantID, "share_granted",
		fmt.Sprintf("granted %s access to %s for features: %v", grant.Permission, grant.ToTenantID, grant.Features))

	return nil
}

// RevokeShare removes a sharing grant by ID.
func (r *TenantRegistry) RevokeShare(grantID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for i, g := range r.shares {
		if g.ID == grantID {
			r.addAuditEntry(g.FromTenantID, "share_revoked",
				fmt.Sprintf("revoked share %s to %s", grantID, g.ToTenantID))
			r.shares = append(r.shares[:i], r.shares[i+1:]...)
			return nil
		}
	}
	return errors.New("share grant not found")
}

// ListShares returns all shares for a tenant (both granted and received).
func (r *TenantRegistry) ListShares(tenantID string) []*ShareGrant {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*ShareGrant, 0)
	for _, g := range r.shares {
		if g.FromTenantID == tenantID || g.ToTenantID == tenantID {
			result = append(result, g)
		}
	}
	return result
}

// CanAccess checks if a tenant has access to a feature in another tenant's namespace.
func (r *TenantRegistry) CanAccess(requestingTenant, ownerTenant, featureName, permission string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if requestingTenant == ownerTenant {
		return true
	}

	now := time.Now()
	for _, g := range r.shares {
		if g.FromTenantID != ownerTenant || g.ToTenantID != requestingTenant {
			continue
		}
		if g.ExpiresAt != nil && now.After(*g.ExpiresAt) {
			continue
		}
		if permission == "read_write" && g.Permission == "read" {
			continue
		}
		if len(g.Features) == 0 {
			return true // all features shared
		}
		for _, f := range g.Features {
			if f == featureName {
				return true
			}
		}
	}
	return false
}

// AuditEntry records an action taken on a tenant.
type AuditEntry struct {
	TenantID  string    `json:"tenant_id"`
	Action    string    `json:"action"`
	Detail    string    `json:"detail"`
	Timestamp time.Time `json:"timestamp"`
}

// GetAuditLog returns the audit log for a tenant.
func (r *TenantRegistry) GetAuditLog(tenantID string, limit int) []AuditEntry {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]AuditEntry, 0)
	for _, e := range r.auditLog {
		if e.TenantID == tenantID {
			result = append(result, e)
		}
	}
	// Return most recent first
	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}
	if limit > 0 && limit < len(result) {
		result = result[:limit]
	}
	return result
}

func (r *TenantRegistry) addAuditEntry(tenantID, action, detail string) {
	r.auditLog = append(r.auditLog, AuditEntry{
		TenantID:  tenantID,
		Action:    action,
		Detail:    detail,
		Timestamp: time.Now(),
	})
}

//revive:enable:exported
