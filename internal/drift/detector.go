// Package drift provides feature drift detection for MLOps monitoring.
// It detects when feature distributions change over time, which can indicate
// data quality issues or model degradation.
package drift

import (
	"math"
	"sort"
	"sync"
	"time"
)

// Detector monitors feature distributions and detects drift.
type Detector struct {
	mu         sync.RWMutex
	monitors   map[string]*FeatureMonitor
	config     Config
	alerts     []Alert
	alertLimit int
}

// Config configures the drift detector.
type Config struct {
	// WindowSize is the number of samples to keep for comparison
	WindowSize int

	// ReferenceSize is the number of samples for the reference distribution
	ReferenceSize int

	// Thresholds for different tests
	KSThreshold  float64 // Kolmogorov-Smirnov test threshold
	PSIThreshold float64 // Population Stability Index threshold

	// AlertCooldown prevents repeated alerts
	AlertCooldown time.Duration
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() Config {
	return Config{
		WindowSize:    1000,
		ReferenceSize: 10000,
		KSThreshold:   0.1,
		PSIThreshold:  0.2,
		AlertCooldown: 5 * time.Minute,
	}
}

// FeatureMonitor tracks a single feature's distribution.
type FeatureMonitor struct {
	Name        string
	Type        FeatureType
	Reference   *Distribution
	Current     *Distribution
	LastAlert   time.Time
	DriftScore  float64
	DriftType   DriftType
	SampleCount int64
}

// FeatureType indicates the data type of the feature.
type FeatureType int

// FeatureType constants.
const (
	TypeNumeric FeatureType = iota
	TypeCategorical
)

// DriftType indicates the type of drift detected.
type DriftType int //nolint:revive

// DriftType constants.
const (
	DriftNone     DriftType = iota
	DriftKS                 // Kolmogorov-Smirnov
	DriftPSI                // Population Stability Index
	DriftMean               // Mean shift
	DriftVariance           // Variance change
)

func (d DriftType) String() string {
	switch d {
	case DriftNone:
		return "none"
	case DriftKS:
		return "ks"
	case DriftPSI:
		return "psi"
	case DriftMean:
		return "mean"
	case DriftVariance:
		return "variance"
	default:
		return "unknown"
	}
}

// Distribution represents a feature's distribution.
type Distribution struct {
	// Numeric features
	Values []float64
	Mean   float64
	StdDev float64
	Min    float64
	Max    float64

	// Categorical features
	Categories map[string]int
	Total      int

	// Histogram for PSI
	Buckets     []int
	BucketEdges []float64
}

// Alert represents a drift alert.
type Alert struct {
	Feature   string
	Type      DriftType
	Score     float64
	Threshold float64
	Timestamp time.Time
	Message   string
}

// NewDetector creates a new drift detector.
func NewDetector(config Config) *Detector {
	if config.WindowSize == 0 {
		config = DefaultConfig()
	}

	return &Detector{
		monitors:   make(map[string]*FeatureMonitor),
		config:     config,
		alerts:     make([]Alert, 0),
		alertLimit: 1000,
	}
}

// RegisterFeature registers a feature for monitoring.
func (d *Detector) RegisterFeature(name string, featureType FeatureType) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.monitors[name] = &FeatureMonitor{
		Name:      name,
		Type:      featureType,
		Reference: &Distribution{Categories: make(map[string]int)},
		Current:   &Distribution{Categories: make(map[string]int)},
	}
}

// RecordNumeric records a numeric feature value.
func (d *Detector) RecordNumeric(name string, value float64) {
	d.mu.Lock()
	defer d.mu.Unlock()

	monitor, exists := d.monitors[name]
	if !exists {
		// Auto-register as numeric
		monitor = &FeatureMonitor{
			Name:      name,
			Type:      TypeNumeric,
			Reference: &Distribution{Categories: make(map[string]int)},
			Current:   &Distribution{Categories: make(map[string]int)},
		}
		d.monitors[name] = monitor
	}

	monitor.SampleCount++

	// Add to current distribution
	monitor.Current.Values = append(monitor.Current.Values, value)

	// Maintain window size
	if len(monitor.Current.Values) > d.config.WindowSize {
		monitor.Current.Values = monitor.Current.Values[1:]
	}

	// Build reference if not enough samples
	if len(monitor.Reference.Values) < d.config.ReferenceSize {
		monitor.Reference.Values = append(monitor.Reference.Values, value)
		monitor.Reference.updateStats()
	}

	monitor.Current.updateStats()

	// Check for drift periodically
	if monitor.SampleCount%100 == 0 && len(monitor.Reference.Values) >= 100 {
		d.checkDrift(monitor)
	}
}

// RecordCategorical records a categorical feature value.
func (d *Detector) RecordCategorical(name string, category string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	monitor, exists := d.monitors[name]
	if !exists {
		monitor = &FeatureMonitor{
			Name:      name,
			Type:      TypeCategorical,
			Reference: &Distribution{Categories: make(map[string]int)},
			Current:   &Distribution{Categories: make(map[string]int)},
		}
		d.monitors[name] = monitor
	}

	monitor.SampleCount++

	// Add to current distribution
	monitor.Current.Categories[category]++
	monitor.Current.Total++

	// Build reference
	if monitor.Reference.Total < d.config.ReferenceSize {
		monitor.Reference.Categories[category]++
		monitor.Reference.Total++
	}

	// Check for drift periodically
	if monitor.SampleCount%100 == 0 && monitor.Reference.Total >= 100 {
		d.checkCategoricalDrift(monitor)
	}
}

// checkDrift performs drift detection for numeric features.
func (d *Detector) checkDrift(monitor *FeatureMonitor) {
	if len(monitor.Current.Values) < 50 {
		return
	}

	// Kolmogorov-Smirnov test
	ksScore := d.ksTest(monitor.Reference.Values, monitor.Current.Values)
	if ksScore > d.config.KSThreshold {
		monitor.DriftScore = ksScore
		monitor.DriftType = DriftKS
		d.maybeAlert(monitor, DriftKS, ksScore, d.config.KSThreshold)
		return
	}

	// Mean shift detection
	if len(monitor.Reference.Values) > 0 {
		refMean := monitor.Reference.Mean
		curMean := monitor.Current.Mean
		refStd := monitor.Reference.StdDev
		if refStd > 0 {
			zScore := math.Abs(curMean-refMean) / refStd
			if zScore > 3.0 { // 3 sigma rule
				monitor.DriftScore = zScore
				monitor.DriftType = DriftMean
				d.maybeAlert(monitor, DriftMean, zScore, 3.0)
				return
			}
		}
	}

	monitor.DriftType = DriftNone
	monitor.DriftScore = ksScore
}

// checkCategoricalDrift performs drift detection for categorical features.
func (d *Detector) checkCategoricalDrift(monitor *FeatureMonitor) {
	if monitor.Current.Total < 50 || monitor.Reference.Total < 50 {
		return
	}

	// PSI test
	psi := d.psiTest(monitor.Reference.Categories, monitor.Current.Categories,
		monitor.Reference.Total, monitor.Current.Total)

	if psi > d.config.PSIThreshold {
		monitor.DriftScore = psi
		monitor.DriftType = DriftPSI
		d.maybeAlert(monitor, DriftPSI, psi, d.config.PSIThreshold)
		return
	}

	monitor.DriftType = DriftNone
	monitor.DriftScore = psi
}

// ksTest performs the two-sample Kolmogorov-Smirnov test.
func (d *Detector) ksTest(ref, cur []float64) float64 {
	if len(ref) == 0 || len(cur) == 0 {
		return 0
	}

	// Sort both samples
	sortedRef := make([]float64, len(ref))
	sortedCur := make([]float64, len(cur))
	copy(sortedRef, ref)
	copy(sortedCur, cur)
	sort.Float64s(sortedRef)
	sort.Float64s(sortedCur)

	// Combine all unique values and compute CDFs
	n1, n2 := float64(len(sortedRef)), float64(len(sortedCur))
	var maxDiff float64
	var i, j int

	for i < len(sortedRef) || j < len(sortedCur) {
		var val float64
		if i >= len(sortedRef) {
			val = sortedCur[j]
		} else if j >= len(sortedCur) {
			val = sortedRef[i]
		} else if sortedRef[i] < sortedCur[j] {
			val = sortedRef[i]
		} else {
			val = sortedCur[j]
		}

		// Advance all indices that match this value
		for i < len(sortedRef) && sortedRef[i] <= val {
			i++
		}
		for j < len(sortedCur) && sortedCur[j] <= val {
			j++
		}

		// Compute CDF values after processing this value
		cdf1 := float64(i) / n1
		cdf2 := float64(j) / n2

		diff := math.Abs(cdf1 - cdf2)
		if diff > maxDiff {
			maxDiff = diff
		}
	}

	return maxDiff
}

// psiTest computes the Population Stability Index.
func (d *Detector) psiTest(refCats, curCats map[string]int, refTotal, curTotal int) float64 {
	if refTotal == 0 || curTotal == 0 {
		return 0
	}

	// Collect all categories
	allCats := make(map[string]bool)
	for c := range refCats {
		allCats[c] = true
	}
	for c := range curCats {
		allCats[c] = true
	}

	var psi float64
	epsilon := 0.0001 // Small value to avoid log(0)

	for cat := range allCats {
		refPct := (float64(refCats[cat]) + epsilon) / float64(refTotal)
		curPct := (float64(curCats[cat]) + epsilon) / float64(curTotal)

		psi += (curPct - refPct) * math.Log(curPct/refPct)
	}

	return psi
}

// maybeAlert creates an alert if cooldown has passed.
func (d *Detector) maybeAlert(monitor *FeatureMonitor, driftType DriftType, score, threshold float64) {
	now := time.Now()
	if now.Sub(monitor.LastAlert) < d.config.AlertCooldown {
		return
	}

	monitor.LastAlert = now

	alert := Alert{
		Feature:   monitor.Name,
		Type:      driftType,
		Score:     score,
		Threshold: threshold,
		Timestamp: now,
		Message:   formatAlertMessage(monitor.Name, driftType, score, threshold),
	}

	d.alerts = append(d.alerts, alert)

	// Keep alerts bounded
	if len(d.alerts) > d.alertLimit {
		d.alerts = d.alerts[len(d.alerts)-d.alertLimit:]
	}
}

// GetAlerts returns recent alerts.
func (d *Detector) GetAlerts(since time.Time) []Alert {
	d.mu.RLock()
	defer d.mu.RUnlock()

	result := make([]Alert, 0, len(d.alerts))
	for _, a := range d.alerts {
		if a.Timestamp.After(since) {
			result = append(result, a)
		}
	}
	return result
}

// GetMonitorStatus returns the current status of all monitors.
func (d *Detector) GetMonitorStatus() []MonitorStatus {
	d.mu.RLock()
	defer d.mu.RUnlock()

	result := make([]MonitorStatus, 0, len(d.monitors))
	for _, m := range d.monitors {
		status := MonitorStatus{
			Feature:     m.Name,
			Type:        m.Type,
			SampleCount: m.SampleCount,
			DriftType:   m.DriftType,
			DriftScore:  m.DriftScore,
			LastAlert:   m.LastAlert,
		}

		if m.Type == TypeNumeric && len(m.Current.Values) > 0 {
			status.CurrentMean = m.Current.Mean
			status.CurrentStdDev = m.Current.StdDev
			status.ReferenceMean = m.Reference.Mean
			status.ReferenceStdDev = m.Reference.StdDev
		}

		result = append(result, status)
	}

	return result
}

// MonitorStatus represents the current state of a feature monitor.
type MonitorStatus struct {
	Feature         string
	Type            FeatureType
	SampleCount     int64
	DriftType       DriftType
	DriftScore      float64
	LastAlert       time.Time
	CurrentMean     float64
	CurrentStdDev   float64
	ReferenceMean   float64
	ReferenceStdDev float64
}

// ResetReference resets the reference distribution for a feature.
func (d *Detector) ResetReference(name string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	monitor, exists := d.monitors[name]
	if !exists {
		return ErrFeatureNotFound
	}

	// Copy current to reference
	if monitor.Type == TypeNumeric {
		monitor.Reference.Values = make([]float64, len(monitor.Current.Values))
		copy(monitor.Reference.Values, monitor.Current.Values)
		monitor.Reference.updateStats()
	} else {
		monitor.Reference.Categories = make(map[string]int)
		for k, v := range monitor.Current.Categories {
			monitor.Reference.Categories[k] = v
		}
		monitor.Reference.Total = monitor.Current.Total
	}

	monitor.DriftType = DriftNone
	monitor.DriftScore = 0

	return nil
}

// updateStats computes statistics for a distribution.
func (dist *Distribution) updateStats() {
	if len(dist.Values) == 0 {
		return
	}

	// Mean
	var sum float64
	for _, v := range dist.Values {
		sum += v
	}
	dist.Mean = sum / float64(len(dist.Values))

	// Variance and StdDev
	var variance float64
	for _, v := range dist.Values {
		diff := v - dist.Mean
		variance += diff * diff
	}
	variance /= float64(len(dist.Values))
	dist.StdDev = math.Sqrt(variance)

	// Min/Max
	dist.Min = dist.Values[0]
	dist.Max = dist.Values[0]
	for _, v := range dist.Values {
		if v < dist.Min {
			dist.Min = v
		}
		if v > dist.Max {
			dist.Max = v
		}
	}
}

func formatAlertMessage(feature string, driftType DriftType, score, threshold float64) string {
	switch driftType {
	case DriftKS:
		return "Kolmogorov-Smirnov test indicates distribution shift"
	case DriftPSI:
		return "Population Stability Index indicates category distribution change"
	case DriftMean:
		return "Feature mean has shifted significantly"
	case DriftVariance:
		return "Feature variance has changed significantly"
	default:
		return "Unknown drift detected"
	}
}
