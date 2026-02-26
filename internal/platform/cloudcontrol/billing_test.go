package cloudcontrol

import (
	"sync"
	"testing"
	"time"
)

func TestNewBillingManager_DefaultPlans(t *testing.T) {
	t.Parallel()
	bm := NewBillingManager()

	tiers := []InstanceTier{TierFree, TierStarter, TierPro, TierEnterprise}
	for _, tier := range tiers {
		plan, ok := bm.GetPlan(tier)
		if !ok {
			t.Errorf("expected plan for tier %s", tier)
			continue
		}
		if plan.Tier != tier {
			t.Errorf("expected tier %s, got %s", tier, plan.Tier)
		}
	}
}

func TestNewBillingManager_FreeTierZeroPricing(t *testing.T) {
	t.Parallel()
	bm := NewBillingManager()

	plan, ok := bm.GetPlan(TierFree)
	if !ok {
		t.Fatal("free plan not found")
	}
	if plan.BasePriceUSD != 0 || plan.PricePerVCPUH != 0 || plan.PricePerGBH != 0 || plan.PricePerMReq != 0 {
		t.Error("free tier should have zero pricing")
	}
	if plan.FreeRequests != 1_000_000 {
		t.Errorf("expected 1M free requests, got %d", plan.FreeRequests)
	}
}

func TestRecordUsage_Single(t *testing.T) {
	t.Parallel()
	bm := NewBillingManager()

	record := UsageRecord{
		InstanceID:   "inst-1",
		TenantID:     "tenant-1",
		VCPUHours:    2.5,
		MemoryGBH:    4.0,
		RequestCount: 5000,
		StorageGB:    10.0,
	}
	bm.RecordUsage(record)

	usage := bm.GetUsage("tenant-1", time.Time{}, time.Now().Add(time.Second))
	if len(usage) != 1 {
		t.Fatalf("expected 1 usage record, got %d", len(usage))
	}
	if usage[0].VCPUHours != 2.5 {
		t.Errorf("expected 2.5 vCPU hours, got %f", usage[0].VCPUHours)
	}
}

func TestRecordUsage_ZeroTimestamp(t *testing.T) {
	t.Parallel()
	bm := NewBillingManager()

	record := UsageRecord{TenantID: "t1", VCPUHours: 1.0}
	bm.RecordUsage(record)

	usage := bm.GetUsage("t1", time.Time{}, time.Now().Add(time.Second))
	if len(usage) != 1 {
		t.Fatal("expected 1 record")
	}
	if usage[0].Timestamp.IsZero() {
		t.Error("expected non-zero timestamp to be auto-set")
	}
}

func TestRecordUsage_Multiple(t *testing.T) {
	t.Parallel()
	bm := NewBillingManager()

	for i := 0; i < 5; i++ {
		bm.RecordUsage(UsageRecord{
			TenantID:  "t1",
			VCPUHours: 1.0,
		})
	}

	usage := bm.GetUsage("t1", time.Time{}, time.Now().Add(time.Second))
	if len(usage) != 5 {
		t.Errorf("expected 5 records, got %d", len(usage))
	}
}

func TestRecordUsage_Concurrent(t *testing.T) {
	t.Parallel()
	bm := NewBillingManager()

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			bm.RecordUsage(UsageRecord{TenantID: "t1", VCPUHours: 1.0})
		}()
	}
	wg.Wait()

	usage := bm.GetUsage("t1", time.Time{}, time.Now().Add(time.Second))
	if len(usage) != 50 {
		t.Errorf("expected 50 records, got %d", len(usage))
	}
}

func TestGetUsage_TimeRangeFiltering(t *testing.T) {
	t.Parallel()
	bm := NewBillingManager()

	now := time.Now()
	bm.RecordUsage(UsageRecord{TenantID: "t1", Timestamp: now.Add(-2 * time.Hour), VCPUHours: 1.0})
	bm.RecordUsage(UsageRecord{TenantID: "t1", Timestamp: now.Add(-1 * time.Hour), VCPUHours: 2.0})
	bm.RecordUsage(UsageRecord{TenantID: "t1", Timestamp: now, VCPUHours: 3.0})

	// Only the middle record
	usage := bm.GetUsage("t1", now.Add(-90*time.Minute), now.Add(-30*time.Minute))
	if len(usage) != 1 {
		t.Fatalf("expected 1 record in range, got %d", len(usage))
	}
	if usage[0].VCPUHours != 2.0 {
		t.Errorf("expected 2.0 vCPU hours, got %f", usage[0].VCPUHours)
	}
}

func TestGetUsage_EmptyRange(t *testing.T) {
	t.Parallel()
	bm := NewBillingManager()

	bm.RecordUsage(UsageRecord{TenantID: "t1", VCPUHours: 1.0})

	usage := bm.GetUsage("t1", time.Now().Add(time.Hour), time.Now().Add(2*time.Hour))
	if len(usage) != 0 {
		t.Errorf("expected 0 records for future range, got %d", len(usage))
	}
}

func TestGetUsage_WrongTenant(t *testing.T) {
	t.Parallel()
	bm := NewBillingManager()

	bm.RecordUsage(UsageRecord{TenantID: "t1", VCPUHours: 1.0})

	usage := bm.GetUsage("t-other", time.Time{}, time.Now().Add(time.Second))
	if len(usage) != 0 {
		t.Errorf("expected 0 records for wrong tenant, got %d", len(usage))
	}
}

func TestGenerateInvoice_StarterTier(t *testing.T) {
	t.Parallel()
	bm := NewBillingManager()
	now := time.Now()
	start := now.Add(-24 * time.Hour)

	bm.RecordUsage(UsageRecord{
		TenantID:     "t1",
		Timestamp:    now.Add(-12 * time.Hour),
		VCPUHours:    100,
		MemoryGBH:    200,
		RequestCount: 20_000_000, // 10M over free tier
		StorageGB:    5.0,
	})

	inv, err := bm.GenerateInvoice("t1", TierStarter, start, now)
	if err != nil {
		t.Fatal(err)
	}
	if inv.TenantID != "t1" {
		t.Errorf("expected tenant t1, got %s", inv.TenantID)
	}
	if inv.Status != InvoiceDraft {
		t.Errorf("expected draft status, got %s", inv.Status)
	}

	// Check line items: base + compute + memory + requests
	hasBase, hasCompute, hasMemory, hasRequests := false, false, false, false
	for _, item := range inv.LineItems {
		switch {
		case item.Description == "starter plan base fee":
			hasBase = true
			if item.AmountUSD != 29.0 {
				t.Errorf("expected base 29, got %f", item.AmountUSD)
			}
		case item.Description == "Compute (vCPU-hours)":
			hasCompute = true
			if item.AmountUSD != 100*0.05 {
				t.Errorf("expected compute %f, got %f", 100*0.05, item.AmountUSD)
			}
		case item.Description == "Memory (GB-hours)":
			hasMemory = true
			if item.AmountUSD != 200*0.008 {
				t.Errorf("expected memory %f, got %f", 200*0.008, item.AmountUSD)
			}
		case item.Description == "API requests (millions, over free tier)":
			hasRequests = true
		}
	}
	if !hasBase || !hasCompute || !hasMemory || !hasRequests {
		t.Errorf("missing line items: base=%v compute=%v memory=%v requests=%v", hasBase, hasCompute, hasMemory, hasRequests)
	}

	if inv.TotalUSD <= 0 {
		t.Error("expected positive total")
	}
}

func TestGenerateInvoice_FreeTier(t *testing.T) {
	t.Parallel()
	bm := NewBillingManager()
	now := time.Now()

	bm.RecordUsage(UsageRecord{
		TenantID:     "t1",
		Timestamp:    now.Add(-time.Hour),
		RequestCount: 500_000,
	})

	inv, err := bm.GenerateInvoice("t1", TierFree, now.Add(-24*time.Hour), now)
	if err != nil {
		t.Fatal(err)
	}
	if inv.TotalUSD != 0 {
		t.Errorf("expected 0 total for free tier, got %f", inv.TotalUSD)
	}
	if len(inv.LineItems) != 0 {
		t.Errorf("expected 0 line items for free tier, got %d", len(inv.LineItems))
	}
}

func TestGenerateInvoice_ProTier(t *testing.T) {
	t.Parallel()
	bm := NewBillingManager()
	now := time.Now()

	bm.RecordUsage(UsageRecord{
		TenantID:     "t1",
		Timestamp:    now.Add(-time.Hour),
		VCPUHours:    50,
		MemoryGBH:    100,
		RequestCount: 200_000_000, // 100M over free tier
	})

	inv, err := bm.GenerateInvoice("t1", TierPro, now.Add(-24*time.Hour), now)
	if err != nil {
		t.Fatal(err)
	}
	if inv.TotalUSD <= 0 {
		t.Error("expected positive total for pro tier")
	}
}

func TestGenerateInvoice_EnterpriseTier(t *testing.T) {
	t.Parallel()
	bm := NewBillingManager()
	now := time.Now()

	bm.RecordUsage(UsageRecord{
		TenantID:  "t1",
		Timestamp: now.Add(-time.Hour),
		VCPUHours: 1000,
	})

	inv, err := bm.GenerateInvoice("t1", TierEnterprise, now.Add(-24*time.Hour), now)
	if err != nil {
		t.Fatal(err)
	}

	// Should have base fee
	if len(inv.LineItems) < 1 {
		t.Error("expected at least base fee line item")
	}
}

func TestGenerateInvoice_UnknownTier(t *testing.T) {
	t.Parallel()
	bm := NewBillingManager()

	_, err := bm.GenerateInvoice("t1", "unknown", time.Now(), time.Now())
	if err == nil {
		t.Fatal("expected error for unknown tier")
	}
}

func TestGenerateInvoice_NoUsage(t *testing.T) {
	t.Parallel()
	bm := NewBillingManager()
	now := time.Now()

	inv, err := bm.GenerateInvoice("t1", TierStarter, now.Add(-24*time.Hour), now)
	if err != nil {
		t.Fatal(err)
	}
	// Should still have base fee
	if len(inv.LineItems) != 1 {
		t.Errorf("expected 1 line item (base fee), got %d", len(inv.LineItems))
	}
}

func TestGetInvoices_Empty(t *testing.T) {
	t.Parallel()
	bm := NewBillingManager()

	invoices := bm.GetInvoices("t1")
	if len(invoices) != 0 {
		t.Errorf("expected 0 invoices, got %d", len(invoices))
	}
}

func TestGetInvoices_Multiple(t *testing.T) {
	t.Parallel()
	bm := NewBillingManager()
	now := time.Now()

	bm.GenerateInvoice("t1", TierStarter, now.Add(-48*time.Hour), now.Add(-24*time.Hour))
	time.Sleep(time.Millisecond)
	bm.GenerateInvoice("t1", TierStarter, now.Add(-24*time.Hour), now)

	invoices := bm.GetInvoices("t1")
	if len(invoices) != 2 {
		t.Errorf("expected 2 invoices, got %d", len(invoices))
	}
}

func TestGetInvoices_IsolatedByTenant(t *testing.T) {
	t.Parallel()
	bm := NewBillingManager()
	now := time.Now()

	bm.GenerateInvoice("t1", TierStarter, now.Add(-24*time.Hour), now)
	bm.GenerateInvoice("t2", TierPro, now.Add(-24*time.Hour), now)

	if len(bm.GetInvoices("t1")) != 1 {
		t.Error("expected 1 invoice for t1")
	}
	if len(bm.GetInvoices("t2")) != 1 {
		t.Error("expected 1 invoice for t2")
	}
}

func TestListPlans(t *testing.T) {
	t.Parallel()
	bm := NewBillingManager()

	plans := bm.ListPlans()
	if len(plans) != 4 {
		t.Errorf("expected 4 plans, got %d", len(plans))
	}
}

func TestGetPlan_AllTiers(t *testing.T) {
	t.Parallel()
	bm := NewBillingManager()

	tiers := []InstanceTier{TierFree, TierStarter, TierPro, TierEnterprise}
	for _, tier := range tiers {
		plan, ok := bm.GetPlan(tier)
		if !ok {
			t.Errorf("expected plan for tier %s", tier)
			continue
		}
		if plan.Tier != tier {
			t.Errorf("expected tier %s, got %s", tier, plan.Tier)
		}
	}
}

func TestGetPlan_NonExistent(t *testing.T) {
	t.Parallel()
	bm := NewBillingManager()

	_, ok := bm.GetPlan("nonexistent")
	if ok {
		t.Error("expected false for nonexistent tier")
	}
}
