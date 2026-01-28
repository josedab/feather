// Package autofe provides automated feature engineering by detecting
// patterns in raw data, generating candidate features, and evaluating
// their predictive power.
package autofe

import (
	"fmt"
	"math"
	"sort"
	"sync"
	"time"
)

// TransformType identifies a feature transformation.
type TransformType string

const (
	// TransformLog applies natural logarithm.
	TransformLog TransformType = "log"
	// TransformSquare applies squaring.
	TransformSquare TransformType = "square"
	// TransformSqrt applies square root.
	TransformSqrt TransformType = "sqrt"
	// TransformBin discretizes into bins.
	TransformBin TransformType = "bin"
	// TransformInteraction multiplies two features.
	TransformInteraction TransformType = "interaction"
	// TransformRatio divides one feature by another.
	TransformRatio TransformType = "ratio"
	// TransformTimeSince calculates time since a timestamp.
	TransformTimeSince TransformType = "time_since"
	// TransformZScore normalizes to z-score.
	TransformZScore TransformType = "zscore"
	// TransformMinMax normalizes to [0,1] range.
	TransformMinMax TransformType = "minmax"
)

// CandidateFeature represents a generated feature candidate.
type CandidateFeature struct {
	Name           string        `json:"name"`
	Expression     string        `json:"expression"`
	Transform      TransformType `json:"transform"`
	SourceFeatures []string      `json:"source_features"`
	Score          float64       `json:"score"` // 0-1, higher is better
	Correlation    float64       `json:"correlation"`
	Coverage       float64       `json:"coverage"` // % non-null
	Explanation    string        `json:"explanation"`
	DataType       string        `json:"data_type"`
	GeneratedAt    time.Time     `json:"generated_at"`
}

// ColumnStats holds descriptive statistics for a data column.
type ColumnStats struct {
	Name      string    `json:"name"`
	DataType  string    `json:"data_type"`
	Count     int       `json:"count"`
	NullCount int       `json:"null_count"`
	Mean      float64   `json:"mean"`
	StdDev    float64   `json:"std_dev"`
	Min       float64   `json:"min"`
	Max       float64   `json:"max"`
	Skewness  float64   `json:"skewness"`
	Kurtosis  float64   `json:"kurtosis"`
	Values    []float64 `json:"-"` // raw data for computations
}

// Dataset represents input data for feature generation.
type Dataset struct {
	Columns map[string]*ColumnStats `json:"columns"`
	Target  string                  `json:"target"`
	Rows    int                     `json:"rows"`
}

// EngineConfig configures the AutoFE engine.
type EngineConfig struct {
	MaxCandidates        int
	CorrelationThreshold float64
	MinCoverage          float64
	EnableInteractions   bool
	EnableTimeBased      bool
	BinCounts            []int
}

// DefaultEngineConfig returns sensible defaults.
func DefaultEngineConfig() EngineConfig {
	return EngineConfig{
		MaxCandidates:        100,
		CorrelationThreshold: 0.95,
		MinCoverage:          0.8,
		EnableInteractions:   true,
		EnableTimeBased:      true,
		BinCounts:            []int{5, 10, 20},
	}
}

// Engine generates and evaluates candidate features.
type Engine struct {
	config     EngineConfig
	candidates []*CandidateFeature
	mu         sync.RWMutex
}

// NewEngine creates a new AutoFE engine.
func NewEngine(cfg EngineConfig) *Engine {
	return &Engine{config: cfg}
}

// Generate produces candidate features from the given dataset.
func (e *Engine) Generate(dataset *Dataset) ([]*CandidateFeature, error) {
	if dataset == nil || len(dataset.Columns) == 0 {
		return nil, fmt.Errorf("dataset must have at least one column")
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	e.candidates = nil
	now := time.Now()

	numericCols := e.getNumericColumns(dataset)

	// Single-column transforms
	for _, col := range numericCols {
		if col.Name == dataset.Target {
			continue
		}

		// Log transform (for right-skewed data)
		if col.Min > 0 && col.Skewness > 1.0 {
			e.addCandidate(&CandidateFeature{
				Name:           fmt.Sprintf("log_%s", col.Name),
				Expression:     fmt.Sprintf("log(%s)", col.Name),
				Transform:      TransformLog,
				SourceFeatures: []string{col.Name},
				Explanation:    fmt.Sprintf("Log transform reduces right skew (skewness=%.2f)", col.Skewness),
				DataType:       "float64",
				GeneratedAt:    now,
			})
		}

		// Z-score normalization
		if col.StdDev > 0 {
			e.addCandidate(&CandidateFeature{
				Name:           fmt.Sprintf("zscore_%s", col.Name),
				Expression:     fmt.Sprintf("((%s - %.4f) / %.4f)", col.Name, col.Mean, col.StdDev),
				Transform:      TransformZScore,
				SourceFeatures: []string{col.Name},
				Explanation:    "Z-score normalization for features with high variance",
				DataType:       "float64",
				GeneratedAt:    now,
			})
		}

		// Min-max normalization
		if col.Max > col.Min {
			e.addCandidate(&CandidateFeature{
				Name:           fmt.Sprintf("minmax_%s", col.Name),
				Expression:     fmt.Sprintf("((%s - %.4f) / %.4f)", col.Name, col.Min, col.Max-col.Min),
				Transform:      TransformMinMax,
				SourceFeatures: []string{col.Name},
				Explanation:    "Min-max normalization to [0,1] range",
				DataType:       "float64",
				GeneratedAt:    now,
			})
		}

		// Square root (for moderate skew)
		if col.Min >= 0 && col.Skewness > 0.5 && col.Skewness <= 1.0 {
			e.addCandidate(&CandidateFeature{
				Name:           fmt.Sprintf("sqrt_%s", col.Name),
				Expression:     fmt.Sprintf("sqrt(%s)", col.Name),
				Transform:      TransformSqrt,
				SourceFeatures: []string{col.Name},
				Explanation:    "Square root transform for moderate positive skew",
				DataType:       "float64",
				GeneratedAt:    now,
			})
		}

		// Binning
		for _, bins := range e.config.BinCounts {
			e.addCandidate(&CandidateFeature{
				Name:           fmt.Sprintf("bin%d_%s", bins, col.Name),
				Expression:     fmt.Sprintf("bin(%s, %d)", col.Name, bins),
				Transform:      TransformBin,
				SourceFeatures: []string{col.Name},
				Explanation:    fmt.Sprintf("Discretize into %d equal-width bins", bins),
				DataType:       "int64",
				GeneratedAt:    now,
			})
		}
	}

	// Interaction features (pairwise)
	if e.config.EnableInteractions && len(numericCols) >= 2 {
		for i := 0; i < len(numericCols); i++ {
			for j := i + 1; j < len(numericCols); j++ {
				a, b := numericCols[i], numericCols[j]
				if a.Name == dataset.Target || b.Name == dataset.Target {
					continue
				}

				// Product interaction
				e.addCandidate(&CandidateFeature{
					Name:           fmt.Sprintf("%s_x_%s", a.Name, b.Name),
					Expression:     fmt.Sprintf("(%s * %s)", a.Name, b.Name),
					Transform:      TransformInteraction,
					SourceFeatures: []string{a.Name, b.Name},
					Explanation:    "Multiplicative interaction captures non-linear relationships",
					DataType:       "float64",
					GeneratedAt:    now,
				})

				// Ratio (if denominator is always positive)
				if b.Min > 0 {
					e.addCandidate(&CandidateFeature{
						Name:           fmt.Sprintf("%s_per_%s", a.Name, b.Name),
						Expression:     fmt.Sprintf("(%s / %s)", a.Name, b.Name),
						Transform:      TransformRatio,
						SourceFeatures: []string{a.Name, b.Name},
						Explanation:    fmt.Sprintf("Ratio normalizes %s by %s", a.Name, b.Name),
						DataType:       "float64",
						GeneratedAt:    now,
					})
				}
			}
		}
	}

	// Score candidates
	e.scoreCandidates(dataset)

	// Sort by score
	sort.Slice(e.candidates, func(i, j int) bool {
		return e.candidates[i].Score > e.candidates[j].Score
	})

	// Trim to max candidates
	if len(e.candidates) > e.config.MaxCandidates {
		e.candidates = e.candidates[:e.config.MaxCandidates]
	}

	result := make([]*CandidateFeature, len(e.candidates))
	copy(result, e.candidates)
	return result, nil
}

// TopN returns the top N candidates by score.
func (e *Engine) TopN(n int) []*CandidateFeature {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if n > len(e.candidates) {
		n = len(e.candidates)
	}
	result := make([]*CandidateFeature, n)
	copy(result, e.candidates[:n])
	return result
}

// Stats returns engine statistics.
func (e *Engine) Stats() map[string]interface{} {
	e.mu.RLock()
	defer e.mu.RUnlock()

	transformCounts := make(map[TransformType]int)
	for _, c := range e.candidates {
		transformCounts[c.Transform]++
	}

	return map[string]interface{}{
		"total_candidates": len(e.candidates),
		"max_candidates":   e.config.MaxCandidates,
		"transform_counts": transformCounts,
	}
}

func (e *Engine) addCandidate(c *CandidateFeature) {
	if len(e.candidates) < e.config.MaxCandidates*2 {
		e.candidates = append(e.candidates, c)
	}
}

func (e *Engine) scoreCandidates(dataset *Dataset) {
	target, hasTarget := dataset.Columns[dataset.Target]

	for _, candidate := range e.candidates {
		score := 0.5 // base score

		// Score based on transform type heuristics
		switch candidate.Transform {
		case TransformLog:
			score += 0.2
		case TransformZScore:
			score += 0.15
		case TransformInteraction:
			score += 0.1
		case TransformRatio:
			score += 0.15
		case TransformBin:
			score += 0.05
		case TransformMinMax:
			score += 0.1
		case TransformSqrt:
			score += 0.1
		}

		// Boost if source has high correlation with target
		if hasTarget && len(target.Values) > 0 {
			for _, src := range candidate.SourceFeatures {
				if col, ok := dataset.Columns[src]; ok && len(col.Values) > 0 {
					corr := math.Abs(pearsonCorrelation(col.Values, target.Values))
					candidate.Correlation = corr
					score += corr * 0.3
				}
			}
		}

		// Coverage score
		for _, src := range candidate.SourceFeatures {
			if col, ok := dataset.Columns[src]; ok && col.Count > 0 {
				coverage := float64(col.Count-col.NullCount) / float64(col.Count)
				candidate.Coverage = coverage
				if coverage < e.config.MinCoverage {
					score *= 0.5
				}
			}
		}

		candidate.Score = math.Min(score, 1.0)
	}
}

func (e *Engine) getNumericColumns(dataset *Dataset) []*ColumnStats {
	var cols []*ColumnStats
	for _, col := range dataset.Columns {
		if col.DataType == "float64" || col.DataType == "int64" {
			cols = append(cols, col)
		}
	}
	return cols
}

func pearsonCorrelation(x, y []float64) float64 {
	n := len(x)
	if n != len(y) || n == 0 {
		return 0
	}

	var sumX, sumY, sumXY, sumX2, sumY2 float64
	for i := 0; i < n; i++ {
		sumX += x[i]
		sumY += y[i]
		sumXY += x[i] * y[i]
		sumX2 += x[i] * x[i]
		sumY2 += y[i] * y[i]
	}

	nf := float64(n)
	num := nf*sumXY - sumX*sumY
	den := math.Sqrt((nf*sumX2 - sumX*sumX) * (nf*sumY2 - sumY*sumY))
	if den == 0 {
		return 0
	}
	return num / den
}

// ComputeColumnStats computes statistics for a column of float64 values.
func ComputeColumnStats(name, dataType string, values []float64) *ColumnStats {
	n := len(values)
	if n == 0 {
		return &ColumnStats{Name: name, DataType: dataType}
	}

	stats := &ColumnStats{
		Name:     name,
		DataType: dataType,
		Count:    n,
		Values:   values,
	}

	var sum float64
	stats.Min = values[0]
	stats.Max = values[0]
	for _, v := range values {
		sum += v
		if v < stats.Min {
			stats.Min = v
		}
		if v > stats.Max {
			stats.Max = v
		}
	}
	stats.Mean = sum / float64(n)

	// StdDev & skewness
	var variance, m3 float64
	for _, v := range values {
		d := v - stats.Mean
		variance += d * d
		m3 += d * d * d
	}
	variance /= float64(n)
	stats.StdDev = math.Sqrt(variance)

	if stats.StdDev > 0 {
		stats.Skewness = (m3 / float64(n)) / math.Pow(stats.StdDev, 3)
	}

	return stats
}
