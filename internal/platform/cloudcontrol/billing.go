package cloudcontrol

import (
	"fmt"
	"sync"
	"time"
)

// BillingPlan defines pricing tiers for usage-based billing.
type BillingPlan struct {
	Tier          InstanceTier `json:"tier"`
	BasePriceUSD  float64      `json:"base_price_usd"`
	PricePerVCPUH float64      `json:"price_per_vcpu_hour"`
	PricePerGBH   float64      `json:"price_per_gb_hour"`
	PricePerMReq  float64      `json:"price_per_million_requests"`
	FreeRequests  int64        `json:"free_requests"`
	FreeStorage   float64      `json:"free_storage_gb"`
}

// UsageRecord tracks resource consumption for an instance.
type UsageRecord struct {
	InstanceID   string    `json:"instance_id"`
	TenantID     string    `json:"tenant_id"`
	Timestamp    time.Time `json:"timestamp"`
	VCPUHours    float64   `json:"vcpu_hours"`
	MemoryGBH    float64   `json:"memory_gb_hours"`
	RequestCount int64     `json:"request_count"`
	StorageGB    float64   `json:"storage_gb"`
}

// Invoice represents a billing invoice for a tenant.
type Invoice struct {
	ID              string         `json:"id"`
	TenantID        string         `json:"tenant_id"`
	PeriodStart     time.Time      `json:"period_start"`
	PeriodEnd       time.Time      `json:"period_end"`
	LineItems       []InvoiceItem  `json:"line_items"`
	TotalUSD        float64        `json:"total_usd"`
	Status          InvoiceStatus  `json:"status"`
	CreatedAt       time.Time      `json:"created_at"`
}

// InvoiceItem is a single line item on an invoice.
type InvoiceItem struct {
	Description string  `json:"description"`
	Quantity    float64 `json:"quantity"`
	UnitPrice   float64 `json:"unit_price"`
	AmountUSD   float64 `json:"amount_usd"`
}

// InvoiceStatus represents the state of an invoice.
type InvoiceStatus string

const (
	InvoiceDraft   InvoiceStatus = "draft"
	InvoicePending InvoiceStatus = "pending"
	InvoicePaid    InvoiceStatus = "paid"
)

var defaultPlans = map[InstanceTier]BillingPlan{
	TierFree: {
		Tier:          TierFree,
		BasePriceUSD:  0,
		PricePerVCPUH: 0,
		PricePerGBH:   0,
		PricePerMReq:  0,
		FreeRequests:  1_000_000,
		FreeStorage:   1.0,
	},
	TierStarter: {
		Tier:          TierStarter,
		BasePriceUSD:  29.0,
		PricePerVCPUH: 0.05,
		PricePerGBH:   0.008,
		PricePerMReq:  0.50,
		FreeRequests:  10_000_000,
		FreeStorage:   10.0,
	},
	TierPro: {
		Tier:          TierPro,
		BasePriceUSD:  199.0,
		PricePerVCPUH: 0.04,
		PricePerGBH:   0.006,
		PricePerMReq:  0.30,
		FreeRequests:  100_000_000,
		FreeStorage:   100.0,
	},
	TierEnterprise: {
		Tier:          TierEnterprise,
		BasePriceUSD:  999.0,
		PricePerVCPUH: 0.03,
		PricePerGBH:   0.004,
		PricePerMReq:  0.15,
		FreeRequests:  1_000_000_000,
		FreeStorage:   1000.0,
	},
}

// BillingManager handles usage-based billing for tenants.
type BillingManager struct {
	mu       sync.RWMutex
	plans    map[InstanceTier]BillingPlan
	usage    map[string][]UsageRecord // tenantID -> records
	invoices map[string][]Invoice     // tenantID -> invoices
}

// NewBillingManager creates a new billing manager.
func NewBillingManager() *BillingManager {
	plans := make(map[InstanceTier]BillingPlan)
	for k, v := range defaultPlans {
		plans[k] = v
	}
	return &BillingManager{
		plans:    plans,
		usage:    make(map[string][]UsageRecord),
		invoices: make(map[string][]Invoice),
	}
}

// RecordUsage records a usage data point for billing.
func (bm *BillingManager) RecordUsage(record UsageRecord) {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	if record.Timestamp.IsZero() {
		record.Timestamp = time.Now()
	}
	bm.usage[record.TenantID] = append(bm.usage[record.TenantID], record)
}

// GetUsage returns usage records for a tenant within a time range.
func (bm *BillingManager) GetUsage(tenantID string, start, end time.Time) []UsageRecord {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	var filtered []UsageRecord
	for _, r := range bm.usage[tenantID] {
		if !r.Timestamp.Before(start) && r.Timestamp.Before(end) {
			filtered = append(filtered, r)
		}
	}
	return filtered
}

// GenerateInvoice creates an invoice for a tenant's usage in a period.
func (bm *BillingManager) GenerateInvoice(tenantID string, tier InstanceTier, start, end time.Time) (*Invoice, error) {
	plan, exists := bm.plans[tier]
	if !exists {
		return nil, fmt.Errorf("unknown billing tier: %s", tier)
	}

	records := bm.GetUsage(tenantID, start, end)

	var totalVCPU, totalMemGB, totalStorage float64
	var totalRequests int64
	for _, r := range records {
		totalVCPU += r.VCPUHours
		totalMemGB += r.MemoryGBH
		totalRequests += r.RequestCount
		if r.StorageGB > totalStorage {
			totalStorage = r.StorageGB
		}
	}

	invoice := &Invoice{
		ID:          fmt.Sprintf("inv-%s-%d", tenantID, time.Now().UnixNano()),
		TenantID:    tenantID,
		PeriodStart: start,
		PeriodEnd:   end,
		Status:      InvoiceDraft,
		CreatedAt:   time.Now(),
	}

	// Base price
	if plan.BasePriceUSD > 0 {
		invoice.LineItems = append(invoice.LineItems, InvoiceItem{
			Description: fmt.Sprintf("%s plan base fee", tier),
			Quantity:    1,
			UnitPrice:   plan.BasePriceUSD,
			AmountUSD:   plan.BasePriceUSD,
		})
	}

	// Compute
	if totalVCPU > 0 && plan.PricePerVCPUH > 0 {
		amount := totalVCPU * plan.PricePerVCPUH
		invoice.LineItems = append(invoice.LineItems, InvoiceItem{
			Description: "Compute (vCPU-hours)",
			Quantity:    totalVCPU,
			UnitPrice:   plan.PricePerVCPUH,
			AmountUSD:   amount,
		})
	}

	// Memory
	if totalMemGB > 0 && plan.PricePerGBH > 0 {
		amount := totalMemGB * plan.PricePerGBH
		invoice.LineItems = append(invoice.LineItems, InvoiceItem{
			Description: "Memory (GB-hours)",
			Quantity:    totalMemGB,
			UnitPrice:   plan.PricePerGBH,
			AmountUSD:   amount,
		})
	}

	// Requests (subtract free tier)
	billableReqs := totalRequests - plan.FreeRequests
	if billableReqs > 0 && plan.PricePerMReq > 0 {
		millions := float64(billableReqs) / 1_000_000
		amount := millions * plan.PricePerMReq
		invoice.LineItems = append(invoice.LineItems, InvoiceItem{
			Description: "API requests (millions, over free tier)",
			Quantity:    millions,
			UnitPrice:   plan.PricePerMReq,
			AmountUSD:   amount,
		})
	}

	for _, item := range invoice.LineItems {
		invoice.TotalUSD += item.AmountUSD
	}

	bm.mu.Lock()
	bm.invoices[tenantID] = append(bm.invoices[tenantID], *invoice)
	bm.mu.Unlock()

	return invoice, nil
}

// GetInvoices returns all invoices for a tenant.
func (bm *BillingManager) GetInvoices(tenantID string) []Invoice {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	out := make([]Invoice, len(bm.invoices[tenantID]))
	copy(out, bm.invoices[tenantID])
	return out
}

// GetPlan returns the billing plan for a tier.
func (bm *BillingManager) GetPlan(tier InstanceTier) (*BillingPlan, bool) {
	bm.mu.RLock()
	defer bm.mu.RUnlock()
	p, ok := bm.plans[tier]
	if !ok {
		return nil, false
	}
	return &p, true
}

// ListPlans returns all available billing plans.
func (bm *BillingManager) ListPlans() []BillingPlan {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	plans := make([]BillingPlan, 0, len(bm.plans))
	for _, p := range bm.plans {
		plans = append(plans, p)
	}
	return plans
}

// AddBilling adds billing management to the control plane.
func (cp *ControlPlane) AddBilling(bm *BillingManager) {
	cp.mu.Lock()
	defer cp.mu.Unlock()
	cp.billing = bm
}

// GetBilling returns the billing manager.
func (cp *ControlPlane) GetBilling() *BillingManager {
	cp.mu.RLock()
	defer cp.mu.RUnlock()
	return cp.billing
}
