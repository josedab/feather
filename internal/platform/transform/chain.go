package transform

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

var (
	// ErrChainNotFound indicates the requested chain does not exist.
	ErrChainNotFound = errors.New("chain not found")
	// ErrChainCycle indicates a cycle was detected in the chain.
	ErrChainCycle = errors.New("cycle detected in chain")
	// ErrEmptyChain indicates the chain has no steps.
	ErrEmptyChain = errors.New("chain has no steps")
)

// ChainStatus indicates the chain's state.
type ChainStatus string

const (
	ChainStatusDraft  ChainStatus = "draft"
	ChainStatusActive ChainStatus = "active"
	ChainStatusPaused ChainStatus = "paused"
)

// ChainStep is a single step in a transform chain.
type ChainStep struct {
	Name       string                 `json:"name"`
	TemplateID string                 `json:"template_id,omitempty"`
	Transform  *Transform             `json:"transform,omitempty"`
	Config     map[string]interface{} `json:"config,omitempty"`
	Order      int                    `json:"order"`
}

// Chain is an ordered sequence of transforms that executes as a pipeline.
type Chain struct {
	ID             string      `json:"id"`
	Name           string      `json:"name"`
	Description    string      `json:"description,omitempty"`
	Steps          []*ChainStep `json:"steps"`
	Status         ChainStatus `json:"status"`
	Version        int         `json:"version"`
	CreatedAt      time.Time   `json:"created_at"`
	UpdatedAt      time.Time   `json:"updated_at"`
	ExecutionCount int64       `json:"execution_count"`
	AvgLatencyMs   float64     `json:"avg_latency_ms"`
}

// ChainResult holds the output of executing a chain.
type ChainResult struct {
	ChainID        string                 `json:"chain_id"`
	FinalOutput    map[string]interface{} `json:"final_output"`
	StepResults    []*StepResult          `json:"step_results"`
	TotalLatencyMs float64                `json:"total_latency_ms"`
	StepCount      int                    `json:"step_count"`
	Timestamp      time.Time              `json:"timestamp"`
}

// StepResult holds the output of a single chain step.
type StepResult struct {
	StepName  string                 `json:"step_name"`
	Output    map[string]interface{} `json:"output"`
	LatencyMs float64                `json:"latency_ms"`
	Success   bool                   `json:"success"`
	Error     string                 `json:"error,omitempty"`
}

// ChainEngine manages and executes transform chains.
type ChainEngine struct {
	mu      sync.RWMutex
	chains  map[string]*Chain
	catalog *Catalog
}

// NewChainEngine creates a new chain engine with the given catalog.
func NewChainEngine(catalog *Catalog) *ChainEngine {
	return &ChainEngine{
		chains:  make(map[string]*Chain),
		catalog: catalog,
	}
}

// CreateChain validates and adds a chain.
func (ce *ChainEngine) CreateChain(chain *Chain) error {
	if len(chain.Steps) == 0 {
		return ErrEmptyChain
	}

	ce.mu.Lock()
	defer ce.mu.Unlock()

	now := time.Now()
	chain.CreatedAt = now
	chain.UpdatedAt = now
	if chain.Status == "" {
		chain.Status = ChainStatusDraft
	}
	if chain.Version == 0 {
		chain.Version = 1
	}

	ce.chains[chain.ID] = chain
	return nil
}

// GetChain retrieves a chain by ID.
func (ce *ChainEngine) GetChain(id string) (*Chain, error) {
	ce.mu.RLock()
	defer ce.mu.RUnlock()

	chain, ok := ce.chains[id]
	if !ok {
		return nil, ErrChainNotFound
	}
	return chain, nil
}

// ListChains returns all chains.
func (ce *ChainEngine) ListChains() []*Chain {
	ce.mu.RLock()
	defer ce.mu.RUnlock()

	result := make([]*Chain, 0, len(ce.chains))
	for _, chain := range ce.chains {
		result = append(result, chain)
	}
	return result
}

// RemoveChain deletes a chain by ID.
func (ce *ChainEngine) RemoveChain(id string) error {
	ce.mu.Lock()
	defer ce.mu.Unlock()

	if _, ok := ce.chains[id]; !ok {
		return ErrChainNotFound
	}
	delete(ce.chains, id)
	return nil
}

// ActivateChain sets a chain's status to active.
func (ce *ChainEngine) ActivateChain(id string) error {
	ce.mu.Lock()
	defer ce.mu.Unlock()

	chain, ok := ce.chains[id]
	if !ok {
		return ErrChainNotFound
	}
	chain.Status = ChainStatusActive
	chain.UpdatedAt = time.Now()
	return nil
}

// PauseChain sets a chain's status to paused.
func (ce *ChainEngine) PauseChain(id string) error {
	ce.mu.Lock()
	defer ce.mu.Unlock()

	chain, ok := ce.chains[id]
	if !ok {
		return ErrChainNotFound
	}
	chain.Status = ChainStatusPaused
	chain.UpdatedAt = time.Now()
	return nil
}

// Execute runs a chain by iterating through its steps in order.
func (ce *ChainEngine) Execute(ctx context.Context, chainID string, input map[string]interface{}) (*ChainResult, error) {
	ce.mu.Lock()
	chain, ok := ce.chains[chainID]
	if !ok {
		ce.mu.Unlock()
		return nil, ErrChainNotFound
	}

	if chain.Status != ChainStatusActive {
		ce.mu.Unlock()
		return nil, fmt.Errorf("chain %s is not active (status: %s)", chainID, chain.Status)
	}
	ce.mu.Unlock()

	totalStart := time.Now()
	currentInput := copyMap(input)
	stepResults := make([]*StepResult, 0, len(chain.Steps))

	for _, step := range chain.Steps {
		stepStart := time.Now()

		output := copyMap(currentInput)

		// Merge step config into output to simulate transform application.
		if step.Config != nil {
			for k, v := range step.Config {
				output[k] = v
			}
		}

		latency := float64(time.Since(stepStart).Microseconds()) / 1000.0

		stepResults = append(stepResults, &StepResult{
			StepName:  step.Name,
			Output:    output,
			LatencyMs: latency,
			Success:   true,
		})

		currentInput = output
	}

	totalLatency := float64(time.Since(totalStart).Microseconds()) / 1000.0

	// Update execution stats.
	ce.mu.Lock()
	chain.ExecutionCount++
	chain.AvgLatencyMs = (chain.AvgLatencyMs*float64(chain.ExecutionCount-1) + totalLatency) / float64(chain.ExecutionCount)
	ce.mu.Unlock()

	return &ChainResult{
		ChainID:        chainID,
		FinalOutput:    currentInput,
		StepResults:    stepResults,
		TotalLatencyMs: totalLatency,
		StepCount:      len(chain.Steps),
		Timestamp:      time.Now(),
	}, nil
}

func copyMap(m map[string]interface{}) map[string]interface{} {
	cp := make(map[string]interface{}, len(m))
	for k, v := range m {
		cp[k] = v
	}
	return cp
}
