package operator

import (
	"fmt"
	"sync"
	"time"
)

// UpgradeStrategy defines how rolling upgrades proceed.
type UpgradeStrategy struct {
	MaxUnavailable int           `json:"max_unavailable" yaml:"max_unavailable"`
	MaxSurge       int           `json:"max_surge" yaml:"max_surge"`
	DrainTimeout   time.Duration `json:"drain_timeout" yaml:"drain_timeout"`
}

// DefaultUpgradeStrategy returns sensible defaults.
func DefaultUpgradeStrategy() UpgradeStrategy {
	return UpgradeStrategy{
		MaxUnavailable: 1,
		MaxSurge:       1,
		DrainTimeout:   30 * time.Second,
	}
}

// UpgradePhase represents the current phase of a rolling upgrade.
type UpgradePhase string

const (
	UpgradePending    UpgradePhase = "pending"
	UpgradeInProgress UpgradePhase = "in_progress"
	UpgradePaused     UpgradePhase = "paused"
	UpgradeCompleted  UpgradePhase = "completed"
	UpgradeRolledBack UpgradePhase = "rolled_back"
	UpgradeFailed     UpgradePhase = "failed"
)

// RollingUpgrade tracks a rolling upgrade operation.
type RollingUpgrade struct {
	ID              string          `json:"id"`
	FeatureStore    string          `json:"feature_store"`
	FromVersion     string          `json:"from_version"`
	ToVersion       string          `json:"to_version"`
	Strategy        UpgradeStrategy `json:"strategy"`
	Phase           UpgradePhase    `json:"phase"`
	TotalReplicas   int             `json:"total_replicas"`
	UpdatedReplicas int             `json:"updated_replicas"`
	ReadyReplicas   int             `json:"ready_replicas"`
	StartedAt       time.Time       `json:"started_at"`
	CompletedAt     *time.Time      `json:"completed_at,omitempty"`
	Error           string          `json:"error,omitempty"`
}

// UpgradeManager manages rolling upgrades.
type UpgradeManager struct {
	mu       sync.RWMutex
	upgrades map[string]*RollingUpgrade // featureStore -> upgrade
	history  []RollingUpgrade
	nextID   int
}

// NewUpgradeManager creates a new upgrade manager.
func NewUpgradeManager() *UpgradeManager {
	return &UpgradeManager{
		upgrades: make(map[string]*RollingUpgrade),
	}
}

// StartUpgrade begins a rolling upgrade.
func (m *UpgradeManager) StartUpgrade(featureStore, fromVersion, toVersion string, totalReplicas int, strategy UpgradeStrategy) (*RollingUpgrade, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.upgrades[featureStore]; exists {
		return nil, fmt.Errorf("upgrade already in progress for %s", featureStore)
	}

	m.nextID++
	upgrade := &RollingUpgrade{
		ID:            fmt.Sprintf("upgrade-%d", m.nextID),
		FeatureStore:  featureStore,
		FromVersion:   fromVersion,
		ToVersion:     toVersion,
		Strategy:      strategy,
		Phase:         UpgradeInProgress,
		TotalReplicas: totalReplicas,
		StartedAt:     time.Now(),
	}
	m.upgrades[featureStore] = upgrade
	return upgrade, nil
}

// AdvanceUpgrade marks one more replica as updated.
func (m *UpgradeManager) AdvanceUpgrade(featureStore string) (*RollingUpgrade, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	upgrade, exists := m.upgrades[featureStore]
	if !exists {
		return nil, fmt.Errorf("no upgrade in progress for %s", featureStore)
	}
	if upgrade.Phase != UpgradeInProgress {
		return nil, fmt.Errorf("upgrade is not in progress (phase: %s)", upgrade.Phase)
	}

	upgrade.UpdatedReplicas++
	upgrade.ReadyReplicas++

	if upgrade.UpdatedReplicas >= upgrade.TotalReplicas {
		upgrade.Phase = UpgradeCompleted
		now := time.Now()
		upgrade.CompletedAt = &now
		m.history = append(m.history, *upgrade)
		delete(m.upgrades, featureStore)
	}

	result := *upgrade
	return &result, nil
}

// PauseUpgrade pauses a rolling upgrade.
func (m *UpgradeManager) PauseUpgrade(featureStore string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	upgrade, exists := m.upgrades[featureStore]
	if !exists {
		return fmt.Errorf("no upgrade in progress for %s", featureStore)
	}
	upgrade.Phase = UpgradePaused
	return nil
}

// ResumeUpgrade resumes a paused upgrade.
func (m *UpgradeManager) ResumeUpgrade(featureStore string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	upgrade, exists := m.upgrades[featureStore]
	if !exists {
		return fmt.Errorf("no upgrade in progress for %s", featureStore)
	}
	if upgrade.Phase != UpgradePaused {
		return fmt.Errorf("upgrade is not paused")
	}
	upgrade.Phase = UpgradeInProgress
	return nil
}

// RollbackUpgrade rolls back a running or paused upgrade.
func (m *UpgradeManager) RollbackUpgrade(featureStore string) (*RollingUpgrade, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	upgrade, exists := m.upgrades[featureStore]
	if !exists {
		return nil, fmt.Errorf("no upgrade in progress for %s", featureStore)
	}
	upgrade.Phase = UpgradeRolledBack
	now := time.Now()
	upgrade.CompletedAt = &now
	result := *upgrade
	m.history = append(m.history, result)
	delete(m.upgrades, featureStore)
	return &result, nil
}

// GetUpgrade returns the current upgrade for a FeatureStore.
func (m *UpgradeManager) GetUpgrade(featureStore string) (*RollingUpgrade, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	upgrade, exists := m.upgrades[featureStore]
	if !exists {
		return nil, fmt.Errorf("no upgrade in progress for %s", featureStore)
	}
	result := *upgrade
	return &result, nil
}

// ListHistory returns completed upgrade history.
func (m *UpgradeManager) ListHistory() []RollingUpgrade {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]RollingUpgrade, len(m.history))
	copy(result, m.history)
	return result
}
