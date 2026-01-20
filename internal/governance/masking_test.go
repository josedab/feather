package governance

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultMaskingConfig(t *testing.T) {
	config := DefaultMaskingConfig()

	assert.True(t, config.Enabled)
	assert.Equal(t, MaskingTypeRedact, config.DefaultMaskingType)
	assert.Equal(t, "[REDACTED]", config.DefaultRedactValue)
	assert.True(t, config.AutoMaskPII)
	assert.Equal(t, "TOK_", config.TokenPrefix)
}

func TestMaskingType_Values(t *testing.T) {
	assert.Equal(t, MaskingType("none"), MaskingTypeNone)
	assert.Equal(t, MaskingType("redact"), MaskingTypeRedact)
	assert.Equal(t, MaskingType("partial"), MaskingTypePartial)
	assert.Equal(t, MaskingType("hash"), MaskingTypeHash)
	assert.Equal(t, MaskingType("tokenize"), MaskingTypeTokenize)
	assert.Equal(t, MaskingType("generalize"), MaskingTypeGeneralize)
	assert.Equal(t, MaskingType("noise"), MaskingTypeNoise)
	assert.Equal(t, MaskingType("null"), MaskingTypeNull)
	assert.Equal(t, MaskingType("custom"), MaskingTypeCustom)
}

func TestNewDataMasker(t *testing.T) {
	config := DefaultMaskingConfig()
	masker := NewDataMasker(config, nil)

	require.NotNil(t, masker)
}

func TestNewDataMasker_WithRules(t *testing.T) {
	config := MaskingConfig{
		Enabled: true,
		Rules: []MaskingRule{
			{
				FeatureName: "secret_key",
				MaskingType: MaskingTypeRedact,
				RedactValue: "***",
				Enabled:     true,
			},
		},
	}

	masker := NewDataMasker(config, nil)
	require.NotNil(t, masker)

	rule, ok := masker.GetRule("secret_key")
	assert.True(t, ok)
	assert.Equal(t, MaskingTypeRedact, rule.MaskingType)
}

func TestDataMasker_Mask_Disabled(t *testing.T) {
	config := MaskingConfig{
		Enabled: false,
	}
	masker := NewDataMasker(config, nil)

	ctx := context.Background()
	result, masked := masker.Mask(ctx, "secret", "sensitive-value", nil)

	assert.Equal(t, "sensitive-value", result)
	assert.False(t, masked)
}

func TestDataMasker_Mask_Redact(t *testing.T) {
	config := MaskingConfig{
		Enabled:            true,
		DefaultRedactValue: "[HIDDEN]",
		Rules: []MaskingRule{
			{
				FeatureName: "password",
				MaskingType: MaskingTypeRedact,
				Enabled:     true,
			},
		},
	}
	masker := NewDataMasker(config, nil)

	ctx := context.Background()
	result, masked := masker.Mask(ctx, "password", "my-secret-password", nil)

	assert.Equal(t, "[HIDDEN]", result)
	assert.True(t, masked)
}

func TestDataMasker_Mask_RedactWithCustomValue(t *testing.T) {
	config := MaskingConfig{
		Enabled: true,
		Rules: []MaskingRule{
			{
				FeatureName: "api_key",
				MaskingType: MaskingTypeRedact,
				RedactValue: "***API_KEY***",
				Enabled:     true,
			},
		},
	}
	masker := NewDataMasker(config, nil)

	ctx := context.Background()
	result, masked := masker.Mask(ctx, "api_key", "sk-12345", nil)

	assert.Equal(t, "***API_KEY***", result)
	assert.True(t, masked)
}

func TestDataMasker_Mask_Partial(t *testing.T) {
	config := MaskingConfig{
		Enabled: true,
		Rules: []MaskingRule{
			{
				FeatureName: "phone",
				MaskingType: MaskingTypePartial,
				PartialConfig: &PartialMaskConfig{
					ShowFirst: 0,
					ShowLast:  4,
					MaskChar:  "*",
				},
				Enabled: true,
			},
		},
	}
	masker := NewDataMasker(config, nil)

	ctx := context.Background()
	result, masked := masker.Mask(ctx, "phone", "555-123-4567", nil)

	assert.True(t, masked)
	assert.Contains(t, result.(string), "4567")
	assert.Contains(t, result.(string), "*")
}

func TestDataMasker_Mask_PartialShowBoth(t *testing.T) {
	config := MaskingConfig{
		Enabled: true,
		Rules: []MaskingRule{
			{
				FeatureName: "email",
				MaskingType: MaskingTypePartial,
				PartialConfig: &PartialMaskConfig{
					ShowFirst: 2,
					ShowLast:  4,
					MaskChar:  "*",
				},
				Enabled: true,
			},
		},
	}
	masker := NewDataMasker(config, nil)

	ctx := context.Background()
	result, masked := masker.Mask(ctx, "email", "john.doe@example.com", nil)

	assert.True(t, masked)
	resultStr := result.(string)
	assert.Equal(t, "jo", resultStr[:2])
	assert.Equal(t, ".com", resultStr[len(resultStr)-4:])
}

func TestDataMasker_Mask_PartialPreserveFormat(t *testing.T) {
	config := MaskingConfig{
		Enabled: true,
		Rules: []MaskingRule{
			{
				FeatureName: "ssn",
				MaskingType: MaskingTypePartial,
				PartialConfig: &PartialMaskConfig{
					ShowFirst:      0,
					ShowLast:       4,
					MaskChar:       "*",
					PreserveFormat: true,
				},
				Enabled: true,
			},
		},
	}
	masker := NewDataMasker(config, nil)

	ctx := context.Background()
	result, masked := masker.Mask(ctx, "ssn", "123-45-6789", nil)

	assert.True(t, masked)
	resultStr := result.(string)
	assert.Equal(t, "6789", resultStr[len(resultStr)-4:])
	assert.Contains(t, resultStr, "-")
}

func TestDataMasker_Mask_PartialShortValue(t *testing.T) {
	config := MaskingConfig{
		Enabled: true,
		Rules: []MaskingRule{
			{
				FeatureName: "short",
				MaskingType: MaskingTypePartial,
				PartialConfig: &PartialMaskConfig{
					ShowFirst: 3,
					ShowLast:  3,
					MaskChar:  "*",
				},
				Enabled: true,
			},
		},
	}
	masker := NewDataMasker(config, nil)

	ctx := context.Background()
	result, masked := masker.Mask(ctx, "short", "abcd", nil)

	assert.True(t, masked)
	assert.Equal(t, "****", result)
}

func TestDataMasker_Mask_Hash(t *testing.T) {
	config := MaskingConfig{
		Enabled: true,
		Rules: []MaskingRule{
			{
				FeatureName: "user_id",
				MaskingType: MaskingTypeHash,
				HashSalt:    "test-salt",
				Enabled:     true,
			},
		},
	}
	masker := NewDataMasker(config, nil)

	ctx := context.Background()
	result, masked := masker.Mask(ctx, "user_id", "user123", nil)

	assert.True(t, masked)
	assert.Len(t, result.(string), 16) // Truncated hash

	// Same input should produce same hash
	result2, _ := masker.Mask(ctx, "user_id", "user123", nil)
	assert.Equal(t, result, result2)

	// Different input should produce different hash
	result3, _ := masker.Mask(ctx, "user_id", "user456", nil)
	assert.NotEqual(t, result, result3)
}

func TestDataMasker_Mask_Tokenize(t *testing.T) {
	config := MaskingConfig{
		Enabled:     true,
		TokenPrefix: "TKN_",
		Rules: []MaskingRule{
			{
				FeatureName: "customer_id",
				MaskingType: MaskingTypeTokenize,
				Enabled:     true,
			},
		},
	}
	masker := NewDataMasker(config, nil)

	ctx := context.Background()
	result, masked := masker.Mask(ctx, "customer_id", "CUST-12345", nil)

	assert.True(t, masked)
	token := result.(string)
	assert.True(t, len(token) > 0)
	assert.Contains(t, token, "TKN_")

	// Same value should return same token
	result2, _ := masker.Mask(ctx, "customer_id", "CUST-12345", nil)
	assert.Equal(t, token, result2)
}

func TestDataMasker_Mask_Null(t *testing.T) {
	config := MaskingConfig{
		Enabled: true,
		Rules: []MaskingRule{
			{
				FeatureName: "sensitive",
				MaskingType: MaskingTypeNull,
				Enabled:     true,
			},
		},
	}
	masker := NewDataMasker(config, nil)

	ctx := context.Background()
	result, masked := masker.Mask(ctx, "sensitive", "some-value", nil)

	assert.True(t, masked)
	assert.Nil(t, result)
}

func TestDataMasker_Mask_Generalize_Range(t *testing.T) {
	config := MaskingConfig{
		Enabled: true,
		Rules: []MaskingRule{
			{
				FeatureName: "age",
				MaskingType: MaskingTypeGeneralize,
				GeneralizeConfig: &GeneralizeConfig{
					Type:      "range",
					RangeSize: 10,
				},
				Enabled: true,
			},
		},
	}
	masker := NewDataMasker(config, nil)

	ctx := context.Background()
	result, masked := masker.Mask(ctx, "age", 25, nil)

	assert.True(t, masked)
	assert.Equal(t, "20-30", result)
}

func TestDataMasker_Mask_Generalize_Truncate(t *testing.T) {
	config := MaskingConfig{
		Enabled: true,
		Rules: []MaskingRule{
			{
				FeatureName: "zip_code",
				MaskingType: MaskingTypeGeneralize,
				GeneralizeConfig: &GeneralizeConfig{
					Type:           "truncate",
					TruncateLength: 3,
				},
				Enabled: true,
			},
		},
	}
	masker := NewDataMasker(config, nil)

	ctx := context.Background()
	result, masked := masker.Mask(ctx, "zip_code", "12345-6789", nil)

	assert.True(t, masked)
	assert.Equal(t, "123...", result)
}

func TestDataMasker_Mask_Generalize_Bucket(t *testing.T) {
	config := MaskingConfig{
		Enabled: true,
		Rules: []MaskingRule{
			{
				FeatureName: "income",
				MaskingType: MaskingTypeGeneralize,
				GeneralizeConfig: &GeneralizeConfig{
					Type: "bucket",
					Buckets: map[string][]string{
						"low":    {"10000", "20000", "30000"},
						"medium": {"50000", "60000", "70000"},
						"high":   {"100000", "150000", "200000"},
					},
				},
				Enabled: true,
			},
		},
	}
	masker := NewDataMasker(config, nil)

	ctx := context.Background()
	result, masked := masker.Mask(ctx, "income", "50000", nil)

	assert.True(t, masked)
	assert.Equal(t, "medium", result)
}

func TestDataMasker_Mask_NoMatchingRule(t *testing.T) {
	config := MaskingConfig{
		Enabled: true,
		Rules: []MaskingRule{
			{
				FeatureName: "other",
				MaskingType: MaskingTypeRedact,
				Enabled:     true,
			},
		},
	}
	masker := NewDataMasker(config, nil)

	ctx := context.Background()
	result, masked := masker.Mask(ctx, "unmasked", "value", nil)

	assert.Equal(t, "value", result)
	assert.False(t, masked)
}

func TestDataMasker_Mask_DisabledRule(t *testing.T) {
	config := MaskingConfig{
		Enabled: true,
		Rules: []MaskingRule{
			{
				FeatureName: "secret",
				MaskingType: MaskingTypeRedact,
				Enabled:     false, // Disabled
			},
		},
	}
	masker := NewDataMasker(config, nil)

	ctx := context.Background()
	result, masked := masker.Mask(ctx, "secret", "value", nil)

	assert.Equal(t, "value", result)
	assert.False(t, masked)
}

func TestDataMasker_Mask_NoneType(t *testing.T) {
	config := MaskingConfig{
		Enabled: true,
		Rules: []MaskingRule{
			{
				FeatureName: "feature",
				MaskingType: MaskingTypeNone,
				Enabled:     true,
			},
		},
	}
	masker := NewDataMasker(config, nil)

	ctx := context.Background()
	result, masked := masker.Mask(ctx, "feature", "value", nil)

	assert.Equal(t, "value", result)
	assert.False(t, masked)
}

func TestDataMasker_Mask_Exemption(t *testing.T) {
	config := MaskingConfig{
		Enabled:            true,
		DefaultRedactValue: "[REDACTED]",
		Rules: []MaskingRule{
			{
				FeatureName: "secret",
				MaskingType: MaskingTypeRedact,
				Enabled:     true,
				ExceptFor:   []string{"admin"},
			},
		},
	}
	masker := NewDataMasker(config, nil)

	ctx := context.Background()

	// Non-admin user should be masked
	userCtx := &MaskingContext{
		UserID: "user1",
		Roles:  []string{"user"},
	}
	result, masked := masker.Mask(ctx, "secret", "value", userCtx)
	assert.True(t, masked)
	assert.Equal(t, "[REDACTED]", result)

	// Admin user should NOT be masked
	adminCtx := &MaskingContext{
		UserID: "admin1",
		Roles:  []string{"admin"},
	}
	result, masked = masker.Mask(ctx, "secret", "value", adminCtx)
	assert.False(t, masked)
	assert.Equal(t, "value", result)
}

func TestDataMasker_Mask_ExemptionByPermission(t *testing.T) {
	config := MaskingConfig{
		Enabled: true,
		Rules: []MaskingRule{
			{
				FeatureName: "secret",
				MaskingType: MaskingTypeRedact,
				Enabled:     true,
				ExceptFor:   []string{"view_sensitive"},
			},
		},
	}
	masker := NewDataMasker(config, nil)

	ctx := context.Background()

	// User with permission should NOT be masked
	userCtx := &MaskingContext{
		UserID:      "user1",
		Permissions: []string{"view_sensitive"},
	}
	result, masked := masker.Mask(ctx, "secret", "value", userCtx)
	assert.False(t, masked)
	assert.Equal(t, "value", result)
}

func TestDataMasker_MaskBatch(t *testing.T) {
	config := MaskingConfig{
		Enabled: true,
		Rules: []MaskingRule{
			{
				FeatureName: "password",
				MaskingType: MaskingTypeRedact,
				RedactValue: "***",
				Enabled:     true,
			},
			{
				FeatureName: "phone",
				MaskingType: MaskingTypePartial,
				PartialConfig: &PartialMaskConfig{
					ShowLast: 4,
					MaskChar: "*",
				},
				Enabled: true,
			},
		},
	}
	masker := NewDataMasker(config, nil)

	features := map[string]interface{}{
		"password": "secret123",
		"phone":    "555-123-4567",
		"username": "johndoe",
	}

	ctx := context.Background()
	result := masker.MaskBatch(ctx, features, nil)

	assert.Equal(t, "***", result["password"])
	assert.Contains(t, result["phone"].(string), "4567")
	assert.Equal(t, "johndoe", result["username"])
}

func TestDataMasker_MaskWithPII(t *testing.T) {
	piiConfig := DefaultPIIConfig()
	piiDetector, _ := NewPIIDetector(piiConfig)

	config := MaskingConfig{
		Enabled:     true,
		AutoMaskPII: true,
	}
	masker := NewDataMasker(config, piiDetector)

	detections := []*PIIDetection{
		{
			Category:    PIICategoryEmail,
			Sensitivity: SensitivityMedium,
		},
	}

	ctx := context.Background()
	result, masked := masker.MaskWithPII(ctx, "user_email", "john@example.com", detections, nil)

	assert.True(t, masked)
	assert.NotEqual(t, "john@example.com", result)
}

func TestDataMasker_MaskWithPII_Disabled(t *testing.T) {
	config := MaskingConfig{
		Enabled:     true,
		AutoMaskPII: false,
	}
	masker := NewDataMasker(config, nil)

	detections := []*PIIDetection{
		{
			Category:    PIICategoryEmail,
			Sensitivity: SensitivityMedium,
		},
	}

	ctx := context.Background()
	result, masked := masker.MaskWithPII(ctx, "email", "john@example.com", detections, nil)

	assert.False(t, masked)
	assert.Equal(t, "john@example.com", result)
}

func TestDataMasker_MaskWithPII_NoDetections(t *testing.T) {
	config := MaskingConfig{
		Enabled:            true,
		AutoMaskPII:        true,
		DefaultRedactValue: "[REDACTED]",
		Rules: []MaskingRule{
			{
				FeatureName: "feature",
				MaskingType: MaskingTypeRedact,
				Enabled:     true,
			},
		},
	}
	masker := NewDataMasker(config, nil)

	ctx := context.Background()
	result, masked := masker.MaskWithPII(ctx, "feature", "value", nil, nil)

	assert.True(t, masked)
	assert.Equal(t, "[REDACTED]", result)
}

func TestDataMasker_Detokenize(t *testing.T) {
	config := MaskingConfig{
		Enabled:     true,
		TokenPrefix: "TKN_",
		Rules: []MaskingRule{
			{
				FeatureName: "customer_id",
				MaskingType: MaskingTypeTokenize,
				Enabled:     true,
			},
		},
	}
	masker := NewDataMasker(config, nil)

	ctx := context.Background()
	token, _ := masker.Mask(ctx, "customer_id", "CUST-12345", nil)

	// Detokenize
	original, found := masker.Detokenize(token.(string))
	assert.True(t, found)
	assert.Equal(t, "CUST-12345", original)

	// Unknown token
	_, found = masker.Detokenize("unknown-token")
	assert.False(t, found)
}

func TestDataMasker_AddRule(t *testing.T) {
	config := DefaultMaskingConfig()
	masker := NewDataMasker(config, nil)

	rule := &MaskingRule{
		FeatureName: "new_feature",
		MaskingType: MaskingTypeRedact,
		RedactValue: "***",
		Enabled:     true,
	}

	masker.AddRule(rule)

	ctx := context.Background()
	result, masked := masker.Mask(ctx, "new_feature", "secret", nil)
	assert.True(t, masked)
	assert.Equal(t, "***", result)
}

func TestDataMasker_AddRule_PIICategory(t *testing.T) {
	config := DefaultMaskingConfig()
	masker := NewDataMasker(config, nil)

	rule := &MaskingRule{
		PIICategory: PIICategoryPassport,
		MaskingType: MaskingTypeRedact,
		RedactValue: "[PASSPORT]",
		Enabled:     true,
	}

	masker.AddRule(rule)

	// The rule should be accessible in PII rules
	stats := masker.Stats()
	assert.Greater(t, stats["pii_rules"].(int), 0)
}

func TestDataMasker_RemoveRule(t *testing.T) {
	config := MaskingConfig{
		Enabled: true,
		Rules: []MaskingRule{
			{
				FeatureName: "secret",
				MaskingType: MaskingTypeRedact,
				Enabled:     true,
			},
		},
	}
	masker := NewDataMasker(config, nil)

	// Rule exists
	_, ok := masker.GetRule("secret")
	assert.True(t, ok)

	// Remove rule
	masker.RemoveRule("secret")

	// Rule no longer exists
	_, ok = masker.GetRule("secret")
	assert.False(t, ok)
}

func TestDataMasker_Stats(t *testing.T) {
	config := MaskingConfig{
		Enabled: true,
		Rules: []MaskingRule{
			{
				FeatureName: "feature",
				MaskingType: MaskingTypeRedact,
				Enabled:     true,
			},
		},
	}
	masker := NewDataMasker(config, nil)

	ctx := context.Background()
	_, _ = masker.Mask(ctx, "feature", "value", nil)

	stats := masker.Stats()
	assert.True(t, stats["enabled"].(bool))
	assert.Equal(t, 1, stats["rules"].(int))
	assert.GreaterOrEqual(t, stats["values_masked"].(int64), int64(1))
}

func TestMaskingContext_Fields(t *testing.T) {
	ctx := &MaskingContext{
		UserID:      "user-1",
		TenantID:    "tenant-1",
		Roles:       []string{"admin", "user"},
		Permissions: []string{"read", "write"},
		Purpose:     "analytics",
		RequestID:   "req-123",
	}

	assert.Equal(t, "user-1", ctx.UserID)
	assert.Equal(t, "tenant-1", ctx.TenantID)
	assert.Len(t, ctx.Roles, 2)
	assert.Len(t, ctx.Permissions, 2)
	assert.Equal(t, "analytics", ctx.Purpose)
}

func TestMaskingRule_Fields(t *testing.T) {
	rule := &MaskingRule{
		FeatureName: "secret",
		MaskingType: MaskingTypePartial,
		PIICategory: PIICategorySSN,
		Sensitivity: SensitivityCritical,
		RedactValue: "***",
		PartialConfig: &PartialMaskConfig{
			ShowFirst:      0,
			ShowLast:       4,
			MaskChar:       "*",
			PreserveFormat: true,
		},
		HashSalt: "salt123",
		GeneralizeConfig: &GeneralizeConfig{
			Type:      "range",
			RangeSize: 10,
		},
		Enabled:     true,
		Description: "SSN masking rule",
		AppliesTo:   []string{"all"},
		ExceptFor:   []string{"admin"},
	}

	assert.Equal(t, "secret", rule.FeatureName)
	assert.Equal(t, MaskingTypePartial, rule.MaskingType)
	assert.NotNil(t, rule.PartialConfig)
	assert.NotNil(t, rule.GeneralizeConfig)
}

func TestMaskEmailDomain(t *testing.T) {
	result := MaskEmailDomain("john.doe@example.com")
	assert.Equal(t, "jo***@example.com", result)

	// Short username
	result = MaskEmailDomain("ab@example.com")
	assert.Equal(t, "***@example.com", result)

	// Invalid email
	result = MaskEmailDomain("not-an-email")
	assert.Equal(t, "[REDACTED]", result)
}

func TestMaskPhoneNumber(t *testing.T) {
	result := MaskPhoneNumber("555-123-4567")
	assert.Equal(t, "***-***-4567", result)

	// Short number
	result = MaskPhoneNumber("123")
	assert.Equal(t, "****", result)
}

func TestMaskCreditCard(t *testing.T) {
	result := MaskCreditCard("4111-1111-1111-1111")
	assert.Equal(t, "**** **** **** 1111", result)

	// Short number
	result = MaskCreditCard("111")
	assert.Equal(t, "****", result)
}

func TestMaskSSN(t *testing.T) {
	result := MaskSSN("123-45-6789")
	assert.Equal(t, "***-**-6789", result)

	// Short number
	result = MaskSSN("123")
	assert.Equal(t, "***-**-****", result)
}

func TestIsAlphanumeric(t *testing.T) {
	assert.True(t, isAlphanumeric('a'))
	assert.True(t, isAlphanumeric('Z'))
	assert.True(t, isAlphanumeric('5'))
	assert.False(t, isAlphanumeric('-'))
	assert.False(t, isAlphanumeric(' '))
}

func TestToFloat64(t *testing.T) {
	// Int
	val, ok := toFloat64(42)
	assert.True(t, ok)
	assert.Equal(t, float64(42), val)

	// Int64
	val, ok = toFloat64(int64(100))
	assert.True(t, ok)
	assert.Equal(t, float64(100), val)

	// Float32
	val, ok = toFloat64(float32(3.14))
	assert.True(t, ok)
	assert.InDelta(t, 3.14, val, 0.01)

	// Float64
	val, ok = toFloat64(float64(2.718))
	assert.True(t, ok)
	assert.Equal(t, 2.718, val)

	// Invalid type
	_, ok = toFloat64("string")
	assert.False(t, ok)
}
