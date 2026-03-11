package diffprivacy

import (
	"fmt"
)

// EnhancedEngine wraps Engine and BudgetManager to provide automatic
// per-(feature, entity_type) budget enforcement on every query.
type EnhancedEngine struct {
	engine        *Engine
	budgetManager *BudgetManager
}

// NewEnhancedEngine creates an engine with integrated budget management.
func NewEnhancedEngine(cfg Config, budgetCfg BudgetManagerConfig) *EnhancedEngine {
	return &EnhancedEngine{
		engine:        NewEngine(cfg),
		budgetManager: NewBudgetManager(budgetCfg),
	}
}

// Engine returns the underlying privacy engine.
func (e *EnhancedEngine) Engine() *Engine {
	return e.engine
}

// BudgetManager returns the budget manager.
func (e *EnhancedEngine) BudgetManager() *BudgetManager {
	return e.budgetManager
}

// RegisterFeature registers a feature in both the engine and budget manager.
func (e *EnhancedEngine) RegisterFeature(name, entityType string, cfg FeaturePrivacyConfig) error {
	if err := e.engine.RegisterFeature(name, cfg); err != nil {
		return err
	}
	return e.budgetManager.RegisterBudget(
		BudgetKey{Feature: name, EntityType: entityType},
		cfg.MaxBudget,
		cfg.Delta*float64(int(cfg.MaxBudget/cfg.Epsilon)),
	)
}

// AddNoiseTracked applies noise and tracks the budget consumption.
func (e *EnhancedEngine) AddNoiseTracked(featureName, entityType string, value float64) (float64, error) {
	// Get feature config for epsilon/delta values
	e.engine.mu.RLock()
	fs, err := e.engine.getFeatureStateLocked(featureName)
	if err != nil {
		e.engine.mu.RUnlock()
		return 0, fmt.Errorf("adding tracked noise to %q: %w", featureName, err)
	}
	epsilon := fs.config.Epsilon
	delta := fs.config.Delta
	mechanism := fs.config.Mechanism
	e.engine.mu.RUnlock()

	// Check and consume budget first
	key := BudgetKey{Feature: featureName, EntityType: entityType}
	if err := e.budgetManager.ConsumeAndCheck(key, epsilon, delta, mechanism, "noise"); err != nil {
		return 0, fmt.Errorf("budget check for %q: %w", featureName, err)
	}

	// Apply noise through the engine
	noisy, err := e.engine.AddNoise(featureName, value)
	if err != nil {
		return 0, err
	}

	return noisy, nil
}

// NoisyCountTracked returns a differentially private count with budget tracking.
func (e *EnhancedEngine) NoisyCountTracked(featureName, entityType string, count int64) (NoisyAggregation, error) {
	e.engine.mu.RLock()
	fs, err := e.engine.getFeatureStateLocked(featureName)
	if err != nil {
		e.engine.mu.RUnlock()
		return NoisyAggregation{}, fmt.Errorf("noisy count for %q: %w", featureName, err)
	}
	epsilon := fs.config.Epsilon
	delta := fs.config.Delta
	mechanism := fs.config.Mechanism
	e.engine.mu.RUnlock()

	key := BudgetKey{Feature: featureName, EntityType: entityType}
	if err := e.budgetManager.ConsumeAndCheck(key, epsilon, delta, mechanism, "count"); err != nil {
		return NoisyAggregation{}, fmt.Errorf("budget check for %q: %w", featureName, err)
	}

	return e.engine.NoisyCount(featureName, count)
}

// GenerateReport creates a compliance report using both engine and budget data.
func (e *EnhancedEngine) GenerateReport(framework ComplianceFramework, period ReportPeriod) *ComplianceReport {
	reporter := NewComplianceReporter(e.engine, e.budgetManager)
	return reporter.GenerateReport(framework, period)
}
