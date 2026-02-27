package server

import (
	"context"
	"net/http"

	"github.com/feather-store/feather/internal/platform/plugin"
)

// PluginHandler handles plugin framework API requests.
type PluginHandler struct {
	registry *plugin.Registry
}

// NewPluginHandler creates a new plugin handler.
func NewPluginHandler(registry *plugin.Registry) *PluginHandler {
	return &PluginHandler{registry: registry}
}

// RegisterRoutes registers plugin API routes.
func (h *PluginHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/plugins", h.handleListPlugins)
	mux.HandleFunc("POST /v1/plugins", h.handleRegisterPlugin)
	mux.HandleFunc("GET /v1/plugins/stats", h.handleStats)
	mux.HandleFunc("GET /v1/plugins/{id}", h.handleGetPlugin)
	mux.HandleFunc("DELETE /v1/plugins/{id}", h.handleUnregisterPlugin)
	mux.HandleFunc("POST /v1/plugins/{id}/enable", h.handleEnable)
	mux.HandleFunc("POST /v1/plugins/{id}/disable", h.handleDisable)
}

func (h *PluginHandler) handleListPlugins(w http.ResponseWriter, r *http.Request) {
	plugins := h.registry.List()
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{"plugins": plugins})
}

func (h *PluginHandler) handleRegisterPlugin(w http.ResponseWriter, r *http.Request) {
	var p plugin.Plugin
	if err := strictDecode(r.Body, &p); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.registry.Register(r.Context(), &p); err != nil {
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusCreated, map[string]interface{}{"success": true, "id": p.ID})
}

func (h *PluginHandler) handleGetPlugin(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "plugin id required")
		return
	}
	p, err := h.registry.Get(id)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusOK, p)
}

func (h *PluginHandler) handleUnregisterPlugin(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "plugin id required")
		return
	}
	if err := h.registry.Unregister(r.Context(), id); err != nil {
		h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusOK, SuccessResponse{Success: true})
}

func (h *PluginHandler) handleEnable(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "plugin id required")
		return
	}
	if err := h.registry.Enable(r.Context(), id); err != nil {
		h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{"success": true, "id": id, "enabled": true})
}

func (h *PluginHandler) handleDisable(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "plugin id required")
		return
	}
	if err := h.registry.Disable(r.Context(), id); err != nil {
		h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{"success": true, "id": id, "enabled": false})
}

func (h *PluginHandler) handleStats(w http.ResponseWriter, r *http.Request) {
	stats := h.registry.Stats()
	h.writeJSON(r.Context(), w, http.StatusOK, stats)
}

func (h *PluginHandler) writeJSON(ctx context.Context, w http.ResponseWriter, status int, data interface{}) {
	writeJSONResponse(ctx, w, status, data)
}

func (h *PluginHandler) writeError(ctx context.Context, w http.ResponseWriter, status int, message string) {
	writeJSONError(ctx, w, status, message)
}
