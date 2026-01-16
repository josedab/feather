package federation

import (
	"math"
	"testing"
)

func TestFederatedTrainer_StartRound(t *testing.T) {
	trainer := NewFederatedTrainer(DefaultTrainingConfig())
	trainer.RegisterClient("client-a")
	trainer.RegisterClient("client-b")

	round, err := trainer.StartRound()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if round.RoundNumber != 1 {
		t.Errorf("expected round 1, got %d", round.RoundNumber)
	}
	if round.Status != RoundStatusInProgress {
		t.Errorf("expected in_progress, got %s", round.Status)
	}
	if len(round.ParticipantsSelected) != 2 {
		t.Errorf("expected 2 participants, got %d", len(round.ParticipantsSelected))
	}
}

func TestFederatedTrainer_StartRound_NotEnoughClients(t *testing.T) {
	trainer := NewFederatedTrainer(DefaultTrainingConfig())
	trainer.RegisterClient("client-a")

	_, err := trainer.StartRound()
	if err == nil {
		t.Fatal("expected error for insufficient clients")
	}
}

func TestFederatedTrainer_RegisterClient_Duplicate(t *testing.T) {
	trainer := NewFederatedTrainer(DefaultTrainingConfig())
	trainer.RegisterClient("client-a")
	trainer.RegisterClient("client-a")

	stats := trainer.Stats()
	if stats.TotalClients != 1 {
		t.Errorf("expected 1 unique client, got %d", stats.TotalClients)
	}
}

func TestFederatedTrainer_ReportUpdate(t *testing.T) {
	trainer := NewFederatedTrainer(DefaultTrainingConfig())
	trainer.RegisterClient("client-a")
	trainer.RegisterClient("client-b")
	round, _ := trainer.StartRound()

	err := trainer.ReportUpdate(round.RoundNumber, "client-a",
		[]float64{1.0, 2.0},
		map[string]float64{"accuracy": 0.9})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	r, _ := trainer.GetRound(round.RoundNumber)
	if len(r.ParticipantsReported) != 1 {
		t.Errorf("expected 1 reported, got %d", len(r.ParticipantsReported))
	}
}

func TestFederatedTrainer_ReportUpdate_InvalidRound(t *testing.T) {
	trainer := NewFederatedTrainer(DefaultTrainingConfig())

	err := trainer.ReportUpdate(999, "client-a", []float64{1.0}, nil)
	if err == nil {
		t.Fatal("expected error for invalid round")
	}
}

func TestFederatedTrainer_ReportUpdate_InvalidClient(t *testing.T) {
	trainer := NewFederatedTrainer(DefaultTrainingConfig())
	trainer.RegisterClient("client-a")
	trainer.RegisterClient("client-b")
	round, _ := trainer.StartRound()

	err := trainer.ReportUpdate(round.RoundNumber, "unknown", []float64{1.0}, nil)
	if err == nil {
		t.Fatal("expected error for unselected client")
	}
}

func TestFederatedTrainer_ReportUpdate_Duplicate(t *testing.T) {
	trainer := NewFederatedTrainer(DefaultTrainingConfig())
	trainer.RegisterClient("client-a")
	trainer.RegisterClient("client-b")
	round, _ := trainer.StartRound()

	_ = trainer.ReportUpdate(round.RoundNumber, "client-a", []float64{1.0}, nil)
	err := trainer.ReportUpdate(round.RoundNumber, "client-a", []float64{2.0}, nil)
	if err == nil {
		t.Fatal("expected error for duplicate report")
	}
}

func TestFederatedTrainer_AggregateRound(t *testing.T) {
	trainer := NewFederatedTrainer(DefaultTrainingConfig())
	trainer.RegisterClient("client-a")
	trainer.RegisterClient("client-b")
	round, _ := trainer.StartRound()

	_ = trainer.ReportUpdate(round.RoundNumber, "client-a",
		[]float64{1.0, 3.0},
		map[string]float64{"accuracy": 0.8})
	_ = trainer.ReportUpdate(round.RoundNumber, "client-b",
		[]float64{3.0, 5.0},
		map[string]float64{"accuracy": 0.9})

	result, err := trainer.AggregateRound(round.RoundNumber)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if result.ParticipantCount != 2 {
		t.Errorf("expected 2 participants, got %d", result.ParticipantCount)
	}

	// FedAvg: (1+3)/2=2, (3+5)/2=4
	if math.Abs(result.Weights[0]-2.0) > 1e-10 {
		t.Errorf("expected weight[0] = 2.0, got %f", result.Weights[0])
	}
	if math.Abs(result.Weights[1]-4.0) > 1e-10 {
		t.Errorf("expected weight[1] = 4.0, got %f", result.Weights[1])
	}

	// Average accuracy: (0.8+0.9)/2 = 0.85
	if math.Abs(result.Metrics["accuracy"]-0.85) > 1e-10 {
		t.Errorf("expected accuracy 0.85, got %f", result.Metrics["accuracy"])
	}

	// Round should be completed
	r, _ := trainer.GetRound(round.RoundNumber)
	if r.Status != RoundStatusCompleted {
		t.Errorf("expected completed, got %s", r.Status)
	}
}

func TestFederatedTrainer_AggregateRound_NotEnough(t *testing.T) {
	trainer := NewFederatedTrainer(DefaultTrainingConfig())
	trainer.RegisterClient("client-a")
	trainer.RegisterClient("client-b")
	round, _ := trainer.StartRound()

	_ = trainer.ReportUpdate(round.RoundNumber, "client-a", []float64{1.0}, nil)

	_, err := trainer.AggregateRound(round.RoundNumber)
	if err == nil {
		t.Fatal("expected error for insufficient updates")
	}
}

func TestFederatedTrainer_AggregateRound_AlreadyCompleted(t *testing.T) {
	trainer := NewFederatedTrainer(DefaultTrainingConfig())
	trainer.RegisterClient("client-a")
	trainer.RegisterClient("client-b")
	round, _ := trainer.StartRound()

	_ = trainer.ReportUpdate(round.RoundNumber, "client-a", []float64{1.0}, nil)
	_ = trainer.ReportUpdate(round.RoundNumber, "client-b", []float64{2.0}, nil)
	_, _ = trainer.AggregateRound(round.RoundNumber)

	_, err := trainer.AggregateRound(round.RoundNumber)
	if err == nil {
		t.Fatal("expected error for already aggregated round")
	}
}

func TestFederatedTrainer_ListRounds(t *testing.T) {
	trainer := NewFederatedTrainer(DefaultTrainingConfig())
	trainer.RegisterClient("client-a")
	trainer.RegisterClient("client-b")

	_, _ = trainer.StartRound()
	_, _ = trainer.StartRound()

	rounds := trainer.ListRounds()
	if len(rounds) != 2 {
		t.Errorf("expected 2 rounds, got %d", len(rounds))
	}
}

func TestFederatedTrainer_GetRound_NotFound(t *testing.T) {
	trainer := NewFederatedTrainer(DefaultTrainingConfig())

	_, err := trainer.GetRound(999)
	if err == nil {
		t.Fatal("expected error for nonexistent round")
	}
}

func TestFederatedTrainer_Stats(t *testing.T) {
	trainer := NewFederatedTrainer(DefaultTrainingConfig())
	trainer.RegisterClient("client-a")
	trainer.RegisterClient("client-b")

	round, _ := trainer.StartRound()
	_ = trainer.ReportUpdate(round.RoundNumber, "client-a",
		[]float64{1.0}, map[string]float64{"accuracy": 0.95})
	_ = trainer.ReportUpdate(round.RoundNumber, "client-b",
		[]float64{2.0}, map[string]float64{"accuracy": 0.85})
	_, _ = trainer.AggregateRound(round.RoundNumber)

	stats := trainer.Stats()
	if stats.TotalRounds != 1 {
		t.Errorf("expected 1 total round, got %d", stats.TotalRounds)
	}
	if stats.CompletedRounds != 1 {
		t.Errorf("expected 1 completed round, got %d", stats.CompletedRounds)
	}
	if stats.TotalClients != 2 {
		t.Errorf("expected 2 clients, got %d", stats.TotalClients)
	}
	if math.Abs(stats.AvgAccuracy-0.9) > 1e-10 {
		t.Errorf("expected avg accuracy 0.9, got %f", stats.AvgAccuracy)
	}
}

func TestFederatedTrainer_Stats_Empty(t *testing.T) {
	trainer := NewFederatedTrainer(DefaultTrainingConfig())

	stats := trainer.Stats()
	if stats.TotalRounds != 0 {
		t.Errorf("expected 0 rounds, got %d", stats.TotalRounds)
	}
	if stats.AvgAccuracy != 0 {
		t.Errorf("expected 0 accuracy, got %f", stats.AvgAccuracy)
	}
}

func TestDefaultTrainingConfig(t *testing.T) {
	config := DefaultTrainingConfig()

	if config.Rounds != 10 {
		t.Errorf("expected 10 rounds, got %d", config.Rounds)
	}
	if config.MinClients != 2 {
		t.Errorf("expected 2 min clients, got %d", config.MinClients)
	}
	if config.FractionFit != 1.0 {
		t.Errorf("expected fraction_fit 1.0, got %f", config.FractionFit)
	}
	if config.AggStrategy != "fedavg" {
		t.Errorf("expected fedavg, got %s", config.AggStrategy)
	}
}

func TestFederatedTrainer_MultipleRounds(t *testing.T) {
	trainer := NewFederatedTrainer(DefaultTrainingConfig())
	trainer.RegisterClient("client-a")
	trainer.RegisterClient("client-b")

	// Round 1
	r1, _ := trainer.StartRound()
	_ = trainer.ReportUpdate(r1.RoundNumber, "client-a", []float64{1.0}, nil)
	_ = trainer.ReportUpdate(r1.RoundNumber, "client-b", []float64{2.0}, nil)
	_, _ = trainer.AggregateRound(r1.RoundNumber)

	// Round 2
	r2, _ := trainer.StartRound()
	if r2.RoundNumber != 2 {
		t.Errorf("expected round 2, got %d", r2.RoundNumber)
	}

	_ = trainer.ReportUpdate(r2.RoundNumber, "client-a", []float64{3.0}, nil)
	_ = trainer.ReportUpdate(r2.RoundNumber, "client-b", []float64{4.0}, nil)
	_, _ = trainer.AggregateRound(r2.RoundNumber)

	stats := trainer.Stats()
	if stats.CompletedRounds != 2 {
		t.Errorf("expected 2 completed rounds, got %d", stats.CompletedRounds)
	}
}
