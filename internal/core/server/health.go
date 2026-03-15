package server

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/feather-store/feather/internal/core/aggregation"
	"github.com/feather-store/feather/internal/core/storage"
)

// HealthStatus represents the health status of a component.
type HealthStatus string

const (
	// HealthStatusHealthy indicates the component is operating normally.
	HealthStatusHealthy HealthStatus = "healthy"
	// HealthStatusDegraded indicates the component is degraded but usable.
	HealthStatusDegraded HealthStatus = "degraded"
	// HealthStatusUnhealthy indicates the component is unhealthy.
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

// componentCheckTimeout is the maximum time allowed for each health check component.
const componentCheckTimeout = 5 * time.Second

// Check performs a comprehensive health check.
func (h *HealthChecker) Check(ctx context.Context) *HealthCheckResult {
	result := &HealthCheckResult{
		Status:     HealthStatusHealthy,
		Timestamp:  time.Now(),
		Components: make(map[string]*ComponentHealth),
	}

	// Check all components with per-component timeout
	type componentResult struct {
		name   string
		health *ComponentHealth
	}
	checks := []struct {
		name string
		fn   func(context.Context) *ComponentHealth
	}{
		{"hot_tier", h.checkHotTier},
		{"warm_tier", h.checkWarmTier},
		{"schema_registry", h.checkSchemaRegistry},
		{"aggregation_engine", h.checkAggregationEngine},
	}

	var wg sync.WaitGroup
	results := make(chan componentResult, len(checks))

	for _, check := range checks {
		wg.Add(1)
		go func(name string, fn func(context.Context) *ComponentHealth) {
			defer wg.Done()
			checkCtx, cancel := context.WithTimeout(ctx, componentCheckTimeout)
			defer cancel()

			done := make(chan *ComponentHealth, 1)
			go func() {
				defer func() {
					if rec := recover(); rec != nil {
						done <- &ComponentHealth{
							Status:  HealthStatusUnhealthy,
							Message: fmt.Sprintf("health check panicked: %v", rec),
						}
					}
				}()
				done <- fn(checkCtx)
			}()

			select {
			case health := <-done:
				results <- componentResult{name: name, health: health}
			case <-checkCtx.Done():
				results <- componentResult{name: name, health: &ComponentHealth{
					Status:  HealthStatusUnhealthy,
					Message: "health check timed out",
				}}
			}
		}(check.name, check.fn)
	}

	wg.Wait()
	close(results)
	for cr := range results {
		result.Components[cr.name] = cr.health
	}

	// Aggregate status: use worst status from all components
	result.Status = h.aggregateStatus(result.Components)

	return result
}

// aggregateStatus returns the worst status from all components.
// Priority: unhealthy > degraded > healthy
func (h *HealthChecker) aggregateStatus(components map[string]*ComponentHealth) HealthStatus {
	worst := HealthStatusHealthy

	for _, comp := range components {
		if comp.Status == HealthStatusUnhealthy {
			return HealthStatusUnhealthy // Can't get worse, return early
		}
		if comp.Status == HealthStatusDegraded {
			worst = HealthStatusDegraded
		}
	}

	return worst
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

	// Detect degraded cache performance using hit rate ratio.
	// Only flag if we have enough data (>1000 requests) and hit rate is below 1%,
	// which accounts for cold starts where hits are naturally zero.
	totalRequests := metrics.Hits + metrics.Misses
	if totalRequests > 1000 {
		hitRate := float64(metrics.Hits) / float64(totalRequests)
		if hitRate < 0.01 {
			health.Status = HealthStatusDegraded
			health.Message = fmt.Sprintf("low cache hit rate: %.1f%%", hitRate*100)
		}
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

	latency, err := h.store.CheckWarmHealth()
	if err != nil {
		return &ComponentHealth{
			Status:  HealthStatusUnhealthy,
			Message: err.Error(),
		}
	}

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

	start := time.Now()
	// Verify the engine responds to a spec lookup (lightweight operation)
	_ = h.agg.GetSpec("__health_check__")
	latency := time.Since(start)

	return &ComponentHealth{
		Status:  HealthStatusHealthy,
		Message: "operational",
		Latency: latency.String(),
	}
}

func formatMessage(format string, args ...interface{}) string {
	return fmt.Sprintf(format, args...)
}

// LivenessCheck performs a simple liveness check.
func (h *HealthChecker) LivenessCheck() bool {
	return h.IsHealthy()
}

// ReadinessCheck performs a readiness check by verifying the store is accessible
// and responsive. Goes beyond a nil check by probing the warm tier.
func (h *HealthChecker) ReadinessCheck() bool {
	if !h.IsReady() {
		return false
	}

	if h.store == nil {
		return false
	}

	// Probe the warm tier to verify actual I/O functionality
	latency, err := h.store.CheckWarmHealth()
	if err != nil {
		return false
	}

	// Reject if warm tier latency exceeds a reasonable threshold
	if latency > 500*time.Millisecond {
		return false
	}

	return true
}
