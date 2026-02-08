package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/feather-store/feather/internal/platform/tenant"
)

// TenantHandler handles multi-tenant management API requests.
type TenantHandler struct {
	registry      *tenant.TenantRegistry
	hotTier       *tenant.PartitionedHotTier
	middleware    *tenant.Middleware
	priorityQueue *tenant.PriorityQueue
}

// NewTenantHandler creates a new tenant handler.
func NewTenantHandler(totalHotTierSize int64) *TenantHandler {
	registry := tenant.NewTenantRegistry()
	hotTier := tenant.NewPartitionedHotTier(totalHotTierSize, registry)
	middleware := tenant.NewTenantMiddleware(registry)
	priorityQueue := tenant.NewPriorityQueue(registry, 10)

	return &TenantHandler{
		registry:      registry,
		hotTier:       hotTier,
		middleware:    middleware,
		priorityQueue: priorityQueue,
	}
}

// NewTenantHandlerWithRegistry creates a tenant handler with an existing registry.
func NewTenantHandlerWithRegistry(registry *tenant.TenantRegistry, totalHotTierSize int64) *TenantHandler {
	hotTier := tenant.NewPartitionedHotTier(totalHotTierSize, registry)
	middleware := tenant.NewTenantMiddleware(registry)
	priorityQueue := tenant.NewPriorityQueue(registry, 10)

	return &TenantHandler{
		registry:      registry,
		hotTier:       hotTier,
		middleware:    middleware,
		priorityQueue: priorityQueue,
	}
}

// RegisterRoutes registers tenant management API routes.
func (h *TenantHandler) RegisterRoutes(mux *http.ServeMux) {
	// Tenant CRUD routes
	mux.HandleFunc("GET /v1/tenants", h.handleListTenants)
	mux.HandleFunc("POST /v1/tenants", h.handleCreateTenant)
	mux.HandleFunc("GET /v1/tenants/{id}", h.handleGetTenant)
	mux.HandleFunc("PUT /v1/tenants/{id}", h.handleUpdateTenant)
	mux.HandleFunc("DELETE /v1/tenants/{id}", h.handleDeleteTenant)

	// Tenant status routes
	mux.HandleFunc("POST /v1/tenants/{id}/enable", h.handleEnableTenant)
	mux.HandleFunc("POST /v1/tenants/{id}/disable", h.handleDisableTenant)

	// Quota management routes
	mux.HandleFunc("GET /v1/tenants/{id}/quotas", h.handleGetQuotas)
	mux.HandleFunc("PUT /v1/tenants/{id}/quotas", h.handleUpdateQuotas)

	// Usage and metrics routes
	mux.HandleFunc("GET /v1/tenants/{id}/usage", h.handleGetUsage)
	mux.HandleFunc("GET /v1/tenants/{id}/metrics", h.handleGetMetrics)
	mux.HandleFunc("POST /v1/tenants/{id}/metrics/reset", h.handleResetMetrics)

	// Partition management routes
	mux.HandleFunc("GET /v1/tenants/{id}/partition", h.handleGetPartition)
	mux.HandleFunc("PUT /v1/tenants/{id}/partition/resize", h.handleResizePartition)

	// Global stats routes
	mux.HandleFunc("GET /v1/tenants/stats", h.handleGlobalStats)
	mux.HandleFunc("GET /v1/tenants/partitions", h.handleListPartitions)

	// Cross-tenant sharing routes
	mux.HandleFunc("GET /v1/tenants/{id}/shares", h.handleListShares)
	mux.HandleFunc("POST /v1/tenants/{id}/shares", h.handleGrantShare)
	mux.HandleFunc("DELETE /v1/tenants/shares/{grantId}", h.handleRevokeShare)

	// Audit log routes
	mux.HandleFunc("GET /v1/tenants/{id}/audit", h.handleGetAuditLog)
}

// Registry returns the tenant registry for middleware integration.
func (h *TenantHandler) Registry() *tenant.TenantRegistry {
	return h.registry
}

// Middleware returns the tenant middleware.
func (h *TenantHandler) Middleware() *tenant.Middleware {
	return h.middleware
}

// HotTier returns the partitioned hot tier.
func (h *TenantHandler) HotTier() *tenant.PartitionedHotTier {
	return h.hotTier
}

// PriorityQueue returns the priority queue.
func (h *TenantHandler) PriorityQueue() *tenant.PriorityQueue {
	return h.priorityQueue
}

// Request/Response types

// TenantCreateRequest represents a tenant creation request for multi-tenant management.
type TenantCreateRequest struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Tier        string                 `json:"tier,omitempty"`
	Quotas      *tenant.TenantQuotas   `json:"quotas,omitempty"`
	Settings    *tenant.TenantSettings `json:"settings,omitempty"`
	Metadata    map[string]string      `json:"metadata,omitempty"`
}

// UpdateTenantRequest represents a tenant update request.
type UpdateTenantRequest struct {
	Name        string                 `json:"name,omitempty"`
	Description string                 `json:"description,omitempty"`
	Tier        string                 `json:"tier,omitempty"`
	Settings    *tenant.TenantSettings `json:"settings,omitempty"`
	Metadata    map[string]string      `json:"metadata,omitempty"`
}

// UpdateQuotasRequest represents a quota update request.
type UpdateQuotasRequest struct {
	MaxFeatures           int   `json:"max_features,omitempty"`
	MaxRequestsPerSecond  int   `json:"max_requests_per_second,omitempty"`
	MaxStorageBytes       int64 `json:"max_storage_bytes,omitempty"`
	MaxHotTierBytes       int64 `json:"max_hot_tier_bytes,omitempty"`
	MaxConcurrentRequests int   `json:"max_concurrent_requests,omitempty"`
	MaxEntities           int64 `json:"max_entities,omitempty"`
	BurstMultiplier       int   `json:"burst_multiplier,omitempty"`
}

// ResizePartitionRequest represents a partition resize request.
type ResizePartitionRequest struct {
	MaxSize int64 `json:"max_size"`
}

// handleListTenants handles GET /v1/tenants
func (h *TenantHandler) handleListTenants(w http.ResponseWriter, r *http.Request) {
	// Parse query params
	tierFilter := r.URL.Query().Get("tier")
	enabledFilter := r.URL.Query().Get("enabled")

	tenants := h.registry.ListTenants()

	result := make([]map[string]interface{}, 0, len(tenants))
	for _, t := range tenants {
		// Apply filters
		if tierFilter != "" && string(t.Tier) != tierFilter {
			continue
		}
		if enabledFilter != "" {
			enabled, err := strconv.ParseBool(enabledFilter)
			if err == nil && t.Enabled != enabled {
				continue
			}
		}

		result = append(result, map[string]interface{}{
			"id":          t.ID,
			"name":        t.Name,
			"description": t.Description,
			"tier":        t.Tier,
			"enabled":     t.Enabled,
			"created_at":  t.CreatedAt,
			"updated_at":  t.UpdatedAt,
		})
	}

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"tenants": result,
		"count":   len(result),
	})
}

// handleCreateTenant handles POST /v1/tenants
func (h *TenantHandler) handleCreateTenant(w http.ResponseWriter, r *http.Request) {
	var req TenantCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.ID == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "id is required")
		return
	}
	if req.Name == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "name is required")
		return
	}

	// Parse tier
	tier := tenant.TierStandard
	if req.Tier != "" {
		switch req.Tier {
		case "free":
			tier = tenant.TierFree
		case "standard":
			tier = tenant.TierStandard
		case "premium":
			tier = tenant.TierPremium
		case "enterprise":
			tier = tenant.TierEnterprise
		default:
			h.writeError(r.Context(), w, http.StatusBadRequest, "invalid tier")
			return
		}
	}

	t := &tenant.Tenant{
		ID:          req.ID,
		Name:        req.Name,
		Description: req.Description,
		Tier:        tier,
		Metadata:    req.Metadata,
	}

	// Apply custom quotas if provided
	if req.Quotas != nil {
		t.Quotas = *req.Quotas
	}

	// Apply custom settings if provided
	if req.Settings != nil {
		t.Settings = *req.Settings
	}

	if err := h.registry.CreateTenant(t); err != nil {
		h.writeError(r.Context(), w, http.StatusConflict, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusCreated, map[string]interface{}{
		"success": true,
		"tenant":  t,
	})
}

// handleGetTenant handles GET /v1/tenants/{id}
func (h *TenantHandler) handleGetTenant(w http.ResponseWriter, r *http.Request) {
	tenantID := r.PathValue("id")
	if tenantID == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "tenant id required")
		return
	}

	t, err := h.registry.GetTenant(tenantID)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, t)
}

// handleUpdateTenant handles PUT /v1/tenants/{id}
func (h *TenantHandler) handleUpdateTenant(w http.ResponseWriter, r *http.Request) {
	tenantID := r.PathValue("id")
	if tenantID == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "tenant id required")
		return
	}

	var req UpdateTenantRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	t, err := h.registry.GetTenant(tenantID)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}

	// Apply updates
	if req.Name != "" {
		t.Name = req.Name
	}
	if req.Description != "" {
		t.Description = req.Description
	}
	if req.Tier != "" {
		switch req.Tier {
		case "free":
			t.Tier = tenant.TierFree
		case "standard":
			t.Tier = tenant.TierStandard
		case "premium":
			t.Tier = tenant.TierPremium
		case "enterprise":
			t.Tier = tenant.TierEnterprise
		default:
			h.writeError(r.Context(), w, http.StatusBadRequest, "invalid tier")
			return
		}
	}
	if req.Settings != nil {
		t.Settings = *req.Settings
	}
	if req.Metadata != nil {
		t.Metadata = req.Metadata
	}
	t.UpdatedAt = time.Now()

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success": true,
		"tenant":  t,
	})
}

// handleDeleteTenant handles DELETE /v1/tenants/{id}
func (h *TenantHandler) handleDeleteTenant(w http.ResponseWriter, r *http.Request) {
	tenantID := r.PathValue("id")
	if tenantID == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "tenant id required")
		return
	}

	if err := h.registry.DeleteTenant(tenantID); err != nil {
		h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}

	// Also delete the partition if it exists
	if err := h.hotTier.DeletePartition(tenantID); err != nil && !errors.Is(err, tenant.ErrPartitionNotFound) {
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success": true,
	})
}

// handleEnableTenant handles POST /v1/tenants/{id}/enable
func (h *TenantHandler) handleEnableTenant(w http.ResponseWriter, r *http.Request) {
	tenantID := r.PathValue("id")
	if tenantID == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "tenant id required")
		return
	}

	if err := h.registry.EnableTenant(tenantID); err != nil {
		h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success": true,
		"enabled": true,
	})
}

// handleDisableTenant handles POST /v1/tenants/{id}/disable
func (h *TenantHandler) handleDisableTenant(w http.ResponseWriter, r *http.Request) {
	tenantID := r.PathValue("id")
	if tenantID == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "tenant id required")
		return
	}

	if err := h.registry.DisableTenant(tenantID); err != nil {
		h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success": true,
		"enabled": false,
	})
}

// handleGetQuotas handles GET /v1/tenants/{id}/quotas
func (h *TenantHandler) handleGetQuotas(w http.ResponseWriter, r *http.Request) {
	tenantID := r.PathValue("id")
	if tenantID == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "tenant id required")
		return
	}

	t, err := h.registry.GetTenant(tenantID)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"tenant_id": tenantID,
		"quotas":    t.Quotas,
	})
}

// handleUpdateQuotas handles PUT /v1/tenants/{id}/quotas
func (h *TenantHandler) handleUpdateQuotas(w http.ResponseWriter, r *http.Request) {
	tenantID := r.PathValue("id")
	if tenantID == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "tenant id required")
		return
	}

	var req UpdateQuotasRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	t, err := h.registry.GetTenant(tenantID)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}

	// Apply quota updates
	if req.MaxFeatures > 0 {
		t.Quotas.MaxFeatures = req.MaxFeatures
	}
	if req.MaxRequestsPerSecond > 0 {
		t.Quotas.MaxRequestsPerSecond = req.MaxRequestsPerSecond
	}
	if req.MaxStorageBytes > 0 {
		t.Quotas.MaxStorageBytes = req.MaxStorageBytes
	}
	if req.MaxHotTierBytes > 0 {
		t.Quotas.MaxHotTierBytes = req.MaxHotTierBytes
		// Also resize the partition
		if err := h.hotTier.ResizePartition(tenantID, req.MaxHotTierBytes); err != nil {
			h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	if req.MaxConcurrentRequests > 0 {
		t.Quotas.MaxConcurrentRequests = req.MaxConcurrentRequests
	}
	if req.MaxEntities > 0 {
		t.Quotas.MaxEntityCount = req.MaxEntities
	}
	if req.BurstMultiplier > 0 {
		t.Settings.BurstMultiplier = float64(req.BurstMultiplier)
	}
	t.UpdatedAt = time.Now()

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success":   true,
		"tenant_id": tenantID,
		"quotas":    t.Quotas,
	})
}

// handleGetUsage handles GET /v1/tenants/{id}/usage
func (h *TenantHandler) handleGetUsage(w http.ResponseWriter, r *http.Request) {
	tenantID := r.PathValue("id")
	if tenantID == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "tenant id required")
		return
	}

	usage, err := h.registry.GetUsage(tenantID)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}

	// Also include quota for comparison
	t, _ := h.registry.GetTenant(tenantID)

	response := map[string]interface{}{
		"tenant_id": tenantID,
		"usage":     usage,
	}

	if t != nil {
		response["quotas"] = t.Quotas
		response["utilization"] = map[string]float64{
			"features_pct": float64(usage.FeatureCount) / float64(t.Quotas.MaxFeatures) * 100,
			"storage_pct":  float64(usage.StorageBytes) / float64(t.Quotas.MaxStorageBytes) * 100,
			"entities_pct": float64(usage.EntityCount) / float64(t.Quotas.MaxEntityCount) * 100,
		}
	}

	h.writeJSON(r.Context(), w, http.StatusOK, response)
}

// handleGetMetrics handles GET /v1/tenants/{id}/metrics
func (h *TenantHandler) handleGetMetrics(w http.ResponseWriter, r *http.Request) {
	tenantID := r.PathValue("id")
	if tenantID == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "tenant id required")
		return
	}

	metrics, err := h.registry.GetMetrics(tenantID)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}

	// Calculate average latency
	var avgLatency float64
	if metrics.RequestCount > 0 {
		avgLatency = float64(metrics.TotalLatencyNs) / float64(metrics.RequestCount) / 1e6 // ms
	}

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"tenant_id":      tenantID,
		"total_requests": metrics.RequestCount,
		"total_errors":   metrics.ErrorCount,
		"error_rate":     float64(metrics.ErrorCount) / float64(max(metrics.RequestCount, 1)) * 100,
		"avg_latency_ms": avgLatency,
		"p50_latency_ns": metrics.P50LatencyNs,
		"p99_latency_ns": metrics.P99LatencyNs,
		"rate_limited":   metrics.RateLimitedCount,
		"quota_exceeded": metrics.QuotaExceededCount,
		"last_reset":     metrics.LastReset,
	})
}

// handleResetMetrics handles POST /v1/tenants/{id}/metrics/reset
func (h *TenantHandler) handleResetMetrics(w http.ResponseWriter, r *http.Request) {
	tenantID := r.PathValue("id")
	if tenantID == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "tenant id required")
		return
	}

	// Verify tenant exists
	if _, err := h.registry.GetTenant(tenantID); err != nil {
		h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}

	// Note: Metric reset is not currently supported in the registry
	// This endpoint exists for future implementation
	h.writeJSON(r.Context(), w, http.StatusNotImplemented, map[string]interface{}{
		"success": false,
		"message": "metrics reset not yet implemented",
	})
}

// handleGetPartition handles GET /v1/tenants/{id}/partition
func (h *TenantHandler) handleGetPartition(w http.ResponseWriter, r *http.Request) {
	tenantID := r.PathValue("id")
	if tenantID == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "tenant id required")
		return
	}

	// Get partition stats from hot tier
	stats := h.hotTier.PartitionStats()

	var partitionStats map[string]interface{}
	for _, s := range stats {
		if s["tenant_id"] == tenantID {
			partitionStats = s
			break
		}
	}

	if partitionStats == nil {
		// Return empty partition info
		partitionStats = map[string]interface{}{
			"tenant_id":    tenantID,
			"current_size": int64(0),
			"max_size":     int64(0),
			"entity_count": int64(0),
			"utilization":  0.0,
		}
	}

	h.writeJSON(r.Context(), w, http.StatusOK, partitionStats)
}

// handleResizePartition handles PUT /v1/tenants/{id}/partition/resize
func (h *TenantHandler) handleResizePartition(w http.ResponseWriter, r *http.Request) {
	tenantID := r.PathValue("id")
	if tenantID == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "tenant id required")
		return
	}

	var req ResizePartitionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.MaxSize <= 0 {
		h.writeError(r.Context(), w, http.StatusBadRequest, "max_size must be positive")
		return
	}

	if err := h.hotTier.ResizePartition(tenantID, req.MaxSize); err != nil {
		h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}

	// Also update tenant quota
	t, err := h.registry.GetTenant(tenantID)
	if err == nil {
		t.Quotas.MaxHotTierBytes = req.MaxSize
		t.UpdatedAt = time.Now()
	}

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success":  true,
		"max_size": req.MaxSize,
	})
}

// handleGlobalStats handles GET /v1/tenants/stats
func (h *TenantHandler) handleGlobalStats(w http.ResponseWriter, r *http.Request) {
	tenants := h.registry.ListTenants()
	hotTierMetrics := h.hotTier.Metrics()

	// Aggregate stats
	byTier := make(map[string]int)
	enabledCount := 0
	disabledCount := 0

	for _, t := range tenants {
		byTier[string(t.Tier)]++
		if t.Enabled {
			enabledCount++
		} else {
			disabledCount++
		}
	}

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"total_tenants":    len(tenants),
		"enabled_tenants":  enabledCount,
		"disabled_tenants": disabledCount,
		"tenants_by_tier":  byTier,
		"hot_tier_metrics": hotTierMetrics,
	})
}

// handleListPartitions handles GET /v1/tenants/partitions
func (h *TenantHandler) handleListPartitions(w http.ResponseWriter, r *http.Request) {
	stats := h.hotTier.PartitionStats()

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"partitions":     stats,
		"count":          len(stats),
		"total_size":     h.hotTier.Size(),
		"total_entities": h.hotTier.EntityCount(),
	})
}

func (h *TenantHandler) writeJSON(ctx context.Context, w http.ResponseWriter, status int, data interface{}) {
	writeJSONResponse(ctx, w, status, data)
}

func (h *TenantHandler) writeError(ctx context.Context, w http.ResponseWriter, status int, message string) {
	writeJSONError(ctx, w, status, message)
}

// GrantShareRequest represents a cross-tenant share grant request.
type GrantShareRequest struct {
	ToTenantID string   `json:"to_tenant_id"`
	Features   []string `json:"features,omitempty"` // empty = all
	Permission string   `json:"permission"`         // "read" or "read_write"
}

func (h *TenantHandler) handleGrantShare(w http.ResponseWriter, r *http.Request) {
	fromTenantID := r.PathValue("id")
	if fromTenantID == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "tenant id required")
		return
	}

	var req GrantShareRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	grantedBy := r.Header.Get("X-User-ID")
	if grantedBy == "" {
		grantedBy = "system"
	}

	grant := &tenant.ShareGrant{
		FromTenantID: fromTenantID,
		ToTenantID:   req.ToTenantID,
		Features:     req.Features,
		Permission:   req.Permission,
		GrantedBy:    grantedBy,
	}

	if err := h.registry.GrantShare(grant); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusCreated, map[string]interface{}{
		"success": true,
		"grant":   grant,
	})
}

func (h *TenantHandler) handleListShares(w http.ResponseWriter, r *http.Request) {
	tenantID := r.PathValue("id")
	if tenantID == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "tenant id required")
		return
	}

	shares := h.registry.ListShares(tenantID)
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"shares": shares,
		"count":  len(shares),
	})
}

func (h *TenantHandler) handleRevokeShare(w http.ResponseWriter, r *http.Request) {
	grantID := r.PathValue("grantId")
	if grantID == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "grant id required")
		return
	}

	if err := h.registry.RevokeShare(grantID); err != nil {
		h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{"success": true})
}

func (h *TenantHandler) handleGetAuditLog(w http.ResponseWriter, r *http.Request) {
	tenantID := r.PathValue("id")
	if tenantID == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "tenant id required")
		return
	}

	limit := 100
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	entries := h.registry.GetAuditLog(tenantID, limit)
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"entries": entries,
		"count":   len(entries),
	})
}
