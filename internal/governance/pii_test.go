package governance

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultPIIConfig(t *testing.T) {
	config := DefaultPIIConfig()

	assert.True(t, config.Enabled)
	assert.True(t, config.ScanOnWrite)
	assert.False(t, config.ScanOnRead)
	assert.Equal(t, 24*time.Hour, config.ScanInterval)
	assert.False(t, config.BlockOnDetection)
	assert.Equal(t, SensitivityCritical, config.MinSensitivityToBlock)
	assert.Equal(t, 1.0, config.SampleRate)
}

func TestPIICategory_Values(t *testing.T) {
	assert.Equal(t, PIICategory("email"), PIICategoryEmail)
	assert.Equal(t, PIICategory("phone"), PIICategoryPhone)
	assert.Equal(t, PIICategory("ssn"), PIICategorySSN)
	assert.Equal(t, PIICategory("credit_card"), PIICategoryCreditCard)
	assert.Equal(t, PIICategory("ip_address"), PIICategoryIPAddress)
	assert.Equal(t, PIICategory("address"), PIICategoryAddress)
	assert.Equal(t, PIICategory("name"), PIICategoryName)
	assert.Equal(t, PIICategory("date_of_birth"), PIICategoryDateOfBirth)
	assert.Equal(t, PIICategory("passport"), PIICategoryPassport)
	assert.Equal(t, PIICategory("drivers_license"), PIICategoryDriversLicense)
	assert.Equal(t, PIICategory("bank_account"), PIICategoryBankAccount)
	assert.Equal(t, PIICategory("health_info"), PIICategoryHealthInfo)
	assert.Equal(t, PIICategory("biometric"), PIICategoryBiometric)
	assert.Equal(t, PIICategory("geolocation"), PIICategoryGeolocation)
	assert.Equal(t, PIICategory("custom"), PIICategoryCustom)
}

func TestPIISensitivity_Values(t *testing.T) {
	assert.Equal(t, PIISensitivity("low"), SensitivityLow)
	assert.Equal(t, PIISensitivity("medium"), SensitivityMedium)
	assert.Equal(t, PIISensitivity("high"), SensitivityHigh)
	assert.Equal(t, PIISensitivity("critical"), SensitivityCritical)
}

func TestNewPIIDetector(t *testing.T) {
	config := DefaultPIIConfig()

	detector, err := NewPIIDetector(config)
	require.NoError(t, err)
	require.NotNil(t, detector)

	// Should have default patterns loaded
	stats := detector.Stats()
	assert.Greater(t, stats["patterns"].(int), 0)
}

func TestNewPIIDetector_WithCustomPatterns(t *testing.T) {
	config := PIIConfig{
		Enabled: true,
		CustomPatterns: []PIIPattern{
			{
				Name:        "employee_id",
				Category:    PIICategoryCustom,
				Sensitivity: SensitivityMedium,
				Regex:       `EMP-\d{6}`,
				Enabled:     true,
			},
		},
	}

	detector, err := NewPIIDetector(config)
	require.NoError(t, err)
	require.NotNil(t, detector)
}

func TestNewPIIDetector_InvalidPattern(t *testing.T) {
	config := PIIConfig{
		Enabled: true,
		CustomPatterns: []PIIPattern{
			{
				Name:        "invalid",
				Category:    PIICategoryCustom,
				Sensitivity: SensitivityMedium,
				Regex:       `[invalid(regex`, // Invalid regex
				Enabled:     true,
			},
		},
	}

	_, err := NewPIIDetector(config)
	assert.Error(t, err)
}

func TestPIIDetector_Scan_Email(t *testing.T) {
	config := DefaultPIIConfig()
	detector, err := NewPIIDetector(config)
	require.NoError(t, err)

	ctx := context.Background()
	detections, err := detector.Scan(ctx, "user_email", "john.doe@example.com")
	require.NoError(t, err)

	assert.Len(t, detections, 1)
	assert.Equal(t, PIICategoryEmail, detections[0].Category)
	assert.Equal(t, SensitivityMedium, detections[0].Sensitivity)
	assert.Equal(t, "user_email", detections[0].FeatureName)
}

func TestPIIDetector_Scan_Phone(t *testing.T) {
	config := DefaultPIIConfig()
	detector, err := NewPIIDetector(config)
	require.NoError(t, err)

	ctx := context.Background()
	detections, err := detector.Scan(ctx, "contact_phone", "(555) 123-4567")
	require.NoError(t, err)

	assert.Len(t, detections, 1)
	assert.Equal(t, PIICategoryPhone, detections[0].Category)
}

func TestPIIDetector_Scan_SSN(t *testing.T) {
	config := DefaultPIIConfig()
	detector, err := NewPIIDetector(config)
	require.NoError(t, err)

	ctx := context.Background()
	// SSN pattern requires keyword match
	detections, err := detector.Scan(ctx, "ssn_number", "123-45-6789")
	require.NoError(t, err)

	assert.Len(t, detections, 1)
	assert.Equal(t, PIICategorySSN, detections[0].Category)
	assert.Equal(t, SensitivityCritical, detections[0].Sensitivity)
}

func TestPIIDetector_Scan_CreditCard(t *testing.T) {
	config := DefaultPIIConfig()
	detector, err := NewPIIDetector(config)
	require.NoError(t, err)

	ctx := context.Background()

	// Visa - also matches phone pattern, so check for credit card detection
	detections, err := detector.Scan(ctx, "card_number", "4111111111111111")
	require.NoError(t, err)
	assert.NotEmpty(t, detections)

	// Find the credit card detection
	var foundCreditCard bool
	for _, det := range detections {
		if det.Category == PIICategoryCreditCard {
			foundCreditCard = true
			break
		}
	}
	assert.True(t, foundCreditCard, "Should detect credit card")
}

func TestPIIDetector_Scan_IPAddress(t *testing.T) {
	config := DefaultPIIConfig()
	detector, err := NewPIIDetector(config)
	require.NoError(t, err)

	ctx := context.Background()
	detections, err := detector.Scan(ctx, "client_ip", "192.168.1.1")
	require.NoError(t, err)

	assert.Len(t, detections, 1)
	assert.Equal(t, PIICategoryIPAddress, detections[0].Category)
	assert.Equal(t, SensitivityLow, detections[0].Sensitivity)
}

func TestPIIDetector_Scan_Disabled(t *testing.T) {
	config := PIIConfig{
		Enabled: false,
	}
	detector, err := NewPIIDetector(config)
	require.NoError(t, err)

	ctx := context.Background()
	detections, err := detector.Scan(ctx, "user_email", "john@example.com")
	require.NoError(t, err)
	assert.Empty(t, detections)
}

func TestPIIDetector_Scan_ExcludedFeature(t *testing.T) {
	config := PIIConfig{
		Enabled:          true,
		ExcludedFeatures: []string{"safe_email"},
	}
	detector, err := NewPIIDetector(config)
	require.NoError(t, err)

	ctx := context.Background()
	detections, err := detector.Scan(ctx, "safe_email", "john@example.com")
	require.NoError(t, err)
	assert.Empty(t, detections)
}

func TestPIIDetector_Scan_EmptyValue(t *testing.T) {
	config := DefaultPIIConfig()
	detector, err := NewPIIDetector(config)
	require.NoError(t, err)

	ctx := context.Background()
	detections, err := detector.Scan(ctx, "feature", "")
	require.NoError(t, err)
	assert.Empty(t, detections)
}

func TestPIIDetector_Scan_NilValue(t *testing.T) {
	config := DefaultPIIConfig()
	detector, err := NewPIIDetector(config)
	require.NoError(t, err)

	ctx := context.Background()
	detections, err := detector.Scan(ctx, "feature", nil)
	require.NoError(t, err)
	assert.Empty(t, detections)
}

func TestPIIDetector_Scan_ByteSlice(t *testing.T) {
	config := DefaultPIIConfig()
	detector, err := NewPIIDetector(config)
	require.NoError(t, err)

	ctx := context.Background()
	detections, err := detector.Scan(ctx, "email_field", []byte("john@example.com"))
	require.NoError(t, err)
	assert.Len(t, detections, 1)
}

func TestPIIDetector_ScanBatch(t *testing.T) {
	config := DefaultPIIConfig()
	detector, err := NewPIIDetector(config)
	require.NoError(t, err)

	features := map[string]interface{}{
		"user_email":  "john@example.com",
		"contact_ssn": "123-45-6789",
		"safe_value":  "no pii here",
		"client_ip":   "192.168.1.1",
	}

	ctx := context.Background()
	results, err := detector.ScanBatch(ctx, features)
	require.NoError(t, err)

	assert.Len(t, results["user_email"], 1)
	assert.Len(t, results["contact_ssn"], 1)
	assert.Len(t, results["client_ip"], 1)
	assert.Empty(t, results["safe_value"])
}

func TestPIIDetector_ScanBatch_ContextCancelled(t *testing.T) {
	config := DefaultPIIConfig()
	detector, err := NewPIIDetector(config)
	require.NoError(t, err)

	features := map[string]interface{}{
		"email1": "a@example.com",
		"email2": "b@example.com",
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	results, err := detector.ScanBatch(ctx, features)
	assert.Equal(t, context.Canceled, err)
	assert.NotNil(t, results)
}

func TestPIIDetector_ShouldBlock(t *testing.T) {
	config := PIIConfig{
		Enabled:               true,
		BlockOnDetection:      true,
		MinSensitivityToBlock: SensitivityHigh,
	}
	detector, err := NewPIIDetector(config)
	require.NoError(t, err)

	// Low sensitivity should not block
	lowDetections := []*PIIDetection{
		{Sensitivity: SensitivityLow},
	}
	assert.False(t, detector.ShouldBlock(lowDetections))

	// High sensitivity should block
	highDetections := []*PIIDetection{
		{Sensitivity: SensitivityHigh},
	}
	assert.True(t, detector.ShouldBlock(highDetections))

	// Critical should block
	criticalDetections := []*PIIDetection{
		{Sensitivity: SensitivityCritical},
	}
	assert.True(t, detector.ShouldBlock(criticalDetections))
}

func TestPIIDetector_ShouldBlock_Disabled(t *testing.T) {
	config := PIIConfig{
		Enabled:          true,
		BlockOnDetection: false,
	}
	detector, err := NewPIIDetector(config)
	require.NoError(t, err)

	detections := []*PIIDetection{
		{Sensitivity: SensitivityCritical},
	}
	assert.False(t, detector.ShouldBlock(detections))
}

func TestPIIDetector_AddPattern(t *testing.T) {
	config := DefaultPIIConfig()
	detector, err := NewPIIDetector(config)
	require.NoError(t, err)

	pattern := &PIIPattern{
		Name:        "custom_id",
		Category:    PIICategoryCustom,
		Sensitivity: SensitivityMedium,
		Regex:       `CUST-\d{8}`,
		Enabled:     true,
	}

	err = detector.AddPattern(pattern)
	require.NoError(t, err)

	// Test detection with new pattern
	ctx := context.Background()
	detections, err := detector.Scan(ctx, "customer", "CUST-12345678")
	require.NoError(t, err)
	assert.Len(t, detections, 1)
	assert.Equal(t, PIICategoryCustom, detections[0].Category)
}

func TestPIIDetector_AddPattern_EmptyRegex(t *testing.T) {
	config := DefaultPIIConfig()
	detector, err := NewPIIDetector(config)
	require.NoError(t, err)

	pattern := &PIIPattern{
		Name:        "invalid",
		Category:    PIICategoryCustom,
		Sensitivity: SensitivityMedium,
		Regex:       "",
		Enabled:     true,
	}

	err = detector.AddPattern(pattern)
	assert.ErrorIs(t, err, ErrInvalidPIIPattern)
}

func TestPIIDetector_OnDetection(t *testing.T) {
	config := DefaultPIIConfig()
	detector, err := NewPIIDetector(config)
	require.NoError(t, err)

	var receivedDetection *PIIDetection
	detector.OnDetection(func(d *PIIDetection) {
		receivedDetection = d
	})

	ctx := context.Background()
	_, err = detector.Scan(ctx, "email", "test@example.com")
	require.NoError(t, err)

	assert.NotNil(t, receivedDetection)
	assert.Equal(t, PIICategoryEmail, receivedDetection.Category)
}

func TestPIIDetector_GetDetections(t *testing.T) {
	config := DefaultPIIConfig()
	detector, err := NewPIIDetector(config)
	require.NoError(t, err)

	ctx := context.Background()
	_, _ = detector.Scan(ctx, "email1", "a@example.com")
	_, _ = detector.Scan(ctx, "email2", "b@example.com")

	detections := detector.GetDetections()
	assert.GreaterOrEqual(t, len(detections), 2)
}

func TestPIIDetector_GetDetectionsByFeature(t *testing.T) {
	config := DefaultPIIConfig()
	detector, err := NewPIIDetector(config)
	require.NoError(t, err)

	ctx := context.Background()
	_, _ = detector.Scan(ctx, "user_email", "test@example.com")
	_, _ = detector.Scan(ctx, "other_feature", "192.168.1.1")

	detections := detector.GetDetectionsByFeature("user_email")
	assert.Len(t, detections, 1)
	assert.Equal(t, "user_email", detections[0].FeatureName)
}

func TestPIIDetector_ClassifyFeature(t *testing.T) {
	config := DefaultPIIConfig()
	detector, err := NewPIIDetector(config)
	require.NoError(t, err)

	ctx := context.Background()
	_, _ = detector.Scan(ctx, "user_ssn", "123-45-6789")

	category, sensitivity, hasPII := detector.ClassifyFeature("user_ssn")
	assert.True(t, hasPII)
	assert.Equal(t, PIICategorySSN, category)
	assert.Equal(t, SensitivityCritical, sensitivity)
}

func TestPIIDetector_ClassifyFeature_NoPII(t *testing.T) {
	config := DefaultPIIConfig()
	detector, err := NewPIIDetector(config)
	require.NoError(t, err)

	category, sensitivity, hasPII := detector.ClassifyFeature("unknown_feature")
	assert.False(t, hasPII)
	assert.Empty(t, category)
	assert.Empty(t, sensitivity)
}

func TestPIIDetector_ExcludeFeature(t *testing.T) {
	config := DefaultPIIConfig()
	detector, err := NewPIIDetector(config)
	require.NoError(t, err)

	// Exclude feature
	detector.ExcludeFeature("excluded_email")

	ctx := context.Background()
	detections, err := detector.Scan(ctx, "excluded_email", "test@example.com")
	require.NoError(t, err)
	assert.Empty(t, detections)
}

func TestPIIDetector_IncludeFeature(t *testing.T) {
	config := PIIConfig{
		Enabled:          true,
		ExcludedFeatures: []string{"test_email"},
	}
	detector, err := NewPIIDetector(config)
	require.NoError(t, err)

	// Feature is excluded
	ctx := context.Background()
	detections, err := detector.Scan(ctx, "test_email", "test@example.com")
	require.NoError(t, err)
	assert.Empty(t, detections)

	// Include feature
	detector.IncludeFeature("test_email")

	detections, err = detector.Scan(ctx, "test_email", "test@example.com")
	require.NoError(t, err)
	assert.Len(t, detections, 1)
}

func TestPIIDetector_Stats(t *testing.T) {
	config := DefaultPIIConfig()
	detector, err := NewPIIDetector(config)
	require.NoError(t, err)

	ctx := context.Background()
	_, _ = detector.Scan(ctx, "email", "test@example.com")

	stats := detector.Stats()
	assert.True(t, stats["enabled"].(bool))
	assert.Greater(t, stats["patterns"].(int), 0)
	assert.GreaterOrEqual(t, stats["scans_performed"].(int64), int64(1))
}

func TestPIIDetection_Fields(t *testing.T) {
	detection := &PIIDetection{
		Category:      PIICategoryEmail,
		Sensitivity:   SensitivityMedium,
		Confidence:    0.95,
		Count:         5,
		FirstDetected: time.Time{},
	}

	assert.Equal(t, PIICategoryEmail, detection.Category)
	assert.Equal(t, SensitivityMedium, detection.Sensitivity)
	assert.Equal(t, 0.95, detection.Confidence)
	assert.Equal(t, int64(5), detection.Count)
}

func TestPIIPattern_Fields(t *testing.T) {
	pattern := &PIIPattern{
		Name:     "custom_pattern",
		Category: PIICategoryCustom,
		Keywords: []string{"custom"},
	}

	assert.Equal(t, "custom_pattern", pattern.Name)
	assert.Equal(t, PIICategoryCustom, pattern.Category)
	assert.Len(t, pattern.Keywords, 1)
}

func TestMaskSample(t *testing.T) {
	// Short value
	result := maskSample("abc")
	assert.Equal(t, "****", result)

	// Normal value
	result = maskSample("test@example.com")
	assert.Equal(t, "te************om", result)
}

func TestValueToString(t *testing.T) {
	// String
	result := valueToString("hello")
	assert.Equal(t, "hello", result)

	// Bytes
	result = valueToString([]byte("world"))
	assert.Equal(t, "world", result)

	// Nil
	result = valueToString(nil)
	assert.Equal(t, "", result)

	// Unknown type
	result = valueToString(12345)
	assert.Equal(t, "", result)
}
