package quality

import (
	"context"
	"fmt"
	"regexp"
	"sync"
	"time"
)

// RuleType defines the type of validation rule.
type RuleType string

// RuleType constants for validation rules.
const (
	RuleTypeNotNull      RuleType = "not_null"
	RuleTypeUnique       RuleType = "unique"
	RuleTypeRange        RuleType = "range"
	RuleTypePattern      RuleType = "pattern"
	RuleTypeEnum         RuleType = "enum"
	RuleTypeCustom       RuleType = "custom"
	RuleTypeFreshness    RuleType = "freshness"
	RuleTypeCompleteness RuleType = "completeness"
	RuleTypeConsistency  RuleType = "consistency"
	RuleTypeAccuracy     RuleType = "accuracy"
)

// Severity defines the severity of a validation failure.
type Severity string

// Severity constants for validation outcomes.
const (
	SeverityInfo     Severity = "info"
	SeverityWarning  Severity = "warning"
	SeverityError    Severity = "error"
	SeverityCritical Severity = "critical"
)

// ValidationRule defines a data quality rule.
type ValidationRule struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Type        RuleType               `json:"type"`
	FeatureID   string                 `json:"feature_id"`
	GroupID     string                 `json:"group_id,omitempty"`
	Severity    Severity               `json:"severity"`
	Config      map[string]interface{} `json:"config,omitempty"`
	Enabled     bool                   `json:"enabled"`
	Tags        []string               `json:"tags,omitempty"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
}

// RangeConfig configures range validation.
type RangeConfig struct {
	Min          *float64 `json:"min,omitempty"`
	Max          *float64 `json:"max,omitempty"`
	MinInclusive bool     `json:"min_inclusive"`
	MaxInclusive bool     `json:"max_inclusive"`
}

// PatternConfig configures pattern validation.
type PatternConfig struct {
	Regex       string `json:"regex"`
	Description string `json:"description,omitempty"`
}

// EnumConfig configures enum validation.
type EnumConfig struct {
	AllowedValues []interface{} `json:"allowed_values"`
	CaseSensitive bool          `json:"case_sensitive"`
}

// FreshnessConfig configures freshness validation.
type FreshnessConfig struct {
	MaxAge     string `json:"max_age"`
	Timezone   string `json:"timezone,omitempty"`
	CheckField string `json:"check_field,omitempty"`
}

// CompletenessConfig configures completeness validation.
type CompletenessConfig struct {
	MinCompleteness float64  `json:"min_completeness"`
	Fields          []string `json:"fields,omitempty"`
}

// ConsistencyConfig configures consistency validation.
type ConsistencyConfig struct {
	ReferenceField string `json:"reference_field"`
	Comparator     string `json:"comparator"` // eq, neq, gt, lt, gte, lte
	CrossCheck     bool   `json:"cross_check,omitempty"`
}

// CustomRuleConfig configures custom validation.
type CustomRuleConfig struct {
	Expression string `json:"expression"`
	Language   string `json:"language,omitempty"` // cel, sql, js
}

// ValidationResult represents the result of a validation.
type ValidationResult struct {
	RuleID      string                 `json:"rule_id"`
	FeatureID   string                 `json:"feature_id"`
	Passed      bool                   `json:"passed"`
	Severity    Severity               `json:"severity"`
	Message     string                 `json:"message"`
	Details     map[string]interface{} `json:"details,omitempty"`
	SampleSize  int                    `json:"sample_size,omitempty"`
	FailureRate float64                `json:"failure_rate,omitempty"`
	Timestamp   time.Time              `json:"timestamp"`
}

// ValidationReport represents a complete validation report.
type ValidationReport struct {
	ID           string                 `json:"id"`
	Timestamp    time.Time              `json:"timestamp"`
	Duration     time.Duration          `json:"duration"`
	TotalRules   int                    `json:"total_rules"`
	PassedRules  int                    `json:"passed_rules"`
	FailedRules  int                    `json:"failed_rules"`
	SkippedRules int                    `json:"skipped_rules"`
	Results      []ValidationResult     `json:"results"`
	Summary      map[Severity]int       `json:"summary"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// QualityScore represents overall data quality metrics.
type QualityScore struct { //nolint:revive
	FeatureID      string             `json:"feature_id,omitempty"`
	GroupID        string             `json:"group_id,omitempty"`
	OverallScore   float64            `json:"overall_score"`
	Dimensions     map[string]float64 `json:"dimensions"`
	TrendDirection string             `json:"trend_direction"` // improving, degrading, stable
	LastUpdated    time.Time          `json:"last_updated"`
}

// Validator manages data quality validation.
type Validator struct {
	mu              sync.RWMutex
	rules           map[string]*ValidationRule
	rulesByFeature  map[string][]*ValidationRule
	rulesByGroup    map[string][]*ValidationRule
	history         []*ValidationReport
	qualityScores   map[string]*QualityScore
	compiledRegexes map[string]*regexp.Regexp
}

// NewValidator creates a new validator.
func NewValidator() *Validator {
	return &Validator{
		rules:           make(map[string]*ValidationRule),
		rulesByFeature:  make(map[string][]*ValidationRule),
		rulesByGroup:    make(map[string][]*ValidationRule),
		history:         make([]*ValidationReport, 0),
		qualityScores:   make(map[string]*QualityScore),
		compiledRegexes: make(map[string]*regexp.Regexp),
	}
}

// AddRule adds a validation rule.
func (v *Validator) AddRule(rule *ValidationRule) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	if rule.ID == "" {
		return fmt.Errorf("rule ID is required")
	}

	if rule.FeatureID == "" {
		return fmt.Errorf("feature ID is required")
	}

	// Compile regex if pattern rule
	if rule.Type == RuleTypePattern {
		pattern, ok := rule.Config["regex"].(string)
		if !ok {
			return fmt.Errorf("pattern rule requires regex config")
		}
		compiled, err := regexp.Compile(pattern)
		if err != nil {
			return fmt.Errorf("invalid regex pattern: %w", err)
		}
		v.compiledRegexes[rule.ID] = compiled
	}

	rule.CreatedAt = time.Now()
	rule.UpdatedAt = time.Now()
	rule.Enabled = true

	v.rules[rule.ID] = rule
	v.rulesByFeature[rule.FeatureID] = append(v.rulesByFeature[rule.FeatureID], rule)
	if rule.GroupID != "" {
		v.rulesByGroup[rule.GroupID] = append(v.rulesByGroup[rule.GroupID], rule)
	}

	return nil
}

// RemoveRule removes a validation rule.
func (v *Validator) RemoveRule(ruleID string) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	rule, exists := v.rules[ruleID]
	if !exists {
		return fmt.Errorf("rule not found")
	}

	delete(v.rules, ruleID)
	delete(v.compiledRegexes, ruleID)

	// Remove from feature index
	if rules, ok := v.rulesByFeature[rule.FeatureID]; ok {
		v.rulesByFeature[rule.FeatureID] = removeRule(rules, ruleID)
	}

	// Remove from group index
	if rule.GroupID != "" {
		if rules, ok := v.rulesByGroup[rule.GroupID]; ok {
			v.rulesByGroup[rule.GroupID] = removeRule(rules, ruleID)
		}
	}

	return nil
}

// GetRule returns a rule by ID.
func (v *Validator) GetRule(ruleID string) (*ValidationRule, error) {
	v.mu.RLock()
	defer v.mu.RUnlock()

	rule, exists := v.rules[ruleID]
	if !exists {
		return nil, fmt.Errorf("rule not found")
	}

	return rule, nil
}

// ListRules returns all rules.
func (v *Validator) ListRules() []*ValidationRule {
	v.mu.RLock()
	defer v.mu.RUnlock()

	rules := make([]*ValidationRule, 0, len(v.rules))
	for _, rule := range v.rules {
		rules = append(rules, rule)
	}
	return rules
}

// GetRulesForFeature returns rules for a feature.
func (v *Validator) GetRulesForFeature(featureID string) []*ValidationRule {
	v.mu.RLock()
	defer v.mu.RUnlock()

	rules, ok := v.rulesByFeature[featureID]
	if !ok {
		return []*ValidationRule{}
	}
	return rules
}

// ValidateValue validates a single value against rules.
func (v *Validator) ValidateValue(ctx context.Context, featureID string, value interface{}, metadata map[string]interface{}) []ValidationResult {
	v.mu.RLock()
	rules := v.rulesByFeature[featureID]
	v.mu.RUnlock()

	results := make([]ValidationResult, 0)

	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}

		result := v.applyRule(rule, value, metadata)
		results = append(results, result)
	}

	return results
}

// ValidateBatch validates a batch of values.
func (v *Validator) ValidateBatch(ctx context.Context, featureID string, values []interface{}, metadata map[string]interface{}) *ValidationReport {
	start := time.Now()

	v.mu.RLock()
	rules := v.rulesByFeature[featureID]
	v.mu.RUnlock()

	report := &ValidationReport{
		ID:        fmt.Sprintf("report-%d", time.Now().UnixNano()),
		Timestamp: start,
		Results:   make([]ValidationResult, 0),
		Summary:   make(map[Severity]int),
	}

	for _, rule := range rules {
		if !rule.Enabled {
			report.SkippedRules++
			continue
		}

		report.TotalRules++

		// Validate all values and aggregate results
		failures := 0
		for _, value := range values {
			result := v.applyRule(rule, value, metadata)
			if !result.Passed {
				failures++
			}
		}

		passed := failures == 0
		failureRate := 0.0
		if len(values) > 0 {
			failureRate = float64(failures) / float64(len(values))
		}

		result := ValidationResult{
			RuleID:      rule.ID,
			FeatureID:   featureID,
			Passed:      passed,
			Severity:    rule.Severity,
			SampleSize:  len(values),
			FailureRate: failureRate,
			Timestamp:   time.Now(),
		}

		if passed {
			report.PassedRules++
			result.Message = fmt.Sprintf("Rule %s passed", rule.Name)
		} else {
			report.FailedRules++
			result.Message = fmt.Sprintf("Rule %s failed with %.2f%% failure rate", rule.Name, failureRate*100)
			report.Summary[rule.Severity]++
		}

		report.Results = append(report.Results, result)
	}

	report.Duration = time.Since(start)

	// Store report in history
	v.mu.Lock()
	v.history = append(v.history, report)
	if len(v.history) > 1000 {
		v.history = v.history[1:]
	}
	v.mu.Unlock()

	return report
}

// applyRule applies a single rule to a value.
func (v *Validator) applyRule(rule *ValidationRule, value interface{}, metadata map[string]interface{}) ValidationResult {
	result := ValidationResult{
		RuleID:    rule.ID,
		FeatureID: rule.FeatureID,
		Severity:  rule.Severity,
		Timestamp: time.Now(),
		Details:   make(map[string]interface{}),
	}

	switch rule.Type {
	case RuleTypeNotNull:
		result.Passed = value != nil
		if !result.Passed {
			result.Message = "Value is null"
		}

	case RuleTypeRange:
		result.Passed, result.Message = v.validateRange(rule, value)

	case RuleTypePattern:
		result.Passed, result.Message = v.validatePattern(rule, value)

	case RuleTypeEnum:
		result.Passed, result.Message = v.validateEnum(rule, value)

	case RuleTypeFreshness:
		result.Passed, result.Message = v.validateFreshness(rule, metadata)

	case RuleTypeCompleteness:
		result.Passed, result.Message = v.validateCompleteness(rule, value, metadata)

	case RuleTypeConsistency:
		result.Passed, result.Message = v.validateConsistency(rule, value, metadata)

	case RuleTypeCustom:
		result.Passed, result.Message = v.validateCustom(rule, value, metadata)

	default:
		result.Passed = true
		result.Message = "Unknown rule type, skipped"
	}

	if result.Passed && result.Message == "" {
		result.Message = "Validation passed"
	}

	return result
}

func (v *Validator) validateRange(rule *ValidationRule, value interface{}) (bool, string) {
	num, ok := toFloat64(value)
	if !ok {
		return false, "Value is not numeric"
	}

	minVal, hasMin := rule.Config["min"]
	maxVal, hasMax := rule.Config["max"]
	minInclusive, _ := rule.Config["min_inclusive"].(bool)
	maxInclusive, _ := rule.Config["max_inclusive"].(bool)

	if hasMin {
		minNum, _ := toFloat64(minVal)
		if minInclusive {
			if num < minNum {
				return false, fmt.Sprintf("Value %v is less than minimum %v", value, minNum)
			}
		} else {
			if num <= minNum {
				return false, fmt.Sprintf("Value %v is less than or equal to minimum %v", value, minNum)
			}
		}
	}

	if hasMax {
		maxNum, _ := toFloat64(maxVal)
		if maxInclusive {
			if num > maxNum {
				return false, fmt.Sprintf("Value %v is greater than maximum %v", value, maxNum)
			}
		} else {
			if num >= maxNum {
				return false, fmt.Sprintf("Value %v is greater than or equal to maximum %v", value, maxNum)
			}
		}
	}

	return true, ""
}

func (v *Validator) validatePattern(rule *ValidationRule, value interface{}) (bool, string) {
	str := fmt.Sprintf("%v", value)

	regex, ok := v.compiledRegexes[rule.ID]
	if !ok {
		return false, "Pattern not compiled"
	}

	if !regex.MatchString(str) {
		return false, fmt.Sprintf("Value '%s' does not match pattern", str)
	}

	return true, ""
}

func (v *Validator) validateEnum(rule *ValidationRule, value interface{}) (bool, string) {
	allowedValues, ok := rule.Config["allowed_values"].([]interface{})
	if !ok {
		return false, "Invalid enum configuration"
	}

	caseSensitive, _ := rule.Config["case_sensitive"].(bool)
	valueStr := fmt.Sprintf("%v", value)

	for _, allowed := range allowedValues {
		allowedStr := fmt.Sprintf("%v", allowed)
		if caseSensitive {
			if valueStr == allowedStr {
				return true, ""
			}
		} else {
			if equalFold(valueStr, allowedStr) {
				return true, ""
			}
		}
	}

	return false, fmt.Sprintf("Value '%v' is not in allowed values", value)
}

func (v *Validator) validateFreshness(rule *ValidationRule, metadata map[string]interface{}) (bool, string) {
	maxAgeStr, ok := rule.Config["max_age"].(string)
	if !ok {
		return false, "Invalid freshness configuration"
	}

	maxAge, err := time.ParseDuration(maxAgeStr)
	if err != nil {
		return false, fmt.Sprintf("Invalid max_age duration: %v", err)
	}

	checkField := "timestamp"
	if field, hasField := rule.Config["check_field"].(string); hasField {
		checkField = field
	}

	timestampVal, ok := metadata[checkField]
	if !ok {
		return false, fmt.Sprintf("Timestamp field '%s' not found", checkField)
	}

	var timestamp time.Time
	switch t := timestampVal.(type) {
	case time.Time:
		timestamp = t
	case string:
		parsed, err := time.Parse(time.RFC3339, t)
		if err != nil {
			return false, fmt.Sprintf("Invalid timestamp format: %v", err)
		}
		timestamp = parsed
	case int64:
		timestamp = time.Unix(t, 0)
	default:
		return false, "Invalid timestamp type"
	}

	age := time.Since(timestamp)
	if age > maxAge {
		return false, fmt.Sprintf("Data is stale: age %v exceeds max age %v", age.Round(time.Second), maxAge)
	}

	return true, ""
}

func (v *Validator) validateCompleteness(rule *ValidationRule, value interface{}, metadata map[string]interface{}) (bool, string) {
	minCompleteness, ok := rule.Config["min_completeness"].(float64)
	if !ok {
		minCompleteness = 1.0
	}

	if value == nil {
		if minCompleteness > 0 {
			return false, "Value is null, completeness is 0%"
		}
		return true, ""
	}

	// For maps/structs, check field completeness
	if m, ok := value.(map[string]interface{}); ok {
		fields, _ := rule.Config["fields"].([]interface{})
		if len(fields) == 0 {
			return true, ""
		}

		complete := 0
		for _, field := range fields {
			fieldStr := fmt.Sprintf("%v", field)
			if val, exists := m[fieldStr]; exists && val != nil {
				complete++
			}
		}

		completeness := float64(complete) / float64(len(fields))
		if completeness < minCompleteness {
			return false, fmt.Sprintf("Completeness %.2f%% is below threshold %.2f%%", completeness*100, minCompleteness*100)
		}
	}

	return true, ""
}

func (v *Validator) validateConsistency(rule *ValidationRule, value interface{}, metadata map[string]interface{}) (bool, string) {
	refField, ok := rule.Config["reference_field"].(string)
	if !ok {
		return false, "Invalid consistency configuration"
	}

	comparator, _ := rule.Config["comparator"].(string)
	if comparator == "" {
		comparator = "eq"
	}

	refValue, ok := metadata[refField]
	if !ok {
		return false, fmt.Sprintf("Reference field '%s' not found", refField)
	}

	result := compare(value, refValue, comparator)
	if !result {
		return false, fmt.Sprintf("Consistency check failed: %v %s %v", value, comparator, refValue)
	}

	return true, ""
}

func (v *Validator) validateCustom(rule *ValidationRule, value interface{}, metadata map[string]interface{}) (bool, string) {
	expression, ok := rule.Config["expression"].(string)
	if !ok {
		return false, "Invalid custom rule configuration"
	}

	// Simple expression evaluation
	// In production, this would use a proper expression language like CEL
	switch expression {
	case "not_empty":
		str := fmt.Sprintf("%v", value)
		if str == "" || str == "<nil>" {
			return false, "Value is empty"
		}
		return true, ""
	case "positive":
		num, ok := toFloat64(value)
		if !ok {
			return false, "Value is not numeric"
		}
		if num <= 0 {
			return false, "Value is not positive"
		}
		return true, ""
	case "non_negative":
		num, ok := toFloat64(value)
		if !ok {
			return false, "Value is not numeric"
		}
		if num < 0 {
			return false, "Value is negative"
		}
		return true, ""
	default:
		return true, "Custom expression not evaluated"
	}
}

// CalculateQualityScore calculates quality score for a feature.
func (v *Validator) CalculateQualityScore(featureID string) *QualityScore {
	v.mu.RLock()
	defer v.mu.RUnlock()

	score := &QualityScore{
		FeatureID:   featureID,
		Dimensions:  make(map[string]float64),
		LastUpdated: time.Now(),
	}

	// Find recent reports for this feature
	var recentReports []*ValidationReport
	for _, report := range v.history {
		for _, result := range report.Results {
			if result.FeatureID == featureID {
				recentReports = append(recentReports, report)
				break
			}
		}
	}

	if len(recentReports) == 0 {
		score.OverallScore = 1.0
		return score
	}

	// Calculate dimension scores
	dimensions := map[string]struct {
		passed int
		total  int
	}{
		"completeness": {},
		"validity":     {},
		"timeliness":   {},
		"consistency":  {},
	}

	for _, report := range recentReports {
		for _, result := range report.Results {
			if result.FeatureID != featureID {
				continue
			}

			rule, exists := v.rules[result.RuleID]
			if !exists {
				continue
			}

			dim := getDimension(rule.Type)
			d := dimensions[dim]
			d.total++
			if result.Passed {
				d.passed++
			}
			dimensions[dim] = d
		}
	}

	totalScore := 0.0
	dimCount := 0
	for dim, stats := range dimensions {
		if stats.total > 0 {
			dimScore := float64(stats.passed) / float64(stats.total)
			score.Dimensions[dim] = dimScore
			totalScore += dimScore
			dimCount++
		}
	}

	if dimCount > 0 {
		score.OverallScore = totalScore / float64(dimCount)
	} else {
		score.OverallScore = 1.0
	}

	// Determine trend
	if prevScore, ok := v.qualityScores[featureID]; ok {
		if score.OverallScore > prevScore.OverallScore+0.01 {
			score.TrendDirection = "improving"
		} else if score.OverallScore < prevScore.OverallScore-0.01 {
			score.TrendDirection = "degrading"
		} else {
			score.TrendDirection = "stable"
		}
	} else {
		score.TrendDirection = "stable"
	}

	return score
}

// GetQualityHistory returns validation history.
func (v *Validator) GetQualityHistory(limit int) []*ValidationReport {
	v.mu.RLock()
	defer v.mu.RUnlock()

	if limit <= 0 || limit > len(v.history) {
		limit = len(v.history)
	}

	start := len(v.history) - limit
	if start < 0 {
		start = 0
	}

	result := make([]*ValidationReport, len(v.history[start:]))
	copy(result, v.history[start:])
	return result
}

// GetStats returns validator statistics.
func (v *Validator) GetStats() map[string]interface{} {
	v.mu.RLock()
	defer v.mu.RUnlock()

	rulesByType := make(map[string]int)
	for _, rule := range v.rules {
		rulesByType[string(rule.Type)]++
	}

	return map[string]interface{}{
		"total_rules":      len(v.rules),
		"rules_by_type":    rulesByType,
		"features_covered": len(v.rulesByFeature),
		"groups_covered":   len(v.rulesByGroup),
		"history_count":    len(v.history),
	}
}

// Helper functions

func removeRule(rules []*ValidationRule, ruleID string) []*ValidationRule {
	result := make([]*ValidationRule, 0, len(rules))
	for _, r := range rules {
		if r.ID != ruleID {
			result = append(result, r)
		}
	}
	return result
}

func toFloat64(v interface{}) (float64, bool) {
	switch val := v.(type) {
	case float64:
		return val, true
	case float32:
		return float64(val), true
	case int:
		return float64(val), true
	case int64:
		return float64(val), true
	case int32:
		return float64(val), true
	default:
		return 0, false
	}
}

func equalFold(s1, s2 string) bool {
	if len(s1) != len(s2) {
		return false
	}
	for i := 0; i < len(s1); i++ {
		c1, c2 := s1[i], s2[i]
		if c1 >= 'A' && c1 <= 'Z' {
			c1 += 32
		}
		if c2 >= 'A' && c2 <= 'Z' {
			c2 += 32
		}
		if c1 != c2 {
			return false
		}
	}
	return true
}

func compare(a, b interface{}, op string) bool {
	aNum, aOk := toFloat64(a)
	bNum, bOk := toFloat64(b)

	if aOk && bOk {
		switch op {
		case "eq":
			return aNum == bNum
		case "neq":
			return aNum != bNum
		case "gt":
			return aNum > bNum
		case "lt":
			return aNum < bNum
		case "gte":
			return aNum >= bNum
		case "lte":
			return aNum <= bNum
		}
	}

	// Fall back to string comparison
	aStr := fmt.Sprintf("%v", a)
	bStr := fmt.Sprintf("%v", b)

	switch op {
	case "eq":
		return aStr == bStr
	case "neq":
		return aStr != bStr
	default:
		return false
	}
}

func getDimension(ruleType RuleType) string {
	switch ruleType {
	case RuleTypeNotNull, RuleTypeCompleteness:
		return "completeness"
	case RuleTypeRange, RuleTypePattern, RuleTypeEnum, RuleTypeCustom:
		return "validity"
	case RuleTypeFreshness:
		return "timeliness"
	case RuleTypeConsistency, RuleTypeAccuracy:
		return "consistency"
	default:
		return "validity"
	}
}
