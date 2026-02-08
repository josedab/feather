package cost

import (
	"testing"
	"time"
)

func TestNewTracker(t *testing.T) {
	tracker := NewTracker("USD")
	if tracker == nil {
		t.Fatal("Expected non-nil tracker")
	}
	if tracker.currency != "USD" {
		t.Errorf("Expected currency USD, got %s", tracker.currency)
	}
	if tracker.maxRecords != 1000000 {
		t.Errorf("Expected maxRecords 1000000, got %d", tracker.maxRecords)
	}
}

func TestTracker_DefaultRates(t *testing.T) {
	tracker := NewTracker("USD")

	// Check that default rates are initialized
	rates := tracker.ListRates()
	if len(rates) == 0 {
		t.Error("Expected default rates to be initialized")
	}

	// Check specific default rates
	tests := []struct {
		category CostCategory
		unit     CostUnit
	}{
		{CostCategoryStorage, CostUnitBytes},
		{CostCategoryAPI, CostUnitRequests},
		{CostCategoryCompute, CostUnitCPUSeconds},
		{CostCategoryCompute, CostUnitGPUSeconds},
		{CostCategoryNetwork, CostUnitBytes},
		{CostCategoryML, CostUnitTokens},
		{CostCategoryML, CostUnitEmbeddings},
		{CostCategoryVector, CostUnitRequests},
	}

	for _, tt := range tests {
		rate, exists := tracker.GetRate(tt.category, tt.unit)
		if !exists {
			t.Errorf("Expected rate for %s/%s to exist", tt.category, tt.unit)
		}
		if rate.PricePerUnit <= 0 {
			t.Errorf("Expected positive price for %s/%s", tt.category, tt.unit)
		}
	}
}

func TestTracker_SetRate(t *testing.T) {
	tracker := NewTracker("USD")

	rate := &CostRate{
		Category:     CostCategoryStorage,
		Unit:         CostUnitBytes,
		PricePerUnit: 0.0001,
		Description:  "Custom storage rate",
	}

	tracker.SetRate(rate)

	retrieved, exists := tracker.GetRate(CostCategoryStorage, CostUnitBytes)
	if !exists {
		t.Fatal("Expected rate to exist")
	}
	if retrieved.PricePerUnit != 0.0001 {
		t.Errorf("Expected price 0.0001, got %f", retrieved.PricePerUnit)
	}
	if retrieved.Description != "Custom storage rate" {
		t.Errorf("Expected description 'Custom storage rate', got %s", retrieved.Description)
	}
}

func TestTracker_GetRate_NotFound(t *testing.T) {
	tracker := NewTracker("USD")

	_, exists := tracker.GetRate("nonexistent", "unit")
	if exists {
		t.Error("Expected rate not to exist")
	}
}

func TestTracker_RecordUsage(t *testing.T) {
	tracker := NewTracker("USD")

	record := UsageRecord{
		TenantID:     "tenant-1",
		FeatureGroup: "recommendations",
		Feature:      "user-embeddings",
		Category:     CostCategoryML,
		Unit:         CostUnitTokens,
		Quantity:     1000,
		Timestamp:    time.Now(),
	}

	entry, err := tracker.RecordUsage(record)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if entry.ID == "" {
		t.Error("Expected entry to have ID")
	}
	if entry.TenantID != "tenant-1" {
		t.Errorf("Expected tenant-1, got %s", entry.TenantID)
	}
	if entry.Cost <= 0 {
		t.Error("Expected positive cost")
	}
	if entry.Currency != "USD" {
		t.Errorf("Expected USD, got %s", entry.Currency)
	}
}

func TestTracker_RecordUsage_NoRate(t *testing.T) {
	tracker := NewTracker("USD")

	record := UsageRecord{
		TenantID: "tenant-1",
		Category: "nonexistent",
		Unit:     "unit",
		Quantity: 100,
	}

	_, err := tracker.RecordUsage(record)
	if err == nil {
		t.Error("Expected error for missing rate")
	}
}

func TestTracker_RecordUsage_FreeAllowance(t *testing.T) {
	tracker := NewTracker("USD")

	// Set a rate with free allowance
	tracker.SetRate(&CostRate{
		Category:      CostCategoryAPI,
		Unit:          CostUnitRequests,
		PricePerUnit:  0.001,
		FreeAllowance: 100,
	})

	// Usage within free allowance
	record := UsageRecord{
		TenantID: "tenant-1",
		Category: CostCategoryAPI,
		Unit:     CostUnitRequests,
		Quantity: 50,
	}

	entry, err := tracker.RecordUsage(record)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if entry.Cost != 0 {
		t.Errorf("Expected zero cost within free allowance, got %f", entry.Cost)
	}

	// Usage exceeding free allowance
	record.Quantity = 150
	entry, err = tracker.RecordUsage(record)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	expectedCost := 50 * 0.001 // 150 - 100 free = 50 billable
	if entry.Cost != expectedCost {
		t.Errorf("Expected cost %f, got %f", expectedCost, entry.Cost)
	}
}

func TestTracker_RecordUsage_MinCharge(t *testing.T) {
	tracker := NewTracker("USD")

	// Set a rate with minimum charge
	tracker.SetRate(&CostRate{
		Category:     CostCategoryAPI,
		Unit:         CostUnitRequests,
		PricePerUnit: 0.0001,
		MinCharge:    0.01,
	})

	record := UsageRecord{
		TenantID: "tenant-1",
		Category: CostCategoryAPI,
		Unit:     CostUnitRequests,
		Quantity: 10, // 10 * 0.0001 = 0.001 < min charge
	}

	entry, err := tracker.RecordUsage(record)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if entry.Cost != 0.01 {
		t.Errorf("Expected min charge 0.01, got %f", entry.Cost)
	}
}

func TestTracker_GetUsage(t *testing.T) {
	tracker := NewTracker("USD")
	now := time.Now()

	// Record some usage
	for i := 0; i < 5; i++ {
		tracker.RecordUsage(UsageRecord{
			TenantID:  "tenant-1",
			Category:  CostCategoryAPI,
			Unit:      CostUnitRequests,
			Quantity:  float64(i+1) * 100,
			Timestamp: now.Add(time.Duration(i) * time.Hour),
		})
	}

	// Also record for different tenant
	tracker.RecordUsage(UsageRecord{
		TenantID:  "tenant-2",
		Category:  CostCategoryAPI,
		Unit:      CostUnitRequests,
		Quantity:  500,
		Timestamp: now,
	})

	// Get usage for tenant-1
	records := tracker.GetUsage("tenant-1", now.Add(-time.Hour), now.Add(6*time.Hour))
	if len(records) != 5 {
		t.Errorf("Expected 5 records for tenant-1, got %d", len(records))
	}

	// Get usage for all tenants
	records = tracker.GetUsage("", now.Add(-time.Hour), now.Add(6*time.Hour))
	if len(records) != 6 {
		t.Errorf("Expected 6 total records, got %d", len(records))
	}
}

func TestTracker_GetCosts(t *testing.T) {
	tracker := NewTracker("USD")
	now := time.Now()

	// Record some usage
	for i := 0; i < 3; i++ {
		tracker.RecordUsage(UsageRecord{
			TenantID:  "tenant-1",
			Category:  CostCategoryAPI,
			Unit:      CostUnitRequests,
			Quantity:  1000,
			Timestamp: now.Add(time.Duration(i) * time.Hour),
		})
	}

	entries := tracker.GetCosts("tenant-1", now.Add(-time.Hour), now.Add(4*time.Hour))
	if len(entries) != 3 {
		t.Errorf("Expected 3 cost entries, got %d", len(entries))
	}
}

func TestTracker_GetCostSummary(t *testing.T) {
	tracker := NewTracker("USD")
	now := time.Now()

	// Record varied usage
	tracker.RecordUsage(UsageRecord{
		TenantID:     "tenant-1",
		FeatureGroup: "recs",
		Category:     CostCategoryAPI,
		Unit:         CostUnitRequests,
		Quantity:     1000,
		Timestamp:    now,
	})
	tracker.RecordUsage(UsageRecord{
		TenantID:     "tenant-1",
		FeatureGroup: "search",
		Category:     CostCategoryML,
		Unit:         CostUnitTokens,
		Quantity:     5000,
		Timestamp:    now,
	})

	summary := tracker.GetCostSummary("tenant-1", now.Add(-time.Hour), now.Add(time.Hour))

	if summary.TenantID != "tenant-1" {
		t.Errorf("Expected tenant-1, got %s", summary.TenantID)
	}
	if summary.TotalCost <= 0 {
		t.Error("Expected positive total cost")
	}
	if len(summary.ByCategory) != 2 {
		t.Errorf("Expected 2 categories, got %d", len(summary.ByCategory))
	}
	if len(summary.ByFeature) != 2 {
		t.Errorf("Expected 2 features, got %d", len(summary.ByFeature))
	}
}

func TestTracker_GetFeatureCosts(t *testing.T) {
	tracker := NewTracker("USD")
	now := time.Now()

	// Record usage for different feature groups
	tracker.RecordUsage(UsageRecord{
		TenantID:     "tenant-1",
		FeatureGroup: "recommendations",
		Category:     CostCategoryML,
		Unit:         CostUnitTokens,
		Quantity:     1000,
		Timestamp:    now,
	})
	tracker.RecordUsage(UsageRecord{
		TenantID:     "tenant-1",
		FeatureGroup: "recommendations",
		Category:     CostCategoryAPI,
		Unit:         CostUnitRequests,
		Quantity:     500,
		Timestamp:    now,
	})
	tracker.RecordUsage(UsageRecord{
		TenantID:     "tenant-1",
		FeatureGroup: "search",
		Category:     CostCategoryVector,
		Unit:         CostUnitRequests,
		Quantity:     2000,
		Timestamp:    now,
	})

	features := tracker.GetFeatureCosts("tenant-1", now.Add(-time.Hour), now.Add(time.Hour))

	if len(features) != 2 {
		t.Errorf("Expected 2 feature groups, got %d", len(features))
	}

	if recs, ok := features["recommendations"]; ok {
		if len(recs.ByCategory) != 2 {
			t.Errorf("Expected 2 categories for recommendations, got %d", len(recs.ByCategory))
		}
	} else {
		t.Error("Expected recommendations feature group")
	}
}

func TestTracker_GetTenantCosts(t *testing.T) {
	tracker := NewTracker("USD")
	now := time.Now()

	// Record usage for different tenants
	for i := 1; i <= 3; i++ {
		tracker.RecordUsage(UsageRecord{
			TenantID:  "tenant-" + string(rune('0'+i)),
			Category:  CostCategoryAPI,
			Unit:      CostUnitRequests,
			Quantity:  float64(i * 1000),
			Timestamp: now,
		})
	}

	tenants := tracker.GetTenantCosts(now.Add(-time.Hour), now.Add(time.Hour))

	if len(tenants) != 3 {
		t.Errorf("Expected 3 tenants, got %d", len(tenants))
	}
}

func TestTracker_GetTimeSeries(t *testing.T) {
	tracker := NewTracker("USD")
	now := time.Now().Truncate(24 * time.Hour)

	// Record usage across multiple days
	for i := 0; i < 7; i++ {
		tracker.RecordUsage(UsageRecord{
			TenantID:  "tenant-1",
			Category:  CostCategoryAPI,
			Unit:      CostUnitRequests,
			Quantity:  1000,
			Timestamp: now.Add(time.Duration(i) * 24 * time.Hour),
		})
	}

	// Daily granularity
	points := tracker.GetTimeSeries("tenant-1", now.Add(-time.Hour), now.Add(8*24*time.Hour), "daily")
	if len(points) != 7 {
		t.Errorf("Expected 7 daily points, got %d", len(points))
	}

	// Weekly granularity
	points = tracker.GetTimeSeries("tenant-1", now.Add(-time.Hour), now.Add(8*24*time.Hour), "weekly")
	if len(points) == 0 {
		t.Error("Expected at least 1 weekly point")
	}
}

func TestTracker_CalculateTrends(t *testing.T) {
	tracker := NewTracker("USD")
	now := time.Now().Truncate(24 * time.Hour)

	// Record varying usage across multiple days
	for i := 0; i < 10; i++ {
		tracker.RecordUsage(UsageRecord{
			TenantID:  "tenant-1",
			Category:  CostCategoryAPI,
			Unit:      CostUnitRequests,
			Quantity:  float64((i + 1) * 1000),
			Timestamp: now.Add(time.Duration(i) * 24 * time.Hour),
		})
	}

	trends := tracker.CalculateTrends("tenant-1", now.Add(-time.Hour), now.Add(11*24*time.Hour))

	if trends == nil {
		t.Fatal("Expected non-nil trends")
	}
	if trends.AverageDailyCost <= 0 {
		t.Error("Expected positive average daily cost")
	}
	if trends.ProjectedMonthly <= 0 {
		t.Error("Expected positive projected monthly")
	}
	if trends.HighestDayCost < trends.LowestDayCost {
		t.Error("Highest day cost should be >= lowest day cost")
	}
}

func TestTracker_CalculateTrends_Empty(t *testing.T) {
	tracker := NewTracker("USD")
	now := time.Now()

	trends := tracker.CalculateTrends("tenant-1", now.Add(-time.Hour), now.Add(time.Hour))

	if trends != nil {
		t.Error("Expected nil trends for empty data")
	}
}

func TestTracker_MaxRecords(t *testing.T) {
	tracker := NewTracker("USD")
	tracker.SetMaxRecords(10)

	// Record more than max
	for i := 0; i < 20; i++ {
		tracker.RecordUsage(UsageRecord{
			TenantID:  "tenant-1",
			Category:  CostCategoryAPI,
			Unit:      CostUnitRequests,
			Quantity:  100,
			Timestamp: time.Now(),
		})
	}

	count := tracker.RecordCount()
	if count != 10 {
		t.Errorf("Expected 10 records (max), got %d", count)
	}
}

func TestTracker_RecordCount(t *testing.T) {
	tracker := NewTracker("USD")

	if tracker.RecordCount() != 0 {
		t.Error("Expected 0 records initially")
	}

	tracker.RecordUsage(UsageRecord{
		TenantID: "tenant-1",
		Category: CostCategoryAPI,
		Unit:     CostUnitRequests,
		Quantity: 100,
	})

	if tracker.RecordCount() != 1 {
		t.Errorf("Expected 1 record, got %d", tracker.RecordCount())
	}
}
