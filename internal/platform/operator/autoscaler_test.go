package operator

import (
	"testing"
	"time"
)

func TestDefaultScalingPolicy(t *testing.T) {
	p := DefaultScalingPolicy()
	if p.MinReplicas != 1 {
		t.Errorf("expected MinReplicas=1, got %d", p.MinReplicas)
	}
	if p.MaxReplicas != 10 {
		t.Errorf("expected MaxReplicas=10, got %d", p.MaxReplicas)
	}
	if p.TargetQPS != 1000 {
		t.Errorf("expected TargetQPS=1000, got %f", p.TargetQPS)
	}
}

func TestAutoscaler_EvaluateNoPolicy(t *testing.T) {
	a := NewAutoscaler()
	_, err := a.Evaluate("unknown", MetricsSnapshot{})
	if err == nil {
		t.Fatal("expected error for unknown policy")
	}
}

func TestAutoscaler_ScaleUpOnQPS(t *testing.T) {
	a := NewAutoscaler()
	policy := DefaultScalingPolicy()
	a.SetPolicy("store-1", policy)

	snapshot := MetricsSnapshot{
		CurrentQPS:      3000,
		P99LatencyMs:    10,
		CPUPercent:      40,
		MemoryPercent:   30,
		CurrentReplicas: 1,
		Timestamp:       time.Now(),
	}

	decision, err := a.Evaluate("store-1", snapshot)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if decision.DesiredReplicas <= decision.CurrentReplicas {
		t.Errorf("expected scale up, got desired=%d current=%d", decision.DesiredReplicas, decision.CurrentReplicas)
	}
}

func TestAutoscaler_ScaleUpOnLatency(t *testing.T) {
	a := NewAutoscaler()
	policy := DefaultScalingPolicy()
	a.SetPolicy("store-1", policy)

	snapshot := MetricsSnapshot{
		CurrentQPS:      500,
		P99LatencyMs:    200,
		CPUPercent:      40,
		CurrentReplicas: 2,
		Timestamp:       time.Now(),
	}

	decision, err := a.Evaluate("store-1", snapshot)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if decision.DesiredReplicas <= decision.CurrentReplicas {
		t.Errorf("expected scale up on latency, got desired=%d", decision.DesiredReplicas)
	}
}

func TestAutoscaler_RespectsMaxReplicas(t *testing.T) {
	a := NewAutoscaler()
	policy := DefaultScalingPolicy()
	policy.MaxReplicas = 3
	a.SetPolicy("store-1", policy)

	snapshot := MetricsSnapshot{
		CurrentQPS:      50000,
		P99LatencyMs:    500,
		CPUPercent:      95,
		CurrentReplicas: 2,
		Timestamp:       time.Now(),
	}

	decision, err := a.Evaluate("store-1", snapshot)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if decision.DesiredReplicas > 3 {
		t.Errorf("expected max 3 replicas, got %d", decision.DesiredReplicas)
	}
}

func TestAutoscaler_NoChangeWhenHealthy(t *testing.T) {
	a := NewAutoscaler()
	policy := DefaultScalingPolicy()
	a.SetPolicy("store-1", policy)

	snapshot := MetricsSnapshot{
		CurrentQPS:      500,
		P99LatencyMs:    10,
		CPUPercent:      30,
		MemoryPercent:   20,
		CurrentReplicas: 2,
		Timestamp:       time.Now(),
	}

	decision, err := a.Evaluate("store-1", snapshot)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if decision.DesiredReplicas != decision.CurrentReplicas {
		t.Errorf("expected no change, got desired=%d current=%d", decision.DesiredReplicas, decision.CurrentReplicas)
	}
}

func TestAutoscaler_History(t *testing.T) {
	a := NewAutoscaler()
	policy := DefaultScalingPolicy()
	policy.ScaleUpCooldown = 0
	a.SetPolicy("store-1", policy)

	for i := 0; i < 5; i++ {
		_, _ = a.Evaluate("store-1", MetricsSnapshot{
			CurrentQPS:      500,
			P99LatencyMs:    10,
			CurrentReplicas: 2,
			Timestamp:       time.Now(),
		})
	}

	history := a.GetHistory("store-1", 3)
	if len(history) != 3 {
		t.Errorf("expected 3 history entries, got %d", len(history))
	}
}

func TestAutoscaler_Stats(t *testing.T) {
	a := NewAutoscaler()
	policy := DefaultScalingPolicy()
	a.SetPolicy("store-1", policy)

	_, _ = a.Evaluate("store-1", MetricsSnapshot{
		CurrentQPS:      500,
		P99LatencyMs:    10,
		CurrentReplicas: 2,
		Timestamp:       time.Now(),
	})

	stats := a.Stats()
	if stats.TotalDecisions != 1 {
		t.Errorf("expected 1 total decision, got %d", stats.TotalDecisions)
	}
}
