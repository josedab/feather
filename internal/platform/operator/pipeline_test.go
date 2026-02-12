package operator

import (
	"context"
	"testing"
)

func TestNewAutoScaler(t *testing.T) {
	policy := DefaultAutoScalePolicy()
	scaler := NewAutoScaler(policy)
	if scaler.CurrentReplicas() != policy.MinReplicas {
		t.Errorf("expected %d replicas, got %d", policy.MinReplicas, scaler.CurrentReplicas())
	}
}

func TestAutoScalerScaleUp(t *testing.T) {
	policy := DefaultAutoScalePolicy()
	policy.MinReplicas = 1
	policy.MaxReplicas = 5
	scaler := NewAutoScaler(policy)

	decision := scaler.Evaluate(context.Background(), map[string]float64{
		"cpu_percent": 90.0,
	})
	if decision == nil {
		t.Fatal("expected scale-up decision")
	}
	if decision.DesiredReplicas != 2 {
		t.Errorf("expected 2 replicas, got %d", decision.DesiredReplicas)
	}
	if scaler.CurrentReplicas() != 2 {
		t.Errorf("expected current to be 2, got %d", scaler.CurrentReplicas())
	}
}

func TestAutoScalerScaleDown(t *testing.T) {
	policy := DefaultAutoScalePolicy()
	policy.MinReplicas = 1
	policy.MaxReplicas = 5
	policy.TargetCPUPercent = 70
	scaler := NewAutoScaler(policy)

	// Scale up first.
	scaler.Evaluate(context.Background(), map[string]float64{"cpu_percent": 90.0})
	scaler.Evaluate(context.Background(), map[string]float64{"cpu_percent": 90.0})

	// Now scale down.
	decision := scaler.Evaluate(context.Background(), map[string]float64{
		"cpu_percent": 20.0, // well below 70*0.5 = 35
	})
	if decision == nil {
		t.Fatal("expected scale-down decision")
	}
	if decision.DesiredReplicas >= 3 {
		t.Errorf("expected fewer replicas, got %d", decision.DesiredReplicas)
	}
}

func TestAutoScalerMaxReplicas(t *testing.T) {
	policy := DefaultAutoScalePolicy()
	policy.MinReplicas = 1
	policy.MaxReplicas = 2
	scaler := NewAutoScaler(policy)

	scaler.Evaluate(context.Background(), map[string]float64{"cpu_percent": 90.0})
	decision := scaler.Evaluate(context.Background(), map[string]float64{"cpu_percent": 90.0})
	if decision != nil && decision.DesiredReplicas > 2 {
		t.Errorf("should not exceed max replicas, got %d", decision.DesiredReplicas)
	}
}

func TestAutoScalerLatency(t *testing.T) {
	policy := DefaultAutoScalePolicy()
	policy.TargetLatencyP99Ms = 10
	scaler := NewAutoScaler(policy)

	decision := scaler.Evaluate(context.Background(), map[string]float64{
		"latency_p99_ms": 50.0,
	})
	if decision == nil {
		t.Fatal("expected scale-up on high latency")
	}
	if decision.Metric != "latency_p99_ms" {
		t.Errorf("expected latency metric, got %q", decision.Metric)
	}
}

func TestAutoScalerDisabled(t *testing.T) {
	policy := DefaultAutoScalePolicy()
	policy.Enabled = false
	scaler := NewAutoScaler(policy)

	decision := scaler.Evaluate(context.Background(), map[string]float64{"cpu_percent": 99.0})
	if decision != nil {
		t.Error("expected nil when autoscaler is disabled")
	}
}

func TestAutoScalerHistory(t *testing.T) {
	policy := DefaultAutoScalePolicy()
	scaler := NewAutoScaler(policy)

	scaler.Evaluate(context.Background(), map[string]float64{"cpu_percent": 90.0})
	history := scaler.History()
	if len(history) != 1 {
		t.Errorf("expected 1 history entry, got %d", len(history))
	}
}

func TestPipelineControllerCRUD(t *testing.T) {
	ctrl := NewPipelineController()

	pipeline := &FeaturePipeline{
		ObjectMeta: ObjectMeta{Namespace: "default", Name: "user-clicks"},
		Spec: FeaturePipelineSpec{
			Source: PipelineSource{Type: "kafka", Config: map[string]string{"topic": "clicks"}},
			Sink:   PipelineSink{FeatureGroup: "user_features"},
		},
	}

	if err := ctrl.Create(pipeline); err != nil {
		t.Fatalf("create: %v", err)
	}

	// Duplicate create fails.
	if err := ctrl.Create(pipeline); err == nil {
		t.Error("expected error on duplicate create")
	}

	// Get
	got, err := ctrl.Get("default", "user-clicks")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Status.Phase != "Pending" {
		t.Errorf("expected Pending phase, got %q", got.Status.Phase)
	}

	// List
	list := ctrl.List("default")
	if len(list) != 1 {
		t.Errorf("expected 1 pipeline, got %d", len(list))
	}

	// Update status
	if err := ctrl.UpdateStatus("default", "user-clicks", FeaturePipelineStatus{
		Phase:    "Running",
		RunCount: 5,
	}); err != nil {
		t.Fatalf("update status: %v", err)
	}

	got, _ = ctrl.Get("default", "user-clicks")
	if got.Status.Phase != "Running" {
		t.Errorf("expected Running, got %q", got.Status.Phase)
	}

	// Delete
	if err := ctrl.Delete("default", "user-clicks"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := ctrl.Get("default", "user-clicks"); err == nil {
		t.Error("expected error after delete")
	}
}

func TestPipelineControllerValidation(t *testing.T) {
	ctrl := NewPipelineController()

	// Missing source type
	err := ctrl.Create(&FeaturePipeline{
		ObjectMeta: ObjectMeta{Namespace: "ns", Name: "bad"},
		Spec: FeaturePipelineSpec{
			Source: PipelineSource{},
			Sink:   PipelineSink{FeatureGroup: "g"},
		},
	})
	if err == nil {
		t.Error("expected error for missing source type")
	}

	// Missing sink
	err = ctrl.Create(&FeaturePipeline{
		ObjectMeta: ObjectMeta{Namespace: "ns", Name: "bad2"},
		Spec: FeaturePipelineSpec{
			Source: PipelineSource{Type: "kafka"},
			Sink:   PipelineSink{},
		},
	})
	if err == nil {
		t.Error("expected error for missing sink feature group")
	}
}
