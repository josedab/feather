package edgeruntime

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFleetManager_RegisterAndGet(t *testing.T) {
	fm := NewFleetManager(DefaultFleetConfig())

	err := fm.RegisterDevice(&DeviceInfo{ID: "d1", Name: "edge-1", Region: "us-east"})
	require.NoError(t, err)

	dev, err := fm.GetDevice("d1")
	require.NoError(t, err)
	assert.Equal(t, "d1", dev.ID)
	assert.Equal(t, "edge-1", dev.Name)
	assert.Equal(t, DeviceStatusOnline, dev.Status)
	assert.False(t, dev.RegisteredAt.IsZero())
}

func TestFleetManager_RegisterDuplicate(t *testing.T) {
	fm := NewFleetManager(DefaultFleetConfig())
	_ = fm.RegisterDevice(&DeviceInfo{ID: "d1"})

	err := fm.RegisterDevice(&DeviceInfo{ID: "d1"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already registered")
}

func TestFleetManager_RegisterEmptyID(t *testing.T) {
	fm := NewFleetManager(DefaultFleetConfig())
	err := fm.RegisterDevice(&DeviceInfo{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ID is required")
}

func TestFleetManager_MaxDevices(t *testing.T) {
	cfg := DefaultFleetConfig()
	cfg.MaxDevices = 2
	fm := NewFleetManager(cfg)

	_ = fm.RegisterDevice(&DeviceInfo{ID: "d1"})
	_ = fm.RegisterDevice(&DeviceInfo{ID: "d2"})
	err := fm.RegisterDevice(&DeviceInfo{ID: "d3"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "max devices")
}

func TestFleetManager_Deregister(t *testing.T) {
	fm := NewFleetManager(DefaultFleetConfig())
	_ = fm.RegisterDevice(&DeviceInfo{ID: "d1"})

	err := fm.DeregisterDevice("d1")
	require.NoError(t, err)

	_, err = fm.GetDevice("d1")
	require.Error(t, err)
}

func TestFleetManager_DeregisterNotFound(t *testing.T) {
	fm := NewFleetManager(DefaultFleetConfig())
	err := fm.DeregisterDevice("missing")
	require.Error(t, err)
}

func TestFleetManager_UpdateStatus(t *testing.T) {
	fm := NewFleetManager(DefaultFleetConfig())
	_ = fm.RegisterDevice(&DeviceInfo{ID: "d1"})

	err := fm.UpdateStatus("d1", DeviceStatusSyncing)
	require.NoError(t, err)

	dev, _ := fm.GetDevice("d1")
	assert.Equal(t, DeviceStatusSyncing, dev.Status)
}

func TestFleetManager_UpdateStatusNotFound(t *testing.T) {
	fm := NewFleetManager(DefaultFleetConfig())
	err := fm.UpdateStatus("missing", DeviceStatusOnline)
	require.Error(t, err)
}

func TestFleetManager_ListDevices(t *testing.T) {
	fm := NewFleetManager(DefaultFleetConfig())
	_ = fm.RegisterDevice(&DeviceInfo{ID: "d1"})
	_ = fm.RegisterDevice(&DeviceInfo{ID: "d2"})

	devices := fm.ListDevices()
	assert.Len(t, devices, 2)
}

func TestFleetManager_ListByStatus(t *testing.T) {
	fm := NewFleetManager(DefaultFleetConfig())
	_ = fm.RegisterDevice(&DeviceInfo{ID: "d1", Status: DeviceStatusOnline})
	_ = fm.RegisterDevice(&DeviceInfo{ID: "d2", Status: DeviceStatusError})
	_ = fm.RegisterDevice(&DeviceInfo{ID: "d3", Status: DeviceStatusOnline})

	online := fm.ListByStatus(DeviceStatusOnline)
	assert.Len(t, online, 2)
}

func TestFleetManager_ListByRegion(t *testing.T) {
	fm := NewFleetManager(DefaultFleetConfig())
	_ = fm.RegisterDevice(&DeviceInfo{ID: "d1", Region: "us-east"})
	_ = fm.RegisterDevice(&DeviceInfo{ID: "d2", Region: "eu-west"})
	_ = fm.RegisterDevice(&DeviceInfo{ID: "d3", Region: "us-east"})

	usEast := fm.ListByRegion("us-east")
	assert.Len(t, usEast, 2)
}

func TestFleetManager_HealthCheck(t *testing.T) {
	fm := NewFleetManager(DefaultFleetConfig())
	_ = fm.RegisterDevice(&DeviceInfo{ID: "d1", Status: DeviceStatusOnline})
	_ = fm.RegisterDevice(&DeviceInfo{ID: "d2", Status: DeviceStatusOffline})
	_ = fm.RegisterDevice(&DeviceInfo{ID: "d3", Status: DeviceStatusSyncing})
	_ = fm.RegisterDevice(&DeviceInfo{ID: "d4", Status: DeviceStatusError})

	h := fm.HealthCheck()
	assert.Equal(t, 4, h.TotalDevices)
	assert.Equal(t, 1, h.OnlineDevices)
	assert.Equal(t, 1, h.OfflineDevices)
	assert.Equal(t, 1, h.SyncingDevices)
	assert.Equal(t, 1, h.ErrorDevices)
	assert.Equal(t, 50.0, h.HealthPct)
}

func TestFleetManager_HealthCheckEmpty(t *testing.T) {
	fm := NewFleetManager(DefaultFleetConfig())
	h := fm.HealthCheck()
	assert.Equal(t, 0, h.TotalDevices)
	assert.Equal(t, 0.0, h.HealthPct)
}

func TestFleetManager_Stats(t *testing.T) {
	fm := NewFleetManager(DefaultFleetConfig())
	_ = fm.RegisterDevice(&DeviceInfo{ID: "d1", Region: "us-east", Features: 10, PendingSync: 2})
	_ = fm.RegisterDevice(&DeviceInfo{ID: "d2", Region: "eu-west", Features: 5, PendingSync: 1})

	s := fm.Stats()
	assert.Equal(t, 2, s.TotalDevices)
	assert.Equal(t, 15, s.TotalFeatures)
	assert.Equal(t, 3, s.TotalPendingSync)
	assert.Equal(t, 1, s.DevicesByRegion["us-east"])
	assert.Equal(t, 1, s.DevicesByRegion["eu-west"])
}

func TestDefaultFleetConfig(t *testing.T) {
	cfg := DefaultFleetConfig()
	assert.Equal(t, 10000, cfg.MaxDevices)
	assert.Equal(t, 3, cfg.SyncRetryLimit)
}
