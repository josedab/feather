package dashboard

import (
	"math"
	"testing"
	"time"
)

func TestComputeCorrelation_Positive(t *testing.T) {
	e := NewExplorer(DefaultExplorerConfig())
	a := []float64{1, 2, 3, 4, 5}
	b := []float64{2, 4, 6, 8, 10}
	result, err := e.ComputeCorrelation("f1", "f2", a, b)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Correlation < 0.99 {
		t.Errorf("expected strong positive correlation, got %f", result.Correlation)
	}
}

func TestComputeCorrelation_Negative(t *testing.T) {
	e := NewExplorer(DefaultExplorerConfig())
	a := []float64{1, 2, 3, 4, 5}
	b := []float64{10, 8, 6, 4, 2}
	result, err := e.ComputeCorrelation("f1", "f2", a, b)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Correlation > -0.99 {
		t.Errorf("expected strong negative correlation, got %f", result.Correlation)
	}
}

func TestComputeCorrelation_Zero(t *testing.T) {
	e := NewExplorer(DefaultExplorerConfig())
	a := []float64{1, 2, 3, 4, 5}
	b := []float64{3, 1, 4, 1, 5}
	result, err := e.ComputeCorrelation("f1", "f2", a, b)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if math.Abs(result.Correlation) > 0.5 {
		t.Errorf("expected near-zero correlation, got %f", result.Correlation)
	}
}

func TestComputeCorrelation_SingleDataPoint(t *testing.T) {
	e := NewExplorer(DefaultExplorerConfig())
	_, err := e.ComputeCorrelation("f1", "f2", []float64{1}, []float64{2})
	if err == nil {
		t.Error("expected error for single data point")
	}
}

func TestComputeCorrelation_ZeroVariance(t *testing.T) {
	e := NewExplorer(DefaultExplorerConfig())
	a := []float64{5, 5, 5, 5}
	b := []float64{1, 2, 3, 4}
	_, err := e.ComputeCorrelation("f1", "f2", a, b)
	if err == nil {
		t.Error("expected error for zero variance")
	}
}

func TestComputeCorrelation_EmptyData(t *testing.T) {
	e := NewExplorer(DefaultExplorerConfig())
	_, err := e.ComputeCorrelation("f1", "f2", []float64{}, []float64{})
	if err == nil {
		t.Error("expected error for empty data")
	}
}

func TestRecordInsight_GetInsight(t *testing.T) {
	e := NewExplorer(DefaultExplorerConfig())
	insight := &FeatureInsight{
		FeatureID:   "clicks",
		EntityCount: 100,
		MeanValue:   42.5,
		LastUpdated: time.Now(),
	}
	e.RecordInsight(insight)

	got, err := e.GetInsight("clicks")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.EntityCount != 100 {
		t.Errorf("expected entity count 100, got %d", got.EntityCount)
	}
}

func TestGetInsight_NotFound(t *testing.T) {
	e := NewExplorer(DefaultExplorerConfig())
	_, err := e.GetInsight("nonexistent")
	if err == nil {
		t.Error("expected error for missing insight")
	}
}

func TestListInsights(t *testing.T) {
	e := NewExplorer(DefaultExplorerConfig())
	e.RecordInsight(&FeatureInsight{FeatureID: "f1"})
	e.RecordInsight(&FeatureInsight{FeatureID: "f2"})
	e.RecordInsight(&FeatureInsight{FeatureID: "f3"})

	insights := e.ListInsights()
	if len(insights) != 3 {
		t.Errorf("expected 3 insights, got %d", len(insights))
	}
}

func TestSearchInsights_SubstringMatch(t *testing.T) {
	e := NewExplorer(DefaultExplorerConfig())
	e.RecordInsight(&FeatureInsight{FeatureID: "user_clicks"})
	e.RecordInsight(&FeatureInsight{FeatureID: "user_views"})
	e.RecordInsight(&FeatureInsight{FeatureID: "product_price"})

	results := e.SearchInsights("user")
	if len(results) != 2 {
		t.Errorf("expected 2 matches for 'user', got %d", len(results))
	}
}

func TestSearchInsights_EmptyQuery(t *testing.T) {
	e := NewExplorer(DefaultExplorerConfig())
	e.RecordInsight(&FeatureInsight{FeatureID: "f1"})
	e.RecordInsight(&FeatureInsight{FeatureID: "f2"})

	results := e.SearchInsights("")
	if len(results) != 2 {
		t.Errorf("empty query should match all, got %d", len(results))
	}
}

func TestSearchInsights_NoMatches(t *testing.T) {
	e := NewExplorer(DefaultExplorerConfig())
	e.RecordInsight(&FeatureInsight{FeatureID: "clicks"})

	results := e.SearchInsights("zzzzz")
	if len(results) != 0 {
		t.Errorf("expected 0 matches, got %d", len(results))
	}
}

func TestRecordUsagePattern_GetUsagePattern(t *testing.T) {
	e := NewExplorer(DefaultExplorerConfig())
	e.RecordUsagePattern(&UsagePattern{
		FeatureID:     "clicks",
		TotalAccesses: 500,
		PeakHour:      14,
	})

	got, err := e.GetUsagePattern("clicks")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.TotalAccesses != 500 {
		t.Errorf("expected 500 accesses, got %d", got.TotalAccesses)
	}
}

func TestGetUsagePattern_NotFound(t *testing.T) {
	e := NewExplorer(DefaultExplorerConfig())
	_, err := e.GetUsagePattern("nonexistent")
	if err == nil {
		t.Error("expected error for missing pattern")
	}
}

func TestListUsagePatterns(t *testing.T) {
	e := NewExplorer(DefaultExplorerConfig())
	e.RecordUsagePattern(&UsagePattern{FeatureID: "f1"})
	e.RecordUsagePattern(&UsagePattern{FeatureID: "f2"})
	patterns := e.ListUsagePatterns()
	if len(patterns) != 2 {
		t.Errorf("expected 2 patterns, got %d", len(patterns))
	}
}

func TestRecordCost_GetCostBreakdown(t *testing.T) {
	e := NewExplorer(DefaultExplorerConfig())
	e.RecordCost(&CostBreakdown{
		FeatureID:               "clicks",
		StorageMB:               100,
		ReadOps:                 5000,
		EstimatedMonthlyCostUSD: 12.50,
	})

	got, err := e.GetCostBreakdown("clicks")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.EstimatedMonthlyCostUSD != 12.50 {
		t.Errorf("expected cost 12.50, got %f", got.EstimatedMonthlyCostUSD)
	}
}

func TestGetCostBreakdown_NotFound(t *testing.T) {
	e := NewExplorer(DefaultExplorerConfig())
	_, err := e.GetCostBreakdown("nonexistent")
	if err == nil {
		t.Error("expected error for missing cost")
	}
}

func TestGetTotalCosts(t *testing.T) {
	e := NewExplorer(DefaultExplorerConfig())
	e.RecordCost(&CostBreakdown{FeatureID: "f1", EstimatedMonthlyCostUSD: 10})
	e.RecordCost(&CostBreakdown{FeatureID: "f2", EstimatedMonthlyCostUSD: 20})

	costs := e.GetTotalCosts()
	if len(costs) != 2 {
		t.Errorf("expected 2 costs, got %d", len(costs))
	}
}

func TestGetTotalCosts_Empty(t *testing.T) {
	e := NewExplorer(DefaultExplorerConfig())
	costs := e.GetTotalCosts()
	if len(costs) != 0 {
		t.Errorf("expected 0 costs, got %d", len(costs))
	}
}

func TestListCorrelations_ThresholdFiltering(t *testing.T) {
	e := NewExplorer(DefaultExplorerConfig())
	// Generate a strong correlation
	a := []float64{1, 2, 3, 4, 5}
	b := []float64{2, 4, 6, 8, 10}
	_, _ = e.ComputeCorrelation("f1", "f2", a, b)

	// Generate a weak correlation
	c := []float64{1, 2, 3, 4, 5}
	d := []float64{3, 1, 4, 1, 5}
	_, _ = e.ComputeCorrelation("f3", "f4", c, d)

	strong := e.ListCorrelations(0.9)
	if len(strong) < 1 {
		t.Error("expected at least 1 strong correlation")
	}

	all := e.ListCorrelations(0.0)
	if len(all) < 2 {
		t.Error("expected at least 2 correlations with threshold 0")
	}
}

func TestExplorerStats(t *testing.T) {
	e := NewExplorer(DefaultExplorerConfig())
	e.RecordInsight(&FeatureInsight{FeatureID: "f1"})
	e.RecordInsight(&FeatureInsight{FeatureID: "f2"})
	a := []float64{1, 2, 3, 4, 5}
	b := []float64{2, 4, 6, 8, 10}
	_, _ = e.ComputeCorrelation("f1", "f2", a, b)

	stats := e.Stats()
	if stats.TotalInsights != 2 {
		t.Errorf("expected 2 insights, got %d", stats.TotalInsights)
	}
	if stats.CorrelationsComputed != 1 {
		t.Errorf("expected 1 correlation, got %d", stats.CorrelationsComputed)
	}
}
