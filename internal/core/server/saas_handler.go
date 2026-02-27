package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/feather-store/feather/internal/core/logging"
	"github.com/feather-store/feather/internal/platform/saas"
)

// SaaSHandler handles SaaS management API endpoints.
type SaaSHandler struct {
	planRegistry        *saas.PlanRegistry
	billingManager      *saas.BillingManager
	provisioningManager *saas.ProvisioningManager
}

// NewSaaSHandler creates a new SaaS handler.
func NewSaaSHandler(planRegistry *saas.PlanRegistry, billingManager *saas.BillingManager, provisioningManager *saas.ProvisioningManager) *SaaSHandler {
	return &SaaSHandler{
		planRegistry:        planRegistry,
		billingManager:      billingManager,
		provisioningManager: provisioningManager,
	}
}

// RegisterRoutes registers SaaS API routes.
func (h *SaaSHandler) RegisterRoutes(mux *http.ServeMux) {
	// Plan endpoints
	mux.HandleFunc("GET /v1/saas/plans", h.handleListPlans)
	mux.HandleFunc("GET /v1/saas/plans/{id}", h.handleGetPlan)
	mux.HandleFunc("POST /v1/saas/plans", h.handleCreatePlan)
	mux.HandleFunc("PUT /v1/saas/plans/{id}", h.handleUpdatePlan)
	mux.HandleFunc("DELETE /v1/saas/plans/{id}", h.handleDeactivatePlan)
	mux.HandleFunc("GET /v1/saas/plans/compare", h.handleComparePlans)

	// Subscription endpoints
	mux.HandleFunc("GET /v1/saas/subscriptions", h.handleListSubscriptions)
	mux.HandleFunc("GET /v1/saas/subscriptions/{id}", h.handleGetSubscription)
	mux.HandleFunc("POST /v1/saas/subscriptions", h.handleCreateSubscription)
	mux.HandleFunc("PUT /v1/saas/subscriptions/{id}/plan", h.handleChangePlan)
	mux.HandleFunc("DELETE /v1/saas/subscriptions/{id}", h.handleCancelSubscription)

	// Instance provisioning endpoints
	mux.HandleFunc("GET /v1/saas/instances", h.handleListInstances)
	mux.HandleFunc("GET /v1/saas/instances/{id}", h.handleGetInstance)
	mux.HandleFunc("POST /v1/saas/instances", h.handleCreateInstance)
	mux.HandleFunc("PUT /v1/saas/instances/{id}/config", h.handleUpdateInstance)
	mux.HandleFunc("PUT /v1/saas/instances/{id}/resize", h.handleResizeInstance)
	mux.HandleFunc("POST /v1/saas/instances/{id}/start", h.handleStartInstance)
	mux.HandleFunc("POST /v1/saas/instances/{id}/stop", h.handleStopInstance)
	mux.HandleFunc("POST /v1/saas/instances/{id}/restart", h.handleRestartInstance)
	mux.HandleFunc("DELETE /v1/saas/instances/{id}", h.handleTerminateInstance)
	mux.HandleFunc("GET /v1/saas/instances/{id}/metrics", h.handleGetInstanceMetrics)

	// Billing endpoints
	mux.HandleFunc("POST /v1/saas/usage", h.handleRecordUsage)
	mux.HandleFunc("GET /v1/saas/usage", h.handleGetUsageSummary)
	mux.HandleFunc("GET /v1/saas/invoices", h.handleListInvoices)
	mux.HandleFunc("GET /v1/saas/invoices/{id}", h.handleGetInvoice)
	mux.HandleFunc("POST /v1/saas/invoices", h.handleGenerateInvoice)
	mux.HandleFunc("POST /v1/saas/invoices/{id}/finalize", h.handleFinalizeInvoice)
	mux.HandleFunc("POST /v1/saas/invoices/{id}/paid", h.handleMarkInvoicePaid)

	// Payment method endpoints
	mux.HandleFunc("GET /v1/saas/payment-methods", h.handleListPaymentMethods)
	mux.HandleFunc("POST /v1/saas/payment-methods", h.handleAddPaymentMethod)

	// Region and size info
	mux.HandleFunc("GET /v1/saas/regions", h.handleGetRegions)
	mux.HandleFunc("GET /v1/saas/sizes", h.handleGetSizes)
}

// Plan handlers

func (h *SaaSHandler) handleListPlans(w http.ResponseWriter, r *http.Request) {
	plans := h.planRegistry.ListPlans()
	h.writeJSON(r.Context(), w, http.StatusOK, plans)
}

func (h *SaaSHandler) handleGetPlan(w http.ResponseWriter, r *http.Request) {
	planID := r.PathValue("id")
	plan, err := h.planRegistry.GetPlan(planID)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusOK, plan)
}

func (h *SaaSHandler) handleCreatePlan(w http.ResponseWriter, r *http.Request) {
	var plan saas.Plan
	if err := strictDecode(r.Body, &plan); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.planRegistry.RegisterPlan(&plan); err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, saas.ErrPlanAlreadyExists) || errors.Is(err, saas.ErrInvalidPlan) {
			status = http.StatusBadRequest
		}
		h.writeError(r.Context(), w, status, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusCreated, plan)
}

func (h *SaaSHandler) handleUpdatePlan(w http.ResponseWriter, r *http.Request) {
	planID := r.PathValue("id")

	var plan saas.Plan
	if err := strictDecode(r.Body, &plan); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}
	plan.ID = planID

	if err := h.planRegistry.UpdatePlan(&plan); err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, saas.ErrPlanNotFound) {
			status = http.StatusNotFound
		}
		h.writeError(r.Context(), w, status, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusOK, plan)
}

func (h *SaaSHandler) handleDeactivatePlan(w http.ResponseWriter, r *http.Request) {
	planID := r.PathValue("id")

	if err := h.planRegistry.DeactivatePlan(planID); err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, saas.ErrPlanNotFound) {
			status = http.StatusNotFound
		}
		h.writeError(r.Context(), w, status, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *SaaSHandler) handleComparePlans(w http.ResponseWriter, r *http.Request) {
	plan1 := r.URL.Query().Get("plan1")
	plan2 := r.URL.Query().Get("plan2")

	if plan1 == "" || plan2 == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "plan1 and plan2 query parameters required")
		return
	}

	comparison, err := h.planRegistry.ComparePlans(plan1, plan2)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusOK, comparison)
}

// Subscription handlers

func (h *SaaSHandler) handleListSubscriptions(w http.ResponseWriter, r *http.Request) {
	orgID := r.URL.Query().Get("org_id")
	if orgID == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "org_id query parameter required")
		return
	}

	subs := h.billingManager.GetSubscriptionByOrg(orgID)
	h.writeJSON(r.Context(), w, http.StatusOK, subs)
}

func (h *SaaSHandler) handleGetSubscription(w http.ResponseWriter, r *http.Request) {
	subID := r.PathValue("id")

	sub, err := h.billingManager.GetSubscription(subID)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusOK, sub)
}

func (h *SaaSHandler) handleCreateSubscription(w http.ResponseWriter, r *http.Request) {
	var req struct {
		OrganizationID string             `json:"organization_id"`
		PlanID         string             `json:"plan_id"`
		BillingPeriod  saas.BillingPeriod `json:"billing_period"`
	}
	if err := strictDecode(r.Body, &req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	sub, err := h.billingManager.CreateSubscription(req.OrganizationID, req.PlanID, req.BillingPeriod)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusCreated, sub)
}

func (h *SaaSHandler) handleChangePlan(w http.ResponseWriter, r *http.Request) {
	subID := r.PathValue("id")

	var req struct {
		PlanID string `json:"plan_id"`
	}
	if err := strictDecode(r.Body, &req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.billingManager.ChangePlan(subID, req.PlanID); err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, saas.ErrSubscriptionNotFound) {
			status = http.StatusNotFound
		}
		h.writeError(r.Context(), w, status, err.Error())
		return
	}

	sub, _ := h.billingManager.GetSubscription(subID)
	h.writeJSON(r.Context(), w, http.StatusOK, sub)
}

func (h *SaaSHandler) handleCancelSubscription(w http.ResponseWriter, r *http.Request) {
	subID := r.PathValue("id")
	immediate := r.URL.Query().Get("immediate") == "true"

	if err := h.billingManager.CancelSubscription(subID, immediate); err != nil {
		h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Instance handlers

func (h *SaaSHandler) handleListInstances(w http.ResponseWriter, r *http.Request) {
	orgID := r.URL.Query().Get("org_id")
	if orgID == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "org_id query parameter required")
		return
	}

	instances := h.provisioningManager.ListInstances(orgID)
	h.writeJSON(r.Context(), w, http.StatusOK, instances)
}

func (h *SaaSHandler) handleGetInstance(w http.ResponseWriter, r *http.Request) {
	instanceID := r.PathValue("id")

	instance, err := h.provisioningManager.GetInstance(instanceID)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusOK, instance)
}

func (h *SaaSHandler) handleCreateInstance(w http.ResponseWriter, r *http.Request) {
	var req saas.ProvisioningRequest
	if err := strictDecode(r.Body, &req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	instance, err := h.provisioningManager.CreateInstance(&req)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, saas.ErrInstanceLimitExceeded) {
			status = http.StatusForbidden
		}
		h.writeError(r.Context(), w, status, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusCreated, instance)
}

func (h *SaaSHandler) handleUpdateInstance(w http.ResponseWriter, r *http.Request) {
	instanceID := r.PathValue("id")

	var config saas.InstanceConfig
	if err := strictDecode(r.Body, &config); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.provisioningManager.UpdateInstance(instanceID, config); err != nil {
		h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}

	instance, _ := h.provisioningManager.GetInstance(instanceID)
	h.writeJSON(r.Context(), w, http.StatusOK, instance)
}

func (h *SaaSHandler) handleResizeInstance(w http.ResponseWriter, r *http.Request) {
	instanceID := r.PathValue("id")

	var req struct {
		Size saas.InstanceSize `json:"size"`
	}
	if err := strictDecode(r.Body, &req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.provisioningManager.ResizeInstance(instanceID, req.Size); err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, saas.ErrInstanceNotFound) {
			status = http.StatusNotFound
		}
		h.writeError(r.Context(), w, status, err.Error())
		return
	}

	instance, _ := h.provisioningManager.GetInstance(instanceID)
	h.writeJSON(r.Context(), w, http.StatusOK, instance)
}

func (h *SaaSHandler) handleStartInstance(w http.ResponseWriter, r *http.Request) {
	instanceID := r.PathValue("id")

	if err := h.provisioningManager.StartInstance(instanceID); err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, saas.ErrInstanceNotFound) {
			status = http.StatusNotFound
		}
		h.writeError(r.Context(), w, status, err.Error())
		return
	}

	instance, _ := h.provisioningManager.GetInstance(instanceID)
	h.writeJSON(r.Context(), w, http.StatusOK, instance)
}

func (h *SaaSHandler) handleStopInstance(w http.ResponseWriter, r *http.Request) {
	instanceID := r.PathValue("id")

	if err := h.provisioningManager.StopInstance(r.Context(), instanceID); err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, saas.ErrInstanceNotFound) {
			status = http.StatusNotFound
		}
		h.writeError(r.Context(), w, status, err.Error())
		return
	}

	instance, _ := h.provisioningManager.GetInstance(instanceID)
	h.writeJSON(r.Context(), w, http.StatusOK, instance)
}

func (h *SaaSHandler) handleRestartInstance(w http.ResponseWriter, r *http.Request) {
	instanceID := r.PathValue("id")

	if err := h.provisioningManager.RestartInstance(r.Context(), instanceID); err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, saas.ErrInstanceNotFound) {
			status = http.StatusNotFound
		}
		h.writeError(r.Context(), w, status, err.Error())
		return
	}

	instance, _ := h.provisioningManager.GetInstance(instanceID)
	h.writeJSON(r.Context(), w, http.StatusOK, instance)
}

func (h *SaaSHandler) handleTerminateInstance(w http.ResponseWriter, r *http.Request) {
	instanceID := r.PathValue("id")

	if err := h.provisioningManager.TerminateInstance(r.Context(), instanceID); err != nil {
		h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *SaaSHandler) handleGetInstanceMetrics(w http.ResponseWriter, r *http.Request) {
	instanceID := r.PathValue("id")

	metrics, err := h.provisioningManager.GetInstanceMetrics(instanceID)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusOK, metrics)
}

// Billing handlers

func (h *SaaSHandler) handleRecordUsage(w http.ResponseWriter, r *http.Request) {
	var record saas.UsageRecord
	if err := strictDecode(r.Body, &record); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.billingManager.RecordUsage(record); err != nil {
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusCreated)
}

func (h *SaaSHandler) handleGetUsageSummary(w http.ResponseWriter, r *http.Request) {
	orgID := r.URL.Query().Get("org_id")
	startStr := r.URL.Query().Get("start")
	endStr := r.URL.Query().Get("end")

	if orgID == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "org_id query parameter required")
		return
	}

	start, end, err := parseDateRange(startStr, endStr)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}

	summary, err := h.billingManager.GetUsageSummary(orgID, start, end)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusOK, summary)
}

func (h *SaaSHandler) handleListInvoices(w http.ResponseWriter, r *http.Request) {
	orgID := r.URL.Query().Get("org_id")
	if orgID == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "org_id query parameter required")
		return
	}

	invoices := h.billingManager.ListInvoices(orgID)
	h.writeJSON(r.Context(), w, http.StatusOK, invoices)
}

func (h *SaaSHandler) handleGetInvoice(w http.ResponseWriter, r *http.Request) {
	invoiceID := r.PathValue("id")

	invoice, err := h.billingManager.GetInvoice(invoiceID)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusOK, invoice)
}

func (h *SaaSHandler) handleGenerateInvoice(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SubscriptionID string `json:"subscription_id"`
	}
	if err := strictDecode(r.Body, &req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	invoice, err := h.billingManager.GenerateInvoice(req.SubscriptionID)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, saas.ErrSubscriptionNotFound) {
			status = http.StatusNotFound
		}
		h.writeError(r.Context(), w, status, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusCreated, invoice)
}

func (h *SaaSHandler) handleFinalizeInvoice(w http.ResponseWriter, r *http.Request) {
	invoiceID := r.PathValue("id")

	if err := h.billingManager.FinalizeInvoice(invoiceID); err != nil {
		h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}

	invoice, _ := h.billingManager.GetInvoice(invoiceID)
	h.writeJSON(r.Context(), w, http.StatusOK, invoice)
}

func (h *SaaSHandler) handleMarkInvoicePaid(w http.ResponseWriter, r *http.Request) {
	invoiceID := r.PathValue("id")

	if err := h.billingManager.MarkInvoicePaid(invoiceID); err != nil {
		h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}

	invoice, _ := h.billingManager.GetInvoice(invoiceID)
	h.writeJSON(r.Context(), w, http.StatusOK, invoice)
}

// Payment method handlers

func (h *SaaSHandler) handleListPaymentMethods(w http.ResponseWriter, r *http.Request) {
	orgID := r.URL.Query().Get("org_id")
	if orgID == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "org_id query parameter required")
		return
	}

	methods := h.billingManager.GetPaymentMethods(orgID)
	h.writeJSON(r.Context(), w, http.StatusOK, methods)
}

func (h *SaaSHandler) handleAddPaymentMethod(w http.ResponseWriter, r *http.Request) {
	orgID := r.URL.Query().Get("org_id")
	if orgID == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "org_id query parameter required")
		return
	}

	var method saas.PaymentMethod
	if err := strictDecode(r.Body, &method); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.billingManager.AddPaymentMethod(orgID, &method); err != nil {
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusCreated, method)
}

// Region and size handlers

func (h *SaaSHandler) handleGetRegions(w http.ResponseWriter, r *http.Request) {
	regions := h.provisioningManager.GetRegions()
	h.writeJSON(r.Context(), w, http.StatusOK, regions)
}

func (h *SaaSHandler) handleGetSizes(w http.ResponseWriter, r *http.Request) {
	sizes := h.provisioningManager.GetInstanceSizes()

	// Convert to a more JSON-friendly format
	result := make([]map[string]interface{}, 0)
	for size, spec := range sizes {
		result = append(result, map[string]interface{}{
			"id":             size,
			"vcpus":          spec.VCPUs,
			"memory_gb":      spec.MemoryGB,
			"price_per_hour": spec.PricePerHour,
		})
	}
	h.writeJSON(r.Context(), w, http.StatusOK, result)
}

// Helper methods

func (h *SaaSHandler) writeJSON(ctx context.Context, w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		logging.FromContext(ctx, nil).Error("failed to encode JSON response", "error", err)
	}
}

func (h *SaaSHandler) writeError(ctx context.Context, w http.ResponseWriter, status int, message string) {
	h.writeJSON(ctx, w, status, map[string]string{"error": message})
}

func parseDateRange(startStr, endStr string) (start, end time.Time, err error) {
	if startStr == "" || endStr == "" {
		// Default to last 30 days
		end = time.Now()
		start = end.AddDate(0, 0, -30)
		return
	}

	start, err = time.Parse(time.RFC3339, startStr)
	if err != nil {
		start, err = time.Parse("2006-01-02", startStr)
		if err != nil {
			return time.Time{}, time.Time{}, err
		}
	}

	end, err = time.Parse(time.RFC3339, endStr)
	if err != nil {
		end, err = time.Parse("2006-01-02", endStr)
		if err != nil {
			return time.Time{}, time.Time{}, err
		}
	}
	return
}
