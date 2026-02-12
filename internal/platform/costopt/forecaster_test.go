package costopt

import (
	"testing"
	"time"
)

func TestDefaultForecastConfig(t *testing.T) {
	cfg := DefaultForecastConfig()
	if cfg.ForecastHorizonDays != 30 {
		t.Errorf("expected 30 days, got %d", cfg.ForecastHorizonDays)
	}
	if cfg.ConfidenceLevel != 0.95 {
		t.Errorf("expected 0.95 confidence, got %f", cfg.ConfidenceLevel)
	}
}

func TestPredictInsufficient(t *testing.T) {
	f := NewForecaster(DefaultForecastConfig())
	fc := f.Predict()
	if fc.Trend != "stable" {
		t.Errorf("expected stable trend with no data, got %s", fc.Trend)
	}
}

func TestPredictLinearTrend(t *testing.T) {
	f := NewForecaster(DefaultForecastConfig())
	base := time.Now().Add(-30 * 24 * time.Hour)
	for i := 0; i < 30; i++ {
		f.AddDataPoint(CostDataPoint{
			Timestamp: base.Add(time.Duration(i) * 24 * time.Hour),
			Amount:    100 + float64(i)*10,
			Category:  "storage",
		})
	}

	fc := f.Predict()
	if fc.PredictedAmount <= 0 {
		t.Error("expected positive prediction")
	}
	if fc.Trend != "increasing" {
		t.Errorf("expected increasing trend, got %s", fc.Trend)
	}
	if fc.LowerBound > fc.PredictedAmount {
		t.Error("lower bound should be <= predicted")
	}
	if fc.UpperBound < fc.PredictedAmount {
		t.Error("upper bound should be >= predicted")
	}
}

func TestDetectAnomalies(t *testing.T) {
	f := NewForecaster(DefaultForecastConfig())
	base := time.Now()
	// Add normal data
	for i := 0; i < 20; i++ {
		f.AddDataPoint(CostDataPoint{
			Timestamp: base.Add(time.Duration(i) * time.Hour),
			Amount:    100,
			Category:  "storage",
		})
	}
	// Add anomaly
	f.AddDataPoint(CostDataPoint{
		Timestamp: base.Add(21 * time.Hour),
		Amount:    500,
		Category:  "storage",
	})

	anomalies := f.DetectAnomalies()
	if len(anomalies) == 0 {
		t.Error("expected at least one anomaly")
	}
	if anomalies[0].Severity != "critical" && anomalies[0].Severity != "warning" {
		t.Errorf("unexpected severity: %s", anomalies[0].Severity)
	}
}

func TestDetectAnomaliesNoData(t *testing.T) {
	f := NewForecaster(DefaultForecastConfig())
	if a := f.DetectAnomalies(); a != nil {
		t.Error("expected nil for insufficient data")
	}
}

func TestLinearRegression(t *testing.T) {
	xs := []float64{1, 2, 3, 4, 5}
	ys := []float64{2, 4, 6, 8, 10}
	slope, intercept := linearRegression(xs, ys)
	if slope != 2.0 {
		t.Errorf("expected slope 2.0, got %f", slope)
	}
	if intercept != 0.0 {
		t.Errorf("expected intercept 0.0, got %f", intercept)
	}
}

func TestLinearRegressionEmpty(t *testing.T) {
	slope, intercept := linearRegression(nil, nil)
	if slope != 0 || intercept != 0 {
		t.Error("expected zero slope and intercept for empty data")
	}
}
