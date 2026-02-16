// Package saas provides subscription, billing, and provisioning support.
package saas

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// Billing errors
var (
	ErrInvoiceNotFound      = errors.New("invoice not found")
	ErrPaymentFailed        = errors.New("payment failed")
	ErrInvalidBillingPeriod = errors.New("invalid billing period")
)

// UsageMetric represents a usage metric type.
type UsageMetric string

// UsageMetric constants for usage tracking.
const (
	MetricRequests     UsageMetric = "requests"
	MetricStorage      UsageMetric = "storage_gb"
	MetricEntities     UsageMetric = "entities"
	MetricVectorOps    UsageMetric = "vector_operations"
	MetricDataTransfer UsageMetric = "data_transfer_gb"
	MetricComputeHours UsageMetric = "compute_hours"
)

// UsageRecord represents a usage event.
type UsageRecord struct {
	ID             string            `json:"id"`
	OrganizationID string            `json:"organization_id"`
	SubscriptionID string            `json:"subscription_id"`
	Metric         UsageMetric       `json:"metric"`
	Quantity       float64           `json:"quantity"`
	Timestamp      time.Time         `json:"timestamp"`
	Metadata       map[string]string `json:"metadata,omitempty"`
}

// UsageSummary aggregates usage for a period.
type UsageSummary struct {
	OrganizationID string                             `json:"organization_id"`
	PeriodStart    time.Time                          `json:"period_start"`
	PeriodEnd      time.Time                          `json:"period_end"`
	Metrics        map[UsageMetric]UsageMetricSummary `json:"metrics"`
}

// UsageMetricSummary summarizes a single metric.
type UsageMetricSummary struct {
	Total       float64 `json:"total"`
	Included    float64 `json:"included"`
	Overage     float64 `json:"overage"`
	OverageCost float64 `json:"overage_cost"`
	Peak        float64 `json:"peak"`
	Average     float64 `json:"average"`
	Count       int64   `json:"count"`
}

// Invoice represents a billing invoice.
type Invoice struct {
	ID             string            `json:"id"`
	OrganizationID string            `json:"organization_id"`
	SubscriptionID string            `json:"subscription_id"`
	Number         string            `json:"number"`
	Status         InvoiceStatus     `json:"status"`
	PeriodStart    time.Time         `json:"period_start"`
	PeriodEnd      time.Time         `json:"period_end"`
	Subtotal       float64           `json:"subtotal"`
	Tax            float64           `json:"tax"`
	Total          float64           `json:"total"`
	Currency       string            `json:"currency"`
	LineItems      []InvoiceLineItem `json:"line_items"`
	DueDate        time.Time         `json:"due_date"`
	PaidAt         *time.Time        `json:"paid_at,omitempty"`
	CreatedAt      time.Time         `json:"created_at"`
}

// InvoiceStatus defines invoice states.
type InvoiceStatus string

// InvoiceStatus constants for invoices.
const (
	InvoiceDraft         InvoiceStatus = "draft"
	InvoiceOpen          InvoiceStatus = "open"
	InvoicePaid          InvoiceStatus = "paid"
	InvoiceVoid          InvoiceStatus = "void"
	InvoiceUncollectible InvoiceStatus = "uncollectible"
)

// InvoiceLineItem represents a line item on an invoice.
type InvoiceLineItem struct {
	Description string  `json:"description"`
	Quantity    float64 `json:"quantity"`
	UnitPrice   float64 `json:"unit_price"`
	Amount      float64 `json:"amount"`
	Type        string  `json:"type"` // subscription, overage, credit, adjustment
}

// PaymentMethod represents a customer's payment method.
type PaymentMethod struct {
	ID             string    `json:"id"`
	OrganizationID string    `json:"organization_id"`
	Type           string    `json:"type"` // card, bank_account, invoice
	Last4          string    `json:"last4,omitempty"`
	ExpiryMonth    int       `json:"expiry_month,omitempty"`
	ExpiryYear     int       `json:"expiry_year,omitempty"`
	Brand          string    `json:"brand,omitempty"`
	IsDefault      bool      `json:"is_default"`
	CreatedAt      time.Time `json:"created_at"`
}

// BillingManager handles billing operations.
type BillingManager struct {
	planRegistry   *PlanRegistry
	subscriptions  map[string]*Subscription
	usageRecords   []UsageRecord
	invoices       map[string]*Invoice
	paymentMethods map[string][]*PaymentMethod
	mu             sync.RWMutex
}

// NewBillingManager creates a new billing manager.
func NewBillingManager(planRegistry *PlanRegistry) *BillingManager {
	return &BillingManager{
		planRegistry:   planRegistry,
		subscriptions:  make(map[string]*Subscription),
		usageRecords:   make([]UsageRecord, 0),
		invoices:       make(map[string]*Invoice),
		paymentMethods: make(map[string][]*PaymentMethod),
	}
}

// CreateSubscription creates a new subscription.
func (m *BillingManager) CreateSubscription(orgID, planID string, period BillingPeriod) (*Subscription, error) {
	plan, err := m.planRegistry.GetPlan(planID)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	periodEnd := now.AddDate(0, 1, 0) // Default to monthly
	if period == BillingYearly {
		periodEnd = now.AddDate(1, 0, 0)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	subID := generateID("sub")
	sub := &Subscription{
		ID:                 subID,
		OrganizationID:     orgID,
		PlanID:             plan.ID,
		Status:             SubscriptionActive,
		BillingPeriod:      period,
		CurrentPeriodStart: now,
		CurrentPeriodEnd:   periodEnd,
		CreatedAt:          now,
		UpdatedAt:          now,
	}

	m.subscriptions[subID] = sub
	return sub, nil
}

// GetSubscription retrieves a subscription.
func (m *BillingManager) GetSubscription(id string) (*Subscription, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sub, exists := m.subscriptions[id]
	if !exists {
		return nil, ErrSubscriptionNotFound
	}
	return sub, nil
}

// GetSubscriptionByOrg retrieves subscriptions for an organization.
func (m *BillingManager) GetSubscriptionByOrg(orgID string) []*Subscription {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Subscription, 0)
	for _, sub := range m.subscriptions {
		if sub.OrganizationID == orgID {
			result = append(result, sub)
		}
	}
	return result
}

// UpdateSubscription updates a subscription.
func (m *BillingManager) UpdateSubscription(sub *Subscription) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.subscriptions[sub.ID]; !exists {
		return ErrSubscriptionNotFound
	}

	sub.UpdatedAt = time.Now()
	m.subscriptions[sub.ID] = sub
	return nil
}

// CancelSubscription schedules cancellation at period end.
func (m *BillingManager) CancelSubscription(id string, immediate bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	sub, exists := m.subscriptions[id]
	if !exists {
		return ErrSubscriptionNotFound
	}

	if immediate {
		sub.Status = SubscriptionCanceled
	} else {
		sub.CancelAtPeriodEnd = true
	}
	sub.UpdatedAt = time.Now()
	return nil
}

// ChangePlan changes a subscription's plan.
func (m *BillingManager) ChangePlan(subID, newPlanID string) error {
	_, err := m.planRegistry.GetPlan(newPlanID)
	if err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	sub, exists := m.subscriptions[subID]
	if !exists {
		return ErrSubscriptionNotFound
	}

	sub.PlanID = newPlanID
	sub.UpdatedAt = time.Now()
	return nil
}

// RecordUsage records a usage event.
func (m *BillingManager) RecordUsage(record UsageRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if record.ID == "" {
		record.ID = generateID("usage")
	}
	if record.Timestamp.IsZero() {
		record.Timestamp = time.Now()
	}

	m.usageRecords = append(m.usageRecords, record)
	return nil
}

// GetUsageSummary returns usage summary for a period.
func (m *BillingManager) GetUsageSummary(orgID string, start, end time.Time) (*UsageSummary, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	summary := &UsageSummary{
		OrganizationID: orgID,
		PeriodStart:    start,
		PeriodEnd:      end,
		Metrics:        make(map[UsageMetric]UsageMetricSummary),
	}

	// Aggregate usage records
	metricTotals := make(map[UsageMetric][]float64)
	for _, record := range m.usageRecords {
		if record.OrganizationID != orgID {
			continue
		}
		if record.Timestamp.Before(start) || record.Timestamp.After(end) {
			continue
		}
		metricTotals[record.Metric] = append(metricTotals[record.Metric], record.Quantity)
	}

	// Calculate summaries
	for metric, values := range metricTotals {
		var total, peak, sum float64
		for _, v := range values {
			total += v
			sum += v
			if v > peak {
				peak = v
			}
		}
		avg := float64(0)
		if len(values) > 0 {
			avg = sum / float64(len(values))
		}

		summary.Metrics[metric] = UsageMetricSummary{
			Total:   total,
			Peak:    peak,
			Average: avg,
			Count:   int64(len(values)),
		}
	}

	return summary, nil
}

// GenerateInvoice creates an invoice for a subscription.
func (m *BillingManager) GenerateInvoice(subID string) (*Invoice, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	sub, exists := m.subscriptions[subID]
	if !exists {
		return nil, ErrSubscriptionNotFound
	}

	plan, err := m.planRegistry.GetPlan(sub.PlanID)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	invoiceID := generateID("inv")

	// Calculate base price
	basePrice := plan.Pricing.MonthlyPrice
	if sub.BillingPeriod == BillingYearly {
		basePrice = plan.Pricing.YearlyPrice
	}

	invoice := &Invoice{
		ID:             invoiceID,
		OrganizationID: sub.OrganizationID,
		SubscriptionID: sub.ID,
		Number:         generateInvoiceNumber(),
		Status:         InvoiceDraft,
		PeriodStart:    sub.CurrentPeriodStart,
		PeriodEnd:      sub.CurrentPeriodEnd,
		Currency:       plan.Pricing.Currency,
		DueDate:        now.AddDate(0, 0, 30),
		CreatedAt:      now,
	}

	// Add subscription line item
	invoice.LineItems = append(invoice.LineItems, InvoiceLineItem{
		Description: plan.Name + " - " + string(sub.BillingPeriod) + " subscription",
		Quantity:    1,
		UnitPrice:   basePrice,
		Amount:      basePrice,
		Type:        "subscription",
	})

	// Calculate overage charges
	usageSummary, _ := m.getUsageSummaryUnlocked(sub.OrganizationID, sub.CurrentPeriodStart, sub.CurrentPeriodEnd)
	for metric, summary := range usageSummary.Metrics {
		included := float64(0)
		if units, ok := plan.Pricing.IncludedUnits[string(metric)]; ok {
			included = float64(units)
		}

		overage := summary.Total - included
		if overage > 0 {
			rate := plan.Pricing.OverageRates[string(metric)]
			overageCost := overage * rate
			if overageCost > 0 {
				invoice.LineItems = append(invoice.LineItems, InvoiceLineItem{
					Description: string(metric) + " overage",
					Quantity:    overage,
					UnitPrice:   rate,
					Amount:      overageCost,
					Type:        "overage",
				})
			}
		}
	}

	// Calculate totals
	for _, item := range invoice.LineItems {
		invoice.Subtotal += item.Amount
	}
	invoice.Tax = invoice.Subtotal * 0 // No tax for now
	invoice.Total = invoice.Subtotal + invoice.Tax

	m.invoices[invoiceID] = invoice
	return invoice, nil
}

func (m *BillingManager) getUsageSummaryUnlocked(orgID string, start, end time.Time) (*UsageSummary, error) {
	summary := &UsageSummary{
		OrganizationID: orgID,
		PeriodStart:    start,
		PeriodEnd:      end,
		Metrics:        make(map[UsageMetric]UsageMetricSummary),
	}

	metricTotals := make(map[UsageMetric][]float64)
	for _, record := range m.usageRecords {
		if record.OrganizationID != orgID {
			continue
		}
		if record.Timestamp.Before(start) || record.Timestamp.After(end) {
			continue
		}
		metricTotals[record.Metric] = append(metricTotals[record.Metric], record.Quantity)
	}

	for metric, values := range metricTotals {
		var total float64
		for _, v := range values {
			total += v
		}
		summary.Metrics[metric] = UsageMetricSummary{
			Total: total,
		}
	}

	return summary, nil
}

// GetInvoice retrieves an invoice.
func (m *BillingManager) GetInvoice(id string) (*Invoice, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	invoice, exists := m.invoices[id]
	if !exists {
		return nil, ErrInvoiceNotFound
	}
	return invoice, nil
}

// ListInvoices returns invoices for an organization.
func (m *BillingManager) ListInvoices(orgID string) []*Invoice {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Invoice, 0)
	for _, inv := range m.invoices {
		if inv.OrganizationID == orgID {
			result = append(result, inv)
		}
	}
	return result
}

// FinalizeInvoice marks an invoice as open.
func (m *BillingManager) FinalizeInvoice(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	invoice, exists := m.invoices[id]
	if !exists {
		return ErrInvoiceNotFound
	}

	invoice.Status = InvoiceOpen
	return nil
}

// MarkInvoicePaid marks an invoice as paid.
func (m *BillingManager) MarkInvoicePaid(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	invoice, exists := m.invoices[id]
	if !exists {
		return ErrInvoiceNotFound
	}

	now := time.Now()
	invoice.Status = InvoicePaid
	invoice.PaidAt = &now
	return nil
}

// AddPaymentMethod adds a payment method for an organization.
func (m *BillingManager) AddPaymentMethod(orgID string, method *PaymentMethod) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if method.ID == "" {
		method.ID = generateID("pm")
	}
	method.OrganizationID = orgID
	method.CreatedAt = time.Now()

	methods := m.paymentMethods[orgID]

	// If this is the first or marked as default, update others
	if method.IsDefault || len(methods) == 0 {
		method.IsDefault = true
		for _, pm := range methods {
			pm.IsDefault = false
		}
	}

	m.paymentMethods[orgID] = append(methods, method)
	return nil
}

// GetPaymentMethods returns payment methods for an organization.
func (m *BillingManager) GetPaymentMethods(orgID string) []*PaymentMethod {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.paymentMethods[orgID]
}

// Helper functions
var idCounter int64

func generateID(prefix string) string {
	n := atomic.AddInt64(&idCounter, 1)
	return fmt.Sprintf("%s_%s_%06d", prefix, time.Now().Format("20060102"), n)
}

func generateInvoiceNumber() string {
	n := atomic.AddInt64(&idCounter, 1)
	return fmt.Sprintf("INV-%s-%06d", time.Now().Format("200601"), 1000+n)
}
