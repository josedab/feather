package server

import (
	"net/http"
	"time"

	"github.com/feather-store/feather/internal/extensions/marketplace"
)

// BillingHandler provides HTTP endpoints for marketplace billing and revenue sharing.
type BillingHandler struct {
	billing *marketplace.BillingEngine
}

// NewBillingHandler creates a new billing handler.
func NewBillingHandler(billing *marketplace.BillingEngine) *BillingHandler {
	return &BillingHandler{billing: billing}
}

// RegisterRoutes registers billing API routes.
func (h *BillingHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/billing/plans", h.handleListPlans)
	mux.HandleFunc("POST /v1/billing/plans", h.handleCreatePlan)
	mux.HandleFunc("GET /v1/billing/plans/{id}", h.handleGetPlan)
	mux.HandleFunc("POST /v1/billing/usage", h.handleRecordUsage)
	mux.HandleFunc("POST /v1/billing/invoices/generate", h.handleGenerateInvoice)
	mux.HandleFunc("GET /v1/billing/invoices", h.handleListInvoices)
	mux.HandleFunc("GET /v1/billing/invoices/{id}", h.handleGetInvoice)
	mux.HandleFunc("POST /v1/billing/invoices/{id}/pay", h.handleProcessPayment)
	mux.HandleFunc("POST /v1/billing/invoices/{id}/distribute", h.handleDistributeRevenue)
	mux.HandleFunc("GET /v1/billing/revenue", h.handleGetRevenueShares)
	mux.HandleFunc("GET /v1/billing/stats", h.handleBillingStats)
}

func (h *BillingHandler) handleListPlans(w http.ResponseWriter, r *http.Request) {
	plans := h.billing.ListPlans()
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success": true,
		"plans":   plans,
	})
}

func (h *BillingHandler) handleCreatePlan(w http.ResponseWriter, r *http.Request) {
	var plan marketplace.BillingPlan
	if err := strictDecode(r.Body, &plan); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.billing.CreatePlan(&plan); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusCreated, map[string]interface{}{
		"success": true,
		"plan":    plan,
	})
}

func (h *BillingHandler) handleGetPlan(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	plan, err := h.billing.GetPlan(id)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success": true,
		"plan":    plan,
	})
}

func (h *BillingHandler) handleRecordUsage(w http.ResponseWriter, r *http.Request) {
	var req struct {
		FeatureID    string `json:"feature_id"`
		SubscriberID string `json:"subscriber_id"`
		Requests     int64  `json:"requests"`
		Bytes        int64  `json:"bytes"`
	}
	if err := strictDecode(r.Body, &req); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.FeatureID == "" || req.SubscriberID == "" {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "feature_id and subscriber_id are required")
		return
	}
	h.billing.RecordUsage(req.FeatureID, req.SubscriberID, req.Requests, req.Bytes)
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "usage recorded",
	})
}

func (h *BillingHandler) handleGenerateInvoice(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SubscriberID string    `json:"subscriber_id"`
		FeatureID    string    `json:"feature_id"`
		PlanID       string    `json:"plan_id"`
		PeriodStart  time.Time `json:"period_start"`
		PeriodEnd    time.Time `json:"period_end"`
	}
	if err := strictDecode(r.Body, &req); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}
	inv, err := h.billing.GenerateInvoice(req.SubscriberID, req.FeatureID, req.PlanID, req.PeriodStart, req.PeriodEnd)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusCreated, map[string]interface{}{
		"success": true,
		"invoice": inv,
	})
}

func (h *BillingHandler) handleListInvoices(w http.ResponseWriter, r *http.Request) {
	subscriberID := r.URL.Query().Get("subscriber_id")
	if subscriberID == "" {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "subscriber_id query parameter is required")
		return
	}
	invoices := h.billing.ListInvoices(subscriberID)
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success":  true,
		"invoices": invoices,
	})
}

func (h *BillingHandler) handleGetInvoice(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	inv, err := h.billing.GetInvoice(id)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success": true,
		"invoice": inv,
	})
}

func (h *BillingHandler) handleProcessPayment(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.billing.ProcessPayment(id); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "payment processed",
	})
}

func (h *BillingHandler) handleDistributeRevenue(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	rs, err := h.billing.DistributeRevenue(id)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success":       true,
		"revenue_share": rs,
	})
}

func (h *BillingHandler) handleGetRevenueShares(w http.ResponseWriter, r *http.Request) {
	ownerID := r.URL.Query().Get("owner_id")
	if ownerID == "" {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "owner_id query parameter is required")
		return
	}
	shares := h.billing.GetRevenueShares(ownerID)
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success":        true,
		"revenue_shares": shares,
	})
}

func (h *BillingHandler) handleBillingStats(w http.ResponseWriter, r *http.Request) {
	stats := h.billing.Stats()
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success": true,
		"stats":   stats,
	})
}
