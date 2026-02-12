package airflow

import (
	"fmt"
	"sync"
	"time"
)

// Config holds Airflow provider configuration.
type Config struct {
	AirflowURL             string
	DAGPrefix              string
	FreshnessCheckInterval time.Duration
}

// DefaultConfig returns sensible defaults for Airflow integration.
func DefaultConfig() Config {
	return Config{
		AirflowURL:             "http://localhost:8080",
		DAGPrefix:              "feather_",
		FreshnessCheckInterval: 5 * time.Minute,
	}
}

// DAGOperator represents an Airflow DAG operator for feature operations.
type DAGOperator struct {
	ID         string                 `json:"id"`
	Name       string                 `json:"name"`
	Type       string                 `json:"type"`
	FeatureIDs []string               `json:"feature_ids"`
	Schedule   string                 `json:"schedule"`
	Config     map[string]interface{} `json:"config,omitempty"`
	Enabled    bool                   `json:"enabled"`
	CreatedAt  time.Time              `json:"created_at"`
	LastRunAt  time.Time              `json:"last_run_at,omitempty"`
}

// SensorResult represents the result of a freshness check.
type SensorResult struct {
	OperatorID string        `json:"operator_id"`
	FeatureID  string        `json:"feature_id"`
	IsFresh    bool          `json:"is_fresh"`
	Staleness  time.Duration `json:"staleness"`
	CheckedAt  time.Time     `json:"checked_at"`
}

// ProviderStats provides summary statistics.
type ProviderStats struct {
	TotalOperators  int `json:"total_operators"`
	ActiveOperators int `json:"active_operators"`
	SensorChecks    int `json:"sensor_checks"`
	StaleFeatures   int `json:"stale_features"`
}

// Provider manages Airflow DAG operators and freshness sensors.
type Provider struct {
	mu            sync.RWMutex
	config        Config
	operators     map[string]*DAGOperator
	sensorResults []*SensorResult
}

// NewProvider creates a new Airflow provider.
func NewProvider(cfg Config) *Provider {
	return &Provider{
		config:        cfg,
		operators:     make(map[string]*DAGOperator),
		sensorResults: make([]*SensorResult, 0),
	}
}

// RegisterOperator registers a new DAG operator.
func (p *Provider) RegisterOperator(op *DAGOperator) error {
	if op.ID == "" || op.Name == "" {
		return fmt.Errorf("operator id and name are required")
	}
	if op.Type != "feature_compute" && op.Type != "feature_backfill" && op.Type != "freshness_sensor" {
		return fmt.Errorf("operator type must be 'feature_compute', 'feature_backfill', or 'freshness_sensor'")
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if _, exists := p.operators[op.ID]; exists {
		return fmt.Errorf("operator already exists: %s", op.ID)
	}
	op.CreatedAt = time.Now()
	p.operators[op.ID] = op
	return nil
}

// GetOperator returns an operator by ID.
func (p *Provider) GetOperator(id string) (*DAGOperator, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	op, ok := p.operators[id]
	if !ok {
		return nil, fmt.Errorf("operator not found: %s", id)
	}
	return op, nil
}

// ListOperators returns all registered operators.
func (p *Provider) ListOperators() []*DAGOperator {
	p.mu.RLock()
	defer p.mu.RUnlock()

	ops := make([]*DAGOperator, 0, len(p.operators))
	for _, op := range p.operators {
		ops = append(ops, op)
	}
	return ops
}

// EnableOperator enables a DAG operator.
func (p *Provider) EnableOperator(id string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	op, ok := p.operators[id]
	if !ok {
		return fmt.Errorf("operator not found: %s", id)
	}
	op.Enabled = true
	return nil
}

// DisableOperator disables a DAG operator.
func (p *Provider) DisableOperator(id string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	op, ok := p.operators[id]
	if !ok {
		return fmt.Errorf("operator not found: %s", id)
	}
	op.Enabled = false
	return nil
}

// CheckFreshness checks the freshness of a feature.
func (p *Provider) CheckFreshness(featureID string) (*SensorResult, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Find a freshness_sensor operator that covers this feature
	var sensorOp *DAGOperator
	for _, op := range p.operators {
		if op.Type != "freshness_sensor" || !op.Enabled {
			continue
		}
		for _, fid := range op.FeatureIDs {
			if fid == featureID {
				sensorOp = op
				break
			}
		}
		if sensorOp != nil {
			break
		}
	}

	staleness := time.Duration(0)
	isFresh := true
	operatorID := ""

	if sensorOp != nil {
		operatorID = sensorOp.ID
		if !sensorOp.LastRunAt.IsZero() {
			staleness = time.Since(sensorOp.LastRunAt)
			isFresh = staleness <= p.config.FreshnessCheckInterval
		}
	}

	result := &SensorResult{
		OperatorID: operatorID,
		FeatureID:  featureID,
		IsFresh:    isFresh,
		Staleness:  staleness,
		CheckedAt:  time.Now(),
	}
	p.sensorResults = append(p.sensorResults, result)
	return result, nil
}

// ListSensorResults returns all sensor results.
func (p *Provider) ListSensorResults() []*SensorResult {
	p.mu.RLock()
	defer p.mu.RUnlock()

	results := make([]*SensorResult, len(p.sensorResults))
	copy(results, p.sensorResults)
	return results
}

// Stats returns provider statistics.
func (p *Provider) Stats() *ProviderStats {
	p.mu.RLock()
	defer p.mu.RUnlock()

	stats := &ProviderStats{
		TotalOperators: len(p.operators),
		SensorChecks:   len(p.sensorResults),
	}
	for _, op := range p.operators {
		if op.Enabled {
			stats.ActiveOperators++
		}
	}
	for _, r := range p.sensorResults {
		if !r.IsFresh {
			stats.StaleFeatures++
		}
	}
	return stats
}
