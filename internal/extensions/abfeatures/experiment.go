package abfeatures

import (
	"fmt"
	"hash/fnv"
	"math"
	"sync"
	"time"
)

// ExperimentStatus indicates the lifecycle state of an experiment.
type ExperimentStatus string

// ExperimentStatus constants.
const (
	Draft     ExperimentStatus = "draft"
	Running   ExperimentStatus = "running"
	Concluded ExperimentStatus = "concluded"
)

// Variant represents one arm of an experiment.
type Variant struct {
	ID             string
	Name           string
	FeatureVersion string
	TrafficPercent float64
	Metrics        VariantMetrics
}

// VariantMetrics tracks performance metrics for a variant.
type VariantMetrics struct {
	Requests     int64
	AvgLatencyMs float64
	ErrorRate    float64
	CustomScore  float64
	ScoreCount   int64
}

// Experiment defines a multi-variant experiment.
type Experiment struct {
	ID           string
	Name         string
	FeatureGroup string
	Status       ExperimentStatus
	Variants     []Variant
	WinnerID     string
	StartedAt    *time.Time
	ConcludedAt  *time.Time
	CreatedAt    time.Time
}

// ExperimentConfig configures the experiment manager.
type ExperimentConfig struct {
	MaxExperiments    int
	MinSampleSize     int64
	SignificanceLevel float64
}

// DefaultExperimentConfig returns sensible defaults.
func DefaultExperimentConfig() ExperimentConfig {
	return ExperimentConfig{
		MaxExperiments:    1000,
		MinSampleSize:     1000,
		SignificanceLevel: 0.05,
	}
}

// SignificanceResult holds the outcome of a significance test.
type SignificanceResult struct {
	ExperimentID string
	Significant  bool
	WinnerID     string
	PValue       float64
	SampleSizeA  int64
	SampleSizeB  int64
}

// ManagerStats holds aggregate statistics.
type ManagerStats struct {
	Total     int
	Running   int
	Concluded int
}

// Manager manages experiments and traffic routing.
type Manager struct {
	mu          sync.RWMutex
	config      ExperimentConfig
	experiments map[string]*Experiment
}

// NewManager creates a new experiment manager.
func NewManager(config ExperimentConfig) *Manager {
	if config.MaxExperiments == 0 {
		config = DefaultExperimentConfig()
	}
	return &Manager{
		config:      config,
		experiments: make(map[string]*Experiment),
	}
}

// CreateExperiment creates a new experiment in draft status.
func (m *Manager) CreateExperiment(exp Experiment) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if exp.ID == "" {
		return fmt.Errorf("experiment ID is required")
	}
	if _, exists := m.experiments[exp.ID]; exists {
		return ErrExperimentExists
	}
	if len(m.experiments) >= m.config.MaxExperiments {
		return fmt.Errorf("max experiments (%d) reached", m.config.MaxExperiments)
	}

	exp.Status = Draft
	exp.CreatedAt = time.Now()
	m.experiments[exp.ID] = &exp
	return nil
}

// StartExperiment transitions an experiment to running.
func (m *Manager) StartExperiment(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	exp, exists := m.experiments[id]
	if !exists {
		return ErrExperimentNotFound
	}
	if exp.Status != Draft {
		return fmt.Errorf("experiment must be in draft status to start, current: %s", exp.Status)
	}

	now := time.Now()
	exp.Status = Running
	exp.StartedAt = &now
	return nil
}

// StopExperiment concludes a running experiment.
func (m *Manager) StopExperiment(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	exp, exists := m.experiments[id]
	if !exists {
		return ErrExperimentNotFound
	}
	if exp.Status != Running {
		return fmt.Errorf("experiment must be running to stop, current: %s", exp.Status)
	}

	now := time.Now()
	exp.Status = Concluded
	exp.ConcludedAt = &now
	return nil
}

// ResolveVariant deterministically routes an entity to a variant.
func (m *Manager) ResolveVariant(experimentID, entityID string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	exp, exists := m.experiments[experimentID]
	if !exists {
		return "", ErrExperimentNotFound
	}
	if exp.Status != Running {
		return "", fmt.Errorf("experiment %s is not running", experimentID)
	}
	if len(exp.Variants) == 0 {
		return "", ErrVariantNotFound
	}

	// Deterministic hash-based routing
	h := fnv.New32a()
	h.Write([]byte(experimentID + ":" + entityID))
	hashVal := float64(h.Sum32()) / float64(math.MaxUint32)

	var cumulative float64
	for _, v := range exp.Variants {
		cumulative += v.TrafficPercent / 100.0
		if hashVal <= cumulative {
			return v.ID, nil
		}
	}

	// Fallback to last variant
	return exp.Variants[len(exp.Variants)-1].ID, nil
}

// RecordMetric records a request metric for a variant.
func (m *Manager) RecordMetric(experimentID, variantID string, latencyMs float64, reqErr error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	exp, exists := m.experiments[experimentID]
	if !exists {
		return
	}

	for i := range exp.Variants {
		if exp.Variants[i].ID == variantID {
			v := &exp.Variants[i]
			v.Metrics.Requests++
			// Running average for latency
			v.Metrics.AvgLatencyMs = v.Metrics.AvgLatencyMs +
				(latencyMs-v.Metrics.AvgLatencyMs)/float64(v.Metrics.Requests)
			if reqErr != nil {
				errCount := v.Metrics.ErrorRate * float64(v.Metrics.Requests-1)
				v.Metrics.ErrorRate = (errCount + 1) / float64(v.Metrics.Requests)
			}
			return
		}
	}
}

// RecordScore records a custom score for a variant.
func (m *Manager) RecordScore(experimentID, variantID string, score float64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	exp, exists := m.experiments[experimentID]
	if !exists {
		return
	}

	for i := range exp.Variants {
		if exp.Variants[i].ID == variantID {
			v := &exp.Variants[i]
			v.Metrics.ScoreCount++
			// Running average
			v.Metrics.CustomScore = v.Metrics.CustomScore +
				(score-v.Metrics.CustomScore)/float64(v.Metrics.ScoreCount)
			return
		}
	}
}

// GetExperiment returns an experiment by ID.
func (m *Manager) GetExperiment(id string) (*Experiment, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	exp, exists := m.experiments[id]
	if !exists {
		return nil, ErrExperimentNotFound
	}
	copy := *exp
	return &copy, nil
}

// ListExperiments returns all experiments.
func (m *Manager) ListExperiments() []Experiment {
	m.mu.RLock()
	defer m.mu.RUnlock()

	out := make([]Experiment, 0, len(m.experiments))
	for _, exp := range m.experiments {
		out = append(out, *exp)
	}
	return out
}

// EvaluateSignificance performs a basic z-test comparing variant scores.
func (m *Manager) EvaluateSignificance(experimentID string) (*SignificanceResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	exp, exists := m.experiments[experimentID]
	if !exists {
		return nil, ErrExperimentNotFound
	}
	if len(exp.Variants) < 2 {
		return nil, fmt.Errorf("need at least 2 variants for significance test")
	}

	a := exp.Variants[0]
	b := exp.Variants[1]

	result := &SignificanceResult{
		ExperimentID: experimentID,
		SampleSizeA:  a.Metrics.ScoreCount,
		SampleSizeB:  b.Metrics.ScoreCount,
	}

	// Need minimum samples
	if a.Metrics.ScoreCount < m.config.MinSampleSize || b.Metrics.ScoreCount < m.config.MinSampleSize {
		result.PValue = 1.0
		return result, nil
	}

	// Simple z-test approximation
	diff := math.Abs(a.Metrics.CustomScore - b.Metrics.CustomScore)
	// Approximate standard error using score variance estimate
	se := math.Sqrt(1.0/float64(a.Metrics.ScoreCount) + 1.0/float64(b.Metrics.ScoreCount))
	if se == 0 {
		result.PValue = 1.0
		return result, nil
	}

	z := diff / se
	// Approximate p-value from z-score using error function
	pValue := math.Erfc(z / math.Sqrt(2))
	result.PValue = pValue
	result.Significant = pValue < m.config.SignificanceLevel

	if result.Significant {
		if a.Metrics.CustomScore > b.Metrics.CustomScore {
			result.WinnerID = a.ID
		} else {
			result.WinnerID = b.ID
		}
	}

	return result, nil
}

// Stats returns aggregate statistics.
func (m *Manager) Stats() ManagerStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := ManagerStats{Total: len(m.experiments)}
	for _, exp := range m.experiments {
		switch exp.Status {
		case Running:
			stats.Running++
		case Concluded:
			stats.Concluded++
		}
	}
	return stats
}
