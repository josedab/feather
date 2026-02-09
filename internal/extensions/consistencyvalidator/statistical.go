package consistencyvalidator

import (
	"math"
	"sort"
)

// StatisticalTest identifies a specific divergence test.
type StatisticalTest string

const (
	TestKS             StatisticalTest = "ks"
	TestPSI            StatisticalTest = "psi"
	TestChiSquared     StatisticalTest = "chi_squared"
	TestJensenShannon  StatisticalTest = "jensen_shannon"
)

// StatResult holds the output of a statistical test.
type StatResult struct {
	Test       StatisticalTest `json:"test"`
	Statistic  float64         `json:"statistic"`
	Threshold  float64         `json:"threshold"`
	Passed     bool            `json:"passed"`
}

// PerFeatureConfig allows configurable thresholds per feature.
type PerFeatureConfig struct {
	KSThreshold float64         `json:"ks_threshold"`
	PSIThreshold float64        `json:"psi_threshold"`
	ChiSquaredThreshold float64 `json:"chi_squared_threshold"`
	JSThreshold float64         `json:"js_threshold"`
	EnabledTests []StatisticalTest `json:"enabled_tests"`
}

// DefaultPerFeatureConfig returns default thresholds for all tests.
func DefaultPerFeatureConfig() PerFeatureConfig {
	return PerFeatureConfig{
		KSThreshold:         0.05,
		PSIThreshold:        0.2,
		ChiSquaredThreshold: 0.05,
		JSThreshold:         0.1,
		EnabledTests:        []StatisticalTest{TestKS, TestPSI, TestJensenShannon},
	}
}

// RunAllTests runs all enabled statistical tests on two sample distributions.
func RunAllTests(online, offline []float64, cfg PerFeatureConfig) []StatResult {
	if len(cfg.EnabledTests) == 0 {
		cfg.EnabledTests = []StatisticalTest{TestKS, TestPSI, TestJensenShannon}
	}

	var results []StatResult
	for _, test := range cfg.EnabledTests {
		var result StatResult
		result.Test = test
		switch test {
		case TestKS:
			result.Statistic = ksTest(online, offline)
			result.Threshold = cfg.KSThreshold
			result.Passed = result.Statistic <= cfg.KSThreshold
		case TestPSI:
			result.Statistic = computePSI(online, offline, 10)
			result.Threshold = cfg.PSIThreshold
			result.Passed = result.Statistic <= cfg.PSIThreshold
		case TestChiSquared:
			result.Statistic = computeChiSquared(online, offline, 10)
			result.Threshold = cfg.ChiSquaredThreshold
			result.Passed = result.Statistic <= cfg.ChiSquaredThreshold
		case TestJensenShannon:
			result.Statistic = computeJensenShannon(online, offline, 10)
			result.Threshold = cfg.JSThreshold
			result.Passed = result.Statistic <= cfg.JSThreshold
		}
		results = append(results, result)
	}
	return results
}

// computePSI calculates Population Stability Index between two distributions.
// PSI = Σ (actual% - expected%) × ln(actual% / expected%)
// Uses a shared bin range across both distributions for valid comparison.
func computePSI(actual, expected []float64, numBins int) float64 {
	if len(actual) == 0 || len(expected) == 0 {
		return 0
	}

	actualHist, expectedHist := sharedHistogram(actual, expected, numBins)

	psi := 0.0
	for i := 0; i < numBins; i++ {
		a := math.Max(actualHist[i], 0.0001)
		e := math.Max(expectedHist[i], 0.0001)
		psi += (a - e) * math.Log(a/e)
	}
	return psi
}

// computeChiSquared calculates the chi-squared statistic between two distributions.
// χ² = Σ (O - E)² / E
// Uses a shared bin range across both distributions for valid comparison.
func computeChiSquared(observed, expected []float64, numBins int) float64 {
	if len(observed) == 0 || len(expected) == 0 {
		return 0
	}

	obsHist, expHist := sharedHistogram(observed, expected, numBins)

	chiSq := 0.0
	for i := 0; i < numBins; i++ {
		e := math.Max(expHist[i], 0.0001)
		diff := obsHist[i] - e
		chiSq += (diff * diff) / e
	}
	return chiSq
}

// computeJensenShannon calculates Jensen-Shannon divergence.
// JSD(P||Q) = ½ KL(P||M) + ½ KL(Q||M), where M = ½(P+Q)
// Uses a shared bin range across both distributions for valid comparison.
func computeJensenShannon(p, q []float64, numBins int) float64 {
	if len(p) == 0 || len(q) == 0 {
		return 0
	}

	pHist, qHist := sharedHistogram(p, q, numBins)

	// Compute M = ½(P + Q)
	m := make([]float64, numBins)
	for i := 0; i < numBins; i++ {
		m[i] = 0.5 * (pHist[i] + qHist[i])
	}

	jsd := 0.5*klDivergence(pHist, m) + 0.5*klDivergence(qHist, m)
	return math.Sqrt(math.Max(jsd, 0)) // sqrt for the metric form
}

// klDivergence computes KL(P||Q) = Σ P(i) × log(P(i)/Q(i))
func klDivergence(p, q []float64) float64 {
	kl := 0.0
	for i := range p {
		pi := math.Max(p[i], 1e-10)
		qi := math.Max(q[i], 1e-10)
		kl += pi * math.Log(pi/qi)
	}
	return kl
}

// histogram creates a normalized histogram from values with the given number of bins.
func histogram(values []float64, numBins int) []float64 {
	if len(values) == 0 || numBins <= 0 {
		return make([]float64, numBins)
	}

	sorted := make([]float64, len(values))
	copy(sorted, values)
	sort.Float64s(sorted)

	minVal := sorted[0]
	maxVal := sorted[len(sorted)-1]

	// Handle constant values
	if maxVal == minVal {
		hist := make([]float64, numBins)
		hist[0] = 1.0
		return hist
	}

	binWidth := (maxVal - minVal) / float64(numBins)
	counts := make([]float64, numBins)

	for _, v := range values {
		bin := int((v - minVal) / binWidth)
		if bin >= numBins {
			bin = numBins - 1
		}
		counts[bin]++
	}

	// Normalize
	total := float64(len(values))
	hist := make([]float64, numBins)
	for i, c := range counts {
		hist[i] = c / total
	}
	return hist
}

// sharedHistogram creates normalized histograms for two distributions using a shared
// min/max range, ensuring the bins are comparable across distributions.
func sharedHistogram(a, b []float64, numBins int) ([]float64, []float64) {
	if numBins <= 0 {
		return make([]float64, 0), make([]float64, 0)
	}

	// Find global min/max
	globalMin := math.Inf(1)
	globalMax := math.Inf(-1)
	for _, v := range a {
		if v < globalMin {
			globalMin = v
		}
		if v > globalMax {
			globalMax = v
		}
	}
	for _, v := range b {
		if v < globalMin {
			globalMin = v
		}
		if v > globalMax {
			globalMax = v
		}
	}

	if globalMax == globalMin {
		aHist := make([]float64, numBins)
		bHist := make([]float64, numBins)
		aHist[0] = 1.0
		bHist[0] = 1.0
		return aHist, bHist
	}

	binWidth := (globalMax - globalMin) / float64(numBins)
	binFunc := func(values []float64) []float64 {
		counts := make([]float64, numBins)
		for _, v := range values {
			bin := int((v - globalMin) / binWidth)
			if bin >= numBins {
				bin = numBins - 1
			}
			counts[bin]++
		}
		total := float64(len(values))
		hist := make([]float64, numBins)
		for i, c := range counts {
			hist[i] = c / total
		}
		return hist
	}

	return binFunc(a), binFunc(b)
}

// DistributionSnapshot captures the distribution state at a point in time.
type DistributionSnapshot struct {
	Feature   string    `json:"feature"`
	Source    string    `json:"source"`
	Count    int       `json:"count"`
	Mean     float64   `json:"mean"`
	StdDev   float64   `json:"std_dev"`
	Min      float64   `json:"min"`
	Max      float64   `json:"max"`
	P50      float64   `json:"p50"`
	P95      float64   `json:"p95"`
	P99      float64   `json:"p99"`
	Histogram []float64 `json:"histogram"`
}

// TakeSnapshot creates a distribution snapshot from a set of values.
func TakeSnapshot(feature, source string, values []float64) DistributionSnapshot {
	snap := DistributionSnapshot{
		Feature: feature,
		Source:  source,
		Count:  len(values),
	}

	if len(values) == 0 {
		return snap
	}

	sorted := make([]float64, len(values))
	copy(sorted, values)
	sort.Float64s(sorted)

	snap.Min = sorted[0]
	snap.Max = sorted[len(sorted)-1]
	snap.Mean, snap.StdDev = meanStdDev(values)
	snap.P50 = percentile(sorted, 0.50)
	snap.P95 = percentile(sorted, 0.95)
	snap.P99 = percentile(sorted, 0.99)
	snap.Histogram = histogram(values, 10)

	return snap
}

func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	idx := p * float64(len(sorted)-1)
	lower := int(math.Floor(idx))
	upper := int(math.Ceil(idx))
	if lower == upper || upper >= len(sorted) {
		return sorted[lower]
	}
	frac := idx - float64(lower)
	return sorted[lower]*(1-frac) + sorted[upper]*frac
}
