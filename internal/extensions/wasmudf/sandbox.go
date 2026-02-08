package wasmudf

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// SandboxConfig configures the WASM UDF sandbox with resource limits.
type SandboxConfig struct {
	MaxMemoryMB     int           `json:"max_memory_mb" yaml:"max_memory_mb"`
	MaxCPUTimeMs    int           `json:"max_cpu_time_ms" yaml:"max_cpu_time_ms"`
	MaxModuleSize   int64         `json:"max_module_size_bytes" yaml:"max_module_size_bytes"`
	EnableHotReload bool          `json:"enable_hot_reload" yaml:"enable_hot_reload"`
	MaxVersions     int           `json:"max_versions" yaml:"max_versions"`
	GCInterval      time.Duration `json:"gc_interval" yaml:"gc_interval"`
}

// DefaultSandboxConfig returns production defaults matching requirements.
func DefaultSandboxConfig() SandboxConfig {
	return SandboxConfig{
		MaxMemoryMB:     64,
		MaxCPUTimeMs:    100,
		MaxModuleSize:   10 * 1024 * 1024, // 10MB
		EnableHotReload: true,
		MaxVersions:     10,
		GCInterval:      5 * time.Minute,
	}
}

// SupportedLanguage describes a WASM-compilable language.
type SupportedLanguage struct {
	Name       string `json:"name"`
	Extensions []string `json:"extensions"`
	Compiler   string `json:"compiler"`
	Version    string `json:"version"`
}

// SupportedLanguages returns the list of supported UDF languages.
func SupportedLanguages() []SupportedLanguage {
	return []SupportedLanguage{
		{Name: "rust", Extensions: []string{".rs"}, Compiler: "rustc --target wasm32-wasi", Version: "1.70+"},
		{Name: "go", Extensions: []string{".go"}, Compiler: "tinygo build -target wasi", Version: "0.28+"},
		{Name: "assemblyscript", Extensions: []string{".ts"}, Compiler: "asc", Version: "0.27+"},
	}
}

// ModuleVersion represents a versioned snapshot of a WASM module.
type ModuleVersion struct {
	Version     string    `json:"version"`
	WasmBytes   []byte    `json:"-"`
	WasmSize    int64     `json:"wasm_size_bytes"`
	Language    string    `json:"language"`
	Checksum    string    `json:"checksum"`
	CreatedAt   time.Time `json:"created_at"`
	IsActive    bool      `json:"is_active"`
	ChangeSummary string  `json:"change_summary,omitempty"`
}

// ResourceUsage tracks per-execution resource consumption.
type ResourceUsage struct {
	MemoryPeakMB float64       `json:"memory_peak_mb"`
	CPUTimeMs    float64       `json:"cpu_time_ms"`
	WallTimeMs   float64       `json:"wall_time_ms"`
	GasUsed      int64         `json:"gas_used,omitempty"`
	WithinLimits bool          `json:"within_limits"`
	Violations   []string      `json:"violations,omitempty"`
}

// Sandbox wraps the Runtime with resource limits, versioning, and hot-reload.
type Sandbox struct {
	mu            sync.RWMutex
	runtime       *Runtime
	config        SandboxConfig
	versions      map[string][]ModuleVersion // moduleID -> version history
	hotReloadSubs map[string][]chan string     // moduleID -> notification channels
	logger        *slog.Logger
	stopCh        chan struct{}
}

// NewSandbox creates a new WASM UDF sandbox with resource limits.
func NewSandbox(runtime *Runtime, config SandboxConfig) *Sandbox {
	if config.MaxMemoryMB == 0 {
		config = DefaultSandboxConfig()
	}
	return &Sandbox{
		runtime:       runtime,
		config:        config,
		versions:      make(map[string][]ModuleVersion),
		hotReloadSubs: make(map[string][]chan string),
		logger:        slog.Default(),
		stopCh:        make(chan struct{}),
	}
}

// ExecuteWithLimits runs a WASM module with enforced resource limits.
func (s *Sandbox) ExecuteWithLimits(ctx context.Context, moduleID string, input map[string]interface{}) (*ExecutionResult, *ResourceUsage, error) {
	mod, err := s.runtime.GetModule(moduleID)
	if err != nil {
		return nil, nil, err
	}

	// Check memory limit
	if mod.MemoryLimitMB > s.config.MaxMemoryMB {
		return nil, &ResourceUsage{
			WithinLimits: false,
			Violations:   []string{fmt.Sprintf("memory limit %dMB exceeds sandbox max %dMB", mod.MemoryLimitMB, s.config.MaxMemoryMB)},
		}, fmt.Errorf("module memory limit %dMB exceeds sandbox maximum %dMB", mod.MemoryLimitMB, s.config.MaxMemoryMB)
	}

	// Check timeout limit
	if mod.TimeoutMs > s.config.MaxCPUTimeMs {
		return nil, &ResourceUsage{
			WithinLimits: false,
			Violations:   []string{fmt.Sprintf("timeout %dms exceeds sandbox max %dms", mod.TimeoutMs, s.config.MaxCPUTimeMs)},
		}, fmt.Errorf("module timeout %dms exceeds sandbox maximum %dms", mod.TimeoutMs, s.config.MaxCPUTimeMs)
	}

	// Execute with context timeout
	timeoutCtx, cancel := context.WithTimeout(ctx, time.Duration(s.config.MaxCPUTimeMs)*time.Millisecond)
	defer cancel()

	start := time.Now()
	resultCh := make(chan *ExecutionResult, 1)
	errCh := make(chan error, 1)

	go func() {
		result, err := s.runtime.Execute(moduleID, input)
		if err != nil {
			errCh <- err
			return
		}
		resultCh <- result
	}()

	select {
	case <-timeoutCtx.Done():
		return nil, &ResourceUsage{
			CPUTimeMs:    float64(time.Since(start).Milliseconds()),
			WithinLimits: false,
			Violations:   []string{"execution timed out"},
		}, fmt.Errorf("execution timed out after %dms", s.config.MaxCPUTimeMs)
	case err := <-errCh:
		return nil, &ResourceUsage{
			CPUTimeMs:    float64(time.Since(start).Milliseconds()),
			WallTimeMs:   float64(time.Since(start).Milliseconds()),
			WithinLimits: true,
		}, err
	case result := <-resultCh:
		usage := &ResourceUsage{
			MemoryPeakMB: float64(result.MemoryUsedMB),
			CPUTimeMs:    result.DurationMs,
			WallTimeMs:   float64(time.Since(start).Milliseconds()),
			WithinLimits: true,
		}
		if usage.MemoryPeakMB > float64(s.config.MaxMemoryMB) {
			usage.WithinLimits = false
			usage.Violations = append(usage.Violations, "memory peak exceeded limit")
		}
		return result, usage, nil
	}
}

// RegisterVersion adds a new version of a module.
func (s *Sandbox) RegisterVersion(moduleID string, version ModuleVersion) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check module size
	if version.WasmSize > s.config.MaxModuleSize {
		return fmt.Errorf("module size %d exceeds maximum %d bytes", version.WasmSize, s.config.MaxModuleSize)
	}

	version.CreatedAt = time.Now()
	versions := s.versions[moduleID]

	// Deactivate previous versions
	for i := range versions {
		versions[i].IsActive = false
	}

	version.IsActive = true
	versions = append(versions, version)

	// Cap version history
	if len(versions) > s.config.MaxVersions {
		versions = versions[len(versions)-s.config.MaxVersions:]
	}

	s.versions[moduleID] = versions
	return nil
}

// GetVersionHistory returns the version history for a module.
func (s *Sandbox) GetVersionHistory(moduleID string) []ModuleVersion {
	s.mu.RLock()
	defer s.mu.RUnlock()

	versions := s.versions[moduleID]
	result := make([]ModuleVersion, len(versions))
	copy(result, versions)
	return result
}

// RollbackVersion activates a previous version.
func (s *Sandbox) RollbackVersion(moduleID, targetVersion string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	versions, exists := s.versions[moduleID]
	if !exists || len(versions) == 0 {
		return fmt.Errorf("no versions for module %s", moduleID)
	}

	found := false
	for i := range versions {
		if versions[i].Version == targetVersion {
			versions[i].IsActive = true
			found = true
		} else {
			versions[i].IsActive = false
		}
	}
	if !found {
		return fmt.Errorf("version %s not found for module %s", targetVersion, moduleID)
	}

	s.versions[moduleID] = versions

	// Trigger hot-reload notification
	s.notifyHotReload(moduleID)
	return nil
}

// HotReload updates a module and notifies subscribers.
func (s *Sandbox) HotReload(ctx context.Context, moduleID string, mod Module) error {
	if !s.config.EnableHotReload {
		return fmt.Errorf("hot reload disabled")
	}

	if err := s.runtime.UpdateModule(moduleID, mod); err != nil {
		return err
	}

	s.notifyHotReload(moduleID)
	s.logger.Info("hot-reload completed", "module", moduleID)
	return nil
}

// SubscribeHotReload subscribes to hot-reload notifications for a module.
func (s *Sandbox) SubscribeHotReload(moduleID string) <-chan string {
	s.mu.Lock()
	defer s.mu.Unlock()

	ch := make(chan string, 1)
	s.hotReloadSubs[moduleID] = append(s.hotReloadSubs[moduleID], ch)
	return ch
}

func (s *Sandbox) notifyHotReload(moduleID string) {
	subs := s.hotReloadSubs[moduleID]
	for _, ch := range subs {
		select {
		case ch <- moduleID:
		default:
			// Don't block if subscriber isn't listening
		}
	}
}

// GetConfig returns the sandbox configuration.
func (s *Sandbox) GetConfig() SandboxConfig {
	return s.config
}

// SandboxStats returns sandbox statistics.
type SandboxStats struct {
	RuntimeStats RuntimeStats      `json:"runtime_stats"`
	Config       SandboxConfig     `json:"config"`
	Languages    []SupportedLanguage `json:"supported_languages"`
	TotalVersions int              `json:"total_versions"`
}

// Stats returns combined sandbox and runtime statistics.
func (s *Sandbox) Stats() SandboxStats {
	s.mu.RLock()
	totalVersions := 0
	for _, v := range s.versions {
		totalVersions += len(v)
	}
	s.mu.RUnlock()

	return SandboxStats{
		RuntimeStats:  s.runtime.Stats(),
		Config:        s.config,
		Languages:     SupportedLanguages(),
		TotalVersions: totalVersions,
	}
}
