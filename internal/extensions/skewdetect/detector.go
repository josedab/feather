package skewdetect

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"sync"
	"time"
)

// SkewType indicates the category of skew detected.
type SkewType string

const (
	SkewDistribution SkewType = "distribution"
	SkewSchema       SkewType = "schema"
	SkewMissing      SkewType = "missing"
	SkewRange        SkewType = "range"
	SkewLatency      SkewType = "latency"
)

// Severity indicates the severity of a skew alert.
type Severity string

const (
	SeverityLow      Severity = "low"
	SeverityMedium   Severity = "medium"
	SeverityHigh     Severity = "high"
	SeverityCritical Severity = "critical"
)

var (
	// ErrFeatureNotFound is returned when a feature is not registered.
	ErrFeatureNotFound = errors.New("feature not found")

	// ErrFeatureExists is returned when registering a duplicate feature.
	ErrFeatureExists = errors.New("feature already exists")
)

// DetectorConfig configures the skew detector.
type DetectorConfig struct {
	KSThreshold   float64       // Kolmogorov-Smirnov threshold
	PSIThreshold  float64       // Population Stability Index threshold
	JSThreshold   float64       // Jensen-Shannon divergence threshold
	CheckInterval time.Duration // Interval between automatic checks
	MaxSamples    int           // Maximum samples to retain per side
	AlertCooldown time.Duration // Minimum time between alerts per feature
}

// DefaultDetectorConfig returns sensible defaults.
func DefaultDetectorConfig() DetectorConfig {
	return DetectorConfig{
		KSThreshold:   0.1,
		PSIThreshold:  0.25,
		JSThreshold:   0.1,
		CheckInterval: 5 * time.Minute,
		MaxSamples:    10000,
		AlertCooldown: 15 * time.Minute,
	}
}

// Detector monitors online/offline feature distributions for skew.
type Detector struct {
	config    DetectorConfig
	mu        sync.RWMutex
	features  map[string]*FeatureProfile
	alerts    []SkewAlert
	contracts map[string]*DataContract
	alertSeq  int
}

// FeatureProfile holds online and offline sample data for a feature.
type FeatureProfile struct {
	Name           string    `json:"name"`
	OnlineSamples  []float64 `json:"-"`
	OfflineSamples []float64 `json:"-"`
	OnlineStats    DistStats `json:"online_stats"`
	OfflineStats   DistStats `json:"offline_stats"`
	LastCheck      time.Time `json:"last_check"`
	SkewScore      float64   `json:"skew_score"`
	KSStatistic    float64   `json:"ks_statistic"`
	PSIScore       float64   `json:"psi_score"`
	JSScore        float64   `json:"js_divergence"`
	Status         string    `json:"status"`
}

// DistStats holds summary statistics for a distribution.
type DistStats struct {
	Count    int     `json:"count"`
	Mean     float64 `json:"mean"`
	StdDev   float64 `json:"std_dev"`
	Min      float64 `json:"min"`
	Max      float64 `json:"max"`
	P50      float64 `json:"p50"`
	P95      float64 `json:"p95"`
	P99      float64 `json:"p99"`
	NullRate float64 `json:"null_rate"`
}

// SkewAlert represents a detected skew event.
type SkewAlert struct {
	ID         string    `json:"id"`
	Feature    string    `json:"feature"`
	Type       SkewType  `json:"type"`
	Severity   Severity  `json:"severity"`
	Score      float64   `json:"score"`
	Threshold  float64   `json:"threshold"`
	Message    string    `json:"message"`
	DetectedAt time.Time `json:"detected_at"`
	Resolved   bool      `json:"resolved"`
}

// DataContract defines constraints that a feature must satisfy.
type DataContract struct {
	Feature      string    `json:"feature"`
	MinValue     *float64  `json:"min_value,omitempty"`
	MaxValue     *float64  `json:"max_value,omitempty"`
	NotNull      bool      `json:"not_null"`
	MaxNullRate  float64   `json:"max_null_rate"`
	MaxSkew      float64   `json:"max_skew"`
	AllowedTypes []string  `json:"allowed_types,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

// ContractViolation describes a single contract rule failure.
type ContractViolation struct {
	Feature  string   `json:"feature"`
	Rule     string   `json:"rule"`
	Expected string   `json:"expected"`
	Actual   string   `json:"actual"`
	Severity Severity `json:"severity"`
}

// DetectorStats summarises the detector state.
type DetectorStats struct {
	TrackedFeatures int     `json:"tracked_features"`
	HealthyCount    int     `json:"healthy_count"`
	WarningCount    int     `json:"warning_count"`
	SkewedCount     int     `json:"skewed_count"`
	TotalAlerts     int     `json:"total_alerts"`
	ActiveAlerts    int     `json:"active_alerts"`
	ContractsCount  int     `json:"contracts_count"`
	AvgSkewScore    float64 `json:"avg_skew_score"`
}

// NewDetector creates a new skew detector.
func NewDetector(cfg DetectorConfig) *Detector {
	if cfg.MaxSamples == 0 {
		cfg = DefaultDetectorConfig()
	}
	return &Detector{
		config:    cfg,
		features:  make(map[string]*FeatureProfile),
		alerts:    make([]SkewAlert, 0),
		contracts: make(map[string]*DataContract),
	}
}

// RegisterFeature registers a new feature for skew monitoring.
func (d *Detector) RegisterFeature(name string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if _, exists := d.features[name]; exists {
		return fmt.Errorf("%w: %s", ErrFeatureExists, name)
	}

	d.features[name] = &FeatureProfile{
		Name:   name,
		Status: "healthy",
	}
	return nil
}

// RecordOnline records online (serving) sample values for a feature.
func (d *Detector) RecordOnline(feature string, values []float64) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	fp, exists := d.features[feature]
	if !exists {
		return fmt.Errorf("%w: %s", ErrFeatureNotFound, feature)
	}

	fp.OnlineSamples = append(fp.OnlineSamples, values...)
	if len(fp.OnlineSamples) > d.config.MaxSamples {
		fp.OnlineSamples = fp.OnlineSamples[len(fp.OnlineSamples)-d.config.MaxSamples:]
	}
	fp.OnlineStats = computeStats(fp.OnlineSamples)
	return nil
}

// RecordOffline records offline (training) sample values for a feature.
func (d *Detector) RecordOffline(feature string, values []float64) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	fp, exists := d.features[feature]
	if !exists {
		return fmt.Errorf("%w: %s", ErrFeatureNotFound, feature)
	}

	fp.OfflineSamples = append(fp.OfflineSamples, values...)
	if len(fp.OfflineSamples) > d.config.MaxSamples {
		fp.OfflineSamples = fp.OfflineSamples[len(fp.OfflineSamples)-d.config.MaxSamples:]
	}
	fp.OfflineStats = computeStats(fp.OfflineSamples)
	return nil
}

// Check runs skew detection for a single feature and returns its profile.
func (d *Detector) Check(feature string) (*FeatureProfile, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	fp, exists := d.features[feature]
	if !exists {
		return nil, fmt.Errorf("%w: %s", ErrFeatureNotFound, feature)
	}

	d.checkFeature(fp)

	out := *fp
	return &out, nil
}

// CheckAll runs skew detection for every registered feature.
func (d *Detector) CheckAll() map[string]*FeatureProfile {
	d.mu.Lock()
	defer d.mu.Unlock()

	result := make(map[string]*FeatureProfile, len(d.features))
	for name, fp := range d.features {
		d.checkFeature(fp)
		out := *fp
		result[name] = &out
	}
	return result
}

// GetAlerts returns alerts detected after the given time.
func (d *Detector) GetAlerts(since time.Time) []SkewAlert {
	d.mu.RLock()
	defer d.mu.RUnlock()

	result := make([]SkewAlert, 0, len(d.alerts))
	for _, a := range d.alerts {
		if a.DetectedAt.After(since) {
			result = append(result, a)
		}
	}
	return result
}

// SetContract registers or updates a data contract for a feature.
func (d *Detector) SetContract(contract DataContract) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if _, exists := d.features[contract.Feature]; !exists {
		return fmt.Errorf("%w: %s", ErrFeatureNotFound, contract.Feature)
	}

	if contract.CreatedAt.IsZero() {
		contract.CreatedAt = time.Now()
	}
	d.contracts[contract.Feature] = &contract
	return nil
}

// ValidateContract checks the current profile against its data contract.
func (d *Detector) ValidateContract(feature string) ([]ContractViolation, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	fp, exists := d.features[feature]
	if !exists {
		return nil, fmt.Errorf("%w: %s", ErrFeatureNotFound, feature)
	}

	contract, hasContract := d.contracts[feature]
	if !hasContract {
		return nil, nil
	}

	var violations []ContractViolation

	// Min value check (against online stats).
	if contract.MinValue != nil && fp.OnlineStats.Count > 0 {
		if fp.OnlineStats.Min < *contract.MinValue {
			violations = append(violations, ContractViolation{
				Feature:  feature,
				Rule:     "min_value",
				Expected: fmt.Sprintf(">= %.4f", *contract.MinValue),
				Actual:   fmt.Sprintf("%.4f", fp.OnlineStats.Min),
				Severity: SeverityHigh,
			})
		}
	}

	// Max value check.
	if contract.MaxValue != nil && fp.OnlineStats.Count > 0 {
		if fp.OnlineStats.Max > *contract.MaxValue {
			violations = append(violations, ContractViolation{
				Feature:  feature,
				Rule:     "max_value",
				Expected: fmt.Sprintf("<= %.4f", *contract.MaxValue),
				Actual:   fmt.Sprintf("%.4f", fp.OnlineStats.Max),
				Severity: SeverityHigh,
			})
		}
	}

	// Null rate check.
	if contract.NotNull && fp.OnlineStats.NullRate > 0 {
		violations = append(violations, ContractViolation{
			Feature:  feature,
			Rule:     "not_null",
			Expected: "null_rate = 0",
			Actual:   fmt.Sprintf("null_rate = %.4f", fp.OnlineStats.NullRate),
			Severity: SeverityCritical,
		})
	}

	if contract.MaxNullRate > 0 && fp.OnlineStats.NullRate > contract.MaxNullRate {
		violations = append(violations, ContractViolation{
			Feature:  feature,
			Rule:     "max_null_rate",
			Expected: fmt.Sprintf("<= %.4f", contract.MaxNullRate),
			Actual:   fmt.Sprintf("%.4f", fp.OnlineStats.NullRate),
			Severity: SeverityMedium,
		})
	}

	// Skew score check.
	if contract.MaxSkew > 0 && fp.SkewScore > contract.MaxSkew {
		violations = append(violations, ContractViolation{
			Feature:  feature,
			Rule:     "max_skew",
			Expected: fmt.Sprintf("<= %.4f", contract.MaxSkew),
			Actual:   fmt.Sprintf("%.4f", fp.SkewScore),
			Severity: SeverityHigh,
		})
	}

	return violations, nil
}

// GetProfile returns the current profile for a feature.
func (d *Detector) GetProfile(feature string) (*FeatureProfile, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	fp, exists := d.features[feature]
	if !exists {
		return nil, fmt.Errorf("%w: %s", ErrFeatureNotFound, feature)
	}
	out := *fp
	return &out, nil
}

// ListProfiles returns all registered feature profiles.
func (d *Detector) ListProfiles() []*FeatureProfile {
	d.mu.RLock()
	defer d.mu.RUnlock()

	result := make([]*FeatureProfile, 0, len(d.features))
	for _, fp := range d.features {
		out := *fp
		result = append(result, &out)
	}
	return result
}

// Stats returns aggregate statistics about the detector.
func (d *Detector) Stats() DetectorStats {
	d.mu.RLock()
	defer d.mu.RUnlock()

	stats := DetectorStats{
		TrackedFeatures: len(d.features),
		TotalAlerts:     len(d.alerts),
		ContractsCount:  len(d.contracts),
	}

	var totalSkew float64
	for _, fp := range d.features {
		totalSkew += fp.SkewScore
		switch fp.Status {
		case "healthy":
			stats.HealthyCount++
		case "warning":
			stats.WarningCount++
		case "skewed":
			stats.SkewedCount++
		}
	}
	if stats.TrackedFeatures > 0 {
		stats.AvgSkewScore = totalSkew / float64(stats.TrackedFeatures)
	}

	for _, a := range d.alerts {
		if !a.Resolved {
			stats.ActiveAlerts++
		}
	}

	return stats
}

// ---------- internal helpers ----------

// checkFeature computes all skew metrics and updates profile status.
func (d *Detector) checkFeature(fp *FeatureProfile) {
	if len(fp.OnlineSamples) == 0 || len(fp.OfflineSamples) == 0 {
		return
	}

	fp.KSStatistic = ksStatistic(fp.OnlineSamples, fp.OfflineSamples)
	fp.PSIScore = psiScore(fp.OfflineSamples, fp.OnlineSamples, 10)
	fp.JSScore = jsDivergence(fp.OfflineSamples, fp.OnlineSamples, 10)
	fp.SkewScore = math.Max(fp.KSStatistic, math.Max(fp.PSIScore, fp.JSScore))
	fp.LastCheck = time.Now()

	// Determine status.
	fp.Status = "healthy"
	if fp.KSStatistic > d.config.KSThreshold*0.7 ||
		fp.PSIScore > d.config.PSIThreshold*0.7 ||
		fp.JSScore > d.config.JSThreshold*0.7 {
		fp.Status = "warning"
	}
	if fp.KSStatistic > d.config.KSThreshold ||
		fp.PSIScore > d.config.PSIThreshold ||
		fp.JSScore > d.config.JSThreshold {
		fp.Status = "skewed"
	}

	// Emit alerts when thresholds are exceeded.
	if fp.KSStatistic > d.config.KSThreshold {
		d.emitAlert(fp, SkewDistribution, fp.KSStatistic, d.config.KSThreshold,
			"KS statistic exceeds threshold: distribution skew detected")
	}
	if fp.PSIScore > d.config.PSIThreshold {
		d.emitAlert(fp, SkewDistribution, fp.PSIScore, d.config.PSIThreshold,
			"PSI score exceeds threshold: population shift detected")
	}
	if fp.JSScore > d.config.JSThreshold {
		d.emitAlert(fp, SkewDistribution, fp.JSScore, d.config.JSThreshold,
			"JS divergence exceeds threshold: distribution skew detected")
	}
}

func (d *Detector) emitAlert(fp *FeatureProfile, skewType SkewType, score, threshold float64, msg string) {
	d.alertSeq++
	d.alerts = append(d.alerts, SkewAlert{
		ID:         fmt.Sprintf("skew-%d", d.alertSeq),
		Feature:    fp.Name,
		Type:       skewType,
		Severity:   scoreSeverity(score, threshold),
		Score:      score,
		Threshold:  threshold,
		Message:    msg,
		DetectedAt: time.Now(),
	})
}

func scoreSeverity(score, threshold float64) Severity {
	ratio := score / threshold
	switch {
	case ratio > 2.0:
		return SeverityCritical
	case ratio > 1.5:
		return SeverityHigh
	case ratio > 1.0:
		return SeverityMedium
	default:
		return SeverityLow
	}
}

// ---------- statistical functions ----------

// ksStatistic computes the two-sample Kolmogorov-Smirnov statistic.
func ksStatistic(a, b []float64) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}

	sa := make([]float64, len(a))
	sb := make([]float64, len(b))
	copy(sa, a)
	copy(sb, b)
	sort.Float64s(sa)
	sort.Float64s(sb)

	n1, n2 := float64(len(sa)), float64(len(sb))
	var maxDiff float64
	var i, j int

	for i < len(sa) || j < len(sb) {
		var val float64
		switch {
		case i >= len(sa):
			val = sb[j]
		case j >= len(sb):
			val = sa[i]
		case sa[i] < sb[j]:
			val = sa[i]
		default:
			val = sb[j]
		}

		for i < len(sa) && sa[i] <= val {
			i++
		}
		for j < len(sb) && sb[j] <= val {
			j++
		}

		diff := math.Abs(float64(i)/n1 - float64(j)/n2)
		if diff > maxDiff {
			maxDiff = diff
		}
	}

	return maxDiff
}

// psiScore computes the Population Stability Index using equal-width bins.
func psiScore(expected, actual []float64, numBins int) float64 {
	if len(expected) == 0 || len(actual) == 0 || numBins <= 0 {
		return 0
	}

	// Determine global min/max.
	allMin, allMax := expected[0], expected[0]
	for _, v := range expected {
		if v < allMin {
			allMin = v
		}
		if v > allMax {
			allMax = v
		}
	}
	for _, v := range actual {
		if v < allMin {
			allMin = v
		}
		if v > allMax {
			allMax = v
		}
	}

	if allMin == allMax {
		return 0
	}

	binWidth := (allMax - allMin) / float64(numBins)
	epsilon := 1e-4

	expBins := make([]float64, numBins)
	actBins := make([]float64, numBins)

	binIndex := func(v float64) int {
		idx := int((v - allMin) / binWidth)
		if idx >= numBins {
			idx = numBins - 1
		}
		return idx
	}

	for _, v := range expected {
		expBins[binIndex(v)]++
	}
	for _, v := range actual {
		actBins[binIndex(v)]++
	}

	nExp := float64(len(expected))
	nAct := float64(len(actual))
	var psi float64
	for i := 0; i < numBins; i++ {
		ePct := expBins[i]/nExp + epsilon
		aPct := actBins[i]/nAct + epsilon
		psi += (aPct - ePct) * math.Log(aPct/ePct)
	}

	return psi
}

// jsDivergence computes the Jensen-Shannon divergence using equal-width bins.
func jsDivergence(p, q []float64, numBins int) float64 {
	if len(p) == 0 || len(q) == 0 || numBins <= 0 {
		return 0
	}

	allMin, allMax := p[0], p[0]
	for _, v := range p {
		if v < allMin {
			allMin = v
		}
		if v > allMax {
			allMax = v
		}
	}
	for _, v := range q {
		if v < allMin {
			allMin = v
		}
		if v > allMax {
			allMax = v
		}
	}

	if allMin == allMax {
		return 0
	}

	binWidth := (allMax - allMin) / float64(numBins)
	epsilon := 1e-10

	pBins := make([]float64, numBins)
	qBins := make([]float64, numBins)

	binIndex := func(v float64) int {
		idx := int((v - allMin) / binWidth)
		if idx >= numBins {
			idx = numBins - 1
		}
		return idx
	}

	for _, v := range p {
		pBins[binIndex(v)]++
	}
	for _, v := range q {
		qBins[binIndex(v)]++
	}

	nP := float64(len(p))
	nQ := float64(len(q))

	// Normalise into probability distributions.
	for i := 0; i < numBins; i++ {
		pBins[i] = pBins[i]/nP + epsilon
		qBins[i] = qBins[i]/nQ + epsilon
	}

	// M = (P+Q)/2, JS = 0.5 * KL(P||M) + 0.5 * KL(Q||M)
	var js float64
	for i := 0; i < numBins; i++ {
		m := (pBins[i] + qBins[i]) / 2
		js += 0.5*pBins[i]*math.Log(pBins[i]/m) + 0.5*qBins[i]*math.Log(qBins[i]/m)
	}

	return js
}

// computeStats derives DistStats from a slice of float64 values.
func computeStats(values []float64) DistStats {
	n := len(values)
	if n == 0 {
		return DistStats{}
	}

	sorted := make([]float64, n)
	copy(sorted, values)
	sort.Float64s(sorted)

	var sum float64
	for _, v := range sorted {
		sum += v
	}
	mean := sum / float64(n)

	var variance float64
	for _, v := range sorted {
		d := v - mean
		variance += d * d
	}
	variance /= float64(n)

	return DistStats{
		Count:  n,
		Mean:   mean,
		StdDev: math.Sqrt(variance),
		Min:    sorted[0],
		Max:    sorted[n-1],
		P50:    percentile(sorted, 0.50),
		P95:    percentile(sorted, 0.95),
		P99:    percentile(sorted, 0.99),
	}
}

// percentile returns an interpolated percentile from sorted data.
func percentile(sorted []float64, p float64) float64 {
	n := len(sorted)
	if n == 0 {
		return 0
	}
	if n == 1 {
		return sorted[0]
	}

	rank := p * float64(n-1)
	lower := int(math.Floor(rank))
	upper := lower + 1
	if upper >= n {
		return sorted[n-1]
	}

	frac := rank - float64(lower)
	return sorted[lower]*(1-frac) + sorted[upper]*frac
}
