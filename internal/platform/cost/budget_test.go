package cost

import (
	"testing"
	"time"
)

func TestNewBudgetManager(t *testing.T) {
	tracker := NewTracker("USD")
	manager := NewBudgetManager(tracker)

	if manager == nil {
		t.Fatal("Expected non-nil manager")
	}
	if manager.tracker != tracker {
		t.Error("Expected tracker to be set")
	}
}

func TestBudgetManager_CreateBudget(t *testing.T) {
	tracker := NewTracker("USD")
	manager := NewBudgetManager(tracker)

	budget := &Budget{
		TenantID: "tenant-1",
		Name:     "Monthly API Budget",
		Amount:   1000.00,
		Period:   BudgetPeriodMonthly,
	}

	err := manager.CreateBudget(budget)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if budget.ID == "" {
		t.Error("Expected budget to have ID")
	}
	if budget.Currency != "USD" {
		t.Errorf("Expected USD default currency, got %s", budget.Currency)
	}
	if len(budget.AlertThresholds) != 3 {
		t.Errorf("Expected 3 default thresholds, got %d", len(budget.AlertThresholds))
	}
}

func TestBudgetManager_CreateBudget_Validation(t *testing.T) {
	tracker := NewTracker("USD")
	manager := NewBudgetManager(tracker)

	tests := []struct {
		name    string
		budget  *Budget
		wantErr bool
	}{
		{
			name: "missing name",
			budget: &Budget{
				TenantID: "tenant-1",
				Amount:   1000,
			},
			wantErr: true,
		},
		{
			name: "zero amount",
			budget: &Budget{
				TenantID: "tenant-1",
				Name:     "Test",
				Amount:   0,
			},
			wantErr: true,
		},
		{
			name: "negative amount",
			budget: &Budget{
				TenantID: "tenant-1",
				Name:     "Test",
				Amount:   -100,
			},
			wantErr: true,
		},
		{
			name: "valid budget",
			budget: &Budget{
				TenantID: "tenant-1",
				Name:     "Test",
				Amount:   100,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := manager.CreateBudget(tt.budget)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateBudget() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestBudgetManager_UpdateBudget(t *testing.T) {
	tracker := NewTracker("USD")
	manager := NewBudgetManager(tracker)

	budget := &Budget{
		TenantID: "tenant-1",
		Name:     "Test Budget",
		Amount:   1000,
	}
	manager.CreateBudget(budget)

	// Update the budget
	budget.Amount = 2000
	err := manager.UpdateBudget(budget)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Verify update
	retrieved, exists := manager.GetBudget(budget.ID)
	if !exists {
		t.Fatal("Expected budget to exist")
	}
	if retrieved.Amount != 2000 {
		t.Errorf("Expected amount 2000, got %f", retrieved.Amount)
	}
}

func TestBudgetManager_UpdateBudget_NotFound(t *testing.T) {
	tracker := NewTracker("USD")
	manager := NewBudgetManager(tracker)

	err := manager.UpdateBudget(&Budget{ID: "nonexistent"})
	if err == nil {
		t.Error("Expected error for nonexistent budget")
	}
}

func TestBudgetManager_GetBudget(t *testing.T) {
	tracker := NewTracker("USD")
	manager := NewBudgetManager(tracker)

	budget := &Budget{
		TenantID: "tenant-1",
		Name:     "Test Budget",
		Amount:   1000,
	}
	manager.CreateBudget(budget)

	retrieved, exists := manager.GetBudget(budget.ID)
	if !exists {
		t.Fatal("Expected budget to exist")
	}
	if retrieved.Name != "Test Budget" {
		t.Errorf("Expected 'Test Budget', got %s", retrieved.Name)
	}
}

func TestBudgetManager_GetBudget_NotFound(t *testing.T) {
	tracker := NewTracker("USD")
	manager := NewBudgetManager(tracker)

	_, exists := manager.GetBudget("nonexistent")
	if exists {
		t.Error("Expected budget not to exist")
	}
}

func TestBudgetManager_DeleteBudget(t *testing.T) {
	tracker := NewTracker("USD")
	manager := NewBudgetManager(tracker)

	budget := &Budget{
		TenantID: "tenant-1",
		Name:     "Test Budget",
		Amount:   1000,
	}
	manager.CreateBudget(budget)

	err := manager.DeleteBudget(budget.ID)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	_, exists := manager.GetBudget(budget.ID)
	if exists {
		t.Error("Expected budget to be deleted")
	}
}

func TestBudgetManager_DeleteBudget_NotFound(t *testing.T) {
	tracker := NewTracker("USD")
	manager := NewBudgetManager(tracker)

	err := manager.DeleteBudget("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent budget")
	}
}

func TestBudgetManager_ListBudgets(t *testing.T) {
	tracker := NewTracker("USD")
	manager := NewBudgetManager(tracker)

	// Create budgets for different tenants
	manager.CreateBudget(&Budget{TenantID: "tenant-1", Name: "Budget 1", Amount: 1000})
	manager.CreateBudget(&Budget{TenantID: "tenant-1", Name: "Budget 2", Amount: 2000})
	manager.CreateBudget(&Budget{TenantID: "tenant-2", Name: "Budget 3", Amount: 3000})

	// List all budgets
	all := manager.ListBudgets("")
	if len(all) != 3 {
		t.Errorf("Expected 3 budgets, got %d", len(all))
	}

	// List budgets for tenant-1
	tenant1 := manager.ListBudgets("tenant-1")
	if len(tenant1) != 2 {
		t.Errorf("Expected 2 budgets for tenant-1, got %d", len(tenant1))
	}

	// List budgets for tenant-2
	tenant2 := manager.ListBudgets("tenant-2")
	if len(tenant2) != 1 {
		t.Errorf("Expected 1 budget for tenant-2, got %d", len(tenant2))
	}
}

func TestBudgetManager_GetBudgetStatus(t *testing.T) {
	tracker := NewTracker("USD")
	manager := NewBudgetManager(tracker)

	budget := &Budget{
		TenantID: "tenant-1",
		Name:     "Test Budget",
		Amount:   100,
		Period:   BudgetPeriodMonthly,
	}
	manager.CreateBudget(budget)

	// Record some usage
	tracker.RecordUsage(UsageRecord{
		TenantID:  "tenant-1",
		Category:  CostCategoryAPI,
		Unit:      CostUnitRequests,
		Quantity:  100000, // This should generate some cost
		Timestamp: time.Now(),
	})

	status, err := manager.GetBudgetStatus(budget.ID)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if status.BudgetID != budget.ID {
		t.Errorf("Expected budget ID %s, got %s", budget.ID, status.BudgetID)
	}
	if status.BudgetAmount != 100 {
		t.Errorf("Expected budget amount 100, got %f", status.BudgetAmount)
	}
	if status.Spent < 0 {
		t.Error("Expected non-negative spent amount")
	}
}

func TestBudgetManager_GetBudgetStatus_NotFound(t *testing.T) {
	tracker := NewTracker("USD")
	manager := NewBudgetManager(tracker)

	_, err := manager.GetBudgetStatus("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent budget")
	}
}

func TestBudgetManager_GetBudgetStatus_WithFilters(t *testing.T) {
	tracker := NewTracker("USD")
	manager := NewBudgetManager(tracker)

	budget := &Budget{
		TenantID:   "tenant-1",
		Name:       "API Budget",
		Amount:     100,
		Period:     BudgetPeriodMonthly,
		Categories: []CostCategory{CostCategoryAPI},
	}
	manager.CreateBudget(budget)

	// Record API usage
	tracker.RecordUsage(UsageRecord{
		TenantID:  "tenant-1",
		Category:  CostCategoryAPI,
		Unit:      CostUnitRequests,
		Quantity:  50000,
		Timestamp: time.Now(),
	})

	// Record ML usage (should be excluded)
	tracker.RecordUsage(UsageRecord{
		TenantID:  "tenant-1",
		Category:  CostCategoryML,
		Unit:      CostUnitTokens,
		Quantity:  100000,
		Timestamp: time.Now(),
	})

	status, err := manager.GetBudgetStatus(budget.ID)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Only API costs should be counted
	if status.Spent <= 0 {
		t.Error("Expected positive spent amount for API costs")
	}
}

func TestBudgetManager_CheckAllBudgets(t *testing.T) {
	tracker := NewTracker("USD")
	manager := NewBudgetManager(tracker)

	budget := &Budget{
		TenantID:        "tenant-1",
		Name:            "Test Budget",
		Amount:          0.001, // Very small budget to trigger alert
		Period:          BudgetPeriodMonthly,
		AlertThresholds: []float64{0.5, 0.8, 0.95},
	}
	manager.CreateBudget(budget)

	// Record usage that exceeds budget
	tracker.RecordUsage(UsageRecord{
		TenantID:  "tenant-1",
		Category:  CostCategoryAPI,
		Unit:      CostUnitRequests,
		Quantity:  1000,
		Timestamp: time.Now(),
	})

	alerts := manager.CheckAllBudgets()
	if len(alerts) == 0 {
		t.Error("Expected at least one alert")
	}
}

func TestBudgetManager_GetAlerts(t *testing.T) {
	tracker := NewTracker("USD")
	manager := NewBudgetManager(tracker)

	// Create custom alerts
	manager.CreateAlert(Alert{
		TenantID:  "tenant-1",
		Type:      AlertTypeBudgetThreshold,
		Severity:  "warning",
		Message:   "Test alert 1",
		Timestamp: time.Now(),
	})
	manager.CreateAlert(Alert{
		TenantID:  "tenant-2",
		Type:      AlertTypeBudgetThreshold,
		Severity:  "warning",
		Message:   "Test alert 2",
		Timestamp: time.Now(),
	})
	manager.CreateAlert(Alert{
		TenantID:  "tenant-1",
		Type:      AlertTypeBudgetThreshold,
		Severity:  "critical",
		Message:   "Test alert 3",
		Timestamp: time.Now().Add(-2 * time.Hour),
	})

	// Get alerts for tenant-1
	tenant1Alerts := manager.GetAlerts("tenant-1", time.Now().Add(-time.Hour))
	if len(tenant1Alerts) != 1 {
		t.Errorf("Expected 1 recent alert for tenant-1, got %d", len(tenant1Alerts))
	}

	// Get all alerts
	allAlerts := manager.GetAlerts("", time.Now().Add(-3*time.Hour))
	if len(allAlerts) != 3 {
		t.Errorf("Expected 3 alerts total, got %d", len(allAlerts))
	}
}

func TestBudgetManager_AcknowledgeAlert(t *testing.T) {
	tracker := NewTracker("USD")
	manager := NewBudgetManager(tracker)

	alert := Alert{
		TenantID:  "tenant-1",
		Type:      AlertTypeBudgetThreshold,
		Severity:  "warning",
		Message:   "Test alert",
		Timestamp: time.Now(),
	}
	manager.CreateAlert(alert)

	// Get the alert ID
	alerts := manager.GetAlerts("tenant-1", time.Now().Add(-time.Hour))
	if len(alerts) == 0 {
		t.Fatal("Expected alert to exist")
	}

	// Acknowledge the alert
	err := manager.AcknowledgeAlert(alerts[0].ID)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Verify acknowledged
	alerts = manager.GetAlerts("tenant-1", time.Now().Add(-time.Hour))
	if len(alerts) == 0 || !alerts[0].Acknowledged {
		t.Error("Expected alert to be acknowledged")
	}
}

func TestBudgetManager_AcknowledgeAlert_NotFound(t *testing.T) {
	tracker := NewTracker("USD")
	manager := NewBudgetManager(tracker)

	err := manager.AcknowledgeAlert("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent alert")
	}
}

func TestBudgetManager_AlertCount(t *testing.T) {
	tracker := NewTracker("USD")
	manager := NewBudgetManager(tracker)

	// Create alerts
	manager.CreateAlert(Alert{TenantID: "tenant-1", Timestamp: time.Now()})
	manager.CreateAlert(Alert{TenantID: "tenant-1", Timestamp: time.Now()})
	manager.CreateAlert(Alert{TenantID: "tenant-2", Timestamp: time.Now()})

	// Total unacknowledged
	if count := manager.AlertCount(""); count != 3 {
		t.Errorf("Expected 3 alerts total, got %d", count)
	}

	// Tenant-1 alerts
	if count := manager.AlertCount("tenant-1"); count != 2 {
		t.Errorf("Expected 2 alerts for tenant-1, got %d", count)
	}

	// Acknowledge one
	alerts := manager.GetAlerts("tenant-1", time.Now().Add(-time.Hour))
	manager.AcknowledgeAlert(alerts[0].ID)

	// Verify count decreased
	if count := manager.AlertCount("tenant-1"); count != 1 {
		t.Errorf("Expected 1 unacknowledged alert, got %d", count)
	}
}

func TestBudgetManager_CreateAlert(t *testing.T) {
	tracker := NewTracker("USD")
	manager := NewBudgetManager(tracker)

	alert := Alert{
		TenantID: "tenant-1",
		Type:     AlertTypeCostSpike,
		Severity: "critical",
		Message:  "Cost spike detected",
	}

	err := manager.CreateAlert(alert)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	alerts := manager.GetAlerts("tenant-1", time.Time{})
	if len(alerts) != 1 {
		t.Errorf("Expected 1 alert, got %d", len(alerts))
	}
	if alerts[0].ID == "" {
		t.Error("Expected alert to have ID")
	}
	if alerts[0].Timestamp.IsZero() {
		t.Error("Expected alert to have timestamp")
	}
}

func TestBudgetManager_CalculatePeriod(t *testing.T) {
	tracker := NewTracker("USD")
	manager := NewBudgetManager(tracker)

	tests := []struct {
		period BudgetPeriod
	}{
		{BudgetPeriodDaily},
		{BudgetPeriodWeekly},
		{BudgetPeriodMonthly},
		{BudgetPeriodYearly},
	}

	for _, tt := range tests {
		t.Run(string(tt.period), func(t *testing.T) {
			start, end := manager.calculatePeriod(tt.period)
			if !start.Before(end) {
				t.Errorf("Expected start before end for %s", tt.period)
			}
			if start.After(time.Now()) {
				t.Errorf("Expected start not after now for %s", tt.period)
			}
		})
	}
}
