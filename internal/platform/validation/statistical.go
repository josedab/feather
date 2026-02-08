package validation

import (
	"math"
	"sort"
)

// KolmogorovSmirnov computes the KS test statistic and approximate p-value
// for the two-sample Kolmogorov-Smirnov test. It returns the maximum absolute
// difference between the two empirical CDFs and an approximate p-value.
func KolmogorovSmirnov(sample1, sample2 []float64) (statistic float64, pvalue float64) {
	n1 := len(sample1)
	n2 := len(sample2)
	if n1 == 0 || n2 == 0 {
		return 0, 1.0
	}

	// Sort copies to avoid mutating input
	s1 := make([]float64, n1)
	s2 := make([]float64, n2)
	copy(s1, sample1)
	copy(s2, sample2)
	sort.Float64s(s1)
	sort.Float64s(s2)

	// Compute the KS statistic by walking both sorted samples
	var i, j int
	var maxDiff float64
	for i < n1 && j < n2 {
		if s1[i] < s2[j] {
			i++
		} else if s1[i] > s2[j] {
			j++
		} else {
			// Equal values: advance both pointers
			i++
			j++
		}
		cdf1 := float64(i) / float64(n1)
		cdf2 := float64(j) / float64(n2)
		diff := math.Abs(cdf1 - cdf2)
		if diff > maxDiff {
			maxDiff = diff
		}
	}

	statistic = maxDiff

	// Approximate p-value using the asymptotic formula
	n := float64(n1*n2) / float64(n1+n2)
	lambda := (math.Sqrt(n) + 0.12 + 0.11/math.Sqrt(n)) * statistic
	pvalue = 2 * math.Exp(-2*lambda*lambda)
	if pvalue > 1.0 {
		pvalue = 1.0
	}
	if pvalue < 0.0 {
		pvalue = 0.0
	}

	return statistic, pvalue
}

// MeanAbsoluteError computes the mean absolute error between predicted and
// actual slices. Returns 0 if either slice is empty.
func MeanAbsoluteError(predicted, actual []float64) float64 {
	n := len(predicted)
	if n == 0 || len(actual) == 0 {
		return 0
	}
	if len(actual) < n {
		n = len(actual)
	}

	var sum float64
	for i := 0; i < n; i++ {
		sum += math.Abs(predicted[i] - actual[i])
	}
	return sum / float64(n)
}

// RootMeanSquaredError computes the root mean squared error between predicted
// and actual slices. Returns 0 if either slice is empty.
func RootMeanSquaredError(predicted, actual []float64) float64 {
	n := len(predicted)
	if n == 0 || len(actual) == 0 {
		return 0
	}
	if len(actual) < n {
		n = len(actual)
	}

	var sumSq float64
	for i := 0; i < n; i++ {
		diff := predicted[i] - actual[i]
		sumSq += diff * diff
	}
	return math.Sqrt(sumSq / float64(n))
}

// PearsonCorrelation computes Pearson's correlation coefficient between x and y.
// Returns 0 if either slice has fewer than 2 elements or if the standard
// deviation of either variable is zero.
func PearsonCorrelation(x, y []float64) float64 {
	n := len(x)
	if n < 2 || len(y) < 2 {
		return 0
	}
	if len(y) < n {
		n = len(y)
	}

	var sumX, sumY float64
	for i := 0; i < n; i++ {
		sumX += x[i]
		sumY += y[i]
	}
	meanX := sumX / float64(n)
	meanY := sumY / float64(n)

	var cov, varX, varY float64
	for i := 0; i < n; i++ {
		dx := x[i] - meanX
		dy := y[i] - meanY
		cov += dx * dy
		varX += dx * dx
		varY += dy * dy
	}

	if varX == 0 || varY == 0 {
		return 0
	}

	return cov / (math.Sqrt(varX) * math.Sqrt(varY))
}

// PopulationStabilityIndex computes the PSI between expected and actual
// distributions using the specified number of bins. Returns 0 if either
// slice is empty or bins is less than 1.
func PopulationStabilityIndex(expected, actual []float64, bins int) float64 {
	if len(expected) == 0 || len(actual) == 0 || bins < 1 {
		return 0
	}

	// Find global min/max across both samples
	allMin := expected[0]
	allMax := expected[0]
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

	binWidth := (allMax - allMin) / float64(bins)

	// Count proportions in each bin
	expCounts := make([]float64, bins)
	actCounts := make([]float64, bins)

	for _, v := range expected {
		idx := int((v - allMin) / binWidth)
		if idx >= bins {
			idx = bins - 1
		}
		expCounts[idx]++
	}
	for _, v := range actual {
		idx := int((v - allMin) / binWidth)
		if idx >= bins {
			idx = bins - 1
		}
		actCounts[idx]++
	}

	// Convert to proportions with small epsilon to avoid division by zero
	const epsilon = 1e-10
	expTotal := float64(len(expected))
	actTotal := float64(len(actual))

	var psi float64
	for i := 0; i < bins; i++ {
		expProp := expCounts[i]/expTotal + epsilon
		actProp := actCounts[i]/actTotal + epsilon
		psi += (actProp - expProp) * math.Log(actProp/expProp)
	}

	return psi
}
