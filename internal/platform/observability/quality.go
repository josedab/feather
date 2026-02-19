package observability

import (
	"context"
	"sync"
	"time"

	"github.com/feather-store/feather/internal/core/domain"
	"github.com/feather-store/feather/internal/core/storage"
)

// QualityScore represents quality metrics for a feature.
type QualityScore struct {
	Feature        string    `json:"feature"`
	Completeness   float64   `json:"completeness"`  // % of entities with this feature
	Freshness      float64   `json:"freshness"`     // % within TTL
	Consistency    float64   `json:"consistency"`   // % matching expected schema
	Accuracy       float64   `json:"accuracy"`      // Based on validation rules
	OverallScore   float64   `json:"overall_score"` // Weighted average
	LastCalculated time.Time `json:"last_calculated"`
	SampleSize     int       `json:"sample_size"`
}

// QualityRule defines a validation rule for feature quality.
type QualityRule struct {
	Name     string                 `json:"name"`
	Feature  string                 `json:"feature"`
	RuleType string                 `json:"rule_type"` // range, regex, enum, null_check, custom
	Config   map[string]interface{} `json:"config"`
	Severity string                 `json:"severity"` // warning, error, critical
	Enabled  bool                   `json:"enabled"`
}

// QualityViolation represents a quality rule violation.
type QualityViolation struct {
	Rule      string      `json:"rule"`
	Feature   string      `json:"feature"`
	EntityID  string      `json:"entity_id"`
	Value     interface{} `json:"value"`
	Expected  string      `json:"expected"`
	Severity  string      `json:"severity"`
	Timestamp time.Time   `json:"timestamp"`
}

// QualityMonitor monitors feature quality.
type QualityMonitor struct {
	store      *storage.Store
	rules      map[string][]*QualityRule // feature -> rules
	violations []*QualityViolation
	scores     map[string]*QualityScore
	mu         sync.RWMutex
}

// NewQualityMonitor creates a new quality monitor.
func NewQualityMonitor(store *storage.Store) *QualityMonitor {
	return &QualityMonitor{
		store:      store,
		rules:      make(map[string][]*QualityRule),
		violations: make([]*QualityViolation, 0),
		scores:     make(map[string]*QualityScore),
	}
}

// AddRule adds a quality rule.
func (m *QualityMonitor) AddRule(rule *QualityRule) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.rules[rule.Feature] == nil {
		m.rules[rule.Feature] = make([]*QualityRule, 0)
	}
	m.rules[rule.Feature] = append(m.rules[rule.Feature], rule)
}

// RemoveRule removes a quality rule.
func (m *QualityMonitor) RemoveRule(ruleName string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for feature, rules := range m.rules {
		filtered := make([]*QualityRule, 0)
		for _, r := range rules {
			if r.Name != ruleName {
				filtered = append(filtered, r)
			}
		}
		m.rules[feature] = filtered
	}
}

// ValidateValue validates a feature value against rules.
func (m *QualityMonitor) ValidateValue(ctx context.Context, feature string, entityID string, value interface{}) []*QualityViolation {
	m.mu.RLock()
	rules := m.rules[feature]
	m.mu.RUnlock()

	if len(rules) == 0 {
		return nil
	}

	violations := make([]*QualityViolation, 0)

	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}

		violation := m.checkRule(rule, entityID, value)
		if violation != nil {
			violations = append(violations, violation)
		}
	}

	if len(violations) > 0 {
		m.mu.Lock()
		m.violations = append(m.violations, violations...)
		// Keep last 10000 violations
		if len(m.violations) > 10000 {
			m.violations = m.violations[len(m.violations)-10000:]
		}
		m.mu.Unlock()
	}

	return violations
}

func (m *QualityMonitor) checkRule(rule *QualityRule, entityID string, value interface{}) *QualityViolation {
	switch rule.RuleType {
	case "null_check":
		if value == nil {
			return &QualityViolation{
				Rule:      rule.Name,
				Feature:   rule.Feature,
				EntityID:  entityID,
				Value:     value,
				Expected:  "non-null value",
				Severity:  rule.Severity,
				Timestamp: time.Now(),
			}
		}

	case "range":
		minValue, hasMin := rule.Config["min"].(float64)
		maxValue, hasMax := rule.Config["max"].(float64)

		var numVal float64
		switch v := value.(type) {
		case float64:
			numVal = v
		case int:
			numVal = float64(v)
		case int64:
			numVal = float64(v)
		default:
			return nil
		}

		if hasMin && numVal < minValue {
			return &QualityViolation{
				Rule:      rule.Name,
				Feature:   rule.Feature,
				EntityID:  entityID,
				Value:     value,
				Expected:  "value >= min",
				Severity:  rule.Severity,
				Timestamp: time.Now(),
			}
		}
		if hasMax && numVal > maxValue {
			return &QualityViolation{
				Rule:      rule.Name,
				Feature:   rule.Feature,
				EntityID:  entityID,
				Value:     value,
				Expected:  "value <= max",
				Severity:  rule.Severity,
				Timestamp: time.Now(),
			}
		}

	case "enum":
		allowed, ok := rule.Config["values"].([]interface{})
		if !ok {
			return nil
		}

		found := false
		for _, a := range allowed {
			if a == value {
				found = true
				break
			}
		}

		if !found {
			return &QualityViolation{
				Rule:      rule.Name,
				Feature:   rule.Feature,
				EntityID:  entityID,
				Value:     value,
				Expected:  "value in allowed set",
				Severity:  rule.Severity,
				Timestamp: time.Now(),
			}
		}

	case "type_check":
		expectedType, ok := rule.Config["type"].(string)
		if !ok {
			return nil
		}

		var actualType string
		switch value.(type) {
		case float64:
			actualType = "float"
		case int, int64:
			actualType = "int"
		case string:
			actualType = "string"
		case bool:
			actualType = "bool"
		case []interface{}:
			actualType = "array"
		case map[string]interface{}:
			actualType = "object"
		}

		if actualType != expectedType {
			return &QualityViolation{
				Rule:      rule.Name,
				Feature:   rule.Feature,
				EntityID:  entityID,
				Value:     value,
				Expected:  "type: " + expectedType,
				Severity:  rule.Severity,
				Timestamp: time.Now(),
			}
		}
	}

	return nil
}

// GetViolations returns recent violations.
func (m *QualityMonitor) GetViolations(feature string, since time.Time) []*QualityViolation {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*QualityViolation, 0)
	for _, v := range m.violations {
		if (feature == "" || v.Feature == feature) && v.Timestamp.After(since) {
			result = append(result, v)
		}
	}

	return result
}

// CalculateScore calculates quality score for a feature.
func (m *QualityMonitor) CalculateScore(ctx context.Context, feature string, sampleEntities []string) *QualityScore {
	if len(sampleEntities) == 0 {
		return nil
	}

	var (
		complete    int
		fresh       int
		consistent  int
		accurate    int
		totalChecks int
	)

	// Check each entity
	for _, entityID := range sampleEntities {
		values, err := m.store.Get(ctx, entityID, []string{feature})
		if err != nil {
			continue
		}

		totalChecks++

		fv, ok := values[feature]
		if !ok || fv == nil {
			continue
		}

		complete++

		// Check freshness (within last hour as default)
		ts := time.Unix(0, fv.Timestamp)
		if time.Since(ts) < time.Hour {
			fresh++
		}

		// Check consistency (value not nil and has expected type)
		if fv.Value != nil {
			consistent++
		}

		// Check accuracy against rules
		violations := m.ValidateValue(ctx, feature, entityID, fv.Value)
		if len(violations) == 0 {
			accurate++
		}
	}

	if totalChecks == 0 {
		return nil
	}

	score := &QualityScore{
		Feature:        feature,
		Completeness:   float64(complete) / float64(totalChecks) * 100,
		Freshness:      float64(fresh) / float64(totalChecks) * 100,
		Consistency:    float64(consistent) / float64(totalChecks) * 100,
		Accuracy:       float64(accurate) / float64(totalChecks) * 100,
		LastCalculated: time.Now(),
		SampleSize:     totalChecks,
	}

	// Calculate overall score (weighted average)
	score.OverallScore = (score.Completeness*0.3 +
		score.Freshness*0.3 +
		score.Consistency*0.2 +
		score.Accuracy*0.2)

	m.mu.Lock()
	m.scores[feature] = score
	m.mu.Unlock()

	return score
}

// GetScore returns the cached quality score for a feature.
func (m *QualityMonitor) GetScore(feature string) *QualityScore {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.scores[feature]
}

// GetAllScores returns all cached quality scores.
func (m *QualityMonitor) GetAllScores() []*QualityScore {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*QualityScore, 0, len(m.scores))
	for _, s := range m.scores {
		result = append(result, s)
	}
	return result
}

// GetRules returns rules for a feature.
func (m *QualityMonitor) GetRules(feature string) []*QualityRule {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if rules, ok := m.rules[feature]; ok {
		result := make([]*QualityRule, len(rules))
		copy(result, rules)
		return result
	}
	return nil
}

// FeatureProfile contains statistical profile of a feature.
type FeatureProfile struct {
	Feature      string          `json:"feature"`
	DataType     domain.DataType `json:"data_type"`
	NullCount    int             `json:"null_count"`
	UniqueCount  int             `json:"unique_count"`
	SampleSize   int             `json:"sample_size"`
	NumericStats *NumericStats   `json:"numeric_stats,omitempty"`
	StringStats  *StringStats    `json:"string_stats,omitempty"`
	CreatedAt    time.Time       `json:"created_at"`
}

// NumericStats contains statistics for numeric features.
type NumericStats struct {
	Min    float64 `json:"min"`
	Max    float64 `json:"max"`
	Mean   float64 `json:"mean"`
	Median float64 `json:"median"`
	StdDev float64 `json:"std_dev"`
	P25    float64 `json:"p25"`
	P75    float64 `json:"p75"`
	P95    float64 `json:"p95"`
}

// StringStats contains statistics for string features.
type StringStats struct {
	MinLength int            `json:"min_length"`
	MaxLength int            `json:"max_length"`
	AvgLength float64        `json:"avg_length"`
	TopValues map[string]int `json:"top_values"`
}
