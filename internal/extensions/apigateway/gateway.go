package apigateway

import (
	"fmt"
	"math/rand"
	"sync"
	"time"
)

// BackendStatus represents the health state of a backend instance.
type BackendStatus string

const (
	Healthy  BackendStatus = "healthy"
	Degraded BackendStatus = "degraded"
	Down     BackendStatus = "down"
)

// Backend represents a downstream Feather instance.
type Backend struct {
	ID             string
	URL            string
	Status         BackendStatus
	Weight         int
	HealthCheckURL string
	LastCheck      time.Time
	Latency        float64
}

// CoalesceWindow defines the batching window for request coalescing.
type CoalesceWindow struct {
	DurationMs int
	MaxBatch   int
}

// GatewayConfig configures the API gateway.
type GatewayConfig struct {
	MaxBackends             int
	CoalesceWindowMs        int
	MaxCoalesceBatch        int
	RateLimitPerSec         int
	CircuitBreakerThreshold int
	HealthCheckInterval     time.Duration
}

// DefaultGatewayConfig returns sensible defaults.
func DefaultGatewayConfig() GatewayConfig {
	return GatewayConfig{
		MaxBackends:             100,
		CoalesceWindowMs:        5,
		MaxCoalesceBatch:        100,
		RateLimitPerSec:         10000,
		CircuitBreakerThreshold: 5,
		HealthCheckInterval:     30 * time.Second,
	}
}

// RouteResult contains the outcome of a routing decision.
type RouteResult struct {
	BackendID    string
	BackendURL   string
	Coalesced    bool
	CoalesceCount int
	LatencyMs    float64
}

// BackendStat holds per-backend statistics.
type BackendStat struct {
	ID            string
	TotalRequests int64
	AvgLatencyMs  float64
	ErrorRate     float64
}

// GatewayStats holds aggregate gateway statistics.
type GatewayStats struct {
	TotalBackends    int
	HealthyBackends  int
	TotalRouted      int64
	TotalCoalesced   int64
	TotalRateLimited int64
}

// Gateway routes requests across multiple backend instances.
type Gateway struct {
	mu               sync.RWMutex
	config           GatewayConfig
	backends         map[string]*Backend
	requestCounts    map[string]int64
	coalescePending  map[string][]chan RouteResult
	backendRequests  map[string]int64
	backendLatencies map[string][]float64
	totalRouted      int64
	totalCoalesced   int64
	totalRateLimited int64
}

// NewGateway creates a new Gateway with the given configuration.
func NewGateway(config GatewayConfig) *Gateway {
	return &Gateway{
		config:           config,
		backends:         make(map[string]*Backend),
		requestCounts:    make(map[string]int64),
		coalescePending:  make(map[string][]chan RouteResult),
		backendRequests:  make(map[string]int64),
		backendLatencies: make(map[string][]float64),
	}
}

// AddBackend registers a new backend instance.
func (g *Gateway) AddBackend(b Backend) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if len(g.backends) >= g.config.MaxBackends {
		return fmt.Errorf("maximum backends (%d) reached", g.config.MaxBackends)
	}
	if _, exists := g.backends[b.ID]; exists {
		return fmt.Errorf("backend %q already exists", b.ID)
	}
	cp := b
	g.backends[b.ID] = &cp
	return nil
}

// RemoveBackend removes a backend by ID.
func (g *Gateway) RemoveBackend(id string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if _, exists := g.backends[id]; !exists {
		return fmt.Errorf("backend %q not found", id)
	}
	delete(g.backends, id)
	return nil
}

// ListBackends returns all registered backends.
func (g *Gateway) ListBackends() []Backend {
	g.mu.RLock()
	defer g.mu.RUnlock()

	out := make([]Backend, 0, len(g.backends))
	for _, b := range g.backends {
		out = append(out, *b)
	}
	return out
}

// UpdateBackendStatus updates a backend's health status and latency.
func (g *Gateway) UpdateBackendStatus(id string, status BackendStatus, latency float64) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	b, exists := g.backends[id]
	if !exists {
		return fmt.Errorf("backend %q not found", id)
	}
	b.Status = status
	b.Latency = latency
	b.LastCheck = time.Now()
	return nil
}

// Route selects the best backend for the given tenant and entity key.
func (g *Gateway) Route(tenantID, entityKey string) (*RouteResult, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if !g.checkRateLimitLocked(tenantID) {
		g.totalRateLimited++
		return nil, ErrRateLimited
	}

	// Check for coalescing opportunity
	coalesceKey := tenantID + ":" + entityKey
	if pending, ok := g.coalescePending[coalesceKey]; ok && len(pending) < g.config.MaxCoalesceBatch {
		g.totalCoalesced++
		result := RouteResult{
			Coalesced:     true,
			CoalesceCount: len(pending) + 1,
		}
		return &result, nil
	}

	// Select best healthy backend using weighted random selection
	selected, err := g.selectBackendLocked()
	if err != nil {
		return nil, err
	}

	g.totalRouted++
	g.backendRequests[selected.ID]++

	// Set up coalescing window for this key
	g.coalescePending[coalesceKey] = make([]chan RouteResult, 0)
	go func() {
		time.Sleep(time.Duration(g.config.CoalesceWindowMs) * time.Millisecond)
		g.mu.Lock()
		delete(g.coalescePending, coalesceKey)
		g.mu.Unlock()
	}()

	return &RouteResult{
		BackendID:  selected.ID,
		BackendURL: selected.URL,
		LatencyMs:  selected.Latency,
	}, nil
}

// selectBackendLocked picks a healthy backend weighted by inverse latency.
// Must be called with g.mu held.
func (g *Gateway) selectBackendLocked() (*Backend, error) {
	var candidates []*Backend
	for _, b := range g.backends {
		if b.Status != Down {
			candidates = append(candidates, b)
		}
	}
	if len(candidates) == 0 {
		return nil, ErrBackendUnavailable
	}

	// Weighted selection: weight * (1 / (latency + 1))
	totalWeight := 0.0
	weights := make([]float64, len(candidates))
	for i, b := range candidates {
		w := float64(b.Weight) / (b.Latency + 1.0)
		if b.Status == Degraded {
			w *= 0.5
		}
		weights[i] = w
		totalWeight += w
	}

	r := rand.Float64() * totalWeight
	cumulative := 0.0
	for i, w := range weights {
		cumulative += w
		if r <= cumulative {
			return candidates[i], nil
		}
	}
	return candidates[len(candidates)-1], nil
}

// CheckRateLimit checks whether the given tenant is within rate limits.
func (g *Gateway) CheckRateLimit(tenantID string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.checkRateLimitLocked(tenantID)
}

// checkRateLimitLocked performs rate limit check; must be called with g.mu held.
func (g *Gateway) checkRateLimitLocked(tenantID string) bool {
	count := g.requestCounts[tenantID]
	if count >= int64(g.config.RateLimitPerSec) {
		return false
	}
	g.requestCounts[tenantID] = count + 1
	return true
}

// GetBackendStats returns per-backend statistics.
func (g *Gateway) GetBackendStats() []BackendStat {
	g.mu.RLock()
	defer g.mu.RUnlock()

	var stats []BackendStat
	for id, reqs := range g.backendRequests {
		var avg float64
		if lats := g.backendLatencies[id]; len(lats) > 0 {
			sum := 0.0
			for _, l := range lats {
				sum += l
			}
			avg = sum / float64(len(lats))
		}
		stats = append(stats, BackendStat{
			ID:            id,
			TotalRequests: reqs,
			AvgLatencyMs:  avg,
		})
	}
	return stats
}

// Stats returns aggregate gateway statistics.
func (g *Gateway) Stats() GatewayStats {
	g.mu.RLock()
	defer g.mu.RUnlock()

	healthy := 0
	for _, b := range g.backends {
		if b.Status == Healthy {
			healthy++
		}
	}
	return GatewayStats{
		TotalBackends:    len(g.backends),
		HealthyBackends:  healthy,
		TotalRouted:      g.totalRouted,
		TotalCoalesced:   g.totalCoalesced,
		TotalRateLimited: g.totalRateLimited,
	}
}
