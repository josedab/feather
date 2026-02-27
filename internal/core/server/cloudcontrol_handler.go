package server

import (
	"context"
	"errors"
	"net/http"
	"time"

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

	// Billing
	mux.HandleFunc("GET /v1/cloud/billing/plans", h.handleListPlans)
	mux.HandleFunc("POST /v1/cloud/billing/usage", h.handleRecordUsage)
	mux.HandleFunc("GET /v1/cloud/billing/usage/{tenant_id}", h.handleGetUsage)
	mux.HandleFunc("POST /v1/cloud/billing/invoices", h.handleGenerateInvoice)
	mux.HandleFunc("GET /v1/cloud/billing/invoices/{tenant_id}", h.handleGetInvoices)

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
	if err := strictDecode(r.Body, &t); err != nil {
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
	if err := strictDecode(r.Body, &inst); err != nil {
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
	if err := strictDecode(r.Body, &req); err != nil {
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
	if err := strictDecode(r.Body, &policy); err != nil {
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

func (h *CloudControlHandler) handleListPlans(w http.ResponseWriter, r *http.Request) {
	bm := h.cp.GetBilling()
	if bm == nil {
		bm = cloudcontrol.NewBillingManager()
		h.cp.AddBilling(bm)
	}
	plans := bm.ListPlans()
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"plans": plans,
		"count": len(plans),
	})
}

func (h *CloudControlHandler) handleRecordUsage(w http.ResponseWriter, r *http.Request) {
	bm := h.cp.GetBilling()
	if bm == nil {
		bm = cloudcontrol.NewBillingManager()
		h.cp.AddBilling(bm)
	}

	var record cloudcontrol.UsageRecord
	if err := strictDecode(r.Body, &record); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}
	if record.TenantID == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "tenant_id is required")
		return
	}

	bm.RecordUsage(record)
	h.writeJSON(r.Context(), w, http.StatusCreated, SuccessResponse{Success: true, Message: "usage recorded"})
}

func (h *CloudControlHandler) handleGetUsage(w http.ResponseWriter, r *http.Request) {
	bm := h.cp.GetBilling()
	if bm == nil {
		h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{"records": []interface{}{}, "count": 0})
		return
	}

	tenantID := r.PathValue("tenant_id")
	start := time.Now().AddDate(0, -1, 0) // default: last month
	end := time.Now()

	if s := r.URL.Query().Get("start"); s != "" {
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			start = t
		}
	}
	if e := r.URL.Query().Get("end"); e != "" {
		if t, err := time.Parse(time.RFC3339, e); err == nil {
			end = t
		}
	}

	records := bm.GetUsage(tenantID, start, end)
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"records": records,
		"count":   len(records),
	})
}

func (h *CloudControlHandler) handleGenerateInvoice(w http.ResponseWriter, r *http.Request) {
	bm := h.cp.GetBilling()
	if bm == nil {
		bm = cloudcontrol.NewBillingManager()
		h.cp.AddBilling(bm)
	}

	var req struct {
		TenantID    string `json:"tenant_id"`
		Tier        string `json:"tier"`
		PeriodStart string `json:"period_start"`
		PeriodEnd   string `json:"period_end"`
	}
	if err := strictDecode(r.Body, &req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.TenantID == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "tenant_id is required")
		return
	}

	tier := cloudcontrol.InstanceTier(req.Tier)
	if tier == "" {
		tier = cloudcontrol.TierStarter
	}

	start := time.Now().AddDate(0, -1, 0)
	end := time.Now()
	if req.PeriodStart != "" {
		if t, err := time.Parse(time.RFC3339, req.PeriodStart); err == nil {
			start = t
		}
	}
	if req.PeriodEnd != "" {
		if t, err := time.Parse(time.RFC3339, req.PeriodEnd); err == nil {
			end = t
		}
	}

	invoice, err := bm.GenerateInvoice(req.TenantID, tier, start, end)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusCreated, invoice)
}

func (h *CloudControlHandler) handleGetInvoices(w http.ResponseWriter, r *http.Request) {
	bm := h.cp.GetBilling()
	if bm == nil {
		h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{"invoices": []interface{}{}, "count": 0})
		return
	}

	tenantID := r.PathValue("tenant_id")
	invoices := bm.GetInvoices(tenantID)
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"invoices": invoices,
		"count":    len(invoices),
	})
}

func (h *CloudControlHandler) writeJSON(ctx context.Context, w http.ResponseWriter, status int, data interface{}) {
	writeJSONResponse(ctx, w, status, data)
}

func (h *CloudControlHandler) writeError(ctx context.Context, w http.ResponseWriter, status int, message string) {
	writeJSONError(ctx, w, status, message)
}
