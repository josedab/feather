package autofe

import (
	"fmt"
	"math"
	"sort"
)

// ProfilerConfig configures the data profiler.
type ProfilerConfig struct {
	// OutlierStdDevThreshold defines how many std devs from the mean counts as an outlier.
	OutlierStdDevThreshold float64
	// HighCorrelationThreshold defines the minimum absolute correlation to flag.
	HighCorrelationThreshold float64
	// LowUniquenessThreshold defines the minimum uniqueness ratio to consider healthy.
	LowUniquenessThreshold float64
	// HighMissingThreshold defines the maximum missing ratio before flagging.
	HighMissingThreshold float64
}

// DefaultProfilerConfig returns sensible defaults.
func DefaultProfilerConfig() ProfilerConfig {
	return ProfilerConfig{
		OutlierStdDevThreshold:   3.0,
		HighCorrelationThreshold: 0.9,
		LowUniquenessThreshold:   0.01,
		HighMissingThreshold:     0.2,
	}
}

// ColumnProfile contains detailed profiling information for a single column.
type ColumnProfile struct {
	Name             string  `json:"name"`
	DataType         string  `json:"data_type"`
	DistributionType string  `json:"distribution_type"`
	OutlierCount     int     `json:"outlier_count"`
	UniquenessRatio  float64 `json:"uniqueness_ratio"`
	MissingRatio     float64 `json:"missing_ratio"`
	Mean             float64 `json:"mean"`
	StdDev           float64 `json:"std_dev"`
	Min              float64 `json:"min"`
	Max              float64 `json:"max"`
	Skewness         float64 `json:"skewness"`
}

// ProfileReport contains the full profiling output for a dataset.
type ProfileReport struct {
	ColumnProfiles   []*ColumnProfile   `json:"column_profiles"`
	Correlations     map[string]float64 `json:"correlations"`
	DataQualityScore float64            `json:"data_quality_score"`
	Recommendations  []string           `json:"recommendations"`
	TotalRows        int                `json:"total_rows"`
	TotalColumns     int                `json:"total_columns"`
}

// Profiler analyzes datasets and generates quality reports.
type Profiler struct {
	config ProfilerConfig
}

// NewProfiler creates a new Profiler.
func NewProfiler(cfg ProfilerConfig) *Profiler {
	return &Profiler{config: cfg}
}

// Profile analyzes the dataset and produces a ProfileReport.
func (p *Profiler) Profile(dataset *Dataset) (*ProfileReport, error) {
	if dataset == nil || len(dataset.Columns) == 0 {
		return nil, fmt.Errorf("profiling dataset: dataset must have at least one column")
	}

	report := &ProfileReport{
		Correlations: make(map[string]float64),
		TotalRows:    dataset.Rows,
		TotalColumns: len(dataset.Columns),
	}

	// Profile each column.
	for _, col := range dataset.Columns {
		cp := p.profileColumn(col)
		report.ColumnProfiles = append(report.ColumnProfiles, cp)
	}

	// Sort profiles by name for deterministic output.
	sort.Slice(report.ColumnProfiles, func(i, j int) bool {
		return report.ColumnProfiles[i].Name < report.ColumnProfiles[j].Name
	})

	// Compute pairwise correlations for numeric columns.
	p.computeCorrelations(dataset, report)

	// Compute quality score and recommendations.
	p.computeQualityScore(report)

	return report, nil
}

func (p *Profiler) profileColumn(col *ColumnStats) *ColumnProfile {
	cp := &ColumnProfile{
		Name:     col.Name,
		DataType: col.DataType,
		Mean:     col.Mean,
		StdDev:   col.StdDev,
		Min:      col.Min,
		Max:      col.Max,
		Skewness: col.Skewness,
	}

	if col.Count > 0 {
		cp.MissingRatio = float64(col.NullCount) / float64(col.Count)
	}

	// Compute uniqueness ratio from values.
	if len(col.Values) > 0 {
		unique := make(map[float64]struct{}, len(col.Values))
		for _, v := range col.Values {
			unique[v] = struct{}{}
		}
		cp.UniquenessRatio = float64(len(unique)) / float64(len(col.Values))
	}

	// Count outliers using std dev threshold.
	if col.StdDev > 0 && len(col.Values) > 0 {
		threshold := p.config.OutlierStdDevThreshold
		for _, v := range col.Values {
			if math.Abs(v-col.Mean) > threshold*col.StdDev {
				cp.OutlierCount++
			}
		}
	}

	cp.DistributionType = DetectDistribution(col.Values)
	return cp
}

func (p *Profiler) computeCorrelations(dataset *Dataset, report *ProfileReport) {
	cols := make([]*ColumnStats, 0, len(dataset.Columns))
	for _, col := range dataset.Columns {
		if len(col.Values) > 0 {
			cols = append(cols, col)
		}
	}

	for i := 0; i < len(cols); i++ {
		for j := i + 1; j < len(cols); j++ {
			corr := pearsonCorrelation(cols[i].Values, cols[j].Values)
			key := fmt.Sprintf("%s_%s", cols[i].Name, cols[j].Name)
			report.Correlations[key] = corr
		}
	}
}

func (p *Profiler) computeQualityScore(report *ProfileReport) {
	if len(report.ColumnProfiles) == 0 {
		return
	}

	var totalScore float64
	for _, cp := range report.ColumnProfiles {
		colScore := 1.0

		// Penalize high missing ratio.
		if cp.MissingRatio > p.config.HighMissingThreshold {
			colScore -= 0.3
			report.Recommendations = append(report.Recommendations,
				fmt.Sprintf("Column %q has %.0f%% missing values — consider imputation", cp.Name, cp.MissingRatio*100))
		}

		// Penalize low uniqueness.
		if cp.UniquenessRatio > 0 && cp.UniquenessRatio < p.config.LowUniquenessThreshold {
			colScore -= 0.2
			report.Recommendations = append(report.Recommendations,
				fmt.Sprintf("Column %q has very low uniqueness (%.2f%%) — may be constant", cp.Name, cp.UniquenessRatio*100))
		}

		// Penalize many outliers.
		if cp.OutlierCount > 0 && len(report.ColumnProfiles) > 0 {
			outlierRatio := float64(cp.OutlierCount) / float64(report.TotalRows)
			if outlierRatio > 0.05 {
				colScore -= 0.1
				report.Recommendations = append(report.Recommendations,
					fmt.Sprintf("Column %q has %d outliers (%.1f%%) — consider winsorizing", cp.Name, cp.OutlierCount, outlierRatio*100))
			}
		}

		if colScore < 0 {
			colScore = 0
		}
		totalScore += colScore
	}

	report.DataQualityScore = totalScore / float64(len(report.ColumnProfiles))

	// Flag highly correlated pairs.
	for pair, corr := range report.Correlations {
		if math.Abs(corr) > p.config.HighCorrelationThreshold {
			report.Recommendations = append(report.Recommendations,
				fmt.Sprintf("Pair %s has high correlation (%.2f) — consider dropping one", pair, corr))
		}
	}
}

// DetectDistribution classifies a set of values into a distribution type.
func DetectDistribution(values []float64) string {
	if len(values) < 4 {
		return "unknown"
	}

	n := float64(len(values))
	var sum float64
	for _, v := range values {
		sum += v
	}
	mean := sum / n

	var variance, m3, m4 float64
	for _, v := range values {
		d := v - mean
		d2 := d * d
		variance += d2
		m3 += d2 * d
		m4 += d2 * d2
	}
	variance /= n
	stdDev := math.Sqrt(variance)

	if stdDev == 0 {
		return "uniform"
	}

	skewness := (m3 / n) / math.Pow(stdDev, 3)
	kurtosis := (m4/n)/math.Pow(stdDev, 4) - 3.0 // excess kurtosis

	// Heuristic classification.
	absSkew := math.Abs(skewness)

	if absSkew < 0.5 && kurtosis > -0.5 && kurtosis < 1.0 {
		return "normal"
	}
	if absSkew < 0.5 && kurtosis < -1.0 {
		return "uniform"
	}
	if kurtosis < -1.0 {
		return "bimodal"
	}
	return "skewed"
}
