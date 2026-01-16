package edgeruntime

import (
	"fmt"
	"sync"
	"time"
)

// DeviceStatus represents the operational status of an edge device.
type DeviceStatus string

const (
	DeviceStatusOnline         DeviceStatus = "online"
	DeviceStatusOffline        DeviceStatus = "offline"
	DeviceStatusSyncing        DeviceStatus = "syncing"
	DeviceStatusError          DeviceStatus = "error"
	DeviceStatusDecommissioned DeviceStatus = "decommissioned"
)

// DeviceInfo holds metadata about a registered edge device.
type DeviceInfo struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Status       DeviceStatus      `json:"status"`
	Region       string            `json:"region"`
	Tags         []string          `json:"tags"`
	LastSeen     time.Time         `json:"last_seen"`
	SyncState    SyncState         `json:"sync_state"`
	Features     int               `json:"features"`
	PendingSync  int               `json:"pending_sync"`
	TotalSyncs   int64             `json:"total_syncs"`
	ErrorCount   int               `json:"error_count"`
	Metadata     map[string]string `json:"metadata"`
	RegisteredAt time.Time         `json:"registered_at"`
}

// FleetConfig configures the fleet manager.
type FleetConfig struct {
	MaxDevices          int           `json:"max_devices"`
	HealthCheckInterval time.Duration `json:"health_check_interval"`
	OfflineThreshold    time.Duration `json:"offline_threshold"`
	SyncRetryLimit      int           `json:"sync_retry_limit"`
}

// DefaultFleetConfig returns sensible defaults for fleet management.
func DefaultFleetConfig() FleetConfig {
	return FleetConfig{
		MaxDevices:          10000,
		HealthCheckInterval: 60 * time.Second,
		OfflineThreshold:    5 * time.Minute,
		SyncRetryLimit:      3,
	}
}

// FleetHealth summarises the health of all devices.
type FleetHealth struct {
	TotalDevices   int     `json:"total_devices"`
	OnlineDevices  int     `json:"online_devices"`
	OfflineDevices int     `json:"offline_devices"`
	SyncingDevices int     `json:"syncing_devices"`
	ErrorDevices   int     `json:"error_devices"`
	HealthPct      float64 `json:"health_pct"`
}

// FleetStats provides aggregate statistics for the fleet.
type FleetStats struct {
	TotalDevices    int               `json:"total_devices"`
	TotalFeatures   int               `json:"total_features"`
	TotalPendingSync int              `json:"total_pending_sync"`
	AvgSyncLatency  time.Duration     `json:"avg_sync_latency"`
	DevicesByRegion map[string]int    `json:"devices_by_region"`
	DevicesByStatus map[string]int    `json:"devices_by_status"`
}

// FleetManager manages a fleet of edge devices.
type FleetManager struct {
	config  FleetConfig
	devices map[string]*DeviceInfo
	mu      sync.RWMutex
}

// NewFleetManager creates a new FleetManager.
func NewFleetManager(config FleetConfig) *FleetManager {
	return &FleetManager{
		config:  config,
		devices: make(map[string]*DeviceInfo),
	}
}

// RegisterDevice adds an edge device to the fleet.
func (fm *FleetManager) RegisterDevice(info *DeviceInfo) error {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	if info.ID == "" {
		return fmt.Errorf("device ID is required")
	}
	if _, exists := fm.devices[info.ID]; exists {
		return fmt.Errorf("device %q already registered", info.ID)
	}
	if len(fm.devices) >= fm.config.MaxDevices {
		return fmt.Errorf("max devices (%d) reached", fm.config.MaxDevices)
	}

	info.RegisteredAt = time.Now()
	if info.LastSeen.IsZero() {
		info.LastSeen = info.RegisteredAt
	}
	if info.Status == "" {
		info.Status = DeviceStatusOnline
	}
	if info.SyncState == "" {
		info.SyncState = SyncStateOffline
	}
	if info.Metadata == nil {
		info.Metadata = make(map[string]string)
	}

	fm.devices[info.ID] = info
	return nil
}

// DeregisterDevice removes a device from the fleet.
func (fm *FleetManager) DeregisterDevice(id string) error {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	if _, exists := fm.devices[id]; !exists {
		return fmt.Errorf("device %q not found", id)
	}
	delete(fm.devices, id)
	return nil
}

// UpdateStatus changes the status of a device.
func (fm *FleetManager) UpdateStatus(id string, status DeviceStatus) error {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	dev, exists := fm.devices[id]
	if !exists {
		return fmt.Errorf("device %q not found", id)
	}
	dev.Status = status
	dev.LastSeen = time.Now()
	return nil
}

// GetDevice returns a copy of the device info.
func (fm *FleetManager) GetDevice(id string) (*DeviceInfo, error) {
	fm.mu.RLock()
	defer fm.mu.RUnlock()

	dev, exists := fm.devices[id]
	if !exists {
		return nil, fmt.Errorf("device %q not found", id)
	}
	cp := *dev
	return &cp, nil
}

// ListDevices returns all registered devices.
func (fm *FleetManager) ListDevices() []*DeviceInfo {
	fm.mu.RLock()
	defer fm.mu.RUnlock()

	result := make([]*DeviceInfo, 0, len(fm.devices))
	for _, dev := range fm.devices {
		cp := *dev
		result = append(result, &cp)
	}
	return result
}

// ListByStatus returns devices matching the given status.
func (fm *FleetManager) ListByStatus(status DeviceStatus) []*DeviceInfo {
	fm.mu.RLock()
	defer fm.mu.RUnlock()

	var result []*DeviceInfo
	for _, dev := range fm.devices {
		if dev.Status == status {
			cp := *dev
			result = append(result, &cp)
		}
	}
	return result
}

// ListByRegion returns devices in the given region.
func (fm *FleetManager) ListByRegion(region string) []*DeviceInfo {
	fm.mu.RLock()
	defer fm.mu.RUnlock()

	var result []*DeviceInfo
	for _, dev := range fm.devices {
		if dev.Region == region {
			cp := *dev
			result = append(result, &cp)
		}
	}
	return result
}

// HealthCheck evaluates the health of the fleet.
func (fm *FleetManager) HealthCheck() *FleetHealth {
	fm.mu.RLock()
	defer fm.mu.RUnlock()

	h := &FleetHealth{TotalDevices: len(fm.devices)}
	for _, dev := range fm.devices {
		switch dev.Status {
		case DeviceStatusOnline:
			h.OnlineDevices++
		case DeviceStatusOffline:
			h.OfflineDevices++
		case DeviceStatusSyncing:
			h.SyncingDevices++
		case DeviceStatusError:
			h.ErrorDevices++
		}
	}
	if h.TotalDevices > 0 {
		h.HealthPct = float64(h.OnlineDevices+h.SyncingDevices) / float64(h.TotalDevices) * 100.0
	}
	return h
}

// Stats returns aggregate fleet statistics.
func (fm *FleetManager) Stats() *FleetStats {
	fm.mu.RLock()
	defer fm.mu.RUnlock()

	s := &FleetStats{
		TotalDevices:    len(fm.devices),
		DevicesByRegion: make(map[string]int),
		DevicesByStatus: make(map[string]int),
	}
	for _, dev := range fm.devices {
		s.TotalFeatures += dev.Features
		s.TotalPendingSync += dev.PendingSync
		s.DevicesByRegion[dev.Region]++
		s.DevicesByStatus[string(dev.Status)]++
	}
	return s
}
