// Package mobilesync provides the sync protocol for mobile SDKs (iOS, Android).
//
// It supports offline-first architectures with delta sync, conflict resolution,
// and bandwidth-aware synchronization for feature store data. Supported platforms
// include iOS, Android, React Native, and Flutter.
//
// Sync modes:
//   - Full: complete snapshot sync
//   - Delta: incremental changes only
//   - Selective: sync specific feature subsets
//
// Usage:
//
//	syncer := mobilesync.NewSyncer(mobilesync.DefaultConfig())
//	syncer.RegisterDevice("device-001", mobilesync.PlatformIOS)
//	result, err := syncer.Sync(ctx, "device-001", mobilesync.SyncModeDelta)
package mobilesync
