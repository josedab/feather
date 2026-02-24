package server

import (
	"encoding/json"
	"net/http"

	"github.com/feather-store/feather/internal/extensions/pythonsdk"
)

// ---------------------------------------------------------------------------
// PythonSidecarHandler
// ---------------------------------------------------------------------------

// PythonSidecarHandler exposes Python transform sidecar management endpoints.
type PythonSidecarHandler struct {
	manager *pythonsdk.SidecarManager
}

// NewPythonSidecarHandler creates a new PythonSidecarHandler.
func NewPythonSidecarHandler(mgr *pythonsdk.SidecarManager) *PythonSidecarHandler {
	return &PythonSidecarHandler{manager: mgr}
}

// RegisterRoutes registers Python sidecar API routes.
func (h *PythonSidecarHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/python/sidecar/start", h.handleStart)
	mux.HandleFunc("POST /v1/python/sidecar/stop", h.handleStop)
	mux.HandleFunc("POST /v1/python/sidecar/deploy", h.handleDeploy)
	mux.HandleFunc("POST /v1/python/sidecar/execute/{transformID}", h.handleExecute)
	mux.HandleFunc("DELETE /v1/python/sidecar/undeploy/{transformID}", h.handleUndeploy)
	mux.HandleFunc("GET /v1/python/sidecar/workers", h.handleListWorkers)
	mux.HandleFunc("GET /v1/python/sidecar/deps/{transformID}", h.handleGetDeps)
	mux.HandleFunc("GET /v1/python/sidecar/stats", h.handleStats)
}

func (h *PythonSidecarHandler) handleStart(w http.ResponseWriter, r *http.Request) {
	if err := h.manager.Start(); err != nil {
		writeJSONError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]string{"status": "started"})
}

func (h *PythonSidecarHandler) handleStop(w http.ResponseWriter, r *http.Request) {
	h.manager.Stop()
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]string{"status": "stopped"})
}

func (h *PythonSidecarHandler) handleDeploy(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TransformID  string                `json:"transform_id"`
		Dependencies []pythonsdk.Dependency `json:"dependencies,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	worker, err := h.manager.DeployTransform(r.Context(), req.TransformID, req.Dependencies)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusCreated, worker)
}

func (h *PythonSidecarHandler) handleExecute(w http.ResponseWriter, r *http.Request) {
	transformID := r.PathValue("transformID")
	var req struct {
		Inputs map[string]interface{} `json:"inputs"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	result, err := h.manager.ExecuteTransform(r.Context(), transformID, req.Inputs)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{"result": result})
}

func (h *PythonSidecarHandler) handleUndeploy(w http.ResponseWriter, r *http.Request) {
	transformID := r.PathValue("transformID")
	if err := h.manager.UndeployTransform(transformID); err != nil {
		writeJSONError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]string{"undeployed": transformID})
}

func (h *PythonSidecarHandler) handleListWorkers(w http.ResponseWriter, r *http.Request) {
	workers := h.manager.ListWorkers()
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"workers": workers,
		"total":   len(workers),
	})
}

func (h *PythonSidecarHandler) handleGetDeps(w http.ResponseWriter, r *http.Request) {
	transformID := r.PathValue("transformID")
	deps := h.manager.GetDependencies(transformID)
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"transform_id": transformID,
		"dependencies": deps,
	})
}

func (h *PythonSidecarHandler) handleStats(w http.ResponseWriter, r *http.Request) {
	writeJSONResponse(r.Context(), w, http.StatusOK, h.manager.Stats())
}
