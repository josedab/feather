package cost

import (
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// BudgetManager manages budgets and alerts.
type BudgetManager struct {
	mu      sync.RWMutex
	budgets map[string]*Budget
	alerts  []Alert
	tracker *Tracker
}

// NewBudgetManager creates a new budget manager.
func NewBudgetManager(tracker *Tracker) *BudgetManager {
	return &BudgetManager{
		budgets: make(map[string]*Budget),
		tracker: tracker,
	}
}

// CreateBudget creates a new budget.
func (m *BudgetManager) CreateBudget(budget *Budget) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if budget.ID == "" {
		budget.ID = uuid.New().String()
	}
	if budget.Name == "" {
		return fmt.Errorf("budget name is required")
	}
	if budget.Amount <= 0 {
		return fmt.Errorf("budget amount must be positive")
	}
	if budget.Currency == "" {
		budget.Currency = "USD"
	}
	if budget.Period == "" {
		budget.Period = BudgetPeriodMonthly
	}
	if len(budget.AlertThresholds) == 0 {
		budget.AlertThresholds = []float64{0.5, 0.8, 0.95}
	}

	budget.CreatedAt = time.Now()
	budget.UpdatedAt = time.Now()

	m.budgets[budget.ID] = budget
	return nil
}

// UpdateBudget updates an existing budget.
func (m *BudgetManager) UpdateBudget(budget *Budget) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.budgets[budget.ID]; !exists {
		return fmt.Errorf("budget not found: %s", budget.ID)
	}

	budget.UpdatedAt = time.Now()
	m.budgets[budget.ID] = budget
	return nil
}

// GetBudget returns a budget by ID.
func (m *BudgetManager) GetBudget(id string) (*Budget, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	budget, exists := m.budgets[id]
	return budget, exists
}

// DeleteBudget removes a budget.
func (m *BudgetManager) DeleteBudget(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.budgets[id]; !exists {
		return fmt.Errorf("budget not found: %s", id)
	}

	delete(m.budgets, id)
	return nil
}

// ListBudgets returns all budgets for a tenant.
func (m *BudgetManager) ListBudgets(tenantID string) []*Budget {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var budgets []*Budget
	for _, b := range m.budgets {
		if tenantID == "" || b.TenantID == tenantID {
			budgets = append(budgets, b)
		}
	}
	return budgets
}

// GetBudgetStatus returns the current status of a budget.
func (m *BudgetManager) GetBudgetStatus(budgetID string) (*BudgetStatus, error) {
	m.mu.RLock()
	budget, exists := m.budgets[budgetID]
	m.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("budget not found: %s", budgetID)
	}

	// Calculate period
	start, end := m.calculatePeriod(budget.Period)

	// Get costs for the period
	var totalCost float64
	entries := m.tracker.GetCosts(budget.TenantID, start, end)

	for _, entry := range entries {
		// Filter by categories if specified
		if len(budget.Categories) > 0 {
			found := false
			for _, cat := range budget.Categories {
				if entry.Category == cat {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		// Filter by feature groups if specified
		if len(budget.FeatureGroups) > 0 {
			found := false
			for _, fg := range budget.FeatureGroups {
				if entry.FeatureGroup == fg {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		totalCost += entry.Cost
	}

	// Calculate projected cost
	elapsed := time.Since(start)
	periodDuration := end.Sub(start)
	var projected float64
	if elapsed > 0 {
		projected = totalCost * (float64(periodDuration) / float64(elapsed))
	}

	percentUsed := (totalCost / budget.Amount) * 100

	// Determine alert level
	var alertLevel string
	if percentUsed >= 100 {
		alertLevel = "exceeded"
	} else if percentUsed >= 95 {
		alertLevel = "critical"
	} else if percentUsed >= 80 {
		alertLevel = "warning"
	}

	return &BudgetStatus{
		BudgetID:     budget.ID,
		TenantID:     budget.TenantID,
		PeriodStart:  start,
		PeriodEnd:    end,
		Spent:        totalCost,
		BudgetAmount: budget.Amount,
		Remaining:    budget.Amount - totalCost,
		PercentUsed:  percentUsed,
		Projected:    projected,
		Currency:     budget.Currency,
		AlertLevel:   alertLevel,
	}, nil
}

// CheckAllBudgets checks all budgets and generates alerts.
func (m *BudgetManager) CheckAllBudgets() []Alert {
	// Copy budget data while holding the lock to avoid deadlock with GetBudgetStatus
	m.mu.RLock()
	budgetsCopy := make([]*Budget, 0, len(m.budgets))
	for _, budget := range m.budgets {
		// Make a shallow copy of budget
		b := *budget
		budgetsCopy = append(budgetsCopy, &b)
	}
	m.mu.RUnlock()

	var newAlerts []Alert

	for _, budget := range budgetsCopy {
		status, err := m.GetBudgetStatus(budget.ID)
		if err != nil {
			continue
		}

		// Check thresholds
		for _, threshold := range budget.AlertThresholds {
			thresholdPercent := threshold * 100
			if status.PercentUsed >= thresholdPercent {
				// Check if we already have an alert for this threshold
				m.mu.RLock()
				hasAlert := false
				for _, a := range m.alerts {
					if a.BudgetID == budget.ID &&
						a.Type == AlertTypeBudgetThreshold &&
						a.Threshold == thresholdPercent {
						hasAlert = true
						break
					}
				}
				m.mu.RUnlock()

				if !hasAlert {
					alertType := AlertTypeBudgetThreshold
					severity := "warning"
					if status.PercentUsed >= 100 {
						alertType = AlertTypeBudgetExceeded
						severity = "critical"
					} else if status.PercentUsed >= 95 {
						severity = "critical"
					}

					alert := Alert{
						ID:          uuid.New().String(),
						TenantID:    budget.TenantID,
						Type:        alertType,
						Severity:    severity,
						Message:     fmt.Sprintf("Budget '%s' has reached %.1f%% of limit", budget.Name, status.PercentUsed),
						BudgetID:    budget.ID,
						Threshold:   thresholdPercent,
						ActualValue: status.PercentUsed,
						Timestamp:   time.Now(),
					}
					newAlerts = append(newAlerts, alert)

					m.mu.Lock()
					m.alerts = append(m.alerts, alert)
					m.mu.Unlock()
				}
			}
		}
	}

	return newAlerts
}

// GetAlerts returns alerts for a tenant.
func (m *BudgetManager) GetAlerts(tenantID string, since time.Time) []Alert {
	m.mu.RLock()
	defer m.mu.RUnlock()

	alerts := make([]Alert, 0, len(m.alerts))
	for _, a := range m.alerts {
		if tenantID != "" && a.TenantID != tenantID {
			continue
		}
		if a.Timestamp.Before(since) {
			continue
		}
		alerts = append(alerts, a)
	}
	return alerts
}

// AcknowledgeAlert acknowledges an alert.
func (m *BudgetManager) AcknowledgeAlert(alertID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i := range m.alerts {
		if m.alerts[i].ID == alertID {
			m.alerts[i].Acknowledged = true
			return nil
		}
	}
	return fmt.Errorf("alert not found: %s", alertID)
}

// calculatePeriod returns the start and end times for a budget period.
func (m *BudgetManager) calculatePeriod(period BudgetPeriod) (time.Time, time.Time) {
	now := time.Now()

	switch period {
	case BudgetPeriodDaily:
		start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		return start, start.Add(24 * time.Hour)
	case BudgetPeriodWeekly:
		// Start of week (Sunday)
		weekday := int(now.Weekday())
		start := time.Date(now.Year(), now.Month(), now.Day()-weekday, 0, 0, 0, 0, now.Location())
		return start, start.Add(7 * 24 * time.Hour)
	case BudgetPeriodMonthly:
		start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		return start, start.AddDate(0, 1, 0)
	case BudgetPeriodYearly:
		start := time.Date(now.Year(), 1, 1, 0, 0, 0, 0, now.Location())
		return start, start.AddDate(1, 0, 0)
	default:
		start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		return start, start.AddDate(0, 1, 0)
	}
}

// CreateAlert creates a custom alert.
func (m *BudgetManager) CreateAlert(alert Alert) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if alert.ID == "" {
		alert.ID = uuid.New().String()
	}
	if alert.Timestamp.IsZero() {
		alert.Timestamp = time.Now()
	}

	m.alerts = append(m.alerts, alert)
	return nil
}

// AlertCount returns the number of unacknowledged alerts.
func (m *BudgetManager) AlertCount(tenantID string) int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	count := 0
	for _, a := range m.alerts {
		if tenantID != "" && a.TenantID != tenantID {
			continue
		}
		if !a.Acknowledged {
			count++
		}
	}
	return count
}
