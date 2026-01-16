package costopt

import (
	"fmt"
	"math"
	"sort"
	"sync"
	"time"
)

// ForecastConfig controls cost forecasting behaviour.
type ForecastConfig struct {
	ForecastHorizonDays int
	ConfidenceLevel     float64
}

// DefaultForecastConfig returns sensible defaults.
func DefaultForecastConfig() ForecastConfig {
	return ForecastConfig{
		ForecastHorizonDays: 30,
		ConfidenceLevel:     0.95,
	}
}

// CostDataPoint is a single historical cost observation.
type CostDataPoint struct {
	Timestamp time.Time `json:"timestamp"`
	Amount    float64   `json:"amount"`
	Category  string    `json:"category"`
}

// Forecast is a predicted future cost.
type Forecast struct {
	Period          string  `json:"period"`
	PredictedAmount float64 `json:"predicted_amount"`
	LowerBound      float64 `json:"lower_bound"`
	UpperBound      float64 `json:"upper_bound"`
	Trend           string  `json:"trend"`
	TrendPct        float64 `json:"trend_pct"`
}

// CostAnomaly describes an unusual cost data point.
type CostAnomaly struct {
	Timestamp    time.Time `json:"timestamp"`
	Amount       float64   `json:"amount"`
	ExpectedAmount float64 `json:"expected_amount"`
	DeviationPct float64   `json:"deviation_pct"`
	Severity     string    `json:"severity"`
}

// Forecaster performs cost forecasting using linear regression.
type Forecaster struct {
	mu     sync.RWMutex
	config ForecastConfig
	points []CostDataPoint
}

// NewForecaster creates a Forecaster with the given configuration.
func NewForecaster(config ForecastConfig) *Forecaster {
	return &Forecaster{
		config: config,
		points: make([]CostDataPoint, 0, 256),
	}
}

// AddDataPoint adds a historical cost observation.
func (f *Forecaster) AddDataPoint(point CostDataPoint) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.points = append(f.points, point)
}

// Predict generates a cost forecast using linear regression over data points.
func (f *Forecaster) Predict() *Forecast {
	f.mu.RLock()
	defer f.mu.RUnlock()

	if len(f.points) < 2 {
		return &Forecast{
			Period: fmt.Sprintf("%d days", f.config.ForecastHorizonDays),
			Trend:  "stable",
		}
	}

	sorted := make([]CostDataPoint, len(f.points))
	copy(sorted, f.points)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Timestamp.Before(sorted[j].Timestamp)
	})

	origin := sorted[0].Timestamp
	xs := make([]float64, len(sorted))
	ys := make([]float64, len(sorted))
	for i, p := range sorted {
		xs[i] = p.Timestamp.Sub(origin).Hours() / 24 // days since origin
		ys[i] = p.Amount
	}

	slope, intercept := linearRegression(xs, ys)
	futureDays := float64(f.config.ForecastHorizonDays) + xs[len(xs)-1]
	predicted := slope*futureDays + intercept

	// Compute residual standard error for confidence bounds.
	var sumResidSq float64
	for i := range xs {
		residual := ys[i] - (slope*xs[i] + intercept)
		sumResidSq += residual * residual
	}
	stdErr := math.Sqrt(sumResidSq / float64(len(xs)))
	// Approximate z-score for 95% CI.
	z := 1.96
	if f.config.ConfidenceLevel < 0.95 {
		z = 1.645
	}

	lower := predicted - z*stdErr
	upper := predicted + z*stdErr
	if lower < 0 {
		lower = 0
	}

	// Determine trend.
	firstVal := ys[0]
	lastVal := ys[len(ys)-1]
	trend := "stable"
	trendPct := 0.0
	if firstVal > 0 {
		trendPct = ((lastVal - firstVal) / firstVal) * 100
	}
	if trendPct > 5 {
		trend = "increasing"
	} else if trendPct < -5 {
		trend = "decreasing"
	}

	return &Forecast{
		Period:          fmt.Sprintf("%d days", f.config.ForecastHorizonDays),
		PredictedAmount: math.Round(predicted*100) / 100,
		LowerBound:      math.Round(lower*100) / 100,
		UpperBound:      math.Round(upper*100) / 100,
		Trend:           trend,
		TrendPct:        math.Round(trendPct*100) / 100,
	}
}

// DetectAnomalies returns data points that deviate more than 2 standard deviations from the mean.
func (f *Forecaster) DetectAnomalies() []*CostAnomaly {
	f.mu.RLock()
	defer f.mu.RUnlock()

	if len(f.points) < 3 {
		return nil
	}

	var sum float64
	for _, p := range f.points {
		sum += p.Amount
	}
	mean := sum / float64(len(f.points))

	var varianceSum float64
	for _, p := range f.points {
		d := p.Amount - mean
		varianceSum += d * d
	}
	stdDev := math.Sqrt(varianceSum / float64(len(f.points)))
	if stdDev == 0 {
		return nil
	}

	var anomalies []*CostAnomaly
	for _, p := range f.points {
		deviation := math.Abs(p.Amount - mean)
		if deviation > 2*stdDev {
			devPct := (deviation / mean) * 100
			severity := "warning"
			if deviation > 3*stdDev {
				severity = "critical"
			}
			anomalies = append(anomalies, &CostAnomaly{
				Timestamp:      p.Timestamp,
				Amount:         p.Amount,
				ExpectedAmount: math.Round(mean*100) / 100,
				DeviationPct:   math.Round(devPct*100) / 100,
				Severity:       severity,
			})
		}
	}
	return anomalies
}

// linearRegression computes slope and intercept for y = slope*x + intercept.
func linearRegression(xs, ys []float64) (slope, intercept float64) {
	n := float64(len(xs))
	if n == 0 {
		return 0, 0
	}
	var sumX, sumY, sumXY, sumX2 float64
	for i := range xs {
		sumX += xs[i]
		sumY += ys[i]
		sumXY += xs[i] * ys[i]
		sumX2 += xs[i] * xs[i]
	}
	denom := n*sumX2 - sumX*sumX
	if denom == 0 {
		return 0, sumY / n
	}
	slope = (n*sumXY - sumX*sumY) / denom
	intercept = (sumY - slope*sumX) / n
	return slope, intercept
}
