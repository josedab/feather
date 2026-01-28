package wasmruntime

import (
	"fmt"
	"sync"
	"time"
)

// ModuleStatus represents the deployment state of a WASM module.
type ModuleStatus string

const (
	ModuleRegistered ModuleStatus = "registered"
	ModuleCompiled   ModuleStatus = "compiled"
	ModuleDeployed   ModuleStatus = "deployed"
	ModuleFailed     ModuleStatus = "failed"
)

// DeviceStatus represents the connection state of an edge device.
type DeviceStatus string

const (
	DeviceOnline  DeviceStatus = "online"
	DeviceOffline DeviceStatus = "offline"
	DeviceSyncing DeviceStatus = "syncing"
)

// Module represents a WASM module for edge computation.
type Module struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	WasmBytes   int64             `json:"wasm_bytes"` // Size of WASM binary
	Entrypoint  string            `json:"entrypoint"`
	Inputs      []string          `json:"inputs"`
	Outputs     []string          `json:"outputs"`
	MemoryLimit int64             `json:"memory_limit_bytes"`
	TimeoutMs   int               `json:"timeout_ms"`
	Status      ModuleStatus      `json:"status"`
	Version     int               `json:"version"`
	Tags        map[string]string `json:"tags,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// Device represents an edge device in the fleet.
type Device struct {
	ID              string            `json:"id"`
	Name            string            `json:"name"`
	Region          string            `json:"region,omitempty"`
	Status          DeviceStatus      `json:"status"`
	DeployedModules []string          `json:"deployed_modules"`
	LastSyncAt      time.Time         `json:"last_sync_at"`
	LastHeartbeat   time.Time         `json:"last_heartbeat"`
	CachedFeatures  int64             `json:"cached_features"`
	PendingSync     int64             `json:"pending_sync"`
	Metadata        map[string]string `json:"metadata,omitempty"`
	CreatedAt       time.Time         `json:"created_at"`
}

// SyncResult represents the outcome of a device synchronization.
type SyncResult struct {
	DeviceID        string        `json:"device_id"`
	FeaturesUpdated int64         `json:"features_updated"`
	ModulesDeployed int           `json:"modules_deployed"`
	Duration        time.Duration `json:"duration_ns"`
	Success         bool          `json:"success"`
	Error           string        `json:"error,omitempty"`
	Timestamp       time.Time     `json:"timestamp"`
}

// EdgeManagerConfig configures the edge manager.
type EdgeManagerConfig struct {
	MaxModules       int           `json:"max_modules"`
	MaxDevices       int           `json:"max_devices"`
	DefaultMemLimit  int64         `json:"default_memory_limit"`
	DefaultTimeout   int           `json:"default_timeout_ms"`
	HeartbeatTimeout time.Duration `json:"heartbeat_timeout"`
	SyncInterval     time.Duration `json:"sync_interval"`
}

// DefaultEdgeManagerConfig returns sensible defaults.
func DefaultEdgeManagerConfig() EdgeManagerConfig {
	return EdgeManagerConfig{
		MaxModules:       10000,
		MaxDevices:       100000,
		DefaultMemLimit:  64 * 1024 * 1024, // 64MB
		DefaultTimeout:   5000,
		HeartbeatTimeout: 60 * time.Second,
		SyncInterval:     30 * time.Second,
	}
}

// EdgeManager manages edge device fleet and module deployment.
type EdgeManager struct {
	mu      sync.RWMutex
	config  EdgeManagerConfig
	modules map[string]*Module
	devices map[string]*Device
	syncs   []SyncResult
}

// NewEdgeManager creates a new edge manager.
func NewEdgeManager(config EdgeManagerConfig) *EdgeManager {
	if config.MaxModules == 0 {
		config = DefaultEdgeManagerConfig()
	}
	return &EdgeManager{
		config:  config,
		modules: make(map[string]*Module),
		devices: make(map[string]*Device),
	}
}

// RegisterModule registers a WASM module.
func (m *EdgeManager) RegisterModule(mod Module) (*Module, error) {
	if mod.ID == "" || mod.Name == "" {
		return nil, fmt.Errorf("%w: id and name are required", ErrInvalidModule)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.modules[mod.ID]; exists {
		return nil, ErrModuleExists
	}
	if len(m.modules) >= m.config.MaxModules {
		return nil, fmt.Errorf("max modules reached (%d)", m.config.MaxModules)
	}

	now := time.Now()
	mod.Status = ModuleRegistered
	mod.Version = 1
	mod.CreatedAt = now
	mod.UpdatedAt = now
	if mod.MemoryLimit == 0 {
		mod.MemoryLimit = m.config.DefaultMemLimit
	}
	if mod.TimeoutMs == 0 {
		mod.TimeoutMs = m.config.DefaultTimeout
	}
	if mod.Entrypoint == "" {
		mod.Entrypoint = "compute"
	}

	m.modules[mod.ID] = &mod
	return &mod, nil
}

// GetModule returns a module by ID.
func (m *EdgeManager) GetModule(id string) (*Module, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	mod, exists := m.modules[id]
	if !exists {
		return nil, ErrModuleNotFound
	}
	return mod, nil
}

// ListModules returns all registered modules.
func (m *EdgeManager) ListModules() []Module {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]Module, 0, len(m.modules))
	for _, mod := range m.modules {
		result = append(result, *mod)
	}
	return result
}

// DeleteModule removes a WASM module.
func (m *EdgeManager) DeleteModule(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.modules[id]; !exists {
		return ErrModuleNotFound
	}
	delete(m.modules, id)
	return nil
}

// RegisterDevice registers an edge device.
func (m *EdgeManager) RegisterDevice(dev Device) (*Device, error) {
	if dev.ID == "" || dev.Name == "" {
		return nil, fmt.Errorf("id and name are required")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.devices[dev.ID]; exists {
		return nil, ErrDeviceExists
	}
	if len(m.devices) >= m.config.MaxDevices {
		return nil, fmt.Errorf("max devices reached (%d)", m.config.MaxDevices)
	}

	now := time.Now()
	dev.Status = DeviceOnline
	dev.CreatedAt = now
	dev.LastHeartbeat = now
	dev.LastSyncAt = now
	if dev.DeployedModules == nil {
		dev.DeployedModules = []string{}
	}

	m.devices[dev.ID] = &dev
	return &dev, nil
}

// GetDevice returns a device by ID.
func (m *EdgeManager) GetDevice(id string) (*Device, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	dev, exists := m.devices[id]
	if !exists {
		return nil, ErrDeviceNotFound
	}
	return dev, nil
}

// ListDevices returns all registered devices.
func (m *EdgeManager) ListDevices() []Device {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]Device, 0, len(m.devices))
	for _, dev := range m.devices {
		result = append(result, *dev)
	}
	return result
}

// DeployModule deploys a module to a device.
func (m *EdgeManager) DeployModule(deviceID, moduleID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	dev, exists := m.devices[deviceID]
	if !exists {
		return ErrDeviceNotFound
	}
	mod, exists := m.modules[moduleID]
	if !exists {
		return ErrModuleNotFound
	}

	// Check if already deployed
	for _, id := range dev.DeployedModules {
		if id == moduleID {
			return nil // idempotent
		}
	}

	dev.DeployedModules = append(dev.DeployedModules, moduleID)
	mod.Status = ModuleDeployed
	mod.UpdatedAt = time.Now()
	return nil
}

// SyncDevice triggers synchronization for a device.
func (m *EdgeManager) SyncDevice(deviceID string) (*SyncResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	dev, exists := m.devices[deviceID]
	if !exists {
		return nil, ErrDeviceNotFound
	}

	if dev.Status == DeviceOffline {
		return nil, ErrDeviceOffline
	}

	start := time.Now()
	dev.Status = DeviceSyncing
	dev.LastSyncAt = time.Now()
	dev.PendingSync = 0
	dev.Status = DeviceOnline

	result := &SyncResult{
		DeviceID:        deviceID,
		FeaturesUpdated: dev.CachedFeatures,
		ModulesDeployed: len(dev.DeployedModules),
		Duration:        time.Since(start),
		Success:         true,
		Timestamp:       time.Now(),
	}

	m.syncs = append(m.syncs, *result)
	if len(m.syncs) > 10000 {
		m.syncs = m.syncs[1:]
	}

	return result, nil
}

// Heartbeat updates the heartbeat timestamp for a device.
func (m *EdgeManager) Heartbeat(deviceID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	dev, exists := m.devices[deviceID]
	if !exists {
		return ErrDeviceNotFound
	}
	dev.LastHeartbeat = time.Now()
	dev.Status = DeviceOnline
	return nil
}

// Stats returns edge fleet statistics.
func (m *EdgeManager) Stats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	online := 0
	for _, dev := range m.devices {
		if dev.Status == DeviceOnline {
			online++
		}
	}

	return map[string]interface{}{
		"total_modules":  len(m.modules),
		"total_devices":  len(m.devices),
		"online_devices": online,
		"total_syncs":    len(m.syncs),
	}
}
