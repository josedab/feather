package governance

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Errors returned by PII detection.
var (
	ErrPIIDetectorNotConfigured = errors.New("PII detector not configured")
	ErrInvalidPIIPattern        = errors.New("invalid PII pattern")
)

// PIICategory represents a category of personally identifiable information.
type PIICategory string

// PIICategory constants.
const (
	PIICategoryEmail          PIICategory = "email"
	PIICategoryPhone          PIICategory = "phone"
	PIICategorySSN            PIICategory = "ssn"
	PIICategoryCreditCard     PIICategory = "credit_card"
	PIICategoryIPAddress      PIICategory = "ip_address"
	PIICategoryAddress        PIICategory = "address"
	PIICategoryName           PIICategory = "name"
	PIICategoryDateOfBirth    PIICategory = "date_of_birth"
	PIICategoryPassport       PIICategory = "passport"
	PIICategoryDriversLicense PIICategory = "drivers_license"
	PIICategoryBankAccount    PIICategory = "bank_account"
	PIICategoryHealthInfo     PIICategory = "health_info"
	PIICategoryBiometric      PIICategory = "biometric"
	PIICategoryGeolocation    PIICategory = "geolocation"
	PIICategoryCustom         PIICategory = "custom"
)

// PIISensitivity indicates the sensitivity level of PII.
type PIISensitivity string

// PIISensitivity constants.
const (
	SensitivityLow      PIISensitivity = "low"
	SensitivityMedium   PIISensitivity = "medium"
	SensitivityHigh     PIISensitivity = "high"
	SensitivityCritical PIISensitivity = "critical"
)

// PIIDetection represents a detected PII instance.
type PIIDetection struct {
	// Category is the PII category.
	Category PIICategory `json:"category"`

	// Sensitivity is the sensitivity level.
	Sensitivity PIISensitivity `json:"sensitivity"`

	// FeatureName is the feature containing PII.
	FeatureName string `json:"feature_name"`

	// Confidence is the detection confidence (0-1).
	Confidence float64 `json:"confidence"`

	// MatchedPattern is the pattern that matched.
	MatchedPattern string `json:"matched_pattern,omitempty"`

	// SampleValue is a masked sample of the detected value.
	SampleValue string `json:"sample_value,omitempty"`

	// Count is the number of instances detected.
	Count int64 `json:"count"`

	// FirstDetected is when this PII was first detected.
	FirstDetected time.Time `json:"first_detected"`

	// LastDetected is when this PII was last detected.
	LastDetected time.Time `json:"last_detected"`
}

// PIIPattern defines a pattern for detecting PII.
type PIIPattern struct {
	// Name is the pattern identifier.
	Name string `json:"name"`

	// Category is the PII category.
	Category PIICategory `json:"category"`

	// Sensitivity is the sensitivity level.
	Sensitivity PIISensitivity `json:"sensitivity"`

	// Regex is the detection regex pattern.
	Regex string `json:"regex"`

	// compiled is the compiled regex.
	compiled *regexp.Regexp

	// Keywords are trigger keywords.
	Keywords []string `json:"keywords,omitempty"`

	// Enabled indicates if the pattern is active.
	Enabled bool `json:"enabled"`

	// Description describes the pattern.
	Description string `json:"description,omitempty"`
}

// PIIConfig configures the PII detector.
type PIIConfig struct {
	// Enabled enables PII detection.
	Enabled bool `json:"enabled" yaml:"enabled"`

	// ScanOnWrite scans data during write operations.
	ScanOnWrite bool `json:"scan_on_write" yaml:"scan_on_write"`

	// ScanOnRead scans data during read operations.
	ScanOnRead bool `json:"scan_on_read" yaml:"scan_on_read"`

	// ScanInterval is the background scan interval.
	ScanInterval time.Duration `json:"scan_interval" yaml:"scan_interval"`

	// BlockOnDetection blocks operations when PII is detected.
	BlockOnDetection bool `json:"block_on_detection" yaml:"block_on_detection"`

	// MinSensitivityToBlock is the minimum sensitivity to block.
	MinSensitivityToBlock PIISensitivity `json:"min_sensitivity_to_block" yaml:"min_sensitivity_to_block"`

	// CustomPatterns are additional patterns to detect.
	CustomPatterns []PIIPattern `json:"custom_patterns" yaml:"custom_patterns"`

	// ExcludedFeatures are features to skip scanning.
	ExcludedFeatures []string `json:"excluded_features" yaml:"excluded_features"`

	// SampleRate is the sampling rate for detection (0-1).
	SampleRate float64 `json:"sample_rate" yaml:"sample_rate"`
}

// DefaultPIIConfig returns the default PII configuration.
func DefaultPIIConfig() PIIConfig {
	return PIIConfig{
		Enabled:               true,
		ScanOnWrite:           true,
		ScanOnRead:            false,
		ScanInterval:          24 * time.Hour,
		BlockOnDetection:      false,
		MinSensitivityToBlock: SensitivityCritical,
		SampleRate:            1.0,
	}
}

// PIIDetector detects PII in feature data.
type PIIDetector struct {
	mu         sync.RWMutex
	config     PIIConfig
	patterns   []*PIIPattern
	detections map[string]*PIIDetection // key: "feature:category"
	excluded   map[string]bool

	// Metrics
	scansPerformed int64
	piiDetected    int64
	bytesScanned   int64

	// Callbacks
	onDetection []func(*PIIDetection)
}

// NewPIIDetector creates a new PII detector.
func NewPIIDetector(config PIIConfig) (*PIIDetector, error) {
	d := &PIIDetector{
		config:     config,
		patterns:   make([]*PIIPattern, 0),
		detections: make(map[string]*PIIDetection),
		excluded:   make(map[string]bool),
	}

	// Initialize excluded features
	for _, f := range config.ExcludedFeatures {
		d.excluded[f] = true
	}

	// Load default patterns
	if err := d.loadDefaultPatterns(); err != nil {
		return nil, err
	}

	// Load custom patterns
	for _, p := range config.CustomPatterns {
		if err := d.AddPattern(&p); err != nil {
			return nil, err
		}
	}

	return d, nil
}

// loadDefaultPatterns loads the built-in PII patterns.
func (d *PIIDetector) loadDefaultPatterns() error {
	defaultPatterns := []PIIPattern{
		{
			Name:        "email",
			Category:    PIICategoryEmail,
			Sensitivity: SensitivityMedium,
			Regex:       `[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`,
			Enabled:     true,
			Description: "Email addresses",
		},
		{
			Name:        "phone_us",
			Category:    PIICategoryPhone,
			Sensitivity: SensitivityMedium,
			Regex:       `(\+1[-.\s]?)?\(?[2-9]\d{2}\)?[-.\s]?\d{3}[-.\s]?\d{4}`,
			Enabled:     true,
			Description: "US phone numbers",
		},
		{
			Name:        "ssn",
			Category:    PIICategorySSN,
			Sensitivity: SensitivityCritical,
			Regex:       `\b\d{3}[-\s]?\d{2}[-\s]?\d{4}\b`,
			Keywords:    []string{"ssn", "social", "security"},
			Enabled:     true,
			Description: "US Social Security Numbers",
		},
		{
			Name:        "credit_card",
			Category:    PIICategoryCreditCard,
			Sensitivity: SensitivityCritical,
			Regex:       `\b(?:4[0-9]{12}(?:[0-9]{3})?|5[1-5][0-9]{14}|3[47][0-9]{13}|6(?:011|5[0-9]{2})[0-9]{12})\b`,
			Enabled:     true,
			Description: "Credit card numbers (Visa, MC, Amex, Discover)",
		},
		{
			Name:        "ip_address",
			Category:    PIICategoryIPAddress,
			Sensitivity: SensitivityLow,
			Regex:       `\b(?:(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.){3}(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\b`,
			Enabled:     true,
			Description: "IPv4 addresses",
		},
		{
			Name:        "date_of_birth",
			Category:    PIICategoryDateOfBirth,
			Sensitivity: SensitivityMedium,
			Regex:       `\b(?:0?[1-9]|1[0-2])[/\-](?:0?[1-9]|[12]\d|3[01])[/\-](?:19|20)\d{2}\b`,
			Keywords:    []string{"dob", "birth", "birthday"},
			Enabled:     true,
			Description: "Dates of birth",
		},
		{
			Name:        "passport",
			Category:    PIICategoryPassport,
			Sensitivity: SensitivityHigh,
			Regex:       `\b[A-Z]{1,2}[0-9]{6,9}\b`,
			Keywords:    []string{"passport"},
			Enabled:     true,
			Description: "Passport numbers",
		},
		{
			Name:        "geolocation",
			Category:    PIICategoryGeolocation,
			Sensitivity: SensitivityMedium,
			Regex:       `[-+]?([1-8]?\d(\.\d+)?|90(\.0+)?),\s*[-+]?(180(\.0+)?|((1[0-7]\d)|([1-9]?\d))(\.\d+)?)`,
			Keywords:    []string{"lat", "lng", "latitude", "longitude", "coordinates"},
			Enabled:     true,
			Description: "Geographic coordinates",
		},
	}

	for _, p := range defaultPatterns {
		pattern := p
		if err := d.AddPattern(&pattern); err != nil {
			return err
		}
	}

	return nil
}

// AddPattern adds a detection pattern.
func (d *PIIDetector) AddPattern(pattern *PIIPattern) error {
	if pattern.Regex == "" {
		return ErrInvalidPIIPattern
	}

	compiled, err := regexp.Compile(pattern.Regex)
	if err != nil {
		return err
	}

	pattern.compiled = compiled

	d.mu.Lock()
	d.patterns = append(d.patterns, pattern)
	d.mu.Unlock()

	return nil
}

// Scan scans a value for PII.
func (d *PIIDetector) Scan(ctx context.Context, featureName string, value interface{}) ([]*PIIDetection, error) {
	if !d.config.Enabled {
		return nil, nil
	}

	// Check if excluded
	d.mu.RLock()
	if d.excluded[featureName] {
		d.mu.RUnlock()
		return nil, nil
	}
	d.mu.RUnlock()

	// Convert value to string
	strValue := valueToString(value)
	if strValue == "" {
		return nil, nil
	}

	atomic.AddInt64(&d.scansPerformed, 1)
	atomic.AddInt64(&d.bytesScanned, int64(len(strValue)))

	var detections []*PIIDetection

	d.mu.RLock()
	patterns := d.patterns
	d.mu.RUnlock()

	for _, pattern := range patterns {
		if !pattern.Enabled {
			continue
		}

		// Check keywords first (faster)
		if len(pattern.Keywords) > 0 {
			featureLower := strings.ToLower(featureName)
			keywordMatch := false
			for _, kw := range pattern.Keywords {
				if strings.Contains(featureLower, kw) {
					keywordMatch = true
					break
				}
			}
			if !keywordMatch {
				continue
			}
		}

		// Check regex
		if pattern.compiled != nil && pattern.compiled.MatchString(strValue) {
			detection := &PIIDetection{
				Category:       pattern.Category,
				Sensitivity:    pattern.Sensitivity,
				FeatureName:    featureName,
				Confidence:     0.9,
				MatchedPattern: pattern.Name,
				SampleValue:    maskSample(strValue),
				Count:          1,
				FirstDetected:  time.Now(),
				LastDetected:   time.Now(),
			}

			detections = append(detections, detection)
			atomic.AddInt64(&d.piiDetected, 1)

			// Update detection tracking
			d.trackDetection(detection)

			// Trigger callbacks
			d.triggerCallbacks(detection)
		}
	}

	return detections, nil
}

// ScanBatch scans multiple features for PII.
func (d *PIIDetector) ScanBatch(ctx context.Context, features map[string]interface{}) (map[string][]*PIIDetection, error) {
	results := make(map[string][]*PIIDetection)

	for name, value := range features {
		select {
		case <-ctx.Done():
			return results, ctx.Err()
		default:
		}

		detections, err := d.Scan(ctx, name, value)
		if err != nil {
			continue
		}
		if len(detections) > 0 {
			results[name] = detections
		}
	}

	return results, nil
}

// ShouldBlock determines if an operation should be blocked.
func (d *PIIDetector) ShouldBlock(detections []*PIIDetection) bool {
	if !d.config.BlockOnDetection {
		return false
	}

	sensitivityOrder := map[PIISensitivity]int{
		SensitivityLow:      0,
		SensitivityMedium:   1,
		SensitivityHigh:     2,
		SensitivityCritical: 3,
	}

	minLevel := sensitivityOrder[d.config.MinSensitivityToBlock]

	for _, det := range detections {
		if sensitivityOrder[det.Sensitivity] >= minLevel {
			return true
		}
	}

	return false
}

// trackDetection updates detection tracking.
func (d *PIIDetector) trackDetection(detection *PIIDetection) {
	key := detection.FeatureName + ":" + string(detection.Category)

	d.mu.Lock()
	defer d.mu.Unlock()

	if existing, ok := d.detections[key]; ok {
		existing.Count++
		existing.LastDetected = time.Now()
	} else {
		d.detections[key] = detection
	}
}

// triggerCallbacks notifies registered callbacks.
func (d *PIIDetector) triggerCallbacks(detection *PIIDetection) {
	d.mu.RLock()
	callbacks := d.onDetection
	d.mu.RUnlock()

	for _, cb := range callbacks {
		cb(detection)
	}
}

// OnDetection registers a callback for PII detections.
func (d *PIIDetector) OnDetection(callback func(*PIIDetection)) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.onDetection = append(d.onDetection, callback)
}

// GetDetections returns all tracked detections.
func (d *PIIDetector) GetDetections() []*PIIDetection {
	d.mu.RLock()
	defer d.mu.RUnlock()

	detections := make([]*PIIDetection, 0, len(d.detections))
	for _, det := range d.detections {
		detections = append(detections, det)
	}

	return detections
}

// GetDetectionsByFeature returns detections for a specific feature.
func (d *PIIDetector) GetDetectionsByFeature(featureName string) []*PIIDetection {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var detections []*PIIDetection
	for key, det := range d.detections {
		if strings.HasPrefix(key, featureName+":") {
			detections = append(detections, det)
		}
	}

	return detections
}

// ClassifyFeature classifies a feature based on its detected PII.
func (d *PIIDetector) ClassifyFeature(featureName string) (PIICategory, PIISensitivity, bool) {
	detections := d.GetDetectionsByFeature(featureName)
	if len(detections) == 0 {
		return "", "", false
	}

	// Return the highest sensitivity detection
	highestSensitivity := SensitivityLow
	var category PIICategory

	sensitivityOrder := map[PIISensitivity]int{
		SensitivityLow:      0,
		SensitivityMedium:   1,
		SensitivityHigh:     2,
		SensitivityCritical: 3,
	}

	for _, det := range detections {
		if sensitivityOrder[det.Sensitivity] > sensitivityOrder[highestSensitivity] {
			highestSensitivity = det.Sensitivity
			category = det.Category
		}
	}

	return category, highestSensitivity, true
}

// ExcludeFeature excludes a feature from scanning.
func (d *PIIDetector) ExcludeFeature(featureName string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.excluded[featureName] = true
}

// IncludeFeature removes a feature from the exclusion list.
func (d *PIIDetector) IncludeFeature(featureName string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	delete(d.excluded, featureName)
}

// Stats returns PII detector statistics.
func (d *PIIDetector) Stats() map[string]interface{} {
	d.mu.RLock()
	patternCount := len(d.patterns)
	detectionCount := len(d.detections)
	d.mu.RUnlock()

	return map[string]interface{}{
		"enabled":         d.config.Enabled,
		"patterns":        patternCount,
		"detections":      detectionCount,
		"scans_performed": atomic.LoadInt64(&d.scansPerformed),
		"pii_detected":    atomic.LoadInt64(&d.piiDetected),
		"bytes_scanned":   atomic.LoadInt64(&d.bytesScanned),
	}
}

// valueToString converts a value to string for scanning.
func valueToString(value interface{}) string {
	switch v := value.(type) {
	case string:
		return v
	case []byte:
		return string(v)
	case nil:
		return ""
	default:
		return ""
	}
}

// maskSample masks a sample value for safe display.
func maskSample(value string) string {
	if len(value) <= 4 {
		return "****"
	}
	return value[:2] + strings.Repeat("*", len(value)-4) + value[len(value)-2:]
}
