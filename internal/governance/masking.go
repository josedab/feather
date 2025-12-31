package governance

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
)

// Errors returned by masking operations.
var (
	ErrMaskingRuleNotFound = errors.New("masking rule not found")
	ErrInvalidMaskingType  = errors.New("invalid masking type")
)

// MaskingType represents the type of masking to apply.
type MaskingType string

const (
	MaskingTypeNone       MaskingType = "none"       // No masking
	MaskingTypeRedact     MaskingType = "redact"     // Replace with fixed string
	MaskingTypePartial    MaskingType = "partial"    // Show partial value
	MaskingTypeHash       MaskingType = "hash"       // Cryptographic hash
	MaskingTypeTokenize   MaskingType = "tokenize"   // Replace with token
	MaskingTypeGeneralize MaskingType = "generalize" // Generalize value (e.g., age range)
	MaskingTypeNoise      MaskingType = "noise"      // Add statistical noise
	MaskingTypeNull       MaskingType = "null"       // Replace with null
	MaskingTypeCustom     MaskingType = "custom"     // Custom masking function
)

// MaskingRule defines how to mask a specific feature.
type MaskingRule struct {
	// FeatureName is the feature to mask.
	FeatureName string `json:"feature_name"`

	// MaskingType is the type of masking.
	MaskingType MaskingType `json:"masking_type"`

	// PIICategory is the PII category this rule applies to.
	PIICategory PIICategory `json:"pii_category,omitempty"`

	// Sensitivity is the minimum sensitivity to trigger masking.
	Sensitivity PIISensitivity `json:"sensitivity,omitempty"`

	// RedactValue is the replacement value for redaction.
	RedactValue string `json:"redact_value,omitempty"`

	// PartialConfig configures partial masking.
	PartialConfig *PartialMaskConfig `json:"partial_config,omitempty"`

	// HashSalt is the salt for hashing.
	HashSalt string `json:"hash_salt,omitempty"`

	// GeneralizeConfig configures generalization.
	GeneralizeConfig *GeneralizeConfig `json:"generalize_config,omitempty"`

	// Enabled indicates if the rule is active.
	Enabled bool `json:"enabled"`

	// Description describes the rule.
	Description string `json:"description,omitempty"`

	// AppliesTo lists roles/permissions this rule applies to.
	AppliesTo []string `json:"applies_to,omitempty"`

	// ExceptFor lists roles/permissions exempt from this rule.
	ExceptFor []string `json:"except_for,omitempty"`
}

// PartialMaskConfig configures partial masking.
type PartialMaskConfig struct {
	// ShowFirst is the number of characters to show at the start.
	ShowFirst int `json:"show_first"`

	// ShowLast is the number of characters to show at the end.
	ShowLast int `json:"show_last"`

	// MaskChar is the masking character.
	MaskChar string `json:"mask_char"`

	// PreserveFormat preserves the original format (e.g., xxx-xxx-xxxx).
	PreserveFormat bool `json:"preserve_format"`
}

// GeneralizeConfig configures value generalization.
type GeneralizeConfig struct {
	// Type is the generalization type.
	Type string `json:"type"` // "range", "bucket", "truncate"

	// RangeSize is the range size for numeric values.
	RangeSize float64 `json:"range_size,omitempty"`

	// Buckets are predefined buckets for categorical values.
	Buckets map[string][]string `json:"buckets,omitempty"`

	// TruncateLength is the truncation length for strings.
	TruncateLength int `json:"truncate_length,omitempty"`
}

// MaskingConfig configures the data masking system.
type MaskingConfig struct {
	// Enabled enables data masking.
	Enabled bool `json:"enabled" yaml:"enabled"`

	// DefaultMaskingType is the default masking type.
	DefaultMaskingType MaskingType `json:"default_masking_type" yaml:"default_masking_type"`

	// DefaultRedactValue is the default redaction value.
	DefaultRedactValue string `json:"default_redact_value" yaml:"default_redact_value"`

	// Rules are the masking rules.
	Rules []MaskingRule `json:"rules" yaml:"rules"`

	// AutoMaskPII automatically masks detected PII.
	AutoMaskPII bool `json:"auto_mask_pii" yaml:"auto_mask_pii"`

	// TokenPrefix is the prefix for generated tokens.
	TokenPrefix string `json:"token_prefix" yaml:"token_prefix"`
}

// DefaultMaskingConfig returns the default masking configuration.
func DefaultMaskingConfig() MaskingConfig {
	return MaskingConfig{
		Enabled:            true,
		DefaultMaskingType: MaskingTypeRedact,
		DefaultRedactValue: "[REDACTED]",
		AutoMaskPII:        true,
		TokenPrefix:        "TOK_",
	}
}

// DataMasker applies data masking rules.
type DataMasker struct {
	mu          sync.RWMutex
	config      MaskingConfig
	rules       map[string]*MaskingRule // by feature name
	piiRules    map[PIICategory]*MaskingRule
	tokenStore  map[string]string // token -> original value
	piiDetector *PIIDetector

	// Metrics
	valuesMasked int64
	tokensIssued int64
}

// NewDataMasker creates a new data masker.
func NewDataMasker(config MaskingConfig, piiDetector *PIIDetector) *DataMasker {
	m := &DataMasker{
		config:      config,
		rules:       make(map[string]*MaskingRule),
		piiRules:    make(map[PIICategory]*MaskingRule),
		tokenStore:  make(map[string]string),
		piiDetector: piiDetector,
	}

	// Load rules
	for _, rule := range config.Rules {
		r := rule
		m.AddRule(&r)
	}

	// Set up default PII masking rules
	if config.AutoMaskPII {
		m.setupDefaultPIIRules()
	}

	return m
}

// setupDefaultPIIRules creates default masking rules for PII categories.
func (m *DataMasker) setupDefaultPIIRules() {
	defaultRules := []MaskingRule{
		{
			PIICategory: PIICategoryEmail,
			MaskingType: MaskingTypePartial,
			PartialConfig: &PartialMaskConfig{
				ShowFirst: 2,
				ShowLast:  0,
				MaskChar:  "*",
			},
			Enabled: true,
		},
		{
			PIICategory: PIICategoryPhone,
			MaskingType: MaskingTypePartial,
			PartialConfig: &PartialMaskConfig{
				ShowFirst: 0,
				ShowLast:  4,
				MaskChar:  "*",
			},
			Enabled: true,
		},
		{
			PIICategory: PIICategorySSN,
			MaskingType: MaskingTypePartial,
			PartialConfig: &PartialMaskConfig{
				ShowFirst: 0,
				ShowLast:  4,
				MaskChar:  "*",
			},
			Enabled: true,
		},
		{
			PIICategory: PIICategoryCreditCard,
			MaskingType: MaskingTypePartial,
			PartialConfig: &PartialMaskConfig{
				ShowFirst: 0,
				ShowLast:  4,
				MaskChar:  "*",
			},
			Enabled: true,
		},
		{
			PIICategory: PIICategoryIPAddress,
			MaskingType: MaskingTypeGeneralize,
			GeneralizeConfig: &GeneralizeConfig{
				Type:           "truncate",
				TruncateLength: 3, // Show only first 3 octets
			},
			Enabled: true,
		},
	}

	for _, rule := range defaultRules {
		r := rule
		m.piiRules[r.PIICategory] = &r
	}
}

// AddRule adds a masking rule.
func (m *DataMasker) AddRule(rule *MaskingRule) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if rule.FeatureName != "" {
		m.rules[rule.FeatureName] = rule
	}
	if rule.PIICategory != "" {
		m.piiRules[rule.PIICategory] = rule
	}
}

// RemoveRule removes a masking rule.
func (m *DataMasker) RemoveRule(featureName string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.rules, featureName)
}

// GetRule returns a masking rule for a feature.
func (m *DataMasker) GetRule(featureName string) (*MaskingRule, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	rule, ok := m.rules[featureName]
	return rule, ok
}

// Mask applies masking to a single value.
func (m *DataMasker) Mask(ctx context.Context, featureName string, value interface{}, userContext *MaskingContext) (interface{}, bool) {
	if !m.config.Enabled {
		return value, false
	}

	// Get rule for feature
	rule := m.getRuleForFeature(featureName, userContext)
	if rule == nil || !rule.Enabled || rule.MaskingType == MaskingTypeNone {
		return value, false
	}

	// Check if user is exempt
	if m.isExempt(rule, userContext) {
		return value, false
	}

	// Apply masking
	masked, err := m.applyMasking(rule, value)
	if err != nil {
		return value, false
	}

	atomic.AddInt64(&m.valuesMasked, 1)
	return masked, true
}

// MaskBatch applies masking to multiple features.
func (m *DataMasker) MaskBatch(ctx context.Context, features map[string]interface{}, userContext *MaskingContext) map[string]interface{} {
	result := make(map[string]interface{})

	for name, value := range features {
		masked, _ := m.Mask(ctx, name, value, userContext)
		result[name] = masked
	}

	return result
}

// MaskWithPII masks values based on PII detection.
func (m *DataMasker) MaskWithPII(ctx context.Context, featureName string, value interface{}, detections []*PIIDetection, userContext *MaskingContext) (interface{}, bool) {
	if !m.config.Enabled || !m.config.AutoMaskPII {
		return value, false
	}

	if len(detections) == 0 {
		return m.Mask(ctx, featureName, value, userContext)
	}

	// Find highest sensitivity detection and apply corresponding rule
	var rule *MaskingRule
	highestSensitivity := -1
	sensitivityOrder := map[PIISensitivity]int{
		SensitivityLow:      0,
		SensitivityMedium:   1,
		SensitivityHigh:     2,
		SensitivityCritical: 3,
	}

	m.mu.RLock()
	for _, det := range detections {
		if r, ok := m.piiRules[det.Category]; ok {
			level := sensitivityOrder[det.Sensitivity]
			if level > highestSensitivity {
				highestSensitivity = level
				rule = r
			}
		}
	}
	m.mu.RUnlock()

	if rule == nil || !rule.Enabled {
		return m.Mask(ctx, featureName, value, userContext)
	}

	// Check exemption
	if m.isExempt(rule, userContext) {
		return value, false
	}

	// Apply masking
	masked, err := m.applyMasking(rule, value)
	if err != nil {
		return value, false
	}

	atomic.AddInt64(&m.valuesMasked, 1)
	return masked, true
}

// getRuleForFeature gets the applicable rule for a feature.
func (m *DataMasker) getRuleForFeature(featureName string, userContext *MaskingContext) *MaskingRule {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Check feature-specific rule
	if rule, ok := m.rules[featureName]; ok {
		return rule
	}

	// Check PII detection
	if m.piiDetector != nil && m.config.AutoMaskPII {
		category, _, hasPII := m.piiDetector.ClassifyFeature(featureName)
		if hasPII {
			if rule, ok := m.piiRules[category]; ok {
				return rule
			}
		}
	}

	return nil
}

// isExempt checks if a user is exempt from a masking rule.
func (m *DataMasker) isExempt(rule *MaskingRule, ctx *MaskingContext) bool {
	if ctx == nil {
		return false
	}

	// Check exceptions
	for _, exempt := range rule.ExceptFor {
		for _, role := range ctx.Roles {
			if role == exempt {
				return true
			}
		}
		for _, perm := range ctx.Permissions {
			if perm == exempt {
				return true
			}
		}
	}

	return false
}

// applyMasking applies the masking rule to a value.
func (m *DataMasker) applyMasking(rule *MaskingRule, value interface{}) (interface{}, error) {
	switch rule.MaskingType {
	case MaskingTypeRedact:
		return m.applyRedaction(rule, value)
	case MaskingTypePartial:
		return m.applyPartialMasking(rule, value)
	case MaskingTypeHash:
		return m.applyHashing(rule, value)
	case MaskingTypeTokenize:
		return m.applyTokenization(rule, value)
	case MaskingTypeGeneralize:
		return m.applyGeneralization(rule, value)
	case MaskingTypeNull:
		return nil, nil
	default:
		return value, nil
	}
}

// applyRedaction replaces value with redaction string.
func (m *DataMasker) applyRedaction(rule *MaskingRule, value interface{}) (interface{}, error) {
	redactValue := rule.RedactValue
	if redactValue == "" {
		redactValue = m.config.DefaultRedactValue
	}
	return redactValue, nil
}

// applyPartialMasking shows partial value.
func (m *DataMasker) applyPartialMasking(rule *MaskingRule, value interface{}) (interface{}, error) {
	str := fmt.Sprintf("%v", value)
	if rule.PartialConfig == nil {
		return str, nil
	}

	cfg := rule.PartialConfig
	maskChar := cfg.MaskChar
	if maskChar == "" {
		maskChar = "*"
	}

	if len(str) <= cfg.ShowFirst+cfg.ShowLast {
		return strings.Repeat(maskChar, len(str)), nil
	}

	result := ""
	if cfg.ShowFirst > 0 {
		result += str[:cfg.ShowFirst]
	}

	maskLen := len(str) - cfg.ShowFirst - cfg.ShowLast
	if cfg.PreserveFormat {
		// Preserve format characters
		for i := cfg.ShowFirst; i < len(str)-cfg.ShowLast; i++ {
			if !isAlphanumeric(str[i]) {
				result += string(str[i])
			} else {
				result += maskChar
			}
		}
	} else {
		result += strings.Repeat(maskChar, maskLen)
	}

	if cfg.ShowLast > 0 {
		result += str[len(str)-cfg.ShowLast:]
	}

	return result, nil
}

// applyHashing creates a hash of the value.
func (m *DataMasker) applyHashing(rule *MaskingRule, value interface{}) (interface{}, error) {
	str := fmt.Sprintf("%v", value)
	salt := rule.HashSalt

	hasher := sha256.New()
	hasher.Write([]byte(salt + str))
	hash := hex.EncodeToString(hasher.Sum(nil))

	return hash[:16], nil // Return truncated hash
}

// applyTokenization replaces value with a token.
func (m *DataMasker) applyTokenization(rule *MaskingRule, value interface{}) (interface{}, error) {
	str := fmt.Sprintf("%v", value)

	// Check if already tokenized
	m.mu.RLock()
	for token, original := range m.tokenStore {
		if original == str {
			m.mu.RUnlock()
			return token, nil
		}
	}
	m.mu.RUnlock()

	// Generate new token
	hasher := sha256.New()
	hasher.Write([]byte(str + fmt.Sprintf("%d", atomic.LoadInt64(&m.tokensIssued))))
	tokenHash := hex.EncodeToString(hasher.Sum(nil))[:12]
	token := m.config.TokenPrefix + tokenHash

	m.mu.Lock()
	m.tokenStore[token] = str
	m.mu.Unlock()

	atomic.AddInt64(&m.tokensIssued, 1)

	return token, nil
}

// applyGeneralization generalizes the value.
func (m *DataMasker) applyGeneralization(rule *MaskingRule, value interface{}) (interface{}, error) {
	if rule.GeneralizeConfig == nil {
		return value, nil
	}

	cfg := rule.GeneralizeConfig

	switch cfg.Type {
	case "range":
		// For numeric values, create ranges
		if num, ok := toFloat64(value); ok {
			rangeStart := float64(int(num/cfg.RangeSize)) * cfg.RangeSize
			rangeEnd := rangeStart + cfg.RangeSize
			return fmt.Sprintf("%.0f-%.0f", rangeStart, rangeEnd), nil
		}
	case "truncate":
		str := fmt.Sprintf("%v", value)
		if cfg.TruncateLength > 0 && len(str) > cfg.TruncateLength {
			return str[:cfg.TruncateLength] + "...", nil
		}
	case "bucket":
		str := fmt.Sprintf("%v", value)
		for bucket, values := range cfg.Buckets {
			for _, v := range values {
				if v == str {
					return bucket, nil
				}
			}
		}
	}

	return value, nil
}

// Detokenize converts a token back to its original value.
func (m *DataMasker) Detokenize(token string) (interface{}, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if original, ok := m.tokenStore[token]; ok {
		return original, true
	}
	return nil, false
}

// Stats returns masking statistics.
func (m *DataMasker) Stats() map[string]interface{} {
	m.mu.RLock()
	rulesCount := len(m.rules)
	piiRulesCount := len(m.piiRules)
	tokensCount := len(m.tokenStore)
	m.mu.RUnlock()

	return map[string]interface{}{
		"enabled":       m.config.Enabled,
		"rules":         rulesCount,
		"pii_rules":     piiRulesCount,
		"tokens_stored": tokensCount,
		"values_masked": atomic.LoadInt64(&m.valuesMasked),
		"tokens_issued": atomic.LoadInt64(&m.tokensIssued),
	}
}

// MaskingContext provides context for masking decisions.
type MaskingContext struct {
	// UserID is the requesting user.
	UserID string

	// TenantID is the tenant context.
	TenantID string

	// Roles are the user's roles.
	Roles []string

	// Permissions are the user's permissions.
	Permissions []string

	// Purpose is the data access purpose.
	Purpose string

	// RequestID is the request identifier.
	RequestID string
}

// Helper functions

func isAlphanumeric(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
}

func toFloat64(value interface{}) (float64, bool) {
	switch v := value.(type) {
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case float32:
		return float64(v), true
	case float64:
		return v, true
	default:
		return 0, false
	}
}

// MaskEmailDomain masks an email address showing only domain.
func MaskEmailDomain(email string) string {
	re := regexp.MustCompile(`^([^@]+)@(.+)$`)
	matches := re.FindStringSubmatch(email)
	if len(matches) == 3 {
		user := matches[1]
		domain := matches[2]
		if len(user) > 2 {
			return user[:2] + "***@" + domain
		}
		return "***@" + domain
	}
	return "[REDACTED]"
}

// MaskPhoneNumber masks a phone number showing last 4 digits.
func MaskPhoneNumber(phone string) string {
	digits := regexp.MustCompile(`[0-9]`).FindAllString(phone, -1)
	if len(digits) < 4 {
		return "****"
	}
	return "***-***-" + strings.Join(digits[len(digits)-4:], "")
}

// MaskCreditCard masks a credit card showing last 4 digits.
func MaskCreditCard(card string) string {
	digits := regexp.MustCompile(`[0-9]`).FindAllString(card, -1)
	if len(digits) < 4 {
		return "****"
	}
	return "**** **** **** " + strings.Join(digits[len(digits)-4:], "")
}

// MaskSSN masks a Social Security Number.
func MaskSSN(ssn string) string {
	digits := regexp.MustCompile(`[0-9]`).FindAllString(ssn, -1)
	if len(digits) < 4 {
		return "***-**-****"
	}
	return "***-**-" + strings.Join(digits[len(digits)-4:], "")
}
