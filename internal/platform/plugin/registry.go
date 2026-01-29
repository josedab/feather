package plugin

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
)

// RegistryConfig holds configuration for the plugin registry.
type RegistryConfig struct {
	MaxPlugins    int           `json:"max_plugins"`
	PluginTimeout time.Duration `json:"plugin_timeout"`
	EnableSandbox bool          `json:"enable_sandbox"`
	AllowedHooks  []HookPoint   `json:"allowed_hooks"`
}

// DefaultRegistryConfig returns a RegistryConfig with sensible defaults.
func DefaultRegistryConfig() RegistryConfig {
	return RegistryConfig{
		MaxPlugins:    64,
		PluginTimeout: 30 * time.Second,
		EnableSandbox: false,
		AllowedHooks: []HookPoint{
			HookPreRead, HookPostRead,
			HookPreWrite, HookPostWrite,
			HookPreIngest, HookPostIngest,
			HookPreTransform, HookPostTransform,
			HookOnError, HookOnStartup, HookOnShutdown,
		},
	}
}

// PluginType identifies the category of a plugin.
type PluginType string

const (
	PluginTypeStorage   PluginType = "storage"
	PluginTypeIngestion PluginType = "ingestion"
	PluginTypeTransform PluginType = "transform"
	PluginTypeAuth      PluginType = "auth"
	PluginTypeExport    PluginType = "export"
	PluginTypeCustom    PluginType = "custom"
)

// PluginStatus represents the current state of a plugin.
type PluginStatus string

const (
	PluginStatusLoaded   PluginStatus = "loaded"
	PluginStatusActive   PluginStatus = "active"
	PluginStatusDisabled PluginStatus = "disabled"
	PluginStatusError    PluginStatus = "error"
)

// HookPoint identifies a specific extension point in the feature store pipeline.
type HookPoint string

const (
	HookPreRead       HookPoint = "pre_read"
	HookPostRead      HookPoint = "post_read"
	HookPreWrite      HookPoint = "pre_write"
	HookPostWrite     HookPoint = "post_write"
	HookPreIngest     HookPoint = "pre_ingest"
	HookPostIngest    HookPoint = "post_ingest"
	HookPreTransform  HookPoint = "pre_transform"
	HookPostTransform HookPoint = "post_transform"
	HookOnError       HookPoint = "on_error"
	HookOnStartup     HookPoint = "on_startup"
	HookOnShutdown    HookPoint = "on_shutdown"
)

// Plugin represents a registered plugin instance.
type Plugin struct {
	ID           string                 `json:"id"`
	Name         string                 `json:"name"`
	Version      string                 `json:"version"`
	Description  string                 `json:"description,omitempty"`
	Author       string                 `json:"author,omitempty"`
	Type         PluginType             `json:"type"`
	Status       PluginStatus           `json:"status"`
	Config       map[string]interface{} `json:"config,omitempty"`
	Hooks        []HookPoint            `json:"hooks"`
	Capabilities []string               `json:"capabilities,omitempty"`
	Metadata     map[string]string      `json:"metadata,omitempty"`
	LoadedAt     time.Time              `json:"loaded_at"`
	UpdatedAt    time.Time              `json:"updated_at"`
	Metrics      *PluginMetrics         `json:"metrics"`
}

// HookRegistration binds a handler to a hook point with a priority.
type HookRegistration struct {
	PluginID string
	Hook     HookPoint
	Priority int
	Handler  HookHandler
}

// HookHandler is the function signature for hook callbacks.
type HookHandler func(ctx context.Context, data *HookData) (*HookData, error)

// HookData carries data through the hook execution chain.
type HookData struct {
	EntityKey string                 `json:"entity_key,omitempty"`
	Features  map[string]interface{} `json:"features,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	Error     error                  `json:"-"`
}

// PluginMetrics tracks runtime statistics for a plugin.
type PluginMetrics struct {
	InvocationCount int64         `json:"invocation_count"`
	ErrorCount      int64         `json:"error_count"`
	TotalDuration   time.Duration `json:"total_duration"`
	AvgDuration     time.Duration `json:"avg_duration"`
	LastInvoked     time.Time     `json:"last_invoked"`
}

// RegistryStats provides an overview of the plugin registry state.
type RegistryStats struct {
	TotalPlugins   int            `json:"total_plugins"`
	ActivePlugins  int            `json:"active_plugins"`
	TotalHooks     int            `json:"total_hooks"`
	PluginsByType  map[string]int `json:"plugins_by_type"`
	PluginsByState map[string]int `json:"plugins_by_state"`
}

// Registry manages plugin registration, lifecycle, and hook execution.
type Registry struct {
	config  RegistryConfig
	plugins map[string]*Plugin
	hooks   map[HookPoint][]*HookRegistration
	mu      sync.RWMutex
}

// NewRegistry creates a new plugin registry with the given configuration.
func NewRegistry(config RegistryConfig) *Registry {
	if config.MaxPlugins <= 0 {
		config.MaxPlugins = DefaultRegistryConfig().MaxPlugins
	}
	if config.PluginTimeout <= 0 {
		config.PluginTimeout = DefaultRegistryConfig().PluginTimeout
	}
	return &Registry{
		config:  config,
		plugins: make(map[string]*Plugin),
		hooks:   make(map[HookPoint][]*HookRegistration),
	}
}

// Register adds a plugin to the registry.
func (r *Registry) Register(ctx context.Context, plugin *Plugin) error {
	if plugin == nil {
		return fmt.Errorf("registering plugin: plugin must not be nil")
	}
	if plugin.Name == "" {
		return fmt.Errorf("registering plugin: name is required")
	}
	if plugin.Version == "" {
		return fmt.Errorf("registering plugin: version is required")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if len(r.plugins) >= r.config.MaxPlugins {
		return fmt.Errorf("registering plugin %q: maximum plugin count (%d) reached", plugin.Name, r.config.MaxPlugins)
	}

	if plugin.ID == "" {
		plugin.ID = uuid.New().String()
	}

	if _, exists := r.plugins[plugin.ID]; exists {
		return fmt.Errorf("registering plugin: plugin with id %q already exists", plugin.ID)
	}

	now := time.Now()
	plugin.Status = PluginStatusLoaded
	plugin.LoadedAt = now
	plugin.UpdatedAt = now
	plugin.Metrics = &PluginMetrics{}

	r.plugins[plugin.ID] = plugin
	return nil
}

// Unregister removes a plugin and its hooks from the registry.
func (r *Registry) Unregister(ctx context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.plugins[id]; !exists {
		return fmt.Errorf("unregistering plugin: plugin %q not found", id)
	}

	// Remove all hooks registered by this plugin.
	for hp, regs := range r.hooks {
		filtered := make([]*HookRegistration, 0, len(regs))
		for _, reg := range regs {
			if reg.PluginID != id {
				filtered = append(filtered, reg)
			}
		}
		r.hooks[hp] = filtered
	}

	delete(r.plugins, id)
	return nil
}

// Get returns a plugin by ID.
func (r *Registry) Get(id string) (*Plugin, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	p, exists := r.plugins[id]
	if !exists {
		return nil, fmt.Errorf("getting plugin: plugin %q not found", id)
	}
	return p, nil
}

// List returns all registered plugins.
func (r *Registry) List() []*Plugin {
	r.mu.RLock()
	defer r.mu.RUnlock()

	plugins := make([]*Plugin, 0, len(r.plugins))
	for _, p := range r.plugins {
		plugins = append(plugins, p)
	}
	return plugins
}

// ListByType returns all plugins of the given type.
func (r *Registry) ListByType(pluginType PluginType) []*Plugin {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var plugins []*Plugin
	for _, p := range r.plugins {
		if p.Type == pluginType {
			plugins = append(plugins, p)
		}
	}
	return plugins
}

// Enable activates a plugin.
func (r *Registry) Enable(ctx context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	p, exists := r.plugins[id]
	if !exists {
		return fmt.Errorf("enabling plugin: plugin %q not found", id)
	}

	p.Status = PluginStatusActive
	p.UpdatedAt = time.Now()
	return nil
}

// Disable deactivates a plugin.
func (r *Registry) Disable(ctx context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	p, exists := r.plugins[id]
	if !exists {
		return fmt.Errorf("disabling plugin: plugin %q not found", id)
	}

	p.Status = PluginStatusDisabled
	p.UpdatedAt = time.Now()
	return nil
}

// RegisterHook registers a hook handler for a specific hook point.
func (r *Registry) RegisterHook(pluginID string, hook HookPoint, priority int, handler HookHandler) error {
	if handler == nil {
		return fmt.Errorf("registering hook: handler must not be nil")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.plugins[pluginID]; !exists {
		return fmt.Errorf("registering hook: plugin %q not found", pluginID)
	}

	if len(r.config.AllowedHooks) > 0 && !r.isHookAllowed(hook) {
		return fmt.Errorf("registering hook: hook %q is not allowed", hook)
	}

	reg := &HookRegistration{
		PluginID: pluginID,
		Hook:     hook,
		Priority: priority,
		Handler:  handler,
	}

	r.hooks[hook] = append(r.hooks[hook], reg)

	// Sort by priority (lower value = higher priority = executed first).
	sort.Slice(r.hooks[hook], func(i, j int) bool {
		return r.hooks[hook][i].Priority < r.hooks[hook][j].Priority
	})

	return nil
}

// ExecuteHooks runs all registered hooks for the given hook point in priority order.
func (r *Registry) ExecuteHooks(ctx context.Context, hook HookPoint, data *HookData) (*HookData, error) {
	r.mu.RLock()
	regs := make([]*HookRegistration, len(r.hooks[hook]))
	copy(regs, r.hooks[hook])
	// Build a set of active plugin IDs for quick lookup.
	activePlugins := make(map[string]bool, len(r.plugins))
	for id, p := range r.plugins {
		if p.Status == PluginStatusActive {
			activePlugins[id] = true
		}
	}
	r.mu.RUnlock()

	current := data
	for _, reg := range regs {
		if !activePlugins[reg.PluginID] {
			continue
		}

		start := time.Now()
		result, err := reg.Handler(ctx, current)
		elapsed := time.Since(start)

		r.recordMetrics(reg.PluginID, elapsed, err != nil)

		if err != nil {
			return current, fmt.Errorf("executing hook %q for plugin %q: %w", hook, reg.PluginID, err)
		}
		if result != nil {
			current = result
		}
	}
	return current, nil
}

// GetPluginMetrics returns metrics for a specific plugin.
func (r *Registry) GetPluginMetrics(id string) (*PluginMetrics, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	p, exists := r.plugins[id]
	if !exists {
		return nil, fmt.Errorf("getting plugin metrics: plugin %q not found", id)
	}
	// Return a copy.
	m := *p.Metrics
	return &m, nil
}

// Stats returns aggregate statistics about the registry.
func (r *Registry) Stats() *RegistryStats {
	r.mu.RLock()
	defer r.mu.RUnlock()

	stats := &RegistryStats{
		TotalPlugins:   len(r.plugins),
		PluginsByType:  make(map[string]int),
		PluginsByState: make(map[string]int),
	}

	for _, p := range r.plugins {
		stats.PluginsByType[string(p.Type)]++
		stats.PluginsByState[string(p.Status)]++
		if p.Status == PluginStatusActive {
			stats.ActivePlugins++
		}
	}

	for _, regs := range r.hooks {
		stats.TotalHooks += len(regs)
	}

	return stats
}

// Close shuts down the registry and clears all plugins and hooks.
func (r *Registry) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.plugins = make(map[string]*Plugin)
	r.hooks = make(map[HookPoint][]*HookRegistration)
	return nil
}

func (r *Registry) isHookAllowed(hook HookPoint) bool {
	for _, h := range r.config.AllowedHooks {
		if h == hook {
			return true
		}
	}
	return false
}

func (r *Registry) recordMetrics(pluginID string, elapsed time.Duration, isError bool) {
	r.mu.RLock()
	p, exists := r.plugins[pluginID]
	r.mu.RUnlock()
	if !exists {
		return
	}

	count := atomic.AddInt64(&p.Metrics.InvocationCount, 1)
	if isError {
		atomic.AddInt64(&p.Metrics.ErrorCount, 1)
	}

	r.mu.Lock()
	p.Metrics.TotalDuration += elapsed
	if count > 0 {
		p.Metrics.AvgDuration = p.Metrics.TotalDuration / time.Duration(count)
	}
	p.Metrics.LastInvoked = time.Now()
	r.mu.Unlock()
}
