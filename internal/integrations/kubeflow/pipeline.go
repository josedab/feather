package kubeflow

import (
	"fmt"
	"sync"
	"time"
)

// Config holds Kubeflow pipeline manager configuration.
type Config struct {
	Namespace    string
	PipelineHost string
	AutoRegister bool
}

// DefaultConfig returns sensible defaults for Kubeflow integration.
func DefaultConfig() Config {
	return Config{
		Namespace:    "default",
		PipelineHost: "http://localhost:8888",
		AutoRegister: true,
	}
}

// Component represents a Kubeflow pipeline component.
type Component struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	Description    string    `json:"description"`
	Type           string    `json:"type"`
	InputFeatures  []string  `json:"input_features"`
	OutputFeatures []string  `json:"output_features"`
	Image          string    `json:"image"`
	CreatedAt      time.Time `json:"created_at"`
}

// PipelineRun represents a Kubeflow pipeline execution.
type PipelineRun struct {
	ID         string    `json:"id"`
	PipelineID string    `json:"pipeline_id"`
	Name       string    `json:"name"`
	Status     string    `json:"status"`
	Namespace  string    `json:"namespace"`
	Components []string  `json:"components"`
	StartTime  time.Time `json:"start_time"`
	EndTime    time.Time `json:"end_time,omitempty"`
}

// ManagerStats provides summary statistics.
type ManagerStats struct {
	TotalComponents int `json:"total_components"`
	TotalPipelines  int `json:"total_pipelines"`
	ActiveRuns      int `json:"active_runs"`
}

// Manager manages Kubeflow pipeline components and runs.
type Manager struct {
	mu         sync.RWMutex
	config     Config
	components map[string]*Component
	runs       map[string]*PipelineRun
}

// NewManager creates a new Kubeflow pipeline manager.
func NewManager(cfg Config) *Manager {
	return &Manager{
		config:     cfg,
		components: make(map[string]*Component),
		runs:       make(map[string]*PipelineRun),
	}
}

// RegisterComponent registers a pipeline component.
func (m *Manager) RegisterComponent(comp *Component) error {
	if comp.ID == "" || comp.Name == "" {
		return fmt.Errorf("component id and name are required")
	}
	if comp.Type != "feature_source" && comp.Type != "feature_sink" && comp.Type != "transform" {
		return fmt.Errorf("component type must be 'feature_source', 'feature_sink', or 'transform'")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.components[comp.ID]; exists {
		return fmt.Errorf("component already exists: %s", comp.ID)
	}
	comp.CreatedAt = time.Now()
	m.components[comp.ID] = comp
	return nil
}

// GetComponent returns a component by ID.
func (m *Manager) GetComponent(id string) (*Component, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	comp, ok := m.components[id]
	if !ok {
		return nil, fmt.Errorf("component not found: %s", id)
	}
	return comp, nil
}

// ListComponents returns all registered components.
func (m *Manager) ListComponents() []*Component {
	m.mu.RLock()
	defer m.mu.RUnlock()

	comps := make([]*Component, 0, len(m.components))
	for _, c := range m.components {
		comps = append(comps, c)
	}
	return comps
}

// CreatePipelineRun creates a new pipeline run from components.
func (m *Manager) CreatePipelineRun(name string, componentIDs []string) (*PipelineRun, error) {
	if name == "" {
		return nil, fmt.Errorf("pipeline run name is required")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	for _, cid := range componentIDs {
		if _, ok := m.components[cid]; !ok {
			return nil, fmt.Errorf("component not found: %s", cid)
		}
	}

	run := &PipelineRun{
		ID:         fmt.Sprintf("prun_%d", time.Now().UnixNano()),
		PipelineID: fmt.Sprintf("pipeline_%d", time.Now().UnixNano()),
		Name:       name,
		Status:     "running",
		Namespace:  m.config.Namespace,
		Components: componentIDs,
		StartTime:  time.Now(),
	}
	m.runs[run.ID] = run
	return run, nil
}

// GetPipelineRun returns a pipeline run by ID.
func (m *Manager) GetPipelineRun(id string) (*PipelineRun, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	run, ok := m.runs[id]
	if !ok {
		return nil, fmt.Errorf("pipeline run not found: %s", id)
	}
	return run, nil
}

// ListPipelineRuns returns all pipeline runs.
func (m *Manager) ListPipelineRuns() []*PipelineRun {
	m.mu.RLock()
	defer m.mu.RUnlock()

	runs := make([]*PipelineRun, 0, len(m.runs))
	for _, r := range m.runs {
		runs = append(runs, r)
	}
	return runs
}

// CompletePipelineRun marks a pipeline run as completed or failed.
func (m *Manager) CompletePipelineRun(id, status string) error {
	if status != "completed" && status != "failed" {
		return fmt.Errorf("status must be 'completed' or 'failed'")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	run, ok := m.runs[id]
	if !ok {
		return fmt.Errorf("pipeline run not found: %s", id)
	}
	if run.Status != "running" {
		return fmt.Errorf("pipeline run %s is not running", id)
	}
	run.Status = status
	run.EndTime = time.Now()
	return nil
}

// Stats returns manager statistics.
func (m *Manager) Stats() *ManagerStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := &ManagerStats{
		TotalComponents: len(m.components),
		TotalPipelines:  len(m.runs),
	}
	for _, r := range m.runs {
		if r.Status == "running" {
			stats.ActiveRuns++
		}
	}
	return stats
}
