package server

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/feather-store/feather/internal/extensions/wasmudf"
)

// WasmUDFHandler handles WebAssembly UDF API requests.
type WasmUDFHandler struct {
	runtime *wasmudf.Runtime
}

// NewWasmUDFHandler creates a new WebAssembly UDF handler.
func NewWasmUDFHandler(runtime *wasmudf.Runtime) *WasmUDFHandler {
	return &WasmUDFHandler{runtime: runtime}
}

// RegisterRoutes registers WebAssembly UDF API routes.
func (h *WasmUDFHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/wasm/modules", h.handleListModules)
	mux.HandleFunc("POST /v1/wasm/modules", h.handleRegisterModule)
	mux.HandleFunc("GET /v1/wasm/modules/{id}", h.handleGetModule)
	mux.HandleFunc("PUT /v1/wasm/modules/{id}", h.handleUpdateModule)
	mux.HandleFunc("DELETE /v1/wasm/modules/{id}", h.handleDeleteModule)
	mux.HandleFunc("POST /v1/wasm/modules/{id}/execute", h.handleExecuteModule)
	mux.HandleFunc("GET /v1/wasm/stats", h.handleGetStats)
}

// handleListModules handles GET /v1/wasm/modules
func (h *WasmUDFHandler) handleListModules(w http.ResponseWriter, r *http.Request) {
	if h.runtime == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "wasm runtime not configured")
		return
	}

	modules := h.runtime.ListModules()

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"modules": modules,
	})
}

// handleRegisterModule handles POST /v1/wasm/modules
func (h *WasmUDFHandler) handleRegisterModule(w http.ResponseWriter, r *http.Request) {
	if h.runtime == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "wasm runtime not configured")
		return
	}

	var mod wasmudf.Module
	if err := json.NewDecoder(r.Body).Decode(&mod); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.runtime.RegisterModule(mod); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusCreated, map[string]interface{}{
		"success": true,
		"id":      mod.ID,
	})
}

// handleGetModule handles GET /v1/wasm/modules/{id}
func (h *WasmUDFHandler) handleGetModule(w http.ResponseWriter, r *http.Request) {
	if h.runtime == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "wasm runtime not configured")
		return
	}

	id := r.PathValue("id")
	if id == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "module id is required")
		return
	}

	mod, err := h.runtime.GetModule(id)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, mod)
}

// handleUpdateModule handles PUT /v1/wasm/modules/{id}
func (h *WasmUDFHandler) handleUpdateModule(w http.ResponseWriter, r *http.Request) {
	if h.runtime == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "wasm runtime not configured")
		return
	}

	id := r.PathValue("id")
	if id == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "module id is required")
		return
	}

	var mod wasmudf.Module
	if err := json.NewDecoder(r.Body).Decode(&mod); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.runtime.UpdateModule(id, mod); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, SuccessResponse{Success: true, Message: "module updated"})
}

// handleDeleteModule handles DELETE /v1/wasm/modules/{id}
func (h *WasmUDFHandler) handleDeleteModule(w http.ResponseWriter, r *http.Request) {
	if h.runtime == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "wasm runtime not configured")
		return
	}

	id := r.PathValue("id")
	if id == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "module id is required")
		return
	}

	if err := h.runtime.DeleteModule(id); err != nil {
		h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, SuccessResponse{Success: true, Message: "module deleted"})
}

// handleExecuteModule handles POST /v1/wasm/modules/{id}/execute
func (h *WasmUDFHandler) handleExecuteModule(w http.ResponseWriter, r *http.Request) {
	if h.runtime == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "wasm runtime not configured")
		return
	}

	id := r.PathValue("id")
	if id == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "module id is required")
		return
	}

	var input map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	result, err := h.runtime.Execute(id, input)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, result)
}

// handleGetStats handles GET /v1/wasm/stats
func (h *WasmUDFHandler) handleGetStats(w http.ResponseWriter, r *http.Request) {
	if h.runtime == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "wasm runtime not configured")
		return
	}

	stats := h.runtime.Stats()

	h.writeJSON(r.Context(), w, http.StatusOK, stats)
}

func (h *WasmUDFHandler) writeJSON(ctx context.Context, w http.ResponseWriter, status int, data interface{}) {
	writeJSONResponse(ctx, w, status, data)
}

func (h *WasmUDFHandler) writeError(ctx context.Context, w http.ResponseWriter, status int, message string) {
	writeJSONError(ctx, w, status, message)
}
