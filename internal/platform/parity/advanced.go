package parity

import (
	"fmt"
	"math"
	"sort"
	"sync"
	"time"
)

// StatisticalTest identifies the type of statistical comparison.
type StatisticalTest string

const (
	TestKS       StatisticalTest = "kolmogorov_smirnov"
	TestChiSq    StatisticalTest = "chi_squared"
	TestPSI      StatisticalTest = "psi"
	TestAbsDiff  StatisticalTest = "absolute_diff"
)

// StatisticalTestResult captures the outcome of a statistical test.
type StatisticalTestResult struct {
	TestName    StatisticalTest `json:"test_name"`
	Statistic   float64         `json:"statistic"`
	PValue      float64         `json:"p_value,omitempty"`
	Significant bool            `json:"significant"`
	Threshold   float64         `json:"threshold"`
	Message     string          `json:"message"`
}

// ParityReport provides a comprehensive parity assessment for a feature.
type ParityReport struct {
	FeatureName    string                  `json:"feature_name"`
	SampleSize     int                     `json:"sample_size"`
	MismatchRate   float64                 `json:"mismatch_rate"`
	MeanDifference float64                 `json:"mean_difference"`
	MaxDifference  float64                 `json:"max_difference"`
	Tests          []StatisticalTestResult `json:"tests"`
	InParity       bool                    `json:"in_parity"`
	GeneratedAt    time.Time               `json:"generated_at"`
}

// AdvancedConfig extends the base Config with statistical test parameters.
type AdvancedConfig struct {
	Config
	KSThreshold    float64 `json:"ks_threshold"`
	PSIThreshold   float64 `json:"psi_threshold"`
	ChiSqThreshold float64 `json:"chi_sq_threshold"`
	NumBins        int     `json:"num_bins"`
}

// DefaultAdvancedConfig returns defaults for advanced parity checking.
func DefaultAdvancedConfig() AdvancedConfig {
	return AdvancedConfig{
		Config:         DefaultConfig(),
		KSThreshold:    0.1,
		PSIThreshold:   0.2,
		ChiSqThreshold: 0.05,
		NumBins:        20,
	}
}

// AdvancedChecker extends the base Checker with statistical tests.
type AdvancedChecker struct {
	mu       sync.RWMutex
	config   AdvancedConfig
	base     *Checker
	samples  map[string]*featureSamples
	reports  map[string]*ParityReport
	webhooks []WebhookConfig
}

type featureSamples struct {
	online  []float64
	offline []float64
}

// WebhookConfig defines an alerting webhook.
type WebhookConfig struct {
	URL         string          `json:"url"`
	MinSeverity string          `json:"min_severity"`
	Events      []string        `json:"events"`
}

// NewAdvancedChecker creates an advanced parity checker.
func NewAdvancedChecker(config AdvancedConfig) *AdvancedChecker {
	return &AdvancedChecker{
		config:  config,
		base:    NewChecker(config.Config),
		samples: make(map[string]*featureSamples),
		reports: make(map[string]*ParityReport),
	}
}

// RecordSample records a pair of online/offline values for statistical testing.
func (c *AdvancedChecker) RecordSample(featureName string, onlineValue, offlineValue float64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	s, ok := c.samples[featureName]
	if !ok {
		s = &featureSamples{}
		c.samples[featureName] = s
	}

	s.online = append(s.online, onlineValue)
	s.offline = append(s.offline, offlineValue)

	if len(s.online) > c.config.MaxSamples {
		s.online = s.online[len(s.online)-c.config.MaxSamples:]
		s.offline = s.offline[len(s.offline)-c.config.MaxSamples:]
	}
}

// RunTests performs all statistical tests on a feature's samples.
func (c *AdvancedChecker) RunTests(featureName string) (*ParityReport, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	s, ok := c.samples[featureName]
	if !ok {
		return nil, fmt.Errorf("no samples for feature %q", featureName)
	}
	if len(s.online) < 10 {
		return nil, fmt.Errorf("insufficient samples for feature %q: need at least 10, have %d", featureName, len(s.online))
	}

	report := &ParityReport{
		FeatureName: featureName,
		SampleSize:  len(s.online),
		InParity:    true,
		GeneratedAt: time.Now(),
	}

	// KS Test
	ksResult := kolmogorovSmirnovTest(s.online, s.offline, c.config.KSThreshold)
	report.Tests = append(report.Tests, ksResult)
	if ksResult.Significant {
		report.InParity = false
	}

	// PSI
	psiResult := populationStabilityIndex(s.online, s.offline, c.config.NumBins, c.config.PSIThreshold)
	report.Tests = append(report.Tests, psiResult)
	if psiResult.Significant {
		report.InParity = false
	}

	// Compute summary statistics.
	var sumDiff, maxDiff float64
	mismatches := 0
	for i := range s.online {
		diff := math.Abs(s.online[i] - s.offline[i])
		sumDiff += diff
		if diff > maxDiff {
			maxDiff = diff
		}
		if diff > c.config.AbsoluteTolerance {
			mismatches++
		}
	}

	report.MeanDifference = sumDiff / float64(len(s.online))
	report.MaxDifference = maxDiff
	report.MismatchRate = float64(mismatches) / float64(len(s.online))

	c.reports[featureName] = report
	return report, nil
}

// GetReport returns the last generated report for a feature.
func (c *AdvancedChecker) GetReport(featureName string) *ParityReport {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.reports[featureName]
}

// GetAllReports returns all generated reports.
func (c *AdvancedChecker) GetAllReports() []*ParityReport {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var result []*ParityReport
	for _, r := range c.reports {
		result = append(result, r)
	}
	return result
}

// RegisterWebhook adds a webhook for parity alerts.
func (c *AdvancedChecker) RegisterWebhook(cfg WebhookConfig) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.webhooks = append(c.webhooks, cfg)
}

// kolmogorovSmirnovTest computes the two-sample KS statistic.
func kolmogorovSmirnovTest(sample1, sample2 []float64, threshold float64) StatisticalTestResult {
	s1 := make([]float64, len(sample1))
	copy(s1, sample1)
	sort.Float64s(s1)

	s2 := make([]float64, len(sample2))
	copy(s2, sample2)
	sort.Float64s(s2)

	n1 := float64(len(s1))
	n2 := float64(len(s2))

	var maxD float64
	i, j := 0, 0
	for i < len(s1) && j < len(s2) {
		d := math.Abs(float64(i+1)/n1 - float64(j+1)/n2)
		if d > maxD {
			maxD = d
		}
		if s1[i] <= s2[j] {
			i++
		} else {
			j++
		}
	}

	significant := maxD > threshold
	msg := "distributions are similar"
	if significant {
		msg = fmt.Sprintf("significant divergence detected (D=%.4f > %.4f)", maxD, threshold)
	}

	return StatisticalTestResult{
		TestName:    TestKS,
		Statistic:   maxD,
		Significant: significant,
		Threshold:   threshold,
		Message:     msg,
	}
}

// populationStabilityIndex computes PSI between two distributions.
func populationStabilityIndex(expected, actual []float64, numBins int, threshold float64) StatisticalTestResult {
	if numBins <= 0 {
		numBins = 20
	}

	all := make([]float64, 0, len(expected)+len(actual))
	all = append(all, expected...)
	all = append(all, actual...)
	sort.Float64s(all)

	if len(all) == 0 {
		return StatisticalTestResult{TestName: TestPSI, Message: "no data"}
	}

	minVal := all[0]
	maxVal := all[len(all)-1]
	if minVal == maxVal {
		return StatisticalTestResult{TestName: TestPSI, Statistic: 0, Message: "constant values"}
	}

	binWidth := (maxVal - minVal) / float64(numBins)
	expBins := make([]float64, numBins)
	actBins := make([]float64, numBins)

	for _, v := range expected {
		idx := int((v - minVal) / binWidth)
		if idx >= numBins {
			idx = numBins - 1
		}
		expBins[idx]++
	}
	for _, v := range actual {
		idx := int((v - minVal) / binWidth)
		if idx >= numBins {
			idx = numBins - 1
		}
		actBins[idx]++
	}

	nExp := float64(len(expected))
	nAct := float64(len(actual))

	var psi float64
	for i := 0; i < numBins; i++ {
		ep := (expBins[i] + 0.5) / (nExp + float64(numBins)*0.5)
		ap := (actBins[i] + 0.5) / (nAct + float64(numBins)*0.5)
		psi += (ap - ep) * math.Log(ap/ep)
	}

	significant := psi > threshold
	msg := "distributions are stable"
	if significant {
		msg = fmt.Sprintf("population shift detected (PSI=%.4f > %.4f)", psi, threshold)
	}

	return StatisticalTestResult{
		TestName:    TestPSI,
		Statistic:   psi,
		Significant: significant,
		Threshold:   threshold,
		Message:     msg,
	}
}
