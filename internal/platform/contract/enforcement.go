package contract

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/feather-store/feather/internal/core/domain"
)

// Enforcer provides runtime contract enforcement on storage writes and aggregation operations.
// It intercepts Put() calls and validates feature values against registered contracts.
type Enforcer struct {
	mu        sync.RWMutex
	manager   *Manager
	mode      EnforcementMode
	stats     EnforcementStats
	listeners []EnforcementListener
}

// EnforcementStats tracks enforcement activity.
type EnforcementStats struct {
	TotalChecks    int64     `json:"total_checks"`
	Passed         int64     `json:"passed"`
	Warned         int64     `json:"warned"`
	Blocked        int64     `json:"blocked"`
	Errors         int64     `json:"errors"`
	LastCheckAt    time.Time `json:"last_check_at"`
	LastViolatedAt time.Time `json:"last_violated_at,omitempty"`
}

// EnforcementListener receives notifications about enforcement decisions.
type EnforcementListener interface {
	OnEnforcementDecision(ctx context.Context, decision EnforcementDecision)
}

// EnforcementDecision describes the result of enforcing a contract on a write.
type EnforcementDecision struct {
	EntityKey    string       `json:"entity_key"`
	FeatureName  string       `json:"feature_name"`
	ContractName string       `json:"contract_name"`
	RuleType     RuleType     `json:"rule_type"`
	Action       string       `json:"action"` // "allow", "warn", "block"
	Message      string       `json:"message,omitempty"`
	Timestamp    time.Time    `json:"timestamp"`
}

// EnforcerConfig configures the enforcement engine.
type EnforcerConfig struct {
	DefaultMode EnforcementMode `json:"default_mode" yaml:"default_mode"`
}

// DefaultEnforcerConfig returns sensible defaults.
func DefaultEnforcerConfig() EnforcerConfig {
	return EnforcerConfig{
		DefaultMode: ModeWarn,
	}
}

// NewEnforcer creates a new enforcement engine.
func NewEnforcer(manager *Manager, cfg EnforcerConfig) *Enforcer {
	if cfg.DefaultMode == "" {
		cfg.DefaultMode = ModeWarn
	}
	return &Enforcer{
		manager: manager,
		mode:    cfg.DefaultMode,
	}
}

// AddListener registers a listener for enforcement decisions.
func (e *Enforcer) AddListener(l EnforcementListener) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.listeners = append(e.listeners, l)
}

// Stats returns current enforcement statistics.
func (e *Enforcer) Stats() EnforcementStats {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.stats
}

// ValidatePut checks feature values against contracts before storage.
// Returns nil if the write is allowed, or an error if blocked (enforce mode only).
func (e *Enforcer) ValidatePut(ctx context.Context, entityKey string, features map[string]*domain.FeatureValue) error {
	if e.manager == nil {
		return nil
	}

	contracts := e.manager.ListContracts()
	if len(contracts) == 0 {
		return nil
	}

	e.mu.Lock()
	e.stats.TotalChecks++
	e.stats.LastCheckAt = time.Now()
	e.mu.Unlock()

	var violations []EnforcementDecision
	now := time.Now()

	for _, spec := range contracts {
		for featureName, fv := range features {
			if spec.FeatureName != "" && spec.FeatureName != featureName {
				continue
			}

			for _, rule := range spec.Rules {
				if decision := e.checkRule(entityKey, featureName, spec, rule, fv, now); decision != nil {
					violations = append(violations, *decision)
				}
			}
		}
	}

	if len(violations) == 0 {
		e.mu.Lock()
		e.stats.Passed++
		e.mu.Unlock()
		return nil
	}

	// Notify listeners
	e.mu.RLock()
	listeners := make([]EnforcementListener, len(e.listeners))
	copy(listeners, e.listeners)
	e.mu.RUnlock()

	for _, d := range violations {
		for _, l := range listeners {
			l.OnEnforcementDecision(ctx, d)
		}
	}

	// Check if any violations should block the write
	for _, d := range violations {
		if d.Action == "block" {
			e.mu.Lock()
			e.stats.Blocked++
			e.stats.LastViolatedAt = now
			e.mu.Unlock()
			return fmt.Errorf("contract %q violated: %s", d.ContractName, d.Message)
		}
	}

	e.mu.Lock()
	e.stats.Warned++
	e.stats.LastViolatedAt = now
	e.mu.Unlock()
	return nil
}

func (e *Enforcer) checkRule(entityKey, featureName string, spec *Spec, rule Rule, fv *domain.FeatureValue, now time.Time) *EnforcementDecision {
	action := "warn"
	if e.mode == ModeEnforce {
		action = "block"
	}
	if e.mode == ModeAudit {
		action = "allow"
	}

	switch rule.Type {
	case RuleSchema:
		if rule.ExpectedType != "" && fv != nil {
			actualType := inferType(fv.Value)
			if actualType != rule.ExpectedType {
				return &EnforcementDecision{
					EntityKey:    entityKey,
					FeatureName:  featureName,
					ContractName: spec.Name,
					RuleType:     RuleSchema,
					Action:       action,
					Message:      fmt.Sprintf("expected type %s, got %s", rule.ExpectedType, actualType),
					Timestamp:    now,
				}
			}
		}
	case RuleDistribution:
		if fv != nil {
			val, ok := toFloat64(fv.Value)
			if ok {
				if rule.MinValue != nil && val < *rule.MinValue {
					return &EnforcementDecision{
						EntityKey:    entityKey,
						FeatureName:  featureName,
						ContractName: spec.Name,
						RuleType:     RuleDistribution,
						Action:       action,
						Message:      fmt.Sprintf("value %.4f below minimum %.4f", val, *rule.MinValue),
						Timestamp:    now,
					}
				}
				if rule.MaxValue != nil && val > *rule.MaxValue {
					return &EnforcementDecision{
						EntityKey:    entityKey,
						FeatureName:  featureName,
						ContractName: spec.Name,
						RuleType:     RuleDistribution,
						Action:       action,
						Message:      fmt.Sprintf("value %.4f above maximum %.4f", val, *rule.MaxValue),
						Timestamp:    now,
					}
				}
			}
		}
	case RuleCompleteness:
		if fv == nil || fv.Value == nil {
			return &EnforcementDecision{
				EntityKey:    entityKey,
				FeatureName:  featureName,
				ContractName: spec.Name,
				RuleType:     RuleCompleteness,
				Action:       action,
				Message:      "null value violates completeness constraint",
				Timestamp:    now,
			}
		}
	}
	return nil
}

func inferType(v interface{}) string {
	switch v.(type) {
	case int, int32, int64:
		return "int"
	case float32, float64:
		return "float"
	case string:
		return "string"
	case bool:
		return "bool"
	case []float64, []float32:
		return "vector"
	default:
		return "unknown"
	}
}

func toFloat64(v interface{}) (float64, bool) {
	switch val := v.(type) {
	case float64:
		return val, true
	case float32:
		return float64(val), true
	case int:
		return float64(val), true
	case int32:
		return float64(val), true
	case int64:
		return float64(val), true
	default:
		return 0, false
	}
}
