package marketplace

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// PricingModel defines how a feature is priced.
type PricingModel string

const (
	PricingFree        PricingModel = "free"
	PricingPerRequest  PricingModel = "per_request"
	PricingPerGB       PricingModel = "per_gb"
	PricingFlatMonthly PricingModel = "flat_monthly"
)

// BillingPlan defines the pricing terms for a marketplace feature.
type BillingPlan struct {
	ID           string       `json:"id"`
	Name         string       `json:"name"`
	PricingModel PricingModel `json:"pricing_model"`
	PricePerUnit float64      `json:"price_per_unit"`
	Currency     string       `json:"currency"`
	Features     []string     `json:"features"`
	CreatedAt    time.Time    `json:"created_at"`
}

// Invoice represents a billing invoice for feature usage.
type Invoice struct {
	ID             string    `json:"id"`
	SubscriberID   string    `json:"subscriber_id"`
	FeatureID      string    `json:"feature_id"`
	PlanID         string    `json:"plan_id"`
	PeriodStart    time.Time `json:"period_start"`
	PeriodEnd      time.Time `json:"period_end"`
	RequestCount   int64     `json:"request_count"`
	DataTransferGB float64   `json:"data_transfer_gb"`
	Amount         float64   `json:"amount"`
	Currency       string    `json:"currency"`
	Status         string    `json:"status"` // pending, paid, overdue
	CreatedAt      time.Time `json:"created_at"`
}

// RevenueShare tracks revenue distribution between feature owner and platform.
type RevenueShare struct {
	ID          string    `json:"id"`
	FeatureID   string    `json:"feature_id"`
	OwnerID     string    `json:"owner_id"`
	InvoiceID   string    `json:"invoice_id"`
	GrossAmount float64   `json:"gross_amount"`
	OwnerShare  float64   `json:"owner_share"`
	PlatformFee float64   `json:"platform_fee"`
	SplitRatio  float64   `json:"split_ratio"`
	Status      string    `json:"status"` // pending, distributed
	CreatedAt   time.Time `json:"created_at"`
}

// UsageMeter tracks real-time usage for a feature subscription.
type UsageMeter struct {
	FeatureID    string     `json:"feature_id"`
	SubscriberID string     `json:"subscriber_id"`
	RequestCount atomic.Int64
	DataBytes    atomic.Int64
	WindowStart  time.Time `json:"window_start"`
}

// BillingStats provides an overview of billing activity.
type BillingStats struct {
	TotalRevenue    float64 `json:"total_revenue"`
	OwnerPayouts    float64 `json:"owner_payouts"`
	PlatformRevenue float64 `json:"platform_revenue"`
	InvoiceCount    int     `json:"invoice_count"`
	PaidCount       int     `json:"paid_count"`
	OverdueCount    int     `json:"overdue_count"`
}

// BillingConfig holds configuration for the billing engine.
type BillingConfig struct {
	DefaultSplitRatio float64 `json:"default_split_ratio"` // fraction paid to owner (e.g. 0.80)
	Currency          string  `json:"currency"`
}

// DefaultBillingConfig returns sensible defaults (80/20 split, USD).
func DefaultBillingConfig() BillingConfig {
	return BillingConfig{
		DefaultSplitRatio: 0.80,
		Currency:          "USD",
	}
}

// BillingEngine manages plans, invoices, usage metering, and revenue sharing.
type BillingEngine struct {
	mu            sync.RWMutex
	plans         map[string]*BillingPlan
	invoices      map[string]*Invoice
	revenueShares map[string]*RevenueShare
	usageMeters   map[string]*UsageMeter // key: featureID:subscriberID
	config        BillingConfig
}

// NewBillingEngine creates a new billing engine with the given config.
func NewBillingEngine(cfg BillingConfig) *BillingEngine {
	return &BillingEngine{
		plans:         make(map[string]*BillingPlan),
		invoices:      make(map[string]*Invoice),
		revenueShares: make(map[string]*RevenueShare),
		usageMeters:   make(map[string]*UsageMeter),
		config:        cfg,
	}
}

// CreatePlan registers a new billing plan.
func (b *BillingEngine) CreatePlan(plan *BillingPlan) error {
	if plan.ID == "" {
		return fmt.Errorf("plan ID is required")
	}
	if plan.Name == "" {
		return fmt.Errorf("plan name is required")
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	if _, exists := b.plans[plan.ID]; exists {
		return fmt.Errorf("plan %q already exists", plan.ID)
	}
	plan.CreatedAt = time.Now()
	if plan.Currency == "" {
		plan.Currency = b.config.Currency
	}
	b.plans[plan.ID] = plan
	return nil
}

// GetPlan retrieves a billing plan by ID.
func (b *BillingEngine) GetPlan(id string) (*BillingPlan, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	plan, ok := b.plans[id]
	if !ok {
		return nil, fmt.Errorf("plan %q not found", id)
	}
	return plan, nil
}

// ListPlans returns all registered billing plans.
func (b *BillingEngine) ListPlans() []*BillingPlan {
	b.mu.RLock()
	defer b.mu.RUnlock()

	result := make([]*BillingPlan, 0, len(b.plans))
	for _, p := range b.plans {
		result = append(result, p)
	}
	return result
}

// RecordUsage adds usage data for a feature subscription.
func (b *BillingEngine) RecordUsage(featureID, subscriberID string, requests int64, bytes int64) {
	key := featureID + ":" + subscriberID

	b.mu.Lock()
	meter, ok := b.usageMeters[key]
	if !ok {
		meter = &UsageMeter{
			FeatureID:    featureID,
			SubscriberID: subscriberID,
			WindowStart:  time.Now(),
		}
		b.usageMeters[key] = meter
	}
	b.mu.Unlock()

	meter.RequestCount.Add(requests)
	meter.DataBytes.Add(bytes)
}

// GenerateInvoice creates an invoice for the given subscription and period.
func (b *BillingEngine) GenerateInvoice(subscriberID, featureID, planID string, periodStart, periodEnd time.Time) (*Invoice, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	plan, ok := b.plans[planID]
	if !ok {
		return nil, fmt.Errorf("plan %q not found", planID)
	}

	key := featureID + ":" + subscriberID
	meter := b.usageMeters[key]

	var requests int64
	var dataGB float64
	if meter != nil {
		requests = meter.RequestCount.Load()
		dataGB = float64(meter.DataBytes.Load()) / (1024 * 1024 * 1024)
	}

	var amount float64
	switch plan.PricingModel {
	case PricingFree:
		amount = 0
	case PricingPerRequest:
		amount = float64(requests) * plan.PricePerUnit
	case PricingPerGB:
		amount = dataGB * plan.PricePerUnit
	case PricingFlatMonthly:
		amount = plan.PricePerUnit
	default:
		return nil, fmt.Errorf("unknown pricing model %q", plan.PricingModel)
	}

	inv := &Invoice{
		ID:             fmt.Sprintf("inv-%s-%s-%d", subscriberID, featureID, time.Now().UnixNano()),
		SubscriberID:   subscriberID,
		FeatureID:      featureID,
		PlanID:         planID,
		PeriodStart:    periodStart,
		PeriodEnd:      periodEnd,
		RequestCount:   requests,
		DataTransferGB: dataGB,
		Amount:         amount,
		Currency:       plan.Currency,
		Status:         "pending",
		CreatedAt:      time.Now(),
	}
	b.invoices[inv.ID] = inv

	// Reset usage meter after invoice generation
	if meter != nil {
		meter.RequestCount.Store(0)
		meter.DataBytes.Store(0)
		meter.WindowStart = time.Now()
	}

	return inv, nil
}

// ProcessPayment marks an invoice as paid.
func (b *BillingEngine) ProcessPayment(invoiceID string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	inv, ok := b.invoices[invoiceID]
	if !ok {
		return fmt.Errorf("invoice %q not found", invoiceID)
	}
	if inv.Status == "paid" {
		return fmt.Errorf("invoice %q is already paid", invoiceID)
	}
	inv.Status = "paid"
	return nil
}

// DistributeRevenue splits the revenue for a paid invoice between owner and platform.
func (b *BillingEngine) DistributeRevenue(invoiceID string) (*RevenueShare, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	inv, ok := b.invoices[invoiceID]
	if !ok {
		return nil, fmt.Errorf("invoice %q not found", invoiceID)
	}
	if inv.Status != "paid" {
		return nil, fmt.Errorf("invoice %q must be paid before distributing revenue", invoiceID)
	}

	splitRatio := b.config.DefaultSplitRatio
	ownerShare := inv.Amount * splitRatio
	platformFee := inv.Amount - ownerShare

	rs := &RevenueShare{
		ID:          fmt.Sprintf("rs-%s-%d", invoiceID, time.Now().UnixNano()),
		FeatureID:   inv.FeatureID,
		OwnerID:     inv.FeatureID, // owner derived from feature
		InvoiceID:   invoiceID,
		GrossAmount: inv.Amount,
		OwnerShare:  ownerShare,
		PlatformFee: platformFee,
		SplitRatio:  splitRatio,
		Status:      "distributed",
		CreatedAt:   time.Now(),
	}
	b.revenueShares[rs.ID] = rs
	return rs, nil
}

// GetInvoice retrieves an invoice by ID.
func (b *BillingEngine) GetInvoice(id string) (*Invoice, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	inv, ok := b.invoices[id]
	if !ok {
		return nil, fmt.Errorf("invoice %q not found", id)
	}
	return inv, nil
}

// ListInvoices returns all invoices for a given subscriber.
func (b *BillingEngine) ListInvoices(subscriberID string) []*Invoice {
	b.mu.RLock()
	defer b.mu.RUnlock()

	var result []*Invoice
	for _, inv := range b.invoices {
		if inv.SubscriberID == subscriberID {
			result = append(result, inv)
		}
	}
	return result
}

// GetRevenueShares returns all revenue shares for a given owner.
func (b *BillingEngine) GetRevenueShares(ownerID string) []*RevenueShare {
	b.mu.RLock()
	defer b.mu.RUnlock()

	var result []*RevenueShare
	for _, rs := range b.revenueShares {
		if rs.OwnerID == ownerID {
			result = append(result, rs)
		}
	}
	return result
}

// Stats returns aggregate billing statistics.
func (b *BillingEngine) Stats() *BillingStats {
	b.mu.RLock()
	defer b.mu.RUnlock()

	stats := &BillingStats{}
	for _, inv := range b.invoices {
		stats.InvoiceCount++
		switch inv.Status {
		case "paid":
			stats.PaidCount++
			stats.TotalRevenue += inv.Amount
		case "overdue":
			stats.OverdueCount++
		}
	}
	for _, rs := range b.revenueShares {
		stats.OwnerPayouts += rs.OwnerShare
		stats.PlatformRevenue += rs.PlatformFee
	}
	return stats
}
