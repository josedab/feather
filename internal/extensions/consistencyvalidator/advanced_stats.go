package consistencyvalidator

import (
	"math"
	"sort"
)

// StatTestResult holds the output of an advanced statistical test including
// p-value approximation and sample sizes.
type StatTestResult struct {
	TestName   string  `json:"test_name"`
	Statistic  float64 `json:"statistic"`
	PValue     float64 `json:"p_value,omitempty"`
	Threshold  float64 `json:"threshold"`
	Passed     bool    `json:"passed"`
	SampleSize [2]int  `json:"sample_size"`
}

// KolmogorovSmirnov performs a two-sample KS test. It sorts both samples,
// computes empirical CDFs, finds the maximum distance D, and approximates
// the p-value.
func KolmogorovSmirnov(online, offline []float64, threshold float64) StatTestResult {
	result := StatTestResult{
		TestName:   "kolmogorov_smirnov",
		Threshold:  threshold,
		SampleSize: [2]int{len(online), len(offline)},
	}

	n1, n2 := len(online), len(offline)
	if n1 == 0 || n2 == 0 {
		result.Passed = true
		return result
	}

	a := make([]float64, n1)
	copy(a, online)
	sort.Float64s(a)

	b := make([]float64, n2)
	copy(b, offline)
	sort.Float64s(b)

	// Walk through both sorted samples to find max |CDF1 - CDF2|.
	var i, j int
	var maxD float64
	fn1, fn2 := float64(n1), float64(n2)

	for i < n1 && j < n2 {
		if a[i] <= b[j] {
			i++
		} else {
			j++
		}
		d := math.Abs(float64(i)/fn1 - float64(j)/fn2)
		if d > maxD {
			maxD = d
		}
	}

	result.Statistic = maxD

	// Approximate p-value using the asymptotic Kolmogorov distribution.
	en := math.Sqrt(fn1 * fn2 / (fn1 + fn2))
	lambda := (en + 0.12 + 0.11/en) * maxD
	result.PValue = ksProb(lambda)

	result.Passed = result.Statistic <= threshold
	return result
}

// ksProb approximates the survival function of the Kolmogorov distribution
// using the series expansion Q_KS(λ) = 2 Σ (-1)^(j-1) exp(-2j²λ²).
func ksProb(lambda float64) float64 {
	if lambda <= 0 {
		return 1.0
	}
	if lambda >= 3.0 {
		return 0.0
	}

	twoLambdaSq := -2.0 * lambda * lambda
	p := 0.0
	sign := 1.0
	for j := 1; j <= 100; j++ {
		term := sign * math.Exp(twoLambdaSq*float64(j*j))
		p += term
		if math.Abs(term) < 1e-12 {
			break
		}
		sign = -sign
	}
	return math.Min(math.Max(2.0*p, 0.0), 1.0)
}

// PopulationStabilityIndex computes the PSI between two distributions.
// PSI = Σ (actual% - expected%) × ln(actual% / expected%)
// Samples are binned into numBins equal-width buckets using a shared range.
func PopulationStabilityIndex(actual, expected []float64, numBins int, threshold float64) StatTestResult {
	result := StatTestResult{
		TestName:   "psi",
		Threshold:  threshold,
		SampleSize: [2]int{len(actual), len(expected)},
	}

	if len(actual) == 0 || len(expected) == 0 || numBins <= 0 {
		result.Passed = true
		return result
	}

	aHist, eHist := sharedHistogram(actual, expected, numBins)

	psi := 0.0
	for i := 0; i < numBins; i++ {
		a := math.Max(aHist[i], 0.0001)
		e := math.Max(eHist[i], 0.0001)
		psi += (a - e) * math.Log(a/e)
	}

	result.Statistic = psi
	result.Passed = psi <= threshold
	return result
}

// ChiSquaredTest performs a chi-squared goodness-of-fit test for categorical data.
// χ² = Σ (O - E)² / E where O and E are observed and expected frequencies.
func ChiSquaredTest(observed, expected map[string]int, threshold float64) StatTestResult {
	result := StatTestResult{
		TestName:  "chi_squared",
		Threshold: threshold,
	}

	// Collect all categories from both maps.
	cats := make(map[string]struct{})
	obsTotal := 0
	expTotal := 0
	for k, v := range observed {
		cats[k] = struct{}{}
		obsTotal += v
	}
	for k, v := range expected {
		cats[k] = struct{}{}
		expTotal += v
	}

	result.SampleSize = [2]int{obsTotal, expTotal}

	if obsTotal == 0 || expTotal == 0 {
		result.Passed = true
		return result
	}

	// Scale expected frequencies to match observed total for fair comparison.
	scale := float64(obsTotal) / float64(expTotal)

	chiSq := 0.0
	for k := range cats {
		o := float64(observed[k])
		e := float64(expected[k]) * scale
		if e < 0.0001 {
			e = 0.0001
		}
		diff := o - e
		chiSq += (diff * diff) / e
	}

	result.Statistic = chiSq
	result.Passed = chiSq <= threshold
	return result
}

// JensenShannonDivergence computes the Jensen-Shannon divergence between two
// distributions. JSD = (KL(P||M) + KL(Q||M)) / 2, where M = (P+Q)/2.
// Returns the square root (metric form) for a bounded [0, 1] range.
func JensenShannonDivergence(p, q []float64, numBins int, threshold float64) StatTestResult {
	result := StatTestResult{
		TestName:   "jensen_shannon",
		Threshold:  threshold,
		SampleSize: [2]int{len(p), len(q)},
	}

	if len(p) == 0 || len(q) == 0 || numBins <= 0 {
		result.Passed = true
		return result
	}

	pHist, qHist := sharedHistogram(p, q, numBins)

	m := make([]float64, numBins)
	for i := 0; i < numBins; i++ {
		m[i] = 0.5 * (pHist[i] + qHist[i])
	}

	jsd := 0.5*klDivergence(pHist, m) + 0.5*klDivergence(qHist, m)
	result.Statistic = math.Sqrt(math.Max(jsd, 0))
	result.Passed = result.Statistic <= threshold
	return result
}

// AdvancedDistributionSnapshot captures a rich statistical summary of a
// distribution including moments (skewness, kurtosis) and additional percentiles.
type AdvancedDistributionSnapshot struct {
	Count     int       `json:"count"`
	Mean      float64   `json:"mean"`
	Variance  float64   `json:"variance"`
	Skewness  float64   `json:"skewness"`
	Kurtosis  float64   `json:"kurtosis"`
	Min       float64   `json:"min"`
	Max       float64   `json:"max"`
	P50       float64   `json:"p50"`
	P90       float64   `json:"p90"`
	P95       float64   `json:"p95"`
	P99       float64   `json:"p99"`
	Histogram []float64 `json:"histogram"`
}

// CaptureDistribution computes a full statistical snapshot of the given values
// including mean, variance, skewness, kurtosis, percentiles, and a histogram.
func CaptureDistribution(values []float64) *AdvancedDistributionSnapshot {
	snap := &AdvancedDistributionSnapshot{
		Count: len(values),
	}

	if len(values) == 0 {
		return snap
	}

	sorted := make([]float64, len(values))
	copy(sorted, values)
	sort.Float64s(sorted)

	snap.Min = sorted[0]
	snap.Max = sorted[len(sorted)-1]

	// Mean
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	snap.Mean = sum / float64(len(values))

	// Variance, skewness, kurtosis via central moments.
	n := float64(len(values))
	var m2, m3, m4 float64
	for _, v := range values {
		d := v - snap.Mean
		d2 := d * d
		m2 += d2
		m3 += d2 * d
		m4 += d2 * d2
	}
	snap.Variance = m2 / n

	if snap.Variance > 0 {
		sd := math.Sqrt(snap.Variance)
		snap.Skewness = (m3 / n) / (sd * sd * sd)
		snap.Kurtosis = (m4/n)/(snap.Variance*snap.Variance) - 3.0
	}

	// Percentiles
	snap.P50 = percentile(sorted, 0.50)
	snap.P90 = percentile(sorted, 0.90)
	snap.P95 = percentile(sorted, 0.95)
	snap.P99 = percentile(sorted, 0.99)

	snap.Histogram = histogram(values, 10)

	return snap
}
