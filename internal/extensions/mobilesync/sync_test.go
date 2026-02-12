package mobilesync

import (
	"testing"
)

func TestDefaultSyncConfig(t *testing.T) {
	cfg := DefaultSyncConfig()
	if cfg.MaxBatchSize != 1000 {
		t.Errorf("MaxBatchSize = %d, want 1000", cfg.MaxBatchSize)
	}
	if !cfg.CompressionEnabled {
		t.Error("CompressionEnabled should be true by default")
	}
	if cfg.DefaultConflictStrategy != ConflictServerWins {
		t.Errorf("DefaultConflictStrategy = %q, want %q", cfg.DefaultConflictStrategy, ConflictServerWins)
	}
	if !cfg.DeltaSyncEnabled {
		t.Error("DeltaSyncEnabled should be true by default")
	}
	if cfg.MaxOfflineDurationHrs != 72 {
		t.Errorf("MaxOfflineDurationHrs = %d, want 72", cfg.MaxOfflineDurationHrs)
	}
}

func TestNewSyncManager(t *testing.T) {
	m := NewSyncManager(DefaultSyncConfig())
	if m == nil {
		t.Fatal("NewSyncManager returned nil")
	}
}

func TestSyncManager_RegisterDevice(t *testing.T) {
	m := NewSyncManager(DefaultSyncConfig())

	device := &Device{
		ID:       "device-1",
		Name:     "iPhone 15",
		Platform: PlatformIOS,
	}

	err := m.RegisterDevice(device)
	if err != nil {
		t.Fatalf("RegisterDevice: %v", err)
	}

	if device.Status != "online" {
		t.Errorf("Status = %q, want %q", device.Status, "online")
	}
	if device.RegisteredAt.IsZero() {
		t.Error("RegisteredAt should be set")
	}
}

func TestSyncManager_RegisterDevice_EmptyID(t *testing.T) {
	m := NewSyncManager(DefaultSyncConfig())
	err := m.RegisterDevice(&Device{ID: ""})
	if err == nil {
		t.Error("expected error for empty device ID")
	}
}

func TestSyncManager_RegisterDevice_Duplicate(t *testing.T) {
	m := NewSyncManager(DefaultSyncConfig())
	_ = m.RegisterDevice(&Device{ID: "device-1"})
	err := m.RegisterDevice(&Device{ID: "device-1"})
	if err == nil {
		t.Error("expected error for duplicate device")
	}
}

func TestSyncManager_DeregisterDevice(t *testing.T) {
	m := NewSyncManager(DefaultSyncConfig())
	_ = m.RegisterDevice(&Device{ID: "device-1"})

	err := m.DeregisterDevice("device-1")
	if err != nil {
		t.Fatalf("DeregisterDevice: %v", err)
	}

	devices := m.ListDevices()
	if len(devices) != 0 {
		t.Errorf("expected 0 devices, got %d", len(devices))
	}
}

func TestSyncManager_DeregisterDevice_NotFound(t *testing.T) {
	m := NewSyncManager(DefaultSyncConfig())
	err := m.DeregisterDevice("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent device")
	}
}

func TestSyncManager_GetDevice(t *testing.T) {
	m := NewSyncManager(DefaultSyncConfig())
	_ = m.RegisterDevice(&Device{ID: "device-1", Name: "Test Phone", Platform: PlatformAndroid})

	device, err := m.GetDevice("device-1")
	if err != nil {
		t.Fatalf("GetDevice: %v", err)
	}
	if device.Name != "Test Phone" {
		t.Errorf("Name = %q, want %q", device.Name, "Test Phone")
	}
}

func TestSyncManager_GetDevice_NotFound(t *testing.T) {
	m := NewSyncManager(DefaultSyncConfig())
	_, err := m.GetDevice("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent device")
	}
}

func TestSyncManager_ListDevices(t *testing.T) {
	m := NewSyncManager(DefaultSyncConfig())
	_ = m.RegisterDevice(&Device{ID: "d1"})
	_ = m.RegisterDevice(&Device{ID: "d2"})
	_ = m.RegisterDevice(&Device{ID: "d3"})

	devices := m.ListDevices()
	if len(devices) != 3 {
		t.Errorf("expected 3 devices, got %d", len(devices))
	}
}

func TestSyncManager_ProcessSync(t *testing.T) {
	m := NewSyncManager(DefaultSyncConfig())
	_ = m.RegisterDevice(&Device{ID: "device-1", Features: []string{"score", "rank"}})

	resp, err := m.ProcessSync(&SyncRequest{
		DeviceID:      "device-1",
		Mode:          SyncModeDelta,
		ClientVersion: 5,
	})
	if err != nil {
		t.Fatalf("ProcessSync: %v", err)
	}
	if resp.DeviceID != "device-1" {
		t.Errorf("DeviceID = %q, want %q", resp.DeviceID, "device-1")
	}
	if resp.ServerVersion != 6 {
		t.Errorf("ServerVersion = %d, want 6", resp.ServerVersion)
	}
	if resp.NextSyncToken == "" {
		t.Error("expected non-empty NextSyncToken")
	}
}

func TestSyncManager_ProcessSync_UnregisteredDevice(t *testing.T) {
	m := NewSyncManager(DefaultSyncConfig())
	_, err := m.ProcessSync(&SyncRequest{DeviceID: "unknown"})
	if err == nil {
		t.Error("expected error for unregistered device")
	}
}

func TestSyncManager_ResolveConflict_ServerWins(t *testing.T) {
	m := NewSyncManager(DefaultSyncConfig())

	conflict := &SyncConflict{
		FeatureID:     "score",
		EntityKey:     "user:1",
		ClientValue:   0.8,
		ServerValue:   0.9,
		ClientVersion: 1,
		ServerVersion: 2,
		Resolution:    ConflictServerWins,
	}

	resolved, err := m.ResolveConflict(conflict)
	if err != nil {
		t.Fatalf("ResolveConflict: %v", err)
	}
	if resolved.ResolvedValue != 0.9 {
		t.Errorf("ResolvedValue = %v, want 0.9", resolved.ResolvedValue)
	}
}

func TestSyncManager_ResolveConflict_ClientWins(t *testing.T) {
	m := NewSyncManager(DefaultSyncConfig())

	conflict := &SyncConflict{
		FeatureID:   "score",
		ClientValue: 0.8,
		ServerValue: 0.9,
		Resolution:  ConflictClientWins,
	}

	resolved, err := m.ResolveConflict(conflict)
	if err != nil {
		t.Fatalf("ResolveConflict: %v", err)
	}
	if resolved.ResolvedValue != 0.8 {
		t.Errorf("ResolvedValue = %v, want 0.8", resolved.ResolvedValue)
	}
}

func TestSyncManager_ResolveConflict_LastWriteWins(t *testing.T) {
	m := NewSyncManager(DefaultSyncConfig())

	// Client has higher version
	conflict := &SyncConflict{
		FeatureID:     "score",
		ClientValue:   0.8,
		ServerValue:   0.9,
		ClientVersion: 5,
		ServerVersion: 3,
		Resolution:    ConflictLastWriteWins,
	}

	resolved, err := m.ResolveConflict(conflict)
	if err != nil {
		t.Fatalf("ResolveConflict: %v", err)
	}
	if resolved.ResolvedValue != 0.8 {
		t.Errorf("ResolvedValue = %v, want 0.8 (client wins by version)", resolved.ResolvedValue)
	}

	// Server has higher version
	conflict2 := &SyncConflict{
		FeatureID:     "score",
		ClientValue:   0.8,
		ServerValue:   0.9,
		ClientVersion: 2,
		ServerVersion: 5,
		Resolution:    ConflictLastWriteWins,
	}

	resolved2, err := m.ResolveConflict(conflict2)
	if err != nil {
		t.Fatalf("ResolveConflict: %v", err)
	}
	if resolved2.ResolvedValue != 0.9 {
		t.Errorf("ResolvedValue = %v, want 0.9 (server wins by version)", resolved2.ResolvedValue)
	}
}

func TestSyncManager_ResolveConflict_Merge(t *testing.T) {
	m := NewSyncManager(DefaultSyncConfig())

	conflict := &SyncConflict{
		FeatureID:   "score",
		ClientValue: 0.8,
		ServerValue: 0.9,
		Resolution:  ConflictMerge,
	}

	resolved, err := m.ResolveConflict(conflict)
	if err != nil {
		t.Fatalf("ResolveConflict: %v", err)
	}
	// Merge defaults to server value
	if resolved.ResolvedValue != 0.9 {
		t.Errorf("ResolvedValue = %v, want 0.9", resolved.ResolvedValue)
	}
}

func TestSyncManager_ResolveConflict_DefaultStrategy(t *testing.T) {
	m := NewSyncManager(DefaultSyncConfig()) // DefaultConflictStrategy = ConflictServerWins

	conflict := &SyncConflict{
		FeatureID:   "score",
		ClientValue: 0.8,
		ServerValue: 0.9,
		// Resolution not set - uses default
	}

	resolved, err := m.ResolveConflict(conflict)
	if err != nil {
		t.Fatalf("ResolveConflict: %v", err)
	}
	if resolved.ResolvedValue != 0.9 {
		t.Errorf("ResolvedValue = %v, want 0.9 (server wins default)", resolved.ResolvedValue)
	}
}

func TestSyncManager_ResolveConflict_EmptyFeatureID(t *testing.T) {
	m := NewSyncManager(DefaultSyncConfig())
	_, err := m.ResolveConflict(&SyncConflict{FeatureID: ""})
	if err == nil {
		t.Error("expected error for empty feature ID")
	}
}

func TestSyncManager_ListConflicts(t *testing.T) {
	m := NewSyncManager(DefaultSyncConfig())

	_, _ = m.ResolveConflict(&SyncConflict{FeatureID: "f1", ClientValue: 1, ServerValue: 2})
	_, _ = m.ResolveConflict(&SyncConflict{FeatureID: "f2", ClientValue: 3, ServerValue: 4})

	conflicts := m.ListConflicts()
	if len(conflicts) != 2 {
		t.Errorf("expected 2 conflicts, got %d", len(conflicts))
	}
}

func TestSyncManager_EstimateBandwidth(t *testing.T) {
	m := NewSyncManager(DefaultSyncConfig())
	_ = m.RegisterDevice(&Device{ID: "device-1", Features: []string{"f1", "f2", "f3"}})

	estimate, err := m.EstimateBandwidth("device-1")
	if err != nil {
		t.Fatalf("EstimateBandwidth: %v", err)
	}
	if estimate.FeatureCount != 3 {
		t.Errorf("FeatureCount = %d, want 3", estimate.FeatureCount)
	}
	if estimate.EstimatedBytes <= 0 {
		t.Error("expected positive EstimatedBytes")
	}
	if estimate.CompressedBytes >= estimate.EstimatedBytes {
		t.Error("compressed should be less than estimated with compression enabled")
	}
}

func TestSyncManager_EstimateBandwidth_NoFeatures(t *testing.T) {
	m := NewSyncManager(DefaultSyncConfig())
	_ = m.RegisterDevice(&Device{ID: "device-1"})

	estimate, err := m.EstimateBandwidth("device-1")
	if err != nil {
		t.Fatalf("EstimateBandwidth: %v", err)
	}
	// Default estimate for unspecified subscriptions
	if estimate.FeatureCount != 10 {
		t.Errorf("FeatureCount = %d, want 10 (default)", estimate.FeatureCount)
	}
}

func TestSyncManager_EstimateBandwidth_NotFound(t *testing.T) {
	m := NewSyncManager(DefaultSyncConfig())
	_, err := m.EstimateBandwidth("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent device")
	}
}

func TestSyncManager_EstimateBandwidth_NoCompression(t *testing.T) {
	cfg := DefaultSyncConfig()
	cfg.CompressionEnabled = false
	m := NewSyncManager(cfg)
	_ = m.RegisterDevice(&Device{ID: "device-1", Features: []string{"f1"}})

	estimate, err := m.EstimateBandwidth("device-1")
	if err != nil {
		t.Fatalf("EstimateBandwidth: %v", err)
	}
	if estimate.CompressedBytes != estimate.EstimatedBytes {
		t.Error("without compression, compressed should equal estimated")
	}
}

func TestSyncManager_Stats(t *testing.T) {
	m := NewSyncManager(DefaultSyncConfig())
	_ = m.RegisterDevice(&Device{ID: "d1", Status: "online"})
	_ = m.RegisterDevice(&Device{ID: "d2", Status: "offline"})

	_, _ = m.ProcessSync(&SyncRequest{DeviceID: "d1", ClientVersion: 1})

	stats := m.Stats()
	if stats.TotalDevices != 2 {
		t.Errorf("TotalDevices = %d, want 2", stats.TotalDevices)
	}
	if stats.ActiveDevices < 1 {
		t.Errorf("ActiveDevices = %d, want >= 1", stats.ActiveDevices)
	}
	if stats.SyncsCompleted != 1 {
		t.Errorf("SyncsCompleted = %d, want 1", stats.SyncsCompleted)
	}
}

func TestPlatform_Constants(t *testing.T) {
	if PlatformIOS != "ios" {
		t.Errorf("PlatformIOS = %q, want %q", PlatformIOS, "ios")
	}
	if PlatformAndroid != "android" {
		t.Errorf("PlatformAndroid = %q, want %q", PlatformAndroid, "android")
	}
	if PlatformReactNative != "react_native" {
		t.Errorf("PlatformReactNative = %q, want %q", PlatformReactNative, "react_native")
	}
	if PlatformFlutter != "flutter" {
		t.Errorf("PlatformFlutter = %q, want %q", PlatformFlutter, "flutter")
	}
}

func TestSyncMode_Constants(t *testing.T) {
	if SyncModeFull != "full" {
		t.Errorf("SyncModeFull = %q, want %q", SyncModeFull, "full")
	}
	if SyncModeDelta != "delta" {
		t.Errorf("SyncModeDelta = %q, want %q", SyncModeDelta, "delta")
	}
	if SyncModeSelective != "selective" {
		t.Errorf("SyncModeSelective = %q, want %q", SyncModeSelective, "selective")
	}
}
