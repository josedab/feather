package server

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/feather-store/feather/internal/auth"
)

// AuthHandler handles authentication and authorization API requests.
type AuthHandler struct {
	controller *auth.AccessController
	middleware *auth.Middleware
}

// NewAuthHandler creates a new auth handler.
func NewAuthHandler() *AuthHandler {
	controller := auth.NewAccessController()
	return &AuthHandler{
		controller: controller,
		middleware: auth.NewMiddleware(controller),
	}
}

// RegisterRoutes registers auth API routes.
func (h *AuthHandler) RegisterRoutes(mux *http.ServeMux) {
	// API Keys
	mux.HandleFunc("POST /v1/auth/keys", h.handleCreateAPIKey)
	mux.HandleFunc("GET /v1/auth/keys", h.handleListAPIKeys)
	mux.HandleFunc("GET /v1/auth/keys/{id}", h.handleGetAPIKey)
	mux.HandleFunc("DELETE /v1/auth/keys/{id}", h.handleDeleteAPIKey)
	mux.HandleFunc("POST /v1/auth/keys/{id}/revoke", h.handleRevokeAPIKey)
	mux.HandleFunc("POST /v1/auth/validate", h.handleValidateAPIKey)

	// Tenants
	mux.HandleFunc("POST /v1/auth/tenants", h.handleCreateTenant)
	mux.HandleFunc("GET /v1/auth/tenants", h.handleListTenants)
	mux.HandleFunc("GET /v1/auth/tenants/{id}", h.handleGetTenant)
	mux.HandleFunc("PUT /v1/auth/tenants/{id}", h.handleUpdateTenant)
	mux.HandleFunc("DELETE /v1/auth/tenants/{id}", h.handleDeleteTenant)

	// Roles
	mux.HandleFunc("POST /v1/auth/roles", h.handleCreateRole)
	mux.HandleFunc("GET /v1/auth/roles", h.handleListRoles)
	mux.HandleFunc("GET /v1/auth/roles/{name}", h.handleGetRole)
	mux.HandleFunc("DELETE /v1/auth/roles/{name}", h.handleDeleteRole)

	// Audit logs
	mux.HandleFunc("GET /v1/auth/audit", h.handleGetAuditLogs)
}

// GetController returns the access controller for integration.
func (h *AuthHandler) GetController() *auth.AccessController {
	return h.controller
}

// GetMiddleware returns the auth middleware for use in other handlers.
func (h *AuthHandler) GetMiddleware() *auth.Middleware {
	return h.middleware
}

// CreateAPIKeyRequest represents a request to create an API key.
type CreateAPIKeyRequest struct {
	Name        string            `json:"name"`
	Tenant      string            `json:"tenant"`
	Roles       []string          `json:"roles"`
	Permissions []string          `json:"permissions"`
	Namespaces  []string          `json:"namespaces"`
	Features    []string          `json:"features"`
	RateLimit   int               `json:"rate_limit"`
	ExpiresIn   string            `json:"expires_in,omitempty"` // Duration string like "30d", "1y"
	Metadata    map[string]string `json:"metadata"`
}

// handleCreateAPIKey handles POST /v1/auth/keys
func (h *AuthHandler) handleCreateAPIKey(w http.ResponseWriter, r *http.Request) {
	var req CreateAPIKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name == "" || req.Tenant == "" {
		h.writeError(w, http.StatusBadRequest, "name and tenant are required")
		return
	}

	createdBy := r.Header.Get("X-User-ID")
	if createdBy == "" {
		createdBy = "anonymous"
	}

	// Parse expiration
	var expiresAt *time.Time
	if req.ExpiresIn != "" {
		duration, err := parseDuration(req.ExpiresIn)
		if err != nil {
			h.writeError(w, http.StatusBadRequest, "invalid expires_in format")
			return
		}
		t := time.Now().Add(duration)
		expiresAt = &t
	}

	// Convert permissions
	permissions := make([]auth.Permission, len(req.Permissions))
	for i, p := range req.Permissions {
		permissions[i] = auth.Permission(p)
	}

	key := &auth.APIKey{
		Name:        req.Name,
		Tenant:      req.Tenant,
		Roles:       req.Roles,
		Permissions: permissions,
		Namespaces:  req.Namespaces,
		Features:    req.Features,
		RateLimit:   req.RateLimit,
		ExpiresAt:   expiresAt,
		Metadata:    req.Metadata,
	}

	rawKey, err := h.controller.CreateAPIKey(key, createdBy)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	h.writeJSON(w, http.StatusCreated, map[string]interface{}{
		"success": true,
		"key":     rawKey, // Only time the raw key is returned
		"id":      key.ID,
		"prefix":  key.Prefix,
	})
}

func parseDuration(s string) (time.Duration, error) {
	// Handle custom duration formats like "30d", "1y"
	if len(s) == 0 {
		return 0, nil
	}

	unit := s[len(s)-1]
	value := s[:len(s)-1]

	var multiplier time.Duration
	switch unit {
	case 'd':
		multiplier = 24 * time.Hour
	case 'w':
		multiplier = 7 * 24 * time.Hour
	case 'y':
		multiplier = 365 * 24 * time.Hour
	default:
		// Fall back to standard parsing
		return time.ParseDuration(s)
	}

	var num int
	for _, c := range value {
		if c >= '0' && c <= '9' {
			num = num*10 + int(c-'0')
		} else {
			return time.ParseDuration(s)
		}
	}

	return time.Duration(num) * multiplier, nil
}

// handleListAPIKeys handles GET /v1/auth/keys
func (h *AuthHandler) handleListAPIKeys(w http.ResponseWriter, r *http.Request) {
	tenant := r.URL.Query().Get("tenant")
	keys := h.controller.ListAPIKeys(tenant)

	h.writeJSON(w, http.StatusOK, map[string]interface{}{
		"keys":  keys,
		"count": len(keys),
	})
}

// handleGetAPIKey handles GET /v1/auth/keys/{id}
func (h *AuthHandler) handleGetAPIKey(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		h.writeError(w, http.StatusBadRequest, "key id required")
		return
	}

	key := h.controller.GetAPIKey(id)
	if key == nil {
		h.writeError(w, http.StatusNotFound, "API key not found")
		return
	}

	h.writeJSON(w, http.StatusOK, key)
}

// handleDeleteAPIKey handles DELETE /v1/auth/keys/{id}
func (h *AuthHandler) handleDeleteAPIKey(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		h.writeError(w, http.StatusBadRequest, "key id required")
		return
	}

	if err := h.controller.DeleteAPIKey(id); err != nil {
		h.writeError(w, http.StatusNotFound, err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"id":      id,
	})
}

// handleRevokeAPIKey handles POST /v1/auth/keys/{id}/revoke
func (h *AuthHandler) handleRevokeAPIKey(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		h.writeError(w, http.StatusBadRequest, "key id required")
		return
	}

	if err := h.controller.RevokeAPIKey(id); err != nil {
		h.writeError(w, http.StatusNotFound, err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"id":      id,
		"status":  "revoked",
	})
}

// ValidateKeyRequest represents a key validation request.
type ValidateKeyRequest struct {
	Key string `json:"key"`
}

// handleValidateAPIKey handles POST /v1/auth/validate
func (h *AuthHandler) handleValidateAPIKey(w http.ResponseWriter, r *http.Request) {
	var req ValidateKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	key, err := h.controller.ValidateAPIKey(req.Key)
	if err != nil {
		h.writeJSON(w, http.StatusUnauthorized, map[string]interface{}{
			"valid": false,
			"error": err.Error(),
		})
		return
	}

	h.writeJSON(w, http.StatusOK, map[string]interface{}{
		"valid":  true,
		"id":     key.ID,
		"tenant": key.Tenant,
		"roles":  key.Roles,
	})
}

// CreateTenantRequest represents a request to create a tenant.
type CreateTenantRequest struct {
	ID          string             `json:"id"`
	Name        string             `json:"name"`
	Description string             `json:"description"`
	Quotas      *auth.TenantQuotas `json:"quotas"`
	Metadata    map[string]string  `json:"metadata"`
}

// handleCreateTenant handles POST /v1/auth/tenants
func (h *AuthHandler) handleCreateTenant(w http.ResponseWriter, r *http.Request) {
	var req CreateTenantRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.ID == "" || req.Name == "" {
		h.writeError(w, http.StatusBadRequest, "id and name are required")
		return
	}

	tenant := &auth.Tenant{
		ID:          req.ID,
		Name:        req.Name,
		Description: req.Description,
		Metadata:    req.Metadata,
	}

	if req.Quotas != nil {
		tenant.Quotas = *req.Quotas
	}

	if err := h.controller.CreateTenant(tenant); err != nil {
		h.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	h.writeJSON(w, http.StatusCreated, map[string]interface{}{
		"success": true,
		"tenant":  tenant,
	})
}

// handleListTenants handles GET /v1/auth/tenants
func (h *AuthHandler) handleListTenants(w http.ResponseWriter, r *http.Request) {
	tenants := h.controller.ListTenants()

	h.writeJSON(w, http.StatusOK, map[string]interface{}{
		"tenants": tenants,
		"count":   len(tenants),
	})
}

// handleGetTenant handles GET /v1/auth/tenants/{id}
func (h *AuthHandler) handleGetTenant(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		h.writeError(w, http.StatusBadRequest, "tenant id required")
		return
	}

	tenant := h.controller.GetTenant(id)
	if tenant == nil {
		h.writeError(w, http.StatusNotFound, "tenant not found")
		return
	}

	h.writeJSON(w, http.StatusOK, tenant)
}

// handleUpdateTenant handles PUT /v1/auth/tenants/{id}
func (h *AuthHandler) handleUpdateTenant(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		h.writeError(w, http.StatusBadRequest, "tenant id required")
		return
	}

	var tenant auth.Tenant
	if err := json.NewDecoder(r.Body).Decode(&tenant); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	tenant.ID = id

	if err := h.controller.UpdateTenant(&tenant); err != nil {
		h.writeError(w, http.StatusNotFound, err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"tenant":  tenant,
	})
}

// handleDeleteTenant handles DELETE /v1/auth/tenants/{id}
func (h *AuthHandler) handleDeleteTenant(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		h.writeError(w, http.StatusBadRequest, "tenant id required")
		return
	}

	if err := h.controller.DeleteTenant(id); err != nil {
		h.writeError(w, http.StatusNotFound, err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"id":      id,
	})
}

// CreateRoleRequest represents a request to create a role.
type CreateRoleRequest struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Permissions []string `json:"permissions"`
}

// handleCreateRole handles POST /v1/auth/roles
func (h *AuthHandler) handleCreateRole(w http.ResponseWriter, r *http.Request) {
	var req CreateRoleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name == "" {
		h.writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	permissions := make([]auth.Permission, len(req.Permissions))
	for i, p := range req.Permissions {
		permissions[i] = auth.Permission(p)
	}

	role := &auth.Role{
		Name:        req.Name,
		Description: req.Description,
		Permissions: permissions,
	}

	if err := h.controller.CreateRole(role); err != nil {
		h.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	h.writeJSON(w, http.StatusCreated, map[string]interface{}{
		"success": true,
		"role":    role,
	})
}

// handleListRoles handles GET /v1/auth/roles
func (h *AuthHandler) handleListRoles(w http.ResponseWriter, r *http.Request) {
	roles := h.controller.ListRoles()

	h.writeJSON(w, http.StatusOK, map[string]interface{}{
		"roles": roles,
		"count": len(roles),
	})
}

// handleGetRole handles GET /v1/auth/roles/{name}
func (h *AuthHandler) handleGetRole(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		h.writeError(w, http.StatusBadRequest, "role name required")
		return
	}

	role := h.controller.GetRole(name)
	if role == nil {
		h.writeError(w, http.StatusNotFound, "role not found")
		return
	}

	h.writeJSON(w, http.StatusOK, role)
}

// handleDeleteRole handles DELETE /v1/auth/roles/{name}
func (h *AuthHandler) handleDeleteRole(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		h.writeError(w, http.StatusBadRequest, "role name required")
		return
	}

	if err := h.controller.DeleteRole(name); err != nil {
		h.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"name":    name,
	})
}

// handleGetAuditLogs handles GET /v1/auth/audit
func (h *AuthHandler) handleGetAuditLogs(w http.ResponseWriter, r *http.Request) {
	tenant := r.URL.Query().Get("tenant")
	limit := 100

	since := time.Now().Add(-24 * time.Hour)
	if sinceStr := r.URL.Query().Get("since"); sinceStr != "" {
		if parsed, err := time.Parse(time.RFC3339, sinceStr); err == nil {
			since = parsed
		}
	}

	logs := h.controller.GetAuditLogs(tenant, since, limit)

	h.writeJSON(w, http.StatusOK, map[string]interface{}{
		"logs":  logs,
		"count": len(logs),
	})
}

func (h *AuthHandler) writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (h *AuthHandler) writeError(w http.ResponseWriter, status int, message string) {
	h.writeJSON(w, status, map[string]string{"error": message})
}
