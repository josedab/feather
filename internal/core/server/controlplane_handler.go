package server

import (
	"context"
	"net/http"

	"github.com/feather-store/feather/internal/platform/controlplane"
)

// ControlPlaneHandler handles multi-cloud control plane API requests.
type ControlPlaneHandler struct {
	manager *controlplane.Manager
}

// NewControlPlaneHandler creates a new control plane handler.
func NewControlPlaneHandler(manager *controlplane.Manager) *ControlPlaneHandler {
	return &ControlPlaneHandler{manager: manager}
}

// RegisterRoutes registers control plane API routes.
func (h *ControlPlaneHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/controlplane/instances", h.handleListInstances)
	mux.HandleFunc("POST /v1/controlplane/instances", h.handleRegisterInstance)
	mux.HandleFunc("GET /v1/controlplane/instances/{id}", h.handleGetInstance)
	mux.HandleFunc("DELETE /v1/controlplane/instances/{id}", h.handleDeregisterInstance)
	mux.HandleFunc("GET /v1/controlplane/regions", h.handleListRegions)
	mux.HandleFunc("POST /v1/controlplane/regions", h.handleAddRegion)
	mux.HandleFunc("GET /v1/controlplane/fleet/status", h.handleFleetStatus)
	mux.HandleFunc("GET /v1/controlplane/policies", h.handleListPolicies)
	mux.HandleFunc("POST /v1/controlplane/policies", h.handleAddPolicy)
}

func (h *ControlPlaneHandler) handleListInstances(w http.ResponseWriter, r *http.Request) {
	instances := h.manager.ListInstances(r.Context())
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{"instances": instances})
}

func (h *ControlPlaneHandler) handleRegisterInstance(w http.ResponseWriter, r *http.Request) {
	var inst controlplane.Instance
	if err := strictDecode(r.Body, &inst); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.manager.RegisterInstance(r.Context(), &inst); err != nil {
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusCreated, map[string]interface{}{"success": true, "id": inst.ID})
}

func (h *ControlPlaneHandler) handleGetInstance(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "instance id required")
		return
	}
	inst, err := h.manager.GetInstance(r.Context(), id)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusOK, inst)
}

func (h *ControlPlaneHandler) handleDeregisterInstance(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "instance id required")
		return
	}
	if err := h.manager.DeregisterInstance(r.Context(), id); err != nil {
		h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusOK, SuccessResponse{Success: true})
}

func (h *ControlPlaneHandler) handleListRegions(w http.ResponseWriter, r *http.Request) {
	regions := h.manager.ListRegions(r.Context())
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{"regions": regions})
}

func (h *ControlPlaneHandler) handleAddRegion(w http.ResponseWriter, r *http.Request) {
	var region controlplane.Region
	if err := strictDecode(r.Body, &region); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.manager.AddRegion(r.Context(), &region); err != nil {
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusCreated, map[string]interface{}{"success": true, "name": region.Name})
}

func (h *ControlPlaneHandler) handleFleetStatus(w http.ResponseWriter, r *http.Request) {
	status := h.manager.GetFleetStatus(r.Context())
	h.writeJSON(r.Context(), w, http.StatusOK, status)
}

func (h *ControlPlaneHandler) handleListPolicies(w http.ResponseWriter, r *http.Request) {
	policies := h.manager.ListPolicies(r.Context())
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{"policies": policies})
}

func (h *ControlPlaneHandler) handleAddPolicy(w http.ResponseWriter, r *http.Request) {
	var policy controlplane.Policy
	if err := strictDecode(r.Body, &policy); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.manager.AddPolicy(r.Context(), &policy); err != nil {
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusCreated, map[string]interface{}{"success": true, "name": policy.Name})
}

func (h *ControlPlaneHandler) writeJSON(ctx context.Context, w http.ResponseWriter, status int, data interface{}) {
	writeJSONResponse(ctx, w, status, data)
}

func (h *ControlPlaneHandler) writeError(ctx context.Context, w http.ResponseWriter, status int, message string) {
	writeJSONError(ctx, w, status, message)
}
