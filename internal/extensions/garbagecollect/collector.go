package garbagecollect

import (
	"fmt"
	"sort"
	"sync"
	"time"
)

// GCPolicy defines garbage collection policies for features.
type GCPolicy struct {
	Name                string        `json:"name"`
	MaxIdleDuration     time.Duration `json:"max_idle_duration_ns"`
	MaxAge              time.Duration `json:"max_age_ns"`
	MinAccessCount      int64         `json:"min_access_count"`
	DryRun              bool          `json:"dry_run"`
	ExcludePatterns     []string      `json:"exclude_patterns,omitempty"`
	RetainVersions      int           `json:"retain_versions"`
	EnableRedundancyCheck bool        `json:"enable_redundancy_check"`
}

// DefaultGCPolicy returns sensible defaults.
func DefaultGCPolicy() GCPolicy {
	return GCPolicy{
		Name:                "default",
		MaxIdleDuration:     30 * 24 * time.Hour,
		MaxAge:              90 * 24 * time.Hour,
		MinAccessCount:      0,
		DryRun:              true,
		RetainVersions:      3,
		EnableRedundancyCheck: false,
	}
}

// FeatureAccessRecord tracks access patterns for a feature.
type FeatureAccessRecord struct {
	FeatureName  string    `json:"feature_name"`
	Group        string    `json:"group"`
	LastAccessed time.Time `json:"last_accessed"`
	AccessCount  int64     `json:"access_count"`
	CreatedAt    time.Time `json:"created_at"`
	SizeBytes    int64     `json:"size_bytes"`
	Version      int64     `json:"version"`
}

// GCCandidate represents a feature identified for garbage collection.
type GCCandidate struct {
	FeatureName string   `json:"feature_name"`
	Group       string   `json:"group"`
	Reason      string   `json:"reason"`
	Score       float64  `json:"score"`
	LastAccess  string   `json:"last_access"`
	AccessCount int64    `json:"access_count"`
	SizeBytes   int64    `json:"size_bytes"`
	Dependents  []string `json:"dependents,omitempty"`
}

// GCResult holds the result of a garbage collection run.
type GCResult struct {
	RunID          string        `json:"run_id"`
	Policy         string        `json:"policy"`
	DryRun         bool          `json:"dry_run"`
	StartedAt      time.Time     `json:"started_at"`
	CompletedAt    time.Time     `json:"completed_at"`
	FeaturesScanned int          `json:"features_scanned"`
	Candidates      int          `json:"candidates"`
	Collected       int          `json:"collected"`
	BytesFreed      int64        `json:"bytes_freed"`
	Errors          int          `json:"errors"`
	Details         []GCCandidate `json:"details,omitempty"`
}

// GCStats aggregates GC statistics.
type GCStats struct {
	TotalRuns        int64     `json:"total_runs"`
	TotalCollected   int64     `json:"total_collected"`
	TotalBytesFreed  int64     `json:"total_bytes_freed"`
	LastRunAt        time.Time `json:"last_run_at"`
	ActivePolicies   int       `json:"active_policies"`
	TrackedFeatures  int       `json:"tracked_features"`
}

// Collector manages feature garbage collection.
type Collector struct {
	mu         sync.RWMutex
	policies   map[string]*GCPolicy
	accessLog  map[string]*FeatureAccessRecord
	results    []GCResult
	lineage    map[string][]string // feature -> dependents
	stats      GCStats
	maxResults int
}

// NewCollector creates a new garbage collector.
func NewCollector() *Collector {
	return &Collector{
		policies:   make(map[string]*GCPolicy),
		accessLog:  make(map[string]*FeatureAccessRecord),
		lineage:    make(map[string][]string),
		maxResults: 100,
	}
}

// RegisterPolicy registers a GC policy.
func (c *Collector) RegisterPolicy(policy GCPolicy) error {
	if policy.Name == "" {
		return fmt.Errorf("policy name is required")
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	c.policies[policy.Name] = &policy
	c.stats.ActivePolicies = len(c.policies)
	return nil
}

// ListPolicies returns all registered policies.
func (c *Collector) ListPolicies() []GCPolicy {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make([]GCPolicy, 0, len(c.policies))
	for _, p := range c.policies {
		result = append(result, *p)
	}
	return result
}

// DeletePolicy removes a GC policy.
func (c *Collector) DeletePolicy(name string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.policies[name]; !exists {
		return fmt.Errorf("policy %s not found", name)
	}
	delete(c.policies, name)
	c.stats.ActivePolicies = len(c.policies)
	return nil
}

// RecordAccess logs an access event for a feature.
func (c *Collector) RecordAccess(featureName, group string, sizeBytes int64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := group + ":" + featureName
	rec, exists := c.accessLog[key]
	if !exists {
		rec = &FeatureAccessRecord{
			FeatureName: featureName,
			Group:       group,
			CreatedAt:   time.Now(),
			SizeBytes:   sizeBytes,
		}
		c.accessLog[key] = rec
	}
	rec.LastAccessed = time.Now()
	rec.AccessCount++
	if sizeBytes > 0 {
		rec.SizeBytes = sizeBytes
	}
	c.stats.TrackedFeatures = len(c.accessLog)
}

// SetLineage sets the dependency graph for garbage collection safety checks.
func (c *Collector) SetLineage(feature string, dependents []string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lineage[feature] = dependents
}

// Analyze identifies GC candidates without removing anything.
func (c *Collector) Analyze(policyName string) ([]GCCandidate, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	policy, exists := c.policies[policyName]
	if !exists {
		return nil, fmt.Errorf("policy %s not found", policyName)
	}

	return c.findCandidates(policy), nil
}

// Run executes garbage collection with the specified policy.
func (c *Collector) Run(policyName string) (*GCResult, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	policy, exists := c.policies[policyName]
	if !exists {
		return nil, fmt.Errorf("policy %s not found", policyName)
	}

	result := &GCResult{
		RunID:     fmt.Sprintf("gc-%d", time.Now().UnixNano()),
		Policy:    policyName,
		DryRun:    policy.DryRun,
		StartedAt: time.Now(),
	}

	candidates := c.findCandidates(policy)
	result.FeaturesScanned = len(c.accessLog)
	result.Candidates = len(candidates)
	result.Details = candidates

	if !policy.DryRun {
		for _, candidate := range candidates {
			key := candidate.Group + ":" + candidate.FeatureName
			// Check for dependents before removing
			if deps := c.lineage[candidate.FeatureName]; len(deps) > 0 {
				result.Errors++
				continue
			}
			if rec, ok := c.accessLog[key]; ok {
				result.BytesFreed += rec.SizeBytes
			}
			delete(c.accessLog, key)
			result.Collected++
		}
	}

	result.CompletedAt = time.Now()

	c.stats.TotalRuns++
	c.stats.TotalCollected += int64(result.Collected)
	c.stats.TotalBytesFreed += result.BytesFreed
	c.stats.LastRunAt = result.CompletedAt
	c.stats.TrackedFeatures = len(c.accessLog)

	c.results = append(c.results, *result)
	if len(c.results) > c.maxResults {
		c.results = c.results[1:]
	}

	return result, nil
}

func (c *Collector) findCandidates(policy *GCPolicy) []GCCandidate {
	now := time.Now()
	var candidates []GCCandidate

	for _, rec := range c.accessLog {
		// Check exclusion patterns
		if c.isExcluded(rec.FeatureName, policy.ExcludePatterns) {
			continue
		}

		score := 0.0
		reason := ""

		// Idle check
		idle := now.Sub(rec.LastAccessed)
		if policy.MaxIdleDuration > 0 && idle > policy.MaxIdleDuration {
			score += float64(idle) / float64(policy.MaxIdleDuration)
			reason = fmt.Sprintf("idle for %s (max: %s)", idle.Round(time.Hour), policy.MaxIdleDuration.Round(time.Hour))
		}

		// Age check
		age := now.Sub(rec.CreatedAt)
		if policy.MaxAge > 0 && age > policy.MaxAge {
			score += float64(age) / float64(policy.MaxAge) * 0.5
			if reason == "" {
				reason = fmt.Sprintf("age %s exceeds max %s", age.Round(time.Hour), policy.MaxAge.Round(time.Hour))
			}
		}

		// Low access check
		if policy.MinAccessCount > 0 && rec.AccessCount < policy.MinAccessCount {
			score += 0.5
			if reason == "" {
				reason = fmt.Sprintf("low access count: %d (min: %d)", rec.AccessCount, policy.MinAccessCount)
			}
		}

		if score > 0 {
			candidate := GCCandidate{
				FeatureName: rec.FeatureName,
				Group:       rec.Group,
				Reason:      reason,
				Score:       score,
				LastAccess:  rec.LastAccessed.Format(time.RFC3339),
				AccessCount: rec.AccessCount,
				SizeBytes:   rec.SizeBytes,
				Dependents:  c.lineage[rec.FeatureName],
			}
			candidates = append(candidates, candidate)
		}
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Score > candidates[j].Score
	})

	return candidates
}

func (c *Collector) isExcluded(name string, patterns []string) bool {
	for _, p := range patterns {
		if p == name {
			return true
		}
	}
	return false
}

// GetResults returns recent GC results.
func (c *Collector) GetResults(limit int) []GCResult {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if limit > len(c.results) {
		limit = len(c.results)
	}
	start := len(c.results) - limit
	result := make([]GCResult, limit)
	copy(result, c.results[start:])
	return result
}

// Stats returns GC statistics.
func (c *Collector) Stats() GCStats {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.stats
}
