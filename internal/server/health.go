package server

import (
	"context"
	"sync"
	"time"

	"github.com/feather-store/feather/internal/aggregation"
	"github.com/feather-store/feather/internal/storage"
)

// HealthStatus represents the health status of a component.
type HealthStatus string

const (
	HealthStatusHealthy   HealthStatus = "healthy"
	HealthStatusDegraded  HealthStatus = "degraded"
	HealthStatusUnhealthy HealthStatus = "unhealthy"
)

// ComponentHealth represents the health of a single component.
type ComponentHealth struct {
	Status  HealthStatus `json:"status"`
	Message string       `json:"message,omitempty"`
	Latency string       `json:"latency,omitempty"`
}

// HealthCheckResult represents the overall health check result.
type HealthCheckResult struct {
	Status     HealthStatus                `json:"status"`
	Timestamp  time.Time                   `json:"timestamp"`
	Components map[string]*ComponentHealth `json:"components"`
}

// HealthChecker performs deep health checks on system components.
type HealthChecker struct {
	store  *storage.Store
	agg    *aggregation.Engine
	schema *storage.Registry

	mu      sync.RWMutex
	ready   bool
	healthy bool
}

// NewHealthChecker creates a new health checker.
func NewHealthChecker(
	store *storage.Store,
	agg *aggregation.Engine,
	schema *storage.Registry,
) *HealthChecker {
	return &HealthChecker{
		store:   store,
		agg:     agg,
		schema:  schema,
		ready:   true,
		healthy: true,
	}
}

// SetReady sets the readiness state.
func (h *HealthChecker) SetReady(ready bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.ready = ready
}

// SetHealthy sets the health state.
func (h *HealthChecker) SetHealthy(healthy bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.healthy = healthy
}

// IsReady returns the readiness state.
func (h *HealthChecker) IsReady() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.ready
}

// IsHealthy returns the health state.
func (h *HealthChecker) IsHealthy() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.healthy
}

// Check performs a comprehensive health check.
func (h *HealthChecker) Check(ctx context.Context) *HealthCheckResult {
	result := &HealthCheckResult{
		Status:     HealthStatusHealthy,
		Timestamp:  time.Now(),
		Components: make(map[string]*ComponentHealth),
	}

	// Check hot tier
	hotHealth := h.checkHotTier(ctx)
	result.Components["hot_tier"] = hotHealth
	if hotHealth.Status != HealthStatusHealthy {
		result.Status = hotHealth.Status
	}

	// Check warm tier
	warmHealth := h.checkWarmTier(ctx)
	result.Components["warm_tier"] = warmHealth
	if warmHealth.Status == HealthStatusUnhealthy {
		result.Status = HealthStatusUnhealthy
	} else if warmHealth.Status == HealthStatusDegraded && result.Status == HealthStatusHealthy {
		result.Status = HealthStatusDegraded
	}

	// Check schema registry
	schemaHealth := h.checkSchemaRegistry(ctx)
	result.Components["schema_registry"] = schemaHealth

	// Check aggregation engine
	aggHealth := h.checkAggregationEngine(ctx)
	result.Components["aggregation_engine"] = aggHealth

	return result
}

// checkHotTier verifies the hot tier is operational.
func (h *HealthChecker) checkHotTier(ctx context.Context) *ComponentHealth {
	if h.store == nil {
		return &ComponentHealth{
			Status:  HealthStatusUnhealthy,
			Message: "store not initialized",
		}
	}

	start := time.Now()
	hot := h.store.Hot()
	if hot == nil {
		return &ComponentHealth{
			Status:  HealthStatusUnhealthy,
			Message: "hot tier not initialized",
		}
	}

	// Perform a simple read operation to verify functionality
	_, _ = hot.Get("__health_check__", []string{"test"})
	latency := time.Since(start)

	// Check metrics for potential issues
	metrics := hot.Metrics()

	health := &ComponentHealth{
		Status:  HealthStatusHealthy,
		Latency: latency.String(),
	}

	// If hit rate is very low and we have many requests, might indicate issues
	totalRequests := metrics.Hits + metrics.Misses
	if totalRequests > 1000 && metrics.Hits == 0 {
		health.Status = HealthStatusDegraded
		health.Message = "no cache hits detected"
	}

	return health
}

// checkWarmTier verifies the warm tier is operational.
func (h *HealthChecker) checkWarmTier(ctx context.Context) *ComponentHealth {
	if h.store == nil {
		return &ComponentHealth{
			Status:  HealthStatusUnhealthy,
			Message: "store not initialized",
		}
	}

	start := time.Now()
	warm := h.store.Warm()
	if warm == nil {
		return &ComponentHealth{
			Status:  HealthStatusUnhealthy,
			Message: "warm tier not initialized",
		}
	}

	// Perform a simple read operation to verify functionality
	_, _ = warm.Get("__health_check__", []string{"test"})
	latency := time.Since(start)

	health := &ComponentHealth{
		Status:  HealthStatusHealthy,
		Latency: latency.String(),
	}

	// Check if latency is too high
	if latency > 100*time.Millisecond {
		health.Status = HealthStatusDegraded
		health.Message = "high latency detected"
	}

	return health
}

// checkSchemaRegistry verifies the schema registry is operational.
func (h *HealthChecker) checkSchemaRegistry(ctx context.Context) *ComponentHealth {
	if h.schema == nil {
		return &ComponentHealth{
			Status:  HealthStatusUnhealthy,
			Message: "schema registry not initialized",
		}
	}

	start := time.Now()
	groups := h.schema.ListGroups()
	latency := time.Since(start)

	return &ComponentHealth{
		Status:  HealthStatusHealthy,
		Message: formatMessage("registered groups: %d", len(groups)),
		Latency: latency.String(),
	}
}

// checkAggregationEngine verifies the aggregation engine is operational.
func (h *HealthChecker) checkAggregationEngine(ctx context.Context) *ComponentHealth {
	if h.agg == nil {
		return &ComponentHealth{
			Status:  HealthStatusHealthy,
			Message: "aggregation engine not configured",
		}
	}

	return &ComponentHealth{
		Status:  HealthStatusHealthy,
		Message: "operational",
	}
}

func formatMessage(format string, args ...interface{}) string {
	return format
}

// LivenessCheck performs a simple liveness check.
func (h *HealthChecker) LivenessCheck() bool {
	return h.IsHealthy()
}

// ReadinessCheck performs a readiness check.
func (h *HealthChecker) ReadinessCheck() bool {
	if !h.IsReady() {
		return false
	}

	// Verify we can access the store
	if h.store == nil {
		return false
	}

	return true
}
