// Package plugin provides a plugin and extension framework for Feather.
//
// The plugin system supports registration, lifecycle management, and hook-based
// extension points that allow custom storage backends, transformations,
// authentication, ingestion sources, and export formats.
//
// Plugins register hooks at defined extension points (e.g., pre_read, post_write)
// and are executed in priority order during feature store operations.
package plugin
