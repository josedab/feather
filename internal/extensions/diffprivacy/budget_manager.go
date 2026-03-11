package diffprivacy

import (
	"fmt"
	"sync"
	"time"
)

// BudgetKey uniquely identifies a privacy budget scope.
type BudgetKey struct {
	Feature    string `json:"feature"`
	EntityType string `json:"entity_type"`
}

func (k BudgetKey) String() string {
	return k.Feature + "/" + k.EntityType
}

// BudgetAccount tracks privacy budget for a specific scope.
type BudgetAccount struct {
	Key             BudgetKey `json:"key"`
	MaxEpsilon      float64   `json:"max_epsilon"`
	ConsumedEpsilon float64   `json:"consumed_epsilon"`
	MaxDelta        float64   `json:"max_delta"`
	ConsumedDelta   float64   `json:"consumed_delta"`
	QueryCount      int64     `json:"query_count"`
	LastQueryAt     time.Time `json:"last_query_at"`
	CreatedAt       time.Time `json:"created_at"`
	AlertThreshold  float64   `json:"alert_threshold"` // fraction of budget (0-1) triggering alert
}

// BudgetAlert represents a privacy budget warning.
type BudgetAlert struct {
	Key             BudgetKey `json:"key"`
	AlertType       string    `json:"alert_type"` // "threshold", "exhausted", "near_exhaustion"
	Message         string    `json:"message"`
	ConsumedEpsilon float64   `json:"consumed_epsilon"`
	MaxEpsilon      float64   `json:"max_epsilon"`
	PercentConsumed float64   `json:"percent_consumed"`
	Timestamp       time.Time `json:"timestamp"`
}

// BudgetManagerConfig configures the budget manager.
type BudgetManagerConfig struct {
	DefaultMaxEpsilon   float64       `json:"default_max_epsilon" yaml:"default_max_epsilon"`
	DefaultAlertAt      float64       `json:"default_alert_at" yaml:"default_alert_at"` // e.g. 0.8 = 80%
	BudgetRefreshPeriod time.Duration `json:"budget_refresh_period" yaml:"budget_refresh_period"`
	AutoReject          bool          `json:"auto_reject" yaml:"auto_reject"` // reject queries when budget exhausted
}

// DefaultBudgetManagerConfig returns sensible defaults.
func DefaultBudgetManagerConfig() BudgetManagerConfig {
	return BudgetManagerConfig{
		DefaultMaxEpsilon:   10.0,
		DefaultAlertAt:      0.8,
		BudgetRefreshPeriod: 24 * time.Hour,
		AutoReject:          true,
	}
}

// BudgetManager provides per-(feature, entity_type) privacy budget tracking.
type BudgetManager struct {
	mu       sync.RWMutex
	config   BudgetManagerConfig
	accounts map[string]*BudgetAccount // key string -> account
	alerts   []BudgetAlert
	queryLog []QueryRecord
}

// QueryRecord logs a privacy query for compliance.
type QueryRecord struct {
	Key         BudgetKey `json:"key"`
	EpsilonUsed float64   `json:"epsilon_used"`
	DeltaUsed   float64   `json:"delta_used"`
	Mechanism   Mechanism `json:"mechanism"`
	QueryType   string    `json:"query_type"` // "count", "sum", "avg", "noise"
	Timestamp   time.Time `json:"timestamp"`
	Approved    bool      `json:"approved"`
	Reason      string    `json:"reason,omitempty"`
}

// NewBudgetManager creates a new budget manager.
func NewBudgetManager(config BudgetManagerConfig) *BudgetManager {
	if config.DefaultMaxEpsilon == 0 {
		config = DefaultBudgetManagerConfig()
	}
	return &BudgetManager{
		config:   config,
		accounts: make(map[string]*BudgetAccount),
	}
}

// RegisterBudget creates a budget account for a (feature, entity_type) pair.
func (m *BudgetManager) RegisterBudget(key BudgetKey, maxEpsilon, maxDelta float64) error {
	if key.Feature == "" {
		return fmt.Errorf("feature is required")
	}
	if key.EntityType == "" {
		return fmt.Errorf("entity_type is required")
	}
	if maxEpsilon <= 0 {
		maxEpsilon = m.config.DefaultMaxEpsilon
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	keyStr := key.String()
	if _, exists := m.accounts[keyStr]; exists {
		return fmt.Errorf("budget already exists for %s", keyStr)
	}

	m.accounts[keyStr] = &BudgetAccount{
		Key:            key,
		MaxEpsilon:     maxEpsilon,
		MaxDelta:       maxDelta,
		AlertThreshold: m.config.DefaultAlertAt,
		CreatedAt:      time.Now(),
	}
	return nil
}

// ConsumeAndCheck checks budget, records query, and consumes budget.
// Returns error if budget would be exceeded and auto-reject is enabled.
func (m *BudgetManager) ConsumeAndCheck(key BudgetKey, epsilon, delta float64, mechanism Mechanism, queryType string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	keyStr := key.String()
	account, exists := m.accounts[keyStr]
	if !exists {
		// Auto-create account with defaults
		account = &BudgetAccount{
			Key:            key,
			MaxEpsilon:     m.config.DefaultMaxEpsilon,
			MaxDelta:       1e-5 * m.config.DefaultMaxEpsilon,
			AlertThreshold: m.config.DefaultAlertAt,
			CreatedAt:      time.Now(),
		}
		m.accounts[keyStr] = account
	}

	// Check if budget would be exceeded
	if account.ConsumedEpsilon+epsilon > account.MaxEpsilon {
		record := QueryRecord{
			Key:         key,
			EpsilonUsed: epsilon,
			DeltaUsed:   delta,
			Mechanism:   mechanism,
			QueryType:   queryType,
			Timestamp:   time.Now(),
			Approved:    false,
			Reason:      "budget exhausted",
		}
		m.queryLog = append(m.queryLog, record)

		m.alerts = append(m.alerts, BudgetAlert{
			Key:             key,
			AlertType:       "exhausted",
			Message:         fmt.Sprintf("privacy budget exhausted for %s", keyStr),
			ConsumedEpsilon: account.ConsumedEpsilon,
			MaxEpsilon:      account.MaxEpsilon,
			PercentConsumed: 100.0,
			Timestamp:       time.Now(),
		})

		if m.config.AutoReject {
			return fmt.Errorf("privacy budget exhausted for %s: consumed %.4f of %.4f epsilon", keyStr, account.ConsumedEpsilon, account.MaxEpsilon)
		}
	}

	// Consume budget
	account.ConsumedEpsilon += epsilon
	account.ConsumedDelta += delta
	account.QueryCount++
	account.LastQueryAt = time.Now()

	// Record query
	m.queryLog = append(m.queryLog, QueryRecord{
		Key:         key,
		EpsilonUsed: epsilon,
		DeltaUsed:   delta,
		Mechanism:   mechanism,
		QueryType:   queryType,
		Timestamp:   time.Now(),
		Approved:    true,
	})

	// Check alert threshold
	percentUsed := account.ConsumedEpsilon / account.MaxEpsilon
	if percentUsed >= account.AlertThreshold {
		alertType := "threshold"
		if percentUsed >= 0.95 {
			alertType = "near_exhaustion"
		}
		m.alerts = append(m.alerts, BudgetAlert{
			Key:             key,
			AlertType:       alertType,
			Message:         fmt.Sprintf("privacy budget at %.0f%% for %s", percentUsed*100, keyStr),
			ConsumedEpsilon: account.ConsumedEpsilon,
			MaxEpsilon:      account.MaxEpsilon,
			PercentConsumed: percentUsed * 100,
			Timestamp:       time.Now(),
		})
	}

	// Trim logs
	if len(m.queryLog) > 10000 {
		m.queryLog = m.queryLog[len(m.queryLog)-5000:]
	}
	if len(m.alerts) > 1000 {
		m.alerts = m.alerts[len(m.alerts)-500:]
	}

	return nil
}

// GetBudget returns budget status for a key.
func (m *BudgetManager) GetBudget(key BudgetKey) (*BudgetAccount, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	account, exists := m.accounts[key.String()]
	if !exists {
		return nil, fmt.Errorf("budget not found for %s", key.String())
	}
	result := *account
	return &result, nil
}

// ListBudgets returns all budget accounts.
func (m *BudgetManager) ListBudgets() []BudgetAccount {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]BudgetAccount, 0, len(m.accounts))
	for _, a := range m.accounts {
		result = append(result, *a)
	}
	return result
}

// GetAlerts returns recent budget alerts.
func (m *BudgetManager) GetAlerts(since time.Time) []BudgetAlert {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []BudgetAlert
	for _, a := range m.alerts {
		if a.Timestamp.After(since) {
			result = append(result, a)
		}
	}
	return result
}

// ResetBudget resets the consumed budget for a key.
func (m *BudgetManager) ResetBudget(key BudgetKey) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	account, exists := m.accounts[key.String()]
	if !exists {
		return fmt.Errorf("budget not found for %s", key.String())
	}
	account.ConsumedEpsilon = 0
	account.ConsumedDelta = 0
	account.QueryCount = 0
	return nil
}

// GetQueryLog returns recent query records for compliance.
func (m *BudgetManager) GetQueryLog(key *BudgetKey, limit int) []QueryRecord {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []QueryRecord
	for i := len(m.queryLog) - 1; i >= 0; i-- {
		record := m.queryLog[i]
		if key != nil && (record.Key.Feature != key.Feature || record.Key.EntityType != key.EntityType) {
			continue
		}
		result = append(result, record)
		if limit > 0 && len(result) >= limit {
			break
		}
	}
	return result
}

// BudgetManagerStats returns aggregate statistics.
type BudgetManagerStats struct {
	TotalAccounts    int   `json:"total_accounts"`
	TotalQueries     int64 `json:"total_queries"`
	RejectedQueries  int64 `json:"rejected_queries"`
	ActiveAlerts     int   `json:"active_alerts"`
	ExhaustedBudgets int   `json:"exhausted_budgets"`
}

// Stats returns aggregate statistics.
func (m *BudgetManager) Stats() BudgetManagerStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := BudgetManagerStats{
		TotalAccounts: len(m.accounts),
		ActiveAlerts:  len(m.alerts),
	}
	for _, a := range m.accounts {
		stats.TotalQueries += a.QueryCount
		if a.ConsumedEpsilon >= a.MaxEpsilon {
			stats.ExhaustedBudgets++
		}
	}
	for _, q := range m.queryLog {
		if !q.Approved {
			stats.RejectedQueries++
		}
	}
	return stats
}
