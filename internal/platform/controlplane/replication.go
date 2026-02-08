package controlplane

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ConflictPolicy defines how replication conflicts are resolved.
type ConflictPolicy string

const (
	ConflictLastWrite ConflictPolicy = "last_write_wins"
	ConflictPrimary   ConflictPolicy = "primary_wins"
	ConflictHigher    ConflictPolicy = "higher_version_wins"
)

// ReplicationConfig holds configuration for the ReplicationManager.
type ReplicationConfig struct {
	MaxLagDuration time.Duration  `json:"max_lag_duration"`
	BatchSize      int            `json:"batch_size"`
	RetryAttempts  int            `json:"retry_attempts"`
	ConflictPolicy ConflictPolicy `json:"conflict_policy"`
}

// DefaultReplicationConfig returns a ReplicationConfig with sensible defaults.
func DefaultReplicationConfig() ReplicationConfig {
	return ReplicationConfig{
		MaxLagDuration: 5 * time.Second,
		BatchSize:      1000,
		RetryAttempts:  3,
		ConflictPolicy: ConflictLastWrite,
	}
}

// ReplicationRule defines a replication relationship between two regions.
type ReplicationRule struct {
	ID           string          `json:"id"`
	SourceRegion string          `json:"source_region"`
	TargetRegion string          `json:"target_region"`
	Features     []string        `json:"features,omitempty"`
	Mode         ReplicationMode `json:"mode"`
	Enabled      bool            `json:"enabled"`
}

// ReplicationStatus tracks the state of a single replication rule.
type ReplicationStatus struct {
	RuleID        string        `json:"rule_id"`
	LastSync      time.Time     `json:"last_sync"`
	Lag           time.Duration `json:"lag"`
	RecordsSynced int64         `json:"records_synced"`
	RecordsFailed int64         `json:"records_failed"`
	State         string        `json:"state"` // syncing, idle, error
}

// ReplicationManager handles cross-region data replication.
type ReplicationManager struct {
	config ReplicationConfig
	rules  map[string]*ReplicationRule
	status map[string]*ReplicationStatus
	mu     sync.RWMutex
}

// NewReplicationManager creates a new ReplicationManager.
func NewReplicationManager(config ReplicationConfig) *ReplicationManager {
	return &ReplicationManager{
		config: config,
		rules:  make(map[string]*ReplicationRule),
		status: make(map[string]*ReplicationStatus),
	}
}

// AddRule registers a new replication rule. An ID is assigned if not set.
func (rm *ReplicationManager) AddRule(ctx context.Context, rule *ReplicationRule) error {
	if rule == nil {
		return errors.New("replication rule must not be nil")
	}
	if rule.SourceRegion == "" {
		return errors.New("source region is required")
	}
	if rule.TargetRegion == "" {
		return errors.New("target region is required")
	}
	if rule.SourceRegion == rule.TargetRegion {
		return fmt.Errorf("adding replication rule: source and target regions must differ")
	}

	rm.mu.Lock()
	defer rm.mu.Unlock()

	if rule.ID == "" {
		rule.ID = uuid.New().String()
	}

	rm.rules[rule.ID] = rule
	rm.status[rule.ID] = &ReplicationStatus{
		RuleID: rule.ID,
		State:  "idle",
	}

	return nil
}

// RemoveRule removes a replication rule by ID.
func (rm *ReplicationManager) RemoveRule(ctx context.Context, id string) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if _, ok := rm.rules[id]; !ok {
		return fmt.Errorf("removing replication rule: rule %q not found", id)
	}

	delete(rm.rules, id)
	delete(rm.status, id)
	return nil
}

// GetRule returns a replication rule by ID.
func (rm *ReplicationManager) GetRule(ctx context.Context, id string) (*ReplicationRule, error) {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	rule, ok := rm.rules[id]
	if !ok {
		return nil, fmt.Errorf("getting replication rule: rule %q not found", id)
	}
	return rule, nil
}

// ListRules returns all replication rules.
func (rm *ReplicationManager) ListRules(ctx context.Context) []*ReplicationRule {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	result := make([]*ReplicationRule, 0, len(rm.rules))
	for _, r := range rm.rules {
		result = append(result, r)
	}
	return result
}

// GetStatus returns the replication status for a given rule.
func (rm *ReplicationManager) GetStatus(ctx context.Context, ruleID string) (*ReplicationStatus, error) {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	s, ok := rm.status[ruleID]
	if !ok {
		return nil, fmt.Errorf("getting replication status: rule %q not found", ruleID)
	}
	return s, nil
}

// ListStatuses returns the replication status for all rules.
func (rm *ReplicationManager) ListStatuses(ctx context.Context) []*ReplicationStatus {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	result := make([]*ReplicationStatus, 0, len(rm.status))
	for _, s := range rm.status {
		result = append(result, s)
	}
	return result
}
