package server

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"

	"github.com/feather-store/feather/internal/wasm"
)

// WASMHandler handles WASM plugin API requests.
type WASMHandler struct {
	runtime *wasm.Runtime
}

// NewWASMHandler creates a new WASM handler.
func NewWASMHandler(runtime *wasm.Runtime) *WASMHandler {
	return &WASMHandler{
		runtime: runtime,
	}
}

// RegisterRoutes registers WASM plugin API routes.
func (h *WASMHandler) RegisterRoutes(mux *http.ServeMux) {
	// Plugin management
	mux.HandleFunc("GET /v1/plugins", h.handleListPlugins)
	mux.HandleFunc("GET /v1/plugins/{id}", h.handleGetPlugin)
	mux.HandleFunc("POST /v1/plugins", h.handleLoadPlugin)
	mux.HandleFunc("DELETE /v1/plugins/{id}", h.handleUnloadPlugin)
	mux.HandleFunc("POST /v1/plugins/{id}/enable", h.handleEnablePlugin)
	mux.HandleFunc("POST /v1/plugins/{id}/disable", h.handleDisablePlugin)

	// Function execution
	mux.HandleFunc("POST /v1/plugins/{id}/call/{function}", h.handleCallFunction)

	// Metrics
	mux.HandleFunc("GET /v1/plugins/metrics", h.handleGetMetrics)
}

// PluginJSON represents a plugin in JSON format.
type PluginJSON struct {
	ID          string             `json:"id"`
	Name        string             `json:"name"`
	Description string             `json:"description,omitempty"`
	Version     string             `json:"version,omitempty"`
	Author      string             `json:"author,omitempty"`
	Type        string             `json:"type"`
	Functions   []FunctionSpecJSON `json:"functions"`
	Config      map[string]string  `json:"config,omitempty"`
	State       string             `json:"state"`
	LoadedAt    string             `json:"loaded_at"`
}

// FunctionSpecJSON represents a function spec in JSON.
type FunctionSpecJSON struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Inputs      []ParamSpecJSON `json:"inputs,omitempty"`
	Output      ParamSpecJSON   `json:"output,omitempty"`
}

// ParamSpecJSON represents a parameter spec in JSON.
type ParamSpecJSON struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Required bool   `json:"required,omitempty"`
}

// handleListPlugins handles GET /v1/plugins
func (h *WASMHandler) handleListPlugins(w http.ResponseWriter, r *http.Request) {
	if h.runtime == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "WASM runtime not configured")
		return
	}

	plugins := h.runtime.ListPlugins()
	response := make([]PluginJSON, len(plugins))

	for i, p := range plugins {
		response[i] = h.pluginToJSON(p)
	}

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"plugins": response,
		"count":   len(response),
	})
}

// handleGetPlugin handles GET /v1/plugins/{id}
func (h *WASMHandler) handleGetPlugin(w http.ResponseWriter, r *http.Request) {
	if h.runtime == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "WASM runtime not configured")
		return
	}

	pluginID := r.PathValue("id")
	if pluginID == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "plugin ID required")
		return
	}

	plugin, err := h.runtime.GetPlugin(pluginID)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, h.pluginToJSON(plugin))
}

// LoadPluginRequest represents a request to load a plugin.
type LoadPluginRequest struct {
	ID          string             `json:"id"`
	Name        string             `json:"name"`
	Description string             `json:"description,omitempty"`
	Version     string             `json:"version,omitempty"`
	Author      string             `json:"author,omitempty"`
	Type        string             `json:"type"`
	Functions   []FunctionSpecJSON `json:"functions"`
	Config      map[string]string  `json:"config,omitempty"`
	WASMBase64  string             `json:"wasm_base64"` // Base64 encoded WASM binary
}

// handleLoadPlugin handles POST /v1/plugins
func (h *WASMHandler) handleLoadPlugin(w http.ResponseWriter, r *http.Request) {
	if h.runtime == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "WASM runtime not configured")
		return
	}

	var req LoadPluginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.ID == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "id is required")
		return
	}

	if req.Name == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "name is required")
		return
	}

	// Decode WASM binary
	var wasmBytes []byte
	var err error

	if req.WASMBase64 != "" {
		wasmBytes, err = base64.StdEncoding.DecodeString(req.WASMBase64)
		if err != nil {
			h.writeError(r.Context(), w, http.StatusBadRequest, "invalid base64 WASM data")
			return
		}
	} else {
		// Create a minimal valid WASM binary for testing
		// This is the WASM magic number + version
		wasmBytes = []byte{0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00}
	}

	// Convert function specs
	functions := make([]wasm.FunctionSpec, len(req.Functions))
	for i, f := range req.Functions {
		inputs := make([]wasm.ParamSpec, len(f.Inputs))
		for j, in := range f.Inputs {
			inputs[j] = wasm.ParamSpec{
				Name:     in.Name,
				Type:     in.Type,
				Required: in.Required,
			}
		}

		functions[i] = wasm.FunctionSpec{
			Name:        f.Name,
			Description: f.Description,
			Inputs:      inputs,
			Output: wasm.ParamSpec{
				Name:     f.Output.Name,
				Type:     f.Output.Type,
				Required: f.Output.Required,
			},
		}
	}

	manifest := &wasm.PluginManifest{
		Name:        req.Name,
		Description: req.Description,
		Version:     req.Version,
		Author:      req.Author,
		Type:        req.Type,
		Functions:   functions,
		Config:      req.Config,
	}

	if err := h.runtime.LoadPlugin(req.ID, wasmBytes, manifest); err != nil {
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusCreated, map[string]interface{}{
		"success":   true,
		"plugin_id": req.ID,
	})
}

// handleUnloadPlugin handles DELETE /v1/plugins/{id}
func (h *WASMHandler) handleUnloadPlugin(w http.ResponseWriter, r *http.Request) {
	if h.runtime == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "WASM runtime not configured")
		return
	}

	pluginID := r.PathValue("id")
	if pluginID == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "plugin ID required")
		return
	}

	if err := h.runtime.UnloadPlugin(pluginID); err != nil {
		h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success": true,
	})
}

// handleEnablePlugin handles POST /v1/plugins/{id}/enable
func (h *WASMHandler) handleEnablePlugin(w http.ResponseWriter, r *http.Request) {
	if h.runtime == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "WASM runtime not configured")
		return
	}

	pluginID := r.PathValue("id")
	if pluginID == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "plugin ID required")
		return
	}

	if err := h.runtime.EnablePlugin(pluginID); err != nil {
		h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success": true,
		"state":   "active",
	})
}

// handleDisablePlugin handles POST /v1/plugins/{id}/disable
func (h *WASMHandler) handleDisablePlugin(w http.ResponseWriter, r *http.Request) {
	if h.runtime == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "WASM runtime not configured")
		return
	}

	pluginID := r.PathValue("id")
	if pluginID == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "plugin ID required")
		return
	}

	if err := h.runtime.DisablePlugin(pluginID); err != nil {
		h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success": true,
		"state":   "disabled",
	})
}

// CallFunctionRequest represents a request to call a function.
type CallFunctionRequest struct {
	Args map[string]interface{} `json:"args"`
}

// handleCallFunction handles POST /v1/plugins/{id}/call/{function}
func (h *WASMHandler) handleCallFunction(w http.ResponseWriter, r *http.Request) {
	if h.runtime == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "WASM runtime not configured")
		return
	}

	pluginID := r.PathValue("id")
	functionName := r.PathValue("function")

	if pluginID == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "plugin ID required")
		return
	}
	if functionName == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "function name required")
		return
	}

	var req CallFunctionRequest
	body, err := io.ReadAll(r.Body)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "failed to read request body")
		return
	}

	if len(body) > 0 {
		if unmarshalErr := json.Unmarshal(body, &req); unmarshalErr != nil {
			h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
			return
		}
	}

	if req.Args == nil {
		req.Args = make(map[string]interface{})
	}

	result, err := h.runtime.Call(r.Context(), pluginID, functionName, req.Args)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success": true,
		"result":  result,
	})
}

// handleGetMetrics handles GET /v1/plugins/metrics
func (h *WASMHandler) handleGetMetrics(w http.ResponseWriter, r *http.Request) {
	if h.runtime == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "WASM runtime not configured")
		return
	}

	metrics := h.runtime.GetMetrics()

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"total_calls":      metrics.TotalCalls,
		"total_errors":     metrics.TotalErrors,
		"plugins_loaded":   metrics.PluginsLoaded,
		"avg_exec_time_ms": metrics.AvgExecTimeMs,
	})
}

func (h *WASMHandler) pluginToJSON(p *wasm.Plugin) PluginJSON {
	functions := make([]FunctionSpecJSON, len(p.Functions))
	for i, f := range p.Functions {
		inputs := make([]ParamSpecJSON, len(f.Inputs))
		for j, in := range f.Inputs {
			inputs[j] = ParamSpecJSON{
				Name:     in.Name,
				Type:     in.Type,
				Required: in.Required,
			}
		}

		functions[i] = FunctionSpecJSON{
			Name:        f.Name,
			Description: f.Description,
			Inputs:      inputs,
			Output: ParamSpecJSON{
				Name:     f.Output.Name,
				Type:     f.Output.Type,
				Required: f.Output.Required,
			},
		}
	}

	return PluginJSON{
		ID:          p.ID,
		Name:        p.Name,
		Description: p.Description,
		Version:     p.Version,
		Author:      p.Author,
		Type:        string(p.Type),
		Functions:   functions,
		Config:      p.Config,
		State:       string(p.State),
		LoadedAt:    p.LoadedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}

func (h *WASMHandler) writeJSON(ctx context.Context, w http.ResponseWriter, status int, data interface{}) {
	writeJSONResponse(ctx, w, status, data)
}

func (h *WASMHandler) writeError(ctx context.Context, w http.ResponseWriter, status int, message string) {
	writeJSONError(ctx, w, status, message)
}
