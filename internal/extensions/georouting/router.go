// Package georouting provides multi-cloud federation with latency-based
// geo-routing and data residency compliance.
package georouting

import (
	"context"
	"fmt"
	"math"
	"sort"
	"sync"
	"time"
)

// CloudProvider identifies a cloud provider.
type CloudProvider string

const (
	// CloudAWS is Amazon Web Services.
	CloudAWS CloudProvider = "aws"
	// CloudGCP is Google Cloud Platform.
	CloudGCP CloudProvider = "gcp"
	// CloudAzure is Microsoft Azure.
	CloudAzure CloudProvider = "azure"
)

// ResidencyPolicy controls where data can be stored.
type ResidencyPolicy string

const (
	// ResidencyNone allows data anywhere.
	ResidencyNone ResidencyPolicy = "none"
	// ResidencyEU restricts data to EU regions.
	ResidencyEU ResidencyPolicy = "eu"
	// ResidencyUS restricts data to US regions.
	ResidencyUS ResidencyPolicy = "us"
	// ResidencyAPAC restricts data to APAC regions.
	ResidencyAPAC ResidencyPolicy = "apac"
)

// Region represents a cloud region.
type Region struct {
	ID        string        `json:"id"`
	Name      string        `json:"name"`
	Provider  CloudProvider `json:"provider"`
	Location  string        `json:"location"` // geographic area (eu, us, apac)
	Latitude  float64       `json:"latitude"`
	Longitude float64       `json:"longitude"`
	Healthy   bool          `json:"healthy"`
	Priority  int           `json:"priority"`
	Endpoint  string        `json:"endpoint"`
}

// RegionMetrics holds measured latency and health for a region.
type RegionMetrics struct {
	RegionID      string    `json:"region_id"`
	LatencyMs     float64   `json:"latency_ms"`
	ErrorRate     float64   `json:"error_rate"`
	RequestCount  int64     `json:"request_count"`
	LastProbe     time.Time `json:"last_probe"`
	Available     bool      `json:"available"`
}

// RoutingDecision captures why a region was selected.
type RoutingDecision struct {
	SelectedRegion string  `json:"selected_region"`
	Reason         string  `json:"reason"`
	LatencyMs      float64 `json:"latency_ms"`
	Fallback       bool    `json:"fallback"`
}

// RouterConfig configures the geo-router.
type RouterConfig struct {
	DefaultRegion      string
	MaxLatencyMs       float64
	HealthCheckInterval time.Duration
	FailoverEnabled    bool
	ResidencyPolicies  map[string]ResidencyPolicy // entity prefix -> policy
}

// DefaultRouterConfig returns sensible defaults.
func DefaultRouterConfig() RouterConfig {
	return RouterConfig{
		MaxLatencyMs:        50,
		HealthCheckInterval: 10 * time.Second,
		FailoverEnabled:     true,
		ResidencyPolicies:   make(map[string]ResidencyPolicy),
	}
}

// Router routes requests to the lowest-latency region while
// respecting data residency constraints.
type Router struct {
	config  RouterConfig
	regions map[string]*Region
	metrics map[string]*RegionMetrics
	mu      sync.RWMutex
}

// NewRouter creates a new geo-router.
func NewRouter(cfg RouterConfig) *Router {
	return &Router{
		config:  cfg,
		regions: make(map[string]*Region),
		metrics: make(map[string]*RegionMetrics),
	}
}

// AddRegion registers a cloud region.
func (r *Router) AddRegion(region *Region) error {
	if region.ID == "" {
		return fmt.Errorf("region ID is required")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.regions[region.ID] = region
	r.metrics[region.ID] = &RegionMetrics{
		RegionID:  region.ID,
		Available: region.Healthy,
		LastProbe: time.Now(),
	}
	return nil
}

// RemoveRegion removes a cloud region.
func (r *Router) RemoveRegion(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.regions, id)
	delete(r.metrics, id)
}

// UpdateMetrics updates the latency metrics for a region.
func (r *Router) UpdateMetrics(regionID string, latencyMs, errorRate float64) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if m, ok := r.metrics[regionID]; ok {
		m.LatencyMs = latencyMs
		m.ErrorRate = errorRate
		m.Available = errorRate < 0.5
		m.RequestCount++
		m.LastProbe = time.Now()
	}
}

// Route selects the best region for a request.
func (r *Router) Route(_ context.Context, entityKey string) (*RoutingDecision, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if len(r.regions) == 0 {
		return nil, fmt.Errorf("no regions configured")
	}

	// Determine residency constraint
	policy := r.getResidencyPolicy(entityKey)

	// Get eligible regions
	eligible := r.getEligibleRegions(policy)
	if len(eligible) == 0 {
		return nil, fmt.Errorf("no eligible regions for policy %s", policy)
	}

	// Sort by latency (lowest first)
	sort.Slice(eligible, func(i, j int) bool {
		mi := r.metrics[eligible[i].ID]
		mj := r.metrics[eligible[j].ID]
		if mi == nil || mj == nil {
			return eligible[i].Priority < eligible[j].Priority
		}
		return mi.LatencyMs < mj.LatencyMs
	})

	// Select the best healthy region
	for _, region := range eligible {
		m := r.metrics[region.ID]
		if m != nil && m.Available {
			return &RoutingDecision{
				SelectedRegion: region.ID,
				Reason:         "lowest_latency",
				LatencyMs:      m.LatencyMs,
			}, nil
		}
	}

	// Fallback to first eligible region regardless of health
	if r.config.FailoverEnabled {
		return &RoutingDecision{
			SelectedRegion: eligible[0].ID,
			Reason:         "failover",
			Fallback:       true,
		}, nil
	}

	return nil, fmt.Errorf("no healthy regions available")
}

// GetRegions returns all registered regions.
func (r *Router) GetRegions() []*Region {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*Region, 0, len(r.regions))
	for _, region := range r.regions {
		result = append(result, region)
	}
	return result
}

// GetMetrics returns metrics for all regions.
func (r *Router) GetMetrics() map[string]*RegionMetrics {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make(map[string]*RegionMetrics, len(r.metrics))
	for k, v := range r.metrics {
		copy := *v
		result[k] = &copy
	}
	return result
}

// Stats returns router statistics.
func (r *Router) Stats() map[string]interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()

	healthy := 0
	for _, m := range r.metrics {
		if m.Available {
			healthy++
		}
	}

	return map[string]interface{}{
		"total_regions":   len(r.regions),
		"healthy_regions": healthy,
		"policies":        len(r.config.ResidencyPolicies),
	}
}

func (r *Router) getResidencyPolicy(entityKey string) ResidencyPolicy {
	for prefix, policy := range r.config.ResidencyPolicies {
		if len(entityKey) >= len(prefix) && entityKey[:len(prefix)] == prefix {
			return policy
		}
	}
	return ResidencyNone
}

func (r *Router) getEligibleRegions(policy ResidencyPolicy) []*Region {
	var eligible []*Region
	for _, region := range r.regions {
		if matchesResidency(region, policy) {
			eligible = append(eligible, region)
		}
	}
	return eligible
}

func matchesResidency(region *Region, policy ResidencyPolicy) bool {
	switch policy {
	case ResidencyNone:
		return true
	case ResidencyEU:
		return region.Location == "eu"
	case ResidencyUS:
		return region.Location == "us"
	case ResidencyAPAC:
		return region.Location == "apac"
	default:
		return true
	}
}

// HaversineDistance computes great-circle distance between two points in km.
func HaversineDistance(lat1, lon1, lat2, lon2 float64) float64 {
	const R = 6371.0 // Earth radius in km
	dLat := toRadians(lat2 - lat1)
	dLon := toRadians(lon2 - lon1)
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(toRadians(lat1))*math.Cos(toRadians(lat2))*
			math.Sin(dLon/2)*math.Sin(dLon/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return R * c
}

func toRadians(deg float64) float64 {
	return deg * math.Pi / 180
}
