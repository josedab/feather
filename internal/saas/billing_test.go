package saas

import (
	"errors"
	"testing"
	"time"
)

func setupBillingManager() *BillingManager {
	registry := NewPlanRegistry()
	return NewBillingManager(registry)
}

func TestNewBillingManager(t *testing.T) {
	manager := setupBillingManager()
	if manager == nil {
		t.Fatal("Expected manager to be non-nil")
	}
}

func TestBillingManager_CreateSubscription(t *testing.T) {
	manager := setupBillingManager()

	sub, err := manager.CreateSubscription("org_1", "starter", BillingMonthly)
	if err != nil {
		t.Fatalf("CreateSubscription failed: %v", err)
	}

	if sub.OrganizationID != "org_1" {
		t.Errorf("Expected org_1, got %s", sub.OrganizationID)
	}
	if sub.PlanID != "starter" {
		t.Errorf("Expected starter plan, got %s", sub.PlanID)
	}
	if sub.Status != SubscriptionActive {
		t.Errorf("Expected active status, got %s", sub.Status)
	}
	if sub.BillingPeriod != BillingMonthly {
		t.Errorf("Expected monthly billing, got %s", sub.BillingPeriod)
	}
}

func TestBillingManager_CreateSubscription_Yearly(t *testing.T) {
	manager := setupBillingManager()

	sub, err := manager.CreateSubscription("org_1", "pro", BillingYearly)
	if err != nil {
		t.Fatalf("CreateSubscription failed: %v", err)
	}

	if sub.BillingPeriod != BillingYearly {
		t.Errorf("Expected yearly billing, got %s", sub.BillingPeriod)
	}

	// Yearly subscription should have period end ~1 year from now
	expectedEnd := sub.CurrentPeriodStart.AddDate(1, 0, 0)
	if sub.CurrentPeriodEnd.Sub(expectedEnd).Hours() > 24 {
		t.Error("Expected period end to be ~1 year from start")
	}
}

func TestBillingManager_CreateSubscription_InvalidPlan(t *testing.T) {
	manager := setupBillingManager()

	_, err := manager.CreateSubscription("org_1", "nonexistent", BillingMonthly)
	if err == nil {
		t.Error("Expected error for invalid plan")
	}
}

func TestBillingManager_GetSubscription(t *testing.T) {
	manager := setupBillingManager()

	sub, _ := manager.CreateSubscription("org_1", "starter", BillingMonthly)

	retrieved, err := manager.GetSubscription(sub.ID)
	if err != nil {
		t.Fatalf("GetSubscription failed: %v", err)
	}
	if retrieved.ID != sub.ID {
		t.Errorf("Expected ID %s, got %s", sub.ID, retrieved.ID)
	}
}

func TestBillingManager_GetSubscription_NotFound(t *testing.T) {
	manager := setupBillingManager()

	_, err := manager.GetSubscription("nonexistent")
	if !errors.Is(err, ErrSubscriptionNotFound) {
		t.Errorf("Expected ErrSubscriptionNotFound, got %v", err)
	}
}

func TestBillingManager_GetSubscriptionByOrg(t *testing.T) {
	manager := setupBillingManager()

	manager.CreateSubscription("org_1", "starter", BillingMonthly)
	manager.CreateSubscription("org_1", "pro", BillingMonthly)
	manager.CreateSubscription("org_2", "free", BillingMonthly)

	org1Subs := manager.GetSubscriptionByOrg("org_1")
	if len(org1Subs) != 2 {
		t.Errorf("Expected 2 subscriptions for org_1, got %d", len(org1Subs))
	}

	org2Subs := manager.GetSubscriptionByOrg("org_2")
	if len(org2Subs) != 1 {
		t.Errorf("Expected 1 subscription for org_2, got %d", len(org2Subs))
	}
}

func TestBillingManager_CancelSubscription_Immediate(t *testing.T) {
	manager := setupBillingManager()

	sub, _ := manager.CreateSubscription("org_1", "starter", BillingMonthly)

	err := manager.CancelSubscription(sub.ID, true)
	if err != nil {
		t.Fatalf("CancelSubscription failed: %v", err)
	}

	updated, _ := manager.GetSubscription(sub.ID)
	if updated.Status != SubscriptionCanceled {
		t.Errorf("Expected canceled status, got %s", updated.Status)
	}
}

func TestBillingManager_CancelSubscription_AtPeriodEnd(t *testing.T) {
	manager := setupBillingManager()

	sub, _ := manager.CreateSubscription("org_1", "starter", BillingMonthly)

	err := manager.CancelSubscription(sub.ID, false)
	if err != nil {
		t.Fatalf("CancelSubscription failed: %v", err)
	}

	updated, _ := manager.GetSubscription(sub.ID)
	if updated.Status != SubscriptionActive {
		t.Errorf("Expected status to remain active, got %s", updated.Status)
	}
	if !updated.CancelAtPeriodEnd {
		t.Error("Expected CancelAtPeriodEnd to be true")
	}
}

func TestBillingManager_CancelSubscription_NotFound(t *testing.T) {
	manager := setupBillingManager()

	err := manager.CancelSubscription("nonexistent", true)
	if !errors.Is(err, ErrSubscriptionNotFound) {
		t.Errorf("Expected ErrSubscriptionNotFound, got %v", err)
	}
}

func TestBillingManager_ChangePlan(t *testing.T) {
	manager := setupBillingManager()

	sub, _ := manager.CreateSubscription("org_1", "starter", BillingMonthly)

	err := manager.ChangePlan(sub.ID, "pro")
	if err != nil {
		t.Fatalf("ChangePlan failed: %v", err)
	}

	updated, _ := manager.GetSubscription(sub.ID)
	if updated.PlanID != "pro" {
		t.Errorf("Expected plan 'pro', got '%s'", updated.PlanID)
	}
}

func TestBillingManager_ChangePlan_InvalidPlan(t *testing.T) {
	manager := setupBillingManager()

	sub, _ := manager.CreateSubscription("org_1", "starter", BillingMonthly)

	err := manager.ChangePlan(sub.ID, "nonexistent")
	if err == nil {
		t.Error("Expected error for invalid plan")
	}
}

func TestBillingManager_RecordUsage(t *testing.T) {
	manager := setupBillingManager()

	now := time.Now()
	record := UsageRecord{
		OrganizationID: "org_1",
		SubscriptionID: "sub_1",
		Metric:         MetricRequests,
		Quantity:       1000,
		Timestamp:      now,
	}

	err := manager.RecordUsage(record)
	if err != nil {
		t.Fatalf("RecordUsage failed: %v", err)
	}

	// Verify usage was recorded by checking the summary
	summary, _ := manager.GetUsageSummary("org_1", now.Add(-time.Hour), now.Add(time.Hour))
	if summary.Metrics[MetricRequests].Total != 1000 {
		t.Errorf("Expected 1000 total requests, got %f", summary.Metrics[MetricRequests].Total)
	}
}

func TestBillingManager_GetUsageSummary(t *testing.T) {
	manager := setupBillingManager()

	now := time.Now()
	start := now.Add(-24 * time.Hour)
	end := now

	// Record some usage
	manager.RecordUsage(UsageRecord{
		OrganizationID: "org_1",
		Metric:         MetricRequests,
		Quantity:       1000,
		Timestamp:      now.Add(-1 * time.Hour),
	})
	manager.RecordUsage(UsageRecord{
		OrganizationID: "org_1",
		Metric:         MetricRequests,
		Quantity:       2000,
		Timestamp:      now.Add(-2 * time.Hour),
	})
	manager.RecordUsage(UsageRecord{
		OrganizationID: "org_1",
		Metric:         MetricStorage,
		Quantity:       5,
		Timestamp:      now.Add(-1 * time.Hour),
	})

	summary, err := manager.GetUsageSummary("org_1", start, end)
	if err != nil {
		t.Fatalf("GetUsageSummary failed: %v", err)
	}

	requestMetric := summary.Metrics[MetricRequests]
	if requestMetric.Total != 3000 {
		t.Errorf("Expected 3000 total requests, got %f", requestMetric.Total)
	}
	if requestMetric.Peak != 2000 {
		t.Errorf("Expected peak 2000, got %f", requestMetric.Peak)
	}
	if requestMetric.Count != 2 {
		t.Errorf("Expected 2 records, got %d", requestMetric.Count)
	}
}

func TestBillingManager_GenerateInvoice(t *testing.T) {
	manager := setupBillingManager()

	sub, _ := manager.CreateSubscription("org_1", "starter", BillingMonthly)

	invoice, err := manager.GenerateInvoice(sub.ID)
	if err != nil {
		t.Fatalf("GenerateInvoice failed: %v", err)
	}

	if invoice.OrganizationID != "org_1" {
		t.Errorf("Expected org_1, got %s", invoice.OrganizationID)
	}
	if invoice.Status != InvoiceDraft {
		t.Errorf("Expected draft status, got %s", invoice.Status)
	}
	if invoice.Total <= 0 {
		t.Error("Expected positive total")
	}
	if len(invoice.LineItems) == 0 {
		t.Error("Expected at least one line item")
	}
}

func TestBillingManager_GenerateInvoice_NotFound(t *testing.T) {
	manager := setupBillingManager()

	_, err := manager.GenerateInvoice("nonexistent")
	if !errors.Is(err, ErrSubscriptionNotFound) {
		t.Errorf("Expected ErrSubscriptionNotFound, got %v", err)
	}
}

func TestBillingManager_GetInvoice(t *testing.T) {
	manager := setupBillingManager()

	sub, _ := manager.CreateSubscription("org_1", "starter", BillingMonthly)
	invoice, _ := manager.GenerateInvoice(sub.ID)

	retrieved, err := manager.GetInvoice(invoice.ID)
	if err != nil {
		t.Fatalf("GetInvoice failed: %v", err)
	}
	if retrieved.ID != invoice.ID {
		t.Errorf("Expected ID %s, got %s", invoice.ID, retrieved.ID)
	}
}

func TestBillingManager_GetInvoice_NotFound(t *testing.T) {
	manager := setupBillingManager()

	_, err := manager.GetInvoice("nonexistent")
	if !errors.Is(err, ErrInvoiceNotFound) {
		t.Errorf("Expected ErrInvoiceNotFound, got %v", err)
	}
}

func TestBillingManager_ListInvoices(t *testing.T) {
	manager := setupBillingManager()

	sub1, _ := manager.CreateSubscription("org_1", "starter", BillingMonthly)
	sub2, _ := manager.CreateSubscription("org_2", "pro", BillingMonthly)

	manager.GenerateInvoice(sub1.ID)
	manager.GenerateInvoice(sub1.ID)
	manager.GenerateInvoice(sub2.ID)

	org1Invoices := manager.ListInvoices("org_1")
	if len(org1Invoices) != 2 {
		t.Errorf("Expected 2 invoices for org_1, got %d", len(org1Invoices))
	}

	org2Invoices := manager.ListInvoices("org_2")
	if len(org2Invoices) != 1 {
		t.Errorf("Expected 1 invoice for org_2, got %d", len(org2Invoices))
	}
}

func TestBillingManager_FinalizeInvoice(t *testing.T) {
	manager := setupBillingManager()

	sub, _ := manager.CreateSubscription("org_1", "starter", BillingMonthly)
	invoice, _ := manager.GenerateInvoice(sub.ID)

	err := manager.FinalizeInvoice(invoice.ID)
	if err != nil {
		t.Fatalf("FinalizeInvoice failed: %v", err)
	}

	updated, _ := manager.GetInvoice(invoice.ID)
	if updated.Status != InvoiceOpen {
		t.Errorf("Expected open status, got %s", updated.Status)
	}
}

func TestBillingManager_MarkInvoicePaid(t *testing.T) {
	manager := setupBillingManager()

	sub, _ := manager.CreateSubscription("org_1", "starter", BillingMonthly)
	invoice, _ := manager.GenerateInvoice(sub.ID)
	manager.FinalizeInvoice(invoice.ID)

	err := manager.MarkInvoicePaid(invoice.ID)
	if err != nil {
		t.Fatalf("MarkInvoicePaid failed: %v", err)
	}

	updated, _ := manager.GetInvoice(invoice.ID)
	if updated.Status != InvoicePaid {
		t.Errorf("Expected paid status, got %s", updated.Status)
	}
	if updated.PaidAt == nil {
		t.Error("Expected PaidAt to be set")
	}
}

func TestBillingManager_AddPaymentMethod(t *testing.T) {
	manager := setupBillingManager()

	method := &PaymentMethod{
		Type:        "card",
		Last4:       "4242",
		ExpiryMonth: 12,
		ExpiryYear:  2025,
		Brand:       "visa",
	}

	err := manager.AddPaymentMethod("org_1", method)
	if err != nil {
		t.Fatalf("AddPaymentMethod failed: %v", err)
	}

	// First method should be default
	if !method.IsDefault {
		t.Error("Expected first method to be default")
	}

	methods := manager.GetPaymentMethods("org_1")
	if len(methods) != 1 {
		t.Errorf("Expected 1 method, got %d", len(methods))
	}
}

func TestBillingManager_AddPaymentMethod_Multiple(t *testing.T) {
	manager := setupBillingManager()

	method1 := &PaymentMethod{Type: "card", Last4: "4242"}
	method2 := &PaymentMethod{Type: "card", Last4: "5555", IsDefault: true}

	manager.AddPaymentMethod("org_1", method1)
	manager.AddPaymentMethod("org_1", method2)

	methods := manager.GetPaymentMethods("org_1")
	if len(methods) != 2 {
		t.Errorf("Expected 2 methods, got %d", len(methods))
	}

	// Second method marked as default should be default
	defaultCount := 0
	for _, m := range methods {
		if m.IsDefault {
			defaultCount++
		}
	}
	if defaultCount != 1 {
		t.Errorf("Expected exactly 1 default method, got %d", defaultCount)
	}
}

func TestUsageMetrics(t *testing.T) {
	metrics := []UsageMetric{
		MetricRequests,
		MetricStorage,
		MetricEntities,
		MetricVectorOps,
		MetricDataTransfer,
		MetricComputeHours,
	}

	for _, m := range metrics {
		if m == "" {
			t.Error("Expected non-empty metric type")
		}
	}
}

func TestInvoiceLineItem(t *testing.T) {
	item := InvoiceLineItem{
		Quantity:  10,
		UnitPrice: 5.0,
		Amount:    50.0,
	}

	_ = item.Description
	_ = item.Type

	if item.Amount != item.Quantity*item.UnitPrice {
		t.Error("Expected amount to equal quantity * unit price")
	}
}
