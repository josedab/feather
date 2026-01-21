package pluginsdk

import (
	"fmt"
	"sync"
)

// PluginInfo describes a plugin's metadata.
type PluginInfo struct {
	ID          string
	Name        string
	Version     string
	Author      string
	Description string
	Maturity    string
}

// PluginRequest represents an incoming request routed to a plugin.
type PluginRequest struct {
	Method  string
	Path    string
	Headers map[string]string
	Body    []byte
	Params  map[string]string
}

// PluginResponse represents the plugin's response.
type PluginResponse struct {
	StatusCode int
	Headers    map[string]string
	Body       []byte
}

// PluginHandler is the interface that external plugins must implement.
type PluginHandler interface {
	Info() PluginInfo
	Routes() []RouteSpec
	Handle(PluginRequest) PluginResponse
}

// RouteSpec describes a single route exposed by a plugin.
type RouteSpec struct {
	Method  string
	Path    string
	Summary string
}

// PluginRegistry manages registered plugins.
type PluginRegistry struct {
	mu      sync.RWMutex
	plugins map[string]PluginHandler
}

// NewPluginRegistry creates a new empty plugin registry.
func NewPluginRegistry() *PluginRegistry {
	return &PluginRegistry{
		plugins: make(map[string]PluginHandler),
	}
}

// Register adds a plugin handler to the registry.
func (r *PluginRegistry) Register(handler PluginHandler) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	info := handler.Info()
	if info.ID == "" {
		return fmt.Errorf("plugin ID must not be empty")
	}
	if _, exists := r.plugins[info.ID]; exists {
		return fmt.Errorf("plugin %q is already registered", info.ID)
	}
	r.plugins[info.ID] = handler
	return nil
}

// Unregister removes a plugin from the registry.
func (r *PluginRegistry) Unregister(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.plugins[id]; !exists {
		return fmt.Errorf("plugin %q not found", id)
	}
	delete(r.plugins, id)
	return nil
}

// Get returns the plugin handler for the given ID.
func (r *PluginRegistry) Get(id string) (PluginHandler, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	handler, exists := r.plugins[id]
	if !exists {
		return nil, fmt.Errorf("plugin %q not found", id)
	}
	return handler, nil
}

// List returns metadata for all registered plugins.
func (r *PluginRegistry) List() []PluginInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	infos := make([]PluginInfo, 0, len(r.plugins))
	for _, handler := range r.plugins {
		infos = append(infos, handler.Info())
	}
	return infos
}

// Route dispatches a request to the specified plugin.
func (r *PluginRegistry) Route(id string, req PluginRequest) (PluginResponse, error) {
	handler, err := r.Get(id)
	if err != nil {
		return PluginResponse{}, err
	}
	return handler.Handle(req), nil
}
