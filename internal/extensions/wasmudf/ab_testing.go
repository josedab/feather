package wasmudf

import (
	"fmt"
	"hash/fnv"
	"sync"
	"time"
)

// ABTestConfig configures A/B testing between UDF versions.
type ABTestConfig struct {
	ModuleID       string  `json:"module_id"`
	VersionA       string  `json:"version_a"`
	VersionB       string  `json:"version_b"`
	TrafficPercent float64 `json:"traffic_percent_b"` // 0.0-1.0, percentage routed to version B
	Enabled        bool    `json:"enabled"`
}

// ABTestResult tracks per-version metrics during an A/B test.
type ABTestResult struct {
	Config        ABTestConfig `json:"config"`
	VersionAStats VersionStats `json:"version_a_stats"`
	VersionBStats VersionStats `json:"version_b_stats"`
	StartedAt     time.Time    `json:"started_at"`
	Winner        string       `json:"winner,omitempty"` // "A", "B", or ""
}

// VersionStats tracks metrics for a single version in an A/B test.
type VersionStats struct {
	Executions int64   `json:"executions"`
	Errors     int64   `json:"errors"`
	AvgLatency float64 `json:"avg_latency_ms"`
	TotalMs    float64 `json:"-"`
}

// ABTestManager manages A/B tests between UDF versions.
type ABTestManager struct {
	mu    sync.RWMutex
	tests map[string]*ABTestResult // moduleID -> active test
}

// NewABTestManager creates a new A/B test manager.
func NewABTestManager() *ABTestManager {
	return &ABTestManager{
		tests: make(map[string]*ABTestResult),
	}
}

// CreateTest creates a new A/B test.
func (m *ABTestManager) CreateTest(config ABTestConfig) (*ABTestResult, error) {
	if config.ModuleID == "" {
		return nil, fmt.Errorf("module_id is required")
	}
	if config.VersionA == "" || config.VersionB == "" {
		return nil, fmt.Errorf("both version_a and version_b are required")
	}
	if config.TrafficPercent < 0 || config.TrafficPercent > 1 {
		return nil, fmt.Errorf("traffic_percent_b must be between 0 and 1")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.tests[config.ModuleID]; exists {
		return nil, fmt.Errorf("A/B test already exists for module %s", config.ModuleID)
	}

	result := &ABTestResult{
		Config:    config,
		StartedAt: time.Now(),
	}
	m.tests[config.ModuleID] = result
	return result, nil
}

// ResolveVersion determines which version to use for a given entity key.
func (m *ABTestManager) ResolveVersion(moduleID, entityKey string) string {
	m.mu.RLock()
	test, exists := m.tests[moduleID]
	m.mu.RUnlock()

	if !exists || !test.Config.Enabled {
		return "" // no test active, use default
	}

	// Deterministic bucketing based on entity key hash
	h := fnv.New32a()
	h.Write([]byte(entityKey))
	bucket := float64(h.Sum32()%1000) / 1000.0

	if bucket < test.Config.TrafficPercent {
		return test.Config.VersionB
	}
	return test.Config.VersionA
}

// RecordExecution records execution metrics for an A/B test.
func (m *ABTestManager) RecordExecution(moduleID, version string, durationMs float64, isError bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	test, exists := m.tests[moduleID]
	if !exists {
		return
	}

	var stats *VersionStats
	if version == test.Config.VersionA {
		stats = &test.VersionAStats
	} else if version == test.Config.VersionB {
		stats = &test.VersionBStats
	} else {
		return
	}

	stats.Executions++
	stats.TotalMs += durationMs
	if stats.Executions > 0 {
		stats.AvgLatency = stats.TotalMs / float64(stats.Executions)
	}
	if isError {
		stats.Errors++
	}
}

// GetTest returns the active A/B test for a module.
func (m *ABTestManager) GetTest(moduleID string) (*ABTestResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	test, exists := m.tests[moduleID]
	if !exists {
		return nil, fmt.Errorf("no A/B test for module %s", moduleID)
	}
	result := *test
	return &result, nil
}

// EndTest stops an A/B test and declares a winner.
func (m *ABTestManager) EndTest(moduleID string) (*ABTestResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	test, exists := m.tests[moduleID]
	if !exists {
		return nil, fmt.Errorf("no A/B test for module %s", moduleID)
	}

	// Determine winner based on error rate and latency
	test.Config.Enabled = false
	aErrorRate := float64(0)
	bErrorRate := float64(0)
	if test.VersionAStats.Executions > 0 {
		aErrorRate = float64(test.VersionAStats.Errors) / float64(test.VersionAStats.Executions)
	}
	if test.VersionBStats.Executions > 0 {
		bErrorRate = float64(test.VersionBStats.Errors) / float64(test.VersionBStats.Executions)
	}

	if aErrorRate < bErrorRate {
		test.Winner = "A"
	} else if bErrorRate < aErrorRate {
		test.Winner = "B"
	} else if test.VersionAStats.AvgLatency <= test.VersionBStats.AvgLatency {
		test.Winner = "A"
	} else {
		test.Winner = "B"
	}

	result := *test
	delete(m.tests, moduleID)
	return &result, nil
}

// ListTests returns all active A/B tests.
func (m *ABTestManager) ListTests() []ABTestResult {
	m.mu.RLock()
	defer m.mu.RUnlock()
	results := make([]ABTestResult, 0, len(m.tests))
	for _, t := range m.tests {
		results = append(results, *t)
	}
	return results
}
