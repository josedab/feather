package mlflow

import (
	"fmt"
	"sync"
	"time"
)

// Config holds MLflow tracker configuration.
type Config struct {
	TrackingURI       string
	DefaultExperiment string
	AutoLogFeatures   bool
	LineageTracking   bool
}

// DefaultConfig returns sensible defaults for MLflow integration.
func DefaultConfig() Config {
	return Config{
		TrackingURI:       "http://localhost:5000",
		DefaultExperiment: "default",
		AutoLogFeatures:   true,
		LineageTracking:   true,
	}
}

// Run represents an MLflow experiment run.
type Run struct {
	ID           string             `json:"id"`
	ExperimentID string             `json:"experiment_id"`
	Name         string             `json:"name"`
	Status       string             `json:"status"`
	FeaturesUsed []string           `json:"features_used"`
	ModelVersion string             `json:"model_version"`
	StartTime    time.Time          `json:"start_time"`
	EndTime      time.Time          `json:"end_time,omitempty"`
	Metrics      map[string]float64 `json:"metrics,omitempty"`
	Params       map[string]string  `json:"params,omitempty"`
}

// FeatureLineage tracks which features were used in which runs.
type FeatureLineage struct {
	RunID          string    `json:"run_id"`
	FeatureID      string    `json:"feature_id"`
	FeatureVersion string    `json:"feature_version"`
	UsedAt         time.Time `json:"used_at"`
}

// TrackerStats provides summary statistics.
type TrackerStats struct {
	TotalRuns       int `json:"total_runs"`
	ActiveRuns      int `json:"active_runs"`
	FeaturesTracked int `json:"features_tracked"`
	ModelsLinked    int `json:"models_linked"`
}

// Tracker manages MLflow experiment tracking and feature lineage.
type Tracker struct {
	mu      sync.RWMutex
	config  Config
	runs    map[string]*Run
	lineage map[string][]*FeatureLineage
}

// NewTracker creates a new MLflow tracker.
func NewTracker(cfg Config) *Tracker {
	return &Tracker{
		config:  cfg,
		runs:    make(map[string]*Run),
		lineage: make(map[string][]*FeatureLineage),
	}
}

// StartRun begins a new experiment run.
func (t *Tracker) StartRun(name, experimentID string) (*Run, error) {
	if name == "" {
		return nil, fmt.Errorf("run name is required")
	}
	if experimentID == "" {
		experimentID = t.config.DefaultExperiment
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	run := &Run{
		ID:           fmt.Sprintf("run_%d", time.Now().UnixNano()),
		ExperimentID: experimentID,
		Name:         name,
		Status:       "running",
		FeaturesUsed: []string{},
		StartTime:    time.Now(),
		Metrics:      make(map[string]float64),
		Params:       make(map[string]string),
	}
	t.runs[run.ID] = run
	return run, nil
}

// EndRun completes a run with the given status.
func (t *Tracker) EndRun(runID, status string) error {
	if status != "completed" && status != "failed" {
		return fmt.Errorf("status must be 'completed' or 'failed'")
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	run, ok := t.runs[runID]
	if !ok {
		return fmt.Errorf("run not found: %s", runID)
	}
	if run.Status != "running" {
		return fmt.Errorf("run %s is not running", runID)
	}
	run.Status = status
	run.EndTime = time.Now()
	return nil
}

// LogFeatureUsage records features used in a run.
func (t *Tracker) LogFeatureUsage(runID string, featureIDs []string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	run, ok := t.runs[runID]
	if !ok {
		return fmt.Errorf("run not found: %s", runID)
	}

	run.FeaturesUsed = append(run.FeaturesUsed, featureIDs...)

	if t.config.LineageTracking {
		now := time.Now()
		for _, fid := range featureIDs {
			entry := &FeatureLineage{
				RunID:     runID,
				FeatureID: fid,
				UsedAt:    now,
			}
			t.lineage[fid] = append(t.lineage[fid], entry)
		}
	}
	return nil
}

// LogMetrics records metrics for a run.
func (t *Tracker) LogMetrics(runID string, metrics map[string]float64) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	run, ok := t.runs[runID]
	if !ok {
		return fmt.Errorf("run not found: %s", runID)
	}
	for k, v := range metrics {
		run.Metrics[k] = v
	}
	return nil
}

// GetRun returns a run by ID.
func (t *Tracker) GetRun(id string) (*Run, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	run, ok := t.runs[id]
	if !ok {
		return nil, fmt.Errorf("run not found: %s", id)
	}
	return run, nil
}

// ListRuns returns all runs.
func (t *Tracker) ListRuns() []*Run {
	t.mu.RLock()
	defer t.mu.RUnlock()

	runs := make([]*Run, 0, len(t.runs))
	for _, r := range t.runs {
		runs = append(runs, r)
	}
	return runs
}

// GetLineage returns lineage entries for a feature.
func (t *Tracker) GetLineage(featureID string) []*FeatureLineage {
	t.mu.RLock()
	defer t.mu.RUnlock()

	entries := t.lineage[featureID]
	result := make([]*FeatureLineage, len(entries))
	copy(result, entries)
	return result
}

// Stats returns tracker statistics.
func (t *Tracker) Stats() *TrackerStats {
	t.mu.RLock()
	defer t.mu.RUnlock()

	stats := &TrackerStats{
		TotalRuns:       len(t.runs),
		FeaturesTracked: len(t.lineage),
	}
	modelsLinked := make(map[string]bool)
	for _, r := range t.runs {
		if r.Status == "running" {
			stats.ActiveRuns++
		}
		if r.ModelVersion != "" {
			modelsLinked[r.ModelVersion] = true
		}
	}
	stats.ModelsLinked = len(modelsLinked)
	return stats
}
