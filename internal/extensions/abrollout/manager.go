// Package abrollout provides feature versioning with canary deployments,
// traffic splitting, and automated rollback on drift/quality regression.
package abrollout

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"sync"
	"time"
)

// RolloutStatus represents the state of a feature rollout.
type RolloutStatus string

const (
	// RolloutStatusPending means the rollout has not started.
	RolloutStatusPending RolloutStatus = "pending"
	// RolloutStatusCanary means the rollout is in canary phase.
	RolloutStatusCanary RolloutStatus = "canary"
	// RolloutStatusProgressing means the rollout is ramping up.
	RolloutStatusProgressing RolloutStatus = "progressing"
	// RolloutStatusComplete means the rollout is at 100%.
	RolloutStatusComplete RolloutStatus = "complete"
	// RolloutStatusRolledBack means the rollout was reverted.
	RolloutStatusRolledBack RolloutStatus = "rolled_back"
	// RolloutStatusPaused means the rollout was paused.
	RolloutStatusPaused RolloutStatus = "paused"
)

// FeatureVersion represents a specific version of a feature definition.
type FeatureVersion struct {
	ID          string            `json:"id"`
	FeatureName string            `json:"feature_name"`
	Version     int               `json:"version"`
	Expression  string            `json:"expression"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	CreatedBy   string            `json:"created_by"`
}

// Rollout represents a gradual rollout of a feature version.
type Rollout struct {
	ID             string        `json:"id"`
	FeatureName    string        `json:"feature_name"`
	BaseVersion    int           `json:"base_version"`
	CanaryVersion  int           `json:"canary_version"`
	TrafficPct     float64       `json:"traffic_pct"`     // 0-100% sent to canary
	Status         RolloutStatus `json:"status"`
	Steps          []RolloutStep `json:"steps"`
	CurrentStep    int           `json:"current_step"`
	AutoPromote    bool          `json:"auto_promote"`
	AutoRollback   bool          `json:"auto_rollback"`
	QualityGate    *QualityGate  `json:"quality_gate,omitempty"`
	Metrics        *RolloutMetrics `json:"metrics,omitempty"`
	CreatedAt      time.Time     `json:"created_at"`
	UpdatedAt      time.Time     `json:"updated_at"`
	CompletedAt    time.Time     `json:"completed_at,omitempty"`
}

// RolloutStep defines a traffic increase step.
type RolloutStep struct {
	TrafficPct float64       `json:"traffic_pct"`
	Duration   time.Duration `json:"duration"`
}

// QualityGate defines thresholds for automatic promotion/rollback.
type QualityGate struct {
	MaxDriftScore    float64 `json:"max_drift_score"`
	MaxErrorRate     float64 `json:"max_error_rate"`
	MaxLatencyMs     float64 `json:"max_latency_ms"`
	MinDataQuality   float64 `json:"min_data_quality"`
}

// DefaultQualityGate returns a reasonable quality gate.
func DefaultQualityGate() *QualityGate {
	return &QualityGate{
		MaxDriftScore:  0.1,
		MaxErrorRate:   0.05,
		MaxLatencyMs:   100,
		MinDataQuality: 0.95,
	}
}

// RolloutMetrics tracks per-version performance.
type RolloutMetrics struct {
	BaseRequests   int64   `json:"base_requests"`
	CanaryRequests int64   `json:"canary_requests"`
	BaseErrorRate  float64 `json:"base_error_rate"`
	CanaryErrorRate float64 `json:"canary_error_rate"`
	BaseLatencyMs  float64 `json:"base_latency_ms"`
	CanaryLatencyMs float64 `json:"canary_latency_ms"`
	BaseDrift      float64 `json:"base_drift"`
	CanaryDrift    float64 `json:"canary_drift"`
}

// DefaultRolloutSteps returns a standard canary rollout sequence.
func DefaultRolloutSteps() []RolloutStep {
	return []RolloutStep{
		{TrafficPct: 1, Duration: 5 * time.Minute},
		{TrafficPct: 5, Duration: 10 * time.Minute},
		{TrafficPct: 25, Duration: 30 * time.Minute},
		{TrafficPct: 50, Duration: 1 * time.Hour},
		{TrafficPct: 100, Duration: 0},
	}
}

// Manager manages feature versions and rollouts.
type Manager struct {
	versions map[string][]*FeatureVersion // featureName -> versions
	rollouts map[string]*Rollout          // rolloutID -> rollout
	active   map[string]*Rollout          // featureName -> active rollout
	mu       sync.RWMutex
}

// NewManager creates a new rollout manager.
func NewManager() *Manager {
	return &Manager{
		versions: make(map[string][]*FeatureVersion),
		rollouts: make(map[string]*Rollout),
		active:   make(map[string]*Rollout),
	}
}

// CreateVersion registers a new version of a feature.
func (m *Manager) CreateVersion(featureName, expression, createdBy string) (*FeatureVersion, error) {
	if featureName == "" {
		return nil, fmt.Errorf("feature name is required")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	versions := m.versions[featureName]
	nextVersion := len(versions) + 1

	fv := &FeatureVersion{
		ID:          fmt.Sprintf("%s-v%d", featureName, nextVersion),
		FeatureName: featureName,
		Version:     nextVersion,
		Expression:  expression,
		CreatedAt:   time.Now(),
		CreatedBy:   createdBy,
		Metadata:    make(map[string]string),
	}

	m.versions[featureName] = append(versions, fv)
	return fv, nil
}

// GetVersion retrieves a specific version.
func (m *Manager) GetVersion(featureName string, version int) (*FeatureVersion, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	versions := m.versions[featureName]
	for _, v := range versions {
		if v.Version == version {
			return v, nil
		}
	}
	return nil, fmt.Errorf("version %d of %q not found", version, featureName)
}

// ListVersions returns all versions of a feature.
func (m *Manager) ListVersions(featureName string) []*FeatureVersion {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*FeatureVersion, len(m.versions[featureName]))
	copy(result, m.versions[featureName])
	return result
}

// StartRollout begins a canary rollout from base to canary version.
func (m *Manager) StartRollout(featureName string, baseVersion, canaryVersion int, steps []RolloutStep, autoPromote, autoRollback bool) (*Rollout, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Validate versions exist
	if !m.hasVersion(featureName, baseVersion) {
		return nil, fmt.Errorf("base version %d not found", baseVersion)
	}
	if !m.hasVersion(featureName, canaryVersion) {
		return nil, fmt.Errorf("canary version %d not found", canaryVersion)
	}

	// Check no active rollout
	if _, exists := m.active[featureName]; exists {
		return nil, fmt.Errorf("feature %q already has an active rollout", featureName)
	}

	if len(steps) == 0 {
		steps = DefaultRolloutSteps()
	}

	now := time.Now()
	rollout := &Rollout{
		ID:            fmt.Sprintf("rollout-%s-%d", featureName, now.UnixNano()),
		FeatureName:   featureName,
		BaseVersion:   baseVersion,
		CanaryVersion: canaryVersion,
		TrafficPct:    steps[0].TrafficPct,
		Status:        RolloutStatusCanary,
		Steps:         steps,
		CurrentStep:   0,
		AutoPromote:   autoPromote,
		AutoRollback:  autoRollback,
		QualityGate:   DefaultQualityGate(),
		Metrics:       &RolloutMetrics{},
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	m.rollouts[rollout.ID] = rollout
	m.active[featureName] = rollout
	return rollout, nil
}

// Advance moves the rollout to the next traffic step.
func (m *Manager) Advance(rolloutID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	rollout, ok := m.rollouts[rolloutID]
	if !ok {
		return fmt.Errorf("rollout %q not found", rolloutID)
	}

	if rollout.Status != RolloutStatusCanary && rollout.Status != RolloutStatusProgressing {
		return fmt.Errorf("rollout %q is not in a progressable state (status: %s)", rolloutID, rollout.Status)
	}

	rollout.CurrentStep++
	if rollout.CurrentStep >= len(rollout.Steps) {
		rollout.Status = RolloutStatusComplete
		rollout.TrafficPct = 100
		rollout.CompletedAt = time.Now()
		delete(m.active, rollout.FeatureName)
	} else {
		rollout.TrafficPct = rollout.Steps[rollout.CurrentStep].TrafficPct
		rollout.Status = RolloutStatusProgressing
	}
	rollout.UpdatedAt = time.Now()
	return nil
}

// Rollback reverts a rollout to the base version.
func (m *Manager) Rollback(rolloutID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	rollout, ok := m.rollouts[rolloutID]
	if !ok {
		return fmt.Errorf("rollout %q not found", rolloutID)
	}

	rollout.Status = RolloutStatusRolledBack
	rollout.TrafficPct = 0
	rollout.CompletedAt = time.Now()
	rollout.UpdatedAt = time.Now()
	delete(m.active, rollout.FeatureName)
	return nil
}

// Pause pauses an active rollout.
func (m *Manager) Pause(rolloutID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	rollout, ok := m.rollouts[rolloutID]
	if !ok {
		return fmt.Errorf("rollout %q not found", rolloutID)
	}

	rollout.Status = RolloutStatusPaused
	rollout.UpdatedAt = time.Now()
	return nil
}

// UpdateMetrics records per-version metrics for a rollout.
func (m *Manager) UpdateMetrics(rolloutID string, metrics *RolloutMetrics) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	rollout, ok := m.rollouts[rolloutID]
	if !ok {
		return fmt.Errorf("rollout %q not found", rolloutID)
	}

	rollout.Metrics = metrics
	rollout.UpdatedAt = time.Now()
	return nil
}

// EvaluateQualityGates checks if the canary passes quality thresholds.
// Returns true if healthy, false if rollback needed.
func (m *Manager) EvaluateQualityGates(rolloutID string) (bool, string) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rollout, ok := m.rollouts[rolloutID]
	if !ok || rollout.Metrics == nil || rollout.QualityGate == nil {
		return true, "no metrics or gate"
	}

	gate := rollout.QualityGate
	metrics := rollout.Metrics

	if metrics.CanaryErrorRate > gate.MaxErrorRate {
		return false, fmt.Sprintf("canary error rate %.2f%% exceeds threshold %.2f%%",
			metrics.CanaryErrorRate*100, gate.MaxErrorRate*100)
	}

	if metrics.CanaryLatencyMs > gate.MaxLatencyMs {
		return false, fmt.Sprintf("canary latency %.1fms exceeds threshold %.1fms",
			metrics.CanaryLatencyMs, gate.MaxLatencyMs)
	}

	if metrics.CanaryDrift > gate.MaxDriftScore {
		return false, fmt.Sprintf("canary drift %.3f exceeds threshold %.3f",
			metrics.CanaryDrift, gate.MaxDriftScore)
	}

	return true, "all gates passed"
}

// ResolveVersion determines which version to serve for a given entity.
// Uses consistent hashing to ensure the same entity always gets the same version.
func (m *Manager) ResolveVersion(featureName, entityKey string) int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rollout, ok := m.active[featureName]
	if !ok {
		// No active rollout; return latest version
		versions := m.versions[featureName]
		if len(versions) == 0 {
			return 0
		}
		return versions[len(versions)-1].Version
	}

	// Hash entity to [0, 100) deterministically
	bucket := hashToBucket(entityKey, featureName)
	if bucket < rollout.TrafficPct {
		return rollout.CanaryVersion
	}
	return rollout.BaseVersion
}

// GetActiveRollout returns the active rollout for a feature.
func (m *Manager) GetActiveRollout(featureName string) *Rollout {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.active[featureName]
}

// ListRollouts returns all rollouts.
func (m *Manager) ListRollouts() []*Rollout {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Rollout, 0, len(m.rollouts))
	for _, r := range m.rollouts {
		result = append(result, r)
	}
	return result
}

// Stats returns rollout statistics.
func (m *Manager) Stats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return map[string]interface{}{
		"total_features": len(m.versions),
		"total_rollouts": len(m.rollouts),
		"active_rollouts": len(m.active),
	}
}

func (m *Manager) hasVersion(featureName string, version int) bool {
	for _, v := range m.versions[featureName] {
		if v.Version == version {
			return true
		}
	}
	return false
}

func hashToBucket(entityKey, featureName string) float64 {
	h := sha256.New()
	h.Write([]byte(entityKey))
	h.Write([]byte{0})
	h.Write([]byte(featureName))
	sum := h.Sum(nil)
	val := binary.BigEndian.Uint64(sum[:8])
	return float64(val%10000) / 100.0 // 0.00 - 99.99
}
