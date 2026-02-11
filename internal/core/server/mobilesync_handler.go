package server

import (
	"encoding/json"
	"net/http"

	"github.com/feather-store/feather/internal/extensions/mobilesync"
)

// MobileSyncHandler handles HTTP requests for mobile device sync APIs.
type MobileSyncHandler struct {
	manager *mobilesync.SyncManager
}

// NewMobileSyncHandler creates a new mobile sync handler.
func NewMobileSyncHandler(manager *mobilesync.SyncManager) *MobileSyncHandler {
	return &MobileSyncHandler{manager: manager}
}

// RegisterRoutes registers mobile sync routes with the HTTP mux.
func (h *MobileSyncHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/mobile/devices", h.handleRegisterDevice)
	mux.HandleFunc("GET /v1/mobile/devices", h.handleListDevices)
	mux.HandleFunc("GET /v1/mobile/devices/{id}", h.handleGetDevice)
	mux.HandleFunc("DELETE /v1/mobile/devices/{id}", h.handleDeregisterDevice)
	mux.HandleFunc("POST /v1/mobile/sync", h.handleSync)
	mux.HandleFunc("POST /v1/mobile/conflicts/resolve", h.handleResolveConflict)
	mux.HandleFunc("GET /v1/mobile/conflicts", h.handleListConflicts)
	mux.HandleFunc("GET /v1/mobile/bandwidth/{deviceId}", h.handleEstimateBandwidth)
	mux.HandleFunc("GET /v1/mobile/stats", h.handleStats)
}

func (h *MobileSyncHandler) handleRegisterDevice(w http.ResponseWriter, r *http.Request) {
	var device mobilesync.Device
	if err := json.NewDecoder(r.Body).Decode(&device); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.manager.RegisterDevice(&device); err != nil {
		writeJSONError(r.Context(), w, http.StatusConflict, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusCreated, map[string]interface{}{
		"success": true,
		"device":  device,
	})
}

func (h *MobileSyncHandler) handleListDevices(w http.ResponseWriter, r *http.Request) {
	devices := h.manager.ListDevices()
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success": true,
		"devices": devices,
	})
}

func (h *MobileSyncHandler) handleGetDevice(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	device, err := h.manager.GetDevice(id)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success": true,
		"device":  device,
	})
}

func (h *MobileSyncHandler) handleDeregisterDevice(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.manager.DeregisterDevice(id); err != nil {
		writeJSONError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success": true,
	})
}

func (h *MobileSyncHandler) handleSync(w http.ResponseWriter, r *http.Request) {
	var req mobilesync.SyncRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}
	resp, err := h.manager.ProcessSync(&req)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success": true,
		"sync":    resp,
	})
}

func (h *MobileSyncHandler) handleResolveConflict(w http.ResponseWriter, r *http.Request) {
	var conflict mobilesync.SyncConflict
	if err := json.NewDecoder(r.Body).Decode(&conflict); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}
	resolved, err := h.manager.ResolveConflict(&conflict)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success":  true,
		"conflict": resolved,
	})
}

func (h *MobileSyncHandler) handleListConflicts(w http.ResponseWriter, r *http.Request) {
	conflicts := h.manager.ListConflicts()
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success":   true,
		"conflicts": conflicts,
	})
}

func (h *MobileSyncHandler) handleEstimateBandwidth(w http.ResponseWriter, r *http.Request) {
	deviceID := r.PathValue("deviceId")
	estimate, err := h.manager.EstimateBandwidth(deviceID)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success":   true,
		"bandwidth": estimate,
	})
}

func (h *MobileSyncHandler) handleStats(w http.ResponseWriter, r *http.Request) {
	stats := h.manager.Stats()
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success": true,
		"stats":   stats,
	})
}
