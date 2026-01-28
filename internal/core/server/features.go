// Package server provides the HTTP and gRPC serving layer for Feather.
//
// # Handler Registration System
//
// The server uses a pluggable handler architecture (see ADR-0008) to support
// optional feature modules. Each feature area (drift, lineage, marketplace, etc.)
// implements the [FeatureHandler] interface and registers a factory function in
// [featureRegistry] during init().
//
// To enable a handler, add its name to the EnabledFeatures map in
// [HTTPServerFeatureConfig]. Only enabled handlers are instantiated and have
// their routes registered on the ServeMux.
//
// To add a new handler:
//  1. Create a struct implementing [FeatureHandler] (with a RegisterRoutes method).
//  2. Register a factory in the init() function of the appropriate features_*.go file.
//  3. Enable it via the EnabledFeatures map in cmd/feather/main.go.
//  4. Add it to docs/package-guide.md.
//
// Handler registrations are split across files by category:
//   - features_core.go: Stable, production-ready handlers
//   - features_platform.go: Beta platform and extension handlers
//   - features_experimental.go: Experimental handlers
//   - features_nextgen.go: Next-gen and advanced feature handlers
package server

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/feather-store/feather/internal/core/aggregation"
	"github.com/feather-store/feather/internal/core/metrics"
	"github.com/feather-store/feather/internal/core/storage"
)

// FeatureHandler registers routes on a ServeMux.
type FeatureHandler interface {
	RegisterRoutes(mux *http.ServeMux)
}

// Maturity indicates the stability level of a handler.
type Maturity string

const (
	// MaturityStable means production-ready, well-tested, semver-protected.
	MaturityStable Maturity = "stable"
	// MaturityBeta means functional and tested, API may change between minor releases.
	MaturityBeta Maturity = "beta"
	// MaturityExperimental means working implementation, may be incomplete or change significantly.
	MaturityExperimental Maturity = "experimental"
)

// handlerDeps provides dependencies to handler factories.
type handlerDeps struct {
	Ctx         context.Context
	Store       *storage.Store
	Aggregation *aggregation.Engine
	Schema      *storage.Registry
	Metrics     *metrics.Metrics
	Config      HTTPServerConfig
}

// handlerFactory creates a FeatureHandler from dependencies.
// Returns nil if the handler cannot be created.
type handlerFactory func(deps *handlerDeps) FeatureHandler

// HandlerSpec describes a registered handler and its maturity level.
type HandlerSpec struct {
	Name     string
	Maturity Maturity
	Factory  handlerFactory
}

// featureRegistry maps feature names to factory functions.
var featureRegistry = map[string]handlerFactory{}

// handlerSpecs stores maturity metadata for every registered handler.
// Query with RegisteredHandlerSpecs() to inspect maturity levels.
var handlerSpecs []HandlerSpec

// registerHandler registers a handler with its maturity level.
func registerHandler(name string, maturity Maturity, factory handlerFactory) {
	featureRegistry[name] = factory
	handlerSpecs = append(handlerSpecs, HandlerSpec{Name: name, Maturity: maturity, Factory: factory})
}

// RegisteredFeatures returns all available feature names.
func RegisteredFeatures() []string {
	names := make([]string, 0, len(featureRegistry))
	for name := range featureRegistry {
		names = append(names, name)
	}
	return names
}

// RegisteredHandlerSpecs returns handler specs grouped by maturity.
// Useful for CLI tools and diagnostics (e.g., make api-routes).
func RegisteredHandlerSpecs() []HandlerSpec {
	out := make([]HandlerSpec, len(handlerSpecs))
	copy(out, handlerSpecs)
	return out
}

// registerEnabledFeatures creates and registers all enabled feature handlers.
func registerEnabledFeatures(mux *http.ServeMux, enabled map[string]bool, deps *handlerDeps) {
	// Validate that all enabled feature names correspond to registered handlers
	for name := range enabled {
		if !enabled[name] {
			continue
		}
		if _, exists := featureRegistry[name]; !exists {
			slog.Warn("enabled feature has no registered handler, ignoring", "feature", name)
		}
	}

	for name, factory := range featureRegistry {
		if !enabled[name] {
			continue
		}
		handler := factory(deps)
		if handler == nil {
			slog.Warn("feature handler factory returned nil, skipping", "handler", name)
			continue
		}
		handler.RegisterRoutes(mux)
	}
}

// HandlerInventory describes a registered handler for API documentation.
type HandlerInventory struct {
	Name     string `json:"name"`
	Maturity string `json:"maturity"`
	Enabled  bool   `json:"enabled"`
}

// GetHandlerInventory returns all handlers with maturity and enabled status.
func GetHandlerInventory(enabled map[string]bool) []HandlerInventory {
	inv := make([]HandlerInventory, 0, len(handlerSpecs))
	for _, spec := range handlerSpecs {
		inv = append(inv, HandlerInventory{
			Name:     spec.Name,
			Maturity: string(spec.Maturity),
			Enabled:  enabled[spec.Name],
		})
	}
	return inv
}
