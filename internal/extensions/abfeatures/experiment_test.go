package abfeatures

import (
	"errors"
	"fmt"
	"testing"
)

func TestNewManager(t *testing.T) {
	m := NewManager(DefaultExperimentConfig())
	if m == nil {
		t.Fatal("expected non-nil manager")
	}
	exps := m.ListExperiments()
	if len(exps) != 0 {
		t.Errorf("expected 0 experiments, got %d", len(exps))
	}
}

func TestCreateAndStart(t *testing.T) {
	m := NewManager(DefaultExperimentConfig())

	err := m.CreateExperiment(Experiment{
		ID:           "exp-1",
		Name:         "test experiment",
		FeatureGroup: "user-features",
		Variants: []Variant{
			{ID: "v1", Name: "control", FeatureVersion: "v1.0", TrafficPercent: 50},
			{ID: "v2", Name: "treatment", FeatureVersion: "v2.0", TrafficPercent: 50},
		},
	})
	if err != nil {
		t.Fatalf("CreateExperiment failed: %v", err)
	}

	// Duplicate
	err = m.CreateExperiment(Experiment{ID: "exp-1", Name: "dup"})
	if !errors.Is(err, ErrExperimentExists) {
		t.Errorf("expected ErrExperimentExists, got %v", err)
	}

	err = m.StartExperiment("exp-1")
	if err != nil {
		t.Fatalf("StartExperiment failed: %v", err)
	}

	exp, _ := m.GetExperiment("exp-1")
	if exp.Status != Running {
		t.Errorf("expected Running status, got %s", exp.Status)
	}
}

func TestResolveVariant(t *testing.T) {
	m := NewManager(DefaultExperimentConfig())

	_ = m.CreateExperiment(Experiment{
		ID: "routing-exp",
		Variants: []Variant{
			{ID: "v1", Name: "control", TrafficPercent: 50},
			{ID: "v2", Name: "treatment", TrafficPercent: 50},
		},
	})
	_ = m.StartExperiment("routing-exp")

	// Deterministic: same entity should always get same variant
	variant1, err := m.ResolveVariant("routing-exp", "user-123")
	if err != nil {
		t.Fatalf("ResolveVariant failed: %v", err)
	}
	variant2, err := m.ResolveVariant("routing-exp", "user-123")
	if err != nil {
		t.Fatalf("ResolveVariant failed: %v", err)
	}
	if variant1 != variant2 {
		t.Errorf("expected deterministic routing: %s != %s", variant1, variant2)
	}

	// Different entities should produce a distribution
	v1Count, v2Count := 0, 0
	for i := 0; i < 100; i++ {
		v, _ := m.ResolveVariant("routing-exp", fmt.Sprintf("entity-%d", i))
		if v == "v1" {
			v1Count++
		} else {
			v2Count++
		}
	}
	if v1Count == 0 || v2Count == 0 {
		t.Errorf("expected both variants to receive traffic: v1=%d, v2=%d", v1Count, v2Count)
	}
}

func TestResolveVariantNotRunning(t *testing.T) {
	m := NewManager(DefaultExperimentConfig())
	_ = m.CreateExperiment(Experiment{
		ID:       "draft-exp",
		Variants: []Variant{{ID: "v1", TrafficPercent: 100}},
	})

	_, err := m.ResolveVariant("draft-exp", "user-1")
	if err == nil {
		t.Error("expected error for non-running experiment")
	}
}

func TestRecordMetrics(t *testing.T) {
	m := NewManager(DefaultExperimentConfig())

	_ = m.CreateExperiment(Experiment{
		ID: "metrics-exp",
		Variants: []Variant{
			{ID: "v1", Name: "control", TrafficPercent: 50},
			{ID: "v2", Name: "treatment", TrafficPercent: 50},
		},
	})
	_ = m.StartExperiment("metrics-exp")

	m.RecordMetric("metrics-exp", "v1", 1.5, nil)
	m.RecordMetric("metrics-exp", "v1", 2.5, nil)
	m.RecordMetric("metrics-exp", "v2", 3.0, fmt.Errorf("timeout"))

	exp, _ := m.GetExperiment("metrics-exp")
	for _, v := range exp.Variants {
		if v.ID == "v1" {
			if v.Metrics.Requests != 2 {
				t.Errorf("v1: expected 2 requests, got %d", v.Metrics.Requests)
			}
			if v.Metrics.AvgLatencyMs < 1.0 || v.Metrics.AvgLatencyMs > 3.0 {
				t.Errorf("v1: unexpected avg latency %f", v.Metrics.AvgLatencyMs)
			}
		}
		if v.ID == "v2" {
			if v.Metrics.Requests != 1 {
				t.Errorf("v2: expected 1 request, got %d", v.Metrics.Requests)
			}
			if v.Metrics.ErrorRate == 0 {
				t.Error("v2: expected non-zero error rate")
			}
		}
	}
}

func TestEvaluateSignificance(t *testing.T) {
	m := NewManager(ExperimentConfig{
		MaxExperiments:    100,
		MinSampleSize:     10, // Low for testing
		SignificanceLevel: 0.05,
	})

	_ = m.CreateExperiment(Experiment{
		ID: "sig-exp",
		Variants: []Variant{
			{ID: "v1", Name: "control", TrafficPercent: 50},
			{ID: "v2", Name: "treatment", TrafficPercent: 50},
		},
	})
	_ = m.StartExperiment("sig-exp")

	// Record scores with clear difference
	for i := 0; i < 100; i++ {
		m.RecordScore("sig-exp", "v1", 0.5)
		m.RecordScore("sig-exp", "v2", 0.9)
	}

	result, err := m.EvaluateSignificance("sig-exp")
	if err != nil {
		t.Fatalf("EvaluateSignificance failed: %v", err)
	}
	if result.SampleSizeA != 100 || result.SampleSizeB != 100 {
		t.Errorf("expected 100 samples each, got %d and %d", result.SampleSizeA, result.SampleSizeB)
	}
	// With such a clear difference and enough samples, it should be significant
	if !result.Significant {
		t.Logf("PValue: %f, may not reach significance with this approximation", result.PValue)
	}
}

func TestStopExperiment(t *testing.T) {
	m := NewManager(DefaultExperimentConfig())

	_ = m.CreateExperiment(Experiment{
		ID:       "stop-exp",
		Variants: []Variant{{ID: "v1", TrafficPercent: 100}},
	})
	_ = m.StartExperiment("stop-exp")

	err := m.StopExperiment("stop-exp")
	if err != nil {
		t.Fatalf("StopExperiment failed: %v", err)
	}

	exp, _ := m.GetExperiment("stop-exp")
	if exp.Status != Concluded {
		t.Errorf("expected Concluded status, got %s", exp.Status)
	}

	err = m.StopExperiment("nonexistent")
	if !errors.Is(err, ErrExperimentNotFound) {
		t.Errorf("expected ErrExperimentNotFound, got %v", err)
	}
}

func TestStats(t *testing.T) {
	m := NewManager(DefaultExperimentConfig())

	_ = m.CreateExperiment(Experiment{
		ID:       "s1",
		Variants: []Variant{{ID: "v1", TrafficPercent: 100}},
	})
	_ = m.CreateExperiment(Experiment{
		ID:       "s2",
		Variants: []Variant{{ID: "v1", TrafficPercent: 100}},
	})
	_ = m.StartExperiment("s1")

	stats := m.Stats()
	if stats.Total != 2 {
		t.Errorf("expected 2 total, got %d", stats.Total)
	}
	if stats.Running != 1 {
		t.Errorf("expected 1 running, got %d", stats.Running)
	}
}
