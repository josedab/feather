// Package app provides the application bootstrap and lifecycle management for Feather.
//
// It extracts server construction, signal handling, and graceful shutdown from
// cmd/feather/main.go into reusable components.
package app

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
	"strings"
	"sync"
)

// ServerManager tracks all running servers for graceful shutdown.
type ServerManager struct {
	mu      sync.Mutex
	servers map[string]Shutdownable
	logger  *slog.Logger
}

// Shutdownable represents a server that can be gracefully shut down.
type Shutdownable interface {
	Shutdown(ctx context.Context) error
}

// HTTPServerWrapper adapts *http.Server to the Shutdownable interface.
type HTTPServerWrapper struct {
	Server *http.Server
}

// Shutdown gracefully shuts down the HTTP server.
func (h *HTTPServerWrapper) Shutdown(ctx context.Context) error {
	return h.Server.Shutdown(ctx)
}

// NewServerManager creates a new server manager.
func NewServerManager(logger *slog.Logger) *ServerManager {
	return &ServerManager{
		servers: make(map[string]Shutdownable),
		logger:  logger,
	}
}

// Register adds a named server to the manager.
func (m *ServerManager) Register(name string, s Shutdownable) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.servers[name] = s
}

// ShutdownAll shuts down all registered servers concurrently.
func (m *ServerManager) ShutdownAll(ctx context.Context) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var wg sync.WaitGroup
	for name, s := range m.servers {
		wg.Add(1)
		go func(name string, s Shutdownable) {
			defer wg.Done()
			m.logger.Info("shutting down server", "name", name)
			if err := s.Shutdown(ctx); err != nil {
				m.logger.Error("shutdown error", "name", name, "error", err)
			}
		}(name, s)
	}
	wg.Wait()
}

// RecoverPanic recovers from panics in goroutines and logs them.
func RecoverPanic(logger *slog.Logger, component string) {
	if r := recover(); r != nil {
		stack := debug.Stack()
		logger.Error("panic recovered",
			"component", component,
			"panic", r,
			"stack", string(stack),
		)
	}
}

// LogListenError logs a server listen error with actionable hints.
func LogListenError(logger *slog.Logger, component string, port int, err error) {
	if strings.Contains(err.Error(), "address already in use") {
		logger.Error(fmt.Sprintf("%s error: port %d is already in use", component, port),
			"error", err,
			"hint", fmt.Sprintf("Run 'lsof -i :%d' to see what's using it, or set a different port", port),
		)
		return
	}
	logger.Error(fmt.Sprintf("%s error", component), "error", err)
}
