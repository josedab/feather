package server

import (
	"net/http"

	"github.com/feather-store/feather/internal/platform/tenant"
)

// ---------------------------------------------------------------------------
// MultiTenantHandler
// ---------------------------------------------------------------------------

// MultiTenantHandler exposes multi-tenant isolation and usage metering endpoints.
type MultiTenantHandler struct {
	meter *tenant.UsageMeter
	costs tenant.CostConfig
}

// NewMultiTenantHandler creates a new MultiTenantHandler.
func NewMultiTenantHandler(meter *tenant.UsageMeter, costs tenant.CostConfig) *MultiTenantHandler {
	return &MultiTenantHandler{meter: meter, costs: costs}
}

// RegisterRoutes registers multi-tenant API routes.
func (h *MultiTenantHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/tenants/usage/record", h.handleRecord)
	mux.HandleFunc("GET /v1/tenants/usage/{tenantID}", h.handleGetUsage)
	mux.HandleFunc("GET /v1/tenants/usage/{tenantID}/costs", h.handleGetCosts)
	mux.HandleFunc("GET /v1/tenants/usage", h.handleGetAllUsage)
	mux.HandleFunc("POST /v1/tenants/usage/{tenantID}/reset", h.handleReset)
}

func (h *MultiTenantHandler) handleRecord(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TenantID string  `json:"tenant_id"`
		Metric   string  `json:"metric"`
		Value    float64 `json:"value"`
	}
	if err := strictDecode(r.Body, &req); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	if req.TenantID == "" || req.Metric == "" {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "tenant_id and metric are required")
		return
	}
	if err := h.meter.Record(req.TenantID, req.Metric, req.Value); err != nil {
		writeJSONError(r.Context(), w, http.StatusTooManyRequests, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]string{"status": "recorded"})
}

func (h *MultiTenantHandler) handleGetUsage(w http.ResponseWriter, r *http.Request) {
	tenantID := r.PathValue("tenantID")
	summary, err := h.meter.GetSummary(tenantID)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, summary)
}

func (h *MultiTenantHandler) handleGetCosts(w http.ResponseWriter, r *http.Request) {
	tenantID := r.PathValue("tenantID")
	costs, err := h.meter.GetCostAttribution(tenantID, h.costs)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	var total float64
	for _, c := range costs {
		total += c
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"tenant_id":  tenantID,
		"costs":      costs,
		"total_cost": total,
	})
}

func (h *MultiTenantHandler) handleGetAllUsage(w http.ResponseWriter, r *http.Request) {
	summaries := h.meter.GetAllSummaries()
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"summaries": summaries,
		"total":     len(summaries),
	})
}

func (h *MultiTenantHandler) handleReset(w http.ResponseWriter, r *http.Request) {
	tenantID := r.PathValue("tenantID")
	h.meter.Reset(tenantID)
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]string{"status": "reset", "tenant_id": tenantID})
}
