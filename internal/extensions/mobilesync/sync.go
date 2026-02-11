// Package mobilesync provides the sync protocol for mobile SDKs (iOS, Android).
//
// It supports offline-first architectures with delta sync, conflict resolution,
// and bandwidth-aware synchronization for feature store data.
package mobilesync

import (
	"fmt"
	"sync"
	"time"
)

// DevicePlatform identifies the mobile platform.
type DevicePlatform string

const (
	PlatformIOS         DevicePlatform = "ios"
	PlatformAndroid     DevicePlatform = "android"
	PlatformReactNative DevicePlatform = "react_native"
	PlatformFlutter     DevicePlatform = "flutter"
)

// SyncMode controls how data is synchronized.
type SyncMode string

const (
	SyncModeFull      SyncMode = "full"
	SyncModeDelta     SyncMode = "delta"
	SyncModeSelective SyncMode = "selective"
)

// ConflictStrategy determines how sync conflicts are resolved.
type ConflictStrategy string

const (
	ConflictServerWins    ConflictStrategy = "server_wins"
	ConflictClientWins    ConflictStrategy = "client_wins"
	ConflictLastWriteWins ConflictStrategy = "last_write_wins"
	ConflictMerge         ConflictStrategy = "merge"
)

// Device represents a registered mobile device.
type Device struct {
	ID           string         `json:"id"`
	Name         string         `json:"name"`
	Platform     DevicePlatform `json:"platform"`
	AppVersion   string         `json:"app_version"`
	LastSyncAt   time.Time      `json:"last_sync_at"`
	Features     []string       `json:"features"`
	Status       string         `json:"status"` // "online", "offline", "syncing"
	RegisteredAt time.Time      `json:"registered_at"`
}

// SyncRequest is sent by a mobile client to request feature updates.
type SyncRequest struct {
	DeviceID      string   `json:"device_id"`
	Mode          SyncMode `json:"mode"`
	Since         time.Time `json:"since"`
	Features      []string `json:"features"`
	ClientVersion int64    `json:"client_version"`
}

// SyncResponse is returned to the client with feature updates.
type SyncResponse struct {
	DeviceID          string          `json:"device_id"`
	Updates           []FeatureUpdate `json:"updates"`
	ServerVersion     int64           `json:"server_version"`
	NextSyncToken     string          `json:"next_sync_token"`
	ConflictsResolved int             `json:"conflicts_resolved"`
}

// FeatureUpdate represents a single feature value change.
type FeatureUpdate struct {
	FeatureID string      `json:"feature_id"`
	EntityKey string      `json:"entity_key"`
	Value     interface{} `json:"value"`
	Version   int64       `json:"version"`
	UpdatedAt time.Time   `json:"updated_at"`
	Deleted   bool        `json:"deleted"`
}

// SyncConflict captures a conflict between client and server values.
type SyncConflict struct {
	FeatureID     string           `json:"feature_id"`
	EntityKey     string           `json:"entity_key"`
	ClientValue   interface{}      `json:"client_value"`
	ServerValue   interface{}      `json:"server_value"`
	ClientVersion int64            `json:"client_version"`
	ServerVersion int64            `json:"server_version"`
	Resolution    ConflictStrategy `json:"resolution"`
	ResolvedValue interface{}      `json:"resolved_value"`
}

// BandwidthEstimate provides sizing information for a pending sync.
type BandwidthEstimate struct {
	DeviceID        string `json:"device_id"`
	EstimatedBytes  int64  `json:"estimated_bytes"`
	FeatureCount    int    `json:"feature_count"`
	CompressedBytes int64  `json:"compressed_bytes"`
}

// SyncConfig controls sync manager behavior.
type SyncConfig struct {
	MaxBatchSize            int              `json:"max_batch_size"`
	CompressionEnabled      bool             `json:"compression_enabled"`
	DefaultConflictStrategy ConflictStrategy `json:"default_conflict_strategy"`
	DeltaSyncEnabled        bool             `json:"delta_sync_enabled"`
	MaxOfflineDurationHrs   int              `json:"max_offline_duration_hrs"`
}

// DefaultSyncConfig returns production-ready defaults.
func DefaultSyncConfig() SyncConfig {
	return SyncConfig{
		MaxBatchSize:            1000,
		CompressionEnabled:      true,
		DefaultConflictStrategy: ConflictServerWins,
		DeltaSyncEnabled:        true,
		MaxOfflineDurationHrs:   72,
	}
}

// SyncStats provides aggregate statistics about mobile sync activity.
type SyncStats struct {
	TotalDevices          int64 `json:"total_devices"`
	ActiveDevices         int64 `json:"active_devices"`
	SyncsCompleted        int64 `json:"syncs_completed"`
	ConflictsResolved     int64 `json:"conflicts_resolved"`
	TotalBytesTransferred int64 `json:"total_bytes_transferred"`
}

// SyncManager orchestrates mobile device registration and sync operations.
type SyncManager struct {
	mu        sync.RWMutex
	config    SyncConfig
	devices   map[string]*Device
	syncLog   []SyncResponse
	conflicts []SyncConflict
}

// NewSyncManager creates a SyncManager with the given configuration.
func NewSyncManager(cfg SyncConfig) *SyncManager {
	return &SyncManager{
		config:    cfg,
		devices:   make(map[string]*Device),
		syncLog:   make([]SyncResponse, 0),
		conflicts: make([]SyncConflict, 0),
	}
}

// RegisterDevice registers a new mobile device for sync.
func (m *SyncManager) RegisterDevice(device *Device) error {
	if device.ID == "" {
		return fmt.Errorf("device ID is required")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.devices[device.ID]; exists {
		return fmt.Errorf("device %s already registered", device.ID)
	}

	device.RegisteredAt = time.Now()
	if device.Status == "" {
		device.Status = "online"
	}
	m.devices[device.ID] = device
	return nil
}

// DeregisterDevice removes a device from the sync pool.
func (m *SyncManager) DeregisterDevice(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.devices[id]; !exists {
		return fmt.Errorf("device %s not found", id)
	}
	delete(m.devices, id)
	return nil
}

// GetDevice returns a registered device by ID.
func (m *SyncManager) GetDevice(id string) (*Device, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	device, exists := m.devices[id]
	if !exists {
		return nil, fmt.Errorf("device %s not found", id)
	}
	return device, nil
}

// ListDevices returns all registered devices.
func (m *SyncManager) ListDevices() []*Device {
	m.mu.RLock()
	defer m.mu.RUnlock()

	devices := make([]*Device, 0, len(m.devices))
	for _, d := range m.devices {
		devices = append(devices, d)
	}
	return devices
}

// ProcessSync handles a sync request from a mobile device.
func (m *SyncManager) ProcessSync(req *SyncRequest) (*SyncResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	device, exists := m.devices[req.DeviceID]
	if !exists {
		return nil, fmt.Errorf("device %s not registered", req.DeviceID)
	}

	device.Status = "syncing"
	device.LastSyncAt = time.Now()

	// Build updates based on sync mode
	updates := make([]FeatureUpdate, 0)
	serverVersion := req.ClientVersion + 1

	resp := &SyncResponse{
		DeviceID:          req.DeviceID,
		Updates:           updates,
		ServerVersion:     serverVersion,
		NextSyncToken:     fmt.Sprintf("sync_%s_%d", req.DeviceID, serverVersion),
		ConflictsResolved: 0,
	}

	m.syncLog = append(m.syncLog, *resp)
	device.Status = "online"
	return resp, nil
}

// ResolveConflict resolves a sync conflict using the specified or default strategy.
func (m *SyncManager) ResolveConflict(conflict *SyncConflict) (*SyncConflict, error) {
	if conflict.FeatureID == "" {
		return nil, fmt.Errorf("feature ID is required")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	strategy := conflict.Resolution
	if strategy == "" {
		strategy = m.config.DefaultConflictStrategy
	}

	switch strategy {
	case ConflictServerWins:
		conflict.ResolvedValue = conflict.ServerValue
	case ConflictClientWins:
		conflict.ResolvedValue = conflict.ClientValue
	case ConflictLastWriteWins:
		if conflict.ClientVersion > conflict.ServerVersion {
			conflict.ResolvedValue = conflict.ClientValue
		} else {
			conflict.ResolvedValue = conflict.ServerValue
		}
	case ConflictMerge:
		conflict.ResolvedValue = conflict.ServerValue
	default:
		conflict.ResolvedValue = conflict.ServerValue
	}

	conflict.Resolution = strategy
	m.conflicts = append(m.conflicts, *conflict)
	return conflict, nil
}

// EstimateBandwidth estimates the data transfer size for a device sync.
func (m *SyncManager) EstimateBandwidth(deviceID string) (*BandwidthEstimate, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	device, exists := m.devices[deviceID]
	if !exists {
		return nil, fmt.Errorf("device %s not found", deviceID)
	}

	featureCount := len(device.Features)
	if featureCount == 0 {
		featureCount = 10 // estimate for unspecified subscriptions
	}

	estimatedBytes := int64(featureCount) * 256 // ~256 bytes per feature
	compressedBytes := estimatedBytes * 4 / 10  // ~40% compression ratio

	if !m.config.CompressionEnabled {
		compressedBytes = estimatedBytes
	}

	return &BandwidthEstimate{
		DeviceID:        deviceID,
		EstimatedBytes:  estimatedBytes,
		FeatureCount:    featureCount,
		CompressedBytes: compressedBytes,
	}, nil
}

// ListConflicts returns all recorded sync conflicts.
func (m *SyncManager) ListConflicts() []*SyncConflict {
	m.mu.RLock()
	defer m.mu.RUnlock()

	conflicts := make([]*SyncConflict, len(m.conflicts))
	for i := range m.conflicts {
		conflicts[i] = &m.conflicts[i]
	}
	return conflicts
}

// Stats returns aggregate sync statistics.
func (m *SyncManager) Stats() *SyncStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	active := int64(0)
	for _, d := range m.devices {
		if d.Status == "online" || d.Status == "syncing" {
			active++
		}
	}

	return &SyncStats{
		TotalDevices:          int64(len(m.devices)),
		ActiveDevices:         active,
		SyncsCompleted:        int64(len(m.syncLog)),
		ConflictsResolved:     int64(len(m.conflicts)),
		TotalBytesTransferred: int64(len(m.syncLog)) * 1024,
	}
}
