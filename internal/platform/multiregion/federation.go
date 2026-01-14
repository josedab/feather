package multiregion

import (
	"errors"
	"fmt"
	"hash/fnv"
	"strings"
	"sync"
	"time"
)

// Sentinel errors.
var (
	ErrRegionNotFound    = errors.New("region not found")
	ErrRegionExists      = errors.New("region already exists")
	ErrRegionNameEmpty   = errors.New("region name is required")
	ErrEndpointEmpty     = errors.New("region endpoint is required")
	ErrCannotRemoveLocal = errors.New("cannot remove local region")
	ErrNoActiveRegion    = errors.New("no active region available")
	ErrResidencyViolation = errors.New("data residency violation")
)

// ConsistencyLevel controls cross-region read guarantees.
type ConsistencyLevel string

const (
	ConsistencyEventual ConsistencyLevel = "eventual"
	ConsistencyStrong   ConsistencyLevel = "strong"
	ConsistencyBounded  ConsistencyLevel = "bounded_staleness"
)

// ConflictStrategy determines how concurrent writes are resolved.
type ConflictStrategy string

const (
	ConflictLWW            ConflictStrategy = "last_writer_wins"
	ConflictHighestVersion ConflictStrategy = "highest_version"
	ConflictCustom         ConflictStrategy = "custom"
)

// RegionStatus tracks the lifecycle of a region.
type RegionStatus string

const (
	RegionActive   RegionStatus = "active"
	RegionDraining RegionStatus = "draining"
	RegionInactive RegionStatus = "inactive"
	RegionFailed   RegionStatus = "failed"
)

// ResidencyPolicy controls data placement constraints.
type ResidencyPolicy string

const (
	ResidencyNone   ResidencyPolicy = "none"
	ResidencyStrict ResidencyPolicy = "strict" // data must stay in region
	ResidencyPrefer ResidencyPolicy = "prefer" // prefer region but allow cross-region
)

// FederationConfig holds tunables for the federation layer.
type FederationConfig struct {
	LocalRegion       string
	ConsistencyLevel  ConsistencyLevel
	ConflictStrategy  ConflictStrategy
	ReplicationFactor int
	SyncInterval      time.Duration
	MaxLagDuration    time.Duration
	EnableResidency   bool
}

// DefaultFederationConfig returns sensible defaults.
func DefaultFederationConfig() FederationConfig {
	return FederationConfig{
		LocalRegion:       "us-east-1",
		ConsistencyLevel:  ConsistencyEventual,
		ConflictStrategy:  ConflictLWW,
		ReplicationFactor: 2,
		SyncInterval:      5 * time.Second,
		MaxLagDuration:    30 * time.Second,
		EnableResidency:   true,
	}
}

// Federation manages a set of geo-distributed regions with routing,
// residency enforcement, conflict resolution, and replication tracking.
type Federation struct {
	config         FederationConfig
	mu             sync.RWMutex
	regions        map[string]*Region
	routingTable   map[string]string // entity prefix -> region
	residencyRules map[string]ResidencyRule
	replicationLog []ReplicationEvent
	vectorClock    map[string]map[string]int64 // entity -> {region -> counter}
	conflictsResolved int
}

// Region represents a single deployment in the federation.
type Region struct {
	Name     string       `json:"name"`
	Endpoint string       `json:"endpoint"`
	Status   RegionStatus `json:"status"`
	Cloud    string       `json:"cloud"`     // aws, gcp, azure
	Location string       `json:"location"`  // geographic location
	Priority int          `json:"priority"`  // lower = higher priority
	Latency  float64      `json:"latency_ms"`
	Features int64        `json:"feature_count"`
	LastSync time.Time    `json:"last_sync"`
	JoinedAt time.Time    `json:"joined_at"`
}

// ResidencyRule binds entity prefixes to specific regions.
type ResidencyRule struct {
	Pattern string          `json:"pattern"` // entity prefix pattern
	Region  string          `json:"region"`
	Policy  ResidencyPolicy `json:"policy"`
	Reason  string          `json:"reason"` // e.g., "GDPR", "CCPA"
}

// ReplicationEvent records a single cross-region data movement.
type ReplicationEvent struct {
	ID         string    `json:"id"`
	Entity     string    `json:"entity"`
	Feature    string    `json:"feature"`
	FromRegion string    `json:"from_region"`
	ToRegion   string    `json:"to_region"`
	Timestamp  time.Time `json:"timestamp"`
	Status     string    `json:"status"` // pending, replicated, conflict, failed
	Version    int64     `json:"version"`
}

// RouteResult describes where a request should be sent.
type RouteResult struct {
	Region   string `json:"region"`
	Endpoint string `json:"endpoint"`
	Reason   string `json:"reason"`
	Local    bool   `json:"is_local"`
}

// ConflictInfo captures the outcome of a conflict resolution.
type ConflictInfo struct {
	Entity     string           `json:"entity"`
	Feature    string           `json:"feature"`
	Versions   map[string]int64 `json:"versions"` // region -> version
	Resolution string           `json:"resolution"`
	Winner     string           `json:"winner_region"`
}

// FederationStats summarises current federation state.
type FederationStats struct {
	TotalRegions      int              `json:"total_regions"`
	ActiveRegions     int              `json:"active_regions"`
	LocalRegion       string           `json:"local_region"`
	ConsistencyLevel  ConsistencyLevel `json:"consistency_level"`
	ReplicationEvents int              `json:"replication_events"`
	ConflictsResolved int              `json:"conflicts_resolved"`
	ResidencyRules    int              `json:"residency_rules"`
	ByCloud           map[string]int   `json:"by_cloud"`
	AvgLatency        float64          `json:"avg_latency_ms"`
}

// NewFederation creates a federation instance from the given config.
func NewFederation(cfg FederationConfig) *Federation {
	return &Federation{
		config:         cfg,
		regions:        make(map[string]*Region),
		routingTable:   make(map[string]string),
		residencyRules: make(map[string]ResidencyRule),
		replicationLog: make([]ReplicationEvent, 0),
		vectorClock:    make(map[string]map[string]int64),
	}
}

// AddRegion registers a new region in the federation.
func (f *Federation) AddRegion(region Region) error {
	if region.Name == "" {
		return ErrRegionNameEmpty
	}
	if region.Endpoint == "" {
		return ErrEndpointEmpty
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	if _, exists := f.regions[region.Name]; exists {
		return ErrRegionExists
	}

	r := region
	if r.JoinedAt.IsZero() {
		r.JoinedAt = time.Now()
	}
	if r.Status == "" {
		r.Status = RegionActive
	}
	f.regions[r.Name] = &r
	return nil
}

// RemoveRegion marks a region as draining and then removes it.
// The local region cannot be removed.
func (f *Federation) RemoveRegion(name string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if name == f.config.LocalRegion {
		return ErrCannotRemoveLocal
	}

	r, exists := f.regions[name]
	if !exists {
		return ErrRegionNotFound
	}

	r.Status = RegionDraining
	delete(f.regions, name)
	return nil
}

// GetRegion returns a region by name.
func (f *Federation) GetRegion(name string) (*Region, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	r, exists := f.regions[name]
	if !exists {
		return nil, ErrRegionNotFound
	}
	return r, nil
}

// ListRegions returns all registered regions.
func (f *Federation) ListRegions() []*Region {
	f.mu.RLock()
	defer f.mu.RUnlock()

	out := make([]*Region, 0, len(f.regions))
	for _, r := range f.regions {
		out = append(out, r)
	}
	return out
}

// Route determines the target region for an entity.
//
// Priority:
//  1. Strict residency rules matching the entity prefix.
//  2. Explicit routing table entry.
//  3. Local region if active.
//  4. Lowest-latency active region.
func (f *Federation) Route(entity string) (*RouteResult, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	// 1. Residency rules (strict only).
	if f.config.EnableResidency {
		for _, rule := range f.residencyRules {
			if rule.Policy == ResidencyStrict && strings.HasPrefix(entity, rule.Pattern) {
				if r, ok := f.regions[rule.Region]; ok && r.Status == RegionActive {
					return &RouteResult{
						Region:   r.Name,
						Endpoint: r.Endpoint,
						Reason:   fmt.Sprintf("residency:%s", rule.Reason),
						Local:    r.Name == f.config.LocalRegion,
					}, nil
				}
			}
		}
	}

	// 2. Explicit routing table.
	for prefix, regionName := range f.routingTable {
		if strings.HasPrefix(entity, prefix) {
			if r, ok := f.regions[regionName]; ok && r.Status == RegionActive {
				return &RouteResult{
					Region:   r.Name,
					Endpoint: r.Endpoint,
					Reason:   "routing_table",
					Local:    r.Name == f.config.LocalRegion,
				}, nil
			}
		}
	}

	// 3. Local region.
	if r, ok := f.regions[f.config.LocalRegion]; ok && r.Status == RegionActive {
		return &RouteResult{
			Region:   r.Name,
			Endpoint: r.Endpoint,
			Reason:   "local",
			Local:    true,
		}, nil
	}

	// 4. Lowest-latency active region.
	var best *Region
	for _, r := range f.regions {
		if r.Status != RegionActive {
			continue
		}
		if best == nil || r.Latency < best.Latency {
			best = r
		}
	}
	if best != nil {
		return &RouteResult{
			Region:   best.Name,
			Endpoint: best.Endpoint,
			Reason:   "lowest_latency",
			Local:    best.Name == f.config.LocalRegion,
		}, nil
	}

	return nil, ErrNoActiveRegion
}

// SetResidencyRule adds or replaces a residency rule keyed by pattern.
func (f *Federation) SetResidencyRule(rule ResidencyRule) error {
	if rule.Pattern == "" {
		return fmt.Errorf("residency rule pattern is required")
	}
	if rule.Region == "" {
		return fmt.Errorf("residency rule region is required")
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	f.residencyRules[rule.Pattern] = rule
	return nil
}

// GetResidencyRules returns all configured residency rules.
func (f *Federation) GetResidencyRules() []ResidencyRule {
	f.mu.RLock()
	defer f.mu.RUnlock()

	out := make([]ResidencyRule, 0, len(f.residencyRules))
	for _, r := range f.residencyRules {
		out = append(out, r)
	}
	return out
}

// ReplicateEvent records a replication event, enforcing residency and
// resolving conflicts via the configured strategy.
func (f *Federation) ReplicateEvent(event ReplicationEvent) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Check residency compliance.
	if f.config.EnableResidency {
		for _, rule := range f.residencyRules {
			if rule.Policy == ResidencyStrict && strings.HasPrefix(event.Entity, rule.Pattern) {
				if event.ToRegion != rule.Region {
					return fmt.Errorf("%w: entity %q must stay in %s (%s)",
						ErrResidencyViolation, event.Entity, rule.Region, rule.Reason)
				}
			}
		}
	}

	// Resolve conflicts if there are concurrent versions.
	vc := f.vectorClock[event.Entity]
	if vc != nil {
		existing, hasFrom := vc[event.FromRegion]
		if hasFrom && existing >= event.Version {
			info := f.resolveConflictLocked(event.Entity, event.Feature, map[string]int64{
				event.FromRegion: event.Version,
				event.ToRegion:   existing,
			})
			if info.Winner != event.FromRegion {
				event.Status = "conflict"
			}
		}
	}

	// Update vector clock.
	if f.vectorClock[event.Entity] == nil {
		f.vectorClock[event.Entity] = make(map[string]int64)
	}
	if event.Version > f.vectorClock[event.Entity][event.FromRegion] {
		f.vectorClock[event.Entity][event.FromRegion] = event.Version
	}

	if event.Status == "" {
		event.Status = "replicated"
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}
	if event.ID == "" {
		event.ID = generateEventID(event)
	}

	f.replicationLog = append(f.replicationLog, event)
	return nil
}

// ResolveConflict evaluates concurrent versions and returns the winner.
func (f *Federation) ResolveConflict(entity, feature string, versions map[string]int64) *ConflictInfo {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.resolveConflictLocked(entity, feature, versions)
}

func (f *Federation) resolveConflictLocked(entity, feature string, versions map[string]int64) *ConflictInfo {
	info := &ConflictInfo{
		Entity:   entity,
		Feature:  feature,
		Versions: versions,
	}

	switch f.config.ConflictStrategy {
	case ConflictHighestVersion:
		info.Resolution = string(ConflictHighestVersion)
		var maxVer int64
		for region, ver := range versions {
			if ver > maxVer {
				maxVer = ver
				info.Winner = region
			}
		}
	default: // LWW / fallback
		info.Resolution = string(ConflictLWW)
		var maxVer int64
		for region, ver := range versions {
			if ver > maxVer {
				maxVer = ver
				info.Winner = region
			}
		}
	}

	f.conflictsResolved++
	return info
}

// GetVectorClock returns the vector clock for an entity.
func (f *Federation) GetVectorClock(entity string) map[string]int64 {
	f.mu.RLock()
	defer f.mu.RUnlock()

	vc, ok := f.vectorClock[entity]
	if !ok {
		return nil
	}
	out := make(map[string]int64, len(vc))
	for k, v := range vc {
		out[k] = v
	}
	return out
}

// GetReplicationLog returns the most recent events up to limit.
func (f *Federation) GetReplicationLog(limit int) []ReplicationEvent {
	f.mu.RLock()
	defer f.mu.RUnlock()

	if limit <= 0 || limit > len(f.replicationLog) {
		limit = len(f.replicationLog)
	}
	start := len(f.replicationLog) - limit
	out := make([]ReplicationEvent, limit)
	copy(out, f.replicationLog[start:])
	return out
}

// CheckResidency reports whether an entity is allowed in the given region.
func (f *Federation) CheckResidency(entity, region string) (bool, string) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	for _, rule := range f.residencyRules {
		if strings.HasPrefix(entity, rule.Pattern) {
			switch rule.Policy {
			case ResidencyStrict:
				if region != rule.Region {
					return false, fmt.Sprintf("strict residency: must stay in %s (%s)", rule.Region, rule.Reason)
				}
			case ResidencyPrefer:
				if region != rule.Region {
					return true, fmt.Sprintf("preferred region is %s (%s)", rule.Region, rule.Reason)
				}
			}
		}
	}
	return true, ""
}

// Stats returns a snapshot of federation statistics.
func (f *Federation) Stats() FederationStats {
	f.mu.RLock()
	defer f.mu.RUnlock()

	stats := FederationStats{
		TotalRegions:      len(f.regions),
		LocalRegion:       f.config.LocalRegion,
		ConsistencyLevel:  f.config.ConsistencyLevel,
		ReplicationEvents: len(f.replicationLog),
		ConflictsResolved: f.conflictsResolved,
		ResidencyRules:    len(f.residencyRules),
		ByCloud:           make(map[string]int),
	}

	var totalLatency float64
	for _, r := range f.regions {
		if r.Cloud != "" {
			stats.ByCloud[r.Cloud]++
		}
		if r.Status == RegionActive {
			stats.ActiveRegions++
			totalLatency += r.Latency
		}
	}
	if stats.ActiveRegions > 0 {
		stats.AvgLatency = totalLatency / float64(stats.ActiveRegions)
	}

	return stats
}

// generateEventID produces a deterministic ID from event fields.
func generateEventID(e ReplicationEvent) string {
	h := fnv.New64a()
	_, _ = h.Write([]byte(e.Entity + e.Feature + e.FromRegion + e.ToRegion))
	return fmt.Sprintf("rep-%x-%d", h.Sum64(), e.Version)
}
