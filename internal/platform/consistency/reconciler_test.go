package consistency

import (
	"testing"
	"time"
)

func TestMonitor_AnalyzeSkew(t *testing.T) {
	monitor := NewMonitor(nil, DefaultReconcileConfig())

	diff1 := 0.5
	diff2 := 1.5
	diff3 := 0.0
	results := []*Result{
		{Feature: "clicks", IsConsistent: true, Difference: &diff3, CheckedAt: time.Now()},
		{Feature: "clicks", IsConsistent: false, Difference: &diff1, CheckedAt: time.Now()},
		{Feature: "clicks", IsConsistent: false, Difference: &diff2, CheckedAt: time.Now()},
		{Feature: "spend", IsConsistent: true, Difference: &diff3, CheckedAt: time.Now()},
	}

	reports := monitor.AnalyzeSkew(results)
	if len(reports) != 2 {
		t.Fatalf("expected 2 reports, got %d", len(reports))
	}

	var clicksReport *SkewReport
	for _, r := range reports {
		if r.Feature == "clicks" {
			clicksReport = r
			break
		}
	}
	if clicksReport == nil {
		t.Fatal("clicks report not found")
	}

	if clicksReport.SampleSize != 3 {
		t.Fatalf("expected 3 samples, got %d", clicksReport.SampleSize)
	}
	expectedRate := float64(1) / float64(3) * 100
	if clicksReport.ConsistencyRate < expectedRate-0.1 || clicksReport.ConsistencyRate > expectedRate+0.1 {
		t.Fatalf("expected ~%.1f%% consistency, got %.1f%%", expectedRate, clicksReport.ConsistencyRate)
	}
	if clicksReport.MaxAbsDifference != 1.5 {
		t.Fatalf("expected max diff 1.5, got %f", clicksReport.MaxAbsDifference)
	}
}

func TestMonitor_RecordReconciliation(t *testing.T) {
	monitor := NewMonitor(nil, DefaultReconcileConfig())

	monitor.RecordReconciliation(ReconcileResult{
		EntityID:  "user:1",
		Feature:   "clicks",
		OldValue:  10,
		NewValue:  15,
		Action:    "backfill_online",
		Success:   true,
		Timestamp: time.Now(),
	})

	history := monitor.GetHistory(10)
	if len(history) != 1 {
		t.Fatalf("expected 1 record, got %d", len(history))
	}
	if !history[0].Success {
		t.Fatal("expected success")
	}
}

func TestMonitor_Summary(t *testing.T) {
	monitor := NewMonitor(nil, DefaultReconcileConfig())

	monitor.RecordReconciliation(ReconcileResult{Success: true, Timestamp: time.Now()})
	monitor.RecordReconciliation(ReconcileResult{Success: false, Timestamp: time.Now()})

	summary := monitor.Summary()
	if summary["total_reconciled"] != 1 {
		t.Fatalf("expected 1 reconciled, got %v", summary["total_reconciled"])
	}
	if summary["total_failed"] != 1 {
		t.Fatalf("expected 1 failed, got %v", summary["total_failed"])
	}
}

func TestPercentile(t *testing.T) {
	data := []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	p50 := percentile(data, 50)
	if p50 < 5 || p50 > 6 {
		t.Fatalf("expected P50 ~5-6, got %f", p50)
	}
	p99 := percentile(data, 99)
	if p99 < 9 {
		t.Fatalf("expected P99 >= 9, got %f", p99)
	}
}

func TestFormatReport(t *testing.T) {
	report := &SkewReport{
		Feature:           "clicks",
		SampleSize:        100,
		ConsistencyRate:   99.5,
		MeanAbsDifference: 0.001,
		P99Difference:     0.01,
	}
	s := FormatReport(report)
	if s == "" {
		t.Fatal("empty report string")
	}
}
