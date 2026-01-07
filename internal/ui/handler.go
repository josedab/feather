package ui

import (
	"net/http"
	"strings"
)

// Handler serves the embedded UI static files.
type Handler struct {
	fileServer http.Handler
}

// NewHandler creates a new UI handler.
func NewHandler() (*Handler, error) {
	fsys, err := GetFileSystem()
	if err != nil {
		return nil, err
	}

	return &Handler{
		fileServer: http.FileServer(fsys),
	}, nil
}

// RegisterRoutes registers the UI routes with the given mux.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	// Serve static files under /ui/
	mux.Handle("/ui/", http.StripPrefix("/ui/", h))
}

// ServeHTTP implements http.Handler.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	// For SPA routing, serve index.html for non-file paths
	if !strings.Contains(path, ".") || path == "/" || path == "" {
		r.URL.Path = "/index.html"
	}

	// Set appropriate content types
	switch {
	case strings.HasSuffix(r.URL.Path, ".js"):
		w.Header().Set("Content-Type", "application/javascript")
	case strings.HasSuffix(r.URL.Path, ".css"):
		w.Header().Set("Content-Type", "text/css")
	case strings.HasSuffix(r.URL.Path, ".html"):
		w.Header().Set("Content-Type", "text/html")
	case strings.HasSuffix(r.URL.Path, ".json"):
		w.Header().Set("Content-Type", "application/json")
	}

	// Set cache headers for static assets
	if strings.HasPrefix(path, "js/") || strings.HasPrefix(path, "css/") {
		w.Header().Set("Cache-Control", "public, max-age=31536000")
	}

	h.fileServer.ServeHTTP(w, r)
}
