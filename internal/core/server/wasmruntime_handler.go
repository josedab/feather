package server

import (
	"context"
	"errors"
	"net/http"

	"github.com/feather-store/feather/internal/extensions/wasmruntime"
)

// WasmRuntimeHandler handles serverless edge runtime API requests.
type WasmRuntimeHandler struct {
	manager *wasmruntime.EdgeManager
}

// NewWasmRuntimeHandler creates a new WASM edge runtime handler.
func NewWasmRuntimeHandler(manager *wasmruntime.EdgeManager) *WasmRuntimeHandler {
	return &WasmRuntimeHandler{manager: manager}
}

// RegisterRoutes registers edge runtime API routes.
func (h *WasmRuntimeHandler) RegisterRoutes(mux *http.ServeMux) {
	// Modules
	mux.HandleFunc("GET /v1/edge/modules", h.handleListModules)
	mux.HandleFunc("POST /v1/edge/modules", h.handleRegisterModule)
	mux.HandleFunc("GET /v1/edge/modules/{id}", h.handleGetModule)
	mux.HandleFunc("DELETE /v1/edge/modules/{id}", h.handleDeleteModule)

	// Devices
	mux.HandleFunc("GET /v1/edge/devices", h.handleListDevices)
	mux.HandleFunc("POST /v1/edge/devices", h.handleRegisterDevice)
	mux.HandleFunc("GET /v1/edge/devices/{id}", h.handleGetDevice)
	mux.HandleFunc("POST /v1/edge/devices/{id}/deploy/{moduleId}", h.handleDeployModule)
	mux.HandleFunc("POST /v1/edge/devices/{id}/sync", h.handleSyncDevice)
	mux.HandleFunc("POST /v1/edge/devices/{id}/heartbeat", h.handleHeartbeat)

	// Stats
	mux.HandleFunc("GET /v1/edge/stats", h.handleStats)
}

func (h *WasmRuntimeHandler) handleListModules(w http.ResponseWriter, r *http.Request) {
	modules := h.manager.ListModules()
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"modules": modules,
		"count":   len(modules),
	})
}

func (h *WasmRuntimeHandler) handleRegisterModule(w http.ResponseWriter, r *http.Request) {
	var mod wasmruntime.Module
	if err := strictDecode(r.Body, &mod); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	created, err := h.manager.RegisterModule(mod)
	if err != nil {
		if errors.Is(err, wasmruntime.ErrModuleExists) {
			h.writeError(r.Context(), w, http.StatusConflict, "module already exists")
			return
		}
		h.writeError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusCreated, created)
}

func (h *WasmRuntimeHandler) handleGetModule(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	mod, err := h.manager.GetModule(id)
	if err != nil {
		if errors.Is(err, wasmruntime.ErrModuleNotFound) {
			h.writeError(r.Context(), w, http.StatusNotFound, "module not found")
			return
		}
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusOK, mod)
}

func (h *WasmRuntimeHandler) handleDeleteModule(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.manager.DeleteModule(id); err != nil {
		if errors.Is(err, wasmruntime.ErrModuleNotFound) {
			h.writeError(r.Context(), w, http.StatusNotFound, "module not found")
			return
		}
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusOK, SuccessResponse{Success: true, Message: "module deleted"})
}

func (h *WasmRuntimeHandler) handleListDevices(w http.ResponseWriter, r *http.Request) {
	devices := h.manager.ListDevices()
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"devices": devices,
		"count":   len(devices),
	})
}

func (h *WasmRuntimeHandler) handleRegisterDevice(w http.ResponseWriter, r *http.Request) {
	var dev wasmruntime.Device
	if err := strictDecode(r.Body, &dev); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	created, err := h.manager.RegisterDevice(dev)
	if err != nil {
		if errors.Is(err, wasmruntime.ErrDeviceExists) {
			h.writeError(r.Context(), w, http.StatusConflict, "device already exists")
			return
		}
		h.writeError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusCreated, created)
}

func (h *WasmRuntimeHandler) handleGetDevice(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	dev, err := h.manager.GetDevice(id)
	if err != nil {
		if errors.Is(err, wasmruntime.ErrDeviceNotFound) {
			h.writeError(r.Context(), w, http.StatusNotFound, "device not found")
			return
		}
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusOK, dev)
}

func (h *WasmRuntimeHandler) handleDeployModule(w http.ResponseWriter, r *http.Request) {
	deviceID := r.PathValue("id")
	moduleID := r.PathValue("moduleId")

	if err := h.manager.DeployModule(deviceID, moduleID); err != nil {
		if errors.Is(err, wasmruntime.ErrDeviceNotFound) {
			h.writeError(r.Context(), w, http.StatusNotFound, "device not found")
			return
		}
		if errors.Is(err, wasmruntime.ErrModuleNotFound) {
			h.writeError(r.Context(), w, http.StatusNotFound, "module not found")
			return
		}
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusOK, SuccessResponse{Success: true, Message: "module deployed"})
}

func (h *WasmRuntimeHandler) handleSyncDevice(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	result, err := h.manager.SyncDevice(id)
	if err != nil {
		if errors.Is(err, wasmruntime.ErrDeviceNotFound) {
			h.writeError(r.Context(), w, http.StatusNotFound, "device not found")
			return
		}
		if errors.Is(err, wasmruntime.ErrDeviceOffline) {
			h.writeError(r.Context(), w, http.StatusServiceUnavailable, "device is offline")
			return
		}
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusOK, result)
}

func (h *WasmRuntimeHandler) handleHeartbeat(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.manager.Heartbeat(id); err != nil {
		if errors.Is(err, wasmruntime.ErrDeviceNotFound) {
			h.writeError(r.Context(), w, http.StatusNotFound, "device not found")
			return
		}
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusOK, SuccessResponse{Success: true})
}

func (h *WasmRuntimeHandler) handleStats(w http.ResponseWriter, r *http.Request) {
	stats := h.manager.Stats()
	h.writeJSON(r.Context(), w, http.StatusOK, stats)
}

func (h *WasmRuntimeHandler) writeJSON(ctx context.Context, w http.ResponseWriter, status int, data interface{}) {
	writeJSONResponse(ctx, w, status, data)
}

func (h *WasmRuntimeHandler) writeError(ctx context.Context, w http.ResponseWriter, status int, message string) {
	writeJSONError(ctx, w, status, message)
}
