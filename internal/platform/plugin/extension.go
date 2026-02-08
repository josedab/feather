package plugin

import (
	"context"
	"fmt"
	"sync"
)

// StorageExtension allows plugins to provide custom storage backends.
type StorageExtension interface {
	Get(ctx context.Context, key string) (interface{}, error)
	Put(ctx context.Context, key string, value interface{}) error
	Delete(ctx context.Context, key string) error
	List(ctx context.Context, prefix string) ([]string, error)
}

// TransformExtension allows plugins to provide custom transformations.
type TransformExtension interface {
	Transform(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error)
	Validate(input map[string]interface{}) error
}

// AuthExtension allows plugins to provide custom authentication.
type AuthExtension interface {
	Authenticate(ctx context.Context, token string) (*AuthResult, error)
	Authorize(ctx context.Context, subject, resource, action string) (bool, error)
}

// AuthResult holds the outcome of an authentication check.
type AuthResult struct {
	Authenticated bool              `json:"authenticated"`
	Subject       string            `json:"subject"`
	Roles         []string          `json:"roles"`
	Claims        map[string]string `json:"claims,omitempty"`
}

// IngestionExtension allows plugins to provide custom data sources.
type IngestionExtension interface {
	Start(ctx context.Context) error
	Stop() error
	Subscribe(handler func(data map[string]interface{})) error
}

// ExportExtension allows plugins to provide custom export formats.
type ExportExtension interface {
	Export(ctx context.Context, data []map[string]interface{}, format string) ([]byte, error)
	SupportedFormats() []string
}

// ExtensionManager manages plugin extensions grouped by type.
type ExtensionManager struct {
	storageExts   map[string]StorageExtension
	transformExts map[string]TransformExtension
	authExts      map[string]AuthExtension
	ingestionExts map[string]IngestionExtension
	exportExts    map[string]ExportExtension
	mu            sync.RWMutex
}

// NewExtensionManager creates a new ExtensionManager.
func NewExtensionManager() *ExtensionManager {
	return &ExtensionManager{
		storageExts:   make(map[string]StorageExtension),
		transformExts: make(map[string]TransformExtension),
		authExts:      make(map[string]AuthExtension),
		ingestionExts: make(map[string]IngestionExtension),
		exportExts:    make(map[string]ExportExtension),
	}
}

// RegisterStorage registers a storage extension under the given plugin ID.
func (em *ExtensionManager) RegisterStorage(pluginID string, ext StorageExtension) error {
	if ext == nil {
		return fmt.Errorf("registering storage extension: extension must not be nil")
	}
	em.mu.Lock()
	defer em.mu.Unlock()

	if _, exists := em.storageExts[pluginID]; exists {
		return fmt.Errorf("registering storage extension: plugin %q already registered", pluginID)
	}
	em.storageExts[pluginID] = ext
	return nil
}

// GetStorage returns the storage extension for the given plugin ID.
func (em *ExtensionManager) GetStorage(pluginID string) (StorageExtension, error) {
	em.mu.RLock()
	defer em.mu.RUnlock()

	ext, exists := em.storageExts[pluginID]
	if !exists {
		return nil, fmt.Errorf("getting storage extension: plugin %q not found", pluginID)
	}
	return ext, nil
}

// RegisterTransform registers a transform extension under the given plugin ID.
func (em *ExtensionManager) RegisterTransform(pluginID string, ext TransformExtension) error {
	if ext == nil {
		return fmt.Errorf("registering transform extension: extension must not be nil")
	}
	em.mu.Lock()
	defer em.mu.Unlock()

	if _, exists := em.transformExts[pluginID]; exists {
		return fmt.Errorf("registering transform extension: plugin %q already registered", pluginID)
	}
	em.transformExts[pluginID] = ext
	return nil
}

// GetTransform returns the transform extension for the given plugin ID.
func (em *ExtensionManager) GetTransform(pluginID string) (TransformExtension, error) {
	em.mu.RLock()
	defer em.mu.RUnlock()

	ext, exists := em.transformExts[pluginID]
	if !exists {
		return nil, fmt.Errorf("getting transform extension: plugin %q not found", pluginID)
	}
	return ext, nil
}

// RegisterAuth registers an auth extension under the given plugin ID.
func (em *ExtensionManager) RegisterAuth(pluginID string, ext AuthExtension) error {
	if ext == nil {
		return fmt.Errorf("registering auth extension: extension must not be nil")
	}
	em.mu.Lock()
	defer em.mu.Unlock()

	if _, exists := em.authExts[pluginID]; exists {
		return fmt.Errorf("registering auth extension: plugin %q already registered", pluginID)
	}
	em.authExts[pluginID] = ext
	return nil
}

// GetAuth returns the auth extension for the given plugin ID.
func (em *ExtensionManager) GetAuth(pluginID string) (AuthExtension, error) {
	em.mu.RLock()
	defer em.mu.RUnlock()

	ext, exists := em.authExts[pluginID]
	if !exists {
		return nil, fmt.Errorf("getting auth extension: plugin %q not found", pluginID)
	}
	return ext, nil
}

// RegisterIngestion registers an ingestion extension under the given plugin ID.
func (em *ExtensionManager) RegisterIngestion(pluginID string, ext IngestionExtension) error {
	if ext == nil {
		return fmt.Errorf("registering ingestion extension: extension must not be nil")
	}
	em.mu.Lock()
	defer em.mu.Unlock()

	if _, exists := em.ingestionExts[pluginID]; exists {
		return fmt.Errorf("registering ingestion extension: plugin %q already registered", pluginID)
	}
	em.ingestionExts[pluginID] = ext
	return nil
}

// GetIngestion returns the ingestion extension for the given plugin ID.
func (em *ExtensionManager) GetIngestion(pluginID string) (IngestionExtension, error) {
	em.mu.RLock()
	defer em.mu.RUnlock()

	ext, exists := em.ingestionExts[pluginID]
	if !exists {
		return nil, fmt.Errorf("getting ingestion extension: plugin %q not found", pluginID)
	}
	return ext, nil
}

// RegisterExport registers an export extension under the given plugin ID.
func (em *ExtensionManager) RegisterExport(pluginID string, ext ExportExtension) error {
	if ext == nil {
		return fmt.Errorf("registering export extension: extension must not be nil")
	}
	em.mu.Lock()
	defer em.mu.Unlock()

	if _, exists := em.exportExts[pluginID]; exists {
		return fmt.Errorf("registering export extension: plugin %q already registered", pluginID)
	}
	em.exportExts[pluginID] = ext
	return nil
}

// GetExport returns the export extension for the given plugin ID.
func (em *ExtensionManager) GetExport(pluginID string) (ExportExtension, error) {
	em.mu.RLock()
	defer em.mu.RUnlock()

	ext, exists := em.exportExts[pluginID]
	if !exists {
		return nil, fmt.Errorf("getting export extension: plugin %q not found", pluginID)
	}
	return ext, nil
}

// ListExtensions returns a map of plugin type to registered plugin IDs.
func (em *ExtensionManager) ListExtensions() map[PluginType][]string {
	em.mu.RLock()
	defer em.mu.RUnlock()

	result := make(map[PluginType][]string)

	if len(em.storageExts) > 0 {
		ids := make([]string, 0, len(em.storageExts))
		for id := range em.storageExts {
			ids = append(ids, id)
		}
		result[PluginTypeStorage] = ids
	}

	if len(em.transformExts) > 0 {
		ids := make([]string, 0, len(em.transformExts))
		for id := range em.transformExts {
			ids = append(ids, id)
		}
		result[PluginTypeTransform] = ids
	}

	if len(em.authExts) > 0 {
		ids := make([]string, 0, len(em.authExts))
		for id := range em.authExts {
			ids = append(ids, id)
		}
		result[PluginTypeAuth] = ids
	}

	if len(em.ingestionExts) > 0 {
		ids := make([]string, 0, len(em.ingestionExts))
		for id := range em.ingestionExts {
			ids = append(ids, id)
		}
		result[PluginTypeIngestion] = ids
	}

	if len(em.exportExts) > 0 {
		ids := make([]string, 0, len(em.exportExts))
		for id := range em.exportExts {
			ids = append(ids, id)
		}
		result[PluginTypeExport] = ids
	}

	return result
}
