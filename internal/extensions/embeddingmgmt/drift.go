package embeddingmgmt

import (
	"errors"
	"fmt"
	"math"
	"sync"
	"time"
)

// VectorDriftConfig configures the drift detector.
type VectorDriftConfig struct {
	WindowSize     int
	DriftThreshold float64
	CheckInterval  time.Duration
	MinSamples     int
}

// DefaultVectorDriftConfig returns sensible defaults.
func DefaultVectorDriftConfig() VectorDriftConfig {
	return VectorDriftConfig{
		WindowSize:     1000,
		DriftThreshold: 0.1,
		CheckInterval:  5 * time.Minute,
		MinSamples:     50,
	}
}

// DriftStatus reports the drift state for a collection/model pair.
type DriftStatus struct {
	Collection   string       `json:"collection"`
	ModelID      string       `json:"model_id"`
	CurrentDrift float64      `json:"current_drift"`
	Threshold    float64      `json:"threshold"`
	IsDrifting   bool         `json:"is_drifting"`
	SampleCount  int          `json:"sample_count"`
	LastChecked  time.Time    `json:"last_checked"`
	DriftHistory []DriftPoint `json:"drift_history,omitempty"`
}

// DriftPoint records a drift measurement at a point in time.
type DriftPoint struct {
	Timestamp time.Time `json:"timestamp"`
	Value     float64   `json:"value"`
}

// Drift detector errors.
var (
	ErrNoReference      = errors.New("no reference distribution set")
	ErrInsufficientData = errors.New("insufficient samples for drift detection")
)

type driftKey struct {
	collection string
	modelID    string
}

type driftState struct {
	reference [][]float64 // reference centroid vectors
	refMean   []float64   // mean of reference vectors
	samples   [][]float64 // recent sample vectors (sliding window)
	history   []DriftPoint
	lastCheck time.Time
}

// VectorDriftDetector monitors embedding distributions for drift.
type VectorDriftDetector struct {
	mu     sync.RWMutex
	cfg    VectorDriftConfig
	states map[driftKey]*driftState
}

// NewVectorDriftDetector creates a new drift detector.
func NewVectorDriftDetector(cfg VectorDriftConfig) *VectorDriftDetector {
	if cfg.WindowSize <= 0 {
		cfg.WindowSize = DefaultVectorDriftConfig().WindowSize
	}
	if cfg.DriftThreshold <= 0 {
		cfg.DriftThreshold = DefaultVectorDriftConfig().DriftThreshold
	}
	if cfg.MinSamples <= 0 {
		cfg.MinSamples = DefaultVectorDriftConfig().MinSamples
	}
	return &VectorDriftDetector{
		cfg:    cfg,
		states: make(map[driftKey]*driftState),
	}
}

// SetReference sets the reference distribution for a collection/model pair.
func (d *VectorDriftDetector) SetReference(collection, modelID string, vectors [][]float64) error {
	if len(vectors) == 0 {
		return fmt.Errorf("at least one reference vector is required")
	}

	dim := len(vectors[0])
	for _, v := range vectors {
		if len(v) != dim {
			return ErrDimensionMismatch
		}
	}

	mean := computeMean(vectors)

	d.mu.Lock()
	defer d.mu.Unlock()

	key := driftKey{collection: collection, modelID: modelID}
	d.states[key] = &driftState{
		reference: vectors,
		refMean:   mean,
		samples:   make([][]float64, 0, d.cfg.WindowSize),
	}

	return nil
}

// RecordEmbedding records an incoming embedding for drift monitoring.
func (d *VectorDriftDetector) RecordEmbedding(collection, modelID string, vector []float32) {
	d.mu.Lock()
	defer d.mu.Unlock()

	key := driftKey{collection: collection, modelID: modelID}
	state, exists := d.states[key]
	if !exists {
		return
	}

	// Convert float32 to float64
	v := make([]float64, len(vector))
	for i, val := range vector {
		v[i] = float64(val)
	}

	// Sliding window: keep only the most recent WindowSize samples
	state.samples = append(state.samples, v)
	if len(state.samples) > d.cfg.WindowSize {
		state.samples = state.samples[len(state.samples)-d.cfg.WindowSize:]
	}
}

// CheckDrift computes the current drift for a collection/model pair.
func (d *VectorDriftDetector) CheckDrift(collection, modelID string) (*DriftStatus, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	key := driftKey{collection: collection, modelID: modelID}
	state, exists := d.states[key]
	if !exists {
		return nil, ErrNoReference
	}

	if len(state.samples) < d.cfg.MinSamples {
		return nil, fmt.Errorf("%w: have %d, need %d", ErrInsufficientData, len(state.samples), d.cfg.MinSamples)
	}

	// Compute drift as cosine distance between reference mean and sample mean
	sampleMean := computeMean(state.samples)
	drift := cosineDistance(state.refMean, sampleMean)

	now := time.Now()
	state.lastCheck = now
	point := DriftPoint{Timestamp: now, Value: drift}
	state.history = append(state.history, point)

	status := &DriftStatus{
		Collection:   collection,
		ModelID:      modelID,
		CurrentDrift: drift,
		Threshold:    d.cfg.DriftThreshold,
		IsDrifting:   drift > d.cfg.DriftThreshold,
		SampleCount:  len(state.samples),
		LastChecked:  now,
		DriftHistory: make([]DriftPoint, len(state.history)),
	}
	copy(status.DriftHistory, state.history)

	return status, nil
}

// ListMonitored returns drift status for all monitored collection/model pairs.
func (d *VectorDriftDetector) ListMonitored() []DriftStatus {
	d.mu.RLock()
	defer d.mu.RUnlock()

	result := make([]DriftStatus, 0, len(d.states))
	for key, state := range d.states {
		drift := 0.0
		isDrifting := false
		if len(state.samples) >= d.cfg.MinSamples {
			sampleMean := computeMean(state.samples)
			drift = cosineDistance(state.refMean, sampleMean)
			isDrifting = drift > d.cfg.DriftThreshold
		}
		result = append(result, DriftStatus{
			Collection:   key.collection,
			ModelID:      key.modelID,
			CurrentDrift: drift,
			Threshold:    d.cfg.DriftThreshold,
			IsDrifting:   isDrifting,
			SampleCount:  len(state.samples),
			LastChecked:  state.lastCheck,
		})
	}
	return result
}

// computeMean computes the element-wise mean of a set of vectors.
func computeMean(vectors [][]float64) []float64 {
	if len(vectors) == 0 {
		return nil
	}
	dim := len(vectors[0])
	mean := make([]float64, dim)
	for _, v := range vectors {
		for i := range v {
			mean[i] += v[i]
		}
	}
	n := float64(len(vectors))
	for i := range mean {
		mean[i] /= n
	}
	return mean
}

// cosineDistance returns 1 - cosine_similarity.
func cosineDistance(a, b []float64) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 1.0
	}
	var dot, normA, normB float64
	for i := range a {
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}
	denom := math.Sqrt(normA) * math.Sqrt(normB)
	if denom == 0 {
		return 1.0
	}
	return 1.0 - dot/denom
}
