package edgeruntime

import (
	"sync"
)

// BandwidthConfig configures the bandwidth monitor.
type BandwidthConfig struct {
	MaxBytesPerSecond int64   `json:"max_bytes_per_second"`
	SamplingRate      float64 `json:"sampling_rate"`
	CompressionLevel  int     `json:"compression_level"`
}

// DefaultBandwidthConfig returns sensible defaults.
func DefaultBandwidthConfig() BandwidthConfig {
	return BandwidthConfig{
		MaxBytesPerSecond: 1 << 20, // 1 MB/s
		SamplingRate:      1.0,
		CompressionLevel:  6,
	}
}

// DeviceBandwidth holds bandwidth metrics for a single device.
type DeviceBandwidth struct {
	DeviceID        string `json:"device_id"`
	BytesUp         int64  `json:"bytes_up"`
	BytesDown       int64  `json:"bytes_down"`
	TotalBytes      int64  `json:"total_bytes"`
	TransferCount   int    `json:"transfer_count"`
	AvgTransferSize int64  `json:"avg_transfer_size"`
}

// TotalBandwidth holds aggregate bandwidth metrics across all devices.
type TotalBandwidth struct {
	TotalBytesUp      int64 `json:"total_bytes_up"`
	TotalBytesDown    int64 `json:"total_bytes_down"`
	ActiveDevices     int   `json:"active_devices"`
	AvgBytesPerDevice int64 `json:"avg_bytes_per_device"`
}

type deviceBW struct {
	bytesUp       int64
	bytesDown     int64
	transferCount int
}

// BandwidthMonitor tracks bandwidth usage across edge devices.
type BandwidthMonitor struct {
	config  BandwidthConfig
	devices map[string]*deviceBW
	mu      sync.RWMutex
}

// NewBandwidthMonitor creates a new BandwidthMonitor.
func NewBandwidthMonitor(config BandwidthConfig) *BandwidthMonitor {
	return &BandwidthMonitor{
		config:  config,
		devices: make(map[string]*deviceBW),
	}
}

// RecordTransfer records a data transfer for a device.
func (bm *BandwidthMonitor) RecordTransfer(deviceID string, bytesUp, bytesDown int64) {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	d, ok := bm.devices[deviceID]
	if !ok {
		d = &deviceBW{}
		bm.devices[deviceID] = d
	}
	d.bytesUp += bytesUp
	d.bytesDown += bytesDown
	d.transferCount++
}

// GetUsage returns bandwidth metrics for a specific device.
func (bm *BandwidthMonitor) GetUsage(deviceID string) *DeviceBandwidth {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	d, ok := bm.devices[deviceID]
	if !ok {
		return &DeviceBandwidth{DeviceID: deviceID}
	}

	total := d.bytesUp + d.bytesDown
	avg := int64(0)
	if d.transferCount > 0 {
		avg = total / int64(d.transferCount)
	}
	return &DeviceBandwidth{
		DeviceID:        deviceID,
		BytesUp:         d.bytesUp,
		BytesDown:       d.bytesDown,
		TotalBytes:      total,
		TransferCount:   d.transferCount,
		AvgTransferSize: avg,
	}
}

// GetTotalUsage returns aggregate bandwidth metrics.
func (bm *BandwidthMonitor) GetTotalUsage() *TotalBandwidth {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	t := &TotalBandwidth{ActiveDevices: len(bm.devices)}
	for _, d := range bm.devices {
		t.TotalBytesUp += d.bytesUp
		t.TotalBytesDown += d.bytesDown
	}
	if t.ActiveDevices > 0 {
		t.AvgBytesPerDevice = (t.TotalBytesUp + t.TotalBytesDown) / int64(t.ActiveDevices)
	}
	return t
}

// ShouldThrottle returns true if the device has exceeded the bandwidth limit.
func (bm *BandwidthMonitor) ShouldThrottle(deviceID string) bool {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	d, ok := bm.devices[deviceID]
	if !ok {
		return false
	}
	return (d.bytesUp + d.bytesDown) > bm.config.MaxBytesPerSecond
}
