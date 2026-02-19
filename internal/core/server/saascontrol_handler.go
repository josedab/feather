package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/feather-store/feather/internal/extensions/saascontrol"
)

// SaaSControlHandler handles multi-tenant SaaS control plane API requests.
type SaaSControlHandler struct {
	cp *saascontrol.ControlPlane
}

// NewSaaSControlHandler creates a new SaaS control handler.
func NewSaaSControlHandler(cp *saascontrol.ControlPlane) *SaaSControlHandler {
	return &SaaSControlHandler{cp: cp}
}

// RegisterRoutes registers SaaS control plane API routes.
func (h *SaaSControlHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/saas/plans", h.handleListPlans)
	mux.HandleFunc("GET /v1/saas/tenants", h.handleListTenants)
	mux.HandleFunc("POST /v1/saas/tenants", h.handleCreateTenant)
	mux.HandleFunc("GET /v1/saas/tenants/{id}", h.handleGetTenant)
	mux.HandleFunc("POST /v1/saas/tenants/{id}/suspend", h.handleSuspendTenant)
	mux.HandleFunc("GET /v1/saas/tenants/{id}/usage", h.handleGetUsage)
	mux.HandleFunc("GET /v1/saas/tenants/{id}/instances", h.handleListTenantInstances)
	mux.HandleFunc("POST /v1/saas/instances", h.handleProvisionInstance)
	mux.HandleFunc("GET /v1/saas/instances/{id}", h.handleGetInstance)
	mux.HandleFunc("POST /v1/saas/instances/{id}/scale", h.handleScaleInstance)
	mux.HandleFunc("DELETE /v1/saas/instances/{id}", h.handleTerminateInstance)
	mux.HandleFunc("GET /v1/saas/stats", h.handleStats)
}

func (h *SaaSControlHandler) handleListPlans(w http.ResponseWriter, r *http.Request) {
	plans := h.cp.ListPlans()
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{"plans": plans})
}

func (h *SaaSControlHandler) handleListTenants(w http.ResponseWriter, r *http.Request) {
	tenants := h.cp.ListTenants()
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{"tenants": tenants, "count": len(tenants)})
}

func (h *SaaSControlHandler) handleCreateTenant(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID     string `json:"id"`
		Name   string `json:"name"`
		Email  string `json:"email"`
		PlanID string `json:"plan_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.ID == "" || req.Name == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "id and name are required")
		return
	}

	tenant, err := h.cp.CreateTenant(req.ID, req.Name, req.Email, req.PlanID)
	if err != nil {
		if errors.Is(err, saascontrol.ErrTenantExists) {
			h.writeError(r.Context(), w, http.StatusConflict, "tenant already exists")
			return
		}
		if errors.Is(err, saascontrol.ErrInvalidPlan) {
			h.writeError(r.Context(), w, http.StatusBadRequest, "invalid plan")
			return
		}
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusCreated, tenant)
}

func (h *SaaSControlHandler) handleGetTenant(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	tenant, err := h.cp.GetTenant(id)
	if err != nil {
		if errors.Is(err, saascontrol.ErrTenantNotFound) {
			h.writeError(r.Context(), w, http.StatusNotFound, "tenant not found")
			return
		}
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusOK, tenant)
}

func (h *SaaSControlHandler) handleSuspendTenant(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.cp.SuspendTenant(id); err != nil {
		if errors.Is(err, saascontrol.ErrTenantNotFound) {
			h.writeError(r.Context(), w, http.StatusNotFound, "tenant not found")
			return
		}
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusOK, SuccessResponse{Success: true, Message: "tenant suspended"})
}

func (h *SaaSControlHandler) handleGetUsage(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	usage, err := h.cp.GetUsage(id)
	if err != nil {
		if errors.Is(err, saascontrol.ErrTenantNotFound) {
			h.writeError(r.Context(), w, http.StatusNotFound, "tenant not found")
			return
		}
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusOK, usage)
}

func (h *SaaSControlHandler) handleListTenantInstances(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	instances := h.cp.ListInstances(id)
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{"instances": instances, "count": len(instances)})
}

func (h *SaaSControlHandler) handleProvisionInstance(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TenantID string `json:"tenant_id"`
		Region   string `json:"region,omitempty"`
		Replicas int    `json:"replicas,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	inst, err := h.cp.ProvisionInstance(req.TenantID, req.Region, req.Replicas)
	if err != nil {
		if errors.Is(err, saascontrol.ErrTenantNotFound) {
			h.writeError(r.Context(), w, http.StatusNotFound, "tenant not found")
			return
		}
		if errors.Is(err, saascontrol.ErrQuotaExceeded) {
			h.writeError(r.Context(), w, http.StatusForbidden, err.Error())
			return
		}
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusCreated, inst)
}

func (h *SaaSControlHandler) handleGetInstance(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	inst, err := h.cp.GetInstance(id)
	if err != nil {
		if errors.Is(err, saascontrol.ErrInstanceNotFound) {
			h.writeError(r.Context(), w, http.StatusNotFound, "instance not found")
			return
		}
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusOK, inst)
}

func (h *SaaSControlHandler) handleScaleInstance(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req struct {
		Replicas int `json:"replicas"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// Try query param
		if rStr := r.URL.Query().Get("replicas"); rStr != "" {
			v, err := strconv.Atoi(rStr)
			if err != nil {
				h.writeError(r.Context(), w, http.StatusBadRequest, "invalid replicas parameter")
				return
			}
			req.Replicas = v
		}
	}

	if err := h.cp.ScaleInstance(id, req.Replicas); err != nil {
		if errors.Is(err, saascontrol.ErrInstanceNotFound) {
			h.writeError(r.Context(), w, http.StatusNotFound, "instance not found")
			return
		}
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{"success": true, "replicas": req.Replicas})
}

func (h *SaaSControlHandler) handleTerminateInstance(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.cp.TerminateInstance(id); err != nil {
		if errors.Is(err, saascontrol.ErrInstanceNotFound) {
			h.writeError(r.Context(), w, http.StatusNotFound, "instance not found")
			return
		}
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusOK, SuccessResponse{Success: true, Message: "instance terminated"})
}

func (h *SaaSControlHandler) handleStats(w http.ResponseWriter, r *http.Request) {
	stats := h.cp.Stats()
	h.writeJSON(r.Context(), w, http.StatusOK, stats)
}

func (h *SaaSControlHandler) writeJSON(ctx context.Context, w http.ResponseWriter, status int, data interface{}) {
	writeJSONResponse(ctx, w, status, data)
}

func (h *SaaSControlHandler) writeError(ctx context.Context, w http.ResponseWriter, status int, message string) {
	writeJSONError(ctx, w, status, message)
}
