package ml

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"sync"
	"time"
)

// Snapshot errors.
var (
	ErrSnapshotNotFound     = errors.New("snapshot not found")
	ErrSnapshotExists       = errors.New("snapshot already exists")
	ErrFeatureNotInSnapshot = errors.New("feature not in snapshot")
)

// DistributionType represents the type of feature distribution.
type DistributionType string

// DistributionType constants for snapshot data.
const (
	DistTypeNumeric     DistributionType = "numeric"
	DistTypeCategorical DistributionType = "categorical"
	DistTypeVector      DistributionType = "vector"
)

// FeatureSnapshot captures the statistical distribution of a feature at a point in time.
type FeatureSnapshot struct {
	// Name is the feature name
	Name string `json:"name"`
	// Type is the distribution type
	Type DistributionType `json:"type"`

	// Numeric statistics
	Count  int64   `json:"count"`
	Mean   float64 `json:"mean"`
	StdDev float64 `json:"std_dev"`
	Min    float64 `json:"min"`
	Max    float64 `json:"max"`
	Median float64 `json:"median"`
	P5     float64 `json:"p5"`  // 5th percentile
	P25    float64 `json:"p25"` // 25th percentile
	P75    float64 `json:"p75"` // 75th percentile
	P95    float64 `json:"p95"` // 95th percentile
	P99    float64 `json:"p99"` // 99th percentile

	// Histogram for numeric features (compressed representation)
	HistogramBuckets []float64 `json:"histogram_buckets,omitempty"`
	HistogramCounts  []int64   `json:"histogram_counts,omitempty"`

	// Categorical statistics
	Categories    map[string]int64   `json:"categories,omitempty"`
	CategoryRates map[string]float64 `json:"category_rates,omitempty"`
	Cardinality   int                `json:"cardinality,omitempty"`

	// Vector statistics (for embedding features)
	VectorDimension int     `json:"vector_dimension,omitempty"`
	VectorNormMean  float64 `json:"vector_norm_mean,omitempty"`
	VectorNormStd   float64 `json:"vector_norm_std,omitempty"`

	// Data quality metrics
	NullCount   int64   `json:"null_count"`
	NullRate    float64 `json:"null_rate"`
	UniqueCount int64   `json:"unique_count"`

	// Timestamp of snapshot creation
	CapturedAt time.Time `json:"captured_at"`
}

// TrainingSnapshot captures the feature distributions at model training time.
type TrainingSnapshot struct {
	// ID is the unique snapshot identifier
	ID string `json:"id"`
	// ModelID references the model this snapshot belongs to
	ModelID string `json:"model_id"`
	// ModelVersion is the model version this snapshot is for
	ModelVersion string `json:"model_version"`
	// Description provides context about this snapshot
	Description string `json:"description"`

	// Features maps feature name to its snapshot
	Features map[string]*FeatureSnapshot `json:"features"`

	// TrainingMetadata captures training context
	TrainingMetadata TrainingMetadata `json:"training_metadata"`

	// Timestamps
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TrainingMetadata captures context about the training data.
type TrainingMetadata struct {
	// DatasetName is the name of the training dataset
	DatasetName string `json:"dataset_name,omitempty"`
	// DatasetVersion is the version of the training dataset
	DatasetVersion string `json:"dataset_version,omitempty"`
	// SampleCount is the number of samples used
	SampleCount int64 `json:"sample_count"`
	// StartDate is the earliest data timestamp
	StartDate *time.Time `json:"start_date,omitempty"`
	// EndDate is the latest data timestamp
	EndDate *time.Time `json:"end_date,omitempty"`
	// TrainingJobID references the training job
	TrainingJobID string `json:"training_job_id,omitempty"`
	// ExtraMetadata for custom key-value pairs
	ExtraMetadata map[string]string `json:"extra_metadata,omitempty"`
}

// SnapshotStore manages training snapshots.
type SnapshotStore struct {
	mu        sync.RWMutex
	snapshots map[string]*TrainingSnapshot
	// byModel maps modelID:version to snapshot ID
	byModel map[string]string
}

// NewSnapshotStore creates a new snapshot store.
func NewSnapshotStore() *SnapshotStore {
	return &SnapshotStore{
		snapshots: make(map[string]*TrainingSnapshot),
		byModel:   make(map[string]string),
	}
}

// CreateSnapshot creates a new training snapshot.
func (s *SnapshotStore) CreateSnapshot(snapshot *TrainingSnapshot) error {
	if snapshot.ID == "" {
		return errors.New("snapshot ID is required")
	}
	if snapshot.ModelID == "" {
		return errors.New("model ID is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.snapshots[snapshot.ID]; exists {
		return fmt.Errorf("%w: %s", ErrSnapshotExists, snapshot.ID)
	}

	now := time.Now()
	snapshot.CreatedAt = now
	snapshot.UpdatedAt = now

	if snapshot.Features == nil {
		snapshot.Features = make(map[string]*FeatureSnapshot)
	}

	s.snapshots[snapshot.ID] = snapshot

	// Index by model
	if snapshot.ModelVersion != "" {
		key := fmt.Sprintf("%s:%s", snapshot.ModelID, snapshot.ModelVersion)
		s.byModel[key] = snapshot.ID
	}

	return nil
}

// GetSnapshot retrieves a snapshot by ID.
func (s *SnapshotStore) GetSnapshot(id string) (*TrainingSnapshot, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	snapshot, exists := s.snapshots[id]
	if !exists {
		return nil, fmt.Errorf("%w: %s", ErrSnapshotNotFound, id)
	}
	return snapshot, nil
}

// GetSnapshotForModel retrieves the snapshot for a specific model version.
func (s *SnapshotStore) GetSnapshotForModel(modelID, version string) (*TrainingSnapshot, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	key := fmt.Sprintf("%s:%s", modelID, version)
	snapshotID, exists := s.byModel[key]
	if !exists {
		return nil, fmt.Errorf("%w: model %s version %s", ErrSnapshotNotFound, modelID, version)
	}

	return s.snapshots[snapshotID], nil
}

// UpdateSnapshot updates an existing snapshot.
func (s *SnapshotStore) UpdateSnapshot(snapshot *TrainingSnapshot) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.snapshots[snapshot.ID]; !exists {
		return fmt.Errorf("%w: %s", ErrSnapshotNotFound, snapshot.ID)
	}

	snapshot.UpdatedAt = time.Now()
	s.snapshots[snapshot.ID] = snapshot

	return nil
}

// DeleteSnapshot removes a snapshot.
func (s *SnapshotStore) DeleteSnapshot(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	snapshot, exists := s.snapshots[id]
	if !exists {
		return fmt.Errorf("%w: %s", ErrSnapshotNotFound, id)
	}

	// Remove from model index
	if snapshot.ModelVersion != "" {
		key := fmt.Sprintf("%s:%s", snapshot.ModelID, snapshot.ModelVersion)
		delete(s.byModel, key)
	}

	delete(s.snapshots, id)
	return nil
}

// ListSnapshots returns all snapshots.
func (s *SnapshotStore) ListSnapshots() []*TrainingSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*TrainingSnapshot, 0, len(s.snapshots))
	for _, snapshot := range s.snapshots {
		result = append(result, snapshot)
	}
	return result
}

// ListSnapshotsForModel returns all snapshots for a model.
func (s *SnapshotStore) ListSnapshotsForModel(modelID string) []*TrainingSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*TrainingSnapshot, 0)
	for _, snapshot := range s.snapshots {
		if snapshot.ModelID == modelID {
			result = append(result, snapshot)
		}
	}
	return result
}

// SnapshotBuilder helps construct feature snapshots from sample data.
type SnapshotBuilder struct {
	snapshot *TrainingSnapshot
	samples  map[string][]interface{}
	mu       sync.Mutex
}

// NewSnapshotBuilder creates a new snapshot builder.
func NewSnapshotBuilder(modelID, modelVersion, description string) *SnapshotBuilder {
	return &SnapshotBuilder{
		snapshot: &TrainingSnapshot{
			ID:           fmt.Sprintf("snap_%s_%s_%d", modelID, modelVersion, time.Now().UnixNano()),
			ModelID:      modelID,
			ModelVersion: modelVersion,
			Description:  description,
			Features:     make(map[string]*FeatureSnapshot),
		},
		samples: make(map[string][]interface{}),
	}
}

// AddSample adds a sample value for a feature.
func (b *SnapshotBuilder) AddSample(featureName string, value interface{}) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.samples[featureName] = append(b.samples[featureName], value)
}

// AddSamples adds multiple samples for a feature.
func (b *SnapshotBuilder) AddSamples(featureName string, values []interface{}) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.samples[featureName] = append(b.samples[featureName], values...)
}

// SetTrainingMetadata sets the training metadata.
func (b *SnapshotBuilder) SetTrainingMetadata(meta TrainingMetadata) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.snapshot.TrainingMetadata = meta
}

// Build computes statistics and returns the snapshot.
func (b *SnapshotBuilder) Build() *TrainingSnapshot {
	b.mu.Lock()
	defer b.mu.Unlock()

	now := time.Now()

	for featureName, samples := range b.samples {
		if len(samples) == 0 {
			continue
		}

		fs := &FeatureSnapshot{
			Name:       featureName,
			Count:      int64(len(samples)),
			CapturedAt: now,
		}

		// Determine type from first non-nil sample
		var firstNonNil interface{}
		for _, s := range samples {
			if s != nil {
				firstNonNil = s
				break
			}
		}

		switch v := firstNonNil.(type) {
		case float64, float32, int, int64, int32:
			fs.Type = DistTypeNumeric
			b.computeNumericStats(fs, samples)
		case string:
			fs.Type = DistTypeCategorical
			b.computeCategoricalStats(fs, samples)
		case []float32, []float64:
			fs.Type = DistTypeVector
			b.computeVectorStats(fs, samples, v)
		default:
			// Default to categorical for unknown types
			fs.Type = DistTypeCategorical
			b.computeCategoricalStats(fs, samples)
		}

		b.snapshot.Features[featureName] = fs
	}

	return b.snapshot
}

func (b *SnapshotBuilder) computeNumericStats(fs *FeatureSnapshot, samples []interface{}) {
	values := make([]float64, 0, len(samples))
	var nullCount int64

	for _, s := range samples {
		if s == nil {
			nullCount++
			continue
		}

		var val float64
		switch v := s.(type) {
		case float64:
			val = v
		case float32:
			val = float64(v)
		case int:
			val = float64(v)
		case int64:
			val = float64(v)
		case int32:
			val = float64(v)
		default:
			continue
		}
		values = append(values, val)
	}

	fs.NullCount = nullCount
	fs.NullRate = float64(nullCount) / float64(fs.Count)

	if len(values) == 0 {
		return
	}

	// Sort for percentile calculations
	sort.Float64s(values)

	// Basic statistics
	var sum float64
	fs.Min = values[0]
	fs.Max = values[len(values)-1]

	for _, v := range values {
		sum += v
	}
	fs.Mean = sum / float64(len(values))

	// Standard deviation
	var variance float64
	for _, v := range values {
		diff := v - fs.Mean
		variance += diff * diff
	}
	variance /= float64(len(values))
	fs.StdDev = math.Sqrt(variance)

	// Percentiles
	fs.P5 = percentile(values, 5)
	fs.P25 = percentile(values, 25)
	fs.Median = percentile(values, 50)
	fs.P75 = percentile(values, 75)
	fs.P95 = percentile(values, 95)
	fs.P99 = percentile(values, 99)

	// Unique count
	uniqueMap := make(map[float64]bool)
	for _, v := range values {
		uniqueMap[v] = true
	}
	fs.UniqueCount = int64(len(uniqueMap))

	// Build histogram (10 buckets)
	numBuckets := 10
	if len(values) < numBuckets {
		numBuckets = len(values)
	}

	if numBuckets > 0 && fs.Max > fs.Min {
		bucketWidth := (fs.Max - fs.Min) / float64(numBuckets)
		fs.HistogramBuckets = make([]float64, numBuckets+1)
		fs.HistogramCounts = make([]int64, numBuckets)

		for i := 0; i <= numBuckets; i++ {
			fs.HistogramBuckets[i] = fs.Min + float64(i)*bucketWidth
		}

		for _, v := range values {
			bucket := int((v - fs.Min) / bucketWidth)
			if bucket >= numBuckets {
				bucket = numBuckets - 1
			}
			fs.HistogramCounts[bucket]++
		}
	}
}

func (b *SnapshotBuilder) computeCategoricalStats(fs *FeatureSnapshot, samples []interface{}) {
	categories := make(map[string]int64)
	var nullCount int64

	for _, s := range samples {
		if s == nil {
			nullCount++
			continue
		}

		var strVal string
		switch v := s.(type) {
		case string:
			strVal = v
		default:
			strVal = fmt.Sprintf("%v", v)
		}
		categories[strVal]++
	}

	fs.NullCount = nullCount
	fs.NullRate = float64(nullCount) / float64(fs.Count)
	fs.Categories = categories
	fs.Cardinality = len(categories)
	fs.UniqueCount = int64(len(categories))

	// Compute rates
	total := float64(fs.Count - nullCount)
	if total > 0 {
		fs.CategoryRates = make(map[string]float64, len(categories))
		for cat, count := range categories {
			fs.CategoryRates[cat] = float64(count) / total
		}
	}
}

func (b *SnapshotBuilder) computeVectorStats(fs *FeatureSnapshot, samples []interface{}, first interface{}) {
	norms := make([]float64, 0, len(samples))
	var dimension int

	switch v := first.(type) {
	case []float32:
		dimension = len(v)
	case []float64:
		dimension = len(v)
	}

	fs.VectorDimension = dimension

	for _, s := range samples {
		if s == nil {
			fs.NullCount++
			continue
		}

		var norm float64
		switch v := s.(type) {
		case []float32:
			for _, x := range v {
				norm += float64(x) * float64(x)
			}
		case []float64:
			for _, x := range v {
				norm += x * x
			}
		}
		norms = append(norms, math.Sqrt(norm))
	}

	fs.NullRate = float64(fs.NullCount) / float64(fs.Count)

	if len(norms) > 0 {
		// Compute norm statistics
		var sum float64
		for _, n := range norms {
			sum += n
		}
		fs.VectorNormMean = sum / float64(len(norms))

		var variance float64
		for _, n := range norms {
			diff := n - fs.VectorNormMean
			variance += diff * diff
		}
		variance /= float64(len(norms))
		fs.VectorNormStd = math.Sqrt(variance)
	}
}

func percentile(sorted []float64, p int) float64 {
	if len(sorted) == 0 {
		return 0
	}
	if p <= 0 {
		return sorted[0]
	}
	if p >= 100 {
		return sorted[len(sorted)-1]
	}

	idx := float64(len(sorted)-1) * float64(p) / 100.0
	lower := int(math.Floor(idx))
	upper := int(math.Ceil(idx))

	if lower == upper || upper >= len(sorted) {
		return sorted[lower]
	}

	fraction := idx - float64(lower)
	return sorted[lower] + fraction*(sorted[upper]-sorted[lower])
}

// MarshalJSON implements json.Marshaler.
func (s *SnapshotStore) MarshalJSON() ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return json.Marshal(map[string]interface{}{
		"snapshots": s.snapshots,
	})
}
