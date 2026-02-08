package cost

import (
	"testing"
	"time"
)

func TestNewChargebackManager(t *testing.T) {
	tracker := NewTracker("USD")
	manager := NewChargebackManager(tracker)

	if manager == nil {
		t.Fatal("Expected non-nil manager")
	}
	if manager.tracker != tracker {
		t.Error("Expected tracker to be set")
	}
	if manager.currency != "USD" {
		t.Errorf("Expected USD currency, got %s", manager.currency)
	}
}

func TestChargebackManager_SetTaxRate(t *testing.T) {
	tracker := NewTracker("USD")
	manager := NewChargebackManager(tracker)

	manager.SetTaxRate(0.10)
	if manager.taxRate != 0.10 {
		t.Errorf("Expected tax rate 0.10, got %f", manager.taxRate)
	}
}

func TestChargebackManager_CreateRule(t *testing.T) {
	tracker := NewTracker("USD")
	manager := NewChargebackManager(tracker)

	rule := &CostAllocationRule{
		TenantID:   "tenant-1",
		Name:       "Engineering Allocation",
		CostCenter: "engineering",
		Percentage: 100,
	}

	err := manager.CreateRule(rule)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if rule.ID == "" {
		t.Error("Expected rule to have ID")
	}
	if !rule.Active {
		t.Error("Expected rule to be active")
	}
	if rule.CreatedAt.IsZero() {
		t.Error("Expected rule to have CreatedAt")
	}
}

func TestChargebackManager_CreateRule_Validation(t *testing.T) {
	tracker := NewTracker("USD")
	manager := NewChargebackManager(tracker)

	tests := []struct {
		name    string
		rule    *CostAllocationRule
		wantErr bool
	}{
		{
			name:    "missing name",
			rule:    &CostAllocationRule{CostCenter: "eng", Percentage: 100},
			wantErr: true,
		},
		{
			name:    "missing cost center",
			rule:    &CostAllocationRule{Name: "Test", Percentage: 100},
			wantErr: true,
		},
		{
			name:    "zero percentage",
			rule:    &CostAllocationRule{Name: "Test", CostCenter: "eng", Percentage: 0},
			wantErr: true,
		},
		{
			name:    "negative percentage",
			rule:    &CostAllocationRule{Name: "Test", CostCenter: "eng", Percentage: -10},
			wantErr: true,
		},
		{
			name:    "over 100 percentage",
			rule:    &CostAllocationRule{Name: "Test", CostCenter: "eng", Percentage: 150},
			wantErr: true,
		},
		{
			name:    "invalid source pattern",
			rule:    &CostAllocationRule{Name: "Test", CostCenter: "eng", Percentage: 100, SourcePattern: "[invalid"},
			wantErr: true,
		},
		{
			name:    "valid rule",
			rule:    &CostAllocationRule{Name: "Test", CostCenter: "eng", Percentage: 100},
			wantErr: false,
		},
		{
			name:    "valid with pattern",
			rule:    &CostAllocationRule{Name: "Test", CostCenter: "eng", Percentage: 50, SourcePattern: "^ml-.*"},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := manager.CreateRule(tt.rule)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateRule() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestChargebackManager_UpdateRule(t *testing.T) {
	tracker := NewTracker("USD")
	manager := NewChargebackManager(tracker)

	rule := &CostAllocationRule{
		Name:       "Test",
		CostCenter: "eng",
		Percentage: 100,
	}
	manager.CreateRule(rule)

	// Update the rule
	rule.Percentage = 50
	err := manager.UpdateRule(rule)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Verify update
	retrieved, exists := manager.GetRule(rule.ID)
	if !exists {
		t.Fatal("Expected rule to exist")
	}
	if retrieved.Percentage != 50 {
		t.Errorf("Expected percentage 50, got %f", retrieved.Percentage)
	}
}

func TestChargebackManager_UpdateRule_NotFound(t *testing.T) {
	tracker := NewTracker("USD")
	manager := NewChargebackManager(tracker)

	err := manager.UpdateRule(&CostAllocationRule{ID: "nonexistent"})
	if err == nil {
		t.Error("Expected error for nonexistent rule")
	}
}

func TestChargebackManager_GetRule(t *testing.T) {
	tracker := NewTracker("USD")
	manager := NewChargebackManager(tracker)

	rule := &CostAllocationRule{
		Name:       "Test",
		CostCenter: "eng",
		Percentage: 100,
	}
	manager.CreateRule(rule)

	retrieved, exists := manager.GetRule(rule.ID)
	if !exists {
		t.Fatal("Expected rule to exist")
	}
	if retrieved.Name != "Test" {
		t.Errorf("Expected 'Test', got %s", retrieved.Name)
	}
}

func TestChargebackManager_GetRule_NotFound(t *testing.T) {
	tracker := NewTracker("USD")
	manager := NewChargebackManager(tracker)

	_, exists := manager.GetRule("nonexistent")
	if exists {
		t.Error("Expected rule not to exist")
	}
}

func TestChargebackManager_DeleteRule(t *testing.T) {
	tracker := NewTracker("USD")
	manager := NewChargebackManager(tracker)

	rule := &CostAllocationRule{
		Name:       "Test",
		CostCenter: "eng",
		Percentage: 100,
	}
	manager.CreateRule(rule)

	err := manager.DeleteRule(rule.ID)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	_, exists := manager.GetRule(rule.ID)
	if exists {
		t.Error("Expected rule to be deleted")
	}
}

func TestChargebackManager_DeleteRule_NotFound(t *testing.T) {
	tracker := NewTracker("USD")
	manager := NewChargebackManager(tracker)

	err := manager.DeleteRule("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent rule")
	}
}

func TestChargebackManager_ListRules(t *testing.T) {
	tracker := NewTracker("USD")
	manager := NewChargebackManager(tracker)

	// Create rules for different tenants with different priorities
	manager.CreateRule(&CostAllocationRule{TenantID: "tenant-1", Name: "Rule 1", CostCenter: "eng", Percentage: 100, Priority: 2})
	manager.CreateRule(&CostAllocationRule{TenantID: "tenant-1", Name: "Rule 2", CostCenter: "ml", Percentage: 100, Priority: 1})
	manager.CreateRule(&CostAllocationRule{TenantID: "tenant-2", Name: "Rule 3", CostCenter: "data", Percentage: 100, Priority: 1})

	// List all rules
	all := manager.ListRules("")
	if len(all) != 3 {
		t.Errorf("Expected 3 rules, got %d", len(all))
	}

	// List tenant-1 rules (should be sorted by priority)
	tenant1 := manager.ListRules("tenant-1")
	if len(tenant1) != 2 {
		t.Errorf("Expected 2 rules for tenant-1, got %d", len(tenant1))
	}
	if tenant1[0].Priority > tenant1[1].Priority {
		t.Error("Expected rules sorted by priority")
	}
}

func TestChargebackManager_AllocateCosts(t *testing.T) {
	tracker := NewTracker("USD")
	manager := NewChargebackManager(tracker)
	now := time.Now()

	// Create allocation rules
	manager.CreateRule(&CostAllocationRule{
		TenantID:      "tenant-1",
		Name:          "ML Allocation",
		CostCenter:    "ml-team",
		SourcePattern: "^ml-.*",
		Percentage:    100,
		Priority:      1,
	})
	manager.CreateRule(&CostAllocationRule{
		TenantID:   "tenant-1",
		Name:       "Default",
		CostCenter: "general",
		Percentage: 100,
		Priority:   10,
	})

	// Record some usage
	tracker.RecordUsage(UsageRecord{
		TenantID:     "tenant-1",
		FeatureGroup: "ml-embeddings",
		Category:     CostCategoryML,
		Unit:         CostUnitTokens,
		Quantity:     10000,
		Timestamp:    now,
	})
	tracker.RecordUsage(UsageRecord{
		TenantID:     "tenant-1",
		FeatureGroup: "api-gateway",
		Category:     CostCategoryAPI,
		Unit:         CostUnitRequests,
		Quantity:     5000,
		Timestamp:    now,
	})

	chargebacks := manager.AllocateCosts("tenant-1", now.Add(-time.Hour), now.Add(time.Hour))

	if len(chargebacks) < 2 {
		t.Errorf("Expected at least 2 cost centers, got %d", len(chargebacks))
	}

	// Check ML team allocation
	if mlChargeback, ok := chargebacks["ml-team"]; ok {
		if mlChargeback.TotalCost <= 0 {
			t.Error("Expected positive cost for ml-team")
		}
	} else {
		t.Error("Expected ml-team cost center")
	}

	// Check general allocation
	if generalChargeback, ok := chargebacks["general"]; ok {
		if generalChargeback.TotalCost <= 0 {
			t.Error("Expected positive cost for general")
		}
	} else {
		t.Error("Expected general cost center")
	}
}

func TestChargebackManager_AllocateCosts_Unallocated(t *testing.T) {
	tracker := NewTracker("USD")
	manager := NewChargebackManager(tracker)
	now := time.Now()

	// No rules, so costs go to _unallocated
	tracker.RecordUsage(UsageRecord{
		TenantID:     "tenant-1",
		FeatureGroup: "test",
		Category:     CostCategoryAPI,
		Unit:         CostUnitRequests,
		Quantity:     1000,
		Timestamp:    now,
	})

	chargebacks := manager.AllocateCosts("tenant-1", now.Add(-time.Hour), now.Add(time.Hour))

	if unallocated, ok := chargebacks["_unallocated"]; ok {
		if unallocated.TotalCost <= 0 {
			t.Error("Expected positive cost for _unallocated")
		}
	} else {
		t.Error("Expected _unallocated cost center")
	}
}

func TestChargebackManager_GenerateInvoice(t *testing.T) {
	tracker := NewTracker("USD")
	manager := NewChargebackManager(tracker)
	manager.SetTaxRate(0.10)
	now := time.Now()

	// Record usage
	tracker.RecordUsage(UsageRecord{
		TenantID:  "tenant-1",
		Category:  CostCategoryAPI,
		Unit:      CostUnitRequests,
		Quantity:  10000,
		Timestamp: now,
	})
	tracker.RecordUsage(UsageRecord{
		TenantID:  "tenant-1",
		Category:  CostCategoryML,
		Unit:      CostUnitTokens,
		Quantity:  50000,
		Timestamp: now,
	})

	invoice, err := manager.GenerateInvoice("tenant-1", now.Add(-time.Hour), now.Add(time.Hour))
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if invoice.ID == "" {
		t.Error("Expected invoice to have ID")
	}
	if invoice.TenantID != "tenant-1" {
		t.Errorf("Expected tenant-1, got %s", invoice.TenantID)
	}
	if invoice.Subtotal <= 0 {
		t.Error("Expected positive subtotal")
	}
	if invoice.Tax <= 0 {
		t.Error("Expected positive tax with 10% rate")
	}
	if invoice.Total != invoice.Subtotal+invoice.Tax {
		t.Error("Expected total = subtotal + tax")
	}
	if invoice.Status != InvoiceStatusDraft {
		t.Errorf("Expected draft status, got %s", invoice.Status)
	}
	if len(invoice.LineItems) != 2 {
		t.Errorf("Expected 2 line items, got %d", len(invoice.LineItems))
	}
}

func TestChargebackManager_GenerateInvoice_NoCosts(t *testing.T) {
	tracker := NewTracker("USD")
	manager := NewChargebackManager(tracker)
	now := time.Now()

	_, err := manager.GenerateInvoice("tenant-1", now.Add(-time.Hour), now.Add(time.Hour))
	if err == nil {
		t.Error("Expected error for no costs")
	}
}

func TestChargebackManager_GetInvoice(t *testing.T) {
	tracker := NewTracker("USD")
	manager := NewChargebackManager(tracker)
	now := time.Now()

	// Record usage to generate invoice
	tracker.RecordUsage(UsageRecord{
		TenantID:  "tenant-1",
		Category:  CostCategoryAPI,
		Unit:      CostUnitRequests,
		Quantity:  1000,
		Timestamp: now,
	})

	invoice, _ := manager.GenerateInvoice("tenant-1", now.Add(-time.Hour), now.Add(time.Hour))

	retrieved, exists := manager.GetInvoice(invoice.ID)
	if !exists {
		t.Fatal("Expected invoice to exist")
	}
	if retrieved.ID != invoice.ID {
		t.Errorf("Expected ID %s, got %s", invoice.ID, retrieved.ID)
	}
}

func TestChargebackManager_GetInvoice_NotFound(t *testing.T) {
	tracker := NewTracker("USD")
	manager := NewChargebackManager(tracker)

	_, exists := manager.GetInvoice("nonexistent")
	if exists {
		t.Error("Expected invoice not to exist")
	}
}

func TestChargebackManager_ListInvoices(t *testing.T) {
	tracker := NewTracker("USD")
	manager := NewChargebackManager(tracker)
	now := time.Now()

	// Record usage and generate invoices
	for i := 0; i < 3; i++ {
		tracker.RecordUsage(UsageRecord{
			TenantID:  "tenant-1",
			Category:  CostCategoryAPI,
			Unit:      CostUnitRequests,
			Quantity:  1000,
			Timestamp: now.Add(time.Duration(i) * time.Hour),
		})
	}

	manager.GenerateInvoice("tenant-1", now.Add(-2*time.Hour), now)

	tracker.RecordUsage(UsageRecord{
		TenantID:  "tenant-2",
		Category:  CostCategoryAPI,
		Unit:      CostUnitRequests,
		Quantity:  1000,
		Timestamp: now,
	})
	manager.GenerateInvoice("tenant-2", now.Add(-time.Hour), now.Add(time.Hour))

	// List all invoices
	all := manager.ListInvoices("")
	if len(all) != 2 {
		t.Errorf("Expected 2 invoices, got %d", len(all))
	}

	// List tenant-1 invoices
	tenant1 := manager.ListInvoices("tenant-1")
	if len(tenant1) != 1 {
		t.Errorf("Expected 1 invoice for tenant-1, got %d", len(tenant1))
	}
}

func TestChargebackManager_UpdateInvoiceStatus(t *testing.T) {
	tracker := NewTracker("USD")
	manager := NewChargebackManager(tracker)
	now := time.Now()

	tracker.RecordUsage(UsageRecord{
		TenantID:  "tenant-1",
		Category:  CostCategoryAPI,
		Unit:      CostUnitRequests,
		Quantity:  1000,
		Timestamp: now,
	})

	invoice, _ := manager.GenerateInvoice("tenant-1", now.Add(-time.Hour), now.Add(time.Hour))

	err := manager.UpdateInvoiceStatus(invoice.ID, InvoiceStatusPending)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	retrieved, _ := manager.GetInvoice(invoice.ID)
	if retrieved.Status != InvoiceStatusPending {
		t.Errorf("Expected pending status, got %s", retrieved.Status)
	}
}

func TestChargebackManager_UpdateInvoiceStatus_NotFound(t *testing.T) {
	tracker := NewTracker("USD")
	manager := NewChargebackManager(tracker)

	err := manager.UpdateInvoiceStatus("nonexistent", InvoiceStatusPaid)
	if err == nil {
		t.Error("Expected error for nonexistent invoice")
	}
}

func TestChargebackManager_ApplyCredit(t *testing.T) {
	tracker := NewTracker("USD")
	manager := NewChargebackManager(tracker)
	now := time.Now()

	tracker.RecordUsage(UsageRecord{
		TenantID:  "tenant-1",
		Category:  CostCategoryAPI,
		Unit:      CostUnitRequests,
		Quantity:  100000,
		Timestamp: now,
	})

	invoice, _ := manager.GenerateInvoice("tenant-1", now.Add(-time.Hour), now.Add(time.Hour))
	originalTotal := invoice.Total

	credit := CreditEntry{
		Description: "Promotional credit",
		Amount:      1.00,
		Reason:      "Welcome bonus",
	}

	err := manager.ApplyCredit(invoice.ID, credit)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	retrieved, _ := manager.GetInvoice(invoice.ID)
	if len(retrieved.Credits) != 1 {
		t.Errorf("Expected 1 credit, got %d", len(retrieved.Credits))
	}
	if retrieved.Total != originalTotal-1.00 {
		t.Errorf("Expected total %f, got %f", originalTotal-1.00, retrieved.Total)
	}
}

func TestChargebackManager_ApplyCredit_FullCredit(t *testing.T) {
	tracker := NewTracker("USD")
	manager := NewChargebackManager(tracker)
	now := time.Now()

	tracker.RecordUsage(UsageRecord{
		TenantID:  "tenant-1",
		Category:  CostCategoryAPI,
		Unit:      CostUnitRequests,
		Quantity:  100,
		Timestamp: now,
	})

	invoice, _ := manager.GenerateInvoice("tenant-1", now.Add(-time.Hour), now.Add(time.Hour))

	// Apply credit larger than total
	credit := CreditEntry{
		Description: "Full refund",
		Amount:      1000.00,
	}

	manager.ApplyCredit(invoice.ID, credit)

	retrieved, _ := manager.GetInvoice(invoice.ID)
	if retrieved.Total != 0 {
		t.Errorf("Expected total 0, got %f", retrieved.Total)
	}
}

func TestChargebackManager_ApplyCredit_NotFound(t *testing.T) {
	tracker := NewTracker("USD")
	manager := NewChargebackManager(tracker)

	err := manager.ApplyCredit("nonexistent", CreditEntry{Amount: 1.00})
	if err == nil {
		t.Error("Expected error for nonexistent invoice")
	}
}

func TestChargebackManager_GenerateReport(t *testing.T) {
	tracker := NewTracker("USD")
	manager := NewChargebackManager(tracker)
	now := time.Now()

	// Create rules
	manager.CreateRule(&CostAllocationRule{
		TenantID:   "tenant-1",
		Name:       "Engineering",
		CostCenter: "engineering",
		Percentage: 100,
	})

	// Record varied usage
	tracker.RecordUsage(UsageRecord{
		TenantID:     "tenant-1",
		FeatureGroup: "recs",
		Category:     CostCategoryAPI,
		Unit:         CostUnitRequests,
		Quantity:     10000,
		Timestamp:    now,
	})
	tracker.RecordUsage(UsageRecord{
		TenantID:     "tenant-1",
		FeatureGroup: "search",
		Category:     CostCategoryML,
		Unit:         CostUnitTokens,
		Quantity:     50000,
		Timestamp:    now,
	})

	config := ReportConfig{
		TenantID:      "tenant-1",
		Granularity:   "daily",
		IncludeTrends: true,
	}

	report, err := manager.GenerateReport(config, now.Add(-time.Hour), now.Add(time.Hour))
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if report.ID == "" {
		t.Error("Expected report to have ID")
	}
	if report.TotalCost <= 0 {
		t.Error("Expected positive total cost")
	}
	if len(report.ByCategory) != 2 {
		t.Errorf("Expected 2 categories, got %d", len(report.ByCategory))
	}
	if len(report.ByFeature) != 2 {
		t.Errorf("Expected 2 features, got %d", len(report.ByFeature))
	}
	if len(report.ByCostCenter) == 0 {
		t.Error("Expected cost center breakdown")
	}
}

func TestChargebackManager_GenerateReport_WithFilters(t *testing.T) {
	tracker := NewTracker("USD")
	manager := NewChargebackManager(tracker)
	now := time.Now()

	// Record usage
	tracker.RecordUsage(UsageRecord{
		TenantID:     "tenant-1",
		FeatureGroup: "recs",
		Category:     CostCategoryAPI,
		Unit:         CostUnitRequests,
		Quantity:     10000,
		Timestamp:    now,
	})
	tracker.RecordUsage(UsageRecord{
		TenantID:     "tenant-1",
		FeatureGroup: "search",
		Category:     CostCategoryML,
		Unit:         CostUnitTokens,
		Quantity:     50000,
		Timestamp:    now,
	})

	// Filter by categories
	config := ReportConfig{
		TenantID:   "tenant-1",
		Categories: []CostCategory{CostCategoryAPI},
	}

	report, _ := manager.GenerateReport(config, now.Add(-time.Hour), now.Add(time.Hour))

	if len(report.ByCategory) != 1 {
		t.Errorf("Expected 1 category with filter, got %d", len(report.ByCategory))
	}
	if _, ok := report.ByCategory[CostCategoryAPI]; !ok {
		t.Error("Expected API category in filtered report")
	}
}

func TestChargebackManager_GenerateReport_FeatureGroupFilter(t *testing.T) {
	tracker := NewTracker("USD")
	manager := NewChargebackManager(tracker)
	now := time.Now()

	// Record usage for different feature groups
	tracker.RecordUsage(UsageRecord{
		TenantID:     "tenant-1",
		FeatureGroup: "recs",
		Category:     CostCategoryAPI,
		Unit:         CostUnitRequests,
		Quantity:     10000,
		Timestamp:    now,
	})
	tracker.RecordUsage(UsageRecord{
		TenantID:     "tenant-1",
		FeatureGroup: "search",
		Category:     CostCategoryAPI,
		Unit:         CostUnitRequests,
		Quantity:     5000,
		Timestamp:    now,
	})
	tracker.RecordUsage(UsageRecord{
		TenantID:     "tenant-1",
		FeatureGroup: "other",
		Category:     CostCategoryAPI,
		Unit:         CostUnitRequests,
		Quantity:     3000,
		Timestamp:    now,
	})

	config := ReportConfig{
		TenantID:      "tenant-1",
		FeatureGroups: []string{"recs", "search"},
	}

	report, _ := manager.GenerateReport(config, now.Add(-time.Hour), now.Add(time.Hour))

	if len(report.ByFeature) != 2 {
		t.Errorf("Expected 2 feature groups with filter, got %d", len(report.ByFeature))
	}
}
