package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"sync"
	"time"
)

// Permission represents a specific permission.
type Permission string

const (
	// PermRead allows read access.
	PermRead Permission = "read"
	// PermWrite allows write access.
	PermWrite Permission = "write"
	// PermDelete allows delete access.
	PermDelete Permission = "delete"
	// PermAdmin allows administrative access.
	PermAdmin Permission = "admin"
	// PermManageKeys allows API key management.
	PermManageKeys Permission = "manage_keys"
)

// Role represents a predefined set of permissions.
type Role struct {
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Permissions []Permission `json:"permissions"`
	IsBuiltin   bool         `json:"is_builtin"`
}

// Predefined roles
var (
	RoleReader = &Role{
		Name:        "reader",
		Description: "Read-only access to features",
		Permissions: []Permission{PermRead},
		IsBuiltin:   true,
	}

	RoleWriter = &Role{
		Name:        "writer",
		Description: "Read and write access to features",
		Permissions: []Permission{PermRead, PermWrite},
		IsBuiltin:   true,
	}

	RoleAdmin = &Role{
		Name:        "admin",
		Description: "Full access including admin operations",
		Permissions: []Permission{PermRead, PermWrite, PermDelete, PermAdmin, PermManageKeys},
		IsBuiltin:   true,
	}
)

// APIKey represents an API key for authentication.
type APIKey struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	KeyHash     string            `json:"-"`      // Hashed key (never expose)
	Prefix      string            `json:"prefix"` // First 8 chars for identification
	Tenant      string            `json:"tenant"`
	Roles       []string          `json:"roles"`
	Permissions []Permission      `json:"permissions"` // Direct permissions (in addition to roles)
	Namespaces  []string          `json:"namespaces"`  // Allowed namespaces (empty = all)
	Features    []string          `json:"features"`    // Allowed features (empty = all)
	RateLimit   int               `json:"rate_limit"`  // Requests per minute (0 = unlimited)
	ExpiresAt   *time.Time        `json:"expires_at,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	CreatedBy   string            `json:"created_by"`
	LastUsedAt  *time.Time        `json:"last_used_at,omitempty"`
	Metadata    map[string]string `json:"metadata"`
	Enabled     bool              `json:"enabled"`
}

// Tenant represents a tenant in the multi-tenant system.
type Tenant struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Enabled     bool              `json:"enabled"`
	Quotas      TenantQuotas      `json:"quotas"`
	Metadata    map[string]string `json:"metadata"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// TenantQuotas defines resource limits for a tenant.
type TenantQuotas struct {
	MaxFeatures       int   `json:"max_features"`         // Max number of features
	MaxNamespaces     int   `json:"max_namespaces"`       // Max number of namespaces
	MaxAPIKeys        int   `json:"max_api_keys"`         // Max number of API keys
	MaxRequestsPerMin int   `json:"max_requests_per_min"` // Rate limit
	MaxStorageBytes   int64 `json:"max_storage_bytes"`    // Storage quota
}

// AuditLog represents an audit log entry.
type AuditLog struct {
	ID        string                 `json:"id"`
	Timestamp time.Time              `json:"timestamp"`
	Tenant    string                 `json:"tenant"`
	UserID    string                 `json:"user_id"`
	APIKeyID  string                 `json:"api_key_id,omitempty"`
	Action    string                 `json:"action"`
	Resource  string                 `json:"resource"`
	Details   map[string]interface{} `json:"details,omitempty"`
	IP        string                 `json:"ip,omitempty"`
	UserAgent string                 `json:"user_agent,omitempty"`
	Success   bool                   `json:"success"`
	Error     string                 `json:"error,omitempty"`
}

// AccessController manages authentication and authorization.
type AccessController struct {
	apiKeys      map[string]*APIKey // keyHash -> APIKey
	keysByID     map[string]*APIKey // id -> APIKey
	tenants      map[string]*Tenant
	roles        map[string]*Role
	auditLogs    []AuditLog
	maxAuditLogs int
	mu           sync.RWMutex
}

// NewAccessController creates a new access controller.
func NewAccessController() *AccessController {
	ac := &AccessController{
		apiKeys:      make(map[string]*APIKey),
		keysByID:     make(map[string]*APIKey),
		tenants:      make(map[string]*Tenant),
		roles:        make(map[string]*Role),
		auditLogs:    make([]AuditLog, 0),
		maxAuditLogs: 10000,
	}

	// Register builtin roles
	ac.roles["reader"] = RoleReader
	ac.roles["writer"] = RoleWriter
	ac.roles["admin"] = RoleAdmin

	return ac
}

// CreateAPIKey creates a new API key and returns the raw key.
func (ac *AccessController) CreateAPIKey(key *APIKey, createdBy string) (string, error) {
	if key.Name == "" {
		return "", ErrNameRequired
	}
	if key.Tenant == "" {
		return "", ErrTenantRequired
	}

	// Generate random key
	rawKey := generateAPIKey()
	keyHash := hashKey(rawKey)
	prefix := rawKey[:8]

	ac.mu.Lock()
	defer ac.mu.Unlock()

	// Check tenant exists
	if _, ok := ac.tenants[key.Tenant]; !ok {
		return "", ErrTenantNotFound
	}

	key.ID = generateID()
	key.KeyHash = keyHash
	key.Prefix = prefix
	key.CreatedAt = time.Now()
	key.CreatedBy = createdBy
	key.Enabled = true

	if key.Metadata == nil {
		key.Metadata = make(map[string]string)
	}

	ac.apiKeys[keyHash] = key
	ac.keysByID[key.ID] = key

	return rawKey, nil
}

// ValidateAPIKey validates an API key and returns the associated key info.
func (ac *AccessController) ValidateAPIKey(rawKey string) (*APIKey, error) {
	keyHash := hashKey(rawKey)

	ac.mu.RLock()
	key, ok := ac.apiKeys[keyHash]
	ac.mu.RUnlock()

	if !ok {
		return nil, ErrInvalidAPIKey
	}

	if !key.Enabled {
		return nil, ErrAPIKeyDisabled
	}

	if key.ExpiresAt != nil && key.ExpiresAt.Before(time.Now()) {
		return nil, ErrAPIKeyExpired
	}

	// Update last used
	ac.mu.Lock()
	now := time.Now()
	key.LastUsedAt = &now
	ac.mu.Unlock()

	return key, nil
}

// GetAPIKey retrieves an API key by ID.
func (ac *AccessController) GetAPIKey(id string) *APIKey {
	ac.mu.RLock()
	defer ac.mu.RUnlock()
	return ac.keysByID[id]
}

// ListAPIKeys lists all API keys for a tenant.
func (ac *AccessController) ListAPIKeys(tenant string) []*APIKey {
	ac.mu.RLock()
	defer ac.mu.RUnlock()

	var keys []*APIKey
	for _, key := range ac.keysByID {
		if tenant == "" || key.Tenant == tenant {
			keys = append(keys, key)
		}
	}
	return keys
}

// RevokeAPIKey revokes an API key.
func (ac *AccessController) RevokeAPIKey(id string) error {
	ac.mu.Lock()
	defer ac.mu.Unlock()

	key, ok := ac.keysByID[id]
	if !ok {
		return ErrAPIKeyNotFound
	}

	key.Enabled = false
	return nil
}

// DeleteAPIKey deletes an API key.
func (ac *AccessController) DeleteAPIKey(id string) error {
	ac.mu.Lock()
	defer ac.mu.Unlock()

	key, ok := ac.keysByID[id]
	if !ok {
		return ErrAPIKeyNotFound
	}

	delete(ac.apiKeys, key.KeyHash)
	delete(ac.keysByID, id)
	return nil
}

// HasPermission checks if an API key has a specific permission.
func (ac *AccessController) HasPermission(key *APIKey, perm Permission) bool {
	// Check direct permissions
	for _, p := range key.Permissions {
		if p == perm || p == PermAdmin {
			return true
		}
	}

	// Check role permissions
	ac.mu.RLock()
	defer ac.mu.RUnlock()

	for _, roleName := range key.Roles {
		if role, ok := ac.roles[roleName]; ok {
			for _, p := range role.Permissions {
				if p == perm || p == PermAdmin {
					return true
				}
			}
		}
	}

	return false
}

// CanAccessNamespace checks if an API key can access a namespace.
func (ac *AccessController) CanAccessNamespace(key *APIKey, namespace string) bool {
	if len(key.Namespaces) == 0 {
		return true // No restrictions
	}

	for _, ns := range key.Namespaces {
		if ns == namespace || ns == "*" {
			return true
		}
	}
	return false
}

// CanAccessFeature checks if an API key can access a feature.
func (ac *AccessController) CanAccessFeature(key *APIKey, feature string) bool {
	if len(key.Features) == 0 {
		return true // No restrictions
	}

	for _, f := range key.Features {
		if f == feature || f == "*" {
			return true
		}
	}
	return false
}

// CreateTenant creates a new tenant.
func (ac *AccessController) CreateTenant(tenant *Tenant) error {
	if tenant.ID == "" {
		return ErrTenantIDRequired
	}

	ac.mu.Lock()
	defer ac.mu.Unlock()

	if _, ok := ac.tenants[tenant.ID]; ok {
		return ErrTenantExists
	}

	tenant.CreatedAt = time.Now()
	tenant.UpdatedAt = time.Now()
	tenant.Enabled = true

	if tenant.Metadata == nil {
		tenant.Metadata = make(map[string]string)
	}

	ac.tenants[tenant.ID] = tenant
	return nil
}

// GetTenant retrieves a tenant by ID.
func (ac *AccessController) GetTenant(id string) *Tenant {
	ac.mu.RLock()
	defer ac.mu.RUnlock()
	return ac.tenants[id]
}

// ListTenants lists all tenants.
func (ac *AccessController) ListTenants() []*Tenant {
	ac.mu.RLock()
	defer ac.mu.RUnlock()

	tenants := make([]*Tenant, 0, len(ac.tenants))
	for _, t := range ac.tenants {
		tenants = append(tenants, t)
	}
	return tenants
}

// UpdateTenant updates a tenant.
func (ac *AccessController) UpdateTenant(tenant *Tenant) error {
	ac.mu.Lock()
	defer ac.mu.Unlock()

	existing, ok := ac.tenants[tenant.ID]
	if !ok {
		return ErrTenantNotFound
	}

	tenant.CreatedAt = existing.CreatedAt
	tenant.UpdatedAt = time.Now()
	ac.tenants[tenant.ID] = tenant
	return nil
}

// DeleteTenant deletes a tenant.
func (ac *AccessController) DeleteTenant(id string) error {
	ac.mu.Lock()
	defer ac.mu.Unlock()

	if _, ok := ac.tenants[id]; !ok {
		return ErrTenantNotFound
	}

	// Delete all API keys for this tenant
	for keyHash, key := range ac.apiKeys {
		if key.Tenant == id {
			delete(ac.apiKeys, keyHash)
			delete(ac.keysByID, key.ID)
		}
	}

	delete(ac.tenants, id)
	return nil
}

// CreateRole creates a custom role.
func (ac *AccessController) CreateRole(role *Role) error {
	if role.Name == "" {
		return ErrNameRequired
	}

	ac.mu.Lock()
	defer ac.mu.Unlock()

	if existing, ok := ac.roles[role.Name]; ok && existing.IsBuiltin {
		return ErrCannotModifyBuiltin
	}

	role.IsBuiltin = false
	ac.roles[role.Name] = role
	return nil
}

// GetRole retrieves a role by name.
func (ac *AccessController) GetRole(name string) *Role {
	ac.mu.RLock()
	defer ac.mu.RUnlock()
	return ac.roles[name]
}

// ListRoles lists all roles.
func (ac *AccessController) ListRoles() []*Role {
	ac.mu.RLock()
	defer ac.mu.RUnlock()

	roles := make([]*Role, 0, len(ac.roles))
	for _, r := range ac.roles {
		roles = append(roles, r)
	}
	return roles
}

// DeleteRole deletes a custom role.
func (ac *AccessController) DeleteRole(name string) error {
	ac.mu.Lock()
	defer ac.mu.Unlock()

	role, ok := ac.roles[name]
	if !ok {
		return ErrRoleNotFound
	}

	if role.IsBuiltin {
		return ErrCannotModifyBuiltin
	}

	delete(ac.roles, name)
	return nil
}

// LogAudit records an audit log entry.
func (ac *AccessController) LogAudit(log AuditLog) {
	log.ID = generateID()
	log.Timestamp = time.Now()

	ac.mu.Lock()
	defer ac.mu.Unlock()

	ac.auditLogs = append(ac.auditLogs, log)

	// Trim if over limit
	if len(ac.auditLogs) > ac.maxAuditLogs {
		ac.auditLogs = ac.auditLogs[len(ac.auditLogs)-ac.maxAuditLogs:]
	}
}

// GetAuditLogs retrieves audit logs with optional filtering.
func (ac *AccessController) GetAuditLogs(tenant string, since time.Time, limit int) []AuditLog {
	ac.mu.RLock()
	defer ac.mu.RUnlock()

	var result []AuditLog
	for i := len(ac.auditLogs) - 1; i >= 0 && len(result) < limit; i-- {
		log := ac.auditLogs[i]
		if log.Timestamp.Before(since) {
			continue
		}
		if tenant != "" && log.Tenant != tenant {
			continue
		}
		result = append(result, log)
	}
	return result
}

// Context key for storing auth info
type contextKey string

const (
	// ContextKeyAPIKey stores API key info in the request context.
	ContextKeyAPIKey contextKey = "api_key"
	// ContextKeyTenant stores tenant info in the request context.
	ContextKeyTenant contextKey = "tenant"
)

// WithAPIKey adds API key info to context.
func WithAPIKey(ctx context.Context, key *APIKey) context.Context {
	return context.WithValue(ctx, ContextKeyAPIKey, key)
}

// APIKeyFromContext retrieves API key info from context.
func APIKeyFromContext(ctx context.Context) *APIKey {
	key, _ := ctx.Value(ContextKeyAPIKey).(*APIKey)
	return key
}

// WithTenant adds tenant info to context.
func WithTenant(ctx context.Context, tenant string) context.Context {
	return context.WithValue(ctx, ContextKeyTenant, tenant)
}

// TenantFromContext retrieves tenant from context.
func TenantFromContext(ctx context.Context) string {
	tenant, _ := ctx.Value(ContextKeyTenant).(string)
	return tenant
}

// Helper functions
// generateAPIKey creates a new API key with cryptographic randomness.
// Panics on crypto/rand failure because a predictable API key would be
// a critical security vulnerability.
func generateAPIKey() string {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		panic("crypto/rand failed: " + err.Error())
	}
	return "fk_" + hex.EncodeToString(bytes)
}

// generateID creates a random identifier.
// Panics on crypto/rand failure because falling back to a predictable ID
// could cause collisions or security issues in authentication contexts.
func generateID() string {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		panic("crypto/rand failed: " + err.Error())
	}
	return hex.EncodeToString(bytes)
}

func hashKey(key string) string {
	hash := sha256.Sum256([]byte(key))
	return hex.EncodeToString(hash[:])
}

// Errors
var (
	ErrNameRequired        = errors.New("name is required")
	ErrTenantRequired      = errors.New("tenant is required")
	ErrTenantIDRequired    = errors.New("tenant ID is required")
	ErrTenantNotFound      = errors.New("tenant not found")
	ErrTenantExists        = errors.New("tenant already exists")
	ErrInvalidAPIKey       = errors.New("invalid API key")
	ErrAPIKeyDisabled      = errors.New("API key is disabled")
	ErrAPIKeyExpired       = errors.New("API key has expired")
	ErrAPIKeyNotFound      = errors.New("API key not found")
	ErrRoleNotFound        = errors.New("role not found")
	ErrCannotModifyBuiltin = errors.New("cannot modify builtin role")
	ErrPermissionDenied    = errors.New("permission denied")
)
