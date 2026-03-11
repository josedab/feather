package diffprivacy

import (
	"testing"
	"time"
)

func TestBudgetManager_RegisterAndGet(t *testing.T) {
	m := NewBudgetManager(DefaultBudgetManagerConfig())

	key := BudgetKey{Feature: "clicks", EntityType: "user"}
	if err := m.RegisterBudget(key, 5.0, 1e-5); err != nil {
		t.Fatalf("RegisterBudget: %v", err)
	}

	account, err := m.GetBudget(key)
	if err != nil {
		t.Fatalf("GetBudget: %v", err)
	}
	if account.MaxEpsilon != 5.0 {
		t.Errorf("MaxEpsilon = %f, want 5.0", account.MaxEpsilon)
	}
	if account.MaxDelta != 1e-5 {
		t.Errorf("MaxDelta = %e, want 1e-5", account.MaxDelta)
	}
	if account.ConsumedEpsilon != 0 {
		t.Errorf("ConsumedEpsilon = %f, want 0", account.ConsumedEpsilon)
	}
}

func TestBudgetManager_RegisterValidation(t *testing.T) {
	m := NewBudgetManager(DefaultBudgetManagerConfig())

	tests := []struct {
		name    string
		key     BudgetKey
		wantErr bool
	}{
		{"valid", BudgetKey{Feature: "f", EntityType: "user"}, false},
		{"empty feature", BudgetKey{Feature: "", EntityType: "user"}, true},
		{"empty entity", BudgetKey{Feature: "f", EntityType: ""}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr := NewBudgetManager(DefaultBudgetManagerConfig())
			err := mgr.RegisterBudget(tt.key, 10.0, 1e-5)
			if tt.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}

	// Duplicate registration
	key := BudgetKey{Feature: "dup", EntityType: "user"}
	if err := m.RegisterBudget(key, 10.0, 1e-5); err != nil {
		t.Fatal(err)
	}
	if err := m.RegisterBudget(key, 10.0, 1e-5); err == nil {
		t.Error("expected error for duplicate registration")
	}
}

func TestBudgetManager_RegisterDefaultEpsilon(t *testing.T) {
	m := NewBudgetManager(DefaultBudgetManagerConfig())
	key := BudgetKey{Feature: "f", EntityType: "user"}
	if err := m.RegisterBudget(key, -1, 1e-5); err != nil {
		t.Fatal(err)
	}
	account, _ := m.GetBudget(key)
	if account.MaxEpsilon != 10.0 {
		t.Errorf("MaxEpsilon = %f, want default 10.0", account.MaxEpsilon)
	}
}

func TestBudgetManager_ConsumeAndCheck(t *testing.T) {
	m := NewBudgetManager(DefaultBudgetManagerConfig())
	key := BudgetKey{Feature: "clicks", EntityType: "user"}
	if err := m.RegisterBudget(key, 5.0, 1e-5); err != nil {
		t.Fatal(err)
	}

	// Consume some budget
	if err := m.ConsumeAndCheck(key, 1.0, 1e-6, MechanismLaplace, "count"); err != nil {
		t.Fatalf("ConsumeAndCheck: %v", err)
	}

	account, _ := m.GetBudget(key)
	if account.ConsumedEpsilon != 1.0 {
		t.Errorf("ConsumedEpsilon = %f, want 1.0", account.ConsumedEpsilon)
	}
	if account.QueryCount != 1 {
		t.Errorf("QueryCount = %d, want 1", account.QueryCount)
	}
}

func TestBudgetManager_AutoReject(t *testing.T) {
	cfg := DefaultBudgetManagerConfig()
	cfg.AutoReject = true
	m := NewBudgetManager(cfg)

	key := BudgetKey{Feature: "clicks", EntityType: "user"}
	if err := m.RegisterBudget(key, 2.0, 1e-5); err != nil {
		t.Fatal(err)
	}

	// First two queries should succeed
	if err := m.ConsumeAndCheck(key, 1.0, 1e-6, MechanismLaplace, "count"); err != nil {
		t.Fatalf("first consume: %v", err)
	}
	if err := m.ConsumeAndCheck(key, 1.0, 1e-6, MechanismLaplace, "count"); err != nil {
		t.Fatalf("second consume: %v", err)
	}

	// Third should be rejected
	err := m.ConsumeAndCheck(key, 1.0, 1e-6, MechanismLaplace, "count")
	if err == nil {
		t.Fatal("expected rejection error, got nil")
	}
}

func TestBudgetManager_NoAutoReject(t *testing.T) {
	cfg := DefaultBudgetManagerConfig()
	cfg.AutoReject = false
	m := NewBudgetManager(cfg)

	key := BudgetKey{Feature: "clicks", EntityType: "user"}
	if err := m.RegisterBudget(key, 2.0, 1e-5); err != nil {
		t.Fatal(err)
	}

	// Consume all budget
	for i := 0; i < 2; i++ {
		if err := m.ConsumeAndCheck(key, 1.0, 1e-6, MechanismLaplace, "count"); err != nil {
			t.Fatalf("consume %d: %v", i, err)
		}
	}

	// Should NOT be rejected when AutoReject is false
	if err := m.ConsumeAndCheck(key, 1.0, 1e-6, MechanismLaplace, "count"); err != nil {
		t.Fatalf("expected no error with AutoReject=false, got: %v", err)
	}
}

func TestBudgetManager_AlertGeneration(t *testing.T) {
	cfg := DefaultBudgetManagerConfig()
	cfg.DefaultAlertAt = 0.8
	m := NewBudgetManager(cfg)

	key := BudgetKey{Feature: "clicks", EntityType: "user"}
	if err := m.RegisterBudget(key, 10.0, 1e-5); err != nil {
		t.Fatal(err)
	}

	// Consume 80% of budget (threshold)
	for i := 0; i < 8; i++ {
		if err := m.ConsumeAndCheck(key, 1.0, 1e-6, MechanismLaplace, "count"); err != nil {
			t.Fatalf("consume %d: %v", i, err)
		}
	}

	alerts := m.GetAlerts(time.Time{})
	if len(alerts) == 0 {
		t.Fatal("expected alerts after reaching threshold")
	}

	foundThreshold := false
	for _, a := range alerts {
		if a.AlertType == "threshold" || a.AlertType == "near_exhaustion" {
			foundThreshold = true
			break
		}
	}
	if !foundThreshold {
		t.Error("expected threshold or near_exhaustion alert")
	}
}

func TestBudgetManager_NearExhaustionAlert(t *testing.T) {
	cfg := DefaultBudgetManagerConfig()
	cfg.DefaultAlertAt = 0.5
	m := NewBudgetManager(cfg)

	key := BudgetKey{Feature: "clicks", EntityType: "user"}
	if err := m.RegisterBudget(key, 10.0, 1e-5); err != nil {
		t.Fatal(err)
	}

	// Consume 96% of budget
	for i := 0; i < 9; i++ {
		if err := m.ConsumeAndCheck(key, 1.0, 1e-6, MechanismLaplace, "count"); err != nil {
			t.Fatalf("consume %d: %v", i, err)
		}
	}
	if err := m.ConsumeAndCheck(key, 0.6, 1e-6, MechanismLaplace, "count"); err != nil {
		t.Fatal(err)
	}

	alerts := m.GetAlerts(time.Time{})
	foundNearExhaustion := false
	for _, a := range alerts {
		if a.AlertType == "near_exhaustion" {
			foundNearExhaustion = true
			break
		}
	}
	if !foundNearExhaustion {
		t.Error("expected near_exhaustion alert at 96%")
	}
}

func TestBudgetManager_ResetBudget(t *testing.T) {
	m := NewBudgetManager(DefaultBudgetManagerConfig())
	key := BudgetKey{Feature: "clicks", EntityType: "user"}
	if err := m.RegisterBudget(key, 5.0, 1e-5); err != nil {
		t.Fatal(err)
	}

	// Consume some budget
	if err := m.ConsumeAndCheck(key, 2.0, 1e-6, MechanismLaplace, "sum"); err != nil {
		t.Fatal(err)
	}

	// Reset
	if err := m.ResetBudget(key); err != nil {
		t.Fatalf("ResetBudget: %v", err)
	}

	account, _ := m.GetBudget(key)
	if account.ConsumedEpsilon != 0 {
		t.Errorf("ConsumedEpsilon = %f, want 0 after reset", account.ConsumedEpsilon)
	}
	if account.QueryCount != 0 {
		t.Errorf("QueryCount = %d, want 0 after reset", account.QueryCount)
	}

	// Reset nonexistent key
	if err := m.ResetBudget(BudgetKey{Feature: "nope", EntityType: "user"}); err == nil {
		t.Error("expected error resetting nonexistent key")
	}
}

func TestBudgetManager_QueryLog(t *testing.T) {
	m := NewBudgetManager(DefaultBudgetManagerConfig())
	key := BudgetKey{Feature: "clicks", EntityType: "user"}
	if err := m.RegisterBudget(key, 10.0, 1e-5); err != nil {
		t.Fatal(err)
	}

	// Generate some queries
	for i := 0; i < 3; i++ {
		if err := m.ConsumeAndCheck(key, 1.0, 1e-6, MechanismLaplace, "count"); err != nil {
			t.Fatal(err)
		}
	}

	// Get all logs
	logs := m.GetQueryLog(nil, 0)
	if len(logs) != 3 {
		t.Errorf("len(logs) = %d, want 3", len(logs))
	}

	// Get filtered logs
	filteredLogs := m.GetQueryLog(&key, 2)
	if len(filteredLogs) != 2 {
		t.Errorf("len(filteredLogs) = %d, want 2", len(filteredLogs))
	}

	// All should be approved
	for _, l := range logs {
		if !l.Approved {
			t.Error("expected approved query")
		}
	}
}

func TestBudgetManager_QueryLogRejected(t *testing.T) {
	cfg := DefaultBudgetManagerConfig()
	cfg.AutoReject = true
	m := NewBudgetManager(cfg)

	key := BudgetKey{Feature: "clicks", EntityType: "user"}
	if err := m.RegisterBudget(key, 1.0, 1e-5); err != nil {
		t.Fatal(err)
	}

	// One approved, one rejected
	_ = m.ConsumeAndCheck(key, 1.0, 1e-6, MechanismLaplace, "count")
	_ = m.ConsumeAndCheck(key, 1.0, 1e-6, MechanismLaplace, "count") // rejected

	logs := m.GetQueryLog(nil, 0)
	rejected := 0
	for _, l := range logs {
		if !l.Approved {
			rejected++
		}
	}
	if rejected != 1 {
		t.Errorf("rejected = %d, want 1", rejected)
	}
}

func TestBudgetManager_AutoCreateAccount(t *testing.T) {
	m := NewBudgetManager(DefaultBudgetManagerConfig())
	key := BudgetKey{Feature: "new_feature", EntityType: "user"}

	// ConsumeAndCheck should auto-create
	if err := m.ConsumeAndCheck(key, 1.0, 1e-6, MechanismLaplace, "count"); err != nil {
		t.Fatalf("ConsumeAndCheck auto-create: %v", err)
	}

	account, err := m.GetBudget(key)
	if err != nil {
		t.Fatalf("GetBudget after auto-create: %v", err)
	}
	if account.MaxEpsilon != 10.0 {
		t.Errorf("MaxEpsilon = %f, want default 10.0", account.MaxEpsilon)
	}
}

func TestBudgetManager_ListBudgets(t *testing.T) {
	m := NewBudgetManager(DefaultBudgetManagerConfig())
	keys := []BudgetKey{
		{Feature: "clicks", EntityType: "user"},
		{Feature: "views", EntityType: "session"},
	}
	for _, k := range keys {
		if err := m.RegisterBudget(k, 10.0, 1e-5); err != nil {
			t.Fatal(err)
		}
	}

	budgets := m.ListBudgets()
	if len(budgets) != 2 {
		t.Errorf("len(budgets) = %d, want 2", len(budgets))
	}
}

func TestBudgetManager_Stats(t *testing.T) {
	cfg := DefaultBudgetManagerConfig()
	cfg.AutoReject = true
	m := NewBudgetManager(cfg)

	key := BudgetKey{Feature: "clicks", EntityType: "user"}
	if err := m.RegisterBudget(key, 2.0, 1e-5); err != nil {
		t.Fatal(err)
	}

	// Two approved queries
	_ = m.ConsumeAndCheck(key, 1.0, 1e-6, MechanismLaplace, "count")
	_ = m.ConsumeAndCheck(key, 1.0, 1e-6, MechanismLaplace, "count")
	// One rejected
	_ = m.ConsumeAndCheck(key, 1.0, 1e-6, MechanismLaplace, "count")

	stats := m.Stats()
	if stats.TotalAccounts != 1 {
		t.Errorf("TotalAccounts = %d, want 1", stats.TotalAccounts)
	}
	if stats.TotalQueries != 2 {
		t.Errorf("TotalQueries = %d, want 2", stats.TotalQueries)
	}
	if stats.RejectedQueries != 1 {
		t.Errorf("RejectedQueries = %d, want 1", stats.RejectedQueries)
	}
	if stats.ExhaustedBudgets != 1 {
		t.Errorf("ExhaustedBudgets = %d, want 1", stats.ExhaustedBudgets)
	}
}

func TestBudgetManager_GetBudgetNotFound(t *testing.T) {
	m := NewBudgetManager(DefaultBudgetManagerConfig())
	_, err := m.GetBudget(BudgetKey{Feature: "nope", EntityType: "user"})
	if err == nil {
		t.Error("expected error for nonexistent budget")
	}
}

func TestBudgetKey_String(t *testing.T) {
	key := BudgetKey{Feature: "clicks", EntityType: "user"}
	if key.String() != "clicks/user" {
		t.Errorf("String() = %q, want %q", key.String(), "clicks/user")
	}
}

func TestNewBudgetManager_ZeroConfig(t *testing.T) {
	m := NewBudgetManager(BudgetManagerConfig{})
	if m == nil {
		t.Fatal("NewBudgetManager returned nil")
	}
	// Should have applied defaults
	if m.config.DefaultMaxEpsilon != 10.0 {
		t.Errorf("DefaultMaxEpsilon = %f, want 10.0", m.config.DefaultMaxEpsilon)
	}
}
