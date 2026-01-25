package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/feather-store/feather/internal/platform/cloudcontrol"
)

// CloudControlHandler handles managed cloud control plane API requests.
type CloudControlHandler struct {
	cp *cloudcontrol.ControlPlane
}

// NewCloudControlHandler creates a new cloud control plane handler.
func NewCloudControlHandler(cp *cloudcontrol.ControlPlane) *CloudControlHandler {
	return &CloudControlHandler{cp: cp}
}

// RegisterRoutes registers cloud control plane API routes.
func (h *CloudControlHandler) RegisterRoutes(mux *http.ServeMux) {
	// Tenants
	mux.HandleFunc("GET /v1/cloud/tenants", h.handleListTenants)
	mux.HandleFunc("POST /v1/cloud/tenants", h.handleCreateTenant)
	mux.HandleFunc("GET /v1/cloud/tenants/{id}", h.handleGetTenant)

	// Instances
	mux.HandleFunc("GET /v1/cloud/instances", h.handleListInstances)
	mux.HandleFunc("POST /v1/cloud/instances", h.handleProvisionInstance)
	mux.HandleFunc("GET /v1/cloud/instances/{id}", h.handleGetInstance)
	mux.HandleFunc("DELETE /v1/cloud/instances/{id}", h.handleTerminateInstance)
	mux.HandleFunc("POST /v1/cloud/instances/{id}/scale", h.handleScaleInstance)
	mux.HandleFunc("POST /v1/cloud/instances/{id}/autoscale", h.handleSetAutoscale)

	// Stats
	mux.HandleFunc("GET /v1/cloud/control/stats", h.handleStats)
}

func (h *CloudControlHandler) handleListTenants(w http.ResponseWriter, r *http.Request) {
	tenants := h.cp.ListTenants()
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"tenants": tenants,
		"count":   len(tenants),
	})
}

func (h *CloudControlHandler) handleCreateTenant(w http.ResponseWriter, r *http.Request) {
	var t cloudcontrol.Tenant
	if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	created, err := h.cp.CreateTenant(t)
	if err != nil {
		if errors.Is(err, cloudcontrol.ErrTenantExists) {
			h.writeError(r.Context(), w, http.StatusConflict, "tenant already exists")
			return
		}
		h.writeError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusCreated, created)
}

func (h *CloudControlHandler) handleGetTenant(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	t, err := h.cp.GetTenant(id)
	if err != nil {
		if errors.Is(err, cloudcontrol.ErrTenantNotFound) {
			h.writeError(r.Context(), w, http.StatusNotFound, "tenant not found")
			return
		}
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusOK, t)
}

func (h *CloudControlHandler) handleListInstances(w http.ResponseWriter, r *http.Request) {
	tenantID := r.URL.Query().Get("tenant_id")
	instances := h.cp.ListInstances(tenantID)
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"instances": instances,
		"count":     len(instances),
	})
}

func (h *CloudControlHandler) handleProvisionInstance(w http.ResponseWriter, r *http.Request) {
	var inst cloudcontrol.Instance
	if err := json.NewDecoder(r.Body).Decode(&inst); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	created, err := h.cp.ProvisionInstance(inst)
	if err != nil {
		if errors.Is(err, cloudcontrol.ErrInstanceExists) {
			h.writeError(r.Context(), w, http.StatusConflict, "instance already exists")
			return
		}
		if errors.Is(err, cloudcontrol.ErrTenantNotFound) {
			h.writeError(r.Context(), w, http.StatusBadRequest, "tenant not found")
			return
		}
		if errors.Is(err, cloudcontrol.ErrQuotaExceeded) {
			h.writeError(r.Context(), w, http.StatusForbidden, err.Error())
			return
		}
		h.writeError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusCreated, created)
}

func (h *CloudControlHandler) handleGetInstance(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	inst, err := h.cp.GetInstance(id)
	if err != nil {
		if errors.Is(err, cloudcontrol.ErrInstanceNotFound) {
			h.writeError(r.Context(), w, http.StatusNotFound, "instance not found")
			return
		}
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusOK, inst)
}

func (h *CloudControlHandler) handleTerminateInstance(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.cp.TerminateInstance(id); err != nil {
		if errors.Is(err, cloudcontrol.ErrInstanceNotFound) {
			h.writeError(r.Context(), w, http.StatusNotFound, "instance not found")
			return
		}
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusOK, SuccessResponse{Success: true, Message: "instance terminated"})
}

func (h *CloudControlHandler) handleScaleInstance(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req cloudcontrol.ScaleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	inst, err := h.cp.ScaleInstance(id, req)
	if err != nil {
		if errors.Is(err, cloudcontrol.ErrInstanceNotFound) {
			h.writeError(r.Context(), w, http.StatusNotFound, "instance not found")
			return
		}
		if errors.Is(err, cloudcontrol.ErrQuotaExceeded) {
			h.writeError(r.Context(), w, http.StatusForbidden, err.Error())
			return
		}
		h.writeError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusOK, inst)
}

func (h *CloudControlHandler) handleSetAutoscale(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var policy cloudcontrol.AutoscalePolicy
	if err := json.NewDecoder(r.Body).Decode(&policy); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	inst, err := h.cp.SetAutoscalePolicy(id, policy)
	if err != nil {
		if errors.Is(err, cloudcontrol.ErrInstanceNotFound) {
			h.writeError(r.Context(), w, http.StatusNotFound, "instance not found")
			return
		}
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusOK, inst)
}

func (h *CloudControlHandler) handleStats(w http.ResponseWriter, r *http.Request) {
	stats := h.cp.Stats()
	h.writeJSON(r.Context(), w, http.StatusOK, stats)
}

func (h *CloudControlHandler) writeJSON(ctx context.Context, w http.ResponseWriter, status int, data interface{}) {
	writeJSONResponse(ctx, w, status, data)
}

func (h *CloudControlHandler) writeError(ctx context.Context, w http.ResponseWriter, status int, message string) {
	writeJSONError(ctx, w, status, message)
}
