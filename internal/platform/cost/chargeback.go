package cost

import (
	"fmt"
	"regexp"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ChargebackManager manages cost allocation and chargeback.
type ChargebackManager struct {
	mu       sync.RWMutex
	rules    map[string]*CostAllocationRule
	invoices map[string]*Invoice
	tracker  *Tracker
	taxRate  float64
	currency string
}

// NewChargebackManager creates a new chargeback manager.
func NewChargebackManager(tracker *Tracker) *ChargebackManager {
	return &ChargebackManager{
		rules:    make(map[string]*CostAllocationRule),
		invoices: make(map[string]*Invoice),
		tracker:  tracker,
		taxRate:  0,
		currency: "USD",
	}
}

// SetTaxRate sets the default tax rate.
func (m *ChargebackManager) SetTaxRate(rate float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.taxRate = rate
}

// CreateRule creates a cost allocation rule.
func (m *ChargebackManager) CreateRule(rule *CostAllocationRule) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if rule.ID == "" {
		rule.ID = uuid.New().String()
	}
	if rule.Name == "" {
		return fmt.Errorf("rule name is required")
	}
	if rule.CostCenter == "" {
		return fmt.Errorf("cost center is required")
	}
	if rule.Percentage <= 0 || rule.Percentage > 100 {
		return fmt.Errorf("percentage must be between 0 and 100")
	}

	// Validate pattern
	if rule.SourcePattern != "" {
		if _, err := regexp.Compile(rule.SourcePattern); err != nil {
			return fmt.Errorf("invalid source pattern: %w", err)
		}
	}

	rule.Active = true
	rule.CreatedAt = time.Now()
	rule.UpdatedAt = time.Now()

	m.rules[rule.ID] = rule
	return nil
}

// UpdateRule updates a cost allocation rule.
func (m *ChargebackManager) UpdateRule(rule *CostAllocationRule) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.rules[rule.ID]; !exists {
		return fmt.Errorf("rule not found: %s", rule.ID)
	}

	rule.UpdatedAt = time.Now()
	m.rules[rule.ID] = rule
	return nil
}

// GetRule returns a rule by ID.
func (m *ChargebackManager) GetRule(id string) (*CostAllocationRule, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rule, exists := m.rules[id]
	return rule, exists
}

// DeleteRule removes a rule.
func (m *ChargebackManager) DeleteRule(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.rules[id]; !exists {
		return fmt.Errorf("rule not found: %s", id)
	}

	delete(m.rules, id)
	return nil
}

// ListRules returns all allocation rules for a tenant.
func (m *ChargebackManager) ListRules(tenantID string) []*CostAllocationRule {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var rules []*CostAllocationRule
	for _, r := range m.rules {
		if tenantID == "" || r.TenantID == tenantID {
			rules = append(rules, r)
		}
	}

	// Sort by priority
	sort.Slice(rules, func(i, j int) bool {
		return rules[i].Priority < rules[j].Priority
	})

	return rules
}

// AllocateCosts allocates costs to cost centers based on rules.
func (m *ChargebackManager) AllocateCosts(tenantID string, start, end time.Time) map[string]*Chargeback {
	m.mu.RLock()
	rules := m.getSortedRules(tenantID)
	m.mu.RUnlock()

	entries := m.tracker.GetCosts(tenantID, start, end)
	chargebacks := make(map[string]*Chargeback)

	for _, entry := range entries {
		// Find matching rule
		var matchedRule *CostAllocationRule
		for _, rule := range rules {
			if !rule.Active {
				continue
			}
			if rule.SourcePattern != "" {
				re, err := regexp.Compile(rule.SourcePattern)
				if err != nil {
					continue
				}
				if !re.MatchString(entry.FeatureGroup) {
					continue
				}
			}
			matchedRule = rule
			break
		}

		costCenter := "_unallocated"
		percentage := 100.0
		if matchedRule != nil {
			costCenter = matchedRule.CostCenter
			percentage = matchedRule.Percentage
		}

		allocatedCost := entry.Cost * (percentage / 100)

		if chargebacks[costCenter] == nil {
			chargebacks[costCenter] = &Chargeback{
				ID:          uuid.New().String(),
				TenantID:    tenantID,
				CostCenter:  costCenter,
				PeriodStart: start,
				PeriodEnd:   end,
				Currency:    m.currency,
				GeneratedAt: time.Now(),
			}
		}

		chargebacks[costCenter].TotalCost += allocatedCost

		// Add breakdown item
		found := false
		for i := range chargebacks[costCenter].Breakdown {
			if chargebacks[costCenter].Breakdown[i].FeatureGroup == entry.FeatureGroup &&
				chargebacks[costCenter].Breakdown[i].Category == entry.Category {
				chargebacks[costCenter].Breakdown[i].Cost += allocatedCost
				found = true
				break
			}
		}
		if !found {
			chargebacks[costCenter].Breakdown = append(chargebacks[costCenter].Breakdown, ChargebackItem{
				FeatureGroup: entry.FeatureGroup,
				Category:     entry.Category,
				Cost:         allocatedCost,
			})
		}
	}

	// Calculate percentages
	var totalCost float64
	for _, cb := range chargebacks {
		totalCost += cb.TotalCost
	}
	for _, cb := range chargebacks {
		if totalCost > 0 {
			for i := range cb.Breakdown {
				cb.Breakdown[i].Percentage = (cb.Breakdown[i].Cost / totalCost) * 100
			}
		}
	}

	return chargebacks
}

// getSortedRules returns rules sorted by priority.
func (m *ChargebackManager) getSortedRules(tenantID string) []*CostAllocationRule {
	var rules []*CostAllocationRule
	for _, r := range m.rules {
		if tenantID == "" || r.TenantID == tenantID {
			rules = append(rules, r)
		}
	}
	sort.Slice(rules, func(i, j int) bool {
		return rules[i].Priority < rules[j].Priority
	})
	return rules
}

// GenerateInvoice generates an invoice for a tenant.
func (m *ChargebackManager) GenerateInvoice(tenantID string, start, end time.Time) (*Invoice, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	entries := m.tracker.GetCosts(tenantID, start, end)
	if len(entries) == 0 {
		return nil, fmt.Errorf("no costs found for period")
	}

	// Group by category
	categoryTotals := make(map[CostCategory]struct {
		quantity float64
		amount   float64
		unit     CostUnit
	})

	for _, e := range entries {
		cat := categoryTotals[e.Category]
		cat.quantity += e.Quantity
		cat.amount += e.Cost
		cat.unit = e.Unit
		categoryTotals[e.Category] = cat
	}

	// Create line items
	lineItems := make([]LineItem, 0, len(categoryTotals))
	var subtotal float64

	for cat, totals := range categoryTotals {
		item := LineItem{
			Description: fmt.Sprintf("%s usage", cat),
			Category:    cat,
			Unit:        totals.unit,
			Quantity:    totals.quantity,
			UnitPrice:   totals.amount / totals.quantity,
			Amount:      totals.amount,
		}
		lineItems = append(lineItems, item)
		subtotal += totals.amount
	}

	// Calculate tax
	tax := subtotal * m.taxRate

	invoice := &Invoice{
		ID:          uuid.New().String(),
		TenantID:    tenantID,
		PeriodStart: start,
		PeriodEnd:   end,
		GeneratedAt: time.Now(),
		DueDate:     time.Now().AddDate(0, 0, 30),
		Status:      InvoiceStatusDraft,
		Subtotal:    subtotal,
		Tax:         tax,
		TaxRate:     m.taxRate,
		Total:       subtotal + tax,
		Currency:    m.currency,
		LineItems:   lineItems,
	}

	m.invoices[invoice.ID] = invoice
	return invoice, nil
}

// GetInvoice returns an invoice by ID.
func (m *ChargebackManager) GetInvoice(id string) (*Invoice, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	invoice, exists := m.invoices[id]
	return invoice, exists
}

// ListInvoices returns invoices for a tenant.
func (m *ChargebackManager) ListInvoices(tenantID string) []*Invoice {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var invoices []*Invoice
	for _, inv := range m.invoices {
		if tenantID == "" || inv.TenantID == tenantID {
			invoices = append(invoices, inv)
		}
	}

	// Sort by generated date descending
	sort.Slice(invoices, func(i, j int) bool {
		return invoices[i].GeneratedAt.After(invoices[j].GeneratedAt)
	})

	return invoices
}

// UpdateInvoiceStatus updates the status of an invoice.
func (m *ChargebackManager) UpdateInvoiceStatus(id string, status InvoiceStatus) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	invoice, exists := m.invoices[id]
	if !exists {
		return fmt.Errorf("invoice not found: %s", id)
	}

	invoice.Status = status
	return nil
}

// ApplyCredit applies a credit to an invoice.
func (m *ChargebackManager) ApplyCredit(invoiceID string, credit CreditEntry) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	invoice, exists := m.invoices[invoiceID]
	if !exists {
		return fmt.Errorf("invoice not found: %s", invoiceID)
	}

	invoice.Credits = append(invoice.Credits, credit)
	invoice.Total -= credit.Amount
	if invoice.Total < 0 {
		invoice.Total = 0
	}

	return nil
}

// GenerateReport generates a cost report.
func (m *ChargebackManager) GenerateReport(config ReportConfig, start, end time.Time) (*CostReport, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var entries []CostEntry

	// Filter by tenant
	if config.TenantID != "" {
		entries = m.tracker.GetCosts(config.TenantID, start, end)
	} else {
		entries = m.tracker.GetCosts("", start, end)
	}

	// Filter by categories
	if len(config.Categories) > 0 {
		var filtered []CostEntry
		for _, e := range entries {
			for _, cat := range config.Categories {
				if e.Category == cat {
					filtered = append(filtered, e)
					break
				}
			}
		}
		entries = filtered
	}

	// Filter by feature groups
	if len(config.FeatureGroups) > 0 {
		var filtered []CostEntry
		for _, e := range entries {
			for _, fg := range config.FeatureGroups {
				if e.FeatureGroup == fg {
					filtered = append(filtered, e)
					break
				}
			}
		}
		entries = filtered
	}

	report := &CostReport{
		ID:          uuid.New().String(),
		Config:      config,
		PeriodStart: start,
		PeriodEnd:   end,
		GeneratedAt: time.Now(),
		Currency:    m.currency,
		ByCategory:  make(map[CostCategory]float64),
		ByFeature:   make(map[string]float64),
		ByTenant:    make(map[string]float64),
	}

	for _, e := range entries {
		report.TotalCost += e.Cost
		report.ByCategory[e.Category] += e.Cost
		if e.FeatureGroup != "" {
			report.ByFeature[e.FeatureGroup] += e.Cost
		}
		if e.TenantID != "" {
			report.ByTenant[e.TenantID] += e.Cost
		}
	}

	// Add cost center breakdown if rules exist
	if len(config.CostCenters) > 0 || len(m.rules) > 0 {
		chargebacks := m.AllocateCosts(config.TenantID, start, end)
		report.ByCostCenter = make(map[string]float64)
		for cc, cb := range chargebacks {
			if len(config.CostCenters) > 0 {
				found := false
				for _, wanted := range config.CostCenters {
					if cc == wanted {
						found = true
						break
					}
				}
				if !found {
					continue
				}
			}
			report.ByCostCenter[cc] = cb.TotalCost
		}
	}

	// Add time series
	report.TimeSeries = m.tracker.GetTimeSeries(config.TenantID, start, end, config.Granularity)

	// Add trends if requested
	if config.IncludeTrends {
		report.Trends = m.tracker.CalculateTrends(config.TenantID, start, end)
	}

	return report, nil
}
