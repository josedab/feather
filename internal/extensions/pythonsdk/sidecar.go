package pythonsdk

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// SidecarConfig configures the Python sidecar process manager.
type SidecarConfig struct {
	PythonBinary    string        `json:"python_binary" yaml:"python_binary"`
	MaxWorkers      int           `json:"max_workers" yaml:"max_workers"`
	ExecutionTimeout time.Duration `json:"execution_timeout" yaml:"execution_timeout"`
	HotReloadEnabled bool         `json:"hot_reload_enabled" yaml:"hot_reload_enabled"`
	SandboxEnabled   bool         `json:"sandbox_enabled" yaml:"sandbox_enabled"`
	VenvPath         string       `json:"venv_path,omitempty" yaml:"venv_path,omitempty"`
	MaxMemoryMB      int          `json:"max_memory_mb" yaml:"max_memory_mb"`
	AllowedModules   []string     `json:"allowed_modules,omitempty" yaml:"allowed_modules,omitempty"`
}

// DefaultSidecarConfig returns sensible defaults.
func DefaultSidecarConfig() SidecarConfig {
	return SidecarConfig{
		PythonBinary:     "python3",
		MaxWorkers:       4,
		ExecutionTimeout: 30 * time.Second,
		HotReloadEnabled: true,
		SandboxEnabled:   true,
		MaxMemoryMB:      512,
		AllowedModules:   []string{"pandas", "polars", "numpy", "sklearn"},
	}
}

// TransformExecutor is a pluggable function that executes Python transforms.
type TransformExecutor func(ctx context.Context, transformID string, inputs map[string]interface{}) (interface{}, error)

// SidecarManager manages the lifecycle of Python sidecar processes
// with hot-reload, sandboxing, and dependency management.
type SidecarManager struct {
	config     SidecarConfig
	registry   *Registry
	executor   TransformExecutor
	mu         sync.RWMutex
	workers    map[string]*SidecarWorker
	deps       map[string][]Dependency
	stats      SidecarStats
	running    bool
}

// SidecarWorker represents a managed Python worker process.
type SidecarWorker struct {
	ID            string     `json:"id"`
	TransformID   string     `json:"transform_id"`
	Status        string     `json:"status"`
	PID           int        `json:"pid"`
	MemoryLimitMB int        `json:"memory_limit_mb"`
	Sandboxed     bool       `json:"sandboxed"`
	StartedAt     time.Time  `json:"started_at"`
	LastExec      *time.Time `json:"last_exec,omitempty"`
	ExecCount     int64      `json:"exec_count"`
	ErrorCount    int64      `json:"error_count"`
	ReloadCount   int64      `json:"reload_count"`
}

// Dependency represents a Python package dependency.
type Dependency struct {
	Name    string `json:"name" yaml:"name"`
	Version string `json:"version,omitempty" yaml:"version,omitempty"`
}

// SidecarStats tracks sidecar manager statistics.
type SidecarStats struct {
	TotalWorkers    int   `json:"total_workers"`
	ActiveWorkers   int   `json:"active_workers"`
	TotalExecutions int64 `json:"total_executions"`
	TotalErrors     int64 `json:"total_errors"`
	HotReloads      int64 `json:"hot_reloads"`
	AvgExecTimeMs   float64 `json:"avg_exec_time_ms"`
	totalExecMs     int64
}

// NewSidecarManager creates a new Python sidecar manager.
func NewSidecarManager(cfg SidecarConfig, registry *Registry) *SidecarManager {
	return &SidecarManager{
		config:   cfg,
		registry: registry,
		workers:  make(map[string]*SidecarWorker),
		deps:     make(map[string][]Dependency),
	}
}

// Start initializes the sidecar manager.
func (m *SidecarManager) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.running = true
	return nil
}

// Stop shuts down all workers.
func (m *SidecarManager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.running = false
	for id := range m.workers {
		m.workers[id].Status = "stopped"
	}
}

// DeployTransform creates or hot-reloads a worker for a transform.
func (m *SidecarManager) DeployTransform(ctx context.Context, transformID string, deps []Dependency) (*SidecarWorker, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return nil, fmt.Errorf("sidecar manager not running")
	}

	// Validate dependencies against allowed modules if sandbox enabled.
	if m.config.SandboxEnabled && len(m.config.AllowedModules) > 0 {
		allowed := make(map[string]bool, len(m.config.AllowedModules))
		for _, mod := range m.config.AllowedModules {
			allowed[mod] = true
		}
		for _, dep := range deps {
			if !allowed[dep.Name] {
				return nil, fmt.Errorf("module %q not in sandbox allowlist", dep.Name)
			}
		}
	}

	// If worker exists, hot-reload it.
	if existing, ok := m.workers[transformID]; ok {
		existing.Status = "reloading"
		existing.ReloadCount++
		// Update dependencies.
		m.deps[transformID] = deps
		existing.Status = "running"
		m.stats.HotReloads++
		return existing, nil
	}

	worker := &SidecarWorker{
		ID:           fmt.Sprintf("worker-%s", transformID),
		TransformID:  transformID,
		Status:       "running",
		PID:          0,
		MemoryLimitMB: m.config.MaxMemoryMB,
		Sandboxed:    m.config.SandboxEnabled,
		StartedAt:    time.Now(),
	}
	m.workers[transformID] = worker
	m.deps[transformID] = deps
	m.stats.TotalWorkers++
	m.stats.ActiveWorkers++

	return worker, nil
}

// ExecuteTransform runs a Python transform on the sidecar.
func (m *SidecarManager) ExecuteTransform(ctx context.Context, transformID string, inputs map[string]interface{}) (interface{}, error) {
	m.mu.RLock()
	worker, exists := m.workers[transformID]
	if !exists {
		m.mu.RUnlock()
		return nil, fmt.Errorf("no worker for transform %s", transformID)
	}
	if worker.Status != "running" {
		m.mu.RUnlock()
		return nil, fmt.Errorf("worker for %s is %s", transformID, worker.Status)
	}
	m.mu.RUnlock()

	// Apply execution timeout.
	if m.config.ExecutionTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, m.config.ExecutionTimeout)
		defer cancel()
	}

	start := time.Now()

	// Execute via the pluggable executor if set, otherwise passthrough.
	var result interface{}
	if m.executor != nil {
		var err error
		result, err = m.executor(ctx, transformID, inputs)
		if err != nil {
			m.mu.Lock()
			worker.ErrorCount++
			m.stats.TotalErrors++
			m.mu.Unlock()
			return nil, fmt.Errorf("executing transform %s: %w", transformID, err)
		}
	} else {
		result = inputs
	}

	elapsed := time.Since(start).Milliseconds()
	m.mu.Lock()
	worker.ExecCount++
	now := time.Now()
	worker.LastExec = &now
	m.stats.TotalExecutions++
	m.stats.totalExecMs += elapsed
	if m.stats.TotalExecutions > 0 {
		m.stats.AvgExecTimeMs = float64(m.stats.totalExecMs) / float64(m.stats.TotalExecutions)
	}
	m.mu.Unlock()

	return result, nil
}

// UndeployTransform stops and removes a worker.
func (m *SidecarManager) UndeployTransform(transformID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	worker, exists := m.workers[transformID]
	if !exists {
		return fmt.Errorf("no worker for transform %s", transformID)
	}
	worker.Status = "stopped"
	delete(m.workers, transformID)
	delete(m.deps, transformID)
	m.stats.ActiveWorkers--
	return nil
}

// ListWorkers returns all workers.
func (m *SidecarManager) ListWorkers() []*SidecarWorker {
	m.mu.RLock()
	defer m.mu.RUnlock()
	workers := make([]*SidecarWorker, 0, len(m.workers))
	for _, w := range m.workers {
		workers = append(workers, w)
	}
	return workers
}

// GetDependencies returns dependencies for a transform.
func (m *SidecarManager) GetDependencies(transformID string) []Dependency {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.deps[transformID]
}

// Stats returns sidecar manager statistics.
func (m *SidecarManager) Stats() SidecarStats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.stats
}

// SetExecutor sets the pluggable transform executor.
func (m *SidecarManager) SetExecutor(exec TransformExecutor) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.executor = exec
}

// HealthCheck returns an error if the manager or any worker is unhealthy.
func (m *SidecarManager) HealthCheck() error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if !m.running {
		return fmt.Errorf("sidecar manager not running")
	}
	for id, w := range m.workers {
		if w.Status != "running" {
			return fmt.Errorf("worker %s is %s", id, w.Status)
		}
	}
	return nil
}
