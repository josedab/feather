package auth

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestNewAccessController(t *testing.T) {
	ac := NewAccessController()
	if ac == nil {
		t.Fatal("NewAccessController returned nil")
	}

	// Verify builtin roles are registered
	if ac.GetRole("reader") == nil {
		t.Error("reader role not registered")
	}
	if ac.GetRole("writer") == nil {
		t.Error("writer role not registered")
	}
	if ac.GetRole("admin") == nil {
		t.Error("admin role not registered")
	}
}

func TestAccessController_CreateTenant(t *testing.T) {
	ac := NewAccessController()

	tests := []struct {
		name    string
		tenant  *Tenant
		wantErr error
	}{
		{
			name:    "valid tenant",
			tenant:  &Tenant{ID: "tenant1", Name: "Test Tenant"},
			wantErr: nil,
		},
		{
			name:    "missing ID",
			tenant:  &Tenant{Name: "Test Tenant"},
			wantErr: ErrTenantIDRequired,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ac.CreateTenant(tt.tenant)
			if err != tt.wantErr {
				t.Errorf("CreateTenant() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestAccessController_CreateTenant_Duplicate(t *testing.T) {
	ac := NewAccessController()

	tenant := &Tenant{ID: "tenant1", Name: "Test Tenant"}
	err := ac.CreateTenant(tenant)
	if err != nil {
		t.Fatalf("first CreateTenant() failed: %v", err)
	}

	err = ac.CreateTenant(tenant)
	if err != ErrTenantExists {
		t.Errorf("CreateTenant() duplicate error = %v, want %v", err, ErrTenantExists)
	}
}

func TestAccessController_GetTenant(t *testing.T) {
	ac := NewAccessController()

	// Get non-existent tenant
	if ac.GetTenant("nonexistent") != nil {
		t.Error("GetTenant() should return nil for non-existent tenant")
	}

	// Create and get tenant
	tenant := &Tenant{ID: "tenant1", Name: "Test Tenant"}
	ac.CreateTenant(tenant)

	got := ac.GetTenant("tenant1")
	if got == nil {
		t.Fatal("GetTenant() returned nil for existing tenant")
	}
	if got.Name != "Test Tenant" {
		t.Errorf("GetTenant() Name = %v, want %v", got.Name, "Test Tenant")
	}
	if !got.Enabled {
		t.Error("Tenant should be enabled by default")
	}
}

func TestAccessController_ListTenants(t *testing.T) {
	ac := NewAccessController()

	// Empty list
	if len(ac.ListTenants()) != 0 {
		t.Error("ListTenants() should return empty list initially")
	}

	// Add tenants
	ac.CreateTenant(&Tenant{ID: "t1", Name: "Tenant 1"})
	ac.CreateTenant(&Tenant{ID: "t2", Name: "Tenant 2"})

	tenants := ac.ListTenants()
	if len(tenants) != 2 {
		t.Errorf("ListTenants() count = %d, want 2", len(tenants))
	}
}

func TestAccessController_UpdateTenant(t *testing.T) {
	ac := NewAccessController()

	// Update non-existent
	err := ac.UpdateTenant(&Tenant{ID: "nonexistent"})
	if err != ErrTenantNotFound {
		t.Errorf("UpdateTenant() error = %v, want %v", err, ErrTenantNotFound)
	}

	// Create and update
	ac.CreateTenant(&Tenant{ID: "tenant1", Name: "Original"})
	originalCreatedAt := ac.GetTenant("tenant1").CreatedAt

	time.Sleep(10 * time.Millisecond)
	err = ac.UpdateTenant(&Tenant{ID: "tenant1", Name: "Updated"})
	if err != nil {
		t.Fatalf("UpdateTenant() failed: %v", err)
	}

	got := ac.GetTenant("tenant1")
	if got.Name != "Updated" {
		t.Errorf("UpdateTenant() Name = %v, want %v", got.Name, "Updated")
	}
	if !got.CreatedAt.Equal(originalCreatedAt) {
		t.Error("CreatedAt should be preserved")
	}
	if !got.UpdatedAt.After(originalCreatedAt) {
		t.Error("UpdatedAt should be updated")
	}
}

func TestAccessController_DeleteTenant(t *testing.T) {
	ac := NewAccessController()

	// Delete non-existent
	err := ac.DeleteTenant("nonexistent")
	if err != ErrTenantNotFound {
		t.Errorf("DeleteTenant() error = %v, want %v", err, ErrTenantNotFound)
	}

	// Create tenant with API key, then delete
	ac.CreateTenant(&Tenant{ID: "tenant1", Name: "Test"})
	rawKey, _ := ac.CreateAPIKey(&APIKey{Name: "key1", Tenant: "tenant1"}, "admin")

	// Validate key works before deletion
	_, err = ac.ValidateAPIKey(rawKey)
	if err != nil {
		t.Fatalf("API key should be valid before tenant deletion: %v", err)
	}

	// Delete tenant
	err = ac.DeleteTenant("tenant1")
	if err != nil {
		t.Fatalf("DeleteTenant() failed: %v", err)
	}

	// Verify tenant deleted
	if ac.GetTenant("tenant1") != nil {
		t.Error("Tenant should be deleted")
	}

	// Verify API key deleted
	_, err = ac.ValidateAPIKey(rawKey)
	if err != ErrInvalidAPIKey {
		t.Errorf("API key should be invalid after tenant deletion, got: %v", err)
	}
}

func TestAccessController_CreateAPIKey(t *testing.T) {
	ac := NewAccessController()

	// Create tenant first
	ac.CreateTenant(&Tenant{ID: "tenant1", Name: "Test"})

	tests := []struct {
		name    string
		key     *APIKey
		wantErr error
	}{
		{
			name:    "valid key",
			key:     &APIKey{Name: "key1", Tenant: "tenant1"},
			wantErr: nil,
		},
		{
			name:    "missing name",
			key:     &APIKey{Tenant: "tenant1"},
			wantErr: ErrNameRequired,
		},
		{
			name:    "missing tenant",
			key:     &APIKey{Name: "key1"},
			wantErr: ErrTenantRequired,
		},
		{
			name:    "non-existent tenant",
			key:     &APIKey{Name: "key1", Tenant: "nonexistent"},
			wantErr: ErrTenantNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rawKey, err := ac.CreateAPIKey(tt.key, "admin")
			if err != tt.wantErr {
				t.Errorf("CreateAPIKey() error = %v, want %v", err, tt.wantErr)
			}
			if tt.wantErr == nil {
				if rawKey == "" {
					t.Error("CreateAPIKey() should return raw key")
				}
				if !tt.key.Enabled {
					t.Error("API key should be enabled by default")
				}
				if tt.key.ID == "" {
					t.Error("API key should have ID")
				}
				if tt.key.Prefix == "" {
					t.Error("API key should have prefix")
				}
			}
		})
	}
}

func TestAccessController_ValidateAPIKey(t *testing.T) {
	ac := NewAccessController()
	ac.CreateTenant(&Tenant{ID: "tenant1", Name: "Test"})

	// Create a valid key
	rawKey, _ := ac.CreateAPIKey(&APIKey{Name: "key1", Tenant: "tenant1"}, "admin")

	// Valid key
	key, err := ac.ValidateAPIKey(rawKey)
	if err != nil {
		t.Errorf("ValidateAPIKey() valid key error = %v", err)
	}
	if key == nil {
		t.Error("ValidateAPIKey() should return key info")
	}
	if key.LastUsedAt == nil {
		t.Error("LastUsedAt should be set after validation")
	}

	// Invalid key
	_, err = ac.ValidateAPIKey("invalid_key")
	if err != ErrInvalidAPIKey {
		t.Errorf("ValidateAPIKey() invalid key error = %v, want %v", err, ErrInvalidAPIKey)
	}
}

func TestAccessController_ValidateAPIKey_Disabled(t *testing.T) {
	ac := NewAccessController()
	ac.CreateTenant(&Tenant{ID: "tenant1", Name: "Test"})

	rawKey, _ := ac.CreateAPIKey(&APIKey{Name: "key1", Tenant: "tenant1"}, "admin")
	keyInfo, _ := ac.ValidateAPIKey(rawKey)

	// Revoke the key
	ac.RevokeAPIKey(keyInfo.ID)

	// Try to validate
	_, err := ac.ValidateAPIKey(rawKey)
	if err != ErrAPIKeyDisabled {
		t.Errorf("ValidateAPIKey() disabled key error = %v, want %v", err, ErrAPIKeyDisabled)
	}
}

func TestAccessController_ValidateAPIKey_Expired(t *testing.T) {
	ac := NewAccessController()
	ac.CreateTenant(&Tenant{ID: "tenant1", Name: "Test"})

	// Create key with past expiration
	expiredTime := time.Now().Add(-1 * time.Hour)
	apiKey := &APIKey{
		Name:      "key1",
		Tenant:    "tenant1",
		ExpiresAt: &expiredTime,
	}
	rawKey, _ := ac.CreateAPIKey(apiKey, "admin")

	_, err := ac.ValidateAPIKey(rawKey)
	if err != ErrAPIKeyExpired {
		t.Errorf("ValidateAPIKey() expired key error = %v, want %v", err, ErrAPIKeyExpired)
	}
}

func TestAccessController_GetAPIKey(t *testing.T) {
	ac := NewAccessController()
	ac.CreateTenant(&Tenant{ID: "tenant1", Name: "Test"})

	// Non-existent
	if ac.GetAPIKey("nonexistent") != nil {
		t.Error("GetAPIKey() should return nil for non-existent key")
	}

	// Create and get
	rawKey, _ := ac.CreateAPIKey(&APIKey{Name: "key1", Tenant: "tenant1"}, "admin")
	keyInfo, _ := ac.ValidateAPIKey(rawKey)

	got := ac.GetAPIKey(keyInfo.ID)
	if got == nil {
		t.Error("GetAPIKey() should return key")
	}
	if got.Name != "key1" {
		t.Errorf("GetAPIKey() Name = %v, want %v", got.Name, "key1")
	}
}

func TestAccessController_ListAPIKeys(t *testing.T) {
	ac := NewAccessController()
	ac.CreateTenant(&Tenant{ID: "tenant1", Name: "Tenant 1"})
	ac.CreateTenant(&Tenant{ID: "tenant2", Name: "Tenant 2"})

	// Create keys for both tenants
	ac.CreateAPIKey(&APIKey{Name: "key1", Tenant: "tenant1"}, "admin")
	ac.CreateAPIKey(&APIKey{Name: "key2", Tenant: "tenant1"}, "admin")
	ac.CreateAPIKey(&APIKey{Name: "key3", Tenant: "tenant2"}, "admin")

	// List all
	all := ac.ListAPIKeys("")
	if len(all) != 3 {
		t.Errorf("ListAPIKeys('') count = %d, want 3", len(all))
	}

	// List by tenant
	t1Keys := ac.ListAPIKeys("tenant1")
	if len(t1Keys) != 2 {
		t.Errorf("ListAPIKeys('tenant1') count = %d, want 2", len(t1Keys))
	}
}

func TestAccessController_RevokeAPIKey(t *testing.T) {
	ac := NewAccessController()
	ac.CreateTenant(&Tenant{ID: "tenant1", Name: "Test"})

	// Revoke non-existent
	err := ac.RevokeAPIKey("nonexistent")
	if err != ErrAPIKeyNotFound {
		t.Errorf("RevokeAPIKey() error = %v, want %v", err, ErrAPIKeyNotFound)
	}

	// Create and revoke
	rawKey, _ := ac.CreateAPIKey(&APIKey{Name: "key1", Tenant: "tenant1"}, "admin")
	keyInfo, _ := ac.ValidateAPIKey(rawKey)

	err = ac.RevokeAPIKey(keyInfo.ID)
	if err != nil {
		t.Fatalf("RevokeAPIKey() failed: %v", err)
	}

	// Key still exists but is disabled
	got := ac.GetAPIKey(keyInfo.ID)
	if got == nil {
		t.Error("Key should still exist after revocation")
	}
	if got.Enabled {
		t.Error("Key should be disabled after revocation")
	}
}

func TestAccessController_DeleteAPIKey(t *testing.T) {
	ac := NewAccessController()
	ac.CreateTenant(&Tenant{ID: "tenant1", Name: "Test"})

	// Delete non-existent
	err := ac.DeleteAPIKey("nonexistent")
	if err != ErrAPIKeyNotFound {
		t.Errorf("DeleteAPIKey() error = %v, want %v", err, ErrAPIKeyNotFound)
	}

	// Create and delete
	rawKey, _ := ac.CreateAPIKey(&APIKey{Name: "key1", Tenant: "tenant1"}, "admin")
	keyInfo, _ := ac.ValidateAPIKey(rawKey)

	err = ac.DeleteAPIKey(keyInfo.ID)
	if err != nil {
		t.Fatalf("DeleteAPIKey() failed: %v", err)
	}

	// Key should not exist
	if ac.GetAPIKey(keyInfo.ID) != nil {
		t.Error("Key should be deleted")
	}
}

func TestAccessController_HasPermission(t *testing.T) {
	ac := NewAccessController()

	tests := []struct {
		name        string
		key         *APIKey
		permission  Permission
		hasPermission bool
	}{
		{
			name:        "direct permission",
			key:         &APIKey{Permissions: []Permission{PermRead, PermWrite}},
			permission:  PermRead,
			hasPermission: true,
		},
		{
			name:        "admin has all",
			key:         &APIKey{Permissions: []Permission{PermAdmin}},
			permission:  PermDelete,
			hasPermission: true,
		},
		{
			name:        "missing direct permission",
			key:         &APIKey{Permissions: []Permission{PermRead}},
			permission:  PermWrite,
			hasPermission: false,
		},
		{
			name:        "role permission - reader",
			key:         &APIKey{Roles: []string{"reader"}},
			permission:  PermRead,
			hasPermission: true,
		},
		{
			name:        "role permission - writer",
			key:         &APIKey{Roles: []string{"writer"}},
			permission:  PermWrite,
			hasPermission: true,
		},
		{
			name:        "role missing permission",
			key:         &APIKey{Roles: []string{"reader"}},
			permission:  PermWrite,
			hasPermission: false,
		},
		{
			name:        "admin role has all",
			key:         &APIKey{Roles: []string{"admin"}},
			permission:  PermDelete,
			hasPermission: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ac.HasPermission(tt.key, tt.permission)
			if result != tt.hasPermission {
				t.Errorf("HasPermission() = %v, want %v", result, tt.hasPermission)
			}
		})
	}
}

func TestAccessController_CanAccessNamespace(t *testing.T) {
	ac := NewAccessController()

	tests := []struct {
		name      string
		key       *APIKey
		namespace string
		canAccess bool
	}{
		{
			name:      "no restrictions",
			key:       &APIKey{},
			namespace: "any-namespace",
			canAccess: true,
		},
		{
			name:      "allowed namespace",
			key:       &APIKey{Namespaces: []string{"ns1", "ns2"}},
			namespace: "ns1",
			canAccess: true,
		},
		{
			name:      "denied namespace",
			key:       &APIKey{Namespaces: []string{"ns1", "ns2"}},
			namespace: "ns3",
			canAccess: false,
		},
		{
			name:      "wildcard access",
			key:       &APIKey{Namespaces: []string{"*"}},
			namespace: "any-namespace",
			canAccess: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ac.CanAccessNamespace(tt.key, tt.namespace)
			if result != tt.canAccess {
				t.Errorf("CanAccessNamespace() = %v, want %v", result, tt.canAccess)
			}
		})
	}
}

func TestAccessController_CanAccessFeature(t *testing.T) {
	ac := NewAccessController()

	tests := []struct {
		name      string
		key       *APIKey
		feature   string
		canAccess bool
	}{
		{
			name:      "no restrictions",
			key:       &APIKey{},
			feature:   "any-feature",
			canAccess: true,
		},
		{
			name:      "allowed feature",
			key:       &APIKey{Features: []string{"feature1", "feature2"}},
			feature:   "feature1",
			canAccess: true,
		},
		{
			name:      "denied feature",
			key:       &APIKey{Features: []string{"feature1"}},
			feature:   "feature2",
			canAccess: false,
		},
		{
			name:      "wildcard access",
			key:       &APIKey{Features: []string{"*"}},
			feature:   "any-feature",
			canAccess: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ac.CanAccessFeature(tt.key, tt.feature)
			if result != tt.canAccess {
				t.Errorf("CanAccessFeature() = %v, want %v", result, tt.canAccess)
			}
		})
	}
}

func TestAccessController_CreateRole(t *testing.T) {
	ac := NewAccessController()

	// Create valid role
	role := &Role{
		Name:        "custom",
		Description: "Custom role",
		Permissions: []Permission{PermRead, PermWrite},
	}
	err := ac.CreateRole(role)
	if err != nil {
		t.Fatalf("CreateRole() failed: %v", err)
	}

	got := ac.GetRole("custom")
	if got == nil {
		t.Error("Role should be created")
	}
	if got.IsBuiltin {
		t.Error("Custom role should not be builtin")
	}

	// Missing name
	err = ac.CreateRole(&Role{})
	if err != ErrNameRequired {
		t.Errorf("CreateRole() missing name error = %v, want %v", err, ErrNameRequired)
	}

	// Cannot modify builtin
	err = ac.CreateRole(&Role{Name: "reader"})
	if err != ErrCannotModifyBuiltin {
		t.Errorf("CreateRole() builtin error = %v, want %v", err, ErrCannotModifyBuiltin)
	}
}

func TestAccessController_GetRole(t *testing.T) {
	ac := NewAccessController()

	// Get builtin role
	reader := ac.GetRole("reader")
	if reader == nil {
		t.Error("GetRole() should return builtin reader role")
	}
	if !reader.IsBuiltin {
		t.Error("Reader role should be builtin")
	}

	// Get non-existent
	if ac.GetRole("nonexistent") != nil {
		t.Error("GetRole() should return nil for non-existent role")
	}
}

func TestAccessController_ListRoles(t *testing.T) {
	ac := NewAccessController()

	// Should have 3 builtin roles
	roles := ac.ListRoles()
	if len(roles) != 3 {
		t.Errorf("ListRoles() count = %d, want 3", len(roles))
	}

	// Add custom role
	ac.CreateRole(&Role{Name: "custom", Permissions: []Permission{PermRead}})

	roles = ac.ListRoles()
	if len(roles) != 4 {
		t.Errorf("ListRoles() count = %d, want 4", len(roles))
	}
}

func TestAccessController_DeleteRole(t *testing.T) {
	ac := NewAccessController()

	// Delete non-existent
	err := ac.DeleteRole("nonexistent")
	if err != ErrRoleNotFound {
		t.Errorf("DeleteRole() error = %v, want %v", err, ErrRoleNotFound)
	}

	// Cannot delete builtin
	err = ac.DeleteRole("reader")
	if err != ErrCannotModifyBuiltin {
		t.Errorf("DeleteRole() builtin error = %v, want %v", err, ErrCannotModifyBuiltin)
	}

	// Delete custom role
	ac.CreateRole(&Role{Name: "custom", Permissions: []Permission{PermRead}})
	err = ac.DeleteRole("custom")
	if err != nil {
		t.Fatalf("DeleteRole() failed: %v", err)
	}
	if ac.GetRole("custom") != nil {
		t.Error("Role should be deleted")
	}
}

func TestAccessController_LogAudit(t *testing.T) {
	ac := NewAccessController()

	log := AuditLog{
		Tenant:   "tenant1",
		UserID:   "user1",
		Action:   "GET",
		Resource: "/v1/features",
		Success:  true,
	}

	ac.LogAudit(log)

	logs := ac.GetAuditLogs("", time.Time{}, 100)
	if len(logs) != 1 {
		t.Errorf("GetAuditLogs() count = %d, want 1", len(logs))
	}
	if logs[0].ID == "" {
		t.Error("Audit log should have ID")
	}
	if logs[0].Timestamp.IsZero() {
		t.Error("Audit log should have timestamp")
	}
}

func TestAccessController_GetAuditLogs_Filtering(t *testing.T) {
	ac := NewAccessController()

	// Add multiple logs
	for i := 0; i < 5; i++ {
		ac.LogAudit(AuditLog{
			Tenant:  "tenant1",
			Action:  "GET",
			Success: true,
		})
	}
	for i := 0; i < 3; i++ {
		ac.LogAudit(AuditLog{
			Tenant:  "tenant2",
			Action:  "POST",
			Success: true,
		})
	}

	// Filter by tenant
	t1Logs := ac.GetAuditLogs("tenant1", time.Time{}, 100)
	if len(t1Logs) != 5 {
		t.Errorf("GetAuditLogs(tenant1) count = %d, want 5", len(t1Logs))
	}

	// Filter by time
	time.Sleep(10 * time.Millisecond)
	since := time.Now()
	ac.LogAudit(AuditLog{Tenant: "tenant1", Action: "DELETE", Success: true})

	sinceLogs := ac.GetAuditLogs("", since, 100)
	if len(sinceLogs) != 1 {
		t.Errorf("GetAuditLogs(since) count = %d, want 1", len(sinceLogs))
	}

	// Limit
	limitedLogs := ac.GetAuditLogs("", time.Time{}, 3)
	if len(limitedLogs) != 3 {
		t.Errorf("GetAuditLogs(limit=3) count = %d, want 3", len(limitedLogs))
	}
}

func TestAccessController_AuditLogs_MaxLimit(t *testing.T) {
	ac := NewAccessController()
	ac.maxAuditLogs = 10 // Set low limit for testing

	// Add more than max
	for i := 0; i < 15; i++ {
		ac.LogAudit(AuditLog{
			Tenant:  "tenant1",
			Action:  "GET",
			Success: true,
		})
	}

	logs := ac.GetAuditLogs("", time.Time{}, 100)
	if len(logs) != 10 {
		t.Errorf("GetAuditLogs() should be trimmed to %d, got %d", 10, len(logs))
	}
}

func TestContextHelpers(t *testing.T) {
	ctx := context.Background()

	// Test APIKey context
	key := &APIKey{ID: "key1", Name: "Test Key"}
	ctx = WithAPIKey(ctx, key)

	gotKey := APIKeyFromContext(ctx)
	if gotKey == nil || gotKey.ID != "key1" {
		t.Error("APIKeyFromContext() failed")
	}

	// Test Tenant context
	ctx = WithTenant(ctx, "tenant1")
	gotTenant := TenantFromContext(ctx)
	if gotTenant != "tenant1" {
		t.Errorf("TenantFromContext() = %v, want tenant1", gotTenant)
	}

	// Empty context
	emptyCtx := context.Background()
	if APIKeyFromContext(emptyCtx) != nil {
		t.Error("APIKeyFromContext() should return nil for empty context")
	}
	if TenantFromContext(emptyCtx) != "" {
		t.Error("TenantFromContext() should return empty string for empty context")
	}
}

func TestAccessController_Concurrency(t *testing.T) {
	ac := NewAccessController()
	ac.CreateTenant(&Tenant{ID: "tenant1", Name: "Test"})

	var wg sync.WaitGroup
	numGoroutines := 50
	numOperations := 20

	// Concurrent API key creation
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				rawKey, _ := ac.CreateAPIKey(&APIKey{
					Name:   "key",
					Tenant: "tenant1",
				}, "admin")
				if rawKey != "" {
					ac.ValidateAPIKey(rawKey)
				}
			}
		}(i)
	}

	// Concurrent reads
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				ac.ListAPIKeys("tenant1")
				ac.ListTenants()
				ac.ListRoles()
				ac.GetAuditLogs("", time.Time{}, 10)
			}
		}()
	}

	// Concurrent audit logging
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				ac.LogAudit(AuditLog{
					Tenant:  "tenant1",
					Action:  "GET",
					Success: true,
				})
			}
		}()
	}

	wg.Wait()
}

func TestBuiltinRoles(t *testing.T) {
	// Verify RoleReader
	if len(RoleReader.Permissions) != 1 || RoleReader.Permissions[0] != PermRead {
		t.Error("RoleReader should only have read permission")
	}
	if !RoleReader.IsBuiltin {
		t.Error("RoleReader should be builtin")
	}

	// Verify RoleWriter
	if len(RoleWriter.Permissions) != 2 {
		t.Error("RoleWriter should have read and write permissions")
	}
	if !RoleWriter.IsBuiltin {
		t.Error("RoleWriter should be builtin")
	}

	// Verify RoleAdmin
	if len(RoleAdmin.Permissions) != 5 {
		t.Error("RoleAdmin should have all permissions")
	}
	if !RoleAdmin.IsBuiltin {
		t.Error("RoleAdmin should be builtin")
	}
}

func TestAPIKey_GeneratedFields(t *testing.T) {
	ac := NewAccessController()
	ac.CreateTenant(&Tenant{ID: "tenant1", Name: "Test"})

	rawKey, err := ac.CreateAPIKey(&APIKey{
		Name:   "test-key",
		Tenant: "tenant1",
		Roles:  []string{"reader"},
	}, "creator")

	if err != nil {
		t.Fatalf("CreateAPIKey failed: %v", err)
	}

	// Verify raw key format
	if len(rawKey) < 10 {
		t.Error("Raw key should be at least 10 characters")
	}
	if rawKey[:3] != "fk_" {
		t.Error("Raw key should start with 'fk_' prefix")
	}

	// Verify key info
	keyInfo, _ := ac.ValidateAPIKey(rawKey)
	if keyInfo.ID == "" {
		t.Error("Key should have ID")
	}
	if keyInfo.Prefix != rawKey[:8] {
		t.Errorf("Key prefix should match first 8 chars: got %s, want %s", keyInfo.Prefix, rawKey[:8])
	}
	if keyInfo.CreatedBy != "creator" {
		t.Errorf("CreatedBy = %v, want creator", keyInfo.CreatedBy)
	}
	if keyInfo.CreatedAt.IsZero() {
		t.Error("CreatedAt should be set")
	}
}
