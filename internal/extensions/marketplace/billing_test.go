package marketplace

import (
	"math"
	"sync"
	"testing"
	"time"
)

func newTestEngine() *BillingEngine {
	return NewBillingEngine(DefaultBillingConfig())
}

func approxEqual(a, b float64) bool {
	return math.Abs(a-b) < 1e-9
}

func createTestPlan(id, name string, model PricingModel, price float64) *BillingPlan {
	return &BillingPlan{
		ID:           id,
		Name:         name,
		PricingModel: model,
		PricePerUnit: price,
		Features:     []string{"feature1"},
	}
}

// --- CreatePlan tests ---

func TestCreatePlan_Valid(t *testing.T) {
	t.Parallel()
	engine := newTestEngine()

	plan := createTestPlan("plan-1", "Basic Plan", PricingPerRequest, 0.001)
	if err := engine.CreatePlan(plan); err != nil {
		t.Fatalf("CreatePlan() error = %v", err)
	}
	if plan.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be set")
	}
	if plan.Currency != "USD" {
		t.Errorf("expected default currency USD, got %s", plan.Currency)
	}
}

func TestCreatePlan_DuplicateID(t *testing.T) {
	t.Parallel()
	engine := newTestEngine()

	plan := createTestPlan("dup", "Plan A", PricingFree, 0)
	engine.CreatePlan(plan)

	err := engine.CreatePlan(createTestPlan("dup", "Plan B", PricingFree, 0))
	if err == nil {
		t.Fatal("expected error for duplicate plan ID")
	}
}

func TestCreatePlan_EmptyID(t *testing.T) {
	t.Parallel()
	engine := newTestEngine()

	err := engine.CreatePlan(&BillingPlan{Name: "No ID"})
	if err == nil {
		t.Fatal("expected error for empty plan ID")
	}
}

func TestCreatePlan_EmptyName(t *testing.T) {
	t.Parallel()
	engine := newTestEngine()

	err := engine.CreatePlan(&BillingPlan{ID: "p1"})
	if err == nil {
		t.Fatal("expected error for empty plan name")
	}
}

func TestCreatePlan_AllPricingModels(t *testing.T) {
	t.Parallel()
	engine := newTestEngine()

	models := []PricingModel{PricingFree, PricingPerRequest, PricingPerGB, PricingFlatMonthly}
	for i, m := range models {
		plan := createTestPlan("model-"+string(m), "Plan "+string(m), m, float64(i)*0.01)
		if err := engine.CreatePlan(plan); err != nil {
			t.Errorf("CreatePlan(%s) error = %v", m, err)
		}
	}
	if len(engine.ListPlans()) != 4 {
		t.Errorf("expected 4 plans, got %d", len(engine.ListPlans()))
	}
}

func TestCreatePlan_CustomCurrencyPreserved(t *testing.T) {
	t.Parallel()
	engine := newTestEngine()

	plan := createTestPlan("eur", "EUR Plan", PricingFree, 0)
	plan.Currency = "EUR"
	engine.CreatePlan(plan)

	got, _ := engine.GetPlan("eur")
	if got.Currency != "EUR" {
		t.Errorf("expected currency EUR, got %s", got.Currency)
	}
}

// --- GetPlan / ListPlans tests ---

func TestGetPlan_NotFound(t *testing.T) {
	t.Parallel()
	engine := newTestEngine()

	_, err := engine.GetPlan("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent plan")
	}
}

func TestListPlans_Empty(t *testing.T) {
	t.Parallel()
	engine := newTestEngine()

	plans := engine.ListPlans()
	if len(plans) != 0 {
		t.Errorf("expected 0 plans, got %d", len(plans))
	}
}

// --- RecordUsage tests ---

func TestRecordUsage_AtomicIncrements(t *testing.T) {
	t.Parallel()
	engine := newTestEngine()

	engine.RecordUsage("feat-1", "sub-1", 10, 1024)
	engine.RecordUsage("feat-1", "sub-1", 5, 512)

	// Generate invoice to check accumulated usage
	engine.CreatePlan(createTestPlan("p", "P", PricingPerRequest, 0.01))
	inv, err := engine.GenerateInvoice("sub-1", "feat-1", "p", time.Now().Add(-time.Hour), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if inv.RequestCount != 15 {
		t.Errorf("expected 15 requests, got %d", inv.RequestCount)
	}
}

func TestRecordUsage_Concurrent(t *testing.T) {
	t.Parallel()
	engine := newTestEngine()

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			engine.RecordUsage("feat-c", "sub-c", 1, 100)
		}()
	}
	wg.Wait()

	engine.CreatePlan(createTestPlan("pc", "PC", PricingPerRequest, 0.01))
	inv, _ := engine.GenerateInvoice("sub-c", "feat-c", "pc", time.Now().Add(-time.Hour), time.Now())
	if inv.RequestCount != 100 {
		t.Errorf("expected 100 requests after concurrent usage, got %d", inv.RequestCount)
	}
}

// --- GenerateInvoice tests ---

func TestGenerateInvoice_FreeModel(t *testing.T) {
	t.Parallel()
	engine := newTestEngine()

	engine.CreatePlan(createTestPlan("free", "Free", PricingFree, 0))
	engine.RecordUsage("f1", "s1", 100, 1024*1024*1024)

	inv, err := engine.GenerateInvoice("s1", "f1", "free", time.Now().Add(-time.Hour), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if inv.Amount != 0 {
		t.Errorf("expected amount 0 for free plan, got %f", inv.Amount)
	}
}

func TestGenerateInvoice_PerRequestModel(t *testing.T) {
	t.Parallel()
	engine := newTestEngine()

	engine.CreatePlan(createTestPlan("req", "Per Request", PricingPerRequest, 0.01))
	engine.RecordUsage("f1", "s1", 100, 0)

	inv, err := engine.GenerateInvoice("s1", "f1", "req", time.Now().Add(-time.Hour), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	expected := 100 * 0.01
	if inv.Amount != expected {
		t.Errorf("expected amount %f, got %f", expected, inv.Amount)
	}
}

func TestGenerateInvoice_PerGBModel(t *testing.T) {
	t.Parallel()
	engine := newTestEngine()

	engine.CreatePlan(createTestPlan("gb", "Per GB", PricingPerGB, 0.10))
	// 2 GB
	engine.RecordUsage("f1", "s1", 0, 2*1024*1024*1024)

	inv, err := engine.GenerateInvoice("s1", "f1", "gb", time.Now().Add(-time.Hour), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	expected := 2.0 * 0.10
	if inv.Amount != expected {
		t.Errorf("expected amount %f, got %f", expected, inv.Amount)
	}
}

func TestGenerateInvoice_FlatMonthlyModel(t *testing.T) {
	t.Parallel()
	engine := newTestEngine()

	engine.CreatePlan(createTestPlan("flat", "Flat", PricingFlatMonthly, 99.99))

	inv, err := engine.GenerateInvoice("s1", "f1", "flat", time.Now().Add(-time.Hour), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if inv.Amount != 99.99 {
		t.Errorf("expected amount 99.99, got %f", inv.Amount)
	}
}

func TestGenerateInvoice_MissingPlan(t *testing.T) {
	t.Parallel()
	engine := newTestEngine()

	_, err := engine.GenerateInvoice("s1", "f1", "nonexistent", time.Now(), time.Now())
	if err == nil {
		t.Fatal("expected error for missing plan")
	}
}

func TestGenerateInvoice_ZeroUsage(t *testing.T) {
	t.Parallel()
	engine := newTestEngine()

	engine.CreatePlan(createTestPlan("req", "Per Request", PricingPerRequest, 0.01))

	inv, err := engine.GenerateInvoice("s1", "f1", "req", time.Now().Add(-time.Hour), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if inv.Amount != 0 {
		t.Errorf("expected 0 for zero usage, got %f", inv.Amount)
	}
}

func TestGenerateInvoice_ResetsUsage(t *testing.T) {
	t.Parallel()
	engine := newTestEngine()

	engine.CreatePlan(createTestPlan("req", "Per Request", PricingPerRequest, 0.01))
	engine.RecordUsage("f1", "s1", 50, 0)

	engine.GenerateInvoice("s1", "f1", "req", time.Now().Add(-time.Hour), time.Now())

	// Second invoice after reset
	inv2, _ := engine.GenerateInvoice("s1", "f1", "req", time.Now().Add(-time.Hour), time.Now())
	if inv2.RequestCount != 0 {
		t.Errorf("expected 0 requests after reset, got %d", inv2.RequestCount)
	}
}

// --- ProcessPayment tests ---

func TestProcessPayment_Valid(t *testing.T) {
	t.Parallel()
	engine := newTestEngine()

	engine.CreatePlan(createTestPlan("p1", "Plan", PricingFlatMonthly, 10))
	inv, _ := engine.GenerateInvoice("s1", "f1", "p1", time.Now(), time.Now())

	if err := engine.ProcessPayment(inv.ID); err != nil {
		t.Fatalf("ProcessPayment() error = %v", err)
	}

	got, _ := engine.GetInvoice(inv.ID)
	if got.Status != "paid" {
		t.Errorf("expected status paid, got %s", got.Status)
	}
}

func TestProcessPayment_AlreadyPaid(t *testing.T) {
	t.Parallel()
	engine := newTestEngine()

	engine.CreatePlan(createTestPlan("p1", "Plan", PricingFlatMonthly, 10))
	inv, _ := engine.GenerateInvoice("s1", "f1", "p1", time.Now(), time.Now())
	engine.ProcessPayment(inv.ID)

	err := engine.ProcessPayment(inv.ID)
	if err == nil {
		t.Fatal("expected error for already paid invoice")
	}
}

func TestProcessPayment_NonExistent(t *testing.T) {
	t.Parallel()
	engine := newTestEngine()

	err := engine.ProcessPayment("inv-none")
	if err == nil {
		t.Fatal("expected error for non-existent invoice")
	}
}

// --- DistributeRevenue tests ---

func TestDistributeRevenue_8020Split(t *testing.T) {
	t.Parallel()
	engine := newTestEngine()

	engine.CreatePlan(createTestPlan("p1", "Plan", PricingFlatMonthly, 100))
	inv, _ := engine.GenerateInvoice("s1", "f1", "p1", time.Now(), time.Now())
	engine.ProcessPayment(inv.ID)

	rs, err := engine.DistributeRevenue(inv.ID)
	if err != nil {
		t.Fatalf("DistributeRevenue() error = %v", err)
	}
	if !approxEqual(rs.OwnerShare, 80) {
		t.Errorf("expected owner share 80, got %f", rs.OwnerShare)
	}
	if !approxEqual(rs.PlatformFee, 20) {
		t.Errorf("expected platform fee 20, got %f", rs.PlatformFee)
	}
	if !approxEqual(rs.GrossAmount, 100) {
		t.Errorf("expected gross 100, got %f", rs.GrossAmount)
	}
	if rs.SplitRatio != 0.80 {
		t.Errorf("expected split ratio 0.80, got %f", rs.SplitRatio)
	}
	if rs.Status != "distributed" {
		t.Errorf("expected status distributed, got %s", rs.Status)
	}
}

func TestDistributeRevenue_UnpaidInvoice(t *testing.T) {
	t.Parallel()
	engine := newTestEngine()

	engine.CreatePlan(createTestPlan("p1", "Plan", PricingFlatMonthly, 100))
	inv, _ := engine.GenerateInvoice("s1", "f1", "p1", time.Now(), time.Now())

	_, err := engine.DistributeRevenue(inv.ID)
	if err == nil {
		t.Fatal("expected error for unpaid invoice")
	}
}

func TestDistributeRevenue_NonExistent(t *testing.T) {
	t.Parallel()
	engine := newTestEngine()

	_, err := engine.DistributeRevenue("inv-none")
	if err == nil {
		t.Fatal("expected error for non-existent invoice")
	}
}

func TestDistributeRevenue_Rounding(t *testing.T) {
	t.Parallel()
	engine := newTestEngine()

	engine.CreatePlan(createTestPlan("p1", "Plan", PricingFlatMonthly, 33.33))
	inv, _ := engine.GenerateInvoice("s1", "f1", "p1", time.Now(), time.Now())
	engine.ProcessPayment(inv.ID)

	rs, _ := engine.DistributeRevenue(inv.ID)
	total := rs.OwnerShare + rs.PlatformFee
	if !approxEqual(total, rs.GrossAmount) {
		t.Errorf("owner + platform (%f) != gross (%f)", total, rs.GrossAmount)
	}
}

// --- GetRevenueShares tests ---

func TestGetRevenueShares_Empty(t *testing.T) {
	t.Parallel()
	engine := newTestEngine()

	shares := engine.GetRevenueShares("owner1")
	if len(shares) != 0 {
		t.Errorf("expected 0 shares, got %d", len(shares))
	}
}

func TestGetRevenueShares_MultipleInvoices(t *testing.T) {
	t.Parallel()
	engine := newTestEngine()

	engine.CreatePlan(createTestPlan("p1", "Plan", PricingFlatMonthly, 100))

	for i := 0; i < 3; i++ {
		inv, _ := engine.GenerateInvoice("s1", "feat1", "p1", time.Now(), time.Now())
		engine.ProcessPayment(inv.ID)
		engine.DistributeRevenue(inv.ID)
		time.Sleep(time.Millisecond)
	}

	// OwnerID is set to FeatureID in DistributeRevenue
	shares := engine.GetRevenueShares("feat1")
	if len(shares) != 3 {
		t.Errorf("expected 3 shares, got %d", len(shares))
	}
}

func TestGetRevenueShares_FilterByOwner(t *testing.T) {
	t.Parallel()
	engine := newTestEngine()

	engine.CreatePlan(createTestPlan("p1", "Plan", PricingFlatMonthly, 50))

	inv1, _ := engine.GenerateInvoice("s1", "feat_a", "p1", time.Now(), time.Now())
	engine.ProcessPayment(inv1.ID)
	engine.DistributeRevenue(inv1.ID)

	inv2, _ := engine.GenerateInvoice("s1", "feat_b", "p1", time.Now(), time.Now())
	engine.ProcessPayment(inv2.ID)
	engine.DistributeRevenue(inv2.ID)

	sharesA := engine.GetRevenueShares("feat_a")
	if len(sharesA) != 1 {
		t.Errorf("expected 1 share for feat_a, got %d", len(sharesA))
	}
}

// --- Stats tests ---

func TestStats_AggregationAccuracy(t *testing.T) {
	t.Parallel()
	engine := newTestEngine()

	engine.CreatePlan(createTestPlan("p1", "Plan", PricingFlatMonthly, 100))

	// Create 2 paid invoices
	for i := 0; i < 2; i++ {
		inv, _ := engine.GenerateInvoice("s1", "f1", "p1", time.Now(), time.Now())
		engine.ProcessPayment(inv.ID)
		engine.DistributeRevenue(inv.ID)
		time.Sleep(time.Millisecond) // Ensure unique invoice IDs
	}

	// Create 1 pending invoice
	engine.GenerateInvoice("s1", "f1", "p1", time.Now(), time.Now())

	stats := engine.Stats()
	if stats.InvoiceCount != 3 {
		t.Errorf("expected 3 invoices, got %d", stats.InvoiceCount)
	}
	if stats.PaidCount != 2 {
		t.Errorf("expected 2 paid, got %d", stats.PaidCount)
	}
	if !approxEqual(stats.TotalRevenue, 200) {
		t.Errorf("expected total revenue 200, got %f", stats.TotalRevenue)
	}
	if !approxEqual(stats.OwnerPayouts, 160) {
		t.Errorf("expected owner payouts 160, got %f", stats.OwnerPayouts)
	}
	if !approxEqual(stats.PlatformRevenue, 40) {
		t.Errorf("expected platform revenue 40, got %f", stats.PlatformRevenue)
	}
}

func TestStats_Empty(t *testing.T) {
	t.Parallel()
	engine := newTestEngine()

	stats := engine.Stats()
	if stats.InvoiceCount != 0 || stats.TotalRevenue != 0 {
		t.Error("expected zero stats for empty engine")
	}
}

// --- ListInvoices tests ---

func TestListInvoices_BySubscriber(t *testing.T) {
	t.Parallel()
	engine := newTestEngine()

	engine.CreatePlan(createTestPlan("p1", "Plan", PricingFlatMonthly, 10))
	engine.GenerateInvoice("sub-a", "f1", "p1", time.Now(), time.Now())
	engine.GenerateInvoice("sub-b", "f1", "p1", time.Now(), time.Now())

	invoices := engine.ListInvoices("sub-a")
	if len(invoices) != 1 {
		t.Errorf("expected 1 invoice for sub-a, got %d", len(invoices))
	}
}
