package tenant

import (
	"testing"
)

func TestUsageMeterRecord(t *testing.T) {
	t.Parallel()
	meter := NewUsageMeter()
	_ = meter.Record("tenant-1", "requests", 100)
	_ = meter.Record("tenant-1", "requests", 50)
	_ = meter.Record("tenant-1", "storage_gb", 2.5)

	summary, err := meter.GetSummary("tenant-1")
	if err != nil {
		t.Fatal(err)
	}
	if summary.Metrics["requests"] != 150 {
		t.Errorf("expected 150 requests, got %f", summary.Metrics["requests"])
	}
	if summary.Metrics["storage_gb"] != 2.5 {
		t.Errorf("expected 2.5 GB, got %f", summary.Metrics["storage_gb"])
	}
}

func TestUsageMeterCostAttribution(t *testing.T) {
	t.Parallel()
	meter := NewUsageMeter()
	_ = meter.Record("tenant-1", "requests", 1000)
	_ = meter.Record("tenant-1", "storage_gb", 10)

	costs, err := meter.GetCostAttribution("tenant-1", DefaultCostConfig())
	if err != nil {
		t.Fatal(err)
	}
	if costs["requests"] != 0.1 { // 1000 * 0.0001
		t.Errorf("expected $0.10 for requests, got %f", costs["requests"])
	}
	if costs["storage_gb"] < 0.229 || costs["storage_gb"] > 0.231 { // 10 * 0.023
		t.Errorf("expected ~$0.23 for storage, got %f", costs["storage_gb"])
	}
}

func TestUsageMeterNotFound(t *testing.T) {
	t.Parallel()
	meter := NewUsageMeter()
	_, err := meter.GetSummary("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent tenant")
	}
}

func TestUsageMeterGetAllSummaries(t *testing.T) {
	t.Parallel()
	meter := NewUsageMeter()
	_ = meter.Record("t1", "requests", 10)
	_ = meter.Record("t2", "requests", 20)

	summaries := meter.GetAllSummaries()
	if len(summaries) != 2 {
		t.Errorf("expected 2 summaries, got %d", len(summaries))
	}
}

func TestUsageMeterReset(t *testing.T) {
	t.Parallel()
	meter := NewUsageMeter()
	_ = meter.Record("t1", "requests", 10)
	meter.Reset("t1")

	_, err := meter.GetSummary("t1")
	if err == nil {
		t.Error("expected error after reset")
	}
}

func TestUsageMeterQuotaEnforcement(t *testing.T) {
	t.Parallel()
	meter := NewUsageMeter()
	meter.SetQuota("t1", "requests", 100)

	if err := meter.Record("t1", "requests", 80); err != nil {
		t.Fatal("should allow 80 when limit is 100")
	}
	err := meter.Record("t1", "requests", 30)
	if err == nil {
		t.Fatal("should reject 30 more when 80/100 used")
	}
	qe, ok := err.(*QuotaExceeded)
	if !ok {
		t.Fatalf("expected QuotaExceeded, got %T", err)
	}
	if qe.Metric != "requests" {
		t.Errorf("expected requests metric, got %s", qe.Metric)
	}
}

func TestUsageMeterNegativeValue(t *testing.T) {
	t.Parallel()
	meter := NewUsageMeter()
	err := meter.Record("t1", "requests", -5)
	if err == nil {
		t.Error("expected error for negative value")
	}
}
