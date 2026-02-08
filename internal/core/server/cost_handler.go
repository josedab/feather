package server

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/feather-store/feather/internal/platform/cost"
)

// CostHandler handles cost attribution API requests.
type CostHandler struct {
	tracker           *cost.Tracker
	budgetManager     *cost.BudgetManager
	chargebackManager *cost.ChargebackManager
}

// NewCostHandler creates a new cost handler.
func NewCostHandler(tracker *cost.Tracker, budgetManager *cost.BudgetManager, chargebackManager *cost.ChargebackManager) *CostHandler {
	return &CostHandler{
		tracker:           tracker,
		budgetManager:     budgetManager,
		chargebackManager: chargebackManager,
	}
}

// RegisterRoutes registers cost routes.
func (h *CostHandler) RegisterRoutes(mux *http.ServeMux) {
	// Usage tracking endpoints
	mux.HandleFunc("POST /v1/cost/usage", h.handleRecordUsage)
	mux.HandleFunc("GET /v1/cost/usage", h.handleGetUsage)
	mux.HandleFunc("GET /v1/cost/summary", h.handleGetCostSummary)

	// Rate endpoints
	mux.HandleFunc("GET /v1/cost/rates", h.handleListRates)
	mux.HandleFunc("PUT /v1/cost/rates", h.handleSetRate)

	// Budget endpoints
	mux.HandleFunc("GET /v1/cost/budgets", h.handleListBudgets)
	mux.HandleFunc("POST /v1/cost/budgets", h.handleCreateBudget)
	mux.HandleFunc("GET /v1/cost/budgets/{id}", h.handleGetBudget)
	mux.HandleFunc("PUT /v1/cost/budgets/{id}", h.handleUpdateBudget)
	mux.HandleFunc("DELETE /v1/cost/budgets/{id}", h.handleDeleteBudget)
	mux.HandleFunc("GET /v1/cost/budgets/{id}/status", h.handleGetBudgetStatus)

	// Alert endpoints
	mux.HandleFunc("GET /v1/cost/alerts", h.handleGetAlerts)
	mux.HandleFunc("POST /v1/cost/alerts/{id}/acknowledge", h.handleAcknowledgeAlert)

	// Chargeback/allocation endpoints
	mux.HandleFunc("GET /v1/cost/rules", h.handleListRules)
	mux.HandleFunc("POST /v1/cost/rules", h.handleCreateRule)
	mux.HandleFunc("GET /v1/cost/rules/{id}", h.handleGetRule)
	mux.HandleFunc("PUT /v1/cost/rules/{id}", h.handleUpdateRule)
	mux.HandleFunc("DELETE /v1/cost/rules/{id}", h.handleDeleteRule)

	// Invoice endpoints
	mux.HandleFunc("GET /v1/cost/invoices", h.handleListInvoices)
	mux.HandleFunc("POST /v1/cost/invoices", h.handleGenerateInvoice)
	mux.HandleFunc("GET /v1/cost/invoices/{id}", h.handleGetInvoice)
	mux.HandleFunc("PUT /v1/cost/invoices/{id}/status", h.handleUpdateInvoiceStatus)
	mux.HandleFunc("POST /v1/cost/invoices/{id}/credit", h.handleApplyCredit)

	// Report endpoints
	mux.HandleFunc("POST /v1/cost/reports", h.handleGenerateReport)
	mux.HandleFunc("GET /v1/cost/chargebacks", h.handleGetChargebacks)
}

// UsageRequest is the request body for recording usage.
type UsageRequest struct {
	TenantID     string            `json:"tenantId"`
	FeatureGroup string            `json:"featureGroup,omitempty"`
	Feature      string            `json:"feature,omitempty"`
	Category     string            `json:"category"`
	Unit         string            `json:"unit"`
	Quantity     float64           `json:"quantity"`
	Timestamp    *time.Time        `json:"timestamp,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

func (h *CostHandler) handleRecordUsage(w http.ResponseWriter, r *http.Request) {
	var req UsageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeJSON(r.Context(), w, http.StatusBadRequest, map[string]string{"error": "invalid JSON: " + err.Error()})
		return
	}

	if req.Category == "" {
		h.writeJSON(r.Context(), w, http.StatusBadRequest, map[string]string{"error": "category is required"})
		return
	}
	if req.Unit == "" {
		h.writeJSON(r.Context(), w, http.StatusBadRequest, map[string]string{"error": "unit is required"})
		return
	}
	if req.Quantity <= 0 {
		h.writeJSON(r.Context(), w, http.StatusBadRequest, map[string]string{"error": "quantity must be positive"})
		return
	}

	record := cost.UsageRecord{
		TenantID:     req.TenantID,
		FeatureGroup: req.FeatureGroup,
		Feature:      req.Feature,
		Category:     cost.CostCategory(req.Category),
		Unit:         cost.CostUnit(req.Unit),
		Quantity:     req.Quantity,
		Metadata:     req.Metadata,
	}
	if req.Timestamp != nil {
		record.Timestamp = *req.Timestamp
	}

	entry, err := h.tracker.RecordUsage(record)
	if err != nil {
		h.writeJSON(r.Context(), w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	h.writeJSON(r.Context(), w, http.StatusCreated, entry)
}

func (h *CostHandler) handleGetUsage(w http.ResponseWriter, r *http.Request) {
	tenantID := r.URL.Query().Get("tenant_id")
	start, end := h.parseTimeRange(r)

	records := h.tracker.GetUsage(tenantID, start, end)
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"records": records,
		"count":   len(records),
	})
}

func (h *CostHandler) handleGetCostSummary(w http.ResponseWriter, r *http.Request) {
	tenantID := r.URL.Query().Get("tenant_id")
	start, end := h.parseTimeRange(r)

	summary := h.tracker.GetCostSummary(tenantID, start, end)
	h.writeJSON(r.Context(), w, http.StatusOK, summary)
}

func (h *CostHandler) handleListRates(w http.ResponseWriter, r *http.Request) {
	rates := h.tracker.ListRates()
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"rates": rates,
		"count": len(rates),
	})
}

// SetRateRequest is the request body for setting a rate.
type SetRateRequest struct {
	Category      string  `json:"category"`
	Unit          string  `json:"unit"`
	PricePerUnit  float64 `json:"pricePerUnit"`
	Description   string  `json:"description,omitempty"`
	MinCharge     float64 `json:"minCharge,omitempty"`
	FreeAllowance float64 `json:"freeAllowance,omitempty"`
}

func (h *CostHandler) handleSetRate(w http.ResponseWriter, r *http.Request) {
	var req SetRateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeJSON(r.Context(), w, http.StatusBadRequest, map[string]string{"error": "invalid JSON: " + err.Error()})
		return
	}

	if req.Category == "" {
		h.writeJSON(r.Context(), w, http.StatusBadRequest, map[string]string{"error": "category is required"})
		return
	}
	if req.Unit == "" {
		h.writeJSON(r.Context(), w, http.StatusBadRequest, map[string]string{"error": "unit is required"})
		return
	}
	if req.PricePerUnit < 0 {
		h.writeJSON(r.Context(), w, http.StatusBadRequest, map[string]string{"error": "pricePerUnit must be non-negative"})
		return
	}

	rate := &cost.CostRate{
		Category:      cost.CostCategory(req.Category),
		Unit:          cost.CostUnit(req.Unit),
		PricePerUnit:  req.PricePerUnit,
		Description:   req.Description,
		MinCharge:     req.MinCharge,
		FreeAllowance: req.FreeAllowance,
	}

	h.tracker.SetRate(rate)
	h.writeJSON(r.Context(), w, http.StatusOK, rate)
}

// Budget endpoints

func (h *CostHandler) handleListBudgets(w http.ResponseWriter, r *http.Request) {
	tenantID := r.URL.Query().Get("tenant_id")
	budgets := h.budgetManager.ListBudgets(tenantID)
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"budgets": budgets,
		"count":   len(budgets),
	})
}

// BudgetRequest is the request body for creating/updating a budget.
type BudgetRequest struct {
	TenantID        string    `json:"tenantId"`
	Name            string    `json:"name"`
	Amount          float64   `json:"amount"`
	Currency        string    `json:"currency,omitempty"`
	Period          string    `json:"period,omitempty"`
	AlertThresholds []float64 `json:"alertThresholds,omitempty"`
	Categories      []string  `json:"categories,omitempty"`
	FeatureGroups   []string  `json:"featureGroups,omitempty"`
}

func (h *CostHandler) handleCreateBudget(w http.ResponseWriter, r *http.Request) {
	var req BudgetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeJSON(r.Context(), w, http.StatusBadRequest, map[string]string{"error": "invalid JSON: " + err.Error()})
		return
	}

	budget := &cost.Budget{
		TenantID:        req.TenantID,
		Name:            req.Name,
		Amount:          req.Amount,
		Currency:        req.Currency,
		Period:          cost.BudgetPeriod(req.Period),
		AlertThresholds: req.AlertThresholds,
		FeatureGroups:   req.FeatureGroups,
	}

	// Convert categories
	for _, cat := range req.Categories {
		budget.Categories = append(budget.Categories, cost.CostCategory(cat))
	}

	if err := h.budgetManager.CreateBudget(budget); err != nil {
		h.writeJSON(r.Context(), w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	h.writeJSON(r.Context(), w, http.StatusCreated, budget)
}

func (h *CostHandler) handleGetBudget(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	budget, exists := h.budgetManager.GetBudget(id)
	if !exists {
		h.writeJSON(r.Context(), w, http.StatusNotFound, map[string]string{"error": "budget not found"})
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, budget)
}

func (h *CostHandler) handleUpdateBudget(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req BudgetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeJSON(r.Context(), w, http.StatusBadRequest, map[string]string{"error": "invalid JSON: " + err.Error()})
		return
	}

	budget, exists := h.budgetManager.GetBudget(id)
	if !exists {
		h.writeJSON(r.Context(), w, http.StatusNotFound, map[string]string{"error": "budget not found"})
		return
	}

	// Update fields
	if req.Name != "" {
		budget.Name = req.Name
	}
	if req.Amount > 0 {
		budget.Amount = req.Amount
	}
	if req.Currency != "" {
		budget.Currency = req.Currency
	}
	if req.Period != "" {
		budget.Period = cost.BudgetPeriod(req.Period)
	}
	if len(req.AlertThresholds) > 0 {
		budget.AlertThresholds = req.AlertThresholds
	}
	if len(req.Categories) > 0 {
		budget.Categories = nil
		for _, cat := range req.Categories {
			budget.Categories = append(budget.Categories, cost.CostCategory(cat))
		}
	}
	if len(req.FeatureGroups) > 0 {
		budget.FeatureGroups = req.FeatureGroups
	}

	if err := h.budgetManager.UpdateBudget(budget); err != nil {
		h.writeJSON(r.Context(), w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, budget)
}

func (h *CostHandler) handleDeleteBudget(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	if err := h.budgetManager.DeleteBudget(id); err != nil {
		h.writeJSON(r.Context(), w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *CostHandler) handleGetBudgetStatus(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	status, err := h.budgetManager.GetBudgetStatus(id)
	if err != nil {
		h.writeJSON(r.Context(), w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, status)
}

// Alert endpoints

func (h *CostHandler) handleGetAlerts(w http.ResponseWriter, r *http.Request) {
	tenantID := r.URL.Query().Get("tenant_id")
	since := time.Time{}
	if sinceStr := r.URL.Query().Get("since"); sinceStr != "" {
		if parsed, err := time.Parse(time.RFC3339, sinceStr); err == nil {
			since = parsed
		}
	}

	alerts := h.budgetManager.GetAlerts(tenantID, since)
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"alerts": alerts,
		"count":  len(alerts),
	})
}

func (h *CostHandler) handleAcknowledgeAlert(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	if err := h.budgetManager.AcknowledgeAlert(id); err != nil {
		h.writeJSON(r.Context(), w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// Chargeback/allocation endpoints

func (h *CostHandler) handleListRules(w http.ResponseWriter, r *http.Request) {
	tenantID := r.URL.Query().Get("tenant_id")
	rules := h.chargebackManager.ListRules(tenantID)
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"rules": rules,
		"count": len(rules),
	})
}

// AllocationRuleRequest is the request body for creating/updating an allocation rule.
type AllocationRuleRequest struct {
	TenantID      string  `json:"tenantId"`
	Name          string  `json:"name"`
	Description   string  `json:"description,omitempty"`
	SourcePattern string  `json:"sourcePattern,omitempty"`
	CostCenter    string  `json:"costCenter"`
	Percentage    float64 `json:"percentage"`
	Priority      int     `json:"priority"`
}

func (h *CostHandler) handleCreateRule(w http.ResponseWriter, r *http.Request) {
	var req AllocationRuleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeJSON(r.Context(), w, http.StatusBadRequest, map[string]string{"error": "invalid JSON: " + err.Error()})
		return
	}

	rule := &cost.CostAllocationRule{
		TenantID:      req.TenantID,
		Name:          req.Name,
		Description:   req.Description,
		SourcePattern: req.SourcePattern,
		CostCenter:    req.CostCenter,
		Percentage:    req.Percentage,
		Priority:      req.Priority,
	}

	if err := h.chargebackManager.CreateRule(rule); err != nil {
		h.writeJSON(r.Context(), w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	h.writeJSON(r.Context(), w, http.StatusCreated, rule)
}

func (h *CostHandler) handleGetRule(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	rule, exists := h.chargebackManager.GetRule(id)
	if !exists {
		h.writeJSON(r.Context(), w, http.StatusNotFound, map[string]string{"error": "rule not found"})
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, rule)
}

func (h *CostHandler) handleUpdateRule(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req AllocationRuleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeJSON(r.Context(), w, http.StatusBadRequest, map[string]string{"error": "invalid JSON: " + err.Error()})
		return
	}

	rule, exists := h.chargebackManager.GetRule(id)
	if !exists {
		h.writeJSON(r.Context(), w, http.StatusNotFound, map[string]string{"error": "rule not found"})
		return
	}

	// Update fields
	if req.Name != "" {
		rule.Name = req.Name
	}
	if req.Description != "" {
		rule.Description = req.Description
	}
	if req.SourcePattern != "" {
		rule.SourcePattern = req.SourcePattern
	}
	if req.CostCenter != "" {
		rule.CostCenter = req.CostCenter
	}
	if req.Percentage > 0 {
		rule.Percentage = req.Percentage
	}
	if req.Priority > 0 {
		rule.Priority = req.Priority
	}

	if err := h.chargebackManager.UpdateRule(rule); err != nil {
		h.writeJSON(r.Context(), w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, rule)
}

func (h *CostHandler) handleDeleteRule(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	if err := h.chargebackManager.DeleteRule(id); err != nil {
		h.writeJSON(r.Context(), w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// Invoice endpoints

func (h *CostHandler) handleListInvoices(w http.ResponseWriter, r *http.Request) {
	tenantID := r.URL.Query().Get("tenant_id")
	invoices := h.chargebackManager.ListInvoices(tenantID)
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"invoices": invoices,
		"count":    len(invoices),
	})
}

// GenerateInvoiceRequest is the request body for generating an invoice.
type GenerateInvoiceRequest struct {
	TenantID string    `json:"tenantId"`
	Start    time.Time `json:"start"`
	End      time.Time `json:"end"`
}

func (h *CostHandler) handleGenerateInvoice(w http.ResponseWriter, r *http.Request) {
	var req GenerateInvoiceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeJSON(r.Context(), w, http.StatusBadRequest, map[string]string{"error": "invalid JSON: " + err.Error()})
		return
	}

	if req.TenantID == "" {
		h.writeJSON(r.Context(), w, http.StatusBadRequest, map[string]string{"error": "tenantId is required"})
		return
	}
	if req.Start.IsZero() || req.End.IsZero() {
		h.writeJSON(r.Context(), w, http.StatusBadRequest, map[string]string{"error": "start and end times are required"})
		return
	}

	invoice, err := h.chargebackManager.GenerateInvoice(req.TenantID, req.Start, req.End)
	if err != nil {
		h.writeJSON(r.Context(), w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	h.writeJSON(r.Context(), w, http.StatusCreated, invoice)
}

func (h *CostHandler) handleGetInvoice(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	invoice, exists := h.chargebackManager.GetInvoice(id)
	if !exists {
		h.writeJSON(r.Context(), w, http.StatusNotFound, map[string]string{"error": "invoice not found"})
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, invoice)
}

// UpdateInvoiceStatusRequest is the request body for updating invoice status.
type UpdateInvoiceStatusRequest struct {
	Status string `json:"status"`
}

func (h *CostHandler) handleUpdateInvoiceStatus(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req UpdateInvoiceStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeJSON(r.Context(), w, http.StatusBadRequest, map[string]string{"error": "invalid JSON: " + err.Error()})
		return
	}

	if err := h.chargebackManager.UpdateInvoiceStatus(id, cost.InvoiceStatus(req.Status)); err != nil {
		h.writeJSON(r.Context(), w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}

	invoice, _ := h.chargebackManager.GetInvoice(id)
	h.writeJSON(r.Context(), w, http.StatusOK, invoice)
}

// ApplyCreditRequest is the request body for applying a credit.
type ApplyCreditRequest struct {
	Description string  `json:"description"`
	Amount      float64 `json:"amount"`
	Reason      string  `json:"reason,omitempty"`
}

func (h *CostHandler) handleApplyCredit(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req ApplyCreditRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeJSON(r.Context(), w, http.StatusBadRequest, map[string]string{"error": "invalid JSON: " + err.Error()})
		return
	}

	if req.Amount <= 0 {
		h.writeJSON(r.Context(), w, http.StatusBadRequest, map[string]string{"error": "amount must be positive"})
		return
	}

	credit := cost.CreditEntry{
		Description: req.Description,
		Amount:      req.Amount,
		Reason:      req.Reason,
	}

	if err := h.chargebackManager.ApplyCredit(id, credit); err != nil {
		h.writeJSON(r.Context(), w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}

	invoice, _ := h.chargebackManager.GetInvoice(id)
	h.writeJSON(r.Context(), w, http.StatusOK, invoice)
}

// Report endpoints

// ReportRequest is the request body for generating a report.
type ReportRequest struct {
	TenantID      string    `json:"tenantId,omitempty"`
	FeatureGroups []string  `json:"featureGroups,omitempty"`
	Categories    []string  `json:"categories,omitempty"`
	CostCenters   []string  `json:"costCenters,omitempty"`
	Granularity   string    `json:"granularity,omitempty"`
	Format        string    `json:"format,omitempty"`
	IncludeTrends bool      `json:"includeTrends,omitempty"`
	Start         time.Time `json:"start"`
	End           time.Time `json:"end"`
}

func (h *CostHandler) handleGenerateReport(w http.ResponseWriter, r *http.Request) {
	var req ReportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeJSON(r.Context(), w, http.StatusBadRequest, map[string]string{"error": "invalid JSON: " + err.Error()})
		return
	}

	if req.Start.IsZero() || req.End.IsZero() {
		h.writeJSON(r.Context(), w, http.StatusBadRequest, map[string]string{"error": "start and end times are required"})
		return
	}

	config := cost.ReportConfig{
		TenantID:      req.TenantID,
		FeatureGroups: req.FeatureGroups,
		CostCenters:   req.CostCenters,
		Granularity:   req.Granularity,
		Format:        req.Format,
		IncludeTrends: req.IncludeTrends,
	}

	// Convert categories
	for _, cat := range req.Categories {
		config.Categories = append(config.Categories, cost.CostCategory(cat))
	}

	report, err := h.chargebackManager.GenerateReport(config, req.Start, req.End)
	if err != nil {
		h.writeJSON(r.Context(), w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, report)
}

// ChargebacksRequest is the request body for getting chargebacks.
type ChargebacksRequest struct {
	TenantID string    `json:"tenantId"`
	Start    time.Time `json:"start"`
	End      time.Time `json:"end"`
}

func (h *CostHandler) handleGetChargebacks(w http.ResponseWriter, r *http.Request) {
	tenantID := r.URL.Query().Get("tenant_id")
	start, end := h.parseTimeRange(r)

	chargebacks := h.chargebackManager.AllocateCosts(tenantID, start, end)
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"chargebacks": chargebacks,
		"count":       len(chargebacks),
	})
}

// Helper functions

func (h *CostHandler) parseTimeRange(r *http.Request) (time.Time, time.Time) {
	start := time.Now().AddDate(0, -1, 0) // Default: 1 month ago
	end := time.Now()

	if startStr := r.URL.Query().Get("start"); startStr != "" {
		if parsed, err := time.Parse(time.RFC3339, startStr); err == nil {
			start = parsed
		}
	}
	if endStr := r.URL.Query().Get("end"); endStr != "" {
		if parsed, err := time.Parse(time.RFC3339, endStr); err == nil {
			end = parsed
		}
	}

	return start, end
}

func (h *CostHandler) writeJSON(ctx context.Context, w http.ResponseWriter, status int, data interface{}) {
	if data == nil {
		w.WriteHeader(status)
		return
	}
	writeJSONResponse(ctx, w, status, data)
}
