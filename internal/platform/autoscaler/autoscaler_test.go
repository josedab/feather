package autoscaler

import (
	"testing"
	"time"
)

func TestNewAutoscaler(t *testing.T) {
	a := NewAutoscaler(DefaultConfig())
	if a.CurrentReplicas() != 1 {
		t.Errorf("expected 1 initial replica, got %d", a.CurrentReplicas())
	}
	if len(a.policies) == 0 {
		t.Fatal("expected default policies to be created")
	}
	if len(a.shardMap) == 0 {
		t.Fatal("expected initial shard map to be populated")
	}
}

func TestNewAutoscaler_InvalidConfig(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MinReplicas = 0
	cfg.MaxReplicas = -1
	cfg.ShardsPerReplica = 0

	a := NewAutoscaler(cfg)
	if a.CurrentReplicas() < 1 {
		t.Errorf("expected at least 1 replica, got %d", a.CurrentReplicas())
	}
}

func TestRecordMetric(t *testing.T) {
	a := NewAutoscaler(DefaultConfig())
	a.RecordMetric(MetricQPS, 500)
	a.RecordMetric(MetricQPS, 600)

	metrics := a.GetMetrics()
	found := false
	for _, m := range metrics {
		if m.Type == MetricQPS {
			if m.Value != 600 {
				t.Errorf("expected latest QPS 600, got %f", m.Value)
			}
			found = true
		}
	}
	if !found {
		t.Error("expected QPS metric in GetMetrics output")
	}
}

func TestRecordMetric_UnknownType(t *testing.T) {
	a := NewAutoscaler(DefaultConfig())
	a.RecordMetric(MetricShardBalance, 0.95)

	metrics := a.GetMetrics()
	found := false
	for _, m := range metrics {
		if m.Type == MetricShardBalance {
			found = true
		}
	}
	if !found {
		t.Error("expected shard balance metric to be recorded")
	}
}

func TestEvaluate_ScaleUp(t *testing.T) {
	cfg := DefaultConfig()
	cfg.ScaleUpCooldown = 0
	cfg.ScaleDownCooldown = 0
	a := NewAutoscaler(cfg)

	// Push metrics well above targets.
	for i := 0; i < 5; i++ {
		a.RecordMetric(MetricQPS, 3000)
		a.RecordMetric(MetricP99Latency, 30)
		a.RecordMetric(MetricCPUUsage, 90)
		a.RecordMetric(MetricMemoryUsage, 95)
		a.RecordMetric(MetricCacheHitRate, 0.4)
	}

	rec := a.Evaluate()
	if rec.Direction != ScaleUp {
		t.Errorf("expected scale up, got %s", rec.Direction)
	}
	if rec.DesiredReplicas <= 1 {
		t.Errorf("expected desired > 1, got %d", rec.DesiredReplicas)
	}
	if rec.Confidence == 0 {
		t.Error("expected non-zero confidence")
	}
}

func TestEvaluate_ScaleDown(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MinReplicas = 1
	cfg.MaxReplicas = 20
	cfg.ScaleUpCooldown = 0
	cfg.ScaleDownCooldown = 0
	a := NewAutoscaler(cfg)

	// Manually set replicas high.
	a.mu.Lock()
	a.currentReplicas = 10
	a.rebalanceShards()
	a.mu.Unlock()

	// Push metrics well below targets.
	for i := 0; i < 5; i++ {
		a.RecordMetric(MetricQPS, 100)
		a.RecordMetric(MetricP99Latency, 1)
		a.RecordMetric(MetricCPUUsage, 10)
		a.RecordMetric(MetricMemoryUsage, 10)
		a.RecordMetric(MetricCacheHitRate, 0.99)
	}

	rec := a.Evaluate()
	if rec.Direction != ScaleDown {
		t.Errorf("expected scale down, got %s", rec.Direction)
	}
	if rec.DesiredReplicas >= 10 {
		t.Errorf("expected desired < 10, got %d", rec.DesiredReplicas)
	}
}

func TestEvaluate_ScaleNone(t *testing.T) {
	cfg := DefaultConfig()
	cfg.ScaleUpCooldown = 0
	cfg.ScaleDownCooldown = 0
	a := NewAutoscaler(cfg)

	// Push metrics exactly at target (within tolerance).
	for i := 0; i < 5; i++ {
		a.RecordMetric(MetricQPS, 1000)
		a.RecordMetric(MetricP99Latency, 10)
		a.RecordMetric(MetricCPUUsage, 70)
		a.RecordMetric(MetricMemoryUsage, 80)
		a.RecordMetric(MetricCacheHitRate, 0.8)
	}

	rec := a.Evaluate()
	if rec.Direction != ScaleNone {
		t.Errorf("expected scale none, got %s (desired=%d)", rec.Direction, rec.DesiredReplicas)
	}
}

func TestEvaluate_NoMetrics(t *testing.T) {
	cfg := DefaultConfig()
	a := NewAutoscaler(cfg)

	rec := a.Evaluate()
	if rec.Direction != ScaleNone {
		t.Errorf("expected scale none with no data, got %s", rec.Direction)
	}
	if rec.Reason != "no metric data available" {
		t.Errorf("unexpected reason: %s", rec.Reason)
	}
}

func TestEvaluate_Cooldown(t *testing.T) {
	cfg := DefaultConfig()
	cfg.ScaleUpCooldown = 1 * time.Hour
	cfg.ScaleDownCooldown = 1 * time.Hour
	a := NewAutoscaler(cfg)

	// Simulate a recent scale-up.
	a.mu.Lock()
	a.lastScaleUp = time.Now()
	a.mu.Unlock()

	for i := 0; i < 5; i++ {
		a.RecordMetric(MetricQPS, 5000)
	}

	rec := a.Evaluate()
	if !rec.Cooldown {
		t.Error("expected cooldown to be active")
	}
	if rec.Direction != ScaleNone {
		t.Errorf("expected scale none during cooldown, got %s", rec.Direction)
	}
}

func TestApply(t *testing.T) {
	cfg := DefaultConfig()
	cfg.ScaleUpCooldown = 0
	cfg.ScaleDownCooldown = 0
	a := NewAutoscaler(cfg)

	rec := &ScaleRecommendation{
		Direction:       ScaleUp,
		CurrentReplicas: 1,
		DesiredReplicas: 3,
		Reason:          "test scale up",
	}

	if err := a.Apply(rec); err != nil {
		t.Fatalf("apply failed: %v", err)
	}

	if a.CurrentReplicas() != 3 {
		t.Errorf("expected 3 replicas, got %d", a.CurrentReplicas())
	}
}

func TestApply_NilRecommendation(t *testing.T) {
	a := NewAutoscaler(DefaultConfig())
	if err := a.Apply(nil); err == nil {
		t.Error("expected error for nil recommendation")
	}
}

func TestApply_ScaleNone(t *testing.T) {
	a := NewAutoscaler(DefaultConfig())
	rec := &ScaleRecommendation{Direction: ScaleNone}
	if err := a.Apply(rec); err != nil {
		t.Errorf("expected no error for ScaleNone, got: %v", err)
	}
	if a.CurrentReplicas() != 1 {
		t.Errorf("replicas should not change on ScaleNone")
	}
}

func TestApply_ClampsToMax(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxReplicas = 5
	a := NewAutoscaler(cfg)

	rec := &ScaleRecommendation{
		Direction:       ScaleUp,
		DesiredReplicas: 100,
		Reason:          "excessive",
	}
	if err := a.Apply(rec); err != nil {
		t.Fatalf("apply failed: %v", err)
	}
	if a.CurrentReplicas() != 5 {
		t.Errorf("expected clamped to 5, got %d", a.CurrentReplicas())
	}
}

func TestShardRebalancing(t *testing.T) {
	cfg := DefaultConfig()
	cfg.ShardsPerReplica = 4
	a := NewAutoscaler(cfg)

	// Initial: 1 replica, 4 shards.
	assignments := a.GetShardAssignments()
	if len(assignments) != 4 {
		t.Fatalf("expected 4 shards, got %d", len(assignments))
	}

	// Scale to 2 replicas.
	rec := &ScaleRecommendation{
		Direction:       ScaleUp,
		DesiredReplicas: 2,
		Reason:          "test",
	}
	if err := a.Apply(rec); err != nil {
		t.Fatalf("apply failed: %v", err)
	}

	assignments = a.GetShardAssignments()
	if len(assignments) != 8 {
		t.Fatalf("expected 8 shards after scale to 2 replicas, got %d", len(assignments))
	}

	// Verify even distribution.
	counts := map[int]int{}
	for _, sa := range assignments {
		counts[sa.ReplicaID]++
	}
	for replica, count := range counts {
		if count != 4 {
			t.Errorf("replica %d has %d shards, expected 4", replica, count)
		}
	}
}

func TestRebalanceShards(t *testing.T) {
	cfg := DefaultConfig()
	cfg.ShardsPerReplica = 2
	a := NewAutoscaler(cfg)

	a.mu.Lock()
	a.currentReplicas = 3
	a.mu.Unlock()

	assignments := a.RebalanceShards()
	if len(assignments) != 6 {
		t.Fatalf("expected 6 shards, got %d", len(assignments))
	}
}

func TestGetScaleHistory(t *testing.T) {
	cfg := DefaultConfig()
	a := NewAutoscaler(cfg)

	for i := 2; i <= 5; i++ {
		rec := &ScaleRecommendation{
			Direction:       ScaleUp,
			DesiredReplicas: i,
			Reason:          "growth",
		}
		_ = a.Apply(rec)
	}

	history := a.GetScaleHistory(2)
	if len(history) != 2 {
		t.Fatalf("expected 2 events, got %d", len(history))
	}
	// Most recent events.
	if history[1].ToReplicas != 5 {
		t.Errorf("expected last event to scale to 5, got %d", history[1].ToReplicas)
	}

	all := a.GetScaleHistory(0)
	if len(all) != 4 {
		t.Errorf("expected 4 total events, got %d", len(all))
	}
}

func TestStats(t *testing.T) {
	cfg := DefaultConfig()
	cfg.ShardsPerReplica = 4
	a := NewAutoscaler(cfg)

	a.RecordMetric(MetricQPS, 500)

	stats := a.Stats()
	if stats.CurrentReplicas != 1 {
		t.Errorf("expected 1 replica, got %d", stats.CurrentReplicas)
	}
	if stats.MinReplicas != cfg.MinReplicas {
		t.Errorf("expected min %d, got %d", cfg.MinReplicas, stats.MinReplicas)
	}
	if stats.MaxReplicas != cfg.MaxReplicas {
		t.Errorf("expected max %d, got %d", cfg.MaxReplicas, stats.MaxReplicas)
	}
	if stats.TotalShards != 4 {
		t.Errorf("expected 4 shards, got %d", stats.TotalShards)
	}
	if stats.PolicyCount == 0 {
		t.Error("expected non-zero policy count")
	}
	if _, ok := stats.MetricSummary[string(MetricQPS)]; !ok {
		t.Error("expected QPS in metric summary")
	}
	if stats.LastScaleUp != nil {
		t.Error("expected nil LastScaleUp before any scaling")
	}
}

func TestStats_AfterScaling(t *testing.T) {
	cfg := DefaultConfig()
	a := NewAutoscaler(cfg)

	rec := &ScaleRecommendation{
		Direction:       ScaleUp,
		DesiredReplicas: 3,
		Reason:          "test",
	}
	_ = a.Apply(rec)

	stats := a.Stats()
	if stats.ScaleEvents != 1 {
		t.Errorf("expected 1 scale event, got %d", stats.ScaleEvents)
	}
	if stats.LastScaleUp == nil {
		t.Error("expected LastScaleUp to be set")
	}
}
