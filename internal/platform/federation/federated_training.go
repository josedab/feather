package federation

import (
	"fmt"
	"sync"
	"time"
)

// TrainingConfig configures federated training.
type TrainingConfig struct {
	Rounds      int     `json:"rounds"`
	MinClients  int     `json:"min_clients"`
	FractionFit float64 `json:"fraction_fit"`
	AggStrategy string  `json:"agg_strategy"` // "fedavg" or "fedprox"
}

// DefaultTrainingConfig returns default federated training configuration.
func DefaultTrainingConfig() TrainingConfig {
	return TrainingConfig{
		Rounds:      10,
		MinClients:  2,
		FractionFit: 1.0,
		AggStrategy: "fedavg",
	}
}

// TrainingRoundStatus represents the status of a training round.
type TrainingRoundStatus string

// TrainingRoundStatus constants.
const (
	RoundStatusPending    TrainingRoundStatus = "pending"
	RoundStatusInProgress TrainingRoundStatus = "in_progress"
	RoundStatusCompleted  TrainingRoundStatus = "completed"
	RoundStatusFailed     TrainingRoundStatus = "failed"
)

// TrainingRound represents a single round of federated training.
type TrainingRound struct {
	RoundNumber          int                 `json:"round_number"`
	Status               TrainingRoundStatus `json:"status"`
	ParticipantsSelected []string            `json:"participants_selected"`
	ParticipantsReported []string            `json:"participants_reported"`
	StartedAt            time.Time           `json:"started_at"`
	CompletedAt          time.Time           `json:"completed_at,omitempty"`
	Metrics              map[string]float64  `json:"metrics,omitempty"`

	updates map[string]*clientUpdate
}

type clientUpdate struct {
	weights []float64
	metrics map[string]float64
}

// AggregatedUpdate holds the result of aggregating a training round.
type AggregatedUpdate struct {
	Weights          []float64          `json:"weights"`
	Metrics          map[string]float64 `json:"metrics"`
	ParticipantCount int                `json:"participant_count"`
}

// TrainingStats provides statistics about the training process.
type TrainingStats struct {
	TotalRounds     int     `json:"total_rounds"`
	CompletedRounds int     `json:"completed_rounds"`
	TotalClients    int     `json:"total_clients"`
	AvgAccuracy     float64 `json:"avg_accuracy"`
}

// FederatedTrainer coordinates federated training rounds.
type FederatedTrainer struct {
	mu        sync.RWMutex
	config    TrainingConfig
	rounds    map[int]*TrainingRound
	clients   []string
	nextRound int
}

// NewFederatedTrainer creates a new federated trainer.
func NewFederatedTrainer(config TrainingConfig) *FederatedTrainer {
	return &FederatedTrainer{
		config:    config,
		rounds:    make(map[int]*TrainingRound),
		clients:   make([]string, 0),
		nextRound: 1,
	}
}

// RegisterClient adds a client to the pool of available training clients.
func (ft *FederatedTrainer) RegisterClient(clientID string) {
	ft.mu.Lock()
	defer ft.mu.Unlock()

	for _, c := range ft.clients {
		if c == clientID {
			return
		}
	}
	ft.clients = append(ft.clients, clientID)
}

// StartRound begins a new federated training round.
func (ft *FederatedTrainer) StartRound() (*TrainingRound, error) {
	ft.mu.Lock()
	defer ft.mu.Unlock()

	if len(ft.clients) < ft.config.MinClients {
		return nil, fmt.Errorf("not enough clients: have %d, need %d", len(ft.clients), ft.config.MinClients)
	}

	// Select clients based on FractionFit
	numSelected := int(float64(len(ft.clients)) * ft.config.FractionFit)
	if numSelected < ft.config.MinClients {
		numSelected = ft.config.MinClients
	}
	if numSelected > len(ft.clients) {
		numSelected = len(ft.clients)
	}

	selected := make([]string, numSelected)
	copy(selected, ft.clients[:numSelected])

	round := &TrainingRound{
		RoundNumber:          ft.nextRound,
		Status:               RoundStatusInProgress,
		ParticipantsSelected: selected,
		ParticipantsReported: make([]string, 0),
		StartedAt:            time.Now(),
		Metrics:              make(map[string]float64),
		updates:              make(map[string]*clientUpdate),
	}

	ft.rounds[ft.nextRound] = round
	ft.nextRound++

	return round, nil
}

// ReportUpdate records a client's model update for a training round.
func (ft *FederatedTrainer) ReportUpdate(roundNumber int, clientID string, weights []float64, metrics map[string]float64) error {
	ft.mu.Lock()
	defer ft.mu.Unlock()

	round, exists := ft.rounds[roundNumber]
	if !exists {
		return fmt.Errorf("round %d not found", roundNumber)
	}

	if round.Status != RoundStatusInProgress {
		return fmt.Errorf("round %d is not in progress (status: %s)", roundNumber, round.Status)
	}

	// Verify client was selected
	found := false
	for _, p := range round.ParticipantsSelected {
		if p == clientID {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("client %s was not selected for round %d", clientID, roundNumber)
	}

	// Check for duplicate
	if _, submitted := round.updates[clientID]; submitted {
		return fmt.Errorf("client %s already reported for round %d", clientID, roundNumber)
	}

	round.updates[clientID] = &clientUpdate{
		weights: weights,
		metrics: metrics,
	}
	round.ParticipantsReported = append(round.ParticipantsReported, clientID)

	return nil
}

// AggregateRound aggregates all client updates for a round using the configured strategy.
func (ft *FederatedTrainer) AggregateRound(roundNumber int) (*AggregatedUpdate, error) {
	ft.mu.Lock()
	defer ft.mu.Unlock()

	round, exists := ft.rounds[roundNumber]
	if !exists {
		return nil, fmt.Errorf("round %d not found", roundNumber)
	}

	if round.Status == RoundStatusCompleted {
		return nil, fmt.Errorf("round %d already aggregated", roundNumber)
	}

	if len(round.updates) < ft.config.MinClients {
		return nil, fmt.Errorf("round %d needs at least %d updates, has %d",
			roundNumber, ft.config.MinClients, len(round.updates))
	}

	// Determine weight vector length
	var vectorLen int
	for _, u := range round.updates {
		vectorLen = len(u.weights)
		break
	}

	// FedAvg: simple average of weights
	aggregatedWeights := make([]float64, vectorLen)
	aggregatedMetrics := make(map[string]float64)
	count := 0

	for _, u := range round.updates {
		for i := 0; i < vectorLen && i < len(u.weights); i++ {
			aggregatedWeights[i] += u.weights[i]
		}
		for k, v := range u.metrics {
			aggregatedMetrics[k] += v
		}
		count++
	}

	for i := range aggregatedWeights {
		aggregatedWeights[i] /= float64(count)
	}
	for k := range aggregatedMetrics {
		aggregatedMetrics[k] /= float64(count)
	}

	round.Status = RoundStatusCompleted
	round.CompletedAt = time.Now()
	round.Metrics = aggregatedMetrics

	return &AggregatedUpdate{
		Weights:          aggregatedWeights,
		Metrics:          aggregatedMetrics,
		ParticipantCount: count,
	}, nil
}

// GetRound returns a training round by number.
func (ft *FederatedTrainer) GetRound(roundNumber int) (*TrainingRound, error) {
	ft.mu.RLock()
	defer ft.mu.RUnlock()

	round, exists := ft.rounds[roundNumber]
	if !exists {
		return nil, fmt.Errorf("round %d not found", roundNumber)
	}

	return round, nil
}

// ListRounds returns all training rounds.
func (ft *FederatedTrainer) ListRounds() []*TrainingRound {
	ft.mu.RLock()
	defer ft.mu.RUnlock()

	rounds := make([]*TrainingRound, 0, len(ft.rounds))
	for _, r := range ft.rounds {
		rounds = append(rounds, r)
	}
	return rounds
}

// Stats returns statistics about the training process.
func (ft *FederatedTrainer) Stats() *TrainingStats {
	ft.mu.RLock()
	defer ft.mu.RUnlock()

	completed := 0
	totalAccuracy := 0.0
	accuracyCount := 0

	for _, r := range ft.rounds {
		if r.Status == RoundStatusCompleted {
			completed++
			if acc, ok := r.Metrics["accuracy"]; ok {
				totalAccuracy += acc
				accuracyCount++
			}
		}
	}

	avgAcc := 0.0
	if accuracyCount > 0 {
		avgAcc = totalAccuracy / float64(accuracyCount)
	}

	return &TrainingStats{
		TotalRounds:     len(ft.rounds),
		CompletedRounds: completed,
		TotalClients:    len(ft.clients),
		AvgAccuracy:     avgAcc,
	}
}
