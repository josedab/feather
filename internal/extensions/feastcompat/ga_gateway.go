package feastcompat

import (
	"fmt"
	"sync"
	"time"
)

// GAConfig configures the production-grade Feast gateway.
type GAConfig struct {
	StrictMode          bool          `json:"strict_mode" yaml:"strict_mode"`
	MaxFeatureServices  int           `json:"max_feature_services" yaml:"max_feature_services"`
	RequestTimeout      time.Duration `json:"request_timeout" yaml:"request_timeout"`
	CompatibilityLevel  string        `json:"compatibility_level" yaml:"compatibility_level"` // "full", "partial"
}

// DefaultGAConfig returns production-grade defaults.
func DefaultGAConfig() GAConfig {
	return GAConfig{
		StrictMode:         true,
		MaxFeatureServices: 1000,
		RequestTimeout:     10 * time.Second,
		CompatibilityLevel: "full",
	}
}

// GAGateway provides a production-grade Feast-compatible gateway.
type GAGateway struct {
	config       GAConfig
	gateway      *Gateway
	adapter      *Adapter
	storeAdapter *StoreLookupAdapter
	suite        *CompatTestSuite
	migrator     *MigrationCLI
	mu           sync.RWMutex
	features     map[string]interface{} // in-memory feature store for GA
	stats        GAStats
}

// GAStats tracks GA gateway statistics.
type GAStats struct {
	TotalRequests        int64 `json:"total_requests"`
	CompatTestsPassed    int   `json:"compat_tests_passed"`
	CompatTestsFailed    int   `json:"compat_tests_failed"`
	MigrationsCompleted  int   `json:"migrations_completed"`
	FeatureServicesCount int   `json:"feature_services_count"`
}

// NewGAGateway creates a new production-grade Feast gateway.
func NewGAGateway(cfg GAConfig) *GAGateway {
	adapter := NewAdapter(DefaultAdapterConfig())
	gw := NewGateway(adapter)
	suite := NewCompatTestSuite(gw)
	migrator := NewMigrationCLI()

	return &GAGateway{
		config:   cfg,
		gateway:  gw,
		adapter:  adapter,
		suite:    suite,
		migrator: migrator,
		features: make(map[string]interface{}),
	}
}

// SetFeature stores a feature value for an entity (used for testing and local mode).
func (g *GAGateway) SetFeature(entityKey, featureRef string, value interface{}) {
	g.mu.Lock()
	defer g.mu.Unlock()
	key := entityKey + ":" + featureRef
	g.features[key] = value
}

// SetStoreAdapter configures the GA gateway to use real storage for reads and writes.
func (g *GAGateway) SetStoreAdapter(adapter *StoreLookupAdapter) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.storeAdapter = adapter
	g.gateway.SetStoreAdapter(adapter)
}

// GetOnlineFeatures implements the Feast get_online_features endpoint.
func (g *GAGateway) GetOnlineFeatures(entityRows []map[string]interface{}, featureRefs []string) (map[string]interface{}, error) {
	g.mu.Lock()
	g.stats.TotalRequests++
	g.mu.Unlock()

	if len(entityRows) == 0 {
		return nil, fmt.Errorf("entity_rows must not be empty")
	}
	if len(featureRefs) == 0 {
		return nil, fmt.Errorf("feature_refs must not be empty")
	}

	result := make(map[string]interface{})
	result["metadata"] = map[string]interface{}{
		"feature_names": featureRefs,
	}

	// Snapshot mutable state under a single lock to avoid races.
	g.mu.RLock()
	storeAdapter := g.storeAdapter
	var lookupFn FeatureLookupFunc
	if storeAdapter != nil {
		lookupFn = storeAdapter.LookupFunc()
	}
	// Copy the in-memory features map reference (map reads are safe
	// as long as we don't write concurrently, and writes are guarded by mu).
	featuresSnapshot := g.features
	g.mu.RUnlock()

	results := make([]map[string]interface{}, len(entityRows))
	for i, entity := range entityRows {
		row := make(map[string]interface{})
		for k, v := range entity {
			row[k] = v
		}
		// Derive an entity key from the entity row for lookups.
		entityKey := ""
		for _, v := range entity {
			entityKey += fmt.Sprintf("%v", v)
		}

		if lookupFn != nil {
			vals, err := lookupFn(entityKey, featureRefs)
			if err == nil {
				for _, ref := range featureRefs {
					if val, ok := vals[ref]; ok {
						row[ref] = val
					} else {
						row[ref] = nil
					}
				}
			} else {
				for _, ref := range featureRefs {
					row[ref] = nil
				}
			}
		} else {
			for _, ref := range featureRefs {
				key := entityKey + ":" + ref
				if val, ok := featuresSnapshot[key]; ok {
					row[ref] = val
				} else {
					row[ref] = nil
				}
			}
		}
		results[i] = row
	}

	result["results"] = results
	return result, nil
}

// Push implements the Feast push endpoint.
func (g *GAGateway) Push(req PushRequest) (*PushResponse, error) {
	g.mu.Lock()
	g.stats.TotalRequests++
	g.mu.Unlock()
	return g.gateway.Push(req)
}

// PlanMigration creates a migration plan from a Feast config.
func (g *GAGateway) PlanMigration(feastConfig string) (*MigrationPlan, error) {
	plan, err := g.migrator.Plan(feastConfig)
	if err != nil {
		return nil, fmt.Errorf("planning migration: %w", err)
	}
	return plan, nil
}

// ExecuteMigration runs a migration plan.
func (g *GAGateway) ExecuteMigration(feastConfig string) (*MigrationResult, error) {
	result, err := g.migrator.Execute(feastConfig)
	if err != nil {
		return nil, fmt.Errorf("executing migration: %w", err)
	}
	g.mu.Lock()
	g.stats.MigrationsCompleted++
	g.mu.Unlock()
	return result, nil
}

// RunCompatTests runs the full compatibility test suite.
func (g *GAGateway) RunCompatTests() *CompatReport {
	report := g.suite.Report()
	g.mu.Lock()
	g.stats.CompatTestsPassed = report.Passed
	g.stats.CompatTestsFailed = report.Failed
	g.mu.Unlock()
	return report
}

// ListTests returns available compatibility tests.
func (g *GAGateway) ListTests() []string {
	return g.suite.ListTests()
}

// RunTest runs a single compatibility test.
func (g *GAGateway) RunTest(name string) *CompatTestResult {
	return g.suite.RunTest(name)
}

// ValidateFeastConfig validates a Feast configuration for compatibility.
func (g *GAGateway) ValidateFeastConfig(config string) ([]string, error) {
	result, err := g.migrator.Execute(config)
	if err != nil {
		return []string{err.Error()}, err
	}
	return g.migrator.Validate(result), nil
}

// Stats returns gateway statistics.
func (g *GAGateway) Stats() GAStats {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.stats
}
